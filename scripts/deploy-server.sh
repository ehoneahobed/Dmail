#!/usr/bin/env bash
set -euo pipefail

# Dmail Server Deployment Script
# Run this on your Ubuntu Lightsail instance.
# Usage: bash scripts/deploy-server.sh

echo "=== Dmail Server Setup ==="

# 1. Install Go (if not present)
if ! command -v go &>/dev/null; then
  echo "Installing Go 1.23..."
  wget -q https://go.dev/dl/go1.23.6.linux-amd64.tar.gz
  sudo rm -rf /usr/local/go
  sudo tar -C /usr/local -xzf go1.23.6.linux-amd64.tar.gz
  rm go1.23.6.linux-amd64.tar.gz
  export PATH=$PATH:/usr/local/go/bin
  echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
  echo "Go installed: $(go version)"
else
  echo "Go already installed: $(go version)"
fi

# 2. Install Node.js (if not present)
if ! command -v node &>/dev/null; then
  echo "Installing Node.js 20..."
  curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
  sudo apt-get install -y nodejs
  echo "Node installed: $(node --version)"
else
  echo "Node already installed: $(node --version)"
fi

# 3. Build the Go daemon
echo "Building dmaild..."
export PATH=$PATH:/usr/local/go/bin
CGO_ENABLED=0 go build -o dmaild ./cmd/dmaild/
echo "Daemon built successfully"

# 4. Build the frontend
echo "Building frontend..."
cd frontend
npm install
npm run build
cd ..
echo "Frontend built successfully"

# 5. Create data directory
sudo mkdir -p /var/lib/dmail
sudo chown "$USER:$USER" /var/lib/dmail

# 6. Generate JWT secret if not set
JWT_FILE="/var/lib/dmail/jwt_secret"
if [ ! -f "$JWT_FILE" ]; then
  openssl rand -hex 32 > "$JWT_FILE"
  chmod 600 "$JWT_FILE"
  echo "Generated JWT secret"
fi

# 7. Install as systemd service
echo "Installing systemd service..."
DMAIL_DIR="$(pwd)"
JWT_SECRET="$(cat $JWT_FILE)"

sudo tee /etc/systemd/system/dmail.service > /dev/null <<UNIT
[Unit]
Description=Dmail Multi-Tenant Web Service
After=network.target

[Service]
Type=simple
User=$USER
WorkingDirectory=$DMAIL_DIR
Environment=JWT_SECRET=$JWT_SECRET
ExecStart=$DMAIL_DIR/dmaild --multi-tenant --port 7777 --listen-addr 0.0.0.0 --data-dir /var/lib/dmail --static-dir $DMAIL_DIR/frontend/dist
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
UNIT

sudo systemctl daemon-reload
sudo systemctl enable dmail
sudo systemctl restart dmail

echo ""
echo "=== Dmail is running! ==="
echo ""
echo "Check status: sudo systemctl status dmail"
echo "View logs:    sudo journalctl -u dmail -f"
echo ""
echo "IMPORTANT: Open port 7777 in your Lightsail firewall!"
echo "  Lightsail Console → Networking tab → Add rule:"
echo "  Application: Custom, Protocol: TCP, Port: 7777"
echo ""
echo "Then share this URL with friends:"
PUBLIC_IP=$(curl -s http://checkip.amazonaws.com 2>/dev/null || echo "YOUR_IP")
echo "  http://$PUBLIC_IP:7777"
echo ""
