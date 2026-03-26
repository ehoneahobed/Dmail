# Dmail User Guide

This guide walks you through installing, setting up, and using Dmail — a decentralized, encrypted messaging application that runs entirely on your computer with no central servers.

---

## Table of Contents

1. [Installation](#installation)
2. [First Launch & Creating Your Identity](#first-launch--creating-your-identity)
3. [Understanding Your Dmail Address](#understanding-your-dmail-address)
4. [Sending a Message](#sending-a-message)
5. [Receiving Messages](#receiving-messages)
6. [Managing Contacts (Petnames)](#managing-contacts-petnames)
7. [Replying and Deleting Messages](#replying-and-deleting-messages)
8. [Recovering Your Identity on a New Device](#recovering-your-identity-on-a-new-device)
9. [Running Two Instances Locally (Testing)](#running-two-instances-locally-testing)
10. [Troubleshooting](#troubleshooting)
11. [How It Works (For the Curious)](#how-it-works-for-the-curious)

---

## Installation

### Requirements

- A Mac, Linux, or Windows computer
- Go 1.25.7 or later (download from https://go.dev/dl/)
- Node.js 18 or later (download from https://nodejs.org/)
- Git (to clone the repository)

### Step 1: Clone and build the daemon

```bash
git clone https://github.com/ehoneahobed/dmail.git
cd dmail
CGO_ENABLED=1 go build -o dmaild ./cmd/dmaild
```

This produces a single binary called `dmaild`.

### Step 2: Install the frontend

```bash
cd frontend
npm install
```

---

## First Launch & Creating Your Identity

### Start the daemon

Open a terminal and run:

```bash
./dmaild
```

The daemon will:
1. Generate a brand-new Ed25519 keypair (your identity)
2. Print your **Dmail address** and **12-word recovery phrase**
3. Start the P2P network node
4. Begin listening for API requests on `http://127.0.0.1:7777`

You will see output like this:

```
=== NEW IDENTITY CREATED ===
Address:  dmail:2aZzTCGH7XQrRN2bYdZu6ScYmoNzkLbgL5ajVAxMWkNd8tA54H
Mnemonic: radar blur cabin loyal crash shock prize once plug innocent month radio
SAVE YOUR MNEMONIC! You need it to recover your identity.
============================
2026/03/26 09:00:00 Dmail address: dmail:2aZzTCGH7XQrRN2bYdZu6ScYmoNzkLbgL5ajVAxMWkNd8tA54H
2026/03/26 09:00:00 HTTP API listening on http://127.0.0.1:7777
```

**Write down your 12-word recovery phrase on paper and store it somewhere safe.** This is the only way to recover your identity if you lose your computer. There is no "forgot password" button — if you lose the phrase, your identity is gone forever.

### Start the frontend

In a second terminal:

```bash
cd frontend
npm run dev
```

Open your browser to `http://localhost:5173`.

### Complete onboarding

The first time you open the frontend, you will see the **Onboarding screen**:

1. Your Dmail address and 12-word recovery phrase are displayed
2. You must confirm you saved the phrase by typing word #3
3. After confirmation, click **Enter Dmail** to access your inbox

---

## Understanding Your Dmail Address

Your Dmail address looks like this:

```
dmail:2aZzTCGH7XQrRN2bYdZu6ScYmoNzkLbgL5ajVAxMWkNd8tA54H
```

- It always starts with `dmail:`
- The rest is your public key encoded in Base58 with a checksum
- The checksum prevents typos — if someone enters a wrong character, the app detects it immediately
- You can share this address publicly. Anyone with it can send you encrypted messages. Only you can decrypt them.
- Your address never changes. It is derived from your recovery phrase.

---

## Sending a Message

1. Click **Compose** in the sidebar (or the **Compose** button in the inbox)
2. In the **To** field, enter the recipient's Dmail address (`dmail:...`) or a contact's petname (e.g., "Alice")
3. Enter a **Subject** and **Body**
4. Click **Send**

### What happens next

After you click Send, the daemon performs three steps:

1. **Encrypt** — Your message is encrypted with the recipient's public key. Only they can read it.
2. **Stamp** — The daemon computes a proof-of-work hash (the "stamp"). This takes about 5 seconds and prevents spam. You will see a "Stamping your message..." indicator.
3. **Publish** — The encrypted, stamped packet is pushed to the peer-to-peer network.

The message appears in your **Sent** folder. The recipient will see it in their inbox the next time their node polls the network (every 60 seconds).

### If the recipient is offline

Messages are stored on the DHT network for 7 days. If the recipient comes online within that window, they will receive your message — even if you are offline by then.

---

## Receiving Messages

Messages arrive automatically. The daemon polls the P2P network every 60 seconds in the background. When a new message is found:

1. The PoW stamp is verified (spam protection)
2. The sender's signature is verified (authenticity)
3. The message is decrypted with your private key
4. It appears in your **Inbox**

Unread messages have a blue left border. Click a message to read it, which marks it as read.

---

## Managing Contacts (Petnames)

Raw Dmail addresses are long and hard to remember. The **Contacts** system lets you assign friendly names ("petnames") to addresses.

### Adding a contact

1. Click **Contacts** in the sidebar
2. Enter the person's Dmail address and a name (e.g., "Alice")
3. Click **Add**

### What petnames do

- In the **Inbox** and **Sent** folders, raw addresses are replaced with the petname
- In **Compose**, you can type a petname instead of the full address — it auto-resolves
- Contacts are stored locally. They are never shared with the network.

### Removing a contact

Click the **Remove** button next to any contact in the list. This only removes the name mapping — it does not block the sender.

---

## Replying and Deleting Messages

### Replying

1. Open a message from your inbox
2. Click **Reply**
3. The compose form opens with the recipient pre-filled and the reply linked to the original message

### Deleting

1. Open a message
2. Click **Delete**
3. The message moves to the **Trash** folder

---

## Recovering Your Identity on a New Device

If you get a new computer or need to reinstall:

1. Install Dmail on the new machine (follow the [Installation](#installation) steps)
2. Start the daemon with your recovery phrase:

```bash
./dmaild -mnemonic "radar blur cabin loyal crash shock prize once plug innocent month radio"
```

3. Your original Dmail address is restored. New messages sent to that address will arrive as usual.

**Note:** Messages that were stored only on your old device's local database are not recoverable. Only new messages sent after recovery will appear.

---

## Running Two Instances Locally (Testing)

To test sending messages between two identities on the same computer:

### Terminal 1 — Alice's node

```bash
./dmaild -port 7777 -p2p-port 4001 -data-dir ./alice-data
```

Note Alice's address from the output.

### Terminal 2 — Bob's node

```bash
./dmaild -port 7778 -p2p-port 4002 -data-dir ./bob-data
```

Note Bob's address from the output.

### Send a message via the API

From a third terminal, send a message from Alice to Bob:

```bash
curl -X POST http://127.0.0.1:7777/api/v1/messages/send \
  -H "Content-Type: application/json" \
  -d '{"recipient":"dmail:BOB_ADDRESS_HERE","subject":"Hello Bob","body":"This is a test!"}'
```

After about 60 seconds (or when Bob's node polls), check Bob's inbox:

```bash
curl http://127.0.0.1:7778/api/v1/messages?folder=inbox
```

---

## Troubleshooting

### "Cannot connect to the Dmail daemon"

The frontend cannot reach the daemon at `http://127.0.0.1:7777`.

**Fix:** Make sure the daemon is running in another terminal. Start it with `./dmaild`.

### "No peers connected" (0 peers in sidebar)

This is normal when running locally without bootstrap nodes. Messages between your own local nodes still work via the DHT. In a production deployment, bootstrap nodes would provide initial peer connections.

### The PoW takes a long time

On slower hardware, the proof-of-work stamp may take longer than 5 seconds. The daemon runs it in a background goroutine, so your node stays responsive. The frontend shows a "Stamping your message..." indicator while it runs.

### Messages not arriving

- The recipient's daemon must be running and connected to the same DHT network
- Messages are stored on the DHT for 7 days, then dropped
- Check the recipient's daemon logs for any verification errors
- Verify the recipient address is correct (checksums prevent typos)

### Database issues

The SQLite database is stored at `~/.dmail/dmail.db` by default. To reset:

```bash
rm -rf ~/.dmail
./dmaild
```

This generates a new identity. Your old identity is lost unless you have the recovery phrase.

---

## How It Works (For the Curious)

### Identity

Your identity is an **Ed25519 keypair**. The private key is derived deterministically from a 12-word BIP-39 mnemonic seed phrase. Your address is the Base58Check encoding of the 32-byte public key plus a 4-byte SHA-256 checksum.

### Sending

When you send a message:

1. The plaintext (subject + body + message ID) is serialized as a Protocol Buffer
2. It is encrypted using **NaCl box** (XSalsa20-Poly1305), which performs an X25519 Diffie-Hellman key exchange between your Ed25519 key (converted to Curve25519) and the recipient's public key
3. A **Hashcash proof-of-work** is computed: the daemon searches for a nonce such that `SHA-256(packet_data || nonce)` has 20 leading zero bits (~1 million attempts, ~5 seconds)
4. The entire packet (version, keys, timestamp, PoW nonce, encrypted payload) is signed with your Ed25519 private key
5. The packet is pushed to the **Kademlia DHT** using `SHA-256(recipient_public_key)` as the routing key

### Receiving

The daemon polls the DHT every 60 seconds for packets stored under its own routing key. For each packet found:

1. Packet size is checked (max 256KB)
2. Timestamp is checked (reject if >1 hour in future or >24 hours in past)
3. Proof-of-work is verified
4. Ed25519 signature is verified against the sender's public key
5. If all checks pass, the encrypted payload is decrypted with your private key
6. The plaintext message is stored in your local SQLite database

### Spam Resistance

Every message requires ~5 seconds of CPU time to stamp. A spammer trying to send 1000 messages would need ~83 minutes of computation. This makes mass spam economically unviable without any content filtering or centralized blacklists.

### Privacy

- Messages are end-to-end encrypted. DHT nodes storing the packet cannot read the subject, body, or any content.
- Only the sender and recipient public keys are visible on the network (needed for routing).
- All data is stored locally in SQLite. Nothing is sent to any cloud service.
