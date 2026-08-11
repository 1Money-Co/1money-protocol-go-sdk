package onemoney

import (
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

// TestMemoValidationMirrorsNodeRules pins the memo rules the node enforces at
// admission (om-primitives-types payload/memo.rs `Memo::validate`). These are
// protocol constants, not SDK policy, so the SDK rejects a violating memo before
// signing rather than letting a caller sign a transaction that cannot be
// accepted.
//
// Note on the total-size cap: MEMO_TOTAL_MAX_BYTES is 512 while the per-field
// caps sum to 128+64+256 plus a 16-byte envelope allowance = 464. The total cap
// is therefore unreachable whenever the per-field caps hold, on both sides, so
// there is no test case for it -- writing one would require a memo that already
// fails a per-field rule.
func TestMemoValidationMirrorsNodeRules(t *testing.T) {
	valid := []struct {
		name string
		memo Memo
	}{
		{"empty is canonical", EmptyMemo()},
		{"populated within limits", Memo{Type: "purpose/SALA", Format: "text/plain", Data: "invoice-0001"}},
		{"type at the cap", Memo{Type: strings.Repeat("a", memoTypeMaxBytes)}},
		{"format at the cap", Memo{Format: strings.Repeat("a", memoFormatMaxBytes)}},
		{"data at the cap", Memo{Data: strings.Repeat("a", memoDataMaxBytes)}},
		{"url-safe punctuation in type", Memo{Type: "a-b.c_d~e:f/g?h#i[j]k@l!m$n&o'p(q)r*s+t,u;v=w%20"}},
		{"data accepts arbitrary text", Memo{Data: "space and 中文 and emoji-free punctuation: ;,."}},
		// 84 * 3 + 4 = 256 bytes exactly, so the cap is a byte cap, not a rune cap.
		{"data at the byte cap with multibyte", Memo{Data: strings.Repeat("界", 84) + "abcd"}},
		// U+FFFD is an ordinary character. Ranging over a Go string yields
		// utf8.RuneError for it as well as for genuinely invalid bytes, so a
		// per-rune validity test would wrongly reject it; the node accepts it.
		{"data accepts the replacement character", Memo{Data: "invoice \ufffd 0001"}},
		// U+00A0 sits immediately after the C1 control block and must be accepted;
		// U+009F, one codepoint below, must not (see the invalid table).
		{"data accepts U+00A0 just past the C1 block", Memo{Data: "a\u00a0b"}},
		{"data accepts DEL-adjacent U+007E", Memo{Data: "a~b"}},
	}
	for _, tc := range valid {
		tc := tc
		t.Run("valid/"+tc.name, func(t *testing.T) {
			if err := tc.memo.validate(); err != nil {
				t.Errorf("memo %+v should be valid: %v", tc.memo, err)
			}
		})
	}

	invalid := []struct {
		name string
		memo Memo
		want string
	}{
		{"type over cap", Memo{Type: strings.Repeat("a", memoTypeMaxBytes+1)}, "memo.type exceeds"},
		{"format over cap", Memo{Format: strings.Repeat("a", memoFormatMaxBytes+1)}, "memo.format exceeds"},
		{"data over cap", Memo{Data: strings.Repeat("a", memoDataMaxBytes+1)}, "memo.data exceeds"},
		{"data over cap by multibyte", Memo{Data: strings.Repeat("界", memoDataMaxBytes/3+1)}, "memo.data exceeds"},
		{"data one byte over the cap with multibyte", Memo{Data: strings.Repeat("界", 84) + "abcde"}, "memo.data exceeds"},
		{"space in type", Memo{Type: "purpose SALA"}, "memo.type contains an invalid character"},
		{"multibyte in type", Memo{Type: "purpose/工资"}, "memo.type contains an invalid character"},
		{"space in format", Memo{Format: "text plain"}, "memo.format contains an invalid character"},
		{"quote in format", Memo{Format: "text/\"plain\""}, "memo.format contains an invalid character"},
		{"nul in data", Memo{Data: "invoice\x00001"}, "memo.data contains a control character"},
		{"newline in data", Memo{Data: "line1\nline2"}, "memo.data contains a control character"},
		{"tab in data", Memo{Data: "a\tb"}, "memo.data contains a control character"},
		// The Cc category has two ranges. Pin both ends of each so a future switch
		// to a different control-character predicate cannot silently narrow it.
		{"C0 upper bound U+001F in data", Memo{Data: "a\u001fb"}, "memo.data contains a control character"},
		{"DEL U+007F in data", Memo{Data: "a\u007fb"}, "memo.data contains a control character"},
		{"C1 lower bound U+0080 in data", Memo{Data: "a\u0080b"}, "memo.data contains a control character"},
		{"NEL U+0085 in data", Memo{Data: "a\u0085b"}, "memo.data contains a control character"},
		{"C1 upper bound U+009F in data", Memo{Data: "a\u009fb"}, "memo.data contains a control character"},
		// Invalid UTF-8 has no counterpart in Memo::validate() -- a Rust String
		// cannot hold it -- but the node's JSON decoder rejects it before validate()
		// runs, so the SDK rejects it too, as its own rule with its own message.
		{"invalid utf-8 in data", Memo{Data: string([]byte{'a', 0xff, 0xfe, 'b'})}, "memo.data is not valid UTF-8"},
		// type/format need no separate UTF-8 rule: invalid bytes decode to U+FFFD,
		// which is not URL-safe, so the character rule already rejects them.
		{"invalid utf-8 in type", Memo{Type: string([]byte{'a', 0xff})}, "memo.type contains an invalid character"},
		{"replacement character in type", Memo{Type: "a\ufffdb"}, "memo.type contains an invalid character"},
	}
	for _, tc := range invalid {
		tc := tc
		t.Run("invalid/"+tc.name, func(t *testing.T) {
			err := tc.memo.validate()
			if err == nil {
				t.Fatalf("memo %+v was accepted; want %q", tc.memo, tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %q, want it to contain %q", err, tc.want)
			}
		})
	}
}

// TestPrepareRejectsInvalidMemoForEveryOperation proves the memo gate lives in
// the one shared prepare path rather than in a single operation's branch. All
// fourteen canonical operations are memo-bearing after the BatchPayment
// re-baseline, so a memo rule that only fired for one of them would be a new
// internal inconsistency.
func TestPrepareRejectsInvalidMemoForEveryOperation(t *testing.T) {
	tooLong := Memo{Data: strings.Repeat("a", memoDataMaxBytes+1)}

	payloads := map[string]any{
		"Payment": PaymentPayload{
			ChainID: 1, Nonce: 1, Recipient: repeatAddr(0x02), Value: big.NewInt(1), Token: repeatAddr(0x01),
		},
		"TokenMint": TokenMintPayload{
			ChainID: 1, Nonce: 1, Recipient: repeatAddr(0x02), Value: big.NewInt(1), Token: repeatAddr(0x01),
		},
		"BatchPayment": BatchPaymentPayload{
			ChainID: 1, Nonce: 1, Token: repeatAddr(0x01),
			Operations: []PaymentOperation{{Recipient: repeatAddr(0x0c), Amount: big.NewInt(1)}},
			CreatedAt:  1,
		},
	}
	for name, payload := range payloads {
		payload := payload
		t.Run(name, func(t *testing.T) {
			if _, err := PrepareTransaction(payload, WithMemo(tooLong)); err == nil {
				t.Fatal("an over-long memo was accepted; want it rejected before signing")
			}
			// The same payload with a legal memo must still prepare, so the guard
			// is not rejecting everything.
			if _, err := PrepareTransaction(payload, WithMemo(Memo{Type: "purpose/SALA"})); err != nil {
				t.Fatalf("a legal memo should prepare: %v", err)
			}
		})
	}
}

// TestMemoValidationCheckOrderMatchesNode pins the order in which rules fire.
// Memo::validate() checks type (length, then characters), then format the same
// way, then data (length, then control codepoints), then the total size. Nothing
// else in the suite would notice if that order changed, and a caller matching on
// the message would see a different error for the same memo.
func TestMemoValidationCheckOrderMatchesNode(t *testing.T) {
	overLongType := strings.Repeat("a", memoTypeMaxBytes+1)
	overLongFormat := strings.Repeat("a", memoFormatMaxBytes+1)

	cases := []struct {
		name string
		memo Memo
		want string
	}{
		{
			"type precedes format",
			Memo{Type: overLongType, Format: "bad format"},
			"memo.type exceeds",
		},
		{
			"type precedes data",
			Memo{Type: overLongType, Data: "line1\nline2"},
			"memo.type exceeds",
		},
		{
			"format precedes data",
			Memo{Format: overLongFormat, Data: "line1\nline2"},
			"memo.format exceeds",
		},
		{
			"length precedes characters within type",
			Memo{Type: overLongType + " with a space"},
			"memo.type exceeds",
		},
		{
			"length precedes characters within format",
			Memo{Format: overLongFormat + " with a space"},
			"memo.format exceeds",
		},
		{
			"length precedes control check within data",
			Memo{Data: strings.Repeat("a", memoDataMaxBytes+1) + "\n"},
			"memo.data exceeds",
		},
		{
			"utf-8 validity precedes the control check within data",
			Memo{Data: string([]byte{0xff, '\n'})},
			"memo.data is not valid UTF-8",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := tc.memo.validate()
			if err == nil {
				t.Fatalf("memo was accepted; want %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %q, want it to contain %q (check order changed)", err, tc.want)
			}
		})
	}
}

// TestPrepareGateOrderMatchesNode pins the order of the three gates against the
// order the node reaches the same conclusions in. The node deserializes the
// request first (so a value with no U256 wire form fails before any validator
// runs), then validates the memo for every origin, then applies
// operation-specific static rules. Simply swapping the memo and admission checks
// would get the first case right and the second wrong.
func TestPrepareGateOrderMatchesNode(t *testing.T) {
	badMemo := WithMemo(Memo{Data: "line\nbreak"})

	t.Run("memo precedes batch admission", func(t *testing.T) {
		// Encodable amounts, but no operations at all, plus an illegal memo. The
		// node validates the memo before it looks at the batch rules.
		payload := BatchPaymentPayload{
			ChainID: 1, Nonce: 1, Token: repeatAddr(0x01), CreatedAt: 1,
		}
		_, err := PrepareTransaction(payload, badMemo)
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "memo.data contains a control character") {
			t.Errorf("err = %q, want the memo error first (got the admission error instead?)", err)
		}
	})

	t.Run("encoding validity precedes memo", func(t *testing.T) {
		// A negative amount has no U256 wire form, so the node's JSON extractor
		// rejects the request before the verifier validates the memo.
		payload := BatchPaymentPayload{
			ChainID: 1, Nonce: 1, Token: repeatAddr(0x01), CreatedAt: 1,
			Operations: []PaymentOperation{{Recipient: repeatAddr(0x0c), Amount: big.NewInt(-5)}},
		}
		_, err := PrepareTransaction(payload, badMemo)
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "must be non-negative") {
			t.Errorf("err = %q, want the U256 encoding error first", err)
		}
	})

	t.Run("encoding validity precedes batch admission", func(t *testing.T) {
		// Both broken: an over-wide amount (no wire form) and a zero-address
		// recipient (inadmissible). Encoding wins.
		tooWide := new(big.Int).Lsh(big.NewInt(1), 256)
		payload := BatchPaymentPayload{
			ChainID: 1, Nonce: 1, Token: repeatAddr(0x01), CreatedAt: 1,
			Operations: []PaymentOperation{{Recipient: common.Address{}, Amount: tooWide}},
		}
		_, err := PrepareTransaction(payload)
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "exceeds U256") {
			t.Errorf("err = %q, want the U256 encoding error first", err)
		}
	})
}
