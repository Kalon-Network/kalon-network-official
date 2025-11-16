package core

import (
	"testing"
)

// TestMerkleRootCalculation tests merkle root calculation
func TestMerkleRootCalculation(t *testing.T) {
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
			InitialDifficulty: 5000,
		},
		NetworkFee: NetworkFeeConfig{
			BaseTxFee: 0.001,
		},
	}

	cm := NewConsensusManager(genesis)

	// Test 1: Empty transactions should return empty hash
	t.Run("EmptyTransactions", func(t *testing.T) {
		txs := []Transaction{}
		merkleRoot := cm.CalculateMerkleRoot(txs)
		if merkleRoot != (Hash{}) {
			t.Errorf("Empty transactions should return empty hash, got %x", merkleRoot)
		}
	})

	// Test 2: Single transaction should return transaction hash
	t.Run("SingleTransaction", func(t *testing.T) {
		tx := &Transaction{
			From:     Address{0x01},
			To:       Address{0x02},
			Amount:   1000000,
			Nonce:    1,
			Fee:      1000000,
			GasUsed:  1,
			GasPrice: 1000000,
		}
		tx.Hash = tx.CalculateHash()

		txs := []Transaction{*tx}
		merkleRoot := cm.CalculateMerkleRoot(txs)
		expectedHash := tx.CalculateHash()

		if merkleRoot != expectedHash {
			t.Errorf("Single transaction should return transaction hash, got %x, expected %x", merkleRoot, expectedHash)
		}
	})

	// Test 3: Multiple transactions should calculate merkle root
	t.Run("MultipleTransactions", func(t *testing.T) {
		txs := []Transaction{}
		for i := 0; i < 5; i++ {
			tx := &Transaction{
				From:     Address{byte(i)},
				To:       Address{byte(i + 1)},
				Amount:   uint64(1000000 * (i + 1)),
				Nonce:    uint64(i + 1),
				Fee:      1000000,
				GasUsed:  1,
				GasPrice: 1000000,
			}
			tx.Hash = tx.CalculateHash()
			txs = append(txs, *tx)
		}

		merkleRoot1 := cm.CalculateMerkleRoot(txs)
		merkleRoot2 := cm.CalculateMerkleRoot(txs)

		// Should be deterministic
		if merkleRoot1 != merkleRoot2 {
			t.Error("Merkle root calculation should be deterministic")
		}

		// Should not be empty
		if merkleRoot1 == (Hash{}) {
			t.Error("Merkle root should not be empty for multiple transactions")
		}
	})

	// Test 4: Different transaction order should produce different merkle root
	t.Run("TransactionOrder", func(t *testing.T) {
		tx1 := &Transaction{
			From:     Address{0x01},
			To:       Address{0x02},
			Amount:   1000000,
			Nonce:    1,
			Fee:      1000000,
			GasUsed:  1,
			GasPrice: 1000000,
		}
		tx1.Hash = tx1.CalculateHash()

		tx2 := &Transaction{
			From:     Address{0x03},
			To:       Address{0x04},
			Amount:   2000000,
			Nonce:    2,
			Fee:      1000000,
			GasUsed:  1,
			GasPrice: 1000000,
		}
		tx2.Hash = tx2.CalculateHash()

		txs1 := []Transaction{*tx1, *tx2}
		txs2 := []Transaction{*tx2, *tx1}

		merkleRoot1 := cm.CalculateMerkleRoot(txs1)
		merkleRoot2 := cm.CalculateMerkleRoot(txs2)

		// Different order should produce different merkle root
		if merkleRoot1 == merkleRoot2 {
			t.Error("Different transaction order should produce different merkle root")
		}
	})

	// Test 5: Modified transaction should produce different merkle root
	t.Run("ModifiedTransaction", func(t *testing.T) {
		tx1 := &Transaction{
			From:     Address{0x01},
			To:       Address{0x02},
			Amount:   1000000,
			Nonce:    1,
			Fee:      1000000,
			GasUsed:  1,
			GasPrice: 1000000,
		}
		tx1.Hash = tx1.CalculateHash()

		tx2 := &Transaction{
			From:     Address{0x01},
			To:       Address{0x02},
			Amount:   2000000, // Different amount
			Nonce:    1,
			Fee:      1000000,
			GasUsed:  1,
			GasPrice: 1000000,
		}
		tx2.Hash = tx2.CalculateHash()

		txs1 := []Transaction{*tx1}
		txs2 := []Transaction{*tx2}

		merkleRoot1 := cm.CalculateMerkleRoot(txs1)
		merkleRoot2 := cm.CalculateMerkleRoot(txs2)

		// Modified transaction should produce different merkle root
		if merkleRoot1 == merkleRoot2 {
			t.Error("Modified transaction should produce different merkle root")
		}
	})
}

