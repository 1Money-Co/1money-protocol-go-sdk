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

	onemoney "github.com/1Money-Co/1money-go-sdk"
	"github.com/ethereum/go-ethereum/common"
)

var (
	nodeList   = flag.String("nodes", "", "Comma-separated list of node URLs (e.g. '127.0.0.1:18555,127.0.0.1:18556')")
	useTestnet = flag.Bool("testnet", true, "Use testnet (true) or mainnet (false)")
	postRate   = flag.Int("post-rate", POST_RATE_LIMIT_TPS, "Total POST rate limit in TPS")
	getRate    = flag.Int("get-rate", GET_RATE_LIMIT_TPS, "Total GET rate limit in TPS")
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

	return &StressTester{
		nodePool:       nodePool,
		operatorWallet: operatorWallet,
		ctx:            context.Background(),
		rateLimiter:    rateLimiter,
	}, nil
}

// getAccountNonce gets account nonce using node pool
func (st *StressTester) getAccountNonce(address string) (uint64, error) {
	// Get a node for GET operation
	client, nodeURL, nodeIndex, err := st.nodePool.GetNodeForGet()
	if err != nil {
		return 0, fmt.Errorf("failed to get node for nonce check: %w", err)
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
		log.Printf("Failed to get account nonce for %s from node %s: %v", address, nodeURL, err)
		return 0, err
	}

	return accountNonce.Nonce, nil
}

// waitForTransactionReceipt waits for transaction receipt using node pool
func (st *StressTester) waitForTransactionReceipt(txHash string, fromAddress string, toAddress string, operationType string) error {
	for {
		// Get a node for GET operation
		client, nodeURL, nodeIndex, err := st.nodePool.GetNodeForGet()
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
			log.Printf("%s_RECEIPT_CHECK: Waiting for transaction %s (from: %s, to: %s) on node %s...",
				operationType, txHash, fromAddress, toAddress, nodeURL)
			continue
		}

		if receipt.Success {
			log.Printf("%s_RECEIPT_SUCCESS: Transaction %s confirmed successfully (from: %s, to: %s)",
				operationType, txHash, fromAddress, toAddress)
			return nil
		} else {
			return fmt.Errorf("%s_RECEIPT_FAILED: Transaction %s failed (from: %s, to: %s)",
				operationType, txHash, fromAddress, toAddress)
		}
	}
}

// validateNonceIncrement validates nonce increment using node pool
func (st *StressTester) validateNonceIncrement(address string, expectedNonce uint64, walletType string, operationType string) error {
	for {
		// Get a node for GET operation
		client, nodeURL, nodeIndex, err := st.nodePool.GetNodeForGet()
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
			log.Printf("%s_NONCE_CHECK: Failed to get account nonce for %s (%s) from node %s: %v",
				operationType, walletType, address, nodeURL, err)
			continue
		}

		if accountNonce.Nonce == expectedNonce {
			log.Printf("%s_NONCE_VALIDATED: %s (%s) nonce correctly incremented to %d",
				operationType, walletType, address, expectedNonce)
			return nil
		}

		if accountNonce.Nonce > expectedNonce {
			return fmt.Errorf("%s_NONCE_UNEXPECTED: %s (%s) nonce jumped to %d, expected %d",
				operationType, walletType, address, accountNonce.Nonce, expectedNonce)
		}

		log.Printf("%s_NONCE_WAITING: %s (%s) current nonce: %d, waiting for: %d on node %s",
			operationType, walletType, address, accountNonce.Nonce, expectedNonce, nodeURL)
	}
}

