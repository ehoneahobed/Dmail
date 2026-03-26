package packet

import (
	"crypto/ed25519"
	"fmt"
	"time"

	"github.com/ehoneahobed/dmail/internal/pb"
	dmcrypto "github.com/ehoneahobed/dmail/pkg/crypto"
	"github.com/ehoneahobed/dmail/pkg/pow"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
)

const (
	ProtocolVersion = 1
	MaxPacketSize   = 256 * 1024 // 256 KB
	MaxFutureSkew   = 1 * time.Hour
	MaxPastSkew     = 24 * time.Hour
)

// BuildAndSend creates, encrypts, signs, and PoW-stamps a DmailPacket.
func Build(subject, body, replyToID string, senderKP *dmcrypto.KeyPair, recipientPub ed25519.PublicKey, difficulty uint32) (*pb.DmailPacket, error) {
	// 1. Build the plaintext payload.
	payload := &pb.Payload{
		MessageId: uuid.NewString(),
		Subject:   subject,
		Body:      body,
		ReplyToId: replyToID,
	}
	payloadBytes, err := proto.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	// 2. Encrypt the payload for the recipient.
	encrypted, err := dmcrypto.Encrypt(payloadBytes, recipientPub, senderKP.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("encrypt payload: %w", err)
	}

	// 3. Build the packet (without signature and PoW yet).
	pkt := &pb.DmailPacket{
		Version:          ProtocolVersion,
		RecipientPubkey:  recipientPub,
		SenderPubkey:     senderKP.PublicKey,
		Timestamp:        uint64(time.Now().Unix()),
		PowDifficulty:    difficulty,
		EncryptedPayload: encrypted,
	}

	// 4. Compute PoW over the signable content.
	signableData := signableBytes(pkt)
	pkt.PowNonce = pow.Compute(signableData, difficulty)

	// 5. Sign fields 1-7.
	dataToSign := signableBytesWithNonce(pkt)
	pkt.Signature = dmcrypto.Sign(dataToSign, senderKP.PrivateKey)

	return pkt, nil
}

// Verify validates a received DmailPacket: checks size, timestamp, PoW, and signature.
func Verify(pkt *pb.DmailPacket) error {
	// Check packet size.
	data, err := proto.Marshal(pkt)
	if err != nil {
		return fmt.Errorf("marshal for size check: %w", err)
	}
	if len(data) > MaxPacketSize {
		return fmt.Errorf("packet too large: %d bytes (max %d)", len(data), MaxPacketSize)
	}

	// Check timestamp.
	pktTime := time.Unix(int64(pkt.Timestamp), 0)
	now := time.Now()
	if pktTime.After(now.Add(MaxFutureSkew)) {
		return fmt.Errorf("packet timestamp too far in future")
	}
	if pktTime.Before(now.Add(-MaxPastSkew)) {
		return fmt.Errorf("packet timestamp too old")
	}

	// Verify PoW.
	signableData := signableBytes(pkt)
	if !pow.Verify(signableData, pkt.PowNonce, pkt.PowDifficulty) {
		return fmt.Errorf("invalid proof of work")
	}

	// Verify signature.
	dataToSign := signableBytesWithNonce(pkt)
	if !dmcrypto.VerifySignature(dataToSign, pkt.Signature, pkt.SenderPubkey) {
		return fmt.Errorf("invalid signature")
	}

	return nil
}

// Decrypt decrypts and unmarshals the payload from a verified packet.
func Decrypt(pkt *pb.DmailPacket, recipientPriv ed25519.PrivateKey) (*pb.Payload, error) {
	plaintext, err := dmcrypto.Decrypt(pkt.EncryptedPayload, pkt.SenderPubkey, recipientPriv)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	payload := &pb.Payload{}
	if err := proto.Unmarshal(plaintext, payload); err != nil {
		return nil, fmt.Errorf("unmarshal payload: %w", err)
	}
	return payload, nil
}

// DecryptRaw unmarshals raw bytes into a packet and decrypts the payload.
func DecryptRaw(data []byte, recipientPriv ed25519.PrivateKey) (*pb.Payload, error) {
	pkt := &pb.DmailPacket{}
	if err := proto.Unmarshal(data, pkt); err != nil {
		return nil, fmt.Errorf("unmarshal packet: %w", err)
	}
	return Decrypt(pkt, recipientPriv)
}

// UnmarshalAndVerify unmarshals raw bytes and verifies the packet.
func UnmarshalAndVerify(data []byte) (*pb.DmailPacket, error) {
	pkt := &pb.DmailPacket{}
	if err := proto.Unmarshal(data, pkt); err != nil {
		return nil, fmt.Errorf("unmarshal packet: %w", err)
	}
	if err := Verify(pkt); err != nil {
		return nil, err
	}
	return pkt, nil
}

// signableBytes returns the deterministic byte representation of fields 1-6 (no nonce, no sig).
func signableBytes(pkt *pb.DmailPacket) []byte {
	tmp := &pb.DmailPacket{
		Version:          pkt.Version,
		RecipientPubkey:  pkt.RecipientPubkey,
		SenderPubkey:     pkt.SenderPubkey,
		Timestamp:        pkt.Timestamp,
		PowDifficulty:    pkt.PowDifficulty,
		EncryptedPayload: pkt.EncryptedPayload,
	}
	data, _ := proto.Marshal(tmp)
	return data
}

// signableBytesWithNonce returns the deterministic byte representation of fields 1-7 (no sig).
func signableBytesWithNonce(pkt *pb.DmailPacket) []byte {
	tmp := &pb.DmailPacket{
		Version:          pkt.Version,
		RecipientPubkey:  pkt.RecipientPubkey,
		SenderPubkey:     pkt.SenderPubkey,
		Timestamp:        pkt.Timestamp,
		PowNonce:         pkt.PowNonce,
		PowDifficulty:    pkt.PowDifficulty,
		EncryptedPayload: pkt.EncryptedPayload,
	}
	data, _ := proto.Marshal(tmp)
	return data
}
