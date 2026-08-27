[![Go Reference](https://pkg.go.dev/badge/github.com/1Money-Co/1money-protocol-go-sdk.svg)](https://pkg.go.dev/github.com/1Money-Co/1money-protocol-go-sdk)
[![Go Report Card](https://goreportcard.com/badge/github.com/1Money-Co/1money-protocol-go-sdk)](https://goreportcard.com/report/github.com/1Money-Co/1money-protocol-go-sdk)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/1Money-Co/1money-protocol-go-sdk)
[![GitHub Tag](https://img.shields.io/github/v/tag/1Money-Co/1money-protocol-go-sdk?label=Latest%20Version)](https://pkg.go.dev/github.com/1Money-Co/1money-protocol-go-sdk)

# 1money-protocol-go-sdk

An SDK for the 1money blockchain in Go.

## Getting started

Add go to your `go.mod` file

```bash
go get -u github.com/1Money-Co/1money-protocol-go-sdk
```

## v1.2.0: Domain-Separated Transaction Submission (default)

As of v1.2.0 the new submit API signs transactions with the domain-separated
("native v2") scheme (1Money L1 issue #1038) **by default**. This is a
backward-compatible release: the import path is unchanged and existing code
keeps working.

```go
import onemoney "github.com/1Money-Co/1money-protocol-go-sdk"
```

(The "v2" here is the L1 REST/signing **protocol** version, not the Go module
version — this is still a v1 module.)

### One-step submission

Submitting a transaction no longer requires hand-signing. Pass a payload and a
`Signer`; the SDK handles signing, encoding, memo, endpoint selection, and
response-hash verification:

```go
signer, _ := onemoney.NewPrivateKeySigner(privateKeyHex)
client := onemoney.NewClient() // domain-separated v2 by default

paymentResp, _ := client.Transactions().Payment(ctx, paymentPayload, signer)
mintResp, _    := client.Tokens().Mint(ctx, mintPayload, signer)
acct, _        := client.Accounts().CreateMultisig(ctx, multisigPayload, signer)
```

Submit methods live under `Transactions()`, `Tokens()`, and `Accounts()` and
share the shape `(ctx, payload, signer, ...SubmitOption)`. The payload carries
`chain_id` and `nonce` (fetch them via `GetChainId` and `GetAccountNonce`).
Attach a memo with `onemoney.WithMemo(memo)`.
See [`examples/v2_operations`](./examples/v2_operations) for compile-checked
`WithMemo` and default-`EmptyMemo` examples for every public v2 submit method.

### Custom signers (KMS / HSM / MPC)

Any backend can sign by implementing the `Signer` interface (`SignHash`,
`CompressedPublicKey`, `Address`) and passing it to the same methods — no other
change. `SignHash` must return the 0/1 y-parity `v` (never the legacy 27/28).

### Offline / external signing

When signing happens out-of-band, build the transaction, sign the digest
yourself, then submit — the one-step methods above run on this exact pipeline
internally:

```go
prep, _ := onemoney.PrepareTransaction(mintPayload) // offline, no network
digest  := prep.SigningHash()                       // 32-byte digest to sign
sig     := signExternally(digest)                   // -> onemoney.Signature{R, S, V}

authorized, _ := prep.Authorize(sig)                // holds the final tx hash
resp, _       := client.Submit(ctx, authorized)     // submit + hash-verify
```

### Multisig account address

`DeriveMultisigAddress(signers, threshold)` computes the account address
deterministically — identical to the address the node assigns — so you can
pre-fund or display it before submitting.

### Legacy v1

To stay on the legacy v1 path during the migration window, opt in explicitly:

```go
client := onemoney.NewClientWithOpts(onemoney.WithLegacyV1())
```

The pre-v2 methods (`SendPayment`, `MintToken`, `SignMessage`, the `*Request`
types, etc.) still work but are deprecated. See [MIGRATION.md](./MIGRATION.md)
for the migration guide, including how the v2 signing hash is built and why, and
[CHANGELOG.md](./CHANGELOG.md) for the full change list.

## v1.3.0: BatchPayment re-baselined (breaking)

**The canonical-format and public-API breaking changes in this release are
limited to `BatchPayment`.** `BatchPaymentPayload` is re-baselined onto the
current L1 canonical transaction format:

- `MaxFee` is removed from both the signed payload (`BatchPaymentPayload`) and
  the read-side decoded type (`BatchPaymentData`). Code that sets or reads
  `MaxFee` on either type no longer compiles.
- The payload now always signs as `WithMemo<BatchPaymentPayload>`, matching
  every other v2 operation.
- **Signing hashes and transaction hashes produced by earlier SDK versions are
  no longer valid for `BatchPayment`.** Rebuild and re-sign any
  prepared-but-unsubmitted batch; a cached `SigningHash()` from an
  offline/HSM flow is stale; a stored transaction hash used as a
  reconciliation key will not match.

Every other operation keeps the same signing bytes, wire format, and successful
submission behavior. Separately, all v2 operations now validate the node's
static memo rules before signing, so a memo the node would reject is returned as
a local error instead of being signed and sent first. See
[MIGRATION.md](./MIGRATION.md#batchpayment-2026-08-10-re-baseline) for the
caller-facing migration steps and [CHANGELOG.md](./CHANGELOG.md) for the full
detail.

`Transactions().BatchPayment` pays many recipients of one token in a single
transaction. It is v2-only: a client configured with `WithLegacyV1` returns an
error before signing or any network call, so there is no legacy fallback.

```go
payload := onemoney.BatchPaymentPayload{
    ChainID:   chainID,
    Nonce:     nonce,
    Token:     tokenAddress,
    CreatedAt: createdAt,
    Operations: []onemoney.PaymentOperation{
        {Recipient: recipientOne, Amount: amountOne},
        {Recipient: recipientTwo, Amount: amountTwo},
    },
}

resp, _ := client.Transactions().BatchPayment(ctx, payload, signer)
```

Attach a memo the same way as any other v2 operation, with
`onemoney.WithMemo(memo)`; omitting it signs the canonical empty memo.
`OperationsHash` is optional — populate it with
`DeriveBatchPaymentOperationsHash` if your system needs to publish the
operation set independently of whatever signs the transaction:

```go
hash, _ := onemoney.DeriveBatchPaymentOperationsHash(payload.Operations)
payload.OperationsHash = &hash
```

Get a non-binding, point-in-time fee quote for an unsigned batch before
submitting — it does not guarantee admission:

```go
estimate, _ := client.GetBatchPaymentEstimateFee(ctx, onemoney.BatchPaymentFeeEstimateRequest{
    From:       fromAddress,
    Token:      tokenAddress,
    Operations: payload.Operations,
})
```

The SDK rejects only amounts that cannot be encoded as U256 on this unsigned
path. The estimate service is authoritative for empty batches, recipient and
amount admission, aggregate overflow, and any future pricing semantics.

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
   - Unit tests, signing/conformance tests, and the v2 submission tests (routing,
     fail-closed hash verification, offline pipeline) run automatically — the v2
     tests use an in-memory HTTP transport and open no socket
   - The legacy socket-based HTTP client tests are disabled by default and require
     localhost (enable with `ENABLE_HTTP_CLIENT_TESTS=1`)
6. Commit with a good description
7. Submit a PR

## Testing

### Quick Start

```bash
# Run all tests. Includes the v2 submission tests (in-memory transport, no
# socket); only the legacy socket-based HTTP client tests are skipped here.
go test ./...

# Also run the legacy socket-based HTTP client tests (requires localhost)
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
