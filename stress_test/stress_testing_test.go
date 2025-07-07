package main

import (
	"context"
	"crypto/ecdsa"
	"encoding/csv"
	"fmt"
	"log"
	"math/big"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	onemoney "github.com/1Money-Co/1money-go-sdk"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// Configurable Constants - Modify these values as needed for different stress test scenarios
const (
	// Wallet Configuration
	MINT_WALLETS_COUNT     = 20  // Number of mint authority wallets
	TRANSFER_WALLETS_COUNT = 500 // Number of primary transfer recipient wallets
	WALLETS_PER_MINT       = 25  // Number of transfer wallets per mint wallet (should equal TRANSFER_WALLETS_COUNT / MINT_WALLETS_COUNT)

	// Multi-tier Distribution Configuration
	TRANSFER_MULTIPLIER        = 4                                            // Number of distribution wallets per primary wallet
	DISTRIBUTION_WALLETS_COUNT = TRANSFER_WALLETS_COUNT * TRANSFER_MULTIPLIER // Total distribution wallets (8000)
	TRANSFER_WORKERS_COUNT     = 10                                           // Number of concurrent transfer worker goroutines

	// Token Configuration
	TOKEN_SYMBOL   = "STRESS15"
	TOKEN_NAME     = "Stress Test Token"
	TOKEN_DECIMALS = 6
	CHAIN_ID       = 1212101

	// Mint Configuration
	MINT_ALLOWANCE  = 1000000000                              // Allowance granted to each mint wallet
	MINT_AMOUNT     = 1000                                    // Amount to mint per operation
	TRANSFER_AMOUNT = MINT_AMOUNT / (TRANSFER_MULTIPLIER + 1) // Amount to transfer per distribution operation (250)

	// Transaction Validation Configuration
	RECEIPT_CHECK_TIMEOUT    = 10 * time.Second       // Timeout for waiting for transaction receipt
	RECEIPT_CHECK_INTERVAL   = 150 * time.Millisecond // Interval between receipt checks
	NONCE_VALIDATION_TIMEOUT = 10 * time.Second       // Timeout for nonce validation
	NONCE_CHECK_INTERVAL     = 150 * time.Millisecond // Interval between nonce checks
)

// Wallet represents a wallet with private key, public key, and address
type Wallet struct {
	PrivateKey string
	PublicKey  string
	Address    string
}

// TransferTask represents a task for transferring tokens from a primary wallet to distribution wallets
type TransferTask struct {
	PrimaryWallet *Wallet // The primary wallet that received minted tokens
	StartIdx      int     // Starting index in distributionWallets array
	EndIdx        int     // Ending index in distributionWallets array
	PrimaryIndex  int     // Index of the primary wallet for logging purposes
}

// StressTester manages the stress testing operations
type StressTester struct {
	client              *onemoney.Client
	operatorWallet      *Wallet
	mintWallets         []*Wallet
	transferWallets     []*Wallet // Primary transfer wallets (tier 2)
	distributionWallets []*Wallet // Distribution wallets (tier 3)
	tokenAddress        string
	ctx                 context.Context
}

// generateWallet creates a new wallet with private key, public key, and address
func generateWallet() (*Wallet, error) {
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("failed to cast public key to ECDSA")
	}

	address := crypto.PubkeyToAddress(*publicKeyECDSA)

	return &Wallet{
		PrivateKey: fmt.Sprintf("%x", crypto.FromECDSA(privateKey)),
		PublicKey:  fmt.Sprintf("%x", crypto.FromECDSAPub(publicKeyECDSA)),
		Address:    address.Hex(),
	}, nil
}

// getOperatorConfig retrieves operator wallet configuration from environment variables or SDK defaults
func getOperatorConfig() (privateKey, address string, err error) {
	// Priority: use environment variables
	if pk := os.Getenv("OPERATOR_PRIVATE_KEY"); pk != "" {
		if addr := os.Getenv("OPERATOR_ADDRESS"); addr != "" {
			return pk, addr, nil
		}
	}

	// Fallback: use SDK test configuration (if available)
	if onemoney.TestOperatorPrivateKey != "" && onemoney.TestOperatorAddress != "" {
		return onemoney.TestOperatorPrivateKey, onemoney.TestOperatorAddress, nil
	}

	return "", "", fmt.Errorf("operator configuration not found. Please set OPERATOR_PRIVATE_KEY and OPERATOR_ADDRESS environment variables, or configure TestOperatorPrivateKey and TestOperatorAddress in the SDK")
}

// NewStressTester creates a new stress tester instance
func NewStressTester() (*StressTester, error) {
	client := onemoney.NewTestClient()

	// Get operator wallet configuration
	privateKey, address, err := getOperatorConfig()
	if err != nil {
		return nil, err
	}

	operatorWallet := &Wallet{
		PrivateKey: privateKey,
		Address:    address,
	}

	return &StressTester{
		client:         client,
		operatorWallet: operatorWallet,
		ctx:            context.Background(),
	}, nil
}

