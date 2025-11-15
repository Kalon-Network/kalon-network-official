#!/bin/bash

# Kalon Network - Community Installation Script
# This script installs and sets up Kalon Network for the community

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
echo "║     Kalon Network - Community Installation                ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""

# Function to print colored output
print_info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

print_success() {
    echo -e "${GREEN}✅${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

print_error() {
    echo -e "${RED}❌${NC} $1"
}

# Check if running as root
if [ "$EUID" -eq 0 ]; then 
    print_warning "Please do not run this script as root"
    exit 1
fi

# Step 1: Check/Install Go
print_info "Step 1: Checking Go installation..."
if command -v go &> /dev/null; then
    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    print_success "Go is already installed: $GO_VERSION"
    
    # Check if version is 1.21 or later
    REQUIRED_VERSION="1.21"
    if [ "$(printf '%s\n' "$REQUIRED_VERSION" "$GO_VERSION" | sort -V | head -n1)" != "$REQUIRED_VERSION" ]; then
        print_warning "Go version $GO_VERSION is older than required $REQUIRED_VERSION"
        read -p "Do you want to install Go 1.21.5? (y/n) " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            INSTALL_GO=true
        else
            print_error "Please install Go 1.21 or later manually"
            exit 1
        fi
    fi
else
    print_warning "Go is not installed"
    read -p "Do you want to install Go 1.21.5? (y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        INSTALL_GO=true
    else
        print_error "Go is required. Please install it manually."
        exit 1
    fi
fi

# Install Go if needed
if [ "$INSTALL_GO" = true ]; then
    print_info "Installing Go 1.21.5..."
    
    # Detect architecture
    ARCH=$(uname -m)
    if [ "$ARCH" = "x86_64" ]; then
        GO_ARCH="amd64"
    elif [ "$ARCH" = "aarch64" ] || [ "$ARCH" = "arm64" ]; then
        GO_ARCH="arm64"
    else
        print_error "Unsupported architecture: $ARCH"
        exit 1
    fi
    
    GO_VERSION="1.21.5"
    GO_TAR="go${GO_VERSION}.linux-${GO_ARCH}.tar.gz"
    GO_URL="https://go.dev/dl/${GO_TAR}"
    
    cd /tmp
    wget -q "$GO_URL" || {
        print_error "Failed to download Go"
        exit 1
    }
    
    sudo rm -rf /usr/local/go
    sudo tar -C /usr/local -xzf "$GO_TAR"
    rm "$GO_TAR"
    
    # Add to PATH
    if ! grep -q '/usr/local/go/bin' ~/.bashrc; then
        echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
    fi
    
    export PATH=$PATH:/usr/local/go/bin
    print_success "Go ${GO_VERSION} installed successfully"
fi

# Verify Go installation
if ! command -v go &> /dev/null; then
    print_error "Go is not in PATH. Please run: export PATH=\$PATH:/usr/local/go/bin"
    print_info "Or restart your terminal and run this script again"
    exit 1
fi

# Step 2: Build binaries
print_info "Step 2: Building Kalon Network binaries..."
cd "$SCRIPT_DIR"

# Create build directory
mkdir -p build

# Build node
print_info "Building kalon-node-v2..."
go build -o build/kalon-node-v2 cmd/kalon-node-v2/main.go
if [ $? -eq 0 ]; then
    print_success "kalon-node-v2 built successfully"
else
    print_error "Failed to build kalon-node-v2"
    exit 1
fi

# Build miner
print_info "Building kalon-miner-v2..."
go build -o build/kalon-miner-v2 cmd/kalon-miner-v2/main.go
if [ $? -eq 0 ]; then
    print_success "kalon-miner-v2 built successfully"
else
    print_error "Failed to build kalon-miner-v2"
    exit 1
fi

# Build wallet
print_info "Building kalon-wallet..."
go build -o build/kalon-wallet cmd/kalon-wallet/main.go
if [ $? -eq 0 ]; then
    print_success "kalon-wallet built successfully"
else
    print_error "Failed to build kalon-wallet"
    exit 1
fi

# Step 3: Create directories
print_info "Step 3: Creating directories..."
mkdir -p data/testnet logs
print_success "Directories created"

# Step 4: Make scripts executable
print_info "Step 4: Making scripts executable..."
chmod +x start.sh stop.sh status.sh logs.sh 2>/dev/null || true
print_success "Scripts are ready"

echo ""
print_success "Installation completed successfully!"
echo ""
echo "════════════════════════════════════════════════════════════"
echo "Next Steps:"
echo "════════════════════════════════════════════════════════════"
echo ""
echo "1. Run the setup wizard:"
echo "   ./setup.sh"
echo ""
echo "   This will guide you through:"
echo "   - Starting your node"
echo "   - Creating a wallet"
echo "   - Starting the miner"
echo ""
echo "2. Or use the management scripts:"
echo "   ./start.sh   - Start node and miner"
echo "   ./stop.sh    - Stop node and miner"
echo "   ./status.sh  - Check status"
echo "   ./logs.sh    - View logs"
echo ""
echo "════════════════════════════════════════════════════════════"
echo ""

