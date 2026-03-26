package node

import (
	"context"
	"testing"
	"time"

	dmcrypto "github.com/ehoneahobed/dmail/pkg/crypto"
	"github.com/ehoneahobed/dmail/pkg/packet"
	"github.com/libp2p/go-libp2p/core/peer"
)

// TestTwoNodePushPoll is the Phase 2 integration test.
// Node A pushes a message to the DHT. Node B polls and retrieves it.
func TestTwoNodePushPoll(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Generate keypairs for Alice and Bob.
	alice, err := dmcrypto.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	bob, err := dmcrypto.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	// Start Node A (Alice) on port 0 (random available port).
	nodeA, err := New(ctx, Config{
		ListenPort: 0,
		KeyPair:    alice,
	})
	if err != nil {
		t.Fatalf("create node A: %v", err)
	}
	defer nodeA.Close()

	t.Logf("Node A listening on %v", nodeA.Host.Addrs())

	// Start Node B (Bob) with Node A as bootstrap peer.
	nodeB, err := New(ctx, Config{
		ListenPort:     0,
		KeyPair:        bob,
		BootstrapPeers: []peer.AddrInfo{nodeA.AddrInfo()},
	})
	if err != nil {
		t.Fatalf("create node B: %v", err)
	}
	defer nodeB.Close()

	t.Logf("Node B listening on %v", nodeB.Host.Addrs())

	// Wait for DHT routing tables to populate.
	time.Sleep(2 * time.Second)

	// Alice builds and pushes a message for Bob.
	subject := "DHT Test Message"
	body := "Hello Bob, this message traveled through the DHT!"
	difficulty := uint32(16) // Low for testing

	pkt, err := packet.Build(subject, body, "", alice, bob.PublicKey, difficulty)
	if err != nil {
		t.Fatalf("build packet: %v", err)
	}

	if err := nodeA.Push(ctx, pkt); err != nil {
		t.Fatalf("push: %v", err)
	}
	t.Log("Alice pushed message to DHT")

	// Bob polls for the message.
	received, err := nodeB.Poll(ctx)
	if err != nil {
		t.Fatalf("poll: %v", err)
	}

	// Bob decrypts the payload.
	payload, err := packet.Decrypt(received, bob.PrivateKey)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}

	if payload.Subject != subject {
		t.Errorf("subject = %q, want %q", payload.Subject, subject)
	}
	if payload.Body != body {
		t.Errorf("body = %q, want %q", payload.Body, body)
	}

	t.Logf("Bob received and decrypted: subject=%q body=%q", payload.Subject, payload.Body)
}