// waitForTransactionReceipt waits for transaction receipt and validates success
func (st *StressTester) waitForTransactionReceipt(txHash string) error {
	timeout := time.After(RECEIPT_CHECK_TIMEOUT)
	ticker := time.NewTicker(RECEIPT_CHECK_INTERVAL)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for receipt of transaction %s", txHash)
		case <-ticker.C:
			receipt, err := st.client.GetTransactionReceipt(st.ctx, txHash)
			if err != nil {
				log.Printf("Error getting receipt for transaction %s: %v", txHash, err)
				continue
			}

			if receipt != nil {
				if receipt.Success {
					log.Printf("Transaction %s confirmed successfully", txHash)
					return nil
				} else {
					return fmt.Errorf("transaction %s failed", txHash)
				}
			}
		}
	}
}

// validateNonceIncrement validates that the nonce has incremented by 1 after a transaction
func (st *StressTester) validateNonceIncrement(address string, expectedNonce uint64) error {
	timeout := time.After(NONCE_VALIDATION_TIMEOUT)
	ticker := time.NewTicker(NONCE_CHECK_INTERVAL)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for nonce validation for address %s", address)
		case <-ticker.C:
			accountNonce, err := st.client.GetAccountNonce(st.ctx, address)
			if err != nil {
				log.Printf("Error getting nonce for address %s: %v", address, err)
				continue
			}

			if accountNonce.Nonce == expectedNonce {
				log.Printf("Nonce validated for address %s: %d", address, accountNonce.Nonce)
				return nil
			}

			if accountNonce.Nonce > expectedNonce {
				return fmt.Errorf("nonce validation failed for address %s: expected %d, got %d", address, expectedNonce, accountNonce.Nonce)
			}
		}
	}
}

// getAccountNonce retrieves the current nonce for an address
func (st *StressTester) getAccountNonce(address string) (uint64, error) {
	accountNonce, err := st.client.GetAccountNonce(st.ctx, address)
	if err != nil {
		return 0, fmt.Errorf("failed to get account nonce for %s: %w", address, err)
	}
	return accountNonce.Nonce, nil
}

// Step 1: Create mint wallets
func (st *StressTester) createMintWallets() error {
	log.Printf("Creating %d mint wallets...", MINT_WALLETS_COUNT)

	st.mintWallets = make([]*Wallet, MINT_WALLETS_COUNT)
	for i := 0; i < MINT_WALLETS_COUNT; i++ {
		wallet, err := generateWallet()
		if err != nil {
			return fmt.Errorf("failed to create mint wallet %d: %w", i, err)
		}
		st.mintWallets[i] = wallet
		log.Printf("Created mint wallet %d: %s", i+1, wallet.Address)
	}

	return nil
}

// Step 2: Create transfer wallets (primary tier)
func (st *StressTester) createTransferWallets() error {
	log.Printf("Creating %d primary transfer wallets...", TRANSFER_WALLETS_COUNT)

	st.transferWallets = make([]*Wallet, TRANSFER_WALLETS_COUNT)
	for i := 0; i < TRANSFER_WALLETS_COUNT; i++ {
		wallet, err := generateWallet()
		if err != nil {
			return fmt.Errorf("failed to create primary transfer wallet %d: %w", i, err)
		}
		st.transferWallets[i] = wallet
		log.Printf("Created primary transfer wallet %d: %s", i+1, wallet.Address)
	}

	return nil
}

// Step 2b: Create distribution wallets (third tier)
func (st *StressTester) createDistributionWallets() error {
	log.Printf("Creating %d distribution wallets...", DISTRIBUTION_WALLETS_COUNT)

	st.distributionWallets = make([]*Wallet, DISTRIBUTION_WALLETS_COUNT)
	for i := 0; i < DISTRIBUTION_WALLETS_COUNT; i++ {
		wallet, err := generateWallet()
		if err != nil {
			return fmt.Errorf("failed to create distribution wallet %d: %w", i, err)
		}
		st.distributionWallets[i] = wallet

		// Log progress every 500 wallets to avoid excessive logging
		if (i+1)%500 == 0 {
			log.Printf("Created distribution wallets: %d/%d", i+1, DISTRIBUTION_WALLETS_COUNT)
		}
	}

	log.Printf("Successfully created all %d distribution wallets", DISTRIBUTION_WALLETS_COUNT)
	return nil
}

