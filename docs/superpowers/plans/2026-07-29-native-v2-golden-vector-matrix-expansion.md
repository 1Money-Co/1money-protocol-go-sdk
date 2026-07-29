# Native-v2 Golden Vector Matrix Expansion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expand the Rust-oracle fixture from representative edge coverage to exhaustive finite-state coverage, explicit numeric/string boundaries, and machine-enforced BatchPayment pairwise interactions.

**Architecture:** The Rust example remains the sole producer of successful expected hashes and exports only original fields. It gains deterministic data generators and a semantic self-test. Go continues to reconstruct public payload types, but its completeness test derives coverage facts from decoded payload values instead of trusting vector names.

**Tech Stack:** Rust, `om-primitives-types`, `onemoney-protocol`, `k256`, Serde JSON, Go, go-ethereum, standard `testing`.

## Global Constraints

- Keep one consolidated fixture: `testdata/prepare-authorize-hash-vectors.json`.
- Never export payload RLP, transaction RLP, private keys, or signer-generation scalars.
- Use Rust production `signing_hash_single` and `sign_single` as the sole successful-hash oracle.
- Enumerate every finite enum/boolean tuple; use boundary classes and deterministic pairwise coverage for infinite domains.
- Coverage guards must inspect decoded values, not infer coverage from vector names.
- Do not change any public Go API or L1 production behavior.
- Preserve Go `nil *big.Int -> U256 zero` behavior and current invalid-U256 rejection.
- Do not stage or commit changes unless the user explicitly requests it.

---

### Task 1: Make the Rust exporter support generated matrices

**Files:**
- Modify: `l1client/crates/om-sdk/examples/export_go_sdk_native_v2_vectors.rs`
- Test: `l1client/crates/om-sdk/examples/export_go_sdk_native_v2_vectors.rs`

**Interfaces:**
- Consumes: existing `vector`, `memo_vector`, `list_vector`, `batch_vector`, and `build_fixture`.
- Produces: dynamically named deterministic `Vector` values and reusable exact-byte string/key helpers.

- [x] **Step 1: Add a failing exporter self-test for generated names and exact byte lengths**

Add tests that require:

```rust
assert_eq!(boundary_string(55, false).len(), 55);
assert_eq!(boundary_string(56, true).len(), 56);
assert!(!boundary_string(56, true).is_ascii());
let signers = deterministic_signers(257, 255)?;
assert_eq!(signers.len(), 257);
let total = signers
    .iter()
    .try_fold(0_u16, |total, signer| {
        total.checked_add(u16::from(signer.weight))
    })
    .expect("fixture signer weight must not overflow u16");
assert_eq!(total, u16::MAX);
```

The signer total assertion uses checked accumulation.

- [x] **Step 2: Run the exporter test and confirm it fails**

Run:

```bash
CARGO_TARGET_DIR=/tmp/l1client-native-v2-target \
  cargo test -p onemoney-protocol --example export_go_sdk_native_v2_vectors
```

Expected: compilation failure because `boundary_string` and `deterministic_signers` do not exist.

- [x] **Step 3: Change generated labels from borrowed strings to owned strings**

Change:

```rust
struct Vector {
    name: String,
    class: &'static str,
    operation: &'static str,
    // unchanged fields
}
```

Make `vector`, `memo_vector`, `list_vector`, and `batch_vector` accept
`name: impl Into<String>` and store `name.into()`. Keep `class` and operation
names static because their vocabulary is fixed.

- [x] **Step 4: Add deterministic boundary and signer helpers**

Implement:

```rust
fn boundary_string(byte_len: usize, utf8: bool) -> String {
    if !utf8 || byte_len < 3 {
        return "a".repeat(byte_len);
    }
    format!("界{}", "a".repeat(byte_len - "界".len()))
}
```

Implement `deterministic_signers(count, weight)` with `k256::ecdsa::SigningKey`.
For index `0..count`, encode scalar `index + 1` into a 32-byte big-endian array,
derive the SEC1 compressed verifying key, and reject a zero/invalid scalar.
Return `Result<Vec<MultiSigSigner>, Box<dyn Error>>`. Assert uniqueness in the
exporter self-test and ensure the generated set includes both `0x02` and `0x03`
prefixes.

- [x] **Step 5: Run the exporter self-test**

Run the Task 1 test command. Expected: PASS.

---

### Task 2: Add exhaustive enum/boolean and per-field numeric vectors

**Files:**
- Modify: `l1client/crates/om-sdk/examples/export_go_sdk_native_v2_vectors.rs`
- Test: `l1client/crates/om-sdk/examples/export_go_sdk_native_v2_vectors.rs`

