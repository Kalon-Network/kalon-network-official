package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/kalon-network/kalon/core"
	"github.com/kalon-network/kalon/crypto"
)

func init() {
	// Set default log level for miner
	core.SetLogLevelString("info")
}

// MinerV2 represents a professional miner
type MinerV2 struct {
	config     *MinerConfig
	blockchain *RPCBlockchainV2
	running    bool
	mu         sync.RWMutex
	stats      *MiningStats
	eventBus   chan MiningEvent
	stopChan   chan struct{}
	wg         sync.WaitGroup
}

// MinerConfig represents miner configuration
type MinerConfig struct {
	Wallet        string
	Threads       int
	RPCURL        string
	StatsInterval time.Duration
}

// MiningStats represents mining statistics
type MiningStats struct {
	mu            sync.RWMutex
	StartTime     time.Time
	TotalHashes   uint64
	BlocksFound   uint64
	CurrentRate   float64
	LastBlockTime time.Time
}

// MiningEvent represents a mining event
type MiningEvent struct {
	Type      string
	Data      interface{}
	Timestamp time.Time
}

// RPCBlockchainV2 represents professional RPC blockchain client
type RPCBlockchainV2 struct {
	rpcURL    string
	client    *http.Client
	mu        sync.RWMutex
	lastBlock *core.Block
}

// NewMinerV2 creates a new professional miner
func NewMinerV2(config *MinerConfig) *MinerV2 {
	blockchain, err := NewRPCBlockchainV2(config.RPCURL)
	if err != nil {
		core.LogError("Failed to create RPC blockchain: %v", err)
		return nil
	}

	return &MinerV2{
		config:     config,
		blockchain: blockchain,
		running:    false,
		stats:      &MiningStats{StartTime: time.Now()},
		eventBus:   make(chan MiningEvent, 100),
		stopChan:   make(chan struct{}),
	}
}

// NewRPCBlockchainV2 creates a new professional RPC blockchain client
func NewRPCBlockchainV2(rpcURL string) (*RPCBlockchainV2, error) {
	return &RPCBlockchainV2{
		rpcURL: rpcURL,
		client: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// Start starts the miner professionally
func (m *MinerV2) Start() error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return fmt.Errorf("miner is already running")
	}
	m.running = true
	m.mu.Unlock()

	core.LogInfo("Starting Professional Kalon Miner v2.0")
	core.LogInfo("   Wallet: %s", m.config.Wallet)
	core.LogInfo("   Threads: %d", m.config.Threads)
	core.LogInfo("   RPC URL: %s", m.config.RPCURL)

	// Get max mining threads from network (genesis config)
	maxThreads := m.getMaxMiningThreads()
	if maxThreads > 0 && uint64(m.config.Threads) > maxThreads {
		core.LogWarn("Thread limit exceeded: requested %d, max allowed %d. Limiting to %d threads for fair mining.",
			m.config.Threads, maxThreads, maxThreads)
		m.config.Threads = int(maxThreads)
	}

	// Start mining threads - Use configured thread count (limited by network max)
	threadCount := m.config.Threads
	if threadCount <= 0 {
		threadCount = 1 // Default to 1 thread
	}
	if maxThreads > 0 && uint64(threadCount) > maxThreads {
		threadCount = int(maxThreads)
	}

	core.LogInfo("   Using %d mining thread(s) (network limit: %d)", threadCount, maxThreads)

	for i := 0; i < threadCount; i++ {
		m.wg.Add(1)
		go m.miningWorker(i)
	}

	// Start stats reporter
	m.wg.Add(1)
	go m.statsReporter()

	// Start event processor
	m.wg.Add(1)
	go m.eventProcessor()

	return nil
}

// Stop stops the miner gracefully
func (m *MinerV2) Stop() error {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return fmt.Errorf("miner is not running")
	}
	m.running = false
	m.mu.Unlock()

	core.LogInfo("Stopping miner...")

	// Signal all workers to stop
	close(m.stopChan)

	// Wait for all workers to finish
	m.wg.Wait()

	core.LogInfo("Miner stopped successfully")
	return nil
}

