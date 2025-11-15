#!/bin/bash

# Kalon Network - Logs Script
# Shows logs from node and miner

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "════════════════════════════════════════════════════════════"
echo "Kalon Network Logs"
echo "════════════════════════════════════════════════════════════"
echo ""

if [ "$1" = "node" ]; then
    echo "Node Logs (last 50 lines):"
    echo "────────────────────────────────────────────────────────"
    tail -50 logs/node.log 2>/dev/null || echo "No node logs found"
elif [ "$1" = "miner" ]; then
    echo "Miner Logs (last 50 lines):"
    echo "────────────────────────────────────────────────────────"
    tail -50 logs/miner.log 2>/dev/null || echo "No miner logs found"
else
    echo "Node Logs (last 20 lines):"
    echo "────────────────────────────────────────────────────────"
    tail -20 logs/node.log 2>/dev/null || echo "No node logs found"
    echo ""
    echo "Miner Logs (last 20 lines):"
    echo "────────────────────────────────────────────────────────"
    tail -20 logs/miner.log 2>/dev/null || echo "No miner logs found"
    echo ""
    echo "Usage:"
    echo "  ./logs.sh node   - Show only node logs"
    echo "  ./logs.sh miner  - Show only miner logs"
fi

echo ""
echo "════════════════════════════════════════════════════════════"
echo ""
echo "To follow logs in real-time:"
echo "  tail -f logs/node.log"
echo "  tail -f logs/miner.log"

