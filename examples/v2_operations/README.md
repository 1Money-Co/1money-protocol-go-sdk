# Native-v2 operation memo examples

This directory contains compile-checked examples for every public native-v2
submission method. Each operation has two examples:

- `withoutMemo` omits the `WithMemo` submit option. The SDK still signs and
  sends the canonical `EmptyMemo` (`type`, `format`, and `data` are all empty).
- `withMemo` passes an explicit `WithMemo(Memo{...})`. The memo is part of the
  signed transaction, so changing it changes both the signing hash and the
  transaction hash.

There is no native-v2 wire form without a memo object. "Without memo" in these
example names means "without an explicit business memo", not "omit the memo
field from the request".

## Covered methods

| Namespace | Methods |
| --- | --- |
| `Transactions()` | `Payment`, `BatchPayment` |
| `Tokens()` | `Issue`, `Mint`, `Burn`, `BridgeAndMint`, `BurnAndBridge`, `GrantAuthority`, `RevokeAuthority`, `Clawback`, `ManageBlacklist`, `ManageWhitelist`, `Pause`, `Unpause`, `UpdateMetadata` |
| `Accounts()` | `CreateMultisig` |

The common setup in `helpers_test.go` always reads:

- `API_URL`
- `PRIVATE_KEY`

Individual payload builders read only the addresses they need from:

- `TOKEN_ADDRESS`
- `RECIPIENT_ADDRESS`
- `ACCOUNT_ADDRESS`

It then fetches the current chain ID and signer nonce. Replace the sample
payload values with values valid for the target network and signer before
submitting. In particular, token authorities, balances, bridge parameters,
list configuration, and token feature flags must permit the selected operation.

The examples intentionally have no Go `Output:` directive. This means the Go
toolchain compiles them but does not execute state-changing network requests:

```bash
go test ./examples/v2_operations
```

To use one, copy the corresponding Example function and its payload builder into
your application. Run only the operation you intend to submit, fetch a fresh
nonce for it, and always handle the returned error. Do not run the `withMemo`
and `withoutMemo` variants back-to-back with the same nonce.
