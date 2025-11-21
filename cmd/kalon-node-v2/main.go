package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/kalon-network/kalon/core"
	"github.com/kalon-network/kalon/network"
	"github.com/kalon-network/kalon/rpc"
	"github.com/kalon-network/kalon/storage"
)

// NodeV2 represents a professional node
type NodeV2 struct {
	config     *NodeConfig
	blockchain *core.BlockchainV2
	rpcServer  *rpc.ServerV2
	p2p        *network.P2P
	running    bool
}

// NodeConfig represents node configuration
type NodeConfig struct {
	DataDir   string
	Genesis   string
	RPCAddr   string
	HTTPSAddr string
	CertFile  string
	KeyFile   string
	P2PAddr   string
	SeedNodes []string // Seed nodes from command line (will be combined with genesis seed nodes)
}

func main() {
	var (
		dataDir   = flag.String("datadir", "data/testnet", "Data directory")
		genesis   = flag.String("genesis", "genesis/testnet.json", "Genesis file")
		rpcAddr   = flag.String("rpc", ":16316", "RPC server address")
		httpsAddr = flag.String("https", "", "HTTPS server address (e.g. :16317)")
		certFile  = flag.String("certfile", "", "SSL certificate file path")
		keyFile   = flag.String("keyfile", "", "SSL private key file path")
		p2pAddr   = flag.String("p2p", ":17335", "P2P server address")
		seedNodes = flag.String("seednodes", "", "Comma-separated list of seed nodes (e.g. 'node1:17335,node2:17335')")
		logLevel  = flag.String("loglevel", "info", "Log level (debug, info, warn, error)")
	)
	flag.Parse()

	// Set log level
	core.SetLogLevelString(*logLevel)

	// Parse seed nodes from command line
	var seedNodesList []string
	if *seedNodes != "" {
		seedNodesList = strings.Split(*seedNodes, ",")
		// Trim whitespace from each seed node
		for i, node := range seedNodesList {
			seedNodesList[i] = strings.TrimSpace(node)
		}
	}

	config := &NodeConfig{
		DataDir:   *dataDir,
		Genesis:   *genesis,
		RPCAddr:   *rpcAddr,
		HTTPSAddr: *httpsAddr,
		CertFile:  *certFile,
		KeyFile:   *keyFile,
		P2PAddr:   *p2pAddr,
		SeedNodes: seedNodesList,
	}

	node := NewNodeV2(config)

	// Start node
	if err := node.Start(); err != nil {
		core.LogError("Failed to start node: %v", err)
		os.Exit(1)
	}

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	core.LogInfo("Shutdown signal received")

	// Stop node
	if err := node.Stop(); err != nil {
		core.LogError("Error stopping node: %v", err)
	}
}

// NewNodeV2 creates a new professional node
func NewNodeV2(config *NodeConfig) *NodeV2 {
	return &NodeV2{
		config: config,
	}
}

