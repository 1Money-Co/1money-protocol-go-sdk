# Go SDK v1.2.0 — Domain-Separated Transaction Submission Upgrade

- Status: accepted (design)
- Date: 2026-07-27
- Repo: `1money-protocol-go-sdk`
- Upstream contract: 1Money L1 issue #1038 "Domain-Separated Transaction
  Signatures". Normative signing spec: l1client
  `docs/specs/native-v2-signing-spec.md`. Rust reference implementation:
  l1client `crates/om-sdk` (`NativeSubmissionMode` model).
- Conformance fixture: l1client
  `docs/specs/fixtures/native-v2-signing-vectors.json`.

## 1. Problem and goals

The Go SDK currently signs every native transaction with the **legacy v1**
scheme — `keccak256(rlp(payload))` with no domain tag or operation-type binding
(`sign.go`) — and POSTs to hardcoded `/v1/...` endpoints. This is exactly the
cross-type signature-replay vulnerability #1038 closes. The L1 has shipped a
parallel domain-separated v2 signing scheme and a versioned `/v2` write surface;
partners must migrate.

Goals:

1. Add domain-separated v2 signing + `/v2` submission, **default**; legacy v1
   only when explicitly selected (mirrors l1client `NativeSubmissionMode`).
2. **Zero source breakage** of existing exported symbols: all current methods
   and types keep their signatures and behavior (marked `Deprecated:`).
3. **Hide all signing detail.** New API takes only a payload + a `Signer`; RLP,
   domain separation, operation-type derivation, memo canonicalization,
   authorization-union assembly, endpoint selection, and hash verification are
   internal.
4. Single-signature one-call submission for every operation; multisig **account
   creation** (single-signed). Multisig-*authorized* operation submission is
   deferred (see §11).
5. Fill missing REST coverage (§9).
6. Ship as `v1.2.0` (backward-compatible minor; module path unchanged) with a
   partner-facing changelog.

Non-goals: changing read endpoints (they stay on `/v1`), Ethereum/EIP-712
signing, or the WebSocket surface. No multisig-authorized operation submission
this release.

## 2. Module versioning (minor release, no path change)

The change is backward-compatible — purely additive (new symbols) plus
deprecation doc-comments; no existing exported signature or behavior changes.
Under Go's [Semantic Import Versioning](https://go.dev/ref/mod#major-version-suffixes)
this is a **minor** release, so the module path is unchanged (no `/v2` suffix):

```
module github.com/1Money-Co/1money-protocol-go-sdk
```

Tag `v1.2.0`. Consumers upgrade with no import-path change and no code changes;
existing code keeps its v1 behavior. The `V2` in
`SubmissionModeDomainSeparatedV2` and the `/v2` REST paths denote the L1
REST/signing **protocol** version, unrelated to the Go module version. A future
`v2.0.0` (with a `/v2` module path) is warranted only when the deprecated legacy
API is actually removed, or an existing signature/behavior changes
incompatibly.

## 3. Architecture overview

New files (existing files untouched except `sign.go` gains no changes; deprecations are doc-comment only):

| File | Responsibility |
|---|---|
| `submission_mode.go` | `SubmissionMode` enum + `WithSubmissionMode` / `WithLegacyV1` client options; mode field on `Client` |
| `signer.go` | `Signer` interface + `PrivateKeySigner` implementation |
| `memo.go` | `Memo` type, empty-memo default, `WithMemo` submit option, `SubmitOption` type |
| `native_v2.go` | **Correctness engine** (internal): domain constant, operation-type registry, descriptor + `WithMemo` + payload RLP encoding, signing hash, signed-tx assembly, transaction hash |
| `native_v2_requests.go` | v2 request DTOs (`authorization` union, `RequiredMemo`) and per-operation request assembly |
| `api_transactions.go` | `client.Transactions()` sub-API |
| `api_tokens.go` | `client.Tokens()` sub-API |
| `api_accounts.go` | `client.Accounts()` sub-API (CreateMultisig) |
| `gap_*.go` | New endpoint coverage (§9): batch payment, clawback, pricing reads, status/health |
| `native_v2_conformance_test.go` + `testdata/native-v2-signing-vectors.json` | Byte-for-byte conformance against the L1 golden vectors |

