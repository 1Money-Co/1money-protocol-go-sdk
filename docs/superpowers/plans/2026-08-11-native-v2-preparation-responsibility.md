# Native V2 Preparation Responsibility Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Separate canonical encoding, public preparation, admission rejection, endpoint mapping, and fee-estimate responsibilities without changing valid transaction bytes or public method signatures.

**Architecture:** Make endpoint lookup fallible at operation resolution, split BatchPayment submission validation into the node's ordered phases, and leave estimate admission to the server. Keep a canonical oracle pass for all 213 vectors, then add explicit SDK-owned fixture metadata that drives public-accept and public-reject passes without consulting the validator under test.

**Tech Stack:** Go 1.24, `encoding/json`, go-ethereum RLP/crypto, Go standard tests, vendored JSON fixtures.

## Global Constraints

- Preserve all existing uncommitted user changes in `compatibility_test.go`, `memo_test.go`, `native_v2_prepare.go`, `transactions_test.go`, and `transactions_types.go`.
- Do not change canonical RLP, hashes, request bodies for valid inputs, operation numbers, or exported method signatures.
- Do not add an `l1client` commit or generator dependency to either fixture.
- Keep negative and wider-than-U256 amounts as local encoding failures.
- Do not stage or commit; the user has requested implementation but has not requested a commit.

---

### Task 1: Make endpoint mapping fallible

**Files:**
- Modify: `native_v2_requests.go:120-173`
- Modify: `native_v2_requests_test.go`
- Test: `api_v2_test.go`

**Interfaces:**
- Produces: `pathsForOp(op nativeOperationType) (v1 string, v2 string, err error)`.
- `opFromPayload` consumes the fallible mapping and propagates its error.

- [ ] **Step 1: Add failing mapping tests**

Update `TestPathsForOp` to require `err == nil` for all 14 operations and add:

```go
func TestPathsForOpRejectsUnmappedOperation(t *testing.T) {
    v1, v2, err := pathsForOp(nativeOperationType(999))
    if err == nil || !strings.Contains(err.Error(), "no domain-separated v2 endpoint configured") {
        t.Fatalf("paths = (%q, %q), err = %v; want unmapped-v2 error", v1, v2, err)
    }
}
```

- [ ] **Step 2: Run the focused tests and confirm the signature mismatch/failure**

Run:

```bash
go test -run 'TestPathsForOp|TestLegacyModeRejectsV2OnlyOperations' ./...
```

Expected: FAIL until `pathsForOp` returns an error and callers handle it.

- [ ] **Step 3: Implement the fallible mapping**

Change every mapped switch arm to return a nil error. The default must be:

```go
default:
    return "", "", fmt.Errorf(
        "%s: no domain-separated v2 endpoint configured",
        op.label(),
    )
```

In `opFromPayload`:

```go
v1, v2, err := pathsForOp(op)
if err != nil {
    return nativeV2Op{}, err
}
```

An empty `v1` is interpreted as v2-only only after mapping succeeds.

- [ ] **Step 4: Run focused tests**

Run the command from Step 2. Expected: PASS.

---

### Task 2: Match BatchPayment submission-validation order

**Files:**
- Modify: `native_v2_batch.go:57-121`
- Modify: `native_v2_batch_test.go`

**Interfaces:**
- Produces: `validateBatchOperationItems([]PaymentOperation) error` for non-empty, recipient, and positive-amount checks.
- Produces: `validateBatchOperationTotal([]PaymentOperation) error` for the final overflow fold.
- `validateBatchPaymentSubmission` owns the ordered orchestration.

- [ ] **Step 1: Add failing precedence tests**

Add subtests to `TestPrepareRejectsInadmissibleBatchPayment`:

```go
t.Run("recipient error precedes overflow", func(t *testing.T) {
    p := base([]PaymentOperation{
        {Recipient: repeatAddr(0x0c), Amount: maxU256},
        {Recipient: repeatAddr(0x0d), Amount: big.NewInt(1)},
        {Recipient: common.Address{}, Amount: big.NewInt(1)},
    })
    _, err := PrepareTransaction(p)
    if err == nil || !strings.Contains(err.Error(), "operation 2 has an invalid recipient") {
        t.Fatalf("err = %v, want recipient error before overflow", err)
    }
})

t.Run("operations hash precedes overflow", func(t *testing.T) {
    p := base([]PaymentOperation{
        {Recipient: repeatAddr(0x0c), Amount: maxU256},
        {Recipient: repeatAddr(0x0d), Amount: big.NewInt(1)},
    })
    p.OperationsHash = &wrongHash
    _, err := PrepareTransaction(p)
    if err == nil || !strings.Contains(err.Error(), "operations_hash mismatch") {
        t.Fatalf("err = %v, want operations_hash error before overflow", err)
    }
})
```

