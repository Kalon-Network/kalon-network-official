package rpc

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/kalon-network/kalon/core"
)

const (
	// MaxTransactionSizeBytes is the maximum size of a transaction in bytes (100 KB)
	MaxTransactionSizeBytes = 100 * 1024
	// MaxBlockSubmissionsPerMinute is the maximum number of block submissions per minute per IP
	MaxBlockSubmissionsPerMinute = 2
)

func init() {
	// Set default log level for RPC server
	core.SetLogLevelString("info")
}

// RPCRequest represents a JSON-RPC request
type RPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	ID      interface{} `json:"id"`
}

// RPCResponse represents a JSON-RPC response
type RPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
	ID      interface{} `json:"id"`
}

// RPCError represents an RPC error
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data,omitempty"`
}

// ServerV2 represents a professional RPC server
type ServerV2 struct {
	addr                  string
	httpsAddr             string // HTTPS server address (e.g. ":16317")
	certFile              string // SSL certificate file path
	keyFile               string // SSL private key file path
	blockchain            *core.BlockchainV2
	p2pNetwork            interface{ GetPeerCount() int } // Interface for P2P network (optional)
	mu                    sync.RWMutex
	connections           map[string]*Connection
	eventBus              *core.EventBus
	ctx                   context.Context
	cancel                context.CancelFunc
	allowedIPs            map[string]bool        // Whitelist of allowed IPs
	rateLimits            map[string]*RateLimit  // Rate limiting per IP
	blockSubmissionLimits map[string][]time.Time // Block submission timestamps per IP
	requireAuth           bool                   // Whether auth is required
	authTokens            map[string]bool        // Valid auth tokens
	server                *http.Server           // HTTP server instance for shutdown
	httpsServer           *http.Server           // HTTPS server instance for shutdown
}

// Connection represents a client connection
type Connection struct {
	ID        string
	CreatedAt time.Time
	LastSeen  time.Time
	Requests  int64
}

// RateLimit tracks rate limiting for an IP
type RateLimit struct {
	mu             sync.RWMutex
	Count          int
	LastReset      time.Time
	RequestsPerMin int
}

// NewServerV2 creates a new professional RPC server
func NewServerV2(addr string, blockchain *core.BlockchainV2) *ServerV2 {
	ctx, cancel := context.WithCancel(context.Background())

	server := &ServerV2{
		addr:                  addr,
		blockchain:            blockchain,
		p2pNetwork:            nil, // Will be set via SetP2PNetwork if needed
		connections:           make(map[string]*Connection),
		eventBus:              blockchain.GetEventBus(),
		ctx:                   ctx,
		cancel:                cancel,
		allowedIPs:            make(map[string]bool),
		rateLimits:            make(map[string]*RateLimit),
		blockSubmissionLimits: make(map[string][]time.Time),
		requireAuth:           false, // For testnet: auth disabled by default
		authTokens:            make(map[string]bool),
	}

	// Start connection cleanup routine
	go server.cleanupConnections()

	return server
}

// SetP2PNetwork sets the P2P network for peer count queries
func (s *ServerV2) SetP2PNetwork(p2p interface{ GetPeerCount() int }) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.p2pNetwork = p2p
}

// SetHTTPS configures HTTPS for the RPC server
func (s *ServerV2) SetHTTPS(httpsAddr, certFile, keyFile string) {
	// Validate certificate files exist before configuring
	if certFile != "" && keyFile != "" {
		// Check if files exist (but don't fail if they don't - let ListenAndServeTLS handle it)
		s.httpsAddr = httpsAddr
		s.certFile = certFile
		s.keyFile = keyFile
	}
}

// Start starts the RPC server professionally
func (s *ServerV2) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRequest)
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/rpc", s.handleRequest)

	// Create server with professional settings
	server := &http.Server{
		Addr:           s.addr,
		Handler:        s.limitConnections(mux),
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1MB
	}

	core.LogInfo("Professional RPC Server starting on %s", s.addr)

	// Store server reference for shutdown BEFORE starting
	s.server = server

	// Start server in goroutine
	started := make(chan bool, 1)
	go func() {
		core.LogDebug("Attempting to bind to %s", s.addr)
		started <- true
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			core.LogError("RPC Server error: %v", err)
		}
	}()

	// Wait for goroutine to start
	<-started

	// Give server time to bind to port (increased timeout)
	time.Sleep(1 * time.Second)

	// Log success
	core.LogInfo("RPC Server should be listening on %s", s.addr)

	// Start HTTPS server if configured
	if s.httpsAddr != "" && s.certFile != "" && s.keyFile != "" {
		go func() {
			httpsMux := http.NewServeMux()
			httpsMux.HandleFunc("/", s.handleRequest)
			httpsMux.HandleFunc("/health", s.handleHealth)
			httpsMux.HandleFunc("/rpc", s.handleRequest)

			httpsServer := &http.Server{
				Addr:           s.httpsAddr,
				Handler:        s.limitConnections(httpsMux),
				ReadTimeout:    30 * time.Second,
				WriteTimeout:   30 * time.Second,
				IdleTimeout:    60 * time.Second,
				MaxHeaderBytes: 1 << 20, // 1MB
			}

			s.httpsServer = httpsServer

			core.LogInfo("HTTPS RPC Server starting on %s", s.httpsAddr)
			core.LogInfo("   Certificate: %s", s.certFile)
			core.LogInfo("   Key: %s", s.keyFile)

			// ListenAndServeTLS will validate certificate files on start
			// If files are missing or invalid, it will return an error
			// We catch and log it, but don't crash the Node
			if err := httpsServer.ListenAndServeTLS(s.certFile, s.keyFile); err != nil && err != http.ErrServerClosed {
				core.LogError("HTTPS RPC Server error: %v", err)
				core.LogError("   Check if certificate files exist: %s, %s", s.certFile, s.keyFile)
			}
		}()
		time.Sleep(500 * time.Millisecond) // Give HTTPS server time to start
		core.LogInfo("HTTPS RPC Server should be listening on %s", s.httpsAddr)
	}

	// Handle shutdown in background (non-blocking)
	go func() {
		<-s.ctx.Done()
		// Graceful shutdown
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if s.server != nil {
			if err := s.server.Shutdown(ctx); err != nil {
				core.LogWarn("Error shutting down HTTP RPC server: %v", err)
			}
		}
		if s.httpsServer != nil {
			if err := s.httpsServer.Shutdown(ctx); err != nil {
				core.LogWarn("Error shutting down HTTPS RPC server: %v", err)
			}
		}
	}()

	// Return immediately (non-blocking)
	return nil
}

// Stop stops the RPC server
func (s *ServerV2) Stop() {
	s.cancel()
}

