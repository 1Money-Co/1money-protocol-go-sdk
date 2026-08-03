package onemoney

// Memo is the optional signed memo attached to a domain-separated v2
// transaction. All three fields are always present on the wire; empty strings
// mean "no business memo". The JSON tags match the L1 /v2 request memo object.
type Memo struct {
	Type   string `json:"type"`
	Format string `json:"format"`
	Data   string `json:"data"`
}

// EmptyMemo returns the canonical "no business memo" value (three empty
// strings). It is the default for every submit method when WithMemo is not
// supplied.
func EmptyMemo() Memo { return Memo{} }

type submitConfig struct {
	memo Memo
	// memoSet is true once WithMemo was applied, letting paths that cannot carry
	// a memo reject it instead of silently dropping it.
	memoSet  bool
	listKind *ManageListKind
}

// SubmitOption customizes a single submit call (e.g. attaching a memo).
type SubmitOption func(*submitConfig)

// WithMemo attaches a signed memo to the submitted transaction. Without it, the
// canonical empty memo is used. Submitting a memo on a path that cannot carry
// one (legacy v1 mode, or a batch payment) is rejected rather than silently
// dropped, so audit data is never lost without notice.
func WithMemo(m Memo) SubmitOption {
	return func(c *submitConfig) {
		c.memo = m
		c.memoSet = true
	}
}

func resolveSubmit(opts []SubmitOption) submitConfig {
	cfg := submitConfig{memo: EmptyMemo()}
	for _, o := range opts {
		o(&cfg)
	}
	return cfg
}
