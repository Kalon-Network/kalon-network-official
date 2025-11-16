package core

import (
	"crypto/sha256"
	"encoding/binary"
	"sync"
)

// UTXO represents an Unspent Transaction Output
type UTXO struct {
	TxHash    Hash    // Hash of the transaction that created this output
	Index     uint32  // Index of the output in the transaction
	Amount    uint64  // Amount in smallest units
	Address   Address // Owner address
	Spent     bool    // Whether this UTXO has been spent
	BlockHash Hash    // Hash of the block that created this UTXO
}

// UTXOSet manages all unspent transaction outputs
type UTXOSet struct {
	mu    sync.RWMutex
	utxos map[string]*UTXO // Key: "txHash:index"
}

// NewUTXOSet creates a new UTXO set
func NewUTXOSet() *UTXOSet {
	return &UTXOSet{
		utxos: make(map[string]*UTXO),
	}
}

// AddUTXO adds a new UTXO to the set
func (us *UTXOSet) AddUTXO(txHash Hash, index uint32, amount uint64, address Address, blockHash Hash) {
	us.mu.Lock()
	defer us.mu.Unlock()

	key := us.getKey(txHash, index)
	utxo := &UTXO{
		TxHash:    txHash,
		Index:     index,
		Amount:    amount,
		Address:   address,
		Spent:     false,
		BlockHash: blockHash,
	}
	us.utxos[key] = utxo
}

// SpendUTXO marks a UTXO as spent
func (us *UTXOSet) SpendUTXO(txHash Hash, index uint32) bool {
	us.mu.Lock()
	defer us.mu.Unlock()

	key := us.getKey(txHash, index)
	if utxo, exists := us.utxos[key]; exists && !utxo.Spent {
		utxo.Spent = true
		return true
	}
	return false
}

// RestoreUTXO restores a UTXO that was previously spent (for chain reorganization)
func (us *UTXOSet) RestoreUTXO(txHash Hash, index uint32) bool {
	us.mu.Lock()
	defer us.mu.Unlock()

	key := us.getKey(txHash, index)
	if utxo, exists := us.utxos[key]; exists && utxo.Spent {
		utxo.Spent = false
		return true
	}
	return false
}

// GetUTXOs returns all UTXOs for a given address
func (us *UTXOSet) GetUTXOs(address Address) []*UTXO {
	us.mu.RLock()
	defer us.mu.RUnlock()

	var result []*UTXO
	for _, utxo := range us.utxos {
		// Use bytes.Equal for proper comparison
		if (utxo.Address == address) && !utxo.Spent {
			result = append(result, utxo)
		}
	}
	return result
}

// GetBalance calculates the total balance for an address
func (us *UTXOSet) GetBalance(address Address) uint64 {
	utxos := us.GetUTXOs(address)
	var balance uint64
	for _, utxo := range utxos {
		balance += utxo.Amount
	}
	return balance
}

// RemoveUTXOs removes UTXOs created by a specific block (for reorgs)
func (us *UTXOSet) RemoveUTXOs(blockHash Hash) {
	us.mu.Lock()
	defer us.mu.Unlock()

	for key, utxo := range us.utxos {
		if utxo.BlockHash == blockHash {
			delete(us.utxos, key)
		}
	}
}

// GetKey creates a unique key for a UTXO (exported for use in blockchain.go)
func (us *UTXOSet) GetKey(txHash Hash, index uint32) string {
	indexBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(indexBytes, index)
	return string(txHash[:]) + ":" + string(indexBytes)
}

// getKey creates a unique key for a UTXO (internal use)
func (us *UTXOSet) getKey(txHash Hash, index uint32) string {
	return us.GetKey(txHash, index)
}

// CalculateTransactionHash calculates the hash of a transaction
func CalculateTransactionHash(tx *Transaction) Hash {
	// Create a deterministic hash of the transaction
	data := make([]byte, 0, 200)

	// Add inputs (for block rewards this will be empty but still adds uniqueness)
	for _, input := range tx.Inputs {
		data = append(data, input.PreviousTxHash[:]...)
		inputIndexBytes := make([]byte, 4)
		binary.BigEndian.PutUint32(inputIndexBytes, input.Index)
		data = append(data, inputIndexBytes...)
	}

	// Add outputs
	for _, output := range tx.Outputs {
		data = append(data, output.Address[:]...)
		amountBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(amountBytes, output.Amount)
		data = append(data, amountBytes...)
	}

	// Add timestamp (CRITICAL: includes nanosecond precision for uniqueness)
	timestampBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(timestampBytes, uint64(tx.Timestamp.UnixNano()))
	data = append(data, timestampBytes...)

	// Add from/to addresses for uniqueness
	data = append(data, tx.From[:]...)
	data = append(data, tx.To[:]...)

	hash := sha256.Sum256(data)
	return Hash(hash)
}
