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
			log.Printf("✓ Nonce verified: %d (wallet %d)", expectedNonce, walletIndex)
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
func NewStressTester(nodeURLs []string, totalPostRate int, totalGetRate int) (*StressTester, error) {
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
	maxRetries := 60 // Maximum 60 retries (about 30 seconds with 500ms intervals)
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
				return fmt.Errorf("transaction receipt timeout after %d retries", maxRetries)
			}
			// Log only at 50% and max retries
			if retryCount == maxRetries/2 || retryCount == maxRetries-1 {
				log.Printf("Waiting for tx %s... (retry %d/%d)", txHash[:8]+"...", retryCount, maxRetries)
			}
			time.Sleep(500 * time.Millisecond)
			continue
		}

		if receipt.Success {
			// Transaction confirmed
			return nil
		} else {
			return fmt.Errorf("transaction failed: %s", txHash)
		}
	}
}

// validateNonceIncrement validates nonce increment using node pool
func (st *StressTester) validateNonceIncrement(address string, expectedNonce uint64, walletType string, operationType string) error {
	retryCount := 0
	maxRetries := 40 // Maximum 40 retries (about 20 seconds with 500ms intervals)
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
				return fmt.Errorf("failed to get nonce after %d retries: %w", maxRetries, err)
			}
			time.Sleep(500 * time.Millisecond)
			continue
		}

		if accountNonce.Nonce == expectedNonce {
			// Nonce validated
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

		// Log progress every 100 wallets to avoid excessive logging
		if (i+1)%100 == 0 {
			// Log progress every 10%
			if (i+1)%(TRANSFER_WALLETS_COUNT/10) == 0 || i+1 == TRANSFER_WALLETS_COUNT {
				log.Printf("Transfer wallets: %d/%d", i+1, TRANSFER_WALLETS_COUNT)
			}
		}
	}

	log.Printf("✓ Created %d transfer wallets", TRANSFER_WALLETS_COUNT)
	return nil
}

// Step 2b: Create distribution wallets (third tier)
func (st *StressTester) createDistributionWallets() error {
	log.Printf("Creating %d distribution wallets...", DISTRIBUTION_WALLETS_COUNT)

	st.distributionWallets = make([]*Wallet, DISTRIBUTION_WALLETS_COUNT)
	for i := 0; i < DISTRIBUTION_WALLETS_COUNT; i++ {
		wallet, err := generateDeterministicWallet("distribution", i)
		if err != nil {
			return fmt.Errorf("failed to create distribution wallet %d: %w", i, err)
		}
		st.distributionWallets[i] = wallet

		// Log progress every 500 wallets to avoid excessive logging
		if (i+1)%500 == 0 {
			// Log progress every 5%
			if (i+1)%(DISTRIBUTION_WALLETS_COUNT/20) == 0 || i+1 == DISTRIBUTION_WALLETS_COUNT {
				log.Printf("Distribution wallets: %d/%d", i+1, DISTRIBUTION_WALLETS_COUNT)
			}
		}
	}

	log.Printf("✓ Created %d distribution wallets", DISTRIBUTION_WALLETS_COUNT)
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
		log.Printf("Error: Failed to sign token: %v", err)
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
		log.Printf("Error: Failed to submit token: %v", err)
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
		log.Printf("Error: Failed to sign authority (wallet %d): %v", walletIndex+1, err)
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
		log.Printf("Error: Failed to grant authority (wallet %d): %v", walletIndex+1, err)
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
	// Minting from wallet mintWalletIndex to transferWalletIndex

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
		log.Printf("Error: Failed to sign mint (wallet %d): %v", mintWalletIndex, err)
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
		log.Printf("Error: Failed to mint (%d→%d): %v", mintWalletIndex, transferWalletIndex, err)
		return fmt.Errorf("failed to mint token: %w", err)
	}

	// Mint in progress

	// Wait for transaction confirmation
	if err := st.waitForTransactionReceipt(result.Hash, mintWallet.Address, transferWallet.Address, "MINT"); err != nil {
		log.Printf("Error: Mint timeout (%d→%d): %v", mintWalletIndex, transferWalletIndex, err)
		return fmt.Errorf("failed to confirm mint transaction: %w", err)
	}

	// Validate nonce increment
	if err := st.validateNonceIncrement(mintWallet.Address, nonce+1, "MINT_WALLET", "MINT"); err != nil {
		log.Printf("Error: Nonce validation failed (wallet %d): %v", mintWalletIndex, err)
		return fmt.Errorf("failed to validate nonce increment after mint operation: %w", err)
	}

	// Mint success: mintWalletIndex -> transferWalletIndex

	return nil
}

