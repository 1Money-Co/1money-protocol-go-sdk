package onemoney

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// SignedTransaction is a transaction that the SDK signed and has not sent yet.
type SignedTransaction struct {
	Transaction *AuthorizedTransaction
	// Replaces is the hash of the transaction that this one supersedes: one
	// that a node refused with no effect (a retired legacy envelope, or a
	// reference checkpoint that was not ready). nil for a first signature.
	Replaces *common.Hash
}

// OnSigned receives every signed transaction before its send, so that the
// caller can store it (json.Marshal stores it; ParseAuthorizedTransaction loads
// it). An error stops the send and is returned.
type OnSigned func(ctx context.Context, signed SignedTransaction) error

// WithOnSigned hands every transaction that the call signs to onSigned before
// its send. It needs the domain-separated v2 mode: a legacy v1 request has no
// AuthorizedTransaction to hand over.
func WithOnSigned(onSigned OnSigned) SubmitOption {
	return func(c *submitConfig) { c.onSigned = onSigned }
}

// referenceRetryDelay is the wait before the one retry of a reference that a
// node refused as not committed or as not holding the token: behind a load
// balancer, the node that admits can be a checkpoint behind the node that
// named the reference, and a token issued just before is in no checkpoint yet.
var referenceRetryDelay = time.Second

// LegacyAuthorityOperation is an authority operation in the legacy payload
// that a caller builds. In the settled mode it is signed again as the settled
// authority envelope at the same chain and nonce. Build it with LegacyMint,
// LegacyAuthority, LegacyBlacklist, LegacyWhitelist, LegacyPause or
// LegacyMetadata.
type LegacyAuthorityOperation struct {
	payload  any
	listKind *ManageListKind
}

func LegacyMint(p TokenMintPayload) LegacyAuthorityOperation {
	return LegacyAuthorityOperation{payload: p}
}

// LegacyAuthority is a grant or a revoke, as p.Action says.
func LegacyAuthority(p TokenAuthorityPayload) LegacyAuthorityOperation {
	return LegacyAuthorityOperation{payload: p}
}

func LegacyBlacklist(p TokenManageListPayload) LegacyAuthorityOperation {
	kind := ManageListBlacklist
	return LegacyAuthorityOperation{payload: p, listKind: &kind}
}

func LegacyWhitelist(p TokenManageListPayload) LegacyAuthorityOperation {
	kind := ManageListWhitelist
	return LegacyAuthorityOperation{payload: p, listKind: &kind}
}

// LegacyPause is a pause or an unpause, as p.Action says.
func LegacyPause(p PauseTokenPayload) LegacyAuthorityOperation {
	return LegacyAuthorityOperation{payload: p}
}

func LegacyMetadata(p UpdateMetadataPayload) LegacyAuthorityOperation {
	return LegacyAuthorityOperation{payload: p}
}

// IsListOperation reports whether the operation is a blacklist or whitelist
// operation, whose settled form depends on the token's privacy.
func (o LegacyAuthorityOperation) IsListOperation() bool { return o.listKind != nil }

func (o LegacyAuthorityOperation) header() (chainID, nonce uint64, token common.Address, err error) {
	switch p := o.payload.(type) {
	case TokenMintPayload:
		return p.ChainID, p.Nonce, p.Token, nil
	case TokenAuthorityPayload:
		return p.ChainID, p.Nonce, p.Token, nil
	case TokenManageListPayload:
		return p.ChainID, p.Nonce, p.Token, nil
	case PauseTokenPayload:
		return p.ChainID, p.Nonce, p.Token, nil
	case UpdateMetadataPayload:
		return p.ChainID, p.Nonce, p.Token, nil
	}
	return 0, 0, common.Address{}, fmt.Errorf("empty LegacyAuthorityOperation: build it with LegacyMint, LegacyAuthority, LegacyBlacklist, LegacyWhitelist, LegacyPause or LegacyMetadata")
}