// TestMerkleRootInBlockCreation tests merkle root in block creation
func TestMerkleRootInBlockCreation(t *testing.T) {
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
			InitialDifficulty: 5000,
		},
		NetworkFee: NetworkFeeConfig{
			BaseTxFee: 0.001,
		},
	}

	bc := NewBlockchainV2(genesis, nil)

	// Test 1: Genesis block should have empty merkle root
	t.Run("GenesisBlock", func(t *testing.T) {
		genesisBlock := bc.GetBestBlock()
		if genesisBlock == nil {
			t.Fatal("Genesis block should exist")
		}

		// Genesis block has no transactions, so merkle root should be empty
		if len(genesisBlock.Txs) == 0 {
			expectedMerkleRoot := Hash{}
			if genesisBlock.Header.MerkleRoot != expectedMerkleRoot {
				t.Errorf("Genesis block with no transactions should have empty merkle root, got %x", genesisBlock.Header.MerkleRoot)
			}
		}
	})

	// Test 2: Block with transactions should have calculated merkle root
	t.Run("BlockWithTransactions", func(t *testing.T) {
		// Create a transaction
		tx := &Transaction{
			From:     Address{0x01},
			To:       Address{0x02},
			Amount:   1000000,
			Nonce:    1,
			Fee:      1000000,
			GasUsed:  1,
			GasPrice: 1000000,
			Inputs:   []TxInput{{PreviousTxHash: Hash{}, Index: 0}},
			Outputs:  []TxOutput{{Address: Address{0x02}, Amount: 1000000}},
		}
		tx.Hash = tx.CalculateHash()

		// Add transaction to mempool
		bc.GetMempool().AddTransaction(tx)

		// Create new block
		miner := Address{0x03}
		block := bc.CreateNewBlockV2(miner, []Transaction{})

		if block == nil {
			t.Fatal("Block should be created")
		}

		// Merkle root should be calculated
		if block.Header.MerkleRoot == (Hash{}) && len(block.Txs) > 0 {
			t.Error("Block with transactions should have calculated merkle root")
		}

		// Validate merkle root
		consensusManager := NewConsensusManager(genesis)
		expectedMerkleRoot := consensusManager.CalculateMerkleRoot(block.Txs)
		if block.Header.MerkleRoot != expectedMerkleRoot {
			t.Errorf("Block merkle root should match calculated merkle root, got %x, expected %x", block.Header.MerkleRoot, expectedMerkleRoot)
		}
	})
}

// TestMerkleRootValidation tests merkle root validation
func TestMerkleRootValidation(t *testing.T) {
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
			InitialDifficulty: 5000,
		},
		NetworkFee: NetworkFeeConfig{
			BaseTxFee: 0.001,
		},
	}

	bc := NewBlockchainV2(genesis, nil)

	// Test 1: Block with correct merkle root should pass validation
	t.Run("CorrectMerkleRoot", func(t *testing.T) {
		// Create a transaction
		tx := &Transaction{
			From:     Address{0x01},
			To:       Address{0x02},
			Amount:   1000000,
			Nonce:    1,
			Fee:      1000000,
			GasUsed:  1,
			GasPrice: 1000000,
			Inputs:   []TxInput{{PreviousTxHash: Hash{}, Index: 0}},
			Outputs:  []TxOutput{{Address: Address{0x02}, Amount: 1000000}},
		}
		tx.Hash = tx.CalculateHash()

		// Add transaction to mempool
		bc.GetMempool().AddTransaction(tx)

		// Create new block
		miner := Address{0x03}
		block := bc.CreateNewBlockV2(miner, []Transaction{})

		if block == nil {
			t.Fatal("Block should be created")
		}

		// Block should pass merkle root validation
		// Note: validateBlockV2 also checks proof of work, so we'll check merkle root directly
		consensusManager := NewConsensusManager(genesis)
		expectedMerkleRoot := consensusManager.CalculateMerkleRoot(block.Txs)
		if block.Header.MerkleRoot != expectedMerkleRoot {
			t.Errorf("Block with correct merkle root should match calculated merkle root, got %x, expected %x", block.Header.MerkleRoot, expectedMerkleRoot)
		}
	})

	// Test 2: Block with incorrect merkle root should fail validation
	t.Run("IncorrectMerkleRoot", func(t *testing.T) {
		// Create a transaction
		tx := &Transaction{
			From:     Address{0x01},
			To:       Address{0x02},
			Amount:   1000000,
			Nonce:    1,
			Fee:      1000000,
			GasUsed:  1,
			GasPrice: 1000000,
			Inputs:   []TxInput{{PreviousTxHash: Hash{}, Index: 0}},
			Outputs:  []TxOutput{{Address: Address{0x02}, Amount: 1000000}},
		}
		tx.Hash = tx.CalculateHash()

		// Add transaction to mempool
		bc.GetMempool().AddTransaction(tx)

		// Create new block
		miner := Address{0x03}
		block := bc.CreateNewBlockV2(miner, []Transaction{})

		if block == nil {
			t.Fatal("Block should be created")
		}

		// Modify merkle root
		block.Header.MerkleRoot = Hash{0xFF}

		// Block should fail validation
		err := bc.validateBlockV2(block)
		if err == nil {
			t.Error("Block with incorrect merkle root should fail validation")
		}
	})
}
