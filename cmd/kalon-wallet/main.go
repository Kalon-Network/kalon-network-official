package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/kalon-network/kalon/crypto"
)

var version = "1.0.2"

// WalletManager handles wallet operations
type WalletManager struct {
	wallet *crypto.Wallet
	rpcURL string
	client *http.Client
}

// RPCRequest represents an RPC request
type RPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	ID      int         `json:"id"`
}

// RPCResponse represents an RPC response
type RPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result"`
	Error   *RPCError   `json:"error,omitempty"`
	ID      int         `json:"id"`
}

// RPCError represents an RPC error
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// TransactionRequest represents a transaction request
type TransactionRequest struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Amount uint64 `json:"amount"`
	Fee    uint64 `json:"fee"`
	Data   string `json:"data,omitempty"`
}

// BalanceResponse represents a balance response
type BalanceResponse struct {
	Address string `json:"address"`
	Balance uint64 `json:"balance"`
}

// TransactionResponse represents a transaction response
type TransactionResponse struct {
	Hash    string `json:"hash"`
	From    string `json:"from"`
	To      string `json:"to"`
	Amount  uint64 `json:"amount"`
	Fee     uint64 `json:"fee"`
	Nonce   uint64 `json:"nonce"`
	Success bool   `json:"success"`
}

