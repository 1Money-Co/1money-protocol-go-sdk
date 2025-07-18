package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/big"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	onemoney "github.com/1Money-Co/1money-go-sdk"
	"github.com/ethereum/go-ethereum/common"
)

var (
	nodeList = flag.String("nodes", "", "Comma-separated list of node URLs (e.g. '127.0.0.1:18555,127.0.0.1:18556')")
	postRate = flag.Int("post-rate", POST_RATE_LIMIT_TPS, "Total POST rate limit in TPS")
	getRate  = flag.Int("get-rate", GET_RATE_LIMIT_TPS, "Total GET rate limit in TPS")
	csvRate  = flag.Int("csv-rate", CSV_BALANCE_QUERY_RATE_LIMIT, "Balance query rate limit for CSV generation in QPS")
)

// ParseNodeURLs parses comma-separated node URLs and ensures they have http:// prefix
func ParseNodeURLs(nodeListStr string) ([]string, error) {
	if nodeListStr == "" {
		return nil, fmt.Errorf("node list cannot be empty")
	}

	urls := strings.Split(nodeListStr, ",")
	parsedURLs := make([]string, 0, len(urls))

	for _, url := range urls {
		url = strings.TrimSpace(url)
		if url == "" {
			continue
		}

		// Add http:// prefix if not present
		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			url = "http://" + url
		}

		parsedURLs = append(parsedURLs, url)
	}

	if len(parsedURLs) == 0 {
		return nil, fmt.Errorf("no valid URLs found in node list")
	}

	return parsedURLs, nil
}

// getInitialNonce gets the initial nonce for an address using the node pool
func getInitialNonce(nodePool *NodePool, address string) (uint64, error) {
	if nodePool == nil {
		return 0, fmt.Errorf("node pool is nil")
	}
	if address == "" {
		return 0, fmt.Errorf("address is empty")
	}

	client, _, _, err := nodePool.GetNodeForGet()
	if err != nil {
		return 0, fmt.Errorf("failed to get node for initial nonce check: %w", err)
	}
	if client == nil {
		return 0, fmt.Errorf("client is nil")
	}

	accountNonce, err := client.GetAccountNonce(context.Background(), address)
	if err != nil {
		log.Printf("❌ API ERROR: GetAccountNonce failed | Purpose: Initial nonce fetch | Address: %s | Error: %v", address, err)
		return 0, fmt.Errorf("failed to get initial nonce for %s: %w", address, err)
	}
	if accountNonce == nil {
		return 0, fmt.Errorf("account nonce response is nil")
	}

	return accountNonce.Nonce, nil
}

// getNextOperatorNonce returns the next nonce for the operator wallet in a thread-safe manner
func (st *StressTester) getNextOperatorNonce() (uint64, error) {
	st.operatorNonceMutex.Lock()
	defer st.operatorNonceMutex.Unlock()

	// Always get current nonce from blockchain to ensure accuracy
	currentNonce, err := st.getAccountNonce(st.operatorWallet.Address)
	if err != nil {
		return 0, fmt.Errorf("failed to get current operator nonce: %w", err)
	}

	return currentNonce, nil
}

// verifyNonceIncrement verifies that the operator wallet nonce has incremented to the expected value
func (st *StressTester) verifyNonceIncrement(expectedNonce uint64, walletIndex int) error {
	// Poll for nonce increment with timeout
	maxRetries := NONCE_VERIFY_MAX_RETRIES
	retryInterval := NONCE_VERIFY_INTERVAL

	for retry := 0; retry < maxRetries; retry++ {
		currentNonce, err := st.getAccountNonce(st.operatorWallet.Address)
		if err != nil {
			return fmt.Errorf("failed to get current nonce during verification: %w", err)
		}

		if currentNonce == expectedNonce {
			return nil
		}

		if currentNonce > expectedNonce {
			return fmt.Errorf("nonce jumped unexpectedly: expected %d, got %d", expectedNonce, currentNonce)
		}

		// Nonce hasn't incremented yet, wait and retry
		// Log only at 50% and max retries
		if retry == maxRetries/2 || retry == maxRetries-1 {
			log.Printf("Waiting for nonce %d→%d (retry %d/%d)", currentNonce, expectedNonce, retry+1, maxRetries)
		}
		time.Sleep(retryInterval)
	}

	// Final check to get the actual nonce for error message
	finalNonce, _ := st.getAccountNonce(st.operatorWallet.Address)
	return fmt.Errorf("nonce verification timeout: expected %d, final nonce: %d", expectedNonce, finalNonce)
}

