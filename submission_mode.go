package onemoney

// SubmissionMode selects the native transaction signing scheme and REST write
// surface used by the Client.Transactions()/Tokens()/Accounts() submit methods.
//
// The zero value is domain-separated v2 (the default and recommended mode), so
// a client built with any existing constructor submits v2 without extra
// configuration. A submission never probes one mode and falls back to the
// other: retrying a signed transaction under a different scheme can create two
// transactions for the same nonce.
type SubmissionMode int

const (
	// SubmissionModeDomainSeparatedV2 signs with the issue-1038 domain-separated
	// scheme and POSTs to /v2. This is the default (zero value).
	SubmissionModeDomainSeparatedV2 SubmissionMode = iota
	// SubmissionModeLegacyV1 signs with the legacy scheme and POSTs to /v1.
	// Explicit opt-in only, for compatibility during the migration window.
	SubmissionModeLegacyV1
)

// WithSubmissionMode sets the native submission mode on the client.
func WithSubmissionMode(m SubmissionMode) ClientOption {
	return func(c *Client) { c.submissionMode = m }
}

// WithLegacyV1 is a convenience option equivalent to
// WithSubmissionMode(SubmissionModeLegacyV1).
func WithLegacyV1() ClientOption {
	return func(c *Client) { c.submissionMode = SubmissionModeLegacyV1 }
}

// mode returns the client's configured submission mode.
func (c *Client) mode() SubmissionMode { return c.submissionMode }
