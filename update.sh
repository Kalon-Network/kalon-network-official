#!/bin/bash

# Kalon Network - Community Update Script
# Updates the repository and rebuilds binaries

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

echo ""
echo "╔════════════════════════════════════════════════════════════╗"
echo "║     Kalon Network - Community Update                      ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""

# Function to print colored output
print_info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

print_success() {
    echo -e "${GREEN}✅${NC} $1"
}

print_error() {
    echo -e "${RED}❌${NC} $1"
}

# Step 1: Pull latest changes
print_info "Step 1: Pulling latest changes from repository..."
if git pull origin main; then
    print_success "Repository updated successfully"
else
    print_error "Failed to pull latest changes"
    exit 1
fi

echo ""

# Step 2: Build binaries
print_info "Step 2: Building binaries..."

# Create build directory
mkdir -p build

# Build miner
print_info "Building kalon-miner-v2..."
if go build -o build/kalon-miner-v2 cmd/kalon-miner-v2/main.go; then
    print_success "kalon-miner-v2 built successfully"
else
    print_error "Failed to build kalon-miner-v2"
    exit 1
fi

# Build wallet
print_info "Building kalon-wallet..."
if go build -o build/kalon-wallet cmd/kalon-wallet/main.go; then
    print_success "kalon-wallet built successfully"
else
    print_error "Failed to build kalon-wallet"
    exit 1
fi

# Make binaries executable
chmod +x build/kalon-miner-v2 build/kalon-wallet 2>/dev/null || true

echo ""
print_success "Update successful. Miner can be restarted."
echo ""

