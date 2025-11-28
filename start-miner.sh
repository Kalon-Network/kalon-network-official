#!/bin/bash

# Kalon Network - Start Miner Script (Community)
# Starts miner with nohup using public RPC endpoint

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Default values
WALLET_ADDRESS=""
THREADS=2
RPC_URL="https://explorer.kalon-network.com/rpc"

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --wallet|-w)
            WALLET_ADDRESS="$2"
            shift 2
            ;;
        --threads|-t)
            THREADS="$2"
            shift 2
            ;;
        --rpc|-r)
            RPC_URL="$2"
            shift 2
            ;;
        --help|-h)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --wallet, -w ADDRESS    Wallet address (required)"
            echo "  --threads, -t NUMBER     Number of CPU threads (default: 2)"
            echo "  --rpc, -r URL            RPC endpoint (default: https://explorer.kalon-network.com/rpc)"
            echo "  --help, -h               Show this help"
            echo ""
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# Check if wallet address is provided
if [ -z "$WALLET_ADDRESS" ]; then
    # Try to find wallet file
    WALLET_FILE=$(ls -1 wallet-*.json 2>/dev/null | head -1)
    if [ -n "$WALLET_FILE" ]; then
        WALLET_ADDRESS=$(cat "$WALLET_FILE" | grep -o '"address":"[^"]*"' | cut -d'"' -f4)
        echo -e "${BLUE}ℹ${NC} Using wallet address from $WALLET_FILE: $WALLET_ADDRESS"
    else
        echo -e "${RED}❌${NC} Error: Wallet address required!"
        echo ""
        echo "Usage: $0 --wallet YOUR_WALLET_ADDRESS [--threads 2] [--rpc URL]"
        echo "   Or: Create a wallet file (wallet-*.json) in the current directory"
        exit 1
    fi
fi

# Check if binary exists
if [ ! -f "build/kalon-miner-v2" ]; then
    echo -e "${RED}❌${NC} Error: kalon-miner-v2 not found!"
    echo "   Run ./install.sh first"
    exit 1
fi

# Check if miner is already running
if pgrep -f kalon-miner-v2 > /dev/null; then
    echo -e "${YELLOW}⚠${NC}  Miner is already running!"
    echo "   Stop it first: pkill -f kalon-miner-v2"
    exit 1
fi

# Create directories
mkdir -p logs

# Start miner
echo -e "${BLUE}ℹ${NC} Starting miner..."
echo "   Wallet: $WALLET_ADDRESS"
echo "   Threads: $THREADS"
echo "   RPC: $RPC_URL"
echo ""

nohup ./build/kalon-miner-v2 \
    --wallet "$WALLET_ADDRESS" \
    --threads "$THREADS" \
    --rpc "$RPC_URL" \
    > logs/miner.log 2>&1 &

MINER_PID=$!
echo $MINER_PID > logs/miner.pid

echo -e "${GREEN}✅${NC} Miner started (PID: $MINER_PID)"
echo ""
echo "════════════════════════════════════════════════════════════"
echo "Miner is running in background"
echo "════════════════════════════════════════════════════════════"
echo ""
echo "Useful commands:"
echo "  - Check status: ./miner-status.sh"
echo "  - View logs: ./miner-logs.sh"
echo "  - Follow logs: tail -f logs/miner.log"
echo "  - Stop miner: pkill -f kalon-miner-v2"
echo ""

