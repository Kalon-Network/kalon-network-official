package core

import (
	"testing"
	"time"
)

// TestBlockchainGetBalance tests getting balance for an address from blockchain
func TestBlockchainGetBalance(t *testing.T) {
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

	address := Address{0x01, 0x02, 0x03}

	// Initial balance should be 0
	balance := bc.GetBalance(address)
	if balance != 0 {
		t.Errorf("Expected initial balance 0, got %d", balance)
	}

	// Create a block with reward to the address
	miner := address
	block := bc.CreateNewBlockV2(miner, []Transaction{})
	if block == nil {
		t.Fatal("Expected non-nil block")
	}

	// Add block
	if err := bc.AddBlockV2(block); err != nil {
		t.Fatalf("Failed to add block: %v", err)
	}

	// Balance should be updated (block reward)
	balanceAfterBlock := bc.GetBalance(address)
	if balanceAfterBlock == 0 {
		t.Error("Expected balance to be greater than 0 after block reward")
	}
}

// TestGetTotalTransactions tests getting total transaction count
func TestGetTotalTransactions(t *testing.T) {
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

	// Get initial transaction count (genesis block may or may not have transactions)
	initialTxs := bc.GetTotalTransactions()

	// Add a block
	miner := Address{0x01}
	block := bc.CreateNewBlockV2(miner, []Transaction{})
	if block == nil {
		t.Fatal("Expected non-nil block")
	}

	if err := bc.AddBlockV2(block); err != nil {
		t.Fatalf("Failed to add block: %v", err)
	}

	// Transaction count should increase (at least 1 for block reward)
	totalTxsAfterBlock := bc.GetTotalTransactions()
	if totalTxsAfterBlock <= initialTxs {
		t.Errorf("Expected transaction count to increase, got %d (was %d)", totalTxsAfterBlock, initialTxs)
	}

	// Verify we can get total transactions
	if totalTxsAfterBlock == 0 {
		t.Error("Expected transaction count > 0 after adding block")
	}
}

// TestGetAddressCount tests getting address count
func TestGetAddressCount(t *testing.T) {
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

	// Initial address count (genesis block may or may not have outputs)
	addressCount := bc.GetAddressCount()
	// Note: Genesis block may not have outputs, so we just check it's >= 0
	if addressCount < 0 {
		t.Errorf("Expected address count >= 0, got %d", addressCount)
	}

	// Add a block with a different miner address
	miner := Address{0x01, 0x02, 0x03}
	block := bc.CreateNewBlockV2(miner, []Transaction{})
	if block == nil {
		t.Fatal("Expected non-nil block")
	}

	if err := bc.AddBlockV2(block); err != nil {
		t.Fatalf("Failed to add block: %v", err)
	}

	// Address count should increase
	addressCountAfterBlock := bc.GetAddressCount()
	if addressCountAfterBlock <= addressCount {
		t.Errorf("Expected address count to increase, got %d (was %d)", addressCountAfterBlock, addressCount)
	}
}

// TestGetTreasuryBalance tests getting treasury balance
func TestGetTreasuryBalance(t *testing.T) {
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
		TreasuryAddress: "tkalon1treasury0000000000000000000000000000000000000000000000000000000000",
	}

	bc := NewBlockchainV2(genesis, nil)

	// Treasury balance should be 0 initially (no treasury rewards yet)
	treasuryBalance := bc.GetTreasuryBalance()
	if treasuryBalance != 0 {
		t.Errorf("Expected initial treasury balance 0, got %d", treasuryBalance)
	}
}

// TestGetTreasuryBalanceNoTreasury tests getting treasury balance when no treasury address is set
func TestGetTreasuryBalanceNoTreasury(t *testing.T) {
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
		// No TreasuryAddress set
	}

	bc := NewBlockchainV2(genesis, nil)

	// Treasury balance should be 0 when no treasury address is set
	treasuryBalance := bc.GetTreasuryBalance()
	if treasuryBalance != 0 {
		t.Errorf("Expected treasury balance 0 when no treasury address, got %d", treasuryBalance)
	}
}

