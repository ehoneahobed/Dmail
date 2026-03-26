package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	dmcrypto "github.com/ehoneahobed/dmail/pkg/crypto"
	"github.com/ehoneahobed/dmail/pkg/store"
)

// newTestDaemon creates a daemon with an ephemeral database and random P2P port.
func newTestDaemon(t *testing.T) *Daemon {
	t.Helper()
	ctx := context.Background()
	kp, err := dmcrypto.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	d, err := New(ctx, Config{
		ListenPort:   0,
		DataDir:      filepath.Join(dir, "test.db"),
		KeyPair:      kp,
		PollInterval: 24 * time.Hour, // don't auto-poll during tests
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestAPIStatus(t *testing.T) {
	d := newTestDaemon(t)
	handler := d.NewHTTPHandler()

	req := httptest.NewRequest("GET", "/api/v1/status", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200", w.Code)
	}

	var resp statusResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Address == "" {
		t.Error("address is empty")
	}
	t.Logf("status: %+v", resp)
}

func TestAPIIdentity(t *testing.T) {
	d := newTestDaemon(t)
	handler := d.NewHTTPHandler()

	req := httptest.NewRequest("GET", "/api/v1/identity", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status code = %d", w.Code)
	}
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["address"] == "" || resp["mnemonic"] == "" {
		t.Errorf("identity response incomplete: %v", resp)
	}
}

func TestAPIMessagesRoundtrip(t *testing.T) {
	d := newTestDaemon(t)
	handler := d.NewHTTPHandler()

	// Manually insert a message into the store (simulating a received message).
	msg := &store.Message{
		ID:              "test-msg-001",
		Folder:          "inbox",
		SenderPubkey:    "dmail:sender123",
		RecipientPubkey: dmcrypto.Address(d.KeyPair.PublicKey),
		Subject:         "Test Subject",
		Body:            "Test Body",
		Timestamp:       time.Now().Unix(),
		IsRead:          false,
	}
	if err := d.Store.SaveMessage(msg); err != nil {
		t.Fatal(err)
	}

	// GET /api/v1/messages?folder=inbox
	req := httptest.NewRequest("GET", "/api/v1/messages?folder=inbox", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("list status = %d", w.Code)
	}
	var listResp struct {
		Messages []store.Message `json:"messages"`
	}
	json.NewDecoder(w.Body).Decode(&listResp)
	if len(listResp.Messages) != 1 {
		t.Fatalf("got %d messages, want 1", len(listResp.Messages))
	}
	if listResp.Messages[0].Subject != "Test Subject" {
		t.Errorf("subject = %q", listResp.Messages[0].Subject)
	}

	// GET /api/v1/messages/{id}
	req = httptest.NewRequest("GET", "/api/v1/messages/test-msg-001", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("get status = %d", w.Code)
	}

	// PUT /api/v1/messages/{id}/read
	req = httptest.NewRequest("PUT", "/api/v1/messages/test-msg-001/read", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("mark read status = %d", w.Code)
	}

	// Verify it's marked read.
	got, _ := d.Store.GetMessage("test-msg-001")
	if !got.IsRead {
		t.Error("message not marked as read")
	}

	// DELETE /api/v1/messages/{id}
	req = httptest.NewRequest("DELETE", "/api/v1/messages/test-msg-001", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d", w.Code)
	}

	// Verify it moved to trash.
	got, _ = d.Store.GetMessage("test-msg-001")
	if got.Folder != "trash" {
		t.Errorf("folder = %q, want trash", got.Folder)
	}
}

func TestAPIContacts(t *testing.T) {
	d := newTestDaemon(t)
	handler := d.NewHTTPHandler()

	// POST /api/v1/contacts
	body, _ := json.Marshal(contactRequest{
		Pubkey:  "dmail:abc123xyz",
		Petname: "Alice",
	})
	req := httptest.NewRequest("POST", "/api/v1/contacts", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("create contact status = %d, body = %s", w.Code, w.Body.String())
	}

	// GET /api/v1/contacts
	req = httptest.NewRequest("GET", "/api/v1/contacts", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("list contacts status = %d", w.Code)
	}
	var resp struct {
		Contacts []store.Contact `json:"contacts"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Contacts) != 1 || resp.Contacts[0].Petname != "Alice" {
		t.Errorf("contacts = %+v", resp.Contacts)
	}
}

func TestAPISendValidation(t *testing.T) {
	d := newTestDaemon(t)
	handler := d.NewHTTPHandler()

	// Missing recipient.
	body, _ := json.Marshal(sendRequest{Subject: "hi"})
	req := httptest.NewRequest("POST", "/api/v1/messages/send", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestAPIEmptyInbox(t *testing.T) {
	d := newTestDaemon(t)
	handler := d.NewHTTPHandler()

	req := httptest.NewRequest("GET", "/api/v1/messages?folder=inbox", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}

	// Should return empty array, not null.
	var resp struct {
		Messages []store.Message `json:"messages"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Messages == nil {
		t.Error("messages should be empty array, not null")
	}
}
