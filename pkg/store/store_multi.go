package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// User represents a registered user in multi-tenant mode.
type User struct {
	ID                string `json:"id"`
	Username          string `json:"username"`
	Email             string `json:"email,omitempty"`
	PasswordHash      []byte `json:"-"`
	PasswordSalt      []byte `json:"-"`
	Pubkey            string `json:"pubkey"`
	EncryptedPrivkey  []byte `json:"-"`
	EncryptedKeySalt  []byte `json:"-"`
	EncryptedKeyNonce []byte `json:"-"`
	CreatedAt         int64  `json:"created_at"`
}

// PendingPacket represents a raw packet awaiting decryption.
type PendingPacket struct {
	ID           int64  `json:"id"`
	UserID       string `json:"user_id"`
	RawData      []byte `json:"-"`
	SenderPubkey string `json:"sender_pubkey"`
	ReceivedAt   int64  `json:"received_at"`
	Processed    bool   `json:"processed"`
}

// MigrateMultiTenant creates the multi-tenant tables.
func (s *Store) MigrateMultiTenant() error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		username TEXT UNIQUE NOT NULL,
		email TEXT NOT NULL DEFAULT '',
		password_hash BLOB NOT NULL,
		password_salt BLOB NOT NULL,
		pubkey TEXT NOT NULL,
		encrypted_privkey BLOB NOT NULL,
		encrypted_key_salt BLOB NOT NULL,
		encrypted_key_nonce BLOB NOT NULL,
		created_at INTEGER NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
	CREATE INDEX IF NOT EXISTS idx_users_pubkey ON users(pubkey);

	CREATE TABLE IF NOT EXISTS pending_packets (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id TEXT NOT NULL,
		raw_data BLOB NOT NULL,
		sender_pubkey TEXT NOT NULL DEFAULT '',
		received_at INTEGER NOT NULL,
		processed INTEGER NOT NULL DEFAULT 0,
		FOREIGN KEY (user_id) REFERENCES users(id)
	);
	CREATE INDEX IF NOT EXISTS idx_pending_user ON pending_packets(user_id, processed);
	`
	_, err := s.db.Exec(schema)
	return err
}

// CreateUser inserts a new user.
func (s *Store) CreateUser(u *User) error {
	if u.CreatedAt == 0 {
		u.CreatedAt = time.Now().Unix()
	}
	_, err := s.db.Exec(
		`INSERT INTO users (id, username, email, password_hash, password_salt, pubkey, encrypted_privkey, encrypted_key_salt, encrypted_key_nonce, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		u.ID, u.Username, u.Email, u.PasswordHash, u.PasswordSalt,
		u.Pubkey, u.EncryptedPrivkey, u.EncryptedKeySalt, u.EncryptedKeyNonce,
		u.CreatedAt,
	)
	return err
}

