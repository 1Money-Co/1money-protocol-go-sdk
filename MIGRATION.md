# Migrating from legacy v1 to domain-separated v2

As of **v1.2.0** the SDK submits transactions with the domain-separated
("native v2", 1Money L1 issue #1038) signing scheme **by default**. The legacy
v1 API still works unchanged (it is deprecated), so you can migrate at your own
pace. This is a short how-to plus an explanation of how the v2 signing hash is
built and why.

## Why v2 exists

Legacy v1 signs `keccak256(rlp(payload))` — the signature commits only to the
payload's field bytes. If two different operations encode to the same bytes,
the verifier cannot tell them apart, so a signature captured for one operation
could be **replayed as a different operation** (cross-type replay).

v2 fixes this by binding every signature to a fixed protocol **domain** and an
explicit **operation type** before hashing. A signature for a payment can never
be reinterpreted as a token mint, a different operation, or a different
protocol.

## Migrate your calls

The v2 API hides all signing detail: build a `Signer` once, then pass a payload
and the signer to a namespace method. The SDK signs, encodes, picks the
endpoint, and verifies the server-returned hash for you.

```go
// Before (deprecated, still works): hand-sign + build a *Request wrapper.
sig, _ := client.SignMessage(payload, privateKeyHex)
resp, _ := client.SendPayment(ctx, &onemoney.PaymentRequest{
    PaymentPayload: payload, Signature: *sig,
})

// After: a Signer + a namespace method (domain-separated v2 by default).
signer, _ := onemoney.NewPrivateKeySigner(privateKeyHex)
resp, _ := client.Transactions().Payment(ctx, payload, signer)
```

Method mapping (all take `(ctx, payload, signer, ...SubmitOption)`):

| Legacy method | v2 method |
| --- | --- |
| `SendPayment` | `Transactions().Payment` |
| `IssueToken` | `Tokens().Issue` |
| `MintToken` | `Tokens().Mint` |
| `BurnToken` | `Tokens().Burn` |
| `BridgeAndMintToken` | `Tokens().BridgeAndMint` |
| `BurnAndBridgeToken` | `Tokens().BurnAndBridge` |
| `GrantTokenAuthority` (Grant) | `Tokens().GrantAuthority` |
| `GrantTokenAuthority` (Revoke) | `Tokens().RevokeAuthority` |
| `SetTokenBlacklist` | `Tokens().ManageBlacklist` |
| `SetTokenWhitelist` | `Tokens().ManageWhitelist` |
| `PauseToken` (Pause/Unpause) | `Tokens().Pause` / `Tokens().Unpause` |
| `UpdateTokenMetadata` | `Tokens().UpdateMetadata` |
| — (new) | `Transactions().BatchPayment`, `Tokens().Clawback`, `Accounts().CreateMultisig` |
| `GetTokenMetadata(hex)` | `Tokens().Metadata(common.Address)` |
| `PrivateKeyToAddress(hex)` | `NewPrivateKeySigner(hex).Address()` |

Notes:

- `GrantAuthority`/`RevokeAuthority` and `Pause`/`Unpause` set the payload
  `Action` for you — you no longer set it by hand.
- Attach a signed memo with `onemoney.WithMemo(memo)`. Batch payments and the
  legacy path carry no memo, so passing `WithMemo` there returns an error rather
  than dropping it silently.

## How the v2 signing hash is built

You do **not** build any of this yourself — the SDK does. This section explains
what it computes so the scheme is auditable and so custom signers know exactly
what they are signing.

The digest a signer signs is:

```
signing_hash = keccak256( rlp([ DOMAIN, op_type, descriptor, payload_rlp ]) )
```

Each element:

- **`DOMAIN`** — the frozen ASCII string `"1money.native.transaction.v2"`,
  encoded as an RLP byte-string. Fixing the domain binds the signature to this
  protocol and version.
- **`op_type`** — a small integer from the frozen operation registry, encoded
  as a minimal RLP integer. It names the exact operation and is what prevents
  cross-type replay:

  | op | operation | op | operation |
  | --- | --- | --- | --- |
  | 1 | Payment | 8 | TokenBurn |
  | 2 | TokenIssue | 9 | TokenClawback |
  | 3 | TokenMint | 10 | TokenMetadata |
  | 4 | TokenAuthority | 11 | TokenBridgeAndMint |
  | 5 | TokenBlacklist | 12 | TokenBurnAndBridge |
  | 6 | TokenWhitelist | 13 | CreateMultiSig |
  | 7 | TokenPause | 14 | BatchPayment |

- **`descriptor`** — the authorization descriptor. Single-signature is `[0]`;
  a multisig-authorized operation is `[1, account]`. (The current public API
  produces single-signature submissions.)
- **`payload_rlp`** — the canonical business payload, embedded as one opaque RLP
  byte-string. For a memo-capable operation it is `rlp([ payload_fields, memo ])`
  where `memo` is the 3-element list `[type, format, data]` (three empty strings
  when there is no memo). For batch payment (which carries no memo) it is just
  `rlp(payload_fields)`. Field order per operation is frozen and validated
  byte-for-byte against the L1 golden vectors.

After signing, the public **transaction hash** appends the proof and re-hashes:

```
transaction_hash = keccak256( rlp([ DOMAIN, op_type, descriptor, payload_rlp, proof ]) )
```

For a single signature the proof is `[r, s, v]`. The SDK verifies the
server-returned hash against this locally computed hash and fails closed on any
mismatch (it never retries a v2 submission under v1 — that could double-spend a
nonce).

## Offline / KMS / HSM signing

If signing happens out of band, drive the same pipeline yourself:

```go
prep, _ := onemoney.PrepareTransaction(payload)   // offline, no network
digest  := prep.SigningHash()                     // the 32-byte signing_hash above
sig     := signExternally(digest)                 // -> onemoney.Signature{R, S, V}

authorized, _ := prep.Authorize(sig)              // holds transaction_hash
resp, _       := client.Submit(ctx, authorized)   // submit + hash-verify
```

The one-step namespace methods run on this exact pipeline internally, so both
paths produce identical bytes and hashes.

### Signer contract

A custom `Signer` must sign the digest **as given** — no extra hashing, no
`"\x19Ethereum Signed Message"` prefix — and return the raw ECDSA components:

- `V` is the **0/1 y-parity**, never the legacy Ethereum 27/28.
- `R` and `S` are the scalars in `[1, N)`.
- `S` must be canonical **low-S** (`S ≤ N/2`).

`Authorize` validates all three exactly as the node does, so a signer that emits
a high-S or 27/28 signature fails fast with a clear error. The built-in
`NewPrivateKeySigner` already produces conforming signatures.

## Staying on legacy v1

To keep submitting on the `/v1` path during the migration window, opt in
explicitly per client:

```go
client := onemoney.NewClientWithOpts(onemoney.WithLegacyV1())
```

A network may also refuse v2 (or v1); check with `client.GetNativeWriteStatus`
(`GET /api/status`) and compare `NativeWriteMode`. A future major release
(`v2.0.0`, `/v2` module path) will remove the deprecated legacy API.
