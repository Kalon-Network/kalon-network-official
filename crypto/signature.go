package crypto

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"fmt"

	"github.com/kalon-network/kalon/core"
)

// Signature represents a cryptographic signature
type Signature struct {
	R [32]byte
	S [32]byte
}

// SignData signs arbitrary data with a keypair
func SignData(keypair *Keypair, data []byte) ([]byte, error) {
	return keypair.Sign(data)
}

// VerifySignature verifies a signature against data and public key
func VerifySignature(publicKey ed25519.PublicKey, data []byte, signature []byte) bool {
	return ed25519.Verify(publicKey, data, signature)
}

// SignTransaction signs a transaction with a keypair
func SignTransaction(keypair *Keypair, tx *core.Transaction) error {
	// Create message to sign
	message := createTransactionMessage(tx)

	// Sign the message
	signature, err := keypair.Sign(message)
	if err != nil {
		return fmt.Errorf("failed to sign transaction: %v", err)
	}

	// Set signature and public key
	tx.Signature = signature
	tx.PublicKey = keypair.Public

	return nil
}

// VerifyTransaction verifies a transaction signature
func VerifyTransaction(tx *core.Transaction, publicKey ed25519.PublicKey) bool {
	// Create message that was signed
	message := createTransactionMessage(tx)

	// Verify signature
	return ed25519.Verify(publicKey, message, tx.Signature)
}

// createTransactionMessage creates the message to sign for a transaction
func createTransactionMessage(tx *core.Transaction) []byte {
	// Create message from transaction fields (excluding signature)
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

// SignBlock signs a block with a keypair
func SignBlock(keypair *Keypair, block *core.Block) error {
	// Create message to sign (block header without signature fields)
	message := createBlockMessage(block)

	// Sign the message
	signature, err := keypair.Sign(message)
	if err != nil {
		return fmt.Errorf("failed to sign block: %v", err)
	}

	// Store signature in block data (simplified)
	// In a real implementation, you might have a signature field in the block
	_ = signature

	return nil
}

// createBlockMessage creates the message to sign for a block
func createBlockMessage(block *core.Block) []byte {
	// Create message from block header fields
	data := make([]byte, 0, 200)
	data = append(data, block.Header.ParentHash.Bytes()...)

	// Use compatible binary encoding for older Go versions
	numberBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(numberBytes, block.Header.Number)
	data = append(data, numberBytes...)

	timestampBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(timestampBytes, uint64(block.Header.Timestamp.Unix()))
	data = append(data, timestampBytes...)

	difficultyBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(difficultyBytes, block.Header.Difficulty)
	data = append(data, difficultyBytes...)

	data = append(data, block.Header.Miner.Bytes()...)

	nonceBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(nonceBytes, block.Header.Nonce)
	data = append(data, nonceBytes...)

	data = append(data, block.Header.MerkleRoot.Bytes()...)

	txCountBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(txCountBytes, block.Header.TxCount)
	data = append(data, txCountBytes...)

	networkFeeBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(networkFeeBytes, block.Header.NetworkFee)
	data = append(data, networkFeeBytes...)

	treasuryFeeBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(treasuryFeeBytes, block.Header.TreasuryFee)
	data = append(data, treasuryFeeBytes...)

	return data
}

// CreateTransaction creates a new transaction
func CreateTransaction(from [20]byte, to [20]byte, amount uint64, nonce uint64, fee uint64, data []byte) *core.Transaction {
	return &core.Transaction{
		From:      from,
		To:        to,
		Amount:    amount,
		Nonce:     nonce,
		Fee:       fee,
		GasUsed:   1,   // Default gas usage
		GasPrice:  fee, // Gas price equals fee for simplicity
		Data:      data,
		Signature: []byte{},   // Will be set when signed
		Hash:      [32]byte{}, // Will be calculated
	}
}

// CalculateTransactionHash calculates the hash of a transaction
func CalculateTransactionHash(tx *core.Transaction) [32]byte {
	message := createTransactionMessage(tx)
	hash := sha256.Sum256(message)
	return [32]byte(hash)
}

// ValidateTransactionSignature validates a transaction signature
func ValidateTransactionSignature(tx *core.Transaction, publicKey ed25519.PublicKey) error {
	if len(tx.Signature) == 0 {
		return fmt.Errorf("transaction has no signature")
	}

	if !VerifyTransaction(tx, publicKey) {
		return fmt.Errorf("invalid transaction signature")
	}

	return nil
}

// RecoverPublicKey recovers the public key from a signature and message
func RecoverPublicKey(message []byte, signature []byte) (ed25519.PublicKey, error) {
	// Ed25519 doesn't support key recovery
	// This is a placeholder for compatibility
	return nil, fmt.Errorf("ed25519 does not support key recovery")
}

// SignMessage signs an arbitrary message
func SignMessage(keypair *Keypair, message string) ([]byte, error) {
	return keypair.Sign([]byte(message))
}

// VerifyMessage verifies a message signature
func VerifyMessage(publicKey ed25519.PublicKey, message string, signature []byte) bool {
	return ed25519.Verify(publicKey, []byte(message), signature)
}

// CreateMultiSigSignature creates a signature for a multi-signature transaction
func CreateMultiSigSignature(keypair *Keypair, message []byte, index int) ([]byte, error) {
	// Add index to message to prevent signature reuse
	indexedMessage := make([]byte, len(message)+4)
	copy(indexedMessage, message)
	binary.BigEndian.PutUint32(indexedMessage[len(message):], uint32(index))

	return keypair.Sign(indexedMessage)
}

// VerifyMultiSigSignature verifies a multi-signature
func VerifyMultiSigSignature(publicKey ed25519.PublicKey, message []byte, signature []byte, index int) bool {
	// Add index to message
	indexedMessage := make([]byte, len(message)+4)
	copy(indexedMessage, message)
	binary.BigEndian.PutUint32(indexedMessage[len(message):], uint32(index))

	return ed25519.Verify(publicKey, indexedMessage, signature)
}
