# BatchPayment V2 Re-baseline Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the Go SDK's BatchPayment signing, submission, fee estimation, and query decoding byte-identical to `l1client` main (`7ad79889`), which removed the signed `max_fee` field and made BatchPayment memo-bearing.

**Architecture:** Two repositories, strictly serial. Phase A ports the Rust golden-vector generator onto `l1client/main` and regenerates the extended fixture — this is a hard prerequisite because the Go test package will not compile once the production `MaxFee` field is removed while the old fixture and its decoder remain. Phase B applies the Go re-baseline against that regenerated fixture. BatchPayment stays inside the existing shared native-v2 pipeline; no BatchPayment-specific signer or hasher is added.

**Tech Stack:** Go 1.x (`package onemoney`, flat layout, `github.com/ethereum/go-ethereum` for `common`, `crypto`, `rlp`); Rust (l1client `crates/om-sdk` example target, `alloy-rlp 0.3.15`).

## Global Constraints

- Native operation type stays `BatchPayment = 14`. Never reorder or reuse operation-type values.
- Canonical v2 `payload_rlp` for **all fourteen** operations is `rlp([payloadList, [memo.type, memo.format, memo.data]])`. There is no bare-payload form after this plan.
- BatchPayment payload RLP field order is exactly: `chain_id, nonce, token, operations, created_at, operations_hash?, batch_id?`. Each operation is `[recipient, amount]`.
- Trailing-option rule (`native-v2-signing-spec` §4.3): both absent → omitted entirely; an absent field before a present one → `0x80` empty-string placeholder. Verified against `alloy-rlp-derive 0.3.15` `en.rs:226-236`; `BatchPaymentPayload` uses plain `#[rlp(trailing)]` with no `no_gaps`.
- All U256 amounts on the wire are **quoted decimal strings**. One serializer for submit and estimate. Never bare JSON numbers, never hexadecimal.
- A nil `PaymentOperation.Amount` encodes as U256 zero everywhere: payload RLP, submit JSON, fee-estimate JSON, operations-hash derivation.
- Non-nil amounts must be non-negative and `<= 2^256-1`; otherwise return an error.
- BatchPayment is v2-only. `pathsForOp(opBatchPayment)` returns `("", "/v2/transactions/batch_payment")`.
- Go fixture oracles are L1-generated. Never hand-calculate a BatchPayment expected hash, and never derive an expected value through the Go implementation under test.
- No intermediate Go state may delete, skip, or defer BatchPayment fixture coverage.
- Go verification commands: `gofumpt -l -w .`, `go test ./...`, `golangci-lint run`.
- l1client verification commands: `cargo check`, `cargo nextest run --all-targets --all-features --workspace -E 'not kind(bench)'`, `cargo clippy --all-targets --all-features --workspace -- -D warnings`, `cargo +nightly fmt`.
- l1client repo rule: never run `git commit`/`git add` on l1client without the user asking. Phase A steps that say "commit" are for the human operator to run or approve.

## Repository Paths

- Go SDK: `/Users/nsh/workspace/1money/layer1/1money-protocol-go-sdk`
- l1client: `/Users/nsh/workspace/1money/layer1/l1client`

All Go paths below are relative to the Go SDK root; all Rust paths are relative to the l1client root.

## File Structure

**Phase A — l1client (separate repository, separate PR, merges first)**

| File | Responsibility |
|---|---|
| `crates/om-sdk/Cargo.toml` | Register the `export_go_sdk_native_v2_vectors` example target (absent on main). |
| `crates/om-sdk/examples/export_go_sdk_native_v2_vectors.rs` | The authoritative Rust oracle. Emits `prepare-authorize-hash-vectors.json`: per-vector payload, options, authorization, and expected `signing_hash` / `transaction_hash`, plus BatchPayment `expected.operations_hash`. |

**Phase B — Go SDK**

| File | Responsibility |
|---|---|
| `native_v2_batch.go` (new) | BatchPayment-specific public surface and shared operation encoders: `batchOperationsRLPList`, `batchOperationsWireList`, `DeriveBatchPaymentOperationsHash`. Mirrors the existing `native_v2_multisig.go` ↔ `native_v2_multisig_test.go` pairing so `native_v2_batch_test.go` finally has a production counterpart. |
| `native_v2_encoding.go` | Per-payload `rlpList()` / `wireFields()`. BatchPayment's two methods delegate operation encoding to `native_v2_batch.go`. |
| `native_v2.go` | Shared native-v2 primitives. Loses the production `encodeBare` helper. |
| `native_v2_prepare.go` | `resolvePayloadOp`, `prepareFromPayload`, `PreparedTransaction`. Loses `memoCapable` throughout. |
| `native_v2_requests.go` | `nativeV2Op`, `pathsForOp`, `submitPayload`, `buildLegacyV1Body`. Gains the v2-only capability check. |
| `transactions_types.go` | `BatchPaymentPayload` (loses `MaxFee`), new `BatchPaymentFeeEstimateRequest` + its `MarshalJSON`. |
| `transactions_payloads.go` | `BatchPaymentData` read DTO (loses `MaxFee`). |
| `transactions.go` | `GetBatchPaymentEstimateFee` + its endpoint constant. |
| `memo.go` | `WithMemo` doc comment (loses the "or a batch payment" carve-out). |

---

# Phase A — l1client golden-vector generator

**These tasks run in the l1client repository on a branch off `main`. They ship as their own PR and must merge before any Phase B task can produce a green Go build.**

### Task 1: Port the generator onto `l1client/main` and make it build

**Files:**
- Create: `crates/om-sdk/examples/export_go_sdk_native_v2_vectors.rs` (ported from `086b69df`)
- Modify: `crates/om-sdk/Cargo.toml` (add the `[[example]]` block)

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: a buildable example target invoked as
  `cargo run -p onemoney-protocol --example export_go_sdk_native_v2_vectors -- --source-commit <40-hex-sha>`,
  writing the fixture JSON to stdout. Its `_source` block is
  `{"repository": "l1client", "commit": "<40-hex>", "generator": "crates/om-sdk/examples/export_go_sdk_native_v2_vectors.rs"}`.

**Context the implementer needs:** `main` and `sec/1038` have diverged in both directions (`main` is 21 commits ahead of `086b69df`; `086b69df` is 69 commits ahead of `main`). This is **not** a cherry-pick. The generator was written against APIs on a branch carrying commits that are not `main` ancestors. Audit every referenced payload type, native-signing trait, SDK signing API, fixture schema, and dependency against current `main` before keeping it. Do **not** pull unrelated `sec/1038` commits in to make it compile.

- [ ] **Step 1: Extract the source file from the branch commit**

```bash
cd /Users/nsh/workspace/1money/layer1/l1client
git show 086b69df:crates/om-sdk/examples/export_go_sdk_native_v2_vectors.rs \
  > crates/om-sdk/examples/export_go_sdk_native_v2_vectors.rs
wc -l crates/om-sdk/examples/export_go_sdk_native_v2_vectors.rs   # expect ~2163
```

- [ ] **Step 2: Register the example target**

Append to `crates/om-sdk/Cargo.toml`, immediately after the existing `governance_example` block and before `[lib]`:

```toml
[[example]]
name = "export_go_sdk_native_v2_vectors"
path = "examples/export_go_sdk_native_v2_vectors.rs"
```

- [ ] **Step 3: Attempt the build and capture the full error set**

```bash
cargo check -p onemoney-protocol --example export_go_sdk_native_v2_vectors 2>&1 | tee /tmp/gen-port-errors.txt
```

Expected: FAIL. At minimum `BatchPaymentPayload` no longer has a `max_fee` field (removed in `7ad79889`), so `batch_payload()` fails to compile. Other errors may appear from the 21 `main`-only commits — record every one before fixing any.

- [ ] **Step 4: Remove `max_fee` from the batch payload constructor**

In `crates/om-sdk/examples/export_go_sdk_native_v2_vectors.rs`, the constructor currently reads:

```rust
fn batch_payload(operations: Vec<PaymentOperation>) -> BatchPaymentPayload {
    BatchPaymentPayload {
        chain_id: CHAIN_ID,
        nonce: 14,
        token: Address::repeat_byte(0x01),
        operations,
        max_fee: U256::from(5000_u64),
        created_at: 1_747_785_600,
        operations_hash: None,
        batch_id: None,
    }
}
```

Delete the `max_fee` line:

```rust
fn batch_payload(operations: Vec<PaymentOperation>) -> BatchPaymentPayload {
    BatchPaymentPayload {
        chain_id: CHAIN_ID,
        nonce: 14,
        token: Address::repeat_byte(0x01),
        operations,
        created_at: 1_747_785_600,
        operations_hash: None,
        batch_id: None,
    }
}
```

- [ ] **Step 5: Delete the `BatchFeeLevel` dimension**

Remove the enum, its `label()` impl, the `fee` field on `BatchPairRow`, every `BatchPairRow` literal's `fee:` entry, and every match arm or helper that sets `max_fee` from a fee level. The three sibling enums stay exactly as they are:

```rust
#[derive(Clone, Copy, Debug)]
enum BatchOptionLevel { Neither, HashOnly, IdOnly, Both }

#[derive(Clone, Copy, Debug)]
enum BatchOperationsLevel { Empty, Single, Forward, Reverse }

#[derive(Clone, Copy, Debug)]
enum BatchAmountLevel { Ordinary, Zero, Max }
```

Also delete the max-fee numeric-boundary vectors (`numeric_batch_max_fee_zero`, `numeric_batch_max_fee_max`) and drop `"max_fee"` from the BatchPayment branch of the serialized-payload decimalizer (`decimalize_field(object, "max_fee")`).

- [ ] **Step 6: Fix the remaining `main`-API mismatches from Step 3's error list**

Work through `/tmp/gen-port-errors.txt` top to bottom. For each error, change the generator to match current `main` — never change `main` to match the generator. Re-run after each fix:

```bash
cargo check -p onemoney-protocol --example export_go_sdk_native_v2_vectors
```

- [ ] **Step 7: Verify the build passes**

Run: `cargo check -p onemoney-protocol --example export_go_sdk_native_v2_vectors`
Expected: PASS, no warnings.

- [ ] **Step 8: Verify the CLI contract still works**

```bash
cargo run -p onemoney-protocol --example export_go_sdk_native_v2_vectors -- --source-commit $(git rev-parse HEAD) | head -20
```

Expected: JSON on stdout whose `_source.commit` equals the printed SHA. A missing or malformed `--source-commit` must still error via the existing guard (`source_commit()`, generator lines 637-649).

- [ ] **Step 9: Run the l1client quality gate**

```bash
cargo +nightly fmt
cargo clippy --all-targets --all-features --workspace -- -D warnings
cargo check
```

Expected: all PASS.

- [ ] **Step 10: Commit**

```bash
git add crates/om-sdk/Cargo.toml crates/om-sdk/examples/export_go_sdk_native_v2_vectors.rs
git commit -m "test(sdk): port go-sdk native v2 vector generator onto main"
```

---

### Task 2: Wrap BatchPayment in `WithMemo` and add the memo, optional-field, and operations-hash coverage

**Files:**
- Modify: `crates/om-sdk/examples/export_go_sdk_native_v2_vectors.rs`

**Interfaces:**
- Consumes: the buildable example target from Task 1.
- Produces: fixture vectors where every `"operation": "BatchPayment"` entry
  (a) is signed over `WithMemo<BatchPaymentPayload>`,
  (b) carries `options.memo` as a three-field object when populated and omits it for the canonical empty memo,
  (c) carries `expected.operations_hash` as a `0x`-prefixed 32-byte hex string,
  and no entry carries a `max_fee` key in `payload`.

- [ ] **Step 1: Change the BatchPayment signing input to the memo wrapper**

BatchPayment is currently the one operation the generator signs bare. Route it through the same `WithMemo` construction the other thirteen use. The batch vector helper currently reads:

```rust
fn batch_vector(
    name: &str,
    class: &str,
    payload: BatchPaymentPayload,
) -> Result<Value, Box<dyn Error>> {
    let raw = raw_payload("BatchPayment", &payload)?;
    vector(name, class, "BatchPayment", &payload, raw, json!({}))
}
```

