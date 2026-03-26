package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Store wraps a SQLite database for local Dmail storage.
type Store struct {
	db *sql.DB
}

// Message represents a stored message.
type Message struct {
	ID              string `json:"id"`
	Folder          string `json:"folder"`
	SenderPubkey    string `json:"sender"`
	RecipientPubkey string `json:"recipient"`
	Subject         string `json:"subject"`
	Body            string `json:"body"`
	Timestamp       int64  `json:"timestamp"`
	IsRead          bool   `json:"is_read"`
	ReplyToID       string `json:"reply_to_id,omitempty"`
	ThreadID        string `json:"thread_id,omitempty"`
	Status          string `json:"status,omitempty"`
}

// NameEntry represents a registered .dmail name.
type NameEntry struct {
	Name      string `json:"name"`
	Pubkey    string `json:"pubkey"`
	Timestamp int64  `json:"timestamp"`
	RawRecord []byte `json:"-"`
	IsMine    bool   `json:"is_mine"`
}

// Contact represents a petname mapping.
type Contact struct {
	Pubkey    string `json:"pubkey"`
	Petname   string `json:"petname"`
	CreatedAt int64  `json:"created_at"`
}

// Open creates or opens a SQLite database at the given path and runs migrations.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *Store) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS messages (
		id TEXT PRIMARY KEY,
		folder TEXT NOT NULL DEFAULT 'inbox',
		sender_pubkey TEXT NOT NULL,
		recipient_pubkey TEXT NOT NULL,
		subject TEXT NOT NULL,
		body TEXT NOT NULL,
		timestamp INTEGER NOT NULL,
		is_read INTEGER NOT NULL DEFAULT 0,
		reply_to_id TEXT NOT NULL DEFAULT '',
		thread_id TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT ''
	);
	CREATE INDEX IF NOT EXISTS idx_messages_folder ON messages(folder);
	CREATE INDEX IF NOT EXISTS idx_messages_timestamp ON messages(timestamp);
	CREATE INDEX IF NOT EXISTS idx_messages_thread ON messages(thread_id);

	CREATE TABLE IF NOT EXISTS contacts (
		pubkey TEXT PRIMARY KEY,
		petname TEXT NOT NULL,
		created_at INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS keypair (
		pubkey TEXT PRIMARY KEY,
		encrypted_privkey TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS names (
		name TEXT PRIMARY KEY,
		pubkey TEXT NOT NULL,
		timestamp INTEGER NOT NULL,
		raw_record BLOB NOT NULL,
		is_mine INTEGER NOT NULL DEFAULT 0
	);
	`
	if _, err := s.db.Exec(schema); err != nil {
		return err
	}

	// Add columns if they don't exist (for existing databases).
	alters := []string{
		"ALTER TABLE messages ADD COLUMN reply_to_id TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE messages ADD COLUMN thread_id TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE messages ADD COLUMN status TEXT NOT NULL DEFAULT ''",
	}
	for _, q := range alters {
		s.db.Exec(q) // ignore errors — column may already exist
	}

	// Create FTS5 virtual table for full-text search.
	fts := `
	CREATE VIRTUAL TABLE IF NOT EXISTS messages_fts USING fts5(
		id UNINDEXED, subject, body, content='messages', content_rowid='rowid'
	);

	CREATE TRIGGER IF NOT EXISTS messages_ai AFTER INSERT ON messages BEGIN
		INSERT INTO messages_fts(rowid, id, subject, body) VALUES (new.rowid, new.id, new.subject, new.body);
	END;

	CREATE TRIGGER IF NOT EXISTS messages_ad AFTER DELETE ON messages BEGIN
		INSERT INTO messages_fts(messages_fts, rowid, id, subject, body) VALUES('delete', old.rowid, old.id, old.subject, old.body);
	END;

	CREATE TRIGGER IF NOT EXISTS messages_au AFTER UPDATE ON messages BEGIN
		INSERT INTO messages_fts(messages_fts, rowid, id, subject, body) VALUES('delete', old.rowid, old.id, old.subject, old.body);
		INSERT INTO messages_fts(rowid, id, subject, body) VALUES (new.rowid, new.id, new.subject, new.body);
	END;
	`
	if _, err := s.db.Exec(fts); err != nil {
		return err
	}

	// Rebuild FTS index from existing data (idempotent).
	s.db.Exec("INSERT INTO messages_fts(messages_fts) VALUES('rebuild')")

	return nil
}

// SaveMessage inserts or replaces a message.
func (s *Store) SaveMessage(m *Message) error {
	if m.ThreadID == "" && m.ReplyToID != "" {
		m.ThreadID = m.ReplyToID
	}
	if m.ThreadID == "" {
		m.ThreadID = m.ID
	}
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO messages (id, folder, sender_pubkey, recipient_pubkey, subject, body, timestamp, is_read, reply_to_id, thread_id, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.Folder, m.SenderPubkey, m.RecipientPubkey, m.Subject, m.Body, m.Timestamp, m.IsRead, m.ReplyToID, m.ThreadID, m.Status,
	)
	return err
}

// GetMessage returns a single message by ID.
func (s *Store) GetMessage(id string) (*Message, error) {
	m := &Message{}
	err := s.db.QueryRow(
		`SELECT id, folder, sender_pubkey, recipient_pubkey, subject, body, timestamp, is_read, reply_to_id, thread_id, status
		 FROM messages WHERE id = ?`, id,
	).Scan(&m.ID, &m.Folder, &m.SenderPubkey, &m.RecipientPubkey, &m.Subject, &m.Body, &m.Timestamp, &m.IsRead, &m.ReplyToID, &m.ThreadID, &m.Status)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return m, err
}

// ListMessages returns messages in the given folder, ordered by timestamp desc.
func (s *Store) ListMessages(folder string) ([]Message, error) {
	rows, err := s.db.Query(
		`SELECT id, folder, sender_pubkey, recipient_pubkey, subject, body, timestamp, is_read, reply_to_id, thread_id, status
		 FROM messages WHERE folder = ? ORDER BY timestamp DESC`, folder,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.Folder, &m.SenderPubkey, &m.RecipientPubkey, &m.Subject, &m.Body, &m.Timestamp, &m.IsRead, &m.ReplyToID, &m.ThreadID, &m.Status); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

// MarkRead marks a message as read.
func (s *Store) MarkRead(id string) error {
	_, err := s.db.Exec(`UPDATE messages SET is_read = 1 WHERE id = ?`, id)
	return err
}

// DeleteMessage moves a message to trash or permanently deletes it.
func (s *Store) DeleteMessage(id string) error {
	_, err := s.db.Exec(`UPDATE messages SET folder = 'trash' WHERE id = ?`, id)
	return err
}

// HasMessage checks if a message ID already exists.
func (s *Store) HasMessage(id string) (bool, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM messages WHERE id = ?`, id).Scan(&count)
	return count > 0, err
}

