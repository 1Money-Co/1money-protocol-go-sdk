package onemoney

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/ethereum/go-ethereum/common"
)

// PricingPlan is a fee pricing plan. The nested criteria and tiers are captured
// as raw JSON to keep the SDK surface small; decode them further if needed.
type PricingPlan struct {
	Address    common.Address  `json:"address"`
	Version    string          `json:"version"`
	Token      *common.Address `json:"token,omitempty"`
	Criteria   json.RawMessage `json:"criteria,omitempty"`
	Tiers      json.RawMessage `json:"tiers,omitempty"`
	ActiveFrom *uint64         `json:"active_from,omitempty"`
	ActiveTo   *uint64         `json:"active_to,omitempty"`
}

// PricingPlanLookup is the per-scope pricing plan result returned by
// GET /v1/pricing/plans.
type PricingPlanLookup struct {
	Sender    *PricingPlan `json:"sender"`
	Recipient *PricingPlan `json:"recipient"`
	Token     *PricingPlan `json:"token"`
}

// GetPricingPlanByID fetches a pricing plan by its id
// (GET /v1/pricing/plans/{plan_id}).
func (c *Client) GetPricingPlanByID(ctx context.Context, planID string) (*PricingPlan, error) {
	result := new(PricingPlan)
	return result, c.GetMethod(ctx, "/v1/pricing/plans/"+url.PathEscape(planID), result)
}

// GetPricingPlans looks up pricing plans by scope (GET /v1/pricing/plans). At
// least one of sender, recipient, or token must be non-empty.
func (c *Client) GetPricingPlans(ctx context.Context, sender, recipient, token string) (*PricingPlanLookup, error) {
	params := url.Values{}
	if sender != "" {
		params.Set("sender", sender)
	}
	if recipient != "" {
		params.Set("recipient", recipient)
	}
	if token != "" {
		params.Set("token", token)
	}
	if len(params) == 0 {
		return nil, fmt.Errorf("at least one of sender, recipient, token is required")
	}
	result := new(PricingPlanLookup)
	return result, c.GetMethod(ctx, fmt.Sprintf("/v1/pricing/plans?%s", params.Encode()), result)
}
