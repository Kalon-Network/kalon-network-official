package core

import (
	"testing"
	"time"
)

// TestNewEventBus tests the creation of a new event bus
func TestNewEventBus(t *testing.T) {
	eventBus := NewEventBus()
	if eventBus == nil {
		t.Fatal("Expected non-nil event bus")
	}
	if eventBus.channels == nil {
		t.Fatal("Expected non-nil channels map")
	}
}

// TestSubscribe tests subscribing to an event
func TestSubscribe(t *testing.T) {
	eventBus := NewEventBus()

	ch := eventBus.Subscribe("testEvent")
	if ch == nil {
		t.Fatal("Expected non-nil channel")
	}

	// Verify channel was added
	eventBus.mu.RLock()
	channels := eventBus.channels["testEvent"]
	eventBus.mu.RUnlock()

	if len(channels) != 1 {
		t.Errorf("Expected 1 channel, got %d", len(channels))
	}
}

// TestEmit tests emitting an event
func TestEmit(t *testing.T) {
	eventBus := NewEventBus()

	ch := eventBus.Subscribe("testEvent")

	// Emit event
	testData := "test data"
	eventBus.Emit("testEvent", testData)

	// Verify event was received
	select {
	case data := <-ch:
		if data != testData {
			t.Errorf("Expected data %v, got %v", testData, data)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Event was not received within timeout")
	}
}

// TestEmitMultipleSubscribers tests emitting to multiple subscribers
func TestEmitMultipleSubscribers(t *testing.T) {
	eventBus := NewEventBus()

	ch1 := eventBus.Subscribe("testEvent")
	ch2 := eventBus.Subscribe("testEvent")

	// Emit event
	testData := "test data"
	eventBus.Emit("testEvent", testData)

	// Verify both subscribers received the event
	select {
	case data1 := <-ch1:
		if data1 != testData {
			t.Errorf("Expected data %v, got %v", testData, data1)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Event was not received by subscriber 1 within timeout")
	}

	select {
	case data2 := <-ch2:
		if data2 != testData {
			t.Errorf("Expected data %v, got %v", testData, data2)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Event was not received by subscriber 2 within timeout")
	}
}

// TestEmitDifferentEvents tests emitting different events
func TestEmitDifferentEvents(t *testing.T) {
	eventBus := NewEventBus()

	ch1 := eventBus.Subscribe("event1")
	ch2 := eventBus.Subscribe("event2")

	// Emit different events
	data1 := "data1"
	data2 := "data2"

	eventBus.Emit("event1", data1)
	eventBus.Emit("event2", data2)

	// Verify subscribers only receive their subscribed events
	select {
	case data := <-ch1:
		if data != data1 {
			t.Errorf("Expected data %v, got %v", data1, data)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Event1 was not received within timeout")
	}

	select {
	case data := <-ch2:
		if data != data2 {
			t.Errorf("Expected data %v, got %v", data2, data)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Event2 was not received within timeout")
	}
}

// TestEmitNoSubscribers tests emitting when there are no subscribers
func TestEmitNoSubscribers(t *testing.T) {
	eventBus := NewEventBus()

	// Emit event with no subscribers - should not panic
	eventBus.Emit("testEvent", "test data")

	// Test passes if no panic occurs
}

// TestEmitBlockAddedEvent tests the blockAdded event
func TestEmitBlockAddedEvent(t *testing.T) {
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
	eventBus := bc.GetEventBus()

	ch := eventBus.Subscribe("blockAdded")

	// Create and add a block
	miner := Address{0x01}
	block := bc.CreateNewBlockV2(miner, []Transaction{})
	if block == nil {
		t.Fatal("Expected non-nil block")
	}

	// Add block in goroutine to avoid blocking
	go func() {
		if err := bc.AddBlockV2(block); err != nil {
			t.Errorf("Failed to add block: %v", err)
		}
	}()

	// Verify event was received
	select {
	case event := <-ch:
		eventData, ok := event.(map[string]interface{})
		if !ok {
			t.Fatal("Expected event data to be a map")
		}

		blockFromEvent, ok := eventData["block"].(*Block)
		if !ok {
			t.Fatal("Expected block in event data")
		}

		if blockFromEvent.Hash != block.Hash {
			t.Errorf("Expected block hash %x, got %x", block.Hash, blockFromEvent.Hash)
		}

		height, ok := eventData["height"].(uint64)
		if !ok {
			t.Fatal("Expected height in event data")
		}

		if height != 1 {
			t.Errorf("Expected height 1, got %d", height)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("blockAdded event was not received within timeout")
	}
}
