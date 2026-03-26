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
	"github.com/ehoneahobed/dmail/pkg/names"
	"github.com/ehoneahobed/dmail/pkg/node"
	"github.com/ehoneahobed/dmail/pkg/packet"
	"github.com/ehoneahobed/dmail/pkg/pow"
	"github.com/ehoneahobed/dmail/pkg/store"

	"github.com/libp2p/go-libp2p/core/peer"
)

// Daemon is the core Dmail daemon that ties together networking, storage, and the API.
type Daemon struct {
	Node    *node.Node
	Store   *store.Store
	KeyPair *dmcrypto.KeyPair

	pendingPoW atomic.Int32
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

// Config holds daemon startup parameters.
type Config struct {
	ListenPort     int
	DataDir        string // path to SQLite database file
	KeyPair        *dmcrypto.KeyPair
	BootstrapPeers []peer.AddrInfo
	PollInterval   time.Duration // default 60s
}

// New creates and starts a new Daemon.
func New(ctx context.Context, cfg Config) (*Daemon, error) {
	// Open SQLite store.
	st, err := store.Open(cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}

	// Save keypair to store.
	pubHex := hex.EncodeToString(cfg.KeyPair.PublicKey)
	privHex := hex.EncodeToString(cfg.KeyPair.PrivateKey)
	if err := st.SaveKeyPair(pubHex, privHex); err != nil {
		st.Close()
		return nil, fmt.Errorf("save keypair: %w", err)
	}

	// Create the P2P node.
	n, err := node.New(ctx, node.Config{
		ListenPort:     cfg.ListenPort,
		KeyPair:        cfg.KeyPair,
		BootstrapPeers: cfg.BootstrapPeers,
	})
	if err != nil {
		st.Close()
		return nil, fmt.Errorf("create node: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	d := &Daemon{
		Node:    n,
		Store:   st,
		KeyPair: cfg.KeyPair,
		cancel:  cancel,
	}

	// Start background DHT poller.
	interval := cfg.PollInterval
	if interval == 0 {
		interval = 60 * time.Second
	}
	d.wg.Add(1)
	go d.pollLoop(ctx, interval)

	return d, nil
}

// SendMessage encrypts, PoW-stamps, and pushes a message to the DHT.
// The PoW runs in a background goroutine so it doesn't block the API.
func (d *Daemon) SendMessage(ctx context.Context, recipientAddr, subject, body, replyToID string) error {
	recipientPub, err := dmcrypto.PubKeyFromAddress(recipientAddr)
	if err != nil {
		return fmt.Errorf("invalid recipient address: %w", err)
	}

	d.pendingPoW.Add(1)
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		defer d.pendingPoW.Add(-1)

		pkt, err := packet.Build(subject, body, replyToID, d.KeyPair, recipientPub, pow.DefaultDifficulty)
		if err != nil {
			log.Printf("ERROR build packet: %v", err)
			return
		}

		// Decrypt the payload we just built to store locally in sent folder.
		payload, err := packet.Decrypt(pkt, d.KeyPair.PrivateKey)
		if err != nil {
			// We encrypted with recipient's key — we can't decrypt it ourselves.
			// Store with the info we have.
			msg := &store.Message{
				ID:              subject, // fallback
				Folder:          "sent",
				SenderPubkey:    dmcrypto.Address(d.KeyPair.PublicKey),
				RecipientPubkey: recipientAddr,
				Subject:         subject,
				Body:            body,
				Timestamp:       time.Now().Unix(),
				IsRead:          true,
			}
			d.Store.SaveMessage(msg)
		} else {
			msg := &store.Message{
				ID:              payload.MessageId,
				Folder:          "sent",
				SenderPubkey:    dmcrypto.Address(d.KeyPair.PublicKey),
				RecipientPubkey: recipientAddr,
				Subject:         payload.Subject,
				Body:            payload.Body,
				Timestamp:       int64(pkt.Timestamp),
				IsRead:          true,
			}
			d.Store.SaveMessage(msg)
		}

		if err := d.Node.Push(ctx, pkt); err != nil {
			log.Printf("ERROR push to DHT: %v", err)
			return
		}
		log.Printf("message sent to %s", recipientAddr)
	}()

	return nil
}

// PendingPoWTasks returns the number of in-flight PoW calculations.
func (d *Daemon) PendingPoWTasks() int {
	return int(d.pendingPoW.Load())
}

// ConnectedPeers returns the number of connected DHT peers.
func (d *Daemon) ConnectedPeers() int {
	return len(d.Node.Host.Network().Peers())
}

// RegisterName starts an async goroutine to compute PoW and register a .dmail name.
func (d *Daemon) RegisterName(ctx context.Context, name string) error {
	if err := names.ValidateName(name); err != nil {
		return err
	}

	d.pendingPoW.Add(1)
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		defer d.pendingPoW.Add(-1)

		rec, err := names.BuildNameRecord(name, d.KeyPair)
		if err != nil {
			log.Printf("ERROR build name record: %v", err)
			return
		}

		if err := d.Node.RegisterName(ctx, rec); err != nil {
			log.Printf("ERROR register name on DHT: %v", err)
			return
		}

		// Save locally.
		raw, _ := names.Marshal(rec)
		entry := &store.NameEntry{
			Name:      name,
			Pubkey:    dmcrypto.Address(d.KeyPair.PublicKey),
			Timestamp: int64(rec.Timestamp),
			RawRecord: raw,
			IsMine:    true,
		}
		if err := d.Store.SaveName(entry); err != nil {
			log.Printf("ERROR save name: %v", err)
			return
		}
		log.Printf("registered name %s.dmail", name)
	}()

	return nil
}

