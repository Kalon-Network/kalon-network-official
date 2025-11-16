package core

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"testing"
)

// TestTransactionSignatureValidation tests the transaction signature validation
func TestTransactionSignatureValidation(t *testing.T) {
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

	// Helper function to generate keypair (to avoid import cycle)
	generateKeypair := func() (ed25519.PublicKey, ed25519.PrivateKey, error) {
		return ed25519.GenerateKey(rand.Reader)
	}

	// Helper function to calculate address from public key
	addressFromPubKey := func(pub ed25519.PublicKey) Address {
		hash := sha256.Sum256(pub)
		var result Address
		copy(result[:], hash[:20])
		return result
	}

	// Helper function to sign transaction
	signTransaction := func(priv ed25519.PrivateKey, tx *Transaction) error {
		message := createTransactionMessageForTest(tx)
		signature := ed25519.Sign(priv, message)
		tx.Signature = signature
		tx.PublicKey = priv.Public().(ed25519.PublicKey)
		return nil
	}

	// Test 1: Valid signed transaction should pass
	t.Run("ValidSignedTransaction", func(t *testing.T) {
		// Generate keypair
		pub, priv, err := generateKeypair()
		if err != nil {
			t.Fatalf("Failed to generate keypair: %v", err)
		}

		// Create transaction
		fromAddr := addressFromPubKey(pub)
		toAddr := Address{0x01, 0x02, 0x03}
		tx := &Transaction{
			From:     fromAddr,
			To:       toAddr,
			Amount:   1000000,
			Nonce:    1,
			Fee:      1000000,
			GasUsed:  1,
			GasPrice: 1000000,
			Inputs:   []TxInput{{PreviousTxHash: Hash{}, Index: 0}},
			Outputs:  []TxOutput{{Address: toAddr, Amount: 1000000}},
		}

		// Sign transaction
		err = signTransaction(priv, tx)
		if err != nil {
			t.Fatalf("Failed to sign transaction: %v", err)
		}

		// Validate transaction
		err = cm.ValidateTransaction(tx)
		if err != nil {
			t.Errorf("Valid signed transaction should pass validation, got error: %v", err)
		}
	})

	// Test 2: Transaction without signature should fail
	t.Run("TransactionWithoutSignature", func(t *testing.T) {
		pub, _, err := generateKeypair()
		if err != nil {
			t.Fatalf("Failed to generate keypair: %v", err)
		}

		fromAddr := addressFromPubKey(pub)
		toAddr := Address{0x01, 0x02, 0x03}
		tx := &Transaction{
			From:     fromAddr,
			To:       toAddr,
			Amount:   1000000,
			Nonce:    1,
			Fee:      1000000,
			GasUsed:  1,
			GasPrice: 1000000,
			Inputs:   []TxInput{{PreviousTxHash: Hash{}, Index: 0}},
			Outputs:  []TxOutput{{Address: toAddr, Amount: 1000000}},
			// No signature
		}

		err = cm.ValidateTransaction(tx)
		if err == nil {
			t.Error("Transaction without signature should fail validation")
		}
	})

	// Test 3: Transaction without public key should fail
	t.Run("TransactionWithoutPublicKey", func(t *testing.T) {
		pub, _, err := generateKeypair()
		if err != nil {
			t.Fatalf("Failed to generate keypair: %v", err)
		}

		fromAddr := addressFromPubKey(pub)
		toAddr := Address{0x01, 0x02, 0x03}
		tx := &Transaction{
			From:      fromAddr,
			To:        toAddr,
			Amount:    1000000,
			Nonce:     1,
			Fee:       1000000,
			GasUsed:   1,
			GasPrice:  1000000,
			Inputs:    []TxInput{{PreviousTxHash: Hash{}, Index: 0}},
			Outputs:   []TxOutput{{Address: toAddr, Amount: 1000000}},
			Signature: []byte{0x01, 0x02, 0x03}, // Fake signature
			// No public key
		}

		err = cm.ValidateTransaction(tx)
		if err == nil {
			t.Error("Transaction without public key should fail validation")
		}
	})

	// Test 4: Transaction with wrong public key should fail
	t.Run("TransactionWithWrongPublicKey", func(t *testing.T) {
		pub1, priv1, err := generateKeypair()
		if err != nil {
			t.Fatalf("Failed to generate keypair: %v", err)
		}

		pub2, _, err := generateKeypair()
		if err != nil {
			t.Fatalf("Failed to generate keypair: %v", err)
		}

		// Use keypair1's address but keypair2's public key
		fromAddr := addressFromPubKey(pub1)
		toAddr := Address{0x01, 0x02, 0x03}
		tx := &Transaction{
			From:      fromAddr,
			To:        toAddr,
			Amount:    1000000,
			Nonce:     1,
			Fee:       1000000,
			GasUsed:   1,
			GasPrice:  1000000,
			Inputs:    []TxInput{{PreviousTxHash: Hash{}, Index: 0}},
			Outputs:   []TxOutput{{Address: toAddr, Amount: 1000000}},
			PublicKey: pub2, // Wrong public key
		}

		// Sign with keypair1
		err = signTransaction(priv1, tx)
		if err != nil {
			t.Fatalf("Failed to sign transaction: %v", err)
		}

		// Overwrite with wrong public key
		tx.PublicKey = pub2

		err = cm.ValidateTransaction(tx)
		if err == nil {
			t.Error("Transaction with wrong public key should fail validation")
		}
	})

	// Test 5: Transaction with invalid signature should fail
	t.Run("TransactionWithInvalidSignature", func(t *testing.T) {
		pub, _, err := generateKeypair()
		if err != nil {
			t.Fatalf("Failed to generate keypair: %v", err)
		}

		fromAddr := addressFromPubKey(pub)
		toAddr := Address{0x01, 0x02, 0x03}
		tx := &Transaction{
			From:      fromAddr,
			To:        toAddr,
			Amount:    1000000,
			Nonce:     1,
			Fee:       1000000,
			GasUsed:   1,
			GasPrice:  1000000,
			Inputs:    []TxInput{{PreviousTxHash: Hash{}, Index: 0}},
			Outputs:   []TxOutput{{Address: toAddr, Amount: 1000000}},
			PublicKey: pub,
			Signature: make([]byte, 64), // Invalid signature (all zeros)
		}

		err = cm.ValidateTransaction(tx)
		if err == nil {
			t.Error("Transaction with invalid signature should fail validation")
		}
	})

	// Test 6: Block reward transaction should pass (no signature required)
	t.Run("BlockRewardTransaction", func(t *testing.T) {
		tx := &Transaction{
			From:     Address{0x01},
			To:       Address{0x02},
			Amount:   5000000,
			Nonce:    0,
			Fee:      1000000, // Set fee to pass fee validation
			GasUsed:  1,
			GasPrice: 1000000,
			Inputs:   []TxInput{}, // No inputs = block reward
			Outputs:  []TxOutput{{Address: Address{0x01}, Amount: 5000000}},
		}

		err := cm.ValidateTransaction(tx)
		if err != nil {
			t.Errorf("Block reward transaction should pass validation, got error: %v", err)
		}
	})

	// Test 7: Transaction with modified data should fail
	t.Run("TransactionWithModifiedData", func(t *testing.T) {
		pub, priv, err := generateKeypair()
		if err != nil {
			t.Fatalf("Failed to generate keypair: %v", err)
		}

		fromAddr := addressFromPubKey(pub)
		toAddr := Address{0x01, 0x02, 0x03}
		tx := &Transaction{
			From:     fromAddr,
			To:       toAddr,
			Amount:   1000000,
			Nonce:    1,
			Fee:      1000000,
			GasUsed:  1,
			GasPrice: 1000000,
			Inputs:   []TxInput{{PreviousTxHash: Hash{}, Index: 0}},
			Outputs:  []TxOutput{{Address: toAddr, Amount: 1000000}},
		}

		// Sign transaction
		err = signTransaction(priv, tx)
		if err != nil {
			t.Fatalf("Failed to sign transaction: %v", err)
		}

		// Modify transaction data after signing
		tx.Amount = 2000000

		err = cm.ValidateTransaction(tx)
		if err == nil {
			t.Error("Transaction with modified data should fail validation")
		}
	})

	// Test 8: Multiple valid transactions should all pass
	t.Run("MultipleValidTransactions", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			pub, priv, err := generateKeypair()
			if err != nil {
				t.Fatalf("Failed to generate keypair: %v", err)
			}

			fromAddr := addressFromPubKey(pub)
			toAddr := Address{byte(i), 0x02, 0x03}
			tx := &Transaction{
				From:     fromAddr,
				To:       toAddr,
				Amount:   1000000,
				Nonce:    uint64(i + 1),
				Fee:      1000000,
				GasUsed:  1,
				GasPrice: 1000000,
				Inputs:   []TxInput{{PreviousTxHash: Hash{}, Index: 0}},
				Outputs:  []TxOutput{{Address: toAddr, Amount: 1000000}},
			}

			err = signTransaction(priv, tx)
			if err != nil {
				t.Fatalf("Failed to sign transaction: %v", err)
			}

			err = cm.ValidateTransaction(tx)
			if err != nil {
				t.Errorf("Valid transaction %d should pass validation, got error: %v", i, err)
			}
		}
	})
}

