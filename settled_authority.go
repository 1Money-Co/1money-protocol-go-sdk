package onemoney

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// VerifyTokenAddress is the reserved mint address of the VERIFY token. A
// clawback of VERIFY by its master stays the legacy clawback in every envelope
// mode, and VERIFY has no lane clawback.
var VerifyTokenAddress = common.HexToAddress("0x0000000000000000000000000000000000000002")

// IsVerifyToken reports whether token is the VERIFY token.
func IsVerifyToken(token common.Address) bool { return token == VerifyTokenAddress }

// SettledRole is a delegated role of a token that its master grants and
// revokes in a settled authority operation.
type SettledRole string

const (
	SettledRolePause          SettledRole = "Pause"
	SettledRoleManageList     SettledRole = "ManageList"
	SettledRoleUpdateMetadata SettledRole = "UpdateMetadata"
	SettledRoleBridge         SettledRole = "Bridge"
	SettledRoleClawback       SettledRole = "Clawback"
)

var settledRoleWire = map[SettledRole]string{
	SettledRolePause: "pause", SettledRoleManageList: "manage_list",
	SettledRoleUpdateMetadata: "update_metadata", SettledRoleBridge: "bridge",
	SettledRoleClawback: "clawback",
}

// SettledListKind names the list that a settled list operation writes.
type SettledListKind string

const (
	SettledListBlacklist SettledListKind = "Blacklist"
	SettledListWhitelist SettledListKind = "Whitelist"
)

var settledListWire = map[SettledListKind]string{SettledListBlacklist: "blacklist", SettledListWhitelist: "whitelist"}

// SettledListAction adds an account to a list or removes it.
type SettledListAction string

const (
	SettledListAdd    SettledListAction = "Add"
	SettledListRemove SettledListAction = "Remove"
)

var settledListActionWire = map[SettledListAction]string{SettledListAdd: "add", SettledListRemove: "remove"}

// SettledAuthorityOperation is the operation of a settled authority envelope.
// The implementations are the Settled* types of this package.
type SettledAuthorityOperation interface {
	// rlpName tags the operation in its RLP encoding; wireType tags it in JSON.
	rlpName() string
	wireType() string
	rlpFields() []interface{}
	wireMembers() map[string]interface{}
	validate() error
}

type (
	SettledPause                struct{}
	SettledUnpause              struct{}
	SettledRevokeMasterMintBurn struct{}
	SettledList                 struct {
		List    SettledListKind
		Action  SettledListAction
		Account common.Address
	}
	SettledUpdateMetadata struct {
		Name               string
		URI                string
		AdditionalMetadata []AdditionalMetadata
	}
	SettledGrantRole struct {
		Role    SettledRole
		Account common.Address
	}
	SettledRevokeRole struct {
		Role    SettledRole
		Account common.Address
	}
	// SettledSetMinterAllowance sets the minter's absolute allowance, and adds
	// the minter when it is new.
	SettledSetMinterAllowance struct {
		Minter    common.Address
		Allowance *big.Int
	}
	SettledRevokeMinter         struct{ Minter common.Address }
	SettledRotateMasterMintBurn struct{ Account common.Address }
	SettledMint                 struct {
		Recipient common.Address
		Value     *big.Int
	}
)

func (SettledPause) rlpName() string                { return "Pause" }
func (SettledUnpause) rlpName() string              { return "Unpause" }
func (SettledRevokeMasterMintBurn) rlpName() string { return "RevokeMasterMintBurn" }
func (SettledList) rlpName() string                 { return "List" }
func (SettledUpdateMetadata) rlpName() string       { return "UpdateMetadata" }
func (SettledGrantRole) rlpName() string            { return "GrantRole" }
func (SettledRevokeRole) rlpName() string           { return "RevokeRole" }
func (SettledSetMinterAllowance) rlpName() string   { return "SetMinterAllowance" }
func (SettledRevokeMinter) rlpName() string         { return "RevokeMinter" }
func (SettledRotateMasterMintBurn) rlpName() string { return "RotateMasterMintBurn" }
func (SettledMint) rlpName() string                 { return "Mint" }

