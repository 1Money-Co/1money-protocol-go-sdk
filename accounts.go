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
