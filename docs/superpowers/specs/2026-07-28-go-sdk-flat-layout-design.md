# Go SDK Flat Package Layout Design

**Status:** Approved in design discussion; awaiting written-spec review
**Date:** 2026-07-28
**Target repository:** `1money-protocol-go-sdk`
**Target release:** `v1.2.0`

## 1. Purpose

The Go SDK has added a domain-separated native-v2 submission surface while
retaining its legacy-v1 API. The implementation is correct but the root package
now mixes domain clients, public DTOs, legacy compatibility, transaction
decoding, and native-v2 protocol layers across inconsistently named files.

This design reorganizes those declarations inside the existing root `onemoney`
package. It does not create a subpackage, internal package, conversion layer, or
second type model.

The refactor is structural only. It must not change valid or invalid runtime
behavior, public API, request JSON, endpoints, hashes, errors, or submission
mode.

## 2. Why a Flat Package

The repository already organizes source files by domain in one root package.
The SDK is small enough that its unexported native-v2 engine remains
understandable without a compile-time subpackage boundary.

Keeping one package provides:

- No public/internal type duplication.
- No conversion adapters.
- No import-cycle risk.
- No change to public type identity.
- Minimal risk to the security-sensitive encoder and hash implementation.
- Preservation of the existing repository organization.

An `internal/nativev2` package would provide a stronger compile-time boundary,
but would require duplicate protocol payload types, validating constructors,
root facades, and fourteen sets of conversions. That cost and migration risk are
not justified by the current code size.

## 3. Goals

- Add package-level documentation and a stable responsibility map.
- Isolate the deprecated legacy request, hashing, and exposed-signing surface in
  one compatibility file.
- Apply one domain naming convention to accounts, tokens, transactions, and
  checkpoints.
- Split transaction DTOs, polymorphic payloads, and decoding responsibilities.
- Keep native-v2 protocol layers visually identifiable through file names and
  package documentation.
- Preserve every existing public symbol required for a non-breaking `v1.2.0`.
- Preserve all current tests and golden-vector coverage.

## 4. Non-goals

- No `internal/nativev2`, public `nativev2`, or `v2` subpackage.
- No Go module v2 or `/v2` module import path.
- No duplicate or replacement payload types.
- No changes to operation types, RLP, descriptors, proofs, signing hashes, or
  transaction hashes.
- No changes to REST paths, JSON tags, request bodies, responses, or modes.
- No new validation or error behavior.
- No nil-U256, memo, signer, option, or signature behavior changes.
- No automatic v1/v2 fallback.
- No new endpoint or API.
- No multisig-authorized operation submission.
- No unrelated Client, transport, hooks, logger, pricing, or status refactor.

## 5. Zero-behavior-change Contract

The comparison baseline is:

- Public legacy behavior and API from `main`.
- Current branch behavior for the unreleased native-v2 APIs.

The layout refactor must preserve:

- Exported names, type identity, fields, methods, signatures, and JSON tags.
- Which value and pointer payload forms are accepted.
- Existing nil-to-zero handling through `bigOrZero`.
- Existing validation and error text.
- Default submission mode and explicit legacy behavior.
- Every v1 and v2 endpoint mapping.
- Request JSON shape.
- L1 golden signing and transaction hashes.
- Hook and logger behavior.

`PrivateKeyToAddress` has already been restored in the current branch as a
deprecated compatibility wrapper. The layout refactor only moves it from
`signer.go` to `legacy.go`; it does not change its implementation or public API.

Behavior hardening may be designed and reviewed separately after this layout
refactor. It must not be mixed into this change.

## 6. Package Documentation

Add a root `doc.go` of approximately 40 lines. It documents concepts rather than
maintaining an exhaustive file inventory that would become stale.

It covers:

- The `onemoney` package purpose.
- The domain convention:
  - `<domain>.go` for client and namespace methods.
  - `<domain>_types.go` for domain DTOs.
  - `<domain>_payloads.go` for large polymorphic payload families when needed.
  - `<domain>_decode.go` for substantial custom decoding when needed.
- The native-v2 layers:
  - canonical encoding and hashing.
  - prepare and authorize.
  - REST wire snapshot assembly.
  - submission pipeline.
- The `legacy.go` compatibility boundary.
- The rule that implementation remains in one root package.

Package documentation may name representative files but must not promise a
complete file-by-file map.

## 7. Final Source Layout

```text
doc.go

client.go
primitives.go
signature.go
legacy.go

accounts.go
accounts_types.go

tokens.go
tokens_types.go

transactions.go
transactions_types.go
transactions_payloads.go
transactions_decode.go

native_v2.go
native_v2_encoding.go
native_v2_prepare.go
native_v2_requests.go
native_v2_wire.go
native_v2_multisig.go

memo.go
submission_mode.go
pricing.go
status.go
chains.go
checkpoints.go
checkpoints_types.go
```

Test files remain beside the corresponding source in the root package.
Golden-vector fixtures remain in the existing root `testdata/` directory.

## 8. Shared Primitives and Signing

### 8.1 `primitives.go`

`primitives.go` contains cross-domain byte primitives and their JSON helpers:

