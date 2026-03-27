package daemon

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	dmcrypto "github.com/ehoneahobed/dmail/pkg/crypto"
	"github.com/ehoneahobed/dmail/pkg/auth"
	"github.com/ehoneahobed/dmail/pkg/names"
	"github.com/ehoneahobed/dmail/pkg/node"
	"github.com/ehoneahobed/dmail/pkg/packet"
	"github.com/ehoneahobed/dmail/pkg/pow"
	"github.com/ehoneahobed/dmail/pkg/store"

	"google.golang.org/protobuf/proto"

	"github.com/libp2p/go-libp2p/core/peer"
)

// MultiTenantDaemon manages a shared libp2p node for multiple users.
type MultiTenantDaemon struct {
	Node         *node.Node
	Store        *store.Store
	Sessions     *auth.SessionCache
	JWTSecret    []byte

	// The service identity keypair (used for DHT node identity).
	ServiceKeyPair *dmcrypto.KeyPair

	pendingPoW atomic.Int32
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

// MultiTenantConfig holds configuration for multi-tenant mode.
type MultiTenantConfig struct {
	ListenPort     int
	DataDir        string
	ServiceKeyPair *dmcrypto.KeyPair
	BootstrapPeers []peer.AddrInfo
	PollInterval   time.Duration
	JWTSecret      []byte
}

// NewMultiTenant creates and starts a multi-tenant daemon.
func NewMultiTenant(ctx context.Context, cfg MultiTenantConfig) (*MultiTenantDaemon, error) {
	st, err := store.Open(cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}

	if err := st.MigrateMultiTenant(); err != nil {
		st.Close()
		return nil, fmt.Errorf("migrate multi-tenant: %w", err)
	}

	// Save the service keypair.
	pubHex := hex.EncodeToString(cfg.ServiceKeyPair.PublicKey)
	privHex := hex.EncodeToString(cfg.ServiceKeyPair.PrivateKey)
	if err := st.SaveKeyPair(pubHex, privHex); err != nil {
		st.Close()
		return nil, fmt.Errorf("save service keypair: %w", err)
	}

	n, err := node.New(ctx, node.Config{
		ListenPort:     cfg.ListenPort,
		KeyPair:        cfg.ServiceKeyPair,
		BootstrapPeers: cfg.BootstrapPeers,
	})
	if err != nil {
		st.Close()
		return nil, fmt.Errorf("create node: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	d := &MultiTenantDaemon{
		Node:           n,
		Store:          st,
		Sessions:       auth.NewSessionCache(),
		JWTSecret:      cfg.JWTSecret,
		ServiceKeyPair: cfg.ServiceKeyPair,
		cancel:         cancel,
	}

	interval := cfg.PollInterval
	if interval == 0 {
		interval = 60 * time.Second
	}
	d.wg.Add(1)
	go d.pollLoop(ctx, interval)

	return d, nil
}

// SendMessageForUser sends a message on behalf of a specific user.
func (d *MultiTenantDaemon) SendMessageForUser(ctx context.Context, userID, recipientAddr, subject, body, replyToID string) error {
	session := d.Sessions.Get(userID)
	if session == nil {
		return fmt.Errorf("session expired, please login again")
	}

	recipientPub, err := dmcrypto.PubKeyFromAddress(recipientAddr)
	if err != nil {
		return fmt.Errorf("invalid recipient address: %w", err)
	}

	kp := &dmcrypto.KeyPair{
		PublicKey:  session.PublicKey,
		PrivateKey: session.PrivateKey,
	}

	senderAddr := dmcrypto.Address(kp.PublicKey)
	msgID := fmt.Sprintf("msg-%d", time.Now().UnixNano())
	now := time.Now().Unix()

	// Save sent copy for sender immediately.
	sentMsg := &store.Message{
		ID:              msgID,
		Folder:          "sent",
		SenderPubkey:    senderAddr,
		RecipientPubkey: recipientAddr,
		Subject:         subject,
		Body:            body,
		Timestamp:       now,
		IsRead:          true,
		ReplyToID:       replyToID,
		Status:          "sending",
	}
	d.Store.SaveMessageForUser(userID, sentMsg)

	// Check if recipient is a local user — deliver directly without DHT.
	localRecipient, _ := d.Store.GetUserByPubkey(recipientAddr)
	if localRecipient != nil {
		inboxMsg := &store.Message{
			ID:              msgID,
			Folder:          "inbox",
			SenderPubkey:    senderAddr,
			RecipientPubkey: recipientAddr,
			Subject:         subject,
			Body:            body,
			Timestamp:       now,
			IsRead:          false,
			ReplyToID:       replyToID,
		}
		d.Store.SaveMessageForUser(localRecipient.ID, inboxMsg)
		// Mark sender's copy as delivered.
		d.Store.UpdateMessageStatusForUser(userID, msgID, "delivered")
		log.Printf("message delivered locally from user %s to user %s", userID, localRecipient.ID)
		return nil
	}

	// Remote recipient — build encrypted packet and push to DHT.
	d.pendingPoW.Add(1)
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		defer d.pendingPoW.Add(-1)

		pkt, err := packet.Build(subject, body, replyToID, kp, recipientPub, pow.DefaultDifficulty)
		if err != nil {
			log.Printf("ERROR build packet for user %s: %v", userID, err)
			return
		}

		if err := d.Node.Push(ctx, pkt); err != nil {
			log.Printf("ERROR push to DHT for user %s: %v", userID, err)
			return
		}
		// Update status to sent after DHT push.
		d.Store.UpdateMessageStatusForUser(userID, msgID, "sent")
		log.Printf("message sent via DHT for user %s to %s", userID, recipientAddr)
	}()

	return nil
}

// SendMessageFederated sends a message to a user on a remote federated server.
func (d *MultiTenantDaemon) SendMessageFederated(ctx context.Context, userID, username, host, subject, body, replyToID string) error {
	session := d.Sessions.Get(userID)
	if session == nil {
		return fmt.Errorf("session expired, please login again")
	}

	// Resolve the remote user's pubkey via federation.
	recipientAddr, err := resolveViaFederation(ctx, username, host)
	if err != nil {
		return fmt.Errorf("resolve %s@%s: %w", username, host, err)
	}

	recipientPub, err := dmcrypto.PubKeyFromAddress(recipientAddr)
	if err != nil {
		return fmt.Errorf("invalid recipient pubkey from remote: %w", err)
	}

	kp := &dmcrypto.KeyPair{
		PublicKey:  session.PublicKey,
		PrivateKey: session.PrivateKey,
	}

	senderAddr := dmcrypto.Address(kp.PublicKey)
	msgID := fmt.Sprintf("msg-%d", time.Now().UnixNano())
	now := time.Now().Unix()

	// Save sent copy for sender immediately.
	sentMsg := &store.Message{
		ID:              msgID,
		Folder:          "sent",
		SenderPubkey:    senderAddr,
		RecipientPubkey: recipientAddr,
		Subject:         subject,
		Body:            body,
		Timestamp:       now,
		IsRead:          true,
		ReplyToID:       replyToID,
		Status:          "sending",
	}
	d.Store.SaveMessageForUser(userID, sentMsg)

	// Build and deliver in background (PoW takes time).
	d.pendingPoW.Add(1)
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		defer d.pendingPoW.Add(-1)

		pkt, err := packet.Build(subject, body, replyToID, kp, recipientPub, pow.DefaultDifficulty)
		if err != nil {
			log.Printf("ERROR build packet for federated send: %v", err)
			return
		}

		pktData, err := proto.Marshal(pkt)
		if err != nil {
			log.Printf("ERROR marshal packet for federated send: %v", err)
			return
		}

		// Try federation delivery first.
		deliverCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := deliverViaFederation(deliverCtx, host, pktData); err != nil {
			// Fall back to DHT push.
			log.Printf("WARNING federation delivery to %s failed (%v), falling back to DHT", host, err)
			if dhtErr := d.Node.Push(context.Background(), pkt); dhtErr != nil {
				log.Printf("ERROR DHT fallback also failed for user %s: %v", userID, dhtErr)
				return
			}
			d.Store.UpdateMessageStatusForUser(userID, msgID, "sent")
			log.Printf("message sent via DHT fallback for user %s to %s@%s", userID, username, host)
			return
		}

		d.Store.UpdateMessageStatusForUser(userID, msgID, "delivered")
		log.Printf("message delivered via federation for user %s to %s@%s", userID, username, host)
	}()

	return nil
}

