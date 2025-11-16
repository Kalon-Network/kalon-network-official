#!/bin/bash

# Kalon Network - Miner Logs Script
# Shows last 20 lines of miner logs (or follows logs)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Colors
YELLOW='\033[1;33m'
NC='\033[0m'

print_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

# Check if log file exists
if [ ! -f "logs/miner.log" ]; then
    print_warning "No log file found (logs/miner.log)"
    echo "Miner might not have been started yet"
    exit 1
fi

# Check if follow mode is requested
if [ "$1" = "-f" ] || [ "$1" = "--follow" ]; then
    echo "Following miner logs (Ctrl+C to exit)..."
    echo ""
    tail -f logs/miner.log
else
    echo "Last 20 lines of miner logs:"
    echo ""
    tail -20 logs/miner.log
    echo ""
    echo "To follow logs in real-time: ./miner-logs.sh -f"
fi

