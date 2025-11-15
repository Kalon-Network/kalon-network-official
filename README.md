# Kalon Network - Official Community Release

Welcome to Kalon Network! This is the official community release - a clean, simple setup for running your own node and mining tKALON.

## 🚀 Quick Start (5 Minutes)

```bash
# 1. Clone repository
git clone https://github.com/Kalon-Network/kalon-network-official.git
cd kalon-network-official

# 2. Run installation
./install.sh

# 3. Run setup wizard (guides you through everything)
./setup.sh
```

That's it! Your node and miner will be running and continue even after you disconnect SSH.

## ✨ Features

- **Easy Installation**: One script installs everything
- **Interactive Setup**: Step-by-step wizard guides you through setup
- **Background Running**: Node and miner continue after SSH disconnect
- **Simple Management**: Easy scripts to start, stop, and check status
- **Pre-configured**: Seed nodes and settings already configured

## 📋 Requirements

- **OS**: Linux (Ubuntu 20.04+ recommended)
- **RAM**: 2GB minimum
- **Storage**: 10GB free space
- **CPU**: 2+ cores
- **Network**: Stable internet connection

The installation script will automatically install Go if needed.

## 📖 Detailed Guide

### Installation

Run the installation script:

```bash
./install.sh
```

This will:
- Check/install Go (if needed)
- Build all binaries (node, miner, wallet)
- Create necessary directories
- Set up everything for you

### Initial Setup

Run the interactive setup wizard:

```bash
./setup.sh
```

The wizard will guide you through:
1. **Starting the Node** - Connects to Kalon Network
2. **Creating a Wallet** - Stores your address and keys
3. **Starting the Miner** - Begins mining for rewards

### Management Scripts

After setup, use these simple scripts:

```bash
./start.sh    # Start node and miner
./stop.sh     # Stop node and miner
./status.sh   # Check status and balance
./logs.sh     # View logs
```

### Manual Commands

#### Start Node
```bash
./build/kalon-node-v2 \
  -datadir data/testnet \
  -genesis genesis/testnet.json \
  -rpc :16316 \
  -p2p :17335
```

#### Create Wallet
```bash
./build/kalon-wallet create --name mywallet
```

#### Start Miner
```bash
./build/kalon-miner-v2 \
  -wallet YOUR_ADDRESS \
  -threads 2 \
  -rpc http://localhost:16316
```

#### Check Balance
```bash
./build/kalon-wallet balance \
  --address YOUR_ADDRESS \
  --rpc http://localhost:16316
```

## 💰 Mining Rewards

- **Block Reward**: 4.75 tKALON per block
- **Network Limit**: 2 threads per miner (for fair mining)
- **Difficulty**: Automatically adjusts

## 🔧 Configuration

### Seed Nodes

Seed nodes are pre-configured in `genesis/testnet.json`. Your node will automatically connect to them.

### Ports

- **RPC**: 16316 (for wallet and miner communication)
- **P2P**: 17335 (for network communication)

### Data Directory

All blockchain data is stored in `data/testnet/`. Back up this directory regularly.

## 📝 Wallet Management

### Create Wallet
```bash
./build/kalon-wallet create --name mywallet
```

### List Wallets
```bash
ls wallet-*.json
```

### Check Balance
```bash
./build/kalon-wallet balance \
  --address kalon1abc... \
  --rpc http://localhost:16316
```

### Send Transaction
```bash
./build/kalon-wallet send \
  --to kalon1def... \
  --amount 1000000 \
  --fee 100000 \
  --input wallet-mywallet.json \
  --rpc http://localhost:16316
```

**Note**: Amounts are in micro-KALON (1 tKALON = 1,000,000 micro-KALON)

## 🛠️ Troubleshooting

### Node not starting
- Check if port 16316 is available: `netstat -tuln | grep 16316`
- Check logs: `./logs.sh node`
- Ensure data directory exists: `mkdir -p data/testnet`

### Miner not starting
- Check if node is running: `./status.sh`
- Check wallet exists: `ls wallet-*.json`
- Check logs: `./logs.sh miner`

### Can't connect to network
- Check internet connection
- Check firewall settings (ports 16316 and 17335)
- Seed nodes are pre-configured, should connect automatically

### Process stops after SSH disconnect
- Use `./start.sh` which uses `nohup` to keep processes running
- Or use `screen` or `tmux` for interactive sessions

## 📚 More Information

- **RPC API**: Full JSON-RPC interface on port 16316
- **Network**: P2P network on port 17335
- **Block Time**: ~15 seconds
- **Reward**: 4.75 tKALON per block (95% to miner, 5% to treasury)

## 🔒 Security

- **Backup your wallet**: Save `wallet-*.json` files securely
- **Save mnemonic**: Write down your mnemonic phrase in a safe place
- **Protect your keys**: Never share your private keys or mnemonic

## 📞 Support

For issues and questions:
- Check logs: `./logs.sh`
- Check status: `./status.sh`
- Review this README

## 📄 License

See LICENSE file for details.

---

**Happy Mining! 🎉**