Give it a memo parameter and wrap:

```rust
fn batch_vector(
    name: &str,
    class: &str,
    payload: BatchPaymentPayload,
    memo: Memo,
) -> Result<Value, Box<dyn Error>> {
    let raw = raw_payload("BatchPayment", &payload)?;
    let options = if memo == Memo::default() {
        json!({})
    } else {
        json!({ "memo": { "type": memo.memo_type, "format": memo.memo_format, "data": memo.memo_data } })
    };
    let signed = WithMemo::new(payload.clone(), memo);
    vector_from_signing_payload(name, class, "BatchPayment", &signed, raw, options)
}
```

Adapt the exact helper names to whatever the ported generator uses for the other thirteen operations — the requirement is that BatchPayment reaches the identical `WithMemo`-based hashing path, not that these names match.

- [ ] **Step 2: Add the memo dimension to the pairwise coverage**

```rust
#[derive(Clone, Copy, Debug)]
enum BatchMemoLevel { Empty, Populated }

impl BatchMemoLevel {
    fn label(self) -> &'static str {
        match self {
            Self::Empty => "empty",
            Self::Populated => "populated",
        }
    }

    fn memo(self) -> Memo {
        match self {
            Self::Empty => Memo::default(),
            Self::Populated => Memo {
                memo_type: "purpose/SALA".to_string(),
                memo_format: "text/plain".to_string(),
                memo_data: "invoice-0001".to_string(),
            },
        }
    }
}
```

Add `memo: BatchMemoLevel` to `BatchPairRow` in the slot vacated by `fee`, and give every row a level so the pairwise matrix covers `option x memo`, `operations x memo`, and `amount x memo`.

- [ ] **Step 3: Emit the six required trailing-option cases**

Keep named vectors for exactly these classes, each with both memo levels:

| Vector name | `operations_hash` | `batch_id` |
|---|---|---|
| `batch_option_neither` | `None` | `None` |
| `batch_option_hash_only` | `Some(0x11 * 32)` | `None` |
| `batch_option_id_only` | `None` | `Some("batch-1")` |
| `batch_option_both` | `Some(0x11 * 32)` | `Some("batch-1")` |
| `batch_option_empty_id` | `None` | `Some("")` |
| `batch_option_zero_hash` | `Some(B256::ZERO)` | `None` |

`batch_option_id_only` is the case that pins the `0x80` placeholder behavior. Note that this fixture format deliberately carries **no** `payload_rlp` field — the generator's own `fixture_covers_every_canonical_operation_without_encoded_payloads` test asserts `!serialized.contains("payload_rlp")`. The placeholder is therefore pinned end-to-end rather than byte-wise: the fixture carries the structured `payload` plus the Rust `expected.signing_hash`, so a Go encoder that dropped or zero-filled the absent `operations_hash` slot would produce a different signing hash and fail. Do not add a `payload_rlp` field to satisfy this.

- [ ] **Step 4: Emit the canonical operations hash for every BatchPayment vector**

Add to the BatchPayment `expected` object, using the node's own method so the Go helper has a true Rust oracle:

```rust
expected["operations_hash"] = json!(format!("{:#x}", payload.canonical_operations_hash()));
```

`BatchPaymentPayload::canonical_operations_hash` lives at `crates/types/om-primitives-types/src/transaction/payload/batch_payment.rs:47-51` and is `keccak256(rlp(self.operations))`.

- [ ] **Step 5: Regenerate and eyeball the BatchPayment output**

```bash
cargo run -p onemoney-protocol --example export_go_sdk_native_v2_vectors -- --source-commit $(git rev-parse HEAD) > /tmp/vectors.json
python3 -c "
import json
d = json.load(open('/tmp/vectors.json'))
b = [v for v in d['vectors'] if v['operation'] == 'BatchPayment']
print('BatchPayment vectors:', len(b))
assert not any('max_fee' in v['payload'] for v in b), 'max_fee still present'
assert all('operations_hash' in v['expected'] for v in b), 'missing expected.operations_hash'
names = {v['name'] for v in b}
for required in ['batch_option_neither','batch_option_hash_only','batch_option_id_only',
                 'batch_option_both','batch_option_empty_id','batch_option_zero_hash']:
    assert required in names, f'missing {required}'
print('ok')
"
```

Expected: prints a vector count and `ok`.

- [ ] **Step 6: Run the l1client quality gate**

```bash
cargo +nightly fmt
cargo clippy --all-targets --all-features --workspace -- -D warnings
cargo check
```

Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add crates/om-sdk/examples/export_go_sdk_native_v2_vectors.rs
git commit -m "test(sdk): re-baseline batch payment vectors on WithMemo"
```

---

### Task 3: Lock the generator's BatchPayment coverage with Rust tests

**Files:**
- Modify: `crates/om-sdk/Cargo.toml` (add `test = true` to the example target)
- Modify: `crates/om-sdk/examples/export_go_sdk_native_v2_vectors.rs` (append a `#[cfg(test)] mod tests`)

**Interfaces:**
- Consumes: the fixture-building entry point from Task 2 (`build_fixture(source_commit: String) -> Result<Value, Box<dyn Error>>`, generator line ~1680).
- Produces: `cargo nextest run -p onemoney-protocol --example export_go_sdk_native_v2_vectors` guarding that required BatchPayment coverage cannot silently disappear.

- [ ] **Step 0: Confirm the example's tests are actually collected**

A `#[cfg(test)] mod tests` inside a Cargo example is only executed if the example target is built as a test. Task 1's implementer reported that the ported file's five existing example tests DO run under `cargo nextest run -p onemoney-protocol --all-targets`, so collection likely already works — but verify it before writing tests that might never execute:

```bash
cargo nextest list -p onemoney-protocol --all-targets 2>&1 | grep -i export_go_sdk
```

Expected: the example's tests are listed. **Only if nothing is listed**, add `test = true` to the block Task 1 registered in `crates/om-sdk/Cargo.toml` and re-check:

```toml
[[example]]
name = "export_go_sdk_native_v2_vectors"
path = "examples/export_go_sdk_native_v2_vectors.rs"
test = true
```

Do not add the flag if collection already works — an unnecessary manifest change is out of scope. Either way, use `--all-targets` in every verification command in this task, and treat Step 3's bite check as the authoritative proof that the tests execute.

- [ ] **Step 1: Write the failing coverage test**

Append to `crates/om-sdk/examples/export_go_sdk_native_v2_vectors.rs`:

```rust
#[cfg(test)]
mod tests {
    use std::collections::BTreeSet;

    use super::*;

    const TEST_COMMIT: &str = "0000000000000000000000000000000000000000";

    fn batch_vectors() -> Vec<Value> {
        let fixture = build_fixture(TEST_COMMIT.to_string()).expect("fixture builds");
        fixture["vectors"]
            .as_array()
            .expect("vectors array")
            .iter()
            .filter(|vector| vector["operation"] == "BatchPayment")
            .cloned()
            .collect()
    }

    #[test]
    fn batch_payment_never_emits_max_fee() {
        for vector in batch_vectors() {
            assert!(
                vector["payload"].get("max_fee").is_none(),
                "vector {} still carries max_fee",
                vector["name"]
            );
        }
    }

    #[test]
    fn batch_payment_pins_every_trailing_option_class() {
        let names: BTreeSet<String> = batch_vectors()
            .iter()
            .map(|vector| vector["name"].as_str().unwrap_or_default().to_string())
            .collect();
        for required in [
            "batch_option_neither",
            "batch_option_hash_only",
            "batch_option_id_only",
            "batch_option_both",
            "batch_option_empty_id",
            "batch_option_zero_hash",
        ] {
            assert!(names.contains(required), "missing required vector {required}");
        }
    }

    #[test]
    fn batch_payment_covers_both_memo_levels() {
        let vectors = batch_vectors();
        let populated = vectors
            .iter()
            .filter(|vector| vector["options"].get("memo").is_some())
            .count();
        let empty = vectors.len() - populated;
        assert!(populated > 0, "no populated-memo BatchPayment vector");
        assert!(empty > 0, "no empty-memo BatchPayment vector");
    }

    #[test]
    fn batch_payment_exports_an_operations_hash_oracle() {
        for vector in batch_vectors() {
            let hash = vector["expected"]["operations_hash"]
                .as_str()
                .unwrap_or_else(|| panic!("vector {} missing expected.operations_hash", vector["name"]));
            assert_eq!(hash.len(), 66, "operations_hash must be 0x + 64 hex chars");
        }
    }

    #[test]
    fn batch_payment_covers_amount_boundaries() {
        let mut amounts = BTreeSet::new();
        for vector in batch_vectors() {
            if let Some(operations) = vector["payload"]["operations"].as_array() {
                for operation in operations {
                    if let Some(amount) = operation["amount"].as_str() {
                        amounts.insert(amount.to_string());
                    }
                }
            }
        }
        let max_u256 =
            "115792089237316195423570985008687907853269984665640564039457584007913129639935";
        assert!(amounts.contains("0"), "no zero-amount operation vector");
        assert!(amounts.contains(max_u256), "no max-U256 operation vector");
    }
}
```

- [ ] **Step 2: Run the tests to verify they pass against Task 2's output**

Run: `cargo nextest run -p onemoney-protocol --example export_go_sdk_native_v2_vectors`
Expected: all five PASS. If `batch_payment_covers_amount_boundaries` fails, Task 2 Step 2 dropped an amount level while removing the fee dimension — restore it rather than weakening the test.

- [ ] **Step 3: Prove the tests actually bite**

Temporarily re-add `max_fee: U256::from(5000_u64)` to `batch_payload()`, re-run, confirm `batch_payment_never_emits_max_fee` FAILS, then revert.

- [ ] **Step 4: Run the full l1client quality gate**

```bash
cargo +nightly fmt
cargo check
cargo nextest run --all-targets --all-features --workspace -E 'not kind(bench)'
cargo clippy --all-targets --all-features --workspace -- -D warnings
cargo deny check
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add crates/om-sdk/examples/export_go_sdk_native_v2_vectors.rs
git commit -m "test(sdk): lock batch payment vector coverage"
```

---

## GATE — cross-repository handoff

> **Correction (decision made after this plan was written, 2026-08-10):** the
> merge-to-`main` gate below was explicitly waived by human decision: there is
> no L1 PR and no merge. The oracle is the local, unpushed l1client branch
> `feat/go-sdk-vector-generator` at commit
> `ee4ce971644587c6903cb8a393371088f8279c56`, used directly. Read the two
> checklist items below as historical record of the original plan, not as
> live requirements; the corresponding design-level correction is in
> `docs/superpowers/specs/2026-08-10-batch-payment-v2-rebaseline-design.md`
> §9.3.3.

**Do not start Phase B Task 5 until every box below is checked.** Task 4 is independent of the fixture and may run in parallel with the L1 PR review.

- [ ] The Phase A PR is **merged** into `l1client/main`.
- [ ] Record the commit on the local branch (superseded — no merge; see the correction above): `cd /Users/nsh/workspace/1money/layer1/l1client && git checkout feat/go-sdk-vector-generator && git rev-parse HEAD` (expected `ee4ce971644587c6903cb8a393371088f8279c56`)
- [ ] Generate the extended fixture from that exact merged commit:

```bash
cd /Users/nsh/workspace/1money/layer1/l1client
L1_SHA=$(git rev-parse HEAD)
cargo run -p onemoney-protocol --example export_go_sdk_native_v2_vectors -- --source-commit "$L1_SHA" \
  > /Users/nsh/workspace/1money/layer1/1money-protocol-go-sdk/testdata/prepare-authorize-hash-vectors.json
```

- [ ] Copy the frozen fixture verbatim:

```bash
cp /Users/nsh/workspace/1money/layer1/l1client/docs/specs/fixtures/native-v2-signing-vectors.json \
   /Users/nsh/workspace/1money/layer1/1money-protocol-go-sdk/testdata/native-v2-signing-vectors.json
```

- [ ] Confirm both fixtures landed correctly:

