package onemoney

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Problem bodies with the shapes of the node's problem catalog (l1client
// crates/types/om-rest-types/tests/golden/problem_contract.txt).
const (
	inFlightBody = `{"type":"https://developer.1moneynetwork.com/problem/submission_already_in_flight",` +
		`"title":"Submission already in flight","status":422,"instance":"urn:uuid:0b6f3c1e-1111-4a2b-9c3d-000000000001",` +
		`"action":"in_progress","error_code":"business_transaction_failed","message":"already inflight",` +
		`"tx_hash":"0xabc123"}`
	rateLimitedBody = `{"type":"https://developer.1moneynetwork.com/problem/rate_limited","title":"Rate limited",` +
		`"status":429,"instance":"urn:uuid:1","action":"retry_later","retry_after_secs":3}`
	legacyBody = `{"error_code":"business_transaction_failed","message":"already inflight"}`
)

func TestProblemActionIsKnown(t *testing.T) {
	for _, a := range []ProblemAction{ActionFixRequest, ActionRetryLater, ActionInProgress, ActionCheckStatus, ActionUseOtherNode} {
		assert.True(t, a.IsKnown(), "%q", a)
	}
	for _, a := range []ProblemAction{"", "Retry_Later", "retry_later ", "wait"} {
		assert.False(t, a.IsKnown(), "%q", a)
	}
}

func TestParseProblem_ProblemBody(t *testing.T) {
	p := ParseProblem([]byte(inFlightBody))

	assert.Equal(t, "https://developer.1moneynetwork.com/problem/submission_already_in_flight", p.Type)
	assert.Equal(t, "Submission already in flight", p.Title)
	assert.Equal(t, "urn:uuid:0b6f3c1e-1111-4a2b-9c3d-000000000001", p.Instance)
	assert.Equal(t, ActionInProgress, p.Action)
	assert.True(t, p.HasAction())
	assert.JSONEq(t, `"in_progress"`, string(p.RawAction))
	hash, ok := p.TxHash()
	assert.True(t, ok)
	assert.Equal(t, "0xabc123", hash)
	assert.True(t, p.HasTxHash())
	// Only the typed members stay in Members.
	assert.Equal(t, []string{"tx_hash"}, memberNames(p))
}

func TestParseProblem_EachKnownAction(t *testing.T) {
	for _, a := range []ProblemAction{ActionFixRequest, ActionRetryLater, ActionInProgress, ActionCheckStatus, ActionUseOtherNode} {
		p := ParseProblem([]byte(`{"type":"about:blank","action":"` + string(a) + `"}`))
		assert.Equal(t, a, p.Action)
		assert.True(t, p.Action.IsKnown())
		assert.True(t, p.HasAction())
	}
}

