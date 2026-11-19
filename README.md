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

## Token authority model: settled envelope and lane clawback

The next l1client release changes how token authority operations reach the
chain ([l1client#675](https://github.com/1Money-Co/l1client/issues/675)). After
a node activates the model, it refuses the legacy envelopes of mint, grant and
revoke, blacklist, whitelist, pause and unpause, metadata update, and the
clawback of an ordinary account. It takes two new envelopes instead:

- **Settled authority envelope** (native operation 17,
  [`SettledAuthorityPayload`](./settled_authority.go)): signed at the signer's
  nonce with a reference checkpoint; its checkpoint decides it.
- **Lane clawback** (native operation 18,
  [`LaneClawbackPayload`](./settled_authority.go)): takes the holder's next
  nonce (`FromNonce`) and no nonce of its signer; it executes at certification.

Two clawbacks stay legacy in every mode: the clawback of a Reserved Payment
hold, and the clawback of VERIFY by its master. VERIFY has no lane clawback.

### Send by envelope mode

[`Client.GetEnvelopeMode`](./envelope_mode.go) reads `envelope_mode` from
`GET /api/status`. Only a reported mode leaves the legacy envelopes:

| Node reports | The SDK sends |
|---|---|
| no mode (older node, hidden or failed status) | the legacy envelope |
| `upgrade_gap` | nothing: `ErrUpgradeGap`, retry later |
| `settled` | the settled envelope |
| a value this SDK does not know | nothing: `*UnrecognizedEnvelopeModeError` |

[`TokensAPI.AuthorityOperationByMode`](./authority_send.go) takes the legacy
payload that you build today (`LegacyMint`, `LegacyAuthority`,
`LegacyBlacklist`, `LegacyWhitelist`, `LegacyPause`, `LegacyMetadata`) and
sends it in the envelope of the node's mode. `SendAuthorityOperation` does the
same with an envelope that you name. Both sign again only after a refusal that
had no effect, at the same nonce:

- `legacy_envelope_retired`: the SDK reads the mode again and, once the node
  reports `settled`, signs the settled envelope. It never resends the legacy form.
- `reference_checkpoint_not_committed` or `token_not_at_reference_checkpoint`:
  one more signature with a fresh reference after a short wait.

Pass `WithOnSigned` to store each signed transaction before its send.
`AuthorityRefusalOf` reads the five refusals of the model, with their members,
from a rejected `/v2` submit (a `*SubmitError` whose `RequestHadNoEffect`). A
legacy v1 send is not classified, so its refusal is returned as it is.

If a gateway hides `GET /api/status`, the by-mode call sends the legacy
envelope and a node of the release refuses it as retired. Name the envelope:
`SendAuthorityOperation(ctx, onemoney.AuthorityEnvelopeSettled, ...)`.

### Read the outcome from positive evidence only

[`Client.AuthorityOperationState`](./authority_state.go) and
`Client.LaneClawbackFate` decide only on a receipt or on a counter-signed status
by hash (`certified_pending_checkpoint`, `checkpointed`, `finalized`). "Nonce
passed, no receipt" without such a status is **unknown**, never failed: a node
bootstrapped from a snapshot, or one that pruned receipts, gives that answer for
a transaction that executed. A settled operation or a lane clawback can stay
pending for up to three epoch transitions before the nodes evict it.

[`ReceiptOutcomeOf`](./authority_reads.go) reads a receipt as success, no-op,
soft fail (with its `SoftFailReason`) or failed. Show a no-op revoke
(`outcome.NoOpRevoke`) to the user as a warning: the named account held no
role, so another key can still hold it.

### Clawback

[`TokensAPI.ClawbackByMode`](./clawback_router.go) takes the legacy
`TokenClawbackPayload` and a **required** `onSigned` hook:

- VERIFY (by its master) and a Reserved Payment hold (`ClawbackFromHold`) stay
  the legacy clawback, with no mode read.
- In the settled mode, or after a retired legacy clawback once the node reports
  `settled`, the clawback of an ordinary account becomes the lane clawback at
  the holder's next nonce. It is signed only while the signer's nonce has not
  passed the intent's nonce (`*ClawbackIntentNonceUsedError`).
- Every signed clawback, legacy and lane, goes to `onSigned` before its send.
  An error from `onSigned` sends nothing.

**Call it once per intent.** The SDK keeps no record of earlier calls, and a
lane clawback never advances the signer's nonce, so a second call for the same
intent can sign a second lane clawback that debits the holder twice. After any
error, read the outcome of the last transaction stored
(`ParseLaneClawbackRecord` and `Client.LaneClawbackFate`, or
`Client.AuthorityOperationState` for a legacy clawback) and do not call again.

`SendClawback` does the same with an envelope that you name, and
`IssueLaneClawback` issues the lane clawback directly. All three refuse a client
in legacy v1 mode before signing: a `/v1` clawback has no
`AuthorizedTransaction` to store.

A clawback whose slot another transaction took cannot be told apart, by absence,
from one that executed. Issue it again only through
[`Client.PrepareLaneClawbackReissue`](./clawback_router.go), after a check on a
node with the full history (`FullHistoryChecked`). It refuses, with nothing
signed, when the reads show the clawback executed, certified, its slot not
passed, or known to a node. Otherwise it signs the replacement on the holder's
next slot and sends nothing: store it, send it with `Client.Submit`, and never
send the old one again.

For an offline signer, build the lane clawback with `Client.LaneClawbackPayload`,
sign it with `PrepareTransaction` and `Authorize`, and store it
(`NewLaneClawbackRecord`, `json.Marshal`) before `Client.Submit`.

See [`examples/v2_operations/authority_test.go`](./examples/v2_operations/authority_test.go).

## Error handling: problem details

A node answers every REST error with an
[RFC 9457](https://www.rfc-editor.org/rfc/rfc9457) problem body. `*APIError`
keeps `StatusCode`, `ErrorCode` and `Message`, and embeds a
[`Problem`](./problem.go) with the problem type, the client action and the typed
members. A body from an older node has no problem members, so the fields stay
empty. The [client guide](https://github.com/1Money-Co/l1client/blob/main/docs/api/error-handling.md)
lists every type, its action and its members.

The SDK follows the Rust SDK (`onemoney-protocol`, l1client `crates/om-sdk`):

- Only the node's statement decides. `Problem.Action` is set only when `Type` is
  in the node's namespace (`ProblemTypePrefix`) or is `about:blank`. A proxy, a
  firewall or a load balancer in front of the node can send problem details too;
  their `action` stays in `RawAction` for diagnostics and decides nothing. A type
  of the node that is newer than the SDK follows its action.
- Decide on `Type` and `Action`. Do not decide on `Message`, `Title`, the legacy
  `ErrorCode` or the HTTP status.
- Any 2xx answer is a success.

## Submit outcomes

Every `/v2` submit (`Client.Submit` and the `TokensAPI` and `TransactionsAPI`
methods) that does not end with the transaction admitted under its local hash
returns a [`*SubmitError`](./submit_error.go). `errors.As(err, &apiErr)` still
finds the `*APIError` of the answer. Its `Kind` tells the caller's duty:

| Kind | Proof | Next step |
|---|---|---|
| `SubmitRejected` | The node's action `fix_request`, `retry_later` or `use_other_node`. | This request had no effect. Fix the cause. Sign again only if no other transaction for the intent is locked, executed or unknown. |
| `SubmitLocked` | The node's action `in_progress` for this hash. | Resend the same signed transaction (`Transaction`) until it executes. |
| `SubmitOutcomeUnknown` | Any other answer: `check_status`, no action, a timeout, a 5xx, a firewall page, a redirect. | Find the state first: look up the hash and resend the same signed transaction. Do not sign the intent again. |
| `SubmitHashMismatch` | A 2xx with another hash. | The node admitted other bytes: send nothing more. |

`NonceUsed` is set only by the node's `nonce_too_low` type: never sign again at
that nonce. `Refusal` tells whether a rejection refused the signed bytes
themselves (`SubmitRefusalBytes`) or only this request. To resend, pass the same
`AuthorizedTransaction` to `Client.Submit`: the same hash cannot execute twice.
A legacy v1 send (`WithLegacyV1`) is not classified and returns an
`*APIError`, as in the Rust SDK.

See [`ExampleSubmitError`](./example_submit_test.go).

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
