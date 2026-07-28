# Go SDK Native-v2 Internal Package Design

**Status:** Approved in design discussion; awaiting written-spec review  
**Date:** 2026-07-28  
**Target repository:** `1money-protocol-go-sdk`  
**Target release:** `v1.2.0`

## 1. Purpose

The domain-separated native-v2 implementation adds a security-critical
transaction encoding and hashing engine to the Go SDK. The current implementation
places that engine, the public SDK API, REST request assembly, and HTTP submission
in the root `onemoney` package.

This design extracts the pure native-v2 protocol engine into
`internal/nativev2` while retaining every public API in the root package. The
result must make the protocol implementation independently understandable and
testable without changing valid transaction bytes, hashes, endpoints, request
JSON, module path, or public call sites.

## 2. Goals

- Isolate native-v2 domain separation, operation encoding, authorization proofs,
  hashes, signature validation, U256 validation, and multisig derivation.
- Keep all public SDK types and methods in the root `onemoney` package.
- Give the namespace one-step API and offline API one shared v2 pipeline.
- Make all deterministic validation failures occur before network I/O.
- Preserve the Go module path and the public surface required for a compatible
  `v1.2.0` release.
- Retain byte-for-byte conformance with the L1 native-v2 golden vectors.
- Remove the duplicate protocol implementation from the root package after the
  migration.

## 3. Non-goals

- No public `nativev2` or `v2` subpackage.
- No Go module v2 or `/v2` module import path.
- No new L1 endpoint or operation type.
- No multisig-authorized operation submission; multisig account creation and
  address derivation remain in scope.
- No public payload or response field redesign.
- No redesign of the SDK HTTP client, hooks, logger, or transport.
- No automatic fallback between v2 and v1.
- No change to the legacy-v1 signing algorithm or valid request wire format.
- No refactor of pricing or unrelated read APIs.
- No `GetHealth` API work; that API has been removed from the branch.

## 4. Considered Approaches

### 4.1 Extract only the pure protocol core

Create `internal/nativev2` for deterministic protocol behavior and keep a root
package facade for public types, REST assembly, and HTTP submission.

This is the selected approach. It provides a strict dependency boundary without
fragmenting the public SDK.

### 4.2 Move protocol, wire assembly, and prepare logic together

This would make the root package thinner, but the internal package would need to
depend on root payload types or duplicate REST DTOs. A root-to-internal-to-root
dependency would create an import cycle, while duplicating REST DTOs would couple
the protocol engine to transport details.

This approach is rejected.

### 4.3 Keep all files in the root package

Keeping the `native_v2_*.go` prefix avoids conversion types, but leaves protocol
encoding, public APIs, REST JSON, and networking in one package. The security
boundary would remain implicit.

This approach is rejected as the long-term structure.

## 5. Architecture

The dependency direction is one-way:

```text
public payloads / Signer / Memo
                |
                v
root adapters and submit pipeline
                |
                v
       internal/nativev2
                |
                v
       go-ethereum RLP/crypto
```

`internal/nativev2` must never import the root `onemoney` package. The root
package may import `internal/nativev2`.

### 5.1 Internal package responsibilities

`internal/nativev2` owns:

- The frozen native-v2 domain.
- The operation-type registry.
- Canonical payload RLP field order for all supported operations.
- Required-memo and bare-payload encoding.
- Authorization descriptors and proofs.
- Signing-hash and transaction-hash computation.
- Strict signature component validation.
- U256 validation and immutable values.
- Multisig signer validation, proof ordering, and address derivation.

It does not own:

- REST endpoints.
- JSON tags or JSON request bodies.
- `Client`, submission modes, hooks, logging, or HTTP.
- Operation-specific response types.
- Public SDK errors or namespace wording.

### 5.2 Root package responsibilities

The root `onemoney` package owns:

- All public payload, response, signer, memo, option, prepared-transaction, and
  authorized-transaction types.
- The public `Transactions()`, `Tokens()`, and `Accounts()` namespaces.
- Public-to-internal type conversion.
- Immutable REST business-field snapshots.
- Operation-to-v1/v2 endpoint mapping.
- Submission-mode and option validation.
- Signer invocation.
- REST authorization and memo JSON assembly.
- HTTP submission through the existing `Client`.
- Server-response transaction-hash verification.
- Legacy-v1 request assembly.