func (SettledPause) wireType() string                { return "pause" }
func (SettledUnpause) wireType() string              { return "unpause" }
func (SettledRevokeMasterMintBurn) wireType() string { return "revoke_master_mint_burn" }
func (SettledList) wireType() string                 { return "list" }
func (SettledUpdateMetadata) wireType() string       { return "update_metadata" }
func (SettledGrantRole) wireType() string            { return "grant_role" }
func (SettledRevokeRole) wireType() string           { return "revoke_role" }
func (SettledSetMinterAllowance) wireType() string   { return "set_minter_allowance" }
func (SettledRevokeMinter) wireType() string         { return "revoke_minter" }
func (SettledRotateMasterMintBurn) wireType() string { return "rotate_master_mint_burn" }
func (SettledMint) wireType() string                 { return "mint" }

func (SettledPause) rlpFields() []interface{}                { return nil }
func (SettledUnpause) rlpFields() []interface{}              { return nil }
func (SettledRevokeMasterMintBurn) rlpFields() []interface{} { return nil }
func (o SettledList) rlpFields() []interface{} {
	return []interface{}{[]byte(o.List), []byte(o.Action), o.Account}
}

func (o SettledUpdateMetadata) rlpFields() []interface{} {
	return []interface{}{[]byte(o.Name), []byte(o.URI), metadataPairsRLP(o.AdditionalMetadata)}
}
func (o SettledGrantRole) rlpFields() []interface{}  { return []interface{}{[]byte(o.Role), o.Account} }
func (o SettledRevokeRole) rlpFields() []interface{} { return []interface{}{[]byte(o.Role), o.Account} }
func (o SettledSetMinterAllowance) rlpFields() []interface{} {
	return []interface{}{o.Minter, bigOrZero(o.Allowance)}
}
func (o SettledRevokeMinter) rlpFields() []interface{}         { return []interface{}{o.Minter} }
func (o SettledRotateMasterMintBurn) rlpFields() []interface{} { return []interface{}{o.Account} }
func (o SettledMint) rlpFields() []interface{}                 { return []interface{}{o.Recipient, bigOrZero(o.Value)} }

func (SettledPause) wireMembers() map[string]interface{}                { return nil }
func (SettledUnpause) wireMembers() map[string]interface{}              { return nil }
func (SettledRevokeMasterMintBurn) wireMembers() map[string]interface{} { return nil }
func (o SettledList) wireMembers() map[string]interface{} {
	return map[string]interface{}{
		"list": settledListWire[o.List], "action": settledListActionWire[o.Action], "account": o.Account,
	}
}

func (o SettledUpdateMetadata) wireMembers() map[string]interface{} {
	return map[string]interface{}{
		"name": o.Name, "uri": o.URI,
		"additional_metadata": append([]AdditionalMetadata{}, o.AdditionalMetadata...),
	}
}

func (o SettledGrantRole) wireMembers() map[string]interface{} {
	return map[string]interface{}{"role": settledRoleWire[o.Role], "account": o.Account}
}

func (o SettledRevokeRole) wireMembers() map[string]interface{} {
	return map[string]interface{}{"role": settledRoleWire[o.Role], "account": o.Account}
}

func (o SettledSetMinterAllowance) wireMembers() map[string]interface{} {
	return map[string]interface{}{"minter": o.Minter, "allowance": bigStr(o.Allowance)}
}

func (o SettledRevokeMinter) wireMembers() map[string]interface{} {
	return map[string]interface{}{"minter": o.Minter}
}

func (o SettledRotateMasterMintBurn) wireMembers() map[string]interface{} {
	return map[string]interface{}{"account": o.Account}
}

func (o SettledMint) wireMembers() map[string]interface{} {
	return map[string]interface{}{"recipient": o.Recipient, "value": bigStr(o.Value)}
}

func (SettledPause) validate() error                { return nil }
func (SettledUnpause) validate() error              { return nil }
func (SettledRevokeMasterMintBurn) validate() error { return nil }
func (SettledUpdateMetadata) validate() error       { return nil }
func (SettledRevokeMinter) validate() error         { return nil }
func (SettledRotateMasterMintBurn) validate() error { return nil }

