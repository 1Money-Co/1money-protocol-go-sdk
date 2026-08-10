package onemoney

import (
	"bytes"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
)

// TestBatchPaymentOptionalTrailingFields covers the one payload branch the L1
// golden vectors do not exercise: BatchPayment's optional trailing fields. Per
// native-v2-signing-spec §4.3, both fields absent -> omitted (no placeholders);
// an absent field before a present one -> the empty-string placeholder 0x80.
func TestBatchPaymentOptionalTrailingFields(t *testing.T) {
	base := func() BatchPaymentPayload {
		return BatchPaymentPayload{
			ChainID: 1, Nonce: 1, Token: repeatAddr(0x01),
			Operations: []PaymentOperation{{Recipient: repeatAddr(0x0c), Amount: big.NewInt(1000)}},
			CreatedAt:  1,
		}
	}
	h := common.BytesToHash(repeatBytes(0x11, 32))
	id := "batch-1"

	neither := base()
	hashOnly := base()
	hashOnly.OperationsHash = &h
	both := base()
	both.OperationsHash, both.BatchID = &h, &id
	idOnly := base()
	idOnly.BatchID = &id

	// elems returns the top-level RLP elements of a payload's field list.
	elems := func(p BatchPaymentPayload) []rlp.RawValue {
		raw, err := rlp.EncodeToBytes(p.rlpList())
		if err != nil {
			t.Fatal(err)
		}
		var out []rlp.RawValue
		if err := rlp.DecodeBytes(raw, &out); err != nil {
			t.Fatalf("decode payload rlp: %v", err)
		}
		return out
	}

	// Element counts per §4.3 (5 fixed fields + trailing optionals).
	if got := len(elems(neither)); got != 5 {
		t.Errorf("neither: got %d elements, want 5 (no trailing placeholders)", got)
	}
	if got := len(elems(hashOnly)); got != 6 {
		t.Errorf("hash-only: got %d elements, want 6", got)
	}
	if got := len(elems(both)); got != 7 {
		t.Errorf("both: got %d elements, want 7", got)
	}
	idOnlyElems := elems(idOnly)
	if len(idOnlyElems) != 7 {
		t.Fatalf("batch_id-only: got %d elements, want 7 (placeholder + batch_id)", len(idOnlyElems))
	}
	// The operations_hash slot (index 5) must be the 0x80 empty-string placeholder
	// when absent-before-present, not dropped or zero-filled.
	if !bytes.Equal(idOnlyElems[5], []byte{0x80}) {
		t.Errorf("batch_id-only: operations_hash slot = %x, want 0x80 placeholder", idOnlyElems[5])
	}

	// Every combination must feed the signed preimage: distinct inputs -> distinct
	// signing hashes through the public API (a dropped field would collide).
	seen := map[string]string{}
	for name, p := range map[string]BatchPaymentPayload{
		"neither": neither, "hashOnly": hashOnly, "both": both, "idOnly": idOnly,
	} {
		// Canonical encoder: `hashOnly` and `both` carry an arbitrary
		// operations_hash, which the admission gate rejects. What this loop pins
		// is that each optional-field combination reaches the signed preimage.
		prep, err := prepareCanonical(p, resolveSubmit(nil))
		if err != nil {
			t.Fatalf("%s: prepare: %v", name, err)
		}
		hh := hexLower(prep.SigningHash())
		if prev, dup := seen[hh]; dup {
			t.Errorf("%s and %s produced the same signing hash %s (an optional field was dropped)", name, prev, hh)
		}
		seen[hh] = name
	}
}

