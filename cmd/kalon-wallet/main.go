package main

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kalon-network/kalon/core"
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
	case "deploy-token":
		handleDeployToken(walletManager, args)
	case "send-token":
		handleSendToken(walletManager, args)
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
	rpcURL := fs.String("rpc", defaultRPC, "RPC server URL")
	fs.Parse(args)

	// Ensure RPC URL has /rpc endpoint
	rpcURLStr := *rpcURL
	if !strings.HasSuffix(rpcURLStr, "/rpc") {
		if !strings.HasSuffix(rpcURLStr, "/") {
			rpcURLStr += "/rpc"
		} else {
			rpcURLStr += "rpc"
		}
	}

	// Get address - interactive if not provided
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
		// Interactive wallet selection
		walletFile, wallet, err := selectWallet("Select wallet to check balance:")
		if err != nil {
			log.Fatal(err)
		}
		targetAddress, _ = wallet.GetAddressString()
		fmt.Printf("Selected: %s (%s)\n\n", walletFile, targetAddress)
	}

	// Query balance and token balances via getAddressInfo RPC
	client := &http.Client{Timeout: 30 * time.Second}
	req := RPCRequest{
		JSONRPC: "2.0",
		Method:  "getAddressInfo",
		Params: map[string]interface{}{
			"address": targetAddress,
		},
		ID: 1,
	}

	reqData, err := json.Marshal(req)
	if err != nil {
		log.Fatalf("Failed to marshal request: %v", err)
	}

	resp, err := client.Post(rpcURLStr, "application/json", bytes.NewBuffer(reqData))
	if err != nil {
		log.Fatalf("Failed to query balance: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read response: %v", err)
	}

	var rpcResp RPCResponse
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		log.Fatalf("Failed to parse response: %v", err)
	}

	if rpcResp.Error != nil {
		log.Fatalf("RPC error: %s", rpcResp.Error.Message)
	}

	// Parse response
	result, ok := rpcResp.Result.(map[string]interface{})
	if !ok {
		log.Fatalf("Invalid response format")
	}

	// Get KALON balance
	balanceMicro := uint64(0)
	if balance, ok := result["balance"].(float64); ok {
		balanceMicro = uint64(balance)
	}
	balanceTKALON := float64(balanceMicro) / 1000000.0

	// Get token balances
	tokenBalances := make(map[string]uint64)
	if tokenBalancesMap, ok := result["tokenBalances"].(map[string]interface{}); ok {
		for tokenName, balance := range tokenBalancesMap {
			if balanceFloat, ok := balance.(float64); ok {
				tokenBalances[tokenName] = uint64(balanceFloat)
			}
		}
	}

	// Output result
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Printf("💰 WALLET BALANCE\n")
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Printf("Address: %s\n", targetAddress)
	fmt.Printf("KALON Balance: %.2f tKALON (%d micro-KALON)\n", balanceTKALON, balanceMicro)
	fmt.Println()

	// Show token balances if any
	if len(tokenBalances) > 0 {
		fmt.Println("🪙 TOKEN BALANCES:")
		for tokenName, balance := range tokenBalances {
			fmt.Printf("  %s: %s\n", tokenName, formatLargeNumber(balance))
		}
		fmt.Println()
	} else {
		fmt.Println("🪙 TOKEN BALANCES: None")
		fmt.Println()
	}

	// Show transaction stats if available
	if txCount, ok := result["transactionCount"].(float64); ok {
		fmt.Printf("📊 Transaction Count: %.0f\n", txCount)
	}
	if sentCount, ok := result["sentCount"].(float64); ok {
		fmt.Printf("📤 Sent Transactions: %.0f\n", sentCount)
	}
	if receivedCount, ok := result["receivedCount"].(float64); ok {
		fmt.Printf("📥 Received Transactions: %.0f\n", receivedCount)
	}
	fmt.Println("═══════════════════════════════════════════════════════════")
}

// formatLargeNumber formats large numbers with commas
func formatLargeNumber(n uint64) string {
	str := fmt.Sprintf("%d", n)
	if len(str) <= 3 {
		return str
	}
	var result strings.Builder
	for i, r := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			result.WriteString(",")
		}
		result.WriteRune(r)
	}
	return result.String()
}

// getAvailableWallets returns list of available wallet files
func getAvailableWallets() []string {
	wd, err := os.Getwd()
	if err != nil {
		return []string{}
	}

	files, err := os.ReadDir(wd)
	if err != nil {
		return []string{}
	}

	var wallets []string
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if strings.HasPrefix(file.Name(), "wallet") && strings.HasSuffix(file.Name(), ".json") {
			wallets = append(wallets, file.Name())
		}
	}
	return wallets
}

// selectWallet interactively selects a wallet
func selectWallet(prompt string) (string, *crypto.Wallet, error) {
	wallets := getAvailableWallets()
	if len(wallets) == 0 {
		return "", nil, fmt.Errorf("no wallets found. Use 'create' to create one")
	}

	reader := bufio.NewReader(os.Stdin)

	if len(wallets) == 1 {
		walletFile := wallets[0]
		walletInfo, err := loadWallet(walletFile)
		if err != nil {
			return "", nil, err
		}
		bm := crypto.NewBIP39Manager()
		wallet, err := bm.CreateWalletFromMnemonic(walletInfo.Mnemonic, "")
		if err != nil {
			return "", nil, err
		}
		return walletFile, wallet, nil
	}

	fmt.Printf("%s\n", prompt)
	// Generate wallets first to show correct addresses
	bm := crypto.NewBIP39Manager()
	generatedWallets := make([]*crypto.Wallet, len(wallets))
	for i, w := range wallets {
		walletInfo, err := loadWallet(w)
		if err == nil {
			// Generate wallet from mnemonic to get correct address
			genWallet, err := bm.CreateWalletFromMnemonic(walletInfo.Mnemonic, "")
			if err == nil {
				generatedWallets[i] = genWallet
				addr, _ := genWallet.GetAddressString()
				fmt.Printf("  [%d] %s (Address: %s)\n", i+1, w, addr)
			} else {
				fmt.Printf("  [%d] %s (error: %v)\n", i+1, w, err)
			}
		} else {
			fmt.Printf("  [%d] %s (corrupted)\n", i+1, w)
		}
	}
	fmt.Print("Select wallet (number or filename): ")
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	var walletFile string
	var selectedIndex int
	if num, err := strconv.Atoi(input); err == nil && num > 0 && num <= len(wallets) {
		selectedIndex = num - 1
		walletFile = wallets[selectedIndex]
	} else {
		// Check if input is a filename
		for i, w := range wallets {
			if w == input || strings.Contains(w, input) {
				selectedIndex = i
				walletFile = w
				break
			}
		}
		if walletFile == "" {
			return "", nil, fmt.Errorf("invalid selection")
		}
	}

	// Use pre-generated wallet if available, otherwise generate new one
	var wallet *crypto.Wallet
	if selectedIndex < len(generatedWallets) && generatedWallets[selectedIndex] != nil {
		wallet = generatedWallets[selectedIndex]
	} else {
		walletInfo, err := loadWallet(walletFile)
		if err != nil {
			return "", nil, err
		}
		wallet, err = bm.CreateWalletFromMnemonic(walletInfo.Mnemonic, "")
		if err != nil {
			return "", nil, err
		}
	}
	return walletFile, wallet, nil
}

