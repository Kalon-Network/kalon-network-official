#!/bin/bash

# Kalon Network - Status Script
# Shows status of node and miner

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "════════════════════════════════════════════════════════════"
echo "Kalon Network Status"
echo "════════════════════════════════════════════════════════════"
echo ""

# Check Node
if pgrep -f "kalon-node-v2" > /dev/null; then
    NODE_PID=$(pgrep -f "kalon-node-v2" | head -1)
    echo "✅ Node: Running (PID: $NODE_PID)"
    
    # Try to get height
    HEIGHT=$(curl -s -X POST http://localhost:16316 -H "Content-Type: application/json" \
        -d '{"jsonrpc":"2.0","method":"getHeight","id":1}' 2>/dev/null | \
        grep -o '"result":[0-9]*' | cut -d: -f2)
    
    if [ -n "$HEIGHT" ]; then
        echo "   Height: $HEIGHT"
    else
        echo "   Status: Starting..."
    fi
else
    echo "❌ Node: Not running"
fi

echo ""

# Check Miner
if pgrep -f "kalon-miner-v2" > /dev/null; then
    MINER_PID=$(pgrep -f "kalon-miner-v2" | head -1)
    echo "✅ Miner: Running (PID: $MINER_PID)"
    
    # Get wallet address from running miner
    MINER_CMD=$(ps -p $MINER_PID -o cmd= 2>/dev/null | grep -o 'kalon-miner-v2.*' || echo "")
    if [ -n "$MINER_CMD" ]; then
        WALLET=$(echo "$MINER_CMD" | grep -o '\-wallet [^ ]*' | cut -d' ' -f2 || echo "N/A")
        THREADS=$(echo "$MINER_CMD" | grep -o '\-threads [0-9]*' | cut -d' ' -f2 || echo "N/A")
        echo "   Wallet: ${WALLET:0:20}..."
        echo "   Threads: $THREADS"
    fi
else
    echo "❌ Miner: Not running"
fi

echo ""

# Check Wallets
WALLET_COUNT=$(ls -1 wallet-*.json 2>/dev/null | wc -l)
if [ "$WALLET_COUNT" -gt 0 ]; then
    echo "✅ Wallets: $WALLET_COUNT wallet(s) found"
    WALLET_FILE=$(ls -1 wallet-*.json 2>/dev/null | head -1)
    WALLET_ADDRESS=$(cat "$WALLET_FILE" | grep -o '"address":"[^"]*"' | cut -d'"' -f4)
    
    # Get balance if node is running
    if pgrep -f "kalon-node-v2" > /dev/null; then
        BALANCE=$(curl -s -X POST http://localhost:16316 -H "Content-Type: application/json" \
            -d "{\"jsonrpc\":\"2.0\",\"method\":\"getBalance\",\"params\":{\"address\":\"$WALLET_ADDRESS\"},\"id\":1}" 2>/dev/null | \
            grep -o '"result":[0-9.]*' | cut -d: -f2)
        
        if [ -n "$BALANCE" ]; then
            BALANCE_TKALON=$(echo "scale=2; $BALANCE / 1000000" | bc 2>/dev/null || echo "N/A")
            echo "   Balance: $BALANCE_TKALON tKALON"
        fi
    fi
else
    echo "⚠️  Wallets: No wallet found. Run ./setup.sh to create one"
fi

echo ""
echo "════════════════════════════════════════════════════════════"
echo ""
echo "Useful commands:"
echo "  ./logs.sh    - View logs"
echo "  ./stop.sh    - Stop node and miner"
echo "  ./start.sh   - Start node and miner"

