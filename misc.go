package onemoney

import (
	"context"
	"fmt"
	"net/url"

	"github.com/ethereum/go-ethereum/common"
)

// This file holds small, peripheral network-metadata read APIs that do not
// warrant their own domain files: chain identity (GET /v1/chains/chain_id), the
// node's native-write status (GET /api/status), and fee pricing plans
// (GET /v1/pricing/plans).

// -----------------------------------------------------------------------------
// Chain identity (GET /v1/chains/chain_id)
// -----------------------------------------------------------------------------

type ChainIdResponse struct {
	ChainId uint64 `json:"chain_id"`
}

func (client *Client) GetChainId(ctx context.Context) (*ChainIdResponse, error) {
	result := new(ChainIdResponse)
	return result, client.GetMethod(ctx, "/v1/chains/chain_id", result)
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

// PricingPlanVersion is the pricing-plan schema version. The wire values are
// lowercase.
type PricingPlanVersion string

const (
	PricingPlanVersionV0 PricingPlanVersion = "v0"
	PricingPlanVersionV1 PricingPlanVersion = "v1"
)

// PricingPlan is a fee pricing plan. Token, ActiveFrom and ActiveTo are nil when
// the node returns null for them.
type PricingPlan struct {
	Address    common.Address     `json:"address"`
	Version    PricingPlanVersion `json:"version"`
	Token      *common.Address    `json:"token"`
	Criteria   []PricingCriteria  `json:"criteria"`
	Tiers      []PricingFeeTier   `json:"tiers"`
	ActiveFrom *uint64            `json:"active_from"`
	ActiveTo   *uint64            `json:"active_to"`
}

// PricingCriteria is one scope predicate of a pricing plan. It is internally
// tagged by Type (token, sender, recipient, sender_token, recipient_token,
// sender_general, recipient_general); Token is set only for the *_token types.
type PricingCriteria struct {
	Type    string          `json:"type"`
	Address common.Address  `json:"address"`
	Token   *common.Address `json:"token,omitempty"`
}

// PricingFeeTier is one fee tier of a pricing plan. MaxAmount is nil for an
// open-ended top tier.
type PricingFeeTier struct {
	MinAmount string            `json:"min_amount"`
	MaxAmount *string           `json:"max_amount"`
	Fee       PricingFeeFormula `json:"fee"`
}

// PricingFeeFormula is a tier's fee formula, internally tagged by Type: "fixed"
// (Amount set), "percentage" or "defaultratio" (Points set).
type PricingFeeFormula struct {
	Type   string  `json:"type"`
	Amount *uint64 `json:"amount,omitempty"`
	Points *uint16 `json:"points,omitempty"`
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
