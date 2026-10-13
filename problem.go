package onemoney

import (
	"bytes"
	"encoding/json"
)

// ProblemAction is the client action of an RFC 9457 problem body: what the
// caller should do next. The node can add new values, so an Action can hold a
// value that is not one of the constants below. Use IsKnown to check.
type ProblemAction string

// The client actions that this SDK knows. See
// https://github.com/1Money-Co/l1client/blob/main/docs/api/error-handling.md
const (
	// ActionFixRequest: the request is wrong. Fix it before you send again.
	ActionFixRequest ProblemAction = "fix_request"
	// ActionRetryLater: the node did not process the request. Send it again later.
	ActionRetryLater ProblemAction = "retry_later"
	// ActionInProgress: the node already holds this transaction. Resend the same
	// signed transaction; do not sign a new one.
	ActionInProgress ProblemAction = "in_progress"
	// ActionCheckStatus: the outcome is not known. Look up the transaction
	// before you sign anything new.
	ActionCheckStatus ProblemAction = "check_status"
	// ActionUseOtherNode: this node cannot serve the request. Send it to another node.
	ActionUseOtherNode ProblemAction = "use_other_node"
)

// IsKnown reports whether a is one of the five actions above. The match is
// exact and case-sensitive: "Retry_Later" is not known.
func (a ProblemAction) IsKnown() bool {
	switch a {
	case ActionFixRequest, ActionRetryLater, ActionInProgress, ActionCheckStatus, ActionUseOtherNode:
		return true
	}
	return false
}

// problemFields are the members that Problem keeps in named fields, plus the
// legacy members that stay on ErrorResponse and APIError. They are not in
// Problem.Members.
var problemFields = map[string]bool{
	"type": true, "title": true, "status": true, "instance": true, "action": true,
	"error_code": true, "message": true,
}

// Problem holds the RFC 9457 problem details of an API error body. Decide on
// Type and Action. Title and the legacy Message are for humans only.
//
// ParseProblem fills a Problem. Each member is read on its own, so a missing
// member, an unknown type or action, or a member of an unexpected JSON type
// never makes the parse fail; the field stays empty instead. A body from a
// node without problem details gives a zero Problem.
type Problem struct {
	// Type is the problem type URI, or "about:blank". Compare the whole string.
	// Empty when the member is missing or is not a string.
	Type string `json:"-"`
	// Title is a short summary of the type. Empty when missing or not a string.
	Title string `json:"-"`
	// Instance is the server request ID, "urn:uuid:<id>". Quote it when you
	// report a problem to a node operator. Empty when missing or not a string.
	Instance string `json:"-"`
	// Action is the action member as sent when it is a string, known or not.
	// Empty when the member is missing or is not a string.
	Action ProblemAction `json:"-"`
	// RawAction is the action member as sent, of any JSON type (for example
	// null or 42). Nil when the body has no action member. For diagnostics.
	RawAction json.RawMessage `json:"-"`
	// Members holds the typed members of the problem type, such as tx_hash,
	// nonce or retry_after_secs, as sent. It does not hold the members above,
	// status, error_code or message. Nil when the body is not a JSON object.
	Members map[string]json.RawMessage `json:"-"`
}

// ParseProblem reads the problem details from an error body. It never fails:
// a body that is not a JSON object (empty, text, HTML, an array, null) gives a
// zero Problem.
func ParseProblem(body []byte) Problem {
	var members map[string]json.RawMessage
	if err := json.Unmarshal(body, &members); err != nil || members == nil {
		return Problem{}
	}
	p := Problem{
		Type:     stringMember(members, "type"),
		Title:    stringMember(members, "title"),
		Instance: stringMember(members, "instance"),
		Action:   ProblemAction(stringMember(members, "action")),
		Members:  make(map[string]json.RawMessage),
	}
	if raw, ok := members["action"]; ok {
		p.RawAction = raw
	}
	for name, raw := range members {
		if !problemFields[name] {
			p.Members[name] = raw
		}
	}
	return p
}

// stringMember returns the member name when it is a JSON string, else "".
func stringMember(members map[string]json.RawMessage, name string) string {
	var s string
	if raw, ok := members[name]; ok && json.Unmarshal(raw, &s) == nil {
		return s
	}
	return ""
}

// isJSONNull reports whether raw is the JSON literal null.
func isJSONNull(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

// HasAction reports whether the body has an action member of any value. When
// it is true and Action is not known, the node sent an action that this SDK
// cannot read. On a transaction submission, treat that as ActionCheckStatus.
func (p Problem) HasAction() bool {
	return p.RawAction != nil
}

// HasMember reports whether the body has the typed member name, of any value.
func (p Problem) HasMember(name string) bool {
	_, ok := p.Members[name]
	return ok
}

// Member decodes the typed member name into v, which must be a pointer. It
// returns false when the member is missing, is null, or does not decode into v.
func (p Problem) Member(name string, v any) bool {
	raw, ok := p.Members[name]
	if !ok || isJSONNull(raw) {
		return false
	}
	return json.Unmarshal(raw, v) == nil
}

// TxHash returns the tx_hash member, for example of
// submission_already_in_flight. ok is false when the member is missing or is
// not a string; HasTxHash tells these two cases apart.
func (p Problem) TxHash() (hash string, ok bool) {
	ok = p.Member("tx_hash", &hash)
	return hash, ok
}

// HasTxHash reports whether the body has a tx_hash member of any value.
func (p Problem) HasTxHash() bool {
	return p.HasMember("tx_hash")
}

// RetryAfterSecs returns the retry_after_secs member of a rate-limit or
// back-pressure type. ok is false when the member is missing or is not an
// unsigned 64-bit integer.
func (p Problem) RetryAfterSecs() (secs uint64, ok bool) {
	ok = p.Member("retry_after_secs", &secs)
	return secs, ok
}
