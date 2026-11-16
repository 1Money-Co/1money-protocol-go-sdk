package onemoney

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
)

// storedAuthorized is the stored form of an AuthorizedTransaction: the route and
// the exact request body, plus the transaction hash, which loading recomputes.
type storedAuthorized struct {
	Path            string          `json:"path"`
	Body            json.RawMessage `json:"body"`
	TransactionHash string          `json:"transaction_hash"`
}

// MarshalJSON stores the transaction so that ParseAuthorizedTransaction can
// load it after a restart. Only the authority class is storable: operations
// 17 and 18, and the legacy mint, grant and revoke, blacklist, whitelist,
// pause, metadata update and clawback.
func (a *AuthorizedTransaction) MarshalJSON() ([]byte, error) {
	if _, ok := storableBodies[a.path]; !ok {
		return nil, fmt.Errorf("%s has no stored form: only authority-class transactions are storable", a.op.label())
	}
	body, err := json.Marshal(a.body)
	if err != nil {
		return nil, err
	}
	return json.Marshal(storedAuthorized{Path: a.path, Body: body, TransactionHash: hexLower(a.txHash)})
}

// ParseAuthorizedTransaction loads a transaction that MarshalJSON stored. It
// rebuilds the transaction from its request body and refuses a record whose
// recomputed hash differs from the stored one, a body with an unknown member,
// and a route outside the authority class.
func ParseAuthorizedTransaction(data []byte) (*AuthorizedTransaction, error) {
	var stored storedAuthorized
	if err := strictUnmarshal(data, &stored); err != nil {
		return nil, fmt.Errorf("stored transaction: %w", err)
	}
	authorized, err := authorizedFromBody(stored.Path, stored.Body)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(hexLower(authorized.txHash), stored.TransactionHash) {
		return nil, fmt.Errorf("stored transaction: hash %s does not match its body, which hashes to %s",
			stored.TransactionHash, hexLower(authorized.txHash))
	}
	return authorized, nil
}