// Step 3: Create token using operator wallet
func (st *StressTester) createToken() error {
	log.Println("Creating token...")

	nonce, err := st.getAccountNonce(st.operatorWallet.Address)
	if err != nil {
		return err
	}

	payload := onemoney.TokenIssuePayload{
		ChainID:         CHAIN_ID,
		Nonce:           nonce,
		Symbol:          TOKEN_SYMBOL,
		Name:            TOKEN_NAME,
		Decimals:        TOKEN_DECIMALS,
		MasterAuthority: common.HexToAddress(st.operatorWallet.Address),
		IsPrivate:       false,
	}

	signature, err := st.client.SignMessage(payload, st.operatorWallet.PrivateKey)
	if err != nil {
		return fmt.Errorf("failed to sign token creation: %w", err)
	}

	req := &onemoney.IssueTokenRequest{
		TokenIssuePayload: payload,
		Signature: onemoney.Signature{
			R: signature.R,
			S: signature.S,
			V: signature.V,
		},
	}

	result, err := st.client.IssueToken(st.ctx, req)
	if err != nil {
		return fmt.Errorf("failed to issue token: %w", err)
	}

	st.tokenAddress = result.Token
	log.Printf("Token created successfully: %s", st.tokenAddress)
	log.Printf("Transaction hash: %s", result.Hash)

	// Wait for transaction confirmation
	if err := st.waitForTransactionReceipt(result.Hash); err != nil {
		return fmt.Errorf("failed to confirm token creation: %w", err)
	}

	// Validate nonce increment
	if err := st.validateNonceIncrement(st.operatorWallet.Address, nonce+1); err != nil {
		return fmt.Errorf("failed to validate nonce increment after token creation: %w", err)
	}

	return nil
}

// Step 4: Grant mint permissions to each mint wallet
func (st *StressTester) grantMintAuthorities() error {
	log.Println("Granting mint authorities...")

	for i, mintWallet := range st.mintWallets {
		log.Printf("Granting mint authority to wallet %d: %s", i+1, mintWallet.Address)

		nonce, err := st.getAccountNonce(st.operatorWallet.Address)
		if err != nil {
			return err
		}

		payload := onemoney.TokenAuthorityPayload{
			ChainID:          CHAIN_ID,
			Nonce:            nonce,
			Action:           onemoney.AuthorityActionGrant,
			AuthorityType:    onemoney.AuthorityTypeMintBurnTokens,
			AuthorityAddress: common.HexToAddress(mintWallet.Address),
			Token:            common.HexToAddress(st.tokenAddress),
			Value:            big.NewInt(MINT_ALLOWANCE),
		}

		signature, err := st.client.SignMessage(payload, st.operatorWallet.PrivateKey)
		if err != nil {
			return fmt.Errorf("failed to sign authority grant for wallet %d: %w", i, err)
		}

		req := &onemoney.TokenAuthorityRequest{
			TokenAuthorityPayload: payload,
			Signature: onemoney.Signature{
				R: signature.R,
				S: signature.S,
				V: signature.V,
			},
		}

		result, err := st.client.GrantTokenAuthority(st.ctx, req)
		if err != nil {
			return fmt.Errorf("failed to grant authority to wallet %d: %w", i, err)
		}

		log.Printf("Authority granted to wallet %d, transaction: %s", i+1, result.Hash)

		// Wait for transaction confirmation
		if err := st.waitForTransactionReceipt(result.Hash); err != nil {
			return fmt.Errorf("failed to confirm authority grant for wallet %d: %w", i, err)
		}

		// Validate nonce increment
		if err := st.validateNonceIncrement(st.operatorWallet.Address, nonce+1); err != nil {
			return fmt.Errorf("failed to validate nonce increment after authority grant for wallet %d: %w", i, err)
		}
	}

	return nil
}

// transferFromWallet performs a token transfer from one wallet to another using PaymentPayload
func (st *StressTester) transferFromWallet(fromWallet, toWallet *Wallet, amount int64, fromIndex, toIndex int) error {
	log.Printf("Transferring %d tokens from wallet %d (%s) to distribution wallet %d (%s)",
		amount, fromIndex, fromWallet.Address, toIndex, toWallet.Address)

	// Get sender wallet's current nonce
	nonce, err := st.getAccountNonce(fromWallet.Address)
	if err != nil {
		return err
	}

	// Create payment payload for token transfer
	payload := onemoney.PaymentPayload{
		ChainID:   CHAIN_ID,
		Nonce:     nonce,
		Recipient: common.HexToAddress(toWallet.Address),
		Value:     big.NewInt(amount),
		Token:     common.HexToAddress(st.tokenAddress),
	}

	// Sign the payload
	signature, err := st.client.SignMessage(payload, fromWallet.PrivateKey)
	if err != nil {
		return fmt.Errorf("failed to sign transfer transaction: %w", err)
	}

	// Create payment request
	req := &onemoney.PaymentRequest{
		PaymentPayload: payload,
		Signature: onemoney.Signature{
			R: signature.R,
			S: signature.S,
			V: signature.V,
		},
	}

	// Send payment request
	result, err := st.client.SendPayment(st.ctx, req)
	if err != nil {
		return fmt.Errorf("failed to send transfer: %w", err)
	}

	log.Printf("Transfer transaction sent: %s (wallet %d -> distribution wallet %d)",
		result.Hash, fromIndex, toIndex)

	// Wait for transaction confirmation
	if err := st.waitForTransactionReceipt(result.Hash); err != nil {
		return fmt.Errorf("failed to confirm transfer transaction: %w", err)
	}

	// Validate nonce increment
	if err := st.validateNonceIncrement(fromWallet.Address, nonce+1); err != nil {
		return fmt.Errorf("failed to validate nonce increment after transfer operation: %w", err)
	}

	log.Printf("Transfer confirmed: wallet %d -> distribution wallet %d", fromIndex, toIndex)
	return nil
}

