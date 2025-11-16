package core

import (
	"encoding/hex"
	"testing"
	"time"
)

// TestForkDetection tests fork detection when two blocks have the same parent
func TestForkDetection(t *testing.T) {
	genesis := &GenesisConfig{
		ChainID:            7718,
		Name:               "Test Network",
		Symbol:             "TEST",
		BlockTimeTarget:    30,
		InitialBlockReward: 5.0,
		HalvingSchedule:    []HalvingEvent{},
		Difficulty: DifficultyConfig{
			Algo:              "LWMA",
			Window:            120,
			InitialDifficulty: 10, // Low difficulty for testing
		},
		NetworkFee: NetworkFeeConfig{
			BaseTxFee: 0.001,
		},
	}

	bc := NewBlockchainV2(genesis, nil)

	// Get genesis block
	genesisBlock := bc.GetBestBlock()
	if genesisBlock == nil {
		t.Fatal("Expected genesis block")
	}

	// Create first block (normal case)
	miner1 := Address{0x01}
	block1 := bc.CreateNewBlockV2(miner1, []Transaction{})
	if block1 == nil {
		t.Fatal("Block 1 should be created")
	}

	// Add block 1
	if err := bc.AddBlockV2(block1); err != nil {
		t.Fatalf("Failed to add block 1: %v", err)
	}

	if bc.GetHeight() != 1 {
		t.Errorf("Expected height 1, got %d", bc.GetHeight())
	}

	bestBlock1 := bc.GetBestBlock()
	if bestBlock1.Hash != block1.Hash {
		t.Errorf("Expected best block to be block 1, got %x", bestBlock1.Hash)
	}

	// Create second block with same parent (fork)
	block2 := &Block{
		Header: BlockHeader{
			ParentHash: block1.Header.ParentHash, // Same parent as block1
			Number:     block1.Header.Number,
			Timestamp:  time.Now(),
			Difficulty: block1.Header.Difficulty,
			Miner:      Address{0x02},
			Nonce:      999, // Different nonce to get different hash
			MerkleRoot: Hash{},
			TxCount:    0,
		},
		Txs:  []Transaction{},
		Hash: Hash{},
	}
	block2.Hash = block2.CalculateHash()

	// This should be detected as a fork
	// Since block2 has same parent as block1 but different hash, it's a fork
	// But block2 has same number as block1, so it should be detected as fork
	if block2.Header.ParentHash != block1.Header.ParentHash {
		t.Fatal("Block 2 should have same parent as block 1 for fork test")
	}

	// Create block 2 properly - it should extend block1
	block2Proper := &Block{
		Header: BlockHeader{
			ParentHash: block1.Hash, // Extends block1
			Number:     block1.Header.Number + 1,
			Timestamp:  time.Now(),
			Difficulty: block1.Header.Difficulty,
			Miner:      Address{0x02},
			Nonce:      999,
			MerkleRoot: Hash{},
			TxCount:    0,
		},
		Txs:  []Transaction{},
		Hash: Hash{},
	}
	block2Proper.Hash = block2Proper.CalculateHash()

	// Create block 3 with same parent as block2Proper (fork scenario)
	block3 := &Block{
		Header: BlockHeader{
			ParentHash: block1.Hash, // Same parent as block2Proper
			Number:     block1.Header.Number + 1,
			Timestamp:  time.Now().Add(1 * time.Second),
			Difficulty: block1.Header.Difficulty,
			Miner:      Address{0x03},
			Nonce:      888, // Different nonce
			MerkleRoot: Hash{},
			TxCount:    0,
		},
		Txs:  []Transaction{},
		Hash: Hash{},
	}
	block3.Hash = block3.CalculateHash()

	// Add block2Proper first
	if err := bc.AddBlockV2(block2Proper); err != nil {
		t.Fatalf("Failed to add block 2: %v", err)
	}

	if bc.GetHeight() != 2 {
		t.Errorf("Expected height 2, got %d", bc.GetHeight())
	}

	bestBlock2 := bc.GetBestBlock()
	if bestBlock2.Hash != block2Proper.Hash {
		t.Errorf("Expected best block to be block 2, got %x", bestBlock2.Hash)
	}

	// Add block3 - this should be detected as a fork
	// Since block3 has same parent as block2Proper but different hash
	if err := bc.AddBlockV2(block3); err != nil {
		// Fork detected - block3 should be stored but not become best block
		// (since it has same length as current chain)
		t.Logf("Fork detected (expected): %v", err)
	}

	// Current chain should still be block2Proper (longest chain rule)
	bestBlock3 := bc.GetBestBlock()
	if bestBlock3.Hash != block2Proper.Hash {
		t.Errorf("Expected best block to still be block 2 (longest chain), got %x", bestBlock3.Hash)
	}
}