// authorizedFromBody rebuilds a signed transaction from the request body that
// was sent to path.
func authorizedFromBody(path string, body []byte) (*AuthorizedTransaction, error) {
	decode, ok := storableBodies[path]
	if !ok {
		return nil, fmt.Errorf("%s is not a stored authority-class route", path)
	}
	var envelope struct {
		Memo          Memo `json:"memo"`
		Authorization struct {
			Type      string    `json:"type"`
			Signature Signature `json:"signature"`
		} `json:"authorization"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("stored body: %w", err)
	}
	if envelope.Authorization.Type != "single_secp256k1" {
		return nil, fmt.Errorf("stored body: authorization type %q is not single_secp256k1", envelope.Authorization.Type)
	}
	payload, opts, err := decode(body)
	if err != nil {
		return nil, fmt.Errorf("stored body: %w", err)
	}
	prepared, err := PrepareTransaction(payload, append(opts, WithMemo(envelope.Memo))...)
	if err != nil {
		return nil, err
	}
	authorized, err := prepared.Authorize(envelope.Authorization.Signature)
	if err != nil {
		return nil, err
	}
	if authorized.path != path {
		return nil, fmt.Errorf("stored body: route %s does not match its operation's route %s", path, authorized.path)
	}
	// A member of another operation variant decodes without error but is not
	// signed; only the exact body of the rebuilt transaction is accepted.
	rebuilt, err := json.Marshal(authorized.body)
	if err != nil {
		return nil, err
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, body); err != nil {
		return nil, fmt.Errorf("stored body: %w", err)
	}
	if !bytes.Equal(rebuilt, compact.Bytes()) {
		return nil, fmt.Errorf("stored body: not the exact body of the transaction it signs")
	}
	return authorized, nil
}

type bodyDecoder func(body []byte) (payload any, opts []SubmitOption, err error)

var storableBodies = map[string]bodyDecoder{
	"/v2/tokens/authority_operation": decodeSettledBody,
	"/v2/tokens/lane_clawback":       decodeLaneClawbackBody,
	"/v2/tokens/clawback":            decodeClawbackBody,
	"/v2/tokens/mint":                decodeMintBody,
	"/v2/tokens/grant_authority":     decodeAuthorityBody,
	"/v2/tokens/manage_blacklist":    decodeListBody(ManageListBlacklist),
	"/v2/tokens/manage_whitelist":    decodeListBody(ManageListWhitelist),
	"/v2/tokens/pause":               decodePauseBody,
	"/v2/tokens/update_metadata":     decodeMetadataBody,
}

// strictUnmarshal refuses an unknown member, so that a stored body holds
// nothing that the rebuilt transaction would not sign.
func strictUnmarshal(data []byte, v any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(v)
}

// decimal is a U256 amount sent as a decimal string.
type decimal struct{ *big.Int }

func (d *decimal) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return fmt.Errorf("amount must be a decimal string: %w", err)
	}
	value, ok := new(big.Int).SetString(text, 10)
	if !ok {
		return fmt.Errorf("amount %q is not a decimal integer", text)
	}
	d.Int = value
	return nil
}

// bodyEnvelope holds the members that every /v2 body carries besides its payload.
type bodyEnvelope struct {
	Memo          json.RawMessage `json:"memo"`
	Authorization json.RawMessage `json:"authorization"`
}

func decodeSettledBody(body []byte) (any, []SubmitOption, error) {
	var b struct {
		bodyEnvelope
		ChainID             uint64          `json:"chain_id"`
		Nonce               uint64          `json:"nonce"`
		Token               common.Address  `json:"token"`
		ReferenceCheckpoint uint64          `json:"reference_checkpoint"`
		Operation           json.RawMessage `json:"operation"`
	}
	if err := strictUnmarshal(body, &b); err != nil {
		return nil, nil, err
	}
	operation, err := decodeSettledOperation(b.Operation)
	if err != nil {
		return nil, nil, err
	}
	return SettledAuthorityPayload{
		ChainID: b.ChainID, Nonce: b.Nonce, Token: b.Token,
		ReferenceCheckpoint: b.ReferenceCheckpoint, Operation: operation,
	}, nil, nil
}

func decodeLaneClawbackBody(body []byte) (any, []SubmitOption, error) {
	var b struct {
		bodyEnvelope
		ChainID             uint64         `json:"chain_id"`
		Token               common.Address `json:"token"`
		Holder              common.Address `json:"holder"`
		FromNonce           uint64         `json:"from_nonce"`
		ReferenceCheckpoint uint64         `json:"reference_checkpoint"`
		Recipient           common.Address `json:"recipient"`
		Value               decimal        `json:"value"`
	}
	if err := strictUnmarshal(body, &b); err != nil {
		return nil, nil, err
	}
	return LaneClawbackPayload{
		ChainID: b.ChainID, Token: b.Token, Holder: b.Holder, FromNonce: b.FromNonce,
		ReferenceCheckpoint: b.ReferenceCheckpoint, Recipient: b.Recipient, Value: b.Value.Int,
	}, nil, nil
}

func decodeClawbackBody(body []byte) (any, []SubmitOption, error) {
	var b struct {
		bodyEnvelope
		ChainID   uint64         `json:"chain_id"`
		Nonce     uint64         `json:"nonce"`
		Token     common.Address `json:"token"`
		From      common.Address `json:"from"`
		Recipient common.Address `json:"recipient"`
		Value     decimal        `json:"value"`
	}
	if err := strictUnmarshal(body, &b); err != nil {
		return nil, nil, err
	}
	return TokenClawbackPayload{
		ChainID: b.ChainID, Nonce: b.Nonce, Token: b.Token, From: b.From,
		Recipient: b.Recipient, Value: b.Value.Int,
	}, nil, nil
}

func decodeMintBody(body []byte) (any, []SubmitOption, error) {
	var b struct {
		bodyEnvelope
		ChainID   uint64         `json:"chain_id"`
		Nonce     uint64         `json:"nonce"`
		Recipient common.Address `json:"recipient"`
		Value     decimal        `json:"value"`
		Token     common.Address `json:"token"`
	}
	if err := strictUnmarshal(body, &b); err != nil {
		return nil, nil, err
	}
	return TokenMintPayload{ChainID: b.ChainID, Nonce: b.Nonce, Recipient: b.Recipient, Value: b.Value.Int, Token: b.Token}, nil, nil
}

func decodeAuthorityBody(body []byte) (any, []SubmitOption, error) {
	var b struct {
		bodyEnvelope
		ChainID          uint64          `json:"chain_id"`
		Nonce            uint64          `json:"nonce"`
		Action           AuthorityAction `json:"action"`
		AuthorityType    AuthorityType   `json:"authority_type"`
		AuthorityAddress common.Address  `json:"authority_address"`
		Token            common.Address  `json:"token"`
		Value            decimal         `json:"value"`
	}
	if err := strictUnmarshal(body, &b); err != nil {
		return nil, nil, err
	}
	return TokenAuthorityPayload{
		ChainID: b.ChainID, Nonce: b.Nonce, Action: b.Action, AuthorityType: b.AuthorityType,
		AuthorityAddress: b.AuthorityAddress, Token: b.Token, Value: b.Value.Int,
	}, nil, nil
}

func decodeListBody(kind ManageListKind) bodyDecoder {
	return func(body []byte) (any, []SubmitOption, error) {
		var b struct {
			bodyEnvelope
			ChainID uint64               `json:"chain_id"`
			Nonce   uint64               `json:"nonce"`
			Action  ManageListActionType `json:"action"`
			Address common.Address       `json:"address"`
			Token   common.Address       `json:"token"`
		}
		if err := strictUnmarshal(body, &b); err != nil {
			return nil, nil, err
		}
		payload := TokenManageListPayload{ChainID: b.ChainID, Nonce: b.Nonce, Action: b.Action, Address: b.Address, Token: b.Token}
		return payload, []SubmitOption{WithManageListKind(kind)}, nil
	}
}

func decodePauseBody(body []byte) (any, []SubmitOption, error) {
	var b struct {
		bodyEnvelope
		ChainID uint64          `json:"chain_id"`
		Nonce   uint64          `json:"nonce"`
		Action  PauseActionType `json:"action"`
		Token   common.Address  `json:"token"`
	}
	if err := strictUnmarshal(body, &b); err != nil {
		return nil, nil, err
	}
	return PauseTokenPayload{ChainID: b.ChainID, Nonce: b.Nonce, Action: b.Action, Token: b.Token}, nil, nil
}

func decodeMetadataBody(body []byte) (any, []SubmitOption, error) {
	var b struct {
		bodyEnvelope
		ChainID            uint64               `json:"chain_id"`
		Nonce              uint64               `json:"nonce"`
		Name               string               `json:"name"`
		URI                string               `json:"uri"`
		Token              common.Address       `json:"token"`
		AdditionalMetadata []AdditionalMetadata `json:"additional_metadata"`
	}
	if err := strictUnmarshal(body, &b); err != nil {
		return nil, nil, err
	}
	return UpdateMetadataPayload{
		ChainID: b.ChainID, Nonce: b.Nonce, Name: b.Name, URI: b.URI,
		Token: b.Token, AdditionalMetadata: b.AdditionalMetadata,
	}, nil, nil
}

func decodeSettledOperation(raw json.RawMessage) (SettledAuthorityOperation, error) {
	var tag struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &tag); err != nil {
		return nil, fmt.Errorf("operation: %w", err)
	}
	var member struct {
		Type               string               `json:"type"`
		List               string               `json:"list"`
		Action             string               `json:"action"`
		Account            common.Address       `json:"account"`
		Name               string               `json:"name"`
		URI                string               `json:"uri"`
		AdditionalMetadata []AdditionalMetadata `json:"additional_metadata"`
		Role               string               `json:"role"`
		Minter             common.Address       `json:"minter"`
		Allowance          decimal              `json:"allowance"`
		Recipient          common.Address       `json:"recipient"`
		Value              decimal              `json:"value"`
	}
	if err := strictUnmarshal(raw, &member); err != nil {
		return nil, fmt.Errorf("operation: %w", err)
	}
	var operation SettledAuthorityOperation
	switch tag.Type {
	case "pause":
		operation = SettledPause{}
	case "unpause":
		operation = SettledUnpause{}
	case "revoke_master_mint_burn":
		operation = SettledRevokeMasterMintBurn{}
	case "list":
		operation = SettledList{
			List: wireKey(settledListWire, member.List), Action: wireKey(settledListActionWire, member.Action), Account: member.Account,
		}
	case "update_metadata":
		operation = SettledUpdateMetadata{Name: member.Name, URI: member.URI, AdditionalMetadata: member.AdditionalMetadata}
	case "grant_role":
		operation = SettledGrantRole{Role: wireKey(settledRoleWire, member.Role), Account: member.Account}
	case "revoke_role":
		operation = SettledRevokeRole{Role: wireKey(settledRoleWire, member.Role), Account: member.Account}
	case "set_minter_allowance":
		operation = SettledSetMinterAllowance{Minter: member.Minter, Allowance: member.Allowance.Int}
	case "revoke_minter":
		operation = SettledRevokeMinter{Minter: member.Minter}
	case "rotate_master_mint_burn":
		operation = SettledRotateMasterMintBurn{Account: member.Account}
	case "mint":
		operation = SettledMint{Recipient: member.Recipient, Value: member.Value.Int}
	default:
		return nil, fmt.Errorf("operation: unknown type %q", tag.Type)
	}
	return operation, nil
}

// wireKey maps a wire name back to its constant. An unknown name gives a value
// that the operation's validation refuses.
func wireKey[K ~string](names map[K]string, wire string) K {
	for key, name := range names {
		if name == wire {
			return key
		}
	}
	return K("unknown:" + wire)
}