// Step 1: Create mint wallets
func (st *StressTester) createMintWallets() error {
	log.Printf("Creating %d deterministic mint wallets...", MINT_WALLETS_COUNT)

	st.mintWallets = make([]*Wallet, MINT_WALLETS_COUNT)
	for i := 0; i < MINT_WALLETS_COUNT; i++ {
		wallet, err := generateDeterministicWallet("mint", i)
		if err != nil {
			return fmt.Errorf("failed to create mint wallet %d: %w", i, err)
		}
		st.mintWallets[i] = wallet

		// Log progress every 10 wallets to avoid excessive logging
		if (i+1)%10 == 0 {
			log.Printf("Created mint wallets: %d/%d", i+1, MINT_WALLETS_COUNT)
		}
	}

	log.Printf("Successfully created all %d deterministic mint wallets", MINT_WALLETS_COUNT)
	return nil
}

// Step 2: Create transfer wallets (primary tier)
func (st *StressTester) createTransferWallets() error {
	log.Printf("Creating %d deterministic primary transfer wallets...", TRANSFER_WALLETS_COUNT)

	st.transferWallets = make([]*Wallet, TRANSFER_WALLETS_COUNT)
	for i := 0; i < TRANSFER_WALLETS_COUNT; i++ {
		wallet, err := generateDeterministicWallet("transfer", i)
		if err != nil {
			return fmt.Errorf("failed to create primary transfer wallet %d: %w", i, err)
		}
		st.transferWallets[i] = wallet

		// Log progress every 100 wallets to avoid excessive logging
		if (i+1)%100 == 0 {
			log.Printf("Created primary transfer wallets: %d/%d", i+1, TRANSFER_WALLETS_COUNT)
		}
	}

	log.Printf("Successfully created all %d deterministic primary transfer wallets", TRANSFER_WALLETS_COUNT)
	return nil
}

// Step 2b: Create distribution wallets (third tier)
func (st *StressTester) createDistributionWallets() error {
	log.Printf("Creating %d deterministic distribution wallets...", DISTRIBUTION_WALLETS_COUNT)

	st.distributionWallets = make([]*Wallet, DISTRIBUTION_WALLETS_COUNT)
	for i := 0; i < DISTRIBUTION_WALLETS_COUNT; i++ {
		wallet, err := generateDeterministicWallet("distribution", i)
		if err != nil {
			return fmt.Errorf("failed to create distribution wallet %d: %w", i, err)
		}
		st.distributionWallets[i] = wallet

		// Log progress every 500 wallets to avoid excessive logging
		if (i+1)%500 == 0 {
			log.Printf("Created distribution wallets: %d/%d", i+1, DISTRIBUTION_WALLETS_COUNT)
		}
	}

	log.Printf("Successfully created all %d deterministic distribution wallets", DISTRIBUTION_WALLETS_COUNT)
	return nil
}

// createToken creates token using operator wallet
func (st *StressTester) createToken() error {
	log.Printf("TOKEN_CREATE_START: Creating token using operator wallet (%s)...", st.operatorWallet.Address)

	// Get nonce using multi-node
	nonce, err := st.getAccountNonce(st.operatorWallet.Address)
	if err != nil {
		return err
	}

	// Get a node for POST operation (token creation)
	client, nodeURL, nodeIndex, err := st.nodePool.GetNodeForMint() // Using mint counter for token operations
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

	log.Printf("TOKEN_CREATE_PAYLOAD: ChainID=%d, Nonce=%d, Symbol=%s, Name=%s, Decimals=%d, MasterAuthority=%s, IsPrivate=%t, Node=%s",
		payload.ChainID, payload.Nonce, payload.Symbol, payload.Name, payload.Decimals,
		payload.MasterAuthority.Hex(), payload.IsPrivate, nodeURL)

	signature, err := client.SignMessage(payload, st.operatorWallet.PrivateKey)
	if err != nil {
		log.Printf("TOKEN_CREATE_ERROR: Failed to sign token creation for operator wallet (%s): %v",
			st.operatorWallet.Address, err)
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
		log.Printf("TOKEN_CREATE_ERROR: Failed to submit token creation transaction for operator wallet (%s) to node %s: %v",
			st.operatorWallet.Address, nodeURL, err)
		return fmt.Errorf("failed to issue token: %w", err)
	}

	st.tokenAddress = result.Token
	log.Printf("TOKEN_CREATE_SUBMITTED: Token creation transaction submitted - Address: %s, TxHash: %s, Operator: %s, Node: %s",
		st.tokenAddress, result.Hash, st.operatorWallet.Address, nodeURL)

	// Wait for transaction confirmation
	if err := st.waitForTransactionReceipt(result.Hash, st.operatorWallet.Address, st.tokenAddress, "TOKEN_CREATE"); err != nil {
		log.Printf("TOKEN_CREATE_TIMEOUT: Failed to confirm token creation transaction %s for operator wallet (%s): %v",
			result.Hash, st.operatorWallet.Address, err)
		return fmt.Errorf("failed to confirm token creation: %w", err)
	}

	// Validate nonce increment
	if err := st.validateNonceIncrement(st.operatorWallet.Address, nonce+1, "OPERATOR_WALLET", "TOKEN_CREATE"); err != nil {
		log.Printf("TOKEN_CREATE_NONCE_ERROR: Failed to validate nonce increment for operator wallet (%s): %v",
			st.operatorWallet.Address, err)
		return fmt.Errorf("failed to validate nonce increment after token creation: %w", err)
	}

	log.Printf("TOKEN_CREATE_SUCCESS: Token created successfully - Address: %s, Symbol: %s, TxHash: %s",
		st.tokenAddress, tokenSymbol, result.Hash)

	return nil
}