// SaveContact saves or updates a contact petname.
func (s *Store) SaveContact(c *Contact) error {
	if c.CreatedAt == 0 {
		c.CreatedAt = time.Now().Unix()
	}
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO contacts (pubkey, petname, created_at) VALUES (?, ?, ?)`,
		c.Pubkey, c.Petname, c.CreatedAt,
	)
	return err
}

// GetContact returns a contact by pubkey.
func (s *Store) GetContact(pubkey string) (*Contact, error) {
	c := &Contact{}
	err := s.db.QueryRow(`SELECT pubkey, petname, created_at FROM contacts WHERE pubkey = ?`, pubkey).
		Scan(&c.Pubkey, &c.Petname, &c.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return c, err
}

// ListContacts returns all contacts.
func (s *Store) ListContacts() ([]Contact, error) {
	rows, err := s.db.Query(`SELECT pubkey, petname, created_at FROM contacts ORDER BY petname`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var contacts []Contact
	for rows.Next() {
		var c Contact
		if err := rows.Scan(&c.Pubkey, &c.Petname, &c.CreatedAt); err != nil {
			return nil, err
		}
		contacts = append(contacts, c)
	}
	return contacts, rows.Err()
}

// DeleteContact removes a contact.
func (s *Store) DeleteContact(pubkey string) error {
	_, err := s.db.Exec(`DELETE FROM contacts WHERE pubkey = ?`, pubkey)
	return err
}

// SaveKeyPair stores the keypair.
func (s *Store) SaveKeyPair(pubkey, encryptedPrivkey string) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO keypair (pubkey, encrypted_privkey) VALUES (?, ?)`,
		pubkey, encryptedPrivkey,
	)
	return err
}