// NewStressTester creates a new stress tester instance
func NewStressTester(nodeURLs []string, totalPostRate int, totalGetRate int, csvRateLimit int) (*StressTester, error) {
	// Create node pool
	nodePool := NewNodePool()

	// Add all nodes
	for _, url := range nodeURLs {
		if err := nodePool.AddNode(url); err != nil {
			return nil, fmt.Errorf("failed to add node %s: %w", url, err)
		}
	}

	log.Printf("Created node pool with %d nodes", nodePool.Size())

	// Get operator wallet configuration
	privateKey, address, err := getOperatorConfig()
	if err != nil {
		return nil, err
	}

	operatorWallet := &Wallet{
		PrivateKey: privateKey,
		Address:    address,
	}

	// Create multi-node rate limiter
	rateLimiter := NewMultiNodeRateLimiter(nodeURLs, totalPostRate, totalGetRate)

	// Initialize operator nonce
	initialNonce, err := getInitialNonce(nodePool, operatorWallet.Address)
	if err != nil {
		return nil, fmt.Errorf("failed to get initial operator nonce: %w", err)
	}

	return &StressTester{
		nodePool:       nodePool,
		operatorWallet: operatorWallet,
		ctx:            context.Background(),
		rateLimiter:    rateLimiter,
		operatorNonce:  initialNonce,
		csvRateLimit:   csvRateLimit,
	}, nil
}

// getAccountNonce gets account nonce using node pool
func (st *StressTester) getAccountNonce(address string) (uint64, error) {
	if st == nil {
		return 0, fmt.Errorf("stress tester is nil")
	}
	if address == "" {
		return 0, fmt.Errorf("address is empty")
	}

	// Get a node for GET operation
	client, _, nodeIndex, err := st.nodePool.GetNodeForGet()
	if err != nil {
		return 0, fmt.Errorf("failed to get node for nonce check: %w", err)
	}
	if client == nil {
		return 0, fmt.Errorf("client is nil")
	}

	// Get rate limiter for this node
	nodeRateLimiter := st.rateLimiter.GetNodeRateLimiter(nodeIndex)
	if nodeRateLimiter == nil {
		return 0, fmt.Errorf("no rate limiter for node %d", nodeIndex)
	}

	// Apply rate limiting for GET request
	if err := nodeRateLimiter.WaitForGetToken(st.ctx); err != nil {
		return 0, fmt.Errorf("rate limiting failed for GetAccount: %w", err)
	}

	accountNonce, err := client.GetAccountNonce(st.ctx, address)
	if err != nil {
		// Failed to get nonce
		log.Printf("❌ API ERROR: GetAccountNonce failed | Address: %s | Node: %d | Error: %v", address, nodeIndex, err)
		return 0, err
	}
	if accountNonce == nil {
		return 0, fmt.Errorf("account nonce response is nil for address %s", address)
	}

	return accountNonce.Nonce, nil
}

// waitForTransactionReceipt waits for transaction receipt using node pool
func (st *StressTester) waitForTransactionReceipt(txHash string, fromAddress string, toAddress string, operationType string) error {

	retryCount := 0
	maxRetries := 120 // Maximum 120 retries (about 60 seconds with 500ms intervals)
	for {
		// Get a node for GET operation
		client, _, nodeIndex, err := st.nodePool.GetNodeForGet()
		if err != nil {
			return fmt.Errorf("failed to get node for receipt check: %w", err)
		}

		// Get rate limiter for this node
		nodeRateLimiter := st.rateLimiter.GetNodeRateLimiter(nodeIndex)
		if nodeRateLimiter == nil {
			return fmt.Errorf("no rate limiter for node %d", nodeIndex)
		}

		// Apply rate limiting for GET request
		if err := nodeRateLimiter.WaitForGetToken(st.ctx); err != nil {
			return fmt.Errorf("rate limiting failed for GetTransactionReceipt: %w", err)
		}

		receipt, err := client.GetTransactionReceipt(st.ctx, txHash)
		if err != nil {
			retryCount++
			if retryCount >= maxRetries {
				log.Printf("❌ API ERROR: GetTransactionReceipt timeout | TxHash: %s | From: %s | To: %s | Node: %d | Retry: %d/%d | Error: %v", txHash, fromAddress, toAddress, nodeIndex, retryCount, maxRetries, err)
				return fmt.Errorf("transaction receipt timeout after %d retries", maxRetries)
			}
			time.Sleep(500 * time.Millisecond)
			continue
		}

		if receipt.Success {
			// Transaction confirmed
			log.Printf("✅ Transaction confirmed | TxHash: %s | Operation: %s | From: %s | To: %s", txHash, operationType, fromAddress, toAddress)
			return nil
		} else {
			log.Printf("❌ Transaction failed | TxHash: %s | Operation: %s | From: %s | To: %s", txHash, operationType, fromAddress, toAddress)
			return fmt.Errorf("transaction failed: %s", txHash)
		}
	}
}