// grantMintAuthorities grants mint permissions
func (st *StressTester) grantMintAuthorities() error {
	log.Printf("AUTHORITY_GRANT_START: Granting mint authorities to %d wallets using operator wallet (%s)...",
		len(st.mintWallets), st.operatorWallet.Address)

	var wg sync.WaitGroup
	errorChan := make(chan error, len(st.mintWallets))

	// Limit concurrent grants to avoid overwhelming the operator wallet's nonce
	semaphore := make(chan struct{}, 10) // Max 10 concurrent grants

	for i, mintWallet := range st.mintWallets {
		wg.Add(1)
		go func(index int, wallet *Wallet) {
			defer wg.Done()

			semaphore <- struct{}{}        // Acquire
			defer func() { <-semaphore }() // Release

			if err := st.grantSingleMintAuthority(index, wallet); err != nil {
				errorChan <- err
			}
		}(i, mintWallet)
	}

	wg.Wait()
	close(errorChan)

	// Check for errors
	for err := range errorChan {
		return err
	}

	log.Printf("AUTHORITY_GRANT_COMPLETE: All %d mint authorities granted successfully", len(st.mintWallets))
	return nil
}

// grantSingleMintAuthority grants mint authority to a single wallet
func (st *StressTester) grantSingleMintAuthority(walletIndex int, mintWallet *Wallet) error {
	log.Printf("AUTHORITY_GRANT_START: Granting mint authority to wallet %d (%s)", walletIndex+1, mintWallet.Address)

	// Get nonce using multi-node
	nonce, err := st.getAccountNonce(st.operatorWallet.Address)
	if err != nil {
		return err
	}

	// Get a node for POST operation
	client, nodeURL, nodeIndex, err := st.nodePool.GetNodeForMint()
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

	log.Printf("AUTHORITY_GRANT_PAYLOAD: Wallet=%d, Node=%s, Nonce=%d",
		walletIndex+1, nodeURL, payload.Nonce)

	signature, err := client.SignMessage(payload, st.operatorWallet.PrivateKey)
	if err != nil {
		log.Printf("AUTHORITY_GRANT_ERROR: Failed to sign authority grant for wallet %d (%s): %v",
			walletIndex+1, mintWallet.Address, err)
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
		log.Printf("AUTHORITY_GRANT_ERROR: Failed to submit authority grant transaction for wallet %d (%s) to node %s: %v",
			walletIndex+1, mintWallet.Address, nodeURL, err)
		return fmt.Errorf("failed to grant authority to wallet %d: %w", walletIndex, err)
	}

	log.Printf("AUTHORITY_GRANT_SUBMITTED: Authority grant transaction submitted for wallet %d (%s), TxHash: %s, Node: %s",
		walletIndex+1, mintWallet.Address, result.Hash, nodeURL)

	// Wait for transaction confirmation
	if err := st.waitForTransactionReceipt(result.Hash, st.operatorWallet.Address, mintWallet.Address, "AUTHORITY_GRANT"); err != nil {
		log.Printf("AUTHORITY_GRANT_TIMEOUT: Failed to confirm authority grant transaction %s for wallet %d (%s): %v",
			result.Hash, walletIndex+1, mintWallet.Address, err)
		return fmt.Errorf("failed to confirm authority grant for wallet %d: %w", walletIndex, err)
	}

	// Validate nonce increment
	if err := st.validateNonceIncrement(st.operatorWallet.Address, nonce+1, "OPERATOR_WALLET", "AUTHORITY_GRANT"); err != nil {
		log.Printf("AUTHORITY_GRANT_NONCE_ERROR: Failed to validate nonce increment for operator wallet (%s) after granting authority to wallet %d: %v",
			st.operatorWallet.Address, walletIndex+1, err)
		return fmt.Errorf("failed to validate nonce increment after authority grant for wallet %d: %w", walletIndex, err)
	}

	log.Printf("AUTHORITY_GRANT_SUCCESS: Authority granted to wallet %d (%s), TxHash: %s, Allowance: %d",
		walletIndex+1, mintWallet.Address, result.Hash, MINT_ALLOWANCE)

	return nil
}