// transferWorker processes transfer tasks from the transfer channel
func (st *StressTester) transferWorker(transferTasks <-chan TransferTask, wg *sync.WaitGroup) {
	for task := range transferTasks {
		log.Printf("Processing transfer task for primary wallet %d to %d distribution wallets",
			task.PrimaryIndex, task.EndIdx-task.StartIdx)

		// Transfer tokens from primary wallet to each assigned distribution wallet
		for i := task.StartIdx; i < task.EndIdx; i++ {
			if i >= len(st.distributionWallets) {
				log.Printf("Warning: distribution wallet index %d exceeds available wallets (%d)",
					i, len(st.distributionWallets))
				break
			}

			distributionWallet := st.distributionWallets[i]
			if err := st.transferFromWallet(task.PrimaryWallet, distributionWallet,
				int64(TRANSFER_AMOUNT), task.PrimaryIndex, i+1); err != nil {
				log.Printf("Error transferring from primary wallet %d to distribution wallet %d: %v",
					task.PrimaryIndex, i+1, err)
				// Continue with next transfer instead of failing entire task
				continue
			}
		}

		log.Printf("Completed transfer task for primary wallet %d", task.PrimaryIndex)
		wg.Done()
	}
}

// Step 5: Perform concurrent minting operations with immediate transfers
func (st *StressTester) performConcurrentMinting() error {
	log.Println("Starting concurrent minting operations with multi-tier distribution...")

	var mintWG sync.WaitGroup
	var transferWG sync.WaitGroup
	errorChan := make(chan error, MINT_WALLETS_COUNT+TRANSFER_WALLETS_COUNT)

	// Create buffered channel for transfer tasks
	transferTasks := make(chan TransferTask, TRANSFER_WALLETS_COUNT)

	// Start transfer worker goroutines
	log.Printf("Starting %d transfer workers...", TRANSFER_WORKERS_COUNT)
	for i := 0; i < TRANSFER_WORKERS_COUNT; i++ {
		go st.transferWorker(transferTasks, &transferWG)
	}

	// Launch one goroutine per mint wallet
	for i, mintWallet := range st.mintWallets {
		mintWG.Add(1)
		go func(walletIndex int, wallet *Wallet) {
			defer mintWG.Done()

			// Calculate the range of transfer wallets this mint wallet is responsible for
			startIdx := walletIndex * WALLETS_PER_MINT
			endIdx := startIdx + WALLETS_PER_MINT
			if endIdx > len(st.transferWallets) {
				endIdx = len(st.transferWallets)
			}

			log.Printf("Mint wallet %d (%s) minting to primary wallets %d-%d",
				walletIndex+1, wallet.Address, startIdx+1, endIdx)

			// Perform mint operations to assigned transfer wallets
			for j := startIdx; j < endIdx; j++ {
				transferWallet := st.transferWallets[j]

				if err := st.mintToWallet(wallet, transferWallet, walletIndex+1, j+1); err != nil {
					errorChan <- fmt.Errorf("mint wallet %d failed to mint to primary wallet %d: %w",
						walletIndex+1, j+1, err)
					return
				}

				// After successful mint, immediately queue transfer task
				transferStartIdx := j * TRANSFER_MULTIPLIER
				transferEndIdx := transferStartIdx + TRANSFER_MULTIPLIER
				if transferEndIdx > len(st.distributionWallets) {
					transferEndIdx = len(st.distributionWallets)
				}

				transferTask := TransferTask{
					PrimaryWallet: transferWallet,
					StartIdx:      transferStartIdx,
					EndIdx:        transferEndIdx,
					PrimaryIndex:  j + 1,
				}

				transferWG.Add(1)
				transferTasks <- transferTask
			}

			log.Printf("Mint wallet %d completed all minting operations", walletIndex+1)
		}(i, mintWallet)
	}

	// Wait for all minting operations to complete
	mintWG.Wait()
	log.Println("All minting operations completed, waiting for transfers...")

	// Close transfer tasks channel and wait for all transfers to complete
	close(transferTasks)
	transferWG.Wait()

	close(errorChan)

	// Check for any errors
	for err := range errorChan {
		return err
	}

	log.Println("All concurrent minting and transfer operations completed successfully!")
	return nil
}

