package main

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	dmcrypto "github.com/ehoneahobed/dmail/pkg/crypto"
	"github.com/ehoneahobed/dmail/pkg/daemon"
	"github.com/ehoneahobed/dmail/pkg/store"
)

func main() {
	port := flag.Int("port", 7777, "HTTP API port")
	p2pPort := flag.Int("p2p-port", 0, "libp2p listen port (0 = random)")
	dataDir := flag.String("data-dir", "", "data directory (default: ~/.dmail)")
	mnemonic := flag.String("mnemonic", "", "BIP-39 mnemonic to restore identity")
	multiTenant := flag.Bool("multi-tenant", false, "enable multi-tenant web service mode")
	jwtSecret := flag.String("jwt-secret", "", "JWT signing secret (required in multi-tenant mode)")
	staticDir := flag.String("static-dir", "", "path to frontend dist/ for static file serving")
	listenAddr := flag.String("listen-addr", "127.0.0.1", "HTTP listen address (use 0.0.0.0 for Docker)")
	flag.Parse()

	// Resolve data directory.
	dir := *dataDir
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatal(err)
		}
		dir = filepath.Join(home, ".dmail")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		log.Fatalf("create data dir: %v", err)
	}

	dbPath := filepath.Join(dir, "dmail.db")

	// Try to load existing keypair from the database.
	var kp *dmcrypto.KeyPair
	var err error

	if *mnemonic != "" {
		kp, err = dmcrypto.KeyPairFromMnemonic(*mnemonic)
		if err != nil {
			log.Fatalf("restore from mnemonic: %v", err)
		}
		log.Println("Identity restored from mnemonic")
	} else {
		// Check the database for an existing keypair.
		st, err := store.Open(dbPath)
		if err != nil {
			log.Fatalf("open store for key check: %v", err)
		}
		pubHex, privHex, err := st.GetKeyPair()
		st.Close()
		if err != nil {
			log.Fatalf("read keypair: %v", err)
		}

		if pubHex != "" && privHex != "" {
			pub, err := hex.DecodeString(pubHex)
			if err != nil {
				log.Fatalf("decode stored pubkey: %v", err)
			}
			priv, err := hex.DecodeString(privHex)
			if err != nil {
				log.Fatalf("decode stored privkey: %v", err)
			}
			kp = &dmcrypto.KeyPair{
				PublicKey:  ed25519.PublicKey(pub),
				PrivateKey: ed25519.PrivateKey(priv),
			}
			log.Println("Identity loaded from database")
		} else {
			kp, err = dmcrypto.GenerateKeyPair()
			if err != nil {
				log.Fatalf("generate keypair: %v", err)
			}
			fmt.Println("=== NEW IDENTITY CREATED ===")
			fmt.Printf("Address:  %s\n", dmcrypto.Address(kp.PublicKey))
			fmt.Printf("Mnemonic: %s\n", kp.Mnemonic)
			fmt.Println("SAVE YOUR MNEMONIC! You need it to recover your identity.")
			fmt.Println("============================")
		}
	}

	log.Printf("Dmail address: %s", dmcrypto.Address(kp.PublicKey))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	addr := fmt.Sprintf("%s:%d", *listenAddr, *port)

	var handler http.Handler
	var closer func() error

	if *multiTenant {
		// Multi-tenant mode: requires JWT secret.
		secret := *jwtSecret
		if secret == "" {
			secret = os.Getenv("JWT_SECRET")
		}
		if secret == "" {
			log.Fatal("--jwt-secret or JWT_SECRET env var required in multi-tenant mode")
		}

		md, err := daemon.NewMultiTenant(ctx, daemon.MultiTenantConfig{
			ListenPort:     *p2pPort,
			DataDir:        dbPath,
			ServiceKeyPair: kp,
			PollInterval:   60 * time.Second,
			JWTSecret:      []byte(secret),
		})
		if err != nil {
			log.Fatalf("start multi-tenant daemon: %v", err)
		}
		handler = md.NewMultiTenantHTTPHandler(*staticDir)
		closer = md.Close
		log.Println("Running in multi-tenant web service mode")
	} else {
		// Single-user mode (desktop / CLI).
		d, err := daemon.New(ctx, daemon.Config{
			ListenPort:   *p2pPort,
			DataDir:      dbPath,
			KeyPair:      kp,
			PollInterval: 60 * time.Second,
		})
		if err != nil {
			log.Fatalf("start daemon: %v", err)
		}
		handler = d.NewHTTPHandler()
		closer = d.Close
	}
	defer closer()

	srv := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	go func() {
		log.Printf("HTTP API listening on http://%s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server: %v", err)
		}
	}()

	// Wait for interrupt.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	srv.Shutdown(shutdownCtx)
}
