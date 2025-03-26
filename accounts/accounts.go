package accounts

import (
	"encoding/json"
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
	Nonce int `json:"nonce"`
}

func GetTokenAccount(address, token string) (*TokenAccount, error) {
	gin.SetMode(gin.ReleaseMode)
	client := &http.Client{}

	url := fmt.Sprintf("https://api.testnet.1money.network/v1/accounts/token_account?address=%s&token=%s", address, token)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get token account: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result TokenAccount
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

func GetAccountNonce(address string) (*AccountNonce, error) {
	gin.SetMode(gin.ReleaseMode)
	client := &http.Client{}

	url := fmt.Sprintf("https://api.testnet.1money.network/v1/accounts/nonce?address=%s", address)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get account nonce: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result AccountNonce
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}
