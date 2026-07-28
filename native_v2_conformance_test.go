package onemoney

import (
	"encoding/json"
	"math/big"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// This test proves the Go domain-separated signing core is byte-for-byte
// identical to the L1 reference implementation, by recomputing every field of
// every golden vector and asserting equality. The fixture is copied verbatim
// from l1client docs/specs/fixtures/native-v2-signing-vectors.json.

type conformanceFile struct {
	BaseVectors         []conformanceVector `json:"base_vectors"`
	SupplementalVectors []conformanceVector `json:"supplemental_vectors"`
}

type conformanceVector struct {
	Name                   string          `json:"name"`
	OperationType          uint16          `json:"operation_type"`
	AuthorizationKind      int             `json:"authorization_kind"`
	MultisigAccount        *string         `json:"multisig_account"`
	PayloadRLP             string          `json:"payload_rlp"`
	AuthorizationProof     json.RawMessage `json:"authorization_proof"`
	UnsignedTransactionRLP string          `json:"unsigned_transaction_rlp"`
	SigningHash            string          `json:"signing_hash"`
	SignedTransactionRLP   string          `json:"signed_transaction_rlp"`
	TransactionHash        string          `json:"transaction_hash"`
}

type conformanceSig struct {
	R string `json:"r"`
	S string `json:"s"`
	V uint64 `json:"v"`
}

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

func loadVectors(t *testing.T) []conformanceVector {
	t.Helper()
	raw, err := os.ReadFile("testdata/native-v2-signing-vectors.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var f conformanceFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	return append(append([]conformanceVector{}, f.BaseVectors...), f.SupplementalVectors...)
}

func (v conformanceVector) descriptor(t *testing.T) []interface{} {
	t.Helper()
	switch v.AuthorizationKind {
	case 0:
		return singleDescriptor()
	case 1:
		if v.MultisigAccount == nil {
			t.Fatalf("%s: multisig kind without account", v.Name)
		}
		return multiDescriptor(common.HexToAddress(*v.MultisigAccount))
	default:
		t.Fatalf("%s: unknown authorization_kind %d", v.Name, v.AuthorizationKind)
		return nil
	}
}

func vectorsByName(t *testing.T) map[string]conformanceVector {
	t.Helper()
	m := make(map[string]conformanceVector)
	for _, v := range loadVectors(t) {
		m[v.Name] = v
	}
	return m
}

// TestNativeV2PayloadEncoders validates the Go payload encoders (rlpList) by
// reproducing the exact canonical inputs from the L1 fixture generator
// (l1client crates/types/om-primitives-types/examples/native_domain_separated_payload_fixtures.rs)
// and asserting the resulting payload_rlp matches each golden vector. Together
// with TestNativeV2Conformance (which validates the outer hasher given
// payload_rlp), this closes the loop: Go signing is byte-identical to Rust.
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

func (v conformanceVector) proof(t *testing.T) interface{} {
	t.Helper()
	if v.AuthorizationKind == 0 {
		var p struct {
			Signature conformanceSig `json:"signature"`
		}
		if err := json.Unmarshal(v.AuthorizationProof, &p); err != nil {
			t.Fatalf("%s: proof: %v", v.Name, err)
		}
		return singleProof(Signature{R: p.Signature.R, S: p.Signature.S, V: p.Signature.V})
	}
	var p struct {
		Signatures []struct {
			SignerPubkey string         `json:"signer_pubkey"`
			Signature    conformanceSig `json:"signature"`
		} `json:"signatures"`
	}
	if err := json.Unmarshal(v.AuthorizationProof, &p); err != nil {
		t.Fatalf("%s: proof: %v", v.Name, err)
	}
	entries := make([]multiSigProofEntry, 0, len(p.Signatures))
	for _, e := range p.Signatures {
		entries = append(entries, multiSigProofEntry{
			pubkey: common.FromHex(e.SignerPubkey),
			sig:    Signature{R: e.Signature.R, S: e.Signature.S, V: e.Signature.V},
		})
	}
	return multiProof(entries)
}

func TestNativeV2Conformance(t *testing.T) {
	vectors := loadVectors(t)
	if len(vectors) == 0 {
		t.Fatal("no vectors loaded")
	}
	for _, v := range vectors {
		v := v
		t.Run(v.Name, func(t *testing.T) {
			op := nativeOperationType(v.OperationType)
			descriptor := v.descriptor(t)
			payloadRLP := common.FromHex(v.PayloadRLP)

			unsigned, err := unsignedTxRLP(op, descriptor, payloadRLP)
			if err != nil {
				t.Fatalf("unsigned: %v", err)
			}
			if got := hexLower(unsigned); got != v.UnsignedTransactionRLP {
				t.Fatalf("unsigned_transaction_rlp\n got %s\nwant %s", got, v.UnsignedTransactionRLP)
			}
			if got := hexLower(crypto.Keccak256(unsigned)); got != v.SigningHash {
				t.Fatalf("signing_hash\n got %s\nwant %s", got, v.SigningHash)
			}

			proof := v.proof(t)
			signed, err := signedTxRLP(op, descriptor, payloadRLP, proof)
			if err != nil {
				t.Fatalf("signed: %v", err)
			}
			if got := hexLower(signed); got != v.SignedTransactionRLP {
				t.Fatalf("signed_transaction_rlp\n got %s\nwant %s", got, v.SignedTransactionRLP)
			}
			if got := hexLower(crypto.Keccak256(signed)); got != v.TransactionHash {
				t.Fatalf("transaction_hash\n got %s\nwant %s", got, v.TransactionHash)
			}
		})
	}
}