// TestBatchPaymentOptionalGoldenVectors validates every trailing-Option
// encoding class against raw-field vectors exported by the Rust production
// implementation. Since BatchPayment became memo-bearing, the generator emits
// each option class twice — once with the canonical empty memo and once with a
// populated one (the "_memo" companion) — so both memo states of every option
// class are pinned to the L1 oracle. Structural RLP assertions remain in the
// test above.
func TestBatchPaymentOptionalGoldenVectors(t *testing.T) {
	required := map[string]bool{"BatchPayment_canonical": false}
	for _, class := range []string{"neither", "hash_only", "id_only", "both", "empty_id", "zero_hash"} {
		required["batch_option_"+class] = false
		required["batch_option_"+class+"_memo"] = false
	}
	for _, vector := range loadPrepareAuthorizeFixture(t).Vectors {
		if _, ok := required[vector.Name]; !ok {
			continue
		}
		required[vector.Name] = true
		vector := vector
		t.Run(vector.Name, func(t *testing.T) {
			payload, options := vector.goPayload(t)
			batch, ok := payload.(BatchPaymentPayload)
			if !ok {
				t.Fatalf("decoded payload type = %T, want BatchPaymentPayload", payload)
			}
			// The "_memo" companions must actually carry a memo, and the others must
			// not — otherwise a memo-state regression would silently stop being tested.
			wantMemo := strings.HasSuffix(vector.Name, "_memo")
			if gotMemo := vector.Options.Memo != nil; gotMemo != wantMemo {
				t.Fatalf("options.memo present = %v, want %v", gotMemo, wantMemo)
			}
			// Canonical encoder, not PrepareTransaction: these vectors pin the
			// encoding of optional-field combinations, and several carry an
			// arbitrary operations_hash the admission gate rightly rejects.
			prep, err := prepareCanonical(batch, resolveSubmit(options))
			if err != nil {
				t.Fatalf("prepareCanonical: %v", err)
			}
			if got := hexLower(prep.SigningHash()); got != vector.Expected.SigningHash {
				t.Fatalf("SigningHash\n got %s\nwant %s (Rust oracle)", got, vector.Expected.SigningHash)
			}
		})
	}
	for name, covered := range required {
		if !covered {
			t.Errorf("missing BatchPayment optional vector %q", name)
		}
	}
}

func TestBatchPaymentPairwiseGoldenCoverage(t *testing.T) {
	required := make(fixtureStringSet)
	observed := make(fixtureStringSet)
	cross := func(leftFactor string, left []string, rightFactor string, right []string) {
		for _, leftLevel := range left {
			for _, rightLevel := range right {
				required.add(leftFactor + ":" + leftLevel + "|" + rightFactor + ":" + rightLevel)
			}
		}
	}
	optionLevels := []string{"neither", "hash_only", "id_only", "both"}
	operationLevels := []string{"empty", "single", "forward", "reverse"}
	amountLevels := []string{"ordinary", "zero", "max"}
	memoLevels := []string{"empty", "populated"}
	cross("option", optionLevels, "operations", operationLevels)
	cross("option", optionLevels, "amount", amountLevels)
	cross("option", optionLevels, "memo", memoLevels)
	cross("operations", operationLevels, "memo", memoLevels)
	cross("operations", operationLevels[1:], "amount", amountLevels)
	cross("amount", amountLevels, "memo", memoLevels)

	maxU256 := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1)).String()
	for _, vector := range loadPrepareAuthorizeFixture(t).Vectors {
		if vector.Operation != "BatchPayment" {
			continue
		}
		raw := decodeFixturePayload[batchFixturePayload](t, vector.Payload)
		optionLevel := "neither"
		switch {
		case raw.OperationsHash != nil && raw.BatchID != nil:
			optionLevel = "both"
		case raw.OperationsHash != nil:
			optionLevel = "hash_only"
		case raw.BatchID != nil:
			optionLevel = "id_only"
		}
		operationLevel := ""
		switch len(raw.Operations) {
		case 0:
			operationLevel = "empty"
		case 1:
			operationLevel = "single"
		case 2:
			if strings.HasPrefix(raw.Operations[0].Recipient, "0x0c") {
				operationLevel = "forward"
			} else {
				operationLevel = "reverse"
			}
		default:
			t.Fatalf("%s: unexpected operation count %d", vector.Name, len(raw.Operations))
		}
		memoLevel := "empty"
		if vector.Options.Memo != nil {
			memoLevel = "populated"
		}
		observed.add("option:" + optionLevel + "|operations:" + operationLevel)
		observed.add("option:" + optionLevel + "|memo:" + memoLevel)
		observed.add("operations:" + operationLevel + "|memo:" + memoLevel)

		if len(raw.Operations) != 0 {
			amountLevel := "ordinary"
			if raw.Operations[0].Amount == "0" {
				amountLevel = "zero"
			} else if raw.Operations[0].Amount == maxU256 {
				amountLevel = "max"
			}
			observed.add("option:" + optionLevel + "|amount:" + amountLevel)
			observed.add("operations:" + operationLevel + "|amount:" + amountLevel)
			observed.add("amount:" + amountLevel + "|memo:" + memoLevel)
		}
	}

	assertFixtureSetContains(t, "BatchPayment pairwise", observed, sortedFixtureSet(required)...)
}