// handleSend handles transaction sending
func handleSend(wm *WalletManager, args []string) {
	fs := flag.NewFlagSet("send", flag.ExitOnError)
	fromFlag := fs.String("from", "", "Sender wallet file or address")
	toFlag := fs.String("to", "", "Recipient address or wallet file")
	amountFlag := fs.Uint64("amount", 0, "Amount to send (micro-KALON)")
	feeFlag := fs.Uint64("fee", 0, "Transaction fee (micro-KALON, default: 100000)")
	rpcURLFlag := fs.String("rpc", "", "RPC server URL (default: https://explorer.kalon-network.com/rpc)")
	fs.Parse(args)

	reader := bufio.NewReader(os.Stdin)
	// Use default RPC if not provided
	rpcURL := defaultRPC
	if *rpcURLFlag != "" {
		rpcURL = *rpcURLFlag
	}

	// Interactive mode if parameters are missing
	var fromWallet *crypto.Wallet
	var fromAddress string
	var toAddress string
	var amount uint64
	var fee uint64

	// Get sender wallet
	if *fromFlag != "" {
		// Try to load as wallet file
		if walletInfo, err := loadWallet(*fromFlag); err == nil {
			bm := crypto.NewBIP39Manager()
			fromWallet, err = bm.CreateWalletFromMnemonic(walletInfo.Mnemonic, "")
			if err != nil {
				log.Fatalf("Failed to create wallet from mnemonic: %v", err)
			}
			fromAddress, _ = fromWallet.GetAddressString()
		} else {
			// Assume it's an address
			fromAddress = *fromFlag
		}
	} else if wm.wallet != nil {
		// Use wallet from wallet manager if available
		fromWallet = wm.wallet
		fromAddress, _ = fromWallet.GetAddressString()
	} else {
		// Interactive selection
		walletFile, wallet, err := selectWallet("Select sender wallet:")
		if err != nil {
			log.Fatal(err)
		}
		fromWallet = wallet
		fromAddress, _ = wallet.GetAddressString()
		fmt.Printf("Selected: %s (%s)\n", walletFile, fromAddress)
	}

	// Get recipient address
	if *toFlag != "" {
		// Check if it's a wallet file
		if walletInfo, err := loadWallet(*toFlag); err == nil {
			toAddress = walletInfo.Address
		} else {
			// Assume it's an address
			toAddress = *toFlag
		}
	} else {
		fmt.Print("Enter recipient address (or wallet filename): ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		// Check if it's a wallet file
		if walletInfo, err := loadWallet(input); err == nil {
			toAddress = walletInfo.Address
			fmt.Printf("Using wallet address: %s\n", toAddress)
		} else {
			toAddress = input
		}
	}

	// Get amount
	if *amountFlag > 0 {
		amount = *amountFlag
	} else {
		fmt.Print("Enter amount (in micro-KALON, e.g., 1000000 for 1 tKALON): ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		parsed, err := strconv.ParseUint(input, 10, 64)
		if err != nil {
			log.Fatalf("Invalid amount: %v", err)
		}
		amount = parsed
	}

	// Get fee
	if *feeFlag > 0 {
		fee = *feeFlag
	} else {
		fmt.Print("Enter fee (in micro-KALON, default: 100000): ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			fee = 100000
		} else {
			parsed, err := strconv.ParseUint(input, 10, 64)
			if err != nil {
				log.Fatalf("Invalid fee: %v", err)
			}
			fee = parsed
		}
	}

	// Validate
	if fromAddress == "" || toAddress == "" || amount == 0 {
		log.Fatal("From address, to address, and amount are required")
	}

	// If we don't have a wallet for signing, we need to load it
	if fromWallet == nil {
		// Try to find wallet by address
		wallets := getAvailableWallets()
		for _, wf := range wallets {
			walletInfo, err := loadWallet(wf)
			if err != nil {
				continue
			}
			if walletInfo.Address == fromAddress {
				bm := crypto.NewBIP39Manager()
				fromWallet, err = bm.CreateWalletFromMnemonic(walletInfo.Mnemonic, "")
				if err != nil {
					continue
				}
				break
			}
		}
		if fromWallet == nil {
			log.Fatal("Cannot find wallet for address. Please specify --from with wallet file")
		}
	}

	// Create transaction request
	txReq := &TransactionRequest{
		From:   fromAddress,
		To:     toAddress,
		Amount: amount,
		Fee:    fee,
	}

	// Send transaction
	txResp, err := sendTransaction(rpcURL, txReq, fromWallet)
	if err != nil {
		log.Fatalf("Failed to send transaction: %v", err)
	}

	// Convert amounts to tKALON for display
	amountTKALON := float64(txResp.Amount) / 1000000.0
	feeTKALON := float64(txResp.Fee) / 1000000.0

	// Output result with user-friendly format
	fmt.Printf("\n✅ Transaction sent successfully!\n")
	fmt.Printf("Hash: %s\n", txResp.Hash)
	fmt.Printf("From: %s\n", txResp.From)
	fmt.Printf("To: %s\n", txResp.To)
	fmt.Printf("Amount: %.2f tKALON (%d micro-KALON)\n", amountTKALON, txResp.Amount)
	fmt.Printf("Fee: %.2f tKALON (%d micro-KALON)\n", feeTKALON, txResp.Fee)
	fmt.Printf("Nonce: %d\n", txResp.Nonce)
	fmt.Printf("Success: %v\n", txResp.Success)
}

// handleSendToken handles token transfer
func handleSendToken(wm *WalletManager, args []string) {
	fs := flag.NewFlagSet("send-token", flag.ExitOnError)
	fromFlag := fs.String("from", "", "Sender wallet file or address")
	toFlag := fs.String("to", "", "Recipient address or wallet file")
	tokenFlag := fs.String("token", "", "Token name to send")
	amountFlag := fs.Uint64("amount", 0, "Amount to send")
	rpcURLFlag := fs.String("rpc", "", "RPC server URL (default: https://explorer.kalon-network.com/rpc)")
	fs.Parse(args)

	reader := bufio.NewReader(os.Stdin)
	// Use default RPC if not provided
	rpcURL := defaultRPC
	if *rpcURLFlag != "" {
		rpcURL = *rpcURLFlag
	}

	// Ensure RPC URL has /rpc endpoint
	if !strings.HasSuffix(rpcURL, "/rpc") {
		if !strings.HasSuffix(rpcURL, "/") {
			rpcURL += "/rpc"
		} else {
			rpcURL += "rpc"
		}
	}

	// Interactive mode if parameters are missing
	var fromWallet *crypto.Wallet
	var fromAddress string

	// Get sender wallet
	if *fromFlag != "" {
		// Try to load as wallet file
		if walletInfo, err := loadWallet(*fromFlag); err == nil {
			bm := crypto.NewBIP39Manager()
			fromWallet, err = bm.CreateWalletFromMnemonic(walletInfo.Mnemonic, "")
			if err != nil {
				log.Fatalf("Failed to create wallet from mnemonic: %v", err)
			}
			fromAddress, _ = fromWallet.GetAddressString()
		} else {
			// Assume it's an address
			fromAddress = *fromFlag
		}
	} else if wm.wallet != nil {
		// Use wallet from wallet manager if available
		fromWallet = wm.wallet
		fromAddress, _ = fromWallet.GetAddressString()
	} else {
		// Interactive selection
		walletFile, wallet, err := selectWallet("Select sender wallet:")
		if err != nil {
			log.Fatal(err)
		}
		fromWallet = wallet
		fromAddress, _ = wallet.GetAddressString()
		fmt.Printf("Selected: %s (%s)\n\n", walletFile, fromAddress)
	}

	// Get token balances for this address
	client := &http.Client{Timeout: 30 * time.Second}
	req := RPCRequest{
		JSONRPC: "2.0",
		Method:  "getAddressInfo",
		Params: map[string]interface{}{
			"address": fromAddress,
		},
		ID: 1,
	}

	reqData, err := json.Marshal(req)
	if err != nil {
		log.Fatalf("Failed to marshal request: %v", err)
	}

	resp, err := client.Post(rpcURL, "application/json", bytes.NewBuffer(reqData))
	if err != nil {
		log.Fatalf("Failed to query address info: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read response: %v", err)
	}

	var rpcResp RPCResponse
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		log.Fatalf("Failed to parse response: %v", err)
	}

	if rpcResp.Error != nil {
		log.Fatalf("RPC error: %s", rpcResp.Error.Message)
	}

	// Parse response
	result, ok := rpcResp.Result.(map[string]interface{})
	if !ok {
		log.Fatalf("Invalid response format")
	}

	// Get token balances
	tokenBalances, ok := result["tokenBalances"].(map[string]interface{})
	if !ok {
		tokenBalances = make(map[string]interface{})
	}

	// Filter tokens with balance > 0
	availableTokens := make([]string, 0)
	tokenBalanceMap := make(map[string]uint64)
	for tokenName, balanceInterface := range tokenBalances {
		var balance uint64
		switch v := balanceInterface.(type) {
		case float64:
			balance = uint64(v)
		case uint64:
			balance = v
		default:
			continue
		}
		if balance > 0 {
			availableTokens = append(availableTokens, tokenName)
			tokenBalanceMap[tokenName] = balance
		}
	}

	if len(availableTokens) == 0 {
		log.Fatal("❌ No tokens with balance found in this wallet")
	}

	// Display available tokens
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println("🪙 AVAILABLE TOKENS")
	fmt.Println("═══════════════════════════════════════════════════════")
	for i, tokenName := range availableTokens {
		balance := tokenBalanceMap[tokenName]
		fmt.Printf("  [%d] %s: %s\n", i+1, tokenName, formatTokenAmount(balance))
	}
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println()

	// Get token selection
	var selectedToken string
	if *tokenFlag != "" {
		selectedToken = *tokenFlag
		// Validate token exists and has balance
		if _, exists := tokenBalanceMap[selectedToken]; !exists {
			log.Fatalf("Token '%s' not found or has no balance", selectedToken)
		}
	} else {
		fmt.Printf("Select token (1-%d): ", len(availableTokens))
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		index, err := strconv.Atoi(input)
		if err != nil || index < 1 || index > len(availableTokens) {
			log.Fatalf("Invalid selection: %s", input)
		}
		selectedToken = availableTokens[index-1]
	}

	selectedBalance := tokenBalanceMap[selectedToken]
	fmt.Printf("Selected token: %s (Balance: %s)\n\n", selectedToken, formatTokenAmount(selectedBalance))

	// Get recipient address
	var toAddress string
	if *toFlag != "" {
		// Check if it's a wallet file
		if walletInfo, err := loadWallet(*toFlag); err == nil {
			toAddress = walletInfo.Address
		} else {
			// Assume it's an address
			toAddress = *toFlag
		}
	} else {
		fmt.Print("Enter recipient address (or wallet filename): ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		// Check if it's a wallet file
		if walletInfo, err := loadWallet(input); err == nil {
			toAddress = walletInfo.Address
			fmt.Printf("Using wallet address: %s\n", toAddress)
		} else {
			toAddress = input
		}
	}

	// Get amount
	var amount uint64
	if *amountFlag > 0 {
		amount = *amountFlag
	} else {
		fmt.Printf("Enter amount to send (max: %s): ", formatTokenAmount(selectedBalance))
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		parsed, err := strconv.ParseUint(input, 10, 64)
		if err != nil {
			log.Fatalf("Invalid amount: %v", err)
		}
		amount = parsed
	}

	// Validate amount
	if amount == 0 {
		log.Fatal("Amount must be greater than 0")
	}
	if amount > selectedBalance {
		log.Fatalf("Insufficient balance: need %s, have %s", formatTokenAmount(amount), formatTokenAmount(selectedBalance))
	}

	// Default fee for token transfers (1 tKALON = 1000000 micro-KALON)
	defaultFee := uint64(1000000)
	fmt.Printf("\nNetwork fee: %.2f tKALON (default)\n\n", float64(defaultFee)/1000000.0)

	// If we don't have a wallet for signing, we need to load it
	if fromWallet == nil {
		// Try to find wallet by address
		wallets := getAvailableWallets()
		for _, wf := range wallets {
			walletInfo, err := loadWallet(wf)
			if err != nil {
				continue
			}
			if walletInfo.Address == fromAddress {
				bm := crypto.NewBIP39Manager()
				fromWallet, err = bm.CreateWalletFromMnemonic(walletInfo.Mnemonic, "")
				if err != nil {
					continue
				}
				break
			}
		}
		if fromWallet == nil {
			log.Fatal("Cannot find wallet for address. Please specify --from with wallet file")
		}
	}

	// Send token transfer
	txResp, err := sendTokenTransfer(rpcURL, fromAddress, toAddress, selectedToken, amount, defaultFee, fromWallet)
	if err != nil {
		log.Fatalf("Failed to send token transfer: %v", err)
	}

	// Output result
	fmt.Printf("\n✅ Token transfer sent successfully!\n")
	fmt.Printf("Hash: %s\n", txResp.Hash)
	fmt.Printf("From: %s\n", txResp.From)
	fmt.Printf("To: %s\n", txResp.To)
	fmt.Printf("Token: %s\n", selectedToken)
	fmt.Printf("Amount: %s\n", formatTokenAmount(amount))
	fmt.Printf("Fee: %.2f tKALON (%d micro-KALON)\n", float64(txResp.Fee)/1000000.0, txResp.Fee)
	fmt.Printf("Nonce: %d\n", txResp.Nonce)
	fmt.Printf("Success: %v\n", txResp.Success)
}

// sendTokenTransfer sends a token transfer transaction
func sendTokenTransfer(rpcURL string, fromAddress, toAddress, tokenName string, amount, fee uint64, wallet *crypto.Wallet) (*TransactionResponse, error) {
	// Ensure RPC URL has /rpc endpoint
	if !strings.HasSuffix(rpcURL, "/rpc") {
		if !strings.HasSuffix(rpcURL, "/") {
			rpcURL += "/rpc"
		} else {
			rpcURL += "rpc"
		}
	}

	client := &http.Client{Timeout: 30 * time.Second}

	// Step 1: Create token transfer transaction via sendToken RPC
	sendTokenReq := RPCRequest{
		JSONRPC: "2.0",
		Method:  "sendToken",
		Params: map[string]interface{}{
			"from":      fromAddress,
			"to":        toAddress,
			"tokenName": tokenName,
			"amount":    amount,
			"fee":       fee,
		},
		ID: 1,
	}

	reqData, err := json.Marshal(sendTokenReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal sendToken request: %v", err)
	}

	resp, err := client.Post(rpcURL, "application/json", bytes.NewBuffer(reqData))
	if err != nil {
		return nil, fmt.Errorf("failed to create token transfer transaction: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read sendToken response: %v", err)
	}

	var sendTokenResp RPCResponse
	if err := json.Unmarshal(body, &sendTokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse sendToken response: %v", err)
	}

	if sendTokenResp.Error != nil {
		return nil, fmt.Errorf("RPC error creating token transfer: %s", sendTokenResp.Error.Message)
	}

	// Parse transaction from sendToken response
	txData, ok := sendTokenResp.Result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid sendToken response format")
	}

	// Build transaction from server response
	tx := &core.Transaction{}

	// Parse addresses
	if fromStr, ok := txData["from"].(string); ok {
		tx.From = core.AddressFromString(fromStr)
	}
	if toStr, ok := txData["to"].(string); ok {
		tx.To = core.AddressFromString(toStr)
	}

	// Parse amounts
	if amountFloat, ok := txData["amount"].(float64); ok {
		tx.Amount = uint64(amountFloat)
	}
	if feeFloat, ok := txData["fee"].(float64); ok {
		tx.Fee = uint64(feeFloat)
	}
	if nonce, ok := txData["nonce"].(float64); ok {
		tx.Nonce = uint64(nonce)
	}
	if gasUsed, ok := txData["gasUsed"].(float64); ok {
		tx.GasUsed = uint64(gasUsed)
	}
	if tx.GasUsed == 0 {
		tx.GasUsed = 1
	}
	if gasPrice, ok := txData["gasPrice"].(float64); ok {
		tx.GasPrice = uint64(gasPrice)
	}
	if tx.GasPrice == 0 {
		if tx.Fee > 0 {
			tx.GasPrice = tx.Fee
		} else {
			tx.GasPrice = 10000
		}
	}

	// Parse data (token transfer data)
	if dataStr, ok := txData["data"].(string); ok {
		dataBytes, err := hex.DecodeString(dataStr)
		if err == nil {
			tx.Data = dataBytes
		}
	}

	// Parse inputs and outputs
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

	// Parse hash
	if hashStr, ok := txData["hash"].(string); ok {
		if hashBytes, err := hex.DecodeString(hashStr); err == nil && len(hashBytes) == 32 {
			copy(tx.Hash[:], hashBytes)
		} else {
			tx.Hash = tx.CalculateHash()
		}
	} else {
		tx.Hash = tx.CalculateHash()
	}

	// Step 2: Sign transaction locally
	if err := wallet.SignTransaction(tx); err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %v", err)
	}

	// Step 3: Send signed transaction
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

	signedTxReq := RPCRequest{
		JSONRPC: "2.0",
		Method:  "sendTransaction",
		Params: map[string]interface{}{
			"transaction": map[string]interface{}{
				"from":      hex.EncodeToString(tx.From[:]),
				"to":        hex.EncodeToString(tx.To[:]),
				"amount":    tx.Amount,
				"fee":       tx.Fee,
				"nonce":     tx.Nonce,
				"gasUsed":   tx.GasUsed,
				"gasPrice":  tx.GasPrice,
				"data":      hex.EncodeToString(tx.Data),
				"signature": hex.EncodeToString(tx.Signature),
				"publicKey": hex.EncodeToString(tx.PublicKey),
				"hash":      hex.EncodeToString(tx.Hash[:]),
				"inputs":    inputs,
				"outputs":   outputs,
			},
		},
		ID: 2,
	}

	signedReqData, err := json.Marshal(signedTxReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal signed transaction: %v", err)
	}

	resp2, err := client.Post(rpcURL, "application/json", bytes.NewBuffer(signedReqData))
	if err != nil {
		return nil, fmt.Errorf("failed to send signed transaction: %v", err)
	}
	defer resp2.Body.Close()

	body2, err := io.ReadAll(resp2.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read send response: %v", err)
	}

	var sendResp RPCResponse
	if err := json.Unmarshal(body2, &sendResp); err != nil {
		return nil, fmt.Errorf("failed to parse send response: %v", err)
	}

	if sendResp.Error != nil {
		return nil, fmt.Errorf("RPC error: %s", sendResp.Error.Message)
	}

	result, ok := sendResp.Result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}

	txHash, _ := result["txHash"].(string)
	if txHash == "" {
		// Fallback to hash field
		if hash, ok := result["hash"].(string); ok {
			txHash = hash
		}
	}

	return &TransactionResponse{
		Hash:    txHash,
		From:    fromAddress,
		To:      toAddress,
		Amount:  amount,
		Fee:     fee,
		Nonce:   tx.Nonce,
		Success: true,
	}, nil
}

