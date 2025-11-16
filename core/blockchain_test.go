package core

import (
	"testing"
	"time"
)

// TestNewBlockchainV2 tests the creation of a new blockchain
func TestNewBlockchainV2(t *testing.T) {
	genesis := &GenesisConfig{
		ChainID:            7718,
		Name:               "Test Network",
		Symbol:             "TEST",
		BlockTimeTarget:    30,
		InitialBlockReward: 5.0,
		HalvingSchedule: []HalvingEvent{
			{AfterBlocks: 259200, RewardMultiplier: 0.5},
		},
		Difficulty: DifficultyConfig{
			Algo:              "LWMA",
			Window:            120,
			InitialDifficulty: 5000,
		},
	}

	bc := NewBlockchainV2(genesis, nil)
	if bc == nil {
		t.Fatal("Expected non-nil blockchain")
	}

	if bc.GetHeight() != 0 {
		t.Errorf("Expected height 0, got %d", bc.GetHeight())
	}

	bestBlock := bc.GetBestBlock()
	if bestBlock == nil {
		t.Fatal("Expected non-nil best block")
	}

	if bestBlock.Header.Number != 0 {
		t.Errorf("Expected genesis block number 0, got %d", bestBlock.Header.Number)
	}
}

// TestBlockHash tests block hash calculation
func TestBlockHash(t *testing.T) {
	block := &Block{
		Header: BlockHeader{
			ParentHash: Hash{},
			Number:     1,
			Timestamp:  time.Now(),
			Difficulty: 5000,
			Nonce:      0,
		},
	}

	hash1 := block.CalculateHash()

	// Calculate again - should be the same
	hash2 := block.CalculateHash()

	if hash1 != hash2 {
		t.Error("Hash should be deterministic")
	}

	// Change nonce - hash should be different
	block.Header.Nonce = 1
	hash3 := block.CalculateHash()

	if hash1 == hash3 {
		t.Error("Hash should change when block changes")
	}
}

// TestTransactionValidation tests transaction validation
func TestTransactionValidation(t *testing.T) {
	// Valid transaction
	tx := &Transaction{
		From:   Address{1},
		To:     Address{2},
		Amount: 1000,
		Fee:    100,
	}

	if !tx.IsValid() {
		t.Error("Expected valid transaction")
	}

	// Invalid transaction (zero amount and fee)
	tx2 := &Transaction{
		From:   Address{1},
		To:     Address{2},
		Amount: 0,
		Fee:    0,
	}

	if tx2.IsValid() {
		t.Error("Expected invalid transaction")
	}

	// Invalid transaction (empty addresses)
	tx3 := &Transaction{
		From:   Address{},
		To:     Address{},
		Amount: 1000,
		Fee:    100,
	}

	if tx3.IsValid() {
		t.Error("Expected invalid transaction with empty addresses")
	}
}

// TestAddressString tests address string representation
func TestAddressString(t *testing.T) {
	addr := Address{0x12, 0x34, 0x56}
	str := addr.String()

	if len(str) != 40 {
		t.Errorf("Expected 40 character hex string, got length %d", len(str))
	}

	// Should contain expected hex chars
	if str[:6] != "123456" {
		t.Errorf("Expected address to start with '123456', got %s", str[:6])
	}
}

// TestCalculateBlockReward tests block reward calculation
func TestCalculateBlockReward(t *testing.T) {
	genesis := &GenesisConfig{
		InitialBlockReward: 5.0,
		HalvingSchedule: []HalvingEvent{
			{AfterBlocks: 100, RewardMultiplier: 0.5},
		},
	}

	bc := NewBlockchainV2(genesis, nil)

	// Block 1 should have full reward
	reward1 := bc.calculateBlockReward(1)
	if reward1 != 5000000 { // 5.0 * 1,000,000
		t.Errorf("Expected reward 5000000, got %d", reward1)
	}

	// Block 101 should still have reward
	reward2 := bc.calculateBlockReward(101)
	if reward2 == 0 {
		t.Error("Reward should not be zero")
	}
}