// TestBatchPaymentAcceptsMemo verifies BatchPayment is memo-bearing like every
// other canonical v2 operation: a memo prepares successfully and changes the
// signing hash, because the memo is inside WithMemo<BatchPaymentPayload>.
func TestBatchPaymentAcceptsMemo(t *testing.T) {
	batch := BatchPaymentPayload{
		ChainID: 1, Nonce: 1, Token: repeatAddr(0x01),
		Operations: []PaymentOperation{{Recipient: repeatAddr(0x0c), Amount: big.NewInt(1000)}},
		CreatedAt:  1,
	}

	bare, err := PrepareTransaction(batch)
	if err != nil {
		t.Fatalf("PrepareTransaction(batch) without memo: %v", err)
	}
	withMemo, err := PrepareTransaction(batch, WithMemo(Memo{Type: "purpose/SALA", Format: "text/plain", Data: "x"}))
	if err != nil {
		t.Fatalf("PrepareTransaction(batch) with memo: %v", err)
	}
	if bytes.Equal(bare.SigningHash(), withMemo.SigningHash()) {
		t.Error("memo did not change the batch signing hash; the memo is not in the signed preimage")
	}
}

// TestDeriveBatchPaymentOperationsHashMatchesRustOracle checks the exported
// derivation against expected.operations_hash emitted by the L1 generator. A
// Go-computed hash compared only with another Go-computed value is not an
// acceptable oracle.
func TestDeriveBatchPaymentOperationsHashMatchesRustOracle(t *testing.T) {
	covered := 0
	for _, vector := range loadPrepareAuthorizeFixture(t).Vectors {
		if vector.Operation != "BatchPayment" || vector.Expected.OperationsHash == "" {
			continue
		}
		vector := vector
		t.Run(vector.Name, func(t *testing.T) {
			payload, _ := vector.goPayload(t)
			batch, ok := payload.(BatchPaymentPayload)
			if !ok {
				t.Fatalf("decoded payload type = %T, want BatchPaymentPayload", payload)
			}
			got, err := DeriveBatchPaymentOperationsHash(batch.Operations)
			if err != nil {
				t.Fatalf("derive: %v", err)
			}
			if !strings.EqualFold(got.Hex(), vector.Expected.OperationsHash) {
				t.Fatalf("operations_hash\n got %s\nwant %s (Rust oracle)", got.Hex(), vector.Expected.OperationsHash)
			}
		})
		covered++
	}
	if covered == 0 {
		t.Fatal("no BatchPayment vector carried expected.operations_hash; regenerate the fixture from the L1 oracle")
	}
}

// TestDeriveBatchPaymentOperationsHashNilAmount pins the nil == U256-zero rule
// that the submit encoder already applies, so the helper hashes exactly what the
// submit path signs.
func TestDeriveBatchPaymentOperationsHashNilAmount(t *testing.T) {
	nilAmount, err := DeriveBatchPaymentOperationsHash([]PaymentOperation{{Recipient: repeatAddr(0x0c), Amount: nil}})
	if err != nil {
		t.Fatalf("nil amount must not error: %v", err)
	}
	zeroAmount, err := DeriveBatchPaymentOperationsHash([]PaymentOperation{{Recipient: repeatAddr(0x0c), Amount: big.NewInt(0)}})
	if err != nil {
		t.Fatalf("zero amount: %v", err)
	}
	if nilAmount != zeroAmount {
		t.Errorf("nil amount hash %s != zero amount hash %s", nilAmount.Hex(), zeroAmount.Hex())
	}
}

