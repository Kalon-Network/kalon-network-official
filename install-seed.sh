#!/bin/bash

# Kalon Network - Seed Node Installation Script
# This script installs and configures a Kalon Network seed node

set -e

echo "=========================================="
echo "Kalon Network - Seed Node Installation"
echo "=========================================="
echo ""

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Installing Go 1.21..."
    wget -q https://go.dev/dl/go1.21.5.linux-amd64.tar.gz
    sudo tar -C /usr/local -xzf go1.21.5.linux-amd64.tar.gz
    export PATH=$PATH:/usr/local/go/bin
    rm go1.21.5.linux-amd64.tar.gz
    echo "✅ Go installed successfully"
else
    echo "✅ Go is already installed: $(go version)"
fi

# Create directories
echo ""
echo "Creating directories..."
mkdir -p build logs data/testnet/chaindb
echo "✅ Directories created"

# Build node binary
echo ""
echo "Building kalon-node-v2..."
go build -o build/kalon-node-v2 cmd/kalon-node-v2/main.go
if [ $? -eq 0 ]; then
    echo "✅ Node binary built successfully"
else
    echo "❌ Failed to build node binary"
    exit 1
fi

# Create systemd service file
echo ""
echo "Creating systemd service..."
sudo tee /etc/systemd/system/kalon-seed-node.service > /dev/null <<EOF
[Unit]
Description=Kalon Network Seed Node
After=network.target

[Service]
Type=simple
User=$USER
WorkingDirectory=$(pwd)
ExecStart=$(pwd)/build/kalon-node-v2 \\
    -datadir $(pwd)/data/testnet \\
    -genesis $(pwd)/genesis/testnet.json \\
    -rpc 127.0.0.1:16316 \\
    -p2p 0.0.0.0:17335
Restart=always
RestartSec=10
StandardOutput=append:$(pwd)/logs/node.log
StandardError=append:$(pwd)/logs/node.log

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
echo "✅ Systemd service created"

# Instructions
echo ""
echo "=========================================="
echo "✅ Installation completed successfully!"
echo "=========================================="
echo ""
echo "📋 Next steps:"
echo ""
echo "1. Make sure port 17335 is open in your firewall:"
echo "   sudo ufw allow 17335/tcp"
echo ""
echo "2. Start the seed node:"
echo "   sudo systemctl start kalon-seed-node"
echo ""
echo "3. Enable auto-start on boot:"
echo "   sudo systemctl enable kalon-seed-node"
echo ""
echo "4. Check status:"
echo "   sudo systemctl status kalon-seed-node"
echo ""
echo "5. View logs:"
echo "   tail -f logs/node.log"
echo ""
echo "6. Stop the seed node:"
echo "   sudo systemctl stop kalon-seed-node"
echo ""
echo "=========================================="
echo "Your seed node IP should be: $(curl -s ifconfig.me 2>/dev/null || echo 'YOUR_IP'):17335"
echo "Make sure this IP is added to seed-nodes.json on the master server!"
echo "=========================================="

