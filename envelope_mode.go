package onemoney

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
)

// EnvelopeModeKind is what a node's GET /api/status says about the envelopes of
// the token authority operations that it takes.
type EnvelopeModeKind string

const (
	// EnvelopeModeLegacy: the node does not positively report a mode, so the
	// client sends the legacy envelopes. A node of the release refuses a legacy
	// envelope at admission and takes no nonce and no lock for it, so this
	// reading can cost a refused request but never a stuck one.
	EnvelopeModeLegacy EnvelopeModeKind = "legacy"
	// EnvelopeModeUpgradeGap: the node refuses the legacy envelopes and does not
	// take the new ones yet. Send nothing of the class; retry later.
	EnvelopeModeUpgradeGap EnvelopeModeKind = "upgrade_gap"
	// EnvelopeModeSettled: the node takes the new envelopes, for good.
	EnvelopeModeSettled EnvelopeModeKind = "settled"
	// EnvelopeModeUnrecognized: the node reports a mode that this SDK does not
	// know. Send nothing of the class, and upgrade the SDK.
	EnvelopeModeUnrecognized EnvelopeModeKind = "unrecognized"
)

// LegacyModeReason says why a node reads as EnvelopeModeLegacy.
type LegacyModeReason string

const (
	// LegacyNotReported: the status has no envelope_mode member, or a null one
	// (a node of the release before).
	LegacyNotReported LegacyModeReason = "not_reported"
	// LegacyStatusAnswered: GET /api/status answered a non-200 status. It is an
	// operational endpoint, so a gateway may not expose it; then the by-mode
	// calls send the legacy envelope, and a node of the release refuses it as
	// retired. Name the envelope explicitly in that case.
	LegacyStatusAnswered LegacyModeReason = "status_answered"
	// LegacyNoAnswer: GET /api/status got no answer.
	LegacyNoAnswer LegacyModeReason = "no_answer"
	// LegacyUndecodable: the status body is not a JSON object.
	LegacyUndecodable LegacyModeReason = "undecodable"
)

// EnvelopeMode is one reading of a node's envelope mode. Each node reports its
// own: nodes open their gates at different moments, within about a minute.
type EnvelopeMode struct {
	Kind EnvelopeModeKind
	// LegacyReason is set when Kind is EnvelopeModeLegacy.
	LegacyReason LegacyModeReason
	// HTTPStatus is set for LegacyStatusAnswered.
	HTTPStatus int
	// Detail describes a LegacyNoAnswer or LegacyUndecodable reading.
	Detail string
	// Reported is the envelope_mode member as sent, for EnvelopeModeUnrecognized.
	Reported string
}

// AuthorityEnvelope is the envelope that a client sends for an authority
// operation.
type AuthorityEnvelope string

const (
	AuthorityEnvelopeLegacy  AuthorityEnvelope = "legacy"
	AuthorityEnvelopeSettled AuthorityEnvelope = "settled"
)

// ErrUpgradeGap: the node is in the upgrade gap, so nothing of the class was
// sent. Retry later.
var ErrUpgradeGap = errors.New("the node is in the upgrade gap of the token authority model: nothing was sent; retry later")

// UnrecognizedEnvelopeModeError: the node reports an envelope mode that this
// SDK does not know, so nothing was sent. Upgrade the SDK.
type UnrecognizedEnvelopeModeError struct{ Reported string }

func (e *UnrecognizedEnvelopeModeError) Error() string {
	return fmt.Sprintf("the node reports the envelope mode %s, which this SDK does not know: nothing was sent; upgrade the SDK", e.Reported)
}

// AuthorityEnvelope returns the envelope of an authority operation in this mode.
// Errors: ErrUpgradeGap; *UnrecognizedEnvelopeModeError.
func (m EnvelopeMode) AuthorityEnvelope() (AuthorityEnvelope, error) {
	switch m.Kind {
	case EnvelopeModeLegacy:
		return AuthorityEnvelopeLegacy, nil
	case EnvelopeModeSettled:
		return AuthorityEnvelopeSettled, nil
	case EnvelopeModeUpgradeGap:
		return "", ErrUpgradeGap
	case EnvelopeModeUnrecognized:
		return "", &UnrecognizedEnvelopeModeError{Reported: m.Reported}
	}
	return "", fmt.Errorf("envelope mode %q is not a reading of a node: use Client.GetEnvelopeMode", m.Kind)
}