// TestDeriveBatchPaymentOperationsHashRejectsOutOfRange checks the same U256
// bounds the submit path enforces.
func TestDeriveBatchPaymentOperationsHashRejectsOutOfRange(t *testing.T) {
	tooWide := new(big.Int).Lsh(big.NewInt(1), 256)
	for name, amount := range map[string]*big.Int{
		"negative": big.NewInt(-1),
		"too wide": tooWide,
	} {
		if _, err := DeriveBatchPaymentOperationsHash([]PaymentOperation{{Recipient: repeatAddr(0x0c), Amount: amount}}); err == nil {
			t.Errorf("%s amount was accepted; want an error", name)
		}
	}
}

// TestDeriveBatchPaymentOperationsHashIsOrderSensitive guards against a helper
// that sorts or normalizes operations: L1 hashes the list as given.
func TestDeriveBatchPaymentOperationsHashIsOrderSensitive(t *testing.T) {
	forward := []PaymentOperation{
		{Recipient: repeatAddr(0x0c), Amount: big.NewInt(1000)},
		{Recipient: repeatAddr(0x0d), Amount: big.NewInt(2000)},
	}
	reverse := []PaymentOperation{forward[1], forward[0]}
	a, err := DeriveBatchPaymentOperationsHash(forward)
	if err != nil {
		t.Fatal(err)
	}
	b, err := DeriveBatchPaymentOperationsHash(reverse)
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Error("reordering operations must change the hash")
	}
}

// TestPaymentValueEdgeCases covers U256 boundary inputs the golden vectors do
// not: nil (treated as zero), and the maximum 256-bit value.
func TestPaymentValueEdgeCases(t *testing.T) {
	hashFor := func(v *big.Int) []byte {
		p := PaymentPayload{ChainID: 1, Nonce: 1, Recipient: repeatAddr(0x02), Value: v, Token: repeatAddr(0x01)}
		prep, err := PrepareTransaction(p)
		if err != nil {
			t.Fatalf("prepare: %v", err)
		}
		return prep.SigningHash()
	}

	// A nil Value must not panic and must hash identically to an explicit zero.
	if !bytes.Equal(hashFor(nil), hashFor(big.NewInt(0))) {
		t.Error("nil Value must hash identically to zero Value")
	}

	// The maximum U256 (2^256 - 1) must encode without panic and differ from zero.
	maxU256 := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	if bytes.Equal(hashFor(maxU256), hashFor(big.NewInt(0))) {
		t.Error("max-U256 Value must not collide with zero Value")
	}
}