- [ ] **Step 2: Run the focused test and confirm failure**

```bash
go test -run TestPrepareRejectsInadmissibleBatchPayment ./...
```

Expected: the new precedence cases fail under the interleaved overflow check.

- [ ] **Step 3: Split and order the validator**

Implement item validation without total accumulation, then an independent fold:

```go
func validateBatchOperationTotal(operations []PaymentOperation) error {
    total := new(big.Int)
    for _, operation := range operations {
        total.Add(total, bigOrZero(operation.Amount))
        if total.BitLen() > 256 {
            return fmt.Errorf("batch payment total amount overflow")
        }
    }
    return nil
}
```

`validateBatchPaymentSubmission` must execute:

```go
if err := validateBatchOperationItems(payload.Operations); err != nil { return err }
if payload.OperationsHash != nil { /* derive and compare */ }
return validateBatchOperationTotal(payload.Operations)
```

- [ ] **Step 4: Run BatchPayment tests**

```bash
go test -run 'TestPrepareRejectsInadmissibleBatchPayment|TestDeriveBatchPaymentOperationsHash' ./...
```

Expected: PASS.

---

### Task 3: Layer canonical, public-accept, and public-reject oracle tests

**Files:**
- Modify: `testdata/prepare-authorize-hash-vectors.json`
- Modify: `native_v2_prepare_test.go`
- Modify: `testdata/README.md`

**Interfaces:**
- Extends `prepareAuthorizeFixtureMeta` with `PublicPrepareRejections map[string]string` using JSON key `public_prepare_rejections`.
- Produces one shared assertion helper that accepts a prepared transaction and fixture vector.

- [ ] **Step 1: Add explicit fixture metadata**

Add the 22 reviewed encoding-only vector names under
`_fixture.public_prepare_rejections`. The values are stable error substrings:

```json
{
  "batch_option_hash_only": "operations_hash mismatch",
  "batch_option_hash_only_memo": "operations_hash mismatch",
  "batch_option_both": "operations_hash mismatch",
  "batch_option_both_memo": "operations_hash mismatch",
  "batch_option_zero_hash": "operations_hash mismatch",
  "batch_option_zero_hash_memo": "operations_hash mismatch",
  "batch_operations_empty": "operations must not be empty",
  "batch_operation_amount_zero": "amount must be greater than 0",
  "batch_pairwise_01_neither_forward_zero_populated": "amount must be greater than 0",
  "batch_pairwise_02_hash_only_single_max_populated": "operations_hash mismatch",
  "batch_pairwise_03_hash_only_reverse_zero_empty": "amount must be greater than 0",
  "batch_pairwise_04_id_only_forward_max_empty": "total amount overflow",
  "batch_pairwise_06_both_single_zero_empty": "amount must be greater than 0",
  "batch_pairwise_07_both_forward_ordinary_populated": "operations_hash mismatch",
  "batch_pairwise_08_neither_reverse_max_empty": "total amount overflow",
  "batch_pairwise_09_neither_empty_none_empty": "operations must not be empty",
  "batch_pairwise_10_hash_only_empty_none_populated": "operations must not be empty",
  "batch_pairwise_11_hash_only_forward_ordinary_empty": "operations_hash mismatch",
  "batch_pairwise_12_id_only_single_zero_empty": "amount must be greater than 0",
  "batch_pairwise_13_both_reverse_max_empty": "operations_hash mismatch",
  "batch_pairwise_14_id_only_empty_none_empty": "operations must not be empty",
  "batch_pairwise_15_both_empty_none_empty": "operations must not be empty"
}
```

- [ ] **Step 2: Add completeness failures before changing the oracle loop**

Extend `TestPrepareAuthorizeFixtureCompleteness` to require exactly 22 rejection
entries, require every name to exist, and require every named vector to have
`operation == "BatchPayment"`.

Run:

```bash
go test -run TestPrepareAuthorizeFixtureCompleteness ./...
```

Expected: FAIL until the fixture loader exposes and validates the metadata.

