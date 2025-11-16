#!/bin/bash

# Kalon Network - Miner Status Script
# Shows miner status and last 20 log lines

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

print_success() {
    echo -e "${GREEN}✅${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

print_error() {
    echo -e "${RED}❌${NC} $1"
}

print_info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

echo ""
echo "════════════════════════════════════════════════════════════"
echo "  Kalon Miner Status"
echo "════════════════════════════════════════════════════════════"
echo ""

# Check if miner is running
if pgrep -f "kalon-miner-v2" > /dev/null; then
    MINER_PIDS=$(pgrep -f "kalon-miner-v2")
    print_success "Miner is RUNNING"
    echo ""
    print_info "Process ID(s): $MINER_PIDS"
    
    # Get process details
    ps aux | grep "kalon-miner-v2" | grep -v grep | while read line; do
        PID=$(echo $line | awk '{print $2}')
        CPU=$(echo $line | awk '{print $3}')
        MEM=$(echo $line | awk '{print $4}')
        CMD=$(echo $line | awk '{for(i=11;i<=NF;i++) printf "%s ", $i; print ""}')
        
        # Extract wallet address from command
        WALLET=$(echo "$CMD" | grep -o "kalon[a-z0-9]*" | head -1)
        THREADS=$(echo "$CMD" | grep -o "\--threads [0-9]*" | awk '{print $2}')
        
        echo "  PID: $PID"
        echo "  CPU: ${CPU}%"
        echo "  Memory: ${MEM}%"
        if [ ! -z "$WALLET" ]; then
            echo "  Wallet: $WALLET"
        fi
        if [ ! -z "$THREADS" ]; then
            echo "  Threads: $THREADS"
        fi
        echo ""
    done
else
    print_error "Miner is NOT running"
    echo ""
    print_info "Start miner with:"
    echo "  nohup ./build/kalon-miner-v2 --wallet YOUR_WALLET_ADDRESS --threads 2 --rpc https://explorer.kalon-network.com/rpc > logs/miner.log 2>&1 &"
    echo ""
fi

# Show last 20 log lines if log file exists
if [ -f "logs/miner.log" ]; then
    echo "════════════════════════════════════════════════════════════"
    echo "  Last 20 Log Lines"
    echo "════════════════════════════════════════════════════════════"
    echo ""
    tail -20 logs/miner.log
    echo ""
else
    print_warning "No log file found (logs/miner.log)"
    echo ""
fi

echo "════════════════════════════════════════════════════════════"
echo ""