// TestChainReorganization tests chain reorganization when a longer fork is detected
func TestChainReorganization(t *testing.T) {
	genesis := &GenesisConfig{
		ChainID:            7718,
		Name:               "Test Network",
		Symbol:             "TEST",
		BlockTimeTarget:    30,
		InitialBlockReward: 5.0,
		HalvingSchedule:    []HalvingEvent{},
		Difficulty: DifficultyConfig{
			Algo:              "LWMA",
			Window:            120,
			InitialDifficulty: 10,
		},
		NetworkFee: NetworkFeeConfig{
			BaseTxFee: 0.001,
		},
	}

	bc := NewBlockchainV2(genesis, nil)

	// Get genesis block
	genesisBlock := bc.GetBestBlock()
	if genesisBlock == nil {
		t.Fatal("Expected genesis block")
	}

	// Create and add block 1
	miner1 := Address{0x01}
	block1 := bc.CreateNewBlockV2(miner1, []Transaction{})
	if err := bc.AddBlockV2(block1); err != nil {
		t.Fatalf("Failed to add block 1: %v", err)
	}

	// Create block 2 (chain A)
	block2A := &Block{
		Header: BlockHeader{
			ParentHash: block1.Hash,
			Number:     block1.Header.Number + 1,
			Timestamp:  time.Now(),
			Difficulty: block1.Header.Difficulty,
			Miner:      Address{0x02},
			Nonce:      111,
			MerkleRoot: Hash{},
			TxCount:    0,
		},
		Txs:  []Transaction{},
		Hash: Hash{},
	}
	block2A.Hash = block2A.CalculateHash()

	// Add block 2A
	if err := bc.AddBlockV2(block2A); err != nil {
		t.Fatalf("Failed to add block 2A: %v", err)
	}

	if bc.GetHeight() != 2 {
		t.Errorf("Expected height 2, got %d", bc.GetHeight())
	}

	// Create block 2B (fork - same parent as block2A)
	block2B := &Block{
		Header: BlockHeader{
			ParentHash: block1.Hash, // Same parent as block2A
			Number:     block1.Header.Number + 1,
			Timestamp:  time.Now().Add(1 * time.Second),
			Difficulty: block1.Header.Difficulty,
			Miner:      Address{0x03},
			Nonce:      222,
			MerkleRoot: Hash{},
			TxCount:    0,
		},
		Txs:  []Transaction{},
		Hash: Hash{},
	}
	block2B.Hash = block2B.CalculateHash()

	// Add block 2B - fork detected, but same length, so current chain stays
	if err := bc.AddBlockV2(block2B); err != nil {
		t.Logf("Fork detected (expected): %v", err)
	}

	// Current chain should still be block2A (same length)
	bestBlock := bc.GetBestBlock()
	if bestBlock.Hash != block2A.Hash {
		t.Errorf("Expected best block to be block 2A (same length), got %x", bestBlock.Hash)
	}

	// Create block 3B extending block2B (longer fork)
	block3B := &Block{
		Header: BlockHeader{
			ParentHash: block2B.Hash, // Extends block2B
			Number:     block2B.Header.Number + 1,
			Timestamp:  time.Now().Add(2 * time.Second),
			Difficulty: block2B.Header.Difficulty,
			Miner:      Address{0x04},
			Nonce:      333,
			MerkleRoot: Hash{},
			TxCount:    0,
		},
		Txs:  []Transaction{},
		Hash: Hash{},
	}
	block3B.Hash = block3B.CalculateHash()

	// Add block 3B - this should trigger reorganization
	// New chain (block1 -> block2B -> block3B) is longer than current (block1 -> block2A)
	if err := bc.AddBlockV2(block3B); err != nil {
		t.Fatalf("Failed to add block 3B (should trigger reorganization): %v", err)
	}

	// After reorganization, best block should be block3B
	bestBlockAfterReorg := bc.GetBestBlock()
	if bestBlockAfterReorg.Hash != block3B.Hash {
		t.Errorf("Expected best block to be block 3B after reorganization, got %x", bestBlockAfterReorg.Hash)
	}

	if bc.GetHeight() != 3 {
		t.Errorf("Expected height 3 after reorganization, got %d", bc.GetHeight())
	}
}