## 6. Proposed File Layout

### 6.1 Internal package

```text
internal/nativev2/
├── operation.go
├── amount.go
├── memo.go
├── payload.go
├── encoding.go
├── signature.go
├── authorization.go
├── transaction.go
├── multisig.go
├── errors.go
├── conformance_test.go
├── payload_test.go
├── multisig_test.go
└── testdata/
    ├── native-v2-signing-vectors.json
    └── multisig-address-vectors.json
```

Responsibilities by file:

- `operation.go`: domain, `Operation`, values 1 through 14, and static protocol
  properties such as memo policy. It contains no endpoint paths.
- `amount.go`: immutable U256 construction, bounds validation, and canonical
  integer access.
- `memo.go`: internal memo value, canonical three-field RLP representation, and
  memo policy.
- `payload.go`: package-private operation payloads, validating constructors, the
  closed payload interface, and canonical business-field order.
- `encoding.go`: bare payload, memo-wrapped payload, unsigned transaction, and
  signed transaction RLP.
- `signature.go`: strict R/S/V parsing and canonical signature values.
- `authorization.go`: single and multisig descriptors and proof values,
  including canonical multisig proof order.
- `transaction.go`: prepare, signing hash, authorize, and transaction hash.
- `multisig.go`: compressed-key/configuration validation and address derivation.
- `errors.go`: internal errors suitable for `errors.Is` or `errors.As`.

### 6.2 Root package

```text
native_v2_adapter.go
native_v2_wire.go
native_v2_prepare.go
native_v2_requests.go
signer.go
memo.go
submission_mode.go
accounts.go
accounts_types.go
tokens_client.go
tokens_types.go
tokens_models.go
transactions_client.go
transactions_payloads.go
transactions_types.go
status.go
pricing.go
```

- `native_v2_adapter.go` is the only public-payload type switch. It performs
  public-to-internal conversion, selects the operation, validates options and
  mode, and returns endpoint metadata.
- `native_v2_wire.go` creates an immutable snapshot of REST business fields. It
  performs no RLP or hash computation and does not choose the operation.
- `native_v2_prepare.go` exposes the public prepare/authorize facade and delegates
  all protocol computation to the internal package.
- `native_v2_requests.go` owns the one-step pipeline, legacy pipeline, HTTP
  submission, and response-hash verification.
- Public v2 DTOs remain in their existing domain files:
  `accounts_types.go`, `transactions_types.go`, `transactions_payloads.go`, and
  `tokens_types.go`. There is no aggregate `native_v2_types.go`.

## 7. Internal Type Model

### 7.1 Operation and payload

The internal package defines a closed interface:

```go
type Payload interface {
    operation() Operation
    encodeRLP(Memo) ([]byte, error)
    memoPolicy() MemoPolicy
}
```

The methods are unexported so code outside `internal/nativev2` cannot implement
new operations or assign arbitrary operation types.

Each operation has a package-private internal payload with no JSON tags. For
example:

```go
type payment struct {
    chainID   uint64
    nonce     uint64
    recipient common.Address
    value     U256
    token     common.Address
}

func NewPayment(
    chainID uint64,
    nonce uint64,
    recipient common.Address,
    value *big.Int,
    token common.Address,
) (Payload, error)
```

All concrete payload structs and fields are package-private. Operation-specific
exported constructors are the only way the root adapter can obtain a `Payload`.
This makes the interface genuinely closed: the root package can neither
implement a new payload nor construct a partially valid internal payload with a
struct literal.

The internal operation payload owns its canonical RLP field order. The root
adapter owns only conversion and REST wire shape.

The protocol package exposes two computation entry points:

```go
func Prepare(payload Payload, memo Memo) (*PreparedTransaction, error)
func EncodeLegacyPayload(payload Payload) ([]byte, error)
```

`Prepare` applies the payload's memo policy and builds the domain-separated v2
transaction. `EncodeLegacyPayload` returns only the canonical bare business
payload RLP required by legacy signing; it never constructs a v2 descriptor or
hash.

The current public API accepts payload values. Pointer payloads remain rejected
unless a separately approved API change adds pointer support.

### 7.2 U256

An internal U256 is constructed through a validating constructor. It clones its
input and rejects:

- nil.
- Negative values.
- Values larger than 256 bits.

Zero and the maximum U256 value are valid. For currently valid inputs, the RLP
and REST value remain unchanged.

