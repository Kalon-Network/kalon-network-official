package network

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

// P2PConfig represents P2P network configuration
type P2PConfig struct {
	ListenAddr    string
	SeedNodes     []string
	MaxPeers      int
	DialTimeout   time.Duration
	ReadTimeout   time.Duration
	WriteTimeout  time.Duration
	KeepAlive     time.Duration
	DiscoveryPort int
}

// P2P represents the P2P network manager
type P2P struct {
	config    *P2PConfig
	listener  net.Listener
	peers     map[string]*Peer
	peerMutex sync.RWMutex
	running   bool
	stopChan  chan struct{}
	blockChan chan *Block
	txChan    chan *Transaction
	mu        sync.RWMutex
	// Whitelist of allowed peer IPs (without port)
	allowedIPs map[string]bool
	// Callback functions for blockchain integration
	onBlockReceived       func(*Block) error
	onTransactionReceived func(*Transaction) error
	onGetBlocksRequest    func(startHeight uint64, endHeight uint64) ([]*Block, error)
}

// Peer represents a connected peer
type Peer struct {
	ID        string
	Address   string
	Conn      net.Conn
	LastSeen  time.Time
	Connected bool
	Version   string
	Height    uint64
	Services  uint64
	UserAgent string
	mu        sync.RWMutex
}

// Block represents a blockchain block
type Block struct {
	Header BlockHeader
	Txs    []Transaction
	Hash   [32]byte
}

// BlockHeader represents a block header
type BlockHeader struct {
	ParentHash  [32]byte
	Number      uint64
	Timestamp   time.Time
	Difficulty  uint64
	Miner       [20]byte
	Nonce       uint64
	MerkleRoot  [32]byte
	TxCount     uint32
	NetworkFee  uint64
	TreasuryFee uint64
}

// Transaction represents a blockchain transaction
type Transaction struct {
	From      [20]byte
	To        [20]byte
	Amount    uint64
	Nonce     uint64
	Fee       uint64
	GasUsed   uint64
	GasPrice  uint64
	Data      []byte
	Signature []byte
	Hash      [32]byte
}

// Message represents a P2P message
type Message struct {
	Type    string      `json:"type"`
	Data    interface{} `json:"data"`
	ID      string      `json:"id"`
	Version string      `json:"version"`
	Time    time.Time   `json:"time"`
}

// NewP2P creates a new P2P network manager
func NewP2P(config *P2PConfig) *P2P {
	// Build whitelist from seed nodes
	allowedIPs := make(map[string]bool)
	for _, seedNode := range config.SeedNodes {
		// Extract IP from "IP:PORT" format
		if host, _, err := net.SplitHostPort(seedNode); err == nil {
			allowedIPs[host] = true
		} else {
			// If no port, use as-is
			allowedIPs[seedNode] = true
		}
	}

	return &P2P{
		config:     config,
		peers:      make(map[string]*Peer),
		stopChan:   make(chan struct{}),
		blockChan:  make(chan *Block, 1000), // Increased from 100 to 1000 to prevent "Block channel full" errors
		txChan:     make(chan *Transaction, 1000),
		allowedIPs: allowedIPs,
	}
}

// SetAllowedIPs sets the whitelist of allowed peer IPs
func (p *P2P) SetAllowedIPs(ips []string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	p.allowedIPs = make(map[string]bool)
	for _, ip := range ips {
		// Extract IP from "IP:PORT" format if needed
		if host, _, err := net.SplitHostPort(ip); err == nil {
			p.allowedIPs[host] = true
		} else {
			p.allowedIPs[ip] = true
		}
	}
}

// SetBlockHandler sets the callback function for received blocks
func (p *P2P) SetBlockHandler(handler func(*Block) error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onBlockReceived = handler
}

// SetTransactionHandler sets the callback function for received transactions
func (p *P2P) SetTransactionHandler(handler func(*Transaction) error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onTransactionReceived = handler
}

// SetGetBlocksHandler sets the callback function for get_blocks requests
func (p *P2P) SetGetBlocksHandler(handler func(startHeight uint64, endHeight uint64) ([]*Block, error)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onGetBlocksRequest = handler
}

