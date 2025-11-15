package core

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// BlockchainV2 represents a professional blockchain implementation
type BlockchainV2 struct {
	mu              sync.RWMutex
	blocks          []*Block
	height          uint64
	bestBlock       *Block
	genesis         *GenesisConfig
	consensus       *ConsensusV2
	eventBus        *EventBus
	stateManager    *StateManager
	utxoSet         *UTXOSet
	mempool         *Mempool
	storage         BlockPersister // Interface for persistent storage
	SnapshotManager *SnapshotManager
	// Fork detection: Map of parent hash -> list of blocks with same parent (forks)
	forkBlocks map[string][]*Block // Key: hex-encoded parent hash
	// Block index: Map of block hash -> block for quick lookup
	blockIndex map[string]*Block // Key: hex-encoded block hash
	// Block history: Separate structure for LWMA difficulty adjustment (uses separate lock)
	blockHistory *BlockHistory
}

// BlockPersister defines the interface for persisting blocks
type BlockPersister interface {
	StoreBlock(block *Block) error
	GetBlockByNumber(number uint64) (*Block, error)
	GetBlockByHash(hash []byte) (*Block, error)
	GetBestBlock() (*Block, error)
	GetBlockCount() (uint64, error)
	Close() error
}

// Mempool manages pending transactions
type Mempool struct {
	mu           sync.RWMutex
	transactions map[string]*Transaction // Key: transaction hash
}

// EventBus handles blockchain events
type EventBus struct {
	mu       sync.RWMutex
	channels map[string][]chan interface{}
}

// StateManager manages blockchain state
type StateManager struct {
	mu    sync.RWMutex
	state map[string]interface{}
}

// ConsensusV2 represents professional consensus mechanism
type ConsensusV2 struct {
	mu         sync.RWMutex
	difficulty uint64
	target     uint64
	blockTime  time.Duration
	adjustment *DifficultyAdjustment
}

// DifficultyAdjustment handles LWMA difficulty adjustment
type DifficultyAdjustment struct {
	mu          sync.RWMutex
	windowSize  int
	blockTimes  []time.Time
	adjustments []uint64
}

// BlockHistory manages block timestamps for LWMA difficulty adjustment
// Uses separate lock to avoid contention with main blockchain lock
type BlockHistory struct {
	mu         sync.RWMutex
	timestamps []time.Time // Oldest first (chronological order)
	windowSize int
	maxSize    int // windowSize + buffer for safety
}

// NewBlockchainV2 creates a new professional blockchain
func NewBlockchainV2(genesis *GenesisConfig, persister BlockPersister) *BlockchainV2 {
	// Determine window size for block history
	windowSize := 120
	if genesis.Difficulty.Window > 0 {
		windowSize = int(genesis.Difficulty.Window)
	}

	bc := &BlockchainV2{
		blocks:          make([]*Block, 0),
		height:          0,
		genesis:         genesis,
		consensus:       NewConsensusV2(),
		eventBus:        NewEventBus(),
		stateManager:    NewStateManager(),
		utxoSet:         NewUTXOSet(),
		mempool:         NewMempool(),
		storage:         persister,
		SnapshotManager: NewSnapshotManager(),
		forkBlocks:      make(map[string][]*Block),
		blockIndex:      make(map[string]*Block),
		blockHistory:    NewBlockHistory(windowSize), // NEW: Initialize block history
	}

	// Try to load existing chain from storage
	if bc.storage != nil {
		bc.loadChainFromStorage()
	}

	// Initialize history from existing blocks
	if bc.height > 0 {
		bc.blockHistory.Rebuild(bc.blocks)
	}

	// Create genesis block if chain is empty
	if bc.height == 0 {
		genesisBlock := bc.createGenesisBlockV2()
		bc.addBlockV2(genesisBlock)

		// Restore snapshot from genesis config if available
		LogDebug("Checking for snapshot in genesis config...")
		if err := bc.CreateSnapshotFromGenesis(); err != nil {
			LogWarn("Failed to restore snapshot from genesis: %v", err)
		} else {
			LogInfo("Snapshot check completed")
		}
	}

	return bc
}

// NewEventBus creates a new event bus
func NewEventBus() *EventBus {
	return &EventBus{
		channels: make(map[string][]chan interface{}),
	}
}

// NewStateManager creates a new state manager
func NewStateManager() *StateManager {
	return &StateManager{
		state: make(map[string]interface{}),
	}
}

// NewConsensusV2 creates a new consensus mechanism
func NewConsensusV2() *ConsensusV2 {
	return &ConsensusV2{
		difficulty: 10,            // Default difficulty for testnet
		target:     1 << (64 - 1), // 1 difficulty = 2^63 target
		blockTime:  15 * time.Second,
		adjustment: NewDifficultyAdjustment(),
	}
}

// NewDifficultyAdjustment creates a new difficulty adjustment
func NewDifficultyAdjustment() *DifficultyAdjustment {
	return &DifficultyAdjustment{
		windowSize:  144, // 24 hours at 30s blocks
		blockTimes:  make([]time.Time, 0),
		adjustments: make([]uint64, 0),
	}
}

// NewBlockHistory creates a new block history
func NewBlockHistory(windowSize int) *BlockHistory {
	maxSize := windowSize
	if maxSize < 120 {
		maxSize = 120 // Minimum buffer
	}
	return &BlockHistory{
		timestamps: make([]time.Time, 0, maxSize),
		windowSize: windowSize,
		maxSize:    maxSize,
	}
}

// AddBlock adds a new block timestamp (called from addBlockV2)
func (bh *BlockHistory) AddBlock(timestamp time.Time) {
	bh.mu.Lock()
	defer bh.mu.Unlock()

	// Add new timestamp
	bh.timestamps = append(bh.timestamps, timestamp)

	// Keep only last maxSize timestamps
	if len(bh.timestamps) > bh.maxSize {
		bh.timestamps = bh.timestamps[len(bh.timestamps)-bh.maxSize:]
	}
}

