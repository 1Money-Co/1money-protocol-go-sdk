package onemoney

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetChainIdUsesChainsEndpoint(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(ChainIdResponse{ChainId: 1_212_101}); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer server.Close()

	client := NewClientWithCustomUrl(server.URL)
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
	raw := `{"address":"0x5458747a0efb9ebeb8696fcac1479278c0872fbe","version":"v1","criteria":[{"x":1}],"tiers":[]}`
	var p PricingPlan
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatal(err)
	}
	if p.Version != "v1" {
		t.Errorf("version = %s", p.Version)
	}
	if string(p.Criteria) != `[{"x":1}]` {
		t.Errorf("criteria = %s", p.Criteria)
	}
}

func TestGetPricingPlansRequiresScope(t *testing.T) {
	c := NewClientWithCustomUrl("http://127.0.0.1:0")
	if _, err := c.GetPricingPlans(nil, "", "", ""); err == nil {
		t.Fatal("expected error when no scope provided")
	}
}
