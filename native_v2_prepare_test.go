package onemoney

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

type prepareAuthorizeFixture struct {
	Meta prepareAuthorizeFixtureMeta `json:"_fixture"`
	// Source must stay absent. It is the retired external-provenance block; a
	// non-empty value means someone reintroduced a dependency on another
	// repository's history, which the vendored-fixture model forbids.
	Source  map[string]any           `json:"_source"`
	Vectors []prepareAuthorizeVector `json:"vectors"`
}

// prepareAuthorizeFixtureMeta is the fixture's self-description. The fixture is
// owned by this SDK: it records no external repository identity, source path, or
// generator, and the suite never needs another checkout to run.
type prepareAuthorizeFixtureMeta struct {
	Owner            string `json:"owner"`
	Status           string `json:"status"`
	ProtocolContract string `json:"protocol_contract"`
	Note             string `json:"note"`
}

type prepareAuthorizeVector struct {
	Name          string                   `json:"name"`
	Class         string                   `json:"class"`
	Operation     string                   `json:"operation"`
	OperationType uint16                   `json:"operation_type"`
	Payload       json.RawMessage          `json:"payload"`
	Options       prepareAuthorizeOptions  `json:"options"`
	Authorization Signature                `json:"authorization"`
	Expected      prepareAuthorizeExpected `json:"expected"`
}

type prepareAuthorizeOptions struct {
	Memo           *Memo  `json:"memo"`
	ManageListKind string `json:"manage_list_kind"`
}

type prepareAuthorizeExpected struct {
	SigningHash     string `json:"signing_hash"`
	TransactionHash string `json:"transaction_hash"`
	OperationsHash  string `json:"operations_hash"`
}

func loadPrepareAuthorizeFixture(t *testing.T) prepareAuthorizeFixture {
	t.Helper()
	data, err := os.ReadFile("testdata/prepare-authorize-hash-vectors.json")
	if err != nil {
		t.Fatalf("read prepare/authorize fixture: %v", err)
	}
	if bytes.Contains(data, []byte("payload_rlp")) || bytes.Contains(data, []byte("transaction_rlp")) {
		t.Fatal("prepare/authorize fixture must contain original fields, not encoded payloads")
	}
	var fixture prepareAuthorizeFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("decode prepare/authorize fixture: %v", err)
	}
	return fixture
}

func parseFixtureBig(t *testing.T, value string) *big.Int {
	t.Helper()
	parsed, ok := new(big.Int).SetString(value, 10)
	if !ok || parsed.Sign() < 0 || parsed.BitLen() > 256 {
		t.Fatalf("invalid fixture U256 %q", value)
	}
	return parsed
}

func parseFixtureHex(t *testing.T, value string, size int) []byte {
	t.Helper()
	if !strings.HasPrefix(value, "0x") {
		t.Fatalf("fixture hex value %q has no 0x prefix", value)
	}
	decoded, err := hex.DecodeString(value[2:])
	if err != nil {
		t.Fatalf("decode fixture hex value %q: %v", value, err)
	}
	if size >= 0 && len(decoded) != size {
		t.Fatalf("fixture hex value %q has %d bytes, want %d", value, len(decoded), size)
	}
	return decoded
}

func parseFixtureAddress(t *testing.T, value string) common.Address {
	t.Helper()
	return common.BytesToAddress(parseFixtureHex(t, value, common.AddressLength))
}

func parseFixtureHashPtr(t *testing.T, value *string) *common.Hash {
	t.Helper()
	if value == nil {
		return nil
	}
	hash := common.BytesToHash(parseFixtureHex(t, *value, common.HashLength))
	return &hash
}

func parseFixtureHexBytes(t *testing.T, value string) HexBytes {
	t.Helper()
	return HexBytes(parseFixtureHex(t, value, -1))
}

func decodeFixturePayload[T any](t *testing.T, raw json.RawMessage) T {
	t.Helper()
	var payload T
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decode fixture payload: %v", err)
	}
	return payload
}

type transferFixturePayload struct {
	ChainID   uint64 `json:"chain_id"`
	Nonce     uint64 `json:"nonce"`
	Recipient string `json:"recipient"`
	Value     string `json:"value"`
	Token     string `json:"token"`
}

type issueFixturePayload struct {
	ChainID         uint64 `json:"chain_id"`
	Nonce           uint64 `json:"nonce"`
	Symbol          string `json:"symbol"`
	Name            string `json:"name"`
	Decimals        uint8  `json:"decimals"`
	MasterAuthority string `json:"master_authority"`
	IsPrivate       bool   `json:"is_private"`
	ClawbackEnabled bool   `json:"clawback_enabled"`
}

type authorityFixturePayload struct {
	ChainID          uint64 `json:"chain_id"`
	Nonce            uint64 `json:"nonce"`
	Action           string `json:"action"`
	AuthorityType    string `json:"authority_type"`
	AuthorityAddress string `json:"authority_address"`
	Token            string `json:"token"`
	Value            string `json:"value"`
}

type manageListFixturePayload struct {
	ChainID uint64 `json:"chain_id"`
	Nonce   uint64 `json:"nonce"`
	Action  string `json:"action"`
	Address string `json:"address"`
	Token   string `json:"token"`
}

type pauseFixturePayload struct {
	ChainID uint64 `json:"chain_id"`
	Nonce   uint64 `json:"nonce"`
	Action  string `json:"action"`
	Token   string `json:"token"`
}

type burnFixturePayload struct {
	ChainID uint64 `json:"chain_id"`
	Nonce   uint64 `json:"nonce"`
	Value   string `json:"value"`
	Token   string `json:"token"`
}

type clawbackFixturePayload struct {
	ChainID   uint64 `json:"chain_id"`
	Nonce     uint64 `json:"nonce"`
	Token     string `json:"token"`
	From      string `json:"from"`
	Recipient string `json:"recipient"`
	Value     string `json:"value"`
}

