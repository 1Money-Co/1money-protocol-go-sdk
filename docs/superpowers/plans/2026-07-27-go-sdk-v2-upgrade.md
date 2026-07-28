# Go SDK Domain-Separated Upgrade (v1.2.0) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Upgrade the 1Money Go SDK to domain-separated v2 transaction submission (default), hiding all signing detail behind a namespaced API + `Signer` interface, while keeping the existing v1 API source-compatible, and fill missing REST coverage — shipped as a backward-compatible `v1.2.0` (module path unchanged).

**Architecture:** New files add a client-level `SubmissionMode` (default v2), a `Signer` abstraction, an internal domain-separated signing core validated byte-for-byte against the L1 golden vectors, and three resource sub-APIs (`Transactions()/Tokens()/Accounts()`) whose methods take `(ctx, payload, signer, ...opt)` and internally sign+submit per mode. Existing exported symbols are untouched (deprecated in doc comments).

**Tech Stack:** Go 1.24+, `github.com/ethereum/go-ethereum` (`crypto`, `rlp`, `common`), stdlib `net/http`/`net/http/httptest`/`encoding/json`.

## Global Constraints

- Module path stays `github.com/1Money-Co/1money-protocol-go-sdk` (backward-compatible minor release v1.2.0; no `/v2` suffix). Package name stays `onemoney`.
- Zero source breakage of existing exported symbols; deprecations are doc-comment only.
- Domain constant: `NATIVE_TX_DOMAIN_V2 = "1money.native.transaction.v2"` (28 bytes).
- `NativeOperationType` (u16, frozen): Payment=1, TokenIssue=2, TokenMint=3, TokenAuthority=4, TokenBlacklist=5, TokenWhitelist=6, TokenPause=7, TokenBurn=8, TokenClawback=9, TokenMetadata=10, TokenBridgeAndMint=11, TokenBurnAndBridge=12, CreateMultiSig=13, BatchPayment=14.
- Signature `v` ∈ {0,1}. Integers RLP-encoded minimal big-endian (zero = `0x80`). Addresses = fixed 20-byte strings.
- Correctness backbone: `testdata/native-v2-signing-vectors.json` (copied from l1client `docs/specs/fixtures/native-v2-signing-vectors.json`); every vector must pass.
- Never auto-fall-back between modes. v2 verifies server hash, fail-closed.
- Gates (local; golangci-lint/gofumpt absent): `go build ./...`, `go vet ./...`, `gofmt -l .` (must be empty), `go test ./...`.
- No git commits unless the user explicitly asks (repo rule). "Commit" steps below are recorded but SKIPPED unless the user opts in; run the gate instead.

---

### Task 1: Client mode field + `SubmissionMode` (module path unchanged)

**Files:**
- Create: `submission_mode.go`
- Modify: `client.go` (add `submissionMode` field to `Client`, default zero = v2)
- Test: `submission_mode_test.go`

**Interfaces:**
- Produces: `SubmissionMode` (int enum: `SubmissionModeDomainSeparatedV2=0`, `SubmissionModeLegacyV1=1`); `func WithSubmissionMode(SubmissionMode) ClientOption`; `func WithLegacyV1() ClientOption`; `(*Client).submissionMode` field; unexported `(*Client).mode() SubmissionMode`.

- [ ] **Step 1:** Leave `go.mod` module path unchanged (`github.com/1Money-Co/1money-protocol-go-sdk`) — this is a backward-compatible minor release. Run `grep -rn "1money-protocol-go-sdk" *.go` to confirm no internal file imports the module path (single package `onemoney`; expect no matches).
- [ ] **Step 2:** Write `submission_mode.go`:

```go
package onemoney

// SubmissionMode selects the native transaction signing scheme and REST write
// surface. The zero value is domain-separated v2 (the default and recommended
// mode). A submission never falls back across modes.
type SubmissionMode int

const (
	// SubmissionModeDomainSeparatedV2 signs with domain separation and POSTs to
	// /v2. Default.
	SubmissionModeDomainSeparatedV2 SubmissionMode = iota
	// SubmissionModeLegacyV1 signs with the legacy scheme and POSTs to /v1.
	// Explicit opt-in only, for compatibility during the migration window.
	SubmissionModeLegacyV1
)

// WithSubmissionMode sets the native submission mode on the client.
func WithSubmissionMode(m SubmissionMode) ClientOption {
	return func(c *Client) { c.submissionMode = m }
}

// WithLegacyV1 is sugar for WithSubmissionMode(SubmissionModeLegacyV1).
func WithLegacyV1() ClientOption {
	return func(c *Client) { c.submissionMode = SubmissionModeLegacyV1 }
}

func (c *Client) mode() SubmissionMode { return c.submissionMode }
```