// SettledOperation returns the settled operation with the legacy operation's
// effect. A list operation names the list that the legacy envelope writes on a
// token of this privacy: on a private token a freeze is a whitelist remove.
// A grant of MintBurnTokens sets the minter's absolute allowance, and a grant
// of MasterMintBurn rotates the master minter, or revokes it when it names the
// zero address.
//
// Errors: an unknown action or authority type.
func (o LegacyAuthorityOperation) SettledOperation(tokenIsPrivate bool) (SettledAuthorityOperation, error) {
	switch p := o.payload.(type) {
	case TokenMintPayload:
		return SettledMint{Recipient: p.Recipient, Value: p.Value}, nil
	case TokenAuthorityPayload:
		return settledAuthority(p)
	case TokenManageListPayload:
		return settledList(*o.listKind, p, tokenIsPrivate)
	case PauseTokenPayload:
		switch p.Action {
		case Pause:
			return SettledPause{}, nil
		case UnPause:
			return SettledUnpause{}, nil
		}
		return nil, fmt.Errorf("unknown pause action %q", p.Action)
	case UpdateMetadataPayload:
		return SettledUpdateMetadata{Name: p.Name, URI: p.URI, AdditionalMetadata: p.AdditionalMetadata}, nil
	}
	_, _, _, err := o.header()
	return nil, err
}

// Settled returns the settled authority payload at the legacy payload's chain
// and nonce, with the given reference checkpoint.
func (o LegacyAuthorityOperation) Settled(tokenIsPrivate bool, referenceCheckpoint uint64) (SettledAuthorityPayload, error) {
	chainID, nonce, token, err := o.header()
	if err != nil {
		return SettledAuthorityPayload{}, err
	}
	operation, err := o.SettledOperation(tokenIsPrivate)
	if err != nil {
		return SettledAuthorityPayload{}, err
	}
	return SettledAuthorityPayload{
		ChainID: chainID, Nonce: nonce, Token: token,
		ReferenceCheckpoint: referenceCheckpoint, Operation: operation,
	}, nil
}

func settledAuthority(p TokenAuthorityPayload) (SettledAuthorityOperation, error) {
	grant := p.Action == AuthorityActionGrant
	if !grant && p.Action != AuthorityActionRevoke {
		return nil, fmt.Errorf("unknown authority action %q", p.Action)
	}
	account := p.AuthorityAddress
	switch p.AuthorityType {
	case AuthorityTypeMasterMintBurn:
		if grant && account != (common.Address{}) {
			return SettledRotateMasterMintBurn{Account: account}, nil
		}
		return SettledRevokeMasterMintBurn{}, nil
	case AuthorityTypeMintBurnTokens:
		if grant {
			return SettledSetMinterAllowance{Minter: account, Allowance: p.Value}, nil
		}
		return SettledRevokeMinter{Minter: account}, nil
	}
	role, ok := map[AuthorityType]SettledRole{
		AuthorityTypePause: SettledRolePause, AuthorityTypeManageList: SettledRoleManageList,
		AuthorityTypeUpdateMetadata: SettledRoleUpdateMetadata, AuthorityTypeBridge: SettledRoleBridge,
		AuthorityTypeClawback: SettledRoleClawback,
	}[p.AuthorityType]
	if !ok {
		return nil, fmt.Errorf("unknown authority type %q", p.AuthorityType)
	}
	if grant {
		return SettledGrantRole{Role: role, Account: account}, nil
	}
	return SettledRevokeRole{Role: role, Account: account}, nil
}