// miningWorker performs mining work
func (m *MinerV2) miningWorker(workerID int) {
	defer m.wg.Done()

	core.LogDebug("Mining worker %d started", workerID)

	for {
		select {
		case <-m.stopChan:
			core.LogDebug("Mining worker %d stopped", workerID)
			return
		default:
			m.mineBlock(workerID)
		}
	}
}

// mineBlock mines a single block
func (m *MinerV2) mineBlock(workerID int) {
	// Get miner address
	miner, err := m.parseAddress(m.config.Wallet)
	if err != nil {
		core.LogError("Failed to parse wallet address: %v", err)
		time.Sleep(1 * time.Second)
		return
	}

	// CRITICAL: Get fresh template for each mining attempt
	// This prevents multiple workers from using the same outdated template
	block := m.blockchain.CreateNewBlock(miner, []core.Transaction{}, m.config.Wallet)
	if block == nil {
		core.LogError("Failed to create block template")
		time.Sleep(1 * time.Second)
		return
	}

	// CRITICAL: Double-check template validity before mining
	currentHeight := m.getCurrentHeight()
	if currentHeight >= block.Header.Number {
		core.LogDebug("Template outdated: current height %d >= template height %d", currentHeight, block.Header.Number)
		time.Sleep(100 * time.Millisecond)
		return
	}

	// Additional check: Verify parent hash is still current
	bestBlock := m.getBestBlock()
	if bestBlock != nil && bestBlock.Hash != block.Header.ParentHash {
		core.LogDebug("Template parent hash outdated: expected %x, got %x", bestBlock.Hash, block.Header.ParentHash)
		time.Sleep(100 * time.Millisecond)
		return
	}

	// Mine the block
	startTime := time.Now()
	nonce := uint64(0)
	target := uint64(1) << (64 - block.Header.Difficulty) // Use 64-bit target, not 256-bit

	for {
		select {
		case <-m.stopChan:
			return
		default:
			// Update nonce
			block.Header.Nonce = nonce
			block.Hash = block.CalculateHash()

			// Check if hash meets target
			hashInt := binary.BigEndian.Uint64(block.Hash[:8])
			if hashInt < target {
				// Block found!
				m.handleBlockFound(block, workerID, time.Since(startTime))
				return
			}

			nonce++
			if nonce%1000000 == 0 {
				// Update stats every million hashes
				m.updateStats(1000000, time.Since(startTime))
			}
		}
	}
}

// handleBlockFound handles a found block
func (m *MinerV2) handleBlockFound(block *core.Block, workerID int, duration time.Duration) {
	// Extract block reward from first transaction (block reward transaction)
	var rewardAmount uint64 = 0
	var rewardTKALON float64 = 0.0
	if len(block.Txs) > 0 {
		// First transaction is always the miner reward transaction
		rewardTx := block.Txs[0]
		// Check if it's a block reward transaction (has "block_reward" in data)
		if len(rewardTx.Data) > 0 && string(rewardTx.Data) == "block_reward" {
			rewardAmount = rewardTx.Amount
			rewardTKALON = float64(rewardAmount) / 1000000.0
		}
	}

	// Log block found with reward information
	if rewardAmount > 0 {
		core.LogInfo("🎉 Block #%d found by worker %d! Reward: %.2f tKALON (Hash: %x, Nonce: %d, Time: %v)",
			block.Header.Number, workerID, rewardTKALON, block.Hash, block.Header.Nonce, duration)
	} else {
		core.LogInfo("Block found by worker %d! Hash: %x, Nonce: %d, Time: %v",
			workerID, block.Hash, block.Header.Nonce, duration)
	}

	// Update stats
	m.stats.mu.Lock()
	m.stats.BlocksFound++
	m.stats.LastBlockTime = time.Now()
	m.stats.mu.Unlock()

	// Emit event
	m.eventBus <- MiningEvent{
		Type: "blockFound",
		Data: map[string]interface{}{
			"block":        block,
			"workerID":     workerID,
			"duration":     duration,
			"reward":       rewardAmount,
			"rewardTKALON": rewardTKALON,
		},
		Timestamp: time.Now(),
	}

	// Submit block
	if err := m.blockchain.AddBlock(block); err != nil {
		core.LogError("Failed to submit block: %v", err)
	} else {
		if rewardAmount > 0 {
			core.LogInfo("✅ Block #%d submitted successfully! Reward: %.2f tKALON (Hash: %x)",
				block.Header.Number, rewardTKALON, block.Hash)
		} else {
			core.LogInfo("Block #%d submitted successfully: %x", block.Header.Number, block.Hash)
		}
	}
}

