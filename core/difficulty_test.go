package core

import (
	"testing"
	"time"
)

// TestDifficultyValidation tests difficulty validation in validateBlockV2
func TestDifficultyValidation(t *testing.T) {
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

	// Test 1: Block with correct difficulty should pass validation
	t.Run("CorrectDifficulty", func(t *testing.T) {
		// Create block 1
		miner1 := Address{0x01}
		block1 := bc.CreateNewBlockV2(miner1, []Transaction{})
		if block1 == nil {
			t.Fatal("Block 1 should be created")
		}

		// Add block 1
		if err := bc.AddBlockV2(block1); err != nil {
			t.Fatalf("Failed to add block 1: %v", err)
		}

		// Block 1 should have correct difficulty (same as parent for height < window)
		consensusManager := NewConsensusManager(genesis)
		expectedDifficulty := consensusManager.CalculateDifficulty(block1.Header.Number, genesisBlock, []time.Time{})
		if block1.Header.Difficulty != expectedDifficulty {
			t.Errorf("Block 1 should have difficulty %d, got %d", expectedDifficulty, block1.Header.Difficulty)
		}
	})

	// Test 2: Block with incorrect difficulty should fail validation
	t.Run("IncorrectDifficulty", func(t *testing.T) {
		// Get current best block
		bestBlock := bc.GetBestBlock()
		if bestBlock == nil {
			t.Fatal("Expected best block")
		}

		// Create block with incorrect difficulty
		block := &Block{
			Header: BlockHeader{
				ParentHash: bestBlock.Hash,
				Number:     bestBlock.Header.Number + 1,
				Timestamp:  time.Now(),
				Difficulty: 999, // Incorrect difficulty
				Miner:      Address{0x02},
				Nonce:      0,
				MerkleRoot: Hash{},
				TxCount:    0,
			},
			Txs:  []Transaction{},
			Hash: Hash{},
		}
		block.Hash = block.CalculateHash()

		// Try to add block with incorrect difficulty
		err := bc.AddBlockV2(block)
		if err == nil {
			t.Error("Expected error when adding block with incorrect difficulty")
		}
		if err != nil && !contains(err.Error(), "invalid difficulty") {
			t.Errorf("Expected 'invalid difficulty' error, got: %v", err)
		}
	})

	// Test 3: Block with correct difficulty calculated from parent should pass
	t.Run("CorrectDifficultyFromParent", func(t *testing.T) {
		// Get current best block
		bestBlock := bc.GetBestBlock()
		if bestBlock == nil {
			t.Fatal("Expected best block")
		}

		// Calculate expected difficulty
		consensusManager := NewConsensusManager(genesis)
		expectedDifficulty := consensusManager.CalculateDifficulty(bestBlock.Header.Number+1, bestBlock, []time.Time{})

		// Create block with correct difficulty
		block := &Block{
			Header: BlockHeader{
				ParentHash: bestBlock.Hash,
				Number:     bestBlock.Header.Number + 1,
				Timestamp:  time.Now(),
				Difficulty: expectedDifficulty, // Correct difficulty
				Miner:      Address{0x03},
				Nonce:      0,
				MerkleRoot: Hash{},
				TxCount:    0,
			},
			Txs:  []Transaction{},
			Hash: Hash{},
		}
		block.Hash = block.CalculateHash()

		// Block should pass validation
		if err := bc.AddBlockV2(block); err != nil {
			t.Errorf("Block with correct difficulty should pass validation, got error: %v", err)
		}
	})
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
