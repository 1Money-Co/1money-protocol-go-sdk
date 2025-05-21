package onemoney

import (
	"context"
	"fmt"
)

type TokenAccountResponse struct {
	Balance             string `json:"balance"`
	Nonce               int    `json:"nonce"`
	TokenAccountAddress string `json:"token_account_address"`
}

type AccountNonceResponse struct {
	Nonce uint64 `json:"nonce"`
}

func (client *Client) GetTokenAccount(ctx context.Context, address, token string) (*TokenAccountResponse, error) {
	result := new(TokenAccountResponse)
	return result, client.GetMethod(ctx, fmt.Sprintf("/v1/accounts/token_account?address=%s&token=%s", address, token), result)
}

func (client *Client) GetAccountNonce(ctx context.Context, address string) (*AccountNonceResponse, error) {
	result := new(AccountNonceResponse)
	return result, client.GetMethod(ctx, fmt.Sprintf("/v1/accounts/nonce?address=%s", address), result)
}
