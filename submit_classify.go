package onemoney

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
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
	// action is the action member as sent, only in a body whose type is the
	// node's or about:blank.
	action json.RawMessage
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

// parseHash reads 32 bytes of hex, with or without 0x, in any case.
func parseHash(s string) (common.Hash, bool) {
	decoded, err := hex.DecodeString(strings.TrimPrefix(s, "0x"))
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

type readKind int

const (
	// readFound: a 2xx body for the requested hash.
	readFound readKind = iota
	readNotFound
	// readThrottled: 429, or the node's action retry_later. Back off.
	readThrottled
	// readFailed: any other answer, a body that does not decode, another hash,
	// or no answer.
	readFailed
	// readUnreadable: a 2xx receipt for the requested hash that this SDK
	// cannot decode, for example from a newer node: the receipt exists.
	readUnreadable
)

func (k readKind) String() string {
	return [...]string{"found", "not_found", "throttled", "failed", "unreadable"}[k]
}

type readResult[T any] struct {
	kind   readKind
	value  T
	detail answerDetail
}

// readReceipt reads the answer to GET receipt by hash.
func readReceipt(a answer, hash common.Hash) readResult[*TransactionReceiptResponse] {
	result := read(a, func(body []byte) (*TransactionReceiptResponse, bool) {
		receipt := new(TransactionReceiptResponse)
		if json.Unmarshal(body, receipt) != nil {
			return nil, false
		}
		parsed, ok := parseHash(receipt.TransactionHash)
		return receipt, ok && parsed == hash
	})
	if result.kind == readFailed && namesReceiptOf(a, hash) {
		result.kind = readUnreadable
	}
	return result
}

// namesReceiptOf: a 2xx answer whose body names the receipt of hash, whatever
// its other members hold.
func namesReceiptOf(a answer, hash common.Hash) bool {
	var receipt struct {
		TransactionHash string `json:"transaction_hash"`
	}
	if a.err != nil || !isSuccessStatus(a.status) || json.Unmarshal(a.body, &receipt) != nil {
		return false
	}
	parsed, ok := parseHash(receipt.TransactionHash)
	return ok && parsed == hash
}

// readTransaction reads the answer to GET transaction by hash. Only the hash
// is decoded, so a transaction that this SDK cannot fully decode is found.
func readTransaction(a answer, hash common.Hash) readResult[struct{}] {
	return read(a, func(body []byte) (struct{}, bool) {
		var transaction struct {
			Hash string `json:"hash"`
		}
		if json.Unmarshal(body, &transaction) != nil {
			return struct{}{}, false
		}
		parsed, ok := parseHash(transaction.Hash)
		return struct{}{}, ok && parsed == hash
	})
}

// readStatus reads the answer to GET status by hash. A status that this SDK
// does not know is a failed read.
func readStatus(a answer, hash common.Hash) readResult[TransactionStatus] {
	return read(a, func(body []byte) (TransactionStatus, bool) {
		var status TransactionStatusResponse
		if json.Unmarshal(body, &status) != nil || !status.Status.IsKnown() {
			return "", false
		}
		parsed, ok := parseHash(status.Hash)
		return status.Status, ok && parsed == hash
	})
}

func read[T any](a answer, found func([]byte) (T, bool)) readResult[T] {
	if a.err != nil {
		return readResult[T]{kind: readFailed, detail: transportDetail(a)}
	}
	if isSuccessStatus(a.status) {
		if value, ok := found(a.body); ok {
			return readResult[T]{kind: readFound, value: value}
		}
	}
	detail := httpDetail(a, readBodyFields(a.body))
	switch {
	case isThrottled(detail):
		return readResult[T]{kind: readThrottled, detail: detail}
	case a.status == http.StatusNotFound:
		return readResult[T]{kind: readNotFound, detail: detail}
	}
	return readResult[T]{kind: readFailed, detail: detail}
}

// isThrottled: 429, or the node's action retry_later. The node or the firewall
// turned the request away for now.
func isThrottled(detail answerDetail) bool {
	var action string
	return isRateLimited(detail) || (detail.action != nil && json.Unmarshal(detail.action, &action) == nil && action == string(ActionRetryLater))
}

// isRateLimited: a 429 of the node or the firewall, which holds for every
// client of that node.
func isRateLimited(detail answerDetail) bool {
	return detail.summary.Status == http.StatusTooManyRequests
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
		action:  fields.action,
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
