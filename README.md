[![Go Reference](https://pkg.go.dev/badge/github.com/1Money-Co/1money-go-sdk.svg)](https://pkg.go.dev/github.com/1Money-Co/1money-go-sdk)
[![Go Report Card](https://goreportcard.com/badge/github.com/1Money-Co/1money-go-sdk)](https://goreportcard.com/report/github.com/1Money-Co/1money-go-sdk)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/1Money-Co/1money-go-sdk)
[![GitHub Tag](https://img.shields.io/github/v/tag/1Money-Co/1money-go-sdk?label=Latest%20Version)](https://pkg.go.dev/github.com/1Money-Co/1money-go-sdk)

# 1money-go-sdk

An SDK for the 1money blockchain in Go.

## Getting started

Add go to your `go.mod` file

```bash
go get -u  https://github.com/1Money-Co/1money-go-sdk
```

## Where can I see examples?

Take a look at `xx_test` for some examples of how to write clients.

## Where can I learn more?

You can read more about the Go SDK documentation on [1Money developer portal](https://developer.1moneynetwork.com/integrations/sdks/golang)

## Development

1. Make your changes
2. Update the CHANGELOG.md
3. Run `gofumpt -l -w .`
4. Run `golangci-lint run`
5. Commit with a good description
6. Submit a PR

# How to publish

1. Update changelog with a pull request
2. Create a new tag via e.g. v1.1.0 with the list of changes