// TestGetAddressTransactions tests getting transactions for an address
func TestGetAddressTransactions(t *testing.T) {
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

	address := Address{0x01, 0x02, 0x03}

	// Initial transactions should be empty (or contain genesis block reward if address matches)
	transactions := bc.GetAddressTransactions(address)
	initialCount := len(transactions)

	// Add a block with reward to the address
	miner := address
	block := bc.CreateNewBlockV2(miner, []Transaction{})
	if block == nil {
		t.Fatal("Expected non-nil block")
	}

	if err := bc.AddBlockV2(block); err != nil {
		t.Fatalf("Failed to add block: %v", err)
	}

	// Transactions should increase (at least 1 for block reward)
	transactionsAfterBlock := bc.GetAddressTransactions(address)
	if len(transactionsAfterBlock) <= initialCount {
		t.Errorf("Expected transaction count to increase, got %d (was %d)", len(transactionsAfterBlock), initialCount)
	}
}

// TestGetBlockByNumber tests getting a block by number
func TestGetBlockByNumber(t *testing.T) {
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

	// Get genesis block (number 0)
	genesisBlock, err := bc.GetBlockByNumber(0)
	if err != nil {
		t.Fatalf("Failed to get genesis block: %v", err)
	}
	if genesisBlock == nil {
		t.Fatal("Expected non-nil genesis block")
	}
	if genesisBlock.Header.Number != 0 {
		t.Errorf("Expected genesis block number 0, got %d", genesisBlock.Header.Number)
	}

	// Add a block
	miner := Address{0x01}
	block := bc.CreateNewBlockV2(miner, []Transaction{})
	if block == nil {
		t.Fatal("Expected non-nil block")
	}

	if err := bc.AddBlockV2(block); err != nil {
		t.Fatalf("Failed to add block: %v", err)
	}

	// Get block 1
	block1, err := bc.GetBlockByNumber(1)
	if err != nil {
		t.Fatalf("Failed to get block 1: %v", err)
	}
	if block1 == nil {
		t.Fatal("Expected non-nil block 1")
	}
	if block1.Header.Number != 1 {
		t.Errorf("Expected block number 1, got %d", block1.Header.Number)
	}

	// Try to get non-existent block
	_, err = bc.GetBlockByNumber(999)
	if err == nil {
		t.Error("Expected error when getting non-existent block")
	}
}

// TestGetRecentBlocks tests getting recent blocks
func TestGetRecentBlocks(t *testing.T) {
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

	// Get recent blocks (should include genesis)
	recentBlocks := bc.GetRecentBlocks(10)
	if len(recentBlocks) < 1 {
		t.Errorf("Expected at least 1 recent block, got %d", len(recentBlocks))
	}

	// Add multiple blocks
	miner := Address{0x01}
	for i := 0; i < 5; i++ {
		block := bc.CreateNewBlockV2(miner, []Transaction{})
		if block == nil {
			t.Fatalf("Failed to create block %d", i+1)
		}
		if err := bc.AddBlockV2(block); err != nil {
			t.Fatalf("Failed to add block %d: %v", i+1, err)
		}
		time.Sleep(10 * time.Millisecond) // Small delay to ensure different timestamps
	}

	// Get recent blocks
	recentBlocksAfter := bc.GetRecentBlocks(10)
	if len(recentBlocksAfter) < 6 { // Genesis + 5 new blocks
		t.Errorf("Expected at least 6 recent blocks, got %d", len(recentBlocksAfter))
	}

	// Verify blocks are in reverse order (newest first)
	if len(recentBlocksAfter) > 1 {
		for i := 0; i < len(recentBlocksAfter)-1; i++ {
			if recentBlocksAfter[i].Header.Number < recentBlocksAfter[i+1].Header.Number {
				t.Error("Recent blocks should be in reverse order (newest first)")
			}
		}
	}
}