// handleRequest handles RPC requests professionally
func (s *ServerV2) handleRequest(w http.ResponseWriter, r *http.Request) {
	// CORS headers for browser access (Explorer)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Max-Age", "3600")

	// Security headers
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-XSS-Protection", "1; mode=block")

	// Cache control headers
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	// Handle preflight OPTIONS request
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// CRITICAL: Panic recovery to prevent server crashes
	defer func() {
		if rec := recover(); rec != nil {
			core.LogError("PANIC in handleRequest: %v\nStack trace: %s", rec, debug.Stack())

			// Try to write error response
			w.Header().Set("Content-Type", "application/json")
			errorResponse := map[string]interface{}{
				"jsonrpc": "2.0",
				"error": map[string]interface{}{
					"code":    -32603,
					"message": "Internal error",
					"data":    fmt.Sprintf("Panic: %v", rec),
				},
				"id": nil,
			}
			if jsonBytes, err := json.Marshal(errorResponse); err == nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write(jsonBytes)
			} else {
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
		}
	}()

	// Extract IP
	ip := s.extractIP(r)

	// Check IP whitelist if enabled
	if len(s.allowedIPs) > 0 && !s.allowedIPs[ip] {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Check rate limit
	if !s.checkRateLimit(ip) {
		http.Error(w, "Too many requests", http.StatusTooManyRequests)
		return
	}

	// Track connection
	s.trackConnection(ip)

	// Parse request
	var req RPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, nil, -32700, "Parse error", err.Error())
		return
	}

	// Check block submission rate limit for submitBlock method
	if req.Method == "submitBlock" {
		if !s.checkBlockSubmissionRateLimit(ip) {
			s.writeError(w, req.ID, -32603, "Block submission rate limit exceeded",
				fmt.Sprintf("Maximum %d block submissions per minute per IP allowed", MaxBlockSubmissionsPerMinute))
			return
		}
	}

	// Handle request
	response := s.handleRPCMethod(&req)

	// Write response
	w.Header().Set("Content-Type", "application/json")

	// CRITICAL: Manual JSON encoding to prevent circular references
	// Wrap in additional safety check
	jsonBytes, err := json.Marshal(response)
	if err != nil {
		core.LogError("Failed to marshal RPC response: %v", err)
		core.LogError("Response type: %T, Response: %+v", response, response)

		// Write safe error response instead of crashing
		errorResponse := map[string]interface{}{
			"jsonrpc": "2.0",
			"error": map[string]interface{}{
				"code":    -32603,
				"message": "Internal error",
				"data":    fmt.Sprintf("JSON marshal error: %v", err),
			},
			"id": req.ID,
		}
		if safeBytes, safeErr := json.Marshal(errorResponse); safeErr == nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write(safeBytes)
		} else {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	if _, err := w.Write(jsonBytes); err != nil {
		core.LogError("Failed to write RPC response: %v", err)
	}
}

// handleRPCMethod handles RPC methods professionally
func (s *ServerV2) handleRPCMethod(req *RPCRequest) *RPCResponse {
	switch req.Method {
	case "getHeight":
		return s.handleGetHeight(req)
	case "getBestBlock":
		return s.handleGetBestBlock(req)
	case "getRecentBlocks":
		return s.handleGetRecentBlocks(req)
	case "createBlockTemplate":
		return s.handleCreateBlockTemplateV2(req)
	case "submitBlock":
		return s.handleSubmitBlockV2(req)
	case "getMiningInfo":
		return s.handleGetMiningInfo(req)
	case "getBalance":
		return s.handleGetBalance(req)
	case "prepareTransaction":
		return s.handlePrepareTransaction(req)
	case "sendTransaction":
		return s.handleSendTransaction(req)
	case "createSnapshot":
		return s.handleCreateSnapshot(req)
	case "restoreSnapshot":
		return s.handleRestoreSnapshot(req)
	case "getSnapshot":
		return s.handleGetSnapshot(req)
	case "getPeerCount":
		return s.handleGetPeerCount(req)
	case "getTotalTransactions":
		return s.handleGetTotalTransactions(req)
	case "getPendingTransactions":
		return s.handleGetPendingTransactions(req)
	case "getAddressCount":
		return s.handleGetAddressCount(req)
	case "getTreasuryBalance":
		return s.handleGetTreasuryBalance(req)
	case "getHashrate":
		return s.handleGetHashrate(req)
	case "getAddressInfo":
		return s.handleGetAddressInfo(req)
	case "getAddressTransactions":
		return s.handleGetAddressTransactions(req)
	case "getBlockByHash":
		return s.handleGetBlockByHash(req)
	case "getBlockByNumber":
		return s.handleGetBlockByNumber(req)
	default:
		return &RPCResponse{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    -32601,
				Message: "Method not found",
			},
			ID: req.ID,
		}
	}
}

// handleCreateSnapshot handles createSnapshot requests
func (s *ServerV2) handleCreateSnapshot(req *RPCRequest) *RPCResponse {
	snapshot, err := s.blockchain.SnapshotManager.CreateSnapshot(s.blockchain)
	if err != nil {
		return &RPCResponse{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    -32603,
				Message: "Internal error",
				Data:    err.Error(),
			},
			ID: req.ID,
		}
	}

	// Save snapshot to file if filename provided
	var filename string
	if params, ok := req.Params.(map[string]interface{}); ok {
		if f, ok := params["filename"].(string); ok && f != "" {
			filename = f
			if err := s.blockchain.SnapshotManager.SaveSnapshot(filename); err != nil {
				core.LogWarn("Failed to save snapshot to file: %v", err)
			}
		}
	}

	return &RPCResponse{
		JSONRPC: "2.0",
		Result: map[string]interface{}{
			"height":       snapshot.Height,
			"timestamp":    snapshot.Timestamp,
			"blockHash":    snapshot.BlockHash,
			"addressCount": len(snapshot.Balances),
			"totalSupply":  snapshot.TotalSupply,
			"filename":     filename,
		},
		ID: req.ID,
	}
}

// handleRestoreSnapshot handles restoreSnapshot requests
func (s *ServerV2) handleRestoreSnapshot(req *RPCRequest) *RPCResponse {
	var filename string
	if params, ok := req.Params.(map[string]interface{}); ok {
		if f, ok := params["filename"].(string); ok && f != "" {
			filename = f
		} else {
			return &RPCResponse{
				JSONRPC: "2.0",
				Error: &RPCError{
					Code:    -32602,
					Message: "Invalid params",
					Data:    "filename parameter required",
				},
				ID: req.ID,
			}
		}
	} else {
		return &RPCResponse{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    -32602,
				Message: "Invalid params",
				Data:    "filename parameter required",
			},
			ID: req.ID,
		}
	}

	snapshot, err := s.blockchain.SnapshotManager.LoadSnapshot(filename)
	if err != nil {
		return &RPCResponse{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    -32603,
				Message: "Internal error",
				Data:    err.Error(),
			},
			ID: req.ID,
		}
	}

	// Restore balances from snapshot
	if err := s.blockchain.SnapshotManager.RestoreBalancesFromSnapshot(s.blockchain); err != nil {
		return &RPCResponse{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    -32603,
				Message: "Internal error",
				Data:    err.Error(),
			},
			ID: req.ID,
		}
	}

	return &RPCResponse{
		JSONRPC: "2.0",
		Result: map[string]interface{}{
			"height":       snapshot.Height,
			"timestamp":    snapshot.Timestamp,
			"blockHash":    snapshot.BlockHash,
			"addressCount": len(snapshot.Balances),
			"totalSupply":  snapshot.TotalSupply,
			"restored":     true,
		},
		ID: req.ID,
	}
}

// handleGetSnapshot handles getSnapshot requests
func (s *ServerV2) handleGetSnapshot(req *RPCRequest) *RPCResponse {
	snapshot := s.blockchain.SnapshotManager.GetSnapshot()
	if snapshot == nil {
		return &RPCResponse{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    -32603,
				Message: "Internal error",
				Data:    "No snapshot available",
			},
			ID: req.ID,
		}
	}

	return &RPCResponse{
		JSONRPC: "2.0",
		Result: map[string]interface{}{
			"height":       snapshot.Height,
			"timestamp":    snapshot.Timestamp,
			"blockHash":    snapshot.BlockHash,
			"addressCount": len(snapshot.Balances),
			"totalSupply":  snapshot.TotalSupply,
			"chainId":      snapshot.ChainID,
		},
		ID: req.ID,
	}
}

// handleGetHeight handles getHeight requests
func (s *ServerV2) handleGetHeight(req *RPCRequest) *RPCResponse {
	height := s.blockchain.GetHeight()
	return &RPCResponse{
		JSONRPC: "2.0",
		Result:  height,
		ID:      req.ID,
	}
}

// handleGetBestBlock handles getBestBlock requests
func (s *ServerV2) handleGetBestBlock(req *RPCRequest) *RPCResponse {
	block := s.blockchain.GetBestBlock()
	if block == nil {
		return &RPCResponse{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    -32603,
				Message: "Internal error",
				Data:    "No blocks found",
			},
			ID: req.ID,
		}
	}

	return &RPCResponse{
		JSONRPC: "2.0",
		Result: map[string]interface{}{
			"hash":       hex.EncodeToString(block.Hash[:]),
			"number":     block.Header.Number,
			"merkleRoot": hex.EncodeToString(block.Header.MerkleRoot[:]), // Include merkle root
			"txCount":    block.Header.TxCount,                           // Include tx count
			"difficulty": block.Header.Difficulty,                        // Include difficulty
		},
		ID: req.ID,
	}
}