```bash
cd /Users/nsh/workspace/1money/layer1/1money-protocol-go-sdk
python3 -c "
import json
ext = json.load(open('testdata/prepare-authorize-hash-vectors.json'))
frozen = json.load(open('testdata/native-v2-signing-vectors.json'))
print('_source.commit:', ext['_source']['commit'])
assert len(ext['_source']['commit']) == 40
b = [v for v in ext['vectors'] if v['operation'] == 'BatchPayment']
assert not any('max_fee' in v['payload'] for v in b)
names = {v['name'] for v in frozen['base_vectors']}
assert 'BatchPayment_single' in names and 'BatchPayment_multi' in names
single = next(v for v in frozen['base_vectors'] if v['name'] == 'BatchPayment_single')
assert single['signing_hash'] == '0xbdd97785d3d9e286f0ae6d4154b02d072c546bcbd1330d353c89ddaca3d212b0', single['signing_hash']
multi = next(v for v in frozen['base_vectors'] if v['name'] == 'BatchPayment_multi')
assert multi['signing_hash'] == '0xabc717c970472db933aff88bd2bee41d6e7c3c42dce74b9b3ac70f454e2e5f08', multi['signing_hash']
print('BatchPayment extended vectors:', len(b))
print('ok')
"
```

Expected: prints the 40-char SHA, the BatchPayment vector count, and `ok`. At this point `go build ./...` still passes but `go test ./...` does **not** — the Go package is mid-re-baseline until Task 5 completes. That red window is expected and is confined to the Go PR's working tree.

---

# Phase B — Go SDK re-baseline

**All Phase B tasks run in `/Users/nsh/workspace/1money/layer1/1money-protocol-go-sdk` on one branch, shipping as a single PR that must not merge before the Phase A PR.**

### Task 4: Remove `max_fee` from the BatchPayment read DTO

**Files:**
- Modify: `transactions_payloads.go:34-41`
- Test: `transactions_test.go:67-95`, `transactions_test.go:412-426`

**Interfaces:**
- Consumes: nothing. This task is independent of the fixture gate and may be done while the Phase A PR is in review.
- Produces: `BatchPaymentData` with fields `Token *common.Address`, `Operations []BatchPaymentOperationData`, `OperationsHash *string`, `BatchID *string`, `CreatedAt uint64` — and no `MaxFee`. `Transaction.AsBatchPaymentData()` keeps its existing `(BatchPaymentData, bool)` shape.

- [ ] **Step 1: Delete the field from the production struct (this is the failing test)**

In `transactions_payloads.go`, `BatchPaymentData` currently reads:

```go
type BatchPaymentData struct {
	Token          *common.Address             `json:"token"`
	MaxFee         string                      `json:"max_fee"`
	Operations     []BatchPaymentOperationData `json:"operations"`
	OperationsHash *string                     `json:"operations_hash"`
	BatchID        *string                     `json:"batch_id"`
	CreatedAt      uint64                      `json:"created_at"`
}
```

Delete the `MaxFee` line:

```go
type BatchPaymentData struct {
	Token          *common.Address             `json:"token"`
	Operations     []BatchPaymentOperationData `json:"operations"`
	OperationsHash *string                     `json:"operations_hash"`
	BatchID        *string                     `json:"batch_id"`
	CreatedAt      uint64                      `json:"created_at"`
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./... -run 'TestTransaction|TestBatchPayment_Unmarshal' 2>&1 | head -20`
Expected: FAIL — compile error `payload.MaxFee undefined (type BatchPaymentData has no field or method MaxFee)` at `transactions_test.go:88`.

- [ ] **Step 3: Update the decode fixtures to the node's current response shape**

In `transactions_test.go`, the `"BatchPayment"` case's `data` literal currently contains a `"max_fee"` key. Remove that line so the JSON matches what `l1client` emits after `7ad79889`:

```go
			data: `{
                "token": "0x1111111111111111111111111111111111111111",
                "operations": [
                    {"recipient": "0x2222222222222222222222222222222222222222", "amount": "1000"}
                ],
                "operations_hash": "0x3333333333333333333333333333333333333333333333333333333333333333",
                "batch_id": "payroll-1",
                "created_at": 1747785600
            }`,
```

Delete this assertion from the same case's `assert` func:

```go
				assert.Equal(t, "5000", payload.MaxFee)
```

And in `TestBatchPayment_Unmarshal_NullOptionals`, drop `"max_fee":"0",` from the raw literal:

```go
	raw := `{"transaction_type":"BatchPayment","data":{"token":null,"operations":[],"operations_hash":null,"batch_id":null,"created_at":0}}`
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./... -run 'TestTransaction|TestBatchPayment_Unmarshal' -v 2>&1 | tail -20`
Expected: PASS.

- [ ] **Step 5: Confirm no `max_fee` remains on the read side**

Run: `grep -n 'MaxFee\|max_fee' transactions_payloads.go transactions_test.go`
Expected: no output.

- [ ] **Step 6: Commit**

```bash
gofumpt -l -w .
git add transactions_payloads.go transactions_test.go
git commit -m "feat!: drop max_fee from BatchPayment read DTO"
```

---

### Task 5: Core re-baseline — remove `max_fee`, wrap BatchPayment in `WithMemo`, retire `memoCapable`

**Blocked by:** the GATE above. Both fixtures must already be in `testdata/`.

**Files:**
- Modify: `transactions_types.go:213-224` (`BatchPaymentPayload`)
- Modify: `native_v2_encoding.go:101-120` (`rlpList`), `native_v2_encoding.go:230-247` (`wireFields`)
- Modify: `native_v2.go:50-54` (delete `encodeBare`)
- Modify: `native_v2_prepare.go:39-69` (`validatePayloadU256`), `:76-135` (`resolvePayloadOp`), `:143-152` (`PreparedTransaction`), `:166-203` (`prepareFromPayload`, `newPrepared`), `:231-239` (`Authorize`)
- Modify: `native_v2_requests.go:30-45` (`nativeV2Op`, `payloadRLP`), `:159-172` (`opFromPayload`)
- Modify: `memo.go:27-31` (`WithMemo` doc)
- Modify: `transactions.go:82-87` (`BatchPayment` doc)
- Test: `native_v2_batch_test.go`, `native_v2_conformance_test.go:130,145`, `native_v2_prepare_test.go:228-238,353-362,546-560,986-999`, `native_v2_wire_test.go:64-78`, `api_v2_test.go:36-53,211-218`

**Interfaces:**
- Consumes: `testdata/prepare-authorize-hash-vectors.json` and `testdata/native-v2-signing-vectors.json` from the GATE.
- Produces:
  - `BatchPaymentPayload{ChainID uint64; Nonce uint64; Token common.Address; Operations []PaymentOperation; CreatedAt uint64; OperationsHash *common.Hash; BatchID *string}`
  - `resolvePayloadOp(payload any, cfg submitConfig) (op nativeOperationType, payloadList []interface{}, bodyFields map[string]interface{}, err error)` — four returns, no `memoCapable`
  - `nativeV2Op{op, payloadList, fields, pathV1, pathV2}` — no `memoCapable`
  - `(op nativeV2Op) payloadRLP(memo Memo) ([]byte, error)` — unconditionally `encodeWithMemo`
  - `encodeBare` no longer exists anywhere in the package
  - `PrepareTransaction(batchPayload, WithMemo(m))` succeeds and produces a different signing hash than the same payload without `WithMemo`

- [ ] **Step 1: Run the suite to see the fixture-driven failure**

Run: `go test ./... 2>&1 | head -40`
Expected: FAIL. The regenerated fixture has no `max_fee` key, so `batchFixturePayload.MaxFee` decodes to `""` and `parseFixtureBig(t, "")` aborts in `native_v2_prepare_test.go`; BatchPayment signing hashes also no longer match because the fixture is now `WithMemo`-wrapped. Record the failure output — it is this task's red state.

- [ ] **Step 2: Remove `MaxFee` from the public payload**

In `transactions_types.go`:

```go
// BatchPaymentPayload pays many recipients of one token in a single
// transaction. operations_hash and batch_id are optional trailing fields.
type BatchPaymentPayload struct {
	ChainID        uint64             `json:"chain_id"`
	Nonce          uint64             `json:"nonce"`
	Token          common.Address     `json:"token"`
	Operations     []PaymentOperation `json:"operations"`
	CreatedAt      uint64             `json:"created_at"`
	OperationsHash *common.Hash       `json:"operations_hash,omitempty"`
	BatchID        *string            `json:"batch_id,omitempty"`
}
```

- [ ] **Step 3: Drop `max_fee` from the canonical RLP list**

In `native_v2_encoding.go`, `rlpList` currently builds `list` with `bigOrZero(p.MaxFee)` in the fifth slot. Replace that one line:

```go
func (p BatchPaymentPayload) rlpList() []interface{} {
	ops := make([]interface{}, 0, len(p.Operations))
	for _, o := range p.Operations {
		ops = append(ops, []interface{}{o.Recipient, bigOrZero(o.Amount)})
	}
	list := []interface{}{p.ChainID, p.Nonce, p.Token, ops, p.CreatedAt}
	// Trailing optional fields (native-v2-signing-spec §4.3): only appended when
	// present; an absent field before a present one becomes an empty placeholder.
	if p.OperationsHash != nil || p.BatchID != nil {
		if p.OperationsHash != nil {
			list = append(list, p.OperationsHash.Bytes())
		} else {
			list = append(list, []byte{})
		}
		if p.BatchID != nil {
			list = append(list, []byte(*p.BatchID))
		}
	}
	return list
}
```

- [ ] **Step 4: Drop `max_fee` from the flattened JSON body**

In `native_v2_encoding.go`, `wireFields` for BatchPayment:

```go
func (p BatchPaymentPayload) wireFields() map[string]interface{} {
	ops := make([]map[string]interface{}, 0, len(p.Operations))
	for _, o := range p.Operations {
		ops = append(ops, map[string]interface{}{"recipient": o.Recipient, "amount": bigStr(o.Amount)})
	}
	body := map[string]interface{}{
		"chain_id": p.ChainID, "nonce": p.Nonce, "token": p.Token, "operations": ops,
		"created_at": p.CreatedAt,
	}
	if p.OperationsHash != nil {
		// Store a value copy so the body never aliases the caller's pointer.
		body["operations_hash"] = *p.OperationsHash
	}
	if p.BatchID != nil {
		body["batch_id"] = *p.BatchID
	}
	return body
}
```

- [ ] **Step 5: Remove the `batch.max_fee` validation branch**

In `native_v2_prepare.go`, `validatePayloadU256`'s BatchPayment case:

```go
	case BatchPaymentPayload:
		for index, operation := range p.Operations {
			if err := validateU256(fmt.Sprintf("batch.operations[%d].amount", index), operation.Amount); err != nil {
				return err
			}
		}
```

- [ ] **Step 6: Drop `memoCapable` from `resolvePayloadOp`**

In `native_v2_prepare.go`, change the doc bullet and the signature, and drop the fourth return value from every arm. The signature becomes:

```go
func resolvePayloadOp(payload any, cfg submitConfig) (op nativeOperationType, payloadList []interface{}, bodyFields map[string]interface{}, err error) {
	if err := validatePayloadU256(payload); err != nil {
		return 0, nil, nil, err
	}
	switch p := payload.(type) {
	case PaymentPayload:
		return opPayment, p.rlpList(), p.wireFields(), nil
	case BatchPaymentPayload:
		return opBatchPayment, p.rlpList(), p.wireFields(), nil
	case TokenIssuePayload:
		return opTokenIssue, p.rlpList(), p.wireFields(), nil
	case TokenMintPayload:
		return opTokenMint, p.rlpList(), p.wireFields(), nil
	case TokenBurnPayload:
		return opTokenBurn, p.rlpList(), p.wireFields(), nil
	case TokenBridgeAndMintPayload:
		return opTokenBridgeAndMint, p.rlpList(), p.wireFields(), nil
	case TokenBurnAndBridgePayload:
		return opTokenBurnAndBridge, p.rlpList(), p.wireFields(), nil
	case TokenAuthorityPayload:
		return opTokenAuthority, p.rlpList(), p.wireFields(), nil
	case TokenClawbackPayload:
		return opTokenClawback, p.rlpList(), p.wireFields(), nil
	case PauseTokenPayload:
		return opTokenPause, p.rlpList(), p.wireFields(), nil
	case UpdateMetadataPayload:
		return opTokenMetadata, p.rlpList(), p.wireFields(), nil
	case CreateMultiSigPayload:
		if err := validateMultisigConfig(p.Signers, p.Threshold); err != nil {
			return 0, nil, nil, err
		}
		return opCreateMultiSig, p.rlpList(), p.wireFields(), nil
	case TokenManageListPayload:
		if cfg.listKind == nil {
			return 0, nil, nil, fmt.Errorf("TokenManageListPayload is ambiguous: pass WithManageListKind(ManageListBlacklist or ManageListWhitelist)")
		}
		// The operation type is part of the signing domain, so an unknown kind
		// must error rather than silently map to blacklist.
		switch *cfg.listKind {
		case ManageListBlacklist:
			return opTokenBlacklist, p.rlpList(), p.wireFields(), nil
		case ManageListWhitelist:
			return opTokenWhitelist, p.rlpList(), p.wireFields(), nil
		default:
			return 0, nil, nil, fmt.Errorf("invalid ManageListKind %d", *cfg.listKind)
		}
	default:
		return 0, nil, nil, fmt.Errorf("unsupported payload type %T", payload)
	}
}
```

