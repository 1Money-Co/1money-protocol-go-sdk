package main

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"sync"

	onemoney "github.com/1Money-Co/1money-go-sdk"
	"github.com/ethereum/go-ethereum/common"
	"golang.org/x/time/rate"
)

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
		client:          client,
		operatorWallet:  operatorWallet,
		ctx:             context.Background(),
		postRateLimiter: rate.NewLimiter(rate.Limit(POST_RATE_LIMIT_TPS), 1), // 250 TPS with burst of 1
		getRateLimiter:  rate.NewLimiter(rate.Limit(GET_RATE_LIMIT_TPS), 1),  // 500 TPS with burst of 1
	}, nil
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

		// Log progress every 10 wallets to avoid excessive logging
		if (i+1)%10 == 0 {
			log.Printf("Created mint wallets: %d/%d", i+1, MINT_WALLETS_COUNT)
		}
	}

	log.Printf("Successfully created all %d mint wallets", MINT_WALLETS_COUNT)
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

		// Log progress every 100 wallets to avoid excessive logging
		if (i+1)%100 == 0 {
			log.Printf("Created primary transfer wallets: %d/%d", i+1, TRANSFER_WALLETS_COUNT)
		}
	}

	log.Printf("Successfully created all %d primary transfer wallets", TRANSFER_WALLETS_COUNT)
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

	// Apply rate limiting for POST request
	if err := st.postRateLimiter.Wait(st.ctx); err != nil {
		return fmt.Errorf("rate limiting failed for IssueToken: %w", err)
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

		// Apply rate limiting for POST request
		if err := st.postRateLimiter.Wait(st.ctx); err != nil {
			return fmt.Errorf("rate limiting failed for GrantTokenAuthority: %w", err)
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

	// Apply rate limiting for POST request
	if err := st.postRateLimiter.Wait(st.ctx); err != nil {
		return fmt.Errorf("rate limiting failed for SendPayment: %w", err)
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

	// Apply rate limiting for POST request
	if err := st.postRateLimiter.Wait(st.ctx); err != nil {
		return fmt.Errorf("rate limiting failed for MintToken: %w", err)
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
	log.Printf("- POST rate limit: %d TPS", POST_RATE_LIMIT_TPS)
	log.Printf("- GET rate limit: %d TPS", GET_RATE_LIMIT_TPS)
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
