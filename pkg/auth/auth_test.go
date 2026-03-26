package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"
)

func TestHashAndVerifyPassword(t *testing.T) {
	hash, salt, err := HashPassword("testpassword123")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword("testpassword123", hash, salt) {
		t.Error("correct password should verify")
	}
	if VerifyPassword("wrongpassword", hash, salt) {
		t.Error("wrong password should not verify")
	}
}

func TestEncryptDecryptPrivateKey(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	ciphertext, salt, nonce, err := EncryptPrivateKey([]byte(priv), "mypassword")
	if err != nil {
		t.Fatal(err)
	}

	decrypted, err := DecryptPrivateKey(ciphertext, salt, nonce, "mypassword")
	if err != nil {
		t.Fatal(err)
	}

	if string(decrypted) != string(priv) {
		t.Error("decrypted key should match original")
	}

	// Wrong password should fail.
	_, err = DecryptPrivateKey(ciphertext, salt, nonce, "wrongpassword")
	if err == nil {
		t.Error("wrong password should fail decryption")
	}
}

func TestJWT(t *testing.T) {
	secret := []byte("test-secret-key-12345678")

	token, err := GenerateJWT("user-123", secret, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	userID, err := ValidateJWT(token, secret)
	if err != nil {
		t.Fatal(err)
	}
	if userID != "user-123" {
		t.Errorf("got userID %q, want user-123", userID)
	}

	// Wrong secret should fail.
	_, err = ValidateJWT(token, []byte("wrong-secret"))
	if err == nil {
		t.Error("wrong secret should fail validation")
	}

	// Expired token should fail.
	expiredToken, err := GenerateJWT("user-123", secret, -1*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ValidateJWT(expiredToken, secret)
	if err == nil {
		t.Error("expired token should fail validation")
	}
}
