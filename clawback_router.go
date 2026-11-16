package onemoney

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
)

// ClawbackIntentNonceUsedError: the signer's nonce has passed the nonce of the
// clawback intent, so the lane clawback was not signed. A legacy clawback of
// the same intent may have executed: a node of the release refuses a resend of
// one that executed as retired too, and a lane clawback takes the holder's
// slot, so the signer's nonce would no longer stop a second debit. Read the
// legacy clawback's outcome by its hash first.
type ClawbackIntentNonceUsedError struct {
	Signer    common.Address
	Nonce     uint64
	NextNonce uint64
}

func (e *ClawbackIntentNonceUsedError) Error() string {
	return fmt.Sprintf("the clawback intent at nonce %d of %s is past: the signer's next nonce is %d; read the earlier clawback's outcome by its hash before you claw back again; nothing was signed",
		e.Nonce, e.Signer.Hex(), e.NextNonce)
}

// ClawbackSubmission is a clawback that was sent once.
type ClawbackSubmission struct {
	// Envelope is AuthorityEnvelopeLegacy for the legacy clawback at the
	// signer's nonce, and AuthorityEnvelopeSettled for the lane clawback.
	Envelope        AuthorityEnvelope
	TransactionHash common.Hash
	// Lane is the record of the lane clawback; read where it stands with
	// Client.LaneClawbackFate. nil for the legacy clawback.
	Lane *LaneClawbackRecord
}

// TakesSignerNonce reports whether the clawback took the signer's nonce: only
// the legacy clawback does.
func (s *ClawbackSubmission) TakesSignerNonce() bool { return s.Lane == nil }

// ClawbackByMode claws back in the envelope that EnvelopeMode.ClawbackEnvelope
// gives for the node's mode, as SendClawback does. A clawback that
// ClawbackStaysLegacy names reads no mode.
//
// Errors: as SendClawback; ErrUpgradeGap and *UnrecognizedEnvelopeModeError
// before anything is signed.
func (a TokensAPI) ClawbackByMode(ctx context.Context, data TokenClawbackPayload, source ClawbackSource, signer Signer, onSigned OnSigned, opts ...SubmitOption) (*ClawbackSubmission, error) {
	if err := a.c.refuseClawbackRouterCall(signer, onSigned); err != nil {
		return nil, err
	}
	envelope := AuthorityEnvelopeLegacy
	if !ClawbackStaysLegacy(data.Token, source) {
		mode, err := a.c.GetEnvelopeMode(ctx)
		if err != nil {
			return nil, err
		}
		if envelope, err = mode.AuthorityEnvelope(); err != nil {
			return nil, err
		}
	}
	return a.SendClawback(ctx, envelope, data, source, signer, onSigned, opts...)
}

// SendClawback claws back data.Value of data.Token from data.From to
// data.Recipient in envelope: the legacy clawback at data.Nonce, or the lane
// clawback at the holder's next nonce. A clawback that ClawbackStaysLegacy
// names is the legacy clawback whatever envelope says.
//
// Call it once per intent. Every signed clawback, legacy and lane, goes to
// onSigned before its send; an error from onSigned sends nothing. After any
// error, read the outcome of the last transaction stored
// (Client.LaneClawbackFate through ParseLaneClawbackRecord, or
// Client.AuthorityOperationState for a legacy clawback) and never call again:
// the SDK keeps no record of earlier calls, and a lane clawback never advances
// the signer's nonce, so a second call can sign a second lane clawback that
// debits the holder twice. One call can store two transactions (a legacy
// clawback and, after its retired refusal, the lane clawback; or two lane
// clawbacks after a refused reference); the later one names the earlier in
// SignedTransaction.Replaces.
//
// data.Nonce names the intent on its signer's lane: the signer's next nonce for
// a new clawback, or the nonce of an earlier legacy send of it. The lane
// clawback is signed only while the signer's nonce has not passed it. A node
// that refuses the legacy clawback of an ordinary account as retired took no
// nonce and no lock for it: once the node reports settled, the clawback is
// issued as the lane clawback. A clawback that stays legacy has no new
// envelope: its retired refusal is returned; a node that has not replayed a
// Reserved Payment hold yet refuses the hold's clawback so, and the same
// clawback can succeed later.
//
// It refuses SubmissionModeLegacyV1 before signing: a /v1 clawback has no
// AuthorizedTransaction to hand to onSigned, and every network that runs the
// token authority model serves the domain-separated v2 writes. Tokens().Clawback
// stays available there.
//
// Errors: a nil onSigned or the legacy v1 mode, before anything is signed; as
// the single send; *ClawbackIntentNonceUsedError before the lane clawback is
// signed; as Client.LaneClawbackPayload; after a retired legacy clawback, the
// refusal, or ErrUpgradeGap in the gap.
func (a TokensAPI) SendClawback(ctx context.Context, envelope AuthorityEnvelope, data TokenClawbackPayload, source ClawbackSource, signer Signer, onSigned OnSigned, opts ...SubmitOption) (*ClawbackSubmission, error) {
	if err := a.c.refuseClawbackRouterCall(signer, onSigned); err != nil {
		return nil, err
	}
	cfg := clawbackConfig(opts, onSigned)
	stays := ClawbackStaysLegacy(data.Token, source)
	switch envelope {
	case AuthorityEnvelopeSettled:
		if !stays {
			return a.laneClawbackOfIntent(ctx, data, cfg, signer)
		}
	case AuthorityEnvelopeLegacy:
	default:
		return nil, fmt.Errorf("unknown authority envelope %q", envelope)
	}
	out := new(BurnTokenResponse)
	legacy, err := a.c.signAndSubmitV2(ctx, data, cfg, signer, out)
	if err == nil {
		return &ClawbackSubmission{Envelope: AuthorityEnvelopeLegacy, TransactionHash: *legacy.hash()}, nil
	}
	if stays || !isRefusal(err, RefusalLegacyEnvelopeRetired) {
		return nil, err
	}
	if err := a.c.settledAfterRetirement(ctx, err); err != nil {
		return nil, err
	}
	cfg.replaces = legacy.hash()
	return a.laneClawbackOfIntent(ctx, data, cfg, signer)
}

