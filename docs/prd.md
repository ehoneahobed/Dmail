# Dmail: Product Requirements Document (PRD)

**Status:** Draft / Ready for Engineering
**Author:** Manus AI (Product Manager)
**Target Audience:** Engineering (Claude Code), QA, Design

---

## 1. Product Overview

### 1.1 The Problem
Email is fundamentally broken. It relies on expensive, centralized servers (SMTP/IMAP) to route and store messages. Because sending an email is free, the network is flooded with spam, requiring massive machine-learning filters to keep inboxes usable. This infrastructure cost has forced 90% of the world to rely on a few tech giants (Google, Microsoft, Apple) who act as gatekeepers, scanning content and arbitrarily blacklisting independent servers.

### 1.2 The Solution: Dmail
Dmail is a zero-cost, serverless, asynchronous messaging primitive. It replaces the concept of a "mail server" entirely. 
It combines three battle-tested technologies:
1. **Ed25519 Public Key Cryptography:** For addressing and end-to-end encryption (no DNS, no registries).
2. **Kademlia DHT (libp2p):** For routing and temporary storage (no SMTP/IMAP servers).
3. **Hashcash (Proof-of-Work):** For spam resistance (no ML filters).

To the user, Dmail looks and feels exactly like a clean, modern email client. Under the hood, it is a peer-to-peer node.

### 1.3 Goals
- **Zero Infrastructure Cost:** The system must operate entirely peer-to-peer. No central servers, no DNS, no relays.
- **Economic Spam Resistance:** Spam must be solved by physics (CPU time), not by content filtering.
- **Asynchronous Delivery:** A sender must be able to send a message while the recipient is offline, and the recipient must receive it when they come online.
- **Absolute Privacy:** All messages must be end-to-end encrypted. The network routing layer must not be able to read message contents or metadata (subject lines).
- **Frictionless UX:** The complexity of keys, DHTs, and PoW must be entirely hidden from the user.

### 1.4 Non-Goals (Out of Scope for v1)
- **Large Attachments:** v1 is text-only. (v2 will use IPFS for attachments).
- **Global Name Registry:** v1 relies entirely on local address books (Petnames). (v2 will introduce a DHT-based global name registry).
- **Group Chat / Mailing Lists:** v1 is strictly 1-to-1 messaging.
- **Mobile Apps:** v1 is a Desktop application (Electron/Tauri) with an embedded Go/Rust daemon.

---

## 2. User Personas

### 2.1 Alice (The Privacy Advocate)
Alice wants to communicate without Google scanning her receipts or Microsoft reading her newsletters. She is moderately technical but refuses to manage her own Linux mail server because her IP keeps getting blacklisted. She wants an app she can just install and use.

### 2.2 Bob (The Journalist / Whistleblower)
Bob needs to receive tips from sources. He cannot rely on a centralized service that can be subpoenaed or shut down. He needs an address he can publish publicly that cannot be spammed into oblivion by bad actors.

---

## 3. Core Use Cases

### 3.1 Onboarding & Identity Creation
**User Action:** User opens the app for the first time.
**System Response:** The app generates an Ed25519 keypair locally. It derives the public address (Base58 encoded hash of the public key). It prompts the user to save a 12-word recovery phrase. The user is instantly ready to send and receive. No email confirmation, no phone number required.

### 3.2 Sending a Message
**User Action:** User pastes a recipient's Dmail address, types a subject and body, and clicks "Send."
**System Response:** 
1. The app encrypts the payload with the recipient's public key.
2. The app begins computing a Hashcash Proof-of-Work (target: ~5 seconds of CPU time). The UI shows a "Stamping message..." indicator.
3. Once the PoW is found, the app packages the message and pushes it to the DHT, routing it to the nodes closest to the recipient's public key.
4. The message is moved to the "Sent" folder.

### 3.3 Receiving a Message (Asynchronous)
**User Action:** User opens their laptop after being offline for 2 days.
**System Response:** 
1. The background daemon connects to the DHT.
2. It queries the network for packets addressed to its public key.
3. It downloads the encrypted packets, decrypts them locally, and verifies the sender's signature and PoW stamp.
4. Valid messages appear in the Inbox.
5. The daemon sends a "delete" command to the DHT to clear the retrieved packets from the network.

