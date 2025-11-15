package mining

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/kalon-network/kalon/core"
)

// Miner represents the main mining coordinator
type Miner struct {
	randomXMiner *RandomXMiner
	blockchain   Blockchain
	wallet       Wallet
	running      bool
	mu           sync.RWMutex
	stats        *MinerStats
}

// Blockchain interface for mining
type Blockchain interface {
	GetBestBlock() *core.Block
	CreateNewBlock(miner core.Address, txs []core.Transaction) *core.Block
	AddBlock(block *core.Block) error
	GetConsensus() Consensus
}

// Consensus interface for mining
type Consensus interface {
	CalculateDifficulty(height uint64, parent *core.Block) uint64
	CalculateTarget(difficulty uint64) []byte
	ValidateBlock(block *core.Block, parent *core.Block) error
}

// Wallet interface for mining
type Wallet interface {
	GetAddress() core.Address
	SignTransaction(tx *core.Transaction) error
}

// MinerStats represents miner statistics
type MinerStats struct {
	StartTime       time.Time
	TotalHashes     uint64
	BlocksFound     uint64
	CurrentHashRate float64
	LastBlockTime   time.Time
	Difficulty      uint64
	Target          []byte
}

// NewMiner creates a new miner
func NewMiner(blockchain Blockchain, wallet Wallet, threads int) *Miner {
	randomXMiner := NewRandomXMiner(threads)

	return &Miner{
		randomXMiner: randomXMiner,
		blockchain:   blockchain,
		wallet:       wallet,
		stats: &MinerStats{
			StartTime: time.Now(),
		},
	}
}

// Start starts the miner
func (m *Miner) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("miner is already running")
	}

	// Start RandomX miner
	if err := m.randomXMiner.Start(); err != nil {
		return fmt.Errorf("failed to start RandomX miner: %v", err)
	}

	m.running = true

	// Start mining loop
	go m.miningLoop()

	// Start stats updater
	go m.updateStats()

	log.Printf("Miner started with %d threads", m.randomXMiner.threads)

	return nil
}

// Stop stops the miner
func (m *Miner) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	m.running = false
	m.randomXMiner.Stop()

	log.Println("Miner stopped")
}

// IsRunning returns true if the miner is running
func (m *Miner) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.running
}

// GetStats returns current miner statistics
func (m *Miner) GetStats() *MinerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Create a copy to avoid race conditions
	stats := *m.stats
	return &stats
}

// SetThreads sets the number of mining threads
func (m *Miner) SetThreads(threads int) {
	m.randomXMiner.SetThreads(threads)
}

// miningLoop is the main mining loop
func (m *Miner) miningLoop() {
	for {
		select {
		case <-time.After(1 * time.Second):
			if !m.IsRunning() {
				return
			}

			// Create new block to mine
			block := m.createMiningBlock()
			if block != nil {
				log.Printf("Mining block #%d...", block.Header.Number)
				m.mineBlock(block)
			}
		}
	}
}

// createMiningBlock creates a new block to mine
func (m *Miner) createMiningBlock() *MiningBlock {
	// Get best block
	bestBlock := m.blockchain.GetBestBlock()
	if bestBlock == nil {
		return nil
	}

	// Create new block
	newBlock := m.blockchain.CreateNewBlock(m.wallet.GetAddress(), []core.Transaction{})
	if newBlock == nil {
		return nil
	}

	// Calculate difficulty and target
	consensus := m.blockchain.GetConsensus()
	difficulty := consensus.CalculateDifficulty(newBlock.Header.Number, bestBlock)
	target := consensus.CalculateTarget(difficulty)

	// Update stats
	m.mu.Lock()
	m.stats.Difficulty = difficulty
	m.stats.Target = target
	m.mu.Unlock()

	// Create mining block
	miningBlock := &MiningBlock{
		Header:     newBlock.Header,
		Target:     target,
		StartNonce: 0,
		EndNonce:   ^uint64(0), // Max uint64
	}

	return miningBlock
}

// mineBlock starts mining a block
func (m *Miner) mineBlock(block *MiningBlock) {
	// Start mining the block
	m.randomXMiner.MineBlock(block)

	// Listen for results
	go m.handleMiningResults()
}

// handleMiningResults handles mining results
func (m *Miner) handleMiningResults() {
	for {
		select {
		case result := <-m.randomXMiner.GetHashResult():
			if result.Found {
				m.handleFoundBlock(result)
			} else if result.Error != nil {
				log.Printf("Mining error: %v", result.Error)
			}
		case <-time.After(1 * time.Second):
			// Check if still running
			if !m.IsRunning() {
				return
			}
		}
	}
}