- [ ] **Step 3:** In `client.go`, add field `submissionMode SubmissionMode` to `Client` struct (zero value = v2; no constructor change needed).
- [ ] **Step 4:** Write `submission_mode_test.go`: assert `NewClientWithCustomUrl(url).mode() == SubmissionModeDomainSeparatedV2` (default) and `NewClientWithCustomUrl(url, WithLegacyV1()).mode() == SubmissionModeLegacyV1`.
- [ ] **Step 5:** Gate: `go build ./... && go vet ./... && go test -run TestSubmissionMode ./...`.

---

### Task 2: `Signer` abstraction

**Files:**
- Create: `signer.go`
- Test: `signer_test.go`

**Interfaces:**
- Produces: `Signer` interface { `SignHash(hash []byte) (Signature, error)`; `CompressedPublicKey() []byte`; `Address() common.Address` }; `func NewPrivateKeySigner(hexKey string) (Signer, error)`.

- [ ] **Step 1:** Write `signer.go`:

```go
package onemoney

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// Signer abstracts transaction signing so the SDK never handles a raw private
// key directly. PrivateKeySigner is the built-in implementation; KMS/HSM
// signers can implement the same interface without any API change.
type Signer interface {
	// SignHash signs a 32-byte digest and returns r, s and v (v is the 0/1
	// recovery id).
	SignHash(hash []byte) (Signature, error)
	// CompressedPublicKey returns the 33-byte SEC1-compressed public key.
	CompressedPublicKey() []byte
	// Address returns the signer's 20-byte account address.
	Address() common.Address
}

type privateKeySigner struct {
	key *ecdsaPrivateKey // alias below
}
```

