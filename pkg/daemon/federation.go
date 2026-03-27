package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	dmcrypto "github.com/ehoneahobed/dmail/pkg/crypto"
	"github.com/ehoneahobed/dmail/pkg/packet"
)

// wellKnownResponse is the JSON returned by /.well-known/dmail.
type wellKnownResponse struct {
	ServerPubkey string `json:"server_pubkey"`
	ServerURL    string `json:"server_url"`
	Version      string `json:"version"`
}

// resolveResponse is the JSON returned by /api/v1/federation/resolve.
type resolveResponse struct {
	Username string `json:"username"`
	Pubkey   string `json:"pubkey"`
}

// federationHTTPClient is used for outbound federation requests.
var federationHTTPClient = &http.Client{
	Timeout: 15 * time.Second,
}

// --- Inbound handlers (public, no JWT) ---

// handleWellKnown returns this server's federation identity.
func (d *MultiTenantDaemon) handleWellKnown(w http.ResponseWriter, r *http.Request) {
	serverURL := os.Getenv("DMAIL_SERVER_URL")
	if serverURL == "" {
		writeError(w, http.StatusServiceUnavailable, "federation not enabled")
		return
	}

	resp := wellKnownResponse{
		ServerPubkey: dmcrypto.Address(d.ServiceKeyPair.PublicKey),
		ServerURL:    serverURL,
		Version:      "1",
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleFederationResolve looks up a local user by username and returns their pubkey.
func (d *MultiTenantDaemon) handleFederationResolve(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	if username == "" {
		writeError(w, http.StatusBadRequest, "username parameter required")
		return
	}

	user, err := d.Store.GetUserByUsername(username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup failed")
		return
	}
	if user == nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	writeJSON(w, http.StatusOK, resolveResponse{
		Username: user.Username,
		Pubkey:   user.Pubkey,
	})
}

// handleFederationDeliver receives a raw protobuf DmailPacket from a remote server.
func (d *MultiTenantDaemon) handleFederationDeliver(w http.ResponseWriter, r *http.Request) {
	// Enforce 256KB limit.
	r.Body = http.MaxBytesReader(w, r.Body, packet.MaxPacketSize)
	data, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "packet too large")
		return
	}

	// Unmarshal and verify PoW + signature.
	pkt, err := packet.UnmarshalAndVerify(data)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid packet: "+err.Error())
		return
	}

	// Find the recipient user by their pubkey.
	recipientAddr := dmcrypto.Address(pkt.RecipientPubkey)
	user, err := d.Store.GetUserByPubkey(recipientAddr)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "recipient lookup failed")
		return
	}
	if user == nil {
		writeError(w, http.StatusNotFound, "recipient not found on this server")
		return
	}

	// Store as pending packet for lazy decryption.
	senderAddr := dmcrypto.Address(pkt.SenderPubkey)
	if err := d.Store.SavePendingPacket(user.ID, data, senderAddr); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store packet")
		return
	}

	// If the user has an active session, process immediately.
	if session := d.Sessions.Get(user.ID); session != nil {
		go d.ProcessPendingPackets(user.ID)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "delivered"})
}

// --- Outbound functions ---

// resolveViaFederation resolves a username on a remote server to a dmail address.
func resolveViaFederation(ctx context.Context, username, host string) (string, error) {
	// Discover server URL via well-known endpoint.
	serverURL, err := discoverServerURL(ctx, host)
	if err != nil {
		return "", fmt.Errorf("discover server: %w", err)
	}

	// Resolve username to pubkey.
	resolveURL := serverURL + "/api/v1/federation/resolve?username=" + url.QueryEscape(username)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, resolveURL, nil)
	if err != nil {
		return "", fmt.Errorf("build resolve request: %w", err)
	}

	resp, err := federationHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("resolve request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("resolve failed (status %d): %s", resp.StatusCode, string(body))
	}

	var result resolveResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode resolve response: %w", err)
	}

	if result.Pubkey == "" {
		return "", fmt.Errorf("remote server returned empty pubkey")
	}

	return result.Pubkey, nil
}

// deliverViaFederation sends a raw protobuf packet to a remote server.
func deliverViaFederation(ctx context.Context, serverHost string, pktData []byte) error {
	serverURL, err := discoverServerURL(ctx, serverHost)
	if err != nil {
		return fmt.Errorf("discover server: %w", err)
	}

	deliverURL := serverURL + "/api/v1/federation/deliver"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, deliverURL, bytes.NewReader(pktData))
	if err != nil {
		return fmt.Errorf("build deliver request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := federationHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("deliver request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delivery failed (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// discoverServerURL fetches /.well-known/dmail from a host and returns the server URL.
func discoverServerURL(ctx context.Context, host string) (string, error) {
	// Build the well-known URL. Try HTTPS first, fall back to HTTP for localhost.
	scheme := "https"
	if strings.HasPrefix(host, "localhost") || strings.HasPrefix(host, "127.0.0.1") {
		scheme = "http"
	}

	wellKnownURL := fmt.Sprintf("%s://%s/.well-known/dmail", scheme, host)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, wellKnownURL, nil)
	if err != nil {
		return "", fmt.Errorf("build well-known request: %w", err)
	}

	resp, err := federationHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("well-known request to %s: %w", host, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("well-known endpoint returned status %d", resp.StatusCode)
	}

	var wk wellKnownResponse
	if err := json.NewDecoder(resp.Body).Decode(&wk); err != nil {
		return "", fmt.Errorf("decode well-known response: %w", err)
	}

	if wk.ServerURL == "" {
		return "", fmt.Errorf("remote server returned empty server_url")
	}

	return strings.TrimRight(wk.ServerURL, "/"), nil
}

// parseFederatedAddress splits "user@host" into (username, host).
// Returns empty strings for non-federated addresses (e.g., "dmail:..." or "name.dmail").
func parseFederatedAddress(addr string) (username, host string) {
	// Don't treat dmail: addresses as federated.
	if strings.HasPrefix(addr, "dmail:") {
		return "", ""
	}
	// Don't treat .dmail names as federated.
	if strings.HasSuffix(addr, ".dmail") {
		return "", ""
	}

	at := strings.LastIndex(addr, "@")
	if at <= 0 || at >= len(addr)-1 {
		return "", ""
	}

	username = addr[:at]
	host = addr[at+1:]

	// Basic validation: host must contain a dot or be localhost with port.
	if !strings.Contains(host, ".") && !strings.HasPrefix(host, "localhost") {
		return "", ""
	}

	return username, host
}

