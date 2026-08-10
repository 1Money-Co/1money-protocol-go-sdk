//go:build integration

package onemoney

import (
	"context"
	"fmt"
	"math/big"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
)

// debugEnabled reports whether verbose SDK request/response logging should be
// enabled for the business-flow tests, toggled by the TEST_DEBUG env var
// (1/true/yes/on, case-insensitive). When on, the client is built with
// WithDebug so every call's method, URL, request body, and response status and
// body are printed.
func debugEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("TEST_DEBUG"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// TestAccount represents a test account with its private key and a Signer built
// from it. The v2 namespace API signs through the Signer; the raw PrivateKey is
// retained only for logging and for constructing new signers.
type TestAccount struct {
	PrivateKey string
	Address    common.Address
	Signer     Signer
}

// BusinessFlowTestSuite holds the test environment and accounts
type BusinessFlowTestSuite struct {
	Client           *Client
	OperatorAccount  *TestAccount // Network operator for issuing tokens
	MasterAccount    *TestAccount // Master authority for token management
	Account1         *TestAccount
	Account2         *TestAccount
	ChainID          uint64
	RecentCheckpoint uint64
	t                *testing.T
}

// setupBusinessFlowTest initializes the test suite
func setupBusinessFlowTest(t *testing.T) *BusinessFlowTestSuite {
	// Check if operator private key is set (required for issuing tokens)
	operatorPrivateKey := os.Getenv("TEST_OPERATOR_PRIVATE_KEY")
	if operatorPrivateKey == "" {
		t.Skip("TEST_OPERATOR_PRIVATE_KEY not set, skipping business flow tests")
	}

	// Check if master private key is set (required for token management)
	masterPrivateKey := os.Getenv("TEST_MASTER_PRIVATE_KEY")
	if masterPrivateKey == "" {
		t.Skip("TEST_MASTER_PRIVATE_KEY not set, skipping business flow tests")
	}

	// Get environment
	env := os.Getenv("TEST_ENV")
	if env == "" {
		env = "testnet"
	}

	// Assemble client options. TEST_DEBUG turns on verbose request/response
	// logging (method, URL, request body, response status + body) for every SDK
	// call via the SDK's WithDebug option.
	opts := []ClientOption{WithTimeout(30 * time.Second)}
	if debugEnabled() {
		opts = append(opts, WithDebug())
		t.Log("TEST_DEBUG enabled: printing request/response for every SDK call")
	}

	var client *Client
	switch env {
	case "local":
		url := os.Getenv("LOCAL_API_URL")
		if url == "" {
			url = localEndpoint
		}
		client = NewClientWithCustomUrl(url, opts...)
	case "testnet":
		client = NewTestClientWithOpts(opts...)
	default:
		t.Fatalf("Unsupported TEST_ENV for business flow tests: %s", env)
	}

	operatorAccount := newTestAccount(t, operatorPrivateKey)
	masterAccount := newTestAccount(t, masterPrivateKey)

	suite := &BusinessFlowTestSuite{
		Client:          client,
		OperatorAccount: operatorAccount,
		MasterAccount:   masterAccount,
		t:               t,
	}

	// Get chain ID
	ctx := context.Background()
	chainIDResp, err := client.GetChainId(ctx)
	assert.Nil(t, err, "Should get chain ID from network")
	suite.ChainID = chainIDResp.ChainId

	// Get recent checkpoint
	checkpointResp, err := client.GetCheckpointNumber(ctx)
	assert.Nil(t, err, "Should get checkpoint number from network")
	suite.RecentCheckpoint = uint64(checkpointResp.Number)

	// Generate test accounts if not provided
	suite.Account1 = suite.generateOrGetAccount("TEST_ACCOUNT1_PRIVATE_KEY")
	suite.Account2 = suite.generateOrGetAccount("TEST_ACCOUNT2_PRIVATE_KEY")

	t.Logf("Business Flow Test Suite Initialized:")
	t.Logf("  - Environment: %s", env)
	t.Logf("  - Chain ID: %d", suite.ChainID)
	t.Logf("  - Recent Checkpoint: %d", suite.RecentCheckpoint)
	t.Logf("  - Operator Account: %s (for issuing tokens)", suite.OperatorAccount.Address.Hex())
	t.Logf("  - Master Account: %s (master authority)", suite.MasterAccount.Address.Hex())
	t.Logf("  - Test Account 1: %s", suite.Account1.Address.Hex())
	t.Logf("  - Test Account 2: %s", suite.Account2.Address.Hex())

	return suite
}

// newTestAccount builds a TestAccount (address + Signer) from a hex private key.
func newTestAccount(t *testing.T, privateKeyHex string) *TestAccount {
	t.Helper()
	signer, err := NewPrivateKeySigner(privateKeyHex)
	assert.Nil(t, err, "Should build signer from private key")
	return &TestAccount{
		PrivateKey: privateKeyHex,
		Address:    signer.Address(),
		Signer:     signer,
	}
}

// generateOrGetAccount generates a new account or uses an existing one from env
func (s *BusinessFlowTestSuite) generateOrGetAccount(envVar string) *TestAccount {
	privateKeyHex := os.Getenv(envVar)
	if privateKeyHex != "" {
		return newTestAccount(s.t, privateKeyHex)
	}

	// Generate new account
	privateKey, err := crypto.GenerateKey()
	assert.Nil(s.t, err, "Should random generate private key")

	privateKeyHex = fmt.Sprintf("%x", crypto.FromECDSA(privateKey))
	account := newTestAccount(s.t, privateKeyHex)

	if envVar != "" {
		s.t.Logf("Generated new account for %s: %s", envVar, account.Address.Hex())
	} else {
		s.t.Logf("Generated new account: %s", account.Address.Hex())
	}
	return account
}

// refreshCheckpoint updates the recent checkpoint
func (s *BusinessFlowTestSuite) refreshCheckpoint() {
	ctx := context.Background()
	checkpointResp, err := s.Client.GetCheckpointNumber(ctx)
	assert.Nil(s.t, err, "Should refresh the checkpoint number")
	s.RecentCheckpoint = uint64(checkpointResp.Number)
}

// getNonce gets the current nonce for an account
func (s *BusinessFlowTestSuite) getNonce(address common.Address) uint64 {
	ctx := context.Background()
	nonceResp, err := s.Client.GetAccountNonce(ctx, address)
	assert.Nil(s.t, err, fmt.Sprintf("Should refresh the nonce of account %s", address.Hex()))
	return nonceResp.Nonce
}

// getTokenBalance gets the token balance for an account
func (s *BusinessFlowTestSuite) getTokenBalance(address common.Address, token common.Address) string {
	ctx := context.Background()
	accountResp, err := s.Client.GetTokenAccount(ctx, address, token)
	if err != nil {
		// Return "0" if account doesn't exist yet
		return "0"
	}
	return accountResp.Balance
}

// waitForTransaction waits for a transaction to be confirmed and retrieves transaction details
// u64OrZero dereferences an optional uint64 for logging, yielding 0 when nil.
func u64OrZero(p *uint64) uint64 {
	if p == nil {
		return 0
	}
	return *p
}

func (s *BusinessFlowTestSuite) waitForTransaction(txHash string, maxWait time.Duration) *TransactionReceiptResponse {
	s.t.Logf("⏳ Waiting for transaction %s to be confirmed...", txHash)
	ctx := context.Background()
	start := time.Now()

	for {
		receipt, err := s.Client.GetTransactionReceipt(ctx, txHash)
		if err == nil {
			elapsed := time.Since(start)
			if receipt.Success {
				s.t.Logf("✅ Transaction confirmed in %.2fs (checkpoint %d)", elapsed.Seconds(), u64OrZero(receipt.CheckpointNumber))
			} else {
				s.t.Logf("❌ Transaction failed in %.2fs (checkpoint %d)", elapsed.Seconds(), u64OrZero(receipt.CheckpointNumber))
			}

			// Get transaction details for additional verification and logging
			tx, err := s.Client.GetTransactionByHash(ctx, txHash)
			if err != nil {
				s.t.Logf("⚠️  Warning: Could not retrieve transaction details: %v", err)
			} else {
				// Log transaction details
				s.t.Logf("📋 Transaction details:")
				s.t.Logf("   - From: %s", tx.From.Hex())
				s.t.Logf("   - Type: %s", tx.TransactionType)
				s.t.Logf("   - Nonce: %d", tx.Nonce)
				s.t.Logf("   - Chain ID: %d", tx.ChainID)
				s.t.Logf("   - Checkpoint: %d", u64OrZero(tx.CheckpointNumber))
			}

			return receipt
		}

		if time.Since(start) > maxWait {
			s.t.Fatalf("Transaction %s not confirmed after %v", txHash, maxWait)
		}

		time.Sleep(1 * time.Second)
	}
}

func (s *BusinessFlowTestSuite) fetchTransaction(t *testing.T, hash string) *Transaction {
	t.Helper()
	ctx := context.Background()
	tx, err := s.Client.GetTransactionByHash(ctx, hash)
	assert.NoErrorf(t, err, "failed to fetch transaction %s", hash)
	return tx
}

func (s *BusinessFlowTestSuite) assertReceiptBasics(t *testing.T, receipt *TransactionReceiptResponse, expectedHash string, expectedFrom common.Address) {
	t.Helper()
	assert.Equalf(t, expectedHash, receipt.TransactionHash, "receipt hash mismatch")
	assert.Equalf(t, expectedFrom, receipt.From, "receipt from mismatch")
	assert.NotEmptyf(t, receipt.FeeUsed, "expected fee used to be populated")
}

// ============================================================================
// Test: Complete Token Lifecycle
// ============================================================================

func TestBusinessFlow_CompleteTokenLifecycle(t *testing.T) {
	suite := setupBusinessFlowTest(t)
	ctx := context.Background()

	t.Run("1. Issue New Token", func(t *testing.T) {
		suite.refreshCheckpoint()

		// Generate unique symbol
		symbol := fmt.Sprintf("TEST%d", time.Now().Unix()%100000)
		name := fmt.Sprintf("Test Token %d", time.Now().Unix()%100000)

		payload := TokenIssuePayload{
			ChainID:         suite.ChainID,
			Nonce:           suite.getNonce(suite.OperatorAccount.Address), // Use operator nonce
			Symbol:          symbol,
			Name:            name,
			Decimals:        6,
			MasterAuthority: suite.MasterAccount.Address, // Master authority for token management
			IsPrivate:       false,
			ClawbackEnabled: false,
		}

		t.Logf("📝 Issuing token: %s (%s)", symbol, name)
		t.Logf("   - Signed by operator: %s", suite.OperatorAccount.Address.Hex())
		t.Logf("   - Master authority: %s", suite.MasterAccount.Address.Hex())
		result, err := suite.Client.Tokens().Issue(ctx, payload, suite.OperatorAccount.Signer)
		if err != nil {
			t.Fatalf("Failed to issue token: %v", err)
		}

		t.Logf("✅ Token issued successfully")
		t.Logf("   - Transaction Hash: %s", result.Hash)
		t.Logf("   - Token Address: %s", result.Token)

		// Wait for confirmation
		receipt := suite.waitForTransaction(result.Hash, 10*time.Second)
		if !receipt.Success {
			t.Fatal("Token issuance transaction failed")
		}

		suite.assertReceiptBasics(t, receipt, result.Hash, suite.OperatorAccount.Address)

		tokenAddr := common.HexToAddress(result.Token)
		assert.NotNilf(t, receipt.TokenAddress, "expected token address in receipt")
		if receipt.TokenAddress != nil {
			assert.Equalf(t, tokenAddr, *receipt.TokenAddress, "receipt token address mismatch")
		}
		assert.Nilf(t, receipt.Recipient, "expected recipient to be nil for token issue")

		tx := suite.fetchTransaction(t, result.Hash)
		assert.Equalf(t, TransactionTypeTokenCreate, tx.TransactionType, "unexpected transaction type")
		assert.Equalf(t, suite.OperatorAccount.Address, tx.From, "unexpected transaction sender")
		if createData, ok := tx.AsTokenCreateData(); ok {
			assert.Equalf(t, symbol, createData.Symbol, "unexpected token symbol in transaction")
			assert.Equalf(t, suite.MasterAccount.Address, createData.MasterAuthority, "unexpected master authority")
		}

		// Verify token metadata
		metadata, err := suite.Client.Tokens().Metadata(ctx, tokenAddr)
		if err != nil {
			t.Fatalf("Failed to get token metadata: %v", err)
		}

		if metadata.Symbol != symbol {
			t.Errorf("Expected symbol %s, got %s", symbol, metadata.Symbol)
		}
		if metadata.Decimals != 6 {
			t.Errorf("Expected decimals 6, got %d", metadata.Decimals)
		}
		if metadata.Supply != "0" {
			t.Errorf("Expected initial supply 0, got %s", metadata.Supply)
		}

		t.Logf("✅ Token metadata verified")

		// Store token address for subsequent tests
		suite.t = t // Update t reference for subtest context

		// Generate a minter account to receive mint/burn authority
		minterAccount := suite.generateOrGetAccount("")
		t.Logf("Generated minter account: %s", minterAccount.Address.Hex())

		t.Run("2. Grant Mint Authority", func(t *testing.T) {
			suite.refreshCheckpoint()

			payload := TokenAuthorityPayload{
				ChainID:          suite.ChainID,
				Nonce:            suite.getNonce(suite.MasterAccount.Address),
				AuthorityType:    AuthorityTypeMintBurnTokens,
				AuthorityAddress: minterAccount.Address, // Grant to the new minter account
				Token:            tokenAddr,
				Value:            big.NewInt(1000000000000), // 1M tokens with 6 decimals
			}

			t.Logf("🔐 Granting mint authority from master to minter account")
			t.Logf("   - Master authority: %s", suite.MasterAccount.Address.Hex())
			t.Logf("   - Minter account: %s", minterAccount.Address.Hex())
			result, err := suite.Client.Tokens().GrantAuthority(ctx, payload, suite.MasterAccount.Signer)
			if err != nil {
				t.Fatalf("Failed to grant authority: %v", err)
			}

			receipt := suite.waitForTransaction(result.Hash, 60*time.Second)
			if !receipt.Success {
				t.Fatal("Grant authority transaction failed")
			}

			t.Logf("✅ Mint authority granted to minter account")
		})

		t.Run("3. Mint Tokens", func(t *testing.T) {
			suite.refreshCheckpoint()

			mintAmount := big.NewInt(100000000) // 100 tokens (6 decimals)

			payload := TokenMintPayload{
				ChainID:   suite.ChainID,
				Nonce:     suite.getNonce(minterAccount.Address), // Use minter account nonce
				Recipient: suite.Account1.Address,
				Value:     mintAmount,
				Token:     tokenAddr,
			}

			t.Logf("💰 Minting %s tokens to %s", mintAmount.String(), suite.Account1.Address.Hex())
			t.Logf("   - Minted by: %s", minterAccount.Address.Hex())
			result, err := suite.Client.Tokens().Mint(ctx, payload, minterAccount.Signer)
			if err != nil {
				t.Fatalf("Failed to mint token: %v", err)
			}

			receipt := suite.waitForTransaction(result.Hash, 60*time.Second)
			if !receipt.Success {
				t.Fatal("Mint token transaction failed")
			}

			suite.assertReceiptBasics(t, receipt, result.Hash, minterAccount.Address)
			if receipt.TokenAddress != nil {
				assert.Equalf(t, tokenAddr, *receipt.TokenAddress, "receipt token address mismatch")
			}
			assert.NotNilf(t, receipt.Recipient, "expected recipient in mint receipt")
			if receipt.Recipient != nil {
				assert.Equalf(t, suite.Account1.Address, *receipt.Recipient, "receipt recipient mismatch")
			}

			tx := suite.fetchTransaction(t, result.Hash)
			assert.Equalf(t, TransactionTypeTokenMint, tx.TransactionType, "unexpected transaction type for mint")
			assert.Equalf(t, minterAccount.Address, tx.From, "unexpected mint sender")
			if data, ok := tx.AsTokenMintData(); ok {
				assert.Equalf(t, suite.Account1.Address, data.Recipient, "mint recipient mismatch")
				assert.Equalf(t, tokenAddr, data.Token, "mint token mismatch")
				valueInt, ok := new(big.Int).SetString(data.Value, 10)
				assert.Truef(t, ok, "failed to parse mint value %s", data.Value)
				assert.Zero(t, valueInt.Cmp(mintAmount), "mint amount mismatch")
			}

			// Verify balance
			balance := suite.getTokenBalance(suite.Account1.Address, tokenAddr)
			if balance != mintAmount.String() {
				t.Errorf("Expected balance %s, got %s", mintAmount.String(), balance)
			}

			t.Logf("✅ Tokens minted and balance verified: %s", balance)
		})

		t.Run("4. Transfer Tokens", func(t *testing.T) {
			t.Logf("do transfer Tokens ......")
			suite.refreshCheckpoint()

			transferAmount := big.NewInt(50000000) // 50 tokens

			payload := PaymentPayload{
				ChainID:   suite.ChainID,
				Nonce:     suite.getNonce(suite.Account1.Address),
				Recipient: suite.Account2.Address,
				Value:     transferAmount,
				Token:     tokenAddr,
			}

			t.Logf("💸 Transferring %s tokens from Account1 to Account2", transferAmount.String())
			result, err := suite.Client.Transactions().Payment(ctx, payload, suite.Account1.Signer)
			if err != nil {
				t.Fatalf("Failed to send payment: %v", err)
			}

			receipt := suite.waitForTransaction(result.Hash, 60*time.Second)
			if !receipt.Success {
				t.Fatal("Payment transaction failed")
			}

			suite.assertReceiptBasics(t, receipt, result.Hash, suite.Account1.Address)
			if receipt.TokenAddress != nil {
				assert.Equalf(t, tokenAddr, *receipt.TokenAddress, "receipt token address mismatch")
			}
			assert.NotNilf(t, receipt.Recipient, "expected recipient in payment receipt")
			if receipt.Recipient != nil {
				assert.Equalf(t, suite.Account2.Address, *receipt.Recipient, "payment recipient mismatch")
			}

			tx := suite.fetchTransaction(t, result.Hash)
			if payload, ok := tx.AsTokenTransferData(); ok {
				assert.Equalf(t, suite.Account2.Address, payload.Recipient, "transfer recipient mismatch")
				if assert.NotNilf(t, payload.Token, "expected token in transfer payload") {
					assert.Equalf(t, tokenAddr, *payload.Token, "transfer token mismatch")
				}
				assert.Equalf(t, transferAmount.String(), payload.Value, "transfer value mismatch")
			}

			// Verify balances
			balance1 := suite.getTokenBalance(suite.Account1.Address, tokenAddr)
			balance2 := suite.getTokenBalance(suite.Account2.Address, tokenAddr)

			expectedBalance1 := big.NewInt(50000000) // 100 - 50 = 50 tokens
			expectedBalance2 := transferAmount       // 50 tokens

			if balance1 != expectedBalance1.String() {
				t.Errorf("Account1: expected balance %s, got %s", expectedBalance1.String(), balance1)
			}
			if balance2 != expectedBalance2.String() {
				t.Errorf("Account2: expected balance %s, got %s", expectedBalance2.String(), balance2)
			}

			t.Logf("✅ Transfer completed and balances verified")
			t.Logf("   - Account1 balance: %s", balance1)
			t.Logf("   - Account2 balance: %s", balance2)
		})

		t.Run("5. Burn Tokens", func(t *testing.T) {
			suite.refreshCheckpoint()

			// Transfer token to account that has burn authority
			tPayload := PaymentPayload{
				ChainID:   suite.ChainID,
				Nonce:     suite.getNonce(suite.Account2.Address),
				Recipient: minterAccount.Address,
				Value:     big.NewInt(25000000), // 25 tokens
				Token:     tokenAddr,
			}
			tResult, err := suite.Client.Transactions().Payment(ctx, tPayload, suite.Account2.Signer)
			if err != nil {
				t.Fatalf("Failed to send payment: %v", err)
			}

			tReceipt := suite.waitForTransaction(tResult.Hash, 60*time.Second)
			if !tReceipt.Success {
				t.Fatal("Payment transaction failed")
			}
			suite.assertReceiptBasics(t, tReceipt, tResult.Hash, suite.Account2.Address)
			txTransfer := suite.fetchTransaction(t, tResult.Hash)
			if payload, ok := txTransfer.AsTokenTransferData(); ok {
				assert.Equalf(t, minterAccount.Address, payload.Recipient, "transfer-to-minter recipient mismatch")
				if assert.NotNilf(t, payload.Token, "expected token in transfer-to-minter payload") {
					assert.Equalf(t, tokenAddr, *payload.Token, "transfer-to-minter token mismatch")
				}
			}
			// Verify balances
			minterBalance := suite.getTokenBalance(minterAccount.Address, tokenAddr)
			if minterBalance != big.NewInt(25000000).String() {
				t.Errorf("MinterAccount: expected balance 25000000, got %s", minterBalance)
			}

			burnAmount := big.NewInt(25000000) // 25 tokens

			payload := TokenBurnPayload{
				ChainID: suite.ChainID,
				Nonce:   suite.getNonce(minterAccount.Address), // Use minter account nonce
				Value:   burnAmount,
				Token:   tokenAddr,
			}

			balanceBefore := suite.getTokenBalance(minterAccount.Address, tokenAddr)
			t.Logf("🔥 Burning %s tokens from Account2 (current balance: %s)", burnAmount.String(), balanceBefore)
			t.Logf("   - Burned by: %s", minterAccount.Address.Hex())

			result, err := suite.Client.Tokens().Burn(ctx, payload, minterAccount.Signer)
			if err != nil {
				t.Fatalf("Failed to burn token: %v", err)
			}

			receipt := suite.waitForTransaction(result.Hash, 60*time.Second)
			if !receipt.Success {
				t.Fatal("Burn token transaction failed")
			}

			suite.assertReceiptBasics(t, receipt, result.Hash, minterAccount.Address)
			if receipt.TokenAddress != nil {
				assert.Equalf(t, tokenAddr, *receipt.TokenAddress, "burn receipt token mismatch")
			}
			assert.Nilf(t, receipt.Recipient, "expected burn receipt recipient")
			if receipt.Recipient != nil {
				assert.Equalf(t, minterAccount.Address, *receipt.Recipient, "burn receipt recipient mismatch")
			}

			txBurn := suite.fetchTransaction(t, result.Hash)
			if data, ok := txBurn.AsTokenBurnData(); ok {
				valueInt, ok := new(big.Int).SetString(data.Value, 10)
				assert.Truef(t, ok, "failed to parse burn value %s", data.Value)
				assert.Zero(t, valueInt.Cmp(burnAmount), "burn amount mismatch")
			}

			// Verify balance decreased
			balanceAfter := suite.getTokenBalance(suite.Account2.Address, tokenAddr)
			expectedBalance := big.NewInt(25000000) // 50 - 25 = 25 tokens

			if balanceAfter != expectedBalance.String() {
				t.Errorf("Expected balance %s after burn, got %s", expectedBalance.String(), balanceAfter)
			}

			t.Logf("✅ Tokens burned and balance verified: %s", balanceAfter)
		})

		t.Run("6. Revoke Mint Authority", func(t *testing.T) {
			suite.refreshCheckpoint()

			payload := TokenAuthorityPayload{
				ChainID:          suite.ChainID,
				Nonce:            suite.getNonce(suite.MasterAccount.Address),
				AuthorityType:    AuthorityTypeMintBurnTokens,
				AuthorityAddress: minterAccount.Address, // Revoke from the minter account
				Token:            tokenAddr,
				Value:            big.NewInt(0),
			}

			t.Logf("🔒 Revoking mint authority from minter account")
			t.Logf("   - Revoked by master: %s", suite.MasterAccount.Address.Hex())
			t.Logf("   - Revoked from: %s", minterAccount.Address.Hex())
			result, err := suite.Client.Tokens().RevokeAuthority(ctx, payload, suite.MasterAccount.Signer)
			if err != nil {
				t.Fatalf("Failed to revoke authority: %v", err)
			}

			receipt := suite.waitForTransaction(result.Hash, 60*time.Second)
			if !receipt.Success {
				t.Fatal("Revoke authority transaction failed")
			}

			t.Logf("✅ Mint authority revoked from minter account")
		})
	})

	t.Logf("\n🎉 Complete token lifecycle test passed!")
}

// issueTokenForTest issues a token signed by the operator and returns its
// address, sharing the boilerplate across the focused business-flow tests.
func (s *BusinessFlowTestSuite) issueTokenForTest(t *testing.T, ctx context.Context, prefix, name string, private, clawback bool) common.Address {
	t.Helper()
	s.refreshCheckpoint()
	symbol := fmt.Sprintf("%s%d", prefix, time.Now().Unix()%100000)
	payload := TokenIssuePayload{
		ChainID:         s.ChainID,
		Nonce:           s.getNonce(s.OperatorAccount.Address),
		Symbol:          symbol,
		Name:            name,
		Decimals:        6,
		MasterAuthority: s.MasterAccount.Address,
		IsPrivate:       private,
		ClawbackEnabled: clawback,
	}
	result, err := s.Client.Tokens().Issue(ctx, payload, s.OperatorAccount.Signer)
	if err != nil {
		t.Fatalf("Failed to issue token: %v", err)
	}
	receipt := s.waitForTransaction(result.Hash, 60*time.Second)
	if !receipt.Success {
		t.Fatal("Token issuance transaction failed")
	}
	tokenAddr := common.HexToAddress(result.Token)
	t.Logf("✅ Token issued: %s (%s)", symbol, tokenAddr.Hex())
	return tokenAddr
}

// grantAuthority grants a token authority from the master account and waits for
// confirmation.
func (s *BusinessFlowTestSuite) grantAuthority(t *testing.T, ctx context.Context, authorityType AuthorityType, to, token common.Address, value *big.Int) {
	t.Helper()
	s.refreshCheckpoint()
	payload := TokenAuthorityPayload{
		ChainID:          s.ChainID,
		Nonce:            s.getNonce(s.MasterAccount.Address),
		AuthorityType:    authorityType,
		AuthorityAddress: to,
		Token:            token,
		Value:            value,
	}
	result, err := s.Client.Tokens().GrantAuthority(ctx, payload, s.MasterAccount.Signer)
	if err != nil {
		t.Fatalf("Failed to grant %s authority: %v", authorityType, err)
	}
	receipt := s.waitForTransaction(result.Hash, 60*time.Second)
	if !receipt.Success {
		t.Fatalf("Grant %s authority transaction failed", authorityType)
	}
	t.Logf("🔐 Granted %s authority to %s", authorityType, to.Hex())
}

// ============================================================================
// Test: Token Pause and Unpause
// ============================================================================

func TestBusinessFlow_TokenPauseUnpause(t *testing.T) {
	suite := setupBusinessFlowTest(t)
	ctx := context.Background()

	tokenAddr := suite.issueTokenForTest(t, ctx, "PAUSE", "Pause Test Token", false, false)
	suite.grantAuthority(t, ctx, AuthorityTypePause, suite.MasterAccount.Address, tokenAddr, big.NewInt(0))

	t.Run("1. Pause Token", func(t *testing.T) {
		suite.refreshCheckpoint()

		payload := PauseTokenPayload{
			ChainID: suite.ChainID,
			Nonce:   suite.getNonce(suite.MasterAccount.Address),
			Token:   tokenAddr,
		}

		t.Log("⏸️  Pausing token")
		result, err := suite.Client.Tokens().Pause(ctx, payload, suite.MasterAccount.Signer)
		if err != nil {
			t.Fatalf("Failed to pause token: %v", err)
		}

		receipt := suite.waitForTransaction(result.Hash, 60*time.Second)
		if !receipt.Success {
			t.Fatal("Pause token transaction failed")
		}

		metadata, err := suite.Client.Tokens().Metadata(ctx, tokenAddr)
		if err != nil {
			t.Fatalf("Failed to get token metadata: %v", err)
		}
		if !metadata.IsPaused {
			t.Error("Token should be paused but is not")
		}

		t.Log("✅ Token paused successfully")
	})

	t.Run("2. Unpause Token", func(t *testing.T) {
		suite.refreshCheckpoint()

		payload := PauseTokenPayload{
			ChainID: suite.ChainID,
			Nonce:   suite.getNonce(suite.MasterAccount.Address),
			Token:   tokenAddr,
		}

		t.Log("▶️  Unpausing token")
		result, err := suite.Client.Tokens().Unpause(ctx, payload, suite.MasterAccount.Signer)
		if err != nil {
			t.Fatalf("Failed to unpause token: %v", err)
		}

		receipt := suite.waitForTransaction(result.Hash, 60*time.Second)
		if !receipt.Success {
			t.Fatal("Unpause token transaction failed")
		}

		metadata, err := suite.Client.Tokens().Metadata(ctx, tokenAddr)
		if err != nil {
			t.Fatalf("Failed to get token metadata: %v", err)
		}
		if metadata.IsPaused {
			t.Error("Token should not be paused but is")
		}

		t.Log("✅ Token unpaused successfully")
	})

	t.Log("\n🎉 Token pause/unpause test passed!")
}

// ============================================================================
// Test: Whitelist Management (private token)
// ============================================================================

func TestBusinessFlow_WhitelistManagement(t *testing.T) {
	suite := setupBusinessFlowTest(t)
	ctx := context.Background()

	tokenAddr := suite.issueTokenForTest(t, ctx, "PRIV", "Private Test Token", true, false)
	suite.grantAuthority(t, ctx, AuthorityTypeManageList, suite.MasterAccount.Address, tokenAddr, big.NewInt(0))

	t.Run("1. Add Address to Whitelist", func(t *testing.T) {
		suite.refreshCheckpoint()

		payload := TokenManageListPayload{
			ChainID: suite.ChainID,
			Nonce:   suite.getNonce(suite.MasterAccount.Address),
			Action:  ManageListActionAdd,
			Address: suite.Account1.Address,
			Token:   tokenAddr,
		}

		t.Logf("➕ Adding %s to whitelist", suite.Account1.Address.Hex())
		result, err := suite.Client.Tokens().ManageWhitelist(ctx, payload, suite.MasterAccount.Signer)
		if err != nil {
			t.Fatalf("Failed to add to whitelist: %v", err)
		}

		receipt := suite.waitForTransaction(result.Hash, 60*time.Second)
		if !receipt.Success {
			t.Fatal("Add to whitelist transaction failed")
		}

		metadata, err := suite.Client.Tokens().Metadata(ctx, tokenAddr)
		if err != nil {
			t.Fatalf("Failed to get token metadata: %v", err)
		}
		found := false
		for _, addr := range metadata.WhiteList {
			if common.HexToAddress(addr) == suite.Account1.Address {
				found = true
				break
			}
		}
		if !found {
			t.Error("Address not found in whitelist")
		}

		t.Log("✅ Address added to whitelist")
	})

	t.Run("2. Remove Address from Whitelist", func(t *testing.T) {
		suite.refreshCheckpoint()

		payload := TokenManageListPayload{
			ChainID: suite.ChainID,
			Nonce:   suite.getNonce(suite.MasterAccount.Address),
			Action:  ManageListActionRemove,
			Address: suite.Account1.Address,
			Token:   tokenAddr,
		}

		t.Logf("➖ Removing %s from whitelist", suite.Account1.Address.Hex())
		result, err := suite.Client.Tokens().ManageWhitelist(ctx, payload, suite.MasterAccount.Signer)
		if err != nil {
			t.Fatalf("Failed to remove from whitelist: %v", err)
		}

		receipt := suite.waitForTransaction(result.Hash, 60*time.Second)
		if !receipt.Success {
			t.Fatal("Remove from whitelist transaction failed")
		}

		metadata, err := suite.Client.Tokens().Metadata(ctx, tokenAddr)
		if err != nil {
			t.Fatalf("Failed to get token metadata: %v", err)
		}
		for _, addr := range metadata.WhiteList {
			if common.HexToAddress(addr) == suite.Account1.Address {
				t.Error("Address still in whitelist after removal")
			}
		}

		t.Log("✅ Address removed from whitelist")
	})

	t.Log("\n🎉 Whitelist management test passed!")
}

// ============================================================================
// Test: Update Token Metadata
// ============================================================================

func TestBusinessFlow_UpdateMetadata(t *testing.T) {
	suite := setupBusinessFlowTest(t)
	ctx := context.Background()

	tokenAddr := suite.issueTokenForTest(t, ctx, "META", "Metadata Test Token", false, false)
	suite.grantAuthority(t, ctx, AuthorityTypeUpdateMetadata, suite.MasterAccount.Address, tokenAddr, big.NewInt(0))

	t.Run("1. Update Token Metadata", func(t *testing.T) {
		suite.refreshCheckpoint()

		newName := fmt.Sprintf("Updated Token %d", rand.Intn(10000))
		newURI := fmt.Sprintf("https://example.com/token/%d", time.Now().Unix())

		payload := UpdateMetadataPayload{
			ChainID: suite.ChainID,
			Nonce:   suite.getNonce(suite.MasterAccount.Address),
			Name:    newName,
			URI:     newURI,
			Token:   tokenAddr,
			AdditionalMetadata: []AdditionalMetadata{
				{Key: "website", Value: "https://example.com"},
				{Key: "description", Value: "Test token for integration testing"},
			},
		}

		t.Logf("📝 Updating metadata: %s", newName)
		result, err := suite.Client.Tokens().UpdateMetadata(ctx, payload, suite.MasterAccount.Signer)
		if err != nil {
			t.Fatalf("Failed to update metadata: %v", err)
		}

		receipt := suite.waitForTransaction(result.Hash, 60*time.Second)
		if !receipt.Success {
			t.Fatal("Update metadata transaction failed")
		}

		metadata, err := suite.Client.Tokens().Metadata(ctx, tokenAddr)
		if err != nil {
			t.Fatalf("Failed to get token metadata: %v", err)
		}
		if metadata.Meta.Name != newName {
			t.Errorf("Expected name %s, got %s", newName, metadata.Meta.Name)
		}
		if metadata.Meta.URI != newURI {
			t.Errorf("Expected URI %s, got %s", newURI, metadata.Meta.URI)
		}

		t.Logf("✅ Metadata updated successfully")
		t.Logf("   - Name: %s", metadata.Meta.Name)
		t.Logf("   - URI: %s", metadata.Meta.URI)
		t.Logf("   - Additional metadata: %d items", len(metadata.Meta.AdditionalMetadata))
	})

	t.Log("\n🎉 Update metadata test passed!")
}

// ============================================================================
// Test: Token Bridge and Mint / Burn and Bridge
// ============================================================================

func TestBusinessFlow_BridgeMintAndBurnBridge(t *testing.T) {
	suite := setupBusinessFlowTest(t)
	ctx := context.Background()

	tokenAddr := suite.issueTokenForTest(t, ctx, "BRG", "Bridge Test Token", false, false)
	bridgeAccount := suite.generateOrGetAccount("")
	suite.grantAuthority(t, ctx, AuthorityTypeBridge, bridgeAccount.Address, tokenAddr, big.NewInt(0))

	t.Run("1. Bridge And Mint Tokens", func(t *testing.T) {
		suite.refreshCheckpoint()

		mintAmount := big.NewInt(100000000) // 100 tokens
		payload := TokenBridgeAndMintPayload{
			ChainID:        suite.ChainID,
			Nonce:          suite.getNonce(bridgeAccount.Address),
			Recipient:      suite.Account1.Address,
			Value:          mintAmount,
			Token:          tokenAddr,
			SourceChainID:  1,
			SourceTxHash:   "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			BridgeMetadata: "bridge-and-mint",
		}

		t.Logf("🌉 Bridging and minting %s tokens to %s", mintAmount.String(), suite.Account1.Address.Hex())
		result, err := suite.Client.Tokens().BridgeAndMint(ctx, payload, bridgeAccount.Signer)
		if !assert.NoError(t, err, "bridge and mint token") {
			return
		}

		receipt := suite.waitForTransaction(result.Hash, 60*time.Second)
		assert.True(t, receipt.Success, "bridge and mint transaction should succeed")
		suite.assertReceiptBasics(t, receipt, result.Hash, bridgeAccount.Address)
		if receipt.TokenAddress != nil {
			assert.Equal(t, tokenAddr, *receipt.TokenAddress, "bridge mint receipt token mismatch")
		}
		if receipt.Recipient != nil {
			assert.Equal(t, suite.Account1.Address, *receipt.Recipient, "bridge mint receipt recipient mismatch")
		}

		tx := suite.fetchTransaction(t, result.Hash)
		assert.Equal(t, TransactionTypeTokenBridgeAndMint, tx.TransactionType, "unexpected transaction type")
		assert.Equal(t, bridgeAccount.Address, tx.From, "unexpected bridge mint sender")
		if data, ok := tx.AsTokenBridgeAndMintData(); ok {
			assert.Equal(t, suite.Account1.Address, data.Recipient, "bridge mint recipient mismatch")
			assert.Equal(t, tokenAddr, data.Token, "bridge mint token mismatch")
			assert.Equal(t, uint64(1), data.SourceChainID, "bridge mint source chain mismatch")
			assert.Equal(t, payload.SourceTxHash, data.SourceTxHash, "bridge mint source tx hash mismatch")
			assert.Equal(t, payload.BridgeMetadata, data.BridgeMetadata, "bridge mint metadata mismatch")
			valueInt, ok := new(big.Int).SetString(data.Value, 10)
			assert.True(t, ok, "failed to parse bridge mint value %s", data.Value)
			assert.Zero(t, valueInt.Cmp(mintAmount), "bridge mint amount mismatch")
		}

		balance := suite.getTokenBalance(suite.Account1.Address, tokenAddr)
		assert.Equal(t, mintAmount.String(), balance, "bridge mint balance mismatch")
	})

	t.Run("2. Burn And Bridge Tokens", func(t *testing.T) {
		suite.refreshCheckpoint()

		burnAmount := big.NewInt(40000000) // 40 tokens
		escrowFee := big.NewInt(1000)
		payload := TokenBurnAndBridgePayload{
			ChainID:            suite.ChainID,
			Nonce:              suite.getNonce(suite.Account1.Address),
			Sender:             suite.Account1.Address,
			Value:              burnAmount,
			Token:              tokenAddr,
			DestinationChainID: 137,
			DestinationAddress: "0x1111111111111111111111111111111111111111",
			EscrowFee:          escrowFee,
			BridgeMetadata:     "burn-and-bridge",
			BridgeParam:        HexBytes{0xde, 0xad, 0xbe, 0xef},
		}

		balanceBefore := suite.getTokenBalance(suite.Account1.Address, tokenAddr)
		t.Logf("🔥 Burning and bridging %s tokens from %s", burnAmount.String(), suite.Account1.Address.Hex())
		result, err := suite.Client.Tokens().BurnAndBridge(ctx, payload, suite.Account1.Signer)
		if !assert.NoError(t, err, "burn and bridge token") {
			return
		}

		receipt := suite.waitForTransaction(result.Hash, 60*time.Second)
		assert.True(t, receipt.Success, "burn and bridge transaction should succeed")
		suite.assertReceiptBasics(t, receipt, result.Hash, suite.Account1.Address)
		if receipt.TokenAddress != nil {
			assert.Equal(t, tokenAddr, *receipt.TokenAddress, "burn and bridge receipt token mismatch")
		}

		tx := suite.fetchTransaction(t, result.Hash)
		assert.Equal(t, TransactionTypeTokenBurnAndBridge, tx.TransactionType, "unexpected transaction type")
		assert.Equal(t, suite.Account1.Address, tx.From, "unexpected burn and bridge sender")
		if data, ok := tx.AsTokenBurnAndBridgeData(); ok {
			assert.Equal(t, suite.Account1.Address, data.Sender, "burn and bridge sender mismatch")
			assert.Equal(t, tokenAddr, data.Token, "burn and bridge token mismatch")
			assert.Equal(t, uint64(137), data.DestinationChainID, "burn and bridge destination chain mismatch")
			assert.Equal(t, payload.DestinationAddress, data.DestinationAddress, "burn and bridge destination address mismatch")
			assert.Equal(t, payload.BridgeMetadata, data.BridgeMetadata, "burn and bridge metadata mismatch")
			assert.Equal(t, "0xdeadbeef", data.BridgeParam, "burn and bridge param mismatch")
			valueInt, ok := new(big.Int).SetString(data.Value, 10)
			assert.True(t, ok, "failed to parse burn and bridge value %s", data.Value)
			assert.Zero(t, valueInt.Cmp(burnAmount), "burn and bridge amount mismatch")
			feeInt, ok := new(big.Int).SetString(data.EscrowFee, 10)
			assert.True(t, ok, "failed to parse burn and bridge escrow fee %s", data.EscrowFee)
			assert.Zero(t, feeInt.Cmp(escrowFee), "burn and bridge escrow fee mismatch")
		}

		balanceAfter := suite.getTokenBalance(suite.Account1.Address, tokenAddr)
		expectedBalance := new(big.Int)
		if _, ok := expectedBalance.SetString(balanceBefore, 10); ok {
			totalSpent := burnAmount.Add(burnAmount, escrowFee)
			expectedBalance.Sub(expectedBalance, totalSpent)
			assert.Equal(t, expectedBalance.String(), balanceAfter, "burn and bridge balance mismatch")
		}
	})

	t.Log("\n🎉 Bridge and burn bridge tests passed!")
}

// ============================================================================
// Test: Checkpoint Endpoints (reads)
// ============================================================================

func TestBusinessFlow_CheckpointEndpoints(t *testing.T) {
	suite := setupBusinessFlowTest(t)
	ctx := context.Background()

	numberResp, err := suite.Client.GetCheckpointNumber(ctx)
	if !assert.NoError(t, err) {
		return
	}
	assert.Greater(t, numberResp.Number, uint64(0), "expected positive checkpoint number")

	lightCheckpoint, err := suite.Client.GetCheckpointByNumber(ctx, numberResp.Number)
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, uint64(numberResp.Number), lightCheckpoint.Number, "light checkpoint number mismatch")
	assert.Nil(t, lightCheckpoint.Transactions.Full, "expected light checkpoint without full transactions")

	byHash, err := suite.Client.GetCheckpointByHash(ctx, lightCheckpoint.Hash)
	if assert.NoError(t, err) {
		assert.Equal(t, lightCheckpoint.Hash, byHash.Hash, "checkpoint hash mismatch")
	}

	fullByNumber, err := suite.Client.GetCheckpointByNumber(ctx, numberResp.Number, WithFullTransactions())
	if !assert.NoError(t, err) {
		return
	}
	assert.Nil(t, fullByNumber.Transactions.Hashes, "expected hashes slice to be nil when requesting full transactions")
	assert.NotNil(t, fullByNumber.Transactions.Full, "expected full transactions slice")

	fullByHash, err := suite.Client.GetCheckpointByHash(ctx, lightCheckpoint.Hash, WithFullTransactions())
	if assert.NoError(t, err) {
		assert.Equal(t, lightCheckpoint.Hash, fullByHash.Hash, "full checkpoint hash mismatch")
		assert.NotNil(t, fullByHash.Transactions.Full, "expected full transactions by hash")
	}

	receipts, err := suite.Client.GetCheckpointReceiptsByNumber(ctx, numberResp.Number)
	if !assert.NoError(t, err) {
		return
	}
	assert.NotNil(t, receipts, "expected receipts to be returned")

	if fullByNumber.Transactions.Full != nil {
		assert.Equal(t, len(fullByNumber.Transactions.Full), len(receipts), "receipts count should match transactions count")
		for i, tx := range fullByNumber.Transactions.Full {
			receipt := &receipts[i]
			assert.Equal(t, tx.Hash, receipt.TransactionHash, "transaction hash mismatch at index %d", i)
			assert.Equal(t, tx.From, receipt.From, "from address mismatch at index %d for tx %s", i, tx.Hash)
			assert.Equal(t, tx.CheckpointNumber, receipt.CheckpointNumber, "checkpoint number mismatch at index %d for tx %s", i, tx.Hash)
			assert.Equal(t, tx.CheckpointHash, receipt.CheckpointHash, "checkpoint hash mismatch at index %d for tx %s", i, tx.Hash)
			assert.Equal(t, tx.TransactionIndex, receipt.TransactionIndex, "transaction index mismatch at index %d for tx %s", i, tx.Hash)
			assert.NotEmpty(t, receipt.FeeUsed, "fee used should be populated at index %d for tx %s", i, tx.Hash)
		}
		t.Logf("✅ GetCheckpointReceiptsByNumber verified: %d receipts match %d transactions in order", len(receipts), len(fullByNumber.Transactions.Full))
	}
}