// formatTokenAmount formats token amount for display
func formatTokenAmount(amount uint64) string {
	// Format with thousand separators
	amountStr := strconv.FormatUint(amount, 10)
	if len(amountStr) <= 3 {
		return amountStr
	}

	// Add thousand separators
	var result strings.Builder
	for i, digit := range amountStr {
		if i > 0 && (len(amountStr)-i)%3 == 0 {
			result.WriteString(",")
		}
		result.WriteRune(digit)
	}
	return result.String()
}

// handleDeployToken handles token deployment
func handleDeployToken(wm *WalletManager, args []string) {
	fs := flag.NewFlagSet("deploy-token", flag.ExitOnError)
	fromFlag := fs.String("from", "", "Creator wallet file or address")
	nameFlag := fs.String("name", "", "Token name")
	descriptionFlag := fs.String("description", "", "Token description")
	totalSupplyFlag := fs.Uint64("totalSupply", 0, "Total supply (amount)")
	rpcURLFlag := fs.String("rpc", "", "RPC server URL (default: https://explorer.kalon-network.com/rpc)")
	fs.Parse(args)

	reader := bufio.NewReader(os.Stdin)
	// Use default RPC if not provided
	rpcURL := defaultRPC
	if *rpcURLFlag != "" {
		rpcURL = *rpcURLFlag
	}

	// Ensure RPC URL has /rpc endpoint
	if !strings.HasSuffix(rpcURL, "/rpc") {
		if !strings.HasSuffix(rpcURL, "/") {
			rpcURL += "/rpc"
		} else {
			rpcURL += "rpc"
		}
	}

	// Interactive mode if parameters are missing
	var fromWallet *crypto.Wallet
	var fromAddress string
	var tokenName string
	var tokenDescription string
	var totalSupply uint64

	// Get creator wallet
	if *fromFlag != "" {
		// Try to load as wallet file
		if walletInfo, err := loadWallet(*fromFlag); err == nil {
			bm := crypto.NewBIP39Manager()
			fromWallet, err = bm.CreateWalletFromMnemonic(walletInfo.Mnemonic, "")
			if err != nil {
				log.Fatalf("Failed to create wallet from mnemonic: %v", err)
			}
			fromAddress, _ = fromWallet.GetAddressString()
		} else {
			// Assume it's an address
			fromAddress = *fromFlag
		}
	} else if wm.wallet != nil {
		// Use wallet from wallet manager if available
		fromWallet = wm.wallet
		fromAddress, _ = fromWallet.GetAddressString()
	} else {
		// Interactive selection
		walletFile, wallet, err := selectWallet("Select creator wallet:")
		if err != nil {
			log.Fatal(err)
		}
		fromWallet = wallet
		fromAddress, _ = wallet.GetAddressString()
		fmt.Printf("Selected: %s (%s)\n", walletFile, fromAddress)
	}

	// Get token name
	if *nameFlag != "" {
		tokenName = *nameFlag
	} else {
		fmt.Print("Enter token name: ")
		input, _ := reader.ReadString('\n')
		tokenName = strings.TrimSpace(input)
		if tokenName == "" {
			log.Fatal("Token name is required")
		}
	}

	// Get token description
	if *descriptionFlag != "" {
		tokenDescription = *descriptionFlag
	} else {
		fmt.Print("Enter token description: ")
		input, _ := reader.ReadString('\n')
		tokenDescription = strings.TrimSpace(input)
	}

	// Get total supply
	if *totalSupplyFlag > 0 {
		totalSupply = *totalSupplyFlag
	} else {
		fmt.Print("Enter total supply (amount): ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		parsed, err := strconv.ParseUint(input, 10, 64)
		if err != nil {
			log.Fatalf("Invalid total supply: %v", err)
		}
		totalSupply = parsed
	}

	// Validate
	if fromAddress == "" || tokenName == "" || totalSupply == 0 {
		log.Fatal("Creator address, token name, and total supply are required")
	}

	// If we don't have a wallet for signing, we need to load it
	if fromWallet == nil {
		// Try to find wallet by address
		wallets := getAvailableWallets()
		for _, wf := range wallets {
			walletInfo, err := loadWallet(wf)
			if err != nil {
				continue
			}
			if walletInfo.Address == fromAddress {
				bm := crypto.NewBIP39Manager()
				fromWallet, err = bm.CreateWalletFromMnemonic(walletInfo.Mnemonic, "")
				if err != nil {
					continue
				}
				break
			}
		}
		if fromWallet == nil {
			log.Fatal("Cannot find wallet for address. Please specify --from with wallet file")
		}
	}

	// Deploy token via RPC
	fmt.Printf("\nDeploying token '%s'...\n", tokenName)
	fmt.Printf("Cost: 10 KALON (10,000,000 micro-KALON)\n\n")

	txResp, err := deployToken(rpcURL, fromAddress, tokenName, tokenDescription, totalSupply, fromWallet)
	if err != nil {
		log.Fatalf("Failed to deploy token: %v", err)
	}

	// Output result
	fmt.Printf("\n✅ Token deployed successfully!\n")
	fmt.Printf("Token Name: %s\n", tokenName)
	fmt.Printf("Description: %s\n", tokenDescription)
	fmt.Printf("Total Supply: %d\n", totalSupply)
	fmt.Printf("Creator: %s\n", fromAddress)
	fmt.Printf("Transaction Hash: %s\n", txResp.Hash)
	fmt.Printf("Fee: %.2f tKALON (%d micro-KALON)\n", float64(txResp.Fee)/1000000.0, txResp.Fee)
	fmt.Printf("Nonce: %d\n", txResp.Nonce)
	fmt.Printf("Success: %v\n", txResp.Success)
}

// deployToken deploys a token via RPC
func deployToken(rpcURL string, creatorAddress, tokenName, tokenDescription string, totalSupply uint64, wallet *crypto.Wallet) (*TransactionResponse, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	// Step 1: Call deployToken RPC method
	deployReq := RPCRequest{
		JSONRPC: "2.0",
		Method:  "deployToken",
		Params: map[string]interface{}{
			"name":        tokenName,
			"description": tokenDescription,
			"creator":     creatorAddress,
			"totalSupply": totalSupply,
		},
		ID: 1,
	}

	reqData, err := json.Marshal(deployReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal deploy request: %v", err)
	}

	resp, err := client.Post(rpcURL, "application/json", bytes.NewBuffer(reqData))
	if err != nil {
		return nil, fmt.Errorf("failed to deploy token: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read deploy response: %v", err)
	}

	var deployResp RPCResponse
	if err := json.Unmarshal(body, &deployResp); err != nil {
		return nil, fmt.Errorf("failed to parse deploy response: %v", err)
	}

	if deployResp.Error != nil {
		return nil, fmt.Errorf("RPC error deploying token: %s", deployResp.Error.Message)
	}

	// Parse deploy response - should contain transaction data in "transaction" field
	deployData, ok := deployResp.Result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid deploy token response format")
	}

	// Get transaction from "transaction" field
	txData, ok := deployData["transaction"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid transaction data in deploy response")
	}

	// Build transaction from server response
	tx := &core.Transaction{}

	// CRITICAL: Store fee from transaction data BEFORE signing (for deployToken)
	var transactionFee uint64

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
		transactionFee = uint64(fee)
	}
	if nonce, ok := txData["nonce"].(float64); ok {
		tx.Nonce = uint64(nonce)
	}
	if gasUsed, ok := txData["gasUsed"].(float64); ok {
		tx.GasUsed = uint64(gasUsed)
	}
	if tx.GasUsed == 0 {
		tx.GasUsed = 1
	}
	if gasPrice, ok := txData["gasPrice"].(float64); ok {
		tx.GasPrice = uint64(gasPrice)
	}
	if tx.GasPrice == 0 {
		if tx.Fee > 0 {
			tx.GasPrice = tx.Fee
		} else {
			tx.GasPrice = 100000
		}
	}

	// Parse data (token deployment data)
	if dataStr, ok := txData["data"].(string); ok {
		dataBytes, err := hex.DecodeString(dataStr)
		if err == nil {
			tx.Data = dataBytes
		}
	}

	// Parse inputs
	if inputs, ok := txData["inputs"].([]interface{}); ok {
		tx.Inputs = make([]core.TxInput, 0, len(inputs))
		for _, inputData := range inputs {
			if inputMap, ok := inputData.(map[string]interface{}); ok {
				var input core.TxInput
				if prevHashStr, ok := inputMap["previousTxHash"].(string); ok {
					prevHashBytes, err := hex.DecodeString(prevHashStr)
					if err == nil && len(prevHashBytes) == 32 {
						copy(input.PreviousTxHash[:], prevHashBytes)
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
	if outputs, ok := txData["outputs"].([]interface{}); ok {
		tx.Outputs = make([]core.TxOutput, 0, len(outputs))
		for _, outputData := range outputs {
			if outputMap, ok := outputData.(map[string]interface{}); ok {
				var output core.TxOutput
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

	// Parse timestamp
	if timestamp, ok := txData["timestamp"].(float64); ok {
		tx.Timestamp = time.Unix(int64(timestamp), 0)
	} else {
		tx.Timestamp = time.Now()
	}

	// Sign transaction
	if err := wallet.SignTransaction(tx); err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %v", err)
	}

	// Calculate hash
	tx.Hash = tx.CalculateHash()

	// Step 2: Send signed transaction
	// Serialize inputs
	inputs := make([]interface{}, 0, len(tx.Inputs))
	for _, input := range tx.Inputs {
		inputs = append(inputs, map[string]interface{}{
			"previousTxHash": hex.EncodeToString(input.PreviousTxHash[:]),
			"index":          input.Index,
		})
	}

	// Serialize outputs
	outputs := make([]interface{}, 0, len(tx.Outputs))
	for _, output := range tx.Outputs {
		outputs = append(outputs, map[string]interface{}{
			"address": hex.EncodeToString(output.Address[:]),
			"amount":  output.Amount,
		})
	}

	sendReq := RPCRequest{
		JSONRPC: "2.0",
		Method:  "sendTransaction",
		Params: map[string]interface{}{
			"transaction": map[string]interface{}{
				"from":      hex.EncodeToString(tx.From[:]),
				"to":        hex.EncodeToString(tx.To[:]),
				"amount":    tx.Amount,
				"fee":       tx.Fee,
				"nonce":     tx.Nonce,
				"gasUsed":   tx.GasUsed,
				"gasPrice":  tx.GasPrice,
				"data":      hex.EncodeToString(tx.Data),
				"signature": hex.EncodeToString(tx.Signature),
				"publicKey": hex.EncodeToString(tx.PublicKey),
				"inputs":    inputs,
				"outputs":   outputs,
				"timestamp": tx.Timestamp.Unix(),
			},
		},
		ID: 2,
	}

	reqData, err = json.Marshal(sendReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal send request: %v", err)
	}

	resp, err = client.Post(rpcURL, "application/json", bytes.NewBuffer(reqData))
	if err != nil {
		return nil, fmt.Errorf("failed to send transaction: %v", err)
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read send response: %v", err)
	}

	var sendResp RPCResponse
	if err := json.Unmarshal(body, &sendResp); err != nil {
		return nil, fmt.Errorf("failed to parse send response: %v", err)
	}

	if sendResp.Error != nil {
		return nil, fmt.Errorf("RPC error sending transaction: %s", sendResp.Error.Message)
	}

	// Parse send response
	sendData, ok := sendResp.Result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid send transaction response format")
	}

	// Build response
	txResp := &TransactionResponse{
		Success: true,
	}

	// RPC returns "txHash", but also check "hash" for compatibility
	if hashStr, ok := sendData["txHash"].(string); ok {
		txResp.Hash = hashStr
	} else if hashStr, ok := sendData["hash"].(string); ok {
		txResp.Hash = hashStr
	}
	if fromStr, ok := sendData["from"].(string); ok {
		txResp.From = fromStr
	}
	if toStr, ok := sendData["to"].(string); ok {
		txResp.To = toStr
	}
	if amount, ok := sendData["amount"].(float64); ok {
		txResp.Amount = uint64(amount)
	}
	// CRITICAL: Fee is not returned by sendTransaction RPC, use the fee from the original transaction
	if transactionFee > 0 {
		txResp.Fee = transactionFee
	} else if fee, ok := sendData["fee"].(float64); ok {
		txResp.Fee = uint64(fee)
	} else {
		// Fallback: use fee from tx object if available
		txResp.Fee = tx.Fee
	}
	if nonce, ok := sendData["nonce"].(float64); ok {
		txResp.Nonce = uint64(nonce)
	} else {
		txResp.Nonce = tx.Nonce
	}

	return txResp, nil
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
	// Ensure RPC URL has /rpc endpoint if it's a base URL
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
// SECURITY: Uses prepareTransaction -> local signing -> sendTransaction flow
// This ensures the private key never leaves the client
func sendTransaction(rpcURL string, txReq *TransactionRequest, wallet *crypto.Wallet) (*TransactionResponse, error) {
	// Ensure RPC URL has /rpc endpoint
	if !strings.HasSuffix(rpcURL, "/rpc") {
		if !strings.HasSuffix(rpcURL, "/") {
			rpcURL += "/rpc"
		} else {
			rpcURL += "rpc"
		}
	}

	client := &http.Client{Timeout: 30 * time.Second}

	// Step 1: Prepare transaction from server (get UTXOs and structure)
	prepareReq := RPCRequest{
		JSONRPC: "2.0",
		Method:  "prepareTransaction",
		Params: map[string]interface{}{
			"from":   txReq.From,
			"to":     txReq.To,
			"amount": txReq.Amount,
			"fee":    txReq.Fee,
		},
		ID: 1,
	}

	reqData, err := json.Marshal(prepareReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal prepare request: %v", err)
	}

	resp, err := client.Post(rpcURL, "application/json", bytes.NewBuffer(reqData))
	if err != nil {
		return nil, fmt.Errorf("failed to prepare transaction: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read prepare response: %v", err)
	}

	var prepareResp RPCResponse
	if err := json.Unmarshal(body, &prepareResp); err != nil {
		return nil, fmt.Errorf("failed to parse prepare response: %v", err)
	}

	if prepareResp.Error != nil {
		return nil, fmt.Errorf("RPC error preparing transaction: %s", prepareResp.Error.Message)
	}

	// Parse prepared transaction
	txData, ok := prepareResp.Result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid prepare transaction response format")
	}

	// Build transaction from server response
	tx := &core.Transaction{}

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
	if tx.GasUsed == 0 {
		tx.GasUsed = 1
	}
	if gasPrice, ok := txData["gasPrice"].(float64); ok {
		tx.GasPrice = uint64(gasPrice)
	}
	if tx.GasPrice == 0 {
		if tx.Fee > 0 {
			tx.GasPrice = tx.Fee
		} else {
			tx.GasPrice = 100000
		}
	}

	// Parse data
	if dataStr, ok := txData["data"].(string); ok {
		if dataBytes, err := hex.DecodeString(dataStr); err == nil {
			tx.Data = dataBytes
		}
	}

	// Parse inputs and outputs
	if inputsData, ok := txData["inputs"].([]interface{}); ok {
		log.Printf("🔵 [DEBUG] Parsing %d inputs from prepareTransaction response", len(inputsData))
		for i, inputData := range inputsData {
			if inputMap, ok := inputData.(map[string]interface{}); ok {
				input := core.TxInput{}
				if prevTxHashStr, ok := inputMap["previousTxHash"].(string); ok {
					if prevTxHashBytes, err := hex.DecodeString(prevTxHashStr); err == nil && len(prevTxHashBytes) == 32 {
						copy(input.PreviousTxHash[:], prevTxHashBytes)
						log.Printf("🔵 [DEBUG] Input[%d]: previousTxHash=%x, index=%d", i, input.PreviousTxHash, input.Index)
					} else {
						log.Printf("❌ [DEBUG] Input[%d]: Failed to decode previousTxHash: %s (error: %v, len: %d)", i, prevTxHashStr, err, len(prevTxHashBytes))
						// Skip this input if hash is invalid
						continue
					}
				} else {
					log.Printf("❌ [DEBUG] Input[%d]: No previousTxHash field found", i)
					// Skip this input if no hash field
					continue
				}
				if index, ok := inputMap["index"].(float64); ok {
					input.Index = uint32(index)
				}
				tx.Inputs = append(tx.Inputs, input)
			} else {
				log.Printf("❌ [DEBUG] Input[%d]: Failed to parse input map", i)
			}
		}
		log.Printf("🔵 [DEBUG] Successfully parsed %d inputs", len(tx.Inputs))
		// Log each input to verify they were parsed correctly
		for i, input := range tx.Inputs {
			log.Printf("🔵 [DEBUG] Parsed Input[%d]: previousTxHash=%x, index=%d", i, input.PreviousTxHash, input.Index)
		}
	} else {
		log.Printf("⚠️ [DEBUG] No inputs array found in prepareTransaction response")
	}

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

	// Parse hash from prepareTransaction response (if provided)
	if hashStr, ok := txData["hash"].(string); ok {
		if hashBytes, err := hex.DecodeString(hashStr); err == nil && len(hashBytes) == 32 {
			copy(tx.Hash[:], hashBytes)
		} else {
			// If hash is invalid, calculate it
			tx.Hash = tx.CalculateHash()
		}
	} else {
		// If no hash provided, calculate it
		tx.Hash = tx.CalculateHash()
	}

	// Step 2: Sign transaction locally (SECURITY: Private key never leaves client)
	if err := wallet.SignTransaction(tx); err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %v", err)
	}

	// Step 3: Send signed transaction
	// Serialize inputs
	log.Printf("🔵 [DEBUG] Serializing %d inputs for sendTransaction", len(tx.Inputs))
	if len(tx.Inputs) == 0 {
		log.Printf("❌ [DEBUG] ERROR: Transaction has NO INPUTS! This will cause UTXO errors!")
		return nil, fmt.Errorf("transaction has no inputs - cannot send transaction without UTXO references")
	}
	inputs := make([]interface{}, 0, len(tx.Inputs))
	for i, input := range tx.Inputs {
		prevTxHashHex := hex.EncodeToString(input.PreviousTxHash[:])
		log.Printf("🔵 [DEBUG] Serializing Input[%d]: previousTxHash=%x (hex: %s), index=%d", i, input.PreviousTxHash, prevTxHashHex, input.Index)
		if input.PreviousTxHash == (core.Hash{}) {
			log.Printf("❌ [DEBUG] ERROR: Input[%d] has EMPTY previousTxHash! This will cause UTXO errors!", i)
		}
		inputs = append(inputs, map[string]interface{}{
			"previousTxHash": prevTxHashHex,
			"index":          input.Index,
		})
	}
	log.Printf("🔵 [DEBUG] Serialized %d inputs for JSON", len(inputs))

	// Serialize outputs
	outputs := make([]interface{}, 0, len(tx.Outputs))
	for _, output := range tx.Outputs {
		outputs = append(outputs, map[string]interface{}{
			"address": hex.EncodeToString(output.Address[:]),
			"amount":  output.Amount,
		})
	}

	signedTxReq := RPCRequest{
		JSONRPC: "2.0",
		Method:  "sendTransaction",
		Params: map[string]interface{}{
			"transaction": map[string]interface{}{
				"from":      hex.EncodeToString(tx.From[:]),
				"to":        hex.EncodeToString(tx.To[:]),
				"amount":    tx.Amount,
				"fee":       tx.Fee,
				"nonce":     tx.Nonce,
				"gasUsed":   tx.GasUsed,
				"gasPrice":  tx.GasPrice,
				"data":      hex.EncodeToString(tx.Data),
				"signature": hex.EncodeToString(tx.Signature),
				"publicKey": hex.EncodeToString(tx.PublicKey),
				"hash":      hex.EncodeToString(tx.Hash[:]),
				"inputs":    inputs,
				"outputs":   outputs,
			},
		},
		ID: 2,
	}

	signedReqData, err := json.Marshal(signedTxReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal signed transaction: %v", err)
	}

	resp2, err := client.Post(rpcURL, "application/json", bytes.NewBuffer(signedReqData))
	if err != nil {
		return nil, fmt.Errorf("failed to send signed transaction: %v", err)
	}
	defer resp2.Body.Close()

	body2, err := io.ReadAll(resp2.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read send response: %v", err)
	}

	var sendResp RPCResponse
	if err := json.Unmarshal(body2, &sendResp); err != nil {
		return nil, fmt.Errorf("failed to parse send response: %v", err)
	}

	if sendResp.Error != nil {
		return nil, fmt.Errorf("RPC error: %s", sendResp.Error.Message)
	}

	result, ok := sendResp.Result.(map[string]interface{})
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
		Nonce:   tx.Nonce,
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
	fmt.Println("  create        Create a new wallet")
	fmt.Println("  import        Import wallet from mnemonic")
	fmt.Println("  list          List all available wallets")
	fmt.Println("  export        Export wallet information")
	fmt.Println("  balance       Check wallet balance")
	fmt.Println("  send          Send transaction")
	fmt.Println("  send-token    Send token transfer")
	fmt.Println("  deploy-token  Deploy a new token (costs 10 KALON)")
	fmt.Println("  info          Show wallet information")
	fmt.Println("  help          Show this help message")
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
