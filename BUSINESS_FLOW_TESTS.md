# Business Flow Integration Tests

This document describes the business flow integration tests that simulate real-world usage scenarios of the 1Money blockchain.

## Overview

Business flow tests (`business_integration_test.go`, build tag `integration`) are comprehensive end-to-end tests that simulate complete business workflows involving multiple transactions and state changes. Unlike the simple integration tests that verify individual API calls, these tests verify entire user journeys and business processes.

All submissions use the **domain-separated v2 API**: tests build a payload and a `Signer` and call the `Transactions()` / `Tokens()` / `Accounts()` namespaces (e.g. `client.Tokens().Mint(ctx, payload, signer)`). Signing, RLP encoding, endpoint selection, and server-hash verification happen inside the SDK — the tests never hand-sign or build legacy `*Request` wrappers.

## Test Scenarios

### 1. Complete Token Lifecycle

**Test**: `TestBusinessFlow_CompleteTokenLifecycle`

Simulates the complete lifecycle of a token from creation to destruction:

1. **Issue New Token** — `Tokens().Issue`, signed by the operator; master account set as master authority
2. **Grant Mint Authority** — `Tokens().GrantAuthority` grants `MintBurnTokens` to a fresh minter
3. **Mint Tokens** — `Tokens().Mint` by the authorized minter (not the master)
4. **Transfer Tokens** — `Transactions().Payment` between accounts
5. **Burn Tokens** — `Tokens().Burn` by the minter
6. **Revoke Mint Authority** — `Tokens().RevokeAuthority` removes the grant

**Verifications**: token metadata is set; balances update after each step; grants/revocations take effect; delegated accounts can act; all transactions confirm.

### 2. Token Pause and Unpause

**Test**: `TestBusinessFlow_TokenPauseUnpause`

1. Issue a token; grant `Pause` authority (`GrantAuthority`)
2. **Pause** — `Tokens().Pause`; verify `IsPaused == true`
3. **Unpause** — `Tokens().Unpause`; verify `IsPaused == false`

### 3. Whitelist Management (private token)

**Test**: `TestBusinessFlow_WhitelistManagement`

1. Issue a **private** token (whitelist gated); grant `ManageList` authority
2. **Add** an address — `Tokens().ManageWhitelist` (Action `Add`); verify it appears in `WhiteList`
3. **Remove** the address — `Tokens().ManageWhitelist` (Action `Remove`); verify it is gone

### 4. Blacklist Management (public token)

**Test**: `TestBusinessFlow_BlacklistManagement`

1. Issue a **public** token; grant `ManageList` authority
2. **Add** an address — `Tokens().ManageBlacklist` (Action `Add`); verify it appears in `BlackList`
3. **Remove** the address — `Tokens().ManageBlacklist` (Action `Remove`); verify it is gone

### 5. Token Metadata Updates

**Test**: `TestBusinessFlow_UpdateMetadata`

1. Issue a token; grant `UpdateMetadata` authority
2. **Update** — `Tokens().UpdateMetadata` changes name, URI, and additional metadata; verify via `Tokens().Metadata`

### 6. Bridge Mint and Burn Bridge

**Test**: `TestBusinessFlow_BridgeMintAndBurnBridge`

1. Issue a token; grant `Bridge` authority to a bridge account
2. **Bridge and Mint** — `Tokens().BridgeAndMint` mints bridged-in supply; verify balance and decoded `source_chain_id` / `source_tx_hash`
3. **Burn and Bridge** — `Tokens().BurnAndBridge` burns and bridges out; verify balance decreased by value + escrow fee and the decoded `bridge_param`

### 7. Batch Payment

**Test**: `TestBusinessFlow_BatchPayment`

1. Issue a token and mint supply to the sender
2. **Batch pay** — `Transactions().BatchPayment` pays multiple recipients in one transaction; verify each recipient's balance and the decoded operations

### 8. Clawback

**Test**: `TestBusinessFlow_Clawback`

1. Issue a **clawback-enabled** token; grant `Clawback` authority; mint to an account
2. **Clawback** — `Tokens().Clawback` reclaims tokens from one account to another; verify both balances