- [ ] **Step 3: Split the oracle responsibilities**

Keep the existing all-vector loop through `prepareCanonical`, but rename it to
make canonical scope explicit. Extract the repeated signing-hash,
transaction-hash, and signer-recovery assertions into a helper.

Add a public-accept test:

```go
for _, vector := range fixture.Vectors {
    if _, rejected := fixture.Meta.PublicPrepareRejections[vector.Name]; rejected {
        continue
    }
    prepared, err := PrepareTransaction(payload, options...)
    if err != nil { t.Fatalf("PrepareTransaction: %v", err) }
    assertPrepareAuthorizeVector(t, prepared, vector, wantSigner)
}
```

Add a public-reject test that iterates the metadata map, resolves each named
vector, calls `PrepareTransaction`, and checks the declared substring. Do not
fall back dynamically after a public preparation error.

- [ ] **Step 4: Run all fixture and BatchPayment oracle tests**

```bash
go test -run 'TestPrepareAuthorizeFixture|TestCanonicalPrepareAndAuthorize|TestPublicPrepareAndAuthorize|TestEncodingOnly|TestBatchPaymentOptionalGoldenVectors' ./...
```

Expected: PASS with 213 canonical, 191 public-accept, and 22 public-reject cases.

---

### Task 4: Make fee estimation server-authoritative

**Files:**
- Modify: `transactions.go:55-71`
- Modify: `transactions_test.go:847-918`
- Modify: `CHANGELOG.md`
- Modify: `MIGRATION.md`
- Modify: `README.md`
- Modify: `docs/superpowers/specs/2026-08-10-batch-payment-v2-rebaseline-design.md`

**Interfaces:**
- `GetBatchPaymentEstimateFee` delegates every encodable request to `PostMethod`.
- `BatchPaymentFeeEstimateRequest.MarshalJSON` remains the U256 encoding gate.

- [ ] **Step 1: Add failing pass-through tests**

Use an HTTP test server to return a valid estimate for requests containing each
of: empty operations, zero recipient, zero amount, and total overflow. Assert
the method performs one POST per case and returns the server response.

Keep `TestGetBatchPaymentEstimateFeeRejectsOutOfRangeAmount`, and extend it with
a wider-than-U256 amount; both cases must issue zero HTTP requests.

- [ ] **Step 2: Run the focused tests and confirm local admission blocks them**

```bash
go test -run 'TestGetBatchPaymentEstimateFee' ./...
```

Expected: pass-through cases fail before HTTP under the current local admission
validator.

- [ ] **Step 3: Remove the estimate admission gate**

Delete the call to `validateBatchOperationsStatic` from
`GetBatchPaymentEstimateFee`. Do not remove the validation call from
`BatchPaymentFeeEstimateRequest.MarshalJSON`.

- [ ] **Step 4: Update public documentation**

Replace claims that the estimate method duplicates static admission rules with:

```text
The SDK validates only wire encodability. The estimate service is authoritative
for empty batches, recipients, zero amounts, total overflow, governance
configuration, and future quote semantics.
```

- [ ] **Step 5: Run focused tests**

Run the command from Step 2. Expected: PASS.

---

### Task 5: Full verification and scope audit

**Files:**
- Verify all files changed by Tasks 1-4 and the pre-existing user changes.

**Interfaces:**
- Consumes all preceding tasks; produces no new API.

- [ ] **Step 1: Format changed Go files**

```bash
gofumpt -l -w .
```

If `gofumpt` is unavailable, run `gofmt -w` only on the changed Go files and
report that `gofumpt` was unavailable.

- [ ] **Step 2: Run focused and full verification**

```bash
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
golangci-lint run
git diff --check
```

If an optional repository tool is unavailable, report the exact command not
run; do not claim it passed.

- [ ] **Step 3: Audit invariants**

Confirm with searches and tests that:

- no production caller invokes `prepareCanonical`;
- all current native operations have a v2 mapping;
- estimate admission validation has only been removed from the estimate method,
  not from submission;
- fixture expected hashes are unchanged; and
- valid request bodies and public API hashes remain unchanged except for the
  user's already-present `UnmarshalJSON` addition.

- [ ] **Step 4: Report without committing**

Summarize changed behavior, tests, unavailable tools, and the untouched status
of user-owned uncommitted changes. Do not stage, commit, or push without a new
explicit instruction.
