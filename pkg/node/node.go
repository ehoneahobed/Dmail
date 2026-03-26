package node

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/ehoneahobed/dmail/internal/pb"
	dmcrypto "github.com/ehoneahobed/dmail/pkg/crypto"
	"github.com/ehoneahobed/dmail/pkg/names"
	"github.com/ehoneahobed/dmail/pkg/packet"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	record "github.com/libp2p/go-libp2p-record"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"google.golang.org/protobuf/proto"
)

// Node wraps a libp2p host and Kademlia DHT for Dmail messaging.
type Node struct {
	Host    host.Host
	DHT     *dht.IpfsDHT
	KeyPair *dmcrypto.KeyPair
}

// Config holds parameters for creating a new Node.
type Config struct {
	ListenPort     int
	KeyPair        *dmcrypto.KeyPair
	BootstrapPeers []peer.AddrInfo
}

// dmailValidator validates records stored under the "/dmail/" DHT namespace.
type dmailValidator struct{}

func (dv dmailValidator) Validate(key string, value []byte) error {
	if len(value) > packet.MaxPacketSize {
		return fmt.Errorf("packet too large: %d bytes", len(value))
	}
	pkt := &pb.DmailPacket{}
	if err := proto.Unmarshal(value, pkt); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}
	return packet.Verify(pkt)
}

func (dv dmailValidator) Select(key string, values [][]byte) (int, error) {
	if len(values) == 0 {
		return 0, errors.New("no values")
	}
	// All valid copies are equivalent; return the first.
	return 0, nil
}

// New creates and starts a new Dmail node.
func New(ctx context.Context, cfg Config) (*Node, error) {
	listenAddr := fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", cfg.ListenPort)

	h, err := libp2p.New(
		libp2p.ListenAddrStrings(listenAddr),
	)
	if err != nil {
		return nil, fmt.Errorf("create libp2p host: %w", err)
	}

	// Create DHT in server mode with custom protocol prefix and validator.
	kadDHT, err := dht.New(ctx, h,
		dht.Mode(dht.ModeServer),
		dht.ProtocolPrefix("/dmail"),
		dht.NamespacedValidator("dmail", dmailValidator{}),
		dht.NamespacedValidator("names", namesValidator{}),
	)
	if err != nil {
		h.Close()
		return nil, fmt.Errorf("create DHT: %w", err)
	}

	if err := kadDHT.Bootstrap(ctx); err != nil {
		h.Close()
		return nil, fmt.Errorf("bootstrap DHT: %w", err)
	}

	// Connect to bootstrap peers (non-fatal on failure).
	for _, pi := range cfg.BootstrapPeers {
		if err := h.Connect(ctx, pi); err != nil {
			fmt.Printf("warning: failed to connect to peer %s: %v\n", pi.ID, err)
		}
	}

	return &Node{
		Host:    h,
		DHT:     kadDHT,
		KeyPair: cfg.KeyPair,
	}, nil
}

// RoutingKey returns the DHT key for a public key: "/dmail/<sha256hex>".
func RoutingKey(pubKey ed25519.PublicKey) string {
	hash := sha256.Sum256(pubKey)
	return "/dmail/" + fmt.Sprintf("%x", hash)
}

// Push stores a DmailPacket in the DHT under the recipient's routing key.
func (n *Node) Push(ctx context.Context, pkt *pb.DmailPacket) error {
	data, err := proto.Marshal(pkt)
	if err != nil {
		return fmt.Errorf("marshal packet: %w", err)
	}
	if len(data) > packet.MaxPacketSize {
		return fmt.Errorf("packet too large: %d bytes", len(data))
	}

	key := RoutingKey(pkt.RecipientPubkey)
	if err := n.DHT.PutValue(ctx, key, data); err != nil {
		return fmt.Errorf("DHT put: %w", err)
	}
	return nil
}

// Poll queries the DHT for a packet addressed to this node.
func (n *Node) Poll(ctx context.Context) (*pb.DmailPacket, error) {
	key := RoutingKey(n.KeyPair.PublicKey)

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	data, err := n.DHT.GetValue(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("DHT get: %w", err)
	}

	if len(data) > packet.MaxPacketSize {
		return nil, fmt.Errorf("packet too large: %d bytes", len(data))
	}

	pkt := &pb.DmailPacket{}
	if err := proto.Unmarshal(data, pkt); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	// Already validated by the DHT validator, but verify again for defense in depth.
	if err := packet.Verify(pkt); err != nil {
		return nil, fmt.Errorf("verify: %w", err)
	}

	return pkt, nil
}

// RegisterName stores a NameRecord in the DHT under the names namespace.
func (n *Node) RegisterName(ctx context.Context, rec *pb.NameRecord) error {
	data, err := names.Marshal(rec)
	if err != nil {
		return fmt.Errorf("marshal name record: %w", err)
	}
	key := names.RoutingKey(rec.Name)
	if err := n.DHT.PutValue(ctx, key, data); err != nil {
		return fmt.Errorf("DHT put name: %w", err)
	}
	return nil
}

// ResolveName queries the DHT for a name and returns the verified NameRecord.
func (n *Node) ResolveName(ctx context.Context, name string) (*pb.NameRecord, error) {
	key := names.RoutingKey(name)

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	data, err := n.DHT.GetValue(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("DHT get name: %w", err)
	}

	rec, err := names.Unmarshal(data)
	if err != nil {
		return nil, fmt.Errorf("unmarshal name record: %w", err)
	}

	if err := names.VerifyNameRecord(rec); err != nil {
		return nil, fmt.Errorf("verify name record: %w", err)
	}

	return rec, nil
}

// Close shuts down the node.
func (n *Node) Close() error {
	if err := n.DHT.Close(); err != nil {
		return err
	}
	return n.Host.Close()
}

// AddrInfo returns this node's peer address info for connecting.
func (n *Node) AddrInfo() peer.AddrInfo {
	return peer.AddrInfo{
		ID:    n.Host.ID(),
		Addrs: n.Host.Addrs(),
	}
}

// namesValidator validates records stored under the "/names/" DHT namespace.
type namesValidator struct{}

func (nv namesValidator) Validate(key string, value []byte) error {
	rec, err := names.Unmarshal(value)
	if err != nil {
		return fmt.Errorf("unmarshal name record: %w", err)
	}
	return names.VerifyNameRecord(rec)
}

func (nv namesValidator) Select(key string, values [][]byte) (int, error) {
	if len(values) == 0 {
		return 0, errors.New("no values")
	}
	// First-come-first-served: pick earliest valid timestamp.
	bestIdx := 0
	bestTS := uint64(0)
	for i, v := range values {
		rec, err := names.Unmarshal(v)
		if err != nil {
			continue
		}
		if err := names.VerifyNameRecord(rec); err != nil {
			continue
		}
		if bestTS == 0 || rec.Timestamp < bestTS {
			bestTS = rec.Timestamp
			bestIdx = i
		}
	}
	return bestIdx, nil
}

// Ensure interface compliance.
var _ record.Validator = dmailValidator{}
var _ record.Validator = namesValidator{}
