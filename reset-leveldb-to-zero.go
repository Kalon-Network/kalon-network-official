package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/kalon-network/kalon/storage"
)

// resetLevelDBToZero resets LevelDB to height 0 by deleting the best_block key
func resetLevelDBToZero(dbPath string) error {
	// Open LevelDB
	levelDBStorage, err := storage.NewLevelDBStorage(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open LevelDB: %v", err)
	}
	defer levelDBStorage.Close()

	// Delete best_block key - this is the critical key that determines the height
	// When best_block is missing, GetBestBlock() returns nil, and loadChainFromStorage()
	// will not set bc.height, so it stays at 0
	bestKey := []byte("best_block")
	if err := levelDBStorage.Delete(bestKey); err != nil {
		// If key doesn't exist, that's fine - it means we're already at height 0
		log.Printf("Note: best_block key not found or already deleted: %v", err)
	} else {
		log.Printf("✅ Deleted best_block key from LevelDB")
	}

	log.Printf("✅ LevelDB reset to height 0")
	return nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("Usage: %s <leveldb_path>\n", os.Args[0])
		fmt.Printf("Example: %s data/testnet/chaindb\n", os.Args[0])
		os.Exit(1)
	}

	dbPath := os.Args[1]
	if !filepath.IsAbs(dbPath) {
		// Make it relative to current directory
		cwd, _ := os.Getwd()
		dbPath = filepath.Join(cwd, dbPath)
	}

	fmt.Printf("Resetting LevelDB at: %s\n", dbPath)
	if err := resetLevelDBToZero(dbPath); err != nil {
		log.Fatalf("Failed to reset LevelDB: %v", err)
	}

	fmt.Println("✅ LevelDB successfully reset to height 0")
}