Also delete the now-false doc bullet above it:

```go
//   - memoCapable: whether the operation carries a memo (false only for
//     BatchPayment).
```

- [ ] **Step 7: Drop `memoCapable` from `nativeV2Op` and make `payloadRLP` unconditional**

In `native_v2_requests.go`:

```go
// nativeV2Op carries everything the submit core needs for one operation.
type nativeV2Op struct {
	op          nativeOperationType
	payloadList []interface{}          // canonical RLP field list (native_v2_encoding.go)
	fields      map[string]interface{} // flattened JSON body fields (native_v2_encoding.go)
	pathV1      string
	pathV2      string
}

// payloadRLP builds payload_rlp for the canonical native-v2 form. All fourteen
// operations are WithMemo<Payload>; there is no bare-payload alternative.
func (op nativeV2Op) payloadRLP(memo Memo) ([]byte, error) {
	return encodeWithMemo(op.payloadList, memo)
}
```

And `opFromPayload`:

```go
func opFromPayload(payload any, cfg submitConfig) (nativeV2Op, error) {
	op, list, fields, err := resolvePayloadOp(payload, cfg)
	if err != nil {
		return nativeV2Op{}, err
	}
	v1, v2 := pathsForOp(op)
	return nativeV2Op{
		op:          op,
		payloadList: list,
		fields:      fields,
		pathV1:      v1,
		pathV2:      v2,
	}, nil
}
```

- [ ] **Step 8: Delete the production `encodeBare` helper**

Remove from `native_v2.go`:

```go
// encodeBare builds payload_rlp = rlp(payloadList) for an operation with no
// memo wrapper (BatchPayment).
func encodeBare(payloadList []interface{}) ([]byte, error) {
	return rlp.EncodeToBytes(payloadList)
}
```

`buildLegacyV1Body` calls `rlp.EncodeToBytes(op.payloadList)` directly and is unaffected. The `rlp` import in `native_v2.go` is still needed by `encodeWithMemo`.

- [ ] **Step 9: Drop the memo guard and `memoCapable` from the prepare pipeline**

In `native_v2_prepare.go`:

```go
type PreparedTransaction struct {
	op          nativeOperationType
	descriptor  []interface{}
	payloadRLP  []byte
	signingHash []byte
	fields      map[string]interface{}
	memo        Memo
	pathV2      string
}
```

```go
// prepareFromPayload resolves a payload to its operation and builds the
// PreparedTransaction. It is the single payload -> prepared path, shared by the
// public PrepareTransaction (offline) and the namespace submit path, so both run
// on exactly one pipeline. Every canonical native-v2 operation carries a memo,
// so there is no memo-capability guard here.
func prepareFromPayload(payload any, cfg submitConfig) (*PreparedTransaction, error) {
	op, err := opFromPayload(payload, cfg)
	if err != nil {
		return nil, err
	}
	return newPrepared(op, cfg.memo)
}
```

```go
	return &PreparedTransaction{
		op:          op.op,
		descriptor:  descriptor,
		payloadRLP:  payloadRLP,
		signingHash: sh,
		fields:      op.fields,
		memo:        memo,
		pathV2:      op.pathV2,
	}, nil
```

And in `Authorize`, the memo becomes unconditional:

```go
	body := make(map[string]interface{}, len(p.fields)+2)
	for k, v := range p.fields {
		body[k] = v
	}
	body["memo"] = p.memo
	body["authorization"] = singleAuthorization(sig)
```

- [ ] **Step 10: Correct the two doc comments that still say BatchPayment carries no memo**

In `memo.go`, `WithMemo`:

```go
// WithMemo attaches a signed memo to the submitted transaction. Without it, the
// canonical empty memo is used. Submitting a memo in legacy v1 mode is rejected
// rather than silently dropped, so audit data is never lost without notice.
```

In `transactions.go`, `BatchPayment`:

```go
// BatchPayment signs and submits a batch payment. Batch payments are
// memo-bearing like every other canonical v2 operation: pass WithMemo to attach
// one, otherwise the canonical empty memo is signed.
```

- [ ] **Step 11: Update the fixture decoder to the new payload shape**

In `native_v2_prepare_test.go`, drop `MaxFee` from the fixture struct:

```go
type batchFixturePayload struct {
	ChainID        uint64                  `json:"chain_id"`
	Nonce          uint64                  `json:"nonce"`
	Token          string                  `json:"token"`
	Operations     []batchFixtureOperation `json:"operations"`
	CreatedAt      uint64                  `json:"created_at"`
	OperationsHash *string                 `json:"operations_hash"`
	BatchID        *string                 `json:"batch_id"`
}
```

and from the `"BatchPayment"` arm of `goPayload`:

```go
		return BatchPaymentPayload{
			ChainID: raw.ChainID, Nonce: raw.Nonce, Token: parseFixtureAddress(t, raw.Token),
			Operations: operations, CreatedAt: raw.CreatedAt,
			OperationsHash: parseFixtureHashPtr(t, raw.OperationsHash), BatchID: raw.BatchID,
		}, options
```

In the numeric-coverage block, drop the `max_fee` record and the `BatchPayment.max_fee` required field:

```go
			case "BatchPayment":
				raw := decodeFixturePayload[batchFixturePayload](t, vector.Payload)
				record("BatchPayment.created_at", fmt.Sprint(raw.CreatedAt))
				for _, operation := range raw.Operations {
					record("BatchPayment.operations.amount", operation.Amount)
				}
```

```go
		for _, field := range []string{
			"Payment.value", "BatchPayment.operations.amount",
			"TokenMint.value", "TokenAuthority.value", "TokenBurn.value", "TokenClawback.value",
			"TokenBridgeAndMint.value", "TokenBurnAndBridge.value", "TokenBurnAndBridge.escrow_fee",
		} {
```

And delete the whole `{"batch.max_fee", ...}` factory entry from `TestPrepareRejectsOutOfRangeU256`, plus the `MaxFee: big.NewInt(1),` line from the `batch.operations[0].amount` factory:

```go
		{"batch.operations[0].amount", func(value *big.Int) any {
			return BatchPaymentPayload{
				ChainID: 1, Nonce: 1, Token: repeatAddr(2),
				Operations: []PaymentOperation{{Recipient: repeatAddr(1), Amount: value}},
				CreatedAt:  1,
			}
		}},
```

- [ ] **Step 12: Update the remaining `MaxFee` construction sites in tests**

`native_v2_wire_test.go` (`operations_hash` subtest), `api_v2_test.go` (`batchPaymentFixture`), and `native_v2_conformance_test.go` (`BatchPayment_single` case) each construct a `BatchPaymentPayload` with `MaxFee:`. Delete that field from all three. For `api_v2_test.go`:

```go
func batchPaymentFixture() BatchPaymentPayload {
	return BatchPaymentPayload{
		ChainID: 1, Nonce: 1, Token: repeatAddr(0x01),
		Operations: []PaymentOperation{{Recipient: repeatAddr(0x0c), Amount: big.NewInt(1)}},
		CreatedAt:  1,
	}
}
```

Also delete `memoCapable: true,` from `paymentOp` in `api_v2_test.go`.

- [ ] **Step 13: Point the conformance test at the memo wrapper**

In `native_v2_conformance_test.go`, the `BatchPayment_single` case ends with `, nil}` (meaning "encode bare"). Change it to `, &emptyMemo}` so it matches the other thirteen, and drop `MaxFee`:

```go
		{"BatchPayment_single", BatchPaymentPayload{ChainID: fixtureChainID, Nonce: 14, Token: repeatAddr(0x01), Operations: []PaymentOperation{{Recipient: repeatAddr(0x0c), Amount: big.NewInt(1000)}, {Recipient: repeatAddr(0x0d), Amount: big.NewInt(2000)}}, CreatedAt: 1_747_785_600}.rlpList(), &emptyMemo},
```

With no case left carrying a `nil` memo, replace the encode branch:

```go
			got, err := encodeWithMemo(tc.list, *tc.memo)
```

and drop the now-unused `var got []byte` / `var err error` declarations and the `if tc.memo == nil` branch. The `memo *Memo` struct field can stay a pointer — every case now supplies one.

- [ ] **Step 14: Rewrite the batch trailing-option test for the new element counts**

In `native_v2_batch_test.go`, `TestBatchPaymentOptionalTrailingFields`: the fixed field count drops from 6 to 5, so every expected count drops by one and the `operations_hash` slot moves from index 6 to index 5. Also switch `elems` from `encodeBare` to a direct `rlp.EncodeToBytes` of the field list, because the assertion is about the payload list itself, not the memo wrapper:

```go
	base := func() BatchPaymentPayload {
		return BatchPaymentPayload{
			ChainID: 1, Nonce: 1, Token: repeatAddr(0x01),
			Operations: []PaymentOperation{{Recipient: repeatAddr(0x0c), Amount: big.NewInt(1000)}},
			CreatedAt:  1,
		}
	}
```

```go
	// elems returns the top-level RLP elements of a payload's field list.
	elems := func(p BatchPaymentPayload) []rlp.RawValue {
		raw, err := rlp.EncodeToBytes(p.rlpList())
		if err != nil {
			t.Fatal(err)
		}
		var out []rlp.RawValue
		if err := rlp.DecodeBytes(raw, &out); err != nil {
			t.Fatalf("decode payload rlp: %v", err)
		}
		return out
	}

	// Element counts per §4.3 (5 fixed fields + trailing optionals).
	if got := len(elems(neither)); got != 5 {
		t.Errorf("neither: got %d elements, want 5 (no trailing placeholders)", got)
	}
	if got := len(elems(hashOnly)); got != 6 {
		t.Errorf("hash-only: got %d elements, want 6", got)
	}
	if got := len(elems(both)); got != 7 {
		t.Errorf("both: got %d elements, want 7", got)
	}
	idOnlyElems := elems(idOnly)
	if len(idOnlyElems) != 7 {
		t.Fatalf("batch_id-only: got %d elements, want 7 (placeholder + batch_id)", len(idOnlyElems))
	}
	// The operations_hash slot (index 5) must be the 0x80 empty-string placeholder
	// when absent-before-present, not dropped or zero-filled.
	if !bytes.Equal(idOnlyElems[5], []byte{0x80}) {
		t.Errorf("batch_id-only: operations_hash slot = %x, want 0x80 placeholder", idOnlyElems[5])
	}
```

- [ ] **Step 15: Replace the fee dimension with the memo dimension in the pairwise coverage test**

In `native_v2_batch_test.go`, `TestBatchPaymentPairwiseGoldenCoverage`: swap `feeLevels` for `memoLevels`, and derive the level from the vector's options rather than from `raw.MaxFee`:

