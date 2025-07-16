package main

import (
	"context"
	// "math/rand"
	"time"

	onemoney "github.com/1Money-Co/1money-go-sdk"
	"golang.org/x/time/rate"
)

// Configurable Constants - Modify these values as needed for different stress test scenarios
const (
	// Wallet Configuration
	MINT_WALLETS_COUNT     = 20                                          // Number of mint authority wallets
	TRANSFER_WALLETS_COUNT = 1000                                        // Number of primary transfer recipient wallets
	WALLETS_PER_MINT       = TRANSFER_WALLETS_COUNT / MINT_WALLETS_COUNT // Number of transfer wallets per mint wallet (should equal TRANSFER_WALLETS_COUNT / MINT_WALLETS_COUNT)

	// Multi-tier Distribution Configuration
	TRANSFER_MULTIPLIER        = 10                                           // Number of distribution wallets per primary wallet
	DISTRIBUTION_WALLETS_COUNT = TRANSFER_WALLETS_COUNT * TRANSFER_MULTIPLIER // Total distribution wallets (8000)
	TRANSFER_WORKERS_COUNT     = 100                                          // Number of concurrent transfer worker goroutines

	// Token Configuration
	TOKEN_NAME     = "Stress Test Token"
	TOKEN_DECIMALS = 6
	CHAIN_ID       = 1212101

	// Mint Configuration
	MINT_ALLOWANCE  = 1000000000000000000                     // Allowance granted to each mint wallet
	MINT_AMOUNT     = 1000000000 * (TRANSFER_MULTIPLIER + 1)  // Amount to mint per operation
	TRANSFER_AMOUNT = MINT_AMOUNT / (TRANSFER_MULTIPLIER + 1) // Amount to transfer per distribution operation (250)

	// Transaction Validation Configuration
	RECEIPT_CHECK_TIMEOUT    = 10 * time.Second       // Timeout for waiting for transaction receipt
	RECEIPT_CHECK_INTERVAL   = 500 * time.Millisecond // Interval between receipt checks
	NONCE_VALIDATION_TIMEOUT = 10 * time.Second       // Timeout for nonce validation
	NONCE_CHECK_INTERVAL     = 500 * time.Millisecond // Interval between nonce checks

	// Rate Limiting Configuration
	POST_RATE_LIMIT_TPS = 125 // Maximum POST requests per second (configurable)
	GET_RATE_LIMIT_TPS  = 250 // Maximum GET requests per second (configurable)
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
	postRateLimiter     *rate.Limiter // Rate limiter for POST requests
	getRateLimiter      *rate.Limiter // Rate limiter for GET requests
	transferCounter     int64         // Atomic counter for tracking transfer progress
}

// generateTokenSymbol generates a random token symbol with format "1M" + 5 letters + 2 digits
func generateTokenSymbol() string {
	// const letters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	// const digits = "0123456789"

	// // Generate 5 random letters
	// letterPart := make([]byte, 5)
	// for i := range letterPart {
	// 	letterPart[i] = letters[rand.Intn(len(letters))]
	// }

	// // Generate 2 random digits
	// digitPart := make([]byte, 2)
	// for i := range digitPart {
	// 	digitPart[i] = digits[rand.Intn(len(digits))]
	// }

	// return "1USD" + string(letterPart) + string(digitPart)
	return "1USD"
}

// GetTokenSymbol returns a dynamically generated token symbol for each test run
func GetTokenSymbol() string {
	return generateTokenSymbol()
}