// TestAddressFromPubKey tests the address from public key calculation
func TestAddressFromPubKey(t *testing.T) {
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

	// Generate keypair
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	// Calculate address from public key
	addr1 := cm.addressFromPubKey(pub)

	// Should be deterministic
	addr2 := cm.addressFromPubKey(pub)
	if addr1 != addr2 {
		t.Error("Address calculation should be deterministic")
	}
}

// TestVerifyTransactionSignature tests the signature verification
func TestVerifyTransactionSignature(t *testing.T) {
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

	// Helper function to calculate address from public key
	addressFromPubKey := func(pub ed25519.PublicKey) Address {
		hash := sha256.Sum256(pub)
		var result Address
		copy(result[:], hash[:20])
		return result
	}

	// Helper function to sign transaction
	signTransaction := func(priv ed25519.PrivateKey, tx *Transaction) error {
		message := createTransactionMessageForTest(tx)
		signature := ed25519.Sign(priv, message)
		tx.Signature = signature
		tx.PublicKey = priv.Public().(ed25519.PublicKey)
		return nil
	}

	// Generate keypair
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	fromAddr := addressFromPubKey(pub)
	toAddr := Address{0x01, 0x02, 0x03}
	tx := &Transaction{
		From:     fromAddr,
		To:       toAddr,
		Amount:   1000000,
		Nonce:    1,
		Fee:      1000000,
		GasUsed:  1,
		GasPrice: 1000000,
		Inputs:   []TxInput{{PreviousTxHash: Hash{}, Index: 0}},
		Outputs:  []TxOutput{{Address: toAddr, Amount: 1000000}},
	}

	// Sign transaction
	err = signTransaction(priv, tx)
	if err != nil {
		t.Fatalf("Failed to sign transaction: %v", err)
	}

	// Verify signature
	valid := cm.verifyTransactionSignature(tx, pub)
	if !valid {
		t.Error("Valid signature should pass verification")
	}

	// Modify transaction and verify it fails
	tx.Amount = 2000000
	valid = cm.verifyTransactionSignature(tx, pub)
	if valid {
		t.Error("Modified transaction should fail verification")
	}
}

