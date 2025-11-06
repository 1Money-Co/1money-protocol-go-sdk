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
			url = "http://localhost:18555"
		}
		client = NewClientWithCustomUrl(url, WithTimeout(30*time.Second))
	case "testnet":
		client = NewTestClientWithOpts(WithTimeout(30 * time.Second))
	default:
		t.Fatalf("Unsupported TEST_ENV for business flow tests: %s", env)
	}

	// Create operator account (for issuing tokens)
	operatorAddr, err := PrivateKeyToAddress(operatorPrivateKey)
	if err != nil {
		t.Fatalf("Failed to get operator address: %v", err)
	}

	// Create master account (for token management)
	masterAddr, err := PrivateKeyToAddress(masterPrivateKey)
	if err != nil {
		t.Fatalf("Failed to get master address: %v", err)
	}

	suite := &BusinessFlowTestSuite{
		Client: client,
		OperatorAccount: &TestAccount{
			PrivateKey: operatorPrivateKey,
			Address:    common.HexToAddress(operatorAddr),
		},
		MasterAccount: &TestAccount{
			PrivateKey: masterPrivateKey,
			Address:    common.HexToAddress(masterAddr),
		},
		t: t,
	}

	// Get chain ID
	ctx := context.Background()
	chainIDResp, err := client.GetChainId(ctx)
	if err != nil {
		t.Fatalf("Failed to get chain ID: %v", err)
	}
	suite.ChainID = chainIDResp.ChainId

	// Get recent checkpoint
	checkpointResp, err := client.GetCheckpointNumber(ctx)
	if err != nil {
		t.Fatalf("Failed to get checkpoint: %v", err)
	}
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
		addr, err := PrivateKeyToAddress(privateKeyHex)
		if err != nil {
			s.t.Fatalf("Failed to get address from %s: %v", envVar, err)
		}
		return &TestAccount{
			PrivateKey: privateKeyHex,
			Address:    common.HexToAddress(addr),
		}
	}

	// Generate new account
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		s.t.Fatalf("Failed to generate private key: %v", err)
	}

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
	if err != nil {
		s.t.Fatalf("Failed to refresh checkpoint: %v", err)
	}
	s.RecentCheckpoint = uint64(checkpointResp.Number)
}

// getNonce gets the current nonce for an account
func (s *BusinessFlowTestSuite) getNonce(address common.Address) uint64 {
	ctx := context.Background()
	nonceResp, err := s.Client.GetAccountNonce(ctx, address.Hex())
	if err != nil {
		s.t.Fatalf("Failed to get nonce for %s: %v", address.Hex(), err)
	}
	return nonceResp.Nonce
}