// GetHistory returns block history for LWMA calculation (lock-free read)
func (bh *BlockHistory) GetHistory() []time.Time {
	bh.mu.RLock()
	defer bh.mu.RUnlock()

	// Return copy to avoid race conditions
	result := make([]time.Time, len(bh.timestamps))
	copy(result, bh.timestamps)
	return result
}

// GetWindow returns last N timestamps (for LWMA window)
func (bh *BlockHistory) GetWindow(windowSize int) []time.Time {
	bh.mu.RLock()
	defer bh.mu.RUnlock()

	if windowSize > len(bh.timestamps) {
		windowSize = len(bh.timestamps)
	}

	if windowSize == 0 {
		return []time.Time{}
	}

	// Return last windowSize timestamps
	start := len(bh.timestamps) - windowSize
	if start < 0 {
		start = 0
	}

	result := make([]time.Time, windowSize)
	copy(result, bh.timestamps[start:])
	return result
}

// Clear clears the history (for reorganization)
func (bh *BlockHistory) Clear() {
	bh.mu.Lock()
	defer bh.mu.Unlock()
	bh.timestamps = make([]time.Time, 0, bh.maxSize)
}

// Rebuild rebuilds history from blocks (for reorganization)
func (bh *BlockHistory) Rebuild(blocks []*Block) {
	bh.mu.Lock()
	defer bh.mu.Unlock()

	bh.timestamps = make([]time.Time, 0, len(blocks))
	for _, block := range blocks {
		if block != nil {
			bh.timestamps = append(bh.timestamps, block.Header.Timestamp)
		}
	}

	// Keep only last maxSize
	if len(bh.timestamps) > bh.maxSize {
		bh.timestamps = bh.timestamps[len(bh.timestamps)-bh.maxSize:]
	}
}

// createGenesisBlockV2 creates the genesis block with professional approach
func (bc *BlockchainV2) createGenesisBlockV2() *Block {
	genesisTimestamp := time.Unix(1609459200, 0) // 2021-01-01 00:00:00 UTC

	// Genesis block has no transactions, so merkle root is empty
	genesisTxs := []Transaction{}
	consensusManager := NewConsensusManager(bc.genesis)
	merkleRoot := consensusManager.CalculateMerkleRoot(genesisTxs)

	genesisBlock := &Block{
		Header: BlockHeader{
			ParentHash:  Hash{},
			Number:      0,
			Timestamp:   genesisTimestamp,
			Difficulty:  bc.genesis.Difficulty.InitialDifficulty,
			Miner:       Address{},
			Nonce:       0,
			MerkleRoot:  merkleRoot, // Calculate merkle root (empty for genesis)
			TxCount:     0,
			NetworkFee:  0,
			TreasuryFee: 0,
		},
		Txs:  []Transaction{},
		Hash: Hash{},
	}

	// Calculate hash using deterministic method
	genesisBlock.Hash = genesisBlock.CalculateHash()

	return genesisBlock
}

