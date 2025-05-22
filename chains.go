package onemoney

import (
	"context"
)

type ChainIdResponse struct {
	ChainId int `json:"chain_id"`
}

func (client *Client) GetChainId(ctx context.Context) (*ChainIdResponse, error) {
	result := new(ChainIdResponse)
	return result, client.GetMethod(ctx, "/v1/chains/chain_id", result)
}
