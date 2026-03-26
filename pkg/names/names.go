package names

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/ehoneahobed/dmail/internal/pb"
	dmcrypto "github.com/ehoneahobed/dmail/pkg/crypto"
	"github.com/ehoneahobed/dmail/pkg/pow"
	"google.golang.org/protobuf/proto"
)

// NameDifficulty is the PoW difficulty for name registration (~10 min).
const NameDifficulty = 28

var nameRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

var reservedNames = map[string]bool{
	"admin": true, "root": true, "system": true, "dmail": true,
	"postmaster": true, "abuse": true, "support": true, "help": true,
	"null": true, "nobody": true, "daemon": true, "mailer-daemon": true,
}

// ValidateName checks that a name is well-formed.
func ValidateName(name string) error {
	if len(name) < 3 || len(name) > 32 {
		return fmt.Errorf("name must be 3-32 characters, got %d", len(name))
	}
	if name != strings.ToLower(name) {
		return fmt.Errorf("name must be lowercase")
	}
	if !nameRegex.MatchString(name) {
		return fmt.Errorf("name must match [a-z0-9][a-z0-9-]*[a-z0-9]")
	}
	if strings.Contains(name, "--") {
		return fmt.Errorf("name must not contain consecutive hyphens")
	}
	if reservedNames[name] {
		return fmt.Errorf("name %q is reserved", name)
	}
	return nil
}

// BuildNameRecord computes PoW and signs a name registration record.
func BuildNameRecord(name string, kp *dmcrypto.KeyPair) (*pb.NameRecord, error) {
	if err := ValidateName(name); err != nil {
		return nil, err
	}

	rec := &pb.NameRecord{
		Name:           name,
		Pubkey:         kp.PublicKey,
		Timestamp:      uint64(time.Now().Unix()),
		PowDifficulty:  NameDifficulty,
	}

	// Compute PoW over signable bytes (fields 1-3, 5).
	data := signableBytes(rec)
	rec.PowNonce = pow.Compute(data, NameDifficulty)

	// Sign fields 1-5.
	sigData := signableBytesWithNonce(rec)
	rec.Signature = dmcrypto.Sign(sigData, kp.PrivateKey)

	return rec, nil
}

// VerifyNameRecord checks PoW, signature, and name format.
func VerifyNameRecord(rec *pb.NameRecord) error {
	if err := ValidateName(rec.Name); err != nil {
		return err
	}
	if len(rec.Pubkey) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid pubkey length: %d", len(rec.Pubkey))
	}
	if rec.PowDifficulty < NameDifficulty {
		return fmt.Errorf("insufficient PoW difficulty: %d < %d", rec.PowDifficulty, NameDifficulty)
	}

	// Verify PoW.
	data := signableBytes(rec)
	if !pow.Verify(data, rec.PowNonce, rec.PowDifficulty) {
		return fmt.Errorf("invalid PoW")
	}

	// Verify signature.
	sigData := signableBytesWithNonce(rec)
	if !dmcrypto.VerifySignature(sigData, rec.Signature, rec.Pubkey) {
		return fmt.Errorf("invalid signature")
	}

	return nil
}

// RoutingKey returns the DHT key for a name: "/names/<sha256hex(lowercase(name))>".
func RoutingKey(name string) string {
	hash := sha256.Sum256([]byte(strings.ToLower(name)))
	return "/names/" + fmt.Sprintf("%x", hash)
}

// signableBytes serializes fields 1-3 and 5 (before PoW computation).
func signableBytes(rec *pb.NameRecord) []byte {
	nameBytes := []byte(rec.Name)
	buf := make([]byte, 0, len(nameBytes)+32+8+4)
	buf = append(buf, nameBytes...)
	buf = append(buf, rec.Pubkey...)
	ts := make([]byte, 8)
	binary.BigEndian.PutUint64(ts, rec.Timestamp)
	buf = append(buf, ts...)
	diff := make([]byte, 4)
	binary.BigEndian.PutUint32(diff, rec.PowDifficulty)
	buf = append(buf, diff...)
	return buf
}

// signableBytesWithNonce serializes fields 1-5 (for signing).
func signableBytesWithNonce(rec *pb.NameRecord) []byte {
	base := signableBytes(rec)
	nonce := make([]byte, 8)
	binary.BigEndian.PutUint64(nonce, rec.PowNonce)
	return append(base, nonce...)
}

// Marshal serializes a NameRecord to protobuf bytes.
func Marshal(rec *pb.NameRecord) ([]byte, error) {
	return proto.Marshal(rec)
}

// Unmarshal deserializes protobuf bytes to a NameRecord.
func Unmarshal(data []byte) (*pb.NameRecord, error) {
	rec := &pb.NameRecord{}
	if err := proto.Unmarshal(data, rec); err != nil {
		return nil, err
	}
	return rec, nil
}
