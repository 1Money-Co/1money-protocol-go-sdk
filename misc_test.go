package onemoney

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
)

func TestGetChainIdUsesChainsEndpoint(t *testing.T) {
	// In-memory transport (no socket) so this runs under the default `go test`,
	// matching the other routing tests (see fakeHTTPClient in api_v2_test.go).
	var gotPath string
	hc := fakeHTTPClient(&gotPath, func(_ string, _ map[string]json.RawMessage) interface{} {
		return ChainIdResponse{ChainId: 1_212_101}
	})
	client := NewClientWithCustomUrl("http://sdk.test", WithHTTPClient(hc))
	response, err := client.GetChainId(context.Background())
	if err != nil {
		t.Fatalf("GetChainId: %v", err)
	}
	if gotPath != "/v1/chains/chain_id" {
		t.Errorf("request path = %q, want %q", gotPath, "/v1/chains/chain_id")
	}
	if response.ChainId != 1_212_101 {
		t.Errorf("chain ID = %d, want %d", response.ChainId, 1_212_101)
	}
}

func TestNativeWriteStatusResponseUnmarshal(t *testing.T) {
	// Wire values are lowercase snake_case, exactly as the L1 node emits them
	// (om-api-rest native_write_gate as_metric_label: v1_only / dual / v2_only).
	raw := `{
		"native_write_mode": "dual",
		"read_only": false,
		"activation_source": "capability_full",
		"dual_activated_at_secs": 1747785600,
		"native_domain_separated_transactions": {
			"support_count": 13, "required_count": 13, "full_support": true
		}
	}`
	var s NativeWriteStatusResponse
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		t.Fatal(err)
	}
	if s.NativeWriteMode != NativeWriteModeDual || s.ReadOnly {
		t.Errorf("unexpected: %+v", s)
	}
	if s.ActivationSource != ActivationSourceCapabilityFull {
		t.Errorf("activation_source = %q, want %q", s.ActivationSource, ActivationSourceCapabilityFull)
	}
	if NativeWriteModeDual != "dual" || NativeWriteModeV1Only != "v1_only" || NativeWriteModeV2Only != "v2_only" {
		t.Errorf("mode constants drifted from wire values")
	}
	if ActivationSourceNotActivated != "not_activated" || ActivationSourceCapabilityFull != "capability_full" || ActivationSourceBinaryRelease != "binary_release" {
		t.Errorf("activation_source constants drifted from wire values")
	}
	if s.DualActivatedAtSecs == nil || *s.DualActivatedAtSecs != 1747785600 {
		t.Errorf("dual_activated_at_secs = %v", s.DualActivatedAtSecs)
	}
	if !s.NativeDomainSeparatedTransactions.FullSupport || s.NativeDomainSeparatedTransactions.SupportCount != 13 {
		t.Errorf("feature status = %+v", s.NativeDomainSeparatedTransactions)
	}
}

func TestPricingPlanUnmarshal(t *testing.T) {
	raw := `{
		"address":"0x5458747a0efb9ebeb8696fcac1479278c0872fbe",
		"version":"v1",
		"token":null,
		"criteria":[{"type":"sender_token","address":"0x1111111111111111111111111111111111111111","token":"0x2222222222222222222222222222222222222222"}],
		"tiers":[{"min_amount":"0","max_amount":null,"fee":{"type":"defaultratio","points":30}}],
		"active_from":null,
		"active_to":null
	}`
	var p PricingPlan
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatal(err)
	}
	if p.Version != PricingPlanVersionV1 {
		t.Errorf("version = %s", p.Version)
	}
	if assert.Len(t, p.Criteria, 1) {
		assert.Equal(t, "sender_token", p.Criteria[0].Type)
		if assert.NotNil(t, p.Criteria[0].Token) {
			assert.Equal(t, common.HexToAddress("0x2222222222222222222222222222222222222222"), *p.Criteria[0].Token)
		}
	}
	if assert.Len(t, p.Tiers, 1) {
		assert.Equal(t, "0", p.Tiers[0].MinAmount)
		assert.Nil(t, p.Tiers[0].MaxAmount)
		assert.Equal(t, "defaultratio", p.Tiers[0].Fee.Type)
		if assert.NotNil(t, p.Tiers[0].Fee.Points) {
			assert.Equal(t, uint16(30), *p.Tiers[0].Fee.Points)
		}
	}
}

func TestGetPricingPlansRequiresScope(t *testing.T) {
	c := NewClientWithCustomUrl("http://127.0.0.1:0")
	if _, err := c.GetPricingPlans(nil, "", "", ""); err == nil {
		t.Fatal("expected error when no scope provided")
	}
}