### 3.4 Managing Contacts (Petnames)
**User Action:** User receives a message from an unknown address (`dmail:1A2b3C4d...`). They know this is their friend Charlie. They click "Add to Contacts" and name it "Charlie."
**System Response:** The app saves this mapping locally. Future messages from this address display as "Charlie." When composing a new message, typing "Charlie" auto-fills the public key.
## 4. System Architecture

The Dmail system consists of two distinct layers running on the user's machine:
1. **The Core Daemon (Backend):** A headless process (written in Go or Rust) that handles all cryptography, DHT networking, and local database storage.
2. **The Client App (Frontend):** A UI (Electron/React or Tauri) that communicates with the Core Daemon via a local REST/WebSocket API.

### 4.1 The Core Daemon (dmaild)
The daemon is responsible for:
- Bootstrapping into the libp2p Kademlia DHT network.
- Managing the local SQLite/LevelDB database (storing messages, contacts, and keys).
- Performing CPU-intensive Proof-of-Work calculations.
- Encrypting and decrypting payloads using libsodium.
- Periodically polling the DHT for new messages.

### 4.2 The DHT Network Layer
- **Protocol:** libp2p Kademlia DHT.
- **Routing Key:** The SHA-256 hash of the recipient's Ed25519 public key.
- **Storage:** Nodes closest to the routing key store the encrypted packet.
- **TTL (Time to Live):** Packets are stored with a TTL of 7 days. If the recipient does not come online within 7 days, the message is dropped by the network.

---

## 5. Data Models

### 5.1 The Network Packet (Wire Format)
This is the exact byte structure pushed to the DHT. It must be serialized using Protocol Buffers.

```protobuf
syntax = "proto3";

message DmailPacket {
  uint32 version = 1;              // Protocol version (currently 1)
  bytes recipient_pubkey = 2;      // 32-byte Ed25519 public key
  bytes sender_pubkey = 3;         // 32-byte Ed25519 public key
  uint64 timestamp = 4;            // Unix timestamp in seconds
  uint64 pow_nonce = 5;            // The nonce that solves the Hashcash puzzle
  uint32 pow_difficulty = 6;       // Number of leading zero bits required (e.g., 20)
  bytes encrypted_payload = 7;     // XSalsa20-Poly1305 encrypted bytes
  bytes signature = 8;             // 64-byte Ed25519 signature of fields 1-7
}
```

### 5.2 The Encrypted Payload (Decrypted Format)
Once the recipient decrypts `encrypted_payload`, it yields this structure:

```protobuf
message Payload {
  string message_id = 1;           // UUID generated by sender
  string subject = 2;              // Plaintext subject
  string body = 3;                 // Plaintext body (Markdown supported)
  string reply_to_id = 4;          // Optional: ID of the message being replied to
}
```

### 5.3 Local Database Schema (SQLite)

**Table: `messages`**
- `id` (TEXT, Primary Key)
- `folder` (TEXT) - 'inbox', 'sent', 'draft', 'trash'
- `sender_pubkey` (TEXT)
- `recipient_pubkey` (TEXT)
- `subject` (TEXT)
- `body` (TEXT)
- `timestamp` (INTEGER)
- `is_read` (BOOLEAN)

**Table: `contacts`**
- `pubkey` (TEXT, Primary Key)
- `petname` (TEXT) - e.g., "Alice"
- `created_at` (INTEGER)

**Table: `keypair`**
- `pubkey` (TEXT, Primary Key)
- `encrypted_privkey` (TEXT) - AES-256-GCM encrypted with user's local password

---

## 6. API Contracts (Daemon <-> Frontend)

The Daemon exposes a local HTTP API on `127.0.0.1:7777`. The Frontend uses this to interact with the network.

### 6.1 `GET /api/v1/messages?folder=inbox`
Returns a list of messages in the specified folder.
**Response:**
```json
{
  "messages": [
    {
      "id": "uuid-1234",
      "sender": "dmail:1A2b3C4d...",
      "subject": "Hello World",
      "timestamp": 1711450000,
      "is_read": false
    }
  ]
}
```

### 6.2 `POST /api/v1/messages/send`
Initiates the sending process. The daemon will perform the PoW, encrypt, and push to the DHT.
**Request:**
```json
{
  "recipient": "dmail:9Z8y7X6w...",
  "subject": "Meeting tomorrow",
  "body": "Are we still on for 10 AM?"
}
```
**Response:** `202 Accepted` (The daemon processes the PoW asynchronously).