// mintTo issues no token; it grants mint authority to a fresh minter and mints
// `amount` of `token` to `recipient`, returning the minter account. Shared by
// the account/fee/batch tests that need pre-funded balances.
func (s *BusinessFlowTestSuite) mintTo(t *testing.T, ctx context.Context, token, recipient common.Address, amount *big.Int) *TestAccount {
	t.Helper()
	minter := s.generateOrGetAccount("")
	s.grantAuthority(t, ctx, AuthorityTypeMintBurnTokens, minter.Address, token, new(big.Int).Mul(amount, big.NewInt(10)))

	s.refreshCheckpoint()
	payload := TokenMintPayload{
		ChainID:   s.ChainID,
		Nonce:     s.getNonce(minter.Address),
		Recipient: recipient,
		Value:     amount,
		Token:     token,
	}
	result, err := s.Client.Tokens().Mint(ctx, payload, minter.Signer)
	if err != nil {
		t.Fatalf("Failed to mint token: %v", err)
	}
	receipt := s.waitForTransaction(result.Hash, 60*time.Second)
	if !receipt.Success {
		t.Fatal("Mint token transaction failed")
	}
	t.Logf("💰 Minted %s of %s to %s", amount.String(), token.Hex(), recipient.Hex())
	return minter
}

// ============================================================================
// Test: Account Endpoints
// ============================================================================