(Implement with `crypto.HexToECDSA`; `SignHash` calls `crypto.Sign(hash, key)` and maps to `Signature{R,S,V}` exactly as `sign.go`'s `SignMessage` does; `CompressedPublicKey` = `crypto.CompressPubkey(&key.PublicKey)`; `Address` = `crypto.PubkeyToAddress(key.PublicKey)`. Use `crypto.HexToECDSA` with `strings.TrimPrefix(hexKey,"0x")`.)

- [ ] **Step 2:** Write `signer_test.go`: from a known hex key assert `Address()` equals the expected address, `len(CompressedPublicKey())==33`, and `SignHash(keccak256("x"))` returns `V∈{0,1}` and a signature that `crypto.SigToPub` recovers back to `Address()`.
- [ ] **Step 3:** Gate: `go test -run TestSigner ./...`.

---

### Task 3: Domain-separated signing core

**Files:**
- Create: `native_v2.go`
- Test: (validated by Task 4 conformance; add unit tests for descriptor/memo here)

**Interfaces:**
- Produces:
  - `type Memo struct { Type, Format, Data string }` (moved to `memo.go` in Task 5; here assume it exists — define a temporary local if needed, then dedupe). **Correction:** define `Memo` here in `native_v2.go` is wrong; create `memo.go` FIRST. Reorder: do Task 5's `memo.go` Memo type before this. (See Task 5.)
  - `nativeOperationType` (uint16 constants per Global Constraints).
  - `func encodeWithMemo(payloadList []interface{}, memo Memo) ([]byte, error)` → `rlp([ payloadList, [memo.Type, memo.Format, memo.Data] ])`.
  - `func singleDescriptor() []interface{}` → `[]interface{}{uint64(0)}`.
  - `func signingHashV2(opType uint16, descriptor []interface{}, payloadRLP []byte) ([]byte, error)`.
  - `func signedTxRLP(opType uint16, descriptor []interface{}, payloadRLP []byte, proof []interface{}) ([]byte, error)` and `func txHashV2(...) ([]byte, error)`.
  - `func singleProof(sig Signature) ([]interface{}, error)` → `[r,s,v]` as minimal big.Ints/uint.

- [ ] **Step 1:** Write the RLP helpers in `native_v2.go`. Core encoding (the load-bearing detail):

```go
package onemoney

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

const nativeTxDomainV2 = "1money.native.transaction.v2"

// encodeList RLP-encodes a heterogeneous list. []byte elements encode as RLP
// strings; nested []interface{} as lists; uint64/*big.Int as minimal integers;
// common.Address as a 20-byte string.
func encodeList(items []interface{}) ([]byte, error) {
	return rlp.EncodeToBytes(items)
}

func memoList(m Memo) []interface{} {
	return []interface{}{[]byte(m.Type), []byte(m.Format), []byte(m.Data)}
}

// encodeWithMemo builds payload_rlp = rlp([ payloadList, memoList ]).
func encodeWithMemo(payloadList []interface{}, m Memo) ([]byte, error) {
	return encodeList([]interface{}{payloadList, memoList(m)})
}

func singleDescriptor() []interface{} { return []interface{}{uint64(0)} }

// signingHashV2 = keccak256(rlp([ DOMAIN, opType, descriptor, payloadRLP ])).
// payloadRLP is embedded as one opaque byte-string element (passed as []byte).
func signingHashV2(opType uint16, descriptor []interface{}, payloadRLP []byte) ([]byte, error) {
	unsigned, err := encodeList([]interface{}{
		[]byte(nativeTxDomainV2), uint64(opType), descriptor, payloadRLP,
	})
	if err != nil {
		return nil, err
	}
	return crypto.Keccak256(unsigned), nil
}
```

Notes carried into implementation:
- `common.Address` inside a list encodes as a 20-byte string (go-ethereum rlp special-cases byte arrays). Verify via conformance.
- Value fields use `*big.Int` (nil → treat as 0; encode `big.NewInt(0)`), never string, in the RLP path.
- Signature `r,s` from `Signature{R,S string}` (0x-hex 32-byte) → `new(big.Int).SetBytes(common.FromHex(R))`; `v` → `uint64`.
- CreateMultiSig pubkey quirk handled in Task 5's payload encoder (list-of-bytes, not a string).

- [ ] **Step 2:** Add `signedTxRLP` (append `proof` as 5th element and re-encode the whole list) and `txHashV2 = keccak256(signedTxRLP)`; `singleProof(sig)` = `[]interface{}{ rBig, sBig, uint64(v) }`.
- [ ] **Step 3:** Unit test `native_v2_test.go`: assert `singleDescriptor` encodes to `0xc180`; assert an empty memo list encodes to `0xc3808080`.
- [ ] **Step 4:** Gate: `go test -run TestNativeV2 ./...`.

---

### Task 4: Golden-vector conformance (correctness gate)

**Files:**
- Create: `testdata/native-v2-signing-vectors.json` (copy from l1client)
- Create: `native_v2_conformance_test.go`

**Interfaces:**
- Consumes: Task 3 outer hasher (`signingHashV2`, `signedTxRLP`, `txHashV2`).

- [ ] **Step 1:** `cp /Users/nsh/workspace/1money/layer1/l1client/docs/specs/fixtures/native-v2-signing-vectors.json testdata/`.
- [ ] **Step 2:** Write `native_v2_conformance_test.go`: unmarshal the fixture (map or list of vectors, each with `operation_type`, `authorization_kind`, `multisig_account`, `payload_rlp`, `authorization_proof`, `unsigned_transaction_rlp`, `signing_hash`, `signed_transaction_rlp`, `transaction_hash`). For each vector, build the descriptor from `authorization_kind`/`multisig_account`, take `payload_rlp` bytes as-is, build the proof from `authorization_proof`, and assert Go-computed `unsigned`, `signing_hash`, `signed`, `transaction_hash` equal the recorded hex (byte-for-byte). Handle both single (`kind 0`) and multi (`kind 1`, descriptor `[1, account]`, proof list-of-entries) vectors.
- [ ] **Step 3:** Run `go test -run TestNativeV2Conformance ./... -v`. Expected: ALL vectors PASS. If any fail, fix the encoder in Task 3 until green (this is the definition of correctness).
- [ ] **Step 4:** Gate: `go test ./...`.

---

### Task 5: New payload/response types + `memo.go`

**Files:**
- Create: `memo.go`
- Create: `native_v2_types.go` (new payloads/responses)
- Test: `native_v2_types_test.go`

**Interfaces:**
- Produces: `type Memo struct{Type, Format, Data string}` + `func EmptyMemo() Memo`; `type SubmitOption func(*submitConfig)` + `func WithMemo(Memo) SubmitOption`; new payloads `BatchPaymentPayload`, `TokenClawbackPayload`, `CreateMultiSigPayload` (+ `MultiSigSigner{PublicKey []byte; Weight uint8}`); responses `CreateMultisigResponse{Hash, Account}`. Reuse existing `PaymentPayload`, `Token*Payload` from `transactions_types.go`/`tokens_types.go`.

- [ ] **Step 1:** Write `memo.go` (Memo type, EmptyMemo, SubmitOption, submitConfig{memo Memo}, WithMemo, and an internal `resolveSubmit(opts) submitConfig` defaulting to EmptyMemo).
- [ ] **Step 2:** Write `native_v2_types.go` with the three new payloads (field order per native-v2-signing-spec §4.2: BatchPayment = chain_id, nonce, token, operations[]{recipient, amount}, max_fee, created_at, optional operations_hash, optional batch_id; TokenClawback = chain_id, nonce, token, from, recipient, value; CreateMultiSig = chain_id, nonce, signers[]{public_key(33B), weight}, threshold). Add JSON tags matching the L1 v2 DTOs (verify exact tags against l1client `crates/types/om-rest-types/src/requests/{batch_payment,clawback,create_multisig}.rs` during impl).
- [ ] **Step 3:** Write a per-payload `rlpList()` method returning `[]interface{}` in canonical field order for each payload used in v2 (Payment, all Token ops, BatchPayment, CreateMultiSig). CreateMultiSig signer public_key encodes as a list-of-bytes: `func pubkeyAsByteList(pk []byte) []interface{}`.
- [ ] **Step 4:** Test: build the fixture-generator inputs (read l1client `.../examples/native_domain_separated_payload_fixtures.rs` for exact values) for at least Payment, TokenBridgeAndMint, TokenBurnAndBridge, CreateMultiSig; assert `encodeWithMemo(payload.rlpList(), memo)` (or bare list for BatchPayment) equals the corresponding vector's `payload_rlp`. This validates the Go encoders end-to-end.
- [ ] **Step 5:** Gate: `go test -run 'TestNativeV2Types|TestPayloadRLP' ./...`.

---

### Task 6: v2 request DTOs + operation-type mapping + submit core

**Files:**
- Create: `native_v2_requests.go`
- Test: `native_v2_requests_test.go`

**Interfaces:**
- Produces:
  - `type nativeAuthorization struct` marshaling to `{"type":"single_secp256k1","signature":{r,s,v}}` (and a multisig shape, unused this release but defined).
  - `type requiredMemo struct{ Type, Format, Data string }` with json tags `type/format/data`.
  - `func (c *Client) submitNativeV2(ctx, opType uint16, payloadList []interface{}, memo Memo, jsonBody map[string]interface{}, path string, signer Signer, out interface{}) error` — builds descriptor+payloadRLP, signs, injects `authorization`+`memo` into `jsonBody`, POSTs to `path`, then verifies returned `hash` against local `txHashV2` (fail-closed).
  - `func (c *Client) submitLegacyV1(...)` — signs bare payload (`HashMessage`), injects top-level `signature`, POSTs to the `/v1` path.
  - `func opPath(mode SubmissionMode, v1, v2 string) string`.

- [ ] **Step 1:** Write `nativeAuthorization` + `requiredMemo` marshaling; test JSON output equals `{"type":"single_secp256k1","signature":{"r":"0x..","s":"0x..","v":0}}`.
- [ ] **Step 2:** Write `submitNativeV2`: assemble body via json (payload fields + `memo` + `authorization`), POST, unmarshal into `out`, then extract `out`'s hash (via a small `hashable` interface `TxHash() string`) and compare to `hex(txHashV2)`; on mismatch return `fmt.Errorf("transaction hash mismatch: server=%s local=%s", ...)`.
- [ ] **Step 3:** Write `submitLegacyV1` reusing existing `SignMessage` + existing legacy `*Request` assembly per operation (thin adapter; may call existing methods).
- [ ] **Step 4:** Test with `httptest`: a fake server returning the locally-correct hash → success; returning a wrong hash → error. Assert the POST path is `/v2/...` by default and body contains `authorization.type == "single_secp256k1"` and a `memo` object.
- [ ] **Step 5:** Gate: `go test -run TestNativeV2Requests ./...`.

---

### Task 7: `Transactions()` namespace

**Files:**
- Create: `api_transactions.go`
- Test: `api_transactions_test.go`

**Interfaces:**
- Produces: `func (c *Client) Transactions() TransactionsAPI`; `TransactionsAPI` with `Payment(ctx, PaymentPayload, Signer, ...SubmitOption) (*PaymentResponse, error)` and `BatchPayment(ctx, BatchPaymentPayload, Signer, ...SubmitOption) (*PaymentResponse, error)`.

- [ ] **Step 1:** Write `TransactionsAPI{c *Client}` + accessor. `Payment` derives opType=1, builds `payloadList` from `PaymentPayload.rlpList()`, JSON body from payload fields, path via `opPath(c.mode(), "/v1/transactions/payment", "/v2/transactions/payment")`, routes through `submitNativeV2`/`submitLegacyV1`. `BatchPayment` opType=14, no memo, paths `/v{1,2}/transactions/batch_payment`.
- [ ] **Step 2:** Test with `httptest`: `Payment` in v2 posts to `/v2/transactions/payment` with `authorization`+`memo`; in `WithLegacyV1` posts to `/v1/transactions/payment` with top-level `signature`; hash mismatch → error.
- [ ] **Step 3:** Gate: `go test -run TestTransactionsAPI ./...`.

---

### Task 8: `Tokens()` namespace

**Files:**
- Create: `api_tokens.go`
- Test: `api_tokens_test.go`

**Interfaces:**
- Produces: `func (c *Client) Tokens() TokensAPI`; methods `Issue` (→ `*IssueTokenResponse`), `Mint`, `Burn`, `BridgeAndMint`, `BurnAndBridge`, `GrantAuthority`, `RevokeAuthority`, `Clawback`, `ManageBlacklist`, `ManageWhitelist`, `Pause`, `Unpause`, `UpdateMetadata` — each `(ctx, <Payload>, Signer, ...SubmitOption) (*<Response>, error)`.

- [ ] **Step 1:** Write `TokensAPI{c *Client}` + accessor + all methods. Op-type map: Issue=2, Mint=3, Authority=4 (GrantAuthority sets Action=Grant, RevokeAuthority sets Action=Revoke; same endpoint `grant_authority` / payload `TokenAuthorityPayload`), Blacklist=5, Whitelist=6, Pause=7 (Pause sets Action=Pause, Unpause sets Action=Unpause; endpoint `pause`), Burn=8, Clawback=9 (endpoint `clawback`, §9), Metadata=10, BridgeAndMint=11, BurnAndBridge=12. Each routes via `submitNativeV2`/`submitLegacyV1` with the right `/v{1,2}` paths (from `tokens_client.go` constants + `/v2` equivalents).
- [ ] **Step 2:** Tests with `httptest` for a representative subset (Mint, BridgeAndMint, Pause/Unpause action mapping, GrantAuthority/RevokeAuthority action mapping, Clawback) covering v2 path+body and v1 fallback.
- [ ] **Step 3:** Gate: `go test -run TestTokensAPI ./...`.

---

### Task 9: `Accounts()` namespace (CreateMultisig)

**Files:**
- Create: `api_accounts.go`
- Test: `api_accounts_test.go`

**Interfaces:**
- Produces: `func (c *Client) Accounts() AccountsAPI`; `CreateMultisig(ctx, CreateMultiSigPayload, Signer, ...SubmitOption) (*CreateMultisigResponse, error)` — opType=13, v2-only path `/v2/accounts/multisig`, single-signed. Errors clearly if `mode()==LegacyV1` (no v1 form).

- [ ] **Step 1:** Write `AccountsAPI` + `CreateMultisig` (single-sig authorization; payload carries signer set + threshold; pubkey list-of-bytes encoding from Task 5). Return `{Hash, Account}` from response.
- [ ] **Step 2:** Test with `httptest`: posts to `/v2/accounts/multisig`, body has `authorization`+`memo`+signers; `WithLegacyV1` → returns a clear "multisig account creation requires v2" error.
- [ ] **Step 3:** Gate: `go test -run TestAccountsAPI ./...`.

---

### Task 10: Gap read endpoints

**Files:**
- Create: `pricing.go`, `status.go` (governance intentionally NOT exposed)
- Test: `pricing_test.go`, `status_test.go`

**Interfaces:**
- Produces: `GetPricingPlans(ctx) ([]PricingPlan, error)` (`/v1/pricing/plans`), `GetPricingPlan(ctx, id string) (*PricingPlan, error)` (`/v1/pricing/plans/{id}`), `GetNativeWriteStatus(ctx) (*NativeWriteStatusResponse, error)` (`/api/status`), `GetHealth(ctx) (*HealthResponse, error)` (`/api/health`). Response field shapes verified against l1client handlers during impl.

- [ ] **Step 1:** Write the three read files with response structs (verify JSON shapes against l1client `crates/api/om-api-rest/src/api/{pricing,status}.rs` and `status.rs::NativeWriteStatusResponse`).
- [ ] **Step 2:** `httptest` tests decoding representative JSON for each.
- [ ] **Step 3:** Gate: `go test -run 'TestGovernance|TestPricing|TestStatus' ./...`.

---

### Task 11: Deprecate existing exposed-signing API

**Files:**
- Modify: `sign.go`, `transactions_types.go`, `transactions_client.go`, `tokens_client.go` (doc comments only)

- [ ] **Step 1:** Add `// Deprecated: use client.Transactions()/.Tokens()/.Accounts() with a Signer; signing is handled internally.` doc comments to `SignMessage`, `EncodePayload`, `HashMessage`, the `*Request` types, and the legacy write methods (`SendPayment`, `IssueToken`, `MintToken`, `BridgeAndMintToken`, `BurnToken`, `BurnAndBridgeToken`, `SetTokenBlacklist`, `SetTokenWhitelist`, `PauseToken`, `GrantTokenAuthority`, `UpdateTokenMetadata`). Do NOT change signatures/behavior.
- [ ] **Step 2:** Gate: `go build ./... && go vet ./...` (deprecations don't break build).

---

### Task 12: CHANGELOG + README migration section

**Files:**
- Create: `CHANGELOG.md`
- Modify: `README.md` (add "v1.2.0 / adopting the new API" section with side-by-side examples)

- [ ] **Step 1:** Write `CHANGELOG.md` `## [1.2.0]` per design §13 (Added / Notes / Deprecated / Migration; no import-path change).
- [ ] **Step 2:** Add README section: default v2 behavior (import path unchanged), `NewPrivateKeySigner` + namespace example, `WithLegacyV1` opt-in, deprecation note.
- [ ] **Step 3:** Gate: `gofmt -l .` (empty) and no build impact.

---

### Task 13: Final verification (goal gate)

- [ ] **Step 1:** `go build ./...` → success.
- [ ] **Step 2:** `go vet ./...` → clean.
- [ ] **Step 3:** `gofmt -l .` → empty output.
- [ ] **Step 4:** `go test ./...` → all pass, including `TestNativeV2Conformance` (all golden vectors) and payload-RLP tests.
- [ ] **Step 5:** Report a summary of files added/changed, vectors passing, and the changelog location. Do NOT commit unless the user asks.

---

## Self-Review

- **Spec coverage:** mode selector (T1), Signer (T2), signing core (T3), conformance (T4), memo+new types (T5), requests+hash-verify (T6), namespaces (T7–T9), gaps §9 writes (batch/clawback in T7/T8, multisig T9) + reads (T10), v1 compat/deprecation (T11), changelog (T12), gates (T13). All design sections mapped.
- **Placeholder scan:** value JSON serialization and exact JSON tags/response shapes are marked "verify against l1client during impl" with exact file paths — acceptable (data shapes, not logic gaps). Task 3's Memo ordering note corrected: `memo.go` (Task 5) defines `Memo`; Task 3 uses it — implement `memo.go` before wiring Task 3's `encodeWithMemo`, or define `Memo` in `memo.go` first. Reorder if needed: T5's Memo type is a prerequisite for T3 compilation, so create `memo.go`'s `Memo`/`EmptyMemo` before T3 build (do the T5 Memo bits early).
- **Type consistency:** `Signature{R,S string; V uint64}` reused everywhere; `SubmitOption`/`submitConfig` from T5 used in T6–T9; op-type constants centralized in T3/Global Constraints.

**Prerequisite ordering fix:** Do Task 5's `memo.go` (Memo, EmptyMemo, SubmitOption) **before** Task 3, since `native_v2.go` references `Memo`. Everything else follows the numeric order.