### 6.3 `GET /api/v1/status`
Returns the current state of the DHT node.
**Response:**
```json
{
  "connected_peers": 42,
  "is_syncing": false,
  "pending_pow_tasks": 1
}
```

### 6.4 `POST /api/v1/contacts`
Saves a petname for a public key.
**Request:**
```json
{
  "pubkey": "dmail:9Z8y7X6w...",
  "petname": "Charlie"
}
```
## 7. Edge Cases & Failure Modes

A decentralized system fails in very different ways than a centralized one. The following edge cases must be handled gracefully by the daemon and the UI.

### 7.1 The Recipient is Offline for > 7 Days
**The Problem:** The DHT nodes storing the message will drop it after the 7-day TTL expires. If Bob is offline for 8 days, he will never receive Alice's message.
**The Mitigation:** 
- **Sender Side:** The sender's daemon keeps a copy of the sent message in the local `sent` folder. It periodically re-publishes the packet to the DHT every 6 days until it receives a cryptographic "Read Receipt" from the recipient.
- **UI Implication:** The UI should show a "Delivered" checkmark only when the Read Receipt is received.

### 7.2 The Bootstrap Problem (Network Partition)
**The Problem:** When Alice opens the app, her node cannot find any other peers to connect to.
**The Mitigation:** 
- The app ships with a hardcoded list of 20 highly-available "Bootstrap Nodes" (run by the community/foundation).
- If the bootstrap nodes are blocked (e.g., by a national firewall), the UI must allow the user to manually input a peer's IP address/multiaddr to bridge into the network.

### 7.3 Proof-of-Work on Low-End Devices
**The Problem:** A 5-second PoW on a MacBook Pro might take 45 seconds on an old Android phone or a cheap Chromebook.
**The Mitigation:** 
- The PoW calculation must run in a background thread (WebWorker or Go goroutine) so it does not freeze the UI.
- The UI must show a clear, friendly progress state: "Stamping your message... This prevents spam."
- *Future v2 feature:* Allow users to outsource PoW to a trusted secondary device.

### 7.4 Device Loss (Lost Private Key)
**The Problem:** If Alice drops her laptop in a lake, her private key is gone. She loses her address and all her messages.
**The Mitigation:** 
- During onboarding, the app MUST force the user to write down a 12-word BIP-39 mnemonic seed phrase.
- The private key is deterministically derived from this seed.
- If the device is lost, Alice installs Dmail on a new laptop, enters the 12 words, and her address is restored. (Note: past messages stored only on the old local disk are lost, but new messages will arrive).

### 7.5 The Sybil Attack (Spamming the DHT)
**The Problem:** A malicious actor spins up 10,000 fake DHT nodes to surround Bob's public key and drop all messages sent to him (a routing eclipse attack).
**The Mitigation:** 
- libp2p's Kademlia implementation includes Sybil resistance mechanisms (e.g., requiring nodes to have diverse IP addresses or binding Node IDs to a PoW).
- The daemon must be configured to use `S/Kademlia` (Secure Kademlia) parameters to make eclipse attacks computationally expensive.

### 7.6 Clock Drift
**The Problem:** Alice's computer clock is 2 days in the future. She sends a message. The DHT nodes reject it because the timestamp looks invalid.
**The Mitigation:** 
- The daemon must sync its internal clock using NTP (Network Time Protocol) independently of the OS clock before generating PoW or signing packets.
- Packets with timestamps more than 1 hour in the future or 24 hours in the past should be rejected by the DHT.

### 7.7 Malformed Packets (Poison Pills)
**The Problem:** A malicious node sends a packet that is intentionally malformed to crash the Go daemon (e.g., a 5GB payload).
**The Mitigation:** 
- The daemon must enforce strict size limits at the network layer. Max packet size: 256 KB.
- Any packet exceeding this size is dropped before parsing.
- Protobuf parsing must be wrapped in panic-recovery handlers.

### 7.8 Address Typo
**The Problem:** Alice types `dmail:1A2b3C4e...` instead of `dmail:1A2b3C4d...`. The message goes into the void.
**The Mitigation:** 
- Dmail addresses must include a checksum (like Bitcoin Base58Check addresses).
- The UI must validate the checksum instantly. If it fails, the "Send" button remains disabled and shows an "Invalid Address" error.
## 8. Testing Strategy & Acceptance Criteria