// handleGetRecentBlocks handles getRecentBlocks requests
func (s *ServerV2) handleGetRecentBlocks(req *RPCRequest) *RPCResponse {
	// Parse parameters
	params, ok := req.Params.(map[string]interface{})
	limit := 20 // Default limit

	if ok {
		if limitVal, ok := params["limit"].(float64); ok {
			limit = int(limitVal)
			if limit > 100 {
				limit = 100
			}
			if limit < 1 {
				limit = 1
			}
		}
	}

	// Get recent blocks
	blocks := s.blockchain.GetRecentBlocks(limit)
	if len(blocks) == 0 {
		return &RPCResponse{
			JSONRPC: "2.0",
			Result:  []interface{}{},
			ID:      req.ID,
		}
	}

	// Convert blocks to JSON-compatible format
	result := make([]map[string]interface{}, 0, len(blocks))
	for _, block := range blocks {
		txCount := uint32(len(block.Txs))
		blockMap := map[string]interface{}{
			"hash":        hex.EncodeToString(block.Hash[:]),
			"number":      block.Header.Number,
			"parentHash":  hex.EncodeToString(block.Header.ParentHash[:]),
			"timestamp":   float64(block.Header.Timestamp.Unix()),
			"difficulty":  block.Header.Difficulty,
			"nonce":       block.Header.Nonce,
			"merkleRoot":  hex.EncodeToString(block.Header.MerkleRoot[:]),
			"txCount":     txCount,
			"networkFee":  block.Header.NetworkFee,
			"treasuryFee": block.Header.TreasuryFee,
			"gasUsed":     uint64(0), // Not used in Kalon
			"gasLimit":    uint64(0), // Not used in Kalon
		}

		// Add miner if available
		if block.Header.Miner != (core.Address{}) {
			blockMap["miner"] = block.Header.Miner.String()
		}

		result = append(result, blockMap)
	}

	return &RPCResponse{
		JSONRPC: "2.0",
		Result:  result,
		ID:      req.ID,
	}
}

// handleCreateBlockTemplateV2 handles createBlockTemplate requests professionally
func (s *ServerV2) handleCreateBlockTemplateV2(req *RPCRequest) *RPCResponse {
	params, ok := req.Params.(map[string]interface{})
	if !ok {
		return &RPCResponse{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    -32602,
				Message: "Invalid params",
			},
			ID: req.ID,
		}
	}

	minerStr, ok := params["miner"].(string)
	if !ok {
		return &RPCResponse{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    -32602,
				Message: "Invalid params",
				Data:    "miner parameter required",
			},
			ID: req.ID,
		}
	}

	// Parse miner address - handle kalon1 + hex format
	var miner core.Address

	if strings.HasPrefix(minerStr, "kalon1") {
		// Remove "kalon1" prefix and decode hex
		hexStr := strings.TrimPrefix(minerStr, "kalon1")
		if len(hexStr) == 40 {
			// Decode 40-char hex to 20 bytes
			decodedBytes, err := hex.DecodeString(hexStr)
			if err == nil && len(decodedBytes) == 20 {
				copy(miner[:], decodedBytes)
				core.LogDebug("Parsed kalon1+hex address successfully")
			} else {
				core.LogError("Failed to decode kalon1+hex: %v", err)
				return &RPCResponse{
					JSONRPC: "2.0",
					Error:   &RPCError{Code: -32602, Message: "Invalid miner address"},
					ID:      req.ID,
				}
			}
		} else {
			// Not a valid hex after kalon1
			core.LogError("Invalid: kalon1 address has wrong length: %d", len(hexStr))
			return &RPCResponse{
				JSONRPC: "2.0",
				Error:   &RPCError{Code: -32602, Message: "Invalid miner address format"},
				ID:      req.ID,
			}
		}
	} else {
		// Try to parse as plain 40-char hex
		if len(minerStr) == 40 {
			decodedBytes, err := hex.DecodeString(minerStr)
			if err == nil && len(decodedBytes) == 20 {
				copy(miner[:], decodedBytes)
				core.LogDebug("Parsed plain 40-char hex address")
			} else {
				core.LogError("Invalid address format: %s", minerStr)
				return &RPCResponse{
					JSONRPC: "2.0",
					Error:   &RPCError{Code: -32602, Message: "Invalid miner address format"},
					ID:      req.ID,
				}
			}
		} else {
			core.LogError("Invalid address format: %s (len=%d)", minerStr, len(minerStr))
			return &RPCResponse{
				JSONRPC: "2.0",
				Error:   &RPCError{Code: -32602, Message: "Invalid miner address format"},
				ID:      req.ID,
			}
		}
	}
	core.LogDebug("Miner address bytes: %x", miner)

	// Get current blockchain state
	bestBlock := s.blockchain.GetBestBlock()
	if bestBlock == nil {
		return &RPCResponse{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    -32603,
				Message: "Internal error",
				Data:    "No blocks found",
			},
			ID: req.ID,
		}
	}

	// Create new block with rewards using CreateNewBlockV2
	block := s.blockchain.CreateNewBlockV2(miner, []core.Transaction{})
	core.LogDebug("Block created with %d transactions", len(block.Txs))
	core.LogDebug("Miner address in block: %x", block.Header.Miner)
	if len(block.Txs) > 0 && len(block.Txs[0].Outputs) > 0 {
		core.LogDebug("Reward TX Output - Address: %x (40 chars: %t)", block.Txs[0].Outputs[0].Address, len(hex.EncodeToString(block.Txs[0].Outputs[0].Address[:])) == 40)
		core.LogDebug("Reward TX Output - Amount: %d", block.Txs[0].Outputs[0].Amount)
	}
	if block == nil {
		return &RPCResponse{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    -32603,
				Message: "Internal error",
				Data:    "Failed to create block template",
			},
			ID: req.ID,
		}
	}

	core.LogDebug("Creating template for block #%d with parent hash: %x", block.Header.Number, block.Header.ParentHash)

	// Serialize transactions properly for JSON response
	txList := make([]interface{}, 0, len(block.Txs))
	for _, tx := range block.Txs {
		txMap := map[string]interface{}{
			"hash":      hex.EncodeToString(tx.Hash[:]),
			"from":      hex.EncodeToString(tx.From[:]), // Safe: hex encoding
			"to":        hex.EncodeToString(tx.To[:]),   // Safe: hex encoding
			"amount":    tx.Amount,
			"fee":       tx.Fee,
			"nonce":     tx.Nonce,
			"timestamp": tx.Timestamp.Unix(),
			"gasUsed":   tx.GasUsed,
			"gasPrice":  tx.GasPrice,
		}

		// Serialize data field (hex-encoded)
		if len(tx.Data) > 0 {
			txMap["data"] = hex.EncodeToString(tx.Data)
		}

		// Serialize signature (hex-encoded) - required for user transactions
		if len(tx.Signature) > 0 {
			txMap["signature"] = hex.EncodeToString(tx.Signature)
		}

		// Serialize public key (hex-encoded) - required for user transactions
		if len(tx.PublicKey) > 0 {
			txMap["publicKey"] = hex.EncodeToString(tx.PublicKey)
		}

		// Serialize outputs
		outputs := make([]interface{}, 0, len(tx.Outputs))
		for _, output := range tx.Outputs {
			outputs = append(outputs, map[string]interface{}{
				"address": hex.EncodeToString(output.Address[:]),
				"amount":  output.Amount,
			})
		}
		txMap["outputs"] = outputs

		// Serialize inputs to be complete
		inputs := make([]interface{}, 0, len(tx.Inputs))
		for _, input := range tx.Inputs {
			inputs = append(inputs, map[string]interface{}{
				"previousTxHash": hex.EncodeToString(input.PreviousTxHash[:]),
				"index":          input.Index,
			})
		}
		txMap["inputs"] = inputs

		txList = append(txList, txMap)
	}

	return &RPCResponse{
		JSONRPC: "2.0",
		Result: map[string]interface{}{
			"number":       block.Header.Number,
			"difficulty":   block.Header.Difficulty,
			"parentHash":   hex.EncodeToString(block.Header.ParentHash[:]),
			"timestamp":    block.Header.Timestamp.Unix(),
			"miner":        hex.EncodeToString(block.Header.Miner[:]),
			"transactions": txList,
		},
		ID: req.ID,
	}
}