**Interfaces:**
- Consumes: generated-name support from Task 1.
- Produces: fixture classes `enum_matrix`, `issue_matrix`, and `numeric_boundary`.

- [x] **Step 1: Add a failing semantic Rust self-test**

Build the fixture and derive tuples from `vector.payload`. Require equality
with these sets:

```text
TokenAuthority:
  {Grant, Revoke}
    × {MasterMintBurn, MintBurnTokens, Pause, ManageList,
       UpdateMetadata, Bridge, Clawback}

TokenBlacklist: {Add, Remove}
TokenWhitelist: {Add, Remove}
TokenPause: {Pause, Unpause}
TokenIssue booleans:
  {(false,false), (false,true), (true,false), (true,true)}
```

For every U256 field key below, require both decimal `"0"` and
`U256::MAX.to_string()` among vectors for that operation:

```text
Payment.value
BatchPayment.operations[].amount
BatchPayment.max_fee
TokenMint.value
TokenAuthority.value
TokenBurn.value
TokenClawback.value
TokenBridgeAndMint.value
TokenBurnAndBridge.value
TokenBurnAndBridge.escrow_fee
```

- [x] **Step 2: Run the test and confirm the current fixture fails**

Expected: missing enum tuples and missing per-field U256 boundary values.

- [x] **Step 3: Generate the complete finite matrices**

Append deterministic loops to `build_fixture`:

- 14 authority action/type tuples;
- blacklist Add/Remove;
- whitelist Add/Remove;
- pause Pause/Unpause;
- four TokenIssue boolean tuples.

Distribute `decimals=0` and `decimals=255` across the four TokenIssue vectors.
Use stable names containing the encoded values, but rely on the semantic test,
not those labels, for completeness.

- [x] **Step 4: Generate independent U256 boundary vectors**

For each U256 field, clone/reconstruct its canonical payload twice and change
only that field to `U256::ZERO` or `U256::MAX`. For BatchPayment, use one
operation when testing amount and keep the Option fields absent. For
TokenBurnAndBridge, vary `value` and `escrow_fee` independently.

Add fixed-width boundary vectors for:

```text
Payment.chain_id = 0 / u64::MAX
Payment.nonce = 0 / u64::MAX
BatchPayment.created_at = 0 / u64::MAX
TokenBridgeAndMint.source_chain_id = 0 / u64::MAX
TokenBurnAndBridge.destination_chain_id = 0 / u64::MAX
```

- [x] **Step 5: Run the exporter self-test**

Expected: the enum and numeric semantic sets pass.

---

### Task 3: Add exact string/byte and multisig boundaries

**Files:**
- Modify: `l1client/crates/om-sdk/examples/export_go_sdk_native_v2_vectors.rs`
- Test: `l1client/crates/om-sdk/examples/export_go_sdk_native_v2_vectors.rs`

**Interfaces:**
- Consumes: `boundary_string` and `deterministic_signers`.
- Produces: fixture classes `string_boundary`, `bytes`, `memo`, and `multisig_boundary`.

- [x] **Step 1: Add failing semantic assertions for actual byte lengths**

For each target field, collect `value.as_bytes().len()` from decoded fixture
payloads and require:

```text
TokenIssue.symbol/name:                    0, 1, 55, 56, 255, 256
TokenMetadata.name/uri/key/value:          0, 1, 55, 56, 255, 256
TokenBridgeAndMint.source_tx_hash/
  bridge_metadata:                         0, 1, 55, 56, 255, 256
TokenBurnAndBridge.destination_address/
  bridge_metadata/bridge_param:            0, 1, 55, 56, 255, 256
BatchPayment.Some(batch_id):               0, 1, 55, 56, 255, 256
memo.type:                                 0, 1, 55, 56, 128
memo.format:                               0, 1, 55, 56, 64
memo.data:                                 0, 1, 55, 56, 255, 256
```

For every string field, also require at least one non-ASCII UTF-8 value.
For `bridge_param`, require leading-zero and non-empty all-zero values.

- [x] **Step 2: Run the test and confirm missing lengths are reported**

Expected: current fixture lacks most per-field RLP length transitions.

- [x] **Step 3: Generate one-field-at-a-time boundary vectors**

Create small payload-builder closures for TokenIssue, TokenMetadata,
TokenBridgeAndMint, TokenBurnAndBridge, BatchPayment, and Payment-with-memo.
For each target field and required byte length, start from the canonical
payload and change only the target field.

