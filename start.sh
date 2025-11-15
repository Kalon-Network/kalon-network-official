#!/bin/bash

# Kalon Network - Start Script
# Starts node and miner in background (survives SSH disconnect)

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "Starting Kalon Network..."

# Check if binaries exist
if [ ! -f "build/kalon-node-v2" ]; then
    echo "❌ kalon-node-v2 not found. Run ./install.sh first"
    exit 1
fi

# Create directories
mkdir -p logs data/testnet

# Start Node
if pgrep -f "kalon-node-v2" > /dev/null; then
    echo "✅ Node is already running"
else
    echo "Starting node..."
    nohup ./build/kalon-node-v2 \
        -datadir data/testnet \
        -genesis genesis/testnet.json \
        -rpc :16316 \
        -p2p :17335 \
        > logs/node.log 2>&1 &
    
    echo $! > logs/node.pid
    echo "✅ Node started (PID: $(cat logs/node.pid))"
    sleep 5
fi

# Start Miner (if wallet exists)
WALLET_FILE=$(ls -1 wallet-*.json 2>/dev/null | head -1)
if [ -z "$WALLET_FILE" ]; then
    echo "⚠️  No wallet found. Run ./setup.sh to create a wallet first"
    exit 0
fi

WALLET_ADDRESS=$(cat "$WALLET_FILE" | grep -o '"address":"[^"]*"' | cut -d'"' -f4)

if pgrep -f "kalon-miner-v2" > /dev/null; then
    echo "✅ Miner is already running"
else
    echo "Starting miner..."
    nohup ./build/kalon-miner-v2 \
        -wallet "$WALLET_ADDRESS" \
        -threads 2 \
        -rpc http://localhost:16316 \
        > logs/miner.log 2>&1 &
    
    echo $! > logs/miner.pid
    echo "✅ Miner started (PID: $(cat logs/miner.pid))"
fi

echo ""
echo "✅ Kalon Network is running!"
echo "   Node PID: $(cat logs/node.pid 2>/dev/null || echo 'N/A')"
echo "   Miner PID: $(cat logs/miner.pid 2>/dev/null || echo 'N/A')"
echo ""
echo "Use ./status.sh to check status"
echo "Use ./logs.sh to view logs"

