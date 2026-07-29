# Go SDK Prepare/Authorize Hash Coverage Design

**Date:** 2026-07-28

**Expanded coverage revision:** 2026-07-29

**Status:** Approved and implemented

**Repositories:**

- `l1client/` — Rust oracle and deterministic fixture exporter
- `1money-protocol-go-sdk/` — fixture consumer and public-API tests

## 1. Goal

Prove that the Go SDK produces the same native-v2 hashes as the Rust
implementation for:

```text
PrepareTransaction(payload).SigningHash()
PrepareTransaction(payload).Authorize(signature).TransactionHash()
```

The proof must exercise the Go SDK from public payload types and original field
values. Tests must not bypass Go payload construction by consuming
Rust-encoded RLP.

Coverage includes all 14 native operation types, every finite enum and boolean
combination, and the encoding equivalence classes created by optional fields,
vectors, nested vectors, byte arrays, numeric boundaries, memo values, and
operation-domain separation. Infinite value domains use explicit boundary
classes plus deterministic pairwise interaction coverage rather than an
unbounded Cartesian product.

## 2. Source of truth

The new Go compatibility vectors use the Rust production implementation as
their oracle.

For every vector, the Rust exporter:

1. constructs the concrete Rust payload struct from original typed fields;
2. computes the signing hash through the production native-v2 signing API;
3. signs that hash with a fixed test-only secp256k1 private key;
4. computes the authorized transaction hash through the production native-v2
   transaction-hash API;
5. exports the original payload fields, signature, signing hash, and
   transaction hash as JSON.

The exporter may RLP-encode internally because that is how the Rust production
hash implementation works. The exported JSON must not contain `payload_rlp`,
unsigned transaction RLP, or signed transaction RLP.

This makes the compatibility assertion:

```text
same original field values
    Rust production encoder/hash
        == Go production encoder/hash
```

It does not reduce the test to comparing two consumers of Rust-produced
encoded bytes.

## 3. Relationship to existing vectors

The existing `testdata/native-v2-signing-vectors.json` remains unchanged. Its
payload RLP originates from the Rust canonical encoder while its outer hashes
are independently verified by the Python verifier. It continues to provide
low-level specification conformance and collision coverage.

The new fixture has a different purpose:

`1money-protocol-go-sdk/testdata/prepare-authorize-hash-vectors.json`

It treats the Rust production implementation as the cross-SDK oracle and tests
the Go public `PrepareTransaction`/`Authorize` pipeline from original payload
fields.

The two layers are complementary:

1. independent specification vectors check the Rust core algorithm;
2. Rust production results check Go SDK interoperability.

The existing fixture is not rewritten, reinterpreted, or made dependent on the
new exporter.

## 4. Fixture schema

The fixture is a heterogeneous collection keyed by native operation:

```json
{
  "_source": {
    "repository": "l1client",
    "commit": "0123456789abcdef0123456789abcdef01234567",
    "generator": "crates/om-sdk/examples/export_go_sdk_native_v2_vectors.rs"
  },
  "vectors": [
    {
      "name": "batch_payment_operations_hash_absent_batch_id_present",
      "class": "batch_option",
      "operation": "BatchPayment",
      "operation_type": 14,
      "payload": {
        "chain_id": 1212101,
        "nonce": 14,
        "token": "0x0101010101010101010101010101010101010101",
        "operations": [
          {
            "recipient": "0x0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c",
            "amount": "1000"
          }
        ],
        "max_fee": "5000",
        "created_at": 1747785600,
        "operations_hash": null,
        "batch_id": "payroll-001"
      },
      "options": {},
      "authorization": {
        "r": "0x...",
        "s": "0x...",
        "v": 0
      },
      "expected": {
        "signing_hash": "0x...",
        "transaction_hash": "0x..."
      }
    }
  ]
}
```

### 4.1 Raw-field representation

`payload` contains the logical fields of the concrete Rust payload struct:

- unsigned fixed-width integers use JSON numbers;
- `U256` values use base-10 strings to avoid JSON-number precision loss;
- addresses, `B256`, public keys, and arbitrary bytes use `0x`-prefixed hex;
- Rust `Option::None` uses JSON `null`;
- Rust `Option::Some(value)` uses the value, including `""` for
  `Some(String::new())`;
- Rust `Vec` values use JSON arrays and preserve element order;
- strings remain UTF-8 strings;
- booleans remain JSON booleans.

