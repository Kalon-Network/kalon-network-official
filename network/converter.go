package network

import (
	"encoding/hex"
	"fmt"

	"github.com/kalon-network/kalon/core"
)

// ConvertCoreBlockToNetworkBlock converts a core.Block to network.Block
func ConvertCoreBlockToNetworkBlock(coreBlock *core.Block) *Block {
	if coreBlock == nil {
		return nil
	}

	// Convert header
	header := BlockHeader{
		Number:      coreBlock.Header.Number,
		Timestamp:   coreBlock.Header.Timestamp,
		Difficulty:  coreBlock.Header.Difficulty,
		Nonce:       coreBlock.Header.Nonce,
		TxCount:     coreBlock.Header.TxCount,
		NetworkFee:  coreBlock.Header.NetworkFee,
		TreasuryFee: coreBlock.Header.TreasuryFee,
	}

	// Copy hashes
	copy(header.ParentHash[:], coreBlock.Header.ParentHash[:])
	copy(header.MerkleRoot[:], coreBlock.Header.MerkleRoot[:])
	copy(header.Miner[:], coreBlock.Header.Miner[:])

	// Convert transactions
	txs := make([]Transaction, 0, len(coreBlock.Txs))
	for _, coreTx := range coreBlock.Txs {
		tx := Transaction{
			Amount:    coreTx.Amount,
			Nonce:     coreTx.Nonce,
			Fee:       coreTx.Fee,
			GasUsed:   coreTx.GasUsed,
			GasPrice:  coreTx.GasPrice,
			Data:      coreTx.Data,
			Signature: coreTx.Signature,
		}

		// Copy addresses and hash
		copy(tx.From[:], coreTx.From[:])
		copy(tx.To[:], coreTx.To[:])
		copy(tx.Hash[:], coreTx.Hash[:])

		txs = append(txs, tx)
	}

	// Create block
	block := &Block{
		Header: header,
		Txs:    txs,
	}

	// Copy block hash
	copy(block.Hash[:], coreBlock.Hash[:])

	return block
}

// ConvertNetworkBlockToCoreBlock converts a network.Block to core.Block
func ConvertNetworkBlockToCoreBlock(networkBlock *Block) (*core.Block, error) {
	if networkBlock == nil {
		return nil, fmt.Errorf("network block is nil")
	}

	// Convert header
	header := core.BlockHeader{
		Number:      networkBlock.Header.Number,
		Timestamp:   networkBlock.Header.Timestamp,
		Difficulty:  networkBlock.Header.Difficulty,
		Nonce:       networkBlock.Header.Nonce,
		TxCount:     networkBlock.Header.TxCount,
		NetworkFee:  networkBlock.Header.NetworkFee,
		TreasuryFee: networkBlock.Header.TreasuryFee,
	}

	// Copy hashes
	copy(header.ParentHash[:], networkBlock.Header.ParentHash[:])
	copy(header.MerkleRoot[:], networkBlock.Header.MerkleRoot[:])
	copy(header.Miner[:], networkBlock.Header.Miner[:])

	// Convert transactions
	txs := make([]core.Transaction, 0, len(networkBlock.Txs))
	for _, networkTx := range networkBlock.Txs {
		tx := core.Transaction{
			Amount:    networkTx.Amount,
			Nonce:     networkTx.Nonce,
			Fee:       networkTx.Fee,
			GasUsed:   networkTx.GasUsed,
			GasPrice:  networkTx.GasPrice,
			Data:      networkTx.Data,
			Signature: networkTx.Signature,
			Timestamp: networkBlock.Header.Timestamp, // Use block timestamp if transaction timestamp not available
		}

		// Copy addresses and hash
		copy(tx.From[:], networkTx.From[:])
		copy(tx.To[:], networkTx.To[:])
		copy(tx.Hash[:], networkTx.Hash[:])

		txs = append(txs, tx)
	}

	// Create block
	block := &core.Block{
		Header: header,
		Txs:    txs,
	}

	// Copy block hash
	copy(block.Hash[:], networkBlock.Hash[:])

	// Recalculate hash to verify
	calculatedHash := block.CalculateHash()
	if block.Hash != calculatedHash {
		return nil, fmt.Errorf("block hash mismatch: expected %x, got %x", block.Hash, calculatedHash)
	}

	return block, nil
}

// ConvertCoreTransactionToNetworkTransaction converts a core.Transaction to network.Transaction
func ConvertCoreTransactionToNetworkTransaction(coreTx *core.Transaction) *Transaction {
	if coreTx == nil {
		return nil
	}

	tx := Transaction{
		Amount:    coreTx.Amount,
		Nonce:     coreTx.Nonce,
		Fee:       coreTx.Fee,
		GasUsed:   coreTx.GasUsed,
		GasPrice:  coreTx.GasPrice,
		Data:      coreTx.Data,
		Signature: coreTx.Signature,
	}

	// Copy addresses and hash
	copy(tx.From[:], coreTx.From[:])
	copy(tx.To[:], coreTx.To[:])
	copy(tx.Hash[:], coreTx.Hash[:])

	return &tx
}

// ConvertNetworkTransactionToCoreTransaction converts a network.Transaction to core.Transaction
func ConvertNetworkTransactionToCoreTransaction(networkTx *Transaction) *core.Transaction {
	if networkTx == nil {
		return nil
	}

	tx := core.Transaction{
		Amount:    networkTx.Amount,
		Nonce:     networkTx.Nonce,
		Fee:       networkTx.Fee,
		GasUsed:   networkTx.GasUsed,
		GasPrice:  networkTx.GasPrice,
		Data:      networkTx.Data,
		Signature: networkTx.Signature,
	}

	// Copy addresses and hash
	copy(tx.From[:], networkTx.From[:])
	copy(tx.To[:], networkTx.To[:])
	copy(tx.Hash[:], networkTx.Hash[:])

	return &tx
}

// BlockHashString returns the hex string representation of a block hash
func BlockHashString(hash [32]byte) string {
	return hex.EncodeToString(hash[:])
}