type metadataFixturePayload struct {
	ChainID            uint64               `json:"chain_id"`
	Nonce              uint64               `json:"nonce"`
	Name               string               `json:"name"`
	URI                string               `json:"uri"`
	Token              string               `json:"token"`
	AdditionalMetadata []AdditionalMetadata `json:"additional_metadata"`
}

type bridgeAndMintFixturePayload struct {
	ChainID        uint64 `json:"chain_id"`
	Nonce          uint64 `json:"nonce"`
	Recipient      string `json:"recipient"`
	Value          string `json:"value"`
	Token          string `json:"token"`
	SourceChainID  uint64 `json:"source_chain_id"`
	SourceTxHash   string `json:"source_tx_hash"`
	BridgeMetadata string `json:"bridge_metadata"`
}

type burnAndBridgeFixturePayload struct {
	ChainID            uint64 `json:"chain_id"`
	Nonce              uint64 `json:"nonce"`
	Sender             string `json:"sender"`
	Value              string `json:"value"`
	Token              string `json:"token"`
	DestinationChainID uint64 `json:"destination_chain_id"`
	DestinationAddress string `json:"destination_address"`
	EscrowFee          string `json:"escrow_fee"`
	BridgeMetadata     string `json:"bridge_metadata"`
	BridgeParam        string `json:"bridge_param"`
}

type multisigFixtureSigner struct {
	PublicKey string `json:"public_key"`
	Weight    uint8  `json:"weight"`
}

type createMultisigFixturePayload struct {
	ChainID   uint64                  `json:"chain_id"`
	Nonce     uint64                  `json:"nonce"`
	Signers   []multisigFixtureSigner `json:"signers"`
	Threshold uint16                  `json:"threshold"`
}

type batchFixtureOperation struct {
	Recipient string `json:"recipient"`
	Amount    string `json:"amount"`
}

type batchFixturePayload struct {
	ChainID        uint64                  `json:"chain_id"`
	Nonce          uint64                  `json:"nonce"`
	Token          string                  `json:"token"`
	Operations     []batchFixtureOperation `json:"operations"`
	CreatedAt      uint64                  `json:"created_at"`
	OperationsHash *string                 `json:"operations_hash"`
	BatchID        *string                 `json:"batch_id"`
}

func (v prepareAuthorizeVector) options(t *testing.T) []SubmitOption {
	t.Helper()
	var options []SubmitOption
	switch v.Options.ManageListKind {
	case "":
	case "blacklist":
		options = append(options, WithManageListKind(ManageListBlacklist))
	case "whitelist":
		options = append(options, WithManageListKind(ManageListWhitelist))
	default:
		t.Fatalf("%s: invalid manage_list_kind %q", v.Name, v.Options.ManageListKind)
	}
	if v.Options.Memo != nil {
		options = append(options, WithMemo(*v.Options.Memo))
	}
	return options
}