// addBlockV2 adds a block atomically with fork detection and reorganization
func (bc *BlockchainV2) addBlockV2(block *Block) error {
	// CRITICAL: Hold lock only for in-memory operations
	// Storage operations happen AFTER lock release to prevent blocking read operations
	bc.mu.Lock()

	// Check if block already exists (duplicate)
	blockHashKey := hex.EncodeToString(block.Hash[:])
	if existingBlock := bc.blockIndex[blockHashKey]; existingBlock != nil {
		LogWarn("Block #%d already exists: %x", block.Header.Number, block.Hash)
		bc.mu.Unlock()
		return fmt.Errorf("block already exists")
	}

	// Fork detection: Check BEFORE validation if this could be a fork
	isFork := false
	var parentBlock *Block = nil
	var parentHash Hash

	// Determine parent hash first (without storage access)
	if bc.bestBlock != nil {
		if block.Header.Number == bc.bestBlock.Header.Number {
			// Same height but different hash = fork
			if block.Hash != bc.bestBlock.Hash {
				isFork = true
				LogInfo("Fork detected! Block #%d: current=%x, new=%x",
					block.Header.Number, bc.bestBlock.Hash, block.Hash)
				// For fork at same height, parent is the same as bestBlock's parent
				if bc.bestBlock.Header.Number > 0 {
					parentHash = bc.bestBlock.Header.ParentHash
				}
			}
		} else if block.Header.ParentHash == bc.bestBlock.Hash {
			// Normal case: block extends bestBlock
			isFork = false
			parentBlock = bc.bestBlock
		} else if block.Header.ParentHash != bc.bestBlock.Hash {
			// Different parent - could be a fork
			parentHash = block.Header.ParentHash
		}
	}

	// If we need to fetch parent from storage, do it OUTSIDE the lock
	if parentBlock == nil && parentHash != (Hash{}) {
		// Try to get from index first (fast, no I/O)
		parentBlock = bc.getBlockByHash(parentHash)

		// If not in index, release lock and fetch from storage
		if parentBlock == nil && bc.storage != nil {
			bc.mu.Unlock()
			parentBlock, _ = bc.storage.GetBlockByHash(parentHash[:])
			bc.mu.Lock()

			// Re-check if block was added while we were fetching (race condition)
			blockHashKey := hex.EncodeToString(block.Hash[:])
			if existingBlock := bc.blockIndex[blockHashKey]; existingBlock != nil {
				bc.mu.Unlock()
				return fmt.Errorf("block already exists")
			}
		}

		if parentBlock != nil {
			// Parent exists - check if it's a fork
			parentHashKey := hex.EncodeToString(parentHash[:])
			if forkBlocks, exists := bc.forkBlocks[parentHashKey]; exists && len(forkBlocks) > 0 {
				// Parent is a fork block
				isFork = true
				LogInfo("Fork detected! Block #%d extends fork chain", block.Header.Number)
			} else if parentBlock.Hash != bc.bestBlock.Hash {
				// Parent is different from bestBlock - this is a fork
				isFork = true
				LogInfo("Fork detected! Block #%d has different parent: %x (bestBlock: %x)",
					block.Header.Number, parentHash, bc.bestBlock.Hash)
			}
		} else {
			// Parent not found - this is invalid (orphan block)
			bc.mu.Unlock()
			return fmt.Errorf("parent block not found: %x", parentHash)
		}
	}

	// Validate block (with parent block if fork detected)
	if err := bc.validateBlockV2WithParent(block, parentBlock); err != nil {
		bc.mu.Unlock()
		return fmt.Errorf("block validation failed: %v", err)
	}

	// Store block in index
	bc.blockIndex[blockHashKey] = block

	// If fork detected, store in fork blocks and check chain lengths
	if isFork {
		parentHashKey := hex.EncodeToString(block.Header.ParentHash[:])
		bc.forkBlocks[parentHashKey] = append(bc.forkBlocks[parentHashKey], block)

		// Calculate chain lengths
		currentChainLength := bc.calculateChainLength(bc.bestBlock)
		newChainLength := bc.calculateChainLength(block)

		LogDebug("Chain lengths - Current: %d, New: %d", currentChainLength, newChainLength)

		// Longest chain rule: if new chain is longer, reorganize
		if newChainLength > currentChainLength {
			LogInfo("Reorganizing chain: new chain is longer (%d > %d)",
				newChainLength, currentChainLength)

			// Release lock before reorganization (it will re-acquire)
			bc.mu.Unlock()

			// Perform reorganization
			if err := bc.reorganizeChain(block); err != nil {
				return fmt.Errorf("reorganization failed: %v", err)
			}

			// Reorganization successful - block is now part of best chain
			// Block is already added in reorganizeChain, so we're done

			// Save to persistent storage
			if bc.storage != nil {
				if err := bc.storage.StoreBlock(block); err != nil {
					LogWarn("Failed to save block to storage: %v", err)
				} else {
					LogDebug("Block #%d saved to storage", block.Header.Number)
				}
			}

			return nil
		} else {
			// New chain is shorter or equal - keep current chain
			LogDebug("Keeping current chain (length %d >= %d)",
				currentChainLength, newChainLength)
			bc.mu.Unlock()
			return nil
		}
	}

	// Normal case: add block to chain
	// Process UTXOs for all transactions in the block
	for _, tx := range block.Txs {
		bc.processTransactionUTXOs(&tx, block.Hash)
		// Remove from mempool if it exists
		bc.mempool.RemoveTransaction(tx.Hash)
	}

	// Add block atomically to in-memory structures
	bc.blocks = append(bc.blocks, block)
	bc.height = block.Header.Number
	bc.bestBlock = block

	// CRITICAL: Update block history OUTSIDE main lock (uses separate lock)
	// This prevents blocking createBlockTemplate
	bc.blockHistory.AddBlock(block.Header.Timestamp)

	// Update state
	bc.stateManager.SetState("height", bc.height)
	bc.stateManager.SetState("bestBlock", block.Hash)

	// Emit event
	bc.eventBus.Emit("blockAdded", map[string]interface{}{
		"block":  block,
		"height": bc.height,
	})

	// CRITICAL: Release lock BEFORE slow storage operations
	// This allows createBlockTemplate and other read operations to proceed immediately
	bc.mu.Unlock()

	// Save to persistent storage AFTER lock release
	// This I/O operation can take 100-500ms and should not block read operations
	if bc.storage != nil {
		if err := bc.storage.StoreBlock(block); err != nil {
			LogWarn("Failed to save block to storage: %v", err)
		} else {
			LogDebug("Block #%d saved to storage", block.Header.Number)
		}
	}

	LogInfo("Block #%d added successfully: %x", block.Header.Number, block.Hash)

	return nil
}

// CreateTransaction creates a transaction from UTXOs
// Note: Transaction must be signed separately using crypto.SignTransaction()
func (bc *BlockchainV2) CreateTransaction(from Address, to Address, amount uint64, fee uint64) (*Transaction, error) {
	// Get UTXOs for sender
	utxos := bc.utxoSet.GetUTXOs(from)

	// Calculate total available balance
	totalBalance := uint64(0)
	for _, utxo := range utxos {
		totalBalance += utxo.Amount
	}

	if totalBalance < amount+fee {
		return nil, fmt.Errorf("insufficient balance: need %d, have %d", amount+fee, totalBalance)
	}

	// Create inputs
	inputs := []TxInput{}
	totalInput := uint64(0)
	for _, utxo := range utxos {
		if totalInput >= amount+fee {
			break
		}
		inputs = append(inputs, TxInput{
			PreviousTxHash: utxo.TxHash,
			Index:          utxo.Index,
			Signature:      []byte{}, // Signature will be added using crypto.SignTransaction()
		})
		totalInput += utxo.Amount
	}

	// Create outputs
	outputs := []TxOutput{
		{Address: to, Amount: amount},
	}

	// Add change output if needed
	change := totalInput - amount - fee
	if change > 0 {
		outputs = append(outputs, TxOutput{Address: from, Amount: change})
	}

	// Create transaction
	tx := &Transaction{
		From:      from,
		To:        to,
		Amount:    amount,
		Fee:       fee,
		Timestamp: time.Now(),
		Inputs:    inputs,
		Outputs:   outputs,
		Signature: []byte{}, // Will be set when signed
	}

	// Calculate hash
	tx.Hash = tx.CalculateHash()

	return tx, nil
}

