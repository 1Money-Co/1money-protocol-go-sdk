//go:build integration

package onemoney

import (
	"context"
	"fmt"
	"math/big"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
)

// TestAccount represents a test account with private key
type TestAccount struct {
	PrivateKey string
	Address    common.Address
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

	var client *Client
	switch env {
	case "local":
		url := os.Getenv("LOCAL_API_URL")
		if url == "" {
			url = localEndpoint
		}
		client = NewClientWithCustomUrl(url, WithTimeout(30*time.Second))
	case "testnet":
		client = NewTestClientWithOpts(WithTimeout(30 * time.Second))
	default:
		t.Fatalf("Unsupported TEST_ENV for business flow tests: %s", env)
	}

	// Create operator account (for issuing tokens)
	operatorSigner, err := NewPrivateKeySigner(operatorPrivateKey)
	assert.Nil(t, err, "Should get operator address")

	// Create master account (for token management)
	masterSigner, err := NewPrivateKeySigner(masterPrivateKey)
	assert.Nil(t, err, "Should get master address")

	suite := &BusinessFlowTestSuite{
		Client: client,
		OperatorAccount: &TestAccount{
			PrivateKey: operatorPrivateKey,
			Address:    operatorSigner.Address(),
		},
		MasterAccount: &TestAccount{
			PrivateKey: masterPrivateKey,
			Address:    masterSigner.Address(),
		},
		t: t,
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

// generateOrGetAccount generates a new account or uses an existing one from env
func (s *BusinessFlowTestSuite) generateOrGetAccount(envVar string) *TestAccount {
	privateKeyHex := os.Getenv(envVar)
	if privateKeyHex != "" {
		signer, err := NewPrivateKeySigner(privateKeyHex)
		assert.Nil(s.t, err, fmt.Sprintf("Should get address from: %s", envVar))
		return &TestAccount{
			PrivateKey: privateKeyHex,
			Address:    signer.Address(),
		}
	}

	// Generate new account
	privateKey, err := crypto.GenerateKey()
	assert.Nil(s.t, err, "Should random generate private key")

	privateKeyHex = fmt.Sprintf("%x", crypto.FromECDSA(privateKey))
	address := crypto.PubkeyToAddress(privateKey.PublicKey)

	s.t.Logf("Generated new account for %s: %s", envVar, address.Hex())
	return &TestAccount{
		PrivateKey: privateKeyHex,
		Address:    address,
	}
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
func (s *BusinessFlowTestSuite) waitForTransaction(txHash string, maxWait time.Duration) *TransactionReceiptResponse {
	s.t.Logf("⏳ Waiting for transaction %s to be confirmed...", txHash)
	ctx := context.Background()
	start := time.Now()

	for {
		receipt, err := s.Client.GetTransactionReceipt(ctx, txHash)
		if err == nil {
			elapsed := time.Since(start)
			if receipt.Success {
				s.t.Logf("✅ Transaction confirmed in %.2fs (checkpoint %d)", elapsed.Seconds(), receipt.CheckpointNumber)
			} else {
				s.t.Logf("❌ Transaction failed in %.2fs (checkpoint %d)", elapsed.Seconds(), receipt.CheckpointNumber)
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
				s.t.Logf("   - Checkpoint: %d", tx.CheckpointNumber)
			}

			return receipt
		}

		if time.Since(start) > maxWait {
			s.t.Fatalf("Transaction %s not confirmed after %v", txHash, maxWait)
		}

		time.Sleep(1 * time.Second)
	}
}

// signMessage signs a message with the given private key
func (s *BusinessFlowTestSuite) signMessage(payload interface{}, privateKey string) Signature {
	signature, err := s.Client.SignMessage(payload, privateKey)
	if err != nil {
		s.t.Fatalf("Failed to sign message: %v", err)
	}
	return *signature
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

type hashableRequest interface {
	Hash() (common.Hash, error)
}

func (s *BusinessFlowTestSuite) assertRequestHashMatches(t *testing.T, req hashableRequest, resultHash string) {
	t.Helper()
	assert.NotEmptyf(t, resultHash, "expected result hash to be populated")
	localHash, err := req.Hash()
	assert.NoErrorf(t, err, "failed to compute request hash")
	assert.Equalf(t, common.HexToHash(resultHash), localHash, "request hash mismatch")
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

		signature := suite.signMessage(payload, suite.OperatorAccount.PrivateKey) // Sign with operator key

		request := &IssueTokenRequest{
			TokenIssuePayload: payload,
			Signature:         signature,
		}

		t.Logf("📝 Issuing token: %s (%s)", symbol, name)
		t.Logf("   - Signed by operator: %s", suite.OperatorAccount.Address.Hex())
		t.Logf("   - Master authority: %s", suite.MasterAccount.Address.Hex())
		result, err := suite.Client.IssueToken(ctx, request)
		if err != nil {
			t.Fatalf("Failed to issue token: %v", err)
		}
		suite.assertRequestHashMatches(t, request, result.Hash)

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
		metadata, err := suite.Client.GetTokenMetadata(ctx, tokenAddr.Hex())
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
				Action:           AuthorityActionGrant,
				AuthorityType:    AuthorityTypeMintBurnTokens,
				AuthorityAddress: minterAccount.Address, // Grant to the new minter account
				Token:            tokenAddr,
				Value:            big.NewInt(1000000000000), // 1M tokens with 6 decimals
			}

			signature := suite.signMessage(payload, suite.MasterAccount.PrivateKey)

			request := &TokenAuthorityRequest{
				TokenAuthorityPayload: payload,
				Signature:             signature,
			}

			t.Logf("🔐 Granting mint authority from master to minter account")
			t.Logf("   - Master authority: %s", suite.MasterAccount.Address.Hex())
			t.Logf("   - Minter account: %s", minterAccount.Address.Hex())
			result, err := suite.Client.GrantTokenAuthority(ctx, request)
			if err != nil {
				t.Fatalf("Failed to grant authority: %v", err)
			}
			suite.assertRequestHashMatches(t, request, result.Hash)

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

			signature := suite.signMessage(payload, minterAccount.PrivateKey) // Sign with minter account

			request := &MintTokenRequest{
				TokenMintPayload: payload,
				Signature:        signature,
			}

			t.Logf("💰 Minting %s tokens to %s", mintAmount.String(), suite.Account1.Address.Hex())
			t.Logf("   - Minted by: %s", minterAccount.Address.Hex())
			result, err := suite.Client.MintToken(ctx, request)
			if err != nil {
				t.Fatalf("Failed to mint token: %v", err)
			}
			suite.assertRequestHashMatches(t, request, result.Hash)

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

			signature := suite.signMessage(payload, suite.Account1.PrivateKey)

			request := &PaymentRequest{
				PaymentPayload: payload,
				Signature:      signature,
			}

			t.Logf("💸 Transferring %s tokens from Account1 to Account2", transferAmount.String())
			result, err := suite.Client.SendPayment(ctx, request)
			if err != nil {
				t.Fatalf("Failed to send payment: %v", err)
			}
			suite.assertRequestHashMatches(t, request, result.Hash)

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
				assert.Equalf(t, tokenAddr, payload.Token, "transfer token mismatch")
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
			tSignature := suite.signMessage(tPayload, suite.Account2.PrivateKey)
			tRequest := &PaymentRequest{
				PaymentPayload: tPayload,
				Signature:      tSignature,
			}
			tResult, err := suite.Client.SendPayment(ctx, tRequest)
			if err != nil {
				t.Fatalf("Failed to send payment: %v", err)
			}
			suite.assertRequestHashMatches(t, tRequest, tResult.Hash)

			tReceipt := suite.waitForTransaction(tResult.Hash, 60*time.Second)
			if !tReceipt.Success {
				t.Fatal("Payment transaction failed")
			}
			suite.assertReceiptBasics(t, tReceipt, tResult.Hash, suite.Account2.Address)
			txTransfer := suite.fetchTransaction(t, tResult.Hash)
			if payload, ok := txTransfer.AsTokenTransferData(); ok {
				assert.Equalf(t, minterAccount.Address, payload.Recipient, "transfer-to-minter recipient mismatch")
				assert.Equalf(t, tokenAddr, payload.Token, "transfer-to-minter token mismatch")
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

			signature := suite.signMessage(payload, minterAccount.PrivateKey) // Sign with minter account

			request := &BurnTokenRequest{
				TokenBurnPayload: payload,
				Signature:        signature,
			}

			balanceBefore := suite.getTokenBalance(minterAccount.Address, tokenAddr)
			t.Logf("🔥 Burning %s tokens from Account2 (current balance: %s)", burnAmount.String(), balanceBefore)
			t.Logf("   - Burned by: %s", minterAccount.Address.Hex())

			result, err := suite.Client.BurnToken(ctx, request)
			if err != nil {
				t.Fatalf("Failed to burn token: %v", err)
			}
			suite.assertRequestHashMatches(t, request, result.Hash)

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
				Action:           AuthorityActionRevoke,
				AuthorityType:    AuthorityTypeMintBurnTokens,
				AuthorityAddress: minterAccount.Address, // Revoke from the minter account
				Token:            tokenAddr,
				Value:            big.NewInt(0),
			}

			signature := suite.signMessage(payload, suite.MasterAccount.PrivateKey)

			request := &TokenAuthorityRequest{
				TokenAuthorityPayload: payload,
				Signature:             signature,
			}

			t.Logf("🔒 Revoking mint authority from minter account")
			t.Logf("   - Revoked by master: %s", suite.MasterAccount.Address.Hex())
			t.Logf("   - Revoked from: %s", minterAccount.Address.Hex())
			result, err := suite.Client.GrantTokenAuthority(ctx, request)
			if err != nil {
				t.Fatalf("Failed to revoke authority: %v", err)
			}
			suite.assertRequestHashMatches(t, request, result.Hash)

			receipt := suite.waitForTransaction(result.Hash, 60*time.Second)
			if !receipt.Success {
				t.Fatal("Revoke authority transaction failed")
			}

			t.Logf("✅ Mint authority revoked from minter account")
		})
	})

	t.Logf("\n🎉 Complete token lifecycle test passed!")
}