// Start starts the node professionally
func (n *NodeV2) Start() error {
	core.LogInfo("Starting Professional Kalon Node v2.0")
	core.LogInfo("   Data Dir: %s", n.config.DataDir)
	core.LogInfo("   Genesis: %s", n.config.Genesis)
	core.LogInfo("   RPC: %s", n.config.RPCAddr)
	core.LogInfo("   P2P: %s", n.config.P2PAddr)

	// Load genesis configuration
	genesis, err := n.loadGenesis()
	if err != nil {
		return err
	}

	// Initialize persistent storage
	dbPath := n.config.DataDir + "/chaindb"
	core.LogInfo("Initializing persistent storage at %s", dbPath)
	levelDBStorage, err := storage.NewLevelDBStorage(dbPath)
	if err != nil {
		core.LogWarn("Failed to initialize LevelDB: %v. Continuing in-memory mode.", err)
		// Create blockchain without persistence
		n.blockchain = core.NewBlockchainV2(genesis, nil)
	} else {
		// Create storage persister
		persister := storage.NewBlockStorage(levelDBStorage)
		n.blockchain = core.NewBlockchainV2(genesis, persister)
	}
	core.LogInfo("Blockchain initialized with height: %d", n.blockchain.GetHeight())

	// Combine seed nodes from genesis, seed-nodes.json, and command line
	seedNodes := make([]string, 0)
	seedNodeMap := make(map[string]bool) // Use map to avoid duplicates

	// Add seed nodes from genesis (if present)
	if len(genesis.SeedNodes) > 0 {
		core.LogInfo("Found %d seed nodes in genesis configuration", len(genesis.SeedNodes))
		for _, seedNode := range genesis.SeedNodes {
			seedNode = strings.TrimSpace(seedNode)
			if seedNode != "" && !seedNodeMap[seedNode] {
				seedNodes = append(seedNodes, seedNode)
				seedNodeMap[seedNode] = true
			}
		}
	}

	// Add seed nodes from seed-nodes.json (licensed seed nodes)
	licensedSeeds := n.loadLicensedSeedNodes()
	if len(licensedSeeds) > 0 {
		core.LogInfo("Found %d licensed seed nodes", len(licensedSeeds))
		for _, seedNode := range licensedSeeds {
			seedNode = strings.TrimSpace(seedNode)
			if seedNode != "" && !seedNodeMap[seedNode] {
				seedNodes = append(seedNodes, seedNode)
				seedNodeMap[seedNode] = true
			}
		}
	}

	// Add seed nodes from command line (if present)
	if len(n.config.SeedNodes) > 0 {
		core.LogInfo("Found %d seed nodes from command line", len(n.config.SeedNodes))
		for _, seedNode := range n.config.SeedNodes {
			seedNode = strings.TrimSpace(seedNode)
			if seedNode != "" && !seedNodeMap[seedNode] {
				seedNodes = append(seedNodes, seedNode)
				seedNodeMap[seedNode] = true
			}
		}
	}

	// Log final seed nodes configuration
	if len(seedNodes) > 0 {
		core.LogInfo("Configured %d seed nodes:", len(seedNodes))
		for i, seedNode := range seedNodes {
			core.LogInfo("  [%d] %s", i+1, seedNode)
		}
	} else {
		core.LogWarn("No seed nodes configured - P2P network will only accept incoming connections")
	}

	// Initialize P2P network first (before RPC server)
	p2pConfig := &network.P2PConfig{
		ListenAddr:   n.config.P2PAddr,
		SeedNodes:    seedNodes,
		MaxPeers:     50,
		DialTimeout:  10 * time.Second,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		KeepAlive:    60 * time.Second,
	}
	n.p2p = network.NewP2P(p2pConfig)

	// Set allowed IPs from seed nodes (whitelist for incoming connections)
	// This ensures only approved seed nodes can connect
	allowedIPs := make([]string, 0)
	for _, seedNode := range seedNodes {
		allowedIPs = append(allowedIPs, seedNode)
	}
	n.p2p.SetAllowedIPs(allowedIPs)
	core.LogInfo("P2P whitelist enabled: %d approved seed nodes", len(allowedIPs))

	// Create RPC server
	n.rpcServer = rpc.NewServerV2(n.config.RPCAddr, n.blockchain)

	// Set P2P network in RPC server for peer count queries
	if n.p2p != nil {
		n.rpcServer.SetP2PNetwork(n.p2p)
	}

	// Configure HTTPS if provided
	if n.config.HTTPSAddr != "" && n.config.CertFile != "" && n.config.KeyFile != "" {
		n.rpcServer.SetHTTPS(n.config.HTTPSAddr, n.config.CertFile, n.config.KeyFile)
		core.LogInfo("HTTPS RPC configured: %s (cert: %s, key: %s)", n.config.HTTPSAddr, n.config.CertFile, n.config.KeyFile)
	}

	// Start RPC server
	go func() {
		if err := n.rpcServer.Start(); err != nil {
			core.LogWarn("RPC Server error: %v", err)
		}
	}()

	// Start P2P server
	if err := n.p2p.Start(); err != nil {
		core.LogWarn("Failed to start P2P: %v", err)
	} else {
		core.LogInfo("P2P network started on %s", n.config.P2PAddr)
	}

	// Setup P2P integration with blockchain
	n.setupP2PIntegration()

	// Start block synchronization routine
	go n.syncBlocks()

	// Wait a moment for server to start
	time.Sleep(1 * time.Second)

	core.LogInfo("Node started successfully")
	n.running = true

	return nil
}