// processTransactionUTXOs processes UTXOs for a transaction
func (bc *BlockchainV2) processTransactionUTXOs(tx *Transaction, blockHash Hash) {
	// Mark input UTXOs as spent
	for _, input := range tx.Inputs {
		bc.utxoSet.SpendUTXO(input.PreviousTxHash, input.Index)
	}

	// Create new UTXOs for outputs
	for i, output := range tx.Outputs {
		bc.utxoSet.AddUTXO(tx.Hash, uint32(i), output.Amount, output.Address, blockHash)
		LogDebug("UTXO created - Address: %s, Amount: %d, TxHash: %x", hex.EncodeToString(output.Address[:]), output.Amount, tx.Hash)
	}
}

// AddBlockV2 is the main function for adding blocks - ensures UTXO processing
func (bc *BlockchainV2) AddBlockV2(block *Block) error {
	return bc.addBlockV2(block)
}

// GetBalance returns the balance for an address
func (bc *BlockchainV2) GetBalance(address Address) uint64 {
	return bc.utxoSet.GetBalance(address)
}

// GetUTXOs returns all UTXOs for an address
func (bc *BlockchainV2) GetUTXOs(address Address) []*UTXO {
	return bc.utxoSet.GetUTXOs(address)
}

// GetMempool returns the mempool
func (bc *BlockchainV2) GetMempool() *Mempool {
	return bc.mempool
}

// validateBlockV2 validates a block professionally (uses bestBlock as parent)
func (bc *BlockchainV2) validateBlockV2(block *Block) error {
	return bc.validateBlockV2WithParent(block, bc.bestBlock)
}

// validateBlockV2WithParent validates a block with a specific parent block
func (bc *BlockchainV2) validateBlockV2WithParent(block *Block, parent *Block) error {
	// Check if it's genesis block
	if block.Header.Number == 0 {
		// For genesis block, validate merkle root if there are transactions
		if len(block.Txs) > 0 {
			consensusManager := NewConsensusManager(bc.genesis)
			expectedMerkleRoot := consensusManager.CalculateMerkleRoot(block.Txs)
			if block.Header.MerkleRoot != expectedMerkleRoot {
				return fmt.Errorf("invalid merkle root in genesis block")
			}
		}
		return nil
	}

	// Get parent block
	if parent == nil {
		// Try to get parent from block index or storage
		parentHash := block.Header.ParentHash
		parent = bc.getBlockByHash(parentHash)
		if parent == nil && bc.storage != nil {
			parent, _ = bc.storage.GetBlockByHash(parentHash[:])
		}
		if parent == nil {
			return fmt.Errorf("no parent block found: %x", parentHash)
		}
	}

	// Validate parent hash
	if block.Header.ParentHash != parent.Hash {
		return fmt.Errorf("invalid parent hash: expected %x, got %x", parent.Hash, block.Header.ParentHash)
	}

	// Validate block number
	if block.Header.Number != parent.Header.Number+1 {
		return fmt.Errorf("invalid block number: expected %d, got %d", parent.Header.Number+1, block.Header.Number)
	}

	// Validate timestamp
	if block.Header.Timestamp.Before(parent.Header.Timestamp) {
		return fmt.Errorf("block timestamp before parent: %v < %v", block.Header.Timestamp, parent.Header.Timestamp)
	}

	// Validate difficulty
	consensusManager := NewConsensusManager(bc.genesis)
	// For validation, we don't have full block history, so pass empty slice
	// The difficulty is already set in the block, we just validate it matches expected
	expectedDifficulty := consensusManager.CalculateDifficulty(block.Header.Number, parent, []time.Time{})
	if block.Header.Difficulty != expectedDifficulty {
		return fmt.Errorf("invalid difficulty: expected %d, got %d", expectedDifficulty, block.Header.Difficulty)
	}

	// Validate merkle root
	expectedMerkleRoot := consensusManager.CalculateMerkleRoot(block.Txs)
	LogDebug("validateBlockV2WithParent: Block #%d - Expected merkle root: %x, Got: %x, Tx count: %d", block.Header.Number, expectedMerkleRoot, block.Header.MerkleRoot, len(block.Txs))
	for i, tx := range block.Txs {
		LogDebug("validateBlockV2WithParent: Block #%d - Tx[%d] hash: %x (provided: %v)", block.Header.Number, i, tx.Hash, tx.Hash != (Hash{}))
	}
	if block.Header.MerkleRoot != expectedMerkleRoot {
		return fmt.Errorf("invalid merkle root: expected %x, got %x", expectedMerkleRoot, block.Header.MerkleRoot)
	}

	// Validate transaction count
	if block.Header.TxCount != uint32(len(block.Txs)) {
		return fmt.Errorf("invalid transaction count: expected %d, got %d", len(block.Txs), block.Header.TxCount)
	}

	// CRITICAL: Validate all transactions in the block (including signatures)
	// This ensures that only valid, signed transactions are included in blocks
	for i, tx := range block.Txs {
		if err := consensusManager.ValidateTransaction(&tx); err != nil {
			return fmt.Errorf("invalid transaction %d: %v", i, err)
		}
	}

	// Validate proof of work
	if !bc.consensus.ValidateProofOfWorkV2(block) {
		return fmt.Errorf("invalid proof of work")
	}

	return nil
}

// GetBestBlock returns the best block thread-safely
func (bc *BlockchainV2) GetBestBlock() *Block {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.bestBlock
}

// getBlockByHash returns a block by hash from the block index
func (bc *BlockchainV2) getBlockByHash(hash Hash) *Block {
	hashKey := hex.EncodeToString(hash[:])
	return bc.blockIndex[hashKey]
}