// handleSubmitBlockV2 handles submitBlock requests professionally
func (s *ServerV2) handleSubmitBlockV2(req *RPCRequest) (response *RPCResponse) {
	defer func() {
		if r := recover(); r != nil {
			core.LogError("PANIC in handleSubmitBlockV2: %v", r)
			response = &RPCResponse{
				JSONRPC: "2.0",
				Error: &RPCError{
					Code:    -32603,
					Message: "Internal error",
					Data:    fmt.Sprintf("Panic: %v", r),
				},
				ID: req.ID,
			}
		}
	}()

	params, ok := req.Params.(map[string]interface{})
	if !ok {
		return &RPCResponse{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    -32602,
				Message: "Invalid params",
			},
			ID: req.ID,
		}
	}

	blockData, ok := params["block"].(map[string]interface{})
	if !ok {
		return &RPCResponse{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    -32602,
				Message: "Invalid params",
				Data:    "block parameter required",
			},
			ID: req.ID,
		}
	}

	// Parse block data
	block, err := s.parseBlockData(blockData)
	if err != nil {
		core.LogError("Failed to parse block data: %v", err)
		return &RPCResponse{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    -32602,
				Message: "Invalid block data",
				Data:    err.Error(),
			},
			ID: req.ID,
		}
	}

	// Submit block to blockchain using V2 function
	if err := s.blockchain.AddBlockV2(block); err != nil {
		core.LogError("Failed to add block: %v", err)
		return &RPCResponse{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    -32603,
				Message: "Block submission failed",
				Data:    err.Error(),
			},
			ID: req.ID,
		}
	}

	core.LogInfo("Block #%d submitted successfully: %x", block.Header.Number, block.Hash)

	return &RPCResponse{
		JSONRPC: "2.0",
		Result: map[string]interface{}{
			"success": true,
			"hash":    hex.EncodeToString(block.Hash[:]),
			"number":  block.Header.Number,
		},
		ID: req.ID,
	}
}