// mintToWallet performs a single mint operation from mint wallet to transfer wallet
func (st *StressTester) mintToWallet(mintWallet, transferWallet *Wallet, mintWalletIndex, transferWalletIndex int) error {
	log.Printf("Mint wallet %d minting %d tokens to transfer wallet %d (%s)",
		mintWalletIndex, MINT_AMOUNT, transferWalletIndex, transferWallet.Address)

	// Get mint wallet's current nonce
	nonce, err := st.getAccountNonce(mintWallet.Address)
	if err != nil {
		return err
	}

	// Create mint payload
	payload := onemoney.TokenMintPayload{
		ChainID:   CHAIN_ID,
		Nonce:     nonce,
		Recipient: common.HexToAddress(transferWallet.Address),
		Value:     big.NewInt(MINT_AMOUNT),
		Token:     common.HexToAddress(st.tokenAddress),
	}

	// Sign the payload
	signature, err := st.client.SignMessage(payload, mintWallet.PrivateKey)
	if err != nil {
		return fmt.Errorf("failed to sign mint transaction: %w", err)
	}

	// Create mint request
	req := &onemoney.MintTokenRequest{
		TokenMintPayload: payload,
		Signature: onemoney.Signature{
			R: signature.R,
			S: signature.S,
			V: signature.V,
		},
	}

	// Send mint request
	result, err := st.client.MintToken(st.ctx, req)
	if err != nil {
		return fmt.Errorf("failed to mint token: %w", err)
	}

	log.Printf("Mint transaction sent: %s (mint wallet %d -> transfer wallet %d)",
		result.Hash, mintWalletIndex, transferWalletIndex)

	// Wait for transaction confirmation
	if err := st.waitForTransactionReceipt(result.Hash); err != nil {
		return fmt.Errorf("failed to confirm mint transaction: %w", err)
	}

	// Validate nonce increment
	if err := st.validateNonceIncrement(mintWallet.Address, nonce+1); err != nil {
		return fmt.Errorf("failed to validate nonce increment after mint operation: %w", err)
	}

	log.Printf("Mint confirmed: mint wallet %d -> transfer wallet %d",
		mintWalletIndex, transferWalletIndex)

	return nil
}

// RunStressTest executes the complete stress test workflow
func (st *StressTester) RunStressTest() error {
	log.Println("=== Starting 1Money Multi-Tier Batch Mint Stress Test ===")
	log.Printf("Configuration:")
	log.Printf("- Mint wallets: %d", MINT_WALLETS_COUNT)
	log.Printf("- Primary transfer wallets: %d", TRANSFER_WALLETS_COUNT)
	log.Printf("- Distribution wallets: %d", DISTRIBUTION_WALLETS_COUNT)
	log.Printf("- Transfer multiplier: %d", TRANSFER_MULTIPLIER)
	log.Printf("- Transfer workers: %d", TRANSFER_WORKERS_COUNT)
	log.Printf("- Wallets per mint: %d", WALLETS_PER_MINT)
	log.Printf("- Mint allowance: %d", MINT_ALLOWANCE)
	log.Printf("- Mint amount per operation: %d", MINT_AMOUNT)
	log.Printf("- Transfer amount per operation: %d", TRANSFER_AMOUNT)
	log.Printf("- Token symbol: %s", TOKEN_SYMBOL)
	log.Printf("- Token name: %s", TOKEN_NAME)
	log.Printf("- Chain ID: %d", CHAIN_ID)
	log.Println()

	// Step 1: Create mint wallets
	if err := st.createMintWallets(); err != nil {
		return fmt.Errorf("step 1 failed: %w", err)
	}
	log.Println("✓ Step 1: Mint wallets created")

	// Step 2: Create primary transfer wallets
	if err := st.createTransferWallets(); err != nil {
		return fmt.Errorf("step 2 failed: %w", err)
	}
	log.Println("✓ Step 2: Primary transfer wallets created")

	// Step 2b: Create distribution wallets
	if err := st.createDistributionWallets(); err != nil {
		return fmt.Errorf("step 2b failed: %w", err)
	}
	log.Println("✓ Step 2b: Distribution wallets created")

	// Step 3: Create token
	if err := st.createToken(); err != nil {
		return fmt.Errorf("step 3 failed: %w", err)
	}
	log.Println("✓ Step 3: Token created")

	// Step 4: Grant mint authorities
	if err := st.grantMintAuthorities(); err != nil {
		return fmt.Errorf("step 4 failed: %w", err)
	}
	log.Println("✓ Step 4: Mint authorities granted")

	// Step 5: Perform concurrent minting with multi-tier transfers
	if err := st.performConcurrentMinting(); err != nil {
		return fmt.Errorf("step 5 failed: %w", err)
	}
	log.Println("✓ Step 5: Concurrent minting and transfers completed")

	log.Println("=== Multi-Tier Stress Test Completed Successfully! ===")
	log.Printf("Token Address: %s", st.tokenAddress)
	log.Printf("Total mint operations: %d", MINT_WALLETS_COUNT*WALLETS_PER_MINT)
	log.Printf("Total transfer operations: %d", TRANSFER_WALLETS_COUNT*TRANSFER_MULTIPLIER)
	log.Printf("Total tokens minted: %d", MINT_WALLETS_COUNT*WALLETS_PER_MINT*MINT_AMOUNT)
	log.Printf("Total tokens distributed: %d", TRANSFER_WALLETS_COUNT*TRANSFER_MULTIPLIER*TRANSFER_AMOUNT)

	return nil
}