// getTokenBalance gets the token balance for an account
func (s *BusinessFlowTestSuite) getTokenBalance(address common.Address, token common.Address) string {
	ctx := context.Background()
	accountResp, err := s.Client.GetTokenAccount(ctx, address.Hex(), token.Hex())
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
			RecentCheckpoint: suite.RecentCheckpoint,
			ChainID:          suite.ChainID,
			Nonce:            suite.getNonce(suite.OperatorAccount.Address), // Use operator nonce
			Symbol:           symbol,
			Name:             name,
			Decimals:         6,
			MasterAuthority:  suite.MasterAccount.Address, // Master authority for token management
			IsPrivate:        false,
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

		t.Logf("✅ Token issued successfully")
		t.Logf("   - Transaction Hash: %s", result.Hash)
		t.Logf("   - Token Address: %s", result.Token)

		// Wait for confirmation
		receipt := suite.waitForTransaction(result.Hash, 10*time.Second)
		if !receipt.Success {
			t.Fatal("Token issuance transaction failed")
		}

		// Verify token metadata
		tokenAddr := common.HexToAddress(result.Token)
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
				RecentCheckpoint: suite.RecentCheckpoint,
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
				RecentCheckpoint: suite.RecentCheckpoint,
				ChainID:          suite.ChainID,
				Nonce:            suite.getNonce(minterAccount.Address), // Use minter account nonce
				Recipient:        suite.Account1.Address,
				Value:            mintAmount,
				Token:            tokenAddr,
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

			receipt := suite.waitForTransaction(result.Hash, 60*time.Second)
			if !receipt.Success {
				t.Fatal("Mint token transaction failed")
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
				RecentCheckpoint: suite.RecentCheckpoint,
				ChainID:          suite.ChainID,
				Nonce:            suite.getNonce(suite.Account1.Address),
				Recipient:        suite.Account2.Address,
				Value:            transferAmount,
				Token:            tokenAddr,
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

			receipt := suite.waitForTransaction(result.Hash, 60*time.Second)
			if !receipt.Success {
				t.Fatal("Payment transaction failed")
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

			account2Balance := suite.getTokenBalance(suite.Account2.Address, tokenAddr)
			t.Logf("Burn Tokens - account2Balance: %s of account %s", account2Balance, suite.Account2.Address.Hex())

			// Transfer token to account that has burn authority
			tPayload := PaymentPayload{
				RecentCheckpoint: suite.RecentCheckpoint,
				ChainID:          suite.ChainID,
				Nonce:            suite.getNonce(suite.Account2.Address),
				Recipient:        minterAccount.Address,
				Value:            big.NewInt(25000000), // 25 tokens
				Token:            tokenAddr,
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

			tReceipt := suite.waitForTransaction(tResult.Hash, 60*time.Second)
			if !tReceipt.Success {
				t.Fatal("Payment transaction failed")
			}
			// Verify balances
			minterBalance := suite.getTokenBalance(minterAccount.Address, tokenAddr)
			if minterBalance != big.NewInt(25000000).String() {
				t.Errorf("MinterAccount: expected balance 25000000, got %s", minterBalance)
			}

			burnAmount := big.NewInt(25000000) // 25 tokens

			payload := TokenBurnPayload{
				RecentCheckpoint: suite.RecentCheckpoint,
				ChainID:          suite.ChainID,
				Nonce:            suite.getNonce(minterAccount.Address), // Use minter account nonce
				Recipient:        minterAccount.Address,                 // Only can burn own tokens
				Value:            burnAmount,
				Token:            tokenAddr,
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

			receipt := suite.waitForTransaction(result.Hash, 60*time.Second)
			if !receipt.Success {
				t.Fatal("Burn token transaction failed")
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
				RecentCheckpoint: suite.RecentCheckpoint,
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
		RecentCheckpoint: suite.RecentCheckpoint,
		ChainID:          suite.ChainID,
		Nonce:            suite.getNonce(suite.OperatorAccount.Address), // Use operator nonce
		Symbol:           symbol,
		Name:             "Pause Test Token",
		Decimals:         6,
		MasterAuthority:  suite.MasterAccount.Address, // Master authority for token management
		IsPrivate:        false,
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

	suite.waitForTransaction(issueResult.Hash, 60*time.Second)
	tokenAddr := common.HexToAddress(issueResult.Token)
	t.Logf("✅ Token issued: %s", tokenAddr.Hex())

	t.Run("1. Grant Pause Authority", func(t *testing.T) {
		suite.refreshCheckpoint()

		payload := TokenAuthorityPayload{
			RecentCheckpoint: suite.RecentCheckpoint,
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

		receipt := suite.waitForTransaction(result.Hash, 60*time.Second)
		if !receipt.Success {
			t.Fatal("Grant pause authority failed")
		}
	})

	t.Run("2. Pause Token", func(t *testing.T) {
		suite.refreshCheckpoint()

		payload := PauseTokenPayload{
			RecentCheckpoint: suite.RecentCheckpoint,
			ChainID:          suite.ChainID,
			Nonce:            suite.getNonce(suite.MasterAccount.Address),
			Action:           Pause,
			Token:            tokenAddr,
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
			RecentCheckpoint: suite.RecentCheckpoint,
			ChainID:          suite.ChainID,
			Nonce:            suite.getNonce(suite.MasterAccount.Address),
			Action:           UnPause,
			Token:            tokenAddr,
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
		RecentCheckpoint: suite.RecentCheckpoint,
		ChainID:          suite.ChainID,
		Nonce:            suite.getNonce(suite.OperatorAccount.Address), // Use operator nonce
		Symbol:           symbol,
		Name:             "Private Test Token",
		Decimals:         6,
		MasterAuthority:  suite.MasterAccount.Address, // Master authority for token management
		IsPrivate:        true,                        // Private token uses whitelist
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
			RecentCheckpoint: suite.RecentCheckpoint,
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

		receipt := suite.waitForTransaction(result.Hash, 60*time.Second)
		if !receipt.Success {
			t.Fatal("Grant manage list authority failed")
		}
	})

	t.Run("2. Add Address to Whitelist", func(t *testing.T) {
		suite.refreshCheckpoint()

		payload := TokenManageListPayload{
			RecentCheckpoint: suite.RecentCheckpoint,
			ChainID:          suite.ChainID,
			Nonce:            suite.getNonce(suite.MasterAccount.Address),
			Action:           ManageListActionAdd,
			Address:          suite.Account1.Address,
			Token:            tokenAddr,
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
			RecentCheckpoint: suite.RecentCheckpoint,
			ChainID:          suite.ChainID,
			Nonce:            suite.getNonce(suite.MasterAccount.Address),
			Action:           ManageListActionRemove,
			Address:          suite.Account1.Address,
			Token:            tokenAddr,
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
		RecentCheckpoint: suite.RecentCheckpoint,
		ChainID:          suite.ChainID,
		Nonce:            suite.getNonce(suite.OperatorAccount.Address), // Use operator nonce
		Symbol:           symbol,
		Name:             "Metadata Test Token",
		Decimals:         6,
		MasterAuthority:  suite.MasterAccount.Address, // Master authority for token management
		IsPrivate:        false,
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
			RecentCheckpoint: suite.RecentCheckpoint,
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
			RecentCheckpoint: suite.RecentCheckpoint,
			ChainID:          suite.ChainID,
			Nonce:            suite.getNonce(suite.MasterAccount.Address),
			Name:             newName,
			URI:              newURI,
			Token:            tokenAddr,
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
