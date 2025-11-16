#!/bin/bash
# Script to reset LevelDB to height 0 by deleting the best_block key
# This ensures the blockchain starts fresh at height 0

set -e

if [ -z "$1" ]; then
    echo "Usage: $0 <leveldb_path>"
    echo "Example: $0 data/testnet/chaindb"
    exit 1
fi

DB_PATH="$1"

if [ ! -d "$DB_PATH" ]; then
    echo "❌ LevelDB path does not exist: $DB_PATH"
    exit 1
fi

echo "════════════════════════════════════════════════════════════"
echo "🔄 RESET LEVELDB TO HEIGHT 0"
echo "════════════════════════════════════════════════════════════"
echo "LevelDB Path: $DB_PATH"
echo ""
echo "⚠️  WARNING: This will delete the 'best_block' key from LevelDB"
echo "   The blockchain will start at height 0 on next restart"
echo ""
read -p "Continue? (yes/no): " confirm

if [ "$confirm" != "yes" ]; then
    echo "❌ Aborted"
    exit 1
fi

# Build reset tool
echo ""
echo "Building reset tool..."
go build -o /tmp/reset-leveldb reset-leveldb-to-zero.go 2>&1 || {
    echo "❌ Failed to build reset tool"
    exit 1
}

# Run reset
echo ""
echo "Resetting LevelDB..."
/tmp/reset-leveldb "$DB_PATH" || {
    echo "❌ Failed to reset LevelDB"
    exit 1
}

echo ""
echo "✅ LevelDB successfully reset to height 0"
echo "   The blockchain will start at height 0 on next restart"

