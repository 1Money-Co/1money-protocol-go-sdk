package onemoney

import (
	"encoding/json"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

// The typed inputs of the operation 17 and 18 vectors, as l1client pins them in
// crates/types/om-primitives-types/src/transaction/native_domain_separated/tests/hashing.rs.
func settledVectorPayload(operation SettledAuthorityOperation) SettledAuthorityPayload {
	return SettledAuthorityPayload{
		ChainID: fixtureChainID, Nonce: 1, Token: repeatAddr(0x01),
		ReferenceCheckpoint: 42, Operation: operation,
	}
}

func laneVectorPayload() LaneClawbackPayload {
	return LaneClawbackPayload{
		ChainID: fixtureChainID, Token: repeatAddr(0x01), Holder: repeatAddr(0x02),
		FromNonce: 7, ReferenceCheckpoint: 42, Recipient: repeatAddr(0x03),
		Value: big.NewInt(1_000_000_000_000_000_000),
	}
}

func oneToken() *big.Int { return big.NewInt(1_000_000_000_000_000_000) }

func TestSettledAndLaneClawbackPayloadsMatchGoldenVectors(t *testing.T) {
	byName := vectorsByName(t)
	populated := Memo{Type: "purpose/SALA", Format: "text/plain", Data: "invoice-0001"}
	empty := EmptyMemo()
	account := repeatAddr(0x02)

	cases := []struct {
		name    string
		payload any
		memo    Memo
	}{
		{"SettledAuthority_single", settledVectorPayload(SettledGrantRole{Role: SettledRoleClawback, Account: account}), populated},
		{"SettledAuthority_multi", settledVectorPayload(SettledGrantRole{Role: SettledRoleClawback, Account: account}), populated},
		{"settled_authority_pause", settledVectorPayload(SettledPause{}), empty},
		{"settled_authority_list", settledVectorPayload(SettledList{List: SettledListWhitelist, Action: SettledListRemove, Account: account}), empty},
		{"settled_authority_update_metadata_pairs", settledVectorPayload(SettledUpdateMetadata{
			Name: "Token", URI: "https://example.com/token.json",
			AdditionalMetadata: []AdditionalMetadata{{Key: "version", Value: "1"}, {Key: "issuer", Value: "1money"}},
		}), empty},
		{"collision_settled_set_minter_allowance", settledVectorPayload(SettledSetMinterAllowance{Minter: account, Allowance: oneToken()}), empty},
		{"collision_settled_mint", settledVectorPayload(SettledMint{Recipient: account, Value: oneToken()}), empty},
		{"LaneClawback_single", laneVectorPayload(), populated},
		{"LaneClawback_multi", laneVectorPayload(), populated},
		{"lane_clawback_memo_empty", laneVectorPayload(), empty},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want, ok := byName[tc.name]
			if !ok {
				t.Fatalf("no golden vector named %q", tc.name)
			}
			op, list, _, err := resolvePayloadOp(tc.payload, submitConfig{})
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			if uint16(op) != want.OperationType {
				t.Fatalf("operation type = %d, want %d", op, want.OperationType)
			}
			got, err := encodeWithMemo(list, tc.memo)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			if hexLower(got) != want.PayloadRLP {
				t.Fatalf("payload_rlp\n got %s\nwant %s", hexLower(got), want.PayloadRLP)
			}
			if want.AuthorizationKind != 0 {
				return
			}
			prepared, err := prepareCanonical(tc.payload, submitConfig{memo: tc.memo, memoSet: true})
			if err != nil {
				t.Fatalf("prepare: %v", err)
			}
			if got := hexLower(prepared.SigningHash()); got != want.SigningHash {
				t.Fatalf("signing_hash\n got %s\nwant %s", got, want.SigningHash)
			}
			// The vector signatures are synthetic and not low-S, so Authorize
			// refuses them; hash the signed form directly.
			txHash, err := txHashV2(prepared.op, prepared.descriptor, prepared.payloadRLP, want.proof(t))
			if err != nil {
				t.Fatalf("hash: %v", err)
			}
			if got := hexLower(txHash); got != want.TransactionHash {
				t.Fatalf("transaction_hash\n got %s\nwant %s", got, want.TransactionHash)
			}
		})
	}
}

