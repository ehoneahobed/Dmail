package daemon

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ehoneahobed/dmail/pkg/auth"
	dmcrypto "github.com/ehoneahobed/dmail/pkg/crypto"
	"github.com/ehoneahobed/dmail/pkg/names"
	"github.com/ehoneahobed/dmail/pkg/store"
	"github.com/google/uuid"
)

type contextKey string

const userIDKey contextKey = "userID"

const jwtExpiry = 24 * time.Hour

// signupRequest is the JSON body for POST /api/v1/auth/signup.
type signupRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
}

// loginRequest is the JSON body for POST /api/v1/auth/login.
type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// NewMultiTenantHTTPHandler returns an http.Handler for multi-tenant mode.
func (d *MultiTenantDaemon) NewMultiTenantHTTPHandler(staticDir string) http.Handler {
	mux := http.NewServeMux()

	// Public auth endpoints (no JWT required).
	mux.HandleFunc("POST /api/v1/auth/signup", d.handleSignup)
	mux.HandleFunc("POST /api/v1/auth/login", d.handleLogin)

	// Status is public (used for health checks and auth detection).
	mux.HandleFunc("GET /api/v1/status", d.handleMTStatus)

	// Protected endpoints.
	mux.HandleFunc("GET /api/v1/messages/unread-count", d.requireAuth(d.handleMTUnreadCount))
	mux.HandleFunc("GET /api/v1/messages/search", d.requireAuth(d.handleMTSearchMessages))
	mux.HandleFunc("GET /api/v1/messages/thread/{id}", d.requireAuth(d.handleMTGetThread))
	mux.HandleFunc("GET /api/v1/messages", d.requireAuth(d.handleMTListMessages))
	mux.HandleFunc("GET /api/v1/messages/{id}", d.requireAuth(d.handleMTGetMessage))
	mux.HandleFunc("POST /api/v1/messages/send", d.requireAuth(d.handleMTSendMessage))
	mux.HandleFunc("PUT /api/v1/messages/{id}/read", d.requireAuth(d.handleMTMarkRead))
	mux.HandleFunc("DELETE /api/v1/messages/{id}", d.requireAuth(d.handleMTDeleteMessage))

	// User directory.
	mux.HandleFunc("GET /api/v1/users", d.requireAuth(d.handleMTListUsers))

	mux.HandleFunc("GET /api/v1/contacts", d.requireAuth(d.handleMTListContacts))
	mux.HandleFunc("POST /api/v1/contacts", d.requireAuth(d.handleMTSaveContact))
	mux.HandleFunc("DELETE /api/v1/contacts/{pubkey}", d.requireAuth(d.handleMTDeleteContact))

	mux.HandleFunc("POST /api/v1/names/register", d.requireAuth(d.handleMTRegisterName))
	mux.HandleFunc("GET /api/v1/names/resolve/{name}", d.handleMTResolveName) // public
	mux.HandleFunc("GET /api/v1/names/mine", d.requireAuth(d.handleMTMyNames))

	mux.HandleFunc("GET /api/v1/identity", d.requireAuth(d.handleMTIdentity))

	// Serve static frontend files if configured.
	if staticDir != "" {
		mux.HandleFunc("/", spaHandler(staticDir))
	}

	return corsMiddleware(mux)
}

// spaHandler serves static files with SPA fallback to index.html.
func spaHandler(dir string) http.HandlerFunc {
	fs := http.Dir(dir)
	fileServer := http.FileServer(fs)
	return func(w http.ResponseWriter, r *http.Request) {
		// Don't intercept API routes.
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		// Try to serve the file directly.
		path := filepath.Join(dir, r.URL.Path)
		if _, err := os.Stat(path); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}
		// SPA fallback: serve index.html for all other routes.
		http.ServeFile(w, r, filepath.Join(dir, "index.html"))
	}
}

// requireAuth is JWT auth middleware.
func (d *MultiTenantDaemon) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			writeError(w, http.StatusUnauthorized, "missing or invalid authorization header")
			return
		}
		token := strings.TrimPrefix(header, "Bearer ")
		userID, err := auth.ValidateJWT(token, d.JWTSecret)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		ctx := context.WithValue(r.Context(), userIDKey, userID)
		next(w, r.WithContext(ctx))
	}
}

func getUserID(r *http.Request) string {
	v, _ := r.Context().Value(userIDKey).(string)
	return v
}

// --- Auth handlers ---

