package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha512"
	"fmt"

	"filippo.io/edwards25519"
	"golang.org/x/crypto/nacl/box"
)

// Encrypt encrypts plaintext for the recipient using NaCl box (XSalsa20-Poly1305).
// It converts Ed25519 keys to Curve25519 for the X25519 key exchange.
func Encrypt(plaintext []byte, recipientPub ed25519.PublicKey, senderPriv ed25519.PrivateKey) ([]byte, error) {
	recipientCurve, err := ed25519PubToCurve25519(recipientPub)
	if err != nil {
		return nil, fmt.Errorf("convert recipient key: %w", err)
	}
	senderCurve := ed25519PrivToCurve25519(senderPriv)

	var nonce [24]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	// Prepend nonce to ciphertext so recipient can decrypt.
	encrypted := box.Seal(nonce[:], plaintext, &nonce, &recipientCurve, &senderCurve)
	return encrypted, nil
}

// Decrypt decrypts ciphertext from the sender using NaCl box.
func Decrypt(ciphertext []byte, senderPub ed25519.PublicKey, recipientPriv ed25519.PrivateKey) ([]byte, error) {
	if len(ciphertext) < 24+box.Overhead {
		return nil, fmt.Errorf("ciphertext too short")
	}
	senderCurve, err := ed25519PubToCurve25519(senderPub)
	if err != nil {
		return nil, fmt.Errorf("convert sender key: %w", err)
	}
	recipientCurve := ed25519PrivToCurve25519(recipientPriv)

	var nonce [24]byte
	copy(nonce[:], ciphertext[:24])

	plaintext, ok := box.Open(nil, ciphertext[24:], &nonce, &senderCurve, &recipientCurve)
	if !ok {
		return nil, fmt.Errorf("decryption failed: invalid ciphertext or wrong keys")
	}
	return plaintext, nil
}

// Sign signs data with an Ed25519 private key.
func Sign(data []byte, privKey ed25519.PrivateKey) []byte {
	return ed25519.Sign(privKey, data)
}

// VerifySignature verifies an Ed25519 signature.
func VerifySignature(data []byte, sig []byte, pubKey ed25519.PublicKey) bool {
	return ed25519.Verify(pubKey, data, sig)
}

// ed25519PubToCurve25519 converts an Ed25519 public key to a Curve25519 public key
// using the birational map: u = (1 + y) / (1 - y) mod p.
func ed25519PubToCurve25519(pub ed25519.PublicKey) ([32]byte, error) {
	var curvePub [32]byte
	if len(pub) != 32 {
		return curvePub, fmt.Errorf("invalid public key length: %d", len(pub))
	}

	// Decompress the Ed25519 public key as an Edwards point.
	p, err := new(edwards25519.Point).SetBytes(pub)
	if err != nil {
		return curvePub, fmt.Errorf("invalid Ed25519 public key: %w", err)
	}

	// Convert Edwards point to Montgomery u-coordinate.
	// The BytesMontgomery method returns the u-coordinate directly.
	mont := p.BytesMontgomery()
	copy(curvePub[:], mont)
	return curvePub, nil
}

// ed25519PrivToCurve25519 converts an Ed25519 private key to a Curve25519 private key.
func ed25519PrivToCurve25519(priv ed25519.PrivateKey) [32]byte {
	var curvePriv [32]byte
	h := sha512.Sum512(priv.Seed())
	// Clamp as per X25519 / RFC 7748
	h[0] &= 248
	h[31] &= 127
	h[31] |= 64
	copy(curvePriv[:], h[:32])
	return curvePriv
}