func everySettledOperation() []SettledAuthorityOperation {
	account := repeatAddr(0x02)
	ops := []SettledAuthorityOperation{
		SettledPause{}, SettledUnpause{}, SettledRevokeMasterMintBurn{},
		SettledRevokeMinter{Minter: account},
		SettledRotateMasterMintBurn{Account: account},
		SettledSetMinterAllowance{Minter: account, Allowance: oneToken()},
		SettledMint{Recipient: account, Value: oneToken()},
		SettledUpdateMetadata{Name: "Token", URI: "https://example.com/token.json"},
	}
	for _, role := range []SettledRole{SettledRolePause, SettledRoleManageList, SettledRoleUpdateMetadata, SettledRoleBridge, SettledRoleClawback} {
		ops = append(ops, SettledGrantRole{Role: role, Account: account}, SettledRevokeRole{Role: role, Account: account})
	}
	for _, list := range []SettledListKind{SettledListBlacklist, SettledListWhitelist} {
		for _, action := range []SettledListAction{SettledListAdd, SettledListRemove} {
			ops = append(ops, SettledList{List: list, Action: action, Account: account})
		}
	}
	return ops
}

func TestNoTwoSettledOperationsShareASigningHash(t *testing.T) {
	seen := map[string]SettledAuthorityOperation{}
	for _, operation := range everySettledOperation() {
		prepared, err := PrepareTransaction(settledVectorPayload(operation))
		if err != nil {
			t.Fatalf("%#v: %v", operation, err)
		}
		hash := hexLower(prepared.SigningHash())
		if earlier, dup := seen[hash]; dup {
			t.Fatalf("%#v and %#v share signing hash %s", earlier, operation, hash)
		}
		seen[hash] = operation
	}
	if len(seen) != 22 {
		t.Fatalf("covered %d operations, want 22", len(seen))
	}
}

func authorizedBody(t *testing.T, payload any, opts ...SubmitOption) (*AuthorizedTransaction, map[string]any) {
	t.Helper()
	prepared, err := PrepareTransaction(payload, opts...)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	sig, err := testSigner(t).SignHash(prepared.SigningHash())
	if err != nil {
		t.Fatal(err)
	}
	authorized, err := prepared.Authorize(sig)
	if err != nil {
		t.Fatalf("authorize: %v", err)
	}
	raw, err := json.Marshal(authorized.body)
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	return authorized, body
}

func TestSettledAuthorityWireBody(t *testing.T) {
	account := common.HexToAddress("0x9E1E9688A44D058fF181Ed64ddFAFbBE5CC742Ab")
	cases := []struct {
		operation SettledAuthorityOperation
		want      string
	}{
		{SettledPause{}, `{"type":"pause"}`},
		{SettledUnpause{}, `{"type":"unpause"}`},
		{SettledRevokeMasterMintBurn{}, `{"type":"revoke_master_mint_burn"}`},
		{SettledList{List: SettledListBlacklist, Action: SettledListAdd, Account: account}, `{"type":"list","list":"blacklist","action":"add","account":"0x9e1e9688a44d058ff181ed64ddfafbbe5cc742ab"}`},
		{SettledUpdateMetadata{Name: "N", URI: "u", AdditionalMetadata: []AdditionalMetadata{{Key: "k", Value: "v"}}}, `{"type":"update_metadata","name":"N","uri":"u","additional_metadata":[{"key":"k","value":"v"}]}`},
		{SettledUpdateMetadata{Name: "N", URI: "u"}, `{"type":"update_metadata","name":"N","uri":"u","additional_metadata":[]}`},
		{SettledGrantRole{Role: SettledRoleManageList, Account: account}, `{"type":"grant_role","role":"manage_list","account":"0x9e1e9688a44d058ff181ed64ddfafbbe5cc742ab"}`},
		{SettledRevokeRole{Role: SettledRoleClawback, Account: account}, `{"type":"revoke_role","role":"clawback","account":"0x9e1e9688a44d058ff181ed64ddfafbbe5cc742ab"}`},
		{SettledSetMinterAllowance{Minter: account, Allowance: big.NewInt(500)}, `{"type":"set_minter_allowance","minter":"0x9e1e9688a44d058ff181ed64ddfafbbe5cc742ab","allowance":"500"}`},
		{SettledRevokeMinter{Minter: account}, `{"type":"revoke_minter","minter":"0x9e1e9688a44d058ff181ed64ddfafbbe5cc742ab"}`},
		{SettledRotateMasterMintBurn{Account: account}, `{"type":"rotate_master_mint_burn","account":"0x9e1e9688a44d058ff181ed64ddfafbbe5cc742ab"}`},
		{SettledMint{Recipient: account, Value: big.NewInt(7)}, `{"type":"mint","recipient":"0x9e1e9688a44d058ff181ed64ddfafbbe5cc742ab","value":"7"}`},
	}
	for _, tc := range cases {
		authorized, body := authorizedBody(t, settledVectorPayload(tc.operation))
		if authorized.path != "/v2/tokens/authority_operation" {
			t.Fatalf("path = %s", authorized.path)
		}
		var want map[string]any
		if err := json.Unmarshal([]byte(tc.want), &want); err != nil {
			t.Fatal(err)
		}
		got, _ := json.Marshal(lowerAddresses(body["operation"]))
		wantJSON, _ := json.Marshal(want)
		if string(got) != string(wantJSON) {
			t.Errorf("operation\n got %s\nwant %s", got, wantJSON)
		}
		for _, key := range []string{"chain_id", "nonce", "token", "reference_checkpoint", "memo", "authorization"} {
			if _, ok := body[key]; !ok {
				t.Errorf("%T: body has no %s", tc.operation, key)
			}
		}
		if body["reference_checkpoint"] != float64(42) || body["nonce"] != float64(1) {
			t.Errorf("header = %v / %v", body["reference_checkpoint"], body["nonce"])
		}
	}
}