// ResolveNameForUser resolves a .dmail name, checking cache first.
func (d *MultiTenantDaemon) ResolveNameForUser(ctx context.Context, name string) (string, error) {
	name = strings.ToLower(strings.TrimSuffix(name, ".dmail"))

	entry, err := d.Store.GetName(name)
	if err != nil {
		return "", fmt.Errorf("check local names: %w", err)
	}
	if entry != nil {
		return entry.Pubkey, nil
	}

	rec, err := d.Node.ResolveName(ctx, name)
	if err != nil {
		return "", fmt.Errorf("resolve name from DHT: %w", err)
	}

	raw, _ := names.Marshal(rec)
	addr := dmcrypto.Address(rec.Pubkey)
	cacheEntry := &store.NameEntry{
		Name:      name,
		Pubkey:    addr,
		Timestamp: int64(rec.Timestamp),
		RawRecord: raw,
		IsMine:    false,
	}
	d.Store.SaveName(cacheEntry)

	return addr, nil
}

// ProcessPendingPackets decrypts pending packets for a user who just logged in.
func (d *MultiTenantDaemon) ProcessPendingPackets(userID string) {
	session := d.Sessions.Get(userID)
	if session == nil {
		return
	}

	packets, err := d.Store.GetPendingPackets(userID)
	if err != nil {
		log.Printf("ERROR get pending packets for %s: %v", userID, err)
		return
	}

	for _, pp := range packets {
		payload, err := packet.DecryptRaw(pp.RawData, session.PrivateKey)
		if err != nil {
			log.Printf("WARNING: failed to decrypt pending packet %d for user %s: %v", pp.ID, userID, err)
			d.Store.MarkPacketProcessed(pp.ID)
			continue
		}

		msg := &store.Message{
			ID:              payload.MessageId,
			Folder:          "inbox",
			SenderPubkey:    pp.SenderPubkey,
			RecipientPubkey: dmcrypto.Address(session.PublicKey),
			Subject:         payload.Subject,
			Body:            payload.Body,
			Timestamp:       pp.ReceivedAt,
			IsRead:          false,
		}
		d.Store.SaveMessageForUser(userID, msg)
		d.Store.MarkPacketProcessed(pp.ID)
	}
}