### 9. Create Multisig Account

**Test**: `TestBusinessFlow_CreateMultisig`

1. Build a signer set from two accounts' compressed public keys with a threshold
2. **Create** — `Accounts().CreateMultisig`; verify the returned account address equals the local `DeriveMultisigAddress` derivation and the decoded transaction data

### 10. Read Endpoints

- **`TestBusinessFlow_AccountEndpoints`** — nonce and token-account reads after issue/mint
- **`TestBusinessFlow_CheckpointEndpoints`** — light/full checkpoints by number and hash, and receipt-vs-transaction cross-checks
- **`TestBusinessFlow_EstimateFee`** — fee estimation for native and custom tokens across amounts

## Prerequisites

### Required Environment Variables

- `TEST_OPERATOR_PRIVATE_KEY`: Network operator private key for issuing tokens

  - This account has the privilege to issue new tokens on the network
  - Must have native tokens to pay for transaction fees
  - Required for all business flow tests

- `TEST_MASTER_PRIVATE_KEY`: Master authority private key for token management
  - This account will be set as the master authority for issued tokens
  - Used for granting/revoking authorities, minting, burning, pausing, etc.
  - Must have native tokens to pay for transaction fees
  - Should be a fresh account for testing or one you control completely

### Optional Environment Variables

- `TEST_ENV`: Environment to test against (`testnet` or `local`, default: `testnet`)
- `TEST_ACCOUNT1_PRIVATE_KEY`: Private key for first test account (generated if not provided)
- `TEST_ACCOUNT2_PRIVATE_KEY`: Private key for second test account (generated if not provided)
- `LOCAL_API_URL`: URL for local development (default: `http://localhost:18555`)

## Running Business Flow Tests

### Run All Business Flow Tests

```bash
TEST_OPERATOR_PRIVATE_KEY=operator_key \
TEST_MASTER_PRIVATE_KEY=master_key \
go test -v -tags=integration -run "TestBusinessFlow" -timeout 10m
```

### Run Specific Business Flow Tests

```bash
# Test complete token lifecycle
TEST_OPERATOR_PRIVATE_KEY=operator_key \
TEST_MASTER_PRIVATE_KEY=master_key \
go test -v -tags=integration -run "TestBusinessFlow_CompleteTokenLifecycle" -timeout 10m

# Test pause/unpause
TEST_OPERATOR_PRIVATE_KEY=operator_key \
TEST_MASTER_PRIVATE_KEY=master_key \
go test -v -tags=integration -run "TestBusinessFlow_TokenPauseUnpause" -timeout 5m

# Test blacklist management
TEST_OPERATOR_PRIVATE_KEY=operator_key \
TEST_MASTER_PRIVATE_KEY=master_key \
go test -v -tags=integration -run "TestBusinessFlow_BlacklistManagement" -timeout 5m

# Test metadata updates
TEST_OPERATOR_PRIVATE_KEY=operator_key \
TEST_MASTER_PRIVATE_KEY=master_key \
go test -v -tags=integration -run "TestBusinessFlow_UpdateMetadata" -timeout 5m
```

Other individual tests follow the same pattern with `-run`:
`TestBusinessFlow_WhitelistManagement`, `TestBusinessFlow_BridgeMintAndBurnBridge`,
`TestBusinessFlow_BatchPayment`, `TestBusinessFlow_Clawback`,
`TestBusinessFlow_CreateMultisig`, `TestBusinessFlow_AccountEndpoints`,
`TestBusinessFlow_CheckpointEndpoints`, `TestBusinessFlow_EstimateFee`.

### Run on Local Development Environment

```bash
TEST_ENV=local \
LOCAL_API_URL=http://localhost:8080 \
TEST_OPERATOR_PRIVATE_KEY=operator_key \
TEST_MASTER_PRIVATE_KEY=master_key \
go test -v -tags=integration -run "TestBusinessFlow" -timeout 10m
```

### Use Specific Test Accounts

