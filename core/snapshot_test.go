package core

import (
	"testing"
)

// TestSnapshotFromGenesis tests snapshot restoration from genesis config
func TestSnapshotFromGenesis(t *testing.T) {
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
		Snapshot: &SnapshotConfig{
			Enabled: true,
			Height:  1,
			Balances: map[string]uint64{
				"df7552b7f9f25986fd33511e68d5ba5e37fd12d3": 1000000000,
				"a1b2c3d4e5f6789012345678901234567890abcd": 500000000,
			},
		},
	}

	bc := NewBlockchainV2(genesis, nil)

	// Check that snapshot balances were restored
	addr1 := AddressFromString("kalon1df7552b7f9f25986fd33511e68d5ba5e37fd12d3")
	balance1 := bc.GetBalance(addr1)
	if balance1 != 1000000000 {
		t.Errorf("Expected balance 1000000000 for address 1, got %d", balance1)
	}

	addr2 := AddressFromString("kalon1a1b2c3d4e5f6789012345678901234567890abcd")
	balance2 := bc.GetBalance(addr2)
	if balance2 != 500000000 {
		t.Errorf("Expected balance 500000000 for address 2, got %d", balance2)
	}
}

// TestSnapshotWithoutGenesis tests that blockchain works without snapshot
func TestSnapshotWithoutGenesis(t *testing.T) {
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
		// No snapshot
	}

	bc := NewBlockchainV2(genesis, nil)
	if bc == nil {
		t.Fatal("Expected non-nil blockchain")
	}

	// Should still work without snapshot
	if bc.GetHeight() != 0 {
		t.Errorf("Expected height 0, got %d", bc.GetHeight())
	}
}

// TestSnapshotManager tests snapshot manager functionality
func TestSnapshotManager(t *testing.T) {
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
	}

	bc := NewBlockchainV2(genesis, nil)

	// Create a snapshot
	snapshot, err := bc.SnapshotManager.CreateSnapshot(bc)
	if err != nil {
		t.Fatalf("Failed to create snapshot: %v", err)
	}

	if snapshot == nil {
		t.Fatal("Expected non-nil snapshot")
	}

	if snapshot.Height != 0 {
		t.Errorf("Expected snapshot height 0, got %d", snapshot.Height)
	}

	// Get snapshot
	retrievedSnapshot := bc.SnapshotManager.GetSnapshot()
	if retrievedSnapshot == nil {
		t.Fatal("Expected non-nil retrieved snapshot")
	}

	if retrievedSnapshot.Height != snapshot.Height {
		t.Errorf("Expected snapshot height %d, got %d", snapshot.Height, retrievedSnapshot.Height)
	}
}
