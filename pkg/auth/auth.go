package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/argon2"
)

// Argon2id parameters.
const (
	argonTime    = 1
	argonMemory  = 64 * 1024
	argonThreads = 4
	argonKeyLen  = 32
	saltLen      = 16
)

// HashPassword returns an Argon2id hash and its salt.
func HashPassword(password string) (hash, salt []byte, err error) {
	salt = make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return nil, nil, fmt.Errorf("generate salt: %w", err)
	}
	hash = argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return hash, salt, nil
}

// VerifyPassword checks a password against an Argon2id hash and salt.
func VerifyPassword(password string, hash, salt []byte) bool {
	computed := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	if len(computed) != len(hash) {
		return false
	}
	// Constant-time comparison.
	var diff byte
	for i := range computed {
		diff |= computed[i] ^ hash[i]
	}
	return diff == 0
}

// EncryptPrivateKey encrypts a private key with AES-256-GCM using an Argon2-derived key.
func EncryptPrivateKey(privKey []byte, password string) (ciphertext, salt, nonce []byte, err error) {
	salt = make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return nil, nil, nil, fmt.Errorf("generate salt: %w", err)
	}

	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create GCM: %w", err)
	}

	nonce = make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, nil, fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext = gcm.Seal(nil, nonce, privKey, nil)
	return ciphertext, salt, nonce, nil
}

// DecryptPrivateKey decrypts a private key with AES-256-GCM using an Argon2-derived key.
func DecryptPrivateKey(ciphertext, salt, nonce []byte, password string) ([]byte, error) {
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	return plaintext, nil
}

// GenerateJWT creates a signed JWT token for a user.
func GenerateJWT(userID string, secret []byte, expiry time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"sub": userID,
		"exp": time.Now().Add(expiry).Unix(),
		"iat": time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

// ValidateJWT validates a JWT token and returns the user ID.
func ValidateJWT(tokenStr string, secret []byte) (string, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return secret, nil
	})
	if err != nil {
		return "", fmt.Errorf("parse token: %w", err)
	}
	if !token.Valid {
		return "", fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", fmt.Errorf("invalid claims")
	}

	sub, ok := claims["sub"].(string)
	if !ok {
		return "", fmt.Errorf("missing sub claim")
	}

	return sub, nil
}