```bash
TEST_OPERATOR_PRIVATE_KEY=operator_key \
TEST_MASTER_PRIVATE_KEY=master_key \
TEST_ACCOUNT1_PRIVATE_KEY=account1_key \
TEST_ACCOUNT2_PRIVATE_KEY=account2_key \
go test -v -tags=integration -run "TestBusinessFlow" -timeout 10m
```

## Understanding the Test Output

Business flow tests provide detailed, emoji-enhanced output to make it easy to follow the test progress:

```
=== RUN   TestBusinessFlow_CompleteTokenLifecycle
Business Flow Test Suite Initialized:
  - Environment: testnet
  - Chain ID: 1212101
  - Recent Checkpoint: 321950
  - Operator Account: 0x... (for issuing tokens)
  - Master Account: 0x... (master authority)
  - Test Account 1: 0x...
  - Test Account 2: 0x...

=== RUN   TestBusinessFlow_CompleteTokenLifecycle/1._Issue_New_Token
    📝 Issuing token: TEST12345 (Test Token 12345)
       - Signed by operator: 0x...
       - Master authority: 0x...
    ✅ Token issued successfully
       - Transaction Hash: 0x...
       - Token Address: 0x...
    ⏳ Waiting for transaction 0x... to be confirmed...
    ✅ Transaction confirmed in 3.45s (checkpoint 321951)
    📋 Transaction details:
       - From: 0x...
       - Type: token_issue
       - Nonce: 5
       - Chain ID: 1212101
       - Checkpoint: 321951
    ✅ Token metadata verified

=== RUN   TestBusinessFlow_CompleteTokenLifecycle/1._Issue_New_Token/2._Grant_Mint_Authority
    🔐 Granting mint authority to 0x...
    ⏳ Waiting for transaction 0x... to be confirmed...
    ✅ Transaction confirmed in 2.87s (checkpoint 321952)
    📋 Transaction details:
       - From: 0x...
       - Type: token_authority
       - Nonce: 3
       - Chain ID: 1212101
       - Checkpoint: 321952
    ✅ Mint authority granted

=== RUN   TestBusinessFlow_CompleteTokenLifecycle/1._Issue_New_Token/3._Mint_Tokens
    💰 Minting 100000000 tokens to 0x...
    ⏳ Waiting for transaction 0x... to be confirmed...
    ✅ Transaction confirmed in 2.93s (checkpoint 321953)
    📋 Transaction details:
       - From: 0x...
       - Type: token_mint
       - Nonce: 1
       - Chain ID: 1212101
       - Checkpoint: 321953
    ✅ Tokens minted and balance verified: 100000000

... (more steps)

🎉 Complete token lifecycle test passed!
```

### Output Symbols Explained

- 📝 Issue/Create operation
- 🔐 Grant authority operation
- 💰 Mint operation
- 💸 Transfer operation
- 🔥 Burn operation
- 🔒 Revoke authority operation
- ⏸️ Pause operation
- ▶️ Unpause operation
- 🚫 Blacklist operation
- ⏳ Waiting for confirmation
- 📋 Transaction details
- ✅ Success
- ❌ Failure
- ⚠️ Warning

## Test Architecture

### BusinessFlowTestSuite

The `BusinessFlowTestSuite` struct provides:

- **Client**: Configured 1Money client
- **OperatorAccount**: Network operator account with token issuance privileges
- **MasterAccount**: Master authority for token management (minting, burning, pausing, etc.)
- **Account1, Account2**: Test accounts for transfers and operations
- **ChainID**: Current chain ID
- **RecentCheckpoint**: Latest checkpoint number

**Role Separation**:

- **Operator Account**: Signs token issuance transactions (requires network operator privileges)
- **Master Account**: Set as the master authority of issued tokens, manages token operations

### Helper Functions

#### Transaction Management

- `waitForTransaction(txHash, maxWait)`: Polls for transaction receipt and retrieves transaction details
  - Automatically logs progress with emoji indicators
  - Calls `GetTransactionReceipt` to wait for confirmation
  - Calls `GetTransactionByHash` to retrieve and log detailed transaction information
  - Logs transaction details: from address, type, nonce, chain ID, checkpoint
  - Returns receipt once confirmed
  - Fails test if timeout exceeded

#### Account Operations

