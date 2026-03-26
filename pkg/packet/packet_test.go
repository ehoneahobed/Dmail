package packet

import (
	"testing"

	dmcrypto "github.com/ehoneahobed/dmail/pkg/crypto"
	"github.com/ehoneahobed/dmail/pkg/pow"
)

// TestAliceBobRoundtrip is the Phase 1 integration test.
// Alice generates keys, builds a message for Bob (encrypt + PoW + sign),
// Bob verifies and decrypts it.
func TestAliceBobRoundtrip(t *testing.T) {
	// 1. Generate keypairs for Alice and Bob.
	alice, err := dmcrypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate Alice keypair: %v", err)
	}
	bob, err := dmcrypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate Bob keypair: %v", err)
	}

	t.Logf("Alice address: %s", dmcrypto.Address(alice.PublicKey))
	t.Logf("Bob address:   %s", dmcrypto.Address(bob.PublicKey))

	// 2. Alice builds a message for Bob (low difficulty for fast test).
	subject := "Hello Bob!"
	body := "This is a secret message sent via Dmail."
	difficulty := uint32(16) // Low difficulty for testing

	pkt, err := Build(subject, body, "", alice, bob.PublicKey, difficulty)
	if err != nil {
		t.Fatalf("Build packet: %v", err)
	}

	// 3. Verify the packet passes all checks.
	if err := Verify(pkt); err != nil {
		t.Fatalf("Verify packet: %v", err)
	}

	// 4. Bob decrypts the payload.
	payload, err := Decrypt(pkt, bob.PrivateKey)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	// 5. Validate the decrypted content.
	if payload.Subject != subject {
		t.Errorf("subject = %q, want %q", payload.Subject, subject)
	}
	if payload.Body != body {
		t.Errorf("body = %q, want %q", payload.Body, body)
	}
	if payload.MessageId == "" {
		t.Error("message_id is empty")
	}

	t.Logf("Message ID: %s", payload.MessageId)
	t.Logf("Decrypted subject: %s", payload.Subject)
	t.Logf("Decrypted body: %s", payload.Body)
}

// TestInvalidPoW verifies that a packet with an invalid PoW nonce is rejected.
func TestInvalidPoW(t *testing.T) {
	alice, _ := dmcrypto.GenerateKeyPair()
	bob, _ := dmcrypto.GenerateKeyPair()

	pkt, err := Build("test", "body", "", alice, bob.PublicKey, 16)
	if err != nil {
		t.Fatal(err)
	}

	// Tamper with the nonce.
	pkt.PowNonce = pkt.PowNonce + 1
	if err := Verify(pkt); err == nil {
		t.Error("expected verification to fail with tampered PoW nonce")
	}
}

// TestInvalidSignature verifies that a packet with a forged signature is rejected.
func TestInvalidSignature(t *testing.T) {
	alice, _ := dmcrypto.GenerateKeyPair()
	bob, _ := dmcrypto.GenerateKeyPair()
	eve, _ := dmcrypto.GenerateKeyPair()

	pkt, err := Build("test", "body", "", alice, bob.PublicKey, 16)
	if err != nil {
		t.Fatal(err)
	}

	// Eve re-signs the packet — signature won't match Alice's pubkey.
	pkt.Signature = dmcrypto.Sign([]byte("fake"), eve.PrivateKey)
	if err := Verify(pkt); err == nil {
		t.Error("expected verification to fail with forged signature")
	}
}

// TestImpersonation verifies that spoofing sender_pubkey is detected.
func TestImpersonation(t *testing.T) {
	alice, _ := dmcrypto.GenerateKeyPair()
	bob, _ := dmcrypto.GenerateKeyPair()
	eve, _ := dmcrypto.GenerateKeyPair()

	// Eve builds a packet but puts Alice's pubkey as sender.
	pkt, err := Build("from Alice (not really)", "trust me", "", eve, bob.PublicKey, 16)
	if err != nil {
		t.Fatal(err)
	}

	// Replace sender pubkey with Alice's — signature will be Eve's.
	pkt.SenderPubkey = alice.PublicKey
	if err := Verify(pkt); err == nil {
		t.Error("expected verification to fail for impersonated sender")
	}
}

// TestPoWDifficulty verifies that the PoW actually meets the specified difficulty.
func TestPoWDifficulty(t *testing.T) {
	data := []byte("test pow difficulty verification")
	for _, diff := range []uint32{8, 12, 16} {
		nonce := pow.Compute(data, diff)
		if !pow.Verify(data, nonce, diff) {
			t.Errorf("PoW verification failed for difficulty %d", diff)
		}
	}
}
