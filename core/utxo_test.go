package core

import (
	"testing"
)

// TestNewUTXOSet tests the creation of a new UTXO set
func TestNewUTXOSet(t *testing.T) {
	utxoSet := NewUTXOSet()
	if utxoSet == nil {
		t.Fatal("Expected non-nil UTXO set")
	}
	if utxoSet.utxos == nil {
		t.Fatal("Expected non-nil utxos map")
	}
}

// TestAddUTXO tests adding a UTXO to the set
func TestAddUTXO(t *testing.T) {
	utxoSet := NewUTXOSet()

	txHash := Hash{0x01, 0x02, 0x03}
	index := uint32(0)
	amount := uint64(1000000)
	address := Address{0x10, 0x20, 0x30}
	blockHash := Hash{0x04, 0x05, 0x06}

	utxoSet.AddUTXO(txHash, index, amount, address, blockHash)

	// Verify UTXO was added
	utxos := utxoSet.GetUTXOs(address)
	if len(utxos) != 1 {
		t.Fatalf("Expected 1 UTXO, got %d", len(utxos))
	}

	utxo := utxos[0]
	if utxo.TxHash != txHash {
		t.Errorf("Expected TxHash %x, got %x", txHash, utxo.TxHash)
	}
	if utxo.Index != index {
		t.Errorf("Expected Index %d, got %d", index, utxo.Index)
	}
	if utxo.Amount != amount {
		t.Errorf("Expected Amount %d, got %d", amount, utxo.Amount)
	}
	if utxo.Address != address {
		t.Errorf("Expected Address %x, got %x", address, utxo.Address)
	}
	if utxo.BlockHash != blockHash {
		t.Errorf("Expected BlockHash %x, got %x", blockHash, utxo.BlockHash)
	}
	if utxo.Spent {
		t.Error("UTXO should not be spent")
	}
}

// TestSpendUTXO tests spending a UTXO
func TestSpendUTXO(t *testing.T) {
	utxoSet := NewUTXOSet()

	txHash := Hash{0x01, 0x02, 0x03}
	index := uint32(0)
	amount := uint64(1000000)
	address := Address{0x10, 0x20, 0x30}
	blockHash := Hash{0x04, 0x05, 0x06}

	utxoSet.AddUTXO(txHash, index, amount, address, blockHash)

	// Spend the UTXO
	spent := utxoSet.SpendUTXO(txHash, index)
	if !spent {
		t.Error("Expected UTXO to be spent")
	}

	// Verify UTXO is no longer available
	utxos := utxoSet.GetUTXOs(address)
	if len(utxos) != 0 {
		t.Errorf("Expected 0 UTXOs after spending, got %d", len(utxos))
	}

	// Try to spend again - should fail
	spentAgain := utxoSet.SpendUTXO(txHash, index)
	if spentAgain {
		t.Error("Expected UTXO to already be spent")
	}
}

// TestGetUTXOs tests getting UTXOs for an address
func TestGetUTXOs(t *testing.T) {
	utxoSet := NewUTXOSet()

	address1 := Address{0x10, 0x20, 0x30}
	address2 := Address{0x40, 0x50, 0x60}

	// Add UTXOs for address1
	utxoSet.AddUTXO(Hash{0x01}, 0, 1000000, address1, Hash{0x04})
	utxoSet.AddUTXO(Hash{0x02}, 0, 2000000, address1, Hash{0x05})

	// Add UTXO for address2
	utxoSet.AddUTXO(Hash{0x03}, 0, 3000000, address2, Hash{0x06})

	// Get UTXOs for address1
	utxos1 := utxoSet.GetUTXOs(address1)
	if len(utxos1) != 2 {
		t.Errorf("Expected 2 UTXOs for address1, got %d", len(utxos1))
	}

	// Get UTXOs for address2
	utxos2 := utxoSet.GetUTXOs(address2)
	if len(utxos2) != 1 {
		t.Errorf("Expected 1 UTXO for address2, got %d", len(utxos2))
	}

	// Get UTXOs for non-existent address
	address3 := Address{0x70, 0x80, 0x90}
	utxos3 := utxoSet.GetUTXOs(address3)
	if len(utxos3) != 0 {
		t.Errorf("Expected 0 UTXOs for address3, got %d", len(utxos3))
	}
}

