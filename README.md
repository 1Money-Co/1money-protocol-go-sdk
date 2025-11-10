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
5. Run tests: `go test ./...`
   - Unit tests and transaction tests will run automatically
   - HTTP client tests are disabled by default (enable with `ENABLE_HTTP_CLIENT_TESTS=1`)
6. Commit with a good description
7. Submit a PR

## Testing

### Quick Start

```bash
# Run all tests (HTTP client tests disabled by default)
go test ./...

# Enable HTTP client tests (requires localhost)
ENABLE_HTTP_CLIENT_TESTS=1 go test ./...
```

### Business Flow Tests

End-to-end tests simulating complete business workflows like token lifecycle management. See [BUSINESS_FLOW_TESTS.md](./BUSINESS_FLOW_TESTS.md) for details.

```bash
# Requires operator and master private keys
TEST_ENV=local \
TEST_OPERATOR_PRIVATE_KEY=operator_key \
TEST_MASTER_PRIVATE_KEY=master_key \
go test -v -tags=integration -run "TestBusinessFlow" -timeout 10m

# Run all integration tests
TEST_ENV=local \
TEST_OPERATOR_PRIVATE_KEY=0xoperator_key \
TEST_MASTER_PRIVATE_KEY=0xmaster_key \
go test -v -tags=integration ./... -timeout 10m
```

# How to publish

1. Update changelog with a pull request
2. Create a new tag via e.g. v1.1.0 with the list of changes
