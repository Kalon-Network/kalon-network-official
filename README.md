# Kalon Network - Official Community Release

Welcome to Kalon Network! This is the official community release - a clean, simple setup for wallet management and mining tKALON.

## 🚀 Quick Start (5 Minutes)

**Community Members**: You don't need to run your own node! Simply use the public RPC endpoint.

### Step 1-3: Installation

```bash
# 1. Clone repository
git clone https://github.com/Kalon-Network/kalon-network-official.git
cd kalon-network-official

# 2. Make install script executable (if needed)
chmod +x install.sh

# 3. Run installation
./install.sh
```

### Step 4: Create Wallet

```bash
./build/kalon-wallet create --name mywallet
```

**Important**: Save your wallet address and mnemonic phrase! You'll need the wallet address for mining.

### Step 5: Start Mining

```bash
nohup ./build/kalon-miner-v2 \
  --wallet YOUR_WALLET_ADDRESS \
  --threads 2 \
  --rpc https://explorer.kalon-network.com/rpc \
  > logs/miner.log 2>&1 &
```

**Replace `YOUR_WALLET_ADDRESS` with your actual wallet address from Step 4!**

**Important Notes**: 
- Community members **do NOT need their own node** - use the public RPC endpoint: `https://explorer.kalon-network.com/rpc`
- Running your own node in testnet doesn't help - you would need to be manually added as a seed node (requires network configuration)
- For wallet operations and mining, the public RPC endpoint is sufficient
- **In Mainnet**: You can operate a seed node and participate in network rewards - more information will be available when Mainnet launches

## ✨ Features

- **Easy Installation**: One script installs everything
- **No Node Required**: Use the public RPC endpoint - no local node setup needed
- **Simple Wallet Management**: Create wallets and manage your funds easily
- **Mining Ready**: Start mining immediately with just a wallet address
- **Public RPC Endpoint**: All operations work via the public endpoint

## 📋 Requirements

- **OS**: Linux (Ubuntu 20.04+ recommended) or macOS
- **RAM**: 1GB minimum
- **Storage**: 1GB free space (for binaries and wallet files)
- **CPU**: 1+ core (2+ recommended for mining)
- **Network**: Stable internet connection (for RPC endpoint access)

The installation script will automatically install Go if needed.

## 📖 Detailed Guide

### Installation

Run the installation script:

```bash
# Make script executable (if needed)
chmod +x install.sh

# Run installation (as normal user - recommended)
./install.sh

# Or if you need to run as root (not recommended)
./install.sh --allow-root
```

This will:
- Check/install Go (if needed)
- Build all binaries (miner, wallet)
- Create necessary directories
- Set up everything for you

**Note**: 
- The installation script only builds miner and wallet binaries. No node is built as it's not needed for community members.
- **Running as root**: The script will warn you if run as root. Use `--allow-root` flag if you really need to run as root (e.g., for Go installation in /usr/local).

### Manual Commands

#### Create Wallet
```bash
./build/kalon-wallet create --name mywallet
```

#### Start Miner
```bash
nohup ./build/kalon-miner-v2 \
  --wallet YOUR_WALLET_ADDRESS \
  --threads 2 \
  --rpc https://explorer.kalon-network.com/rpc \
  > logs/miner.log 2>&1 &
```

This will:
- Start miner in background using nohup
- Save logs to `logs/miner.log`
- Continue running after SSH disconnect

#### Check Miner Status
```bash
# Make script executable (first time only)
chmod +x miner-status.sh

# Check status
./miner-status.sh
```

Shows:
- Whether miner is running
- Process ID, CPU, Memory usage
- Wallet address and threads
- Last 20 log lines

#### View Miner Logs
```bash
# Make script executable (first time only)
chmod +x miner-logs.sh

# Show last 20 lines
./miner-logs.sh

# Follow logs in real-time
./miner-logs.sh -f
```

#### Stop Miner
```bash
pkill -f kalon-miner-v2
```

#### Check Balance
```bash
./build/kalon-wallet balance \
  --address YOUR_ADDRESS \
  --rpc https://explorer.kalon-network.com/rpc
```

## 💰 Mining Rewards

- **Block Reward**: 4.75 tKALON per block
- **Network Limit**: 2 threads per miner (for fair mining)
- **Difficulty**: Automatically adjusts