// calculateChainLength calculates the chain length from a block to genesis
// NOTE: This function should NOT access storage if called within a lock
// It only uses in-memory block index to avoid lock contention
func (bc *BlockchainV2) calculateChainLength(block *Block) uint64 {
	if block == nil {
		return 0
	}

	length := uint64(1) // Count this block
	current := block

	// Traverse back to genesis using only in-memory index
	// We avoid storage access to prevent lock contention
	for current.Header.Number > 0 {
		parentHash := current.Header.ParentHash
		parent := bc.getBlockByHash(parentHash)
		if parent == nil {
			// Block not in index - cannot continue without storage access
			// This is acceptable for fork detection as we only need approximate length
			break
		}
		length++
		current = parent
	}

	return length
}

// findCommonParent finds the common parent block between two chains
// NOTE: This function should NOT access storage if called within a lock
// It only uses in-memory block index to avoid lock contention
func (bc *BlockchainV2) findCommonParent(block1, block2 *Block) *Block {
	if block1 == nil || block2 == nil {
		return nil
	}

	// Build chain hashes for block1 (using only in-memory index)
	chain1Hashes := make(map[string]bool)
	current := block1
	for current != nil {
		hashKey := hex.EncodeToString(current.Hash[:])
		chain1Hashes[hashKey] = true

		if current.Header.Number == 0 {
			break
		}
		parentHash := current.Header.ParentHash
		current = bc.getBlockByHash(parentHash)
		// Avoid storage access to prevent lock contention
		if current == nil {
			break
		}
	}

	// Traverse block2 chain to find common parent (using only in-memory index)
	current = block2
	for current != nil {
		hashKey := hex.EncodeToString(current.Hash[:])
		if chain1Hashes[hashKey] {
			return current
		}

		if current.Header.Number == 0 {
			break
		}
		parentHash := current.Header.ParentHash
		current = bc.getBlockByHash(parentHash)
		// Avoid storage access to prevent lock contention
		if current == nil {
			break
		}
	}

	return nil
}

// reorganizeChain performs chain reorganization when a longer fork is detected
func (bc *BlockchainV2) reorganizeChain(newBestBlock *Block) error {
	bc.mu.Lock()

	if bc.bestBlock == nil {
		bc.mu.Unlock()
		return fmt.Errorf("no current best block to reorganize from")
	}

	LogInfo("Starting chain reorganization...")
	LogInfo("   Current best: Block #%d (%x)", bc.bestBlock.Header.Number, bc.bestBlock.Hash)
	LogInfo("   New best: Block #%d (%x)", newBestBlock.Header.Number, newBestBlock.Hash)

	// Find common parent
	commonParent := bc.findCommonParent(bc.bestBlock, newBestBlock)
	if commonParent == nil {
		bc.mu.Unlock()
		return fmt.Errorf("no common parent found between chains")
	}

	LogInfo("   Common parent: Block #%d (%x)", commonParent.Header.Number, commonParent.Hash)

	// Build list of blocks to remove (from current chain, after common parent)
	// Use only in-memory index to avoid lock contention
	blocksToRemove := make([]*Block, 0)
	current := bc.bestBlock
	for current != nil && current.Hash != commonParent.Hash {
		blocksToRemove = append(blocksToRemove, current)
		if current.Header.Number == 0 {
			break
		}
		parentHash := current.Header.ParentHash
		current = bc.getBlockByHash(parentHash)
		// Avoid storage access to prevent lock contention
		if current == nil {
			break
		}
	}

	// Build list of blocks to add (from new chain, after common parent)
	// Use only in-memory index to avoid lock contention
	blocksToAdd := make([]*Block, 0)
	current = newBestBlock
	for current != nil && current.Hash != commonParent.Hash {
		blocksToAdd = append(blocksToAdd, current)
		if current.Header.Number == 0 {
			break
		}
		parentHash := current.Header.ParentHash
		current = bc.getBlockByHash(parentHash)
		// Avoid storage access to prevent lock contention
		if current == nil {
			break
		}
	}

	// Reverse blocksToAdd to get correct order (from common parent to new best)
	for i, j := 0, len(blocksToAdd)-1; i < j; i, j = i+1, j-1 {
		blocksToAdd[i], blocksToAdd[j] = blocksToAdd[j], blocksToAdd[i]
	}

	LogInfo("   Blocks to remove: %d", len(blocksToRemove))
	LogInfo("   Blocks to add: %d", len(blocksToAdd))

	// Step 1: Rollback UTXOs from removed blocks (in reverse order)
	for i := len(blocksToRemove) - 1; i >= 0; i-- {
		block := blocksToRemove[i]
		LogDebug("   Rolling back Block #%d (%x)", block.Header.Number, block.Hash)

		// Remove UTXOs created by this block
		bc.utxoSet.RemoveUTXOs(block.Hash)

		// Re-add UTXOs that were spent by this block (rollback spending)
		// Note: UTXO rollback is simplified - in a full implementation, we'd need to track
		// which UTXOs were spent and restore them. For now, we only remove UTXOs created by this block.
	}

	// Step 2: Remove blocks from blocks array and update height
	// Find the index of common parent in blocks array
	commonParentIndex := -1
	for i, block := range bc.blocks {
		if block.Hash == commonParent.Hash {
			commonParentIndex = i
			break
		}
	}

	if commonParentIndex >= 0 {
		// Remove blocks after common parent
		bc.blocks = bc.blocks[:commonParentIndex+1]
		bc.height = commonParent.Header.Number
	}

	// Step 3: Add new blocks and process UTXOs
	for _, block := range blocksToAdd {
		LogDebug("   Adding Block #%d (%x)", block.Header.Number, block.Hash)

		// Process UTXOs for all transactions in the block
		for _, tx := range block.Txs {
			bc.processTransactionUTXOs(&tx, block.Hash)
			// Remove from mempool if it exists
			bc.mempool.RemoveTransaction(tx.Hash)
		}

		// Add block to chain
		bc.blocks = append(bc.blocks, block)
		bc.height = block.Header.Number
		bc.bestBlock = block

		// Update block index
		blockHashKey := hex.EncodeToString(block.Hash[:])
		bc.blockIndex[blockHashKey] = block
	}

	// Update state
	bc.stateManager.SetState("height", bc.height)
	bc.stateManager.SetState("bestBlock", bc.bestBlock.Hash)

	// CRITICAL: Rebuild block history after reorganization (uses separate lock)
	// This prevents blocking createBlockTemplate
	bc.blockHistory.Rebuild(bc.blocks)

	// Emit reorganization event
	bc.eventBus.Emit("chainReorganized", map[string]interface{}{
		"oldBest": bc.bestBlock,
		"newBest": newBestBlock,
		"removed": len(blocksToRemove),
		"added":   len(blocksToAdd),
	})

	LogInfo("Chain reorganization completed")
	LogInfo("   New best: Block #%d (%x)", bc.bestBlock.Header.Number, bc.bestBlock.Hash)

	// Release lock before storage operations to prevent lock contention
	bc.mu.Unlock()

	// Save to persistent storage AFTER lock release
	if bc.storage != nil {
		// Save all new blocks
		for _, block := range blocksToAdd {
			if err := bc.storage.StoreBlock(block); err != nil {
				LogWarn("Failed to save block #%d to storage: %v", block.Header.Number, err)
			}
		}
	}

	return nil
}

