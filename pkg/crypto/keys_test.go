package crypto

import (
	"testing"
)

func TestGenerateKeyPair(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	if len(kp.PublicKey) != 32 {
		t.Errorf("public key length = %d, want 32", len(kp.PublicKey))
	}
	if len(kp.PrivateKey) != 64 {
		t.Errorf("private key length = %d, want 64", len(kp.PrivateKey))
	}
	if kp.Mnemonic == "" {
		t.Error("mnemonic is empty")
	}
}

func TestKeyPairFromMnemonic_Deterministic(t *testing.T) {
	kp1, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	kp2, err := KeyPairFromMnemonic(kp1.Mnemonic)
	if err != nil {
		t.Fatal(err)
	}
	if !kp1.PublicKey.Equal(kp2.PublicKey) {
		t.Error("same mnemonic produced different public keys")
	}
}

func TestAddressRoundtrip(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	addr := Address(kp.PublicKey)
	if addr[:len(AddressPrefix)] != AddressPrefix {
		t.Errorf("address doesn't start with %q: %s", AddressPrefix, addr)
	}
	recovered, err := PubKeyFromAddress(addr)
	if err != nil {
		t.Fatalf("PubKeyFromAddress: %v", err)
	}
	if !kp.PublicKey.Equal(recovered) {
		t.Error("roundtrip failed: recovered key != original")
	}
}

func TestPubKeyFromAddress_InvalidChecksum(t *testing.T) {
	kp, _ := GenerateKeyPair()
	addr := Address(kp.PublicKey)
	// Corrupt last character
	corrupted := addr[:len(addr)-1] + "X"
	_, err := PubKeyFromAddress(corrupted)
	if err == nil {
		t.Error("expected error for corrupted address, got nil")
	}
}
