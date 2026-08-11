package onemoney

import (
	"fmt"
	"unicode"
	"unicode/utf8"
)

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

// Memo size limits, mirroring the node's own constants in
// om-primitives-types/src/transaction/payload/memo.rs. These are protocol
// constants, not SDK policy: a memo that exceeds them is rejected at
// admission, so the SDK enforces them before signing rather than letting a
// caller sign a transaction that cannot be accepted.
const (
	memoTypeMaxBytes   = 128
	memoFormatMaxBytes = 64
	memoDataMaxBytes   = 256
	memoTotalMaxBytes  = 512
	// memoRLPHeaderAllowance matches the node's fixed envelope allowance used
	// when it computes the memo's serialized size.
	memoRLPHeaderAllowance = 16
)

// byteSize matches the node's Memo::byte_size(): the three field lengths plus a
// fixed allowance bounding the RLP envelope.
func (m Memo) byteSize() int {
	return len(m.Type) + len(m.Format) + len(m.Data) + memoRLPHeaderAllowance
}

// isMemoURLSafe mirrors the node's is_url_safe(): RFC 3986 unreserved
// characters, gen-delims, sub-delims, and the percent sign. Anything else --
// including spaces and any non-ASCII rune -- is rejected in memo.type and
// memo.format.
func isMemoURLSafe(ch rune) bool {
	switch {
	case ch >= 'A' && ch <= 'Z', ch >= 'a' && ch <= 'z', ch >= '0' && ch <= '9':
		return true
	}
	switch ch {
	case '-', '.', '_', '~', // unreserved
		':', '/', '?', '#', '[', ']', '@', // gen-delims
		'!', '$', '&', '\'', '(', ')', '*', '+', ',', ';', '=', // sub-delims
		'%': // percent-encoding
		return true
	}
	return false
}

// validate applies the node's hard memo rules exactly (Memo::validate()). An
// empty subfield is always valid -- EmptyMemo() is the canonical "no business
// memo" value -- so each check is skipped for an empty field, matching the
// node. Only length and character-class rules are enforced; soft rules such as
// MIME syntax on memo.format are not checked on either side.
func (m Memo) validate() error {
	if m.Type != "" {
		if len(m.Type) > memoTypeMaxBytes {
			return fmt.Errorf("memo.type exceeds %d bytes (got %d)", memoTypeMaxBytes, len(m.Type))
		}
		for _, ch := range m.Type {
			if !isMemoURLSafe(ch) {
				return fmt.Errorf("memo.type contains an invalid character %q; only RFC 3986 unreserved, gen-delims, sub-delims and %% are allowed", ch)
			}
		}
	}

	if m.Format != "" {
		if len(m.Format) > memoFormatMaxBytes {
			return fmt.Errorf("memo.format exceeds %d bytes (got %d)", memoFormatMaxBytes, len(m.Format))
		}
		for _, ch := range m.Format {
			if !isMemoURLSafe(ch) {
				return fmt.Errorf("memo.format contains an invalid character %q; only RFC 3986 unreserved, gen-delims, sub-delims and %% are allowed", ch)
			}
		}
	}

	if m.Data != "" {
		if len(m.Data) > memoDataMaxBytes {
			return fmt.Errorf("memo.data exceeds %d bytes (got %d)", memoDataMaxBytes, len(m.Data))
		}
		// SDK-only rule, with no counterpart in Memo::validate(): the node's
		// memo.data is a Rust String and is therefore UTF-8 by construction, so
		// its validator has nothing to check. A Go string can hold invalid UTF-8,
		// which the node's JSON deserializer rejects before validate() ever runs.
		// Checking it here fails earlier without ever accepting something the node
		// would reject.
		//
		// This must be a whole-string check, not a per-rune one: ranging over a
		// string yields utf8.RuneError both for genuinely invalid bytes and for a
		// legitimate U+FFFD, so a per-rune test would reject the replacement
		// character -- which the node accepts as ordinary text.
		if !utf8.ValidString(m.Data) {
			return fmt.Errorf("memo.data is not valid UTF-8")
		}
		// memo.data otherwise accepts arbitrary text; only control codepoints are
		// rejected. unicode.IsControl covers Unicode Cc (U+0000-U+001F and
		// U+007F-U+009F), exactly matching Rust's char::is_control(). The explicit
		// NUL test is redundant with it and is kept only to mirror the node's
		// `c == '\0' || c.is_control()`.
		for _, ch := range m.Data {
			if ch == 0 || unicode.IsControl(ch) {
				return fmt.Errorf("memo.data contains a control character")
			}
		}
	}

	if total := m.byteSize(); total > memoTotalMaxBytes {
		return fmt.Errorf("memo object exceeds %d bytes (got %d)", memoTotalMaxBytes, total)
	}

	return nil
}

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
// canonical empty memo is used. Submitting a memo in legacy v1 mode is rejected
// rather than silently dropped, so audit data is never lost without notice.
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