The representation must preserve distinctions that affect canonical encoding,
especially:

- `None` versus `Some("")`;
- `None` versus `Some(B256::ZERO)`;
- empty versus non-empty vectors;
- vector element order;
- empty bytes versus non-empty all-zero bytes.

`options` carries public Go preparation choices that are not fields of the
inner payload:

- `memo` for memo-capable operations;
- `manage_list_kind` for the shared Go `TokenManageListPayload` type.

BatchPayment has no memo option.

### 4.2 Completeness and determinism

The exporter uses one fixed, test-only private key. Every signature must be a
valid canonical low-S signature for the vector's Rust signing hash.

The fixture records the exact source commit passed through the exporter's
required `--source-commit` argument. The exporter validates that it is 40
hexadecimal characters and writes it only as provenance; it does not change
the generated vector values. Running the exporter twice with the same source
commit must produce byte-identical JSON, including stable vector order and
field formatting.

The fixture is checked in as static Go test data. Go tests do not invoke Cargo,
Python, the network, or another repository.

## 5. Rust exporter

Add a test-only example in `l1client`:

`crates/om-sdk/examples/export_go_sdk_native_v2_vectors.rs`

The exporter uses the concrete payload structs and the public or production
Rust native-v2 hashing APIs. It must not duplicate RLP or hashing logic.

For each case it produces:

```text
Rust concrete payload
  ├─ serialize original fields into fixture.payload
  ├─ production signing hash
  ├─ deterministic valid signature
  └─ production authorized transaction hash
```

The exporter prints the complete fixture to stdout. It does not contain a
hard-coded path to the Go repository.

This is the only planned final change in `l1client`; no L1 production behavior,
REST API, payload definition, RLP implementation, or hash algorithm changes.

## 6. Go fixture consumer

The Go test loader switches on `operation` and decodes `payload` into the
corresponding public Go payload type:

| Operation | Go payload/options |
|---|---|
| Payment | `PaymentPayload`, optional `WithMemo` |
| TokenIssue | `TokenIssuePayload`, optional `WithMemo` |
| TokenMint | `TokenMintPayload`, optional `WithMemo` |
| TokenAuthority | `TokenAuthorityPayload`, optional `WithMemo` |
| TokenBlacklist | `TokenManageListPayload`, blacklist kind, optional memo |
| TokenWhitelist | `TokenManageListPayload`, whitelist kind, optional memo |
| TokenPause | `PauseTokenPayload`, optional `WithMemo` |
| TokenBurn | `TokenBurnPayload`, optional `WithMemo` |
| TokenClawback | `TokenClawbackPayload`, optional `WithMemo` |
| TokenMetadata | `UpdateMetadataPayload`, optional `WithMemo` |
| TokenBridgeAndMint | `TokenBridgeAndMintPayload`, optional `WithMemo` |
| TokenBurnAndBridge | `TokenBurnAndBridgePayload`, optional `WithMemo` |
| CreateMultiSig | `CreateMultiSigPayload`, optional `WithMemo` |
| BatchPayment | `BatchPaymentPayload`, no memo |

The loader must not create a generic `[]interface{}` RLP field list from JSON.
It must construct these public payload types so that the test covers:

- JSON fixture-to-Go field conversion;
- Go payload-to-operation mapping;
- Go canonical field ordering and RLP encoding;
- memo wrapping and manage-list disambiguation;
- public signing-hash calculation;
- public authorization encoding;
- public transaction-hash calculation.

Every successful vector executes:

```go
prepared, err := PrepareTransaction(payload, options...)
assertHexEqual(prepared.SigningHash(), vector.Expected.SigningHash)

authorized, err := prepared.Authorize(vector.Authorization)
assertHexEqual(authorized.TransactionHash(), vector.Expected.TransactionHash)
```

The test also recovers the fixed Rust test public key from each exported
signature and Go signing hash. A shape-valid but unrelated signature therefore
cannot satisfy the fixture test.

## 7. Canonical operation coverage

The fixture contains at least one canonical vector for every frozen native
operation type:

```text
1..14
```

The Go completeness test asserts:

- all 14 operation types are present;
- canonical vectors contain each operation exactly once;
- vector names are unique;
- every vector has an operation, original payload fields, authorization,
  signing hash, and transaction hash;
- no vector contains `payload_rlp` or another encoded-payload shortcut.

CreateMultiSig canonical vectors use real compressed secp256k1 public keys.
The synthetic compressed-key-shaped bytes in the older fixture are not reused
for this public-path test.