// TestPrepareRejectsInadmissibleBatchPayment pins the node's static,
// governance-independent BatchPayment rules at the SDK boundary. Each case here
// is one the node rejects at admission without consulting governance state, so
// failing before signing costs the caller nothing and saves a signing operation
// plus a round trip -- and, for an HSM- or KMS-backed Signer, a real key use.
func TestPrepareRejectsInadmissibleBatchPayment(t *testing.T) {
	maxU256 := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	base := func(operations []PaymentOperation) BatchPaymentPayload {
		return BatchPaymentPayload{
			ChainID: 1, Nonce: 1, Token: repeatAddr(0x01),
			Operations: operations, CreatedAt: 1,
		}
	}
	good := []PaymentOperation{{Recipient: repeatAddr(0x0c), Amount: big.NewInt(1000)}}

	wrongHash := common.BytesToHash(repeatBytes(0x11, 32))
	staleOperations := []PaymentOperation{{Recipient: repeatAddr(0x0c), Amount: big.NewInt(1000)}}
	staleHash, err := DeriveBatchPaymentOperationsHash(staleOperations)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name    string
		payload BatchPaymentPayload
		want    string
	}{
		{"empty operations", base(nil), "operations must not be empty"},
		{"empty operations slice", base([]PaymentOperation{}), "operations must not be empty"},
		{
			"zero recipient",
			base([]PaymentOperation{{Recipient: common.Address{}, Amount: big.NewInt(1)}}),
			"operation 0 has an invalid recipient",
		},
		{
			"zero recipient at a later index",
			base([]PaymentOperation{
				{Recipient: repeatAddr(0x0c), Amount: big.NewInt(1)},
				{Recipient: common.Address{}, Amount: big.NewInt(1)},
			}),
			"operation 1 has an invalid recipient",
		},
		{
			"explicit zero amount",
			base([]PaymentOperation{{Recipient: repeatAddr(0x0c), Amount: big.NewInt(0)}}),
			"operation 0 amount must be greater than 0",
		},
		{
			"nil amount is zero and equally inadmissible",
			base([]PaymentOperation{{Recipient: repeatAddr(0x0c), Amount: nil}}),
			"operation 0 amount must be greater than 0",
		},
		{
			"total overflows U256",
			base([]PaymentOperation{
				{Recipient: repeatAddr(0x0c), Amount: maxU256},
				{Recipient: repeatAddr(0x0d), Amount: big.NewInt(1)},
			}),
			"total amount overflows U256",
		},
		{
			"non-canonical operations_hash",
			func() BatchPaymentPayload {
				p := base(good)
				p.OperationsHash = &wrongHash
				return p
			}(),
			"operations_hash mismatch",
		},
		{
			"stale operations_hash after editing operations",
			func() BatchPaymentPayload {
				// The realistic mistake: derive the hash, then change the
				// operations without re-deriving.
				p := base([]PaymentOperation{
					{Recipient: repeatAddr(0x0c), Amount: big.NewInt(1000)},
					{Recipient: repeatAddr(0x0d), Amount: big.NewInt(2000)},
				})
				p.OperationsHash = &staleHash
				return p
			}(),
			"operations_hash mismatch",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_, err := PrepareTransaction(tc.payload)
			if err == nil {
				t.Fatalf("payload was accepted; want rejection containing %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %q, want it to contain %q", err, tc.want)
			}
		})
	}

	t.Run("admissible payload still prepares", func(t *testing.T) {
		if _, err := PrepareTransaction(base(good)); err != nil {
			t.Fatalf("a valid batch payment must prepare: %v", err)
		}
	})

	t.Run("canonical operations_hash is accepted", func(t *testing.T) {
		p := base(good)
		hash, err := DeriveBatchPaymentOperationsHash(good)
		if err != nil {
			t.Fatal(err)
		}
		p.OperationsHash = &hash
		if _, err := PrepareTransaction(p); err != nil {
			t.Fatalf("a canonically derived operations_hash must be accepted: %v", err)
		}
	})
}

// TestDeriveBatchPaymentOperationsHashStaysPermissive guards the other half of
// the split: the exported derivation mirrors the node's PURE
// canonical_operations_hash domain, so it must keep accepting inputs the
// admission gate rejects. Tightening it to match the gate would make it
// impossible to compute the hash of a batch before fixing it up, and would break
// the fixture vectors that pin those encodings.
func TestDeriveBatchPaymentOperationsHashStaysPermissive(t *testing.T) {
	if _, err := DeriveBatchPaymentOperationsHash(nil); err != nil {
		t.Errorf("empty operation list must hash, not error: %v", err)
	}
	zero := []PaymentOperation{{Recipient: common.Address{}, Amount: big.NewInt(0)}}
	if _, err := DeriveBatchPaymentOperationsHash(zero); err != nil {
		t.Errorf("zero recipient and zero amount must hash, not error: %v", err)
	}
	// And the gate rejects that same input, which is the point of the split.
	if err := validateBatchOperationsStatic(zero); err == nil {
		t.Error("the admission gate must reject what the pure helper accepts here")
	}
}