// PendingPoWTasks returns the number of in-flight PoW calculations.
func (d *MultiTenantDaemon) PendingPoWTasks() int {
	return int(d.pendingPoW.Load())
}

// ConnectedPeers returns the number of connected DHT peers.
func (d *MultiTenantDaemon) ConnectedPeers() int {
	return len(d.Node.Host.Network().Peers())
}

// pollLoop periodically queries the DHT for all registered users.
func (d *MultiTenantDaemon) pollLoop(ctx context.Context, interval time.Duration) {
	defer d.wg.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Periodic cleanup of expired sessions.
	cleanupTicker := time.NewTicker(5 * time.Minute)
	defer cleanupTicker.Stop()

	d.doPollAll(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.doPollAll(ctx)
		case <-cleanupTicker.C:
			d.Sessions.EvictExpired()
		}
	}
}

func (d *MultiTenantDaemon) doPollAll(ctx context.Context) {
	// Get all user public keys to poll for.
	pubkeys, err := d.Store.GetAllUserPubKeys()
	if err != nil {
		log.Printf("ERROR list user pubkeys: %v", err)
		return
	}

	for _, pkAddr := range pubkeys {
		pubKey, err := dmcrypto.PubKeyFromAddress(pkAddr)
		if err != nil {
			continue
		}

		key := node.RoutingKey(pubKey)
		pollCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		data, err := d.Node.DHT.GetValue(pollCtx, key)
		cancel()
		if err != nil {
			continue
		}

		// Find user by pubkey.
		users, _ := d.Store.ListUsers()
		for _, u := range users {
			if u.Pubkey == pkAddr {
				senderPub := ""
				// Try to extract sender from packet for display.
				if pkt, err := packet.UnmarshalAndVerify(data); err == nil {
					senderPub = dmcrypto.Address(pkt.SenderPubkey)
				}
				d.Store.SavePendingPacket(u.ID, data, senderPub)
				break
			}
		}
	}
}

// Close shuts down the daemon gracefully.
func (d *MultiTenantDaemon) Close() error {
	d.cancel()
	d.wg.Wait()
	if err := d.Node.Close(); err != nil {
		return err
	}
	return d.Store.Close()
}
