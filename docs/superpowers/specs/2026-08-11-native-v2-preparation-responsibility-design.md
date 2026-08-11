# Native V2 Preparation Responsibility Design

**Date:** 2026-08-11

**Status:** Approved for implementation

## 1. Context

The BatchPayment v2 re-baseline introduced three concepts that are individually
necessary but currently overlap in ways that obscure their responsibilities:

- canonical encoding, which is defined for payloads the node would reject;
- public preparation, which must reject statically invalid submissions before
  signing; and
- fee estimation, which is an unsigned server query rather than transaction
  admission.

The extended L1 fixture contains 213 canonical signing vectors. Twenty-two
BatchPayment vectors intentionally encode inadmissible payloads so that empty
operation lists, zero amounts, overflowing totals, and non-canonical
`operations_hash` values remain covered by an external oracle. The current
oracle test routes all 213 vectors through the encoding-only
`prepareCanonical` entry point merely because those 22 cannot pass
`PrepareTransaction`. That prevents the L1 oracle from protecting the public
preparation pipeline for the other 191 vectors.

The same review found two adjacent responsibility leaks: endpoint lookup can
produce an empty v2 path without an error, and the unsigned estimate method
duplicates transaction-admission rules even though the estimate service is the
authority for its own evolving request semantics.

## 2. Goals

1. Give canonical encoding, public preparation, admission rejection, endpoint
   mapping, and fee estimation one explicit responsibility each.
2. Keep all 213 vectors as L1-oracle checks of canonical signing and transaction
   hashes.
3. Run every admission-valid vector through the public `PrepareTransaction`
   pipeline and compare it with the same L1 oracle.
4. Prove that every encoding-only vector is rejected by the public preparation
   gate for the intended reason.
5. Fail any missing v2 endpoint mapping before signing or constructing a
   submittable transaction.
6. Match the node's BatchPayment static-validation order.
7. Make the estimate endpoint server-authoritative for all encodable requests.

## 3. Non-goals

- Changing canonical RLP, signing hashes, transaction hashes, REST request
  bodies, or operation type numbers.
- Relaxing the pre-signing BatchPayment admission gate.
- Supporting legacy BatchPayment submission.
- Moving governance-dependent BatchPayment rules into the SDK.
- Recording or requiring an `l1client` repository commit in the vendored
  fixture.
- Making invalid U256 values serializable. Negative and wider-than-256-bit
  amounts have no valid protocol wire representation and remain local encoding
  errors.

## 4. Responsibility Boundaries

### 4.1 Payload resolution and encodability

`resolvePayloadOp` remains responsible for:

- recognizing the supported public payload types;
- disambiguating operations that share a Go payload type;
- rejecting numeric values that cannot be represented as U256; and
- producing the operation type, canonical payload field list, and wire fields.

It does not decide whether a canonically encodable payload is admissible.

### 4.2 Endpoint mapping

`pathsForOp` becomes a fallible mapping. A recognized mapping may have:

- a non-empty v2 path and a non-empty v1 path: v1/v2-capable;
- a non-empty v2 path and an empty v1 path: intentionally v2-only.

An operation with no mapping returns a distinct error. It is never represented
as two empty strings and is never classified as v2-only.

`opFromPayload` propagates this mapping error. Because both online submission
and offline `PrepareTransaction` resolve through `opFromPayload`, a missing v2
path fails before a signing hash is exposed, before an HSM/KMS call, and before
HTTP. No later v2-only check needs to infer whether an empty v1 path came from a
missing mapping.

### 4.3 Canonical encoding

`prepareCanonical` remains an unexported encoding-only test entry point. It:

- resolves an encodable payload and endpoint mapping;
- constructs the canonical memo-wrapped payload RLP;
- computes the signing hash; and
- builds the same prepared representation used by authorization.

It deliberately skips memo and operation admission validation. It exists only
because canonical bytes are defined for inputs that cannot be submitted. No
production entry point calls it directly.

### 4.4 Public preparation

`PrepareTransaction` and online namespace submission share
`prepareFromPayload`. The gates execute in this order:

1. payload resolution, U256 encodability, and endpoint mapping;
2. memo validation;
3. operation-specific static admission validation;
4. canonical preparation from the already-resolved operation.

This order matches the corresponding node stages: JSON decoding precedes the
verifier, the verifier checks memo before operation-specific rules, and the
BatchPayment verifier checks all recipients and amounts before
`operations_hash`, then checks total overflow.

### 4.5 BatchPayment admission phases

The BatchPayment submission gate is split into ordered phases:

1. operations must be non-empty;
2. scan every operation for a zero recipient or non-positive amount;
3. when present, compare `operations_hash` with the canonical operations hash;
4. fold all amounts and reject U256 total overflow.

Overflow is not checked inside the recipient/amount scan. This ensures a later
invalid recipient or amount wins over an earlier running-total overflow, and an
`operations_hash` mismatch wins over overflow, matching the node verifier.

U256 encodability remains the earlier payload-resolution gate and is not
duplicated as an admission-order decision.

### 4.6 Fee estimation

`GetBatchPaymentEstimateFee` validates no BatchPayment admission rules locally.
Every request whose fields have a valid wire representation is posted to:

```text
POST /v1/transactions/batch_payment/estimate_fee
```

The server remains authoritative for empty batches, recipients, zero amounts,
total overflow, enablement, operation limits, fee-asset configuration, and any
future estimate semantics. This is intentional: estimation performs no signing,
so duplicating admission rules saves no key use and can only create stale local
false negatives.