// TestCreateTransactionMessage tests the transaction message creation
func TestCreateTransactionMessage(t *testing.T) {
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

	tx := &Transaction{
		From:     Address{0x01, 0x02, 0x03},
		To:       Address{0x04, 0x05, 0x06},
		Amount:   1000000,
		Nonce:    1,
		Fee:      1000000,
		GasUsed:  1,
		GasPrice: 1000000,
		Data:     []byte{0x01, 0x02, 0x03},
	}

	// Create message
	message1 := cm.createTransactionMessage(tx)
	message2 := cm.createTransactionMessage(tx)

	// Should be deterministic
	if len(message1) != len(message2) {
		t.Error("Message should be deterministic")
	}

	for i := range message1 {
		if message1[i] != message2[i] {
			t.Error("Message should be deterministic")
		}
	}

	// Modify transaction and verify message changes
	tx.Amount = 2000000
	message3 := cm.createTransactionMessage(tx)
	if len(message1) == len(message3) {
		// Messages might have same length but different content
		same := true
		for i := range message1 {
			if message1[i] != message3[i] {
				same = false
				break
			}
		}
		if same {
			t.Error("Modified transaction should produce different message")
		}
	}
}

// TestTransactionSignatureEdgeCases tests edge cases
func TestTransactionSignatureEdgeCases(t *testing.T) {
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

	// Helper function to calculate address from public key
	addressFromPubKey := func(pub ed25519.PublicKey) Address {
		hash := sha256.Sum256(pub)
		var result Address
		copy(result[:], hash[:20])
		return result
	}

	// Test with empty signature
	t.Run("EmptySignature", func(t *testing.T) {
		pub, _, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatalf("Failed to generate keypair: %v", err)
		}

		fromAddr := addressFromPubKey(pub)
		tx := &Transaction{
			From:      fromAddr,
			To:        Address{0x01},
			Amount:    1000000,
			Nonce:     1,
			Fee:       1000000,
			GasUsed:   1,
			GasPrice:  1000000,
			Inputs:    []TxInput{{PreviousTxHash: Hash{}, Index: 0}},
			Outputs:   []TxOutput{{Address: Address{0x01}, Amount: 1000000}},
			PublicKey: pub,
			Signature: []byte{}, // Empty signature
		}

		err = cm.ValidateTransaction(tx)
		if err == nil {
			t.Error("Transaction with empty signature should fail")
		}
	})

	// Test with wrong signature length
	t.Run("WrongSignatureLength", func(t *testing.T) {
		pub, _, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatalf("Failed to generate keypair: %v", err)
		}

		fromAddr := addressFromPubKey(pub)
		tx := &Transaction{
			From:      fromAddr,
			To:        Address{0x01},
			Amount:    1000000,
			Nonce:     1,
			Fee:       1000000,
			GasUsed:   1,
			GasPrice:  1000000,
			Inputs:    []TxInput{{PreviousTxHash: Hash{}, Index: 0}},
			Outputs:   []TxOutput{{Address: Address{0x01}, Amount: 1000000}},
			PublicKey: pub,
			Signature: make([]byte, 32), // Wrong length (should be 64 for ed25519)
		}

		err = cm.ValidateTransaction(tx)
		// ed25519.Verify will fail with wrong length signature
		if err == nil {
			t.Error("Transaction with wrong signature length should fail")
		}
	})

	// Test with random public key
	t.Run("RandomPublicKey", func(t *testing.T) {
		randomPubKey := make(ed25519.PublicKey, ed25519.PublicKeySize)
		rand.Read(randomPubKey)

		fromAddr := Address{0x01, 0x02, 0x03}
		tx := &Transaction{
			From:      fromAddr,
			To:        Address{0x04, 0x05, 0x06},
			Amount:    1000000,
			Nonce:     1,
			Fee:       1000000,
			GasUsed:   1,
			GasPrice:  1000000,
			Inputs:    []TxInput{{PreviousTxHash: Hash{}, Index: 0}},
			Outputs:   []TxOutput{{Address: Address{0x04}, Amount: 1000000}},
			PublicKey: randomPubKey,
			Signature: make([]byte, 64),
		}

		err := cm.ValidateTransaction(tx)
		if err == nil {
			t.Error("Transaction with random public key should fail")
		}
	})
}

// createTransactionMessageForTest creates the message to sign for a transaction (for testing)
func createTransactionMessageForTest(tx *Transaction) []byte {
	// Create message from transaction fields (excluding signature and public key)
	data := make([]byte, 0, 200)
	data = append(data, tx.From.Bytes()...)
	data = append(data, tx.To.Bytes()...)

	// Use compatible binary encoding for older Go versions
	amountBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(amountBytes, tx.Amount)
	data = append(data, amountBytes...)

	nonceBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(nonceBytes, tx.Nonce)
	data = append(data, nonceBytes...)

	feeBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(feeBytes, tx.Fee)
	data = append(data, feeBytes...)

	gasUsedBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(gasUsedBytes, tx.GasUsed)
	data = append(data, gasUsedBytes...)

	gasPriceBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(gasPriceBytes, tx.GasPrice)
	data = append(data, gasPriceBytes...)

	data = append(data, tx.Data...)

	return data
}
