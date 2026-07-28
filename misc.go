package onemoney

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/ethereum/go-ethereum/common"
)

// This file holds small, peripheral network-metadata read APIs that do not
// warrant their own domain files: chain identity (GET /v1/chain_id), the node's
// native-write status (GET /api/status), and fee pricing plans
// (GET /v1/pricing/plans).

// -----------------------------------------------------------------------------
// Chain identity (GET /v1/chain_id)
// -----------------------------------------------------------------------------

type ChainIdResponse struct {
	ChainId uint64 `json:"chain_id"`
}

func (client *Client) GetChainId(ctx context.Context) (*ChainIdResponse, error) {
	result := new(ChainIdResponse)
	return result, client.GetMethod(ctx, "/v1/chain_id", result)
}

// -----------------------------------------------------------------------------
// Native-write status (GET /api/status)
// -----------------------------------------------------------------------------

// FeatureSupportStatus reports how many validators advertise a protocol feature
// versus how many are required for full activation.
type FeatureSupportStatus struct {
	SupportCount  int  `json:"support_count"`
	RequiredCount int  `json:"required_count"`
	FullSupport   bool `json:"full_support"`
}

// NativeWriteMode is the node's native-write mode as reported by
// GET /api/status. The wire values are lowercase snake_case.
type NativeWriteMode string

const (
	// NativeWriteModeV1Only: only legacy /v1 native writes are accepted; /v2 is
	// disabled (the network has not activated domain-separated writes yet).
	NativeWriteModeV1Only NativeWriteMode = "v1_only"
	// NativeWriteModeDual: both /v1 (legacy) and /v2 (domain-separated) native
	// writes are accepted during the migration window.
	NativeWriteModeDual NativeWriteMode = "dual"
	// NativeWriteModeV2Only: legacy /v1 native writes are disabled; only /v2 is
	// accepted.
	NativeWriteModeV2Only NativeWriteMode = "v2_only"
)

// ActivationSource explains why the node is in its current NativeWriteMode,
// returned in GET /api/status. The wire values are lowercase snake_case.
type ActivationSource string

const (
	// ActivationSourceNotActivated accompanies NativeWriteModeV1Only:
	// domain-separated writes are not activated yet.
	ActivationSourceNotActivated ActivationSource = "not_activated"
	// ActivationSourceCapabilityFull accompanies NativeWriteModeDual: activated
	// because the NativeDomainSeparatedTransactions capability reached full
	// support across the validator set.
	ActivationSourceCapabilityFull ActivationSource = "capability_full"
	// ActivationSourceBinaryRelease accompanies NativeWriteModeV2Only: fixed by
	// the release N+1 binary.
	ActivationSourceBinaryRelease ActivationSource = "binary_release"
)

// NativeWriteStatusResponse is the node's effective native-write state, returned
// by GET /api/status. Integrators can compare NativeWriteMode against
// NativeWriteModeDual / NativeWriteModeV2Only to confirm a network accepts
// domain-separated v2 writes before switching a client to (or from)
// SubmissionModeLegacyV1.
type NativeWriteStatusResponse struct {
	NativeWriteMode                   NativeWriteMode      `json:"native_write_mode"`
	ReadOnly                          bool                 `json:"read_only"`
	ActivationSource                  ActivationSource     `json:"activation_source"`
	DualActivatedAtSecs               *uint64              `json:"dual_activated_at_secs"`
	NativeDomainSeparatedTransactions FeatureSupportStatus `json:"native_domain_separated_transactions"`
}

// GetNativeWriteStatus fetches the node's native-write status (GET /api/status).
func (c *Client) GetNativeWriteStatus(ctx context.Context) (*NativeWriteStatusResponse, error) {
	result := new(NativeWriteStatusResponse)
	return result, c.GetMethod(ctx, "/api/status", result)
}

// -----------------------------------------------------------------------------
// Pricing plans (GET /v1/pricing/plans)
// -----------------------------------------------------------------------------

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