func TestParseProblem_UnreadableAction(t *testing.T) {
	cases := []struct {
		name       string
		action     string
		wantAction ProblemAction
	}{
		{"unknown value", `"wait_for_epoch"`, "wait_for_epoch"},
		{"wrong case", `"Retry_Later"`, "Retry_Later"},
		{"null", `null`, ""},
		{"number", `42`, ""},
		{"object", `{"next":"retry_later"}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := ParseProblem([]byte(`{"type":"https://developer.1moneynetwork.com/problem/new_type","action":` + tc.action + `,"nonce":7}`))

			assert.Equal(t, tc.wantAction, p.Action)
			assert.False(t, p.Action.IsKnown())
			assert.True(t, p.HasAction(), "an action member of any value must give HasAction")
			assert.JSONEq(t, tc.action, string(p.RawAction), "the raw action must be kept")
			// The rest of the body still parses.
			assert.Equal(t, "https://developer.1moneynetwork.com/problem/new_type", p.Type)
			var nonce uint64
			assert.True(t, p.Member("nonce", &nonce))
			assert.Equal(t, uint64(7), nonce)
		})
	}
}

func TestParseProblem_TxHashShapes(t *testing.T) {
	cases := []struct {
		name     string
		member   string
		wantHas  bool
		wantOK   bool
		wantHash string
	}{
		{"string", `,"tx_hash":"0xdef"`, true, true, "0xdef"},
		{"number", `,"tx_hash":42`, true, false, ""},
		{"null", `,"tx_hash":null`, true, false, ""},
		{"missing", ``, false, false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := ParseProblem([]byte(`{"type":"about:blank","action":"in_progress"` + tc.member + `}`))

			hash, ok := p.TxHash()
			assert.Equal(t, tc.wantOK, ok)
			assert.Equal(t, tc.wantHash, hash)
			assert.Equal(t, tc.wantHas, p.HasTxHash())
		})
	}
}

func TestParseProblem_MissingAndWrongTypeMembers(t *testing.T) {
	p := ParseProblem([]byte(`{"type":7,"title":null,"instance":["x"],"status":"422"}`))

	assert.Equal(t, Problem{Members: map[string]json.RawMessage{}}, p)
	assert.False(t, p.HasAction())
}

func TestParseProblem_RetryAfterSecs(t *testing.T) {
	secs, ok := ParseProblem([]byte(rateLimitedBody)).RetryAfterSecs()
	assert.True(t, ok)
	assert.Equal(t, uint64(3), secs)

	for _, member := range []string{`"3"`, `-1`, `1.5`, `null`} {
		_, ok := ParseProblem([]byte(`{"retry_after_secs":` + member + `}`)).RetryAfterSecs()
		assert.False(t, ok, "retry_after_secs %s", member)
	}
}

func TestParseProblem_LegacyBody(t *testing.T) {
	p := ParseProblem([]byte(legacyBody))

	assert.Empty(t, p.Type)
	assert.Empty(t, p.Action)
	assert.False(t, p.HasAction())
	assert.Empty(t, p.Members, "error_code and message are not typed members")
}

func TestParseProblem_NotAnObject(t *testing.T) {
	bodies := []string{
		``, `   `, `"in_progress"`, `["action"]`, `null`, `42`, `true`,
		`<html><body>404 Not Found</body></html>`, `Method Not Allowed`,
		`{"action":"in_progress"`, // truncated
	}
	for _, body := range bodies {
		p := ParseProblem([]byte(body))
		assert.Equal(t, Problem{}, p, "body %q", body)
		assert.False(t, p.HasAction(), "body %q", body)
		assert.False(t, p.HasTxHash(), "body %q", body)
	}
}

func TestErrorResponseUnmarshal(t *testing.T) {
	var r ErrorResponse
	if assert.NoError(t, json.Unmarshal([]byte(inFlightBody), &r)) {
		assert.Equal(t, "business_transaction_failed", r.ErrorCode)
		assert.Equal(t, "already inflight", r.Message)
		assert.Equal(t, ActionInProgress, r.Action)
	}

	// An unreadable problem member never fails the decode.
	r = ErrorResponse{}
	if assert.NoError(t, json.Unmarshal([]byte(`{"error_code":"x","message":"m","type":1,"action":42}`), &r)) {
		assert.Equal(t, "x", r.ErrorCode)
		assert.True(t, r.HasAction())
	}

	// A wrong type in a legacy member is still an error, as before.
	assert.Error(t, json.Unmarshal([]byte(`{"message":5}`), &ErrorResponse{}))
}

// -----------------------------------------------------------------------------
// The problem details on the APIError of a request (handleAPIResponse)
// -----------------------------------------------------------------------------

func getAPIError(t *testing.T, status int, body string) *APIError {
	t.Helper()
	c, _ := newQueryClientStatus(t, status, body)
	_, err := c.GetChainId(context.Background())
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("want *APIError, got %T: %v", err, err)
	}
	return apiErr
}

func TestAPIError_ProblemBody(t *testing.T) {
	apiErr := getAPIError(t, http.StatusUnprocessableEntity, inFlightBody)

	assert.Equal(t, http.StatusUnprocessableEntity, apiErr.StatusCode)
	assert.Equal(t, "business_transaction_failed", apiErr.ErrorCode)
	assert.Equal(t, "already inflight", apiErr.Message)
	assert.Equal(t, ActionInProgress, apiErr.Action)
	assert.Equal(t, "urn:uuid:0b6f3c1e-1111-4a2b-9c3d-000000000001", apiErr.Instance)
	hash, ok := apiErr.TxHash()
	assert.True(t, ok)
	assert.Equal(t, "0xabc123", hash)
	assert.Equal(t, inFlightBody, string(apiErr.Body))
	// The error text does not change.
	assert.Equal(t, "API error: status=422, code=business_transaction_failed, message=already inflight", apiErr.Error())
}

func TestAPIError_LegacyBody(t *testing.T) {
	apiErr := getAPIError(t, http.StatusUnprocessableEntity, legacyBody)

	assert.Equal(t, "business_transaction_failed", apiErr.ErrorCode)
	assert.Equal(t, "already inflight", apiErr.Message)
	assert.Empty(t, apiErr.Type)
	assert.Empty(t, apiErr.Action)
	assert.False(t, apiErr.HasAction())
}

func TestAPIError_UnknownTypeAndAction(t *testing.T) {
	body := `{"type":"https://developer.1moneynetwork.com/problem/from_the_future","title":"New","status":409,` +
		`"action":"wait_for_epoch","error_code":"new_code","message":"m","epoch":12}`
	apiErr := getAPIError(t, http.StatusConflict, body)

	assert.Equal(t, "new_code", apiErr.ErrorCode)
	assert.Equal(t, "https://developer.1moneynetwork.com/problem/from_the_future", apiErr.Type)
	assert.Equal(t, ProblemAction("wait_for_epoch"), apiErr.Action)
	assert.False(t, apiErr.Action.IsKnown())
	var epoch uint64
	assert.True(t, apiErr.Member("epoch", &epoch))
	assert.Equal(t, uint64(12), epoch)
}

func TestAPIError_MissingMembers(t *testing.T) {
	apiErr := getAPIError(t, http.StatusBadRequest, `{"type":"about:blank"}`)

	assert.Equal(t, "about:blank", apiErr.Type)
	assert.Empty(t, apiErr.ErrorCode)
	assert.Empty(t, apiErr.Message)
	assert.Empty(t, apiErr.Title)
	assert.Empty(t, apiErr.Instance)
	assert.False(t, apiErr.HasAction())
}

// A node without problem details answered an unknown route or a wrong method
// with an empty or text body; a node with them sends a problem body.
func TestAPIError_FormerEmptyBody404And405(t *testing.T) {
	legacy404 := getAPIError(t, http.StatusNotFound, ``)
	assert.Equal(t, "unexpected status code: 404, body: ", legacy404.Message)
	assert.Equal(t, Problem{}, legacy404.Problem)

	legacy405 := getAPIError(t, http.StatusMethodNotAllowed, `Method Not Allowed`)
	assert.Equal(t, "unexpected status code: 405, body: Method Not Allowed", legacy405.Message)
	assert.False(t, legacy405.HasAction())

	problem404 := getAPIError(t, http.StatusNotFound,
		`{"type":"https://developer.1moneynetwork.com/problem/route_not_found","title":"Route not found",`+
			`"status":404,"instance":"urn:uuid:2","action":"fix_request","error_code":"route_not_found","message":"Route not found"}`)
	assert.Equal(t, "route_not_found", problem404.ErrorCode)
	assert.Equal(t, "https://developer.1moneynetwork.com/problem/route_not_found", problem404.Type)
	assert.Equal(t, ActionFixRequest, problem404.Action)

	problem405 := getAPIError(t, http.StatusMethodNotAllowed,
		`{"type":"https://developer.1moneynetwork.com/problem/method_not_allowed","status":405,"action":"fix_request"}`)
	assert.Equal(t, "https://developer.1moneynetwork.com/problem/method_not_allowed", problem405.Type)
	assert.Equal(t, ActionFixRequest, problem405.Action)
}

// A wrong type in a legacy member still gives the "unexpected status code"
// message, but the problem details are kept.
func TestAPIError_WrongTypeLegacyMemberKeepsProblem(t *testing.T) {
	apiErr := getAPIError(t, http.StatusUnprocessableEntity, `{"message":5,"action":"in_progress","tx_hash":"0x1"}`)

	assert.True(t, strings.HasPrefix(apiErr.Message, "unexpected status code: 422"))
	assert.Equal(t, ActionInProgress, apiErr.Action)
	assert.True(t, apiErr.HasTxHash())
}

// -----------------------------------------------------------------------------
// Property: parsing never fails, is deterministic, and HasAction and
// HasTxHash follow the body exactly.
// -----------------------------------------------------------------------------

func FuzzParseProblem(f *testing.F) {
	for _, seed := range []string{
		inFlightBody, rateLimitedBody, legacyBody, ``, `null`, `[]`, `"x"`, `<html></html>`,
		`{"action":null}`, `{"action":42,"tx_hash":null}`, `{"action":"Retry_Later","type":{}}`,
		`{"action":"a","action":"b"}`, `{"tx_hash":"0x1","retry_after_secs":18446744073709551615}`,
	} {
		f.Add(http.StatusUnprocessableEntity, []byte(seed))
	}
	f.Fuzz(func(t *testing.T, status int, body []byte) {
		p := ParseProblem(body)

		if again := ParseProblem(body); !reflect.DeepEqual(p, again) {
			t.Fatalf("not deterministic: %#v != %#v", p, again)
		}

		// Oracle: the body is an object when it is valid JSON that starts
		// with '{'; a member is a string when its raw value starts with '"'.
		trimmed := bytes.TrimSpace(body)
		isObject := json.Valid(body) && len(trimmed) > 0 && trimmed[0] == '{'
		var top map[string]json.RawMessage
		_ = json.Unmarshal(body, &top)
		rawAction, hasAction := top["action"]
		rawTxHash, hasTxHash := top["tx_hash"]
		if p.HasAction() != (isObject && hasAction) {
			t.Fatalf("HasAction = %v, want %v", p.HasAction(), isObject && hasAction)
		}
		if p.HasTxHash() != (isObject && hasTxHash) {
			t.Fatalf("HasTxHash = %v, want %v", p.HasTxHash(), isObject && hasTxHash)
		}
		var wantAction string
		if isJSONString(rawAction) {
			_ = json.Unmarshal(rawAction, &wantAction)
		}
		if p.Action != ProblemAction(wantAction) {
			t.Fatalf("Action = %q, want %q", p.Action, wantAction)
		}
		if _, ok := p.TxHash(); ok != isJSONString(rawTxHash) {
			t.Fatalf("TxHash ok = %v for tx_hash %s", ok, rawTxHash)
		}
		for name := range p.Members {
			if problemFields[name] {
				t.Fatalf("Members holds the named member %q", name)
			}
		}

		// Through the client: any non-200 status gives an *APIError that keeps
		// the status and the raw body.
		if status < 100 || status > 599 || status == http.StatusOK {
			return
		}
		apiErr := getAPIError(t, status, string(body))
		if apiErr.StatusCode != status || !bytes.Equal(apiErr.Body, body) {
			t.Fatalf("APIError status=%d body=%q, want %d %q", apiErr.StatusCode, apiErr.Body, status, body)
		}
		if !reflect.DeepEqual(apiErr.Problem, p) {
			t.Fatalf("APIError problem %#v != ParseProblem %#v", apiErr.Problem, p)
		}
	})
}

func isJSONString(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) > 0 && trimmed[0] == '"'
}

func memberNames(p Problem) []string {
	names := make([]string, 0, len(p.Members))
	for name := range p.Members {
		names = append(names, name)
	}
	return names
}