func TestBusinessFlow_AccountEndpoints(t *testing.T) {
	suite := setupBusinessFlowTest(t)
	ctx := context.Background()
	assert := assert.New(t)

	tokenAddr := suite.issueTokenForTest(t, ctx, "ACCT", "Account Test Token", false, false)
	mintAmount := big.NewInt(500000)
	minter := suite.mintTo(t, ctx, tokenAddr, suite.Account1.Address, mintAmount)

	// Validate account nonce increments.
	newNonceResp, err := suite.Client.GetAccountNonce(ctx, minter.Address)
	if assert.NoError(err) {
		assert.GreaterOrEqual(newNonceResp.Nonce, uint64(1), "expected minter nonce to increase")
	}

	// Validate token account data.
	tokenAccountResp, err := suite.Client.GetTokenAccount(ctx, suite.Account1.Address, tokenAddr)
	if assert.NoError(err) {
		assert.Equal(mintAmount.String(), tokenAccountResp.Balance, "unexpected token balance")
	}
}

// ============================================================================
// Test: Fee Estimation
// ============================================================================

func TestBusinessFlow_EstimateFee(t *testing.T) {
	suite := setupBusinessFlowTest(t)
	ctx := context.Background()
	assert := assert.New(t)

	assertValidFee := func(fee string, context string) {
		assert.NotEmpty(fee, "fee should not be empty for %s", context)
		feeBigInt := new(big.Int)
		_, ok := feeBigInt.SetString(fee, 10)
		assert.True(ok, "fee should be a valid number for %s", context)
		assert.GreaterOrEqual(feeBigInt.Cmp(big.NewInt(0)), 0, "fee should be positive or zero for %s", context)
	}

	t.Run("1. Estimate Zero Address Token Fee", func(t *testing.T) {
		zeroAddress := common.HexToAddress("0x0000000000000000000000000000000000000000")
		feeResp, err := suite.Client.GetEstimateFee(ctx, suite.Account1.Address, suite.Account2.Address, zeroAddress, "1000000")
		if !assert.NoError(err, "should estimate native token fee") {
			return
		}
		assertValidFee(feeResp.Fee, "native token")
		t.Logf("✅ Native token fee estimated: %s", feeResp.Fee)
	})

	t.Run("2. Estimate Custom Token Fee", func(t *testing.T) {
		tokenAddr := suite.issueTokenForTest(t, ctx, "FEE", "Fee Test Token", false, false)
		suite.mintTo(t, ctx, tokenAddr, suite.Account1.Address, big.NewInt(100000000))

		feeResp, err := suite.Client.GetEstimateFee(ctx, suite.Account1.Address, suite.Account2.Address, tokenAddr, "50000000")
		if !assert.NoError(err, "should estimate custom token fee") {
			return
		}
		assertValidFee(feeResp.Fee, "custom token")
		t.Logf("✅ Custom token fee estimated: %s", feeResp.Fee)
	})

	t.Run("3. Estimate Fees for Different Amounts", func(t *testing.T) {
		nativeToken := common.HexToAddress("0x0000000000000000000000000000000000000000")
		for _, amount := range []string{"1", "1000", "1000000", "1000000000"} {
			feeResp, err := suite.Client.GetEstimateFee(ctx, suite.Account1.Address, suite.Account2.Address, nativeToken, amount)
			if !assert.NoError(err, "should estimate fee for amount %s", amount) {
				continue
			}
			assertValidFee(feeResp.Fee, "amount "+amount)
			t.Logf("   - Amount: %s → Fee: %s", amount, feeResp.Fee)
		}
	})

	t.Log("\n🎉 Fee estimation test passed!")
}

