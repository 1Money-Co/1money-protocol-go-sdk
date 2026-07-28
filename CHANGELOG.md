# Changelog

All notable changes to the 1Money Protocol Go SDK are documented here. The
format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and
this project adheres to [Semantic Versioning](https://semver.org/).

## [1.2.0] - 2026-07-28

A **backward-compatible** release that adds domain-separated ("native v2")
transaction submission — the 1Money L1 issue #1038 scheme that closes a
cross-type signature-replay vulnerability. The new namespace submit API uses v2
signing **by default**; every pre-existing method keeps its signature and its
legacy v1 behavior, so upgrading requires no code changes.

> Note on naming: the `V2` in `SubmissionModeDomainSeparatedV2` and the `/v2`
> REST endpoints refer to the L1 REST/signing **protocol** version. They are
> unrelated to this module's (Go) version — this remains a v1 module, so the
> import path is unchanged.

### Added

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
- **Transaction decoding for the new operations**: `Transaction` now decodes
  `BatchPayment` / `TokenClawback` / `CreateMultiSig` transactions into
  `BatchPaymentData`, `TokenClawbackData`, and `CreateMultiSigData` (+
  `MultiSigSignerInfo`) via the `AsBatchPaymentData` / `AsTokenClawbackData` /
  `AsCreateMultiSigData` accessors and the matching `TransactionType` constants.
- **`Tokens().Metadata(common.Address)`** — the namespaced, typed token-metadata read.
- **New endpoint coverage**: batch payment, token clawback, multisig account
  creation (`/v2/accounts/multisig`), and reads `GetPricingPlanByID`,
  `GetPricingPlans`, and `GetNativeWriteStatus` (GET /api/status — check a
  network's native-write mode).
- **Typed status enums** `NativeWriteMode` (`v1_only` / `dual` / `v2_only`) and
  `ActivationSource` (`not_activated` / `capability_full` / `binary_release`)
  with named constants, matching the exact wire values.

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

### Notes

- The new namespace methods default to domain-separated v2 signing and POST to
  `/v2`. Their request body uses a tagged `authorization` union
  (`{"type":"single_secp256k1","signature":{r,s,v}}`, with `v` ∈ {0,1}) and a
  required `memo` object; U256 amounts are sent as decimal strings. This affects
  only the new methods — existing methods are untouched.
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

### Upgrade

The import path is unchanged:

```bash
go get github.com/1Money-Co/1money-protocol-go-sdk@v1.2.0
```

```go
import onemoney "github.com/1Money-Co/1money-protocol-go-sdk"
```

Existing code keeps working as-is. To adopt the secure v2 path, move submissions
to the namespace API (see [MIGRATION.md](./MIGRATION.md) for the full guide,
including how the v2 signing hash is built and why):

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

A future major release (`v2.0.0`, with a `/v2` module path) will remove the
deprecated legacy API; this `v1.2.0` only adds the new API and deprecates the
old one.