// Start starts the P2P network
func (p *P2P) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return fmt.Errorf("P2P network is already running")
	}

	// Start listening
	listener, err := net.Listen("tcp", p.config.ListenAddr)
	if err != nil {
		return fmt.Errorf("failed to start listener: %v", err)
	}

	p.listener = listener
	p.running = true

	// Start accepting connections
	go p.acceptConnections()

	// Start peer discovery
	go p.discoverPeers()

	// Start peer maintenance
	go p.maintainPeers()

	log.Printf("P2P network started on %s", p.config.ListenAddr)

	return nil
}

// Stop stops the P2P network
func (p *P2P) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return
	}

	p.running = false
	close(p.stopChan)

	// Close listener
	if p.listener != nil {
		p.listener.Close()
	}

	// Close all peer connections
	p.peerMutex.Lock()
	for _, peer := range p.peers {
		if peer.Conn != nil {
			peer.Conn.Close()
		}
	}
	p.peers = make(map[string]*Peer)
	p.peerMutex.Unlock()

	log.Println("P2P network stopped")
}

// IsRunning returns true if the P2P network is running
func (p *P2P) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.running
}

// GetPeers returns a list of connected peers
func (p *P2P) GetPeers() []*Peer {
	p.peerMutex.RLock()
	defer p.peerMutex.RUnlock()

	peers := make([]*Peer, 0, len(p.peers))
	for _, peer := range p.peers {
		peers = append(peers, peer)
	}

	return peers
}

// GetPeerCount returns the number of connected peers
func (p *P2P) GetPeerCount() int {
	p.peerMutex.RLock()
	defer p.peerMutex.RUnlock()

	return len(p.peers)
}

// BroadcastBlock broadcasts a block to all peers
func (p *P2P) BroadcastBlock(block *Block) error {
	message := &Message{
		Type:    "block",
		Data:    block,
		ID:      fmt.Sprintf("%x", block.Hash),
		Version: "1.0",
		Time:    time.Now(),
	}

	return p.broadcastMessage(message)
}

// BroadcastTransaction broadcasts a transaction to all peers
func (p *P2P) BroadcastTransaction(tx *Transaction) error {
	message := &Message{
		Type:    "transaction",
		Data:    tx,
		ID:      fmt.Sprintf("%x", tx.Hash),
		Version: "1.0",
		Time:    time.Now(),
	}

	return p.broadcastMessage(message)
}

// RequestBlocks requests blocks from a specific peer
func (p *P2P) RequestBlocks(peerID string, startHeight uint64, endHeight uint64) error {
	p.peerMutex.RLock()
	peer, exists := p.peers[peerID]
	p.peerMutex.RUnlock()

	if !exists {
		return fmt.Errorf("peer %s not found", peerID)
	}

	message := &Message{
		Type:    "get_blocks",
		Data:    map[string]interface{}{"startHeight": startHeight, "endHeight": endHeight},
		Version: "1.0",
		Time:    time.Now(),
	}

	log.Printf("Requesting blocks %d-%d from peer %s", startHeight, endHeight, peerID)
	return p.sendMessage(peer, message)
}

// GetBlockChannel returns the block channel
func (p *P2P) GetBlockChannel() <-chan *Block {
	return p.blockChan
}

// GetTransactionChannel returns the transaction channel
func (p *P2P) GetTransactionChannel() <-chan *Transaction {
	return p.txChan
}

// acceptConnections accepts incoming connections
func (p *P2P) acceptConnections() {
	for {
		select {
		case <-p.stopChan:
			return
		default:
			conn, err := p.listener.Accept()
			if err != nil {
				if p.IsRunning() {
					log.Printf("Failed to accept connection: %v", err)
				}
				continue
			}

			// Handle connection in goroutine
			go p.handleConnection(conn)
		}
	}
}

