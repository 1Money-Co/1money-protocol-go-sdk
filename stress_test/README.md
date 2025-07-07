# 1Money Batch Mint Stress Testing Tool

This stress testing tool performs concurrent batch minting operations to test the 1Money SDK's performance and reliability under load.

## Overview

The `TestBatchMint` test method performs the following workflow:

### Setup Phase
1. **Create 5 mint wallets** - Each containing private key, public key, and address to serve as mint_burn_authority addresses
2. **Create 100 transfer wallets** - Each containing private key, public key, and address to serve as transfer recipient addresses  
3. **Create a new token** - Using the operator wallet
4. **Grant mint permissions** - Operator wallet grants mint permissions to each of the 5 mint wallets, providing an allowance of 10,000,000 tokens each

### Execution Phase
5. **Launch 5 concurrent goroutines** - One per mint wallet, where each goroutine:
   - Takes responsibility for 20 of the 100 transfer wallets (distributed evenly: wallet 1 handles wallets 1-20, wallet 2 handles wallets 21-40, etc.)
   - Performs mint token operations to each of its assigned 20 transfer wallets sequentially

## Key Features

### Configurable Constants
All quantities are defined as configurable constants at the top of the file for easy modification:

```go
const (
    // Wallet Configuration
    MINT_WALLETS_COUNT     = 5   // Number of mint authority wallets
    TRANSFER_WALLETS_COUNT = 100 // Number of transfer recipient wallets
    WALLETS_PER_MINT       = 20  // Number of transfer wallets per mint wallet

    // Token Configuration
    TOKEN_SYMBOL   = "STRESS"
    TOKEN_NAME     = "Stress Test Token"
    TOKEN_DECIMALS = 6
    CHAIN_ID       = 1212101

    // Mint Configuration
    MINT_ALLOWANCE = 10000000 // Allowance granted to each mint wallet
    MINT_AMOUNT    = 1000     // Amount to mint per operation

    // Transaction Validation Configuration
    RECEIPT_CHECK_TIMEOUT    = 60 * time.Second // Timeout for waiting for transaction receipt
    RECEIPT_CHECK_INTERVAL   = 2 * time.Second  // Interval between receipt checks
    NONCE_VALIDATION_TIMEOUT = 30 * time.Second // Timeout for nonce validation
    NONCE_CHECK_INTERVAL     = 1 * time.Second  // Interval between nonce checks
)
```

### Transaction Validation
After each operation, the tool verifies both the transaction receipt and nonce increment:
- **Receipt Verification**: Checks that the transaction receipt is successfully retrieved and the transaction succeeded
- **Nonce Validation**: Verifies that the nonce has incremented by 1 (nonce+1)
- **Retry Loops**: If validation fails, implements retry loops that continue checking until confirmation is successful
- **Essential for Sequential Operations**: This is critical because transactions for each private key are sequential and immediate queries may not reflect completed state

### Concurrency
- Uses proper Go concurrency patterns (goroutines) for the multi-threaded approach
- Each mint wallet operates in its own goroutine for concurrent processing
- Sequential operations within each goroutine ensure proper nonce management

### Error Handling
- Comprehensive error handling and logging for debugging concurrent operations
- Clear progress reporting and result summaries
- Detailed error messages with context for troubleshooting

## Prerequisites

### Environment Setup
You need to configure operator wallet credentials using one of these methods:

#### Option 1: Environment Variables (Recommended)
```bash
export OPERATOR_PRIVATE_KEY="your_operator_private_key_here"
export OPERATOR_ADDRESS="your_operator_address_here"
```

#### Option 2: SDK Configuration
Configure the test constants in the main SDK:
```go
// In 1money.go
const (
    TestOperatorPrivateKey = "your_operator_private_key_here"
    TestOperatorAddress    = "your_operator_address_here"
    TestTokenAddress       = ""  // Not needed for this test
    Test2ndAddress         = ""  // Not needed for this test
)
```

### Dependencies
The tool requires:
- Go 1.21 or later
- 1Money Go SDK
- Ethereum Go library for wallet generation

## Usage

### Running the Test

1. **Navigate to the stress_test directory:**
   ```bash
   cd stress_test
   ```

2. **Install dependencies:**
   ```bash
   go mod tidy
   ```

3. **Run the stress test:**
   ```bash
   go test -v -run TestBatchMint
   ```

### Expected Output

The test will output detailed progress information:

```
=== Starting 1Money Batch Mint Stress Test ===
Configuration:
- Mint wallets: 5
- Transfer wallets: 100
- Wallets per mint: 20
- Mint allowance: 10000000
- Mint amount per operation: 1000
- Token symbol: STRESS
- Token name: Stress Test Token
- Chain ID: 1212101

Creating 5 mint wallets...
Created mint wallet 1: 0x...
...
✓ Step 1: Mint wallets created

Creating 100 transfer wallets...
Created transfer wallet 1: 0x...
...
✓ Step 2: Transfer wallets created

Creating token...
Token created successfully: 0x...
✓ Step 3: Token created

Granting mint authorities...
Authority granted to wallet 1, transaction: 0x...
...
✓ Step 4: Mint authorities granted

Starting concurrent minting operations...
Mint wallet 1 (0x...) minting to wallets 1-20
...
All concurrent minting operations completed successfully!
✓ Step 5: Concurrent minting completed

=== Stress Test Completed Successfully! ===
Token Address: 0x...
Total mint operations: 100
Total tokens minted: 100000
```

## Customization

### Modifying Test Parameters
Edit the constants at the top of `batch_mint_test.go` to customize the test:

- **Scale**: Increase `MINT_WALLETS_COUNT` and `TRANSFER_WALLETS_COUNT` for larger tests
- **Distribution**: Adjust `WALLETS_PER_MINT` to change how wallets are distributed among mint wallets
- **Token amounts**: Modify `MINT_ALLOWANCE` and `MINT_AMOUNT` for different token quantities
- **Timeouts**: Adjust timeout values for different network conditions

### Adding New Test Scenarios
The modular design allows easy extension:
- Add new test methods following the same pattern
- Implement different concurrency patterns
- Test different token operations (burn, transfer, etc.)

## Troubleshooting

### Common Issues

1. **Operator configuration not found**
   - Ensure environment variables are set correctly
   - Verify SDK test constants are configured

2. **Import errors**
   - Run `go mod tidy` to resolve dependencies
   - Ensure you're in the stress_test directory

3. **Transaction timeouts**
   - Increase timeout values in the constants
   - Check network connectivity to the test network

4. **Nonce validation failures**
   - This usually indicates network latency
   - Increase `NONCE_VALIDATION_TIMEOUT` and `NONCE_CHECK_INTERVAL`

### Performance Considerations

- The test creates many wallets and performs many transactions
- Execution time depends on network latency and transaction confirmation times
- Consider reducing quantities for faster testing during development
- Monitor network usage and rate limits

## Implementation Details

The stress testing tool follows Go testing conventions and can be executed using standard Go testing commands. It provides clear output about the progress and results of concurrent minting operations, making it suitable for both development testing and performance validation.
