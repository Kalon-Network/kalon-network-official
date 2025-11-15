package crypto

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// Bech32Encoder handles bech32 encoding/decoding
type Bech32Encoder struct {
	charset string
}

// NewBech32Encoder creates a new bech32 encoder
func NewBech32Encoder() *Bech32Encoder {
	return &Bech32Encoder{
		charset: "qpzry9x8gf2tvdw0s3jn54khce6mua7l",
	}
}

// AddressToBech32 converts an address to bech32 format
func AddressToBech32(address [20]byte, hrp string) (string, error) {
	encoder := NewBech32Encoder()
	return encoder.Encode(hrp, address[:])
}

// Bech32ToAddress converts a bech32 string to an address
func Bech32ToAddress(bech32 string) ([20]byte, error) {
	encoder := NewBech32Encoder()
	_, data, err := encoder.Decode(bech32)
	if err != nil {
		return [20]byte{}, err
	}

	if len(data) != 20 {
		return [20]byte{}, fmt.Errorf("invalid address length: expected 20, got %d", len(data))
	}

	var address [20]byte
	copy(address[:], data)

	return address, nil
}

// Encode encodes data to bech32 format
func (be *Bech32Encoder) Encode(hrp string, data []byte) (string, error) {
	// Convert 8-bit groups to 5-bit groups
	converted, err := be.convertBits(data, 8, 5, true)
	if err != nil {
		return "", err
	}

	// Create checksum
	checksum := be.createChecksum(hrp, converted)

	// Combine data and checksum
	combined := append(converted, checksum...)

	// Encode to string
	result := hrp + "1"
	for _, b := range combined {
		if int(b) >= len(be.charset) {
			return "", fmt.Errorf("invalid data byte: %d", b)
		}
		result += string(be.charset[b])
	}

	return result, nil
}

// Decode decodes a bech32 string
func (be *Bech32Encoder) Decode(bech32 string) (string, []byte, error) {
	// Find the separator
	sep := strings.LastIndex(bech32, "1")
	if sep < 1 || sep+7 > len(bech32) || sep+1+6 > len(bech32) {
		return "", nil, fmt.Errorf("invalid bech32 string")
	}

	hrp := bech32[:sep]
	data := bech32[sep+1:]

	// Validate characters
	for _, c := range data {
		if !strings.ContainsRune(be.charset, c) {
			return "", nil, fmt.Errorf("invalid character: %c", c)
		}
	}

	// Convert to bytes
	decoded := make([]byte, len(data))
	for i, c := range data {
		decoded[i] = byte(strings.IndexRune(be.charset, c))
	}

	// Verify checksum
	if !be.verifyChecksum(hrp, decoded) {
		return "", nil, fmt.Errorf("invalid checksum")
	}

	// Remove checksum
	decoded = decoded[:len(decoded)-6]

	// Convert 5-bit groups to 8-bit groups
	converted, err := be.convertBits(decoded, 5, 8, false)
	if err != nil {
		return "", nil, err
	}

	return hrp, converted, nil
}

// convertBits converts between bit groups
func (be *Bech32Encoder) convertBits(data []byte, fromBits, toBits uint, pad bool) ([]byte, error) {
	acc := uint(0)
	bits := uint(0)
	result := make([]byte, 0)
	maxValue := uint((1 << toBits) - 1)
	maxAcc := uint((1 << (fromBits + toBits - 1)) - 1)

	for _, b := range data {
		acc = ((acc << fromBits) | uint(b)) & maxAcc
		bits += fromBits

		for bits >= toBits {
			bits -= toBits
			result = append(result, byte((acc>>bits)&maxValue))
		}
	}

	if pad {
		if bits > 0 {
			result = append(result, byte((acc<<(toBits-bits))&maxValue))
		}
	} else if bits >= fromBits || ((acc<<(toBits-bits))&maxValue) != 0 {
		return nil, fmt.Errorf("invalid padding")
	}

	return result, nil
}

// createChecksum creates a bech32 checksum
func (be *Bech32Encoder) createChecksum(hrp string, data []byte) []byte {
	// Create polynomial
	values := make([]byte, 0, len(hrp)+len(data)+6)

	// Add HRP
	for _, c := range hrp {
		values = append(values, byte(c>>5))
	}
	values = append(values, 0)
	for _, c := range hrp {
		values = append(values, byte(c&0x1f))
	}

	// Add data
	values = append(values, data...)

	// Add padding
	values = append(values, 0, 0, 0, 0, 0, 0)

	// Calculate checksum
	polymod := be.polymod(values)
	polymod ^= 1

	checksum := make([]byte, 6)
	for i := 0; i < 6; i++ {
		checksum[i] = byte((polymod >> (5 * (5 - i))) & 0x1f)
	}

	return checksum
}

// verifyChecksum verifies a bech32 checksum
func (be *Bech32Encoder) verifyChecksum(hrp string, data []byte) bool {
	// Create polynomial
	values := make([]byte, 0, len(hrp)+len(data))

	// Add HRP
	for _, c := range hrp {
		values = append(values, byte(c>>5))
	}
	values = append(values, 0)
	for _, c := range hrp {
		values = append(values, byte(c&0x1f))
	}

	// Add data
	values = append(values, data...)

	// Calculate checksum
	polymod := be.polymod(values)
	return polymod == 1
}

// polymod calculates the bech32 polynomial
func (be *Bech32Encoder) polymod(values []byte) uint {
	gen := []uint{0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3}

	chk := uint(1)
	for _, v := range values {
		b := chk >> 25
		chk = (chk&0x1ffffff)<<5 ^ uint(v)
		for i := 0; i < 5; i++ {
			if (b>>uint(i))&1 == 1 {
				chk ^= gen[i]
			}
		}
	}

	return chk
}

// ValidateBech32 validates a bech32 string
func ValidateBech32(bech32 string) bool {
	encoder := NewBech32Encoder()
	_, _, err := encoder.Decode(bech32)
	return err == nil
}

// GetHRPFromBech32 extracts the human-readable part from a bech32 string
func GetHRPFromBech32(bech32 string) (string, error) {
	sep := strings.LastIndex(bech32, "1")
	if sep < 1 {
		return "", fmt.Errorf("invalid bech32 string")
	}

	return bech32[:sep], nil
}

// IsValidKalonAddress checks if a string is a valid Kalon address
func IsValidKalonAddress(address string) bool {
	if !ValidateBech32(address) {
		return false
	}

	hrp, err := GetHRPFromBech32(address)
	if err != nil {
		return false
	}

	return hrp == "kalon"
}

// AddressFromHex converts a hex string to an address
func AddressFromHex(hexStr string) ([20]byte, error) {
	// Remove 0x prefix if present
	if strings.HasPrefix(hexStr, "0x") {
		hexStr = hexStr[2:]
	}

	// Decode hex
	data, err := hex.DecodeString(hexStr)
	if err != nil {
		return [20]byte{}, fmt.Errorf("invalid hex string: %v", err)
	}

	if len(data) != 20 {
		return [20]byte{}, fmt.Errorf("invalid address length: expected 20, got %d", len(data))
	}

	var address [20]byte
	copy(address[:], data)

	return address, nil
}

// AddressToHex converts an address to hex string
func AddressToHex(address [20]byte) string {
	return "0x" + hex.EncodeToString(address[:])
}

// DecodeBech32 decodes a bech32 string to bytes (for use in miner)
func DecodeBech32(bech32Str string) ([]byte, error) {
	encoder := NewBech32Encoder()
	_, data, err := encoder.Decode(bech32Str)
	return data, err
}