Because Dmail is a decentralized protocol, testing requires simulating network conditions, not just mocking a database.

### 8.1 Local Network Simulation (The "Alice & Bob" Test)
**Setup:** Run two instances of the `dmaild` daemon on the same machine, bound to different ports (e.g., 7777 and 7778).
**Test:**
1. Node A generates Keypair A. Node B generates Keypair B.
2. Node A sends a message to Node B's public key.
3. Node A computes PoW, encrypts, and pushes to the local DHT.
4. Node B polls the local DHT, retrieves the packet, decrypts it, and verifies the signature.
**Acceptance:** The message appears in Node B's inbox with the correct plaintext.

### 8.2 The Asynchronous Delivery Test (The "Offline Bob" Test)
**Setup:** Run three nodes (Alice, Bob, and Charlie). Charlie acts as the DHT storage node.
**Test:**
1. Bob goes offline (kill Node B process).
2. Alice sends a message to Bob.
3. Alice's node routes the message to Charlie (the closest active node to Bob's key).
4. Alice goes offline (kill Node A process).
5. Bob comes online (start Node B process).
6. Bob queries the DHT. Charlie returns the packet.
**Acceptance:** Bob successfully receives and decrypts the message even though Alice is offline.

### 8.3 The Spam Resistance Test (The "Lazy Spammer" Test)
**Setup:** Node A attempts to send a message to Node B.
**Test:**
1. Node A modifies the code to skip the PoW calculation (sends a packet with an invalid nonce).
2. Node A pushes the packet to the DHT.
**Acceptance:** The DHT nodes (or Node B) immediately drop the packet upon verifying the PoW. The message never appears in Node B's inbox.

### 8.4 The Impersonation Test (The "Fake Alice" Test)
**Setup:** Node C attempts to spoof a message from Node A to Node B.
**Test:**
1. Node C crafts a packet with Node A's public key in the `sender_pubkey` field.
2. Node C signs the packet with its own private key (or a random key).
**Acceptance:** Node B receives the packet, checks the signature against the `sender_pubkey`, sees a mismatch, and silently drops the packet.

### 8.5 UI Acceptance Criteria
- [ ] Onboarding generates a 12-word seed phrase and forces the user to confirm it.
- [ ] The "Send" button is disabled if the recipient address fails the Base58Check validation.
- [ ] Sending a message displays a visual indicator ("Stamping message...") that blocks the UI from sending another message until the PoW is complete.
- [ ] Adding a contact updates the UI so the raw public key is replaced by the Petname in the Inbox and Sent folders.
- [ ] The app can be closed and reopened without losing the local database state or the private key.

---

## 9. Deployment & Release Plan

### 9.1 Phase 1: The CLI Alpha
- **Goal:** Prove the cryptography and DHT routing work in the wild.
- **Deliverable:** A compiled Go binary (`dmail-cli`) for Mac/Linux/Windows.
- **Audience:** Developers and early testers.

### 9.2 Phase 2: The Desktop Beta
- **Goal:** Prove the UX is viable for non-technical users.
- **Deliverable:** An Electron/Tauri app (`Dmail.dmg`, `Dmail.exe`).
- **Infrastructure:** Deploy 5-10 highly available Bootstrap Nodes (e.g., on DigitalOcean/AWS) to ensure the DHT is stable during early adoption.

### 9.3 Phase 3: The Open Protocol Release
- **Goal:** Decentralize development.
- **Deliverable:** Publish the formal protocol specification (RFC style). Open-source the Go daemon and the React frontend under MIT/GPL.

---

## 10. Future Considerations (v2)
- **Attachments:** Integrate IPFS. The sender uploads the file to IPFS, gets a CID, and includes the CID in the encrypted Dmail payload. The recipient's client fetches the CID.
- **Global Naming:** Implement a DHT-based name registry (e.g., `alice.dmail`) requiring a high-difficulty PoW to register.
- **Mobile Push Notifications:** Since mobile OSes kill background processes, mobile clients will need a lightweight "Notification Relay" that watches the DHT for their public key and sends an Apple/Google push notification to wake the app up. (This introduces a slight centralization trade-off for mobile users, which must be strictly opt-in).