Rejecting nil is an intentional behavior change for the new namespace and
offline APIs. The current unreleased implementation converts nil amounts to zero
through `bigOrZero(nil)`. Repository tests, examples, and call sites contain no
dependency on that behavior, and these new APIs have not been released.

The pre-existing legacy methods (`SendPayment`, `MintToken`, and the other
request-based methods) do not use the new adapter and retain their existing
behavior. A compatibility test must lock that separation. If a valid use case
for nil-as-zero is identified before implementation, the design must be revised
rather than silently retaining the coercion.

### 7.3 Signature

Internal signatures cannot be created with a struct literal. Construction
strictly:

- Parses `0x` hexadecimal R and S without silently accepting invalid bytes.
- Rejects empty, zero, or out-of-range scalars.
- Requires the canonical low-S form.
- Accepts only parity V values 0 or 1.

The public `Signature` remains unchanged. The root facade converts it before
authorization.

### 7.4 Prepared and authorized transactions

The internal prepared value contains only protocol data:

```go
type PreparedTransaction struct {
    operation    Operation
    descriptor   Descriptor
    payloadRLP   []byte
    signingHash  [32]byte
}
```

The internal authorized value additionally contains the proof and final
transaction hash.

Byte-returning methods always return a copy. Internal byte slices and `big.Int`
values cannot be mutated through a returned reference.

The public facade combines the internal value with transport state:

```go
type PreparedTransaction struct {
    core   *nativev2.PreparedTransaction
    wire   wireSnapshot
    memo   Memo
    pathV2 string
}
```

The public type keeps its existing exported methods and signatures.

## 8. Root Adapter and Wire Snapshot

The root adapter has one conversion entry point:

```go
func adaptPayload(
    payload any,
    cfg submitConfig,
) (
    nativev2.Payload,
    wireSnapshot,
    endpointSet,
    error,
)
```

It returns:

- The internal payload used for every protocol computation.
- An immutable REST business-field snapshot.
- The operation's v1 and v2 paths.
- A contextual validation error.

The adapter mapping is not one public Go type to one operation in every case.
The resolved mapping is:

| Public entry point | Public payload | Native operation |
| --- | --- | --- |
| `Transactions().Payment` | `PaymentPayload` | 1 Payment |
| `Tokens().Issue` | `TokenIssuePayload` | 2 TokenIssue |
| `Tokens().Mint` | `TokenMintPayload` | 3 TokenMint |
| `Tokens().GrantAuthority` / `RevokeAuthority` | `TokenAuthorityPayload` | 4 TokenAuthority |
| `Tokens().ManageBlacklist` | `TokenManageListPayload` | 5 TokenBlacklist |
| `Tokens().ManageWhitelist` | `TokenManageListPayload` | 6 TokenWhitelist |
| `Tokens().Pause` / `Unpause` | `PauseTokenPayload` | 7 TokenPause |
| `Tokens().Burn` | `TokenBurnPayload` | 8 TokenBurn |
| `Tokens().Clawback` | `TokenClawbackPayload` | 9 TokenClawback |
| `Tokens().UpdateMetadata` | `UpdateMetadataPayload` | 10 TokenMetadata |
| `Tokens().BridgeAndMint` | `TokenBridgeAndMintPayload` | 11 TokenBridgeAndMint |
| `Tokens().BurnAndBridge` | `TokenBurnAndBridgePayload` | 12 TokenBurnAndBridge |
| `Accounts().CreateMultisig` | `CreateMultiSigPayload` | 13 CreateMultiSig |
| `Transactions().BatchPayment` | `BatchPaymentPayload` | 14 BatchPayment |

Grant/revoke and pause/unpause are many public methods to one native operation;
the namespace method forces the signed action before adaptation. Blacklist and
whitelist are one public payload type to two native operations, so the namespace
method supplies the kind. Generic `PrepareTransaction` continues to require
`WithManageListKind` for `TokenManageListPayload` and errors when it is absent or
invalid.

The wire snapshot deep-copies all caller-owned mutable values:

- Slices and byte slices.
- Multisig public keys.
- Additional metadata.
- Batch operations and optional hashes.
- Bridge parameters.

`big.Int` values are validated through the internal U256 constructor and stored
in the wire snapshot as immutable decimal strings. Optional strings and hashes
are copied as values.