func (v prepareAuthorizeVector) goPayload(t *testing.T) (any, []SubmitOption) {
	t.Helper()
	options := v.options(t)
	switch v.Operation {
	case "Payment":
		raw := decodeFixturePayload[transferFixturePayload](t, v.Payload)
		return PaymentPayload{
			ChainID: raw.ChainID, Nonce: raw.Nonce, Recipient: parseFixtureAddress(t, raw.Recipient),
			Value: parseFixtureBig(t, raw.Value), Token: parseFixtureAddress(t, raw.Token),
		}, options
	case "TokenIssue":
		raw := decodeFixturePayload[issueFixturePayload](t, v.Payload)
		return TokenIssuePayload{
			ChainID: raw.ChainID, Nonce: raw.Nonce, Symbol: raw.Symbol, Name: raw.Name,
			Decimals: raw.Decimals, MasterAuthority: parseFixtureAddress(t, raw.MasterAuthority),
			IsPrivate: raw.IsPrivate, ClawbackEnabled: raw.ClawbackEnabled,
		}, options
	case "TokenMint":
		raw := decodeFixturePayload[transferFixturePayload](t, v.Payload)
		return TokenMintPayload{
			ChainID: raw.ChainID, Nonce: raw.Nonce, Recipient: parseFixtureAddress(t, raw.Recipient),
			Value: parseFixtureBig(t, raw.Value), Token: parseFixtureAddress(t, raw.Token),
		}, options
	case "TokenAuthority":
		raw := decodeFixturePayload[authorityFixturePayload](t, v.Payload)
		return TokenAuthorityPayload{
			ChainID: raw.ChainID, Nonce: raw.Nonce, Action: AuthorityAction(raw.Action),
			AuthorityType:    AuthorityType(raw.AuthorityType),
			AuthorityAddress: parseFixtureAddress(t, raw.AuthorityAddress),
			Token:            parseFixtureAddress(t, raw.Token), Value: parseFixtureBig(t, raw.Value),
		}, options
	case "TokenBlacklist", "TokenWhitelist":
		raw := decodeFixturePayload[manageListFixturePayload](t, v.Payload)
		return TokenManageListPayload{
			ChainID: raw.ChainID, Nonce: raw.Nonce, Action: ManageListActionType(raw.Action),
			Address: parseFixtureAddress(t, raw.Address), Token: parseFixtureAddress(t, raw.Token),
		}, options
	case "TokenPause":
		raw := decodeFixturePayload[pauseFixturePayload](t, v.Payload)
		return PauseTokenPayload{
			ChainID: raw.ChainID, Nonce: raw.Nonce, Action: PauseActionType(raw.Action),
			Token: parseFixtureAddress(t, raw.Token),
		}, options
	case "TokenBurn":
		raw := decodeFixturePayload[burnFixturePayload](t, v.Payload)
		return TokenBurnPayload{
			ChainID: raw.ChainID, Nonce: raw.Nonce, Value: parseFixtureBig(t, raw.Value),
			Token: parseFixtureAddress(t, raw.Token),
		}, options
	case "TokenClawback":
		raw := decodeFixturePayload[clawbackFixturePayload](t, v.Payload)
		return TokenClawbackPayload{
			ChainID: raw.ChainID, Nonce: raw.Nonce, Token: parseFixtureAddress(t, raw.Token),
			From: parseFixtureAddress(t, raw.From), Recipient: parseFixtureAddress(t, raw.Recipient),
			Value: parseFixtureBig(t, raw.Value),
		}, options
	case "TokenMetadata":
		raw := decodeFixturePayload[metadataFixturePayload](t, v.Payload)
		return UpdateMetadataPayload{
			ChainID: raw.ChainID, Nonce: raw.Nonce, Name: raw.Name, URI: raw.URI,
			Token: parseFixtureAddress(t, raw.Token), AdditionalMetadata: raw.AdditionalMetadata,
		}, options
	case "TokenBridgeAndMint":
		raw := decodeFixturePayload[bridgeAndMintFixturePayload](t, v.Payload)
		return TokenBridgeAndMintPayload{
			ChainID: raw.ChainID, Nonce: raw.Nonce, Recipient: parseFixtureAddress(t, raw.Recipient),
			Value: parseFixtureBig(t, raw.Value), Token: parseFixtureAddress(t, raw.Token),
			SourceChainID: raw.SourceChainID, SourceTxHash: raw.SourceTxHash,
			BridgeMetadata: raw.BridgeMetadata,
		}, options
	case "TokenBurnAndBridge":
		raw := decodeFixturePayload[burnAndBridgeFixturePayload](t, v.Payload)
		return TokenBurnAndBridgePayload{
			ChainID: raw.ChainID, Nonce: raw.Nonce, Sender: parseFixtureAddress(t, raw.Sender),
			Value: parseFixtureBig(t, raw.Value), Token: parseFixtureAddress(t, raw.Token),
			DestinationChainID: raw.DestinationChainID, DestinationAddress: raw.DestinationAddress,
			EscrowFee: parseFixtureBig(t, raw.EscrowFee), BridgeMetadata: raw.BridgeMetadata,
			BridgeParam: parseFixtureHexBytes(t, raw.BridgeParam),
		}, options
	case "CreateMultiSig":
		raw := decodeFixturePayload[createMultisigFixturePayload](t, v.Payload)
		signers := make([]MultiSigSigner, 0, len(raw.Signers))
		for _, signer := range raw.Signers {
			signers = append(signers, MultiSigSigner{
				PublicKey: parseFixtureHexBytes(t, signer.PublicKey),
				Weight:    signer.Weight,
			})
		}
		return CreateMultiSigPayload{
			ChainID: raw.ChainID, Nonce: raw.Nonce, Signers: signers, Threshold: raw.Threshold,
		}, options
	case "BatchPayment":
		raw := decodeFixturePayload[batchFixturePayload](t, v.Payload)
		operations := make([]PaymentOperation, 0, len(raw.Operations))
		for _, operation := range raw.Operations {
			operations = append(operations, PaymentOperation{
				Recipient: parseFixtureAddress(t, operation.Recipient),
				Amount:    parseFixtureBig(t, operation.Amount),
			})
		}
		return BatchPaymentPayload{
			ChainID: raw.ChainID, Nonce: raw.Nonce, Token: parseFixtureAddress(t, raw.Token),
			Operations: operations, CreatedAt: raw.CreatedAt,
			OperationsHash: parseFixtureHashPtr(t, raw.OperationsHash), BatchID: raw.BatchID,
		}, options
	default:
		t.Fatalf("%s: unsupported operation %q", v.Name, v.Operation)
		return nil, nil
	}
}

func TestPrepareAuthorizeFixtureCompleteness(t *testing.T) {
	fixture := loadPrepareAuthorizeFixture(t)
	if len(fixture.Vectors) <= 14 {
		t.Fatalf("got %d vectors, want canonical and edge vectors", len(fixture.Vectors))
	}
	// The fixture is a self-contained SDK test input. Running the suite must
	// never require another repository, so no external provenance may be
	// recorded, and the fixture must say what protocol contract it encodes.
	if len(fixture.Source) != 0 {
		t.Fatalf("fixture records external provenance %+v; vendored fixtures must be self-contained (see testdata/README.md)", fixture.Source)
	}
	if fixture.Meta.Owner != "1money-protocol-go-sdk" {
		t.Fatalf("fixture owner = %q, want this SDK", fixture.Meta.Owner)
	}
	if fixture.Meta.ProtocolContract == "" || fixture.Meta.Note == "" {
		t.Fatalf("fixture must declare its protocol contract and the never-recompute rule: %+v", fixture.Meta)
	}

	names := make(map[string]struct{}, len(fixture.Vectors))
	var canonical []uint16
	for _, vector := range fixture.Vectors {
		if _, exists := names[vector.Name]; exists {
			t.Fatalf("duplicate vector name %q", vector.Name)
		}
		names[vector.Name] = struct{}{}
		if vector.Operation == "" || len(vector.Payload) == 0 {
			t.Fatalf("%s: incomplete operation/payload", vector.Name)
		}
		if len(parseFixtureHex(t, vector.Authorization.R, 32)) != 32 ||
			len(parseFixtureHex(t, vector.Authorization.S, 32)) != 32 {
			t.Fatalf("%s: incomplete authorization", vector.Name)
		}
		parseFixtureHex(t, vector.Expected.SigningHash, 32)
		parseFixtureHex(t, vector.Expected.TransactionHash, 32)
		if vector.Class == "canonical" {
			canonical = append(canonical, vector.OperationType)
		}
	}
	if len(canonical) != 14 {
		t.Fatalf("got %d canonical vectors, want 14", len(canonical))
	}
	for index, operationType := range canonical {
		if want := uint16(index + 1); operationType != want {
			t.Fatalf("canonical operation[%d] = %d, want %d", index, operationType, want)
		}
	}
}