The single existing `Client` struct gains one field (`submissionMode
SubmissionMode`, zero value = v2). The three sub-API accessors return thin
value types bound to `*Client` (e.g. `type TransactionsAPI struct{ c *Client }`).

## 4. Client mode selector

```go
type SubmissionMode int

const (
    // Zero value: domain-separated v2 signing over /v2 (default).
    SubmissionModeDomainSeparatedV2 SubmissionMode = iota
    // Legacy signing over /v1; explicit opt-in only.
    SubmissionModeLegacyV1
)

func WithSubmissionMode(m SubmissionMode) ClientOption
func WithLegacyV1() ClientOption // sugar for WithSubmissionMode(SubmissionModeLegacyV1)
```

`NewClient()` and all existing constructors default to v2 (zero value). A
submission never auto-falls-back across modes (a failed/ambiguous v2 result is
never retried as v1 — that could double-spend a nonce).

## 5. Signer abstraction

```go
type Signer interface {
    // SignHash signs a 32-byte digest, returning r, s and v (v ∈ {0,1}).
    SignHash(hash []byte) (Signature, error)
    // CompressedPublicKey returns the 33-byte SEC1-compressed public key.
    CompressedPublicKey() []byte
    // Address returns the signer's 20-byte account address.
    Address() common.Address
}

func NewPrivateKeySigner(hexKey string) (Signer, error)
```

`PrivateKeySigner` wraps a secp256k1 key via go-ethereum `crypto`. The
interface is sufficient for both single-sig (used now) and multisig entry
construction (deferred), so KMS/HSM signers can be added later with no API
change. `Signature` reuses the existing `sign.go` type.

## 6. Namespace API

All submit methods share the shape `(ctx, <Payload>, signer, ...SubmitOption)`
and return the operation's response (containing the verified transaction hash).
Internally each: derives its `NativeOperationType`, builds the request per the
client's mode, and (v2) verifies the returned hash.

- `client.Transactions()`:
  - `Payment(ctx, PaymentPayload, signer, ...SubmitOption) (*PaymentResponse, error)`
  - `BatchPayment(ctx, BatchPaymentPayload, signer, ...SubmitOption) (*PaymentResponse, error)` — new (§9)
- `client.Tokens()`:
  - `Issue` (returns hash + minted token address), `Mint`, `Burn`,
    `BridgeAndMint`, `BurnAndBridge`, `GrantAuthority`, `RevokeAuthority`,
    `Clawback` (new, §9), `ManageBlacklist`, `ManageWhitelist`, `Pause`,
    `Unpause`, `UpdateMetadata`
- `client.Accounts()`:
  - `CreateMultisig(ctx, CreateMultiSigPayload, signer, ...SubmitOption) (*CreateMultisigResponse, error)` — v2-only (single-signed creation tx; payload carries the signer set + threshold)

Reads stay on the existing flat `Client` methods (already covered) plus the new
read helpers in §9. Reads are never namespaced.

New payload/response types needed (currently absent): `BatchPaymentPayload`,
`TokenClawbackPayload`, `CreateMultiSigPayload` (+ `MultiSigSigner`),
`CreateMultisigResponse`. Field order and JSON tags follow the L1 canonical
layout (native-v2-signing-spec §4.2).

## 7. Domain-separated signing core (correctness engine)

> **Superseded for BatchPayment (2026-08-10):** `max_fee` was removed from the
> signed payload and BatchPayment became memo-bearing. See
> `docs/superpowers/specs/2026-08-10-batch-payment-v2-rebaseline-design.md`.
> The claim below that BatchPayment's `payload_rlp` skips the memo wrapper is
> stale; BatchPayment now uses `WithMemo<BatchPaymentPayload>` for both the
> default empty memo and a caller-supplied one, exactly like the other
> operations.

`native_v2.go` implements the frozen spec exactly:

