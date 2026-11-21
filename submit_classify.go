package onemoney

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/ethereum/go-ethereum/common"
)

// The actions that prove a request had no effect: the node refused it before
// admission.
var noEffectActions = map[string]bool{
	string(ActionFixRequest): true, string(ActionRetryLater): true, string(ActionUseOtherNode): true,
}

// bytesRefusalTypes are refusals of the signed bytes themselves. Each comes
// only from a check of the bytes and of fixed chain constants that every node
// release with v2 submits (v0.8.3 on) has, so bytes that an older node
// admitted cannot get one. Left out: types that a check of node state or
// governance settings also gives (transaction_too_large,
// batch_payment_invalid), and checks added after v0.8.3.
var bytesRefusalTypes = map[string]bool{
	ProblemTypePrefix + "envelope_signature_invalid": true,
	ProblemTypePrefix + "signature_pairing_invalid":  true,
	ProblemTypePrefix + "chain_id_mismatch":          true,
	ProblemTypePrefix + "sender_not_allowed":         true,
	ProblemTypePrefix + "memo_invalid":               true,
}

const (
	problemTypeNonceTooLow = ProblemTypePrefix + "nonce_too_low"
	summaryBodyRunes       = 200
	redirectedPrefix       = "the answer came after an HTTP redirect: "
)

// answer is what one request got back, before any mapping to an error.
type answer struct {
	// status is 0 when no HTTP answer arrived.
	status int
	body   []byte
	// redirected: the HTTP client followed a redirect, so the answer can come
	// from another request (a POST that gets a 303 is repeated as a GET).
	redirected bool
	// err is set when no complete HTTP answer arrived: timeout, connection,
	// DNS, or a body read that failed (then status is set).
	err error
}

type submitOutcomeKind int

const (
	outcomeAccepted submitOutcomeKind = iota
	outcomeRejected
	outcomeLocked
	outcomeUnknown
)

func (k submitOutcomeKind) String() string {
	return [...]string{"accepted", "rejected", "locked", "unknown"}[k]
}

// submitOutcome is one submit answer, classified by the state it proves.
type submitOutcome struct {
	kind submitOutcomeKind
	// status, body and serverHash are set for outcomeAccepted: the node
	// admitted a transaction, and the caller checks the hash.
	status     int
	body       []byte
	serverHash string
	// detail is set for every other kind.
	detail  answerDetail
	refusal SubmitRefusal
	// nonceUsed: nonce_too_low, an unknown outcome.
	nonceUsed bool
}

// SubmitRefusal is what a rejected submit refused.
type SubmitRefusal string

const (
	// SubmitRefusalBytes: the signed bytes themselves. Every node refuses them,
	// so no send of them was ever admitted and none will be. Retire them.
	SubmitRefusalBytes SubmitRefusal = "bytes"
	// SubmitRefusalRequest: this request only. The same signed bytes can still
	// be admitted later or by another node.
	SubmitRefusalRequest SubmitRefusal = "request"
)

// answerDetail is an answer that is not the success the request wanted.
type answerDetail struct {
	summary AnswerSummary
	// apiErr is nil when no HTTP answer arrived.
	apiErr *APIError
	// cause is the transport failure, when no HTTP answer arrived.
	cause error
}

// bodyFields are the members that the rules decide on, read from a top-level
// JSON object only.
type bodyFields struct {
	hash      *string
	typeURI   string
	action    json.RawMessage
	txHash    json.RawMessage
	errorCode string
	message   *string
	title     *string
}

func readBodyFields(body []byte) bodyFields {
	var object map[string]json.RawMessage
	if json.Unmarshal(body, &object) != nil || object == nil {
		return bodyFields{}
	}
	fields := bodyFields{
		hash:      optionalString(object, "hash"),
		typeURI:   stringMember(object, "type"),
		txHash:    object["tx_hash"],
		errorCode: stringMember(object, "error_code"),
		message:   optionalString(object, "message"),
		title:     optionalString(object, "title"),
	}
	if isNodeStatement(object) {
		fields.action = object["action"]
	}
	return fields
}

// isNodeStatement reports whether the body's type is the node's own, so its
// action is the node's. A proxy, a firewall or a load balancer in front of
// the node can send problem details too; their members decide nothing.
func isNodeStatement(object map[string]json.RawMessage) bool {
	var typeURI string
	if raw, ok := object["type"]; !ok || json.Unmarshal(raw, &typeURI) != nil {
		return false
	}
	return hasNodeOrigin(typeURI)
}

func hasNodeOrigin(typeURI string) bool {
	return strings.HasPrefix(typeURI, ProblemTypePrefix) || typeURI == problemTypeAboutBlank
}

