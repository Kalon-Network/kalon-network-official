#!/bin/bash

# Kalon Network - Interactive Setup Wizard
# Guides users through initial setup: Node -> Wallet -> Miner

set -e

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo ""
echo "╔════════════════════════════════════════════════════════════╗"
echo "║     Kalon Network - Setup Wizard                          ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""

# Check if binaries exist
if [ ! -f "build/kalon-node-v2" ] || [ ! -f "build/kalon-miner-v2" ] || [ ! -f "build/kalon-wallet" ]; then
    echo -e "${YELLOW}⚠${NC} Binaries not found. Running install.sh first..."
    ./install.sh
    if [ $? -ne 0 ]; then
        echo -e "${RED}❌${NC} Installation failed. Please fix errors and try again."
        exit 1
    fi
fi

# Step 1: Start Node
echo ""
echo "════════════════════════════════════════════════════════════"
echo "STEP 1: Start Kalon Node"
echo "════════════════════════════════════════════════════════════"
echo ""
echo "The node connects to the Kalon Network and syncs the blockchain."
echo "It will run in the background and continue after you disconnect SSH."
echo ""

# Check if node is already running
if pgrep -f "kalon-node-v2" > /dev/null; then
    echo -e "${GREEN}✅${NC} Node is already running"
    read -p "Do you want to restart it? (y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo "Stopping existing node..."
        pkill -f kalon-node-v2 || true
        sleep 2
    else
        echo "Using existing node..."
        SKIP_NODE=true
    fi
fi

if [ "$SKIP_NODE" != true ]; then
    echo "Starting node..."
    mkdir -p logs data/testnet
    
    # Start node with nohup (runs after SSH disconnect)
    nohup ./build/kalon-node-v2 \
        -datadir data/testnet \
        -genesis genesis/testnet.json \
        -rpc :16316 \
        -p2p :17335 \
        > logs/node.log 2>&1 &
    
    NODE_PID=$!
    echo $NODE_PID > logs/node.pid
    
    echo -e "${GREEN}✅${NC} Node started (PID: $NODE_PID)"
    echo "   Logs: logs/node.log"
    echo ""
    echo "Waiting for node to initialize (10 seconds)..."
    sleep 10
    
    # Check if node is responding
    if curl -s -X POST http://localhost:16316 -H "Content-Type: application/json" \
        -d '{"jsonrpc":"2.0","method":"getHeight","id":1}' > /dev/null 2>&1; then
        HEIGHT=$(curl -s -X POST http://localhost:16316 -H "Content-Type: application/json" \
            -d '{"jsonrpc":"2.0","method":"getHeight","id":1}' | grep -o '"result":[0-9]*' | cut -d: -f2)
        echo -e "${GREEN}✅${NC} Node is responding (Height: ${HEIGHT:-0})"
    else
        echo -e "${YELLOW}⚠${NC} Node is starting, but not yet responding. It will be ready soon."
    fi
fi

# Step 2: Create Wallet
echo ""
echo "════════════════════════════════════════════════════════════"
echo "STEP 2: Create Wallet"
echo "════════════════════════════════════════════════════════════"
echo ""
echo "A wallet stores your private keys and allows you to:"
echo "  - Receive mining rewards"
echo "  - Send and receive tKALON"
echo "  - Check your balance"
echo ""

# Check if wallet already exists
WALLET_COUNT=$(ls -1 wallet-*.json 2>/dev/null | wc -l)
if [ "$WALLET_COUNT" -gt 0 ]; then
    echo -e "${GREEN}✅${NC} Found $WALLET_COUNT existing wallet(s):"
    ls -1 wallet-*.json 2>/dev/null | head -5
    echo ""
    read -p "Do you want to create a new wallet? (y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        SKIP_WALLET=true
        # Use first wallet
        WALLET_FILE=$(ls -1 wallet-*.json 2>/dev/null | head -1)
        WALLET_ADDRESS=$(cat "$WALLET_FILE" | grep -o '"address":"[^"]*"' | cut -d'"' -f4)
        echo -e "${GREEN}✅${NC} Using existing wallet: $WALLET_ADDRESS"
    fi
fi

