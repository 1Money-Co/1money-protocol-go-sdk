# Native V2 Golden Vector Coverage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Generate deterministic raw-field golden vectors with the Rust production native-v2 implementation and use them to prove every Go `PrepareTransaction` signing hash and `AuthorizedTransaction` transaction hash, including encoding-sensitive edge cases.

**Architecture:** A test-only `om-sdk` example constructs typed Rust payloads, calls `signing_hash_single` and `sign_single`, and emits raw logical fields plus signature and expected hashes. A static copy of that JSON lives in the Go SDK; type-specific Go decoding reconstructs public payload structs before invoking `PrepareTransaction` and `Authorize`. Go-only invalid values remain table-driven rejection tests.

**Tech Stack:** Rust 2024, `om-sdk`, `om-primitives-types`, `serde_json`; Go, go-ethereum `common`/`crypto`, standard `encoding/json`, `math/big`, Go testing.

## Global Constraints

- Do not export payload RLP, unsigned transaction RLP, or signed transaction RLP in the new fixture.
- Rust production native-v2 APIs are the sole oracle for successful expected hashes.
- Preserve `Option::None` versus `Some("")`/`Some(B256::ZERO)` and all `Vec` element order.
- Go tests must build concrete public payload types; they must not construct generic RLP lists from fixture JSON.
- Preserve `nil *big.Int -> U256 zero`; reject negative values and values greater than `U256::MAX`.
- Do not change L1 production behavior or existing low-level conformance vectors.
- Preserve pre-existing workspace changes and do not stage or commit unless separately requested.

---

### Task 1: Add the deterministic Rust exporter

**Files:**
- Modify: `../l1client/crates/om-sdk/Cargo.toml`
- Create: `../l1client/crates/om-sdk/examples/export_go_sdk_native_v2_vectors.rs`

**Interfaces:**
- Consumes: `onemoney_protocol::crypto::native_domain_separated::{signing_hash_single, sign_single}` and concrete payload types re-exported by `onemoney_protocol`.
- Produces: stdout JSON with `_source` and `vectors`; CLI requires `--source-commit <40-hex-sha>`.

- [ ] **Step 1: Register the explicit example target**

Add:

```toml
[[example]]
name = "export_go_sdk_native_v2_vectors"
path = "examples/export_go_sdk_native_v2_vectors.rs"
```

- [ ] **Step 2: Add exporter unit-testable helpers**

Implement these exact internal shapes:

```rust
#[derive(Serialize)]
struct Fixture {
    #[serde(rename = "_source")]
    source: Source,
    vectors: Vec<Vector>,
}

#[derive(Serialize)]
struct Vector {
    name: &'static str,
    class: &'static str,
    operation: &'static str,
    operation_type: u16,
    payload: Value,
    options: Value,
    authorization: Authorization,
    expected: Expected,
}

fn vector<P: NativeSigningPayload>(
    name: &'static str,
    class: &'static str,
    operation: &'static str,
    payload: &P,
    raw_payload: Value,
    options: Value,
) -> Result<Vector, Box<dyn Error>>
```

`vector` must call `signing_hash_single(payload)` and `sign_single(payload,
TEST_PRIVATE_KEY)`, extract the single signature, and serialize only raw fields
and expected values.

- [ ] **Step 3: Add a failing exporter self-test**

Under `#[cfg(test)]`, assert:

```rust
assert_eq!(fixture.vectors.iter().filter(|v| v.class == "canonical").count(), 14);
assert_eq!(
    fixture.vectors.iter().filter(|v| v.class == "canonical")
        .map(|v| v.operation_type).collect::<Vec<_>>(),
    (1_u16..=14).collect::<Vec<_>>(),
);
assert!(serialized.get().contains("\"payload\""));
assert!(!serialized.get().contains("payload_rlp"));
```

- [ ] **Step 4: Run the self-test and observe the incomplete fixture failure**

Run:

```bash
cargo test -p onemoney-protocol --example export_go_sdk_native_v2_vectors
```

Expected: FAIL until `build_fixture` supplies all canonical vectors.

- [ ] **Step 5: Construct the 14 canonical typed payloads**

> **Superseded for BatchPayment (2026-08-10):** the instruction below to use
> bare `BatchPaymentPayload` for operation 14 is stale. `max_fee` was removed
> from the signed payload, and BatchPayment now signs as
> `WithMemo<BatchPaymentPayload>` like every other operation — there is no
> bare-payload form for any of the fourteen operations. See
> `docs/superpowers/specs/2026-08-10-batch-payment-v2-rebaseline-design.md`.

Use `WithMemo<T>` for operation types 1–13 and bare
`BatchPaymentPayload` for operation 14. Raw JSON uses decimal U256 strings,
hex addresses/bytes, JSON arrays, and JSON null for absent options. Use valid
compressed secp256k1 public keys derived from fixed test scalars for
CreateMultiSig.

