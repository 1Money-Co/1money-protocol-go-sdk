# Batch Payment V2 Re-baseline Design

**Date:** 2026-08-10

**Status:** Revised after design review; pending final approval

## 1. Context

`l1client` commit `7ad79889` re-baselines BatchPayment before launch. The
canonical transaction removes the signed `max_fee` field, wraps every batch
payload in `WithMemo<BatchPaymentPayload>`, adds a REST fee-estimate endpoint,
and removes `max_fee` from transaction-query responses.

The Go SDK implementation at `eae96d1` still implements the superseded shape:
it signs a bare BatchPayment payload, includes `max_fee` in RLP and JSON,
rejects memos, has no batch fee-estimate method, and expects `max_fee` in read
responses. Its bundled golden vectors consequently validate the obsolete
hashes.

BatchPayment is a new, not-yet-compatible API. This design supports only the
new canonical format. It deliberately provides no deprecated fields, legacy
encoding branch, fallback, or old-node compatibility layer.

## 2. Goals

1. Make Go BatchPayment signing and transaction hashing byte-identical to the
   canonical L1 v2 format.
2. Expose one BatchPayment submit method that always signs a memo wrapper,
   using `EmptyMemo()` when the caller does not provide `WithMemo`.
3. Make BatchPayment submission v2-only and fail before network I/O when the
   client is configured for legacy v1 submission.
4. Add the unsigned batch fee-estimate REST operation.
5. Provide a public, Rust-conformant derivation function for the optional
   `operations_hash` field.
6. Restore an executable L1 oracle for the extended prepare/authorize fixture
   before consuming that fixture in Go.
7. Align BatchPayment query DTOs, golden vectors, tests, and documentation with
   the new L1 contract.

## 3. Non-goals

- Supporting `/v1/transactions/batch_payment` from the Go SDK.
- Accepting or generating the old bare BatchPayment signature shape.
- Preserving `BatchPaymentPayload.MaxFee` or `BatchPaymentData.MaxFee`.
- Automatically retrying a v2 submission through a legacy endpoint.
- Adding SDK-side governance-dependent validation for batch enablement,
  operation limits, fee assets, or current pricing.
- Changing the native operation type, which remains `BatchPayment = 14`.
- Changing the Go module path or introducing a BatchPayment subpackage.
- Treating either decimal or hexadecimal U256 strings as a server requirement;
  the Go SDK deliberately chooses one consistent representation.

## 4. Public API

### 4.1 BatchPayment submission

Retain the existing namespace method shape:

```go
func (a TransactionsAPI) BatchPayment(
    ctx context.Context,
    payload BatchPaymentPayload,
    signer Signer,
    opts ...SubmitOption,
) (*PaymentResponse, error)
```

The method has the following contract:

- It submits only to `/v2/transactions/batch_payment`.
- Omitting `WithMemo` selects `EmptyMemo()`.
- Supplying `WithMemo(m)` signs and submits `m`.
- Both paths encode `WithMemo<BatchPaymentPayload>`; a bare payload is never a
  valid signing input.
- A client configured with `WithLegacyV1` returns a v2-only error before
  signing or network I/O.
- No separate `BatchPaymentWithMemo`, legacy BatchPayment, or pre-signed
  BatchPayment method is introduced.

### 4.2 Payload type

The public payload becomes:

```go
type BatchPaymentPayload struct {
    ChainID        uint64
    Nonce          uint64
    Token          common.Address
    Operations     []PaymentOperation
    CreatedAt      uint64
    OperationsHash *common.Hash
    BatchID        *string
}
```

`MaxFee` is removed without an alias or compatibility field.

### 4.3 Fee estimation

Add:

```go
type BatchPaymentFeeEstimateRequest struct {
    From       common.Address     `json:"from"`
    Token      common.Address     `json:"token"`
    Operations []PaymentOperation `json:"operations"`
}

func (c *Client) GetBatchPaymentEstimateFee(
    ctx context.Context,
    request BatchPaymentFeeEstimateRequest,
) (*EstimateFeeResponse, error)
```

The method calls:

```text
POST /v1/transactions/batch_payment/estimate_fee
```

The `/v1` prefix belongs to the L1 read/service surface and does not imply that
the SDK supports legacy BatchPayment submission.

The existing `EstimateFeeResponse` is reused:

```go
type EstimateFeeResponse struct {
    Fee  string  `json:"fee"`
    Plan *string `json:"plan,omitempty"`
}
```

