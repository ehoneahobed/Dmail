# Dmail Implementation Guide for Claude Code

Welcome to the Dmail project. You are tasked with building a zero-cost, serverless, asynchronous messaging primitive. 

This document provides the context, constraints, and step-by-step instructions you need to build the system correctly. Please read this entirely before writing any code.

## 1. The Core Philosophy
- **No Servers:** Do not write any code that requires a central server, DNS, or a cloud database. The system is 100% peer-to-peer.
- **No ML Filters:** Spam is solved by Proof-of-Work (Hashcash). Do not implement content filtering.
- **Privacy First:** Everything leaving the local machine must be encrypted using Ed25519/XSalsa20-Poly1305.

## 2. The Tech Stack
You must use the following stack. Do not deviate unless absolutely necessary (and if so, explain why).
- **Backend Daemon:** Go (Golang)
- **P2P Networking:** `go-libp2p` (specifically the Kademlia DHT implementation)
- **Cryptography:** `golang.org/x/crypto/nacl/box` (for encryption) and `crypto/ed25519` (for signing)
- **Serialization:** Protocol Buffers (`google.golang.org/protobuf`)
- **Local Database:** SQLite (`mattn/go-sqlite3`)
- **Frontend UI:** Electron + React + TypeScript (communicating with the Go daemon via local HTTP API)

## 3. Implementation Order

You must build the system in the following strict order. Do not start Phase 2 until Phase 1 is fully tested and working.

### Phase 1: The Cryptographic Core (Go)
1. Implement key generation (Ed25519).
2. Implement the Hashcash Proof-of-Work algorithm (target: ~5 seconds on a standard CPU).
3. Define the Protobuf schema for `DmailPacket` and `Payload` (see PRD).
4. Implement the encryption/decryption wrappers using NaCl box.
5. **Test:** Write a unit test where Alice generates a key, encrypts a message for Bob, computes the PoW, and Bob decrypts it.

### Phase 2: The DHT Network Layer (Go)
1. Initialize a `go-libp2p` host.
2. Configure the Kademlia DHT (`go-libp2p-kad-dht`).
3. Implement the `Push` function: route a `DmailPacket` to the DHT using the SHA-256 hash of the recipient's public key as the routing key.
4. Implement the `Poll` function: query the DHT for packets stored under the local node's public key hash.
5. **Test:** Run two local nodes on different ports. Node A pushes a packet. Node B polls and retrieves it.

### Phase 3: The Local Daemon API (Go)
1. Set up a local SQLite database with `messages`, `contacts`, and `keypair` tables.
2. Expose a local HTTP server (e.g., `localhost:7777`).
3. Implement the endpoints defined in the PRD (`GET /messages`, `POST /messages/send`, etc.).
4. The daemon must run a background goroutine that polls the DHT every 60 seconds and saves new messages to SQLite.

### Phase 4: The Frontend (Electron/React)
1. Scaffold a basic Electron + React app.
2. Build the Onboarding screen (generate key, show recovery phrase).
3. Build the Inbox view (fetch from `localhost:7777/messages`).
4. Build the Compose view (send to `localhost:7777/messages/send`).
5. Implement the Petname system (local address book mapping names to public keys).

## 4. Critical Edge Cases to Handle
When writing the Go daemon, you MUST handle these specific failure modes:
- **PoW Validation:** The receiving node MUST verify the PoW nonce before attempting to decrypt or store the packet. If the PoW is invalid, drop the packet immediately.
- **Packet Size Limits:** Enforce a hard limit of 256KB on incoming DHT packets to prevent memory exhaustion attacks.
- **Clock Drift:** Reject packets with timestamps more than 1 hour in the future or 24 hours in the past.
- **Concurrency:** The PoW calculation must not block the DHT polling or the HTTP API. Run it in a separate goroutine.

## 5. Reference Documents
You have been provided with the full Product Requirements Document (`Dmail_PRD.md`). Refer to it for the exact Protobuf schemas, API contracts, and database schemas.

Good luck. Build it robustly.
