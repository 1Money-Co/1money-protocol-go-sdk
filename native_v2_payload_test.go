package onemoney

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

// This test validates the Go payload encoders (rlpList) by reproducing the
// exact canonical inputs from the L1 fixture generator
// (l1client crates/types/om-primitives-types/examples/native_domain_separated_payload_fixtures.rs)
// and asserting the resulting payload_rlp matches each golden vector. Together
// with TestNativeV2Conformance (which validates the outer hasher given
// payload_rlp), this closes the loop: Go signing is byte-identical to Rust.

const fixtureChainID = 1212101

func repeatAddr(b byte) common.Address {
	var a common.Address
	for i := range a {
		a[i] = b
	}
	return a
}

func repeatBytes(b byte, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = b
	}
	return out
}

func vectorsByName(t *testing.T) map[string]conformanceVector {
	t.Helper()
	m := make(map[string]conformanceVector)
	for _, v := range loadVectors(t) {
		m[v.Name] = v
	}
	return m
}

func TestNativeV2PayloadEncoders(t *testing.T) {
	byName := vectorsByName(t)
	populatedMemo := Memo{Type: "purpose/SALA", Format: "text/plain", Data: "invoice-0001"}
	emptyMemo := EmptyMemo()

	pk1 := append([]byte{0x02}, repeatBytes(0x11, 32)...)
	pk2 := append([]byte{0x03}, repeatBytes(0x22, 32)...)

	cases := []struct {
		name string
		list []interface{}
		memo *Memo // nil => bare (no WithMemo wrapper), e.g. BatchPayment
	}{
		{"Payment_single", PaymentPayload{ChainID: fixtureChainID, Nonce: 1, Recipient: repeatAddr(0x02), Value: big.NewInt(1_000_000_000_000_000_000), Token: repeatAddr(0x01)}.rlpList(), &populatedMemo},
		{"TokenIssue_single", TokenIssuePayload{ChainID: fixtureChainID, Nonce: 2, Symbol: "TEST", Name: "Test Token", Decimals: 8, MasterAuthority: repeatAddr(0x03), IsPrivate: false, ClawbackEnabled: true}.rlpList(), &emptyMemo},
		{"TokenMint_single", TokenMintPayload{ChainID: fixtureChainID, Nonce: 3, Recipient: repeatAddr(0x04), Value: big.NewInt(500_000_000_000), Token: repeatAddr(0x01)}.rlpList(), &emptyMemo},
		{"TokenAuthority_single", TokenAuthorityPayload{ChainID: fixtureChainID, Nonce: 4, Action: AuthorityActionGrant, AuthorityType: AuthorityTypeMintBurnTokens, AuthorityAddress: repeatAddr(0x05), Token: repeatAddr(0x01), Value: big.NewInt(100_000_000)}.rlpList(), &emptyMemo},
		{"TokenBlacklist_single", TokenManageListPayload{ChainID: fixtureChainID, Nonce: 5, Action: ManageListActionAdd, Address: repeatAddr(0x06), Token: repeatAddr(0x01)}.rlpList(), &emptyMemo},
		{"TokenWhitelist_single", TokenManageListPayload{ChainID: fixtureChainID, Nonce: 6, Action: ManageListActionAdd, Address: repeatAddr(0x07), Token: repeatAddr(0x01)}.rlpList(), &emptyMemo},
		{"TokenPause_single", PauseTokenPayload{ChainID: fixtureChainID, Nonce: 7, Action: Pause, Token: repeatAddr(0x01)}.rlpList(), &emptyMemo},
		{"TokenBurn_single", TokenBurnPayload{ChainID: fixtureChainID, Nonce: 8, Value: big.NewInt(250_000_000), Token: repeatAddr(0x01)}.rlpList(), &emptyMemo},
		{"TokenClawback_single", TokenClawbackPayload{ChainID: fixtureChainID, Nonce: 9, Token: repeatAddr(0x01), From: repeatAddr(0x08), Recipient: repeatAddr(0x09), Value: big.NewInt(42_000_000)}.rlpList(), &emptyMemo},
		{"TokenMetadata_single", UpdateMetadataPayload{ChainID: fixtureChainID, Nonce: 10, Name: "Test Token", URI: "https://example.com/token.json", Token: repeatAddr(0x01), AdditionalMetadata: []AdditionalMetadata{{Key: "version", Value: "1.0"}, {Key: "author", Value: "OneMoney Team"}}}.rlpList(), &emptyMemo},
		{"TokenBridgeAndMint_single", TokenBridgeAndMintPayload{ChainID: fixtureChainID, Nonce: 11, Recipient: repeatAddr(0x0a), Value: big.NewInt(1_000_000_000), Token: repeatAddr(0x01), SourceChainID: 1, SourceTxHash: "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", BridgeMetadata: ""}.rlpList(), &emptyMemo},
		{"TokenBurnAndBridge_single", TokenBurnAndBridgePayload{ChainID: fixtureChainID, Nonce: 12, Sender: repeatAddr(0x0b), Value: big.NewInt(500_000_000), Token: repeatAddr(0x01), DestinationChainID: 1, DestinationAddress: "0x1234567890abcdef1234567890abcdef12345678", EscrowFee: big.NewInt(1_000_000), BridgeMetadata: "", BridgeParam: HexBytes{}}.rlpList(), &emptyMemo},
		{"CreateMultiSig_single", CreateMultiSigPayload{ChainID: fixtureChainID, Nonce: 13, Signers: []MultiSigSigner{{PublicKey: pk1, Weight: 1}, {PublicKey: pk2, Weight: 1}}, Threshold: 2}.rlpList(), &emptyMemo},
		{"BatchPayment_single", BatchPaymentPayload{ChainID: fixtureChainID, Nonce: 14, Token: repeatAddr(0x01), Operations: []PaymentOperation{{Recipient: repeatAddr(0x0c), Amount: big.NewInt(1000)}, {Recipient: repeatAddr(0x0d), Amount: big.NewInt(2000)}}, MaxFee: big.NewInt(5000), CreatedAt: 1_747_785_600}.rlpList(), nil},
		{"payment_memo_empty", PaymentPayload{ChainID: fixtureChainID, Nonce: 1, Recipient: repeatAddr(0x02), Value: big.NewInt(1_000_000_000_000_000_000), Token: repeatAddr(0x01)}.rlpList(), &emptyMemo},
		{"payment_memo_populated", PaymentPayload{ChainID: fixtureChainID, Nonce: 1, Recipient: repeatAddr(0x02), Value: big.NewInt(1_000_000_000_000_000_000), Token: repeatAddr(0x01)}.rlpList(), &populatedMemo},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			want, ok := byName[tc.name]
			if !ok {
				t.Fatalf("no golden vector named %q", tc.name)
			}
			var got []byte
			var err error
			if tc.memo == nil {
				got, err = encodeBare(tc.list)
			} else {
				got, err = encodeWithMemo(tc.list, *tc.memo)
			}
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			if hexLower(got) != want.PayloadRLP {
				t.Fatalf("payload_rlp\n got %s\nwant %s", hexLower(got), want.PayloadRLP)
			}
		})
	}
}