Use `boundary_string(length, false)` for exact ASCII byte lengths and one
additional `boundary_string(length, true)` case for UTF-8. For `bridge_param`,
use `Bytes::from(vec![0x61; length])`; retain explicit leading-zero and all-zero
vectors.

- [x] **Step 4: Add valid multisig boundary vectors**

Add actual raw-field vectors for:

- a valid `0x03` compressed key;
- one signer with weight 255 and threshold 255;
- two signers whose threshold is greater than 255;
- 257 deterministic distinct signers, all weight 255, threshold 65535.

The fixture contains only compressed public keys. It must not contain the
derived secret scalar bytes.

- [x] **Step 5: Run the exporter self-test**

Expected: all byte-length, UTF-8, and multisig semantic assertions pass.

---

### Task 4: Generate and prove the BatchPayment pairwise matrix

**Files:**
- Modify: `l1client/crates/om-sdk/examples/export_go_sdk_native_v2_vectors.rs`
- Test: `l1client/crates/om-sdk/examples/export_go_sdk_native_v2_vectors.rs`

**Interfaces:**
- Produces: deterministic `batch_pairwise` vectors and a zero-uncovered-pairs assertion.

- [x] **Step 1: Define explicit factor levels**

Add private enums:

```rust
enum BatchOptionLevel { Neither, HashOnly, IdOnly, Both }
enum BatchOperationsLevel { Empty, Single, Forward, Reverse }
enum BatchAmountLevel { Ordinary, Zero, Max }
enum BatchFeeLevel { Ordinary, Zero, Max }
```

Only non-empty operation levels have an amount level. Forward/reverse require
two operations.

- [x] **Step 2: Add a failing pair-coverage test**

Derive factor levels from each actual `BatchPayment` raw payload and insert
every observed cross-factor pair into a set. Build the required set from all
semantically valid pairs:

- every Option × Operations pair;
- every Option × Amount pair;
- every Option × Fee pair;
- every non-empty Operations × Amount pair;
- every Operations × Fee pair;
- every Amount × Fee pair.

Assert `required - observed` is empty and print missing pairs on failure.

- [x] **Step 3: Implement a deterministic greedy covering-array generator**

Enumerate all valid candidate rows in enum declaration order. Repeatedly select
the candidate covering the largest number of uncovered pairs; break ties by
candidate enumeration order. Stop when no required pairs remain. Convert each
row into a `BatchPaymentPayload` whose actual raw values unambiguously encode
the selected levels.

- [x] **Step 4: Preserve Option value-shape edges**

Keep separate successful vectors for:

- `Some("")` batch ID;
- `Some(B256::ZERO)` operations hash.

Do not treat these as additional presence levels in the covering array.

- [x] **Step 5: Run the exporter self-test twice**

Run the exporter test, then generate two JSON files with the same
`--source-commit` and compare them with `cmp`. Expected: tests pass and output
is byte-identical.

---

### Task 5: Regenerate the static Rust-oracle fixture

**Files:**
- Modify: `1money-protocol-go-sdk/testdata/prepare-authorize-hash-vectors.json`

**Interfaces:**
- Consumes: completed Rust exporter.
- Produces: the sole static input for Go golden tests.

- [x] **Step 1: Run the final exporter checks**

```bash
CARGO_TARGET_DIR=/tmp/l1client-native-v2-target \
  cargo test -p onemoney-protocol --example export_go_sdk_native_v2_vectors

CARGO_TARGET_DIR=/tmp/l1client-native-v2-target \
  cargo clippy -p onemoney-protocol --example export_go_sdk_native_v2_vectors -- -D warnings
```

- [x] **Step 2: Generate with exact oracle provenance**

Use the current `l1client` production commit:

```bash
CARGO_TARGET_DIR=/tmp/l1client-native-v2-target \
  cargo run -q -p onemoney-protocol \
  --example export_go_sdk_native_v2_vectors -- \
  --source-commit b0e6e5b07fbec2c083e5c48f95a6494fb89adf7c \
  > ../1money-protocol-go-sdk/testdata/prepare-authorize-hash-vectors.json
```

- [x] **Step 3: Validate raw-field-only schema**

Use `jq` to assert:

- operation types are exactly 1 through 14;
- no object contains `payload_rlp`, `transaction_rlp`, or a private-key field;
- expected hashes are 32-byte lowercase `0x` hex;
- authorization `r` and `s` are 32-byte hex and `v` is 0 or 1.

---

### Task 6: Replace name-only Go coverage checks with semantic checks