// mintToWallet performs a single mint operation
func (st *StressTester) mintToWallet(mintWallet, transferWallet *Wallet, mintWalletIndex, transferWalletIndex int) error {
	log.Printf("MINT_START: Mint wallet %d (%s) minting %d tokens to transfer wallet %d (%s)",
		mintWalletIndex, mintWallet.Address, MINT_AMOUNT, transferWalletIndex, transferWallet.Address)

	// Get mint wallet's current nonce
	nonce, err := st.getAccountNonce(mintWallet.Address)
	if err != nil {
		return err
	}

	// Get a node for POST operation
	client, nodeURL, nodeIndex, err := st.nodePool.GetNodeForMint()
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

	log.Printf("MINT_PAYLOAD: From=%s, To=%s, Amount=%d, Node=%s",
		mintWallet.Address, payload.Recipient.Hex(), payload.Value.Int64(), nodeURL)

	// Sign the payload
	signature, err := client.SignMessage(payload, mintWallet.PrivateKey)
	if err != nil {
		log.Printf("MINT_ERROR: Failed to sign mint transaction for wallet %d (%s): %v",
			mintWalletIndex, mintWallet.Address, err)
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
		log.Printf("MINT_ERROR: Failed to submit mint transaction for wallet %d (%s) to wallet %d (%s) on node %s: %v",
			mintWalletIndex, mintWallet.Address, transferWalletIndex, transferWallet.Address, nodeURL, err)
		return fmt.Errorf("failed to mint token: %w", err)
	}

	log.Printf("MINT_SUBMITTED: Transaction %s submitted (mint wallet %d -> transfer wallet %d) on node %s",
		result.Hash, mintWalletIndex, transferWalletIndex, nodeURL)

	// Wait for transaction confirmation
	if err := st.waitForTransactionReceipt(result.Hash, mintWallet.Address, transferWallet.Address, "MINT"); err != nil {
		log.Printf("MINT_TIMEOUT: Failed to confirm mint transaction %s from wallet %d (%s) to wallet %d (%s): %v",
			result.Hash, mintWalletIndex, mintWallet.Address, transferWalletIndex, transferWallet.Address, err)
		return fmt.Errorf("failed to confirm mint transaction: %w", err)
	}

	// Validate nonce increment
	if err := st.validateNonceIncrement(mintWallet.Address, nonce+1, "MINT_WALLET", "MINT"); err != nil {
		log.Printf("MINT_NONCE_ERROR: Failed to validate nonce increment for mint wallet %d (%s): %v",
			mintWalletIndex, mintWallet.Address, err)
		return fmt.Errorf("failed to validate nonce increment after mint operation: %w", err)
	}

	log.Printf("MINT_SUCCESS: Mint confirmed - wallet %d (%s) -> wallet %d (%s), TxHash: %s",
		mintWalletIndex, mintWallet.Address, transferWalletIndex, transferWallet.Address, result.Hash)

	return nil
}

