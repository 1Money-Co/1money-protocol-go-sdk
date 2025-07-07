package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log"
	"math/big"
	"os"
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
	MINT_WALLETS_COUNT     = 20   // Number of mint authority wallets
	TRANSFER_WALLETS_COUNT = 2000 // Number of transfer recipient wallets
	WALLETS_PER_MINT       = 100  // Number of transfer wallets per mint wallet (should equal TRANSFER_WALLETS_COUNT / MINT_WALLETS_COUNT)

	// Token Configuration
	TOKEN_SYMBOL   = "STRESS3"
	TOKEN_NAME     = "Stress Test Token"
	TOKEN_DECIMALS = 6
	CHAIN_ID       = 1212101

	// Mint Configuration
	MINT_ALLOWANCE = 1000000000 // Allowance granted to each mint wallet
	MINT_AMOUNT    = 1000       // Amount to mint per operation

	// Transaction Validation Configuration
	RECEIPT_CHECK_TIMEOUT    = 60 * time.Second       // Timeout for waiting for transaction receipt
	RECEIPT_CHECK_INTERVAL   = 250 * time.Millisecond // Interval between receipt checks
	NONCE_VALIDATION_TIMEOUT = 30 * time.Second       // Timeout for nonce validation
	NONCE_CHECK_INTERVAL     = 250 * time.Millisecond // Interval between nonce checks
)

// Wallet represents a wallet with private key, public key, and address
type Wallet struct {
	PrivateKey string
	PublicKey  string
	Address    string
}