// parseBlockData parses block data from RPC request
func (s *ServerV2) parseBlockData(data map[string]interface{}) (*core.Block, error) {
	// Parse number
	number, ok := data["number"].(float64)
	if !ok {
		return nil, fmt.Errorf("invalid number")
	}

	// Parse difficulty
	difficulty, ok := data["difficulty"].(float64)
	if !ok {
		return nil, fmt.Errorf("invalid difficulty")
	}

	// Parse nonce
	nonce, ok := data["nonce"].(float64)
	if !ok {
		return nil, fmt.Errorf("invalid nonce")
	}

	// Parse hash
	hashStr, ok := data["hash"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid hash")
	}

	hashBytes, err := hex.DecodeString(hashStr)
	if err != nil || len(hashBytes) != 32 {
		return nil, fmt.Errorf("invalid hash format")
	}

	// Parse parent hash
	parentHashStr, ok := data["parentHash"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid parentHash")
	}

	parentHashBytes, err := hex.DecodeString(parentHashStr)
	if err != nil || len(parentHashBytes) != 32 {
		return nil, fmt.Errorf("invalid parentHash format")
	}

	// Parse timestamp
	timestamp, ok := data["timestamp"].(float64)
	if !ok {
		return nil, fmt.Errorf("invalid timestamp")
	}

	// Parse transactions from block data
	var transactions []core.Transaction
	core.LogDebug("Parsing block data, checking for transactions...")
	if txsData, ok := data["transactions"].([]interface{}); ok {
		core.LogDebug("Found %d transactions in block data", len(txsData))
		for _, txData := range txsData {
			if txMap, ok := txData.(map[string]interface{}); ok {
				// Parse transaction from map
				tx := core.Transaction{}

				// IMPORTANT: Parse transaction hash FIRST so it's available
				if hashStr, ok := txMap["hash"].(string); ok {
					if hashBytes, err := hex.DecodeString(hashStr); err == nil {
						copy(tx.Hash[:], hashBytes)
					}
				}

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
				// Parse data (hex-encoded string)
				if dataStr, ok := txMap["data"].(string); ok {
					if dataBytes, err := hex.DecodeString(dataStr); err == nil {
						tx.Data = dataBytes
					}
				}
				// Parse signature (hex-encoded string from miner)
				if sigStr, ok := txMap["signature"].(string); ok {
					if sigBytes, err := hex.DecodeString(sigStr); err == nil {
						tx.Signature = sigBytes
					} else {
						core.LogWarn("Failed to decode signature hex string: %v", err)
					}
				} else if signature, ok := txMap["signature"].([]byte); ok {
					// Fallback: if sent as []byte (legacy)
					tx.Signature = signature
				}
				// Parse public key (hex-encoded string from miner)
				if pubKeyStr, ok := txMap["publicKey"].(string); ok {
					if pubKeyBytes, err := hex.DecodeString(pubKeyStr); err == nil {
						tx.PublicKey = pubKeyBytes
					} else {
						core.LogWarn("Failed to decode publicKey hex string: %v", err)
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
							if addressValue, ok := outputMap["address"]; ok {
								// Parse address from various formats
								var address core.Address
								addressSet := false

								// Try to parse as string (40-char hex or kalon1... format)
								if addressStr, ok := addressValue.(string); ok {
									// If it's a hex-encoded address (like from Miner)
									if len(addressStr) == 40 && isHexString(addressStr) {
										if decoded, err := hex.DecodeString(addressStr); err == nil && len(decoded) == 20 {
											copy(address[:], decoded)
											addressSet = true
										}
									} else {
										// Use AddressFromString for other formats
										address = core.AddressFromString(addressStr)
										addressSet = true
									}
								}

								// Try to parse as array of numbers
								if !addressSet {
									if addressBytes, ok := addressValue.([]interface{}); ok && len(addressBytes) == 20 {
										var addrBytes []byte
										for _, b := range addressBytes {
											if byteVal, ok := b.(float64); ok {
												addrBytes = append(addrBytes, byte(byteVal))
											}
										}
										if len(addrBytes) == 20 {
											copy(address[:], addrBytes)
											addressSet = true
										}
									}
								}

								if addressSet {
									output.Address = address
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

				// Calculate transaction hash if not set
				if tx.Hash == (core.Hash{}) {
					tx.Hash = core.CalculateTransactionHash(&tx)
				}

				transactions = append(transactions, tx)
				core.LogDebug("Parsed transaction with %d outputs, total amount: %d, hash: %x", len(tx.Outputs), tx.Amount, tx.Hash)
			}
		}
	} else {
		core.LogWarn("DEBUG: No transactions found in block data!")
	}

	core.LogDebug("Total transactions parsed: %d", len(transactions))

	// Parse merkle root
	var merkleRoot core.Hash
	if merkleRootStr, ok := data["merkleRoot"].(string); ok {
		merkleRootBytes, err := hex.DecodeString(merkleRootStr)
		if err == nil && len(merkleRootBytes) == 32 {
			copy(merkleRoot[:], merkleRootBytes)
		}
	}

	// Parse tx count
	var txCount uint32
	if txCountFloat, ok := data["txCount"].(float64); ok {
		txCount = uint32(txCountFloat)
	} else {
		txCount = uint32(len(transactions))
	}

	// Parse miner address
	var miner core.Address
	if minerStr, ok := data["miner"].(string); ok {
		miner = core.AddressFromString(minerStr)
	}

	// Create block with transactions
	block := &core.Block{
		Header: core.BlockHeader{
			Number:     uint64(number),
			Difficulty: uint64(difficulty),
			Nonce:      uint64(nonce),
			Timestamp:  time.Unix(int64(timestamp), 0),
			MerkleRoot: merkleRoot, // Parse merkle root from block data
			TxCount:    txCount,    // Parse tx count from block data
			Miner:      miner,      // Parse miner address
		},
		Txs: transactions, // Use parsed transactions
	}

	// Copy hashes
	copy(block.Hash[:], hashBytes)
	copy(block.Header.ParentHash[:], parentHashBytes)

	return block, nil
}

// handleGetMiningInfo handles getMiningInfo requests
func (s *ServerV2) handleGetMiningInfo(req *RPCRequest) *RPCResponse {
	bestBlock := s.blockchain.GetBestBlock()
	if bestBlock == nil {
		return &RPCResponse{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    -32603,
				Message: "Internal error",
				Data:    "No blocks found",
			},
			ID: req.ID,
		}
	}

	// Use ConsensusManager to calculate difficulty (uses Genesis config)
	consensusManager := core.NewConsensusManager(s.blockchain.GetGenesis())
	// Get block history for LWMA difficulty adjustment
	windowSize := int(s.blockchain.GetGenesis().Difficulty.Window)
	if windowSize == 0 {
		windowSize = 120 // Default window size
	}
	blockHistory := s.blockchain.GetBlockHistoryForDifficulty(bestBlock.Header.Number + 1)

	// IMPORTANT: Include current time in blockHistory for RPC calls
	// This ensures miners get the correct difficulty immediately, even if no blocks were found recently
	// Without this, miners would mine with old difficulty until they find a block
	currentTime := time.Now()
	if len(blockHistory) > 0 {
		// Extend blockHistory with current time as "virtual block"
		// This allows CalculateDifficulty to consider time since last block
		extendedHistory := make([]time.Time, len(blockHistory)+1)
		copy(extendedHistory, blockHistory)
		extendedHistory[len(blockHistory)] = currentTime
		blockHistory = extendedHistory
	}

	difficulty := consensusManager.CalculateDifficulty(bestBlock.Header.Number+1, bestBlock, blockHistory)

	// Get max mining threads from genesis config
	maxMiningThreads := uint64(0)
	if s.blockchain.GetGenesis().Difficulty.MaxMiningThreads > 0 {
		maxMiningThreads = s.blockchain.GetGenesis().Difficulty.MaxMiningThreads
	}

	return &RPCResponse{
		JSONRPC: "2.0",
		Result: map[string]interface{}{
			"height":           s.blockchain.GetHeight(),
			"difficulty":       difficulty,
			"bestBlock":        hex.EncodeToString(bestBlock.Hash[:]),
			"maxMiningThreads": maxMiningThreads,
		},
		ID: req.ID,
	}
}

// handleGetBalance handles getBalance requests
func (s *ServerV2) handleGetBalance(req *RPCRequest) *RPCResponse {
	// Parse parameters
	params, ok := req.Params.(map[string]interface{})
	if !ok {
		return &RPCResponse{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    -32602,
				Message: "Invalid params",
				Data:    "Expected object with 'address' field",
			},
			ID: req.ID,
		}
	}

	addressStr, ok := params["address"].(string)
	if !ok {
		return &RPCResponse{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    -32602,
				Message: "Invalid params",
				Data:    "Missing or invalid 'address' field",
			},
			ID: req.ID,
		}
	}

	// Convert string address to Address type
	address := core.AddressFromString(addressStr)

	// Get balance from blockchain
	balance := s.blockchain.GetBalance(address)

	// Debug logging
	core.LogDebug("Balance query - Address: %s, Parsed: %s, Balance: %d", addressStr, hex.EncodeToString(address[:]), balance)

	return &RPCResponse{
		JSONRPC: "2.0",
		Result:  balance,
		ID:      req.ID,
	}
}

// handlePrepareTransaction prepares a transaction structure for signing (returns unsigned transaction)
func (s *ServerV2) handlePrepareTransaction(req *RPCRequest) *RPCResponse {
	params, ok := req.Params.(map[string]interface{})
	if !ok {
		return &RPCResponse{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    -32602,
				Message: "Invalid params",
			},
			ID: req.ID,
		}
	}

	fromStr, _ := params["from"].(string)
	toStr, _ := params["to"].(string)
	amount, _ := params["amount"].(float64)
	fee, _ := params["fee"].(float64)

	if fromStr == "" || toStr == "" || amount == 0 {
		return &RPCResponse{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    -32602,
				Message: "Invalid params",
				Data:    "from, to and amount are required",
			},
			ID: req.ID,
		}
	}

	fromAddr := core.AddressFromString(fromStr)
	toAddr := core.AddressFromString(toStr)

	// Create transaction from UTXOs
	tx, err := s.blockchain.CreateTransaction(fromAddr, toAddr, uint64(amount), uint64(fee))
	if err != nil {
		return &RPCResponse{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    -32603,
				Message: "Transaction creation failed",
				Data:    err.Error(),
			},
			ID: req.ID,
		}
	}

	// Serialize transaction for client to sign
	inputs := make([]interface{}, 0, len(tx.Inputs))
	for _, input := range tx.Inputs {
		inputs = append(inputs, map[string]interface{}{
			"previousTxHash": hex.EncodeToString(input.PreviousTxHash[:]),
			"index":          input.Index,
		})
	}

	outputs := make([]interface{}, 0, len(tx.Outputs))
	for _, output := range tx.Outputs {
		outputs = append(outputs, map[string]interface{}{
			"address": hex.EncodeToString(output.Address[:]),
			"amount":  output.Amount,
		})
	}

	return &RPCResponse{
		JSONRPC: "2.0",
		Result: map[string]interface{}{
			"from":      hex.EncodeToString(tx.From[:]),
			"to":        hex.EncodeToString(tx.To[:]),
			"amount":    tx.Amount,
			"fee":       tx.Fee,
			"nonce":     tx.Nonce,
			"gasUsed":   tx.GasUsed,
			"gasPrice":  tx.GasPrice,
			"data":      hex.EncodeToString(tx.Data),
			"hash":      hex.EncodeToString(tx.Hash[:]),
			"inputs":    inputs,
			"outputs":   outputs,
			"timestamp": tx.Timestamp.Unix(),
		},
		ID: req.ID,
	}
}

