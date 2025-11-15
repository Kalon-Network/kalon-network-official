package storage

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/kalon-network/kalon/core"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

// LevelDBStorage implements the Storage interface using LevelDB
type LevelDBStorage struct {
	db   *leveldb.DB
	path string
	mu   sync.RWMutex
}

// NewLevelDBStorage creates a new LevelDB storage instance
func NewLevelDBStorage(path string) (*LevelDBStorage, error) {
	// Ensure directory exists
	if err := ensureDir(path); err != nil {
		return nil, fmt.Errorf("failed to create directory: %v", err)
	}

	// Try to open database
	db, err := leveldb.OpenFile(path, &opt.Options{})
	if err != nil {
		// If database is corrupted, try to recover it
		log.Printf("⚠️ Failed to open LevelDB: %v", err)
		log.Printf("🔧 Attempting to recover database...")
		
		// Check if error is "resource temporarily unavailable" (database locked)
		if strings.Contains(err.Error(), "resource temporarily unavailable") || strings.Contains(err.Error(), "resource busy") {
			log.Printf("⚠️ LevelDB is locked by another process")
			log.Printf("💡 Deleting locked database and starting fresh...")
			// Delete locked database directory
			if rmErr := os.RemoveAll(path); rmErr != nil {
				return nil, fmt.Errorf("failed to remove locked database: %v (original error: %v)", rmErr, err)
			}
			// Recreate directory
			if err := ensureDir(path); err != nil {
				return nil, fmt.Errorf("failed to recreate directory: %v", err)
			}
			// Try to open again
			db, err = leveldb.OpenFile(path, &opt.Options{})
			if err != nil {
				return nil, fmt.Errorf("failed to open database after removing lock: %v", err)
			}
			log.Printf("✅ Created fresh database at %s", path)
		} else {
			// Try to recover the database
			_, recoverErr := leveldb.RecoverFile(path, nil)
			if recoverErr != nil {
				log.Printf("⚠️ Failed to recover database: %v", recoverErr)
				log.Printf("💡 Deleting corrupted database and starting fresh...")
				// Delete corrupted database directory
				if rmErr := os.RemoveAll(path); rmErr != nil {
					return nil, fmt.Errorf("failed to remove corrupted database: %v (original error: %v)", rmErr, err)
				}
				// Recreate directory
				if err := ensureDir(path); err != nil {
					return nil, fmt.Errorf("failed to recreate directory: %v", err)
				}
				// Try to open again
				db, err = leveldb.OpenFile(path, &opt.Options{})
				if err != nil {
					return nil, fmt.Errorf("failed to open database after recovery: %v", err)
				}
				log.Printf("✅ Created fresh database at %s", path)
			} else {
				// Recovery succeeded, try to open again
				db, err = leveldb.OpenFile(path, &opt.Options{})
				if err != nil {
					return nil, fmt.Errorf("failed to open database after recovery: %v", err)
				}
				log.Printf("✅ Recovered database at %s", path)
			}
		}
	}

	return &LevelDBStorage{
		db:   db,
		path: path,
	}, nil
}

// Close closes the database
func (s *LevelDBStorage) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// Put stores a key-value pair
func (s *LevelDBStorage) Put(key, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.Put(key, value, nil)
}

// Get retrieves a value by key
func (s *LevelDBStorage) Get(key []byte) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.db.Get(key, nil)
}

// Delete removes a key-value pair
func (s *LevelDBStorage) Delete(key []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.Delete(key, nil)
}

// Has checks if a key exists
func (s *LevelDBStorage) Has(key []byte) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, err := s.db.Get(key, nil)
	if err == leveldb.ErrNotFound {
		return false, nil
	}
	return err == nil, err
}

// ensureDir ensures a directory exists
func ensureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

// LevelDBStats represents LevelDB statistics
type LevelDBStats struct {
	Stats map[string]interface{}
}

// GetStats returns database statistics
func (s *LevelDBStorage) GetStats() (*LevelDBStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := make(map[string]interface{})
	stats["path"] = s.path

	return &LevelDBStats{
		Stats: stats,
	}, nil
}

// LevelDBIterator wraps a LevelDB iterator
type LevelDBIterator struct {
	// Temporarily disabled due to type issues
}

// Next moves to the next item
func (i *LevelDBIterator) Next() bool {
	return false // Temporarily disabled
}

// Key returns the current key
func (i *LevelDBIterator) Key() []byte {
	return nil // Temporarily disabled
}

// Value returns the current value
func (i *LevelDBIterator) Value() []byte {
	return nil // Temporarily disabled
}

// Release releases the iterator
func (i *LevelDBIterator) Release() {
	// Temporarily disabled
}

// Error returns the iterator error
func (i *LevelDBIterator) Error() error {
	return nil // Temporarily disabled
}

// Iterator creates a new iterator
func (s *LevelDBStorage) Iterator(prefix []byte) *LevelDBIterator {
	return &LevelDBIterator{}
}

// BlockStorage handles block storage operations
type BlockStorage struct {
	storage *LevelDBStorage
	Storage *LevelDBStorage // Export for Close()
}