if [ "$SKIP_WALLET" != true ]; then
    echo "Creating new wallet..."
    echo ""
    read -p "Enter wallet name (or press Enter for 'miner'): " WALLET_NAME
    WALLET_NAME=${WALLET_NAME:-miner}
    
    echo "" | ./build/kalon-wallet create --name "$WALLET_NAME" 2>&1 | grep -v "Enter passphrase"
    
    WALLET_FILE="wallet-${WALLET_NAME}.json"
    if [ -f "$WALLET_FILE" ]; then
        WALLET_ADDRESS=$(cat "$WALLET_FILE" | grep -o '"address":"[^"]*"' | cut -d'"' -f4)
        echo ""
        echo -e "${GREEN}✅${NC} Wallet created successfully!"
        echo "   Address: $WALLET_ADDRESS"
        echo "   File: $WALLET_FILE"
        echo ""
        echo -e "${YELLOW}⚠${NC} IMPORTANT: Save your mnemonic phrase securely!"
        echo "   You will need it to recover your wallet if you lose access."
    else
        echo -e "${RED}❌${NC} Failed to create wallet"
        exit 1
    fi
fi

# Step 3: Start Miner
echo ""
echo "════════════════════════════════════════════════════════════"
echo "STEP 3: Start Miner"
echo "════════════════════════════════════════════════════════════"
echo ""
echo "The miner uses your CPU to find new blocks and earn rewards."
echo "Current reward: 4.75 tKALON per block"
echo ""

# Check if miner is already running
if pgrep -f "kalon-miner-v2" > /dev/null; then
    echo -e "${GREEN}✅${NC} Miner is already running"
    read -p "Do you want to restart it? (y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo "Stopping existing miner..."
        pkill -f kalon-miner-v2 || true
        sleep 2
    else
        echo "Using existing miner..."
        SKIP_MINER=true
    fi
fi

if [ "$SKIP_MINER" != true ]; then
    # Get CPU thread count
    CPU_THREADS=$(nproc)
    echo "Your system has $CPU_THREADS CPU threads available."
    echo "Network limit: 2 threads per miner (for fair mining)"
    echo ""
    read -p "How many threads to use? (1-2, default: 2): " THREADS
    THREADS=${THREADS:-2}
    
    if [ "$THREADS" -gt 2 ]; then
        echo -e "${YELLOW}⚠${NC} Network limit is 2 threads. Using 2 threads."
        THREADS=2
    fi
    
    echo ""
    echo "Starting miner..."
    echo "   Wallet: $WALLET_ADDRESS"
    echo "   Threads: $THREADS"
    echo "   RPC: http://localhost:16316"
    echo ""
    
    # Start miner with nohup (runs after SSH disconnect)
    nohup ./build/kalon-miner-v2 \
        -wallet "$WALLET_ADDRESS" \
        -threads "$THREADS" \
        -rpc http://localhost:16316 \
        > logs/miner.log 2>&1 &
    
    MINER_PID=$!
    echo $MINER_PID > logs/miner.pid
    
    echo -e "${GREEN}✅${NC} Miner started (PID: $MINER_PID)"
    echo "   Logs: logs/miner.log"
fi

# Final summary
echo ""
echo "════════════════════════════════════════════════════════════"
echo "Setup Complete! 🎉"
echo "════════════════════════════════════════════════════════════"
echo ""
echo -e "${GREEN}✅${NC} Node is running (continues after SSH disconnect)"
echo -e "${GREEN}✅${NC} Wallet created: $WALLET_ADDRESS"
echo -e "${GREEN}✅${NC} Miner is running (continues after SSH disconnect)"
echo ""
echo "════════════════════════════════════════════════════════════"
echo "Useful Commands:"
echo "════════════════════════════════════════════════════════════"
echo ""
echo "  ./status.sh          - Check node and miner status"
echo "  ./logs.sh            - View logs"
echo "  ./stop.sh            - Stop node and miner"
echo "  ./start.sh           - Start node and miner"
echo ""
echo "  Check balance:"
echo "    ./build/kalon-wallet balance --address $WALLET_ADDRESS --rpc http://localhost:16316"
echo ""
echo "════════════════════════════════════════════════════════════"
echo ""
echo -e "${YELLOW}ℹ${NC} Your node and miner will continue running after you disconnect SSH."
echo -e "${YELLOW}ℹ${NC} Check logs anytime with: ./logs.sh"
echo ""