```go
	optionLevels := []string{"neither", "hash_only", "id_only", "both"}
	operationLevels := []string{"empty", "single", "forward", "reverse"}
	amountLevels := []string{"ordinary", "zero", "max"}
	memoLevels := []string{"empty", "populated"}
	cross("option", optionLevels, "operations", operationLevels)
	cross("option", optionLevels, "amount", amountLevels)
	cross("option", optionLevels, "memo", memoLevels)
	cross("operations", operationLevels, "memo", memoLevels)
	cross("operations", operationLevels[1:], "amount", amountLevels)
	cross("amount", amountLevels, "memo", memoLevels)
```

```go
		memoLevel := "empty"
		if vector.Options.Memo != nil {
			memoLevel = "populated"
		}
		observed.add("option:" + optionLevel + "|operations:" + operationLevel)
		observed.add("option:" + optionLevel + "|memo:" + memoLevel)
		observed.add("operations:" + operationLevel + "|memo:" + memoLevel)

		if len(raw.Operations) != 0 {
			amountLevel := "ordinary"
			if raw.Operations[0].Amount == "0" {
				amountLevel = "zero"
			} else if raw.Operations[0].Amount == maxU256 {
				amountLevel = "max"
			}
			observed.add("option:" + optionLevel + "|amount:" + amountLevel)
			observed.add("operations:" + operationLevel + "|amount:" + amountLevel)
			observed.add("amount:" + amountLevel + "|memo:" + memoLevel)
		}
```

`vector.Options` is the existing `prepareAuthorizeVector` options struct in `native_v2_prepare_test.go`; add a `Memo *struct{ Type, Format, Data string } \`json:"memo"\`` field to it if it does not already carry one, matching the generator's `options.memo` object from Phase A Task 2 Step 1. Delete the now-unused `maxU256` fee comparison and the `strings` import if it becomes unused.

- [ ] **Step 16: Invert the two "BatchPayment rejects memo" tests**

Replace `TestPrepareTransactionRejectsBatchMemo` in `native_v2_batch_test.go`:

```go
// TestBatchPaymentAcceptsMemo verifies BatchPayment is memo-bearing like every
// other canonical v2 operation: a memo prepares successfully and changes the
// signing hash, because the memo is inside WithMemo<BatchPaymentPayload>.
func TestBatchPaymentAcceptsMemo(t *testing.T) {
	batch := BatchPaymentPayload{
		ChainID: 1, Nonce: 1, Token: repeatAddr(0x01),
		Operations: []PaymentOperation{{Recipient: repeatAddr(0x0c), Amount: big.NewInt(1000)}},
		CreatedAt:  1,
	}

	bare, err := PrepareTransaction(batch)
	if err != nil {
		t.Fatalf("PrepareTransaction(batch) without memo: %v", err)
	}
	withMemo, err := PrepareTransaction(batch, WithMemo(Memo{Type: "purpose/SALA", Format: "text/plain", Data: "x"}))
	if err != nil {
		t.Fatalf("PrepareTransaction(batch) with memo: %v", err)
	}
	if bytes.Equal(bare.SigningHash(), withMemo.SigningHash()) {
		t.Error("memo did not change the batch signing hash; the memo is not in the signed preimage")
	}
}
```

Replace `TestBatchPaymentRejectsMemo` in `api_v2_test.go` with a body-level assertion:

```go
// TestBatchPaymentSubmitsMemo asserts the batch v2 body always carries the
// three-field memo object, which the L1 BatchPaymentRequestV2 requires.
func TestBatchPaymentSubmitsMemo(t *testing.T) {
	memo := Memo{Type: "purpose/PAYROLL", Format: "text/plain", Data: "may-2026"}
	var gotMemo json.RawMessage
	var gotPath string
	hc := fakeHTTPClient(nil, func(path string, body map[string]json.RawMessage) interface{} {
		gotPath, gotMemo = path, body["memo"]
		return map[string]string{"hash": v2HashFromBody(body, opBatchPayment, batchPaymentFixture().rlpList(), memo)}
	})
	c := NewClientWithCustomUrl("http://sdk.test", WithHTTPClient(hc))
	if _, err := c.Transactions().BatchPayment(context.Background(), batchPaymentFixture(), testSigner(t), WithMemo(memo)); err != nil {
		t.Fatalf("batch payment with memo should succeed: %v", err)
	}
	if gotPath != "/v2/transactions/batch_payment" {
		t.Errorf("path = %q, want /v2/transactions/batch_payment", gotPath)
	}
	if string(gotMemo) != `{"type":"purpose/PAYROLL","format":"text/plain","data":"may-2026"}` {
		t.Errorf("memo = %s, want the full three-field object", gotMemo)
	}
}
```

- [ ] **Step 17: Run the full suite to verify it passes**

Run: `go test ./... 2>&1 | tail -30`
Expected: PASS.

- [ ] **Step 18: Confirm the re-baseline invariants hold**

```bash
grep -rn 'MaxFee\|max_fee' --include='*.go' . ; echo "--- (expect no output above) ---"
grep -rn 'memoCapable\|encodeBare' --include='*.go' . ; echo "--- (expect no output above) ---"
```

Expected: both greps silent.

- [ ] **Step 19: Commit**

```bash
gofumpt -l -w .
golangci-lint run
git add -A
git commit -m "feat!: re-baseline BatchPayment on WithMemo and drop max_fee"
```

---

### Task 6: Extract the shared operation encoders and add `DeriveBatchPaymentOperationsHash`

**Blocked by:** Task 5.

**Files:**
- Create: `native_v2_batch.go`
- Modify: `native_v2_encoding.go` (BatchPayment `rlpList` / `wireFields` delegate to the new helpers)
- Test: `native_v2_batch_test.go`, `native_v2_prepare_test.go` (fixture `expected` struct)

**Interfaces:**
- Consumes: `bigOrZero`, `bigStr`, `validateU256` (existing package helpers); the fixture's new `expected.operations_hash` from Phase A Task 2 Step 4.
- Produces:
  - `batchOperationsRLPList(operations []PaymentOperation) []interface{}`
  - `batchOperationsWireList(operations []PaymentOperation) []map[string]interface{}`
  - `DeriveBatchPaymentOperationsHash(operations []PaymentOperation) (common.Hash, error)`

  Task 8 consumes `batchOperationsWireList`.

- [ ] **Step 1: Write the failing oracle-backed test**

First add the field to the fixture's expected struct in `native_v2_prepare_test.go` (alongside the existing `SigningHash` / `TransactionHash` fields):

```go
	OperationsHash string `json:"operations_hash"`
```

Then append to `native_v2_batch_test.go`:

```go
// TestDeriveBatchPaymentOperationsHashMatchesRustOracle checks the exported
// derivation against expected.operations_hash emitted by the L1 generator. A
// Go-computed hash compared only with another Go-computed value is not an
// acceptable oracle.
func TestDeriveBatchPaymentOperationsHashMatchesRustOracle(t *testing.T) {
	covered := 0
	for _, vector := range loadPrepareAuthorizeFixture(t).Vectors {
		if vector.Operation != "BatchPayment" || vector.Expected.OperationsHash == "" {
			continue
		}
		vector := vector
		t.Run(vector.Name, func(t *testing.T) {
			payload, _ := vector.goPayload(t)
			batch, ok := payload.(BatchPaymentPayload)
			if !ok {
				t.Fatalf("decoded payload type = %T, want BatchPaymentPayload", payload)
			}
			got, err := DeriveBatchPaymentOperationsHash(batch.Operations)
			if err != nil {
				t.Fatalf("derive: %v", err)
			}
			if strings.ToLower(got.Hex()) != strings.ToLower(vector.Expected.OperationsHash) {
				t.Fatalf("operations_hash\n got %s\nwant %s (Rust oracle)", got.Hex(), vector.Expected.OperationsHash)
			}
		})
		covered++
	}
	if covered == 0 {
		t.Fatal("no BatchPayment vector carried expected.operations_hash; regenerate the fixture from the L1 oracle")
	}
}

// TestDeriveBatchPaymentOperationsHashNilAmount pins the nil == U256-zero rule
// that the submit encoder already applies, so the helper hashes exactly what the
// submit path signs.
func TestDeriveBatchPaymentOperationsHashNilAmount(t *testing.T) {
	nilAmount, err := DeriveBatchPaymentOperationsHash([]PaymentOperation{{Recipient: repeatAddr(0x0c), Amount: nil}})
	if err != nil {
		t.Fatalf("nil amount must not error: %v", err)
	}
	zeroAmount, err := DeriveBatchPaymentOperationsHash([]PaymentOperation{{Recipient: repeatAddr(0x0c), Amount: big.NewInt(0)}})
	if err != nil {
		t.Fatalf("zero amount: %v", err)
	}
	if nilAmount != zeroAmount {
		t.Errorf("nil amount hash %s != zero amount hash %s", nilAmount.Hex(), zeroAmount.Hex())
	}
}

// TestDeriveBatchPaymentOperationsHashRejectsOutOfRange checks the same U256
// bounds the submit path enforces.
func TestDeriveBatchPaymentOperationsHashRejectsOutOfRange(t *testing.T) {
	tooWide := new(big.Int).Lsh(big.NewInt(1), 256)
	for name, amount := range map[string]*big.Int{
		"negative": big.NewInt(-1),
		"too wide": tooWide,
	} {
		if _, err := DeriveBatchPaymentOperationsHash([]PaymentOperation{{Recipient: repeatAddr(0x0c), Amount: amount}}); err == nil {
			t.Errorf("%s amount was accepted; want an error", name)
		}
	}
}

// TestDeriveBatchPaymentOperationsHashIsOrderSensitive guards against a helper
// that sorts or normalizes operations: L1 hashes the list as given.
func TestDeriveBatchPaymentOperationsHashIsOrderSensitive(t *testing.T) {
	forward := []PaymentOperation{
		{Recipient: repeatAddr(0x0c), Amount: big.NewInt(1000)},
		{Recipient: repeatAddr(0x0d), Amount: big.NewInt(2000)},
	}
	reverse := []PaymentOperation{forward[1], forward[0]}
	a, err := DeriveBatchPaymentOperationsHash(forward)
	if err != nil {
		t.Fatal(err)
	}
	b, err := DeriveBatchPaymentOperationsHash(reverse)
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Error("reordering operations must change the hash")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./... -run TestDeriveBatchPaymentOperationsHash 2>&1 | head -10`
Expected: FAIL with `undefined: DeriveBatchPaymentOperationsHash`.

- [ ] **Step 3: Create `native_v2_batch.go`**

```go
package onemoney

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

// batchOperationsRLPList encodes a batch payment's operations as the canonical
// RLP element list: one [recipient, amount] pair per operation
// (native-v2-signing-spec §4.2). It is the single source of the operation
// encoding, shared by BatchPaymentPayload.rlpList (the signed preimage) and
// DeriveBatchPaymentOperationsHash, so the exported derivation can never drift
// from what the submit path signs. A nil Amount encodes as U256 zero.
func batchOperationsRLPList(operations []PaymentOperation) []interface{} {
	out := make([]interface{}, 0, len(operations))
	for _, operation := range operations {
		out = append(out, []interface{}{operation.Recipient, bigOrZero(operation.Amount)})
	}
	return out
}

// batchOperationsWireList renders a batch payment's operations as JSON body
// elements. Amounts become quoted decimal strings because marshalling *big.Int
// directly emits a bare JSON number. Shared by BatchPaymentPayload.wireFields
// (the v2 submit body) and BatchPaymentFeeEstimateRequest.MarshalJSON, so both
// requests carry one amount representation.
func batchOperationsWireList(operations []PaymentOperation) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(operations))
	for _, operation := range operations {
		out = append(out, map[string]interface{}{
			"recipient": operation.Recipient,
			"amount":    bigStr(operation.Amount),
		})
	}
	return out
}

// validateBatchOperationAmounts applies the U256 bounds the submit path applies,
// so the exported derivation rejects exactly what signing would reject.
func validateBatchOperationAmounts(operations []PaymentOperation) error {
	for index, operation := range operations {
		if err := validateU256(fmt.Sprintf("batch.operations[%d].amount", index), operation.Amount); err != nil {
			return err
		}
	}
	return nil
}

// DeriveBatchPaymentOperationsHash computes the canonical operations hash for a
// batch payment's operation list. It is byte-for-byte identical to the node's
// BatchPaymentPayload::canonical_operations_hash — keccak256 of the RLP-encoded
// operation list — and is a pure function of the operations, so callers can fill
// BatchPaymentPayload.OperationsHash themselves. The node re-derives and compares
// the value whenever the field is present, and rejects the transaction on a
// mismatch.
//
// Leaving OperationsHash nil is always valid; the field exists so a system that
// defines a batch can publish its operation set independently of the system that
// builds and signs the transaction.
//
// A nil PaymentOperation.Amount is treated as U256 zero, matching the submit
// encoder, so the returned hash always covers exactly the bytes the submit path
// would sign. That does not make the operation admission-valid: the node rejects
// batch operations whose amount is zero. Negative amounts and values wider than
// 256 bits return an error. Operation order is significant and is never
// normalized.
func DeriveBatchPaymentOperationsHash(operations []PaymentOperation) (common.Hash, error) {
	if err := validateBatchOperationAmounts(operations); err != nil {
		return common.Hash{}, err
	}
	encoded, err := rlp.EncodeToBytes(batchOperationsRLPList(operations))
	if err != nil {
		return common.Hash{}, fmt.Errorf("rlp encode batch operations: %w", err)
	}
	return common.BytesToHash(crypto.Keccak256(encoded)), nil
}
```

