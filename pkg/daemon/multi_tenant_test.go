package daemon

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"context"
	"os"
	"path/filepath"

	dmcrypto "github.com/ehoneahobed/dmail/pkg/crypto"
)

func setupMultiTenantDaemon(t *testing.T) (*MultiTenantDaemon, func()) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	kp, err := dmcrypto.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	d, err := NewMultiTenant(ctx, MultiTenantConfig{
		ListenPort:     0,
		DataDir:        dbPath,
		ServiceKeyPair: kp,
		PollInterval:   999999, // don't auto-poll in tests
		JWTSecret:      []byte("test-secret-1234567890"),
	})
	if err != nil {
		t.Fatal(err)
	}

	return d, func() {
		d.Close()
		os.RemoveAll(dir)
	}
}

func TestMultiTenantSignupLogin(t *testing.T) {
	d, cleanup := setupMultiTenantDaemon(t)
	defer cleanup()

	handler := d.NewMultiTenantHTTPHandler("")

	// Signup.
	body := `{"username":"alice","password":"password123"}`
	req := httptest.NewRequest("POST", "/api/v1/auth/signup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("signup status = %d, want %d: %s", w.Code, http.StatusCreated, w.Body.String())
	}

	var signupResp map[string]string
	json.NewDecoder(w.Body).Decode(&signupResp)

	if signupResp["token"] == "" {
		t.Fatal("signup should return a token")
	}
	if signupResp["address"] == "" {
		t.Fatal("signup should return an address")
	}
	if signupResp["mnemonic"] == "" {
		t.Fatal("signup should return a mnemonic")
	}

	// Duplicate signup should fail.
	req2 := httptest.NewRequest("POST", "/api/v1/auth/signup", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	if w2.Code != http.StatusConflict {
		t.Fatalf("duplicate signup status = %d, want %d", w2.Code, http.StatusConflict)
	}

	// Login.
	loginBody := `{"username":"alice","password":"password123"}`
	req3 := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(loginBody))
	req3.Header.Set("Content-Type", "application/json")
	w3 := httptest.NewRecorder()
	handler.ServeHTTP(w3, req3)

	if w3.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d: %s", w3.Code, http.StatusOK, w3.Body.String())
	}

	var loginResp map[string]string
	json.NewDecoder(w3.Body).Decode(&loginResp)
	if loginResp["token"] == "" {
		t.Fatal("login should return a token")
	}

	// Wrong password should fail.
	badBody := `{"username":"alice","password":"wrong"}`
	req4 := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(badBody))
	req4.Header.Set("Content-Type", "application/json")
	w4 := httptest.NewRecorder()
	handler.ServeHTTP(w4, req4)
	if w4.Code != http.StatusUnauthorized {
		t.Fatalf("bad login status = %d, want %d", w4.Code, http.StatusUnauthorized)
	}
}

func TestMultiTenantProtectedEndpoints(t *testing.T) {
	d, cleanup := setupMultiTenantDaemon(t)
	defer cleanup()

	handler := d.NewMultiTenantHTTPHandler("")

	// Without auth, protected endpoints should return 401.
	req := httptest.NewRequest("GET", "/api/v1/identity", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unauth identity status = %d, want %d", w.Code, http.StatusUnauthorized)
	}

	// Status should be public.
	req2 := httptest.NewRequest("GET", "/api/v1/status", nil)
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("status should be public, got %d", w2.Code)
	}

	// Signup to get a token.
	body := `{"username":"bob","password":"password123"}`
	req3 := httptest.NewRequest("POST", "/api/v1/auth/signup", strings.NewReader(body))
	req3.Header.Set("Content-Type", "application/json")
	w3 := httptest.NewRecorder()
	handler.ServeHTTP(w3, req3)

	var resp map[string]string
	json.NewDecoder(w3.Body).Decode(&resp)
	token := resp["token"]

	// With auth, identity should work.
	req4 := httptest.NewRequest("GET", "/api/v1/identity", nil)
	req4.Header.Set("Authorization", "Bearer "+token)
	w4 := httptest.NewRecorder()
	handler.ServeHTTP(w4, req4)
	if w4.Code != http.StatusOK {
		t.Fatalf("auth identity status = %d, want %d: %s", w4.Code, http.StatusOK, w4.Body.String())
	}

	// Messages list should work with auth.
	req5 := httptest.NewRequest("GET", "/api/v1/messages", nil)
	req5.Header.Set("Authorization", "Bearer "+token)
	w5 := httptest.NewRecorder()
	handler.ServeHTTP(w5, req5)
	if w5.Code != http.StatusOK {
		t.Fatalf("auth messages status = %d, want %d", w5.Code, http.StatusOK)
	}
}