// updateStats updates mining statistics
func (m *MinerV2) updateStats(hashes uint64, duration time.Duration) {
	m.stats.mu.Lock()
	m.stats.TotalHashes += hashes
	m.stats.CurrentRate = float64(hashes) / duration.Seconds()
	m.stats.mu.Unlock()
}

// statsReporter reports mining statistics
func (m *MinerV2) statsReporter() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.StatsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.printStats()
		case <-m.stopChan:
			return
		}
	}
}

// printStats prints current statistics
func (m *MinerV2) printStats() {
	m.stats.mu.RLock()
	startTime := m.stats.StartTime
	totalHashes := m.stats.TotalHashes
	blocksFound := m.stats.BlocksFound
	currentRate := m.stats.CurrentRate
	m.stats.mu.RUnlock()

	uptime := time.Since(startTime)
	avgRate := float64(totalHashes) / uptime.Seconds()

	core.LogInfo("Mining Stats - Uptime: %v, Total Hashes: %d, Blocks Found: %d, Avg Rate: %.2f H/s, Current Rate: %.2f H/s",
		uptime.Truncate(time.Second), totalHashes, blocksFound, avgRate, currentRate)
}

// eventProcessor processes mining events
func (m *MinerV2) eventProcessor() {
	defer m.wg.Done()

	for {
		select {
		case event := <-m.eventBus:
			m.processEvent(event)
		case <-m.stopChan:
			return
		}
	}
}

// processEvent processes a mining event
func (m *MinerV2) processEvent(event MiningEvent) {
	switch event.Type {
	case "blockFound":
		// Handle block found event
		core.LogDebug("Event: Block found at %v", event.Timestamp)
	default:
		core.LogDebug("Event: %s at %v", event.Type, event.Timestamp)
	}
}

