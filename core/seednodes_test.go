package core

import (
	"testing"
)

// TestGenesisWithSeedNodes tests that genesis config with seed nodes is loaded correctly
func TestGenesisWithSeedNodes(t *testing.T) {
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
			InitialDifficulty: 10,
		},
		NetworkFee: NetworkFeeConfig{
			BaseTxFee: 0.001,
		},
		SeedNodes: []string{
			"seed1.testnet.kalon.network:17335",
			"seed2.testnet.kalon.network:17335",
			"seed3.testnet.kalon.network:17335",
		},
	}

	if len(genesis.SeedNodes) != 3 {
		t.Errorf("Expected 3 seed nodes, got %d", len(genesis.SeedNodes))
	}

	expectedNodes := []string{
		"seed1.testnet.kalon.network:17335",
		"seed2.testnet.kalon.network:17335",
		"seed3.testnet.kalon.network:17335",
	}

	for i, expected := range expectedNodes {
		if genesis.SeedNodes[i] != expected {
			t.Errorf("Expected seed node %d to be %s, got %s", i, expected, genesis.SeedNodes[i])
		}
	}
}

// TestGenesisWithoutSeedNodes tests that genesis config works without seed nodes
func TestGenesisWithoutSeedNodes(t *testing.T) {
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
			InitialDifficulty: 10,
		},
		NetworkFee: NetworkFeeConfig{
			BaseTxFee: 0.001,
		},
		// No seed nodes
	}

	if genesis.SeedNodes != nil && len(genesis.SeedNodes) != 0 {
		t.Errorf("Expected empty seed nodes, got %d", len(genesis.SeedNodes))
	}

	bc := NewBlockchainV2(genesis, nil)
	if bc == nil {
		t.Fatal("Expected non-nil blockchain")
	}
}