// IssueLaneClawback builds a lane clawback of clawback's token, holder (From),
// recipient and value at the holder's next nonce, hands it to onSigned, and
// sends it once. clawback.Nonce is not used and not checked: prefer
// SendClawback for an intent that may have a legacy send. A node that refuses
// the reference as not committed, or as not holding the token, gets one more
// lane clawback built after a short wait; the refused one had no effect.
//
// Call it once per intent, with the contract of SendClawback.
//
// Errors: a nil onSigned or the legacy v1 mode, before anything is signed; as
// Client.LaneClawbackPayload and the single send.
func (a TokensAPI) IssueLaneClawback(ctx context.Context, clawback TokenClawbackPayload, signer Signer, onSigned OnSigned, opts ...SubmitOption) (*LaneClawbackRecord, error) {
	if err := a.c.refuseClawbackRouterCall(signer, onSigned); err != nil {
		return nil, err
	}
	return a.issueLaneClawback(ctx, clawback, clawbackConfig(opts, onSigned), signer)
}

func (c *Client) refuseClawbackRouterCall(signer Signer, onSigned OnSigned) error {
	switch {
	case signer == nil:
		return fmt.Errorf("nil signer: pass a Signer (e.g. NewPrivateKeySigner or a KMS/HSM-backed one)")
	case onSigned == nil:
		return fmt.Errorf("nil onSigned: a clawback must be stored before its send")
	case c.mode() == SubmissionModeLegacyV1:
		return fmt.Errorf("the clawback router does not take legacy v1 mode: a /v1 clawback has no AuthorizedTransaction to store before its send; use the default v2 mode, which every network of the token authority model serves")
	}
	return nil
}

func clawbackConfig(opts []SubmitOption, onSigned OnSigned) submitConfig {
	cfg := resolveSubmit(opts)
	cfg.onSigned = onSigned
	cfg.verifySigner = true
	return cfg
}

func (a TokensAPI) laneClawbackOfIntent(ctx context.Context, data TokenClawbackPayload, cfg submitConfig, signer Signer) (*ClawbackSubmission, error) {
	address := signer.Address()
	next, err := a.c.GetAccountNonce(ctx, address)
	if err != nil {
		return nil, fmt.Errorf("signer nonce read: %w", err)
	}
	if next.Nonce > data.Nonce {
		return nil, &ClawbackIntentNonceUsedError{Signer: address, Nonce: data.Nonce, NextNonce: next.Nonce}
	}
	record, err := a.issueLaneClawback(ctx, data, cfg, signer)
	if err != nil {
		return nil, err
	}
	return &ClawbackSubmission{Envelope: AuthorityEnvelopeSettled, TransactionHash: record.TransactionHash(), Lane: record}, nil
}

func (a TokensAPI) issueLaneClawback(ctx context.Context, clawback TokenClawbackPayload, cfg submitConfig, signer Signer) (*LaneClawbackRecord, error) {
	sent, err := retryOnceOnReferenceNotReady(ctx, cfg, func(cfg submitConfig) (*AuthorizedTransaction, error) {
		payload, err := a.c.LaneClawbackPayload(ctx, clawback.ChainID, clawback.Token, clawback.From, clawback.Recipient, clawback.Value)
		if err != nil {
			return nil, err
		}
		return a.c.signAndSubmitV2(ctx, payload, cfg, signer, new(TransactionResponse))
	})
	if err != nil {
		return nil, err
	}
	return NewLaneClawbackRecord(sent)
}

