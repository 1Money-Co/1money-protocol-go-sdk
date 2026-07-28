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
	// The account address is a deterministic function of the signer set and
	// threshold. Derive it up front so an invalid configuration fails before we
	// submit anything, and so the response can report the created account (the
	// L1 endpoint itself returns only the transaction hash).
	account, err := DeriveMultisigAddress(payload.Signers, payload.Threshold)
	if err != nil {
		return nil, err
	}
	out := new(CreateMultisigResponse)
	if err := a.c.submitPayload(ctx, payload, resolveSubmit(opts), signer, out); err != nil {
		return nil, err
	}
	out.Account = account
	return out, nil
}