func settledList(kind ManageListKind, p TokenManageListPayload, tokenIsPrivate bool) (SettledAuthorityOperation, error) {
	var freeze bool
	switch {
	case p.Action != ManageListActionAdd && p.Action != ManageListActionRemove:
		return nil, fmt.Errorf("unknown list action %q", p.Action)
	case kind == ManageListBlacklist:
		freeze = p.Action == ManageListActionAdd
	default:
		freeze = p.Action == ManageListActionRemove
	}
	operation := SettledList{Account: p.Address}
	switch {
	case tokenIsPrivate && freeze:
		operation.List, operation.Action = SettledListWhitelist, SettledListRemove
	case tokenIsPrivate:
		operation.List, operation.Action = SettledListWhitelist, SettledListAdd
	case freeze:
		operation.List, operation.Action = SettledListBlacklist, SettledListAdd
	default:
		operation.List, operation.Action = SettledListBlacklist, SettledListRemove
	}
	return operation, nil
}

// LatestReferenceCheckpoint is the default reference checkpoint of a new
// envelope: the node's latest committed checkpoint.
func (c *Client) LatestReferenceCheckpoint(ctx context.Context) (uint64, error) {
	number, err := c.GetCheckpointNumber(ctx)
	if err != nil {
		return 0, err
	}
	return number.Number, nil
}

// SettledPayload returns the settled authority payload of a legacy operation,
// at its chain and nonce, with the latest committed checkpoint as its
// reference. A list operation reads the token's privacy, which decides the
// list that it names.
func (a TokensAPI) SettledPayload(ctx context.Context, operation LegacyAuthorityOperation) (SettledAuthorityPayload, error) {
	_, _, token, err := operation.header()
	if err != nil {
		return SettledAuthorityPayload{}, err
	}
	tokenIsPrivate := false
	if operation.IsListOperation() {
		info, err := a.Metadata(ctx, token)
		if err != nil {
			return SettledAuthorityPayload{}, fmt.Errorf("token privacy read: %w", err)
		}
		tokenIsPrivate = info.IsPrivate
	}
	reference, err := a.c.LatestReferenceCheckpoint(ctx)
	if err != nil {
		return SettledAuthorityPayload{}, fmt.Errorf("reference checkpoint read: %w", err)
	}
	return operation.Settled(tokenIsPrivate, reference)
}

// AuthorityOperationSubmission is an authority operation that was sent once.
type AuthorityOperationSubmission struct {
	Envelope        AuthorityEnvelope
	TransactionHash common.Hash
}

// AuthorityOperationByMode reads the node's envelope mode and sends the
// operation in its envelope, as SendAuthorityOperation does.
//
// Errors: ErrUpgradeGap in the upgrade gap and *UnrecognizedEnvelopeModeError
// for a mode that this SDK does not know, both before anything is signed; else
// as SendAuthorityOperation.
func (a TokensAPI) AuthorityOperationByMode(ctx context.Context, operation LegacyAuthorityOperation, signer Signer, opts ...SubmitOption) (*AuthorityOperationSubmission, error) {
	mode, err := a.c.GetEnvelopeMode(ctx)
	if err != nil {
		return nil, err
	}
	envelope, err := mode.AuthorityEnvelope()
	if err != nil {
		return nil, err
	}
	return a.SendAuthorityOperation(ctx, envelope, operation, signer, opts...)
}