- `getNonce(address)`: Gets current nonce for signing
- `getTokenBalance(address, token)`: Gets token balance
- `refreshCheckpoint()`: Updates to latest checkpoint

#### Signing

- Each `TestAccount` carries a `Signer` (built via `NewPrivateKeySigner`). Submit
  methods take the account's `Signer` and the SDK signs internally with
  domain-separated v2 — there is no manual `signMessage` step.

#### Flow Helpers

- `issueTokenForTest(t, ctx, prefix, name, private, clawback)`: issues a token
  signed by the operator and returns its address
- `grantAuthority(t, ctx, authorityType, to, token, value)`: grants a token
  authority from the master account and waits for confirmation
- `mintTo(t, ctx, token, recipient, amount)`: grants mint authority to a fresh
  minter and mints `amount` of `token` to `recipient`

#### Account Generation

- `newTestAccount(t, privateKeyHex)`: builds a `TestAccount` (address + `Signer`)
  from a hex private key
- `generateOrGetAccount(envVar)`: Creates or retrieves a test account
  - Uses environment variable if set
  - Generates a new keypair otherwise
  - Populates the account's `Signer` and logs the address for debugging

## Best Practices

### 1. Use Dedicated Test Accounts

Never use production accounts for testing. You need two types of accounts:

**Operator Account**:

- Requires network operator privileges (granted by network administrators)
- Only needed for issuing new tokens
- Should have sufficient balance for token issuance fees

**Master Authority Account**:

- Will be the master authority for all issued tokens
- Needs balance for token management operations (mint, burn, pause, etc.)
- Can be a regular account without special privileges

```bash
# Generate a new test account (for master authority)
go run -c 'package main; import "github.com/ethereum/go-ethereum/crypto"; import "fmt"; func main() { key, _ := crypto.GenerateKey(); fmt.Printf("Private Key: %x\nAddress: %s\n", crypto.FromECDSA(key), crypto.PubkeyToAddress(key.PublicKey).Hex()) }'
```

### 2. Fund Test Accounts

Ensure both accounts have sufficient native tokens:

**Operator Account**:

- Transaction fees for issuing tokens
- Usually requires more balance as issuance is more expensive

**Master Authority Account**:

- Transaction fees for token management operations
- Multiple transactions per test
- Account creation (if needed)

### 3. Run Tests on Testnet First

Always run tests on testnet before considering other environments:

```bash
# Safe testnet run
TEST_ENV=testnet TEST_MASTER_PRIVATE_KEY=your_testnet_key go test -v -run "TestBusinessFlow"
```

### 4. Monitor Test Progress

Business flow tests can take several minutes. Monitor:

- Transaction confirmation times
- Balance changes
- Authority grants/revokes
- State transitions

### 5. Clean Up After Tests

Consider cleaning up test data:

- Note all created token addresses
- Document test accounts used
- Track any authorities granted

## Troubleshooting

### Test Skipped: "TEST_OPERATOR_PRIVATE_KEY not set" or "TEST_MASTER_PRIVATE_KEY not set"

**Solution**: Set both required environment variables:

```bash
TEST_OPERATOR_PRIVATE_KEY=operator_key \
TEST_MASTER_PRIVATE_KEY=master_key \
go test -v -run "TestBusinessFlow"
```

### Test Fails: "Insufficient balance"

**Cause**: Operator or Master account doesn't have enough native tokens for fees

**Solution**:

1. Check both operator and master account balances
2. Fund both accounts with native tokens (operator needs more for issuing tokens)
3. Retry the test

### Test Fails: "Permission denied" or "Unauthorized" when issuing tokens

**Cause**: The operator account doesn't have token issuance privileges

**Solution**:

1. Verify the `TEST_OPERATOR_PRIVATE_KEY` is for an authorized network operator
2. Contact network administrators to grant operator privileges
3. Use a different operator account with proper permissions

### Test Fails: "Transaction not confirmed after X"

**Cause**: Network congestion or checkpoint delays

**Solution**:

1. Check network status
2. Increase timeout: modify `maxWait` parameter in test
3. Verify the API endpoint is responsive

### Test Fails: "Authority already exists"

