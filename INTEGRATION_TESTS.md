# Integration Tests

This document describes how to run the integration tests for the 1Money Go SDK.

## Test Types

The SDK includes two types of integration tests:

### 1. API Integration Tests (`integration_test.go`)

Simple integration tests that verify individual API calls work correctly. These tests:
- Test each API endpoint independently
- Require minimal setup (read-only operations need no private key)
- Run quickly (complete in ~10 seconds)
- Suitable for CI/CD pipelines

### 2. Business Flow Tests (`business_flow_test.go`)

Comprehensive end-to-end tests that simulate complete business workflows. These tests:
- Test entire user journeys (token lifecycle, authority management, etc.)
- Require private keys and execute write operations
- Run longer (complete in ~5-10 minutes)
- Create real blockchain state changes

**See [BUSINESS_FLOW_TESTS.md](./BUSINESS_FLOW_TESTS.md) for detailed documentation on business flow tests.**

---

## API Integration Tests

The integration tests (`integration_test.go`) provide comprehensive testing of all SDK APIs against live environments. Tests are organized by functionality:

- **Chains**: Chain ID operations
- **Accounts**: Account nonce and token account queries
- **Checkpoints**: Checkpoint queries (by number/hash, with/without full transactions)
- **Tokens**: Token metadata and address derivation
- **Transactions**: Transaction queries, receipts, and fee estimation
- **Write Operations**: Payment sending (requires private key)

## Environment Configuration

Tests support multiple environments through the `TEST_ENV` environment variable:

| Environment | Description | Configuration |
|-------------|-------------|---------------|
| `testnet` (default) | 1Money testnet | Uses `NewTestClient()` |
| `mainnet` | 1Money mainnet | Uses `NewClient()` |
| `devnet` | Development network | Requires `DEVNET_API_URL` env var |
| `local` | Local development | Uses `LOCAL_API_URL` (default: http://localhost:8080) |

## Environment Variables

### Required for All Tests

None - tests will use default testnet configuration.

### Optional Configuration

- `TEST_ENV`: Environment to test against (`local`, `devnet`, `testnet`, `mainnet`)
- `TEST_ADDRESS`: Test wallet address (defaults to a known testnet address)
- `TEST_TOKEN_ADDRESS`: Test token address (defaults to a known testnet token)
- `TEST_TX_HASH`: Test transaction hash (defaults to a known testnet transaction)

### Required for Write Operations

- `TEST_PRIVATE_KEY`: Private key for signing transactions (without `0x` prefix or with it)
  - If not set, write operation tests will be skipped

### Environment-Specific URLs

- `LOCAL_API_URL`: URL for local environment (default: `http://localhost:8080`)
- `DEVNET_API_URL`: URL for devnet environment (required if using devnet)

## Running Tests

### Run All Integration Tests (Testnet)

```bash
go test -v -run "TestIntegration_AllAPIs"
```

### Run Tests on Specific Environment

```bash
# Test against mainnet
TEST_ENV=mainnet go test -v -run "TestIntegration_AllAPIs"

# Test against local development server
TEST_ENV=local LOCAL_API_URL=http://localhost:3000 go test -v -run "TestIntegration_AllAPIs"

# Test against devnet
TEST_ENV=devnet DEVNET_API_URL=https://devnet-api.1money.network go test -v -run "TestIntegration_AllAPIs"
```

### Run Specific Test Categories

```bash
# Test only chain operations
go test -v -run "TestIntegration_GetChainId"

# Test only checkpoint operations
go test -v -run "TestIntegration.*Checkpoint"

# Test only token operations
go test -v -run "TestIntegration.*Token"

# Test only transaction operations
go test -v -run "TestIntegration.*Transaction"
```

### Run Individual Tests

```bash
# Test getting chain ID
go test -v -run "TestIntegration_GetChainId"

# Test getting token metadata
go test -v -run "TestIntegration_GetTokenMetadata"

# Test checkpoint queries
go test -v -run "TestIntegration_GetCheckpointByNumber"
```

### Run Write Operation Tests

Write operations require a private key:

```bash
# With private key (will execute write operations)
TEST_PRIVATE_KEY=your_private_key_here go test -v -run "TestIntegration_SendPayment"

# Without private key (will skip write operations)
go test -v -run "TestIntegration_AllAPIs"
```

## Custom Test Configuration

You can customize test parameters using environment variables:

```bash
# Use custom addresses and tokens
TEST_ADDRESS=0x1234... \
TEST_TOKEN_ADDRESS=0x5678... \
TEST_TX_HASH=0xabcd... \
go test -v -run "TestIntegration_AllAPIs"
```

## Example: Complete Test Run

```bash
# Run all integration tests on testnet with custom configuration
TEST_ENV=testnet \
TEST_ADDRESS=0x0477fFa70fa8078d8265d963895Fa7Fd85426602 \
TEST_TOKEN_ADDRESS=0xb64864f92faf8daa2f27949e9ef374907be0788b \
TEST_TX_HASH=0xa80e74730f5e76d5014f73ac13ba5aa38bf2f0d54901c2b2b218a1a8adabf480 \
go test -v -run "TestIntegration_AllAPIs"
```

## Expected Output

Successful tests will show detailed output:

```
=== RUN   TestIntegration_AllAPIs
    integration_test.go:XXX: Running comprehensive integration tests on testnet environment
=== RUN   TestIntegration_AllAPIs/Chains
=== RUN   TestIntegration_AllAPIs/Chains/GetChainId
    integration_test.go:XXX: Testing GetChainId on testnet environment
    integration_test.go:XXX: ✓ Successfully retrieved chain ID: 1212101
=== RUN   TestIntegration_AllAPIs/Accounts
...
    integration_test.go:XXX: ✅ All integration tests completed on testnet environment
--- PASS: TestIntegration_AllAPIs (X.XXs)
```

## Troubleshooting

### Tests Fail with Connection Error

- Verify the API endpoint is accessible
- Check your internet connection
- For local/devnet, ensure the server is running

### Tests Fail with 404 Not Found

- The default test data (addresses, hashes) may not exist in your environment
- Set custom test data using environment variables
- For new environments, use data you know exists

### Write Tests are Skipped

- This is expected when `TEST_PRIVATE_KEY` is not set
- Write operations modify blockchain state and require authentication
- Set `TEST_PRIVATE_KEY` only if you intend to execute write operations

### Rate Limiting

- If you see rate limit errors, add delays between test runs
- Consider testing against local/devnet for development

## CI/CD Integration

Example GitHub Actions workflow:

```yaml
name: Integration Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - uses: actions/setup-go@v2
        with:
          go-version: 1.21

      - name: Run Integration Tests (Testnet)
        run: go test -v -run "TestIntegration_AllAPIs"
        env:
          TEST_ENV: testnet
          TEST_ADDRESS: ${{ secrets.TEST_ADDRESS }}
          TEST_TOKEN_ADDRESS: ${{ secrets.TEST_TOKEN_ADDRESS }}
          TEST_TX_HASH: ${{ secrets.TEST_TX_HASH }}

      # Optionally test write operations
      - name: Run Write Operation Tests
        if: secrets.TEST_PRIVATE_KEY != ''
        run: go test -v -run "TestIntegration_SendPayment"
        env:
          TEST_ENV: testnet
          TEST_PRIVATE_KEY: ${{ secrets.TEST_PRIVATE_KEY }}
```

## Contributing

When adding new SDK methods:

1. Add corresponding integration tests to `integration_test.go`
2. Follow the existing test naming convention: `TestIntegration_<MethodName>`
3. Add configuration for any required test data
4. Update this documentation with new environment variables or requirements