// handleSendTransaction handles sendTransaction requests
func (s *ServerV2) handleSendTransaction(req *RPCRequest) *RPCResponse {
	params, ok := req.Params.(map[string]interface{})
	if !ok {
		return &RPCResponse{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    -32602,
				Message: "Invalid params",
			},
			ID: req.ID,
		}
	}

	// Check if a fully signed transaction is provided
	if txData, ok := params["transaction"].(map[string]interface{}); ok {
		// Parse fully signed transaction
		tx := core.Transaction{}

		// Parse addresses
		if fromStr, ok := txData["from"].(string); ok {
			tx.From = core.AddressFromString(fromStr)
		}
		if toStr, ok := txData["to"].(string); ok {
			tx.To = core.AddressFromString(toStr)
		}

		// Parse amounts
		if amount, ok := txData["amount"].(float64); ok {
			tx.Amount = uint64(amount)
		}
		if fee, ok := txData["fee"].(float64); ok {
			tx.Fee = uint64(fee)
		}
		if nonce, ok := txData["nonce"].(float64); ok {
			tx.Nonce = uint64(nonce)
		}
		if gasUsed, ok := txData["gasUsed"].(float64); ok {
			tx.GasUsed = uint64(gasUsed)
		}
		if gasPrice, ok := txData["gasPrice"].(float64); ok {
			tx.GasPrice = uint64(gasPrice)
		}

		// Parse signature and public key
		if sigStr, ok := txData["signature"].(string); ok {
			if sigBytes, err := hex.DecodeString(sigStr); err == nil {
				tx.Signature = sigBytes
			}
		}
		if pubKeyStr, ok := txData["publicKey"].(string); ok {
			if pubKeyBytes, err := hex.DecodeString(pubKeyStr); err == nil {
				tx.PublicKey = pubKeyBytes
			}
		}

		// Parse hash
		if hashStr, ok := txData["hash"].(string); ok {
			if hashBytes, err := hex.DecodeString(hashStr); err == nil && len(hashBytes) == 32 {
				copy(tx.Hash[:], hashBytes)
			}
		} else {
			// Calculate hash if not provided
			tx.Hash = tx.CalculateHash()
		}

		// Parse data
		if dataStr, ok := txData["data"].(string); ok {
			if dataBytes, err := hex.DecodeString(dataStr); err == nil {
				tx.Data = dataBytes
			}
		}

		// Parse inputs
		if inputsData, ok := txData["inputs"].([]interface{}); ok {
			for _, inputData := range inputsData {
				if inputMap, ok := inputData.(map[string]interface{}); ok {
					input := core.TxInput{}
					if prevTxHashStr, ok := inputMap["previousTxHash"].(string); ok {
						if prevTxHashBytes, err := hex.DecodeString(prevTxHashStr); err == nil && len(prevTxHashBytes) == 32 {
							copy(input.PreviousTxHash[:], prevTxHashBytes)
						}
					}
					if index, ok := inputMap["index"].(float64); ok {
						input.Index = uint32(index)
					}
					tx.Inputs = append(tx.Inputs, input)
				}
			}
		}

		// Parse outputs
		if outputsData, ok := txData["outputs"].([]interface{}); ok {
			for _, outputData := range outputsData {
				if outputMap, ok := outputData.(map[string]interface{}); ok {
					output := core.TxOutput{}
					if addrStr, ok := outputMap["address"].(string); ok {
						output.Address = core.AddressFromString(addrStr)
					}
					if amount, ok := outputMap["amount"].(float64); ok {
						output.Amount = uint64(amount)
					}
					tx.Outputs = append(tx.Outputs, output)
				}
			}
		}

		// Debug: Log transaction details before validation
		core.LogInfo("Transaction before validation - From: %s, To: %s, Amount: %d, Fee: %d, Nonce: %d, Inputs: %d, Outputs: %d, HasSignature: %v, HasPublicKey: %v",
			hex.EncodeToString(tx.From[:]), hex.EncodeToString(tx.To[:]), tx.Amount, tx.Fee, tx.Nonce, len(tx.Inputs), len(tx.Outputs), len(tx.Signature) > 0, len(tx.PublicKey) > 0)

		// Check transaction size BEFORE validation (DoS protection)
		txSize := estimateTransactionSize(&tx)
		if txSize > MaxTransactionSizeBytes {
			core.LogWarn("Transaction size exceeds limit: %d bytes (max: %d bytes)", txSize, MaxTransactionSizeBytes)
			return &RPCResponse{
				JSONRPC: "2.0",
				Error: &RPCError{
					Code:    -32603,
					Message: "Transaction too large",
					Data:    fmt.Sprintf("Transaction size (%d bytes) exceeds maximum allowed size (%d bytes)", txSize, MaxTransactionSizeBytes),
				},
				ID: req.ID,
			}
		}

		// Validate signed transaction
		consensusManager := core.NewConsensusManager(s.blockchain.GetGenesis())
		if err := consensusManager.ValidateTransaction(&tx); err != nil {
			core.LogInfo("Transaction validation failed: %v", err)
			return &RPCResponse{
				JSONRPC: "2.0",
				Error: &RPCError{
					Code:    -32603,
					Message: "Transaction validation failed",
					Data:    err.Error(),
				},
				ID: req.ID,
			}
		}

		// Debug: Log transaction details before adding to mempool
		core.LogInfo("Signed transaction received - From: %s, To: %s, Amount: %d, Hash: %x, Inputs: %d, Outputs: %d",
			hex.EncodeToString(tx.From[:]), hex.EncodeToString(tx.To[:]), tx.Amount, tx.Hash, len(tx.Inputs), len(tx.Outputs))

		// Add to mempool
		if err := s.blockchain.GetMempool().AddTransaction(&tx); err != nil {
			return &RPCResponse{
				JSONRPC: "2.0",
				Error: &RPCError{
					Code:    -32603,
					Message: "Failed to add transaction to mempool",
					Data:    err.Error(),
				},
				ID: req.ID,
			}
		}

		return &RPCResponse{
			JSONRPC: "2.0",
			Result: map[string]interface{}{
				"txHash": hex.EncodeToString(tx.Hash[:]),
				"status": "pending",
			},
			ID: req.ID,
		}
	}

	// Parse transaction fields (legacy mode - server creates transaction)
	fromStr, _ := params["from"].(string)
	toStr, _ := params["to"].(string)
	amount, _ := params["amount"].(float64)
	fee, _ := params["fee"].(float64)

	if fromStr == "" || toStr == "" || amount == 0 {
		return &RPCResponse{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    -32602,
				Message: "Invalid params",
				Data:    "from, to and amount are required",
			},
			ID: req.ID,
		}
	}

	// Get addresses
	fromAddr := core.AddressFromString(fromStr)
	toAddr := core.AddressFromString(toStr)

	// Create transaction from UTXOs
	tx, err := s.blockchain.CreateTransaction(fromAddr, toAddr, uint64(amount), uint64(fee))
	if err != nil {
		return &RPCResponse{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    -32603,
				Message: "Transaction creation failed",
				Data:    err.Error(),
			},
			ID: req.ID,
		}
	}

	core.LogInfo("Transaction created - From: %s, To: %s, Amount: %d, Hash: %x", fromStr, toStr, tx.Amount, tx.Hash)

	// Check transaction size BEFORE validation (DoS protection)
	txSize := estimateTransactionSize(tx)
	if txSize > MaxTransactionSizeBytes {
		core.LogWarn("Transaction size exceeds limit: %d bytes (max: %d bytes)", txSize, MaxTransactionSizeBytes)
		return &RPCResponse{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    -32603,
				Message: "Transaction too large",
				Data:    fmt.Sprintf("Transaction size (%d bytes) exceeds maximum allowed size (%d bytes)", txSize, MaxTransactionSizeBytes),
			},
			ID: req.ID,
		}
	}

	// NOTE: Transaction created by server is not signed - validation will fail
	// This is expected behavior - client should send signed transaction
	// For now, we skip validation for server-created transactions (legacy mode)
	// In production, all transactions should be signed by client
	consensusManager := core.NewConsensusManager(s.blockchain.GetGenesis())
	if err := consensusManager.ValidateTransaction(tx); err != nil {
		// Return error indicating transaction needs to be signed
		return &RPCResponse{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    -32603,
				Message: "Transaction validation failed",
				Data:    "Transaction must be signed by client. Please send a fully signed transaction using the 'transaction' parameter.",
			},
			ID: req.ID,
		}
	}

	// Add to mempool
	s.blockchain.GetMempool().AddTransaction(tx)

	// Return transaction hash
	return &RPCResponse{
		JSONRPC: "2.0",
		Result: map[string]interface{}{
			"txHash": hex.EncodeToString(tx.Hash[:]),
			"status": "pending",
		},
		ID: req.ID,
	}
}

// handleHealth handles health check requests
func (s *ServerV2) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now(),
		"version":   "2.0",
		"height":    s.blockchain.GetHeight(),
	})
}

