# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is the Go SDK for the 1Money blockchain, providing a client library for interacting with 1Money network operations including token management, transactions, accounts, and checkpoints.

## Key Commands

### Development
- **Format code**: `gofumpt -l -w .`
- **Run linter**: `golangci-lint run`
- **Run all tests**: `go test ./...`
- **Run specific test**: `go test -v -run TestName`
- **Run stress tests**: `cd stress_test && ./run_test.sh`

### Module Management
- **Update dependencies**: `go mod tidy`
- **Download dependencies**: `go mod download`
- **Verify dependencies**: `go mod verify`

## Architecture Overview

### Core Client Structure
The SDK implements a client-server pattern with the main `OneMoneyClient` struct in `1money.go` that:
- Manages HTTP communication with 1Money API endpoints
- Provides logging capabilities with configurable log levels
- Implements request/response hooks for extensibility
- Handles both test network and main network configurations

### Key Components
1. **Client Initialization** (`1money.go`):
   - `NewClient()` - Creates client for main network
   - `NewTestClient()` - Creates client for test network
   - Supports custom configurations via `WithConfig()`

2. **API Operations** are organized by domain:
   - `accounts.go` - Token account operations, nonce management
   - `tokens.go` - Token listing and management
   - `transactions.go` - Transaction creation and submission
   - `checkpoints.go` - Checkpoint retrieval and verification
   - `chains.go` - Chain ID operations

3. **Cryptographic Operations** (`sign.go`):
   - Transaction signing using ECDSA
   - Private key management
   - Signature generation for blockchain operations

4. **Testing Infrastructure**:
   - Comprehensive unit tests using Go's standard testing package
   - HTTP mock servers for API testing
   - Separate stress testing module for performance validation

### Error Handling Pattern
The SDK consistently uses `(result, error)` return patterns. Always check errors before using results:
```go
result, err := client.GetTokens()
if err != nil {
    // Handle error
}
```

### Environment Configuration
For stress tests and operator functions:
- `OPERATOR_PRIVATE_KEY` - Private key for operator actions
- `OPERATOR_ADDRESS` - Operator's blockchain address

### Network Endpoints
- Test Network: `https://testapi.1moneynetwork.com`
- Main Network: `https://api.1moneynetwork.com`