- [ ] **Step 4: Delegate the two BatchPayment encoders to the shared helpers**

In `native_v2_encoding.go`, replace the inline operation loops:

```go
func (p BatchPaymentPayload) rlpList() []interface{} {
	list := []interface{}{p.ChainID, p.Nonce, p.Token, batchOperationsRLPList(p.Operations), p.CreatedAt}
	// Trailing optional fields (native-v2-signing-spec §4.3): only appended when
	// present; an absent field before a present one becomes an empty placeholder.
	if p.OperationsHash != nil || p.BatchID != nil {
		if p.OperationsHash != nil {
			list = append(list, p.OperationsHash.Bytes())
		} else {
			list = append(list, []byte{})
		}
		if p.BatchID != nil {
			list = append(list, []byte(*p.BatchID))
		}
	}
	return list
}
```

```go
func (p BatchPaymentPayload) wireFields() map[string]interface{} {
	body := map[string]interface{}{
		"chain_id": p.ChainID, "nonce": p.Nonce, "token": p.Token,
		"operations": batchOperationsWireList(p.Operations), "created_at": p.CreatedAt,
	}
	if p.OperationsHash != nil {
		// Store a value copy so the body never aliases the caller's pointer.
		body["operations_hash"] = *p.OperationsHash
	}
	if p.BatchID != nil {
		body["batch_id"] = *p.BatchID
	}
	return body
}
```

- [ ] **Step 5: Also reuse the shared validator in `validatePayloadU256`**

In `native_v2_prepare.go`, the BatchPayment case becomes a one-liner, so the submit path and the exported helper cannot diverge:

```go
	case BatchPaymentPayload:
		return validateBatchOperationAmounts(p.Operations)
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test ./... -run TestDeriveBatchPaymentOperationsHash -v 2>&1 | tail -20`
Expected: PASS, with the oracle subtest running once per BatchPayment vector.

- [ ] **Step 7: Run the full suite to confirm the delegation changed no bytes**

Run: `go test ./... 2>&1 | tail -10`
Expected: PASS — the conformance and golden-vector tests still match, proving the extraction was behavior-preserving.

- [ ] **Step 8: Re-pin the public-API surface hash**

`compatibility_test.go`'s `TestPublicAPICompatibility` sha256s `go doc -all .` against a pinned baseline. This task adds an exported symbol (`DeriveBatchPaymentOperationsHash`), so the baseline must be re-pinned or the suite stays red.

```bash
go test ./... -run TestPublicAPICompatibility 2>&1 | tail -5
```

Take the reported `public API hash = <got>` value, confirm by diffing `go doc -all .` against the previous surface that the ONLY delta is your new exported function, then update `publicAPIHash` in `compatibility_test.go` to `<got>` and extend the explanatory comment above it to name this change. Never re-pin without first verifying the delta — the hash exists to catch unintended surface drift.

- [ ] **Step 9: Commit**

```bash
gofumpt -l -w .
git add native_v2_batch.go native_v2_encoding.go native_v2_prepare.go native_v2_batch_test.go native_v2_prepare_test.go compatibility_test.go
git commit -m "feat: add DeriveBatchPaymentOperationsHash with shared operation encoders"
```

---

### Task 7: Make BatchPayment v2-only with a generic capability check

**Blocked by:** Task 5.

**Files:**
- Modify: `native_v2.go` (add `nativeOperationType.label`)
- Modify: `native_v2_requests.go:120-155` (`pathsForOp`), `:176-200` (`submitPayload`)
- Test: `api_v2_test.go`

**Interfaces:**
- Consumes: `nativeV2Op.pathV1` from Task 5.
- Produces:
  - `pathsForOp(opBatchPayment)` returns `("", "/v2/transactions/batch_payment")`
  - `(op nativeOperationType) label() string` — `"batch payment"`, `"create multisig"`, else `"native operation N"`
  - Legacy-mode submission of any operation with an empty v1 path returns `"<label> requires domain-separated v2 submission mode"` before signing and before HTTP I/O

- [ ] **Step 1: Write the failing tests**

Append to `api_v2_test.go`:

```go
// TestLegacyModeRejectsV2OnlyOperations pins the generic capability check: an
// operation with no v1 path fails before signing and before any HTTP request,
// with a stable v2-only error. BatchPayment is v2-only by design; CreateMultisig
// already had an empty v1 path and previously fell through to POST it.
func TestLegacyModeRejectsV2OnlyOperations(t *testing.T) {
	newLegacyClient := func(t *testing.T, requests *int) *Client {
		t.Helper()
		hc := fakeHTTPClient(nil, func(_ string, _ map[string]json.RawMessage) interface{} {
			*requests++
			return map[string]string{"hash": "0x00"}
		})
		return NewClientWithCustomUrl("http://sdk.test", WithHTTPClient(hc), WithLegacyV1())
	}

	t.Run("batch payment default memo", func(t *testing.T) {
		requests := 0
		c := newLegacyClient(t, &requests)
		_, err := c.Transactions().BatchPayment(context.Background(), batchPaymentFixture(), testSigner(t))
		if err == nil || !strings.Contains(err.Error(), "batch payment requires domain-separated v2 submission mode") {
			t.Fatalf("err = %v, want the batch payment v2-only error", err)
		}
		if requests != 0 {
			t.Errorf("issued %d HTTP requests, want 0", requests)
		}
	})

	t.Run("batch payment explicit memo", func(t *testing.T) {
		requests := 0
		c := newLegacyClient(t, &requests)
		_, err := c.Transactions().BatchPayment(context.Background(), batchPaymentFixture(), testSigner(t), WithMemo(Memo{Type: "purpose/SALA"}))
		if err == nil || !strings.Contains(err.Error(), "batch payment requires domain-separated v2 submission mode") {
			t.Fatalf("err = %v, want the v2-only error, not the generic legacy-memo error", err)
		}
		if requests != 0 {
			t.Errorf("issued %d HTTP requests, want 0", requests)
		}
	})

	// CreateMultisig already has its own namespace-level v2-only guard at
	// accounts.go:97-99 with a more specific message, and it returns before
	// reaching submitPayload. That guard stays — this subtest pins its behavior
	// rather than replacing it.
	t.Run("create multisig namespace guard", func(t *testing.T) {
		requests := 0
		c := newLegacyClient(t, &requests)
		payload := CreateMultiSigPayload{
			ChainID: 1, Nonce: 1,
			Signers:   []MultiSigSigner{{PublicKey: validPubkey(t, 2), Weight: 1}},
			Threshold: 1,
		}
		_, err := c.Accounts().CreateMultisig(context.Background(), payload, testSigner(t))
		if err == nil || !strings.Contains(err.Error(), "has no legacy v1 endpoint") {
			t.Fatalf("err = %v, want the existing multisig v2-only error", err)
		}
		if requests != 0 {
			t.Errorf("issued %d HTTP requests, want 0", requests)
		}
	})

	// The generic capability check is what protects a v2-only operation that has
	// no namespace-level guard. Exercise it through the unexported submit core,
	// which is the only path that reaches it for CreateMultisig.
	t.Run("create multisig generic capability check", func(t *testing.T) {
		requests := 0
		c := newLegacyClient(t, &requests)
		payload := CreateMultiSigPayload{
			ChainID: 1, Nonce: 1,
			Signers:   []MultiSigSigner{{PublicKey: validPubkey(t, 2), Weight: 1}},
			Threshold: 1,
		}
		out := new(CreateMultisigResponse)
		err := c.submitPayload(context.Background(), payload, resolveSubmit(nil), testSigner(t), out)
		if err == nil || !strings.Contains(err.Error(), "create multisig requires domain-separated v2 submission mode") {
			t.Fatalf("err = %v, want the generic v2-only capability error", err)
		}
		if requests != 0 {
			t.Errorf("issued %d HTTP requests, want 0", requests)
		}
	})
}

// TestLegacyModeResolvesOperationBeforeMemoGuard documents the intentional
// error-precedence change from moving operation resolution ahead of the legacy
// memo guard: an ambiguous v1-capable operation now reports the more specific
// resolution error rather than the generic legacy-memo error.
//
// This goes through the unexported submitPayload rather than a namespace method
// because Tokens().ManageBlacklist / ManageWhitelist always inject cfg.listKind
// (tokens.go:196-199, 206-209), so an ambiguous TokenManageListPayload is
// unreachable through the public namespace API.
func TestLegacyModeResolvesOperationBeforeMemoGuard(t *testing.T) {
	requests := 0
	hc := fakeHTTPClient(nil, func(_ string, _ map[string]json.RawMessage) interface{} {
		requests++
		return map[string]string{"hash": "0x00"}
	})
	c := NewClientWithCustomUrl("http://sdk.test", WithHTTPClient(hc), WithLegacyV1())
	payload := TokenManageListPayload{ChainID: 1, Nonce: 1, Action: ManageListActionAdd, Address: repeatAddr(0x06), Token: repeatAddr(0x01)}
	// No listKind, plus an explicit memo: resolution must fail first.
	cfg := resolveSubmit([]SubmitOption{WithMemo(Memo{Type: "purpose/SALA"})})
	out := new(SetTokenManageListResponse)
	err := c.submitPayload(context.Background(), payload, cfg, testSigner(t), out)
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("err = %v, want the ambiguous-operation error before the legacy-memo error", err)
	}
	if requests != 0 {
		t.Errorf("issued %d HTTP requests, want 0", requests)
	}
}
```

`Accounts().CreateMultisig` (`accounts.go:96`), `WithLegacyV1` (`client.go:170`), `NewClientWithCustomUrl` (`client.go:91`), `fakeHTTPClient` (`api_v2_test.go:255`), `testSigner` (`api_v2_test.go:17`), and `validPubkey` (`native_v2_multisig_test.go:16`) all exist with these exact names. Add `strings` to the file's imports if it is not already there.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./... -run 'TestLegacyModeRejectsV2OnlyOperations|TestLegacyModeResolvesOperationBeforeMemoGuard' 2>&1 | head -20`
Expected: FAIL — BatchPayment currently POSTs `/v1/transactions/batch_payment`, and the memo variant reports the generic legacy-memo error.

- [ ] **Step 3: Make BatchPayment v2-only in the path table**

In `native_v2_requests.go`, `pathsForOp`:

```go
	case opBatchPayment:
		return "", "/v2/transactions/batch_payment" // v2-only; the /v1 route is deprecated on L1
```

- [ ] **Step 4: Add the operation label used by the v2-only error**

Append to `native_v2.go`:

```go
// label returns a human-readable operation name for error messages. Only the
// v2-only operations need one today, because they are the only operations that
// can reach the capability error in submitPayload.
func (op nativeOperationType) label() string {
	switch op {
	case opBatchPayment:
		return "batch payment"
	case opCreateMultiSig:
		return "create multisig"
	default:
		return fmt.Sprintf("native operation %d", uint16(op))
	}
}
```

- [ ] **Step 5: Reorder the legacy branch and add the capability check**

