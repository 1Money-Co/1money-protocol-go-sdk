package main

import (
	"context"
	"sync"
	"time"
)

// Configurable Constants - Modify these values as needed for different stress test scenarios
const (
	// Wallet Configuration
	MINT_WALLETS_COUNT     = 20                                          // Number of mint authority wallets
	TRANSFER_WALLETS_COUNT = 10000                                       // Number of primary transfer recipient wallets
	WALLETS_PER_MINT       = TRANSFER_WALLETS_COUNT / MINT_WALLETS_COUNT // Number of transfer wallets per mint wallet (should equal TRANSFER_WALLETS_COUNT / MINT_WALLETS_COUNT)

	// Token Configuration
	TOKEN_NAME     = "Stress Test Token"
	TOKEN_DECIMALS = 6
	CHAIN_ID       = 1212101

	// Mint Configuration
	MINT_ALLOWANCE = 1000000000000000000 // Allowance granted to each mint wallet
	MINT_AMOUNT    = 1000000000          // Amount to mint per operation

	// Rate Limiting Configuration
	POST_RATE_LIMIT_TPS = 50  // Maximum POST requests per second (configurable)
	GET_RATE_LIMIT_TPS  = 250 // Maximum GET requests per second (configurable)

	// Nonce Management Configuration
	NONCE_VERIFY_MAX_RETRIES = 20                     // Maximum retries for nonce verification
	NONCE_VERIFY_INTERVAL    = 500 * time.Millisecond // Interval between nonce verification retries

	// CSV Generation Configuration
	CSV_PROGRESS_INTERVAL_WALLETS = 200 // Progress logging interval for wallet CSV generation
	CSV_PROGRESS_INTERVAL_DIST    = 500 // Progress logging interval for distribution CSV generation
)

// Wallet represents a wallet with private key, public key, and address
type Wallet struct {
	PrivateKey string
	PublicKey  string
	Address    string
}

// StressTester manages the stress testing operations
type StressTester struct {
	nodePool           *NodePool
	operatorWallet     *Wallet
	mintWallets        []*Wallet
	transferWallets    []*Wallet // Primary transfer wallets (tier 2)
	tokenAddress       string
	ctx                context.Context
	rateLimiter        *MultiNodeRateLimiter // Rate limiter for distributed nodes
	operatorNonceMutex sync.Mutex            // Mutex for operator wallet nonce management
	operatorNonce      uint64                // Current operator wallet nonce
}

// GetTokenSymbol returns a dynamically generated token symbol for each test run
func GetTokenSymbol() string {
	// Use timestamp to ensure uniqueness
	timestamp := time.Now().Format("150405") // HHMMSS format
	return "1USD" + timestamp
}