// handleConnection handles a new connection
func (p *P2P) handleConnection(conn net.Conn) {
	defer conn.Close()

	// Extract IP from remote address
	remoteAddr := conn.RemoteAddr().String()
	remoteIP, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		log.Printf("Failed to parse remote address %s: %v", remoteAddr, err)
		return
	}

	// Check if IP is in whitelist (if whitelist is enabled)
	p.mu.RLock()
	allowedIPs := p.allowedIPs
	p.mu.RUnlock()

	// Only check whitelist if it's not empty (empty = allow all)
	if len(allowedIPs) > 0 {
		if !allowedIPs[remoteIP] {
			log.Printf("Rejected incoming connection from non-whitelisted IP: %s", remoteIP)
			return
		}
	}

	// Create peer
	peer := &Peer{
		ID:        remoteAddr,
		Address:   remoteAddr,
		Conn:      conn,
		LastSeen:  time.Now(),
		Connected: true,
	}

	// Add peer
	p.addPeer(peer)
	defer p.removePeer(peer.ID)

	// Handle peer communication
	p.handlePeerCommunication(peer)
}

// handlePeerCommunication handles communication with a peer
func (p *P2P) handlePeerCommunication(peer *Peer) {
	reader := bufio.NewReader(peer.Conn)

	for {
		select {
		case <-p.stopChan:
			return
		default:
			// Set read timeout
			peer.Conn.SetReadDeadline(time.Now().Add(p.config.ReadTimeout))

			// Read message
			line, err := reader.ReadBytes('\n')
			if err != nil {
				if err != io.EOF {
					log.Printf("Failed to read from peer %s: %v", peer.ID, err)
				}
				return
			}

			// Parse message
			var message Message
			if err := json.Unmarshal(line, &message); err != nil {
				log.Printf("Failed to parse message from peer %s: %v", peer.ID, err)
				continue
			}

			// Handle message
			p.handleMessage(peer, &message)

			// Update last seen
			peer.mu.Lock()
			peer.LastSeen = time.Now()
			peer.mu.Unlock()
		}
	}
}

// handleMessage handles a received message
func (p *P2P) handleMessage(peer *Peer, message *Message) {
	switch message.Type {
	case "version":
		p.handleVersionMessage(peer, message)
	case "block":
		p.handleBlockMessage(peer, message)
	case "transaction":
		p.handleTransactionMessage(peer, message)
	case "get_blocks":
		p.handleGetBlocksMessage(peer, message)
	case "blocks":
		p.handleBlocksMessage(peer, message)
	case "ping":
		p.handlePingMessage(peer, message)
	case "pong":
		p.handlePongMessage(peer, message)
	default:
		log.Printf("Unknown message type from peer %s: %s", peer.ID, message.Type)
	}
}

// handleVersionMessage handles a version message
func (p *P2P) handleVersionMessage(peer *Peer, message *Message) {
	// Update peer version info
	peer.mu.Lock()
	peer.Version = message.Version
	peer.mu.Unlock()

	// Send version response
	response := &Message{
		Type:    "version",
		Data:    map[string]interface{}{"version": "1.0"},
		Version: "1.0",
		Time:    time.Now(),
	}

	p.sendMessage(peer, response)
}

// handleBlockMessage handles a block message
func (p *P2P) handleBlockMessage(peer *Peer, message *Message) {
	// Parse block data
	blockData, err := json.Marshal(message.Data)
	if err != nil {
		log.Printf("Failed to marshal block data: %v", err)
		return
	}

	var block Block
	if err := json.Unmarshal(blockData, &block); err != nil {
		log.Printf("Failed to unmarshal block: %v", err)
		return
	}

	// Call handler if set
	p.mu.RLock()
	handler := p.onBlockReceived
	p.mu.RUnlock()

	if handler != nil {
		if err := handler(&block); err != nil {
			log.Printf("Block handler error: %v", err)
		}
	}

	// Forward to block channel
	select {
	case p.blockChan <- &block:
	default:
		log.Println("Block channel full, dropping block")
	}
}

// handleTransactionMessage handles a transaction message
func (p *P2P) handleTransactionMessage(peer *Peer, message *Message) {
	// Parse transaction data
	txData, err := json.Marshal(message.Data)
	if err != nil {
		log.Printf("Failed to marshal transaction data: %v", err)
		return
	}

	var tx Transaction
	if err := json.Unmarshal(txData, &tx); err != nil {
		log.Printf("Failed to unmarshal transaction: %v", err)
		return
	}

	// Call handler if set
	p.mu.RLock()
	handler := p.onTransactionReceived
	p.mu.RUnlock()

	if handler != nil {
		if err := handler(&tx); err != nil {
			log.Printf("Transaction handler error: %v", err)
		}
	}

	// Forward to transaction channel
	select {
	case p.txChan <- &tx:
	default:
		log.Println("Transaction channel full, dropping transaction")
	}
}

