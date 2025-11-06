[![Go Reference](https://pkg.go.dev/badge/github.com/1Money-Co/1money-protocol-go-sdk.svg)](https://pkg.go.dev/github.com/1Money-Co/1money-protocol-go-sdk)
[![Go Report Card](https://goreportcard.com/badge/github.com/1Money-Co/1money-protocol-go-sdk)](https://goreportcard.com/report/github.com/1Money-Co/1money-protocol-go-sdk)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/1Money-Co/1money-protocol-go-sdk)
[![GitHub Tag](https://img.shields.io/github/v/tag/1Money-Co/1money-protocol-go-sdk?label=Latest%20Version)](https://pkg.go.dev/github.com/1Money-Co/1money-protocol-go-sdk)

# 1money-protocol-go-sdk

An SDK for the 1money blockchain in Go.

## Getting started

Add go to your `go.mod` file

```bash
go get -u  https://github.com/1Money-Co/1money-protocol-go-sdk
```

## Example

### TestNetwork

    client := onemoney.NewTestClient()
    result, err := client.GetCheckpointNumber()

### MainNetwork

    client := onemoney.NewClient()
    result, err := client.GetCheckpointNumber()

## Where can I learn more?

You can read more about the Go SDK documentation on [1Money developer portal](https://developer.1moneynetwork.com/integrations/sdks/golang)

## Development

1. Make your changes
2. Update the CHANGELOG.md
3. Run `gofumpt -l -w .`
4. Run `golangci-lint run`
5. Run unit tests:
   - `go test -v ./...` or `go test ./...`
6. Run integration tests:

   - `go test -tags=integration ./...`
   - Business flow tests: `TEST_OPERATOR_PRIVATE_KEY=operator_key TEST_MASTER_PRIVATE_KEY=master_key go test -v -run "TestBusinessFlow"`

7. Commit with a good description
8. Submit a PR

## Testing

The SDK includes comprehensive test coverage:

### Unit Tests

Standard Go unit tests for individual functions:

```bash
go test ./...
```

### API Integration Tests

Tests that verify API endpoints against live networks. See [INTEGRATION_TESTS.md](./INTEGRATION_TESTS.md) for details.

```bash
# Run all API integration tests (testnet)
go test -v -run "TestIntegration_AllAPIs"

# Run on specific environment
TEST_ENV=mainnet go test -v -run "TestIntegration"
```

### Business Flow Tests

End-to-end tests simulating complete business workflows like token lifecycle management. See [BUSINESS_FLOW_TESTS.md](./BUSINESS_FLOW_TESTS.md) for details.

```bash
# Requires operator and master private keys
TEST_OPERATOR_PRIVATE_KEY=operator_key \
TEST_MASTER_PRIVATE_KEY=master_key \
go test -v -run "TestBusinessFlow" -timeout 10m
```

# How to publish

1. Update changelog with a pull request
2. Create a new tag via e.g. v1.1.0 with the list of changes
