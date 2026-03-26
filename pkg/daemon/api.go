package daemon

import (
	"encoding/json"
	"net/http"
	"strings"

	dmcrypto "github.com/ehoneahobed/dmail/pkg/crypto"
	"github.com/ehoneahobed/dmail/pkg/store"
)

// nameRequest is the JSON body for POST /api/v1/names/register.
type nameRequest struct {
	Name string `json:"name"`
}

// sendRequest is the JSON body for POST /api/v1/messages/send.
type sendRequest struct {
	Recipient string `json:"recipient"`
	Subject   string `json:"subject"`
	Body      string `json:"body"`
	ReplyToID string `json:"reply_to_id,omitempty"`
}

// contactRequest is the JSON body for POST /api/v1/contacts.
type contactRequest struct {
	Pubkey  string `json:"pubkey"`
	Petname string `json:"petname"`
}

// statusResponse is the JSON body for GET /api/v1/status.
type statusResponse struct {
	ConnectedPeers  int    `json:"connected_peers"`
	IsSyncing       bool   `json:"is_syncing"`
	PendingPoWTasks int    `json:"pending_pow_tasks"`
	Address         string `json:"address"`
}

// NewHTTPHandler returns an http.Handler wired to the daemon.
func (d *Daemon) NewHTTPHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/messages/unread-count", d.handleUnreadCount)
	mux.HandleFunc("GET /api/v1/messages/search", d.handleSearchMessages)
	mux.HandleFunc("GET /api/v1/messages/thread/{id}", d.handleGetThread)
	mux.HandleFunc("GET /api/v1/messages", d.handleListMessages)
	mux.HandleFunc("GET /api/v1/messages/{id}", d.handleGetMessage)
	mux.HandleFunc("POST /api/v1/messages/send", d.handleSendMessage)
	mux.HandleFunc("PUT /api/v1/messages/{id}/read", d.handleMarkRead)
	mux.HandleFunc("DELETE /api/v1/messages/{id}", d.handleDeleteMessage)

	mux.HandleFunc("GET /api/v1/contacts", d.handleListContacts)
	mux.HandleFunc("POST /api/v1/contacts", d.handleSaveContact)
	mux.HandleFunc("DELETE /api/v1/contacts/{pubkey}", d.handleDeleteContact)

	mux.HandleFunc("POST /api/v1/names/register", d.handleRegisterName)
	mux.HandleFunc("GET /api/v1/names/resolve/{name}", d.handleResolveName)
	mux.HandleFunc("GET /api/v1/names/mine", d.handleMyNames)

	mux.HandleFunc("GET /api/v1/status", d.handleStatus)
	mux.HandleFunc("GET /api/v1/identity", d.handleIdentity)

	// Wrap with CORS for the Electron frontend.
	return corsMiddleware(mux)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (d *Daemon) handleListMessages(w http.ResponseWriter, r *http.Request) {
	folder := r.URL.Query().Get("folder")
	if folder == "" {
		folder = "inbox"
	}
	msgs, err := d.Store.ListMessages(folder)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if msgs == nil {
		msgs = []store.Message{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"messages": msgs})
}

func (d *Daemon) handleGetMessage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	msg, err := d.Store.GetMessage(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if msg == nil {
		writeError(w, http.StatusNotFound, "message not found")
		return
	}
	writeJSON(w, http.StatusOK, msg)
}

func (d *Daemon) handleSendMessage(w http.ResponseWriter, r *http.Request) {
	var req sendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Recipient == "" || req.Subject == "" {
		writeError(w, http.StatusBadRequest, "recipient and subject are required")
		return
	}

	// Resolve .dmail names to addresses.
	recipient := req.Recipient
	if strings.HasSuffix(recipient, ".dmail") {
		addr, err := d.ResolveName(r.Context(), recipient)
		if err != nil {
			writeError(w, http.StatusBadRequest, "cannot resolve name: "+err.Error())
			return
		}
		recipient = addr
	}

	if err := d.SendMessage(r.Context(), recipient, req.Subject, req.Body, req.ReplyToID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

func (d *Daemon) handleMarkRead(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := d.Store.MarkRead(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (d *Daemon) handleDeleteMessage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := d.Store.DeleteMessage(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (d *Daemon) handleListContacts(w http.ResponseWriter, r *http.Request) {
	contacts, err := d.Store.ListContacts()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if contacts == nil {
		contacts = []store.Contact{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"contacts": contacts})
}

func (d *Daemon) handleSaveContact(w http.ResponseWriter, r *http.Request) {
	var req contactRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Pubkey == "" || req.Petname == "" {
		writeError(w, http.StatusBadRequest, "pubkey and petname are required")
		return
	}
	// Validate the address format.
	if !strings.HasPrefix(req.Pubkey, "dmail:") {
		writeError(w, http.StatusBadRequest, "pubkey must start with dmail:")
		return
	}
	c := &store.Contact{Pubkey: req.Pubkey, Petname: req.Petname}
	if err := d.Store.SaveContact(c); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

func (d *Daemon) handleDeleteContact(w http.ResponseWriter, r *http.Request) {
	pubkey := r.PathValue("pubkey")
	if err := d.Store.DeleteContact(pubkey); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (d *Daemon) handleStatus(w http.ResponseWriter, r *http.Request) {
	import_addr := dmcryptoAddr(d)
	resp := statusResponse{
		ConnectedPeers:  d.ConnectedPeers(),
		IsSyncing:       false,
		PendingPoWTasks: d.PendingPoWTasks(),
		Address:         import_addr,
	}
	writeJSON(w, http.StatusOK, resp)
}

func (d *Daemon) handleIdentity(w http.ResponseWriter, r *http.Request) {
	import_addr := dmcryptoAddr(d)
	writeJSON(w, http.StatusOK, map[string]string{
		"address":  import_addr,
		"mnemonic": d.KeyPair.Mnemonic,
	})
}

func (d *Daemon) handleRegisterName(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	if err := d.RegisterName(r.Context(), req.Name); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{
		"status": "accepted",
		"name":   req.Name + ".dmail",
	})
}

func (d *Daemon) handleResolveName(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	name = strings.TrimSuffix(name, ".dmail")

	addr, err := d.ResolveName(r.Context(), name)
	if err != nil {
		writeError(w, http.StatusNotFound, "name not found: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"name":    name + ".dmail",
		"address": addr,
	})
}

func (d *Daemon) handleMyNames(w http.ResponseWriter, r *http.Request) {
	entries, err := d.Store.GetMyNames()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if entries == nil {
		entries = []store.NameEntry{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"names": entries})
}

func (d *Daemon) handleUnreadCount(w http.ResponseWriter, r *http.Request) {
	folder := r.URL.Query().Get("folder")
	if folder == "" {
		folder = "inbox"
	}
	count, err := d.Store.CountUnread(folder)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"count": count})
}

func (d *Daemon) handleSearchMessages(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeJSON(w, http.StatusOK, map[string]any{"messages": []store.Message{}})
		return
	}
	msgs, err := d.Store.SearchMessages(q, 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if msgs == nil {
		msgs = []store.Message{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"messages": msgs})
}

func (d *Daemon) handleGetThread(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	msgs, err := d.Store.GetThread(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if msgs == nil {
		msgs = []store.Message{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"messages": msgs})
}

func dmcryptoAddr(d *Daemon) string {
	return dmcrypto.Address(d.KeyPair.PublicKey)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	if status != 0 {
		w.WriteHeader(status)
	}
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