// CreateNewBlock creates a new block template
func (rpc *RPCBlockchainV2) CreateNewBlock(miner core.Address, txs []core.Transaction, walletAddress string) *core.Block {
	req := RPCRequest{
		JSONRPC: "2.0",
		Method:  "createBlockTemplate",
		Params: map[string]interface{}{
			"miner": walletAddress, // Send original Bech32 address
		},
		ID: 2,
	}

	// Add small delay to avoid race conditions with submitBlock
	time.Sleep(10 * time.Millisecond)

	resp, err := rpc.callRPC(req)
	if err != nil {
		core.LogError("Failed to create block template: %v", err)
		return nil
	}

	if resp.Error != nil {
		core.LogError("RPC error: %s", resp.Error.Message)
		return nil
	}

	// Parse response
	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		core.LogError("Invalid response format")
		return nil
	}

	// Extract values
	number, _ := result["number"].(float64)
	difficulty, _ := result["difficulty"].(float64)
	parentHashStr, _ := result["parentHash"].(string)
	timestamp, _ := result["timestamp"].(float64)

	// Parse parent hash
	parentHashBytes, err := hex.DecodeString(parentHashStr)
	if err != nil {
		core.LogError("Failed to parse parent hash: %v", err)
		return nil
	}

	var parentHash core.Hash
	copy(parentHash[:], parentHashBytes)

	// CRITICAL: Extract transactions from RPC response
	var blockTxs []core.Transaction
	if txsData, ok := result["transactions"].([]interface{}); ok {
		for _, txData := range txsData {
			if txMap, ok := txData.(map[string]interface{}); ok {
				// Parse transaction from map
				tx := core.Transaction{}

				// Parse From address
				if fromStr, ok := txMap["from"].(string); ok {
					tx.From = core.AddressFromString(fromStr)
				}

				// Parse To address
				if toStr, ok := txMap["to"].(string); ok {
					tx.To = core.AddressFromString(toStr)
				}

				// Parse Amount
				if amount, ok := txMap["amount"].(float64); ok {
					tx.Amount = uint64(amount)
				}

				// Parse other fields
				if nonce, ok := txMap["nonce"].(float64); ok {
					tx.Nonce = uint64(nonce)
				}
				if fee, ok := txMap["fee"].(float64); ok {
					tx.Fee = uint64(fee)
				}
				if gasUsed, ok := txMap["gasUsed"].(float64); ok {
					tx.GasUsed = uint64(gasUsed)
				}
				if gasPrice, ok := txMap["gasPrice"].(float64); ok {
					tx.GasPrice = uint64(gasPrice)
				}
				// Parse data field (can be hex-encoded string or []byte)
				if dataStr, ok := txMap["data"].(string); ok {
					// Try to decode as hex string
					if dataBytes, err := hex.DecodeString(dataStr); err == nil {
						tx.Data = dataBytes
					} else {
						// If decoding fails, use as raw bytes
						tx.Data = []byte(dataStr)
					}
				} else if data, ok := txMap["data"].([]byte); ok {
					tx.Data = data
				}
				// Parse signature (hex-encoded string from server)
				if sigStr, ok := txMap["signature"].(string); ok {
					if sigBytes, err := hex.DecodeString(sigStr); err == nil {
						tx.Signature = sigBytes
					}
				} else if signature, ok := txMap["signature"].([]byte); ok {
					// Fallback: if sent as []byte
					tx.Signature = signature
				}
				// Parse public key (hex-encoded string from server)
				if pubKeyStr, ok := txMap["publicKey"].(string); ok {
					if pubKeyBytes, err := hex.DecodeString(pubKeyStr); err == nil {
						tx.PublicKey = pubKeyBytes
					}
				}
				if hashStr, ok := txMap["hash"].(string); ok {
					if hashBytes, err := hex.DecodeString(hashStr); err == nil {
						copy(tx.Hash[:], hashBytes)
					}
				}

				// Parse UTXO fields
				if inputs, ok := txMap["inputs"].([]interface{}); ok {
					for _, inputData := range inputs {
						if inputMap, ok := inputData.(map[string]interface{}); ok {
							input := core.TxInput{}
							if prevTxHashStr, ok := inputMap["previousTxHash"].(string); ok {
								if prevTxHashBytes, err := hex.DecodeString(prevTxHashStr); err == nil {
									copy(input.PreviousTxHash[:], prevTxHashBytes)
								}
							}
							if index, ok := inputMap["index"].(float64); ok {
								input.Index = uint32(index)
							}
							if signature, ok := inputMap["signature"].([]byte); ok {
								input.Signature = signature
							}
							tx.Inputs = append(tx.Inputs, input)
						}
					}
				}

				if outputs, ok := txMap["outputs"].([]interface{}); ok {
					for _, outputData := range outputs {
						if outputMap, ok := outputData.(map[string]interface{}); ok {
							output := core.TxOutput{}
							if addressStr, ok := outputMap["address"].(string); ok {
								// CRITICAL: Decode hex directly, don't use AddressFromString!
								if addressBytes, err := hex.DecodeString(addressStr); err == nil && len(addressBytes) == 20 {
									copy(output.Address[:], addressBytes)
									core.LogDebug("Miner: Parsed output address: %s -> %x", addressStr, output.Address)
								}
							}
							if amount, ok := outputMap["amount"].(float64); ok {
								output.Amount = uint64(amount)
							}
							tx.Outputs = append(tx.Outputs, output)
						}
					}
				}

				// Parse timestamp
				if timestamp, ok := txMap["timestamp"].(string); ok {
					if t, err := time.Parse(time.RFC3339, timestamp); err == nil {
						tx.Timestamp = t
					}
				}

				blockTxs = append(blockTxs, tx)
				core.LogDebug("Loaded transaction with %d outputs, total amount: %d", len(tx.Outputs), tx.Amount)
			}
		}
	}

	// Helper function to calculate merkle root
	calculateMerkleRoot := func(txs []core.Transaction) core.Hash {
		if len(txs) == 0 {
			return core.Hash{}
		} else if len(txs) == 1 {
			// CRITICAL: Use hash if already set, otherwise calculate
			if txs[0].Hash != (core.Hash{}) {
				return txs[0].Hash
			}
			return txs[0].CalculateHash()
		} else {
			// Build merkle tree
			hashes := make([][]byte, len(txs))
			for i, tx := range txs {
				// CRITICAL: Use hash if already set (from RPC), otherwise calculate
				if tx.Hash != (core.Hash{}) {
					hashes[i] = tx.Hash.Bytes()
				} else {
					hashes[i] = tx.CalculateHash().Bytes()
				}
			}

			for len(hashes) > 1 {
				var nextLevel [][]byte
				for i := 0; i < len(hashes); i += 2 {
					var left, right []byte
					left = hashes[i]
					if i+1 < len(hashes) {
						right = hashes[i+1]
					} else {
						right = hashes[i] // Duplicate last element if odd number
					}
					// Concatenate and hash
					combined := append(left, right...)
					hash := sha256.Sum256(combined)
					nextLevel = append(nextLevel, hash[:])
				}
				hashes = nextLevel
			}
			var result core.Hash
			copy(result[:], hashes[0])
			return result
		}
	}

	// CRITICAL: Use merkle root from RPC server if provided, otherwise calculate
	// This ensures consistency with the node's merkle root calculation
	var merkleRoot core.Hash
	if merkleRootStr, ok := result["merkleRoot"].(string); ok {
		merkleRootBytes, err := hex.DecodeString(merkleRootStr)
		if err == nil && len(merkleRootBytes) == 32 {
			copy(merkleRoot[:], merkleRootBytes)
			core.LogDebug("Using merkle root from RPC server: %x", merkleRoot)
		} else {
			// Fallback: calculate merkle root
			core.LogDebug("Failed to parse merkle root from RPC, calculating...")
			merkleRoot = calculateMerkleRoot(blockTxs)
		}
	} else {
		// Fallback: calculate merkle root
		core.LogDebug("No merkle root in RPC response, calculating...")
		merkleRoot = calculateMerkleRoot(blockTxs)
	}

	// Create block with transactions from RPC server
	block := &core.Block{
		Header: core.BlockHeader{
			ParentHash:  parentHash, // CRITICAL: Use actual parent hash
			Number:      uint64(number),
			Timestamp:   time.Unix(int64(timestamp), 0),
			Difficulty:  uint64(difficulty),
			Miner:       miner,
			Nonce:       0,
			MerkleRoot:  merkleRoot, // Calculate merkle root from transactions
			TxCount:     uint32(len(blockTxs)),
			NetworkFee:  0,
			TreasuryFee: 0,
		},
		Txs:  blockTxs, // Use transactions from RPC server
		Hash: core.Hash{},
	}

	// Calculate hash
	block.Hash = block.CalculateHash()

	core.LogDebug("Created block template #%d with %d transactions and parent hash: %x", block.Header.Number, len(blockTxs), block.Header.ParentHash)

	return block
}