In `native_v2_requests.go`, `submitPayload`'s legacy branch becomes:

```go
	// Legacy v1 signs the bare payload and does not use a PreparedTransaction.
	// Resolution runs first so a v2-only operation reports its own capability
	// error rather than being masked by the generic legacy-memo guard.
	if c.mode() == SubmissionModeLegacyV1 {
		op, err := opFromPayload(payload, cfg)
		if err != nil {
			return err
		}
		if op.pathV1 == "" {
			return fmt.Errorf("%s requires domain-separated v2 submission mode", op.op.label())
		}
		if cfg.memoSet {
			return fmt.Errorf("memo is not supported in legacy v1 submission mode; use the default v2 mode to sign a memo")
		}
		return c.submitLegacyV1(ctx, op, signer, out)
	}
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test ./... -run 'TestLegacyModeRejectsV2OnlyOperations|TestLegacyModeResolvesOperationBeforeMemoGuard' -v 2>&1 | tail -20`
Expected: PASS on all four subtests.

- [ ] **Step 7: Run the full suite**

Run: `go test ./... 2>&1 | tail -10`
Expected: PASS. If a pre-existing legacy-mode test asserted the old memo-first precedence, update it to the documented new order rather than restoring the old one.

- [ ] **Step 8: Commit**

```bash
gofumpt -l -w .
golangci-lint run
git add native_v2.go native_v2_requests.go api_v2_test.go
git commit -m "feat!: make BatchPayment v2-only via a generic capability check"
```

---

### Task 8: Add the unsigned batch fee-estimate operation

**Blocked by:** Task 6 (needs `batchOperationsWireList` and `validateBatchOperationAmounts`).

**Files:**
- Modify: `transactions_types.go` (add `BatchPaymentFeeEstimateRequest` + `MarshalJSON`)
- Modify: `transactions.go:11-17` (endpoint constant), and append the client method
- Test: `transactions_test.go`

**Interfaces:**
- Consumes: `batchOperationsWireList`, `validateBatchOperationAmounts` (Task 6); `EstimateFeeResponse`, `Client.PostMethod` (existing).
- Produces:
  - `BatchPaymentFeeEstimateRequest{From common.Address; Token common.Address; Operations []PaymentOperation}` with a value-receiver `MarshalJSON`
  - `(client *Client) GetBatchPaymentEstimateFee(ctx context.Context, request BatchPaymentFeeEstimateRequest) (*EstimateFeeResponse, error)`

- [ ] **Step 1: Write the failing tests**

Append to `transactions_test.go`:

```go
// TestBatchPaymentFeeEstimateRequestMarshalsAsWireBody pins the public request
// type as a correct wire type: lowercase keys and quoted decimal amounts. A bare
// struct marshal would emit "Amount" as an unquoted JSON number.
func TestBatchPaymentFeeEstimateRequestMarshalsAsWireBody(t *testing.T) {
	request := BatchPaymentFeeEstimateRequest{
		From:  common.HexToAddress("0x3333333333333333333333333333333333333333"),
		Token: common.HexToAddress("0x1111111111111111111111111111111111111111"),
		Operations: []PaymentOperation{
			{Recipient: common.HexToAddress("0x2222222222222222222222222222222222222222"), Amount: big.NewInt(100)},
			{Recipient: common.HexToAddress("0x4444444444444444444444444444444444444444"), Amount: nil},
		},
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{"from", "token", "operations"} {
		if _, ok := body[key]; !ok {
			t.Errorf("missing key %q in %s", key, encoded)
		}
	}
	var operations []struct {
		Recipient string `json:"recipient"`
		Amount    string `json:"amount"`
	}
	if err := json.Unmarshal(body["operations"], &operations); err != nil {
		t.Fatalf("operations must decode with quoted decimal amounts: %v (%s)", err, encoded)
	}
	if len(operations) != 2 || operations[0].Amount != "100" || operations[1].Amount != "0" {
		t.Errorf("operations = %+v, want amounts [\"100\", \"0\"] (nil == U256 zero)", operations)
	}
}

// TestGetBatchPaymentEstimateFee asserts the endpoint, the exact request body,
// and that a null plan decodes.
func TestGetBatchPaymentEstimateFee(t *testing.T) {
	request := BatchPaymentFeeEstimateRequest{
		From:  common.HexToAddress("0x3333333333333333333333333333333333333333"),
		Token: common.HexToAddress("0x1111111111111111111111111111111111111111"),
		Operations: []PaymentOperation{
			{Recipient: common.HexToAddress("0x2222222222222222222222222222222222222222"), Amount: big.NewInt(100)},
		},
	}
	wantBody, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}

	var gotPath, gotMethod string
	var gotBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"fee":"1500000000","plan":null}`))
	}))
	defer server.Close()

	c := NewClientWithCustomUrl(server.URL)
	result, err := c.GetBatchPaymentEstimateFee(context.Background(), request)
	if err != nil {
		t.Fatalf("GetBatchPaymentEstimateFee: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", gotMethod)
	}
	if gotPath != "/v1/transactions/batch_payment/estimate_fee" {
		t.Errorf("path = %s, want /v1/transactions/batch_payment/estimate_fee", gotPath)
	}
	if !jsonEqual(t, gotBody, wantBody) {
		t.Errorf("request body\n got %s\nwant %s (must match direct json.Marshal)", gotBody, wantBody)
	}
	if result.Fee != "1500000000" {
		t.Errorf("Fee = %q, want 1500000000", result.Fee)
	}
	if result.Plan != nil {
		t.Errorf("Plan = %v, want nil for a null plan", result.Plan)
	}
}

// TestGetBatchPaymentEstimateFeeRejectsOutOfRangeAmount checks the same U256
// bounds the submit path applies, before any HTTP request.
func TestGetBatchPaymentEstimateFeeRejectsOutOfRangeAmount(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		_, _ = w.Write([]byte(`{"fee":"0"}`))
	}))
	defer server.Close()

	c := NewClientWithCustomUrl(server.URL)
	_, err := c.GetBatchPaymentEstimateFee(context.Background(), BatchPaymentFeeEstimateRequest{
		From:       common.HexToAddress("0x3333333333333333333333333333333333333333"),
		Token:      common.HexToAddress("0x1111111111111111111111111111111111111111"),
		Operations: []PaymentOperation{{Recipient: repeatAddr(0x22), Amount: big.NewInt(-1)}},
	})
	if err == nil {
		t.Fatal("negative amount was accepted; want an error")
	}
	if requests != 0 {
		t.Errorf("issued %d HTTP requests, want 0", requests)
	}
}

// TestBatchOperationEncodingIsSharedBySubmitAndEstimate pins design §9.4: one
// operation serializer feeds both requests, so amounts can never drift into two
// representations.
func TestBatchOperationEncodingIsSharedBySubmitAndEstimate(t *testing.T) {
	operations := []PaymentOperation{
		{Recipient: common.HexToAddress("0x2222222222222222222222222222222222222222"), Amount: big.NewInt(100)},
		{Recipient: common.HexToAddress("0x4444444444444444444444444444444444444444"), Amount: nil},
	}

	submitBody := BatchPaymentPayload{
		ChainID: 1, Nonce: 1, Token: common.HexToAddress("0x1111111111111111111111111111111111111111"),
		Operations: operations, CreatedAt: 1,
	}.wireFields()
	submitOperations, err := json.Marshal(submitBody["operations"])
	if err != nil {
		t.Fatal(err)
	}

	estimate, err := json.Marshal(BatchPaymentFeeEstimateRequest{
		From:       common.HexToAddress("0x3333333333333333333333333333333333333333"),
		Token:      common.HexToAddress("0x1111111111111111111111111111111111111111"),
		Operations: operations,
	})
	if err != nil {
		t.Fatal(err)
	}
	var estimateBody map[string]json.RawMessage
	if err := json.Unmarshal(estimate, &estimateBody); err != nil {
		t.Fatal(err)
	}

	if !jsonEqual(t, submitOperations, estimateBody["operations"]) {
		t.Errorf("operations encoding differs\n submit   = %s\n estimate = %s", submitOperations, estimateBody["operations"])
	}
	if !strings.Contains(string(submitOperations), `"amount":"100"`) {
		t.Errorf("submit amounts must be quoted decimal strings, got %s", submitOperations)
	}
}

// jsonEqual reports whether two JSON documents are semantically equal.
func jsonEqual(t *testing.T, a, b []byte) bool {
	t.Helper()
	var av, bv interface{}
	if err := json.Unmarshal(a, &av); err != nil {
		t.Fatalf("unmarshal a: %v (%s)", err, a)
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		t.Fatalf("unmarshal b: %v (%s)", err, b)
	}
	return reflect.DeepEqual(av, bv)
}
```

Add whatever of `io`, `net/http`, `net/http/httptest`, `reflect`, `strings` the file does not already import. If `transactions_test.go` already has a helper equivalent to `jsonEqual`, use that one instead of adding a duplicate.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./... -run 'TestBatchPaymentFeeEstimateRequest|TestGetBatchPaymentEstimateFee|TestBatchOperationEncodingIsShared' 2>&1 | head -10`
Expected: FAIL with `undefined: BatchPaymentFeeEstimateRequest`.

- [ ] **Step 3: Add the request type and its wire marshaller**

Append to `transactions_types.go` (add `"encoding/json"` to its imports):

```go
// BatchPaymentFeeEstimateRequest is the unsigned input to the batch-payment
// fee-estimate endpoint. It carries no nonce, timestamp, memo, authorization, or
// operations hash: the node cannot validate those from an unsigned request, and
// the returned quote is non-binding.
type BatchPaymentFeeEstimateRequest struct {
	From       common.Address     `json:"from"`
	Token      common.Address     `json:"token"`
	Operations []PaymentOperation `json:"operations"`
}

// MarshalJSON renders the request with the same operation encoder the v2 submit
// body uses, so amounts are quoted decimal strings rather than the bare JSON
// numbers a default *big.Int marshal would emit. Client.GetBatchPaymentEstimateFee
// and a caller's direct json.Marshal therefore produce identical bodies.
func (r BatchPaymentFeeEstimateRequest) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"from":       r.From,
		"token":      r.Token,
		"operations": batchOperationsWireList(r.Operations),
	})
}
```

- [ ] **Step 4: Add the endpoint constant and the client method**

In `transactions.go`, extend the const block:

```go
const (
	endpointTransactionsByHashV1           = "/v1/transactions/by_hash"
	endpointTransactionsReceiptByHashV1    = "/v1/transactions/receipt/by_hash"
	endpointTransactionsFinalizedV1        = "/v1/transactions/finalized/by_hash"
	endpointTransactionsEstimateFeeV1      = "/v1/transactions/estimate_fee"
	endpointTransactionsPaymentV1          = "/v1/transactions/payment"
	endpointBatchPaymentEstimateFeeV1      = "/v1/transactions/batch_payment/estimate_fee"
)
```

and add the method immediately after `GetEstimateFee`:

```go
// GetBatchPaymentEstimateFee retrieves an estimated fee for an unsigned batch
// payment. The result is a non-binding, point-in-time quote and does not
// guarantee admission: the node cannot validate encoded size, authorization,
// nonce, chain id, timestamp, memo, or operations hash from this request.
//
// The endpoint is POST because the operation list is a request body; it is a
// read-only fee query. Its /v1 prefix is the L1 read/service surface and does
// not imply legacy batch-payment submission, which this SDK does not support.
func (client *Client) GetBatchPaymentEstimateFee(ctx context.Context, request BatchPaymentFeeEstimateRequest) (*EstimateFeeResponse, error) {
	if err := validateBatchOperationAmounts(request.Operations); err != nil {
		return nil, err
	}
	result := new(EstimateFeeResponse)
	return result, client.PostMethod(ctx, endpointBatchPaymentEstimateFeeV1, request, result)
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./... -run 'TestBatchPaymentFeeEstimateRequest|TestGetBatchPaymentEstimateFee|TestBatchOperationEncodingIsShared' -v 2>&1 | tail -20`
Expected: PASS on all four.

- [ ] **Step 6: Run the full suite**

Run: `go test ./... 2>&1 | tail -10`
Expected: PASS.