type fixtureStringSet map[string]struct{}

func (s fixtureStringSet) add(value string) {
	s[value] = struct{}{}
}

func sortedFixtureSet(s fixtureStringSet) []string {
	values := make([]string, 0, len(s))
	for value := range s {
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}

func assertFixtureSetEqual(t *testing.T, label string, got fixtureStringSet, want ...string) {
	t.Helper()
	expected := make(fixtureStringSet, len(want))
	for _, value := range want {
		expected.add(value)
	}
	missing := make(fixtureStringSet)
	unexpected := make(fixtureStringSet)
	for value := range expected {
		if _, ok := got[value]; !ok {
			missing.add(value)
		}
	}
	for value := range got {
		if _, ok := expected[value]; !ok {
			unexpected.add(value)
		}
	}
	if len(missing) != 0 || len(unexpected) != 0 {
		t.Errorf("%s coverage mismatch: missing=%v unexpected=%v", label, sortedFixtureSet(missing), sortedFixtureSet(unexpected))
	}
}

func assertFixtureSetContains(t *testing.T, label string, got fixtureStringSet, want ...string) {
	t.Helper()
	var missing []string
	for _, value := range want {
		if _, ok := got[value]; !ok {
			missing = append(missing, value)
		}
	}
	if len(missing) != 0 {
		t.Errorf("%s coverage missing %v; got %v", label, missing, sortedFixtureSet(got))
	}
}

func hasNonASCII(value string) bool {
	for _, r := range value {
		if r > 0x7f {
			return true
		}
	}
	return false
}

func TestPrepareAuthorizeFixtureSemanticCoverage(t *testing.T) {
	fixture := loadPrepareAuthorizeFixture(t)

	t.Run("finite enums and booleans", func(t *testing.T) {
		authority := make(fixtureStringSet)
		issue := make(fixtureStringSet)
		actions := map[string]fixtureStringSet{
			"TokenBlacklist": make(fixtureStringSet),
			"TokenWhitelist": make(fixtureStringSet),
			"TokenPause":     make(fixtureStringSet),
		}
		decimals := make(fixtureStringSet)
		for _, vector := range fixture.Vectors {
			switch vector.Operation {
			case "TokenAuthority":
				raw := decodeFixturePayload[authorityFixturePayload](t, vector.Payload)
				authority.add(raw.Action + "|" + raw.AuthorityType)
			case "TokenIssue":
				raw := decodeFixturePayload[issueFixturePayload](t, vector.Payload)
				issue.add(fmt.Sprintf("%t|%t", raw.IsPrivate, raw.ClawbackEnabled))
				decimals.add(fmt.Sprint(raw.Decimals))
			case "TokenBlacklist", "TokenWhitelist":
				raw := decodeFixturePayload[manageListFixturePayload](t, vector.Payload)
				actions[vector.Operation].add(raw.Action)
			case "TokenPause":
				raw := decodeFixturePayload[pauseFixturePayload](t, vector.Payload)
				actions[vector.Operation].add(raw.Action)
			}
		}

		var authorityWant []string
		for _, action := range []string{"Grant", "Revoke"} {
			for _, authorityType := range []string{
				"MasterMintBurn", "MintBurnTokens", "Pause", "ManageList",
				"UpdateMetadata", "Bridge", "Clawback",
			} {
				authorityWant = append(authorityWant, action+"|"+authorityType)
			}
		}
		assertFixtureSetEqual(t, "TokenAuthority action/type", authority, authorityWant...)
		assertFixtureSetEqual(t, "TokenIssue booleans", issue, "false|false", "false|true", "true|false", "true|true")
		assertFixtureSetEqual(t, "TokenBlacklist action", actions["TokenBlacklist"], "Add", "Remove")
		assertFixtureSetEqual(t, "TokenWhitelist action", actions["TokenWhitelist"], "Add", "Remove")
		assertFixtureSetEqual(t, "TokenPause action", actions["TokenPause"], "Pause", "Unpause")
		assertFixtureSetContains(t, "TokenIssue decimals", decimals, "0", "255")
	})

	t.Run("numeric boundaries", func(t *testing.T) {
		values := make(map[string]fixtureStringSet)
		record := func(field, value string) {
			if values[field] == nil {
				values[field] = make(fixtureStringSet)
			}
			values[field].add(value)
		}
		for _, vector := range fixture.Vectors {
			switch vector.Operation {
			case "Payment", "TokenMint":
				raw := decodeFixturePayload[transferFixturePayload](t, vector.Payload)
				record(vector.Operation+".value", raw.Value)
				if vector.Operation == "Payment" {
					record("Payment.chain_id", fmt.Sprint(raw.ChainID))
					record("Payment.nonce", fmt.Sprint(raw.Nonce))
				}
			case "TokenAuthority":
				record("TokenAuthority.value", decodeFixturePayload[authorityFixturePayload](t, vector.Payload).Value)
			case "TokenBurn":
				record("TokenBurn.value", decodeFixturePayload[burnFixturePayload](t, vector.Payload).Value)
			case "TokenClawback":
				record("TokenClawback.value", decodeFixturePayload[clawbackFixturePayload](t, vector.Payload).Value)
			case "TokenBridgeAndMint":
				raw := decodeFixturePayload[bridgeAndMintFixturePayload](t, vector.Payload)
				record("TokenBridgeAndMint.value", raw.Value)
				record("TokenBridgeAndMint.source_chain_id", fmt.Sprint(raw.SourceChainID))
			case "TokenBurnAndBridge":
				raw := decodeFixturePayload[burnAndBridgeFixturePayload](t, vector.Payload)
				record("TokenBurnAndBridge.value", raw.Value)
				record("TokenBurnAndBridge.escrow_fee", raw.EscrowFee)
				record("TokenBurnAndBridge.destination_chain_id", fmt.Sprint(raw.DestinationChainID))
			case "BatchPayment":
				raw := decodeFixturePayload[batchFixturePayload](t, vector.Payload)
				record("BatchPayment.created_at", fmt.Sprint(raw.CreatedAt))
				for _, operation := range raw.Operations {
					record("BatchPayment.operations.amount", operation.Amount)
				}
			}
		}

		maxU256 := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1)).String()
		for _, field := range []string{
			"Payment.value", "BatchPayment.operations.amount",
			"TokenMint.value", "TokenAuthority.value", "TokenBurn.value", "TokenClawback.value",
			"TokenBridgeAndMint.value", "TokenBurnAndBridge.value", "TokenBurnAndBridge.escrow_fee",
		} {
			assertFixtureSetContains(t, field, values[field], "0", maxU256)
		}
		maxU64 := fmt.Sprint(uint64(^uint64(0)))
		for _, field := range []string{
			"Payment.chain_id", "Payment.nonce", "BatchPayment.created_at",
			"TokenBridgeAndMint.source_chain_id", "TokenBurnAndBridge.destination_chain_id",
		} {
			assertFixtureSetContains(t, field, values[field], "0", maxU64)
		}
	})

	t.Run("string and byte boundaries", func(t *testing.T) {
		lengths := make(map[string]fixtureStringSet)
		utf8Fields := make(fixtureStringSet)
		recordString := func(field, value string) {
			if lengths[field] == nil {
				lengths[field] = make(fixtureStringSet)
			}
			lengths[field].add(fmt.Sprint(len(value)))
			if hasNonASCII(value) {
				utf8Fields.add(field)
			}
		}
		for _, vector := range fixture.Vectors {
			switch vector.Operation {
			case "TokenIssue":
				raw := decodeFixturePayload[issueFixturePayload](t, vector.Payload)
				recordString("TokenIssue.symbol", raw.Symbol)
				recordString("TokenIssue.name", raw.Name)
			case "TokenMetadata":
				raw := decodeFixturePayload[metadataFixturePayload](t, vector.Payload)
				recordString("TokenMetadata.name", raw.Name)
				recordString("TokenMetadata.uri", raw.URI)
				for _, metadata := range raw.AdditionalMetadata {
					recordString("TokenMetadata.key", metadata.Key)
					recordString("TokenMetadata.value", metadata.Value)
				}
			case "TokenBridgeAndMint":
				raw := decodeFixturePayload[bridgeAndMintFixturePayload](t, vector.Payload)
				recordString("TokenBridgeAndMint.source_tx_hash", raw.SourceTxHash)
				recordString("TokenBridgeAndMint.bridge_metadata", raw.BridgeMetadata)
			case "TokenBurnAndBridge":
				raw := decodeFixturePayload[burnAndBridgeFixturePayload](t, vector.Payload)
				recordString("TokenBurnAndBridge.destination_address", raw.DestinationAddress)
				recordString("TokenBurnAndBridge.bridge_metadata", raw.BridgeMetadata)
				if lengths["TokenBurnAndBridge.bridge_param"] == nil {
					lengths["TokenBurnAndBridge.bridge_param"] = make(fixtureStringSet)
				}
				lengths["TokenBurnAndBridge.bridge_param"].add(fmt.Sprint(len(parseFixtureHex(t, raw.BridgeParam, -1))))
			case "BatchPayment":
				raw := decodeFixturePayload[batchFixturePayload](t, vector.Payload)
				if raw.BatchID != nil {
					recordString("BatchPayment.batch_id", *raw.BatchID)
				}
			}
			if vector.Options.Memo != nil {
				recordString("memo.type", vector.Options.Memo.Type)
				recordString("memo.format", vector.Options.Memo.Format)
				recordString("memo.data", vector.Options.Memo.Data)
			}
		}

		for _, field := range []string{
			"TokenIssue.symbol", "TokenIssue.name", "TokenMetadata.name", "TokenMetadata.uri",
			"TokenMetadata.key", "TokenMetadata.value", "TokenBridgeAndMint.source_tx_hash",
			"TokenBridgeAndMint.bridge_metadata", "TokenBurnAndBridge.destination_address",
			"TokenBurnAndBridge.bridge_metadata", "TokenBurnAndBridge.bridge_param",
			"BatchPayment.batch_id", "memo.data",
		} {
			assertFixtureSetContains(t, field, lengths[field], "0", "1", "55", "56", "255", "256")
		}
		assertFixtureSetContains(t, "memo.type", lengths["memo.type"], "0", "1", "55", "56", "128")
		assertFixtureSetContains(t, "memo.format", lengths["memo.format"], "0", "1", "55", "56", "64")
		for _, field := range []string{
			"TokenIssue.symbol", "TokenIssue.name", "TokenMetadata.name", "TokenMetadata.uri",
			"TokenMetadata.key", "TokenMetadata.value", "TokenBridgeAndMint.source_tx_hash",
			"TokenBridgeAndMint.bridge_metadata", "TokenBurnAndBridge.destination_address",
			"TokenBurnAndBridge.bridge_metadata", "BatchPayment.batch_id", "memo.data",
		} {
			if _, ok := utf8Fields[field]; !ok {
				t.Errorf("%s lacks multibyte UTF-8 coverage", field)
			}
		}
	})

	t.Run("multisig boundaries", func(t *testing.T) {
		prefixes := make(fixtureStringSet)
		weights := make(fixtureStringSet)
		thresholds := make(fixtureStringSet)
		hasMax := false
		for _, vector := range fixture.Vectors {
			if vector.Operation != "CreateMultiSig" {
				continue
			}
			raw := decodeFixturePayload[createMultisigFixturePayload](t, vector.Payload)
			total := uint32(0)
			keys := make(fixtureStringSet)
			for _, signer := range raw.Signers {
				key := parseFixtureHex(t, signer.PublicKey, 33)
				prefixes.add(fmt.Sprintf("0x%02x", key[0]))
				weights.add(fmt.Sprint(signer.Weight))
				total += uint32(signer.Weight)
				keys.add(signer.PublicKey)
			}
			thresholds.add(fmt.Sprint(raw.Threshold))
			if raw.Threshold == ^uint16(0) {
				hasMax = len(raw.Signers) == 257 && len(keys) == 257 && total == uint32(^uint16(0))
			}
		}
		assertFixtureSetEqual(t, "multisig prefixes", prefixes, "0x02", "0x03")
		assertFixtureSetContains(t, "multisig weights", weights, "255")
		assertFixtureSetContains(t, "multisig thresholds", thresholds, "300", "65535")
		if !hasMax {
			t.Error("missing valid 257-signer threshold=65535 multisig vector")
		}
	})
}

