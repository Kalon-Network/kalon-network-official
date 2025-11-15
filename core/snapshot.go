package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

// Snapshot represents a snapshot of all balances at a specific block height
type Snapshot struct {
	Height      uint64            `json:"height"`
	Timestamp   int64             `json:"timestamp"`
	BlockHash   string            `json:"blockHash"`
	Balances    map[string]uint64 `json:"balances"` // Key: address (hex), Value: balance
	TotalSupply uint64            `json:"totalSupply"`
	ChainID     uint64            `json:"chainId"`
}

// SnapshotManager manages snapshot creation and restoration
type SnapshotManager struct {
	mu       sync.RWMutex
	snapshot *Snapshot
}

// NewSnapshotManager creates a new snapshot manager
func NewSnapshotManager() *SnapshotManager {
	return &SnapshotManager{
		snapshot: nil,
	}
}

// CreateSnapshot creates a snapshot of all balances at current block height
func (sm *SnapshotManager) CreateSnapshot(bc *BlockchainV2) (*Snapshot, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Get current height and best block
	height := bc.GetHeight()
	bestBlock := bc.GetBestBlock()
	if bestBlock == nil {
		return nil, fmt.Errorf("no blocks found, cannot create snapshot")
	}

	log.Printf("📸 Creating snapshot at height %d, block hash: %x", height, bestBlock.Hash)

	// Get all addresses and their balances from UTXO set
	bc.mu.RLock()
	addressBalances := make(map[string]uint64)

	// Collect all unique addresses from all blocks
	allAddresses := make(map[Address]bool)
	for _, block := range bc.blocks {
		for _, tx := range block.Txs {
			for _, output := range tx.Outputs {
				allAddresses[output.Address] = true
			}
		}
	}

	// Calculate balances for all addresses using UTXO set
	for addr := range allAddresses {
		balance := bc.utxoSet.GetBalance(addr)
		if balance > 0 {
			addrHex := hex.EncodeToString(addr[:])
			addressBalances[addrHex] = balance
		}
	}
	bc.mu.RUnlock()

	// Calculate total supply
	var totalSupply uint64
	for _, balance := range addressBalances {
		totalSupply += balance
	}

	snapshot := &Snapshot{
		Height:      height,
		Timestamp:   time.Now().Unix(),
		BlockHash:   hex.EncodeToString(bestBlock.Hash[:]),
		Balances:    addressBalances,
		TotalSupply: totalSupply,
		ChainID:     bc.genesis.ChainID,
	}

	sm.snapshot = snapshot

	log.Printf("✅ Snapshot created: Height %d, %d addresses, Total Supply: %d",
		height, len(addressBalances), totalSupply)

	return snapshot, nil
}

// SaveSnapshot saves snapshot to a file
func (sm *SnapshotManager) SaveSnapshot(filename string) error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if sm.snapshot == nil {
		return fmt.Errorf("no snapshot available to save")
	}

	data, err := json.MarshalIndent(sm.snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %v", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write snapshot file: %v", err)
	}

	log.Printf("✅ Snapshot saved to %s", filename)
	return nil
}

// LoadSnapshot loads snapshot from a file
func (sm *SnapshotManager) LoadSnapshot(filename string) (*Snapshot, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read snapshot file: %v", err)
	}

	var snapshot Snapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, fmt.Errorf("failed to unmarshal snapshot: %v", err)
	}

	sm.mu.Lock()
	sm.snapshot = &snapshot
	sm.mu.Unlock()

	log.Printf("✅ Snapshot loaded from %s: Height %d, %d addresses, Total Supply: %d",
		filename, snapshot.Height, len(snapshot.Balances), snapshot.TotalSupply)

	return &snapshot, nil
}

// RestoreBalancesFromSnapshot restores all balances from snapshot to UTXO set
func (sm *SnapshotManager) RestoreBalancesFromSnapshot(bc *BlockchainV2) error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if sm.snapshot == nil {
		return fmt.Errorf("no snapshot available to restore")
	}

	log.Printf("🔄 Restoring balances from snapshot: Height %d, %d addresses, Total Supply: %d",
		sm.snapshot.Height, len(sm.snapshot.Balances), sm.snapshot.TotalSupply)

	bc.mu.Lock()
	defer bc.mu.Unlock()

	// Create a special transaction in genesis block for snapshot balances
	// We'll create one transaction per address with their balance
	genesisHash := Hash{} // Genesis block hash (empty)
	var txIndex uint32 = 0

	for addrHex, balance := range sm.snapshot.Balances {
		// Decode address
		addrBytes, err := hex.DecodeString(addrHex)
		if err != nil {
			log.Printf("⚠️ Failed to decode address %s: %v", addrHex, err)
			continue
		}

		if len(addrBytes) != 20 {
			log.Printf("⚠️ Invalid address length for %s: %d", addrHex, len(addrBytes))
			continue
		}

		var addr Address
		copy(addr[:], addrBytes)

		// Create a special UTXO for this balance
		// We use a special transaction hash that represents snapshot restoration
		txHash := createSnapshotTxHash(addrHex, balance)
		bc.utxoSet.AddUTXO(txHash, txIndex, balance, addr, genesisHash)
		txIndex++

		log.Printf("✅ Restored balance for %s: %d", addrHex, balance)
	}

	log.Printf("✅ Restored %d balances from snapshot", len(sm.snapshot.Balances))
	return nil
}

// GetSnapshot returns the current snapshot
func (sm *SnapshotManager) GetSnapshot() *Snapshot {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.snapshot
}

// createSnapshotTxHash creates a deterministic hash for snapshot transaction
func createSnapshotTxHash(address string, balance uint64) Hash {
	data := fmt.Sprintf("snapshot:%s:%d", address, balance)
	hash := sha256.Sum256([]byte(data))
	return Hash(hash)
}

// CreateSnapshotFromGenesis loads snapshot from genesis config
func (bc *BlockchainV2) CreateSnapshotFromGenesis() error {
	if bc.genesis.Snapshot == nil {
		return nil // No snapshot in genesis, skip
	}

	log.Printf("🔄 Restoring snapshot from genesis config: %d addresses", len(bc.genesis.Snapshot.Balances))

	bc.mu.Lock()
	defer bc.mu.Unlock()

	genesisHash := Hash{} // Genesis block hash (empty)
	var txIndex uint32 = 0

	for addrHex, balance := range bc.genesis.Snapshot.Balances {
		// Decode address
		addrBytes, err := hex.DecodeString(addrHex)
		if err != nil {
			log.Printf("⚠️ Failed to decode address %s: %v", addrHex, err)
			continue
		}

		if len(addrBytes) != 20 {
			log.Printf("⚠️ Invalid address length for %s: %d", addrHex, len(addrBytes))
			continue
		}

		var addr Address
		copy(addr[:], addrBytes)

		// Create a special UTXO for this balance
		txHash := createSnapshotTxHash(addrHex, balance)
		bc.utxoSet.AddUTXO(txHash, txIndex, balance, addr, genesisHash)
		txIndex++

		log.Printf("✅ Restored balance from genesis for %s: %d", addrHex, balance)
	}

	log.Printf("✅ Restored %d balances from genesis snapshot", len(bc.genesis.Snapshot.Balances))
	return nil
}