// GetKeyPair returns the stored keypair.
func (s *Store) GetKeyPair() (pubkey, encryptedPrivkey string, err error) {
	err = s.db.QueryRow(`SELECT pubkey, encrypted_privkey FROM keypair LIMIT 1`).
		Scan(&pubkey, &encryptedPrivkey)
	if err == sql.ErrNoRows {
		return "", "", nil
	}
	return
}

// SaveName saves or updates a name entry.
func (s *Store) SaveName(n *NameEntry) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO names (name, pubkey, timestamp, raw_record, is_mine) VALUES (?, ?, ?, ?, ?)`,
		n.Name, n.Pubkey, n.Timestamp, n.RawRecord, n.IsMine,
	)
	return err
}

// GetName returns a name entry by name.
func (s *Store) GetName(name string) (*NameEntry, error) {
	n := &NameEntry{}
	err := s.db.QueryRow(
		`SELECT name, pubkey, timestamp, raw_record, is_mine FROM names WHERE name = ?`, name,
	).Scan(&n.Name, &n.Pubkey, &n.Timestamp, &n.RawRecord, &n.IsMine)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return n, err
}

// GetNamesByPubkey returns all names registered to a pubkey.
func (s *Store) GetNamesByPubkey(pubkey string) ([]NameEntry, error) {
	rows, err := s.db.Query(
		`SELECT name, pubkey, timestamp, raw_record, is_mine FROM names WHERE pubkey = ? ORDER BY name`, pubkey,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []NameEntry
	for rows.Next() {
		var n NameEntry
		if err := rows.Scan(&n.Name, &n.Pubkey, &n.Timestamp, &n.RawRecord, &n.IsMine); err != nil {
			return nil, err
		}
		entries = append(entries, n)
	}
	return entries, rows.Err()
}

// GetMyNames returns all names owned by the local user.
func (s *Store) GetMyNames() ([]NameEntry, error) {
	rows, err := s.db.Query(
		`SELECT name, pubkey, timestamp, raw_record, is_mine FROM names WHERE is_mine = 1 ORDER BY name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []NameEntry
	for rows.Next() {
		var n NameEntry
		if err := rows.Scan(&n.Name, &n.Pubkey, &n.Timestamp, &n.RawRecord, &n.IsMine); err != nil {
			return nil, err
		}
		entries = append(entries, n)
	}
	return entries, rows.Err()
}

// CountUnread returns the number of unread messages in a folder.
func (s *Store) CountUnread(folder string) (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM messages WHERE folder = ? AND is_read = 0`, folder).Scan(&count)
	return count, err
}

// SearchMessages performs full-text search on message subjects and bodies.
func (s *Store) SearchMessages(query string, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(
		`SELECT m.id, m.folder, m.sender_pubkey, m.recipient_pubkey, m.subject, m.body, m.timestamp, m.is_read, m.reply_to_id, m.thread_id, m.status
		 FROM messages m
		 JOIN messages_fts f ON m.id = f.id
		 WHERE messages_fts MATCH ?
		 ORDER BY m.timestamp DESC LIMIT ?`,
		query, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.Folder, &m.SenderPubkey, &m.RecipientPubkey, &m.Subject, &m.Body, &m.Timestamp, &m.IsRead, &m.ReplyToID, &m.ThreadID, &m.Status); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

// GetThread returns all messages in a thread, ordered chronologically.
func (s *Store) GetThread(messageID string) ([]Message, error) {
	// First find the thread_id for this message.
	var threadID string
	err := s.db.QueryRow(`SELECT thread_id FROM messages WHERE id = ?`, messageID).Scan(&threadID)
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
		 FROM messages WHERE thread_id = ? ORDER BY timestamp ASC`,
		threadID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.Folder, &m.SenderPubkey, &m.RecipientPubkey, &m.Subject, &m.Body, &m.Timestamp, &m.IsRead, &m.ReplyToID, &m.ThreadID, &m.Status); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

// UpdateMessageStatus updates the delivery status of a message.
func (s *Store) UpdateMessageStatus(id, status string) error {
	_, err := s.db.Exec(`UPDATE messages SET status = ? WHERE id = ?`, status, id)
	return err
}

// Close closes the database.
func (s *Store) Close() error {
	return s.db.Close()
}