// handleGetBlocksMessage handles a get blocks message
func (p *P2P) handleGetBlocksMessage(peer *Peer, message *Message) {
	// Parse request data
	var startHeight, endHeight uint64
	if data, ok := message.Data.(map[string]interface{}); ok {
		if start, ok := data["startHeight"].(float64); ok {
			startHeight = uint64(start)
		}
		if end, ok := data["endHeight"].(float64); ok {
			endHeight = uint64(end)
		}
	}

	log.Printf("Received get_blocks request from peer %s: %d-%d", peer.ID, startHeight, endHeight)

	// Call handler if set
	p.mu.RLock()
	handler := p.onGetBlocksRequest
	p.mu.RUnlock()

	if handler == nil {
		log.Printf("No get_blocks handler set")
		return
	}

	// Get blocks from blockchain
	blocks, err := handler(startHeight, endHeight)
	if err != nil {
		log.Printf("Failed to get blocks: %v", err)
		return
	}

	// Send blocks response
	response := &Message{
		Type:    "blocks",
		Data:    blocks,
		Version: "1.0",
		Time:    time.Now(),
	}

	if err := p.sendMessage(peer, response); err != nil {
		log.Printf("Failed to send blocks to peer %s: %v", peer.ID, err)
	} else {
		log.Printf("Sent %d blocks to peer %s", len(blocks), peer.ID)
	}
}

// handleBlocksMessage handles a blocks message
func (p *P2P) handleBlocksMessage(peer *Peer, message *Message) {
	// Parse blocks data
	blocksData, err := json.Marshal(message.Data)
	if err != nil {
		log.Printf("Failed to marshal blocks data: %v", err)
		return
	}

	var blocks []Block
	if err := json.Unmarshal(blocksData, &blocks); err != nil {
		log.Printf("Failed to unmarshal blocks: %v", err)
		return
	}

	log.Printf("Received %d blocks from peer %s", len(blocks), peer.ID)

	// Process each block sequentially (important for blockchain integrity)
	p.mu.RLock()
	handler := p.onBlockReceived
	p.mu.RUnlock()

	if handler != nil {
		successCount := 0
		errorCount := 0
		for i := range blocks {
			if err := handler(&blocks[i]); err != nil {
				// Log error but continue processing other blocks
				log.Printf("Failed to process block %d (height %d) from peer: %v", i, blocks[i].Header.Number, err)
				errorCount++
				// If parent block not found, we need to request earlier blocks first
				if strings.Contains(err.Error(), "parent") || strings.Contains(err.Error(), "Parent") {
					log.Printf("Parent block missing for block %d - may need to sync from earlier height", blocks[i].Header.Number)
				}
			} else {
				successCount++
			}
		}
		log.Printf("Processed %d blocks from peer %s: %d successful, %d failed", len(blocks), peer.ID, successCount, errorCount)
	}
}

// handlePingMessage handles a ping message
func (p *P2P) handlePingMessage(peer *Peer, message *Message) {
	// Send pong response
	response := &Message{
		Type:    "pong",
		Data:    message.Data,
		Version: "1.0",
		Time:    time.Now(),
	}

	p.sendMessage(peer, response)
}

// handlePongMessage handles a pong message
func (p *P2P) handlePongMessage(peer *Peer, message *Message) {
	// Update peer last seen
	peer.mu.Lock()
	peer.LastSeen = time.Now()
	peer.mu.Unlock()
}

// sendMessage sends a message to a peer
func (p *P2P) sendMessage(peer *Peer, message *Message) error {
	// Set write timeout
	peer.Conn.SetWriteDeadline(time.Now().Add(p.config.WriteTimeout))

	// Marshal message
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %v", err)
	}

	// Send message
	_, err = peer.Conn.Write(append(data, '\n'))
	if err != nil {
		return fmt.Errorf("failed to send message: %v", err)
	}

	return nil
}