## 8. Encoding edge matrix

The edge matrix uses three coverage rules:

1. finite enums and booleans are exhaustively enumerated;
2. ordered, numeric, string, and byte domains cover explicit equivalence
   classes and boundaries;
3. interacting dimensions use a deterministic pairwise matrix, not the full
   Cartesian product of infinite business values.

Every successful edge vector asserts both the Rust signing hash and Rust
authorized transaction hash. This prevents an edge from being covered only in
the unsigned half of the public pipeline.

### 8.1 Machine-enforced coverage manifest

Coverage is asserted from decoded payload values, not inferred from vector
names. Go tests build semantic sets from the fixture and require equality with
the design matrix:

- exact operation IDs and enum/boolean tuples;
- exact numeric boundary values for every field;
- byte lengths, UTF-8 cases, and Option presence/value states;
- Vec cardinality, ordering, and nested value classes;
- every required pair in the BatchPayment interaction matrix.

Names remain stable review labels but are not accepted as proof that a vector
still contains the advertised edge. Adding a new enum variant or required
coverage cell must fail the manifest test until the Rust exporter supplies the
corresponding raw-field vector.

### 8.2 Finite enum and boolean matrices

Cover every legal encoded value:

| Payload | Complete matrix |
|---|---|
| `TokenAuthority` | `Grant` and `Revoke` crossed with all seven authority types |
| `TokenBlacklist` | `Add`, `Remove` |
| `TokenWhitelist` | `Add`, `Remove` |
| `TokenPause` | `Pause`, `Unpause` |
| `TokenIssue` | all four `(is_private, clawback_enabled)` boolean tuples |

The authority matrix contains all `2 × 7 = 14` tuples because both values are
encoded strings and neither mapping may be inferred from one representative.
TokenIssue boolean vectors also carry `decimals=0` and `decimals=255` across
the matrix so both uint8 boundaries are exercised without a separate Cartesian
product.

### 8.3 Memo values

Memo is represented by three strings rather than Rust `Option`, but empty
strings are the canonical unset values. Cover:

- all three fields empty;
- only `type` populated;
- only `format` populated;
- only `data` populated;
- all three fields populated;
- `data` containing multibyte UTF-8;
- exact 55/56-byte RLP string boundaries where permitted;
- valid per-field maximum-length values: type 128 bytes, format 64 bytes, and
  data 256 bytes.

Memo validation failures remain ordinary Go negative tests and are not
successful Rust golden vectors.

### 8.4 BatchPayment Option and pairwise interactions

`operations_hash` and `batch_id` are RLP trailing fields. Cover the complete
presence matrix:

| `operations_hash` | `batch_id` | Purpose |
|---|---|---|
| `None` | `None` | six-field base payload |
| `Some(hash)` | `None` | first trailing field only |
| `None` | `Some(non-empty)` | required empty placeholder before batch ID |
| `Some(hash)` | `Some(non-empty)` | both trailing fields |

Add two value-shape edges:

- `None` plus `Some("")`, proving that a present empty trailing string is not
  treated as absence;
- `Some(B256::ZERO)` plus `None`, proving that 32 zero bytes are not treated as
  an absent field.

In addition, the fixture contains a deterministic pairwise matrix across:

- Option state: neither, hash only, ID only, both;
- operations: empty, singleton, multiple forward, multiple reversed;
- operation amount: ordinary, zero, `U256::MAX`;
- max fee: ordinary, zero, `U256::MAX`.

Every semantically applicable pair of levels from different dimensions appears
together in at least one successful Rust vector. Amount levels apply only when
operations are non-empty, and forward/reversed order applies only to the
multiple-operation shape; the manifest excludes these impossible pairs rather
than pretending they are covered. `Some("")` and `Some(B256::ZERO)` remain
separate value-shape edges and are not treated as new presence levels.

The temporary `testdata/batch-payment-optional-vectors.json` is folded into the
new focused fixture and deleted after equivalent coverage exists.

### 8.5 Vector and nested-vector fields

#### BatchPayment `operations`

- empty vector;
- one operation;
- multiple operations;
- the same operations in reversed order;
- operation amount zero;
- operation amount `U256::MAX`.

The empty case tests hash construction only; it does not assert that a node
would accept an empty payment batch as a valid business transaction.

#### TokenMetadata `additional_metadata`

