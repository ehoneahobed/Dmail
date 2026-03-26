package pow

import (
	"testing"
)

func TestComputeAndVerify(t *testing.T) {
	data := []byte("test hashcash data")
	difficulty := uint32(16) // Use lower difficulty for fast test

	nonce := Compute(data, difficulty)
	if !Verify(data, nonce, difficulty) {
		t.Errorf("Verify returned false for valid nonce %d", nonce)
	}
}

func TestVerify_InvalidNonce(t *testing.T) {
	data := []byte("test hashcash data")
	difficulty := uint32(20)

	// Nonce 0 is almost certainly invalid for difficulty 20
	if Verify(data, 0, difficulty) && Verify(data, 1, difficulty) {
		t.Error("expected at least one of nonce 0 or 1 to be invalid")
	}
}

func TestVerify_ZeroDifficulty(t *testing.T) {
	data := []byte("anything")
	// Difficulty 0 means any nonce is valid
	if !Verify(data, 42, 0) {
		t.Error("difficulty 0 should accept any nonce")
	}
}