func (d *MultiTenantDaemon) handleSignup(w http.ResponseWriter, r *http.Request) {
	var req signupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password are required")
		return
	}
	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	// Check if username already exists.
	existing, err := d.Store.GetUserByUsername(req.Username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if existing != nil {
		writeError(w, http.StatusConflict, "username already taken")
		return
	}

	// Generate keypair.
	kp, err := dmcrypto.GenerateKeyPair()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "generate keypair: "+err.Error())
		return
	}

	// Hash password.
	pwHash, pwSalt, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "hash password: "+err.Error())
		return
	}

	// Encrypt private key with password.
	encKey, encSalt, encNonce, err := auth.EncryptPrivateKey([]byte(kp.PrivateKey), req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "encrypt key: "+err.Error())
		return
	}

	user := &store.User{
		ID:                uuid.NewString(),
		Username:          req.Username,
		Email:             req.Email,
		PasswordHash:      pwHash,
		PasswordSalt:      pwSalt,
		Pubkey:            dmcrypto.Address(kp.PublicKey),
		EncryptedPrivkey:  encKey,
		EncryptedKeySalt:  encSalt,
		EncryptedKeyNonce: encNonce,
	}

	if err := d.Store.CreateUser(user); err != nil {
		writeError(w, http.StatusInternalServerError, "create user: "+err.Error())
		return
	}

	// Generate JWT.
	token, err := auth.GenerateJWT(user.ID, d.JWTSecret, jwtExpiry)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "generate token: "+err.Error())
		return
	}

	// Cache session.
	d.Sessions.Put(user.ID, kp.PrivateKey, kp.PublicKey, jwtExpiry)

	writeJSON(w, http.StatusCreated, map[string]string{
		"token":    token,
		"address":  user.Pubkey,
		"mnemonic": kp.Mnemonic,
		"user_id":  user.ID,
	})
}

func (d *MultiTenantDaemon) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password are required")
		return
	}

	user, err := d.Store.GetUserByUsername(req.Username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if user == nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if !auth.VerifyPassword(req.Password, user.PasswordHash, user.PasswordSalt) {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	// Decrypt private key.
	privKeyBytes, err := auth.DecryptPrivateKey(user.EncryptedPrivkey, user.EncryptedKeySalt, user.EncryptedKeyNonce, req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "decrypt key: "+err.Error())
		return
	}

	pubKey, err := dmcrypto.PubKeyFromAddress(user.Pubkey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "decode pubkey: "+err.Error())
		return
	}

	// Cache session.
	d.Sessions.Put(user.ID, privKeyBytes, pubKey, jwtExpiry)

	// Process any pending packets.
	go d.ProcessPendingPackets(user.ID)

	// Generate JWT.
	token, err := auth.GenerateJWT(user.ID, d.JWTSecret, jwtExpiry)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "generate token: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"token":   token,
		"address": user.Pubkey,
		"user_id": user.ID,
	})
}

// --- Protected handlers ---

func (d *MultiTenantDaemon) handleMTStatus(w http.ResponseWriter, r *http.Request) {
	// Public: return status without user-specific info.
	resp := statusResponse{
		ConnectedPeers:  d.ConnectedPeers(),
		IsSyncing:       false,
		PendingPoWTasks: d.PendingPoWTasks(),
		Address:         dmcrypto.Address(d.ServiceKeyPair.PublicKey),
	}
	// Signal to frontend that this is a multi-tenant instance.
	w.Header().Set("X-Dmail-MultiTenant", "true")
	writeJSON(w, http.StatusOK, resp)
}

func (d *MultiTenantDaemon) handleMTIdentity(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	user, err := d.Store.GetUserByID(userID)
	if err != nil || user == nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"address":  user.Pubkey,
		"mnemonic": "", // never expose mnemonic after signup
	})
}

func (d *MultiTenantDaemon) handleMTListMessages(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	folder := r.URL.Query().Get("folder")
	if folder == "" {
		folder = "inbox"
	}
	msgs, err := d.Store.ListMessagesForUser(userID, folder)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if msgs == nil {
		msgs = []store.Message{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"messages": msgs})
}

