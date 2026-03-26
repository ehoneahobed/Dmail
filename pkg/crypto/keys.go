package crypto

import (
	"crypto/ed25519"
	"crypto/sha256"
	"fmt"

	"github.com/mr-tron/base58"
	"github.com/tyler-smith/go-bip39"
)

const AddressPrefix = "dmail:"

// KeyPair holds an Ed25519 key pair and its mnemonic seed.
type KeyPair struct {
	PublicKey  ed25519.PublicKey
	PrivateKey ed25519.PrivateKey
	Mnemonic   string
}

// GenerateKeyPair creates a new Ed25519 keypair from a random BIP-39 mnemonic.
func GenerateKeyPair() (*KeyPair, error) {
	entropy, err := bip39.NewEntropy(128) // 12-word mnemonic
	if err != nil {
		return nil, fmt.Errorf("generate entropy: %w", err)
	}
	mnemonic, err := bip39.NewMnemonic(entropy)
	if err != nil {
		return nil, fmt.Errorf("generate mnemonic: %w", err)
	}
	return KeyPairFromMnemonic(mnemonic)
}

// KeyPairFromMnemonic deterministically derives an Ed25519 keypair from a BIP-39 mnemonic.
func KeyPairFromMnemonic(mnemonic string) (*KeyPair, error) {
	if !bip39.IsMnemonicValid(mnemonic) {
		return nil, fmt.Errorf("invalid mnemonic")
	}
	// Use mnemonic to seed with empty passphrase — 64 bytes.
	seed := bip39.NewSeed(mnemonic, "")
	// Ed25519 needs 32 bytes for the seed.
	privKey := ed25519.NewKeyFromSeed(seed[:32])
	pubKey := privKey.Public().(ed25519.PublicKey)
	return &KeyPair{
		PublicKey:  pubKey,
		PrivateKey: privKey,
		Mnemonic:   mnemonic,
	}, nil
}

// Address returns the Base58Check-encoded Dmail address for a public key.
// Format: "dmail:" + Base58(pubkey + checksum[0:4])
func Address(pubKey ed25519.PublicKey) string {
	checksum := sha256.Sum256(pubKey)
	payload := make([]byte, len(pubKey)+4)
	copy(payload, pubKey)
	copy(payload[len(pubKey):], checksum[:4])
	return AddressPrefix + base58.Encode(payload)
}

// PubKeyFromAddress decodes a Dmail address back to the raw 32-byte Ed25519 public key.
// Returns an error if the checksum is invalid.
func PubKeyFromAddress(addr string) (ed25519.PublicKey, error) {
	if len(addr) < len(AddressPrefix) || addr[:len(AddressPrefix)] != AddressPrefix {
		return nil, fmt.Errorf("address must start with %q", AddressPrefix)
	}
	decoded, err := base58.Decode(addr[len(AddressPrefix):])
	if err != nil {
		return nil, fmt.Errorf("base58 decode: %w", err)
	}
	if len(decoded) != 36 { // 32-byte key + 4-byte checksum
		return nil, fmt.Errorf("invalid address length: got %d bytes, want 36", len(decoded))
	}
	pubKey := decoded[:32]
	checksum := decoded[32:]
	expected := sha256.Sum256(pubKey)
	for i := 0; i < 4; i++ {
		if checksum[i] != expected[i] {
			return nil, fmt.Errorf("invalid address checksum")
		}
	}
	return ed25519.PublicKey(pubKey), nil
}