// AddBlock submits a mined block
func (rpc *RPCBlockchainV2) AddBlock(block *core.Block) error {
	// Convert transactions to JSON format
	var transactions []map[string]interface{}
	for _, tx := range block.Txs {
		// Convert outputs to JSON format with explicit address strings
		var outputs []map[string]interface{}
		for _, output := range tx.Outputs {
			// Address als hex-string senden (direkt, NICHT nochmal encodieren!)
			outputs = append(outputs, map[string]interface{}{
				"address": output.Address.String(), // String() macht hex.EncodeToString
				"amount":  float64(output.Amount),
			})
		}

		// Serialize signature and public key as hex-encoded strings (not []byte)
		signatureStr := ""
		if len(tx.Signature) > 0 {
			signatureStr = hex.EncodeToString(tx.Signature)
		}
		publicKeyStr := ""
		if len(tx.PublicKey) > 0 {
			publicKeyStr = hex.EncodeToString(tx.PublicKey)
		}
		dataStr := ""
		if len(tx.Data) > 0 {
			dataStr = hex.EncodeToString(tx.Data)
		}

		txMap := map[string]interface{}{
			"from":      tx.From.String(),
			"to":        tx.To.String(),
			"amount":    float64(tx.Amount),
			"nonce":     float64(tx.Nonce),
			"fee":       float64(tx.Fee),
			"gasUsed":   float64(tx.GasUsed),
			"gasPrice":  float64(tx.GasPrice),
			"data":      dataStr,
			"signature": signatureStr,
			"publicKey": publicKeyStr,
			"hash":      hex.EncodeToString(tx.Hash[:]),
			"inputs":    tx.Inputs,
			"outputs":   outputs, // CRITICAL: Use manually constructed outputs
			"timestamp": tx.Timestamp.Format(time.RFC3339),
		}
		transactions = append(transactions, txMap)
	}

	req := RPCRequest{
		JSONRPC: "2.0",
		Method:  "submitBlock",
		Params: map[string]interface{}{
			"block": map[string]interface{}{
				"number":       float64(block.Header.Number),
				"difficulty":   float64(block.Header.Difficulty),
				"nonce":        float64(block.Header.Nonce),
				"hash":         hex.EncodeToString(block.Hash[:]),
				"parentHash":   hex.EncodeToString(block.Header.ParentHash[:]),
				"timestamp":    float64(block.Header.Timestamp.Unix()),
				"merkleRoot":   hex.EncodeToString(block.Header.MerkleRoot[:]), // Include merkle root
				"txCount":      float64(block.Header.TxCount),
				"miner":        hex.EncodeToString(block.Header.Miner[:]), // Include miner address
				"transactions": transactions,                              // CRITICAL: Include transactions!
			},
		},
		ID: 3,
	}

	resp, err := rpc.callRPC(req)
	if err != nil {
		return fmt.Errorf("failed to submit block: %v", err)
	}

	if resp.Error != nil {
		return fmt.Errorf("RPC error: %s", resp.Error.Message)
	}

	core.LogDebug("Block #%d submitted successfully", block.Header.Number)
	return nil
}