- [ ] **Step 7: Re-pin the public-API surface hash**

This task adds exported symbols (`BatchPaymentFeeEstimateRequest`, its `MarshalJSON`, and `Client.GetBatchPaymentEstimateFee`), so `compatibility_test.go`'s `TestPublicAPICompatibility` baseline must be re-pinned or the suite stays red.

```bash
go test ./... -run TestPublicAPICompatibility 2>&1 | tail -5
```

Take the reported `public API hash = <got>` value, confirm by diffing `go doc -all .` against the previous surface that the ONLY delta is your new exported symbols, then update `publicAPIHash` in `compatibility_test.go` to `<got>` and extend the explanatory comment above it to name this change.

- [ ] **Step 8: Commit**

```bash
gofumpt -l -w .
git add transactions.go transactions_types.go transactions_test.go compatibility_test.go
git commit -m "feat: add GetBatchPaymentEstimateFee"
```

---

### Task 9: Update the BatchPayment business-flow integration test

**Blocked by:** Tasks 5-8.

**Files:**
- Modify: `business_integration_test.go:1179-1230` (`TestBusinessFlow_BatchPayment`)

**Interfaces:**
- Consumes: everything from Tasks 5-8.
- Produces: no package-level API. These tests are `integration`-tagged and need a live node (`TEST_ENV`, `TEST_OPERATOR_PRIVATE_KEY`, `TEST_MASTER_PRIVATE_KEY` — see `BUSINESS_FLOW_TESTS.md`), so this task's gate is that they compile and vet cleanly under the tag.

- [ ] **Step 1: Drop `MaxFee` and add the memo dimension**

In `TestBusinessFlow_BatchPayment`, the payload literal currently carries `MaxFee: big.NewInt(100000000),`. Remove it, and submit twice — once with the canonical empty memo, once with a populated one:

```go
	suite.refreshCheckpoint()
	payload := BatchPaymentPayload{
		ChainID: suite.ChainID,
		Nonce:   suite.getNonce(suite.Account1.Address),
		Token:   tokenAddr,
		Operations: []PaymentOperation{
			{Recipient: suite.Account2.Address, Amount: amount2},
			{Recipient: recipient3.Address, Amount: amount3},
		},
		CreatedAt: uint64(time.Now().Unix()),
	}

	t.Logf("Batch paying %d recipients from %s", len(payload.Operations), suite.Account1.Address.Hex())
	result, err := suite.Client.Transactions().BatchPayment(ctx, payload, suite.Account1.Signer)
	if err != nil {
		t.Fatalf("Failed to submit batch payment: %v", err)
	}
```

- [ ] **Step 2: Assert the locally computed hash matches the server's**

Immediately after the first submission, before waiting for the receipt:

```go
	prepared, err := PrepareTransaction(payload)
	if err != nil {
		t.Fatalf("PrepareTransaction for hash cross-check: %v", err)
	}
	signature, err := suite.Account1.Signer.SignHash(prepared.SigningHash())
	if err != nil {
		t.Fatalf("sign for hash cross-check: %v", err)
	}
	authorized, err := prepared.Authorize(signature)
	if err != nil {
		t.Fatalf("authorize for hash cross-check: %v", err)
	}
	if !strings.EqualFold(hexLower(authorized.TransactionHash()), result.Hash) {
		t.Fatalf("local tx hash %s != server hash %s", hexLower(authorized.TransactionHash()), result.Hash)
	}
```

- [ ] **Step 3: Add a populated-memo submission and assert the memo round-trips**

After the existing receipt and transaction assertions:

```go
	suite.refreshCheckpoint()
	memoPayload := payload
	memoPayload.Nonce = suite.getNonce(suite.Account1.Address)
	memoPayload.CreatedAt = uint64(time.Now().Unix())
	memo := Memo{Type: "purpose/PAYROLL", Format: "text/plain", Data: "batch-flow-memo"}

	memoResult, err := suite.Client.Transactions().BatchPayment(ctx, memoPayload, suite.Account1.Signer, WithMemo(memo))
	if err != nil {
		t.Fatalf("Failed to submit batch payment with memo: %v", err)
	}
	memoReceipt := suite.waitForTransaction(memoResult.Hash, 60*time.Second)
	if !memoReceipt.Success {
		t.Fatal("Batch payment with memo failed")
	}
	memoTx := suite.fetchTransaction(t, memoResult.Hash)
	if memoTx.Memo == nil {
		t.Fatal("batch payment response carried no memo")
	}
	if *memoTx.Memo != memo {
		t.Errorf("memo = %+v, want %+v", *memoTx.Memo, memo)
	}
```

- [ ] **Step 4: Assert the decoded payload no longer carries a fee field**

Extend the existing transaction assertions for the first submission:

```go
	batchData, ok := tx.AsBatchPaymentData()
	if !ok {
		t.Fatal("transaction did not decode as BatchPaymentData")
	}
	if len(batchData.Operations) != len(payload.Operations) {
		t.Errorf("decoded %d operations, want %d", len(batchData.Operations), len(payload.Operations))
	}
	if batchData.CreatedAt != payload.CreatedAt {
		t.Errorf("decoded created_at %d, want %d", batchData.CreatedAt, payload.CreatedAt)
	}
```

- [ ] **Step 5: Verify the tagged build compiles and vets**

```bash
go vet -tags integration ./...
```

Expected: PASS with no output. Add `strings` to the file's imports if it is not already there; remove `math/big` only if nothing else in the file uses it.

- [ ] **Step 6: Run the untagged suite to confirm nothing regressed**

Run: `go test ./... 2>&1 | tail -10`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
gofumpt -l -w .
golangci-lint run
git add business_integration_test.go
git commit -m "test: cover memo and hash cross-check in batch payment flow"
```

---

### Task 10: Update the documentation surface

**Blocked by:** Tasks 4-8.

**Files:**
- Modify: `README.md`, `CHANGELOG.md`, `MIGRATION.md`
- Modify: `docs/superpowers/specs/2026-07-27-go-sdk-v2-upgrade-design.md`
- Modify: `docs/superpowers/plans/2026-07-27-go-sdk-v2-upgrade.md`, `docs/superpowers/specs/2026-07-28-native-v2-golden-vector-coverage-design.md`, `docs/superpowers/plans/2026-07-29-native-v2-golden-vector-matrix-expansion.md`

**Interfaces:**
- Consumes: the final public API from Tasks 4-8.
- Produces: no code. This is the last task in the Go PR.

- [ ] **Step 1: Find every stale BatchPayment claim**

```bash
grep -rn 'max_fee\|MaxFee' --include='*.md' .
grep -rni 'batch.*no memo\|memo-incapable\|carries no memo\|carry no memo' --include='*.md' .
```

Expected: hits in `docs/superpowers/plans/2026-07-27-go-sdk-v2-upgrade.md:220`, `docs/superpowers/specs/2026-07-28-native-v2-golden-vector-coverage-design.md:115,467`, `docs/superpowers/plans/2026-07-29-native-v2-golden-vector-matrix-expansion.md:143`.

- [ ] **Step 2: Correct the historical design and plan documents in place**

These are historical records, so do not rewrite them wholesale. Add a superseding note at the top of each affected section, for example in `docs/superpowers/plans/2026-07-27-go-sdk-v2-upgrade.md`:

```markdown
> **Superseded for BatchPayment (2026-08-10):** `max_fee` was removed from the
> signed payload and BatchPayment became memo-bearing. See
> `docs/superpowers/specs/2026-08-10-batch-payment-v2-rebaseline-design.md`.
> The BatchPayment field order below is stale; the current order is
> `chain_id, nonce, token, operations, created_at, operations_hash?, batch_id?`.
```

- [ ] **Step 3: Update `CHANGELOG.md`**

Add an entry under the unreleased heading:

```markdown
### Breaking

- **BatchPayment re-baselined to the current L1 canonical format.**
  `BatchPaymentPayload.MaxFee` and `BatchPaymentData.MaxFee` are removed; the
  signed payload is now `WithMemo<BatchPaymentPayload>` for both the default
  empty memo and a caller-supplied one. Signing hashes and transaction hashes
  produced by earlier SDK versions are no longer valid. BatchPayment is
  v2-only: a client configured with `WithLegacyV1` returns an error before
  signing or network I/O.

### Added

- `Client.GetBatchPaymentEstimateFee` — non-binding fee quote via
  `POST /v1/transactions/batch_payment/estimate_fee`.
- `DeriveBatchPaymentOperationsHash` — the supported way to populate the
  optional `BatchPaymentPayload.OperationsHash`.
- `Transactions().BatchPayment` now accepts `WithMemo`.
```

- [ ] **Step 4: Update `MIGRATION.md`**

Add a BatchPayment section listing the six caller-facing steps:

```markdown
## BatchPayment (2026-08-10 re-baseline)

1. Remove `MaxFee` from every `BatchPaymentPayload` literal — the field no
   longer exists.
2. Keep calling `Transactions().BatchPayment`; the method shape is unchanged.
3. Pass `WithMemo(m)` to attach a memo. Omitting it signs the canonical empty
   memo, exactly as with every other v2 operation.
4. Do not configure the client with `WithLegacyV1` for BatchPayment; it is
   v2-only and returns an error before signing.
5. Call `GetBatchPaymentEstimateFee` for a non-binding fee quote.
6. Call `DeriveBatchPaymentOperationsHash` before setting the optional
   `OperationsHash` field.
```

- [ ] **Step 5: Update `README.md`**

Update any BatchPayment example to drop `MaxFee`, and add a short fee-estimate example beside the existing `GetEstimateFee` one.

- [ ] **Step 6: Verify no stale claims remain**

```bash
grep -rn 'max_fee\|MaxFee' --include='*.md' . | grep -v '2026-08-10-batch-payment'
```

Expected: only superseding notes that explicitly describe `max_fee` as removed.

- [ ] **Step 7: Run the full verification suite**

```bash
gofumpt -l -w .
go test ./...
golangci-lint run
```

Expected: all PASS, `gofumpt` prints nothing.

- [ ] **Step 8: Commit**

```bash
git add README.md CHANGELOG.md MIGRATION.md docs/
git commit -m "docs: record the BatchPayment v2 re-baseline"
```

---

## Final Verification

> **Correction (2026-08-10):** two bullets below no longer apply as written.
> The last bullet's `git -C ../l1client branch --contains <sha> | grep main`
> check is **SUPERSEDED**: the merge-to-`main` gate was explicitly waived by
> human decision (no PR, no merge — see the GATE correction above), so that
> commit is on exactly one local, unpushed l1client branch
> (`feat/go-sdk-vector-generator`) and this check fails by design, not by
> defect. The `golangci-lint` bullet is **OPEN (deferred to CI)**: `golangci-lint`
> is not installed in this environment, so it is not a failed or skipped local
> check — it is left for CI to gate, per this file's environment note.

Run from the Go SDK root once every task is complete:

- [ ] `gofumpt -l -w .` — prints nothing
- [ ] `go test ./...` — all PASS
- [ ] `golangci-lint run` — no findings. **[OPEN — deferred to CI, see correction above.]** **Environment note:** `golangci-lint` is not installed on this machine, so this check cannot be run locally and must be left to CI. Substitute locally with `go vet ./...` (must be clean) and `staticcheck ./...` (one pre-existing `SA1012` in the untouched `misc_test.go` is the known baseline — anything else is a finding).
- [ ] `go vet -tags integration ./...` — no findings
- [ ] `grep -rn 'MaxFee\|max_fee' --include='*.go' .` — no output
- [ ] `grep -rn 'memoCapable\|encodeBare' --include='*.go' .` — no output
- [ ] `python3 -c "import json; print(json.load(open('testdata/prepare-authorize-hash-vectors.json'))['_source']['commit'])"` — prints the merged `l1client/main` SHA from the GATE, and `git -C ../l1client branch --contains <that sha> | grep main` confirms it is on main. **[SUPERSEDED — see correction above; the second half of this check fails by design since the commit was deliberately never merged.]**

Success criteria 1-14 in `docs/superpowers/specs/2026-08-10-batch-payment-v2-rebaseline-design.md` §12 map to these checks plus the per-task assertions above.