**Files:**
- Modify: `1money-protocol-go-sdk/native_v2_prepare_test.go`
- Modify: `1money-protocol-go-sdk/native_v2_batch_test.go`
- Test: `1money-protocol-go-sdk/native_v2_prepare_test.go`
- Test: `1money-protocol-go-sdk/native_v2_batch_test.go`

**Interfaces:**
- Consumes: raw fixture DTOs and public payload decoder already in `native_v2_prepare_test.go`.
- Produces: machine-enforced equality with sections 8.1–8.7 of the approved spec.

- [x] **Step 1: Write failing semantic coverage subtests**

Replace the required-name list in
`TestPrepareAuthorizeFixtureEdgeCoverage` with subtests that inspect actual
decoded values:

```go
t.Run("finite enums", assertFiniteEnumCoverage)
t.Run("numeric boundaries", assertNumericBoundaryCoverage)
t.Run("string and byte boundaries", assertLengthBoundaryCoverage)
t.Run("multisig boundaries", assertMultisigBoundaryCoverage)
```

Each helper receives `[]prepareAuthorizeVector`, unmarshals the appropriate
typed fixture payload, builds actual sets, and compares them with exact
expected sets. Set mismatch messages must print missing and unexpected values.

- [x] **Step 2: Run Go tests and confirm the old fixture fails**

```bash
GOCACHE=/tmp/1money-go-sdk-cache go test ./... \
  -run 'TestPrepareAuthorizeFixtureEdgeCoverage|TestBatchPaymentOptionalGoldenVectors'
```

Expected: missing enum, numeric, length, multisig, and pairwise cells.

- [x] **Step 3: Add the BatchPayment pairwise semantic guard**

In `native_v2_batch_test.go`, derive Option/operations/amount/fee levels from
the actual `batchFixturePayload`. Build applicable required pairs with the same
conditional rules as the Rust test and assert exact coverage.

Retain the focused RLP placeholder assertions in
`TestBatchPaymentOptionalTrailingFields`.

- [x] **Step 4: Run the focused Go tests against the regenerated fixture**

Expected: all semantic coverage and Rust golden comparisons pass.

- [x] **Step 5: Keep every public-path golden assertion**

Confirm `TestPrepareAndAuthorizeMatchRustGoldenVectors` still loops over every
vector, constructs the actual public payload type, verifies Rust signing hash,
authorizes with the exported signature, verifies Rust transaction hash, and
recovers the fixed test signer.

---

### Task 7: Update documentation and run complete verification

**Files:**
- Modify: `1money-protocol-go-sdk/docs/superpowers/specs/2026-07-28-native-v2-golden-vector-coverage-design.md`
- Modify: `1money-protocol-go-sdk/docs/superpowers/plans/2026-07-29-native-v2-golden-vector-matrix-expansion.md`

- [x] **Step 1: Mark the implemented matrix accurately**

Update only statements affected by implementation details discovered during
TDD. Do not relax an acceptance criterion to make a failing implementation
appear complete.

- [x] **Step 2: Run Go repository verification**

```bash
gofumpt -l -w .
golangci-lint run
GOCACHE=/tmp/1money-go-sdk-cache go test ./...
GOCACHE=/tmp/1money-go-sdk-cache ENABLE_HTTP_CLIENT_TESTS=1 go test ./...
GOCACHE=/tmp/1money-go-sdk-cache go test -race ./...
GOCACHE=/tmp/1money-go-sdk-cache go vet ./...
GOCACHE=/tmp/1money-go-sdk-cache go mod tidy -diff
GOCACHE=/tmp/1money-go-sdk-cache go mod verify
```

If `gofumpt` or `golangci-lint` is unavailable, run `gofmt -l`/`go vet` and
report the exact unavailable command.

- [x] **Step 3: Run Rust repository verification**

From `l1client/`:

```bash
cargo +nightly fmt --all -- --check
CARGO_TARGET_DIR=/tmp/l1client-native-v2-target cargo check
CARGO_TARGET_DIR=/tmp/l1client-native-v2-target \
  cargo clippy --all-targets --all-features --workspace -- -D warnings
CARGO_TARGET_DIR=/tmp/l1client-native-v2-target \
  cargo nextest run --all-targets --all-features --workspace -E 'not kind(bench)'
cargo deny check
```

- [x] **Step 4: Review final scope**

In both repositories run `git diff --check`, `git status --short`, and inspect
all untracked generated/source files. Confirm no RLP shortcut, secret,
unrelated edit, build output, staging, or commit was introduced.