// handleFoundBlock handles a found block
func (m *Miner) handleFoundBlock(result HashResult) {
	log.Printf("🎉 Block found! Hash: %x, Nonce: %d", result.Hash, result.Nonce)

	// Update stats
	m.mu.Lock()
	m.stats.BlocksFound++
	m.stats.LastBlockTime = time.Now()
	m.mu.Unlock()

	// Create block with found nonce
	bestBlock := m.blockchain.GetBestBlock()
	newBlock := m.blockchain.CreateNewBlock(m.wallet.GetAddress(), []core.Transaction{})
	if newBlock == nil {
		log.Println("Failed to create block")
		return
	}

	// Set the found nonce
	newBlock.Header.Nonce = result.Nonce
	newBlock.Hash = result.Hash

	// Validate block
	consensus := m.blockchain.GetConsensus()
	if err := consensus.ValidateBlock(newBlock, bestBlock); err != nil {
		log.Printf("Invalid block: %v", err)
		return
	}

	// Add block to blockchain
	if err := m.blockchain.AddBlock(newBlock); err != nil {
		log.Printf("Failed to add block: %v", err)
		return
	}

	log.Printf("✅ Block #%d added to blockchain: %x", newBlock.Header.Number, newBlock.Hash)
}

// updateStats updates miner statistics
func (m *Miner) updateStats() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if !m.IsRunning() {
				return
			}

			// Update hash rate
			randomXStats := m.randomXMiner.GetStats()

			m.mu.Lock()
			m.stats.TotalHashes = randomXStats.TotalHashes
			m.stats.CurrentHashRate = randomXStats.HashesPerSecond
			m.mu.Unlock()
		}
	}
}

// GetMiningInfo returns detailed mining information
func (m *Miner) GetMiningInfo() map[string]interface{} {
	stats := m.GetStats()
	randomXInfo := m.randomXMiner.GetMiningInfo()

	info := map[string]interface{}{
		"running":         m.IsRunning(),
		"startTime":       stats.StartTime,
		"totalHashes":     stats.TotalHashes,
		"blocksFound":     stats.BlocksFound,
		"currentHashRate": stats.CurrentHashRate,
		"lastBlockTime":   stats.LastBlockTime,
		"difficulty":      stats.Difficulty,
		"target":          fmt.Sprintf("%x", stats.Target),
	}

	// Add RandomX specific info
	for k, v := range randomXInfo {
		info["randomx_"+k] = v
	}

	return info
}

// GetHashRate returns the current hash rate
func (m *Miner) GetHashRate() float64 {
	stats := m.GetStats()
	return stats.CurrentHashRate
}

// GetBlocksFound returns the number of blocks found
func (m *Miner) GetBlocksFound() uint64 {
	stats := m.GetStats()
	return stats.BlocksFound
}

// GetTotalHashes returns the total number of hashes computed
func (m *Miner) GetTotalHashes() uint64 {
	stats := m.GetStats()
	return stats.TotalHashes
}

// GetDifficulty returns the current mining difficulty
func (m *Miner) GetDifficulty() uint64 {
	stats := m.GetStats()
	return stats.Difficulty
}

// GetTarget returns the current mining target
func (m *Miner) GetTarget() []byte {
	stats := m.GetStats()
	return stats.Target
}

// GetMiningStatsJSON returns mining stats as JSON
func (m *Miner) GetMiningStatsJSON() ([]byte, error) {
	info := m.GetMiningInfo()
	return json.MarshalIndent(info, "", "  ")
}

// SetMiningAddress sets the mining address (if wallet supports it)
func (m *Miner) SetMiningAddress(address [20]byte) error {
	// This would be implemented if the wallet supports address changes
	return fmt.Errorf("address change not supported")
}

// GetMiningAddress returns the current mining address
func (m *Miner) GetMiningAddress() [20]byte {
	return m.wallet.GetAddress()
}

// EstimateTimeToBlock estimates the time to find the next block
func (m *Miner) EstimateTimeToBlock() time.Duration {
	hashRate := m.GetHashRate()
	if hashRate == 0 {
		return 0
	}

	difficulty := m.GetDifficulty()
	if difficulty == 0 {
		return 0
	}

	// Estimate based on current hash rate and difficulty
	// This is a simplified calculation
	estimatedHashes := float64(difficulty) * 2 // Rough estimate
	estimatedSeconds := estimatedHashes / hashRate

	return time.Duration(estimatedSeconds) * time.Second
}
