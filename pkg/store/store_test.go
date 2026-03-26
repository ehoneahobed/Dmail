package store

import (
	"os"
	"path/filepath"
	"testing"
)

func tempStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestMessageCRUD(t *testing.T) {
	s := tempStore(t)

	msg := &Message{
		ID:              "msg-001",
		Folder:          "inbox",
		SenderPubkey:    "dmail:alice123",
		RecipientPubkey: "dmail:bob456",
		Subject:         "Hello",
		Body:            "World",
		Timestamp:       1700000000,
		IsRead:          false,
	}

	// Save
	if err := s.SaveMessage(msg); err != nil {
		t.Fatal(err)
	}

	// Get
	got, err := s.GetMessage("msg-001")
	if err != nil {
		t.Fatal(err)
	}
	if got.Subject != "Hello" || got.Body != "World" {
		t.Errorf("got %+v", got)
	}

	// List
	msgs, err := s.ListMessages("inbox")
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("len = %d, want 1", len(msgs))
	}

	// Mark read
	if err := s.MarkRead("msg-001"); err != nil {
		t.Fatal(err)
	}
	got, _ = s.GetMessage("msg-001")
	if !got.IsRead {
		t.Error("expected is_read=true")
	}

	// Has message
	has, _ := s.HasMessage("msg-001")
	if !has {
		t.Error("expected HasMessage=true")
	}
	has, _ = s.HasMessage("nonexistent")
	if has {
		t.Error("expected HasMessage=false")
	}

	// Delete (move to trash)
	if err := s.DeleteMessage("msg-001"); err != nil {
		t.Fatal(err)
	}
	msgs, _ = s.ListMessages("inbox")
	if len(msgs) != 0 {
		t.Error("expected inbox empty after delete")
	}
	msgs, _ = s.ListMessages("trash")
	if len(msgs) != 1 {
		t.Error("expected message in trash")
	}
}

func TestContactCRUD(t *testing.T) {
	s := tempStore(t)

	c := &Contact{Pubkey: "dmail:charlie789", Petname: "Charlie"}
	if err := s.SaveContact(c); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetContact("dmail:charlie789")
	if err != nil {
		t.Fatal(err)
	}
	if got.Petname != "Charlie" {
		t.Errorf("petname = %q, want Charlie", got.Petname)
	}

	contacts, err := s.ListContacts()
	if err != nil {
		t.Fatal(err)
	}
	if len(contacts) != 1 {
		t.Fatalf("len = %d, want 1", len(contacts))
	}

	if err := s.DeleteContact("dmail:charlie789"); err != nil {
		t.Fatal(err)
	}
	contacts, _ = s.ListContacts()
	if len(contacts) != 0 {
		t.Error("expected 0 contacts after delete")
	}
}

func TestKeyPair(t *testing.T) {
	s := tempStore(t)

	if err := s.SaveKeyPair("pubkey123", "encpriv456"); err != nil {
		t.Fatal(err)
	}
	pub, priv, err := s.GetKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	if pub != "pubkey123" || priv != "encpriv456" {
		t.Errorf("got pub=%q priv=%q", pub, priv)
	}
}

func TestOpenNonexistentDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "test.db")
	// Parent dir doesn't exist — should fail
	_, err := Open(path)
	if err == nil {
		os.Remove(path)
		t.Error("expected error for nonexistent parent dir")
	}
}
