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

// redirectingNode answers the payment POST with a 303 to target and counts
// every GET. A GET answers a node problem with fix_request: if it were read
// as the answer to the POST, it would falsely prove that the POST had no
// effect.
func redirectingNode(t *testing.T, target string) (*httptest.Server, *int) {
	t.Helper()
	gets := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			gets++
			if r.URL.Path == "/elsewhere" {
				http.Redirect(w, r, "/v2/transactions/payment", http.StatusSeeOther)
				return
			}
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"type":"` + ProblemTypePrefix + `compliance_blocked","action":"fix_request"}`))
			return
		}
		http.Redirect(w, r, target, http.StatusSeeOther)
	}))
	t.Cleanup(server.Close)
	return server, &gets
}

func TestARedirectedSubmitNeverProvesNoEffect(t *testing.T) {
	authorized := authorizedPayment(t)
	for _, target := range []string{"/v2/transactions/payment", "/elsewhere"} {
		server, gets := redirectingNode(t, target)
		_, err := NewClientWithCustomUrl(server.URL).Submit(context.Background(), authorized)
		submitErr := submitError(t, err)
		if submitErr.Kind != SubmitOutcomeUnknown || submitErr.RequestHadNoEffect() || submitErr.Answer.Status != http.StatusSeeOther || *gets != 0 {
			t.Fatalf("default client, redirect to %s: %+v, %d GETs", target, submitErr, *gets)
		}

		server, gets = redirectingNode(t, target)
		following := NewClientWithCustomUrl(server.URL, WithHTTPClient(&http.Client{}))
		_, err = following.Submit(context.Background(), authorized)
		submitErr = submitError(t, err)
		if submitErr.Kind != SubmitOutcomeUnknown || submitErr.RequestHadNoEffect() || submitErr.Transaction != authorized || *gets == 0 {
			t.Fatalf("following client, redirect to %s: %+v, %d GETs", target, submitErr, *gets)
		}
	}
}

func TestReadsDoNotFollowRedirects(t *testing.T) {
	followed := false
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chains/chain_id", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/moved", http.StatusFound)
	})
	mux.HandleFunc("/moved", func(w http.ResponseWriter, _ *http.Request) {
		followed = true
		_, _ = w.Write([]byte(`{"chain_id":1}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	_, err := NewClientWithCustomUrl(server.URL).GetChainId(context.Background())
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusFound || followed {
		t.Fatalf("%v followed=%t", err, followed)
	}
}

// A 2xx that names the local hash with "0X" is a mismatch, as in the pinned
// Rust build, which refuses that prefix.
func TestASubmitAnsweredWithAnUpperCasePrefixIsAHashMismatch(t *testing.T) {
	authorized := authorizedPayment(t)
	upperPrefix := "0X" + hashOf(authorized).Hex()[2:]
	_, err := serveOnce(t, 200, `{"hash":"`+upperPrefix+`"}`).Submit(context.Background(), authorized)
	if submitErr := submitError(t, err); submitErr.Kind != SubmitHashMismatch || submitErr.ServerHash != upperPrefix {
		t.Fatalf("%+v", submitErr)
	}
	bare := hashOf(authorized).Hex()[2:]
	if _, err := serveOnce(t, 200, `{"hash":"`+strings.ToUpper(bare)+`"}`).Submit(context.Background(), authorized); err != nil {
		t.Fatalf("bare upper-case digits: %v", err)
	}
}