// ClawbackSource is the source of a clawback.
type ClawbackSource int

const (
	// ClawbackFromAccount: an ordinary account, frozen in the token.
	ClawbackFromAccount ClawbackSource = iota
	// ClawbackFromHold: a Reserved Payment hold, whose clawback stays the legacy
	// clawback in every mode.
	ClawbackFromHold
)

// ClawbackStaysLegacy reports whether a clawback of token from source is the
// legacy clawback in every envelope mode: a clawback of VERIFY by its master,
// and a clawback of a Reserved Payment hold. Its envelope needs no mode read.
func ClawbackStaysLegacy(token common.Address, source ClawbackSource) bool {
	return IsVerifyToken(token) || source == ClawbackFromHold
}

// ClawbackEnvelope returns the envelope of a clawback of token from source in
// this mode: legacy when ClawbackStaysLegacy, else AuthorityEnvelope.
func (m EnvelopeMode) ClawbackEnvelope(token common.Address, source ClawbackSource) (AuthorityEnvelope, error) {
	if ClawbackStaysLegacy(token, source) {
		return AuthorityEnvelopeLegacy, nil
	}
	return m.AuthorityEnvelope()
}

// GetEnvelopeMode reads the node's envelope mode from GET /api/status. Every
// outcome other than a reported upgrade_gap or settled reads as
// EnvelopeModeLegacy with its reason, and a reported value that this SDK does
// not know reads as EnvelopeModeUnrecognized.
//
// Errors: only the cancellation or deadline of ctx.
func (c *Client) GetEnvelopeMode(ctx context.Context) (EnvelopeMode, error) {
	var body json.RawMessage
	err := c.GetMethod(ctx, endpointStatus, &body)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return EnvelopeMode{}, ctxErr
	}
	var apiErr *APIError
	var syntaxErr *json.SyntaxError
	switch {
	case errors.As(err, &apiErr):
		return EnvelopeMode{Kind: EnvelopeModeLegacy, LegacyReason: LegacyStatusAnswered, HTTPStatus: apiErr.StatusCode}, nil
	case errors.As(err, &syntaxErr):
		return EnvelopeMode{Kind: EnvelopeModeLegacy, LegacyReason: LegacyUndecodable, Detail: err.Error()}, nil
	case err != nil:
		return EnvelopeMode{Kind: EnvelopeModeLegacy, LegacyReason: LegacyNoAnswer, Detail: err.Error()}, nil
	}
	return envelopeModeFromStatus(body), nil
}

func envelopeModeFromStatus(body json.RawMessage) EnvelopeMode {
	var members map[string]json.RawMessage
	if err := json.Unmarshal(body, &members); err != nil || members == nil {
		detail := "the status body is not a JSON object"
		if err != nil {
			detail = err.Error()
		}
		return EnvelopeMode{Kind: EnvelopeModeLegacy, LegacyReason: LegacyUndecodable, Detail: detail}
	}
	reported, ok := members[envelopeModeMember]
	if !ok || isJSONNull(reported) {
		return EnvelopeMode{Kind: EnvelopeModeLegacy, LegacyReason: LegacyNotReported}
	}
	var mode string
	if json.Unmarshal(reported, &mode) == nil {
		switch EnvelopeModeKind(mode) {
		case EnvelopeModeUpgradeGap:
			return EnvelopeMode{Kind: EnvelopeModeUpgradeGap}
		case EnvelopeModeSettled:
			return EnvelopeMode{Kind: EnvelopeModeSettled}
		}
	}
	return EnvelopeMode{Kind: EnvelopeModeUnrecognized, Reported: string(reported)}
}

const (
	endpointStatus     = "/api/status"
	envelopeModeMember = "envelope_mode"
)