func (o SettledList) validate() error {
	if _, ok := settledListWire[o.List]; !ok {
		return fmt.Errorf("settled list operation: unknown list %q", o.List)
	}
	if _, ok := settledListActionWire[o.Action]; !ok {
		return fmt.Errorf("settled list operation: unknown action %q", o.Action)
	}
	return nil
}
func (o SettledGrantRole) validate() error  { return validateSettledRole(o.Role) }
func (o SettledRevokeRole) validate() error { return validateSettledRole(o.Role) }
func (o SettledSetMinterAllowance) validate() error {
	return validateU256("set_minter_allowance.allowance", o.Allowance)
}
func (o SettledMint) validate() error { return validateU256("mint.value", o.Value) }

func validateSettledRole(role SettledRole) error {
	if _, ok := settledRoleWire[role]; !ok {
		return fmt.Errorf("settled role operation: unknown role %q", role)
	}
	return nil
}

func metadataPairsRLP(pairs []AdditionalMetadata) []interface{} {
	out := make([]interface{}, 0, len(pairs))
	for _, kv := range pairs {
		out = append(out, []interface{}{[]byte(kv.Key), []byte(kv.Value)})
	}
	return out
}

// SettledAuthorityPayload is native operation 17: one authority operation on a
// token, signed at the signer's nonce and decided at its checkpoint.
// ReferenceCheckpoint is the committed checkpoint whose state admission reads
// the signer's role at; a node admits it only while it is at most 100
// checkpoints behind the node's latest committed one.
type SettledAuthorityPayload struct {
	ChainID             uint64
	Nonce               uint64
	Token               common.Address
	ReferenceCheckpoint uint64
	Operation           SettledAuthorityOperation
}

func (p SettledAuthorityPayload) validate() error {
	if p.Operation == nil {
		return fmt.Errorf("settled authority payload: operation is required")
	}
	return p.Operation.validate()
}

func (p SettledAuthorityPayload) rlpList() []interface{} {
	operation := append([]interface{}{[]byte(p.Operation.rlpName())}, p.Operation.rlpFields()...)
	return []interface{}{p.ChainID, p.Nonce, p.Token, p.ReferenceCheckpoint, operation}
}

func (p SettledAuthorityPayload) wireFields() map[string]interface{} {
	operation := map[string]interface{}{"type": p.Operation.wireType()}
	for k, v := range p.Operation.wireMembers() {
		operation[k] = v
	}
	return map[string]interface{}{
		"chain_id": p.ChainID, "nonce": p.Nonce, "token": p.Token,
		"reference_checkpoint": p.ReferenceCheckpoint, "operation": operation,
	}
}

// LaneClawbackPayload is native operation 18: a clawback that takes the
// holder's nonce FromNonce (the holder's next nonce when it is signed) and no
// nonce of its signer. It executes at certification, as a payment does.
type LaneClawbackPayload struct {
	ChainID             uint64
	Token               common.Address
	Holder              common.Address
	FromNonce           uint64
	ReferenceCheckpoint uint64
	Recipient           common.Address
	Value               *big.Int
}

func (p LaneClawbackPayload) rlpList() []interface{} {
	return []interface{}{
		p.ChainID, p.Token, p.Holder, p.FromNonce, p.ReferenceCheckpoint, p.Recipient, bigOrZero(p.Value),
	}
}

func (p LaneClawbackPayload) wireFields() map[string]interface{} {
	return map[string]interface{}{
		"chain_id": p.ChainID, "token": p.Token, "holder": p.Holder, "from_nonce": p.FromNonce,
		"reference_checkpoint": p.ReferenceCheckpoint, "recipient": p.Recipient, "value": bigStr(p.Value),
	}
}

// refuseLaneClawbackOfVerify: the nodes refuse a lane clawback of VERIFY for
// every signer, and its master claws VERIFY back with the legacy clawback.
func refuseLaneClawbackOfVerify(token common.Address) error {
	if IsVerifyToken(token) {
		return fmt.Errorf("VERIFY has no lane clawback: its master claws it back with the legacy clawback")
	}
	return nil
}
