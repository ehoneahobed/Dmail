package crypto

import (
	"bytes"
	"testing"
)

func TestEncryptDecryptRoundtrip(t *testing.T) {
	alice, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	bob, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	plaintext := []byte("Hello Bob, this is a secret message from Alice!")
	ciphertext, err := Encrypt(plaintext, bob.PublicKey, alice.PrivateKey)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// Ciphertext should be different from plaintext
	if bytes.Equal(ciphertext, plaintext) {
		t.Error("ciphertext equals plaintext")
	}

	// Bob decrypts
	decrypted, err := Decrypt(ciphertext, alice.PublicKey, bob.PrivateKey)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("decrypted = %q, want %q", decrypted, plaintext)
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	alice, _ := GenerateKeyPair()
	bob, _ := GenerateKeyPair()
	eve, _ := GenerateKeyPair()

	plaintext := []byte("Secret message")
	ciphertext, err := Encrypt(plaintext, bob.PublicKey, alice.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}

	// Eve tries to decrypt with her key — should fail
	_, err = Decrypt(ciphertext, alice.PublicKey, eve.PrivateKey)
	if err == nil {
		t.Error("expected decryption to fail with wrong key")
	}
}

func TestSignAndVerify(t *testing.T) {
	kp, _ := GenerateKeyPair()
	data := []byte("important message to sign")

	sig := Sign(data, kp.PrivateKey)
	if !VerifySignature(data, sig, kp.PublicKey) {
		t.Error("valid signature was rejected")
	}

	// Tamper with data
	tampered := []byte("tampered message")
	if VerifySignature(tampered, sig, kp.PublicKey) {
		t.Error("tampered data signature was accepted")
	}
}
