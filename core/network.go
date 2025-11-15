package core

import (
	"fmt"
	"os"
	"path/filepath"
)

// NetworkType represents the type of network
type NetworkType string

const (
	CommunityTestnet NetworkType = "community-testnet"
	Testnet          NetworkType = "testnet"
	Mainnet          NetworkType = "mainnet"
)

// NetworkConfig represents network configuration
type NetworkConfig struct {
	Type        NetworkType
	GenesisFile string
	DataDir     string
	RPCAddr     string
	P2PAddr     string
	TokenSymbol string
	AddressHRP  string
}

// GetNetworkConfig returns the configuration for a specific network
func GetNetworkConfig(networkType NetworkType) (*NetworkConfig, error) {
	switch networkType {
	case CommunityTestnet:
		return &NetworkConfig{
			Type:        CommunityTestnet,
			GenesisFile: "genesis/community-testnet.json",
			DataDir:     "./data-community-testnet",
			RPCAddr:     "localhost:16315",
			P2PAddr:     "localhost:17334",
			TokenSymbol: "tKALON",
			AddressHRP:  "tkalon",
		}, nil
	case Testnet:
		return &NetworkConfig{
			Type:        Testnet,
			GenesisFile: "genesis/testnet.json",
			DataDir:     "./data-testnet",
			RPCAddr:     "localhost:16316",
			P2PAddr:     "localhost:17335",
			TokenSymbol: "tKALON",
			AddressHRP:  "tkalon",
		}, nil
	case Mainnet:
		return &NetworkConfig{
			Type:        Mainnet,
			GenesisFile: "genesis/mainnet.json",
			DataDir:     "./data-mainnet",
			RPCAddr:     "localhost:16314",
			P2PAddr:     "localhost:17333",
			TokenSymbol: "KALON",
			AddressHRP:  "kalon",
		}, nil
	default:
		return nil, fmt.Errorf("unknown network type: %s", networkType)
	}
}

// GetNetworkTypeFromString converts string to NetworkType
func GetNetworkTypeFromString(s string) (NetworkType, error) {
	switch s {
	case "community-testnet", "community", "ct":
		return CommunityTestnet, nil
	case "testnet", "test", "t":
		return Testnet, nil
	case "mainnet", "main", "m":
		return Mainnet, nil
	default:
		return "", fmt.Errorf("unknown network type: %s", s)
	}
}

// SetupNetworkDirectory creates the network data directory
func (nc *NetworkConfig) SetupNetworkDirectory() error {
	return os.MkdirAll(nc.DataDir, 0755)
}

// GetGenesisPath returns the full path to the genesis file
func (nc *NetworkConfig) GetGenesisPath() string {
	return nc.GenesisFile
}

// GetDataPath returns the full path to the data directory
func (nc *NetworkConfig) GetDataPath() string {
	absPath, _ := filepath.Abs(nc.DataDir)
	return absPath
}

// IsTestnet returns true if this is a testnet
func (nc *NetworkConfig) IsTestnet() bool {
	return nc.Type == CommunityTestnet || nc.Type == Testnet
}

// IsMainnet returns true if this is mainnet
func (nc *NetworkConfig) IsMainnet() bool {
	return nc.Type == Mainnet
}

// GetTokenSymbol returns the token symbol for this network
func (nc *NetworkConfig) GetTokenSymbol() string {
	return nc.TokenSymbol
}

// GetAddressHRP returns the address human-readable prefix
func (nc *NetworkConfig) GetAddressHRP() string {
	return nc.AddressHRP
}