- [ ] **Step 6: Add edge vectors**

Add named vectors for:

```text
memo: empty, type-only, format-only, data-only, full, UTF-8, maximum lengths
batch Option: none/none, some/none, none/some, some/some, none/some-empty,
              some-zero/none
batch operations: empty, singleton, multiple, reversed, amount-zero,
                  amount-U256-max
metadata: empty, singleton, multiple, reversed, duplicate-key, empty-key-value
multisig signers: singleton, multiple, reversed, weighted threshold 300
bridge_param: empty, one byte, multiple bytes, leading zero, non-empty all-zero
numeric: U256 zero/max and u64 zero/max representatives
collisions: Payment/Mint, Blacklist/Whitelist, Pause/Burn
```

Reuse one vector across categories only when its `class`/name and completeness
checks make that coverage explicit.

- [ ] **Step 7: Verify deterministic exporter output**

Run twice with the same source commit and compare:

```bash
cargo run -p onemoney-protocol --example export_go_sdk_native_v2_vectors -- \
  --source-commit "$(git rev-parse HEAD)"
```

Expected: valid JSON, identical output, no encoded transaction fields.

---

### Task 2: Generate the static Go fixture

**Files:**
- Create: `testdata/prepare-authorize-hash-vectors.json`

**Interfaces:**
- Consumes: stdout from Task 1.
- Produces: static data consumed only by Go tests.

- [ ] **Step 1: Generate into a temporary path**

Run from `../l1client`:

```bash
cargo run -p onemoney-protocol --example export_go_sdk_native_v2_vectors -- \
  --source-commit "$(git rev-parse HEAD)" \
  > /tmp/prepare-authorize-hash-vectors.json
```

- [ ] **Step 2: Validate schema without modifying expected values**

Check:

```bash
jq -e '
  (.vectors | length > 14) and
  ([.vectors[] | select(.class == "canonical") | .operation_type] == [1,2,3,4,5,6,7,8,9,10,11,12,13,14]) and
  (all(.vectors[]; has("payload") and has("authorization") and has("expected"))) and
  ([paths | map(tostring) | join(".") | select(contains("payload_rlp"))] | length == 0)
' /tmp/prepare-authorize-hash-vectors.json
```

Expected: exit 0.

- [ ] **Step 3: Add the generated bytes as the Go fixture**

Copy the validated output byte-for-byte to:

`testdata/prepare-authorize-hash-vectors.json`

Do not hand-edit hashes or signatures.

---

### Task 3: Add the type-specific Go fixture consumer

**Files:**
- Modify: `native_v2_prepare_test.go`

**Interfaces:**
- Consumes: `testdata/prepare-authorize-hash-vectors.json`.
- Produces:

```go
type prepareAuthorizeFixture struct {
    Source  prepareAuthorizeSource   `json:"_source"`
    Vectors []prepareAuthorizeVector `json:"vectors"`
}

func loadPrepareAuthorizeFixture(t *testing.T) prepareAuthorizeFixture
func (v prepareAuthorizeVector) goPayload(t *testing.T) (any, []SubmitOption)
```

- [ ] **Step 1: Write fixture completeness tests**

Assert unique names, 14 canonical vectors, exact canonical operation IDs
`1..14`, complete signature/hash fields, and absence of `payload_rlp` at the
raw-JSON level.

- [ ] **Step 2: Run the focused test and observe decoder failure**

Run:

```bash
go test -run TestPrepareAuthorizeFixtureCompleteness
```

Expected: FAIL until the loader and type switch exist.

- [ ] **Step 3: Implement lossless raw-field helpers**

Implement:

```go
func parseFixtureBig(t *testing.T, value string) *big.Int
func parseFixtureAddress(t *testing.T, value string) common.Address
func parseFixtureHashPtr(t *testing.T, value *string) *common.Hash
func parseFixtureHexBytes(t *testing.T, value string) HexBytes
```

Reject malformed decimal/hex, wrong address/hash lengths, and values outside
U256 instead of silently truncating.

- [ ] **Step 4: Implement the operation switch**

Decode each `payload` raw message into a fixture DTO and construct exactly one
of:

```text
PaymentPayload, TokenIssuePayload, TokenMintPayload, TokenAuthorityPayload,
TokenManageListPayload, PauseTokenPayload, TokenBurnPayload,
TokenClawbackPayload, UpdateMetadataPayload, TokenBridgeAndMintPayload,
TokenBurnAndBridgePayload, CreateMultiSigPayload, BatchPaymentPayload
```

Apply `WithMemo` only when `options.memo` is present. Apply
`WithManageListKind` for blacklist/whitelist. Preserve every array's supplied
order.

- [ ] **Step 5: Run completeness/decoder tests**

Run:

```bash
go test -run 'TestPrepareAuthorizeFixtureCompleteness|TestPrepareAuthorizeFixtureDecodesPublicPayloads'
```

Expected: PASS.

---