// FullHistoryCheck is the issuer's statement for PrepareLaneClawbackReissue.
// It is a struct, so that neither a literal true nor a boolean variable makes
// the statement: only FullHistoryChecked does. Its zero value refuses.
type FullHistoryCheck struct{ checked bool }

// FullHistoryChecked states that a node with the full history (not
// bootstrapped from a snapshot taken after the dead clawback's reference
// checkpoint, and keeping receipts that far back) shows no receipt and no
// status by hash for the dead clawback, and the holder's nonce past its slot.
var FullHistoryChecked = FullHistoryCheck{checked: true}

// LaneClawbackReissueKind is the answer of PrepareLaneClawbackReissue.
type LaneClawbackReissueKind string

const (
	// ReissuePrepared: the replacement is signed and not sent.
	ReissuePrepared LaneClawbackReissueKind = "prepared"
	// ReissueRefused: nothing was signed; Fate shows the dead clawback
	// executed, certified, its slot not passed, or known to a node.
	ReissueRefused LaneClawbackReissueKind = "refused"
	// ReissueNodeBehind: nothing was signed; the node served a holder nonce
	// that is not past the dead slot.
	ReissueNodeBehind LaneClawbackReissueKind = "node_behind"
)

// LaneClawbackReissue is the answer of PrepareLaneClawbackReissue.
type LaneClawbackReissue struct {
	Kind LaneClawbackReissueKind
	// Replacement is set for ReissuePrepared.
	Replacement *LaneClawbackRecord
	// Fate is the round of reads about the dead clawback.
	Fate LaneClawbackFate
}

// PrepareLaneClawbackReissue signs the replacement of a lane clawback that did
// not execute, on the holder's next slot with a new reference checkpoint and
// the same memo, and does not send it. This is an explicit issuer action:
//
//   - The SDK cannot tell a clawback whose slot another transaction took from
//     one that executed, on a node without its history. Pass
//     FullHistoryChecked only after that check on a node with the full history.
//   - Store the replacement before you send it with Client.Submit, and from
//     then on read the fate of the replacement, not of dead.
//   - Never prepare twice for the same dead clawback, at once or one after the
//     other: each call reads the holder's nonce and the latest checkpoint
//     again, so two calls can sign two replacements, and both can execute.
//   - Never send dead again.
//
// Errors: checked is not FullHistoryChecked, before any read; a read that did
// not answer; as Client.LaneClawbackPayload; a signing failure.
func (c *Client) PrepareLaneClawbackReissue(ctx context.Context, dead *LaneClawbackRecord, signer Signer, checked FullHistoryCheck) (*LaneClawbackReissue, error) {
	if !checked.checked {
		return nil, fmt.Errorf("a lane clawback is issued again only after a check on a node with the full history: pass FullHistoryChecked once you made it")
	}
	if dead == nil || signer == nil {
		return nil, fmt.Errorf("PrepareLaneClawbackReissue needs the dead clawback's record and a Signer")
	}
	fate, err := c.LaneClawbackFate(ctx, dead)
	if err != nil {
		return nil, err
	}
	if fate.Kind != LaneClawbackUnknown || fate.Unknown.Status != nil {
		return &LaneClawbackReissue{Kind: ReissueRefused, Fate: fate}, nil
	}
	old := dead.Payload()
	payload, err := c.LaneClawbackPayload(ctx, old.ChainID, old.Token, old.Holder, old.Recipient, old.Value)
	if err != nil {
		return nil, err
	}
	if payload.FromNonce <= old.FromNonce {
		return &LaneClawbackReissue{Kind: ReissueNodeBehind, Fate: fate}, nil
	}
	replacement, err := signLaneClawback(payload, dead.Memo(), signer)
	if err != nil {
		return nil, err
	}
	return &LaneClawbackReissue{Kind: ReissuePrepared, Replacement: replacement, Fate: fate}, nil
}

func signLaneClawback(payload LaneClawbackPayload, memo Memo, signer Signer) (*LaneClawbackRecord, error) {
	prepared, err := PrepareTransaction(payload, WithMemo(memo))
	if err != nil {
		return nil, err
	}
	sig, err := signer.SignHash(prepared.SigningHash())
	if err != nil {
		return nil, err
	}
	if err := verifySignerAddress(prepared.SigningHash(), sig, signer.Address()); err != nil {
		return nil, err
	}
	authorized, err := prepared.Authorize(sig)
	if err != nil {
		return nil, err
	}
	return NewLaneClawbackRecord(authorized)
}
