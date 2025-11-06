//go:build integration

package onemoney

import (
	"context"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// TestEnvironment represents the test environment configuration
type TestEnvironment struct {
	Name   string
	Client *Client
}

// getTestEnvironment returns the configured test environment
// Supported environments: local, devnet, testnet (default), mainnet
func getTestEnvironment(t *testing.T) *TestEnvironment {
	env := os.Getenv("TEST_ENV")
	if env == "" {
		env = "testnet"
	}

	var client *Client
	switch env {
	case "local":
		url := os.Getenv("LOCAL_API_URL")
		if url == "" {
			url = "http://localhost:18555"
		}
		client = NewClientWithCustomUrl(url, WithTimeout(10*time.Second))
	case "devnet":
		url := os.Getenv("DEVNET_API_URL")
		if url == "" {
			t.Skip("DEVNET_API_URL not set, skipping devnet tests")
		}
		client = NewClientWithCustomUrl(url, WithTimeout(10*time.Second))
	case "testnet":
		client = NewTestClient()
	case "mainnet":
		client = NewClient()
	default:
		t.Fatalf("Unknown TEST_ENV: %s. Supported: local, devnet, testnet, mainnet", env)
	}

	return &TestEnvironment{
		Name:   env,
		Client: client,
	}
}

// getTestAddress returns a test address from environment or a default one
func getTestAddress() string {
	addr := os.Getenv("TEST_ADDRESS")
	if addr == "" {
		// Default test address - should exist on testnet
		addr = "0x0477fFa70fa8078d8265d963895Fa7Fd85426602"
	}
	return addr
}

// getTestTokenAddress returns a test token address from environment or a default one
func getTestTokenAddress() string {
	token := os.Getenv("TEST_TOKEN_ADDRESS")
	if token == "" {
		// Default test token address - should exist on testnet
		token = "0xb64864f92faf8daa2f27949e9ef374907be0788b"
	}
	return token
}

// getTestTransactionHash returns a test transaction hash
func getTestTransactionHash() string {
	hash := os.Getenv("TEST_TX_HASH")
	if hash == "" {
		// Default test tx hash - should exist on testnet
		hash = "0xa80e74730f5e76d5014f73ac13ba5aa38bf2f0d54901c2b2b218a1a8adabf480"
	}
	return hash
}

// shouldSkipWriteTests checks if write operations should be skipped
// Write operations require a private key to be set
func shouldSkipWriteTests() bool {
	return os.Getenv("TEST_PRIVATE_KEY") == ""
}

// getTestPrivateKey returns the test private key for write operations
func getTestPrivateKey() string {
	return os.Getenv("TEST_PRIVATE_KEY")
}

// ============================================================================
// Integration Tests - Chains
// ============================================================================

func TestIntegration_GetChainId(t *testing.T) {
	env := getTestEnvironment(t)
	ctx := context.Background()

	t.Logf("Testing GetChainId on %s environment", env.Name)
	t.Logf("%s", env.Client.baseHost)

	result, err := env.Client.GetChainId(ctx)
	if err != nil {
		t.Fatalf("GetChainId failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result to not be nil")
	}

	if result.ChainId == 0 {
		t.Error("Expected ChainId to be non-zero")
	}

	t.Logf("✓ Successfully retrieved chain ID: %d", result.ChainId)
}

// ============================================================================
// Integration Tests - Accounts
// ============================================================================

func TestIntegration_GetAccountNonce(t *testing.T) {
	env := getTestEnvironment(t)
	ctx := context.Background()
	address := getTestAddress()

	t.Logf("Testing GetAccountNonce on %s environment", env.Name)
	t.Logf("Using address: %s", address)

	result, err := env.Client.GetAccountNonce(ctx, address)
	if err != nil {
		t.Fatalf("GetAccountNonce failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result to not be nil")
	}

	// Nonce can be 0 for new accounts
	t.Logf("✓ Successfully retrieved account nonce: %d", result.Nonce)
}

func TestIntegration_GetTokenAccount(t *testing.T) {
	env := getTestEnvironment(t)
	ctx := context.Background()
	address := getTestAddress()
	token := getTestTokenAddress()

	t.Logf("Testing GetTokenAccount on %s environment", env.Name)
	t.Logf("Using address: %s, token: %s", address, token)

	result, err := env.Client.GetTokenAccount(ctx, address, token)
	if err != nil {
		t.Fatalf("GetTokenAccount failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result to not be nil")
	}

	t.Logf("✓ Successfully retrieved token account:")
	t.Logf("  - Balance: %s", result.Balance)
	t.Logf("  - Nonce: %d", result.Nonce)
	t.Logf("  - Token Account Address: %s", result.TokenAccountAddress)
}

// ============================================================================
// Integration Tests - Checkpoints
// ============================================================================

func TestIntegration_GetCheckpointNumber(t *testing.T) {
	env := getTestEnvironment(t)
	ctx := context.Background()

	t.Logf("Testing GetCheckpointNumber on %s environment", env.Name)

	result, err := env.Client.GetCheckpointNumber(ctx)
	if err != nil {
		t.Fatalf("GetCheckpointNumber failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result to not be nil")
	}

	if result.Number <= 0 {
		t.Errorf("Expected number to be positive, got %d", result.Number)
	}

	t.Logf("✓ Successfully retrieved checkpoint number: %d", result.Number)
}

func TestIntegration_GetCheckpointByNumber(t *testing.T) {
	env := getTestEnvironment(t)
	ctx := context.Background()

	t.Logf("Testing GetCheckpointByNumber on %s environment", env.Name)

	// First get the current checkpoint number
	numResult, err := env.Client.GetCheckpointNumber(ctx)
	if err != nil {
		t.Fatalf("GetCheckpointNumber failed: %v", err)
	}

	// Query a recent checkpoint (10 checkpoints behind current)
	checkpointNum := numResult.Number - 10

	// Test without full transactions (default)
	result, err := env.Client.GetCheckpointByNumber(ctx, checkpointNum)
	if err != nil {
		t.Fatalf("GetCheckpointByNumber failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result to not be nil")
	}

	if result.Hash == "" {
		t.Error("Expected Hash to be present")
	}

	if result.Transactions.Hashes == nil {
		t.Error("Expected transaction hashes to be present")
	}

	t.Logf("✓ Successfully retrieved checkpoint by number (hashes only):")
	t.Logf("  - Number: %d", result.Number)
	t.Logf("  - Hash: %s", result.Hash)
	t.Logf("  - Transaction hashes count: %d", len(result.Transactions.Hashes))
}

func TestIntegration_GetCheckpointByNumberFull(t *testing.T) {
	env := getTestEnvironment(t)
	ctx := context.Background()

	t.Logf("Testing GetCheckpointByNumber with full transactions on %s environment", env.Name)

	// First get the current checkpoint number
	numResult, err := env.Client.GetCheckpointNumber(ctx)
	if err != nil {
		t.Fatalf("GetCheckpointNumber failed: %v", err)
	}

	// Query a recent checkpoint
	checkpointNum := numResult.Number - 10

	// Test with full transactions
	result, err := env.Client.GetCheckpointByNumber(ctx, checkpointNum, WithFullTransactions())
	if err != nil {
		t.Fatalf("GetCheckpointByNumber with full transactions failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result to not be nil")
	}

	if result.Transactions.Full == nil {
		t.Error("Expected full transactions to be present")
	}

	t.Logf("✓ Successfully retrieved checkpoint by number (full transactions):")
	t.Logf("  - Number: %d", result.Number)
	t.Logf("  - Full transactions count: %d", len(result.Transactions.Full))
}

func TestIntegration_GetCheckpointByHash(t *testing.T) {
	env := getTestEnvironment(t)
	ctx := context.Background()

	t.Logf("Testing GetCheckpointByHash on %s environment", env.Name)

	// First get a valid checkpoint hash
	numResult, err := env.Client.GetCheckpointNumber(ctx)
	if err != nil {
		t.Fatalf("GetCheckpointNumber failed: %v", err)
	}

	cpResult, err := env.Client.GetCheckpointByNumber(ctx, numResult.Number-10)
	if err != nil {
		t.Fatalf("GetCheckpointByNumber failed: %v", err)
	}

	hash := cpResult.Hash

	// Test without full transactions
	result, err := env.Client.GetCheckpointByHash(ctx, hash)
	if err != nil {
		t.Fatalf("GetCheckpointByHash failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result to not be nil")
	}

	if result.Hash != hash {
		t.Errorf("Expected hash %s, got %s", hash, result.Hash)
	}

	if result.Transactions.Hashes == nil {
		t.Error("Expected transaction hashes to be present")
	}

	t.Logf("✓ Successfully retrieved checkpoint by hash (hashes only):")
	t.Logf("  - Hash: %s", result.Hash)
	t.Logf("  - Transaction hashes count: %d", len(result.Transactions.Hashes))
}

func TestIntegration_GetCheckpointByHashFull(t *testing.T) {
	env := getTestEnvironment(t)
	ctx := context.Background()

	t.Logf("Testing GetCheckpointByHash with full transactions on %s environment", env.Name)

	// First get a valid checkpoint hash
	numResult, err := env.Client.GetCheckpointNumber(ctx)
	if err != nil {
		t.Fatalf("GetCheckpointNumber failed: %v", err)
	}

	cpResult, err := env.Client.GetCheckpointByNumber(ctx, numResult.Number-10)
	if err != nil {
		t.Fatalf("GetCheckpointByNumber failed: %v", err)
	}

	hash := cpResult.Hash

	// Test with full transactions
	result, err := env.Client.GetCheckpointByHash(ctx, hash, WithFullTransactions())
	if err != nil {
		t.Fatalf("GetCheckpointByHash with full transactions failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result to not be nil")
	}

	if result.Transactions.Full == nil {
		t.Error("Expected full transactions to be present")
	}

	t.Logf("✓ Successfully retrieved checkpoint by hash (full transactions):")
	t.Logf("  - Hash: %s", result.Hash)
	t.Logf("  - Full transactions count: %d", len(result.Transactions.Full))
}

// ============================================================================
// Integration Tests - Tokens
// ============================================================================

func TestIntegration_GetTokenMetadata(t *testing.T) {
	env := getTestEnvironment(t)
	ctx := context.Background()
	token := getTestTokenAddress()

	t.Logf("Testing GetTokenMetadata on %s environment", env.Name)
	t.Logf("Using token: %s", token)

	result, err := env.Client.GetTokenMetadata(ctx, token)
	if err != nil {
		t.Fatalf("GetTokenMetadata failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result to not be nil")
	}

	if result.Symbol == "" {
		t.Error("Expected Symbol to be present")
	}

	t.Logf("✓ Successfully retrieved token metadata:")
	t.Logf("  - Symbol: %s", result.Symbol)
	t.Logf("  - Decimals: %d", result.Decimals)
	t.Logf("  - Supply: %s", result.Supply)
	t.Logf("  - Master Authority: %s", result.MasterAuthority)
	t.Logf("  - Is Paused: %t", result.IsPaused)
	t.Logf("  - Is Private: %t", result.IsPrivate)
}

func TestIntegration_DeriveTokenAccountAddress(t *testing.T) {
	env := getTestEnvironment(t)
	address := getTestAddress()
	token := getTestTokenAddress()

	t.Logf("Testing DeriveTokenAccountAddress on %s environment", env.Name)
	t.Logf("Using address: %s, token: %s", address, token)

	walletAddr := common.HexToAddress(address)
	tokenAddr := common.HexToAddress(token)

	derivedAddr := env.Client.DeriveTokenAccountAddress(walletAddr, tokenAddr)

	if derivedAddr == (common.Address{}) {
		t.Error("Expected derived address to not be empty")
	}

	t.Logf("✓ Successfully derived token account address: %s", derivedAddr.Hex())
}

// ============================================================================
// Integration Tests - Transactions
// ============================================================================

func TestIntegration_GetTransactionByHash(t *testing.T) {
	env := getTestEnvironment(t)
	ctx := context.Background()
	hash := getTestTransactionHash()

	t.Logf("Testing GetTransactionByHash on %s environment", env.Name)
	t.Logf("Using tx hash: %s", hash)

	result, err := env.Client.GetTransactionByHash(ctx, hash)
	if err != nil {
		t.Fatalf("GetTransactionByHash failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result to not be nil")
	}

	if result.Hash == "" {
		t.Error("Expected Hash to be present")
	}

	if result.TransactionType == "" {
		t.Error("Expected TransactionType to be present")
	}

	t.Logf("✓ Successfully retrieved transaction by hash:")
	t.Logf("  - Hash: %s", result.Hash)
	t.Logf("  - Type: %s", result.TransactionType)
	t.Logf("  - From: %s", result.From.Hex())
	t.Logf("  - Nonce: %d", result.Nonce)
}

func TestIntegration_GetTransactionReceipt(t *testing.T) {
	env := getTestEnvironment(t)
	ctx := context.Background()
	hash := getTestTransactionHash()

	t.Logf("Testing GetTransactionReceipt on %s environment", env.Name)
	t.Logf("Using tx hash: %s", hash)

	result, err := env.Client.GetTransactionReceipt(ctx, hash)
	if err != nil {
		t.Fatalf("GetTransactionReceipt failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result to not be nil")
	}

	if result.TransactionHash == "" {
		t.Error("Expected TransactionHash to be present")
	}

	t.Logf("✓ Successfully retrieved transaction receipt:")
	t.Logf("  - Transaction Hash: %s", result.TransactionHash)
	t.Logf("  - Success: %t", result.Success)
	t.Logf("  - Fee Used: %s", result.FeeUsed)
	t.Logf("  - From: %s", result.From.Hex())
	if result.Recipient != nil {
		t.Logf("  - Recipient: %s", result.Recipient.Hex())
	} else {
		t.Logf("  - Recipient: null")
	}
	if result.TokenAddress != nil {
		t.Logf("  - Token Address: %s", result.TokenAddress.Hex())
	} else {
		t.Logf("  - Token Address: null")
	}
}

func TestIntegration_GetEstimateFee(t *testing.T) {
	env := getTestEnvironment(t)
	ctx := context.Background()
	from := common.HexToAddress(getTestAddress())
	token := common.HexToAddress(getTestTokenAddress())
	value := "1000000"

	t.Logf("Testing GetEstimateFee on %s environment", env.Name)
	t.Logf("Using from: %s, token: %s, value: %s", from.Hex(), token.Hex(), value)

	result, err := env.Client.GetEstimateFee(ctx, from, token, value)
	if err != nil {
		t.Fatalf("GetEstimateFee failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result to not be nil")
	}

	if result.Fee == "" {
		t.Error("Expected Fee to be present")
	}

	t.Logf("✓ Successfully retrieved fee estimate: %s", result.Fee)
}

// ============================================================================
// Integration Tests - Write Operations (require private key)
// ============================================================================

func TestIntegration_SendPayment(t *testing.T) {
	if shouldSkipWriteTests() {
		t.Skip("Skipping write test: TEST_PRIVATE_KEY not set")
	}

	env := getTestEnvironment(t)
	ctx := context.Background()
	privateKey := getTestPrivateKey()
	token := getTestTokenAddress()

	t.Logf("Testing SendPayment on %s environment", env.Name)

	// Get sender address from private key
	senderAddr, err := PrivateKeyToAddress(privateKey)
	if err != nil {
		t.Fatalf("Failed to get address from private key: %v", err)
	}

	// Get chain ID
	chainIDResp, err := env.Client.GetChainId(ctx)
	if err != nil {
		t.Fatalf("Failed to get chain ID: %v", err)
	}

	// Get nonce
	nonceResp, err := env.Client.GetAccountNonce(ctx, senderAddr)
	if err != nil {
		t.Fatalf("Failed to get nonce: %v", err)
	}

	// Get recent checkpoint
	checkpointResp, err := env.Client.GetCheckpointNumber(ctx)
	if err != nil {
		t.Fatalf("Failed to get checkpoint: %v", err)
	}

	// Prepare payment
	value := big.NewInt(1) // Send 1 unit (adjust based on token decimals)
	recipientAddr := common.HexToAddress(getTestAddress())
	tokenAddr := common.HexToAddress(token)

	payload := PaymentPayload{
		RecentCheckpoint: uint64(checkpointResp.Number),
		ChainID:          chainIDResp.ChainId,
		Nonce:            nonceResp.Nonce,
		Recipient:        recipientAddr,
		Value:            value,
		Token:            tokenAddr,
	}

	// Sign the payload
	signature, err := env.Client.SignMessage(payload, privateKey)
	if err != nil {
		t.Fatalf("Failed to sign payload: %v", err)
	}

	request := &PaymentRequest{
		PaymentPayload: payload,
		Signature:      *signature,
	}

	// Send payment
	result, err := env.Client.SendPayment(ctx, request)
	if err != nil {
		t.Fatalf("SendPayment failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result to not be nil")
	}

	if result.Hash == "" {
		t.Error("Expected Hash to be present")
	}

	t.Logf("✓ Successfully sent payment:")
	t.Logf("  - Transaction Hash: %s", result.Hash)
}

// ============================================================================
// Test Runner - Run all integration tests
// ============================================================================

func TestIntegration_AllAPIs(t *testing.T) {
	env := getTestEnvironment(t)
	t.Logf("Running comprehensive integration tests on %s environment", env.Name)

	// Run all sub-tests
	t.Run("Chains", func(t *testing.T) {
		t.Run("GetChainId", TestIntegration_GetChainId)
	})

	t.Run("Accounts", func(t *testing.T) {
		t.Run("GetAccountNonce", TestIntegration_GetAccountNonce)
		t.Run("GetTokenAccount", TestIntegration_GetTokenAccount)
	})

	t.Run("Checkpoints", func(t *testing.T) {
		t.Run("GetCheckpointNumber", TestIntegration_GetCheckpointNumber)
		t.Run("GetCheckpointByNumber", TestIntegration_GetCheckpointByNumber)
		t.Run("GetCheckpointByNumberFull", TestIntegration_GetCheckpointByNumberFull)
		t.Run("GetCheckpointByHash", TestIntegration_GetCheckpointByHash)
		t.Run("GetCheckpointByHashFull", TestIntegration_GetCheckpointByHashFull)
	})

	t.Run("Tokens", func(t *testing.T) {
		t.Run("GetTokenMetadata", TestIntegration_GetTokenMetadata)
		t.Run("DeriveTokenAccountAddress", TestIntegration_DeriveTokenAccountAddress)
	})

	t.Run("Transactions", func(t *testing.T) {
		t.Run("GetTransactionByHash", TestIntegration_GetTransactionByHash)
		t.Run("GetTransactionReceipt", TestIntegration_GetTransactionReceipt)
		t.Run("GetEstimateFee", TestIntegration_GetEstimateFee)
	})

	if !shouldSkipWriteTests() {
		t.Run("WriteOperations", func(t *testing.T) {
			t.Run("SendPayment", TestIntegration_SendPayment)
		})
	} else {
		t.Log("⚠️  Skipping write operations tests (TEST_PRIVATE_KEY not set)")
	}

	t.Logf("✅ All integration tests completed on %s environment", env.Name)
}