Mutating the original public payload after preparation or authorization must not
change the signing hash, transaction hash, or request JSON.

## 9. Submission Data Flows

### 9.1 Domain-separated v2

```text
public payload
    |
    v
adaptPayload
    +-- internal Payload
    +-- immutable wireSnapshot
    `-- endpointSet
    |
    v
nativev2.Prepare
    +-- canonical payload RLP
    +-- authorization descriptor
    `-- signing hash
    |
    v
Signer.SignHash
    |
    v
nativev2.NewSignature and Authorize
    +-- validated proof
    `-- transaction hash
    |
    v
root REST body assembly
    |
    v
Client.PostMethod
    |
    v
server hash equals internal transaction hash
```

The namespace one-step methods and offline API must use this exact pipeline.
There is no second v2 operation registry, payload encoder, proof builder, or hash
implementation.

### 9.2 Legacy v1

Legacy mode reuses the root adapter and internal operation payload but requests
the canonical bare payload RLP. It then applies the unchanged legacy hash and
top-level signature request shape.

Legacy rules:

- No automatic v2-to-v1 or v1-to-v2 fallback.
- An explicitly supplied memo returns an error.
- A v2-only operation returns an error.
- No v2 transaction hash is calculated or compared.

### 9.3 Memo option tracking

`submitConfig` records whether `WithMemo` was explicitly supplied, independently
of whether the memo value is empty:

```go
type submitConfig struct {
    memo     Memo
    memoSet  bool
    listKind *ManageListKind
}
```

This allows the SDK to reject an explicit memo for BatchPayment or legacy mode
instead of silently discarding caller intent.

## 10. Error Handling

All deterministic errors occur before signing or network I/O where applicable:

- Nil signer.
- Nil submit option.
- Unknown submission mode.
- Unsupported or incorrectly shaped payload.
- Invalid U256.
- Invalid action or manage-list kind.
- Explicit memo in legacy mode.
- Explicit memo for BatchPayment.
- Invalid multisig key or configuration.
- Invalid signature R/S/V.
- Missing endpoint mapping.

Internal errors describe protocol failures. The root package wraps them with
operation context using `%w`, for example:

```text
prepare token mint: value exceeds uint256
authorize payment: signature s is not canonical low-S
submit batch payment: memo is not supported
```

An HTTP success response with a mismatched server hash remains a fail-closed
error and is never retried in another submission mode.

## 11. Public Compatibility

The module remains:

```go
module github.com/1Money-Co/1money-protocol-go-sdk
```

The directory refactor must not remove or change an existing public type,
function, method, field, or signature.

For the `v1.2.0` release:

- `PrivateKeyToAddress` remains as a deprecated compatibility wrapper over
  `NewPrivateKeySigner`.
- Legacy request types and exposed-signing methods receive proper `Deprecated:`
  doc comments.
- Internal protocol types are not re-exported.
- Valid legacy and v2 request bodies remain byte/JSON equivalent to the
  pre-refactor implementation.

The intentional behavior changes are limited to rejecting inputs that were
previously silently ignored, invalid, or panic-producing:

- Nil amounts passed to the new namespace or offline APIs are rejected instead
  of being coerced to zero.
- A nil signer returns an error instead of panicking.
- An explicit memo in legacy mode or on BatchPayment returns an error instead of
  being silently discarded.
- Malformed or non-canonical signatures and out-of-range U256 values fail before
  network I/O.

These changes do not alter the pre-existing request-based legacy methods.

## 12. Existing-file Migration

| Current file | Destination or final responsibility |
| --- | --- |
| `native_v2.go` | Split into internal operation, encoding, authorization, and transaction files |
| `native_v2_encoding.go` | Canonical field order moves into internal payloads |
| `native_v2_multisig.go` | Core moves internal; root retains a public derivation wrapper |
| `native_v2_prepare.go` | Remains root as a facade over internal prepared/authorized values |
| `native_v2_requests.go` | Remains root for pipelines and HTTP |
| `native_v2_wire.go` | Remains root as an immutable wire-snapshot builder |
| Public v2 DTOs | Already live in `accounts_types.go`, `transactions_types.go`, `transactions_payloads.go`, and `tokens_types.go`; no migration required |
| Conformance and payload tests | Move to the internal package |
| API, HTTP, compatibility, and guard tests | Remain in the root package |
| Golden fixtures | Move to `internal/nativev2/testdata` |

After migration, the root package must contain no duplicate domain constant,
operation registry, RLP encoder, authorization proof builder, signing hash, or
transaction hash implementation.

## 13. Testing Strategy

### 13.1 Internal protocol tests

- Payload RLP for all 14 operations.
- Empty and populated memo.
- Single and multisig descriptors and proofs.
- Signing and transaction hashes.
- Full L1 golden-vector conformance.
- BatchPayment trailing optional fields.
- R/S/V validation.
- U256 zero, maximum, nil, negative, and overflow cases.
- Multisig compressed keys, duplicates, weights, threshold, overflow, sorting,
  and address vectors.
- Immutability of returned bytes and values.

### 13.2 Root adapter tests

- Every resolved namespace call maps to exactly one operation.
- Grant/revoke and pause/unpause share their documented native operations.
- `TokenManageListPayload` maps to blacklist or whitelist only through the
  namespace method or explicit `WithManageListKind`.
- Blacklist and whitelist are explicitly disambiguated.
- Grant/revoke and pause/unpause force the correct actions.
- Every operation selects the correct v1 and v2 endpoint.
- REST numbers, addresses, bytes, and multisig keys keep the required JSON shape.
- Mutation of the original payload cannot alter the snapshot or hashes.
- Pointer and value payload behavior remains explicit and tested.

### 13.3 Pipeline tests

- Namespace and offline APIs produce the same signing and transaction hashes.
- Default mode uses `/v2`; explicit legacy mode uses `/v1`.
- No automatic fallback occurs.
- Server hash mismatch fails closed.
- Nil signer returns an error rather than panicking.
- Nil amounts are rejected by the new namespace/offline pipeline.
- Existing request-based legacy methods remain outside the new nil-U256 guard.
- Legacy plus an explicit memo returns an error.
- BatchPayment plus an explicit memo returns an error.
- A v2-only operation in legacy mode returns an error.

### 13.4 Compatibility tests

- The module path is unchanged.
- The pre-refactor public API surface is a subset of the post-refactor surface.
- `PrivateKeyToAddress` remains callable.
- Legacy request types are marked deprecated.
- Internal implementation types are not part of the public API.
- Valid request JSON and endpoint fixtures are unchanged.

## 14. Migration Sequence

1. Record the pre-refactor public API, request JSON, endpoint, and golden-vector
   baselines.
2. Create `internal/nativev2` and migrate operation, payload, encoding, and hash
   logic.
3. Migrate signature, U256, and multisig validation.
4. Make the internal golden and payload tests pass independently.
5. Add the root adapter and immutable wire snapshot.
6. Switch `PrepareTransaction`, `Authorize`, `Submit`, namespace methods, and
   legacy adapters to the internal engine.
7. Remove every duplicate protocol implementation from the root package.
8. Apply the approved compatibility and submit-guard fixes.
9. Compare public API, JSON fixtures, endpoints, and hashes with the baseline.
10. Run the complete verification suite.

No intermediate state is published.

## 15. Verification

Required local checks using tools available in the current workspace:

```bash
gofmt -l .
go test -count=1 ./...
ENABLE_HTTP_CLIENT_TESTS=1 go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
go mod tidy -diff
go mod verify
```

The repository release policy additionally requires:

```bash
gofumpt -l .
golangci-lint run
```

Neither `gofumpt` nor `golangci-lint` is installed in the current workspace.
They must run in CI or another prepared environment before release. Local
verification must report them as unavailable rather than claiming they passed;
their absence does not justify replacing or silently skipping the release gate.

Completion additionally requires:

- All L1 golden vectors match byte-for-byte.
- All valid request JSON fixtures remain unchanged.
- All endpoint mappings remain unchanged.
- The public API surface does not unintentionally shrink.
- `internal/nativev2` does not import the root package.
- No duplicate protocol encoder or hasher remains in the root package.

## 16. Review and Commit Organization

The implementation should be reviewable as two conceptual changes:

1. `refactor(native-v2): extract protocol core into internal package`
2. `fix(sdk): preserve v1.2 compatibility and harden submit guards`

The first change is structural and must preserve valid behavior. The second
contains the explicitly approved compatibility and invalid-input behavior
changes. Repository history may use two commits if the user explicitly
authorizes commits; this design does not itself authorize staging or committing.
