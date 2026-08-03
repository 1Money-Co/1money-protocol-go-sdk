# Changelog

All notable changes to the 1Money Protocol Go SDK are documented here. The
format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and
this project adheres to [Semantic Versioning](https://semver.org/).

## [1.2.0] - 2026-07-29

This release is the **complete set of changes since `v1.1.0`**. It does two
things:

1. Adds domain-separated ("native v2", 1Money L1 issue #1038) transaction
   submission, closing a cross-type signature-replay vulnerability.
2. Aligns the query/read response types with the current L1 (`l1client`) REST
   wire format.

The **submit / signing API is additive and backward-compatible**: every
pre-existing method keeps its signature and legacy v1 behavior, and the new
namespace API uses v2 signing by default. The **read / query response-type
alignment includes breaking changes** for code that decodes certain fields (see
"Changed"). The package was also reorganized into a flat, domain-oriented file
layout with **no public-API change** (guarded by a `go doc` surface-hash test).

> Note on naming: the `V2` in `SubmissionModeDomainSeparatedV2` and the `/v2`
> REST endpoints refer to the L1 REST/signing **protocol** version. They are
> unrelated to this module's (Go) version — the import path is unchanged.

### Added — v2 submission and signing

- **`Signer` interface** and `NewPrivateKeySigner(hexKey)`. Signing is done
  through this abstraction; a KMS/HSM signer can implement the same interface
  with no other change.
- **Resource-namespaced submit API**, all hiding signing:
  - `client.Transactions()`: `Payment`, `BatchPayment`.
  - `client.Tokens()`: `Issue`, `Mint`, `Burn`, `BridgeAndMint`,
    `BurnAndBridge`, `GrantAuthority`, `RevokeAuthority`, `Clawback`,
    `ManageBlacklist`, `ManageWhitelist`, `Pause`, `Unpause`, `UpdateMetadata`.
  - `client.Accounts()`: `CreateMultisig` (creates a multisig account; the
    creation transaction is single-signed). The response's `Account` is the
    created multisig address, filled by local derivation since the endpoint
    returns only the transaction hash.
  - Every submit method has the shape `(ctx, payload, signer, ...SubmitOption)`.
- **`DeriveMultisigAddress(signers, threshold)`** — computes a multisig account
  address deterministically, byte-for-byte identical to the address the node
  assigns; usable before submitting (e.g. to pre-fund or display it).
- **Offline / build-sign-submit pipeline** — `PrepareTransaction(payload,
  ...SubmitOption)` returns a `PreparedTransaction` exposing `SigningHash()` (the
  digest a signer must sign). After signing externally, `Authorize(sig)` returns
  an `AuthorizedTransaction` (with `TransactionHash()`), which `Client.Submit(ctx,
  authorized)` sends and hash-verifies. This lets external / offline / HSM signers
  drive the whole flow without touching RLP, memo, authorization, or endpoint
  internals — and the one-step namespace methods run on this exact same pipeline
  internally (prepare → sign → authorize → submit). `WithManageListKind`
  disambiguates a blacklist vs whitelist `TokenManageListPayload`.
- **`SubmissionMode`** (`SubmissionModeDomainSeparatedV2` default,
  `SubmissionModeLegacyV1`) with client options `WithSubmissionMode(mode)` and
  `WithLegacyV1()`.
- **`Memo`** type and the `WithMemo(memo)` submit option. The default is the
  canonical empty memo; no code change is needed for memo-less transactions.
- **New payload/response types**: `BatchPaymentPayload` (+`PaymentOperation`),
  `TokenClawbackPayload`, `CreateMultiSigPayload` (+`MultiSigSigner`),
  `CreateMultisigResponse`.
- **Transaction decoding for the new operations**: `Transaction` decodes
  `BatchPayment` / `TokenClawback` / `CreateMultiSig` transactions into
  `BatchPaymentData`, `TokenClawbackData`, and `CreateMultiSigData` (+
  `MultiSigSignerInfo`) via the `AsBatchPaymentData` / `AsTokenClawbackData` /
  `AsCreateMultiSigData` accessors and the matching `TransactionType` constants.

### Added — read / query surface

- **`Tokens().Metadata(common.Address)`** — the namespaced, typed
  token-metadata read.
- **New read endpoints**: `GetPricingPlanByID`, `GetPricingPlans`, and
  `GetNativeWriteStatus` (GET /api/status — check a network's native-write mode),
  plus multisig account creation (`/v2/accounts/multisig`).
- **Typed status enums** `NativeWriteMode` (`v1_only` / `dual` / `v2_only`) and
  `ActivationSource` (`not_activated` / `capability_full` / `binary_release`)
  with named constants, matching the exact wire values.
- **Receipt execution detail** on `TransactionReceiptResponse`: `SuccessInfo`
  (`+BridgeInfo`), `BatchInfo` (`+BatchFailureInfo`), and `ExecutionEvents` —
  populated for batch payments and successful/bridged transactions.
- **Fee-binding fields** on `FinalizedTransactionResponse`: `Fee` and `FeeBound`.
- **`EstimateFeeResponse.Plan`** — the pricing plan applied to the estimate.
- **Clawback metadata**: `TokenInfoResponse.ClawbackEnabled` /
  `ClawbackAuthorities`, and `TokenCreateData.ClawbackEnabled`.
- **New response types**: `MultiSigSignature`, `MultiSigSignatureEntry`,
  `BlsAggregateSignature`, `SuccessInfo`, `BridgeInfo`, `BatchReceiptInfo`,
  `BatchFailureInfo`, `ExecutionEvent`, `SignatureScheme` (+ constants), and
  pricing types `PricingCriteria`, `PricingFeeTier`, `PricingFeeFormula`,
  `PricingPlanVersion`.

### Changed

Read/query response types were aligned to the L1 wire format. These are
**breaking for callers that decode the affected fields**; the submit/signing API
is unaffected.

- **BREAKING — nullable transaction/checkpoint fields.** On `Transaction` and
  `TransactionReceiptResponse`, `CheckpointHash`, `CheckpointNumber`, and
  `TransactionIndex` are now pointers (`*string` / `*uint64` / `*uint64`); they
  are `nil` until the transaction is included in a checkpoint. Previously they
  were non-nullable, so a pending transaction decoded as `checkpoint 0` /
  `index 0`. `Checkpoint.Size` is likewise now `*uint64`.
- **BREAKING — polymorphic transaction signature.** `Transaction` now exposes
  `SignatureType` (`"Single"` / `"Multi"`) plus `MultiSignature
  (*MultiSigSignature)` alongside `Signature (*Signature)`. A multisig-signed
  transaction populates `MultiSignature`; a single-signer one populates
  `Signature`. Added `Transaction.Memo` and `Transaction.SignatureScheme`.
- **BREAKING — `FinalizedTransactionResponse` counter signature.** Replaced
  `CounterSignatures []Signature` (wire key `counter_signatures`) with
  `CounterSignature BlsAggregateSignature` (wire key `counter_signature`). The
  node returns a single BLS aggregate, so the previous field never decoded.
- **BREAKING — nullable payload fields.** `TokenTransferData.Token` is now
  `*common.Address` (`nil` for a native-value transfer), and
  `TokenGrantAuthorityData.Value` / `TokenRevokeAuthorityData.Value` are now
  `*string` (`nil` when the authority carries no allowance).
- **BREAKING — typed pricing plan.** `PricingPlan.Criteria` and `.Tiers` are now
  strongly typed (`[]PricingCriteria` / `[]PricingFeeTier`) instead of
  `json.RawMessage`, and `PricingPlan.Version` is a typed `PricingPlanVersion`
  enum.
- **BREAKING — removed `TransactionReceiptResponse.To`.** The node no longer
  returns a `to` field (it was already deprecated in favor of `Recipient`).
- Reorganized the package into a flat, domain-oriented file layout (e.g. `doc.go`,
  `primitives.go`, `signature.go`, `legacy.go`, `transactions_decode.go`,
  `misc.go`; `*_client.go` renamed to `tokens.go` / `transactions.go` /
  `checkpoints.go`). The exported API is unchanged and guarded by a `go doc`
  surface-hash test.

### Deprecated

- The legacy exposed-signing API is deprecated (but still works, unchanged) in
  favor of the namespace submit methods: `SignMessage`, `EncodePayload`,
  `HashMessage`, the `*Request` types, and the legacy write methods
  (`SendPayment`, `IssueToken`, `MintToken`, `BridgeAndMintToken`, `BurnToken`,
  `BurnAndBridgeToken`, `SetTokenBlacklist`, `SetTokenWhitelist`, `PauseToken`,
  `GrantTokenAuthority`, `UpdateTokenMetadata`).
- `PrivateKeyToAddress(hex)` — use `NewPrivateKeySigner(hex).Address()`, which
  returns a `common.Address` directly. The function still works, unchanged.
- `Client.GetTokenMetadata(hex)` — use `Tokens().Metadata(common.Address)`, the
  namespaced, typed equivalent. The method still works, unchanged.

### Fixed

- **Chain-id read endpoint** corrected to `GET /v1/chains/chain_id`.
- **Read decode bugs** closed by the wire-format alignment: multisig-signed
  transactions no longer decode with a silently-zeroed signature; the
  finalized-transaction BLS counter signature now decodes; a pending
  transaction's checkpoint fields no longer read as `0` / `""`; and memo,
  signature scheme, batch/success receipt detail, clawback metadata, and
  fee-binding fields are no longer dropped on decode.

### Tests

- **Query interfaces**: every read/query method now has a default-runnable unit
  test (in-memory `http.RoundTripper`, no live node and not gated behind
  `ENABLE_HTTP_CLIENT_TESTS`), asserting route, query params, and response
  decode. Edge cases: nullable pending-transaction fields, single vs multisig
  signatures, the checkpoint `full` flag, the `estimate_fee` `to` param,
  pricing-id path escaping, and non-200 / malformed responses.
- **Native v2 golden vectors**: signing/transaction hashes are validated
  byte-for-byte against fixtures exported from `l1client` production code.
- **Business-flow integration tests** migrated to the v2 namespace API, adding
  batch-payment, clawback, blacklist-management, and multisig flows.

### Notes

- The new namespace methods default to domain-separated v2 signing and POST to
  `/v2`. Their request body uses a tagged `authorization` union
  (`{"type":"single_secp256k1","signature":{r,s,v}}`, with `v` ∈ {0,1}) and a
  required `memo` object; U256 amounts are sent as decimal strings. This affects
  only the new methods — existing submit methods are untouched.
- Do not automatically retry a failed v2 submission as v1: re-signing the same
  nonce under a different scheme can create two transactions for one nonce. The
  v2 path verifies the server-returned transaction hash against a locally
  computed hash and fails closed on mismatch.
- `Authorize` validates a signature exactly as the node does before submission:
  `v` must be the 0/1 y-parity, `r` and `s` must be in `[1, N)`, and `s` must be
  canonical low-S (`s ≤ N/2`). A custom (KMS/HSM) signer that emits a high-S
  signature now fails fast with a clear error instead of a server-side rejection.
  The built-in private-key signer already produces low-S signatures.
- The namespace methods fail fast on caller mistakes rather than proceeding
  silently: a nil `Signer` returns an error (no panic), and an explicit
  `WithMemo` on a path that carries no memo (legacy v1 mode, or a batch payment)
  is rejected instead of being dropped.
- **Read-type migration.** Callers that decode `CheckpointNumber` /
  `CheckpointHash` / `TransactionIndex`,
  `FinalizedTransactionResponse.CounterSignatures`, `TokenTransferData.Token`,
  `TokenGrantAuthorityData.Value` / `TokenRevokeAuthorityData.Value`,
  `Checkpoint.Size`, or `TransactionReceiptResponse.To` must adjust for the new
  pointer / renamed / removed fields.

### Upgrade

The import path is unchanged:

```bash
go get github.com/1Money-Co/1money-protocol-go-sdk@v1.2.0
```

```go
import onemoney "github.com/1Money-Co/1money-protocol-go-sdk"
```

Existing submit code keeps working as-is. To adopt the secure v2 path, move
submissions to the namespace API (see [MIGRATION.md](./MIGRATION.md) for the
full guide, including how the v2 signing hash is built and why):

```go
// Before (still works; deprecated):
sig, _ := client.SignMessage(payload, privateKeyHex)
resp, _ := client.SendPayment(ctx, &onemoney.PaymentRequest{PaymentPayload: payload, Signature: *sig})

// After:
signer, _ := onemoney.NewPrivateKeySigner(privateKeyHex)
resp, _ := client.Transactions().Payment(ctx, payload, signer) // domain-separated v2 by default
```

To stay on legacy v1 during the migration window, opt in explicitly:

```go
client := onemoney.NewClientWithOpts(onemoney.WithLegacyV1())
```

Because the read-type alignment is breaking, code that decodes query responses
should review the "Changed" section above before upgrading. A future major
release (with a `/v2` module path) will remove the deprecated legacy API.