// callRPC makes an RPC call with retry logic
func (rpc *RPCBlockchainV2) callRPC(req RPCRequest) (*RPCResponse, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	maxRetries := 3
	var lastErr error
	var lastResp *http.Response

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 50ms, 100ms, 200ms
			time.Sleep(time.Duration(50*attempt) * time.Millisecond)
		}

		resp, err := rpc.client.Post(rpc.rpcURL, "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			lastErr = err
			continue
		}

		// Read full response body first
		bodyBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			lastResp = resp
			continue
		}

		// Check if response is valid JSON
		if len(bodyBytes) == 0 {
			lastErr = fmt.Errorf("empty response body")
			lastResp = resp
			continue
		}

		// Check if response starts with '{' (valid JSON)
		previewLen := len(bodyBytes)
		if previewLen > 100 {
			previewLen = 100
		}
		if len(bodyBytes) > 0 && (bodyBytes[0] != '{' && bodyBytes[0] != '[') {
			lastErr = fmt.Errorf("invalid JSON response (starts with '%c'): %s", bodyBytes[0], string(bodyBytes[:previewLen]))
			lastResp = resp
			continue
		}

		// Try to decode JSON
		var rpcResp RPCResponse
		previewLen2 := len(bodyBytes)
		if previewLen2 > 200 {
			previewLen2 = 200
		}
		if err := json.Unmarshal(bodyBytes, &rpcResp); err != nil {
			lastErr = fmt.Errorf("JSON decode error: %v, response: %s", err, string(bodyBytes[:previewLen2]))
			lastResp = resp
			continue
		}

		// Success!
		return &rpcResp, nil
	}

	// All retries failed
	if lastResp != nil {
		return nil, fmt.Errorf("RPC call failed after %d attempts, last error: %v (status: %d)", maxRetries, lastErr, lastResp.StatusCode)
	}
	return nil, fmt.Errorf("RPC call failed after %d attempts, last error: %v", maxRetries, lastErr)
}

