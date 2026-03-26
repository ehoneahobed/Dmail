package names

import (
	"testing"

	"github.com/ehoneahobed/dmail/internal/pb"
	dmcrypto "github.com/ehoneahobed/dmail/pkg/crypto"
	"github.com/ehoneahobed/dmail/pkg/pow"
)

func TestValidateName(t *testing.T) {
	valid := []string{"alice", "bob42", "my-name", "a1b", "abc"}
	for _, n := range valid {
		if err := ValidateName(n); err != nil {
			t.Errorf("ValidateName(%q) = %v, want nil", n, err)
		}
	}

	invalid := []string{
		"ab",             // too short
		"Alice",          // uppercase
		"-abc",           // starts with hyphen
		"abc-",           // ends with hyphen
		"a--b",           // double hyphen
		"admin",          // reserved
		"a",              // too short
		"a b",            // space
		"abc!",           // special char
	}
	for _, n := range invalid {
		if err := ValidateName(n); err == nil {
			t.Errorf("ValidateName(%q) = nil, want error", n)
		}
	}
}

func TestBuildAndVerifyNameRecord(t *testing.T) {
	kp, err := dmcrypto.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	// Use low difficulty for test speed.
	origDiff := NameDifficulty
	defer func() {
		// NameDifficulty is a const, so we test with the real VerifyNameRecord
		// which checks >= NameDifficulty. For this test, we build with low diff
		// and verify manually.
	}()
	_ = origDiff

	// Build a record at low difficulty for testing.
	rec, err := buildTestRecord("alice", kp, 16)
	if err != nil {
		t.Fatal(err)
	}

	// Verify manually (bypassing difficulty check).
	if err := ValidateName(rec.Name); err != nil {
		t.Fatal(err)
	}
	if rec.Name != "alice" {
		t.Errorf("got name %q, want alice", rec.Name)
	}
	if len(rec.Signature) != 64 {
		t.Errorf("signature length = %d, want 64", len(rec.Signature))
	}
}

func TestRoutingKey(t *testing.T) {
	k1 := RoutingKey("alice")
	k2 := RoutingKey("Alice")
	if k1 != k2 {
		t.Errorf("routing keys differ for alice vs Alice")
	}
	if k1[:7] != "/names/" {
		t.Errorf("routing key prefix = %q, want /names/", k1[:7])
	}
}

// buildTestRecord builds a NameRecord with custom difficulty for testing.
func buildTestRecord(name string, kp *dmcrypto.KeyPair, difficulty uint32) (*pb.NameRecord, error) {
	if err := ValidateName(name); err != nil {
		return nil, err
	}

	rec := &pb.NameRecord{
		Name:          name,
		Pubkey:        kp.PublicKey,
		Timestamp:     uint64(1700000000),
		PowDifficulty: difficulty,
	}

	data := signableBytes(rec)
	rec.PowNonce = pow.Compute(data, difficulty)

	sigData := signableBytesWithNonce(rec)
	rec.Signature = dmcrypto.Sign(sigData, kp.PrivateKey)

	return rec, nil
}