- `NATIVE_TX_DOMAIN_V2 = "1money.native.transaction.v2"` (28 bytes).
- Static `NativeOperationType` registry (u16, frozen): Payment=1, TokenIssue=2,
  TokenMint=3, TokenAuthority=4, TokenBlacklist=5, TokenWhitelist=6,
  TokenPause=7, TokenBurn=8, TokenClawback=9, TokenMetadata=10,
  TokenBridgeAndMint=11, TokenBurnAndBridge=12, CreateMultiSig=13,
  BatchPayment=14. Each Go payload type maps to exactly one op type; the user
  never supplies it.
- `payload_rlp`: for every memo-capable op (all except BatchPayment) the
  canonical form is `WithMemo<Payload>` = `rlp([ payload_as_list, memo_as_list ])`,
  `memo_as_list = [ memo_type, memo_format, memo_data ]` (three UTF-8 strings,
  always present; empty strings for "no memo"). BatchPayment uses
  `BatchPaymentPayload` directly. `payload_rlp` is embedded into the outer
  transaction list as **one opaque byte-string element** (RLP-string-wrapped a
  second time), never spliced as a sub-list.
- `unsigned = rlp([ DOMAIN, op_type, auth_descriptor, payload_rlp ])`;
  `signing_hash = keccak256(unsigned)`.
- `auth_descriptor`: `[0]` single-sig; `[1, account]` multisig (20-byte
  account). This release always emits `[0]` (single-sig) for operations;
  CreateMultisig is also single-signed.
- `signed = rlp([ DOMAIN, op_type, auth_descriptor, payload_rlp, proof ])`;
  `proof(single) = [r, s, v]`; `transaction_hash = keccak256(signed)` (append +
  re-encode, never byte-concatenation).
- Special encoding: `MultiSigSigner.public_key` inside `CreateMultiSigPayload`
  is a 33-element RLP list of single bytes (spec §3), not a 33-byte string.
  Integers are minimal big-endian (zero = empty `0x80`); addresses are fixed
  20-byte strings — both native to go-ethereum `rlp`.

### Correctness gate

`native_v2_conformance_test.go` loads the L1 golden vectors
(`testdata/native-v2-signing-vectors.json`, copied from l1client). For every
vector it recomputes `unsigned_transaction_rlp`, `signing_hash`,
`signed_transaction_rlp`, and `transaction_hash` from the vector's recorded
`operation_type`, `authorization_kind`, `multisig_account`, `payload_rlp`, and
`authorization_proof`, and asserts byte-equality. This validates the **outer
hasher** independently of the Go payload encoders.

To validate the **payload encoders** (that a Go `PaymentPayload{…}` produces the
same `payload_rlp`), the implementation reads the Rust fixture generator
(l1client
`crates/types/om-primitives-types/examples/native_domain_separated_payload_fixtures.rs`)
to obtain the exact canonical input values behind each vector, replicates those
inputs in Go, and asserts the encoded `payload_rlp` matches the vector. This
closes the loop: Go signing is byte-identical to Rust/Python end to end.

## 8. Memo and transaction-hash verification

> **Superseded for BatchPayment (2026-08-10):** the claim below that
> BatchPayment has no memo is stale. BatchPayment now carries a required
> `memo` object exactly like the other v2 operations. See
> `docs/superpowers/specs/2026-08-10-batch-payment-v2-rebaseline-design.md`.

- `type Memo struct { Type, Format, Data string }`. Default (no option) = empty
  memo (three empty strings), the canonical "no business memo" form. `WithMemo(m
  Memo) SubmitOption` overrides it. The common no-memo path carries zero
  additional burden.
- v2 requests carry the memo as a required object `{ "type","format","data" }`
  (`RequiredMemo`); BatchPayment has no memo.
- After a v2 POST, the SDK recomputes `transaction_hash` locally from the
  complete signed transaction and compares it to the server's returned `hash`.
  On mismatch it returns an error and does **not** retry — fail-closed, matching
  l1client `submit_v2`.

## 9. Endpoint gap-filling (approved scope)