// ============================================================================
// Test: Batch Payment
// ============================================================================

func TestBusinessFlow_BatchPayment(t *testing.T) {
	suite := setupBusinessFlowTest(t)
	ctx := context.Background()

	tokenAddr := suite.issueTokenForTest(t, ctx, "BATCH", "Batch Payment Token", false, false)
	suite.mintTo(t, ctx, tokenAddr, suite.Account1.Address, big.NewInt(100000000)) // 100 tokens to sender

	recipient3 := suite.generateOrGetAccount("")
	amount2 := big.NewInt(10000000) // 10 tokens
	amount3 := big.NewInt(20000000) // 20 tokens

	suite.refreshCheckpoint()
	payload := BatchPaymentPayload{
		ChainID: suite.ChainID,
		Nonce:   suite.getNonce(suite.Account1.Address),
		Token:   tokenAddr,
		Operations: []PaymentOperation{
			{Recipient: suite.Account2.Address, Amount: amount2},
			{Recipient: recipient3.Address, Amount: amount3},
		},
		CreatedAt: uint64(time.Now().Unix()),
	}

	t.Logf("Batch paying %d recipients from %s", len(payload.Operations), suite.Account1.Address.Hex())
	result, err := suite.Client.Transactions().BatchPayment(ctx, payload, suite.Account1.Signer)
	if err != nil {
		t.Fatalf("Failed to submit batch payment: %v", err)
	}

	prepared, err := PrepareTransaction(payload)
	if err != nil {
		t.Fatalf("PrepareTransaction for hash cross-check: %v", err)
	}
	signature, err := suite.Account1.Signer.SignHash(prepared.SigningHash())
	if err != nil {
		t.Fatalf("sign for hash cross-check: %v", err)
	}
	authorized, err := prepared.Authorize(signature)
	if err != nil {
		t.Fatalf("authorize for hash cross-check: %v", err)
	}
	if !strings.EqualFold(hexLower(authorized.TransactionHash()), result.Hash) {
		t.Fatalf("local tx hash %s != server hash %s", hexLower(authorized.TransactionHash()), result.Hash)
	}

	receipt := suite.waitForTransaction(result.Hash, 60*time.Second)
	if !receipt.Success {
		t.Fatal("Batch payment transaction failed")
	}
	suite.assertReceiptBasics(t, receipt, result.Hash, suite.Account1.Address)

	tx := suite.fetchTransaction(t, result.Hash)
	assert.Equal(t, TransactionTypeBatchPayment, tx.TransactionType, "unexpected transaction type")
	if data, ok := tx.AsBatchPaymentData(); ok {
		assert.Len(t, data.Operations, 2, "expected 2 batch operations")
	}

	assert.Equal(t, amount2.String(), suite.getTokenBalance(suite.Account2.Address, tokenAddr), "recipient2 balance mismatch")
	assert.Equal(t, amount3.String(), suite.getTokenBalance(recipient3.Address, tokenAddr), "recipient3 balance mismatch")

	batchData, ok := tx.AsBatchPaymentData()
	if !ok {
		t.Fatal("transaction did not decode as BatchPaymentData")
	}
	if len(batchData.Operations) != len(payload.Operations) {
		t.Errorf("decoded %d operations, want %d", len(batchData.Operations), len(payload.Operations))
	}
	if batchData.CreatedAt != payload.CreatedAt {
		t.Errorf("decoded created_at %d, want %d", batchData.CreatedAt, payload.CreatedAt)
	}

	suite.refreshCheckpoint()
	memoPayload := payload
	memoPayload.Nonce = suite.getNonce(suite.Account1.Address)
	memoPayload.CreatedAt = uint64(time.Now().Unix())
	memo := Memo{Type: "purpose/PAYROLL", Format: "text/plain", Data: "batch-flow-memo"}

	memoResult, err := suite.Client.Transactions().BatchPayment(ctx, memoPayload, suite.Account1.Signer, WithMemo(memo))
	if err != nil {
		t.Fatalf("Failed to submit batch payment with memo: %v", err)
	}
	memoReceipt := suite.waitForTransaction(memoResult.Hash, 60*time.Second)
	if !memoReceipt.Success {
		t.Fatal("Batch payment with memo failed")
	}
	memoTx := suite.fetchTransaction(t, memoResult.Hash)
	if memoTx.Memo == nil {
		t.Fatal("batch payment response carried no memo")
	}
	if *memoTx.Memo != memo {
		t.Errorf("memo = %+v, want %+v", *memoTx.Memo, memo)
	}

	t.Log("\n🎉 Batch payment test passed!")
}