// writeError writes an error response
func (s *ServerV2) writeError(w http.ResponseWriter, id interface{}, code int, message, data string) {
	response := RPCResponse{
		JSONRPC: "2.0",
		Error: &RPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
		ID: id,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// estimateTransactionSize estimates the size of a transaction in bytes
// This is a local copy of the function from core/blockchain.go for use in RPC server
func estimateTransactionSize(tx *core.Transaction) uint64 {
	return 32 + // hash
		uint64(20+20) + // from + to addresses
		uint64(8+8) + // amount + fee
		uint64(8) + // nonce
		uint64(8+8) + // gasUsed + gasPrice
		uint64(len(tx.Signature)+len(tx.PublicKey)) + // signature + public key
		uint64(len(tx.Data)) + // data
		uint64(len(tx.Inputs)*40) + // inputs (32-byte hash + 4-byte index + 4-byte signature estimate)
		uint64(len(tx.Outputs)*28) + // outputs (20-byte address + 8-byte amount)
		uint64(8) // timestamp
}

// limitConnections limits concurrent connections professionally
func (s *ServerV2) limitConnections(h http.Handler) http.Handler {
	semaphore := make(chan struct{}, 50) // Max 50 concurrent connections

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case semaphore <- struct{}{}:
			defer func() { <-semaphore }()
			h.ServeHTTP(w, r)
		default:
			http.Error(w, "Too many connections", http.StatusServiceUnavailable)
		}
	})
}

// trackConnection tracks a client connection
func (s *ServerV2) trackConnection(connID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if conn, exists := s.connections[connID]; exists {
		conn.LastSeen = time.Now()
		conn.Requests++
	} else {
		s.connections[connID] = &Connection{
			ID:        connID,
			CreatedAt: time.Now(),
			LastSeen:  time.Now(),
			Requests:  1,
		}
	}
}

// cleanupConnections cleans up old connections
func (s *ServerV2) cleanupConnections() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.mu.Lock()
			now := time.Now()
			for id, conn := range s.connections {
				if now.Sub(conn.LastSeen) > 10*time.Minute {
					delete(s.connections, id)
				}
			}
			s.mu.Unlock()
		case <-s.ctx.Done():
			return
		}
	}
}

// Helper function to check if string is hex
func isHexString(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// extractIP extracts the client IP from the request
func (s *ServerV2) extractIP(r *http.Request) string {
	ip := r.Header.Get("X-Forwarded-For")
	if ip == "" {
		ip = r.Header.Get("X-Real-Ip")
	}
	if ip == "" {
		ip = strings.Split(r.RemoteAddr, ":")[0]
	}
	return ip
}

// checkBlockSubmissionRateLimit checks if block submission rate is within limits
// Returns true if submission is allowed, false if rate limit exceeded
func (s *ServerV2) checkBlockSubmissionRateLimit(ip string) bool {
	// CRITICAL: Allow unlimited block submissions from localhost (for testing)
	if ip == "127.0.0.1" || ip == "::1" || ip == "localhost" {
		return true
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	// Get or create submission history for this IP
	submissions, exists := s.blockSubmissionLimits[ip]
	if !exists {
		submissions = make([]time.Time, 0)
		s.blockSubmissionLimits[ip] = submissions
	}

	// Remove submissions older than 1 minute
	validSubmissions := make([]time.Time, 0)
	for _, ts := range submissions {
		if now.Sub(ts) <= time.Minute {
			validSubmissions = append(validSubmissions, ts)
		}
	}

	// Check if limit exceeded
	if len(validSubmissions) >= MaxBlockSubmissionsPerMinute {
		core.LogWarn("Block submission rate limit exceeded for IP %s: %d submissions in last minute (max: %d)",
			ip, len(validSubmissions), MaxBlockSubmissionsPerMinute)
		return false
	}

	// Add current submission timestamp
	validSubmissions = append(validSubmissions, now)
	s.blockSubmissionLimits[ip] = validSubmissions

	core.LogDebug("Block submission allowed for IP %s: %d/%d submissions in last minute",
		ip, len(validSubmissions), MaxBlockSubmissionsPerMinute)

	return true
}

// handleGetPeerCount handles getPeerCount requests
func (s *ServerV2) handleGetPeerCount(req *RPCRequest) *RPCResponse {
	s.mu.RLock()
	p2p := s.p2pNetwork
	s.mu.RUnlock()

	if p2p != nil {
		count := p2p.GetPeerCount()
		return &RPCResponse{
			JSONRPC: "2.0",
			Result:  count,
			ID:      req.ID,
		}
	}

	// Return 0 if P2P network not available
	return &RPCResponse{
		JSONRPC: "2.0",
		Result:  0,
		ID:      req.ID,
	}
}

// handleGetTotalTransactions handles getTotalTransactions requests
func (s *ServerV2) handleGetTotalTransactions(req *RPCRequest) *RPCResponse {
	total := s.blockchain.GetTotalTransactions()
	return &RPCResponse{
		JSONRPC: "2.0",
		Result:  total,
		ID:      req.ID,
	}
}

// handleGetPendingTransactions handles getPendingTransactions requests
func (s *ServerV2) handleGetPendingTransactions(req *RPCRequest) *RPCResponse {
	pendingTxs := s.blockchain.GetMempool().GetPendingTransactions()
	return &RPCResponse{
		JSONRPC: "2.0",
		Result: map[string]interface{}{
			"count":        len(pendingTxs),
			"transactions": pendingTxs,
		},
		ID: req.ID,
	}
}

// handleGetAddressCount handles getAddressCount requests
func (s *ServerV2) handleGetAddressCount(req *RPCRequest) *RPCResponse {
	count := s.blockchain.GetAddressCount()
	return &RPCResponse{
		JSONRPC: "2.0",
		Result:  count,
		ID:      req.ID,
	}
}

// handleGetTreasuryBalance handles getTreasuryBalance requests
func (s *ServerV2) handleGetTreasuryBalance(req *RPCRequest) *RPCResponse {
	balance := s.blockchain.GetTreasuryBalance()
	return &RPCResponse{
		JSONRPC: "2.0",
		Result:  balance,
		ID:      req.ID,
	}
}

// handleGetHashrate handles getHashrate requests
func (s *ServerV2) handleGetHashrate(req *RPCRequest) *RPCResponse {
	bestBlock := s.blockchain.GetBestBlock()
	if bestBlock == nil {
		return &RPCResponse{
			JSONRPC: "2.0",
			Result: map[string]interface{}{
				"hashrate":   0.0,
				"difficulty": 0,
			},
			ID: req.ID,
		}
	}

	// Calculate hashrate from difficulty
	// Hashrate = 2^(64-difficulty) / blockTime
	difficulty := bestBlock.Header.Difficulty
	blockTime := 15.0 // Target block time in seconds (from genesis config)

	// Calculate hashrate: 2^(64-difficulty) / blockTime
	// For simplicity, we use a simplified formula
	// Note: difficulty is uint64, so we need to handle overflow
	var hashrate float64
	if difficulty < 64 {
		hashrate = float64(uint64(1)<<(64-difficulty)) / blockTime
	} else {
		// For very high difficulty, use a simplified calculation
		hashrate = float64(1) / blockTime
	}

	return &RPCResponse{
		JSONRPC: "2.0",
		Result: map[string]interface{}{
			"hashrate":   hashrate,
			"difficulty": difficulty,
		},
		ID: req.ID,
	}
}

// handleGetAddressInfo handles getAddressInfo requests
func (s *ServerV2) handleGetAddressInfo(req *RPCRequest) *RPCResponse {
	// Parse parameters
	params, ok := req.Params.(map[string]interface{})
	if !ok {
		return &RPCResponse{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    -32602,
				Message: "Invalid params",
				Data:    "Expected object with 'address' field",
			},
			ID: req.ID,
		}
	}

	addressStr, ok := params["address"].(string)
	if !ok {
		return &RPCResponse{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    -32602,
				Message: "Invalid params",
				Data:    "Missing or invalid 'address' field",
			},
			ID: req.ID,
		}
	}

	// Convert string address to Address type
	address := core.AddressFromString(addressStr)

	// Get balance
	balance := s.blockchain.GetBalance(address)

	// Get transactions
	transactions := s.blockchain.GetAddressTransactions(address)

	// Count sent and received
	sentCount := uint64(0)
	receivedCount := uint64(0)
	totalSent := uint64(0)
	totalReceived := uint64(0)

	for _, tx := range transactions {
		if tx.From == address {
			sentCount++
			totalSent += tx.Amount + tx.Fee
		}
		if tx.To == address {
			receivedCount++
			totalReceived += tx.Amount
		}
		// Also check outputs
		for _, output := range tx.Outputs {
			if output.Address == address {
				receivedCount++
				totalReceived += output.Amount
			}
		}
	}

	return &RPCResponse{
		JSONRPC: "2.0",
		Result: map[string]interface{}{
			"address":          addressStr,
			"balance":          balance,
			"transactionCount": len(transactions),
			"sentCount":        sentCount,
			"receivedCount":    receivedCount,
			"totalSent":        totalSent,
			"totalReceived":    totalReceived,
		},
		ID: req.ID,
	}
}