The current L1 BatchPayment response returns a decimal `fee` and `plan: null`.

The `Get` prefix intentionally matches the existing flat
`Client.GetEstimateFee` API. The endpoint uses POST because its operation list
is a request body; the method remains a non-mutating fee query.

`BatchPaymentFeeEstimateRequest` is also a correct public wire type. It
implements `json.Marshaler` and reuses the shared operation-to-wire helper, so
both `Client.GetBatchPaymentEstimateFee` and a caller's direct
`json.Marshal(request)` produce lowercase field names and quoted decimal amount
strings. The public `Request` type must not expose a second, invalid default
encoding with bare JSON numbers.

### 4.4 Operations-hash derivation

Add a public pure function alongside the BatchPayment transaction types:

```go
func DeriveBatchPaymentOperationsHash(
    operations []PaymentOperation,
) (common.Hash, error)
```

It computes the exact node rule:

```text
keccak256(RLP(operations))
```

where each operation is `[recipient, amount]`. It validates that every amount
is representable as U256, but does not apply governance-dependent limits or
claim that the resulting batch will be admitted. Callers may leave
`OperationsHash` nil; if they set it, this function is the supported way to
derive the value.

A nil `PaymentOperation.Amount` has the same meaning as in the existing submit
encoder: it is encoded as U256 zero. The derivation function therefore accepts
nil and returns the hash of the zero-valued operation. This guarantees that the
helper hashes exactly what the submit path would sign. It does not make the
operation admission-valid: L1 rejects BatchPayment operations whose amount is
zero. Negative non-nil amounts and values wider than 256 bits return an error.

## 5. Internal Architecture

BatchPayment remains part of the existing shared native-v2 pipeline. No
BatchPayment-specific signer or hash implementation is added.

The resolved operation metadata is:

```text
operation type: 14
legacy v1 path: absent
v2 path: /v2/transactions/batch_payment
```

`pathsForOp(opBatchPayment)` returns an empty v1 path and the canonical v2
path. In `submitPayload`, operation resolution moves before the legacy memo
guard. The legacy branch then applies checks in this order:

1. If the resolved operation has no v1 path, return its stable v2-only error.
2. If the operation supports v1 but the caller explicitly supplied `WithMemo`,
   return the existing legacy-memo error.
3. Otherwise build and submit the legacy body.

This ordering ensures `WithLegacyV1 + WithMemo + BatchPayment` reports that
BatchPayment is v2-only rather than being masked by the generic legacy memo
error. It is a generic capability check rather than a BatchPayment type switch.

Moving operation resolution ahead of the legacy memo guard intentionally
changes validation-error precedence for invalid calls to existing v1-capable
operations. For example, a `TokenManageListPayload` carrying `WithMemo` but
missing `WithManageListKind` reports the more specific ambiguous-operation
error before the generic legacy-memo error. Successful behavior and wire output
are unchanged. This limited precedence change is in scope because it is the
direct consequence of making the capability check authoritative and is locked
by tests.