- `B256`.
- `Bytes`.
- `HexBytes`.

These are not transaction-domain concepts and should not remain in
`transactions_types.go`.

### 8.2 `signature.go`

`Signature` is shared by both legacy and v2 APIs, so it is not placed in
`legacy.go`.

`signature.go` contains:

- `Signature`.
- `Signer`.
- The built-in private-key signer implementation.
- `NewPrivateKeySigner`.

This is a declaration move only. The `Signer` contract and signing behavior do
not change.

## 9. Legacy-v1 Compatibility Boundary

`legacy.go` contains exported APIs that exist only for legacy-v1 compatibility:

- `PrivateKeyToAddress`.
- `EncodePayload`.
- `HashMessage`.
- `(*Client).SignMessage`.
- `Hash`.
- Legacy RLP signature helpers.
- Every deprecated `*Request` wrapper.
- Every deprecated request `Hash()` method.

The request set includes payment and all token request wrappers currently
distributed between `transactions_types.go` and `tokens_models.go`.

`legacy.go` does not contain:

- `Signature`, because v2 uses it.
- Shared business payloads, because both legacy and v2 use them.
- Shared `*Response` DTOs.
- Namespace methods.
- Native-v2 protocol helpers.

Responses and shared domain DTOs move to the relevant `<domain>_types.go`.
Deprecated domain submit methods such as `SendPayment`, `IssueToken`, and
`MintToken` intentionally remain in their domain method files beside the
namespace replacements. This keeps migration paths discoverable; `legacy.go`
centralizes legacy request types and exposed signing/hash helpers, not every
deprecated method.

All moved legacy declarations retain their exact exported names, doc comments,
method sets, JSON tags, and behavior. Deprecated doc comments remain visible to
pkg.go.dev and gopls.

## 10. Domain Organization

### 10.1 Accounts

`accounts.go` contains account read methods and the `Accounts()` namespace.

`accounts_types.go` contains account responses, multisig creation payloads,
multisig signer DTOs, and multisig responses.

### 10.2 Tokens

Rename `tokens_client.go` to `tokens.go`.

`tokens.go` contains:

- Token read methods.
- Deprecated legacy submit methods that accept legacy request wrappers.
- The `Tokens()` namespace and v2-aware submit methods.
- Token endpoint constants.

Legacy request types do not remain in this file; they live in `legacy.go`.

`tokens_types.go` contains:

- Shared token business payloads.
- Token action and authority enums.
- Token read responses.
- Token operation responses.
- Metadata DTOs.

The mixed `tokens_models.go` file is eliminated after its legacy requests move
to `legacy.go` and its shared responses move to `tokens_types.go`.

### 10.3 Transactions

Rename `transactions_client.go` to `transactions.go`.

`transactions.go` contains:

- Transaction read methods.
- Deprecated `SendPayment`.
- The `Transactions()` namespace.
- Transaction endpoint constants.

`transactions_types.go` contains stable transaction-domain declarations:

- `TransactionType`.
- `TransactionPayload`.
- `Transaction`.
- Receipt, finalized, fee, payment, and batch-payment DTOs.
- Transaction and payment responses.

`transactions_payloads.go` retains the large polymorphic response-payload
family such as token create, transfer, clawback, multisig, and raw transaction
data.

`transactions_decode.go` contains:

- `Transaction.UnmarshalJSON`.
- Payload discriminator decoding.
- `As*` typed-accessor methods.
- The `isTransactionPayload` implementations needed by the closed response
  payload union.

`PaymentRequest` and its legacy `Hash()` method move to `legacy.go`.
`B256` and `Bytes` move to `primitives.go`.

### 10.4 Checkpoints

Rename `checkpoints_client.go` to `checkpoints.go`.

`checkpoints.go` contains checkpoint client methods and endpoint constants.
`checkpoints_types.go` continues to contain checkpoint DTOs. This applies the
same `<domain>.go` plus `<domain>_types.go` convention used by accounts, tokens,
and transactions.

## 11. Native-v2 File Layers

Native-v2 code remains unexported in the root package except for the existing
public facades and DTOs.

The layer ownership is:

| File | Responsibility |
| --- | --- |
| `native_v2.go` | Frozen domain, operation registry, descriptors, proofs, signing hash, transaction hash, shared protocol helpers |
| `native_v2_encoding.go` | Canonical RLP field order for every native payload |
| `native_v2_prepare.go` | Payload-to-operation resolution, `PrepareTransaction`, `PreparedTransaction`, authorization facade |
| `native_v2_wire.go` | Immutable REST business-field snapshots and JSON wire values |
| `native_v2_requests.go` | v1/v2 request assembly, submit pipeline, endpoint selection, server-hash verification |
| `native_v2_multisig.go` | Compressed-key validation, multisig configuration validation, address derivation |

There is one operation registry, one canonical encoder, and one hash
implementation. The layout refactor must not copy or rewrite these functions.

## 12. Current-to-final File Mapping