// generateAccountsDetailCSV generates a CSV file with account details for all wallet tiers
func (st *StressTester) generateAccountsDetailCSV(timestamp string) error {
	csvFileName := fmt.Sprintf("accounts_detail_%s.csv", timestamp)

	// Create CSV file
	file, err := os.Create(csvFileName)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer file.Close()

	// Create CSV writer
	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write CSV header with additional columns for multi-tier tracking
	header := []string{"privatekey", "token_address", "decimal", "balance", "wallet_tier", "wallet_index", "source_wallet"}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	log.Printf("Generating multi-tier accounts detail CSV file: %s", csvFileName)
	totalWallets := len(st.transferWallets) + len(st.distributionWallets)
	log.Printf("Collecting balance information for %d total wallets (%d primary + %d distribution)...",
		totalWallets, len(st.transferWallets), len(st.distributionWallets))

	processedCount := 0

	// Write data for primary transfer wallets (tier 2)
	log.Printf("Processing primary transfer wallets...")
	for i, wallet := range st.transferWallets {
		// Get token account balance
		tokenAccount, err := st.client.GetTokenAccount(st.ctx, wallet.Address, st.tokenAddress)
		if err != nil {
			log.Printf("Warning: Failed to get balance for primary wallet %d (%s): %v", i+1, wallet.Address, err)
			// Continue with zero balance if account doesn't exist or has error
			tokenAccount = &onemoney.TokenAccountResponse{Balance: "0"}
		}

		// Prepare CSV row for primary wallet
		row := []string{
			wallet.PrivateKey,
			st.tokenAddress,
			strconv.Itoa(int(TOKEN_DECIMALS)),
			tokenAccount.Balance,
			"primary",
			strconv.Itoa(i + 1),
			"mint_wallet", // Primary wallets are funded by mint wallets
		}

		// Write row to CSV
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("failed to write CSV row for primary wallet %d: %w", i+1, err)
		}

		processedCount++
		// Log progress every 200 wallets
		if processedCount%200 == 0 {
			log.Printf("Processed %d/%d total wallets for CSV generation", processedCount, totalWallets)
		}
	}

	// Write data for distribution wallets (tier 3)
	log.Printf("Processing distribution wallets...")
	for i, wallet := range st.distributionWallets {
		// Get token account balance
		tokenAccount, err := st.client.GetTokenAccount(st.ctx, wallet.Address, st.tokenAddress)
		if err != nil {
			log.Printf("Warning: Failed to get balance for distribution wallet %d (%s): %v", i+1, wallet.Address, err)
			// Continue with zero balance if account doesn't exist or has error
			tokenAccount = &onemoney.TokenAccountResponse{Balance: "0"}
		}

		// Calculate which primary wallet funded this distribution wallet
		primaryWalletIndex := i / TRANSFER_MULTIPLIER
		sourceWallet := fmt.Sprintf("primary_wallet_%d", primaryWalletIndex+1)

		// Prepare CSV row for distribution wallet
		row := []string{
			wallet.PrivateKey,
			st.tokenAddress,
			strconv.Itoa(int(TOKEN_DECIMALS)),
			tokenAccount.Balance,
			"distribution",
			strconv.Itoa(i + 1),
			sourceWallet,
		}

		// Write row to CSV
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("failed to write CSV row for distribution wallet %d: %w", i+1, err)
		}

		processedCount++
		// Log progress every 500 wallets for distribution tier
		if processedCount%500 == 0 {
			log.Printf("Processed %d/%d total wallets for CSV generation", processedCount, totalWallets)
		}
	}

	log.Printf("Successfully generated multi-tier accounts detail CSV: %s", csvFileName)
	log.Printf("CSV contains %d total account records (%d primary + %d distribution)",
		totalWallets, len(st.transferWallets), len(st.distributionWallets))

	return nil
}