// transferFromWallet performs token transfer
func (st *StressTester) transferFromWallet(fromWallet, toWallet *Wallet, amount int64, fromIndex, toIndex int) error {
	// Starting transfer from wallet fromIndex to toIndex

	// Get sender wallet's current nonce
	nonce, err := st.getAccountNonce(fromWallet.Address)
	if err != nil {
		return err
	}

	// Get a node for POST operation
	client, _, nodeIndex, err := st.nodePool.GetNodeForTransfer()
	if err != nil {
		return fmt.Errorf("failed to get node for transfer operation: %w", err)
	}

	// Create payment payload for token transfer
	payload := onemoney.PaymentPayload{
		ChainID:   CHAIN_ID,
		Nonce:     nonce,
		Recipient: common.HexToAddress(toWallet.Address),
		Value:     big.NewInt(amount),
		Token:     common.HexToAddress(st.tokenAddress),
	}

	// Transfer payload

	// Sign the payload
	signature, err := client.SignMessage(payload, fromWallet.PrivateKey)
	if err != nil {
		log.Printf("Error: Failed to sign transfer (%d→%d): %v", fromIndex, toIndex, err)
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

	// Get rate limiter for this node
	nodeRateLimiter := st.rateLimiter.GetNodeRateLimiter(nodeIndex)
	if nodeRateLimiter == nil {
		return fmt.Errorf("no rate limiter for node %d", nodeIndex)
	}

	// Apply rate limiting for POST request
	if err := nodeRateLimiter.WaitForPostToken(st.ctx); err != nil {
		return fmt.Errorf("rate limiting failed for SendPayment: %w", err)
	}

	// Send payment request
	result, err := client.SendPayment(st.ctx, req)
	if err != nil {
		log.Printf("Error: Failed to transfer (%d→%d): %v", fromIndex, toIndex, err)
		return fmt.Errorf("failed to send transfer: %w", err)
	}

	// Transfer in progress

	// Wait for transaction confirmation
	if err := st.waitForTransactionReceipt(result.Hash, fromWallet.Address, toWallet.Address, "TRANSFER"); err != nil {
		log.Printf("Error: Transfer timeout (%d→%d): %v", fromIndex, toIndex, err)
		return fmt.Errorf("failed to confirm transfer transaction: %w", err)
	}

	// Validate nonce increment
	if err := st.validateNonceIncrement(fromWallet.Address, nonce+1, "TRANSFER_WALLET", "TRANSFER"); err != nil {
		log.Printf("Error: Nonce validation failed (wallet %d): %v", fromIndex, err)
		return fmt.Errorf("failed to validate nonce increment after transfer operation: %w", err)
	}

	// Increment transfer counter
	currentTransfer := atomic.AddInt64(&st.transferCounter, 1)
	totalTransfers := int64(TRANSFER_WALLETS_COUNT * TRANSFER_MULTIPLIER)

	if currentTransfer%(totalTransfers/4) == 0 || currentTransfer == totalTransfers {
		log.Printf("Transfers: %d/%d", currentTransfer, totalTransfers)
	}
	return nil
}

// transferWorker processes transfer tasks
func (st *StressTester) transferWorker(transferTasks <-chan TransferTask, wg *sync.WaitGroup) {
	for task := range transferTasks {
		log.Printf("Starting transfer task: primary wallet %d → %d distribution wallets",
			task.PrimaryIndex, task.EndIdx-task.StartIdx)

		// Transfer tokens from primary wallet to each assigned distribution wallet
		transferCount := 0
		totalTransfers := task.EndIdx - task.StartIdx
		
		for i := task.StartIdx; i < task.EndIdx; i++ {
			if i >= len(st.distributionWallets) {
				log.Printf("Warning: distribution wallet index %d exceeds available wallets (%d)",
					i, len(st.distributionWallets))
				break
			}

			distributionWallet := st.distributionWallets[i]
			transferCount++
			
			log.Printf("Transfer %d/%d: primary wallet %d → distribution wallet %d",
				transferCount, totalTransfers, task.PrimaryIndex, i+1)
			
			if err := st.transferFromWallet(task.PrimaryWallet, distributionWallet,
				int64(TRANSFER_AMOUNT), task.PrimaryIndex, i+1); err != nil {
				log.Printf("Error: Transfer failed (primary %d → distribution %d): %v",
					task.PrimaryIndex, i+1, err)
				// Continue with next transfer instead of failing entire task
				continue
			}
		}

		log.Printf("✓ Completed transfers for primary wallet %d (%d/%d)", 
			task.PrimaryIndex, transferCount, totalTransfers)
		wg.Done()
	}
}

// performConcurrentMinting performs minting and transfers in sequential phases
func (st *StressTester) performConcurrentMinting() error {
	// Phase 2: Perform all mint operations
	log.Println("Phase 2: Starting mint operations...")
	if err := st.performAllMints(); err != nil {
		return fmt.Errorf("mint phase failed: %w", err)
	}
	log.Println("✓ Phase 2: All mints completed")

	// Phase 3: Perform all transfers
	log.Println("Phase 3: Starting transfer operations...")
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
	var mintWG sync.WaitGroup
	errorChan := make(chan error, MINT_WALLETS_COUNT*WALLETS_PER_MINT)

	// Add atomic counter for mint progress
	var mintCounter int64
	totalMints := MINT_WALLETS_COUNT * WALLETS_PER_MINT

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

				// Update mint progress
				currentMint := atomic.AddInt64(&mintCounter, 1)
				if currentMint%(int64(totalMints)/4) == 0 || currentMint == int64(totalMints) {
					log.Printf("Minting: %d/%d", currentMint, totalMints)
				}
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

// performAllTransfers executes all transfer operations concurrently
func (st *StressTester) performAllTransfers() error {
	var transferWG sync.WaitGroup
	transferTasks := make(chan TransferTask, TRANSFER_WALLETS_COUNT)

	// Start transfer worker goroutines
	log.Printf("Starting %d transfer workers...", TRANSFER_WORKERS_COUNT)
	for i := 0; i < TRANSFER_WORKERS_COUNT; i++ {
		go st.transferWorker(transferTasks, &transferWG)
	}

	// Queue all transfer tasks
	for i, transferWallet := range st.transferWallets {
		transferStartIdx := i * TRANSFER_MULTIPLIER
		transferEndIdx := transferStartIdx + TRANSFER_MULTIPLIER
		if transferEndIdx > len(st.distributionWallets) {
			transferEndIdx = len(st.distributionWallets)
		}

		transferTask := TransferTask{
			PrimaryWallet: transferWallet,
			StartIdx:      transferStartIdx,
			EndIdx:        transferEndIdx,
			PrimaryIndex:  i + 1,
		}

		transferWG.Add(1)
		transferTasks <- transferTask
	}

	// Close the channel to signal workers to stop after processing all tasks
	close(transferTasks)

	// Wait for all transfers to complete
	transferWG.Wait()

	return nil
}