// ============================================================================
// Test: Clawback (requires a clawback-enabled token)
// ============================================================================

func TestBusinessFlow_Clawback(t *testing.T) {
	suite := setupBusinessFlowTest(t)
	ctx := context.Background()

	tokenAddr := suite.issueTokenForTest(t, ctx, "CLAW", "Clawback Test Token", false, true)
	suite.grantAuthority(t, ctx, AuthorityTypeClawback, suite.MasterAccount.Address, tokenAddr, big.NewInt(0))
	suite.grantAuthority(t, ctx, AuthorityTypeManageList, suite.MasterAccount.Address, tokenAddr, big.NewInt(0))
	suite.mintTo(t, ctx, tokenAddr, suite.Account1.Address, big.NewInt(100000000))

	// Clawback only reclaims from a FROZEN source. For a public token, an account
	// is frozen by being on the blacklist (l1client TokenState::is_frozen_via_list).
	suite.refreshCheckpoint()
	freezePayload := TokenManageListPayload{
		ChainID: suite.ChainID,
		Nonce:   suite.getNonce(suite.MasterAccount.Address),
		Action:  ManageListActionAdd,
		Address: suite.Account1.Address,
		Token:   tokenAddr,
	}
	freezeResult, err := suite.Client.Tokens().ManageBlacklist(ctx, freezePayload, suite.MasterAccount.Signer)
	if err != nil {
		t.Fatalf("Failed to freeze (blacklist) source account: %v", err)
	}
	if r := suite.waitForTransaction(freezeResult.Hash, 60*time.Second); !r.Success {
		t.Fatal("Freeze (blacklist) transaction failed")
	}
	t.Logf("🧊 Froze source account %s via blacklist", suite.Account1.Address.Hex())

	clawAmount := big.NewInt(30000000)
	balBefore := suite.getTokenBalance(suite.Account1.Address, tokenAddr)

	suite.refreshCheckpoint()
	payload := TokenClawbackPayload{
		ChainID:   suite.ChainID,
		Nonce:     suite.getNonce(suite.MasterAccount.Address),
		Token:     tokenAddr,
		From:      suite.Account1.Address,
		Recipient: suite.Account2.Address,
		Value:     clawAmount,
	}

	t.Logf("♻️  Clawing back %s from %s to %s", clawAmount.String(), suite.Account1.Address.Hex(), suite.Account2.Address.Hex())
	result, err := suite.Client.Tokens().Clawback(ctx, payload, suite.MasterAccount.Signer)
	if err != nil {
		t.Fatalf("Failed to clawback: %v", err)
	}

	receipt := suite.waitForTransaction(result.Hash, 60*time.Second)
	if !receipt.Success {
		t.Fatal("Clawback transaction failed")
	}
	suite.assertReceiptBasics(t, receipt, result.Hash, suite.MasterAccount.Address)

	tx := suite.fetchTransaction(t, result.Hash)
	assert.Equal(t, TransactionTypeTokenClawback, tx.TransactionType, "unexpected transaction type")
	if data, ok := tx.AsTokenClawbackData(); ok {
		assert.Equal(t, suite.Account1.Address, data.From, "clawback from mismatch")
		assert.Equal(t, suite.Account2.Address, data.Recipient, "clawback recipient mismatch")
		assert.Equal(t, tokenAddr, data.Token, "clawback token mismatch")
	}

	fromBefore := new(big.Int)
	fromBefore.SetString(balBefore, 10)
	assert.Equal(t, new(big.Int).Sub(fromBefore, clawAmount).String(), suite.getTokenBalance(suite.Account1.Address, tokenAddr), "clawback from balance mismatch")
	assert.Equal(t, clawAmount.String(), suite.getTokenBalance(suite.Account2.Address, tokenAddr), "clawback recipient balance mismatch")

	t.Log("\n🎉 Clawback test passed!")
}