// WalletInfo represents wallet information
type WalletInfo struct {
	Address    string `json:"address"`
	PublicKey  string `json:"publicKey"`
	PrivateKey string `json:"privateKey,omitempty"`
	Mnemonic   string `json:"mnemonic,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	command := os.Args[1]
	args := os.Args[2:]

	walletManager := &WalletManager{}

	switch command {
	case "create":
		handleCreate(walletManager, args)
	case "import":
		handleImport(walletManager, args)
	case "list":
		handleList(args)
	case "export":
		handleExport(walletManager, args)
	case "balance":
		handleBalance(walletManager, args)
	case "send":
		handleSend(walletManager, args)
	case "info":
		handleInfo(walletManager, args)
	case "help":
		usage()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		usage()
		os.Exit(1)
	}
}

// handleCreate handles wallet creation
func handleCreate(wm *WalletManager, args []string) {
	fs := flag.NewFlagSet("create", flag.ExitOnError)
	passphrase := fs.String("passphrase", "", "Passphrase for wallet encryption")
	name := fs.String("name", "", "Wallet name (will be saved as wallet-{name}.json)")
	output := fs.String("output", "", "Output file for wallet (overrides name)")
	fs.Parse(args)

	reader := bufio.NewReader(os.Stdin)

	// If no custom output specified, ask for name
	if *output == "" && *name == "" {
		fmt.Print("Enter wallet name (leave empty for 'wallet.json'): ")
		nameInput, err := reader.ReadString('\n')
		if err != nil {
			log.Fatalf("Failed to read wallet name: %v", err)
		}
		nameInput = strings.TrimSpace(nameInput)

		if nameInput != "" {
			*name = nameInput
			*output = fmt.Sprintf("wallet-%s.json", *name)
		} else {
			*output = "wallet.json"
		}
	} else if *output == "" && *name != "" {
		*output = fmt.Sprintf("wallet-%s.json", *name)
	} else if *output == "" {
		*output = "wallet.json"
	}

	// Get passphrase if not provided
	if *passphrase == "" {
		fmt.Print("Enter passphrase (optional): ")
		pass, err := reader.ReadString('\n')
		if err != nil {
			log.Fatalf("Failed to read passphrase: %v", err)
		}
		*passphrase = strings.TrimSpace(pass)
	}

	// Create wallet
	wallet, err := crypto.NewWallet(*passphrase)
	if err != nil {
		log.Fatalf("Failed to create wallet: %v", err)
	}

	wm.wallet = wallet

	// Get address
	address, err := wallet.GetAddressString()
	if err != nil {
		log.Fatalf("Failed to get address: %v", err)
	}

	// Create wallet info
	walletInfo := &WalletInfo{
		Address:   address,
		PublicKey: wallet.Keypair.GetPublicHex(),
		Mnemonic:  wallet.Mnemonic,
	}

	// Check if file already exists
	if _, err := os.Stat(*output); err == nil {
		log.Fatalf("Wallet file already exists: %s. Use --name to create a different wallet.", *output)
	}

	// Save wallet
	if err := saveWallet(walletInfo, *output); err != nil {
		log.Fatalf("Failed to save wallet: %v", err)
	}

	fmt.Printf("Wallet created successfully!\n")
	fmt.Printf("Address: %s\n", address)
	fmt.Printf("Public Key: %s\n", wallet.Keypair.GetPublicHex())
	fmt.Printf("Mnemonic: %s\n", wallet.Mnemonic)
	fmt.Printf("Wallet saved to: %s\n", *output)
	fmt.Println("\n⚠️  IMPORTANT: Save your mnemonic phrase in a safe place!")
	fmt.Println("   You will need it to recover your wallet if you lose access.")
}

// handleImport handles wallet import
func handleImport(wm *WalletManager, args []string) {
	fs := flag.NewFlagSet("import", flag.ExitOnError)
	mnemonic := fs.String("mnemonic", "", "Mnemonic phrase to import")
	passphrase := fs.String("passphrase", "", "Passphrase for wallet encryption")
	output := fs.String("output", "wallet.json", "Output file for wallet")
	fs.Parse(args)

	// Get mnemonic if not provided
	if *mnemonic == "" {
		fmt.Print("Enter mnemonic phrase: ")
		reader := bufio.NewReader(os.Stdin)
		mnemonicInput, err := reader.ReadString('\n')
		if err != nil {
			log.Fatalf("Failed to read mnemonic: %v", err)
		}
		*mnemonic = strings.TrimSpace(mnemonicInput)
	}

	// Get passphrase if not provided
	if *passphrase == "" {
		fmt.Print("Enter passphrase (optional): ")
		reader := bufio.NewReader(os.Stdin)
		pass, err := reader.ReadString('\n')
		if err != nil {
			log.Fatalf("Failed to read passphrase: %v", err)
		}
		*passphrase = strings.TrimSpace(pass)
	}

	// Create BIP39 manager
	bm := crypto.NewBIP39Manager()

	// Import wallet from mnemonic
	wallet, err := bm.CreateWalletFromMnemonic(*mnemonic, *passphrase)
	if err != nil {
		log.Fatalf("Failed to import wallet: %v", err)
	}

	wm.wallet = wallet

	// Get address
	address, err := wallet.GetAddressString()
	if err != nil {
		log.Fatalf("Failed to get address: %v", err)
	}

	// Create wallet info
	walletInfo := &WalletInfo{
		Address:   address,
		PublicKey: wallet.Keypair.GetPublicHex(),
		Mnemonic:  wallet.Mnemonic,
	}

	// Save wallet
	if err := saveWallet(walletInfo, *output); err != nil {
		log.Fatalf("Failed to save wallet: %v", err)
	}

	fmt.Printf("Wallet imported successfully!\n")
	fmt.Printf("Address: %s\n", address)
	fmt.Printf("Public Key: %s\n", wallet.Keypair.GetPublicHex())
	fmt.Printf("Wallet saved to: %s\n", *output)
}

// handleExport handles wallet export
func handleExport(wm *WalletManager, args []string) {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	input := fs.String("input", "wallet.json", "Input wallet file")
	showPrivate := fs.Bool("private", false, "Show private key")
	fs.Parse(args)

	// Load wallet
	walletInfo, err := loadWallet(*input)
	if err != nil {
		log.Fatalf("Failed to load wallet: %v", err)
	}

	// Create export data
	exportData := map[string]interface{}{
		"address":   walletInfo.Address,
		"publicKey": walletInfo.PublicKey,
		"mnemonic":  walletInfo.Mnemonic,
	}

	if *showPrivate {
		exportData["privateKey"] = walletInfo.PrivateKey
	}

	// Export as JSON
	jsonData, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal wallet data: %v", err)
	}

	fmt.Println(string(jsonData))
}

// handleBalance handles balance queries
func handleBalance(wm *WalletManager, args []string) {
	fs := flag.NewFlagSet("balance", flag.ExitOnError)
	address := fs.String("address", "", "Address to check balance")
	rpcURL := fs.String("rpc", "http://localhost:16314", "RPC server URL")
	fs.Parse(args)

	// Get address
	var targetAddress string
	if *address != "" {
		targetAddress = *address
	} else if wm.wallet != nil {
		addr, err := wm.wallet.GetAddressString()
		if err != nil {
			log.Fatalf("Failed to get wallet address: %v", err)
		}
		targetAddress = addr
	} else {
		log.Fatal("No address provided and no wallet loaded")
	}

	// Query balance via RPC
	balanceMicro, err := queryBalance(*rpcURL, targetAddress)
	if err != nil {
		log.Fatalf("Failed to query balance: %v", err)
	}

	// Convert to tKALON for display
	balanceTKALON := float64(balanceMicro) / 1000000.0

	// Create response (keep micro-KALON in JSON for API compatibility)
	response := &BalanceResponse{
		Address: targetAddress,
		Balance: balanceMicro,
	}

	// Output result with user-friendly format
	fmt.Printf("Address: %s\n", targetAddress)
	fmt.Printf("Balance: %.2f tKALON (%d micro-KALON)\n", balanceTKALON, balanceMicro)
	fmt.Println()
	fmt.Println("JSON Format:")
	jsonData, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal balance: %v", err)
	}
	fmt.Println(string(jsonData))
}

// handleSend handles transaction sending
func handleSend(wm *WalletManager, args []string) {
	fs := flag.NewFlagSet("send", flag.ExitOnError)
	to := fs.String("to", "", "Recipient address")
	amount := fs.Uint64("amount", 0, "Amount to send")
	fee := fs.Uint64("fee", 1000000, "Transaction fee (micro-KALON)")
	rpcURL := fs.String("rpc", "http://localhost:16314", "RPC server URL")
	input := fs.String("input", "", "Input wallet file (default: wallet.json or wallet-wallet1.json)")
	fs.Parse(args)

	if *to == "" || *amount == 0 {
		log.Fatal("Recipient address and amount are required")
	}

	// Load wallet if not already loaded
	var wallet *crypto.Wallet
	if wm.wallet != nil {
		wallet = wm.wallet
	} else {
		// Try to find wallet file
		walletFile := *input
		if walletFile == "" {
			// Try common wallet file names
			if _, err := os.Stat("wallet-wallet1.json"); err == nil {
				walletFile = "wallet-wallet1.json"
			} else if _, err := os.Stat("wallet.json"); err == nil {
				walletFile = "wallet.json"
			} else {
				log.Fatal("No wallet loaded and no wallet file found. Use 'create' or 'import' first, or specify -input")
			}
		}

		// Load wallet from file
		walletInfo, err := loadWallet(walletFile)
		if err != nil {
			log.Fatalf("Failed to load wallet: %v", err)
		}

		// Create wallet from mnemonic
		bm := crypto.NewBIP39Manager()
		wallet, err = bm.CreateWalletFromMnemonic(walletInfo.Mnemonic, "")
		if err != nil {
			log.Fatalf("Failed to create wallet from mnemonic: %v", err)
		}
	}

	// Get wallet address
	fromAddress, err := wallet.GetAddressString()
	if err != nil {
		log.Fatalf("Failed to get wallet address: %v", err)
	}

	// Create transaction request
	txReq := &TransactionRequest{
		From:   fromAddress,
		To:     *to,
		Amount: *amount,
		Fee:    *fee,
	}

	// Send transaction
	txResp, err := sendTransaction(*rpcURL, txReq, wallet)
	if err != nil {
		log.Fatalf("Failed to send transaction: %v", err)
	}

	// Convert amounts to tKALON for display
	amountTKALON := float64(txResp.Amount) / 1000000.0
	feeTKALON := float64(txResp.Fee) / 1000000.0

	// Output result with user-friendly format
	fmt.Printf("✅ Transaction sent successfully!\n")
	fmt.Printf("Hash: %s\n", txResp.Hash)
	fmt.Printf("From: %s\n", txResp.From)
	fmt.Printf("To: %s\n", txResp.To)
	fmt.Printf("Amount: %.2f tKALON (%d micro-KALON)\n", amountTKALON, txResp.Amount)
	fmt.Printf("Fee: %.2f tKALON (%d micro-KALON)\n", feeTKALON, txResp.Fee)
	fmt.Printf("Nonce: %d\n", txResp.Nonce)
	fmt.Printf("Success: %v\n", txResp.Success)
	fmt.Println()
	fmt.Println("JSON Format:")
	jsonData, err := json.MarshalIndent(txResp, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal transaction: %v", err)
	}
	fmt.Println(string(jsonData))
}

// handleInfo handles wallet info display
func handleInfo(wm *WalletManager, args []string) {
	fs := flag.NewFlagSet("info", flag.ExitOnError)
	input := fs.String("input", "wallet.json", "Input wallet file")
	fs.Parse(args)

	// Load wallet
	walletInfo, err := loadWallet(*input)
	if err != nil {
		log.Fatalf("Failed to load wallet: %v", err)
	}

	// Output wallet info
	jsonData, err := json.MarshalIndent(walletInfo, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal wallet info: %v", err)
	}

	fmt.Println(string(jsonData))
}

// saveWallet saves wallet to file
func saveWallet(walletInfo *WalletInfo, filename string) error {
	data, err := json.MarshalIndent(walletInfo, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0600)
}

// loadWallet loads wallet from file
func loadWallet(filename string) (*WalletInfo, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var walletInfo WalletInfo
	if err := json.Unmarshal(data, &walletInfo); err != nil {
		return nil, err
	}

	return &walletInfo, nil
}

// queryBalance queries balance via RPC
func queryBalance(rpcURL, address string) (uint64, error) {
	// Create RPC request
	req := RPCRequest{
		JSONRPC: "2.0",
		Method:  "getBalance",
		Params: map[string]string{
			"address": address,
		},
		ID: 1,
	}

	// Marshal request
	reqData, err := json.Marshal(req)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal request: %v", err)
	}

	// Make HTTP request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(rpcURL, "application/json", bytes.NewBuffer(reqData))
	if err != nil {
		return 0, fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read response: %v", err)
	}

	// Parse response
	var rpcResp RPCResponse
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return 0, fmt.Errorf("failed to parse response: %v", err)
	}

	// Check for RPC error
	if rpcResp.Error != nil {
		return 0, fmt.Errorf("RPC error: %s", rpcResp.Error.Message)
	}

	// Extract balance from result
	balance, ok := rpcResp.Result.(float64)
	if !ok {
		return 0, fmt.Errorf("invalid balance format in response")
	}

	return uint64(balance), nil
}

// handleList handles wallet listing
func handleList(args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	fs.Parse(args)

	fmt.Println("Available wallets:")

	// Find all wallet files
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}

	files, err := os.ReadDir(wd)
	if err != nil {
		log.Fatalf("Failed to read directory: %v", err)
	}

	found := false
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		if strings.HasPrefix(file.Name(), "wallet") && strings.HasSuffix(file.Name(), ".json") {
			found = true
			walletInfo, err := loadWallet(file.Name())
			if err != nil {
				fmt.Printf("  ⚠️  %s (corrupted)\n", file.Name())
				continue
			}

			fmt.Printf("  📄 %s\n", file.Name())
			fmt.Printf("     Address: %s\n", walletInfo.Address)
			if walletInfo.PublicKey != "" {
				fmt.Printf("     Public Key: %s\n", walletInfo.PublicKey)
			}
			fmt.Println()
		}
	}

	if !found {
		fmt.Println("  No wallets found. Use 'kalon-wallet create' to create one.")
	}
}

// sendTransaction sends a transaction via RPC
func sendTransaction(rpcURL string, txReq *TransactionRequest, wallet *crypto.Wallet) (*TransactionResponse, error) {
	// Ensure RPC URL has /rpc endpoint
	if !strings.HasSuffix(rpcURL, "/rpc") {
		if !strings.HasSuffix(rpcURL, "/") {
			rpcURL += "/rpc"
		} else {
			rpcURL += "rpc"
		}
	}

	// Create RPC request
	req := RPCRequest{
		JSONRPC: "2.0",
		Method:  "sendTransaction",
		Params: map[string]interface{}{
			"from":   txReq.From,
			"to":     txReq.To,
			"amount": txReq.Amount,
			"fee":    txReq.Fee,
		},
		ID: 1,
	}

	// Marshal request
	reqData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	// Make HTTP request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(rpcURL, "application/json", bytes.NewBuffer(reqData))
	if err != nil {
		return nil, fmt.Errorf("failed to send transaction: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	// Parse response
	var rpcResp RPCResponse
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	// Check for RPC error
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("RPC error: %s", rpcResp.Error.Message)
	}

	// Extract transaction hash from result
	result, ok := rpcResp.Result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}

	txHash, _ := result["txHash"].(string)

	return &TransactionResponse{
		Hash:    txHash,
		From:    txReq.From,
		To:      txReq.To,
		Amount:  txReq.Amount,
		Fee:     txReq.Fee,
		Nonce:   1,
		Success: true,
	}, nil
}

// usage displays usage information
func usage() {
	fmt.Printf("Kalon Wallet CLI v%s\n", version)
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  kalon-wallet <command> [flags]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  create     Create a new wallet")
	fmt.Println("  import     Import wallet from mnemonic")
	fmt.Println("  list       List all available wallets")
	fmt.Println("  export     Export wallet information")
	fmt.Println("  balance    Check wallet balance")
	fmt.Println("  send       Send transaction")
	fmt.Println("  info       Show wallet information")
	fmt.Println("  help       Show this help message")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  kalon-wallet create --name miner")
	fmt.Println("  kalon-wallet create --name test1")
	fmt.Println("  kalon-wallet list")
	fmt.Println("  kalon-wallet import --mnemonic 'word1 word2 ...' --name backup")
	fmt.Println("  kalon-wallet balance --address kalon1abc...")
	fmt.Println("  kalon-wallet send --to kalon1def... --amount 1000000")
	fmt.Println("  kalon-wallet info --input wallet-test.json")
}