// Stop stops the node gracefully
func (n *NodeV2) Stop() error {
	if !n.running {
		return nil
	}

	core.LogInfo("Stopping node...")

	// Stop RPC server
	if n.rpcServer != nil {
		n.rpcServer.Stop()
	}

	// Stop P2P network
	if n.p2p != nil {
		n.p2p.Stop()
	}

	// Close blockchain storage
	if n.blockchain != nil {
		if err := n.blockchain.Close(); err != nil {
			core.LogWarn("Error closing blockchain: %v", err)
		}
	}

	core.LogInfo("Node stopped successfully")
	n.running = false

	return nil
}

// loadGenesis loads the genesis configuration
func (n *NodeV2) loadGenesis() (*core.GenesisConfig, error) {
	// Load genesis from file
	data, err := os.ReadFile(n.config.Genesis)
	if err != nil {
		core.LogWarn("Failed to read genesis file %s: %v. Using defaults.", n.config.Genesis, err)
		// Return default genesis with proper difficulty
		return &core.GenesisConfig{
			ChainID:            7718,
			Name:               "Kalon Testnet",
			Symbol:             "tKALON",
			BlockTimeTarget:    30,
			MaxSupply:          1000000000,
			InitialBlockReward: 5.0,
			HalvingSchedule:    []core.HalvingEvent{},
			Difficulty: core.DifficultyConfig{
				Algo:                 "LWMA",
				Window:               120,
				InitialDifficulty:    5000,
				MaxAdjustPerBlockPct: 25,
				LaunchGuard: core.LaunchGuard{
					Enabled:                   true,
					DurationHours:             24,
					DifficultyFloorMultiplier: 4.0,
					InitialReward:             2.0,
				},
			},
			AddressFormat: core.AddressFormat{
				Type: "bech32",
				HRP:  "tkalon",
			},
			Premine: core.PremineConfig{
				Enabled: false,
			},
			TreasuryAddress: "tkalon1treasury0000000000000000000000000000000000000000000000000000000000",
			NetworkFee: core.NetworkFeeConfig{
				BlockFeeRate:       0.05,
				TxFeeShareTreasury: 0.20,
				BaseTxFee:          0.01,
				GasPrice:           1000,
			},
			Governance: core.GovernanceConfig{
				Parameters: core.GovernanceParameters{
					NetworkFeeRate:     0.05,
					TxFeeShareTreasury: 0.20,
					TreasuryCapPercent: 10,
				},
			},
		}, nil
	}

	// Parse JSON
	var genesis core.GenesisConfig
	if err := json.Unmarshal(data, &genesis); err != nil {
		return nil, fmt.Errorf("failed to parse genesis JSON: %w", err)
	}

	core.LogInfo("Loaded genesis from %s", n.config.Genesis)
	return &genesis, nil
}

// loadLicensedSeedNodes loads licensed seed nodes from seed-nodes.json
func (n *NodeV2) loadLicensedSeedNodes() []string {
	// Try to load from genesis directory first, then from data directory
	seedNodesFile := "genesis/seed-nodes.json"
	if _, err := os.Stat(seedNodesFile); os.IsNotExist(err) {
		// Try data directory
		dataSeedNodesFile := n.config.DataDir + "/seed-nodes.json"
		if _, err := os.Stat(dataSeedNodesFile); err == nil {
			seedNodesFile = dataSeedNodesFile
		} else {
			// File doesn't exist, return empty list
			return []string{}
		}
	}

	data, err := os.ReadFile(seedNodesFile)
	if err != nil {
		core.LogDebug("Could not read seed-nodes.json: %v (this is normal if file doesn't exist)", err)
		return []string{}
	}

	var seedNodesConfig struct {
		LicensedSeedNodes []struct {
			IP            string `json:"ip"`
			LicenseTxHash string `json:"licenseTxHash"`
			LicenseDate   string `json:"licenseDate"`
			Status        string `json:"status"`
		} `json:"licensedSeedNodes"`
	}

	if err := json.Unmarshal(data, &seedNodesConfig); err != nil {
		core.LogWarn("Failed to parse seed-nodes.json: %v", err)
		return []string{}
	}

	// Extract only active seed nodes
	activeSeeds := make([]string, 0)
	for _, seed := range seedNodesConfig.LicensedSeedNodes {
		if seed.Status == "active" && seed.IP != "" {
			activeSeeds = append(activeSeeds, seed.IP)
		}
	}

	return activeSeeds
}