// StressTester manages the stress testing operations
type StressTester struct {
	client          *onemoney.Client
	operatorWallet  *Wallet
	mintWallets     []*Wallet
	transferWallets []*Wallet
	tokenAddress    string
	ctx             context.Context
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

// Step 2: Create transfer wallets
func (st *StressTester) createTransferWallets() error {
	log.Printf("Creating %d transfer wallets...", TRANSFER_WALLETS_COUNT)

	st.transferWallets = make([]*Wallet, TRANSFER_WALLETS_COUNT)
	for i := 0; i < TRANSFER_WALLETS_COUNT; i++ {
		wallet, err := generateWallet()
		if err != nil {
			return fmt.Errorf("failed to create transfer wallet %d: %w", i, err)
		}
		st.transferWallets[i] = wallet
		log.Printf("Created transfer wallet %d: %s", i+1, wallet.Address)
	}

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

// Step 5: Perform concurrent minting operations
func (st *StressTester) performConcurrentMinting() error {
	log.Println("Starting concurrent minting operations...")

	var wg sync.WaitGroup
	errorChan := make(chan error, MINT_WALLETS_COUNT)

	// Launch one goroutine per mint wallet
	for i, mintWallet := range st.mintWallets {
		wg.Add(1)
		go func(walletIndex int, wallet *Wallet) {
			defer wg.Done()

			// Calculate the range of transfer wallets this mint wallet is responsible for
			startIdx := walletIndex * WALLETS_PER_MINT
			endIdx := startIdx + WALLETS_PER_MINT
			if endIdx > len(st.transferWallets) {
				endIdx = len(st.transferWallets)
			}

			log.Printf("Mint wallet %d (%s) minting to wallets %d-%d",
				walletIndex+1, wallet.Address, startIdx+1, endIdx)

			// Perform mint operations to assigned transfer wallets sequentially
			for j := startIdx; j < endIdx; j++ {
				transferWallet := st.transferWallets[j]

				if err := st.mintToWallet(wallet, transferWallet, walletIndex+1, j+1); err != nil {
					errorChan <- fmt.Errorf("mint wallet %d failed to mint to transfer wallet %d: %w",
						walletIndex+1, j+1, err)
					return
				}
			}

			log.Printf("Mint wallet %d completed all minting operations", walletIndex+1)
		}(i, mintWallet)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errorChan)

	// Check for any errors
	for err := range errorChan {
		return err
	}

	log.Println("All concurrent minting operations completed successfully!")
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
	log.Println("=== Starting 1Money Batch Mint Stress Test ===")
	log.Printf("Configuration:")
	log.Printf("- Mint wallets: %d", MINT_WALLETS_COUNT)
	log.Printf("- Transfer wallets: %d", TRANSFER_WALLETS_COUNT)
	log.Printf("- Wallets per mint: %d", WALLETS_PER_MINT)
	log.Printf("- Mint allowance: %d", MINT_ALLOWANCE)
	log.Printf("- Mint amount per operation: %d", MINT_AMOUNT)
	log.Printf("- Token symbol: %s", TOKEN_SYMBOL)
	log.Printf("- Token name: %s", TOKEN_NAME)
	log.Printf("- Chain ID: %d", CHAIN_ID)
	log.Println()

	// Step 1: Create mint wallets
	if err := st.createMintWallets(); err != nil {
		return fmt.Errorf("step 1 failed: %w", err)
	}
	log.Println("✓ Step 1: Mint wallets created")

	// Step 2: Create transfer wallets
	if err := st.createTransferWallets(); err != nil {
		return fmt.Errorf("step 2 failed: %w", err)
	}
	log.Println("✓ Step 2: Transfer wallets created")

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

	// Step 5: Perform concurrent minting
	if err := st.performConcurrentMinting(); err != nil {
		return fmt.Errorf("step 5 failed: %w", err)
	}
	log.Println("✓ Step 5: Concurrent minting completed")

	log.Println("=== Stress Test Completed Successfully! ===")
	log.Printf("Token Address: %s", st.tokenAddress)
	log.Printf("Total mint operations: %d", MINT_WALLETS_COUNT*WALLETS_PER_MINT)
	log.Printf("Total tokens minted: %d", MINT_WALLETS_COUNT*WALLETS_PER_MINT*MINT_AMOUNT)

	return nil
}

// TestBatchMint is the main test method that performs concurrent batch minting stress testing
func TestBatchMint(t *testing.T) {
	t.Log("Initializing 1Money Batch Mint Stress Tester...")

	// Record overall test start time
	overallStartTime := time.Now()

	// Validate configuration
	if MINT_WALLETS_COUNT*WALLETS_PER_MINT != TRANSFER_WALLETS_COUNT {
		t.Fatalf("Configuration error: MINT_WALLETS_COUNT (%d) * WALLETS_PER_MINT (%d) must equal TRANSFER_WALLETS_COUNT (%d)",
			MINT_WALLETS_COUNT, WALLETS_PER_MINT, TRANSFER_WALLETS_COUNT)
	}

	// Record tester creation time
	testerStartTime := time.Now()
	tester, err := NewStressTester()
	if err != nil {
		t.Fatalf("Failed to create stress tester: %v", err)
	}
	testerCreationDuration := time.Since(testerStartTime)

	// Record stress test execution time
	stressTestStartTime := time.Now()
	if err := tester.RunStressTest(); err != nil {
		t.Fatalf("Batch mint stress test failed: %v", err)
	}
	stressTestDuration := time.Since(stressTestStartTime)

	// Calculate overall test duration
	overallDuration := time.Since(overallStartTime)

	t.Log("Batch mint stress test completed successfully!")

	// Detailed timing statistics
	t.Log("=== DETAILED TIMING STATISTICS ===")
	t.Logf("Tester Creation Time: %v", testerCreationDuration)
	t.Logf("Stress Test Execution Time: %v", stressTestDuration)
	t.Logf("Total Test Duration: %v", overallDuration)
	t.Log("")

	// Performance metrics
	totalOperations := MINT_WALLETS_COUNT * WALLETS_PER_MINT
	totalTokensMinted := totalOperations * MINT_AMOUNT

	t.Log("=== PERFORMANCE METRICS ===")
	t.Logf("Total Mint Operations: %d", totalOperations)
	t.Logf("Total Tokens Minted: %d", totalTokensMinted)
	t.Logf("Operations per Second: %.2f", float64(totalOperations)/stressTestDuration.Seconds())
	t.Logf("Tokens Minted per Second: %.2f", float64(totalTokensMinted)/stressTestDuration.Seconds())
	t.Logf("Average Time per Operation: %v", time.Duration(int64(stressTestDuration)/int64(totalOperations)))
	t.Log("")

	// Configuration summary
	t.Log("=== TEST CONFIGURATION ===")
	t.Logf("Mint Wallets Count: %d", MINT_WALLETS_COUNT)
	t.Logf("Transfer Wallets Count: %d", TRANSFER_WALLETS_COUNT)
	t.Logf("Wallets per Mint: %d", WALLETS_PER_MINT)
	t.Logf("Mint Amount per Operation: %d", MINT_AMOUNT)
	t.Logf("Token Symbol: %s", TOKEN_SYMBOL)
	t.Logf("Chain ID: %d", CHAIN_ID)
	t.Log("")

	// Efficiency analysis
	setupTime := testerCreationDuration
	executionTime := stressTestDuration
	setupPercentage := (setupTime.Seconds() / overallDuration.Seconds()) * 100
	executionPercentage := (executionTime.Seconds() / overallDuration.Seconds()) * 100

	t.Log("=== EFFICIENCY ANALYSIS ===")
	t.Logf("Setup Time: %v (%.1f%% of total)", setupTime, setupPercentage)
	t.Logf("Execution Time: %v (%.1f%% of total)", executionTime, executionPercentage)
	t.Logf("Throughput: %.2f operations/minute", float64(totalOperations)/(stressTestDuration.Minutes()))
	t.Log("=== END OF TIMING STATISTICS ===")
}

// main function for standalone execution
func main() {
	log.Println("1Money Batch Mint Stress Testing Tool")
	log.Println("=====================================")
	log.Println("This tool can be run as a Go test using: go test -v -run TestBatchMint")
	log.Println("Or you can run the stress test directly by calling RunStressTest()")
	log.Println()

	// Example of running the stress test directly (uncomment to use)
	/*
		tester, err := NewStressTester()
		if err != nil {
			log.Fatalf("Failed to create stress tester: %v", err)
		}

		if err := tester.RunStressTest(); err != nil {
			log.Fatalf("Batch mint stress test failed: %v", err)
		}

		log.Println("Batch mint stress test completed successfully!")
	*/
}