func lowerAddresses(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, val := range x {
			out[k] = lowerAddresses(val)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			out[i] = lowerAddresses(val)
		}
		return out
	case string:
		if strings.HasPrefix(x, "0x") && len(x) == 42 {
			return strings.ToLower(x)
		}
		return x
	}
	return v
}

func TestLaneClawbackWireBody(t *testing.T) {
	authorized, body := authorizedBody(t, laneVectorPayload())
	if authorized.path != "/v2/tokens/lane_clawback" {
		t.Fatalf("path = %s", authorized.path)
	}
	want := map[string]any{
		"chain_id": float64(fixtureChainID), "from_nonce": float64(7), "reference_checkpoint": float64(42),
		"token": "0x0101010101010101010101010101010101010101", "holder": "0x0202020202020202020202020202020202020202",
		"recipient": "0x0303030303030303030303030303030303030303", "value": "1000000000000000000",
	}
	for key, value := range want {
		if got := lowerAddresses(body[key]); got != value {
			t.Errorf("%s = %v, want %v", key, got, value)
		}
	}
	if _, ok := body["nonce"]; ok {
		t.Error("a lane clawback carries no signer nonce")
	}
}

func TestLaneClawbackOfVerifyIsRefusedBeforeSigning(t *testing.T) {
	payload := laneVectorPayload()
	payload.Token = VerifyTokenAddress
	if _, err := PrepareTransaction(payload); err == nil || !strings.Contains(err.Error(), "VERIFY") {
		t.Fatalf("err = %v, want a VERIFY refusal", err)
	}
	if !IsVerifyToken(common.HexToAddress("0x0000000000000000000000000000000000000002")) {
		t.Fatal("0x…02 is the VERIFY token")
	}
}

func TestSettledAndLaneClawbackAreV2Only(t *testing.T) {
	client := NewClientWithCustomUrl("http://127.0.0.1:1", WithLegacyV1())
	err := client.submitPayload(t.Context(), laneVectorPayload(), submitConfig{}, testSigner(t), new(TransactionResponse))
	if err == nil || !strings.Contains(err.Error(), "lane clawback requires domain-separated v2") {
		t.Fatalf("lane clawback err = %v", err)
	}
	err = client.submitPayload(t.Context(), settledVectorPayload(SettledPause{}), submitConfig{}, testSigner(t), new(TransactionResponse))
	if err == nil || !strings.Contains(err.Error(), "settled authority operation requires domain-separated v2") {
		t.Fatalf("settled err = %v", err)
	}
}

func TestSettledPayloadValidation(t *testing.T) {
	bad := []struct {
		payload any
		want    string
	}{
		{settledVectorPayload(nil), "operation"},
		{settledVectorPayload(SettledMint{Recipient: repeatAddr(2), Value: big.NewInt(-1)}), "non-negative"},
		{settledVectorPayload(SettledSetMinterAllowance{Minter: repeatAddr(2), Allowance: new(big.Int).Lsh(big.NewInt(1), 256)}), "U256"},
		{settledVectorPayload(SettledGrantRole{Role: "Owner", Account: repeatAddr(2)}), "role"},
		{settledVectorPayload(SettledList{List: "Greylist", Action: SettledListAdd, Account: repeatAddr(2)}), "list"},
		{settledVectorPayload(SettledList{List: SettledListBlacklist, Action: "Toggle", Account: repeatAddr(2)}), "action"},
		{LaneClawbackPayload{Token: repeatAddr(1), Value: big.NewInt(-5)}, "non-negative"},
	}
	for _, tc := range bad {
		if _, err := PrepareTransaction(tc.payload); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%#v: err = %v, want it to name %q", tc.payload, err, tc.want)
		}
	}
}