// validateNonceIncrement validates nonce increment using node pool
func (st *StressTester) validateNonceIncrement(address string, expectedNonce uint64, walletType string, operationType string) error {

	retryCount := 0
	maxRetries := 40 // Maximum 80 retries (about 40 seconds with 500ms intervals)
	for {
		// Get a node for GET operation
		client, _, nodeIndex, err := st.nodePool.GetNodeForGet()
		if err != nil {
			return fmt.Errorf("failed to get node for nonce validation: %w", err)
		}

		// Get rate limiter for this node
		nodeRateLimiter := st.rateLimiter.GetNodeRateLimiter(nodeIndex)
		if nodeRateLimiter == nil {
			return fmt.Errorf("no rate limiter for node %d", nodeIndex)
		}

		// Apply rate limiting for GET request
		if err := nodeRateLimiter.WaitForGetToken(st.ctx); err != nil {
			return fmt.Errorf("rate limiting failed for GetAccount: %w", err)
		}

		accountNonce, err := client.GetAccountNonce(st.ctx, address)
		if err != nil {
			retryCount++
			if retryCount >= maxRetries {
				log.Printf("❌ API ERROR: GetAccountNonce validation timeout | Address: %s | Expected: %d | Node: %d | Retry: %d/%d | Error: %v", address, expectedNonce, nodeIndex, retryCount, maxRetries, err)
				return fmt.Errorf("failed to get nonce after %d retries: %w", maxRetries, err)
			}
			time.Sleep(500 * time.Millisecond)
			continue
		}

		if accountNonce.Nonce == expectedNonce {
			return nil
		}

		if accountNonce.Nonce > expectedNonce {
			return fmt.Errorf("nonce jumped to %d, expected %d", accountNonce.Nonce, expectedNonce)
		}

		retryCount++
		if retryCount >= maxRetries {
			return fmt.Errorf("nonce validation timeout after %d retries: current %d, expected %d",
				maxRetries, accountNonce.Nonce, expectedNonce)
		}

		// Log only at 50% and max retries
		if retryCount == maxRetries/2 || retryCount == maxRetries-1 {
			log.Printf("Nonce wait: %d→%d (retry %d/%d)", accountNonce.Nonce, expectedNonce, retryCount, maxRetries)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// Step 1: Create mint wallets
func (st *StressTester) createMintWallets() error {
	log.Printf("Creating %d mint wallets...", MINT_WALLETS_COUNT)

	st.mintWallets = make([]*Wallet, MINT_WALLETS_COUNT)
	for i := 0; i < MINT_WALLETS_COUNT; i++ {
		wallet, err := generateDeterministicWallet("mint", i)
		if err != nil {
			return fmt.Errorf("failed to create mint wallet %d: %w", i, err)
		}
		st.mintWallets[i] = wallet

		// Log progress every 10 wallets to avoid excessive logging
		if (i+1)%10 == 0 {
			// Log progress every 25%
			if (i+1)%(MINT_WALLETS_COUNT/4) == 0 || i+1 == MINT_WALLETS_COUNT {
				log.Printf("Mint wallets: %d/%d", i+1, MINT_WALLETS_COUNT)
			}
		}
	}

	log.Printf("✓ Created %d mint wallets", MINT_WALLETS_COUNT)
	return nil
}

// Step 2: Create transfer wallets (primary tier)
func (st *StressTester) createTransferWallets() error {
	log.Printf("Creating %d transfer wallets...", TRANSFER_WALLETS_COUNT)

	st.transferWallets = make([]*Wallet, TRANSFER_WALLETS_COUNT)
	for i := 0; i < TRANSFER_WALLETS_COUNT; i++ {
		wallet, err := generateDeterministicWallet("transfer", i)
		if err != nil {
			return fmt.Errorf("failed to create primary transfer wallet %d: %w", i, err)
		}
		st.transferWallets[i] = wallet
	}

	log.Printf("✓ Created %d transfer wallets", TRANSFER_WALLETS_COUNT)
	return nil
}

// createDistributionWallets creates distribution wallets (tier 3)
func (st *StressTester) createDistributionWallets() error {
	log.Printf("Creating %d distribution wallets...", TOTAL_DIST_WALLETS)

	st.distributionWallets = make([]*Wallet, TOTAL_DIST_WALLETS)
	for i := 0; i < TOTAL_DIST_WALLETS; i++ {
		wallet, err := generateDeterministicWallet("distribution", i)
		if err != nil {
			return fmt.Errorf("failed to create distribution wallet %d: %w", i, err)
		}
		st.distributionWallets[i] = wallet
	}

	log.Printf("✓ Created %d distribution wallets", TOTAL_DIST_WALLETS)
	return nil
}

// createToken creates token using operator wallet
func (st *StressTester) createToken() error {
	log.Printf("Creating token %s...", GetTokenSymbol())

	// Get next nonce for operator wallet
	nonce, err := st.getNextOperatorNonce()
	if err != nil {
		return err
	}

	// Get a node for POST operation (token creation)
	client, _, nodeIndex, err := st.nodePool.GetNodeForMint() // Using mint counter for token operations
	if err != nil {
		return fmt.Errorf("failed to get node for token creation: %w", err)
	}

	tokenSymbol := GetTokenSymbol()
	payload := onemoney.TokenIssuePayload{
		ChainID:         CHAIN_ID,
		Nonce:           nonce,
		Symbol:          tokenSymbol,
		Name:            TOKEN_NAME,
		Decimals:        TOKEN_DECIMALS,
		MasterAuthority: common.HexToAddress(st.operatorWallet.Address),
		IsPrivate:       false,
	}

	// Token creation payload

	signature, err := client.SignMessage(payload, st.operatorWallet.PrivateKey)
	if err != nil {
		log.Printf("❌ SIGNING ERROR: Token creation signature failed | Symbol: %s | Operator: %s | Nonce: %d | Node: %d | Error: %v", tokenSymbol, st.operatorWallet.Address, nonce, nodeIndex, err)
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

	// Get rate limiter for this node
	nodeRateLimiter := st.rateLimiter.GetNodeRateLimiter(nodeIndex)
	if nodeRateLimiter == nil {
		return fmt.Errorf("no rate limiter for node %d", nodeIndex)
	}

	// Apply rate limiting for POST request
	if err := nodeRateLimiter.WaitForPostToken(st.ctx); err != nil {
		return fmt.Errorf("rate limiting failed for IssueToken: %w", err)
	}

	result, err := client.IssueToken(st.ctx, req)
	if err != nil {
		log.Printf("❌ API ERROR: IssueToken failed | Symbol: %s | Operator: %s | Nonce: %d | Allowance: %d | Node: %d | Error: %v", tokenSymbol, st.operatorWallet.Address, nonce, MINT_ALLOWANCE, nodeIndex, err)
		return fmt.Errorf("failed to issue token: %w", err)
	}

	st.tokenAddress = result.Token
	// Token submission logged internally

	// Wait for transaction confirmation
	if err := st.waitForTransactionReceipt(result.Hash, st.operatorWallet.Address, st.tokenAddress, "TOKEN_CREATE"); err != nil {
		log.Printf("Error: Token creation timeout: %v", err)
		return fmt.Errorf("failed to confirm token creation: %w", err)
	}

	// Note: Nonce management is now handled internally, no need to validate increment

	log.Printf("✓ Token %s created at %s", tokenSymbol, st.tokenAddress)

	return nil
}

// grantMintAuthorities grants mint permissions sequentially (single-threaded)
func (st *StressTester) grantMintAuthorities() error {
	log.Printf("Granting mint authorities to %d wallets...", len(st.mintWallets))

	// Get initial nonce to track progress
	initialNonce, err := st.getAccountNonce(st.operatorWallet.Address)
	if err != nil {
		return fmt.Errorf("failed to get initial nonce: %w", err)
	}

	// Initial nonce: initialNonce

	// Grant authority to each wallet sequentially
	for i, mintWallet := range st.mintWallets {
		// Processing wallet i+1

		// Grant authority to this wallet
		if err := st.grantSingleMintAuthority(i, mintWallet); err != nil {
			return fmt.Errorf("failed to grant authority to wallet %d (%s): %w", i+1, mintWallet.Address, err)
		}

		// Verify nonce has incremented correctly
		expectedNonce := initialNonce + uint64(i+1)
		if err := st.verifyNonceIncrement(expectedNonce, i+1); err != nil {
			return fmt.Errorf("nonce verification failed after granting authority to wallet %d: %w", i+1, err)
		}

		if i+1 == len(st.mintWallets)/2 || i+1 == len(st.mintWallets) {
			log.Printf("Granted authorities: %d/%d", i+1, len(st.mintWallets))
		}
	}

	log.Printf("✓ Granted %d mint authorities", len(st.mintWallets))
	return nil
}

// grantSingleMintAuthority grants mint authority to a single wallet
func (st *StressTester) grantSingleMintAuthority(walletIndex int, mintWallet *Wallet) error {
	// Granting authority to wallet walletIndex+1

	// Get next nonce for operator wallet
	nonce, err := st.getNextOperatorNonce()
	if err != nil {
		return err
	}

	// Get a node for POST operation
	client, _, nodeIndex, err := st.nodePool.GetNodeForMint()
	if err != nil {
		return fmt.Errorf("failed to get node for authority grant: %w", err)
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

	// Authority grant payload

	signature, err := client.SignMessage(payload, st.operatorWallet.PrivateKey)
	if err != nil {
		log.Printf("❌ SIGNING ERROR: Authority grant signature failed | MintWallet: %d (%s) | Operator: %s | Nonce: %d | Allowance: %d | Token: %s | Node: %d | Error: %v", walletIndex+1, mintWallet.Address, st.operatorWallet.Address, nonce, MINT_ALLOWANCE, st.tokenAddress, nodeIndex, err)
		return fmt.Errorf("failed to sign authority grant for wallet %d: %w", walletIndex, err)
	}

	req := &onemoney.TokenAuthorityRequest{
		TokenAuthorityPayload: payload,
		Signature: onemoney.Signature{
			R: signature.R,
			S: signature.S,
			V: signature.V,
		},
	}

	// Get rate limiter for this node
	nodeRateLimiter := st.rateLimiter.GetNodeRateLimiter(nodeIndex)
	if nodeRateLimiter == nil {
		return fmt.Errorf("no rate limiter for node %d", nodeIndex)
	}

	// Apply rate limiting for POST request
	if err := nodeRateLimiter.WaitForPostToken(st.ctx); err != nil {
		return fmt.Errorf("rate limiting failed for GrantTokenAuthority: %w", err)
	}

	result, err := client.GrantTokenAuthority(st.ctx, req)
	if err != nil {
		log.Printf("❌ API ERROR: GrantTokenAuthority failed | MintWallet: %d (%s) | Operator: %s | Nonce: %d | Allowance: %d | Token: %s | Node: %d | Error: %v", walletIndex+1, mintWallet.Address, st.operatorWallet.Address, nonce, MINT_ALLOWANCE, st.tokenAddress, nodeIndex, err)
		return fmt.Errorf("failed to grant authority to wallet %d: %w", walletIndex, err)
	}

	// Authority grant in progress

	// Wait for transaction confirmation
	if err := st.waitForTransactionReceipt(result.Hash, st.operatorWallet.Address, mintWallet.Address, "AUTHORITY_GRANT"); err != nil {
		log.Printf("Error: Authority grant timeout (wallet %d): %v", walletIndex+1, err)
		return fmt.Errorf("failed to confirm authority grant for wallet %d: %w", walletIndex, err)
	}

	// Note: Nonce management is now handled internally, no need to validate increment

	// Authority granted to wallet walletIndex+1

	return nil
}

// mintToWallet performs a single mint operation
func (st *StressTester) mintToWallet(mintWallet, transferWallet *Wallet, mintWalletIndex, transferWalletIndex int) error {
	totalMints := int64(MINT_WALLETS_COUNT * WALLETS_PER_MINT)

	// Get mint wallet's current nonce
	nonce, err := st.getAccountNonce(mintWallet.Address)
	if err != nil {
		return err
	}

	// Get a node for POST operation
	client, _, nodeIndex, err := st.nodePool.GetNodeForMint()
	if err != nil {
		return fmt.Errorf("failed to get node for mint operation: %w", err)
	}

	// Create mint payload
	payload := onemoney.TokenMintPayload{
		ChainID:   CHAIN_ID,
		Nonce:     nonce,
		Recipient: common.HexToAddress(transferWallet.Address),
		Value:     big.NewInt(MINT_AMOUNT),
		Token:     common.HexToAddress(st.tokenAddress),
	}

	// Mint payload

	// Sign the payload
	signature, err := client.SignMessage(payload, mintWallet.PrivateKey)
	if err != nil {
		log.Printf("❌ SIGNING ERROR: Mint transaction signature failed | MintWallet: %d (%s) | TargetWallet: %d (%s) | Nonce: %d | Amount: %d | Token: %s | Node: %d | Error: %v", mintWalletIndex, mintWallet.Address, transferWalletIndex, transferWallet.Address, nonce, MINT_AMOUNT, st.tokenAddress, nodeIndex, err)
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

	// Get rate limiter for this node
	nodeRateLimiter := st.rateLimiter.GetNodeRateLimiter(nodeIndex)
	if nodeRateLimiter == nil {
		return fmt.Errorf("no rate limiter for node %d", nodeIndex)
	}

	// Apply rate limiting for POST request
	if err := nodeRateLimiter.WaitForPostToken(st.ctx); err != nil {
		return fmt.Errorf("rate limiting failed for MintToken: %w", err)
	}

	// Send mint request
	result, err := client.MintToken(st.ctx, req)
	if err != nil {
		log.Printf("❌ API ERROR: MintToken failed | MintWallet: %d (%s) | TargetWallet: %d (%s) | Nonce: %d | Amount: %d | Token: %s | Node: %d | Error: %v", mintWalletIndex, mintWallet.Address, transferWalletIndex, transferWallet.Address, nonce, MINT_AMOUNT, st.tokenAddress, nodeIndex, err)
		return fmt.Errorf("failed to mint token: %w", err)
	}

	// Wait for transaction confirmation
	if err := st.waitForTransactionReceipt(result.Hash, mintWallet.Address, transferWallet.Address, "MINT"); err != nil {
		log.Printf("❌ MINT TIMEOUT: Mint wallet %d → Transfer wallet %d | TxHash: %s | MintAddr: %s | TargetAddr: %s | Amount: %d | Nonce: %d | Error: %v", mintWalletIndex, transferWalletIndex, result.Hash, mintWallet.Address, transferWallet.Address, MINT_AMOUNT, nonce, err)
		return fmt.Errorf("failed to confirm mint transaction: %w", err)
	}

	// Validate nonce increment to ensure transaction was confirmed
	if err := st.validateNonceIncrement(mintWallet.Address, nonce+1, "MINT_WALLET", "MINT"); err != nil {
		log.Printf("❌ NONCE VALIDATION FAILED: Mint wallet %d → Transfer wallet %d | TxHash: %s | MintAddr: %s | TargetAddr: %s | Amount: %d | ExpectedNonce: %d | Error: %v", mintWalletIndex, transferWalletIndex, result.Hash, mintWallet.Address, transferWallet.Address, MINT_AMOUNT, nonce+1, err)
		return fmt.Errorf("failed to validate nonce increment after mint operation: %w", err)
	}

	// Log successful mint completion with progress
	currentMint := atomic.AddInt64(&st.mintCounter, 1)
	log.Printf("✅ MINT COMPLETED: Mint wallet %d → Transfer wallet %d (%d/%d) | TxHash: %s | MintAddr: %s | TargetAddr: %s | Amount: %d", mintWalletIndex, transferWalletIndex, currentMint, totalMints, result.Hash, mintWallet.Address, transferWallet.Address, MINT_AMOUNT)

	return nil
}

// performConcurrentMinting performs minting operations
func (st *StressTester) performConcurrentMinting() error {
	// Phase 2: Perform all mint operations
	log.Println("Phase 2: Starting mint operations...")
	if err := st.performAllMints(); err != nil {
		return fmt.Errorf("mint phase failed: %w", err)
	}
	log.Println("✓ Phase 2: All mints completed")

	// Phase 3: Perform transfers from minted wallets to distribution wallets
	log.Println("Phase 3: Starting transfers to distribution wallets...")
	if err := st.performAllTransfers(); err != nil {
		return fmt.Errorf("transfer phase failed: %w", err)
	}
	log.Println("✓ Phase 3: All transfers completed")

	// Print statistics
	st.rateLimiter.PrintStats()
	st.nodePool.PrintDistribution()

	log.Println("✓ All operations completed successfully!")
	return nil
}

// performAllMints executes all mint operations concurrently
func (st *StressTester) performAllMints() error {
	// Reset mint counter
	atomic.StoreInt64(&st.mintCounter, 0)

	var mintWG sync.WaitGroup
	errorChan := make(chan error, MINT_WALLETS_COUNT*WALLETS_PER_MINT)

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

			// Perform mint operations to assigned transfer wallets
			for j := startIdx; j < endIdx; j++ {
				transferWallet := st.transferWallets[j]

				if err := st.mintToWallet(wallet, transferWallet, walletIndex+1, j+1); err != nil {
					errorChan <- fmt.Errorf("mint wallet %d failed to mint to primary wallet %d: %w",
						walletIndex+1, j+1, err)
					return
				}

				// Update mint progress counter (no batch logging)
			}
		}(i, mintWallet)
	}

	// Wait for all minting operations to complete
	mintWG.Wait()
	close(errorChan)

	// Check for any errors
	for err := range errorChan {
		return err
	}

	return nil
}

// performAllTransfers executes all transfer operations from minted wallets to distribution wallets
func (st *StressTester) performAllTransfers() error {
	// Reset transfer counter
	atomic.StoreInt64(&st.transferCounter, 0)

	var transferWG sync.WaitGroup
	errorChan := make(chan error, TRANSFER_WALLETS_COUNT)

	// Calculate total transfers
	totalTransfers := int64(TRANSFER_WALLETS_COUNT * DIST_WALLETS_PER_TRANSFER)

	// Launch one goroutine per transfer wallet
	for i, transferWallet := range st.transferWallets {
		transferWG.Add(1)
		go func(walletIndex int, wallet *Wallet) {
			defer transferWG.Done()

			// Calculate the range of distribution wallets for this transfer wallet
			startIdx := walletIndex * DIST_WALLETS_PER_TRANSFER
			endIdx := startIdx + DIST_WALLETS_PER_TRANSFER

			// Perform sequential transfers to distribution wallets
			if err := st.transferToDistributionWallets(wallet, walletIndex+1, startIdx, endIdx, totalTransfers); err != nil {
				errorChan <- fmt.Errorf("transfer wallet %d failed to distribute: %w", walletIndex+1, err)
			}
		}(i, transferWallet)
	}

	// Wait for all transfer operations to complete
	transferWG.Wait()
	close(errorChan)

	// Check for any errors
	for err := range errorChan {
		return err
	}

	return nil
}

// transferToDistributionWallets performs sequential transfers from one transfer wallet to its distribution wallets
func (st *StressTester) transferToDistributionWallets(transferWallet *Wallet, transferWalletIndex int, startIdx int, endIdx int, totalTransfers int64) error {
	// Calculate transfer amount (1/5 of minted amount)
	transferAmount := MINT_AMOUNT / 5

	// Get current nonce for the transfer wallet
	currentNonce, err := st.getAccountNonce(transferWallet.Address)
	if err != nil {
		return fmt.Errorf("failed to get initial nonce for transfer wallet %d: %w", transferWalletIndex, err)
	}

	// Sequential transfers to each distribution wallet
	for i := startIdx; i < endIdx; i++ {
		distWallet := st.distributionWallets[i]

		// Perform single transfer
		if err := st.transferToSingleDistWallet(transferWallet, transferWalletIndex, distWallet, i+1, currentNonce, int64(transferAmount), totalTransfers); err != nil {
			return fmt.Errorf("failed to transfer to distribution wallet %d: %w", i+1, err)
		}

		// Increment nonce for next transfer
		currentNonce++

		// Wait for nonce to be confirmed before next transfer
		if err := st.validateNonceIncrement(transferWallet.Address, currentNonce, "TRANSFER_WALLET", "TRANSFER"); err != nil {
			return fmt.Errorf("nonce validation failed after transfer to distribution wallet %d: %w", i+1, err)
		}
	}

	return nil
}

// transferToSingleDistWallet performs a single transfer to a distribution wallet
func (st *StressTester) transferToSingleDistWallet(transferWallet *Wallet, transferWalletIndex int, distWallet *Wallet, distWalletIndex int, nonce uint64, amount int64, totalTransfers int64) error {
	// Get a node for POST operation
	client, _, nodeIndex, err := st.nodePool.GetNodeForMint()
	if err != nil {
		return fmt.Errorf("failed to get node for transfer operation: %w", err)
	}

	// Create transfer payload
	amountBig := big.NewInt(amount)
	payload := onemoney.PaymentPayload{
		ChainID:   CHAIN_ID,
		Nonce:     nonce,
		Recipient: common.HexToAddress(distWallet.Address),
		Value:     amountBig,
		Token:     common.HexToAddress(st.tokenAddress),
	}

	// Sign the payload
	signature, err := client.SignMessage(payload, transferWallet.PrivateKey)
	if err != nil {
		log.Printf("❌ SIGNING ERROR: Transfer transaction signature failed | TransferWallet: %d (%s) | DistWallet: %d (%s) | Nonce: %d | Amount: %d | Token: %s | Node: %d | Error: %v",
			transferWalletIndex, transferWallet.Address, distWalletIndex, distWallet.Address, nonce, amount, st.tokenAddress, nodeIndex, err)
		return fmt.Errorf("failed to sign transfer transaction: %w", err)
	}

	// Create transfer request
	req := &onemoney.PaymentRequest{
		PaymentPayload: payload,
		Signature:      *signature,
	}

	// Get rate limiter for this node
	nodeRateLimiter := st.rateLimiter.GetNodeRateLimiter(nodeIndex)
	if nodeRateLimiter == nil {
		return fmt.Errorf("no rate limiter for node %d", nodeIndex)
	}

	// Apply rate limiting for POST request
	if err := nodeRateLimiter.WaitForPostToken(st.ctx); err != nil {
		return fmt.Errorf("rate limiting failed for Transfer: %w", err)
	}

	// Send transfer request
	result, err := client.SendPayment(st.ctx, req)
	if err != nil {
		log.Printf("❌ API ERROR: SendPayment failed | TransferWallet: %d (%s) | DistWallet: %d (%s) | Nonce: %d | Amount: %d | Token: %s | Node: %d | Error: %v",
			transferWalletIndex, transferWallet.Address, distWalletIndex, distWallet.Address, nonce, amount, st.tokenAddress, nodeIndex, err)
		return fmt.Errorf("failed to send payment: %w", err)
	}

	// Wait for transaction confirmation
	if err := st.waitForTransactionReceipt(result.Hash, transferWallet.Address, distWallet.Address, "TRANSFER"); err != nil {
		log.Printf("❌ TRANSFER TIMEOUT: Transfer wallet %d → Distribution wallet %d | TxHash: %s | TransferAddr: %s | DistAddr: %s | Amount: %d | Nonce: %d | Error: %v",
			transferWalletIndex, distWalletIndex, result.Hash, transferWallet.Address, distWallet.Address, amount, nonce, err)
		return fmt.Errorf("failed to confirm transfer transaction: %w", err)
	}

	// Log successful transfer completion with progress
	currentTransfer := atomic.AddInt64(&st.transferCounter, 1)
	log.Printf("✅ TRANSFER COMPLETED: Transfer wallet %d → Distribution wallet %d (%d/%d) | TxHash: %s | TransferAddr: %s | DistAddr: %s | Amount: %d",
		transferWalletIndex, distWalletIndex, currentTransfer, totalTransfers, result.Hash, transferWallet.Address, distWallet.Address, amount)

	return nil
}