// transferFromWallet performs token transfer
func (st *StressTester) transferFromWallet(fromWallet, toWallet *Wallet, amount int64, fromIndex, toIndex int) error {
	log.Printf("TRANSFER_START: Transferring %d tokens from wallet %d (%s) to distribution wallet %d (%s)",
		amount, fromIndex, fromWallet.Address, toIndex, toWallet.Address)

	// Get sender wallet's current nonce
	nonce, err := st.getAccountNonce(fromWallet.Address)
	if err != nil {
		return err
	}

	// Get a node for POST operation
	client, nodeURL, nodeIndex, err := st.nodePool.GetNodeForTransfer()
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

	log.Printf("TRANSFER_PAYLOAD: From=%s, To=%s, Amount=%d, Node=%s",
		fromWallet.Address, payload.Recipient.Hex(), payload.Value.Int64(), nodeURL)

	// Sign the payload
	signature, err := client.SignMessage(payload, fromWallet.PrivateKey)
	if err != nil {
		log.Printf("TRANSFER_ERROR: Failed to sign transfer transaction from wallet %d (%s) to wallet %d (%s): %v",
			fromIndex, fromWallet.Address, toIndex, toWallet.Address, err)
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
		log.Printf("TRANSFER_ERROR: Failed to submit transfer transaction from wallet %d (%s) to wallet %d (%s) on node %s: %v",
			fromIndex, fromWallet.Address, toIndex, toWallet.Address, nodeURL, err)
		return fmt.Errorf("failed to send transfer: %w", err)
	}

	log.Printf("TRANSFER_SUBMITTED: Transaction %s submitted (wallet %d -> distribution wallet %d) on node %s",
		result.Hash, fromIndex, toIndex, nodeURL)

	// Wait for transaction confirmation
	if err := st.waitForTransactionReceipt(result.Hash, fromWallet.Address, toWallet.Address, "TRANSFER"); err != nil {
		log.Printf("TRANSFER_TIMEOUT: Failed to confirm transfer transaction %s from wallet %d (%s) to wallet %d (%s): %v",
			result.Hash, fromIndex, fromWallet.Address, toIndex, toWallet.Address, err)
		return fmt.Errorf("failed to confirm transfer transaction: %w", err)
	}

	// Validate nonce increment
	if err := st.validateNonceIncrement(fromWallet.Address, nonce+1, "TRANSFER_WALLET", "TRANSFER"); err != nil {
		log.Printf("TRANSFER_NONCE_ERROR: Failed to validate nonce increment for transfer wallet %d (%s): %v",
			fromIndex, fromWallet.Address, err)
		return fmt.Errorf("failed to validate nonce increment after transfer operation: %w", err)
	}

	// Increment transfer counter
	currentTransfer := atomic.AddInt64(&st.transferCounter, 1)
	totalTransfers := int64(TRANSFER_WALLETS_COUNT * TRANSFER_MULTIPLIER)

	log.Printf("TRANSFER_SUCCESS: Transfer confirmed (%d/%d) - wallet %d (%s) -> distribution wallet %d (%s), TxHash: %s",
		currentTransfer, totalTransfers, fromIndex, fromWallet.Address, toIndex, toWallet.Address, result.Hash)
	return nil
}

// transferWorker processes transfer tasks
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

// performConcurrentMinting performs concurrent minting
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

	// Print rate limiter statistics
	st.rateLimiter.PrintStats()

	// Print node distribution statistics
	st.nodePool.PrintDistribution()

	log.Println("All concurrent minting and transfer operations completed successfully!")
	return nil
}