// TestChainLengthCalculation tests chain length calculation
func TestChainLengthCalculation(t *testing.T) {
	genesis := &GenesisConfig{
		ChainID:            7718,
		Name:               "Test Network",
		Symbol:             "TEST",
		BlockTimeTarget:    30,
		InitialBlockReward: 5.0,
		HalvingSchedule:    []HalvingEvent{},
		Difficulty: DifficultyConfig{
			Algo:              "LWMA",
			Window:            120,
			InitialDifficulty: 10,
		},
		NetworkFee: NetworkFeeConfig{
			BaseTxFee: 0.001,
		},
	}

	bc := NewBlockchainV2(genesis, nil)

	// Genesis block should have length 1
	genesisBlock := bc.GetBestBlock()
	length := bc.calculateChainLength(genesisBlock)
	if length != 1 {
		t.Errorf("Expected genesis block length 1, got %d", length)
	}

	// Add block 1
	miner1 := Address{0x01}
	block1 := bc.CreateNewBlockV2(miner1, []Transaction{})
	if err := bc.AddBlockV2(block1); err != nil {
		t.Fatalf("Failed to add block 1: %v", err)
	}

	// Block 1 should have length 2 (genesis + block1)
	length1 := bc.calculateChainLength(block1)
	if length1 != 2 {
		t.Errorf("Expected block 1 length 2, got %d", length1)
	}

	// Add block 2
	block2 := &Block{
		Header: BlockHeader{
			ParentHash: block1.Hash,
			Number:     block1.Header.Number + 1,
			Timestamp:  time.Now(),
			Difficulty: block1.Header.Difficulty,
			Miner:      Address{0x02},
			Nonce:      111,
			MerkleRoot: Hash{},
			TxCount:    0,
		},
		Txs:  []Transaction{},
		Hash: Hash{},
	}
	block2.Hash = block2.CalculateHash()

	if err := bc.AddBlockV2(block2); err != nil {
		t.Fatalf("Failed to add block 2: %v", err)
	}

	// Block 2 should have length 3 (genesis + block1 + block2)
	length2 := bc.calculateChainLength(block2)
	if length2 != 3 {
		t.Errorf("Expected block 2 length 3, got %d", length2)
	}
}

// TestCommonParentFinding tests finding common parent between two chains
func TestCommonParentFinding(t *testing.T) {
	genesis := &GenesisConfig{
		ChainID:            7718,
		Name:               "Test Network",
		Symbol:             "TEST",
		BlockTimeTarget:    30,
		InitialBlockReward: 5.0,
		HalvingSchedule:    []HalvingEvent{},
		Difficulty: DifficultyConfig{
			Algo:              "LWMA",
			Window:            120,
			InitialDifficulty: 10,
		},
		NetworkFee: NetworkFeeConfig{
			BaseTxFee: 0.001,
		},
	}

	bc := NewBlockchainV2(genesis, nil)

	// Create and add block 1
	miner1 := Address{0x01}
	block1 := bc.CreateNewBlockV2(miner1, []Transaction{})
	if err := bc.AddBlockV2(block1); err != nil {
		t.Fatalf("Failed to add block 1: %v", err)
	}

	// Create block 2A
	block2A := &Block{
		Header: BlockHeader{
			ParentHash: block1.Hash,
			Number:     block1.Header.Number + 1,
			Timestamp:  time.Now(),
			Difficulty: block1.Header.Difficulty,
			Miner:      Address{0x02},
			Nonce:      111,
			MerkleRoot: Hash{},
			TxCount:    0,
		},
		Txs:  []Transaction{},
		Hash: Hash{},
	}
	block2A.Hash = block2A.CalculateHash()

	// Add block 2A
	if err := bc.AddBlockV2(block2A); err != nil {
		t.Fatalf("Failed to add block 2A: %v", err)
	}

	// Create block 2B (fork)
	block2B := &Block{
		Header: BlockHeader{
			ParentHash: block1.Hash, // Same parent as block2A
			Number:     block1.Header.Number + 1,
			Timestamp:  time.Now().Add(1 * time.Second),
			Difficulty: block1.Header.Difficulty,
			Miner:      Address{0x03},
			Nonce:      222,
			MerkleRoot: Hash{},
			TxCount:    0,
		},
		Txs:  []Transaction{},
		Hash: Hash{},
	}
	block2B.Hash = block2B.CalculateHash()

	// Store block2B in index manually for testing
	block2BHashKey := hex.EncodeToString(block2B.Hash[:])
	bc.blockIndex[block2BHashKey] = block2B

	// Find common parent between block2A and block2B
	commonParent := bc.findCommonParent(block2A, block2B)
	if commonParent == nil {
		t.Fatal("Expected common parent between block2A and block2B")
	}

	// Common parent should be block1
	if commonParent.Hash != block1.Hash {
		t.Errorf("Expected common parent to be block 1, got %x", commonParent.Hash)
	}
}