// setupP2PIntegration sets up the integration between P2P network and blockchain
func (n *NodeV2) setupP2PIntegration() {
	// Subscribe to blockAdded events and broadcast to peers
	blockAddedChan := n.blockchain.GetEventBus().Subscribe("blockAdded")
	go func() {
		for event := range blockAddedChan {
			if eventData, ok := event.(map[string]interface{}); ok {
				if block, ok := eventData["block"].(*core.Block); ok {
					// Convert core.Block to network.Block
					networkBlock := network.ConvertCoreBlockToNetworkBlock(block)
					if networkBlock != nil {
						// Broadcast to all peers
						if err := n.p2p.BroadcastBlock(networkBlock); err != nil {
							core.LogWarn("Failed to broadcast block: %v", err)
						} else {
							core.LogDebug("Broadcasted block #%d to peers", block.Header.Number)
						}
					}
				}
			}
		}
	}()

	// Set handler for received blocks from peers
	n.p2p.SetBlockHandler(func(networkBlock *network.Block) error {
		// Convert network.Block to core.Block
		coreBlock, err := network.ConvertNetworkBlockToCoreBlock(networkBlock)
		if err != nil {
			core.LogWarn("Failed to convert network block (height %d): %v", networkBlock.Header.Number, err)
			return fmt.Errorf("failed to convert network block: %w", err)
		}

		// Add block to blockchain
		if err := n.blockchain.AddBlockV2(coreBlock); err != nil {
			// Don't log error if block already exists (common case)
			if err.Error() != "block already exists" {
				// Check if it's a parent block issue
				errStr := err.Error()
				if strings.Contains(errStr, "parent") || strings.Contains(errStr, "Parent") {
					// Get current best block to check parent hash
					bestBlock := n.blockchain.GetBestBlock()
					if bestBlock != nil {
						core.LogWarn("Failed to add block #%d: %v (current best block: #%d, hash: %x, expected parent: %x)",
							coreBlock.Header.Number, err, bestBlock.Header.Number, bestBlock.Hash, coreBlock.Header.ParentHash)
					} else {
						core.LogWarn("Failed to add block #%d: %v (may need earlier blocks first)", coreBlock.Header.Number, err)
					}
				} else if strings.Contains(errStr, "validation") || strings.Contains(errStr, "difficulty") || strings.Contains(errStr, "merkle") {
					core.LogWarn("Failed to add block #%d: validation error - %v", coreBlock.Header.Number, err)
				} else {
					core.LogWarn("Failed to add block #%d from peer: %v", coreBlock.Header.Number, err)
				}
			}
			return err
		}

		newHeight := n.blockchain.GetHeight()
		// Only log every 10th block to reduce log spam during full sync
		if newHeight%10 == 0 || newHeight < 200 {
			core.LogInfo("✅ Added block #%d from peer (height now: %d)", coreBlock.Header.Number, newHeight)
		}
		return nil
	})

	// Set handler for received transactions from peers
	n.p2p.SetTransactionHandler(func(networkTx *network.Transaction) error {
		// Convert network.Transaction to core.Transaction
		coreTx := network.ConvertNetworkTransactionToCoreTransaction(networkTx)
		if coreTx == nil {
			return fmt.Errorf("failed to convert network transaction")
		}

		// CRITICAL: Validate transaction before adding to mempool
		// This ensures only valid, signed transactions from peers are accepted
		consensusManager := core.NewConsensusManager(n.blockchain.GetGenesis())
		if err := consensusManager.ValidateTransaction(coreTx); err != nil {
			core.LogWarn("Invalid transaction received from peer: %v, Hash: %x", err, coreTx.Hash)
			return fmt.Errorf("transaction validation failed: %v", err)
		}

		// Add transaction to mempool
		n.blockchain.GetMempool().AddTransaction(coreTx)
		core.LogDebug("Valid transaction received from peer: %x", coreTx.Hash)
		return nil
	})

	// Set handler for get_blocks requests
	n.p2p.SetGetBlocksHandler(func(startHeight uint64, endHeight uint64) ([]*network.Block, error) {
		// Get blocks from blockchain
		currentHeight := n.blockchain.GetHeight()

		core.LogDebug("get_blocks request: startHeight=%d, endHeight=%d, currentHeight=%d", startHeight, endHeight, currentHeight)

		// Limit end height to current height
		if endHeight > currentHeight {
			endHeight = currentHeight
		}

		// Check if startHeight is valid
		if startHeight > currentHeight {
			core.LogDebug("get_blocks: startHeight %d > currentHeight %d, returning empty", startHeight, currentHeight)
			return []*network.Block{}, nil
		}

		// Limit range to 100 blocks max
		if endHeight-startHeight > 100 {
			endHeight = startHeight + 100
		}

		blocks := make([]*network.Block, 0)
		successCount := 0
		errorCount := 0
		for i := startHeight; i <= endHeight; i++ {
			// Get block by number
			block, err := n.blockchain.GetBlockByNumber(i)
			if err != nil {
				core.LogDebug("get_blocks: Failed to get block %d: %v", i, err)
				errorCount++
				continue
			}
			if block == nil {
				core.LogDebug("get_blocks: Block %d is nil", i)
				errorCount++
				continue
			}

			// Convert to network block
			networkBlock := network.ConvertCoreBlockToNetworkBlock(block)
			if networkBlock != nil {
				blocks = append(blocks, networkBlock)
				successCount++
			} else {
				core.LogDebug("get_blocks: Failed to convert block %d to network block", i)
				errorCount++
			}
		}

		core.LogInfo("get_blocks: Returning %d blocks (success: %d, errors: %d) for range %d-%d", len(blocks), successCount, errorCount, startHeight, endHeight)
		return blocks, nil
	})

	core.LogInfo("P2P integration with blockchain setup completed")
}