// SendAuthorityOperation sends an authority operation in envelope: the legacy
// payload as given (on the client's submission mode), or the settled envelope
// at the same chain and nonce with the latest committed checkpoint as its
// reference (always on /v2). Pass WithMemo to sign a memo, and WithOnSigned to
// store each signed transaction before its send.
//
// A refusal with no effect leads to one more signature at the same nonce, so
// it can never make two transactions:
//   - A node that refuses the legacy envelope as retired took no nonce and no
//     lock. The legacy form is never sent again: the mode is read again, and
//     once the node reports settled, the operation is signed as the settled
//     envelope.
//   - A node that refuses the settled envelope's reference as not committed,
//     or as not holding the token yet, gets one more signature with a
//     reference read again after a short wait.
//
// The settled envelope's outcome is decided at its checkpoint: read it with
// Client.AuthorityOperationState.
//
// Errors: as the single send. After a retired legacy envelope: the refusal
// while the node does not report settled, ErrUpgradeGap in the gap. After a
// second reference refusal: that refusal; retry later.
func (a TokensAPI) SendAuthorityOperation(ctx context.Context, envelope AuthorityEnvelope, operation LegacyAuthorityOperation, signer Signer, opts ...SubmitOption) (*AuthorityOperationSubmission, error) {
	if signer == nil {
		return nil, fmt.Errorf("nil signer: pass a Signer (e.g. NewPrivateKeySigner or a KMS/HSM-backed one)")
	}
	if _, _, _, err := operation.header(); err != nil {
		return nil, err
	}
	cfg := resolveSubmit(opts)
	switch envelope {
	case AuthorityEnvelopeSettled:
		return a.sendSettled(ctx, operation, cfg, signer)
	case AuthorityEnvelopeLegacy:
	default:
		return nil, fmt.Errorf("unknown authority envelope %q", envelope)
	}
	legacyCfg := cfg
	legacyCfg.listKind = operation.listKind
	out := new(TransactionResponse)
	var legacyHash *common.Hash
	var err error
	if a.c.mode() == SubmissionModeLegacyV1 {
		err = a.c.submitPayload(ctx, operation.payload, legacyCfg, signer, out)
	} else {
		legacyHash, err = a.c.signAndSubmitV2(ctx, operation.payload, legacyCfg, signer, out)
	}
	if err == nil {
		return &AuthorityOperationSubmission{Envelope: AuthorityEnvelopeLegacy, TransactionHash: common.HexToHash(out.Hash)}, nil
	}
	if !isRefusal(err, RefusalLegacyEnvelopeRetired) {
		return nil, err
	}
	if err := a.c.settledAfterRetirement(ctx, err); err != nil {
		return nil, err
	}
	cfg.replaces = legacyHash
	return a.sendSettled(ctx, operation, cfg, signer)
}

// settledAfterRetirement returns nil once the node reports settled after it
// refused a legacy envelope as retired; else the refusal, or the error of the
// mode that the node reports, so that the caller retries later.
func (c *Client) settledAfterRetirement(ctx context.Context, refusal error) error {
	mode, err := c.GetEnvelopeMode(ctx)
	if err != nil {
		return err
	}
	envelope, err := mode.AuthorityEnvelope()
	switch {
	case err != nil:
		return err
	case envelope == AuthorityEnvelopeSettled:
		return nil
	}
	return refusal
}

func (a TokensAPI) sendSettled(ctx context.Context, operation LegacyAuthorityOperation, cfg submitConfig, signer Signer) (*AuthorityOperationSubmission, error) {
	hash, err := retryOnceOnReferenceNotReady(ctx, &cfg, func(cfg submitConfig) (*common.Hash, error) {
		payload, err := a.SettledPayload(ctx, operation)
		if err != nil {
			return nil, err
		}
		return a.c.signAndSubmitV2(ctx, payload, cfg, signer, new(TransactionResponse))
	})
	if err != nil {
		return nil, err
	}
	return &AuthorityOperationSubmission{Envelope: AuthorityEnvelopeSettled, TransactionHash: *hash}, nil
}

// retryOnceOnReferenceNotReady runs attempt once more, after a short wait,
// when a node refused the reference checkpoint as not committed yet, or as not
// holding the token yet. Either refusal had no effect, and attempt reads the
// reference again; the second attempt names the first in cfg.replaces. A
// second refusal is returned: a token missing at the reference is not evidence
// that it does not exist.
func retryOnceOnReferenceNotReady(ctx context.Context, cfg *submitConfig, attempt func(submitConfig) (*common.Hash, error)) (*common.Hash, error) {
	hash, err := attempt(*cfg)
	if !isRefusal(err, RefusalReferenceCheckpointNotCommitted, RefusalTokenNotAtReferenceCheckpoint) {
		return hash, err
	}
	timer := time.NewTimer(referenceRetryDelay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
	}
	retry := *cfg
	retry.replaces = hash
	return attempt(retry)
}
