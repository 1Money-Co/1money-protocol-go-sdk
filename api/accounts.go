package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

type TokenAccount struct {
	Balance             string `json:"balance"`
	Nonce               int    `json:"nonce"`
	TokenAccountAddress string `json:"token_account_address"`
}

type AccountNonce struct {
	Nonce uint64 `json:"nonce"`
}

func GetTokenAccount(address, token string) (*TokenAccount, error) {
	gin.SetMode(gin.ReleaseMode)
	client := &http.Client{}

	url := fmt.Sprintf(BaseAPIURL+"/v1/accounts/token_account?address=%s&token=%s", address, token)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get token account: %w", err)
	}

	var result TokenAccount
	if err := HandleAPIResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func GetAccountNonce(address string) (*AccountNonce, error) {
	gin.SetMode(gin.ReleaseMode)
	client := &http.Client{}

	url := fmt.Sprintf(BaseAPIURL+"/v1/accounts/nonce?address=%s", address)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get account nonce: %w", err)
	}

	var result AccountNonce
	if err := HandleAPIResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}
