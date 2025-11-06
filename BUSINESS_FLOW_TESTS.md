# Business Flow Integration Tests

This document describes the business flow integration tests that simulate real-world usage scenarios of the 1Money blockchain.

## Overview

Business flow tests (`business_flow_test.go`) are comprehensive end-to-end tests that simulate complete business workflows involving multiple transactions and state changes. Unlike the simple integration tests that verify individual API calls, these tests verify entire user journeys and business processes.

## Test Scenarios

### 1. Complete Token Lifecycle

**Test**: `TestBusinessFlow_CompleteTokenLifecycle`

This test simulates the complete lifecycle of a token from creation to destruction:

1. **Issue New Token**: Create a new token with custom symbol, name, and decimals
   - Signed by operator account
   - Master account set as master authority
2. **Grant Mint Authority**: Generate a new account and grant it minting/burning privileges
   - Master authority grants permission to the new minter account
   - Demonstrates role-based access control
3. **Mint Tokens**: Create new token supply and assign to an account
   - Performed by the authorized minter account (not master authority)
   - Shows that delegated authorities can execute operations
4. **Transfer Tokens**: Move tokens between accounts
   - Standard user-to-user transfer
5. **Burn Tokens**: Destroy tokens to reduce supply
   - Performed by the authorized minter account
   - Demonstrates burn authority usage
6. **Revoke Mint Authority**: Remove minting privileges from the minter account
   - Master authority revokes the previously granted permission
   - Shows authority management capabilities

**Verifications**:
- Token metadata is correctly set
- Balances are accurately updated after each operation
- Authority grants and revocations take effect
- All transactions are successfully confirmed
- Delegated accounts can perform authorized operations
- Master authority can grant and revoke permissions

### 2. Token Pause and Unpause

**Test**: `TestBusinessFlow_TokenPauseUnpause`

This test verifies the token pause mechanism for emergency situations:

1. **Issue Token**: Create a new token
2. **Grant Pause Authority**: Authorize an address to pause the token
3. **Pause Token**: Freeze all token transfers
4. **Unpause Token**: Resume token operations

**Verifications**:
- Pause authority is correctly granted
- Token state changes to paused
- Token state changes back to active after unpause

### 3. Blacklist Management

**Test**: `TestBusinessFlow_BlacklistManagement`

This test verifies blacklist functionality for compliance:

1. **Issue Private Token**: Create a private token (uses blacklist)
2. **Grant ManageList Authority**: Authorize blacklist management
3. **Add Address to Blacklist**: Prevent an address from receiving tokens
4. **Remove Address from Blacklist**: Restore normal operations

**Verifications**:
- Addresses are correctly added to blacklist
- Blacklist status is reflected in token metadata
- Addresses are correctly removed from blacklist

### 4. Token Metadata Updates

**Test**: `TestBusinessFlow_UpdateMetadata`

This test verifies the ability to update token information:

1. **Issue Token**: Create a new token
2. **Grant UpdateMetadata Authority**: Authorize metadata updates
3. **Update Token Metadata**: Change name, URI, and additional metadata

**Verifications**:
- Metadata update authority is granted
- Token name and URI are correctly updated
- Additional metadata fields are stored

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
- `LOCAL_API_URL`: URL for local development (default: `http://localhost:8080`)

## Running Business Flow Tests

### Run All Business Flow Tests

```bash
TEST_OPERATOR_PRIVATE_KEY=operator_key \
TEST_MASTER_PRIVATE_KEY=master_key \
go test -v -run "TestBusinessFlow" -timeout 10m
```

### Run Specific Business Flow Tests

```bash
# Test complete token lifecycle
TEST_OPERATOR_PRIVATE_KEY=operator_key \
TEST_MASTER_PRIVATE_KEY=master_key \
go test -v -run "TestBusinessFlow_CompleteTokenLifecycle" -timeout 10m

# Test pause/unpause
TEST_OPERATOR_PRIVATE_KEY=operator_key \
TEST_MASTER_PRIVATE_KEY=master_key \
go test -v -run "TestBusinessFlow_TokenPauseUnpause" -timeout 5m

# Test blacklist management
TEST_OPERATOR_PRIVATE_KEY=operator_key \
TEST_MASTER_PRIVATE_KEY=master_key \
go test -v -run "TestBusinessFlow_BlacklistManagement" -timeout 5m

# Test metadata updates
TEST_OPERATOR_PRIVATE_KEY=operator_key \
TEST_MASTER_PRIVATE_KEY=master_key \
go test -v -run "TestBusinessFlow_UpdateMetadata" -timeout 5m
```

### Run on Local Development Environment

```bash
TEST_ENV=local \
LOCAL_API_URL=http://localhost:8080 \
TEST_OPERATOR_PRIVATE_KEY=operator_key \
TEST_MASTER_PRIVATE_KEY=master_key \
go test -v -run "TestBusinessFlow" -timeout 10m
```

### Use Specific Test Accounts

```bash
TEST_OPERATOR_PRIVATE_KEY=operator_key \
TEST_MASTER_PRIVATE_KEY=master_key \
TEST_ACCOUNT1_PRIVATE_KEY=account1_key \
TEST_ACCOUNT2_PRIVATE_KEY=account2_key \
go test -v -run "TestBusinessFlow" -timeout 10m
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
- ⏸️  Pause operation
- ▶️  Unpause operation
- 🚫 Blacklist operation
- ⏳ Waiting for confirmation
- 📋 Transaction details
- ✅ Success
- ❌ Failure
- ⚠️  Warning

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

- `signMessage(payload, privateKey)`: Signs any payload with a private key

#### Account Generation

- `generateOrGetAccount(envVar)`: Creates or retrieves test account
  - Uses environment variable if set
  - Generates new keypair otherwise
  - Logs account address for debugging

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

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Business Flow Tests

on:
  schedule:
    - cron: '0 */6 * * *' # Run every 6 hours
  workflow_dispatch: # Allow manual trigger

jobs:
  business-flow-tests:
    runs-on: ubuntu-latest
    timeout-minutes: 30

    steps:
      - uses: actions/checkout@v2

      - uses: actions/setup-go@v2
        with:
          go-version: 1.21

      - name: Run Business Flow Tests
        run: go test -v -run "TestBusinessFlow" -timeout 20m
        env:
          TEST_ENV: testnet
          TEST_OPERATOR_PRIVATE_KEY: ${{ secrets.TEST_OPERATOR_PRIVATE_KEY }}
          TEST_MASTER_PRIVATE_KEY: ${{ secrets.TEST_MASTER_PRIVATE_KEY }}

      - name: Upload Test Results
        if: always()
        uses: actions/upload-artifact@v2
        with:
          name: test-results
          path: |
            *.log
            test-output.txt
```

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

    t.Run("Step 1: Setup", func(t *testing.T) {
        suite.refreshCheckpoint()

        // Your setup logic here

        t.Log("✅ Setup complete")
    })

    t.Run("Step 2: Execute", func(t *testing.T) {
        suite.refreshCheckpoint()

        // Your execution logic here

        receipt := suite.waitForTransaction(txHash, 60*time.Second)
        if !receipt.Success {
            t.Fatal("Transaction failed")
        }

        t.Log("✅ Execution complete")
    })

    t.Run("Step 3: Verify", func(t *testing.T) {
        // Your verification logic here

        t.Log("✅ Verification complete")
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
- **Blacklist Management**: 15-30 seconds (3 transactions)
- **Metadata Updates**: 10-20 seconds (2 transactions)

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