**Cause**: Running test multiple times with same account

**Solution**:

1. Use fresh accounts for each test run
2. Or implement cleanup in test teardown
3. Check token metadata to see existing authorities

### Transactions Fail: "Nonce too low"

**Cause**: Nonce synchronization issue

**Solution**:

1. Test automatically refreshes nonces
2. If persists, wait a few seconds between tests
3. Verify no other processes are using the same account

## Adding New Business Flow Tests

When adding new business flow tests:

1. **Follow the Pattern**:

   - Use `setupBusinessFlowTest(t)` to initialize
   - Structure tests with clear steps using `t.Run()`
   - Use helper functions for common operations

2. **Add Proper Logging**:

   - Use emoji indicators for operation types
   - Log important values (addresses, amounts, hashes)
   - Clearly indicate success/failure

3. **Include Verification**:

   - Check balances after transfers
   - Verify metadata after updates
   - Confirm authority changes

4. **Handle Timing**:

   - Refresh checkpoint before each transaction
   - Wait for confirmation before proceeding
   - Use appropriate timeouts

5. **Update Documentation**:
   - Add test to this document
   - Describe the business flow
   - List verification points

## Example: Creating a Custom Business Flow Test

```go
func TestBusinessFlow_CustomScenario(t *testing.T) {
    suite := setupBusinessFlowTest(t)
    ctx := context.Background()

    // Issue a token and mint supply with the shared helpers.
    tokenAddr := suite.issueTokenForTest(t, ctx, "CUST", "Custom Token", false, false)
    suite.mintTo(t, ctx, tokenAddr, suite.Account1.Address, big.NewInt(100000000))

    t.Run("Transfer", func(t *testing.T) {
        suite.refreshCheckpoint()

        payload := PaymentPayload{
            ChainID:   suite.ChainID,
            Nonce:     suite.getNonce(suite.Account1.Address),
            Recipient: suite.Account2.Address,
            Value:     big.NewInt(25000000),
            Token:     tokenAddr,
        }

        // The SDK signs with the account's Signer (domain-separated v2) and
        // verifies the server-returned transaction hash internally.
        result, err := suite.Client.Transactions().Payment(ctx, payload, suite.Account1.Signer)
        if err != nil {
            t.Fatalf("payment: %v", err)
        }

        receipt := suite.waitForTransaction(result.Hash, 60*time.Second)
        if !receipt.Success {
            t.Fatal("Transaction failed")
        }

        t.Log("✅ Transfer complete")
    })

    t.Log("\n🎉 Custom scenario test passed!")
}
```

## Security Considerations

⚠️ **Important Security Notes**:

1. **Never commit private keys** to version control
2. **Use environment variables** for sensitive data
3. **Separate test and production** accounts
4. **Protect operator keys carefully** - they have network-level privileges
5. **Limit test account funds** to necessary amounts
6. **Rotate test keys** regularly
7. **Monitor test accounts** for unexpected transactions
8. **Operator key security**:
   - Store operator keys in secure vaults
   - Use different operators for different environments (testnet vs mainnet)
   - Limit operator key distribution to authorized personnel only

## Performance Expectations

Typical execution times (on testnet):

- **Complete Token Lifecycle**: 30-60 seconds (6 transactions)
- **Token Pause/Unpause**: 15-30 seconds (3 transactions)
- **Whitelist / Blacklist Management**: 15-30 seconds (3 transactions each)
- **Metadata Updates**: 10-20 seconds (2 transactions)
- **Bridge Mint / Burn Bridge**: 20-40 seconds (4 transactions)
- **Batch Payment**: 20-40 seconds (issue + grant + mint + batch)
- **Clawback**: 20-40 seconds (issue + grant + mint + clawback)
- **Create Multisig**: 10-20 seconds (1 transaction)

Times may vary based on:

- Network congestion
- Checkpoint generation rate
- API response times
- Geographic location

## Contributing

When contributing business flow tests:

1. Ensure tests are idempotent (can run multiple times)
2. Use descriptive test names and step labels
3. Add comprehensive logging
4. Include verification at each step
5. Handle errors gracefully
6. Update this documentation
