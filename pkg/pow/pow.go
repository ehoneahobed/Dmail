package pow

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
)

const DefaultDifficulty = 20 // Number of leading zero bits required

// Compute finds a nonce such that SHA-256(data || nonce) has at least
// `difficulty` leading zero bits. Returns the nonce.
func Compute(data []byte, difficulty uint32) uint64 {
	for nonce := uint64(0); nonce < math.MaxUint64; nonce++ {
		if Verify(data, nonce, difficulty) {
			return nonce
		}
	}
	return 0 // unreachable in practice
}

// Verify checks that SHA-256(data || nonce) has at least `difficulty` leading zero bits.
func Verify(data []byte, nonce uint64, difficulty uint32) bool {
	buf := make([]byte, len(data)+8)
	copy(buf, data)
	binary.BigEndian.PutUint64(buf[len(data):], nonce)
	hash := sha256.Sum256(buf)
	return hasLeadingZeroBits(hash[:], difficulty)
}

// hasLeadingZeroBits returns true if the byte slice has at least n leading zero bits.
func hasLeadingZeroBits(hash []byte, n uint32) bool {
	fullBytes := n / 8
	remainBits := n % 8

	for i := uint32(0); i < fullBytes; i++ {
		if hash[i] != 0 {
			return false
		}
	}
	if remainBits > 0 {
		mask := byte(0xFF << (8 - remainBits))
		if hash[fullBytes]&mask != 0 {
			return false
		}
	}
	return true
}