func (d *MultiTenantDaemon) handleMTGetMessage(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	id := r.PathValue("id")
	// Scope message ID to user.
	scopedID := userID + ":" + id
	msg, err := d.Store.GetMessage(scopedID)
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

func (d *MultiTenantDaemon) handleMTSendMessage(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	var req sendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Recipient == "" || req.Subject == "" {
		writeError(w, http.StatusBadRequest, "recipient and subject are required")
		return
	}

	recipient := req.Recipient
	if strings.HasSuffix(recipient, ".dmail") {
		addr, err := d.ResolveNameForUser(r.Context(), recipient)
		if err != nil {
			writeError(w, http.StatusBadRequest, "cannot resolve name: "+err.Error())
			return
		}
		recipient = addr
	}

	if err := d.SendMessageForUser(r.Context(), userID, recipient, req.Subject, req.Body, req.ReplyToID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

func (d *MultiTenantDaemon) handleMTMarkRead(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	id := r.PathValue("id")
	scopedID := userID + ":" + id
	if err := d.Store.MarkRead(scopedID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Propagate read receipt: if the sender is a local user, update their sent copy.
	msg, _ := d.Store.GetMessage(scopedID)
	if msg != nil {
		senderUser, _ := d.Store.GetUserByPubkey(msg.SenderPubkey)
		if senderUser != nil {
			// Update the sender's sent copy status to "read".
			d.Store.UpdateMessageStatusForUser(senderUser.ID, id, "read")
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

func (d *MultiTenantDaemon) handleMTDeleteMessage(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	id := r.PathValue("id")
	scopedID := userID + ":" + id
	if err := d.Store.DeleteMessage(scopedID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (d *MultiTenantDaemon) handleMTListContacts(w http.ResponseWriter, r *http.Request) {
	// In multi-tenant mode, contacts are shared (not user-scoped for simplicity).
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

func (d *MultiTenantDaemon) handleMTSaveContact(w http.ResponseWriter, r *http.Request) {
	var req contactRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Pubkey == "" || req.Petname == "" {
		writeError(w, http.StatusBadRequest, "pubkey and petname are required")
		return
	}
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

func (d *MultiTenantDaemon) handleMTDeleteContact(w http.ResponseWriter, r *http.Request) {
	pubkey := r.PathValue("pubkey")
	if err := d.Store.DeleteContact(pubkey); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (d *MultiTenantDaemon) handleMTRegisterName(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
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

	session := d.Sessions.Get(userID)
	if session == nil {
		writeError(w, http.StatusUnauthorized, "session expired, please login again")
		return
	}

	if err := names.ValidateName(req.Name); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	kp := &dmcrypto.KeyPair{
		PublicKey:  session.PublicKey,
		PrivateKey: session.PrivateKey,
	}

	// Build and register name in background (PoW takes ~10 min).
	d.pendingPoW.Add(1)
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		defer d.pendingPoW.Add(-1)

		rec, err := names.BuildNameRecord(req.Name, kp)
		if err != nil {
			log.Printf("ERROR build name record for %s: %v", req.Name, err)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := d.Node.RegisterName(ctx, rec); err != nil {
			log.Printf("ERROR register name %s on DHT: %v", req.Name, err)
			return
		}

		raw, _ := names.Marshal(rec)
		entry := &store.NameEntry{
			Name:      req.Name,
			Pubkey:    dmcrypto.Address(kp.PublicKey),
			Timestamp: int64(rec.Timestamp),
			RawRecord: raw,
			IsMine:    true,
		}
		d.Store.SaveName(entry)
		log.Printf("name %s.dmail registered for user %s", req.Name, userID)
	}()

	writeJSON(w, http.StatusAccepted, map[string]string{
		"status": "registration started, PoW computation in progress",
	})
}

func (d *MultiTenantDaemon) handleMTResolveName(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	name = strings.TrimSuffix(name, ".dmail")

	addr, err := d.ResolveNameForUser(r.Context(), name)
	if err != nil {
		writeError(w, http.StatusNotFound, "name not found: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"name":    name + ".dmail",
		"address": addr,
	})
}

func (d *MultiTenantDaemon) handleMTMyNames(w http.ResponseWriter, r *http.Request) {
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

func (d *MultiTenantDaemon) handleMTUnreadCount(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	folder := r.URL.Query().Get("folder")
	if folder == "" {
		folder = "inbox"
	}
	count, err := d.Store.CountUnreadForUser(userID, folder)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"count": count})
}

func (d *MultiTenantDaemon) handleMTSearchMessages(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	q := r.URL.Query().Get("q")
	if q == "" {
		writeJSON(w, http.StatusOK, map[string]any{"messages": []store.Message{}})
		return
	}
	msgs, err := d.Store.SearchMessagesForUser(userID, q, 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if msgs == nil {
		msgs = []store.Message{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"messages": msgs})
}

func (d *MultiTenantDaemon) handleMTGetThread(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	id := r.PathValue("id")
	msgs, err := d.Store.GetThreadForUser(userID, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if msgs == nil {
		msgs = []store.Message{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"messages": msgs})
}

func (d *MultiTenantDaemon) handleMTListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := d.Store.ListUsers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if users == nil {
		users = []store.User{}
	}
	// Return only public fields.
	type publicUser struct {
		ID        string `json:"id"`
		Username  string `json:"username"`
		Pubkey    string `json:"pubkey"`
		CreatedAt int64  `json:"created_at"`
	}
	var result []publicUser
	for _, u := range users {
		result = append(result, publicUser{
			ID:        u.ID,
			Username:  u.Username,
			Pubkey:    u.Pubkey,
			CreatedAt: u.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": result})
}