// ============================================================================
// Test: Blacklist Management (public token)
// ============================================================================

func TestBusinessFlow_BlacklistManagement(t *testing.T) {
	suite := setupBusinessFlowTest(t)
	ctx := context.Background()

	tokenAddr := suite.issueTokenForTest(t, ctx, "BLACK", "Blacklist Test Token", false, false)
	suite.grantAuthority(t, ctx, AuthorityTypeManageList, suite.MasterAccount.Address, tokenAddr, big.NewInt(0))

	t.Run("1. Add Address to Blacklist", func(t *testing.T) {
		suite.refreshCheckpoint()
		payload := TokenManageListPayload{
			ChainID: suite.ChainID,
			Nonce:   suite.getNonce(suite.MasterAccount.Address),
			Action:  ManageListActionAdd,
			Address: suite.Account1.Address,
			Token:   tokenAddr,
		}
		t.Logf("🚫 Adding %s to blacklist", suite.Account1.Address.Hex())
		result, err := suite.Client.Tokens().ManageBlacklist(ctx, payload, suite.MasterAccount.Signer)
		if err != nil {
			t.Fatalf("Failed to add to blacklist: %v", err)
		}
		receipt := suite.waitForTransaction(result.Hash, 60*time.Second)
		if !receipt.Success {
			t.Fatal("Add to blacklist transaction failed")
		}
		metadata, err := suite.Client.Tokens().Metadata(ctx, tokenAddr)
		if err != nil {
			t.Fatalf("Failed to get token metadata: %v", err)
		}
		found := false
		for _, addr := range metadata.BlackList {
			if common.HexToAddress(addr) == suite.Account1.Address {
				found = true
				break
			}
		}
		if !found {
			t.Error("Address not found in blacklist")
		}
		t.Log("✅ Address added to blacklist")
	})

	t.Run("2. Remove Address from Blacklist", func(t *testing.T) {
		suite.refreshCheckpoint()
		payload := TokenManageListPayload{
			ChainID: suite.ChainID,
			Nonce:   suite.getNonce(suite.MasterAccount.Address),
			Action:  ManageListActionRemove,
			Address: suite.Account1.Address,
			Token:   tokenAddr,
		}
		t.Logf("♻️  Removing %s from blacklist", suite.Account1.Address.Hex())
		result, err := suite.Client.Tokens().ManageBlacklist(ctx, payload, suite.MasterAccount.Signer)
		if err != nil {
			t.Fatalf("Failed to remove from blacklist: %v", err)
		}
		receipt := suite.waitForTransaction(result.Hash, 60*time.Second)
		if !receipt.Success {
			t.Fatal("Remove from blacklist transaction failed")
		}
		metadata, err := suite.Client.Tokens().Metadata(ctx, tokenAddr)
		if err != nil {
			t.Fatalf("Failed to get token metadata: %v", err)
		}
		for _, addr := range metadata.BlackList {
			if common.HexToAddress(addr) == suite.Account1.Address {
				t.Error("Address still in blacklist after removal")
			}
		}
		t.Log("✅ Address removed from blacklist")
	})

	t.Log("\n🎉 Blacklist management test passed!")
}

