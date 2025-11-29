#!/bin/bash

# Kalon Network - Emergency Update Fix Script
# This script fixes divergent branches and updates the repository
# Use this if ./update.sh fails with "divergent branches" error

set -e

echo ""
echo "╔════════════════════════════════════════════════════════════╗"
echo "║     Kalon Network - Emergency Update Fix                  ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "Fixing divergent branches..."
echo ""

# Set Git configuration
git config pull.rebase false 2>/dev/null || true

# Fetch latest changes
echo "Fetching latest changes from repository..."
git fetch origin main

# Reset to remote main (this fixes divergent branches)
echo "Resetting to remote main branch..."
git reset --hard origin/main

echo ""
echo "✅ Repository fixed! You can now run ./update.sh normally."
echo ""