func TestPrepareAuthorizeFixtureEdgeCoverage(t *testing.T) {
	fixture := loadPrepareAuthorizeFixture(t)
	byName := make(map[string]prepareAuthorizeVector, len(fixture.Vectors))
	parities := map[uint64]bool{}
	for _, vector := range fixture.Vectors {
		byName[vector.Name] = vector
		parities[vector.Authorization.V] = true
	}
	if !parities[0] || !parities[1] {
		t.Fatalf("fixture signature parities = %v, want both 0 and 1", parities)
	}

	distinctPairs := [][2]string{
		{"batch_operations_order_forward", "batch_operations_order_reverse"},
		{"TokenMetadata_canonical", "metadata_order_reverse"},
		{"CreateMultiSig_canonical", "multisig_signer_order_reverse"},
		{"BatchPayment_canonical", "batch_option_empty_id"},
		{"BatchPayment_canonical", "batch_option_zero_hash"},
		{"collision_payment", "collision_token_mint"},
		{"collision_blacklist", "collision_whitelist"},
		{"collision_pause", "collision_burn"},
	}
	for _, pair := range distinctPairs {
		left, leftOK := byName[pair[0]]
		right, rightOK := byName[pair[1]]
		if !leftOK || !rightOK {
			continue
		}
		if left.Expected.SigningHash == right.Expected.SigningHash {
			t.Errorf("%s and %s have the same signing hash", pair[0], pair[1])
		}
		if left.Expected.TransactionHash == right.Expected.TransactionHash {
			t.Errorf("%s and %s have the same transaction hash", pair[0], pair[1])
		}
	}
}

