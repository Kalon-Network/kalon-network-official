package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// Keypair represents a public/private key pair
type Keypair struct {
	Private ed25519.PrivateKey
	Public  ed25519.PublicKey
}

// Generate creates a new ed25519 keypair
func Generate() (*Keypair, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate keypair: %v", err)
	}

	return &Keypair{
		Private: priv,
		Public:  pub,
	}, nil
}

// FromSeed creates a keypair from a seed
func FromSeed(seed []byte) (*Keypair, error) {
	if len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("invalid seed size: expected %d, got %d", ed25519.SeedSize, len(seed))
	}

	priv := ed25519.NewKeyFromSeed(seed)
	pub := priv.Public().(ed25519.PublicKey)

	return &Keypair{
		Private: priv,
		Public:  pub,
	}, nil
}

// GetSeed returns the seed of the private key
func (kp *Keypair) GetSeed() []byte {
	return kp.Private.Seed()
}

// GetSeedHex returns the seed as a hex string
func (kp *Keypair) GetSeedHex() string {
	return hex.EncodeToString(kp.GetSeed())
}

// GetPublicHex returns the public key as a hex string
func (kp *Keypair) GetPublicHex() string {
	return hex.EncodeToString(kp.Public)
}

// Sign signs a message with the private key
func (kp *Keypair) Sign(message []byte) ([]byte, error) {
	return ed25519.Sign(kp.Private, message), nil
}

// Verify verifies a signature with the public key
func (kp *Keypair) Verify(message []byte, signature []byte) bool {
	return ed25519.Verify(kp.Public, message, signature)
}

// PubKeyHash calculates the hash of a public key
func PubKeyHash(pub ed25519.PublicKey) [20]byte {
	hash := sha256.Sum256(pub)
	var result [20]byte
	copy(result[:], hash[:20])
	return result
}

// AddressFromPubKey creates an address from a public key
func AddressFromPubKey(pub ed25519.PublicKey) [20]byte {
	return PubKeyHash(pub)
}

// String returns a string representation of the keypair
func (kp *Keypair) String() string {
	return fmt.Sprintf("Keypair{Public: %s, Private: %s}",
		kp.GetPublicHex(), kp.GetSeedHex())
}