// GetRecentBlocks returns the most recent blocks
func (bc *BlockchainV2) GetRecentBlocks(limit int) []*Block {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	if len(bc.blocks) == 0 {
		return []*Block{}
	}

	// Get the last 'limit' blocks
	start := len(bc.blocks) - limit
	if start < 0 {
		start = 0
	}

	// Create a slice in reverse order (newest first)
	result := make([]*Block, 0, limit)
	for i := len(bc.blocks) - 1; i >= start; i-- {
		result = append(result, bc.blocks[i])
		if len(result) >= limit {
			break
		}
	}

	return result
}

// GetBlockByNumber returns a block by its number
func (bc *BlockchainV2) GetBlockByNumber(number uint64) (*Block, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	// Check if block is in memory
	if number < uint64(len(bc.blocks)) {
		return bc.blocks[number], nil
	}

	// Try to get from storage
	if bc.storage != nil {
		return bc.storage.GetBlockByNumber(number)
	}

	return nil, fmt.Errorf("block %d not found", number)
}

// GetBlockByHash returns a block by its hash
func (bc *BlockchainV2) GetBlockByHash(hash []byte) (*Block, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	// Convert hash to Hash type
	var hashValue Hash
	if len(hash) != 32 {
		return nil, fmt.Errorf("invalid hash length: expected 32 bytes, got %d", len(hash))
	}
	copy(hashValue[:], hash)

	// Check if block is in memory (block index)
	if block := bc.getBlockByHash(hashValue); block != nil {
		return block, nil
	}

	// Try to get from storage
	if bc.storage != nil {
		return bc.storage.GetBlockByHash(hash)
	}

	return nil, fmt.Errorf("block with hash %x not found", hash)
}

// GetTotalTransactions returns the total number of transactions in all blocks
func (bc *BlockchainV2) GetTotalTransactions() uint64 {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	total := uint64(0)
	for _, block := range bc.blocks {
		total += uint64(len(block.Txs))
	}
	return total
}

// GetAddressCount returns the number of unique addresses with UTXOs
func (bc *BlockchainV2) GetAddressCount() uint64 {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	addresses := make(map[Address]bool)
	for _, block := range bc.blocks {
		for _, tx := range block.Txs {
			for _, output := range tx.Outputs {
				addresses[output.Address] = true
			}
		}
	}
	return uint64(len(addresses))
}

// GetTreasuryBalance returns the balance of the treasury address
func (bc *BlockchainV2) GetTreasuryBalance() uint64 {
	if bc.genesis.TreasuryAddress == "" {
		return 0
	}
	treasuryAddr := AddressFromString(bc.genesis.TreasuryAddress)
	return bc.GetBalance(treasuryAddr)
}

// GetAddressTransactions returns all transactions for a given address
func (bc *BlockchainV2) GetAddressTransactions(address Address) []*Transaction {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	var transactions []*Transaction
	for _, block := range bc.blocks {
		for i := range block.Txs {
			tx := &block.Txs[i]
			// Check if address is involved in transaction (from, to, or in outputs)
			if tx.From == address || tx.To == address {
				transactions = append(transactions, tx)
			} else {
				// Check outputs
				for _, output := range tx.Outputs {
					if output.Address == address {
						transactions = append(transactions, tx)
						break
					}
				}
			}
		}
	}
	return transactions
}

// GetHeight returns the current height thread-safely
func (bc *BlockchainV2) GetHeight() uint64 {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.height
}

// GetBlockHistoryForDifficulty returns block history for LWMA difficulty adjustment
// This is a public method that uses the separate blockHistory lock (not main lock)
func (bc *BlockchainV2) GetBlockHistoryForDifficulty(blockNumber uint64) []time.Time {
	windowSize := int(bc.genesis.Difficulty.Window)
	if windowSize == 0 {
		windowSize = 120 // Default window size
	}
	return bc.blockHistory.GetWindow(windowSize)
}

// GetConsensus returns the consensus mechanism
func (bc *BlockchainV2) GetConsensus() *ConsensusV2 {
	return bc.consensus
}

