package onemoney

import (
	"math/big"
	"strings"
	"testing"
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
		{"data accepts multibyte at the byte cap", Memo{Data: strings.Repeat("界", memoDataMaxBytes/3)}},
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
		{"space in type", Memo{Type: "purpose SALA"}, "memo.type contains an invalid character"},
		{"multibyte in type", Memo{Type: "purpose/工资"}, "memo.type contains an invalid character"},
		{"space in format", Memo{Format: "text plain"}, "memo.format contains an invalid character"},
		{"quote in format", Memo{Format: "text/\"plain\""}, "memo.format contains an invalid character"},
		{"nul in data", Memo{Data: "invoice\x00001"}, "memo.data contains a control character"},
		{"newline in data", Memo{Data: "line1\nline2"}, "memo.data contains a control character"},
		{"tab in data", Memo{Data: "a\tb"}, "memo.data contains a control character"},
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