// handleGetAddressTransactions handles getAddressTransactions requests
func (s *ServerV2) handleGetAddressTransactions(req *RPCRequest) *RPCResponse {
	// Parse parameters
	params, ok := req.Params.(map[string]interface{})
	if !ok {
		return &RPCResponse{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    -32602,
				Message: "Invalid params",
				Data:    "Expected object with 'address' field",
			},
			ID: req.ID,
		}
	}

	addressStr, ok := params["address"].(string)
	if !ok {
		return &RPCResponse{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    -32602,
				Message: "Invalid params",
				Data:    "Missing or invalid 'address' field",
			},
			ID: req.ID,
		}
	}

	// Convert string address to Address type
	address := core.AddressFromString(addressStr)

	// Get transactions
	transactions := s.blockchain.GetAddressTransactions(address)

	// Convert to JSON-compatible format
	txList := make([]map[string]interface{}, 0, len(transactions))
	for _, tx := range transactions {
		txMap := map[string]interface{}{
			"hash":      hex.EncodeToString(tx.Hash[:]),
			"from":      hex.EncodeToString(tx.From[:]),
			"to":        hex.EncodeToString(tx.To[:]),
			"amount":    tx.Amount,
			"fee":       tx.Fee,
			"timestamp": tx.Timestamp.Unix(),
		}
		txList = append(txList, txMap)
	}

	return &RPCResponse{
		JSONRPC: "2.0",
		Result: map[string]interface{}{
			"address":      addressStr,
			"count":        len(transactions),
			"transactions": txList,
		},
		ID: req.ID,
	}
}

// serializeBlock serializes a block to JSON-compatible format
func (s *ServerV2) serializeBlock(block *core.Block) map[string]interface{} {
	if block == nil {
		return nil
	}

	// Serialize transactions
	txList := make([]interface{}, 0, len(block.Txs))
	for _, tx := range block.Txs {
		txMap := map[string]interface{}{
			"hash":      hex.EncodeToString(tx.Hash[:]),
			"from":      hex.EncodeToString(tx.From[:]),
			"to":        hex.EncodeToString(tx.To[:]),
			"amount":    tx.Amount,
			"fee":       tx.Fee,
			"nonce":     tx.Nonce,
			"timestamp": tx.Timestamp.Unix(),
		}

		// Serialize outputs
		outputs := make([]interface{}, 0, len(tx.Outputs))
		for _, output := range tx.Outputs {
			outputs = append(outputs, map[string]interface{}{
				"address": hex.EncodeToString(output.Address[:]),
				"amount":  output.Amount,
			})
		}
		txMap["outputs"] = outputs

		// Serialize inputs
		inputs := make([]interface{}, 0, len(tx.Inputs))
		for _, input := range tx.Inputs {
			inputs = append(inputs, map[string]interface{}{
				"previousTxHash": hex.EncodeToString(input.PreviousTxHash[:]),
				"index":          input.Index,
			})
		}
		txMap["inputs"] = inputs

		txList = append(txList, txMap)
	}

	txCount := uint32(len(block.Txs))
	blockMap := map[string]interface{}{
		"hash":         hex.EncodeToString(block.Hash[:]),
		"number":       block.Header.Number,
		"parentHash":   hex.EncodeToString(block.Header.ParentHash[:]),
		"timestamp":    float64(block.Header.Timestamp.Unix()),
		"difficulty":   block.Header.Difficulty,
		"nonce":        block.Header.Nonce,
		"merkleRoot":   hex.EncodeToString(block.Header.MerkleRoot[:]),
		"txCount":      txCount,
		"networkFee":   block.Header.NetworkFee,
		"treasuryFee":  block.Header.TreasuryFee,
		"transactions": txList,
	}

	// Add miner if available
	if block.Header.Miner != (core.Address{}) {
		blockMap["miner"] = block.Header.Miner.String()
	}

	return blockMap
}

// handleGetBlockByHash handles getBlockByHash requests
func (s *ServerV2) handleGetBlockByHash(req *RPCRequest) *RPCResponse {
	// Parse parameters
	params, ok := req.Params.(map[string]interface{})
	if !ok {
		return &RPCResponse{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    -32602,
				Message: "Invalid params",
				Data:    "Expected object with 'hash' field",
			},
			ID: req.ID,
		}
	}

	hashStr, ok := params["hash"].(string)
	if !ok {
		return &RPCResponse{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    -32602,
				Message: "Invalid params",
				Data:    "Missing or invalid 'hash' field",
			},
			ID: req.ID,
		}
	}

	// Remove "0x" prefix if present
	hashStr = strings.TrimPrefix(hashStr, "0x")

	// Decode hex hash
	hashBytes, err := hex.DecodeString(hashStr)
	if err != nil || len(hashBytes) != 32 {
		return &RPCResponse{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    -32602,
				Message: "Invalid params",
				Data:    "Invalid hash format: expected 64-character hex string",
			},
			ID: req.ID,
		}
	}

	// Get block by hash
	block, err := s.blockchain.GetBlockByHash(hashBytes)
	if err != nil {
		return &RPCResponse{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    -32603,
				Message: "Block not found",
				Data:    err.Error(),
			},
			ID: req.ID,
		}
	}

	// Serialize block
	blockMap := s.serializeBlock(block)

	return &RPCResponse{
		JSONRPC: "2.0",
		Result:  blockMap,
		ID:      req.ID,
	}
}

// handleGetBlockByNumber handles getBlockByNumber requests
func (s *ServerV2) handleGetBlockByNumber(req *RPCRequest) *RPCResponse {
	// Parse parameters
	params, ok := req.Params.(map[string]interface{})
	if !ok {
		return &RPCResponse{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    -32602,
				Message: "Invalid params",
				Data:    "Expected object with 'number' field",
			},
			ID: req.ID,
		}
	}

	number, ok := params["number"].(float64)
	if !ok {
		return &RPCResponse{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    -32602,
				Message: "Invalid params",
				Data:    "Missing or invalid 'number' field",
			},
			ID: req.ID,
		}
	}

	// Get block by number
	block, err := s.blockchain.GetBlockByNumber(uint64(number))
	if err != nil {
		return &RPCResponse{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    -32603,
				Message: "Block not found",
				Data:    err.Error(),
			},
			ID: req.ID,
		}
	}

	// Serialize block
	blockMap := s.serializeBlock(block)

	return &RPCResponse{
		JSONRPC: "2.0",
		Result:  blockMap,
		ID:      req.ID,
	}
}

// checkRateLimit checks if the request rate is within limits
func (s *ServerV2) checkRateLimit(ip string) bool {
	// CRITICAL: Allow unlimited requests from localhost FIRST (before any locking)
	// This prevents rate limiting during local testing
	if ip == "127.0.0.1" || ip == "::1" {
		return true
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Get or create rate limit for this IP
	limit, exists := s.rateLimits[ip]
	if !exists {
		limit = &RateLimit{
			Count:          0,
			LastReset:      time.Now(),
			RequestsPerMin: 600, // Increased to 600 requests per minute for testing
		}
		s.rateLimits[ip] = limit
	}

	// Reset counter if more than a minute has passed
	if time.Since(limit.LastReset) > time.Minute {
		limit.Count = 0
		limit.LastReset = time.Now()
	}

	// Check if limit exceeded
	if limit.Count >= limit.RequestsPerMin {
		return false
	}

	// Increment counter
	limit.Count++
	return true
}
