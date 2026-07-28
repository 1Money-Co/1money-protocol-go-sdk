// Package onemoney is the Go SDK for the 1Money Network REST API. It exposes
// account, token, transaction, checkpoint, chain, pricing, and status reads plus
// signed transaction submission.
//
// # Submitting transactions
//
// Submission defaults to the domain-separated "native v2" scheme. Build a Signer
// (NewPrivateKeySigner, or any KMS/HSM implementation of Signer) and use the
// Transactions(), Tokens(), and Accounts() namespaces, which handle signing,
// encoding, endpoint selection, and server-hash verification internally. For
// offline / air-gapped signing, use PrepareTransaction -> Authorize -> Submit.
// WithLegacyV1 on the client opts a call back onto the legacy /v1 path.
//
// # Source layout
//
// The package is intentionally flat: one package, no subpackages. Files are
// grouped by responsibility:
//
//   - <domain>.go and <domain>_types.go hold each domain's client/namespace
//     methods and DTOs (accounts, tokens, transactions, checkpoints, chains).
//   - transactions_payloads.go and transactions_decode.go hold the polymorphic
//     transaction-response payloads and their discriminated JSON decoding.
//   - primitives.go and signature.go hold cross-domain byte primitives and the
//     Signature/Signer types shared by both the legacy and v2 APIs.
//   - legacy.go is the deprecated legacy-v1 compatibility surface (request
//     wrappers and exposed signing/hash helpers) and is not for new code.
//   - native_v2*.go is the unexported domain-separated v2 protocol engine:
//     canonical encoding and hashing, prepare/authorize, REST wire snapshots,
//     the submission pipeline, and multisig validation/derivation.
//
// The v2 protocol implementation lives entirely in this package with an
// unexported encoder and hasher; there is exactly one operation registry, one
// encoder, and one hash implementation.
package onemoney