### Task 4: Replace partial public hashing checks with full Rust-oracle tests

**Files:**
- Modify: `native_v2_prepare_test.go`
- Modify: `native_v2_batch_test.go`

**Interfaces:**
- Consumes: Task 3 loader and concrete payload constructor.
- Produces: one public-path subtest per fixture vector.

- [ ] **Step 1: Write the public-path golden test**

For every vector:

```go
prepared, err := PrepareTransaction(payload, opts...)
requireNoError(t, err)
assertHexEqual(t, prepared.SigningHash(), v.Expected.SigningHash)

authorized, err := prepared.Authorize(v.Authorization)
requireNoError(t, err)
assertHexEqual(t, authorized.TransactionHash(), v.Expected.TransactionHash)
```

Recover the signer with `crypto.SigToPub` and assert the same fixed Rust test
address for every vector.

- [ ] **Step 2: Run and observe any Go/Rust mismatch**

Run:

```bash
go test -run TestPrepareAndAuthorizeMatchRustGoldenVectors -count=1
```

Expected: FAIL on any public payload mapping, optional trailing-field,
multisig signer-list, memo, byte-vector, or authorization mismatch.

- [ ] **Step 3: Fix only demonstrated compatibility defects**

Do not change expected fixture values. Correct Go construction/encoding only
where a failing Rust vector proves divergence.

- [ ] **Step 4: Consolidate BatchPayment tests**

Keep structural RLP-length assertions in `native_v2_batch_test.go`, but replace
its separate JSON loader with filtered vectors from the consolidated fixture.

- [ ] **Step 5: Run focused golden and BatchPayment tests**

Run:

```bash
go test -run 'TestPrepareAndAuthorizeMatchRustGoldenVectors|TestBatchPayment' -count=1
```

Expected: PASS.

---

### Task 5: Complete Go-only rejection and defensive-copy coverage

**Files:**
- Modify: `native_v2_prepare.go`
- Modify: `native_v2_prepare_test.go`

**Interfaces:**
- Produces:

```go
func validateU256(name string, value *big.Int) error
```

- [ ] **Step 1: Add table-driven invalid-U256 tests**

For every U256 field identified in the design, test negative one and `2^256`;
also prove nil and explicit zero prepare to the same hash.

- [ ] **Step 2: Run and observe current acceptance**

Run:

```bash
go test -run TestPrepareRejectsOutOfRangeU256 -count=1
```

Expected: FAIL before validation is added.

- [ ] **Step 3: Add minimal validation**

Implement:

```go
func validateU256(name string, value *big.Int) error {
    if value == nil {
        return nil
    }
    if value.Sign() < 0 {
        return fmt.Errorf("%s must be non-negative", name)
    }
    if value.BitLen() > 256 {
        return fmt.Errorf("%s exceeds U256", name)
    }
    return nil
}
```

Call it for every U256-bearing payload field before `rlpList()` and
`wireFields()`.

- [ ] **Step 4: Complete authorization negative tests**

Retain or add rejection for invalid parity, zero/out-of-range `r`/`s`, and
high-S; add distinct-signature and defensive-copy transaction-hash tests.

- [ ] **Step 5: Run focused negative tests**

Run:

```bash
go test -run 'TestPrepareRejectsOutOfRangeU256|TestAuthorize|TestAuthorizedTransactionHash' -count=1
```

Expected: PASS.

---

### Task 6: Remove superseded temporary data and verify both repositories

**Files:**
- Delete: `testdata/batch-payment-optional-vectors.json`
- Review: all files changed by Tasks 1–5

- [ ] **Step 1: Prove the consolidated fixture covers every temporary case**

Run the filtered BatchPayment test and compare the old four names/states with
the consolidated fixture before deleting the temporary file.

- [ ] **Step 2: Delete only the superseded temporary fixture**

Do not delete or rewrite unrelated untracked files.

- [ ] **Step 3: Format and verify Rust**

Run from `../l1client`:

```bash
cargo +nightly fmt --all
cargo check -p onemoney-protocol --example export_go_sdk_native_v2_vectors
cargo test -p onemoney-protocol --example export_go_sdk_native_v2_vectors
cargo clippy -p onemoney-protocol --example export_go_sdk_native_v2_vectors -- -D warnings
```

- [ ] **Step 4: Format and verify Go**

Run:

```bash
gofumpt -l -w .
golangci-lint run
go test ./...
ENABLE_HTTP_CLIENT_TESTS=1 go test ./...
go test -race ./...
go vet ./...
go mod tidy -diff
go mod verify
```

If `gofumpt` or `golangci-lint` is unavailable, run `gofmt`/`go vet` and
report the exact unavailable command.

- [ ] **Step 5: Review final scope and generated-data provenance**

Run:

```bash
git status --short
git diff --check
git diff --stat
```

In both repositories, confirm no secrets, unrelated edits, generated build
output, staging, or commits were introduced.