- empty vector;
- one entry;
- multiple entries;
- the same entries in reversed order;
- repeated key;
- empty key/value when the Rust payload type can represent it.

The order-pair vectors must produce different hashes.

#### CreateMultiSig `signers`

Successful vectors cover:

- one valid signer;
- multiple valid signers;
- different non-zero weights;
- the same valid signer set in different supplied order;
- valid public keys using both compressed SEC1 prefixes `0x02` and `0x03`;
- signer weight `255`;
- threshold `1`;
- a valid threshold greater than `255`;
- the maximum valid u16 threshold `65535`, backed by distinct signers whose
  total weight is exactly `65535`.

The maximum-threshold vector uses 257 deterministic distinct valid public keys
with weight 255. Only public keys are exported; the deterministic test scalars
used to derive them remain local to the exporter.

The fixture preserves the payload's supplied signer order. It does not sort the
JSON in the exporter or Go loader.

Empty signers, invalid compressed keys, duplicate keys, zero weight, and an
invalid threshold are Go rejection tests rather than successful golden
vectors.

### 8.6 Variable-length string and byte boundaries

For every variable-length string or byte field that participates in native-v2
payload RLP, cover the field independently with:

- empty;
- one byte;
- multibyte UTF-8 for string fields;
- exactly 55 and 56 UTF-8 bytes;
- exactly 255 and 256 UTF-8 bytes where the field's validation limit permits;
- the field's lower maximum when its valid limit is below 256 bytes.

The fields are:

- TokenIssue `symbol` and `name`;
- TokenMetadata `name`, `uri`, and each metadata `key` and `value`;
- TokenBridgeAndMint `source_tx_hash` and `bridge_metadata`;
- TokenBurnAndBridge `destination_address`, `bridge_metadata`, and
  `bridge_param`;
- BatchPayment present `batch_id`;
- memo `type`, `format`, and `data`, subject to section 8.3 limits.

`bridge_param` additionally covers bytes with a leading zero and non-empty
all-zero bytes. Byte lengths, not Unicode scalar counts, define every boundary.
Vectors change one target field at a time unless that vector is explicitly
part of the BatchPayment pairwise matrix.

### 8.7 Numeric boundaries

Every U256-bearing field has independent successful Rust vectors for:

- `U256::ZERO`;
- ordinary U256 values;
- `U256::MAX`.

The U256-bearing fields are:

- Payment `Value`;
- BatchPayment operation `Amount` and `MaxFee`;
- TokenMint, TokenBurn, TokenClawback, and TokenBridgeAndMint `Value`;
- TokenAuthority `Value`;
- TokenBurnAndBridge `Value` and `EscrowFee`.

Semantically distinct fixed-width fields cover zero and their maximum legal
value:

- common `chain_id` and `nonce` using representative payloads;
- TokenIssue `decimals`;
- BatchPayment `created_at`;
- TokenBridgeAndMint `source_chain_id`;
- TokenBurnAndBridge `destination_chain_id`;
- CreateMultiSig `weight` and `threshold`, subject to valid multisig rules.

For every U256-bearing Go field, table-driven Go negative tests cover:

- a negative `big.Int`, which must fail preparation;
- `2^256`, which must fail preparation.

The existing `nil *big.Int -> U256 zero` behavior is preserved and compared
with an explicit-zero golden case.

### 8.8 Operation-domain collisions

Export raw-field vectors for the known collision pairs:

- Payment versus TokenMint;
- TokenBlacklist versus TokenWhitelist;
- TokenPause versus TokenBurn.

Where the inner payload RLP collides, both public signing hashes and authorized
transaction hashes must still differ because `operation_type` is part of the
native-v2 domain.

## 9. AuthorizedTransaction-specific edges

In addition to the Rust-exported successful vectors, focused Go tests cover:

- valid low-S signatures with both parity values `v=0` and `v=1`;
- rejection of `v` outside `0/1`;
- rejection of zero `r` or `s`;
- rejection of `r` or `s` greater than or equal to the secp256k1 order;
- rejection of high-S signatures;
- distinct valid signatures over one prepared transaction producing distinct
  transaction hashes;
- `TransactionHash()` returning a defensive copy.

Invalid signatures assert errors and do not need Rust golden vectors.

## 10. Production validation

Add one unexported Go helper:

```go
func validateU256(name string, value *big.Int) error
```

Rules:

- `nil` is valid and represents zero;
- negative values return an error;
- `BitLen() > 256` returns an error;
- zero through `2^256 - 1` are valid.