// NewBlockStorage creates a new block storage
func NewBlockStorage(storage *LevelDBStorage) *BlockStorage {
	return &BlockStorage{
		storage: storage,
		Storage: storage, // Export for blockchain close
	}
}

// StoreBlock stores a block
func (bs *BlockStorage) StoreBlock(block *core.Block) error {
	// Serialize block
	data, err := json.Marshal(block)
	if err != nil {
		return fmt.Errorf("failed to marshal block: %v", err)
	}

	// Store by hash
	hashKey := []byte(fmt.Sprintf("block_hash_%x", block.Hash))
	if err := bs.storage.Put(hashKey, data); err != nil {
		return fmt.Errorf("failed to store block by hash: %v", err)
	}

	// Store by number
	numberKey := []byte(fmt.Sprintf("block_number_%d", block.Header.Number))
	if err := bs.storage.Put(numberKey, data); err != nil {
		return fmt.Errorf("failed to store block by number: %v", err)
	}

	// Update best block if this is higher
	bestBlockNumber := uint64(0)
	bestBlock, _ := bs.GetBestBlock()
	if bestBlock != nil {
		bestBlockNumber = bestBlock.Header.Number
	}
	
	if block.Header.Number > bestBlockNumber {
		if err := bs.SetBestBlock(block); err != nil {
			return fmt.Errorf("failed to update best block: %v", err)
		}
	}

	return nil
}

// GetBlockByHash retrieves a block by hash
func (bs *BlockStorage) GetBlockByHash(hash []byte) (*core.Block, error) {
	hashKey := []byte(fmt.Sprintf("block_hash_%x", hash))
	data, err := bs.storage.Get(hashKey)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}

	var block core.Block
	if err := json.Unmarshal(data, &block); err != nil {
		return nil, fmt.Errorf("failed to unmarshal block: %v", err)
	}

	return &block, nil
}

// GetBlockByNumber retrieves a block by number
func (bs *BlockStorage) GetBlockByNumber(number uint64) (*core.Block, error) {
	numberKey := []byte(fmt.Sprintf("block_number_%d", number))
	data, err := bs.storage.Get(numberKey)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}

	var block core.Block
	if err := json.Unmarshal(data, &block); err != nil {
		return nil, fmt.Errorf("failed to unmarshal block: %v", err)
	}

	return &block, nil
}

// GetBestBlock retrieves the best block
func (bs *BlockStorage) GetBestBlock() (*core.Block, error) {
	bestKey := []byte("best_block")
	hashData, err := bs.storage.Get(bestKey)
	if err != nil {
		// If key not found, return nil (no error)
		if err == leveldb.ErrNotFound {
			return nil, nil
		}
		return nil, err
	}
	if hashData == nil {
		return nil, nil
	}

	// Convert hex string to bytes
	hashBytes, err := hex.DecodeString(string(hashData))
	if err != nil {
		return nil, fmt.Errorf("failed to decode hash: %v", err)
	}

	return bs.GetBlockByHash(hashBytes)
}

// SetBestBlockHash sets the best block hash (called internally by StoreBlock)
func (bs *BlockStorage) SetBestBlockHash(hash []byte) error {
	bestKey := []byte("best_block")
	return bs.storage.Put(bestKey, hash)
}

// SetBestBlock sets the best block
func (bs *BlockStorage) SetBestBlock(block *core.Block) error {
	bestKey := []byte("best_block")
	hashData := []byte(fmt.Sprintf("%x", block.Hash))
	log.Printf("💾 Setting best block - Height: %d, Hash: %x", block.Header.Number, block.Hash)
	return bs.storage.Put(bestKey, hashData)
}

// GetBlockCount returns the number of blocks
func (bs *BlockStorage) GetBlockCount() (uint64, error) {
	bestBlock, err := bs.GetBestBlock()
	if err != nil {
		return 0, err
	}
	if bestBlock == nil {
		return 0, nil
	}

	return bestBlock.Header.Number, nil
}

// StoreTransaction stores a transaction
func (bs *BlockStorage) StoreTransaction(tx *core.Transaction) error {
	// Serialize transaction
	data, err := json.Marshal(tx)
	if err != nil {
		return fmt.Errorf("failed to marshal transaction: %v", err)
	}

	// Store by hash
	hashKey := []byte(fmt.Sprintf("tx_hash_%x", tx.Hash))
	return bs.storage.Put(hashKey, data)
}

// GetTransaction retrieves a transaction by hash
func (bs *BlockStorage) GetTransaction(hash []byte) (*core.Transaction, error) {
	hashKey := []byte(fmt.Sprintf("tx_hash_%x", hash))
	data, err := bs.storage.Get(hashKey)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}

	var tx core.Transaction
	if err := json.Unmarshal(data, &tx); err != nil {
		return nil, fmt.Errorf("failed to unmarshal transaction: %v", err)
	}

	return &tx, nil
}

// Close closes the storage
func (bs *BlockStorage) Close() error {
	if bs.storage != nil {
		return bs.storage.Close()
	}
	return nil
}