func TestPrepareAuthorizeFixtureDecodesPublicPayloads(t *testing.T) {
	for _, vector := range loadPrepareAuthorizeFixture(t).Vectors {
		vector := vector
		t.Run(vector.Name, func(t *testing.T) {
			payload, _ := vector.goPayload(t)
			if payload == nil {
				t.Fatal("decoded payload is nil")
			}
		})
	}
}

func TestPrepareAndAuthorizeMatchRustGoldenVectors(t *testing.T) {
	const rustTestPrivateKey = "01833a126ec45d0191519748146b9e35647aab7fed28de1c8e17824970f964a3"
	key, err := crypto.HexToECDSA(rustTestPrivateKey)
	if err != nil {
		t.Fatalf("parse Rust test key: %v", err)
	}
	wantSigner := crypto.PubkeyToAddress(key.PublicKey)

	for _, vector := range loadPrepareAuthorizeFixture(t).Vectors {
		vector := vector
		t.Run(vector.Name, func(t *testing.T) {
			payload, options := vector.goPayload(t)
			// Use the canonical encoder, not PrepareTransaction: the fixture
			// deliberately includes payloads the node rejects at admission (an
			// arbitrary operations_hash, an empty operation list, a zero amount)
			// precisely to pin their ENCODING. The admission gate is exercised
			// separately, in TestPrepareRejectsInadmissibleBatchPayment.
			prepared, err := prepareCanonical(payload, resolveSubmit(options))
			if err != nil {
				t.Fatalf("prepareCanonical: %v", err)
			}
			if got := hexLower(prepared.SigningHash()); got != vector.Expected.SigningHash {
				t.Fatalf("SigningHash\n got %s\nwant %s (Rust oracle)", got, vector.Expected.SigningHash)
			}

			authorized, err := prepared.Authorize(vector.Authorization)
			if err != nil {
				t.Fatalf("Authorize: %v", err)
			}
			if got := hexLower(authorized.TransactionHash()); got != vector.Expected.TransactionHash {
				t.Fatalf("TransactionHash\n got %s\nwant %s (Rust oracle)", got, vector.Expected.TransactionHash)
			}

			signature := append(
				parseFixtureHex(t, vector.Authorization.R, 32),
				parseFixtureHex(t, vector.Authorization.S, 32)...,
			)
			signature = append(signature, byte(vector.Authorization.V))
			publicKey, err := crypto.SigToPub(prepared.SigningHash(), signature)
			if err != nil {
				t.Fatalf("recover Rust fixture signer: %v", err)
			}
			if got := crypto.PubkeyToAddress(*publicKey); got != wantSigner {
				t.Fatalf("recovered signer = %s, want %s", got, wantSigner)
			}
		})
	}
}