func optionalString(object map[string]json.RawMessage, name string) *string {
	var s string
	if raw, ok := object[name]; ok && !isJSONNull(raw) && json.Unmarshal(raw, &s) == nil {
		return &s
	}
	return nil
}

func (f bodyFields) actionString() (string, bool) {
	var action string
	if f.action == nil || json.Unmarshal(f.action, &action) != nil {
		return "", false
	}
	return action, true
}

// isInProgressFor: in_progress with no tx_hash, or a tx_hash equal to local.
func (f bodyFields) isInProgressFor(local common.Hash) bool {
	if action, ok := f.actionString(); !ok || action != string(ActionInProgress) {
		return false
	}
	if f.txHash == nil {
		return true
	}
	var txHash string
	if json.Unmarshal(f.txHash, &txHash) != nil {
		return false
	}
	parsed, ok := parseHash(txHash)
	return ok && parsed == local
}

// parseHash is the one rule for a hash in a node answer: 32 bytes of hex, in
// any case, with "0x", "0X" or no prefix, as the Rust SDK decodes a B256.
func parseHash(s string) (common.Hash, bool) {
	if len(s) >= 2 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X') {
		s = s[2:]
	}
	decoded, err := hex.DecodeString(s)
	if err != nil || len(decoded) != common.HashLength {
		return common.Hash{}, false
	}
	return common.BytesToHash(decoded), true
}

// classifySubmit classifies the answer to a submit of the transaction local.
// Only a no-effect action of the node proves rejection; a missing or unusable
// action, the HTTP status, a legacy code and a message decide nothing.
func classifySubmit(a answer, local common.Hash) submitOutcome {
	if a.err != nil {
		return submitOutcome{kind: outcomeUnknown, detail: transportDetail(a)}
	}
	fields := readBodyFields(a.body)
	if a.redirected {
		detail := httpDetail(a, fields)
		detail.summary.Message = redirectedPrefix + detail.summary.Message
		return submitOutcome{kind: outcomeUnknown, detail: detail}
	}
	if isSuccessStatus(a.status) && fields.hash != nil {
		return submitOutcome{kind: outcomeAccepted, status: a.status, body: a.body, serverHash: *fields.hash}
	}
	outcome := submitOutcome{kind: outcomeUnknown, detail: httpDetail(a, fields), nonceUsed: fields.typeURI == problemTypeNonceTooLow}
	if action, ok := fields.actionString(); ok && noEffectActions[action] {
		outcome.kind, outcome.nonceUsed, outcome.refusal = outcomeRejected, false, SubmitRefusalRequest
		if bytesRefusalTypes[fields.typeURI] {
			outcome.refusal = SubmitRefusalBytes
		}
	} else if fields.isInProgressFor(local) {
		outcome.kind, outcome.nonceUsed = outcomeLocked, false
	}
	return outcome
}

// checkServerHash: the node admitted a transaction, an error unless under local.
func checkServerHash(local common.Hash, server string) error {
	if parsed, ok := parseHash(server); ok && parsed == local {
		return nil
	}
	return &SubmitError{Kind: SubmitHashMismatch, TransactionHash: local, ServerHash: server}
}

func httpDetail(a answer, fields bodyFields) answerDetail {
	message := ""
	switch {
	case fields.message != nil:
		message = *fields.message
	case fields.title != nil:
		message = *fields.title
	default:
		message = bodyPrefix(a.body)
	}
	return answerDetail{
		summary: AnswerSummary{Status: a.status, ErrorCode: fields.errorCode, Message: message},
		apiErr:  newAPIError(a.status, a.body),
	}
}

func bodyPrefix(body []byte) string {
	trimmed := []rune(string(bytes.TrimSpace(body)))
	if len(trimmed) == 0 {
		return "empty response body"
	}
	if len(trimmed) > summaryBodyRunes {
		trimmed = trimmed[:summaryBodyRunes]
	}
	return string(trimmed)
}

func transportDetail(a answer) answerDetail {
	var netErr net.Error
	var opErr *net.OpError
	kind := "request failed"
	switch {
	case errors.Is(a.err, context.DeadlineExceeded) || (errors.As(a.err, &netErr) && netErr.Timeout()):
		kind = "request timed out"
	case errors.As(a.err, &opErr) && opErr.Op == "dial":
		kind = "connection failed"
	case a.status != 0:
		kind = "body read failed"
	}
	return answerDetail{
		summary: AnswerSummary{Status: a.status, Message: fmt.Sprintf("%s: %v", kind, a.err)},
		cause:   a.err,
	}
}