`BatchPaymentFeeEstimateRequest.MarshalJSON` continues to reject negative and
wider-than-U256 amounts. That is encoding validation, not admission validation.

## 5. Fixture Contract

### 5.1 Canonical oracle

Every one of the 213 fixture vectors passes through `prepareCanonical` and
asserts its L1-generated:

- signing hash;
- transaction hash; and
- recovered signer.

This test is named and documented as canonical-encoding conformance. It makes no
claim about public admissibility.

### 5.2 Public-prepare expectations

The SDK-owned `_fixture` metadata gains:

```json
"public_prepare_rejections": {
  "batch_operations_empty": "operations must not be empty",
  "batch_operation_amount_zero": "amount must be greater than 0"
}
```

The complete map contains the 22 encoding-only vector names and the stable
error substring expected from `PrepareTransaction`. Absence from this map means
the vector is expected to be accepted by public preparation.

The metadata is test intent owned by this SDK. It is not an L1 hash oracle,
does not record generator provenance, and does not alter any expected hash.

Tests must not derive this classification by calling the Go admission validator
or by trying `PrepareTransaction` and falling back after an error. Either would
allow a gate regression to change the test route and mask itself. Operation-wide
classification such as `operation == BatchPayment` is also forbidden because
21 BatchPayment vectors are admission-valid.

### 5.3 Public oracle coverage

For every vector absent from `public_prepare_rejections`:

1. call exported `PrepareTransaction(payload, options...)`;
2. compare the signing hash with the L1 oracle;
3. authorize with the fixture signature;
4. compare the transaction hash with the L1 oracle; and
5. recover and compare the signer.

At the current fixture size this covers 191 vectors, including 21 valid
BatchPayment vectors.

For every vector present in `public_prepare_rejections`, call
`PrepareTransaction` and assert rejection with the declared error substring.
At the current fixture size this covers 22 BatchPayment vectors.

Completeness guards require that:

- every rejection-map name exists exactly once in the fixture;
- every rejected vector is BatchPayment;
- the rejection map contains exactly the current 22 reviewed names;
- every non-rejected vector succeeds through public preparation; and
- canonical oracle coverage still consumes all 213 vectors.

The two BatchPayment-specific optional-field tests continue using
`prepareCanonical`, because their sole responsibility is encoding optional
fields, including deliberately non-canonical hashes.

## 6. Error Semantics

Endpoint mapping errors are distinct from capability errors:

```text
unmapped native operation 15: no domain-separated v2 endpoint configured
batch payment requires domain-separated v2 submission mode
```

The first is an SDK implementation/configuration error. The second is a caller
selecting legacy mode for an intentionally v2-only operation.

BatchPayment static errors follow node precedence:

```text
empty operations
→ first invalid recipient/amount after scanning in order
→ operations_hash mismatch
→ total overflow
```

Estimate requests return server HTTP/business errors for encodable but invalid
quote inputs. Only local wire-encoding errors prevent the HTTP request.

## 7. Test Changes

### 7.1 Endpoint mapping

- Assert all 14 current operations have non-empty v2 paths.
- Assert BatchPayment and CreateMultisig alone have empty v1 paths.
- Assert an unknown operation returns the unmapped-operation error.
- Assert mapping failure occurs before a signer is called and before HTTP.

### 7.2 Validation order

- An early running-total overflow followed by a later zero recipient reports
  the recipient error.
- A payload containing both a mismatched `operations_hash` and total overflow
  reports the hash mismatch.
- A payload containing only total overflow reports the node-equivalent generic
  overflow error.

### 7.3 Oracle layering

- Canonical conformance: 213/213 vectors through `prepareCanonical`.
- Public conformance: 191/213 vectors through `PrepareTransaction` and the same
  oracle assertions.
- Admission rejection: 22/213 vectors rejected according to explicit fixture
  metadata.
- Injecting or simulating dropped memo forwarding in `prepareFromPayload` must
  fail the public conformance suite on populated-memo vectors.

### 7.4 Fee estimate boundary

- Negative and wider-than-U256 amounts fail locally with zero HTTP requests.
- Empty operations, zero recipient, zero amount, and overflowing total are
  serialized and sent rather than locally rejected.
- The server response/error is returned through the existing HTTP mechanism.

## 8. Documentation

Update the BatchPayment design, README, CHANGELOG, and migration guidance where
they currently claim `GetBatchPaymentEstimateFee` applies SDK-side static
admission validation. Clarify that submission still validates static rules
locally, while fee estimation delegates all encodable request semantics to the
server.

Update `testdata/README.md` to describe the canonical/public/rejection test
layers and the SDK-owned `public_prepare_rejections` metadata.

## 9. Success Criteria

1. No mapped operation can produce an empty v2 endpoint without an error.
2. Missing mapping fails before signing and HTTP in both online and offline
   preparation flows.
3. All 213 vectors retain canonical L1-oracle coverage.
4. All 191 admission-valid vectors, including 21 BatchPayment vectors, retain
   L1-oracle coverage through exported `PrepareTransaction`.
5. All 22 encoding-only vectors are explicitly classified and rejected by
   exported `PrepareTransaction` for their declared reason.
6. No test dynamically chooses its layer from the Go validator's result.
7. Submission static-validation precedence matches the node verifier.
8. Estimate requests are locally rejected only when they cannot be encoded;
   other business validity is server-authoritative.
9. Existing successful request bodies, signing hashes, transaction hashes, and
   public method signatures remain unchanged.
10. `go test ./...`, `go test -race ./...`, `go vet ./...`, formatting checks,
    and `git diff --check` pass.