// GetUserByUsername finds a user by username.
func (s *Store) GetUserByUsername(username string) (*User, error) {
	u := &User{}
	err := s.db.QueryRow(
		`SELECT id, username, email, password_hash, password_salt, pubkey, encrypted_privkey, encrypted_key_salt, encrypted_key_nonce, created_at
		 FROM users WHERE username = ?`, username,
	).Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.PasswordSalt,
		&u.Pubkey, &u.EncryptedPrivkey, &u.EncryptedKeySalt, &u.EncryptedKeyNonce,
		&u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

// GetUserByPubkey finds a user by their public key address.
func (s *Store) GetUserByPubkey(pubkey string) (*User, error) {
	u := &User{}
	err := s.db.QueryRow(
		`SELECT id, username, email, password_hash, password_salt, pubkey, encrypted_privkey, encrypted_key_salt, encrypted_key_nonce, created_at
		 FROM users WHERE pubkey = ?`, pubkey,
	).Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.PasswordSalt,
		&u.Pubkey, &u.EncryptedPrivkey, &u.EncryptedKeySalt, &u.EncryptedKeyNonce,
		&u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

// GetUserByID finds a user by ID.
func (s *Store) GetUserByID(id string) (*User, error) {
	u := &User{}
	err := s.db.QueryRow(
		`SELECT id, username, email, password_hash, password_salt, pubkey, encrypted_privkey, encrypted_key_salt, encrypted_key_nonce, created_at
		 FROM users WHERE id = ?`, id,
	).Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.PasswordSalt,
		&u.Pubkey, &u.EncryptedPrivkey, &u.EncryptedKeySalt, &u.EncryptedKeyNonce,
		&u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

// ListUsers returns all users.
func (s *Store) ListUsers() ([]User, error) {
	rows, err := s.db.Query(
		`SELECT id, username, email, pubkey, created_at FROM users ORDER BY username`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.Pubkey, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// GetAllUserPubKeys returns all registered user public keys.
func (s *Store) GetAllUserPubKeys() ([]string, error) {
	rows, err := s.db.Query(`SELECT pubkey FROM users`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var pk string
		if err := rows.Scan(&pk); err != nil {
			return nil, err
		}
		keys = append(keys, pk)
	}
	return keys, rows.Err()
}

// SavePendingPacket stores a raw packet for later decryption.
func (s *Store) SavePendingPacket(userID string, rawData []byte, senderPubkey string) error {
	_, err := s.db.Exec(
		`INSERT INTO pending_packets (user_id, raw_data, sender_pubkey, received_at, processed)
		 VALUES (?, ?, ?, ?, 0)`,
		userID, rawData, senderPubkey, time.Now().Unix(),
	)
	return err
}

// GetPendingPackets returns unprocessed packets for a user.
func (s *Store) GetPendingPackets(userID string) ([]PendingPacket, error) {
	rows, err := s.db.Query(
		`SELECT id, user_id, raw_data, sender_pubkey, received_at, processed
		 FROM pending_packets WHERE user_id = ? AND processed = 0 ORDER BY received_at`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var packets []PendingPacket
	for rows.Next() {
		var p PendingPacket
		if err := rows.Scan(&p.ID, &p.UserID, &p.RawData, &p.SenderPubkey, &p.ReceivedAt, &p.Processed); err != nil {
			return nil, err
		}
		packets = append(packets, p)
	}
	return packets, rows.Err()
}

// MarkPacketProcessed marks a pending packet as processed.
func (s *Store) MarkPacketProcessed(id int64) error {
	_, err := s.db.Exec(`UPDATE pending_packets SET processed = 1 WHERE id = ?`, id)
	return err
}

// SaveMessageForUser inserts a user-scoped message.
func (s *Store) SaveMessageForUser(userID string, m *Message) error {
	if m.ThreadID == "" && m.ReplyToID != "" {
		m.ThreadID = m.ReplyToID
	}
	if m.ThreadID == "" {
		m.ThreadID = m.ID
	}
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO messages (id, folder, sender_pubkey, recipient_pubkey, subject, body, timestamp, is_read, reply_to_id, thread_id, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		fmt.Sprintf("%s:%s", userID, m.ID), m.Folder, m.SenderPubkey, m.RecipientPubkey,
		m.Subject, m.Body, m.Timestamp, m.IsRead, m.ReplyToID, m.ThreadID, m.Status,
	)
	return err
}

// ListMessagesForUser returns messages for a specific user in a folder.
// The returned message IDs have the userID: prefix stripped so the frontend gets clean IDs.
func (s *Store) ListMessagesForUser(userID, folder string) ([]Message, error) {
	rows, err := s.db.Query(
		`SELECT id, folder, sender_pubkey, recipient_pubkey, subject, body, timestamp, is_read, reply_to_id, thread_id, status
		 FROM messages WHERE id LIKE ? AND folder = ? ORDER BY timestamp DESC`,
		userID+":%", folder,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	prefix := userID + ":"
	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.Folder, &m.SenderPubkey, &m.RecipientPubkey, &m.Subject, &m.Body, &m.Timestamp, &m.IsRead, &m.ReplyToID, &m.ThreadID, &m.Status); err != nil {
			return nil, err
		}
		// Strip the userID: prefix so the frontend gets clean IDs.
		if strings.HasPrefix(m.ID, prefix) {
			m.ID = m.ID[len(prefix):]
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

// CountUnreadForUser returns unread message count for a user in a folder.
func (s *Store) CountUnreadForUser(userID, folder string) (int, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM messages WHERE id LIKE ? AND folder = ? AND is_read = 0`,
		userID+":%", folder,
	).Scan(&count)
	return count, err
}

// SearchMessagesForUser performs full-text search scoped to a user.
func (s *Store) SearchMessagesForUser(userID, query string, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(
		`SELECT m.id, m.folder, m.sender_pubkey, m.recipient_pubkey, m.subject, m.body, m.timestamp, m.is_read, m.reply_to_id, m.thread_id, m.status
		 FROM messages m
		 JOIN messages_fts f ON m.id = f.id
		 WHERE messages_fts MATCH ? AND m.id LIKE ?
		 ORDER BY m.timestamp DESC LIMIT ?`,
		query, userID+":%", limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	prefix := userID + ":"
	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.Folder, &m.SenderPubkey, &m.RecipientPubkey, &m.Subject, &m.Body, &m.Timestamp, &m.IsRead, &m.ReplyToID, &m.ThreadID, &m.Status); err != nil {
			return nil, err
		}
		if strings.HasPrefix(m.ID, prefix) {
			m.ID = m.ID[len(prefix):]
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

// GetThreadForUser returns all messages in a thread for a user.
func (s *Store) GetThreadForUser(userID, messageID string) ([]Message, error) {
	scopedID := userID + ":" + messageID
	var threadID string
	err := s.db.QueryRow(`SELECT thread_id FROM messages WHERE id = ?`, scopedID).Scan(&threadID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if threadID == "" {
		threadID = messageID
	}

	rows, err := s.db.Query(
		`SELECT id, folder, sender_pubkey, recipient_pubkey, subject, body, timestamp, is_read, reply_to_id, thread_id, status
		 FROM messages WHERE id LIKE ? AND thread_id = ? ORDER BY timestamp ASC`,
		userID+":%", threadID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	prefix := userID + ":"
	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.Folder, &m.SenderPubkey, &m.RecipientPubkey, &m.Subject, &m.Body, &m.Timestamp, &m.IsRead, &m.ReplyToID, &m.ThreadID, &m.Status); err != nil {
			return nil, err
		}
		if strings.HasPrefix(m.ID, prefix) {
			m.ID = m.ID[len(prefix):]
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

// UpdateMessageStatusForUser updates the delivery status of a user-scoped message.
func (s *Store) UpdateMessageStatusForUser(userID, messageID, status string) error {
	scopedID := userID + ":" + messageID
	_, err := s.db.Exec(`UPDATE messages SET status = ? WHERE id = ?`, status, scopedID)
	return err
}