func TestPrepareManageListDisambiguation(t *testing.T) {
	payload := TokenManageListPayload{
		ChainID: 1212101, Nonce: 5, Action: ManageListActionAdd,
		Address: repeatAddr(0x06), Token: repeatAddr(0x01),
	}
	// Ambiguous without a kind.
	if _, err := PrepareTransaction(payload); err == nil {
		t.Fatal("expected error for TokenManageListPayload without WithManageListKind")
	}
	bl, err := PrepareTransaction(payload, WithManageListKind(ManageListBlacklist))
	if err != nil {
		t.Fatal(err)
	}
	wl, err := PrepareTransaction(payload, WithManageListKind(ManageListWhitelist))
	if err != nil {
		t.Fatal(err)
	}
	// Same payload bytes, different operation -> different signing hash (#1038).
	if bytes.Equal(bl.SigningHash(), wl.SigningHash()) {
		t.Error("blacklist and whitelist must produce different signing hashes")
	}
	// An unknown kind must error, not silently sign as blacklist: the operation
	// type is part of the signing domain, so there is no safe default.
	if _, err := PrepareTransaction(payload, WithManageListKind(ManageListKind(2))); err == nil {
		t.Error("expected error for invalid ManageListKind, got nil")
	}
}

func TestPrepareUnsupportedPayload(t *testing.T) {
	if _, err := PrepareTransaction(struct{ X int }{X: 1}); err == nil {
		t.Fatal("expected error for unsupported payload type")
	}
}

// TestPreparedConsistentWithSubmitPath asserts the public prepare API derives
// the same operation/signing hash the internal submit path uses for a payment.
func TestPreparedConsistentWithSubmitPath(t *testing.T) {
	payload := testPaymentPayload()
	prep, err := PrepareTransaction(payload)
	if err != nil {
		t.Fatal(err)
	}
	op := paymentOp(payload) // built exactly as TransactionsAPI.Payment does
	payloadRLP, err := encodeWithMemo(op.payloadList, EmptyMemo())
	if err != nil {
		t.Fatal(err)
	}
	want, err := signingHashV2(op.op, singleDescriptor(), payloadRLP)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(prep.SigningHash(), want) {
		t.Errorf("prepare signing hash diverges from submit path")
	}
}

// TestAuthorizeRejectsNonParityV mirrors the node's strict parity rule: v must
// be 0 or 1; 2 / 27 / 28 / ... are rejected (never normalized).
func TestAuthorizeRejectsNonParityV(t *testing.T) {
	prep, err := PrepareTransaction(testPaymentPayload())
	if err != nil {
		t.Fatal(err)
	}
	sig, err := testSigner(t).SignHash(prep.SigningHash())
	if err != nil {
		t.Fatal(err)
	}
	for _, badV := range []uint64{2, 27, 28, 35, ^uint64(0)} {
		bad := sig
		bad.V = badV
		if _, err := prep.Authorize(bad); err == nil {
			t.Errorf("Authorize accepted invalid v=%d", badV)
		}
	}
	if _, err := prep.Authorize(sig); err != nil {
		t.Errorf("Authorize rejected a valid signature: %v", err)
	}
}

// TestAuthorizeRejectsHighS mirrors the node's canonical-low-S rule
// (CryptoError::HighSSignature): the built-in signer's low-S signature
// authorizes, but its high-S counterpart (s -> N-s) is rejected.
func TestAuthorizeRejectsHighS(t *testing.T) {
	prep, err := PrepareTransaction(testPaymentPayload())
	if err != nil {
		t.Fatal(err)
	}
	sig, err := testSigner(t).SignHash(prep.SigningHash())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := prep.Authorize(sig); err != nil {
		t.Fatalf("low-S signature rejected: %v", err)
	}
	// N - s is the high-S counterpart of a low-S s; the node rejects it, so
	// Authorize must too.
	s, ok := new(big.Int).SetString(sig.S[2:], 16)
	if !ok {
		t.Fatalf("could not parse sig.S %q", sig.S)
	}
	high := sig
	high.S = hexLower(new(big.Int).Sub(secp256k1N, s).Bytes())
	if _, err := prep.Authorize(high); err == nil {
		t.Error("Authorize accepted a high-S signature")
	}
}

func TestAuthorizeRejectsOutOfRangeScalars(t *testing.T) {
	prepared, err := PrepareTransaction(testPaymentPayload())
	if err != nil {
		t.Fatal(err)
	}
	valid, err := testSigner(t).SignHash(prepared.SigningHash())
	if err != nil {
		t.Fatal(err)
	}
	order := "0x" + secp256k1N.Text(16)
	for _, test := range []struct {
		name string
		sig  Signature
	}{
		{"zero_r", Signature{R: "0x0", S: valid.S, V: valid.V}},
		{"zero_s", Signature{R: valid.R, S: "0x0", V: valid.V}},
		{"r_equal_order", Signature{R: order, S: valid.S, V: valid.V}},
		{"s_equal_order", Signature{R: valid.R, S: order, V: valid.V}},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			if _, err := prepared.Authorize(test.sig); err == nil {
				t.Fatal("Authorize accepted an out-of-range signature scalar")
			}
		})
	}
}

// TestAuthorizeRejectsNonHexScalars locks in the Signature contract that r/s
// are 0x-prefixed hex: a decimal or otherwise non-0x-hex scalar must be
// rejected, not silently reinterpreted as hex (which could pass the range/low-S
// checks and submit a corrupted proof).
func TestAuthorizeRejectsNonHexScalars(t *testing.T) {
	prepared, err := PrepareTransaction(testPaymentPayload())
	if err != nil {
		t.Fatal(err)
	}
	valid, err := testSigner(t).SignHash(prepared.SigningHash())
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		sig  Signature
	}{
		{"r_missing_0x_prefix", Signature{R: "12345", S: valid.S, V: valid.V}},
		{"s_missing_0x_prefix", Signature{R: valid.R, S: "12345", V: valid.V}},
		{"r_invalid_hex", Signature{R: "0xZZ", S: valid.S, V: valid.V}},
		{"s_empty_hex", Signature{R: valid.R, S: "0x", V: valid.V}},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			if _, err := prepared.Authorize(test.sig); err == nil {
				t.Fatal("Authorize accepted a non-0x-hex signature scalar")
			}
		})
	}
}

