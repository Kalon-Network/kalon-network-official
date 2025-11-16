package core

import (
	"testing"
	"time"
)

// TestNewMempool tests the creation of a new mempool
func TestNewMempool(t *testing.T) {
	mempool := NewMempool()
	if mempool == nil {
		t.Fatal("Expected non-nil mempool")
	}
	if mempool.transactions == nil {
		t.Fatal("Expected non-nil transactions map")
	}
}

// TestAddTransaction tests adding a transaction to the mempool
func TestAddTransaction(t *testing.T) {
	mempool := NewMempool()

	tx := &Transaction{
		From:      Address{0x01},
		To:        Address{0x02},
		Amount:    1000000,
		Fee:       1000,
		Timestamp: time.Now(),
	}
	tx.Hash = tx.CalculateHash()

	mempool.AddTransaction(tx)

	// Verify transaction was added
	pendingTxs := mempool.GetPendingTransactions()
	if len(pendingTxs) != 1 {
		t.Fatalf("Expected 1 pending transaction, got %d", len(pendingTxs))
	}

	if pendingTxs[0].Hash != tx.Hash {
		t.Errorf("Expected transaction hash %x, got %x", tx.Hash, pendingTxs[0].Hash)
	}
}

// TestGetPendingTransactions tests getting all pending transactions
func TestGetPendingTransactions(t *testing.T) {
	mempool := NewMempool()

	// Add multiple transactions
	tx1 := &Transaction{
		From:      Address{0x01},
		To:        Address{0x02},
		Amount:    1000000,
		Fee:       1000,
		Timestamp: time.Now(),
	}
	tx1.Hash = tx1.CalculateHash()

	tx2 := &Transaction{
		From:      Address{0x03},
		To:        Address{0x04},
		Amount:    2000000,
		Fee:       2000,
		Timestamp: time.Now(),
	}
	tx2.Hash = tx2.CalculateHash()

	mempool.AddTransaction(tx1)
	mempool.AddTransaction(tx2)

	// Get all pending transactions
	pendingTxs := mempool.GetPendingTransactions()
	if len(pendingTxs) != 2 {
		t.Errorf("Expected 2 pending transactions, got %d", len(pendingTxs))
	}

	// Verify transactions are present
	foundTx1 := false
	foundTx2 := false
	for _, tx := range pendingTxs {
		if tx.Hash == tx1.Hash {
			foundTx1 = true
		}
		if tx.Hash == tx2.Hash {
			foundTx2 = true
		}
	}

	if !foundTx1 {
		t.Error("Transaction 1 not found in pending transactions")
	}
	if !foundTx2 {
		t.Error("Transaction 2 not found in pending transactions")
	}
}

// TestRemoveTransaction tests removing a transaction from the mempool
func TestRemoveTransaction(t *testing.T) {
	mempool := NewMempool()

	tx := &Transaction{
		From:      Address{0x01},
		To:        Address{0x02},
		Amount:    1000000,
		Fee:       1000,
		Timestamp: time.Now(),
	}
	tx.Hash = tx.CalculateHash()

	mempool.AddTransaction(tx)

	// Verify transaction is in mempool
	pendingTxs := mempool.GetPendingTransactions()
	if len(pendingTxs) != 1 {
		t.Fatalf("Expected 1 pending transaction, got %d", len(pendingTxs))
	}

	// Remove transaction
	mempool.RemoveTransaction(tx.Hash)

	// Verify transaction is removed
	pendingTxsAfterRemoval := mempool.GetPendingTransactions()
	if len(pendingTxsAfterRemoval) != 0 {
		t.Errorf("Expected 0 pending transactions after removal, got %d", len(pendingTxsAfterRemoval))
	}
}

// TestClear tests clearing all transactions from the mempool
func TestClear(t *testing.T) {
	mempool := NewMempool()

	// Add multiple transactions
	tx1 := &Transaction{
		From:      Address{0x01},
		To:        Address{0x02},
		Amount:    1000000,
		Fee:       1000,
		Timestamp: time.Now(),
	}
	tx1.Hash = tx1.CalculateHash()

	tx2 := &Transaction{
		From:      Address{0x03},
		To:        Address{0x04},
		Amount:    2000000,
		Fee:       2000,
		Timestamp: time.Now(),
	}
	tx2.Hash = tx2.CalculateHash()

	mempool.AddTransaction(tx1)
	mempool.AddTransaction(tx2)

	// Verify transactions are in mempool
	pendingTxs := mempool.GetPendingTransactions()
	if len(pendingTxs) != 2 {
		t.Fatalf("Expected 2 pending transactions, got %d", len(pendingTxs))
	}

	// Clear mempool
	mempool.Clear()

	// Verify mempool is empty
	pendingTxsAfterClear := mempool.GetPendingTransactions()
	if len(pendingTxsAfterClear) != 0 {
		t.Errorf("Expected 0 pending transactions after clear, got %d", len(pendingTxsAfterClear))
	}
}

// TestAddDuplicateTransaction tests that adding the same transaction twice only keeps one
func TestAddDuplicateTransaction(t *testing.T) {
	mempool := NewMempool()

	tx := &Transaction{
		From:      Address{0x01},
		To:        Address{0x02},
		Amount:    1000000,
		Fee:       1000,
		Timestamp: time.Now(),
	}
	tx.Hash = tx.CalculateHash()

	// Add transaction twice
	mempool.AddTransaction(tx)
	mempool.AddTransaction(tx)

	// Verify only one transaction is in mempool
	pendingTxs := mempool.GetPendingTransactions()
	if len(pendingTxs) != 1 {
		t.Errorf("Expected 1 pending transaction (duplicate should be ignored), got %d", len(pendingTxs))
	}
}
