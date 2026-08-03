package onemoney

import (
	"context"
	"fmt"
	"net/url"

	"github.com/ethereum/go-ethereum/common"
)

const (
	endpointAccountsNonce        = "/v1/accounts/nonce"
	endpointAccountsTokenAccount = "/v1/accounts/token_account"
	endpointAccountsBbNonce      = "/v1/accounts/bbnonce"
)

type TokenAccountResponse struct {
	Balance string `json:"balance"` // The balance of the token.
	Nonce   uint64 `json:"nonce"`   // The nonce of the owner account.
}

type AccountNonceResponse struct {
	Nonce uint64 `json:"nonce"`
}

type AccountBbNonceResponse struct {
	BbNonce uint64 `json:"bbnonce"`
}

// MultiSigSigner is one member of a multisig account: a 33-byte SEC1-compressed
// public key and a voting weight.
type MultiSigSigner struct {
	PublicKey HexBytes `json:"public_key"`
	Weight    uint8    `json:"weight"`
}

// CreateMultiSigPayload creates a multisig account with the given signer set and
// approval threshold. The creation transaction itself is single-signed.
type CreateMultiSigPayload struct {
	ChainID   uint64           `json:"chain_id"`
	Nonce     uint64           `json:"nonce"`
	Signers   []MultiSigSigner `json:"signers"`
	Threshold uint16           `json:"threshold"`
}

// CreateMultisigResponse is returned by Accounts().CreateMultisig. Hash is the
// submitted transaction hash from the node. Account is the created multisig
// account address; the L1 endpoint returns only the hash, so the SDK fills
// Account by local derivation (deterministic and identical to the address the
// node assigns — see DeriveMultisigAddress).
type CreateMultisigResponse struct {
	Hash    string         `json:"hash"`
	Account common.Address `json:"account"`
}

// TxHash reports the submitted transaction hash for hash-verification.
func (r *CreateMultisigResponse) TxHash() string { return r.Hash }

func (client *Client) GetTokenAccount(ctx context.Context, address, token common.Address) (*TokenAccountResponse, error) {
	result := new(TokenAccountResponse)
	params := url.Values{}
	params.Set("address", address.Hex())
	params.Set("token", token.Hex())
	return result, client.GetMethod(ctx, fmt.Sprintf("%s?%s", endpointAccountsTokenAccount, params.Encode()), result)
}

func (client *Client) GetAccountNonce(ctx context.Context, address common.Address) (*AccountNonceResponse, error) {
	result := new(AccountNonceResponse)
	params := url.Values{}
	params.Set("address", address.Hex())
	return result, client.GetMethod(ctx, fmt.Sprintf("%s?%s", endpointAccountsNonce, params.Encode()), result)
}

func (client *Client) GetAccountBbNonce(ctx context.Context, address common.Address) (*AccountBbNonceResponse, error) {
	result := new(AccountBbNonceResponse)
	params := url.Values{}
	params.Set("address", address.Hex())
	return result, client.GetMethod(ctx, fmt.Sprintf("%s?%s", endpointAccountsBbNonce, params.Encode()), result)
}

// -----------------------------------------------------------------------------
// Domain-separated v2 account writes (namespace API)
// -----------------------------------------------------------------------------

// AccountsAPI groups account-management submit methods.
type AccountsAPI struct{ c *Client }

// Accounts returns the account submit namespace.
func (c *Client) Accounts() AccountsAPI { return AccountsAPI{c: c} }

// CreateMultisig creates a multisig account with the given signer set and
// threshold. The creation transaction is single-signed by `signer`. This is a
// v2-only endpoint; it has no legacy /v1 form and returns an error under
// WithLegacyV1. The response's Account is the created multisig address (derived
// locally; the endpoint returns only the transaction hash).
func (a AccountsAPI) CreateMultisig(ctx context.Context, payload CreateMultiSigPayload, signer Signer, opts ...SubmitOption) (*CreateMultisigResponse, error) {
	if a.c.mode() == SubmissionModeLegacyV1 {
		return nil, fmt.Errorf("multisig account creation requires domain-separated v2 and has no legacy v1 endpoint")
	}
	out := new(CreateMultisigResponse)
	if err := a.c.submitPayload(ctx, payload, resolveSubmit(opts), signer, out); err != nil {
		return nil, err
	}
	// submitPayload validated the configuration offline (resolvePayloadOp) before
	// signing or any network I/O, so the config is known-good here. The account
	// address is a deterministic function of the signer set and threshold (the L1
	// endpoint returns only the transaction hash, so the SDK fills it locally).
	out.Account = deriveMultisigAddressUnchecked(payload.Signers, payload.Threshold)
	return out, nil
}