// ============================================================================
// Test: Token Pause and Unpause
// ============================================================================

func TestBusinessFlow_TokenPauseUnpause(t *testing.T) {
	suite := setupBusinessFlowTest(t)
	ctx := context.Background()

	// First issue a token
	suite.refreshCheckpoint()
	symbol := fmt.Sprintf("PAUSE%d", time.Now().Unix()%100000)

	issuePayload := TokenIssuePayload{
		ChainID:         suite.ChainID,
		Nonce:           suite.getNonce(suite.OperatorAccount.Address), // Use operator nonce
		Symbol:          symbol,
		Name:            "Pause Test Token",
		Decimals:        6,
		MasterAuthority: suite.MasterAccount.Address, // Master authority for token management
		IsPrivate:       false,
		ClawbackEnabled: false,
	}

	issueSignature := suite.signMessage(issuePayload, suite.OperatorAccount.PrivateKey) // Sign with operator key
	issueRequest := &IssueTokenRequest{
		TokenIssuePayload: issuePayload,
		Signature:         issueSignature,
	}

	t.Log("📝 Issuing token for pause/unpause test")
	issueResult, err := suite.Client.IssueToken(ctx, issueRequest)
	if err != nil {
		t.Fatalf("Failed to issue token: %v", err)
	}
	suite.assertRequestHashMatches(t, issueRequest, issueResult.Hash)

	suite.waitForTransaction(issueResult.Hash, 60*time.Second)
	tokenAddr := common.HexToAddress(issueResult.Token)
	t.Logf("✅ Token issued: %s", tokenAddr.Hex())

	t.Run("1. Grant Pause Authority", func(t *testing.T) {
		suite.refreshCheckpoint()

		payload := TokenAuthorityPayload{
			ChainID:          suite.ChainID,
			Nonce:            suite.getNonce(suite.MasterAccount.Address),
			Action:           AuthorityActionGrant,
			AuthorityType:    AuthorityTypePause,
			AuthorityAddress: suite.MasterAccount.Address,
			Token:            tokenAddr,
			Value:            big.NewInt(0),
		}

		signature := suite.signMessage(payload, suite.MasterAccount.PrivateKey)
		request := &TokenAuthorityRequest{
			TokenAuthorityPayload: payload,
			Signature:             signature,
		}

		t.Log("🔐 Granting pause authority")
		result, err := suite.Client.GrantTokenAuthority(ctx, request)
		if err != nil {
			t.Fatalf("Failed to grant pause authority: %v", err)
		}
		suite.assertRequestHashMatches(t, request, result.Hash)

		receipt := suite.waitForTransaction(result.Hash, 60*time.Second)
		if !receipt.Success {
			t.Fatal("Grant pause authority failed")
		}
	})

	t.Run("2. Pause Token", func(t *testing.T) {
		suite.refreshCheckpoint()

		payload := PauseTokenPayload{
			ChainID: suite.ChainID,
			Nonce:   suite.getNonce(suite.MasterAccount.Address),
			Action:  Pause,
			Token:   tokenAddr,
		}

		signature := suite.signMessage(payload, suite.MasterAccount.PrivateKey)
		request := &PauseTokenRequest{
			PauseTokenPayload: payload,
			Signature:         signature,
		}

		t.Log("⏸️  Pausing token")
		result, err := suite.Client.PauseToken(ctx, request)
		if err != nil {
			t.Fatalf("Failed to pause token: %v", err)
		}
		suite.assertRequestHashMatches(t, request, result.Hash)

		receipt := suite.waitForTransaction(result.Hash, 60*time.Second)
		if !receipt.Success {
			t.Fatal("Pause token transaction failed")
		}

		// Verify token is paused
		metadata, err := suite.Client.GetTokenMetadata(ctx, tokenAddr.Hex())
		if err != nil {
			t.Fatalf("Failed to get token metadata: %v", err)
		}

		if !metadata.IsPaused {
			t.Error("Token should be paused but is not")
		}

		t.Log("✅ Token paused successfully")
	})

	t.Run("3. Unpause Token", func(t *testing.T) {
		suite.refreshCheckpoint()

		payload := PauseTokenPayload{
			ChainID: suite.ChainID,
			Nonce:   suite.getNonce(suite.MasterAccount.Address),
			Action:  UnPause,
			Token:   tokenAddr,
		}

		signature := suite.signMessage(payload, suite.MasterAccount.PrivateKey)
		request := &PauseTokenRequest{
			PauseTokenPayload: payload,
			Signature:         signature,
		}

		t.Log("▶️  Unpausing token")
		result, err := suite.Client.PauseToken(ctx, request)
		if err != nil {
			t.Fatalf("Failed to unpause token: %v", err)
		}
		suite.assertRequestHashMatches(t, request, result.Hash)

		receipt := suite.waitForTransaction(result.Hash, 60*time.Second)
		if !receipt.Success {
			t.Fatal("Unpause token transaction failed")
		}

		// Verify token is not paused
		metadata, err := suite.Client.GetTokenMetadata(ctx, tokenAddr.Hex())
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
// Test: Whitelist Management
// ============================================================================

func TestBusinessFlow_WhitelistManagement(t *testing.T) {
	suite := setupBusinessFlowTest(t)
	ctx := context.Background()

	// Issue a private token for whitelist testing
	suite.refreshCheckpoint()
	symbol := fmt.Sprintf("PRIV%d", time.Now().Unix()%100000)

	issuePayload := TokenIssuePayload{
		ChainID:         suite.ChainID,
		Nonce:           suite.getNonce(suite.OperatorAccount.Address), // Use operator nonce
		Symbol:          symbol,
		Name:            "Private Test Token",
		Decimals:        6,
		MasterAuthority: suite.MasterAccount.Address, // Master authority for token management
		IsPrivate:       true,                        // Private token uses whitelist
		ClawbackEnabled: false,
	}

	issueSignature := suite.signMessage(issuePayload, suite.OperatorAccount.PrivateKey) // Sign with operator key
	issueRequest := &IssueTokenRequest{
		TokenIssuePayload: issuePayload,
		Signature:         issueSignature,
	}

	t.Log("📝 Issuing private token for whitelist test")
	issueResult, err := suite.Client.IssueToken(ctx, issueRequest)
	if err != nil {
		t.Fatalf("Failed to issue token: %v", err)
	}

	suite.waitForTransaction(issueResult.Hash, 60*time.Second)
	tokenAddr := common.HexToAddress(issueResult.Token)
	t.Logf("✅ Private token issued: %s", tokenAddr.Hex())

	t.Run("1. Grant ManageList Authority", func(t *testing.T) {
		suite.refreshCheckpoint()

		payload := TokenAuthorityPayload{
			ChainID:          suite.ChainID,
			Nonce:            suite.getNonce(suite.MasterAccount.Address),
			Action:           AuthorityActionGrant,
			AuthorityType:    AuthorityTypeManageList,
			AuthorityAddress: suite.MasterAccount.Address,
			Token:            tokenAddr,
			Value:            big.NewInt(0),
		}

		signature := suite.signMessage(payload, suite.MasterAccount.PrivateKey)
		request := &TokenAuthorityRequest{
			TokenAuthorityPayload: payload,
			Signature:             signature,
		}

		t.Log("🔐 Granting manage list authority")
		result, err := suite.Client.GrantTokenAuthority(ctx, request)
		if err != nil {
			t.Fatalf("Failed to grant manage list authority: %v", err)
		}
		suite.assertRequestHashMatches(t, request, result.Hash)

		receipt := suite.waitForTransaction(result.Hash, 60*time.Second)
		if !receipt.Success {
			t.Fatal("Grant manage list authority failed")
		}
	})

	t.Run("2. Add Address to Whitelist", func(t *testing.T) {
		suite.refreshCheckpoint()

		payload := TokenManageListPayload{
			ChainID: suite.ChainID,
			Nonce:   suite.getNonce(suite.MasterAccount.Address),
			Action:  ManageListActionAdd,
			Address: suite.Account1.Address,
			Token:   tokenAddr,
		}

		signature := suite.signMessage(payload, suite.MasterAccount.PrivateKey)
		request := &SetTokenManageListRequest{
			TokenManageListPayload: payload,
			Signature:              signature,
		}

		t.Logf("🚫 Adding %s to whitelist", suite.Account1.Address.Hex())
		result, err := suite.Client.SetTokenWhitelist(ctx, request)
		if err != nil {
			t.Fatalf("Failed to add to whitelist: %v", err)
		}
		suite.assertRequestHashMatches(t, request, result.Hash)

		receipt := suite.waitForTransaction(result.Hash, 60*time.Second)
		if !receipt.Success {
			t.Fatal("Add to whitelist transaction failed")
		}

		// Verify address is in whitelist
		metadata, err := suite.Client.GetTokenMetadata(ctx, tokenAddr.Hex())
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

	t.Run("3. Remove Address from Whitelist", func(t *testing.T) {
		suite.refreshCheckpoint()

		payload := TokenManageListPayload{
			ChainID: suite.ChainID,
			Nonce:   suite.getNonce(suite.MasterAccount.Address),
			Action:  ManageListActionRemove,
			Address: suite.Account1.Address,
			Token:   tokenAddr,
		}

		signature := suite.signMessage(payload, suite.MasterAccount.PrivateKey)
		request := &SetTokenManageListRequest{
			TokenManageListPayload: payload,
			Signature:              signature,
		}

		t.Logf("✅ Removing %s from whitelist", suite.Account1.Address.Hex())
		result, err := suite.Client.SetTokenWhitelist(ctx, request)
		if err != nil {
			t.Fatalf("Failed to remove from whitelist: %v", err)
		}
		suite.assertRequestHashMatches(t, request, result.Hash)

		receipt := suite.waitForTransaction(result.Hash, 60*time.Second)
		if !receipt.Success {
			t.Fatal("Remove from whitelist transaction failed")
		}

		// Verify address is not in whitelist
		metadata, err := suite.Client.GetTokenMetadata(ctx, tokenAddr.Hex())
		if err != nil {
			t.Fatalf("Failed to get token metadata: %v", err)
		}

		for _, addr := range metadata.WhiteList {
			if addr == suite.Account1.Address.Hex() {
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

	// Issue a token
	suite.refreshCheckpoint()
	symbol := fmt.Sprintf("META%d", time.Now().Unix()%100000)

	issuePayload := TokenIssuePayload{
		ChainID:         suite.ChainID,
		Nonce:           suite.getNonce(suite.OperatorAccount.Address), // Use operator nonce
		Symbol:          symbol,
		Name:            "Metadata Test Token",
		Decimals:        6,
		MasterAuthority: suite.MasterAccount.Address, // Master authority for token management
		IsPrivate:       false,
		ClawbackEnabled: false,
	}

	issueSignature := suite.signMessage(issuePayload, suite.OperatorAccount.PrivateKey) // Sign with operator key
	issueRequest := &IssueTokenRequest{
		TokenIssuePayload: issuePayload,
		Signature:         issueSignature,
	}

	t.Log("📝 Issuing token for metadata update test")
	issueResult, err := suite.Client.IssueToken(ctx, issueRequest)
	if err != nil {
		t.Fatalf("Failed to issue token: %v", err)
	}

	suite.waitForTransaction(issueResult.Hash, 60*time.Second)
	tokenAddr := common.HexToAddress(issueResult.Token)

	t.Run("1. Grant UpdateMetadata Authority", func(t *testing.T) {
		suite.refreshCheckpoint()

		payload := TokenAuthorityPayload{
			ChainID:          suite.ChainID,
			Nonce:            suite.getNonce(suite.MasterAccount.Address),
			Action:           AuthorityActionGrant,
			AuthorityType:    AuthorityTypeUpdateMetadata,
			AuthorityAddress: suite.MasterAccount.Address,
			Token:            tokenAddr,
			Value:            big.NewInt(0),
		}

		signature := suite.signMessage(payload, suite.MasterAccount.PrivateKey)
		request := &TokenAuthorityRequest{
			TokenAuthorityPayload: payload,
			Signature:             signature,
		}

		t.Log("🔐 Granting update metadata authority")
		result, err := suite.Client.GrantTokenAuthority(ctx, request)
		if err != nil {
			t.Fatalf("Failed to grant update metadata authority: %v", err)
		}
		suite.assertRequestHashMatches(t, request, result.Hash)

		receipt := suite.waitForTransaction(result.Hash, 60*time.Second)
		if !receipt.Success {
			t.Fatal("Grant update metadata authority failed")
		}
	})

	t.Run("2. Update Token Metadata", func(t *testing.T) {
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

		signature := suite.signMessage(payload, suite.MasterAccount.PrivateKey)
		request := &UpdateMetadataRequest{
			UpdateMetadataPayload: payload,
			Signature:             signature,
		}

		t.Logf("📝 Updating metadata: %s", newName)
		result, err := suite.Client.UpdateTokenMetadata(ctx, request)
		if err != nil {
			t.Fatalf("Failed to update metadata: %v", err)
		}
		suite.assertRequestHashMatches(t, request, result.Hash)

		receipt := suite.waitForTransaction(result.Hash, 60*time.Second)
		if !receipt.Success {
			t.Fatal("Update metadata transaction failed")
		}

		// Verify metadata was updated
		metadata, err := suite.Client.GetTokenMetadata(ctx, tokenAddr.Hex())
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

	// Issue a token for bridge tests.
	suite.refreshCheckpoint()
	symbol := fmt.Sprintf("BRG%d", time.Now().Unix()%100000)

	issuePayload := TokenIssuePayload{
		ChainID:         suite.ChainID,
		Nonce:           suite.getNonce(suite.OperatorAccount.Address),
		Symbol:          symbol,
		Name:            "Bridge Test Token",
		Decimals:        6,
		MasterAuthority: suite.MasterAccount.Address,
		IsPrivate:       false,
		ClawbackEnabled: false,
	}

	issueSignature := suite.signMessage(issuePayload, suite.OperatorAccount.PrivateKey)
	issueReq := &IssueTokenRequest{
		TokenIssuePayload: issuePayload,
		Signature:         issueSignature,
	}

	t.Logf("📝 Issuing token for bridge tests: %s", symbol)
	issueResult, err := suite.Client.IssueToken(ctx, issueReq)
	if !assert.NoError(t, err, "issue token for bridge tests") {
		return
	}
	suite.assertRequestHashMatches(t, issueReq, issueResult.Hash)
	issueReceipt := suite.waitForTransaction(issueResult.Hash, 60*time.Second)
	assert.True(t, issueReceipt.Success, "token issue should succeed")
	suite.assertReceiptBasics(t, issueReceipt, issueResult.Hash, suite.OperatorAccount.Address)
	tokenAddr := common.HexToAddress(issueResult.Token)

	// Grant bridge authority to a bridge account.
	suite.refreshCheckpoint()
	bridgeAccount := suite.generateOrGetAccount("")
	grantPayload := TokenAuthorityPayload{
		ChainID:          suite.ChainID,
		Nonce:            suite.getNonce(suite.MasterAccount.Address),
		Action:           AuthorityActionGrant,
		AuthorityType:    AuthorityTypeBridge,
		AuthorityAddress: bridgeAccount.Address,
		Token:            tokenAddr,
		Value:            big.NewInt(0),
	}
	grantSignature := suite.signMessage(grantPayload, suite.MasterAccount.PrivateKey)
	grantReq := &TokenAuthorityRequest{
		TokenAuthorityPayload: grantPayload,
		Signature:             grantSignature,
	}
	t.Logf("🔐 Granting bridge authority to %s", bridgeAccount.Address.Hex())
	grantResult, err := suite.Client.GrantTokenAuthority(ctx, grantReq)
	if !assert.NoError(t, err, "grant bridge authority") {
		return
	}
	suite.assertRequestHashMatches(t, grantReq, grantResult.Hash)
	grantReceipt := suite.waitForTransaction(grantResult.Hash, 60*time.Second)
	assert.True(t, grantReceipt.Success, "grant bridge authority should succeed")

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

		signature := suite.signMessage(payload, bridgeAccount.PrivateKey)
		request := &BridgeAndMintTokenRequest{
			TokenBridgeAndMintPayload: payload,
			Signature:                 signature,
		}

		t.Logf("🌉 Bridging and minting %s tokens to %s", mintAmount.String(), suite.Account1.Address.Hex())
		result, err := suite.Client.BridgeAndMintToken(ctx, request)
		if !assert.NoError(t, err, "bridge and mint token") {
			return
		}
		suite.assertRequestHashMatches(t, request, result.Hash)

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

		signature := suite.signMessage(payload, suite.Account1.PrivateKey)
		request := &BurnAndBridgeTokenRequest{
			TokenBurnAndBridgePayload: payload,
			Signature:                 signature,
		}

		balanceBefore := suite.getTokenBalance(suite.Account1.Address, tokenAddr)
		t.Logf("🔥 Burning and bridging %s tokens from %s", burnAmount.String(), suite.Account1.Address.Hex())
		result, err := suite.Client.BurnAndBridgeToken(ctx, request)
		if !assert.NoError(t, err, "burn and bridge token") {
			return
		}
		suite.assertRequestHashMatches(t, request, result.Hash)

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

	// Test GetCheckpointReceiptsByNumber and compare with full transactions
	receipts, err := suite.Client.GetCheckpointReceiptsByNumber(ctx, numberResp.Number)
	if !assert.NoError(t, err) {
		return
	}
	assert.NotNil(t, receipts, "expected receipts to be returned")

	// Compare receipts with full transactions from checkpoint
	if fullByNumber.Transactions.Full != nil {
		assert.Equal(t, len(fullByNumber.Transactions.Full), len(receipts), "receipts count should match transactions count")

		// Compare each transaction with its corresponding receipt by index
		for i, tx := range fullByNumber.Transactions.Full {
			receipt := &receipts[i]

			// Verify receipt fields match transaction fields at the same index
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

func TestBusinessFlow_AccountEndpoints(t *testing.T) {
	suite := setupBusinessFlowTest(t)
	ctx := context.Background()
	assert := assert.New(t)

	// Issue a new token to exercise account endpoints.
	suite.refreshCheckpoint()
	symbol := fmt.Sprintf("ACCT%d", time.Now().Unix()%100000)
	issuePayload := TokenIssuePayload{
		ChainID:         suite.ChainID,
		Nonce:           suite.getNonce(suite.OperatorAccount.Address),
		Symbol:          symbol,
		Name:            "Account Test Token",
		Decimals:        6,
		MasterAuthority: suite.MasterAccount.Address,
		IsPrivate:       false,
		ClawbackEnabled: false,
	}
	issueSignature := suite.signMessage(issuePayload, suite.OperatorAccount.PrivateKey)
	issueReq := &IssueTokenRequest{
		TokenIssuePayload: issuePayload,
		Signature:         issueSignature,
	}
	issueResult, err := suite.Client.IssueToken(ctx, issueReq)
	if !assert.NoError(err, "issue token for account tests") {
		return
	}
	suite.assertRequestHashMatches(t, issueReq, issueResult.Hash)
	issueReceipt := suite.waitForTransaction(issueResult.Hash, 60*time.Second)
	assert.True(issueReceipt.Success, "token issue transaction should succeed")
	suite.assertReceiptBasics(t, issueReceipt, issueResult.Hash, suite.OperatorAccount.Address)
	tokenAddr := common.HexToAddress(issueResult.Token)

	// Grant mint authority.
	suite.refreshCheckpoint()
	minterAccount := suite.generateOrGetAccount("")
	grantPayload := TokenAuthorityPayload{
		ChainID:          suite.ChainID,
		Nonce:            suite.getNonce(suite.MasterAccount.Address),
		Action:           AuthorityActionGrant,
		AuthorityType:    AuthorityTypeMintBurnTokens,
		AuthorityAddress: minterAccount.Address,
		Token:            tokenAddr,
		Value:            big.NewInt(500000),
	}
	grantSignature := suite.signMessage(grantPayload, suite.MasterAccount.PrivateKey)
	grantReq := &TokenAuthorityRequest{
		TokenAuthorityPayload: grantPayload,
		Signature:             grantSignature,
	}
	grantResult, err := suite.Client.GrantTokenAuthority(ctx, grantReq)
	if !assert.NoError(err, "grant authority for account tests") {
		return
	}
	suite.assertRequestHashMatches(t, grantReq, grantResult.Hash)
	grantReceipt := suite.waitForTransaction(grantResult.Hash, 60*time.Second)
	assert.True(grantReceipt.Success, "grant authority transaction should succeed")

	// Mint tokens to Account1 to backfill token account data.
	suite.refreshCheckpoint()
	mintAmount := big.NewInt(500000)
	mintPayload := TokenMintPayload{
		ChainID:   suite.ChainID,
		Nonce:     suite.getNonce(minterAccount.Address),
		Recipient: suite.Account1.Address,
		Value:     mintAmount,
		Token:     tokenAddr,
	}
	mintSignature := suite.signMessage(mintPayload, minterAccount.PrivateKey)
	mintReq := &MintTokenRequest{
		TokenMintPayload: mintPayload,
		Signature:        mintSignature,
	}
	mintResult, err := suite.Client.MintToken(ctx, mintReq)
	if !assert.NoError(err, "mint token for account tests") {
		return
	}
	suite.assertRequestHashMatches(t, mintReq, mintResult.Hash)
	mintReceipt := suite.waitForTransaction(mintResult.Hash, 60*time.Second)
	assert.True(mintReceipt.Success, "mint transaction should succeed")

	// Validate account nonce increments.
	newNonceResp, err := suite.Client.GetAccountNonce(ctx, minterAccount.Address)
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

	t.Run("1. Estimate Zero Address Token Fee", func(t *testing.T) {
		// Zero Address
		zeroAddress := common.HexToAddress("0x0000000000000000000000000000000000000000")
		transferValue := "1000000" // 1 token with 6 decimals

		t.Logf("💵 Estimating fee for native token transfer")
		t.Logf("   - From: %s", suite.Account1.Address.Hex())
		t.Logf("   - To: %s", suite.Account2.Address.Hex())
		t.Logf("   - Token: %s (native)", zeroAddress.Hex())
		t.Logf("   - Value: %s", transferValue)

		feeResp, err := suite.Client.GetEstimateFee(ctx,
			suite.Account1.Address,
			suite.Account2.Address,
			zeroAddress,
			transferValue)

		if !assert.NoError(err, "should estimate native token fee") {
			return
		}

		// Verify fee response
		assert.NotEmpty(feeResp.Fee, "fee should not be empty")

		// Verify fee is a valid number
		feeBigInt := new(big.Int)
		_, ok := feeBigInt.SetString(feeResp.Fee, 10)
		assert.True(ok, "fee should be a valid number")

		// Fee should be positive
		assert.GreaterOrEqual(feeBigInt.Cmp(big.NewInt(0)), 0, "fee should be positive or zero")

		t.Logf("✅ Native token fee estimated: %s", feeResp.Fee)
	})

	t.Run("2. Estimate Custom Token Fee", func(t *testing.T) {
		// Issue a custom token first
		suite.refreshCheckpoint()
		symbol := fmt.Sprintf("FEE%d", time.Now().Unix()%100000)

		issuePayload := TokenIssuePayload{
			ChainID:         suite.ChainID,
			Nonce:           suite.getNonce(suite.OperatorAccount.Address),
			Symbol:          symbol,
			Name:            "Fee Test Token",
			Decimals:        6,
			MasterAuthority: suite.MasterAccount.Address,
			IsPrivate:       false,
			ClawbackEnabled: false,
		}

		issueSignature := suite.signMessage(issuePayload, suite.OperatorAccount.PrivateKey)
		issueReq := &IssueTokenRequest{
			TokenIssuePayload: issuePayload,
			Signature:         issueSignature,
		}

		t.Logf("📝 Issuing token for fee estimation test: %s", symbol)
		issueResult, err := suite.Client.IssueToken(ctx, issueReq)
		if !assert.NoError(err, "should issue token") {
			return
		}
		suite.assertRequestHashMatches(t, issueReq, issueResult.Hash)

		issueReceipt := suite.waitForTransaction(issueResult.Hash, 60*time.Second)
		assert.True(issueReceipt.Success, "token issue should succeed")
		tokenAddr := common.HexToAddress(issueResult.Token)
		t.Logf("✅ Token issued: %s", tokenAddr.Hex())

		// Grant mint authority to a minter account
		suite.refreshCheckpoint()
		minterAccount := suite.generateOrGetAccount("")

		grantPayload := TokenAuthorityPayload{
			ChainID:          suite.ChainID,
			Nonce:            suite.getNonce(suite.MasterAccount.Address),
			Action:           AuthorityActionGrant,
			AuthorityType:    AuthorityTypeMintBurnTokens,
			AuthorityAddress: minterAccount.Address,
			Token:            tokenAddr,
			Value:            big.NewInt(1000000000000),
		}

		grantSignature := suite.signMessage(grantPayload, suite.MasterAccount.PrivateKey)
		grantReq := &TokenAuthorityRequest{
			TokenAuthorityPayload: grantPayload,
			Signature:             grantSignature,
		}

		t.Logf("🔐 Granting mint authority to minter")
		grantResult, err := suite.Client.GrantTokenAuthority(ctx, grantReq)
		if !assert.NoError(err, "should grant mint authority") {
			return
		}
		suite.assertRequestHashMatches(t, grantReq, grantResult.Hash)

		grantReceipt := suite.waitForTransaction(grantResult.Hash, 60*time.Second)
		assert.True(grantReceipt.Success, "grant authority should succeed")

		// Mint tokens to Account1
		suite.refreshCheckpoint()
		mintAmount := big.NewInt(100000000) // 100 tokens

		mintPayload := TokenMintPayload{
			ChainID:   suite.ChainID,
			Nonce:     suite.getNonce(minterAccount.Address),
			Recipient: suite.Account1.Address,
			Value:     mintAmount,
			Token:     tokenAddr,
		}

		mintSignature := suite.signMessage(mintPayload, minterAccount.PrivateKey)
		mintReq := &MintTokenRequest{
			TokenMintPayload: mintPayload,
			Signature:        mintSignature,
		}

		t.Logf("💰 Minting tokens to Account1 for fee estimation")
		mintResult, err := suite.Client.MintToken(ctx, mintReq)
		if !assert.NoError(err, "should mint tokens") {
			return
		}
		suite.assertRequestHashMatches(t, mintReq, mintResult.Hash)

		mintReceipt := suite.waitForTransaction(mintResult.Hash, 60*time.Second)
		assert.True(mintReceipt.Success, "mint should succeed")

		// Now estimate fee for custom token transfer
		transferValue := "50000000" // 50 tokens

		t.Logf("💵 Estimating fee for custom token transfer")
		t.Logf("   - From: %s", suite.Account1.Address.Hex())
		t.Logf("   - To: %s", suite.Account2.Address.Hex())
		t.Logf("   - Token: %s", tokenAddr.Hex())
		t.Logf("   - Value: %s", transferValue)

		feeResp, err := suite.Client.GetEstimateFee(ctx,
			suite.Account1.Address,
			suite.Account2.Address,
			tokenAddr,
			transferValue)

		if !assert.NoError(err, "should estimate custom token fee") {
			return
		}

		// Verify fee response
		assert.NotEmpty(feeResp.Fee, "fee should not be empty")

		// Verify fee is a valid number
		feeBigInt := new(big.Int)
		_, ok := feeBigInt.SetString(feeResp.Fee, 10)
		assert.True(ok, "fee should be a valid number")

		// Fee should be positive
		assert.GreaterOrEqual(feeBigInt.Cmp(big.NewInt(0)), 0, "fee should be positive or zero")

		t.Logf("✅ Custom token fee estimated: %s", feeResp.Fee)
	})

	t.Run("3. Estimate Fees for Different Amounts", func(t *testing.T) {
		// Test that fee estimation works for various amounts
		nativeToken := common.HexToAddress("0x0000000000000000000000000000000000000000")
		amounts := []string{
			"1",          // Minimal amount
			"1000",       // Small amount
			"1000000",    // Medium amount
			"1000000000", // Large amount
		}

		for _, amount := range amounts {
			t.Logf("💵 Estimating fee for amount: %s", amount)

			feeResp, err := suite.Client.GetEstimateFee(ctx,
				suite.Account1.Address,
				suite.Account2.Address,
				nativeToken,
				amount)

			if !assert.NoError(err, "should estimate fee for amount %s", amount) {
				continue
			}

			assert.NotEmpty(feeResp.Fee, "fee should not be empty for amount %s", amount)

			// Parse and validate fee
			feeBigInt := new(big.Int)
			_, ok := feeBigInt.SetString(feeResp.Fee, 10)
			assert.True(ok, "fee should be valid number for amount %s", amount)
			assert.GreaterOrEqual(feeBigInt.Cmp(big.NewInt(0)), 0, "fee should be positive or zero for amount %s", amount)

			t.Logf("   - Amount: %s → Fee: %s", amount, feeResp.Fee)
		}

		t.Logf("✅ All amount variations estimated successfully")
	})

	t.Log("\n🎉 Fee estimation test passed!")
}