// broadcastMessage broadcasts a message to all peers
func (p *P2P) broadcastMessage(message *Message) error {
	p.peerMutex.RLock()
	defer p.peerMutex.RUnlock()

	var errors []error

	for _, peer := range p.peers {
		if err := p.sendMessage(peer, message); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to send to some peers: %v", errors)
	}

	return nil
}

// addPeer adds a peer to the peer list
func (p *P2P) addPeer(peer *Peer) {
	p.peerMutex.Lock()
	defer p.peerMutex.Unlock()

	p.peers[peer.ID] = peer
	log.Printf("Peer connected: %s", peer.ID)
}

// removePeer removes a peer from the peer list
func (p *P2P) removePeer(peerID string) {
	p.peerMutex.Lock()
	defer p.peerMutex.Unlock()

	if peer, exists := p.peers[peerID]; exists {
		peer.Connected = false
		delete(p.peers, peerID)
		log.Printf("Peer disconnected: %s", peerID)
	}
}

// discoverPeers discovers new peers from seed nodes
func (p *P2P) discoverPeers() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopChan:
			return
		case <-ticker.C:
			p.connectToSeedNodes()
		}
	}
}

// connectToSeedNodes connects to seed nodes
func (p *P2P) connectToSeedNodes() {
	p.peerMutex.RLock()
	currentPeerCount := len(p.peers)
	p.peerMutex.RUnlock()

	if currentPeerCount >= p.config.MaxPeers {
		return
	}

	if len(p.config.SeedNodes) == 0 {
		return // No seed nodes configured
	}

	for _, seedNode := range p.config.SeedNodes {
		if currentPeerCount >= p.config.MaxPeers {
			break
		}

		// Check if already connected
		p.peerMutex.RLock()
		alreadyConnected := false
		for _, peer := range p.peers {
			if peer.Address == seedNode {
				alreadyConnected = true
				break
			}
		}
		p.peerMutex.RUnlock()

		if alreadyConnected {
			continue
		}

		// Connect to seed node
		log.Printf("Attempting to connect to seed node: %s", seedNode)
		go p.connectToPeer(seedNode)
	}
}

// connectToPeer connects to a specific peer
func (p *P2P) connectToPeer(address string) {
	conn, err := net.DialTimeout("tcp", address, p.config.DialTimeout)
	if err != nil {
		log.Printf("Failed to connect to peer %s: %v", address, err)
		return
	}

	// Create peer
	peer := &Peer{
		ID:        address,
		Address:   address,
		Conn:      conn,
		LastSeen:  time.Now(),
		Connected: true,
	}

	// Add peer
	p.addPeer(peer)
	defer p.removePeer(peer.ID)

	// Handle peer communication
	p.handlePeerCommunication(peer)
}

// maintainPeers maintains peer connections
func (p *P2P) maintainPeers() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopChan:
			return
		case <-ticker.C:
			p.cleanupInactivePeers()
		}
	}
}

// cleanupInactivePeers removes inactive peers
func (p *P2P) cleanupInactivePeers() {
	p.peerMutex.Lock()
	defer p.peerMutex.Unlock()

	now := time.Now()
	for id, peer := range p.peers {
		if now.Sub(peer.LastSeen) > p.config.KeepAlive*2 {
			peer.Connected = false
			if peer.Conn != nil {
				peer.Conn.Close()
			}
			delete(p.peers, id)
			log.Printf("Removed inactive peer: %s", id)
		}
	}
}

// GetNetworkInfo returns network information
func (p *P2P) GetNetworkInfo() map[string]interface{} {
	p.peerMutex.RLock()
	defer p.peerMutex.RUnlock()

	peers := make([]map[string]interface{}, 0, len(p.peers))
	for _, peer := range p.peers {
		peer.mu.RLock()
		peerInfo := map[string]interface{}{
			"id":        peer.ID,
			"address":   peer.Address,
			"connected": peer.Connected,
			"version":   peer.Version,
			"height":    peer.Height,
			"lastSeen":  peer.LastSeen,
		}
		peer.mu.RUnlock()
		peers = append(peers, peerInfo)
	}

	return map[string]interface{}{
		"running":    p.IsRunning(),
		"listenAddr": p.config.ListenAddr,
		"peerCount":  len(p.peers),
		"maxPeers":   p.config.MaxPeers,
		"peers":      peers,
	}
}
