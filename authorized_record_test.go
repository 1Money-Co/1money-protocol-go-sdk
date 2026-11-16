package onemoney

import (
	"bytes"
	"encoding/json"
	"math/big"
	"strings"
	"testing"
)

func storableCases() []struct {
	name    string
	payload any
	opts    []SubmitOption
} {
	memo := WithMemo(Memo{Type: "compliance", Format: "text", Data: "case 7"})
	token := repeatAddr(0x01)
	account := repeatAddr(0x02)
	return []struct {
		name    string
		payload any
		opts    []SubmitOption
	}{
		{"settled", settledVectorPayload(SettledUpdateMetadata{Name: "N", URI: "u", AdditionalMetadata: []AdditionalMetadata{{Key: "k", Value: "v"}}}), []SubmitOption{memo}},
		{"settled mint", settledVectorPayload(SettledMint{Recipient: account, Value: big.NewInt(9)}), nil},
		{"lane clawback", laneVectorPayload(), []SubmitOption{memo}},
		{"clawback", TokenClawbackPayload{ChainID: 1, Nonce: 2, Token: token, From: account, Recipient: repeatAddr(3), Value: big.NewInt(4)}, nil},
		{"mint", TokenMintPayload{ChainID: 1, Nonce: 2, Recipient: account, Value: big.NewInt(5), Token: token}, nil},
		{"authority", TokenAuthorityPayload{ChainID: 1, Nonce: 2, Action: AuthorityActionGrant, AuthorityType: AuthorityTypeMintBurnTokens, AuthorityAddress: account, Token: token, Value: big.NewInt(6)}, nil},
		{"blacklist", TokenManageListPayload{ChainID: 1, Nonce: 2, Action: ManageListActionAdd, Address: account, Token: token}, []SubmitOption{WithManageListKind(ManageListBlacklist)}},
		{"whitelist", TokenManageListPayload{ChainID: 1, Nonce: 2, Action: ManageListActionAdd, Address: account, Token: token}, []SubmitOption{WithManageListKind(ManageListWhitelist)}},
		{"pause", PauseTokenPayload{ChainID: 1, Nonce: 2, Action: UnPause, Token: token}, nil},
		{"metadata", UpdateMetadataPayload{ChainID: 1, Nonce: 2, Name: "N", URI: "u", Token: token}, nil},
	}
}

func TestAuthorizedTransactionRoundTripsWithItsHash(t *testing.T) {
	for _, tc := range storableCases() {
		t.Run(tc.name, func(t *testing.T) {
			authorized, _ := authorizedBody(t, tc.payload, tc.opts...)
			stored, err := json.Marshal(authorized)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			loaded, err := ParseAuthorizedTransaction(stored)
			if err != nil {
				t.Fatalf("parse %s: %v", stored, err)
			}
			if !bytes.Equal(loaded.TransactionHash(), authorized.TransactionHash()) || loaded.path != authorized.path || loaded.op != authorized.op {
				t.Fatalf("loaded %x at %s, want %x at %s", loaded.TransactionHash(), loaded.path, authorized.TransactionHash(), authorized.path)
			}
			again, err := json.Marshal(loaded)
			if err != nil || !bytes.Equal(again, stored) {
				t.Fatalf("second marshal differs (%v)\n%s\n%s", err, again, stored)
			}
		})
	}
}

func TestParseAuthorizedTransactionRefusesATamperedRecord(t *testing.T) {
	authorized, _ := authorizedBody(t, laneVectorPayload())
	stored, err := json.Marshal(authorized)
	if err != nil {
		t.Fatal(err)
	}
	tampered := []struct {
		name, from, to, want string
	}{
		{"value", `"value":"1000000000000000000"`, `"value":"2000000000000000000"`, "hash"},
		{"stored hash", `"transaction_hash":"0x`, `"transaction_hash":"0x00`, "hash"},
		{"unknown member", `"from_nonce":7`, `"from_nonce":7,"nonce":7`, "unknown field"},
		{"path", `/v2/tokens/lane_clawback`, `/v2/transactions/payment`, "not a stored authority-class"},
	}
	for _, tc := range tampered {
		bad := strings.Replace(string(stored), tc.from, tc.to, 1)
		if bad == string(stored) {
			t.Fatalf("%s: pattern %s not in %s", tc.name, tc.from, stored)
		}
		if _, err := ParseAuthorizedTransaction([]byte(bad)); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want it to name %q", tc.name, err, tc.want)
		}
	}
}

func TestParseAuthorizedTransactionRefusesAMemberOfAnotherOperation(t *testing.T) {
	authorized, _ := authorizedBody(t, settledVectorPayload(SettledPause{}))
	stored, err := json.Marshal(authorized)
	if err != nil {
		t.Fatal(err)
	}
	bad := strings.Replace(string(stored), `{"type":"pause"}`, `{"account":"0x0202020202020202020202020202020202020202","type":"pause"}`, 1)
	if bad == string(stored) {
		t.Fatalf("pattern not in %s", stored)
	}
	if _, err := ParseAuthorizedTransaction([]byte(bad)); err == nil || !strings.Contains(err.Error(), "exact body") {
		t.Fatalf("err = %v, want the exact-body refusal", err)
	}
}

func TestOnlyAuthorityClassTransactionsAreStorable(t *testing.T) {
	if _, err := json.Marshal(authorizedPayment(t)); err == nil {
		t.Fatal("a payment has no stored form")
	}
}