func TestAuthorizedTransactionHashDependsOnSignatureAndReturnsCopy(t *testing.T) {
	prepared, err := PrepareTransaction(testPaymentPayload())
	if err != nil {
		t.Fatal(err)
	}
	signerA := testSigner(t)
	signerB, err := NewPrivateKeySigner("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	if err != nil {
		t.Fatal(err)
	}
	sigA, err := signerA.SignHash(prepared.SigningHash())
	if err != nil {
		t.Fatal(err)
	}
	sigB, err := signerB.SignHash(prepared.SigningHash())
	if err != nil {
		t.Fatal(err)
	}
	authorizedA, err := prepared.Authorize(sigA)
	if err != nil {
		t.Fatal(err)
	}
	authorizedB, err := prepared.Authorize(sigB)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(authorizedA.TransactionHash(), authorizedB.TransactionHash()) {
		t.Fatal("distinct valid signatures produced the same transaction hash")
	}

	first := authorizedA.TransactionHash()
	original := append([]byte(nil), first...)
	first[0] ^= 0xff
	if got := authorizedA.TransactionHash(); !bytes.Equal(got, original) {
		t.Fatal("TransactionHash returned mutable internal storage")
	}
}

func TestPrepareRejectsOutOfRangeU256(t *testing.T) {
	factories := []struct {
		name string
		make func(*big.Int) any
	}{
		{"payment.value", func(value *big.Int) any {
			return PaymentPayload{ChainID: 1, Nonce: 1, Recipient: repeatAddr(1), Value: value, Token: repeatAddr(2)}
		}},
		{"batch.operations[0].amount", func(value *big.Int) any {
			return BatchPaymentPayload{
				ChainID: 1, Nonce: 1, Token: repeatAddr(2),
				Operations: []PaymentOperation{{Recipient: repeatAddr(1), Amount: value}},
				CreatedAt:  1,
			}
		}},
		{"mint.value", func(value *big.Int) any {
			return TokenMintPayload{ChainID: 1, Nonce: 1, Recipient: repeatAddr(1), Value: value, Token: repeatAddr(2)}
		}},
		{"authority.value", func(value *big.Int) any {
			return TokenAuthorityPayload{
				ChainID: 1, Nonce: 1, Action: AuthorityActionGrant,
				AuthorityType: AuthorityTypeMintBurnTokens, AuthorityAddress: repeatAddr(1),
				Token: repeatAddr(2), Value: value,
			}
		}},
		{"burn.value", func(value *big.Int) any {
			return TokenBurnPayload{ChainID: 1, Nonce: 1, Value: value, Token: repeatAddr(2)}
		}},
		{"clawback.value", func(value *big.Int) any {
			return TokenClawbackPayload{
				ChainID: 1, Nonce: 1, Token: repeatAddr(2), From: repeatAddr(3),
				Recipient: repeatAddr(1), Value: value,
			}
		}},
		{"bridge_and_mint.value", func(value *big.Int) any {
			return TokenBridgeAndMintPayload{
				ChainID: 1, Nonce: 1, Recipient: repeatAddr(1), Value: value, Token: repeatAddr(2),
				SourceChainID: 2, SourceTxHash: "0x01",
			}
		}},
		{"burn_and_bridge.value", func(value *big.Int) any {
			return TokenBurnAndBridgePayload{
				ChainID: 1, Nonce: 1, Sender: repeatAddr(1), Value: value, Token: repeatAddr(2),
				DestinationChainID: 2, DestinationAddress: "destination", EscrowFee: big.NewInt(1),
			}
		}},
		{"burn_and_bridge.escrow_fee", func(value *big.Int) any {
			return TokenBurnAndBridgePayload{
				ChainID: 1, Nonce: 1, Sender: repeatAddr(1), Value: big.NewInt(1), Token: repeatAddr(2),
				DestinationChainID: 2, DestinationAddress: "destination", EscrowFee: value,
			}
		}},
	}

	overflow := new(big.Int).Lsh(big.NewInt(1), 256)
	for _, factory := range factories {
		factory := factory
		t.Run(factory.name+"/nil_equals_zero", func(t *testing.T) {
			// nil == U256 zero is an ENCODING equivalence, so assert it through
			// the canonical encoder. The submission gate legitimately rejects a
			// zero batch-payment amount, which would otherwise mask the property
			// this subtest exists to pin.
			withNil, err := prepareCanonical(factory.make(nil), resolveSubmit(nil))
			if err != nil {
				t.Fatalf("prepare nil: %v", err)
			}
			withZero, err := prepareCanonical(factory.make(big.NewInt(0)), resolveSubmit(nil))
			if err != nil {
				t.Fatalf("prepare zero: %v", err)
			}
			if !bytes.Equal(withNil.SigningHash(), withZero.SigningHash()) {
				t.Fatal("nil U256 field does not hash identically to explicit zero")
			}
		})
		for _, invalid := range []struct {
			name  string
			value *big.Int
		}{
			{"negative", big.NewInt(-1)},
			{"overflow", overflow},
		} {
			invalid := invalid
			t.Run(factory.name+"/"+invalid.name, func(t *testing.T) {
				if _, err := PrepareTransaction(factory.make(new(big.Int).Set(invalid.value))); err == nil {
					t.Fatal("PrepareTransaction accepted a value Rust U256 cannot represent")
				}
			})
		}
	}
}
