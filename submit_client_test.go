package onemoney

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func serveOnce(t *testing.T, status int, body string) *Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return NewClientWithCustomUrl(server.URL)
}

func hashOf(a *AuthorizedTransaction) common.Hash { return common.BytesToHash(a.txHash) }

func submitError(t *testing.T, err error) *SubmitError {
	t.Helper()
	var submitErr *SubmitError
	if !errors.As(err, &submitErr) {
		t.Fatalf("want *SubmitError, got %T: %v", err, err)
	}
	return submitErr
}

func TestSubmitClassifiesEveryAnswerThatDoesNotAdmitTheTransaction(t *testing.T) {
	authorized := authorizedPayment(t)
	local := hashOf(authorized)
	cases := []struct {
		name      string
		status    int
		body      string
		kind      SubmitErrorKind
		nonceUsed bool
	}{
		{"in progress", 422, `{"type":"` + inFlightType + `","action":"in_progress","tx_hash":"` + local.Hex() + `"}`, SubmitLocked, false},
		{"no-effect action", 403, `{"type":"` + ProblemTypePrefix + `compliance_blocked","action":"fix_request"}`, SubmitRejected, false},
		{"nonce too low", 422, `{"type":"` + nonceTooLowType + `","action":"check_status"}`, SubmitOutcomeUnknown, true},
		{"legacy 422", 422, `{"error_code":"business_transaction_failed","message":"already inflight"}`, SubmitOutcomeUnknown, false},
		{"firewall 403", 403, `<html>403</html>`, SubmitOutcomeUnknown, false},
		{"foreign type", 429, `{"type":"https://httpproblems.com/http-status/429","action":"retry_later"}`, SubmitOutcomeUnknown, false},
		{"another hash", 200, `{"hash":"` + anotherHash.Hex() + `"}`, SubmitHashMismatch, false},
		{"no hash", 200, `{}`, SubmitOutcomeUnknown, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := serveOnce(t, tc.status, tc.body).Submit(context.Background(), authorized)
			submitErr := submitError(t, err)
			if submitErr.Kind != tc.kind || submitErr.NonceUsed != tc.nonceUsed {
				t.Fatalf("%+v", submitErr)
			}
			if tc.kind == SubmitLocked || tc.kind == SubmitOutcomeUnknown {
				if submitErr.Transaction != authorized || submitErr.TransactionHash != local {
					t.Fatalf("the transaction to resend: %+v", submitErr)
				}
			}
			var apiErr *APIError
			if tc.kind != SubmitHashMismatch && (!errors.As(err, &apiErr) || apiErr.StatusCode != tc.status || string(apiErr.Body) != tc.body) {
				t.Fatalf("the deciding *APIError: %v", apiErr)
			}
		})
	}
}

func TestSubmitAcceptsAny2xxWithTheLocalHash(t *testing.T) {
	authorized := authorizedPayment(t)
	for _, status := range []int{200, 201, 202} {
		got, err := serveOnce(t, status, `{"hash":"`+strings.ToUpper(hashOf(authorized).Hex()[2:])+`"}`).Submit(context.Background(), authorized)
		if err != nil || got == nil {
			t.Fatalf("%d: %v", status, err)
		}
	}
}

func TestAnAcceptedBodyThatDoesNotDecodeIsOutcomeUnknown(t *testing.T) {
	authorized := authorizedPayment(t)
	client := serveOnce(t, 200, `{"hash":"`+hashOf(authorized).Hex()+`","token":5}`)
	var out IssueTokenResponse
	err := client.submitAuthorized(context.Background(), authorized, &out)
	if submitErr := submitError(t, err); submitErr.Kind != SubmitOutcomeUnknown || !strings.Contains(submitErr.Answer.Message, "did not decode") {
		t.Fatalf("%+v", submitErr)
	}
}

func TestSubmitWithAnEndedContextSendsNothing(t *testing.T) {
	sent := false
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { sent = true }))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := NewClientWithCustomUrl(server.URL).Submit(ctx, authorizedPayment(t))
	var submitErr *SubmitError
	if errors.As(err, &submitErr) || !errors.Is(err, context.Canceled) || sent {
		t.Fatalf("%v sent=%t", err, sent)
	}
}

func TestATransportFailureOfASubmitIsOutcomeUnknownAndUnwrapsTheCause(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	server.Close()
	_, err := NewClientWithCustomUrl(server.URL).Submit(context.Background(), authorizedPayment(t))
	submitErr := submitError(t, err)
	if submitErr.Kind != SubmitOutcomeUnknown || submitErr.Answer.Status != 0 || errors.Unwrap(submitErr) == nil {
		t.Fatalf("%+v", submitErr)
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		t.Fatal("no HTTP answer, no *APIError")
	}
}

func TestAGeneralRequestTakesAny2xx(t *testing.T) {
	got, err := serveOnce(t, http.StatusCreated, `{"chain_id":1212101}`).GetChainId(context.Background())
	if err != nil || got.ChainId != 1212101 {
		t.Fatalf("%+v %v", got, err)
	}
}
