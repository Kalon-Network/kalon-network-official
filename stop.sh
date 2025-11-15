#!/bin/bash

# Kalon Network - Stop Script
# Stops node and miner gracefully

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "Stopping Kalon Network..."

# Stop Miner
if pgrep -f "kalon-miner-v2" > /dev/null; then
    echo "Stopping miner..."
    pkill -f kalon-miner-v2
    sleep 2
    if pgrep -f "kalon-miner-v2" > /dev/null; then
        echo "⚠️  Miner did not stop, forcing..."
        pkill -9 -f kalon-miner-v2
    fi
    rm -f logs/miner.pid
    echo "✅ Miner stopped"
else
    echo "✅ Miner is not running"
fi

# Stop Node
if pgrep -f "kalon-node-v2" > /dev/null; then
    echo "Stopping node..."
    pkill -f kalon-node-v2
    sleep 2
    if pgrep -f "kalon-node-v2" > /dev/null; then
        echo "⚠️  Node did not stop, forcing..."
        pkill -9 -f kalon-node-v2
    fi
    rm -f logs/node.pid
    echo "✅ Node stopped"
else
    echo "✅ Node is not running"
fi

echo ""
echo "✅ Kalon Network stopped"