Implement:
- Writes (into the namespace API): `batch_payment`, `clawback`,
  `CreateMultisig` (`/v2/accounts/multisig`).
- Reads (flat helpers): `GetPricingPlans` (`/v1/pricing/plans`), `GetPricingPlan`
  (`/v1/pricing/plans/{id}`), `GetNativeWriteStatus` (`/api/status`), `GetHealth`
  (`/api/health`).

Skip: all `/v1/governances/*` (epoch reads and the validator/relayer-facing
`certificate` / `auto_countersign` writes) are intentionally not exposed in the
Go SDK — governance is not part of the partner-facing surface.

`GetNativeWriteStatus` is intentionally included: it exposes the node's
native-write mode, letting integrators confirm a target network is `Dual`/`V2`
before switching modes.

## 10. v1 compatibility

- Every existing exported symbol (`SignMessage`, `EncodePayload`, `HashMessage`,
  the `*Request` types, `SendPayment`, `IssueToken`, `MintToken`,
  `BridgeAndMintToken`, `BurnToken`, `BurnAndBridgeToken`, `SetTokenBlacklist`,
  `SetTokenWhitelist`, `PauseToken`, `GrantTokenAuthority`,
  `UpdateTokenMetadata`, all read methods) is preserved unchanged and marked
  `Deprecated:` in doc comments, pointing to the namespace equivalent. Old
  partner code compiles and behaves identically (still v1).
- The namespace API is mode-aware: under `WithLegacyV1()` it signs with the
  legacy scheme and POSTs to `/v1` with a top-level `signature`; under the
  default it uses domain-separated v2. This gives new code a single clean API
  for both eras.

## 11. Deferred: multisig-authorized operations

Submitting an operation authorized by an existing multisig account requires
offline N-party signature collection over the shared domain-separated hash, then
a sorted `multisig_secp256k1` authorization union. The `Signer` interface and
the internal authorization builder are shaped to accommodate it, but the
public collection/submit flow is out of scope for v1.2.0 and tracked for a later
release. `CreateMultisig` (creating the account, single-signed) **is** in scope.

## 12. Testing strategy

- Golden-vector conformance (§7) — the correctness backbone.
- `net/http/httptest` mock server per namespace method asserting: correct
  endpoint (`/v2` default, `/v1` under `WithLegacyV1`), request body shape
  (`authorization` union, `memo` object, business fields), and the fail-closed
  hash-verification path (server returns a wrong hash → method errors).
- Mode-matrix tests: same call under v2 and v1 hits the right path with the
  right signature scheme.
- All existing tests remain green.
- Gates (local, since `golangci-lint`/`gofumpt` are absent): `go build ./...`,
  `go vet ./...`, `gofmt -l` clean, `go test ./...`.

## 13. Changelog (written at completion)

`CHANGELOG.md`, Keep-a-Changelog style, `## [1.2.0]` covering:
- Added: v2 domain-separated signing (default); `Signer` interface +
  `PrivateKeySigner`; namespace API; `SubmissionMode` + `WithLegacyV1`;
  `WithMemo`; new endpoints (§9); multisig account creation.
- Notes (not breaking): the new namespace methods default to domain-separated v2
  and POST to `/v2`; their request bodies use the `authorization` union and a
  required `memo`, and signature `v` is `0/1`. Existing methods are untouched,
  and the Go module path is unchanged (no `/v2` suffix) — this is a minor
  release, not a major one.
- Deprecated: legacy exposed-signing methods and `*Request` types (still work).
- Migration guide: no import-path change; side-by-side legacy→namespace examples
  for the common operations.

## 14. Success criteria (goal)

- All namespace submit methods work in both modes; v2 is default.
- Go domain-separated signing is byte-identical to the L1 golden vectors
  (all vectors pass) and to the Rust payload encoders (fixture inputs match).
- Existing exported API unchanged; old code compiles with no import-path change
  and still submits v1.
- Gap endpoints (§9) implemented.
- `go build`, `go vet`, `gofmt -l` (clean), `go test ./...` all pass.
- `CHANGELOG.md` written.
