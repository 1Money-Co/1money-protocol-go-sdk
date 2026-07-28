package onemoney

import "context"

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
