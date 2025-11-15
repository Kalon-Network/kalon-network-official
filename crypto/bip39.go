package crypto

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/kalon-network/kalon/core"
	"github.com/tyler-smith/go-bip39"
)

// BIP39Manager handles BIP-39 mnemonic operations
type BIP39Manager struct {
	entropyLength int
}

// NewBIP39Manager creates a new BIP-39 manager
func NewBIP39Manager() *BIP39Manager {
	return &BIP39Manager{
		entropyLength: 256, // 256 bits = 24 words
	}
}

// GenerateMnemonic generates a new mnemonic phrase
func (bm *BIP39Manager) GenerateMnemonic() (string, error) {
	entropy, err := bm.generateEntropy()
	if err != nil {
		return "", fmt.Errorf("failed to generate entropy: %v", err)
	}

	mnemonic, err := bip39.NewMnemonic(entropy)
	if err != nil {
		return "", fmt.Errorf("failed to create mnemonic: %v", err)
	}

	return mnemonic, nil
}

// MnemonicToSeed converts a mnemonic to a seed
func (bm *BIP39Manager) MnemonicToSeed(mnemonic string, passphrase string) ([]byte, error) {
	if !bip39.IsMnemonicValid(mnemonic) {
		return nil, fmt.Errorf("invalid mnemonic")
	}

	seed := bip39.NewSeed(mnemonic, passphrase)
	return seed, nil
}

// SeedToKeypair converts a seed to a keypair
func (bm *BIP39Manager) SeedToKeypair(seed []byte) (*Keypair, error) {
	// Use first 32 bytes as ed25519 seed
	if len(seed) < 32 {
		return nil, fmt.Errorf("seed too short: need at least 32 bytes")
	}

	ed25519Seed := seed[:32]
	return FromSeed(ed25519Seed)
}

// MnemonicToKeypair converts a mnemonic directly to a keypair
func (bm *BIP39Manager) MnemonicToKeypair(mnemonic string, passphrase string) (*Keypair, error) {
	seed, err := bm.MnemonicToSeed(mnemonic, passphrase)
	if err != nil {
		return nil, err
	}

	return bm.SeedToKeypair(seed)
}

// ValidateMnemonic validates a mnemonic phrase
func (bm *BIP39Manager) ValidateMnemonic(mnemonic string) bool {
	return bip39.IsMnemonicValid(mnemonic)
}

// GetWordList returns the BIP-39 word list
func (bm *BIP39Manager) GetWordList() []string {
	return bip39.GetWordList()
}

// generateEntropy generates cryptographically secure entropy
func (bm *BIP39Manager) generateEntropy() ([]byte, error) {
	entropy := make([]byte, bm.entropyLength/8)
	_, err := rand.Read(entropy)
	if err != nil {
		return nil, err
	}

	return entropy, nil
}

// GetEntropyLength returns the entropy length in bits
func (bm *BIP39Manager) GetEntropyLength() int {
	return bm.entropyLength
}

// GetWordCount returns the number of words in the mnemonic
func (bm *BIP39Manager) GetWordCount() int {
	return bm.entropyLength / 11 // 11 bits per word
}

// MnemonicToEntropy converts a mnemonic back to entropy
func (bm *BIP39Manager) MnemonicToEntropy(mnemonic string) ([]byte, error) {
	if !bip39.IsMnemonicValid(mnemonic) {
		return nil, fmt.Errorf("invalid mnemonic")
	}

	entropy, err := bip39.EntropyFromMnemonic(mnemonic)
	if err != nil {
		return nil, fmt.Errorf("failed to get entropy from mnemonic: %v", err)
	}

	return entropy, nil
}

// CreateWalletFromMnemonic creates a complete wallet from a mnemonic
func (bm *BIP39Manager) CreateWalletFromMnemonic(mnemonic string, passphrase string) (*Wallet, error) {
	keypair, err := bm.MnemonicToKeypair(mnemonic, passphrase)
	if err != nil {
		return nil, err
	}

	address := AddressFromPubKey(keypair.Public)

	return &Wallet{
		Mnemonic:   mnemonic,
		Passphrase: passphrase,
		Keypair:    keypair,
		Address:    address,
	}, nil
}

// Wallet represents a complete wallet
type Wallet struct {
	Mnemonic   string
	Passphrase string
	Keypair    *Keypair
	Address    [20]byte
}

// NewWallet creates a new wallet with a random mnemonic
func NewWallet(passphrase string) (*Wallet, error) {
	bm := NewBIP39Manager()
	mnemonic, err := bm.GenerateMnemonic()
	if err != nil {
		return nil, err
	}

	return bm.CreateWalletFromMnemonic(mnemonic, passphrase)
}

// GetAddressString returns the address as a bech32 string
func (w *Wallet) GetAddressString() (string, error) {
	// Use simple hex encoding with kalon1 prefix for now
	addressHex := hex.EncodeToString(w.Address[:])
	return "kalon1" + addressHex, nil
}

// GetAddress returns the wallet address
func (w *Wallet) GetAddress() core.Address {
	return core.Address(w.Address)
}

// SignTransaction signs a transaction
func (w *Wallet) SignTransaction(tx *core.Transaction) error {
	// Create message to sign (transaction data without signature)
	message := w.createTransactionMessage(tx)

	signature, err := w.Keypair.Sign(message)
	if err != nil {
		return fmt.Errorf("failed to sign transaction: %v", err)
	}

	// Set signature and public key
	tx.Signature = signature
	tx.PublicKey = w.Keypair.Public

	return nil
}

// createTransactionMessage creates the message to sign for a transaction
func (w *Wallet) createTransactionMessage(tx *core.Transaction) []byte {
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

// VerifyTransaction verifies a transaction signature
func (w *Wallet) VerifyTransaction(tx *core.Transaction) bool {
	message := w.createTransactionMessage(tx)
	return w.Keypair.Verify(message, tx.Signature)
}

// String returns a string representation of the wallet
func (w *Wallet) String() string {
	addressStr, _ := w.GetAddressString()
	return fmt.Sprintf("Wallet{Address: %s, Mnemonic: %s...}",
		addressStr, w.Mnemonic[:20])
}