## 🔧 Configuration

### Public RPC Endpoint

All wallet and mining operations use the public RPC endpoint:
- **RPC URL**: `https://explorer.kalon-network.com/rpc`
- No local configuration needed
- Works out of the box for all operations

## 📝 Wallet Management

### Create Wallet
```bash
./build/kalon-wallet create --name mywallet
```

This will create a wallet file `wallet-mywallet.json` and display your wallet address. **Save your mnemonic phrase securely!**

### List Wallets
```bash
ls wallet-*.json
```

### Check Balance
```bash
./build/kalon-wallet balance \
  --address kalon1abc... \
  --rpc https://explorer.kalon-network.com/rpc
```

### Send Transaction
```bash
./build/kalon-wallet send \
  --to kalon1def... \
  --amount 1000000 \
  --fee 100000 \
  --input wallet-mywallet.json \
  --rpc https://explorer.kalon-network.com/rpc
```

**Note**: 
- Amounts are in micro-KALON (1 tKALON = 1,000,000 micro-KALON)
- For example: 1.5 tKALON = 1500000 micro-KALON
- Minimum fee: 10000 micro-KALON (0.01 tKALON)

## 🛠️ Troubleshooting

### Miner not starting
- Check wallet exists: `ls wallet-*.json`
- Verify wallet address is correct
- Check internet connection (needed for RPC endpoint)
- Ensure RPC endpoint is accessible: `curl https://explorer.kalon-network.com/rpc`

### Can't check balance or send transactions
- Check internet connection
- Verify RPC endpoint is accessible: `curl https://explorer.kalon-network.com/rpc`
- Ensure wallet file exists and is correct
- Check wallet address format

### Process stops after SSH disconnect
- Always use `nohup` when starting the miner (see commands above)
- Miner will continue running in background after SSH disconnect
- Check status with `./miner-status.sh`
- View logs with `./miner-logs.sh`

## 🌐 Public RPC Endpoint

**Community Members: You don't need your own node!** Use the public RPC endpoint:

**Public RPC URL**: `https://explorer.kalon-network.com/rpc`

This endpoint allows you to:
- Check wallet balances
- Send transactions
- Start mining
- Query blockchain information

**Why no own node in Testnet?**
- Running your own node in testnet doesn't automatically connect you to the network
- You would need to be manually added as a seed node (requires network configuration)
- The public RPC endpoint works perfectly for all wallet and mining operations
- No setup required - just use the public endpoint!

**Mainnet Seed Nodes:**
- In Mainnet, you can operate a seed node and participate in network rewards
- Seed nodes help maintain network stability and connectivity
- More information about seed node operation and rewards will be available when Mainnet launches

**Example Usage:**
```bash
# Check balance
./build/kalon-wallet balance \
  --address YOUR_ADDRESS \
  --rpc https://explorer.kalon-network.com/rpc

# Start mining (runs in background)
nohup ./build/kalon-miner-v2 \
  --wallet YOUR_ADDRESS \
  --threads 2 \
  --rpc https://explorer.kalon-network.com/rpc \
  > logs/miner.log 2>&1 &

# Check miner status
./miner-status.sh

# Send transaction
./build/kalon-wallet send \
  --to RECIPIENT_ADDRESS \
  --amount 1000000 \
  --fee 100000 \
  --input wallet-mywallet.json \
  --rpc https://explorer.kalon-network.com/rpc
```

## 📚 More Information

- **RPC API**: Full JSON-RPC interface via public endpoint: `https://explorer.kalon-network.com/rpc`
- **Block Time**: ~15 seconds
- **Reward**: 4.75 tKALON per block (95% to miner, 5% to treasury)
- **Mainnet**: Seed node operation and rewards will be available in Mainnet - stay tuned for updates!

## 🔒 Security

- **Backup your wallet**: Save `wallet-*.json` files securely
- **Save mnemonic**: Write down your mnemonic phrase in a safe place
- **Protect your keys**: Never share your private keys or mnemonic

## 📞 Support

For issues and questions:
- Check miner logs: `tail -f miner.log` (if using nohup)
- Verify RPC endpoint is accessible: `curl https://explorer.kalon-network.com/rpc`
- Review this README
- Ensure wallet file exists and is correct

## 📄 License

See LICENSE file for details.

---

**Happy Mining! 🎉**