// GetGenesis returns the genesis configuration
func (bc *BlockchainV2) GetGenesis() *GenesisConfig {
	return bc.genesis
}

// GetEventBus returns the event bus
func (bc *BlockchainV2) GetEventBus() *EventBus {
	return bc.eventBus
}

// NewMempool creates a new mempool
func NewMempool() *Mempool {
	return &Mempool{
		transactions: make(map[string]*Transaction),
	}
}

// AddTransaction adds a transaction to the mempool
func (m *Mempool) AddTransaction(tx *Transaction) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.transactions[hex.EncodeToString(tx.Hash[:])] = tx
	LogDebug("Transaction added to mempool: %x", tx.Hash)
}

// GetPendingTransactions returns all pending transactions
func (m *Mempool) GetPendingTransactions() []*Transaction {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var txs []*Transaction
	for _, tx := range m.transactions {
		txs = append(txs, tx)
	}
	return txs
}

// RemoveTransaction removes a transaction from the mempool
func (m *Mempool) RemoveTransaction(txHash Hash) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.transactions, hex.EncodeToString(txHash[:]))
}

// Clear removes all transactions from the mempool
func (m *Mempool) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.transactions = make(map[string]*Transaction)
}

// CreateNewBlockV2 creates a new block template professionally
func (bc *BlockchainV2) CreateNewBlockV2(miner Address, txs []Transaction) *Block {
	bc.mu.RLock()
	parent := bc.bestBlock
	bc.mu.RUnlock()

	if parent == nil {
		return nil
	}

	// Get block history for LWMA difficulty adjustment (uses separate lock, no main lock needed)
	windowSize := int(bc.genesis.Difficulty.Window)
	if windowSize == 0 {
		windowSize = 120 // Default window size
	}
	blockHistory := bc.blockHistory.GetWindow(windowSize)

	// Calculate difficulty using ConsensusManager with LWMA (uses Genesis config)
	consensusManager := NewConsensusManager(bc.genesis)
	difficulty := consensusManager.CalculateDifficulty(parent.Header.Number+1, parent, blockHistory)

	// Calculate block reward distribution (Miner + Treasury)
	baseReward := bc.genesis.GetCurrentReward(parent.Header.Number + 1)

	// Calculate transaction fees from pending transactions
	txFees := uint64(0)
	for _, tx := range txs {
		txFees += tx.Fee
	}

	// Calculate network fees (Miner + Treasury distribution)
	blockRewardDist := bc.genesis.CalculateNetworkFees(baseReward, txFees)

	// Create block reward transaction for miner
	minerRewardTx := bc.createBlockRewardTransaction(miner, blockRewardDist.MinerReward)

	// Create treasury reward transaction (if treasury address is configured and reward > 0)
	var treasuryRewardTx *Transaction
	if blockRewardDist.TreasuryReward > 0 && bc.genesis.TreasuryAddress != "" {
		treasuryAddr := AddressFromString(bc.genesis.TreasuryAddress)
		treasuryRewardTx = bc.createBlockRewardTransactionPtr(treasuryAddr, blockRewardDist.TreasuryReward)
	}

	// Get pending transactions from mempool
	pendingTxs := bc.mempool.GetPendingTransactions()
	for _, tx := range pendingTxs {
		txs = append(txs, *tx)
	}

	// Add reward transactions to the beginning of transactions
	allTxs := []Transaction{minerRewardTx}
	if treasuryRewardTx != nil {
		allTxs = append(allTxs, *treasuryRewardTx)
	}
	allTxs = append(allTxs, txs...)

	// Calculate merkle root from all transactions
	merkleRoot := consensusManager.CalculateMerkleRoot(allTxs)

	// Create block template
	block := &Block{
		Header: BlockHeader{
			ParentHash:  parent.Hash, // CRITICAL: Use actual parent hash
			Number:      parent.Header.Number + 1,
			Timestamp:   time.Now(),
			Difficulty:  difficulty,
			Miner:       miner,
			Nonce:       0,
			MerkleRoot:  merkleRoot, // Calculate merkle root from transactions
			TxCount:     uint32(len(allTxs)),
			NetworkFee:  txFees,
			TreasuryFee: blockRewardDist.TreasuryReward,
		},
		Txs:  allTxs,
		Hash: Hash{},
	}

	// Calculate hash
	block.Hash = block.CalculateHash()

	return block
}

// calculateBlockReward calculates the block reward for a given block number
func (bc *BlockchainV2) calculateBlockReward(blockNumber uint64) uint64 {
	// Start with initial block reward (5 tKALON = 5,000,000 units)
	reward := uint64(bc.genesis.InitialBlockReward * 1000000) // Convert to smallest units

	// Apply halving schedule
	for _, halving := range bc.genesis.HalvingSchedule {
		if blockNumber > halving.AfterBlocks {
			reward = uint64(float64(reward) * halving.RewardMultiplier)
		}
	}

	return reward
}

// createBlockRewardTransactionPtr creates a block reward transaction and returns a pointer
func (bc *BlockchainV2) createBlockRewardTransactionPtr(recipient Address, amount uint64) *Transaction {
	tx := bc.createBlockRewardTransaction(recipient, amount)
	return &tx
}

// createBlockRewardTransaction creates a block reward transaction
func (bc *BlockchainV2) createBlockRewardTransaction(miner Address, amount uint64) Transaction {
	LogDebug("createBlockRewardTransaction - Miner address: %x, Amount: %d", miner, amount)

	// Create a special coinbase transaction (no inputs, only output)
	tx := Transaction{
		From:      Address{}, // Empty for coinbase
		To:        miner,
		Amount:    amount,
		Nonce:     0,
		Fee:       0,
		GasUsed:   0,
		GasPrice:  0,
		Data:      []byte("block_reward"),
		Signature: []byte{},    // No signature needed for coinbase
		Inputs:    []TxInput{}, // No inputs for coinbase
		Outputs: []TxOutput{
			{
				Address: miner,
				Amount:  amount,
			},
		},
		Timestamp: time.Now(),
	}

	LogDebug("createBlockRewardTransaction - Created TX with output address: %x", tx.Outputs[0].Address)

	// CRITICAL: Use tx.CalculateHash() for consistency with CalculateMerkleRoot
	// This ensures the hash matches when validating the merkle root
	tx.Hash = tx.CalculateHash()

	return tx
}

