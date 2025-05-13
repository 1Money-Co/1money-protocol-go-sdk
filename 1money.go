package onemoney

import (
	"net/http"
)

const (
	ApiBaseUrl     = "https://api.1money.network"
	ApiBaseUrlTest = "https://api.testnet.1money.network"
)

type Client struct {
	baseUrl string
	client  *http.Client
}

func New(apiBaseUrl string) *Client {
	return &Client{
		baseUrl: apiBaseUrl,
		client:  http.DefaultClient,
	}
}
