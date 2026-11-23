# Test vectors

These fixtures are **self-contained and maintained by this SDK**. Running the
test suite never requires a checkout of `l1client` or any other repository, and
there is no regeneration step at release time.

| File | What it pins |
|---|---|
| `native-v2-signing-vectors.json` | The canonical native-v2 envelope for operations 1 to 18: `payload_rlp`, `unsigned_transaction_rlp`, `signing_hash`, `signed_transaction_rlp`, `transaction_hash`. 36 base vectors + 14 supplemental, copied from l1client `4e8acd8b6` `docs/specs/fixtures/native-v2-signing-vectors.json`. |
| `prepare-authorize-hash-vectors.json` | Extended per-operation coverage for canonical encoding and the public `PrepareTransaction` -> `Authorize` pipeline: structured payloads, submit options, authorization proofs, and expected `signing_hash` / `transaction_hash`. 213 vectors, 43 of them BatchPayment, which additionally carry `expected.operations_hash`. |
| `multisig-address-vectors.json` | Multisig account address derivation. |
| `b256-oracle/b256_oracle.tsv` | How the Rust SDK decodes a hash in a node answer: for each input, whether `B256::from_str` and serde decode it, and the hash, as printed by [`b256-oracle/main.rs`](b256-oracle/main.rs) built with alloy-primitives 1.5.7 and const-hex 1.18.1, the versions in l1client `dc5cffa0b` `Cargo.lock`. To regenerate, copy that `Cargo.lock` next to [`b256-oracle/Cargo.toml`](b256-oracle/Cargo.toml), run `cargo run --offline`, and check that the lockfile still names those two versions. |

## The one rule

**An expected value in these files is an external oracle. Never recompute one
from this SDK's implementation.**

A hash produced by the code under test, compared against a hash produced by the
same code, proves nothing. Every expected value here was calculated
independently of this SDK, against the protocol contract named in each file's
`_fixture.protocol_contract` field.

Changing an expected value is therefore not a test fix — it is a deliberate
statement that the protocol contract changed. Make it a reviewed change,
together with whatever test consumes it, and update
`_fixture.protocol_contract`.

## BatchPayment coverage

`prepare-authorize-hash-vectors.json` covers the trailing-optional-field matrix
that the canonical envelope fixture does not reach. Six named classes, each with
a `_memo`-suffixed populated-memo companion:

| Vector | `operations_hash` | `batch_id` |
|---|---|---|
| `batch_option_neither` | absent | absent |
| `batch_option_hash_only` | present | absent |
| `batch_option_id_only` | absent | present |
| `batch_option_both` | present | present |
| `batch_option_empty_id` | absent | present, empty string |
| `batch_option_zero_hash` | all-zero hash | absent |

`batch_option_id_only` is the case that pins the RLP empty-string placeholder:
with `operations_hash` absent before a present `batch_id`, the absent slot
encodes as `0x80` rather than being dropped. The fixture deliberately carries no
`payload_rlp` for these, so the placeholder is pinned **end-to-end** through
`expected.signing_hash` — an encoder that dropped or zero-filled that slot
produces a different signing hash and fails.

Pairwise coverage crosses the optional-field class against operation count,
operation amount (including zero and maximum U256), and memo level.

## Canonical and public-entry coverage

All 213 extended vectors pin canonical encoding through the internal
encoding-only helper. Of those, 191 are admission-valid and also pass through
the exported `PrepareTransaction` API before their hashes are compared with the
same oracle. The remaining 22 deliberately encode payloads that public
preparation must reject, such as an empty list, a zero amount, an overflowing
total, or a mismatched operations hash.

Those 22 names and their expected public errors are listed explicitly in
`_fixture.public_prepare_rejections`. Tests must never discover this split by
calling the Go validator under test: doing so would let a validator regression
silently reclassify a vector and erase the public-entry oracle coverage it was
supposed to protect.

## Guards

These live in the test suite, not here, and exist so the fixture cannot silently
stop pulling its weight:

- `TestBatchPaymentOptionalGoldenVectors` requires all thirteen named
  BatchPayment vectors and fails if one disappears.
- `TestBatchPaymentPairwiseGoldenCoverage` asserts the observed factor crossings
  cover the required set.
- `TestDeriveBatchPaymentOperationsHashMatchesRustOracle` fails if **no** vector
  carried an `expected.operations_hash`, so an emptied field cannot pass
  vacuously.
- `TestCanonicalPrepareAndAuthorizeMatchRustGoldenVectors` consumes all 213
  vectors through the encoding-only layer.
- `TestPublicPrepareAndAuthorizeMatchRustGoldenVectors` consumes all 191
  admission-valid vectors through the exported public entry point.
- `TestEncodingOnlyVectorsAreRejectedByPublicPrepare` requires the 22 explicit
  encoding-only vectors to fail public preparation with their expected reason.
- `TestNativeV2Conformance` pins the canonical envelope for operations 1 to
  18 against `native-v2-signing-vectors.json`.