| Current file | Final destination |
| --- | --- |
| `client.go` | Remain unchanged |
| `chains.go` | Remain unchanged |
| `tokens_client.go` | Rename to `tokens.go` |
| `tokens_models.go` | Legacy requests to `legacy.go`; responses and metadata models to `tokens_types.go`; then delete the empty source file |
| `tokens_types.go` | Retain token DTOs and receive the non-legacy token models |
| `transactions_client.go` | Rename to `transactions.go` |
| `transactions_types.go` | Retain domain types; decoding to `transactions_decode.go`; primitives to `primitives.go`; `PaymentRequest` to `legacy.go` |
| `transactions_payloads.go` | Retain polymorphic response payloads |
| `sign.go` | `Signature` to `signature.go`; legacy functions to `legacy.go`; then delete the empty source file |
| `signer.go` | `Signer`, private-key signer implementation, and `NewPrivateKeySigner` to `signature.go`; `PrivateKeyToAddress` to `legacy.go`; then delete the empty source file |
| `hash.go` | Move legacy hash implementation to `legacy.go`; then delete the empty source file |
| `hex_bytes.go` | Merge into `primitives.go`; then delete the empty source file |
| `accounts.go` | Retain account methods |
| `accounts_types.go` | Retain account DTOs |
| `checkpoints_client.go` | Rename to `checkpoints.go` |
| `checkpoints_types.go` | Retain checkpoint DTOs |
| `native_v2*.go` | Remain in root package with existing layer boundaries |
| `memo.go`, `submission_mode.go` | Remain unchanged |
| `pricing.go`, `status.go` | Remain unchanged |

Moving declarations between files in the same package does not change their Go
import path or type identity.

After all declarations have moved, `tokens_models.go`, `sign.go`, `signer.go`,
`hash.go`, and `hex_bytes.go` must be deleted. The final tree must not retain
empty or comment-only orphan source files.

## 13. Tests

Existing tests remain in `package onemoney` and require only filename or helper
location adjustments caused by declaration moves.

The refactor must preserve:

- All native-v2 golden-vector tests.
- Payload RLP conformance for all operations.
- Multisig address vectors and validation tests.
- API request-shape and endpoint tests.
- Offline and one-step pipeline equivalence.
- Legacy request hash tests.
- Hook and HTTP client tests.
- Business integration test compilation.

Add or retain compatibility assertions for:

- `PrivateKeyToAddress` remains callable.
- Legacy request types and `Hash()` methods remain callable.
- Public API surface before and after the refactor is exactly equal.
- Valid request JSON, endpoints, signing hashes, and transaction hashes are
  unchanged.
- Current invalid-input behavior and error strings are unchanged.

Tests must not be rewritten to bless behavior changes.

## 14. Migration Sequence

1. Record the current public API, request JSON, endpoint, error, and
   golden-vector baselines.
2. Add `doc.go`.
3. Create `primitives.go` and move byte primitives without modification.
4. Create `signature.go` and move shared signature/signer declarations.
5. Create `legacy.go`, move the already-restored `PrivateKeyToAddress` from
   `signer.go`, and move the remaining legacy declarations.
6. Complete all remaining declaration moves: consolidate token DTOs, split
   transaction decoding from transaction DTO declarations, and delete every
   emptied source file listed in section 12.
7. Run formatting and targeted tests after each coherent declaration-move
   batch. This completes the first review/commit unit from section 16.
8. Rename `tokens_client.go`, `transactions_client.go`, and
   `checkpoints_client.go` to their normalized domain filenames without changing
   declarations. This is the second review/commit unit from section 16.
9. Run the complete verification suite.
10. Compare public API, request JSON, endpoints, errors, and protocol fixtures
    with the recorded baseline.

At no point is protocol logic rewritten or duplicated.

## 15. Verification

Required checks available in the current workspace:

```bash
gofmt -l .
go test -count=1 ./...
ENABLE_HTTP_CLIENT_TESTS=1 go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
go mod tidy -diff
go mod verify
git diff --check
```

The repository release process additionally requires:

```bash
gofumpt -l .
golangci-lint run
```

`gofumpt` and `golangci-lint` are not installed in the current workspace. They
must run in CI or another prepared environment before release. Their absence
must be reported rather than represented as a passing local check.

Completion requires:

- Public API surface is exactly equal to the recorded pre-refactor surface.
- No request JSON or endpoint drift.
- No golden-vector or transaction-hash drift.
- No error-behavior drift.
- No new source subpackage.
- No duplicate protocol implementation.
- No stale source filename references in `doc.go`, README, CHANGELOG, or
  documentation changed by this refactor.

## 16. Review and Commit Scope

This design authorizes only package-internal file organization. It does not
authorize behavior or public API changes.

The implementation should be reviewable as two structural changes:

```text
refactor(sdk): separate shared, legacy, and transaction decoding declarations
refactor(sdk): normalize domain source filenames
```

The first change moves declarations and adds `doc.go`, `primitives.go`,
`signature.go`, `legacy.go`, and `transactions_decode.go`. The second performs
the token, transaction, and checkpoint file renames. Keeping pure renames
separate improves Git rename detection and preserves useful blame history.

Repository instructions still require explicit user authorization before
staging or committing. This design does not itself authorize a commit.