// ResolveName looks up a .dmail name, checking local cache first then DHT.
func (d *Daemon) ResolveName(ctx context.Context, name string) (string, error) {
	name = strings.ToLower(strings.TrimSuffix(name, ".dmail"))

	// Check local cache first.
	entry, err := d.Store.GetName(name)
	if err != nil {
		return "", fmt.Errorf("check local names: %w", err)
	}
	if entry != nil {
		return entry.Pubkey, nil
	}

	// DHT lookup.
	rec, err := d.Node.ResolveName(ctx, name)
	if err != nil {
		return "", fmt.Errorf("resolve name from DHT: %w", err)
	}

	// Cache result locally.
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

// pollLoop periodically queries the DHT for new messages.
func (d *Daemon) pollLoop(ctx context.Context, interval time.Duration) {
	defer d.wg.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Do an initial poll immediately.
	d.doPoll(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.doPoll(ctx)
		}
	}
}

func (d *Daemon) doPoll(ctx context.Context) {
	pkt, err := d.Node.Poll(ctx)
	if err != nil {
		// No message or error — both are normal during polling.
		return
	}

	// Decrypt the payload.
	payload, err := packet.Decrypt(pkt, d.KeyPair.PrivateKey)
	if err != nil {
		log.Printf("WARNING: received packet but decryption failed: %v", err)
		return
	}

	// Check if we already have this message.
	has, err := d.Store.HasMessage(payload.MessageId)
	if err != nil {
		log.Printf("ERROR check message: %v", err)
		return
	}
	if has {
		return
	}

	msg := &store.Message{
		ID:              payload.MessageId,
		Folder:          "inbox",
		SenderPubkey:    dmcrypto.Address(pkt.SenderPubkey),
		RecipientPubkey: dmcrypto.Address(pkt.RecipientPubkey),
		Subject:         payload.Subject,
		Body:            payload.Body,
		Timestamp:       int64(pkt.Timestamp),
		IsRead:          false,
	}

	if err := d.Store.SaveMessage(msg); err != nil {
		log.Printf("ERROR save message: %v", err)
		return
	}
	log.Printf("received message %s from %s", payload.MessageId, msg.SenderPubkey)
}

// Close shuts down the daemon gracefully.
func (d *Daemon) Close() error {
	d.cancel()
	d.wg.Wait()
	if err := d.Node.Close(); err != nil {
		return err
	}
	return d.Store.Close()
}