// ============================================================================
// Test: Create Multisig Account
// ============================================================================

func TestBusinessFlow_CreateMultisig(t *testing.T) {
	suite := setupBusinessFlowTest(t)
	ctx := context.Background()

	suite.refreshCheckpoint()
	signers := []MultiSigSigner{
		{PublicKey: HexBytes(suite.Account1.Signer.CompressedPublicKey()), Weight: 1},
		{PublicKey: HexBytes(suite.Account2.Signer.CompressedPublicKey()), Weight: 1},
	}
	threshold := uint16(2)

	// The SDK derives the account address deterministically; it must match the
	// address the node assigns.
	wantAddr, err := DeriveMultisigAddress(signers, threshold)
	if err != nil {
		t.Fatalf("Failed to derive multisig address: %v", err)
	}

	payload := CreateMultiSigPayload{
		ChainID:   suite.ChainID,
		Nonce:     suite.getNonce(suite.OperatorAccount.Address),
		Signers:   signers,
		Threshold: threshold,
	}

	t.Logf("👥 Creating multisig account (threshold %d of %d signers)", threshold, len(signers))
	result, err := suite.Client.Accounts().CreateMultisig(ctx, payload, suite.OperatorAccount.Signer)
	if err != nil {
		t.Fatalf("Failed to create multisig account: %v", err)
	}
	assert.Equal(t, wantAddr, result.Account, "multisig account address should match local derivation")

	receipt := suite.waitForTransaction(result.Hash, 60*time.Second)
	if !receipt.Success {
		t.Fatal("Create multisig transaction failed")
	}
	suite.assertReceiptBasics(t, receipt, result.Hash, suite.OperatorAccount.Address)

	tx := suite.fetchTransaction(t, result.Hash)
	assert.Equal(t, TransactionTypeCreateMultiSig, tx.TransactionType, "unexpected transaction type")
	if data, ok := tx.AsCreateMultiSigData(); ok {
		assert.Equal(t, threshold, data.Threshold, "multisig threshold mismatch")
		assert.Len(t, data.Signers, len(signers), "multisig signer count mismatch")
		assert.Equal(t, wantAddr, data.MultisigAddress, "multisig derived address mismatch")
	}

	t.Logf("✅ Multisig account created: %s", result.Account.Hex())
	t.Log("\n🎉 Create multisig test passed!")
}