// AddBlock adds a block to the blockchain
func (bc *BlockchainV2) AddBlock(block *Block) error {
	return bc.addBlockV2(block)
}

// loadChainFromStorage loads the blockchain from persistent storage
func (bc *BlockchainV2) loadChainFromStorage() {
	LogDebug("loadChainFromStorage called")
	if bc.storage == nil {
		LogWarn("Storage is nil, skipping load")
		return
	}

	LogDebug("Getting best block from storage")
	// Get the best block
	bestBlock, err := bc.storage.GetBestBlock()
	if err != nil {
		LogWarn("No existing chain found, starting fresh (error: %v)", err)
		return
	}

	if bestBlock == nil {
		LogWarn("Best block is nil, starting fresh")
		return
	}

	LogDebug("Best block found - Height: %d, Hash: %x", bestBlock.Header.Number, bestBlock.Hash)

	// Reconstruct chain by loading blocks from storage
	bc.height = bestBlock.Header.Number
	bc.bestBlock = bestBlock

	LogDebug("Loading %d blocks from storage", bc.height+1)

	// Load all blocks from genesis to best block
	for i := uint64(0); i <= bc.height; i++ {
		// Progress logging every 10 blocks
		if i%10 == 0 || i == bc.height {
			LogDebug("Loading block %d/%d from storage...", i, bc.height)
		}

		block, err := bc.storage.GetBlockByNumber(i)
		if err != nil || block == nil {
			LogWarn("Failed to load block %d: %v", i, err)
			// Reset and start fresh
			bc.height = 0
			bc.bestBlock = nil
			bc.blocks = make([]*Block, 0)
			return
		}
		bc.blocks = append(bc.blocks, block)

		// Add block to index
		blockHashKey := hex.EncodeToString(block.Hash[:])
		bc.blockIndex[blockHashKey] = block

		// IMPORTANT: Reconstruct UTXOs for each block
		// This is critical because UTXOs are in-memory and need to be rebuilt
		for _, tx := range block.Txs {
			bc.processTransactionUTXOs(&tx, block.Hash)
		}
	}

	LogInfo("Loaded blockchain from storage - Height: %d, UTXOs restored", bc.height)
}

// Close closes the blockchain and its storage
func (bc *BlockchainV2) Close() error {
	if bc.storage != nil {
		return bc.storage.Close()
	}
	return nil
}

// ValidateProofOfWorkV2 validates proof of work professionally
func (c *ConsensusV2) ValidateProofOfWorkV2(block *Block) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// PoW is ALWAYS active for fairness - all blocks must have valid proof of work
	// This ensures fair mining competition regardless of difficulty level

	// Calculate target (simplified for testnet)
	target := uint64(1) << (64 - block.Header.Difficulty)

	// Check if hash meets target
	hashInt := binary.BigEndian.Uint64(block.Hash[:8])
	return hashInt < target
}

// CalculateDifficultyV2 calculates difficulty professionally
func (c *ConsensusV2) CalculateDifficultyV2(blockNumber uint64, parent *Block) uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Use parent difficulty if available
	if parent != nil {
		return parent.Header.Difficulty
	}

	// Default difficulty for genesis or fallback (should match testnet config)
	return 10
}

// CalculateDifficulty calculates difficulty using LWMA
func (da *DifficultyAdjustment) CalculateDifficulty(blockNumber uint64, parent *Block) uint64 {
	da.mu.Lock()
	defer da.mu.Unlock()

	// Add current block time
	da.blockTimes = append(da.blockTimes, parent.Header.Timestamp)

	// Keep only window size
	if len(da.blockTimes) > da.windowSize {
		da.blockTimes = da.blockTimes[1:]
	}

	// Need at least 2 blocks for adjustment
	if len(da.blockTimes) < 2 {
		return 4
	}

	// Calculate average block time
	totalTime := da.blockTimes[len(da.blockTimes)-1].Sub(da.blockTimes[0])
	avgBlockTime := totalTime / time.Duration(len(da.blockTimes)-1)

	// Target block time
	targetTime := 15 * time.Second

	// Calculate adjustment factor
	adjustmentFactor := float64(targetTime) / float64(avgBlockTime)

	// Apply adjustment
	newDifficulty := uint64(float64(parent.Header.Difficulty) * adjustmentFactor)

	// Clamp difficulty
	if newDifficulty < 1 {
		newDifficulty = 1
	}
	if newDifficulty > 1000 {
		newDifficulty = 1000
	}

	return newDifficulty
}

// Emit emits an event
func (eb *EventBus) Emit(event string, data interface{}) {
	eb.mu.RLock()
	channels := eb.channels[event]
	eb.mu.RUnlock()

	for _, ch := range channels {
		select {
		case ch <- data:
		default:
			// Channel is full, skip
		}
	}
}

// Subscribe subscribes to an event
func (eb *EventBus) Subscribe(event string) <-chan interface{} {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	ch := make(chan interface{}, 100) // Buffered channel
	eb.channels[event] = append(eb.channels[event], ch)

	return ch
}

// SetState sets a state value
func (sm *StateManager) SetState(key string, value interface{}) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.state[key] = value
}

// GetState gets a state value
func (sm *StateManager) GetState(key string) interface{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.state[key]
}