// TestGetBalance tests balance calculation
func TestGetBalance(t *testing.T) {
	utxoSet := NewUTXOSet()

	address := Address{0x10, 0x20, 0x30}

	// Add multiple UTXOs
	utxoSet.AddUTXO(Hash{0x01}, 0, 1000000, address, Hash{0x04})
	utxoSet.AddUTXO(Hash{0x02}, 0, 2000000, address, Hash{0x05})
	utxoSet.AddUTXO(Hash{0x03}, 0, 3000000, address, Hash{0x06})

	balance := utxoSet.GetBalance(address)
	expectedBalance := uint64(6000000) // 1000000 + 2000000 + 3000000

	if balance != expectedBalance {
		t.Errorf("Expected balance %d, got %d", expectedBalance, balance)
	}

	// Spend one UTXO
	utxoSet.SpendUTXO(Hash{0x02}, 0)

	balanceAfterSpend := utxoSet.GetBalance(address)
	expectedBalanceAfterSpend := uint64(4000000) // 1000000 + 3000000

	if balanceAfterSpend != expectedBalanceAfterSpend {
		t.Errorf("Expected balance after spend %d, got %d", expectedBalanceAfterSpend, balanceAfterSpend)
	}
}

// TestRemoveUTXOs tests removing UTXOs by block hash
func TestRemoveUTXOs(t *testing.T) {
	utxoSet := NewUTXOSet()

	address := Address{0x10, 0x20, 0x30}
	blockHash1 := Hash{0x04, 0x05, 0x06}
	blockHash2 := Hash{0x07, 0x08, 0x09}

	// Add UTXOs from different blocks
	utxoSet.AddUTXO(Hash{0x01}, 0, 1000000, address, blockHash1)
	utxoSet.AddUTXO(Hash{0x02}, 0, 2000000, address, blockHash1)
	utxoSet.AddUTXO(Hash{0x03}, 0, 3000000, address, blockHash2)

	// Verify initial balance
	balance := utxoSet.GetBalance(address)
	if balance != 6000000 {
		t.Errorf("Expected initial balance 6000000, got %d", balance)
	}

	// Remove UTXOs from blockHash1
	utxoSet.RemoveUTXOs(blockHash1)

	// Verify balance after removal
	balanceAfterRemoval := utxoSet.GetBalance(address)
	if balanceAfterRemoval != 3000000 {
		t.Errorf("Expected balance after removal 3000000, got %d", balanceAfterRemoval)
	}

	// Verify only UTXO from blockHash2 remains
	utxos := utxoSet.GetUTXOs(address)
	if len(utxos) != 1 {
		t.Errorf("Expected 1 UTXO after removal, got %d", len(utxos))
	}
	if utxos[0].BlockHash != blockHash2 {
		t.Errorf("Expected remaining UTXO to have BlockHash %x, got %x", blockHash2, utxos[0].BlockHash)
	}
}

// TestGetUTXOsExcludesSpent tests that spent UTXOs are not returned
func TestGetUTXOsExcludesSpent(t *testing.T) {
	utxoSet := NewUTXOSet()

	address := Address{0x10, 0x20, 0x30}

	// Add UTXOs
	utxoSet.AddUTXO(Hash{0x01}, 0, 1000000, address, Hash{0x04})
	utxoSet.AddUTXO(Hash{0x02}, 0, 2000000, address, Hash{0x05})

	// Verify both UTXOs are available
	utxos := utxoSet.GetUTXOs(address)
	if len(utxos) != 2 {
		t.Errorf("Expected 2 UTXOs, got %d", len(utxos))
	}

	// Spend one UTXO
	utxoSet.SpendUTXO(Hash{0x01}, 0)

	// Verify only one UTXO is available
	utxosAfterSpend := utxoSet.GetUTXOs(address)
	if len(utxosAfterSpend) != 1 {
		t.Errorf("Expected 1 UTXO after spending, got %d", len(utxosAfterSpend))
	}
	if utxosAfterSpend[0].TxHash != (Hash{0x02}) {
		t.Errorf("Expected remaining UTXO to have TxHash %x, got %x", Hash{0x02}, utxosAfterSpend[0].TxHash)
	}
}
