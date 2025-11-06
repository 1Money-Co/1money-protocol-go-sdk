package onemoney

import (
	"context"
	"fmt"
	"net/url"
)

type TokenAccountResponse struct {
	Balance             string `json:"balance"` // The balance of the token.
	Nonce               uint64 `json:"nonce"`   // The nonce of the owner account.
	TokenAccountAddress string `json:"token_account_address"`
}

type AccountNonceResponse struct {
	Nonce uint64 `json:"nonce"`
}

func (client *Client) GetTokenAccount(ctx context.Context, address, token string) (*TokenAccountResponse, error) {
	result := new(TokenAccountResponse)
	params := url.Values{}
	params.Set("address", address)
	params.Set("token", token)
	return result, client.GetMethod(ctx, fmt.Sprintf("/v1/accounts/token_account?%s", params.Encode()), result)
}

func (client *Client) GetAccountNonce(ctx context.Context, address string) (*AccountNonceResponse, error) {
	result := new(AccountNonceResponse)
	params := url.Values{}
	params.Set("address", address)
	return result, client.GetMethod(ctx, fmt.Sprintf("/v1/accounts/nonce?%s", params.Encode()), result)
}
