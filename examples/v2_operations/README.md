# Native-v2 submission examples

This directory shows how to submit every public native-v2 operation, both with
an explicit business memo and with the SDK's default empty memo. It also includes
the lower-level prepare, sign, authorize, and submit flow for offline signers,
KMS, HSM, and MPC integrations.

## Memo behavior

Every native-v2 transaction contains a signed memo object. There is no v2 wire
format that omits it.

- If `WithMemo` is omitted, the SDK uses the canonical `EmptyMemo`: `type`,
  `format`, and `data` are all empty strings.
- If `WithMemo` is supplied, the memo is validated before signing and becomes
  part of both the signing hash and the final transaction hash.
- In these examples, `withoutMemo` means "without an explicit business memo";
  it does not mean that the memo field is absent from the request.

Default `EmptyMemo`:

```go
response, err := client.Transactions().Payment(ctx, payload, signer)
```

Explicit business memo:

```go
memo := onemoney.Memo{
    Type:   "purpose/example",
    Format: "text/plain",
    Data:   "v2 operation example",
}

response, err := client.Transactions().Payment(
    ctx,
    payload,
    signer,
    onemoney.WithMemo(memo),
)
```

Always handle the returned error. An invalid memo is rejected before the signer
is used or a network request is sent.

## Choose a submission flow

### One-step submission

Use the namespace methods for normal application code. They prepare the
transaction, sign it with the supplied `Signer`, authorize it, submit it to the
correct endpoint, and verify the node's returned transaction hash in one call.

- [`transactions_test.go`](./transactions_test.go) contains `Payment` and
  `BatchPayment` examples.
- [`tokens_test.go`](./tokens_test.go) contains every token write operation.
- [`accounts_test.go`](./accounts_test.go) contains `CreateMultisig` examples.

Each method has separate `withMemo` and `withoutMemo` examples so that neither
example suggests submitting two transactions with the same nonce.

### Offline or external signing

Use the lower-level flow when the signature must be produced outside the
one-step call:

```go
prepared, err := onemoney.PrepareTransaction(
    payload,
    onemoney.WithMemo(memo), // omit this option to use EmptyMemo
)
if err != nil {
    return err
}

signature, err := signer.SignHash(prepared.SigningHash())
if err != nil {
    return err
}

authorized, err := prepared.Authorize(signature)
if err != nil {
    return err
}

transactionHash := authorized.TransactionHash()
response, err := client.Submit(ctx, authorized)
```

`PrepareTransaction` performs no network access. `SigningHash()` is the exact
32-byte digest that an external KMS, HSM, or MPC signer must sign. `Authorize`
validates and attaches the signature, producing an immutable
`AuthorizedTransaction`. `Submit` sends it and verifies the server-returned hash
against the locally derived `TransactionHash()`.

See [`offline_signing_test.go`](./offline_signing_test.go) for complete examples
covering:

- default `EmptyMemo`;
- explicit `WithMemo`; and
- `TokenManageListPayload`, which must also pass `WithManageListKind` to choose
  blacklist or whitelist when calling `PrepareTransaction` directly.

The prepare/authorize pipeline is generic and accepts every payload used by the
one-step examples. `Client.Submit` returns the generic transaction response;
operation-specific response fields remain available through the one-step
namespace methods.

## Covered operations

| Namespace | Methods |
| --- | --- |
| `Transactions()` | `Payment`, `BatchPayment` |
| `Tokens()` | `Issue`, `Mint`, `Burn`, `BridgeAndMint`, `BurnAndBridge`, `GrantAuthority`, `RevokeAuthority`, `Clawback`, `ManageBlacklist`, `ManageWhitelist`, `Pause`, `Unpause`, `UpdateMetadata` |
| `Accounts()` | `CreateMultisig` |

## Setup

The shared setup in [`helpers_test.go`](./helpers_test.go) requires:

- `API_URL`: the target node URL;
- `PRIVATE_KEY`: the signer used by the example.

Individual payload builders read only the addresses needed by that operation:

- `TOKEN_ADDRESS`;
- `RECIPIENT_ADDRESS`;
- `ACCOUNT_ADDRESS`.

The setup fetches the chain ID and the signer's current nonce. The payload
builders contain illustrative amounts and metadata; replace them with values
valid for the target network and signer. The selected operation must also be
permitted by the token's authorities, balances, bridge configuration, list
configuration, and feature flags.

## Compile without submitting

The Example functions intentionally have no Go `Output:` directive. The Go
toolchain compiles them but does not execute their state-changing network calls:

```bash
go test ./examples/v2_operations
```

These files are reference examples, not a command that submits every operation.
To use one safely:

1. Copy one Example function and its payload builder into your application.
2. Replace the illustrative payload values.
3. Fetch a fresh nonce immediately before preparing or submitting the operation.
4. Run only the operation you intend to submit.

Never run the `withMemo` and `withoutMemo` variants back-to-back with the same
nonce.