// parseAddress parses a wallet address
func (m *MinerV2) parseAddress(address string) (core.Address, error) {
	// If it's a Bech32 address (starts with kalon1), decode it
	if strings.HasPrefix(address, "kalon1") {
		decodedBytes, err := crypto.DecodeBech32(address)
		if err == nil && len(decodedBytes) == 20 {
			var addr core.Address
			copy(addr[:], decodedBytes)
			core.LogDebug("Parsed Bech32 address: %s -> %x", address, addr)
			return addr, nil
		}
	}

	// Try to decode as hex first
	if len(address) == 40 {
		bytes, err := hex.DecodeString(address)
		if err == nil && len(bytes) == 20 {
			var addr core.Address
			copy(addr[:], bytes)
			return addr, nil
		}
	}

	// Fallback: create hash from string
	hash := sha256.Sum256([]byte(address))
	var addr core.Address
	// Use first 20 bytes of hash
	copy(addr[:], hash[:20])
	return addr, nil
}

// getMaxMiningThreads gets the maximum allowed mining threads from the network
func (m *MinerV2) getMaxMiningThreads() uint64 {
	req := RPCRequest{
		JSONRPC: "2.0",
		Method:  "getMiningInfo",
		Params:  map[string]interface{}{},
		ID:      998,
	}

	resp, err := m.blockchain.callRPC(req)
	if err != nil {
		core.LogWarn("Failed to get mining info, using default thread limit: %v", err)
		return 0 // No limit if we can't get it
	}

	if resp.Error != nil {
		core.LogWarn("RPC error getting mining info: %s", resp.Error.Message)
		return 0
	}

	// Parse the result
	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		return 0
	}

	// Extract maxMiningThreads
	if maxThreads, ok := result["maxMiningThreads"].(float64); ok {
		return uint64(maxThreads)
	}

	return 0
}

// getCurrentHeight gets the current blockchain height
func (m *MinerV2) getCurrentHeight() uint64 {
	req := RPCRequest{
		JSONRPC: "2.0",
		Method:  "getHeight",
		Params:  map[string]interface{}{},
		ID:      999,
	}

	resp, err := m.blockchain.callRPC(req)
	if err != nil {
		return 0
	}

	if resp.Error != nil {
		return 0
	}

	if height, ok := resp.Result.(float64); ok {
		return uint64(height)
	}

	return 0
}

// getBestBlock gets the current best block
func (m *MinerV2) getBestBlock() *core.Block {
	req := RPCRequest{
		JSONRPC: "2.0",
		Method:  "getBestBlock",
		Params:  map[string]interface{}{},
		ID:      998,
	}

	resp, err := m.blockchain.callRPC(req)
	if err != nil {
		return nil
	}

	if resp.Error != nil {
		return nil
	}

	// Parse the block data from response
	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		return nil
	}

	// Extract hash
	hashStr, ok := result["hash"].(string)
	if !ok {
		return nil
	}

	hashBytes, err := hex.DecodeString(hashStr)
	if err != nil || len(hashBytes) != 32 {
		return nil
	}

	var hash core.Hash
	copy(hash[:], hashBytes)

	// Create a minimal block with just the hash for comparison
	return &core.Block{
		Hash: hash,
	}
}

// RPCRequest represents a JSON-RPC request
type RPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	ID      int         `json:"id"`
}

// RPCResponse represents a JSON-RPC response
type RPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
	ID      int         `json:"id"`
}

// RPCError represents a JSON-RPC error
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data,omitempty"`
}

func main() {
	var (
		wallet        = flag.String("wallet", "", "Wallet address")
		threads       = flag.Int("threads", 2, "Number of mining threads")
		rpcURL        = flag.String("rpc", "http://localhost:16316", "RPC server URL")
		statsInterval = flag.Duration("stats", 30*time.Second, "Statistics reporting interval")
		logLevel      = flag.String("loglevel", "info", "Log level (debug, info, warn, error)")
	)
	flag.Parse()

	// Set log level
	core.SetLogLevelString(*logLevel)

	if *wallet == "" {
		core.LogError("Wallet address is required")
		os.Exit(1)
	}

	config := &MinerConfig{
		Wallet:        *wallet,
		Threads:       *threads,
		RPCURL:        *rpcURL,
		StatsInterval: *statsInterval,
	}

	miner := NewMinerV2(config)

	// Start miner
	if err := miner.Start(); err != nil {
		core.LogError("Failed to start miner: %v", err)
		os.Exit(1)
	}

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	core.LogInfo("Shutdown signal received")

	// Stop miner
	if err := miner.Stop(); err != nil {
		core.LogError("Error stopping miner: %v", err)
	}
}