Validation runs before payload RLP or wire-field construction for every field
listed in section 8.7. This is the only planned production behavior change. It
rejects values that cannot be represented by the Rust `U256` type.

## 11. File plan

### `l1client/`

Create:

- `crates/om-sdk/examples/export_go_sdk_native_v2_vectors.rs`
  - construct raw typed payloads;
  - calculate hashes with Rust production APIs;
  - sign with the fixed test-only key;
  - serialize original fields and expected results.

No L1 production file is modified.

### `1money-protocol-go-sdk/`

Modify:

- `native_v2_prepare.go`
  - call shared U256 validation during payload resolution.
- `native_v2_prepare_test.go`
  - consume the Rust-exported fixture through public payload types;
  - assert the 14 canonical public signing and transaction hashes;
  - derive and assert the semantic coverage manifest from decoded raw fields;
  - cover signature and transaction-hash edges.
- `native_v2_batch_test.go`
  - retain focused structural assertions;
  - read BatchPayment cases from the consolidated fixture;
  - assert the Option/Vec/numeric pairwise matrix.

Create:

- `testdata/prepare-authorize-hash-vectors.json`

Delete after migration:

- `testdata/batch-payment-optional-vectors.json`

## 12. Generation and test flow

From `l1client/`:

```bash
cargo run -p onemoney-protocol --example export_go_sdk_native_v2_vectors \
  -- --source-commit "$(git rev-parse HEAD)" \
  > ../1money-protocol-go-sdk/testdata/prepare-authorize-hash-vectors.json
```

Generation is an explicit maintainer action, not part of `go test`. A fixture
diff is reviewed like a code change. `_source.commit` identifies the exact
Rust production signing/hash implementation used as the oracle. When the
exporter and fixture are introduced in the same reviewed change, that commit
does not need to contain the exporter itself; the exporter source and fixture
diff must be reviewed together. Unrelated untracked files do not affect
provenance.

Implementation proceeds in this order:

1. add exporter structure and deterministic source metadata;
2. add all 14 canonical raw payloads;
3. add the exhaustive enum/boolean matrices;
4. add Option, Vec, byte, memo, numeric, collision, and exact RLP-boundary
   vectors;
5. add the deterministic BatchPayment pairwise matrix and multisig boundaries;
6. generate the static Go fixture;
7. add the type-specific Go fixture decoder and semantic coverage assertions;
8. add the public Prepare/Authorize golden test;
9. add invalid U256 and invalid-signature tests;
10. implement only the validation required by those negative tests;
11. remove temporary BatchPayment fixture after coverage parity is proven;
12. run targeted and repository-level verification.

## 13. Acceptance criteria

The work is complete when:

- Rust is the sole producer of the new fixture's successful expected hashes;
- every fixture payload consists of original fields, not RLP or encoded
  transaction bytes;
- fixture generation is deterministic;
- Go reconstructs concrete public payload types for every vector;
- all 14 operation types pass both public signing-hash and transaction-hash
  comparison;
- all 14 TokenAuthority action/type tuples, both list actions for blacklist and
  whitelist, both pause actions, and all four TokenIssue boolean tuples pass;
- the full BatchPayment `Option` presence matrix and special empty/zero cases
  pass;
- every required pair across BatchPayment Option, Vec, operation-amount, and
  max-fee levels is present and passes;
- every relevant `Vec` field has empty, singleton, multiple, and ordering
  coverage where valid for that public path;
- every U256 field has independent zero and maximum Rust golden cases;
- every listed variable-length field has its required byte-length boundary
  cases, including exact RLP 55/56 transitions;
- multisig vectors include both compressed-key prefixes, maximum signer weight,
  a threshold greater than 255, and maximum valid u16 threshold;
- byte-vector, memo, numeric-boundary, and collision cases pass;
- coverage assertions derive these facts from decoded values rather than
  trusting vector names;
- exported signatures recover the fixed Rust test public key;
- invalid Go-only values are rejected without being represented as successful
  Rust vectors;
- no existing low-level conformance coverage regresses.

## 14. Verification

Targeted Rust checks:

```bash
cargo check -p onemoney-protocol --example export_go_sdk_native_v2_vectors
cargo run -p onemoney-protocol --example export_go_sdk_native_v2_vectors
```

Required Go SDK checks:

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

For the final cross-repository change, run the applicable repository quality
checks documented by each repository's local guidance. If a required tool is
unavailable, report the exact command not run.