**Correction (found during implementation, 2026-08-10):** an earlier revision of
this section claimed the generic check also fixes an existing CreateMultisig
legacy failure mode in which the legacy branch could POST an empty v1 path.
That is false for the public API. `AccountsAPI.CreateMultisig`
(`accounts.go:97-99`) already guards legacy mode one layer above `submitPayload`
and returns before reaching it, with a more specific message
("multisig account creation requires domain-separated v2 and has no legacy v1
endpoint"). That guard stays, unchanged — degrading a documented public error
message to route it through the generic path would be a regression and is out
of scope here.

`pathsForOp(opCreateMultiSig)` does return an empty v1 path, so the generic
check does cover CreateMultisig — but only for callers of the unexported submit
core, not for anything reachable through the public API. The generic check's
real value is therefore BatchPayment, which has no namespace-level guard and
genuinely needs it, plus defense-in-depth for any future v2-only operation.
Both behaviors are pinned by tests: the namespace guard through
`Accounts().CreateMultisig`, and the generic capability error through a direct
`submitPayload` call.

After the BatchPayment re-baseline, all fourteen canonical native-v2 operations
use `WithMemo<Payload>`. The implementation therefore removes the now-constant
`memoCapable` return value and fields from `resolvePayloadOp`, `nativeV2Op`, and
`PreparedTransaction`. It also removes the conditional bare-payload branch and
the production `encodeBare` helper. Native-v2 preparation always calls
`encodeWithMemo`, and authorization always writes the required memo object.

The existing `prepareFromPayload`, `newPrepared`, and
`PreparedTransaction.Authorize` flow then:

1. Resolves the caller's memo option, defaulting to `EmptyMemo()`.
2. Encodes `WithMemo<BatchPaymentPayload>` with `encodeWithMemo`.
3. Computes the domain-separated signing hash for operation type 14.
4. Obtains the signature through the supplied `Signer`.
5. Computes the signed transaction hash locally.
6. Builds a v2 body containing the required `memo` and `authorization` fields.
7. Posts to `/v2/transactions/batch_payment`.
8. Compares the node-returned hash with the locally computed hash and fails
   closed on a mismatch.

Offline `PrepareTransaction` and online `Transactions().BatchPayment` continue
to share this exact encoding and hashing path.

Legacy-v1 signing for operations that still support it remains in the separate
`buildLegacyV1Body` path. Removing native-v2 `encodeBare` does not change those
legacy payloads.

## 6. Canonical Encoding and Wire Contract

### 6.1 Payload RLP

`BatchPaymentPayload.rlpList()` encodes fields in this order:

```text
chain_id
nonce
token
operations
created_at
operations_hash?
batch_id?
```

Each operation remains `[recipient, amount]`. `operations_hash` and `batch_id`
retain their existing trailing-option behavior: when `batch_id` is present and
`operations_hash` is absent, an empty placeholder represents the absent hash.

The canonical v2 `payload_rlp` is always:

```text
rlp([
  BatchPaymentPayload.rlpList(),
  [memo.type, memo.format, memo.data]
])
```

The `max_fee` element is removed before `created_at`; this is a field-order
change, not merely a JSON change.

### 6.2 Submit JSON

The flattened v2 request always includes a complete memo object and never
includes `max_fee`:

```json
{
  "chain_id": 1212101,
  "nonce": 1,
  "token": "0x1111111111111111111111111111111111111111",
  "operations": [
    {
      "recipient": "0x2222222222222222222222222222222222222222",
      "amount": "100"
    }
  ],
  "created_at": 1747785600,
  "memo": {
    "type": "",
    "format": "",
    "data": ""
  },
  "authorization": {
    "type": "single_secp256k1",
    "signature": {
      "r": "0x...",
      "s": "0x...",
      "v": 0
    }
  }
}
```

Existing decimal-string serialization for submit-operation amounts remains
unchanged. Decimal strings are an SDK consistency choice, not an L1 protocol
requirement; L1 accepts both decimal and hexadecimal U256 strings.

### 6.3 Fee-estimate JSON

The submit and estimate paths share one internal operation-to-wire helper.
Directly marshalling `*big.Int` would emit a bare JSON number, so the helper
converts every amount to a quoted decimal string while copying the recipient.
`BatchPaymentFeeEstimateRequest.MarshalJSON` uses this same helper:

```json
{
  "from": "0x3333333333333333333333333333333333333333",
  "token": "0x1111111111111111111111111111111111111111",
  "operations": [
    {
      "recipient": "0x2222222222222222222222222222222222222222",
      "amount": "100"
    }
  ]
}
```

The estimate body and direct public-request serialization are assembled from
that shared wire-operation representation; neither introduces a second amount
encoding or a dedicated hexadecimal wire DTO. Quoted decimal strings preserve
arbitrary U256 precision and match the existing Go BatchPayment submit
representation.

### 6.4 Operations hash

`DeriveBatchPaymentOperationsHash` uses the same operation RLP helper as the
BatchPayment payload encoder, then hashes the encoded operation list with
Keccak-256. Reusing the operation encoder prevents field-order drift between
the exported derivation function and the signed payload.

The implementation is tested against L1-generated vectors. A Go-computed hash
compared only with another Go-computed value is not an acceptable oracle.

## 7. Read-side Alignment

Remove `MaxFee` from `BatchPaymentData`. The decoded payload continues to
contain:

- `Token`
- `Operations`
- `OperationsHash`
- `BatchID`
- `CreatedAt`

The existing top-level `Transaction.Memo` already represents the signed batch
memo and requires no type change. Receipt types and batch execution-event
types are unchanged.

## 8. Validation and Error Handling

The SDK validates only information required to encode a valid request:

- A nil signer remains an error through the shared submit path.
- A nil operation amount is encoded as U256 zero, consistently across payload
  RLP, submit JSON, fee-estimate JSON, and operations-hash derivation. L1 will
  subsequently reject it as a zero-amount BatchPayment operation.
- Every non-nil operation amount must be non-negative and no greater than
  `2^256-1`.
- Fee-estimate amounts receive the same U256 range validation before wire
  conversion.
- A BatchPayment submission under `WithLegacyV1` fails before signing and
  before any HTTP request with an error equivalent to
  `batch payment requires domain-separated v2 submission mode`.
- A node-returned transaction hash mismatch remains a terminal, non-retried
  error.

The SDK does not duplicate governance-dependent validation. Empty operations,
zero recipients, zero amounts, configured operation limits, fee-asset matching,
batch enablement, and committed-state availability remain authoritative server
checks and are returned through the existing HTTP error mechanism.

Memo strings receive no additional SDK policy validation. `EmptyMemo()` is a
valid canonical value.

## 9. Testing Strategy

### 9.1 Payload and RLP tests

- Assert the canonical field count and order after removing `max_fee`.
- Compare every absent/present `operations_hash`/`batch_id` combination with
  authoritative bytes emitted by the updated L1 generator.
- Specifically pin the empty `operations_hash` placeholder when only
  `batch_id` is present.
- Assert BatchPayment always encodes `[payload, memo]`.
- Test `DeriveBatchPaymentOperationsHash` against L1-generated expected hashes,
  including reordered operations and U256 boundary amounts.
- Assert a nil amount and an explicit zero amount produce identical operation
  RLP and identical derived hashes, while negative and wider-than-U256 values
  fail.

### 9.2 Memo tests

- Omitting `WithMemo` produces the exact `EmptyMemo()` wrapper.
- A populated `WithMemo` changes both signing hash and transaction hash.
- The v2 JSON body always contains all three memo fields.
- Remove tests asserting that BatchPayment rejects memo.

### 9.3 Oracle ownership and golden-vector conformance

The two Go fixture files have different sources and update procedures. They
must not be treated as one copy operation.

#### 9.3.1 Frozen native-v2 fixture

`testdata/native-v2-signing-vectors.json` is a byte-for-byte copy of
`l1client/docs/specs/fixtures/native-v2-signing-vectors.json`. Copy the current
L1 main fixture and verify exact equality for:

- `payload_rlp`
- `unsigned_transaction_rlp`
- `signing_hash`
- `signed_transaction_rlp`
- `transaction_hash`

The canonical base-vector hashes include:

```text
BatchPayment single signing hash:
0xbdd97785d3d9e286f0ae6d4154b02d072c546bcbd1330d353c89ddaca3d212b0

BatchPayment multi signing hash:
0xabc717c970472db933aff88bd2bee41d6e7c3c42dce74b9b3ac70f454e2e5f08
```

No Go-side regeneration is permitted for this frozen fixture.

#### 9.3.2 Extended prepare/authorize fixture

`testdata/prepare-authorize-hash-vectors.json` is generated by an L1 Rust
oracle, not copied from the frozen fixture. Its existing `_source` names
`crates/om-sdk/examples/export_go_sdk_native_v2_vectors.rs`, but that generator
currently exists only on the unmerged `l1client/sec/1038` branch and implements
the obsolete bare, `max_fee`-bearing BatchPayment shape. Updating this fixture
is blocked until the generator becomes an executable L1 main deliverable.

The L1 work item is:

1. Use `crates/om-sdk/examples/export_go_sdk_native_v2_vectors.rs` from commit
   `086b69df` as a behavioral source, then port and adapt it to current
   `l1client/main`. This is not a cherry-pick or mechanical file copy: `main`
   and `sec/1038` have diverged, and the generator was written against APIs
   from the branch containing additional commits that are not main ancestors.
   Audit every referenced payload type, native-signing trait, SDK signing API,
   fixture schema, dependency, and example target against the current main API
   before retaining it. Do not import unrelated `sec/1038` commits to make the
   generator compile.
2. Remove `BatchPayment.max_fee`, `BatchFeeLevel`, the max-fee numeric-boundary
   vectors, and max-fee pairwise coverage.
3. Change every BatchPayment signing input to
   `WithMemo<BatchPaymentPayload>`.
4. Add BatchPayment memo levels for default empty memo and populated memo, and
   include memo in the generator's finite-state coverage.
5. Emit authoritative cases for neither optional field, hash only, batch ID
   only, both fields, empty batch ID, and zero operations hash. The batch-ID-only
   case must pin the empty hash placeholder bytes.
6. Emit the expected canonical operations hash for BatchPayment cases so the Go
   derivation helper has a Rust oracle.
7. Retain operation ordering and amount boundary coverage, including zero and
   maximum U256 encoding cases, without treating every generated case as an
   admission-valid transaction.
8. Add Rust tests that fail if the required BatchPayment optional, memo,
   numeric, ordering, or pairwise coverage disappears.

This generator adaptation is the highest-uncertainty and highest-schedule-risk
step in the delivery. It sits at the start of the hard cross-repository gate,
so estimation must include API compatibility investigation, compilation, and
oracle validation rather than treating it as copying one source file.

Commit the updated generator in `l1client` before exporting the Go fixture.
Run it from that committed state with its full 40-character L1 commit as
`--source-commit`, then replace the Go fixture and re-pin `_source.commit` to
that exact commit. The generator commit and Go fixture commit are separate
cross-repository deliverables.

Go tests consume this regenerated fixture. They must not hand-calculate the
generated BatchPayment expected hashes or derive expected values through the
Go implementation under test.

#### 9.3.3 Hard cross-repository delivery gate

> **Correction (found during implementation, 2026-08-10):** the merge
> requirement below was explicitly waived by human decision: there is no L1
> PR and no merge into `l1client/main`. The extended fixture's oracle is the
> local, unpushed l1client branch `feat/go-sdk-vector-generator` at commit
> `ee4ce971644587c6903cb8a393371088f8279c56`, used directly as
> `--source-commit`. Steps 1 and 5 below (and the "Delivery is therefore
> strictly serial" framing) describe the plan as originally conceived, not
> what happened; read `_source.commit` as identifying that local branch
> revision, not a `l1client/main` commit.

The L1 generator is a hard prerequisite for the entire Go re-baseline, not only
for a fixture refresh. The existing Go fixture decoder declares
`batchFixturePayload.MaxFee` and constructs `BatchPaymentPayload.MaxFee` from
every generated BatchPayment vector. Removing the production `MaxFee` field
while retaining the old fixture and decoder makes the Go test package fail to
compile.

Delivery is therefore strictly serial:

1. Merge the updated generator and its Rust coverage tests into
   `l1client/main`.
2. Run the generator from that merged commit and record that exact commit in
   `_source.commit`.
3. Import the complete regenerated fixture into the Go SDK.
4. Update the Go fixture decoder, production BatchPayment code, and tests in
   the Go re-baseline PR.
5. Require the complete fixture-backed Go suite to be green before merging the
   Go PR.

The design does not permit an intermediate Go state that deletes the generated
BatchPayment vectors, skips them, or defers their coverage to a later PR. Local
development may proceed in parallel, but the L1 PR must merge first and the Go
PR cannot be considered green or mergeable until it consumes the generator
output from that merged L1 commit.

### 9.4 HTTP contract tests

- Assert BatchPayment posts only to `/v2/transactions/batch_payment`.
- Assert submit JSON contains `memo` and excludes `max_fee`.
- Assert the default and populated memo bodies separately.
- Assert fee estimation uses POST
  `/v1/transactions/batch_payment/estimate_fee` with the exact request body.
- Assert submit and estimate operation amounts both use quoted decimal strings.
- Assert direct `json.Marshal(BatchPaymentFeeEstimateRequest)` produces the same
  body as `GetBatchPaymentEstimateFee`, including lowercase keys and quoted
  decimal amounts.
- Decode `{ "fee": "...", "plan": null }` successfully.
- Decode BatchPayment transaction data without `max_fee`.
- Assert BatchPayment under a legacy-configured client returns the v2-only
  error with both default memo and explicit `WithMemo`, with zero HTTP requests.
- Assert CreateMultisig under a legacy-configured client returns its v2-only
  error with zero HTTP requests.
- Assert a legacy `TokenManageListPayload` with explicit memo and no
  `WithManageListKind` returns the ambiguous-operation error before the generic
  legacy-memo error, documenting the intentional precedence change.

### 9.5 Integration tests

Update the existing BatchPayment business flow to:

- Remove `MaxFee` from construction.
- Use domain-separated v2 mode.
- Cover default empty memo and at least one populated memo submission.
- Query the transaction and assert payload, memo, receipt, and execution-event
  data.
- Confirm the locally computed and server-returned transaction hashes match.

## 10. Expected File Scope

Production changes are expected in:

- `native_v2_batch.go` for the public operations-hash derivation helper and
  shared operation encoding helpers
- `transactions.go`
- `transactions_types.go`
- `transactions_payloads.go`
- `native_v2.go`
- `native_v2_encoding.go`
- `native_v2_prepare.go`
- `native_v2_requests.go`
- `memo.go`

Test and fixture changes are expected in:

- `api_v2_test.go`
- `native_v2_batch_test.go`
- `native_v2_conformance_test.go`
- `native_v2_prepare_test.go`
- `native_v2_wire_test.go`
- `transactions_test.go`
- `business_integration_test.go`
- `testdata/native-v2-signing-vectors.json`
- `testdata/prepare-authorize-hash-vectors.json`

Documentation changes are expected in:

- `README.md`
- `CHANGELOG.md`
- `MIGRATION.md`
- `docs/superpowers/specs/2026-07-27-go-sdk-v2-upgrade-design.md`
- Related implementation plans that still describe BatchPayment as bare,
  memo-incapable, or `max_fee`-bearing

The prerequisite L1 repository change is expected in:

- `crates/om-sdk/examples/export_go_sdk_native_v2_vectors.rs`
- The generator's Rust tests and crate metadata only where required to build
  and run that example on current `l1client/main`

The L1 generator change and Go SDK re-baseline must remain separate commits in
their respective repositories. The Go fixture records the committed L1 oracle
revision through `_source.commit`.

No unrelated transaction, token, receipt, or package-layout refactor is in
scope. The validation-error precedence change documented in section 5 is the
only intentional behavior change outside BatchPayment, and it affects invalid
legacy calls only; successful token behavior and wire output remain unchanged.

## 11. Release and Migration

This is a correction to a new BatchPayment API, not a compatibility migration.
It ships in the current v1 Go module and does not require a `/v2` module path.

Callers update by:

1. Removing `MaxFee` from `BatchPaymentPayload` construction.
2. Continuing to call `Transactions().BatchPayment`.
3. Optionally supplying `WithMemo`; omission uses `EmptyMemo()`.
4. Ensuring the client is not configured with `WithLegacyV1` for BatchPayment.
5. Calling `GetBatchPaymentEstimateFee` when a non-binding fee quote is needed.
6. Using `DeriveBatchPaymentOperationsHash` before populating the optional
   `OperationsHash` field.

## 12. Success Criteria

The re-baseline is complete when:

1. No production Go BatchPayment type, encoder, decoder, or request contains
   `max_fee`.
2. No production path encodes or signs a bare BatchPayment payload.
3. BatchPayment submit bodies always contain the required memo object.
4. BatchPayment cannot issue a legacy-v1 HTTP request.
5. The Go SDK matches current L1 BatchPayment golden vectors byte-for-byte.
6. The extended prepare/authorize fixture is reproducibly generated by a
   committed L1 main generator, and `_source.commit` identifies that exact
   oracle revision.
7. The L1 generator PR is merged before the Go re-baseline PR consumes its
   output; no skipped or temporarily deleted BatchPayment fixture coverage is
   accepted.

> **Correction (found during implementation, 2026-08-10):** criteria 6 and 7
> assumed the L1 generator would be merged into `l1client/main`. That merge
> was explicitly waived by human decision — there is no PR and no merge.
> `_source.commit` instead identifies the local, unpushed l1client branch
> `feat/go-sdk-vector-generator` at commit
> `ee4ce971644587c6903cb8a393371088f8279c56`, which is the actual oracle used
> and is reproducible from that branch. Read criterion 6 as satisfied by that
> committed local-branch revision, and criterion 7 as not applicable.

8. L1-generated vectors pin every BatchPayment trailing-option combination,
   default/populated memo behavior, and operations-hash derivation.
9. Submit and estimate requests use one quoted-decimal operation serializer;
   direct `json.Marshal` of the public estimate request produces the same wire
   body as the Client method.
10. `DeriveBatchPaymentOperationsHash` matches L1-generated expected hashes,
    with nil amounts explicitly equivalent to U256 zero.
11. Fee-estimate request and response contract tests match the L1 endpoint.
12. Transaction reads decode the new payload and memo without obsolete fields.
13. Legacy-mode BatchPayment returns its v2-only error before the generic memo
    error, BatchPayment/CreateMultisig both fail before HTTP I/O, and the
    documented operation-resolution-before-memo error precedence is tested for
    a v1-capable operation.
14. `gofumpt -l -w .`, `go test ./...`, and `golangci-lint run` complete
    successfully during implementation verification.