// TestBatchMint is the main test method that performs concurrent batch minting stress testing
func TestBatchMint(t *testing.T) {
	// Create log file with timestamp
	timestamp := time.Now().Format("20060102_150405")
	logFileName := fmt.Sprintf("stress_test_%s.log", timestamp)
	logFile, err := os.Create(logFileName)
	if err != nil {
		t.Fatalf("Failed to create log file: %v", err)
	}
	defer logFile.Close()

	// Create custom logger that writes to file
	fileLogger := log.New(logFile, "", log.LstdFlags|log.Lmicroseconds)

	// Redirect default log output to file
	originalLogOutput := log.Writer()
	log.SetOutput(logFile)
	defer log.SetOutput(originalLogOutput)

	// Helper function to log to both test output and file
	logToFile := func(format string, args ...interface{}) {
		message := fmt.Sprintf(format, args...)
		fileLogger.Println(message)
		t.Log(message) // Still log to test output for immediate feedback
	}

	logToFile("Initializing 1Money Batch Mint Stress Tester...")
	logToFile("Log file created: %s", logFileName)

	// Record overall test start time
	overallStartTime := time.Now()

	// Validate configuration
	if MINT_WALLETS_COUNT*WALLETS_PER_MINT != TRANSFER_WALLETS_COUNT {
		errorMsg := fmt.Sprintf("Configuration error: MINT_WALLETS_COUNT (%d) * WALLETS_PER_MINT (%d) must equal TRANSFER_WALLETS_COUNT (%d)",
			MINT_WALLETS_COUNT, WALLETS_PER_MINT, TRANSFER_WALLETS_COUNT)
		fileLogger.Println("FATAL: " + errorMsg)
		t.Fatal(errorMsg)
	}

	if TRANSFER_WALLETS_COUNT*TRANSFER_MULTIPLIER != DISTRIBUTION_WALLETS_COUNT {
		errorMsg := fmt.Sprintf("Configuration error: TRANSFER_WALLETS_COUNT (%d) * TRANSFER_MULTIPLIER (%d) must equal DISTRIBUTION_WALLETS_COUNT (%d)",
			TRANSFER_WALLETS_COUNT, TRANSFER_MULTIPLIER, DISTRIBUTION_WALLETS_COUNT)
		fileLogger.Println("FATAL: " + errorMsg)
		t.Fatal(errorMsg)
	}

	if MINT_AMOUNT%TRANSFER_MULTIPLIER != 0 {
		errorMsg := fmt.Sprintf("Configuration error: MINT_AMOUNT (%d) must be divisible by TRANSFER_MULTIPLIER (%d) for even distribution",
			MINT_AMOUNT, TRANSFER_MULTIPLIER)
		fileLogger.Println("FATAL: " + errorMsg)
		t.Fatal(errorMsg)
	}

	// Record tester creation time
	testerStartTime := time.Now()
	tester, err := NewStressTester()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to create stress tester: %v", err)
		fileLogger.Println("FATAL: " + errorMsg)
		t.Fatal(errorMsg)
	}
	testerCreationDuration := time.Since(testerStartTime)

	// Record stress test execution time
	stressTestStartTime := time.Now()
	if err := tester.RunStressTest(); err != nil {
		errorMsg := fmt.Sprintf("Batch mint stress test failed: %v", err)
		fileLogger.Println("FATAL: " + errorMsg)
		t.Fatal(errorMsg)
	}
	stressTestDuration := time.Since(stressTestStartTime)

	logToFile("Batch mint stress test completed successfully!")

	// Generate accounts detail CSV file
	csvStartTime := time.Now()
	logToFile("Generating accounts detail CSV file...")
	if err := tester.generateAccountsDetailCSV(timestamp); err != nil {
		errorMsg := fmt.Sprintf("Failed to generate accounts detail CSV: %v", err)
		fileLogger.Println("ERROR: " + errorMsg)
		t.Error(errorMsg) // Use t.Error instead of t.Fatal to continue with other reporting
	} else {
		logToFile("Accounts detail CSV file generated successfully!")
	}
	csvGenerationDuration := time.Since(csvStartTime)

	// Calculate total test duration including CSV generation
	totalDuration := time.Since(overallStartTime)

	// Detailed timing statistics
	logToFile("=== DETAILED TIMING STATISTICS ===")
	logToFile("Tester Creation Time: %v", testerCreationDuration)
	logToFile("Stress Test Execution Time: %v", stressTestDuration)
	logToFile("CSV Generation Time: %v", csvGenerationDuration)
	logToFile("Total Test Duration: %v", totalDuration)
	logToFile("")

	// Performance metrics
	totalMintOperations := MINT_WALLETS_COUNT * WALLETS_PER_MINT
	totalTransferOperations := TRANSFER_WALLETS_COUNT * TRANSFER_MULTIPLIER
	totalOperations := totalMintOperations + totalTransferOperations
	totalTokensMinted := totalMintOperations * MINT_AMOUNT
	totalTokensTransferred := totalTransferOperations * TRANSFER_AMOUNT

	logToFile("=== PERFORMANCE METRICS ===")
	logToFile("Total Mint Operations: %d", totalMintOperations)
	logToFile("Total Transfer Operations: %d", totalTransferOperations)
	logToFile("Total Combined Operations: %d", totalOperations)
	logToFile("Total Tokens Minted: %d", totalTokensMinted)
	logToFile("Total Tokens Transferred: %d", totalTokensTransferred)
	logToFile("Combined Operations per Second: %.2f", float64(totalOperations)/stressTestDuration.Seconds())
	logToFile("Mint Operations per Second: %.2f", float64(totalMintOperations)/stressTestDuration.Seconds())
	logToFile("Transfer Operations per Second: %.2f", float64(totalTransferOperations)/stressTestDuration.Seconds())
	logToFile("Tokens Processed per Second: %.2f", float64(totalTokensMinted+totalTokensTransferred)/stressTestDuration.Seconds())
	logToFile("Average Time per Combined Operation: %v", time.Duration(int64(stressTestDuration)/int64(totalOperations)))
	logToFile("")

	// Configuration summary
	logToFile("=== TEST CONFIGURATION ===")
	logToFile("Mint Wallets Count: %d", MINT_WALLETS_COUNT)
	logToFile("Primary Transfer Wallets Count: %d", TRANSFER_WALLETS_COUNT)
	logToFile("Distribution Wallets Count: %d", DISTRIBUTION_WALLETS_COUNT)
	logToFile("Transfer Multiplier: %d", TRANSFER_MULTIPLIER)
	logToFile("Transfer Workers Count: %d", TRANSFER_WORKERS_COUNT)
	logToFile("Wallets per Mint: %d", WALLETS_PER_MINT)
	logToFile("Mint Amount per Operation: %d", MINT_AMOUNT)
	logToFile("Transfer Amount per Operation: %d", TRANSFER_AMOUNT)
	logToFile("Token Symbol: %s", TOKEN_SYMBOL)
	logToFile("Chain ID: %d", CHAIN_ID)
	logToFile("")

	// Efficiency analysis
	setupTime := testerCreationDuration
	executionTime := stressTestDuration
	csvTime := csvGenerationDuration
	setupPercentage := (setupTime.Seconds() / totalDuration.Seconds()) * 100
	executionPercentage := (executionTime.Seconds() / totalDuration.Seconds()) * 100
	csvPercentage := (csvTime.Seconds() / totalDuration.Seconds()) * 100

	logToFile("=== EFFICIENCY ANALYSIS ===")
	logToFile("Setup Time: %v (%.1f%% of total)", setupTime, setupPercentage)
	logToFile("Execution Time: %v (%.1f%% of total)", executionTime, executionPercentage)
	logToFile("CSV Generation Time: %v (%.1f%% of total)", csvTime, csvPercentage)
	logToFile("Combined Throughput: %.2f operations/minute", float64(totalOperations)/(stressTestDuration.Minutes()))
	logToFile("Mint Throughput: %.2f mint operations/minute", float64(totalMintOperations)/(stressTestDuration.Minutes()))
	logToFile("Transfer Throughput: %.2f transfer operations/minute", float64(totalTransferOperations)/(stressTestDuration.Minutes()))
	logToFile("Token Distribution Efficiency: %.2f tokens/second", float64(totalTokensMinted+totalTokensTransferred)/stressTestDuration.Seconds())
	logToFile("Multi-tier Concurrency Factor: %.2fx", float64(totalOperations)/float64(totalMintOperations))
	logToFile("=== END OF TIMING STATISTICS ===")

	// Final log message with file location
	logToFile("All logs have been written to: %s", logFileName)
	t.Logf("Complete multi-tier stress test logs saved to: %s", logFileName)
}