// syncBlocks periodically checks if the node needs to sync blocks from peers
func (n *NodeV2) syncBlocks() {
	ticker := time.NewTicker(10 * time.Second) // Check every 10 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if !n.running {
				return
			}

			currentHeight := n.blockchain.GetHeight()
			peers := n.p2p.GetPeers()

			if len(peers) == 0 {
				continue
			}

			// Find a peer with higher block height
			// For now, request blocks from any peer (we'll improve this later)
			// Since we don't know peer height, we'll request blocks starting from current height + 1
			for _, peer := range peers {
				peerID := peer.ID

				// CRITICAL: If we're getting parent errors, we have a chain mismatch
				// We need to re-sync from block 1 to ensure correct chain
				var startHeight uint64
				
				// If we're at low height (< 200) and getting parent errors, force full re-sync from block 1
				// This ensures we have the correct chain matching the master node
				if currentHeight < 200 {
					// Force full sync from block 1 to fix chain mismatch
					startHeight = 1
					core.LogWarn("⚠️ Chain mismatch detected (height: %d) - forcing full re-sync from block 1", currentHeight)
					core.LogInfo("🔄 Starting full chain re-sync from block 1 to fix parent hash mismatches")
				} else {
					// Normal sync: request blocks starting from current height + 1
					startHeight = currentHeight + 1
				}

				// Request blocks in batches of 100
				endHeight := startHeight + 99

				// Only request if we're behind (but allow re-syncing from block 1)
				if startHeight > currentHeight+1 && currentHeight >= 200 {
					// Too far ahead, skip (but allow re-sync from block 1)
					continue
				}

				core.LogInfo("🔄 Syncing blocks %d-%d from peer %s (current height: %d)",
					startHeight, endHeight, peerID, currentHeight)

				if err := n.p2p.RequestBlocks(peerID, startHeight, endHeight); err != nil {
					core.LogWarn("Failed to request blocks from peer %s: %v", peerID, err)
				}
				break // Only sync from one peer at a time
			}
		}
	}
}
