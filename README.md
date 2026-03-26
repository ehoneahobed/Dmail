# Dmail

A zero-cost, serverless, peer-to-peer encrypted messaging system. No servers, no DNS, no spam filters — just cryptography, a DHT, and proof-of-work.

Dmail replaces email infrastructure entirely by combining three battle-tested technologies:

- **Ed25519 public key cryptography** for identity and end-to-end encryption
- **Kademlia DHT (libp2p)** for peer-to-peer message routing and storage
- **Hashcash proof-of-work** for spam resistance without ML filters

## How It Works

1. Your identity is an Ed25519 keypair generated locally. Your address is the Base58Check-encoded public key.
2. To send a message, your node encrypts it with the recipient's public key, computes a proof-of-work stamp (~5 seconds of CPU time), signs it, and pushes it to the DHT.
3. The recipient's node polls the DHT, retrieves the encrypted packet, verifies the PoW and signature, and decrypts it locally.
4. Messages are stored on DHT nodes for 7 days, allowing asynchronous delivery when the recipient comes online.

## Architecture

```
┌─────────────────────┐       HTTP :7777       ┌─────────────────────┐
│   Electron/React    │ ◄────────────────────► │    Go Daemon        │
│   Frontend (UI)     │                        │    (dmaild)         │
└─────────────────────┘                        ├─────────────────────┤
                                               │ Crypto   │ PoW     │
                                               │ SQLite   │ Packets │
                                               ├──────────┴─────────┤
                                               │   libp2p + DHT     │
                                               └─────────┬──────────┘
                                                         │
                                                    P2P Network
```

**Backend (Go):** Handles cryptography, DHT networking, proof-of-work, SQLite storage, and exposes a local REST API.

**Frontend (Electron/React/TypeScript):** Communicates with the daemon over `http://127.0.0.1:7777` and provides inbox, compose, contacts, and onboarding views.

## Prerequisites

- **Go** 1.25.7+ (with CGO enabled for SQLite)
- **Node.js** 18+ and npm
- **Protocol Buffers compiler** (`protoc`) — only needed if modifying `.proto` files

## Quick Start

### 1. Build and start the daemon

```bash
cd Dmail
CGO_ENABLED=1 go build -o dmaild ./cmd/dmaild
./dmaild
```

On first run, the daemon generates a new identity and prints your 12-word recovery phrase:

```
=== NEW IDENTITY CREATED ===
Address:  dmail:2aZzTCGH7XQrRN2bYdZu6ScYmoNzkLbgL5ajVAxMWkNd8tA54H
Mnemonic: abandon ability able about above absent ...
SAVE YOUR MNEMONIC! You need it to recover your identity.
============================
```

### 2. Start the frontend

```bash
cd frontend
npm install
npm run dev
```

Open `http://localhost:5173` in your browser.

### 3. Run as Electron desktop app

```bash
cd frontend
npm run electron
```

## Daemon CLI Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-port` | `7777` | HTTP API port |
| `-p2p-port` | `0` (random) | libp2p TCP listen port |
| `-data-dir` | `~/.dmail` | Data directory for SQLite database |
| `-mnemonic` | — | BIP-39 mnemonic to restore an existing identity |

### Examples

```bash
# Start with defaults
./dmaild

# Custom ports
./dmaild -port 8080 -p2p-port 4001

# Restore identity from mnemonic
./dmaild -mnemonic "abandon ability able about above absent absorb abstract absurd abuse access accident"
```

## API Reference

The daemon exposes a REST API on `127.0.0.1:7777`. All responses are JSON.

### Messages

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/messages?folder=inbox` | List messages in folder (`inbox`, `sent`, `trash`) |
| `GET` | `/api/v1/messages/{id}` | Get a single message |
| `POST` | `/api/v1/messages/send` | Send a message (returns `202 Accepted`) |
| `PUT` | `/api/v1/messages/{id}/read` | Mark message as read |
| `DELETE` | `/api/v1/messages/{id}` | Move message to trash |

**Send request body:**
```json
{
  "recipient": "dmail:9Z8y7X6w...",
  "subject": "Hello",
  "body": "Message text here",
  "reply_to_id": "optional-uuid"
}
```

### Contacts

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/contacts` | List all contacts |
| `POST` | `/api/v1/contacts` | Add or update a contact |
| `DELETE` | `/api/v1/contacts/{pubkey}` | Remove a contact |

**Add contact request body:**
```json
{
  "pubkey": "dmail:9Z8y7X6w...",
  "petname": "Alice"
}
```

### System

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/status` | Peer count, sync state, pending PoW tasks |
| `GET` | `/api/v1/identity` | Your address and mnemonic |

## Project Structure

```
Dmail/
├── cmd/dmaild/              # Daemon entry point
├── internal/pb/             # Protobuf generated Go code
├── pkg/
│   ├── crypto/              # Ed25519 keys, BIP-39 mnemonics, NaCl box encryption, Base58Check addresses
│   ├── pow/                 # Hashcash proof-of-work (SHA-256, configurable difficulty)
│   ├── packet/              # Packet build, verify, and decrypt (ties crypto + PoW + protobuf)
│   ├── node/                # libp2p host + Kademlia DHT with Push/Poll
│   ├── store/               # SQLite storage (messages, contacts, keypair)
│   └── daemon/              # Core daemon + HTTP API server
├── proto/                   # Protobuf schema definition
├── frontend/                # Electron + React + TypeScript UI
│   ├── electron/            # Electron main process
│   └── src/
│       ├── api.ts           # REST client for the daemon
│       ├── App.tsx           # Router, layout, global state
│       └── pages/           # Onboarding, Inbox, Compose, Contacts, MessageDetail
└── docs/                    # PRD and implementation guide
```

## Running Tests

```bash
CGO_ENABLED=1 go test ./... -v
```

Tests cover:
- Key generation and mnemonic determinism
- Base58Check address encoding/decoding with checksum validation
- NaCl box encrypt/decrypt roundtrip and wrong-key rejection
- Ed25519 signing and verification
- Hashcash PoW compute and verify
- Full Alice-Bob integration (keygen, encrypt, PoW, sign, verify, decrypt)
- Invalid PoW rejection, forged signature rejection, sender impersonation detection
- Two-node DHT push/poll integration
- SQLite CRUD for messages, contacts, keypairs
- HTTP API endpoints (status, identity, messages, contacts, send validation)

## Security Model

- **Encryption:** XSalsa20-Poly1305 (NaCl box) with Ed25519-to-Curve25519 key conversion
- **Signing:** Ed25519 signatures on all packet fields
- **Spam resistance:** Hashcash proof-of-work (20 leading zero bits, ~5 seconds)
- **Address integrity:** Base58Check encoding with SHA-256 checksum
- **Packet validation:** Size limits (256KB), timestamp bounds (1h future / 24h past), PoW verification before decryption

## License

MIT