// main function for standalone execution
func main() {
	log.Println("1Money Multi-Tier Concurrent Token Distribution Stress Testing Tool")
	log.Println("===================================================================")
	log.Println("This enhanced tool implements a multi-tier concurrent token distribution strategy:")
	log.Println("- Tier 1: 20 mint wallets (operators)")
	log.Println("- Tier 2: 2000 primary transfer wallets (receive minted tokens)")
	log.Println("- Tier 3: 8000 distribution wallets (receive transferred tokens)")
	log.Println()
	log.Println("Key Features:")
	log.Println("- Concurrent minting and transferring operations")
	log.Println("- Configurable transfer multiplier (currently 4)")
	log.Println("- Multi-threaded transfer workers (10 concurrent workers)")
	log.Println("- Comprehensive CSV output with all wallet tiers")
	log.Println("- Enhanced timing statistics and performance metrics")
	log.Println()
	log.Println("Usage:")
	log.Println("  go test -v -run TestBatchMint")
	log.Println()
	log.Println("Configuration Constants (easily modifiable):")
	log.Printf("  TRANSFER_MULTIPLIER: %d (distribution wallets per primary wallet)\n", TRANSFER_MULTIPLIER)
	log.Printf("  TRANSFER_WORKERS_COUNT: %d (concurrent transfer workers)\n", TRANSFER_WORKERS_COUNT)
	log.Printf("  MINT_AMOUNT: %d (tokens minted per operation)\n", MINT_AMOUNT)
	log.Printf("  TRANSFER_AMOUNT: %d (tokens transferred per distribution)\n", TRANSFER_AMOUNT)
	log.Println()

	// Example of running the stress test directly (uncomment to use)
	/*
		tester, err := NewStressTester()
		if err != nil {
			log.Fatalf("Failed to create stress tester: %v", err)
		}

		if err := tester.RunStressTest(); err != nil {
			log.Fatalf("Multi-tier stress test failed: %v", err)
		}

		log.Println("Multi-tier stress test completed successfully!")
	*/
}
