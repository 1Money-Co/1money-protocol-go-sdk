package onemoney

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetEnvelopeModeReadsOnlyAReportedModeAsNonLegacy(t *testing.T) {
	cases := []struct {
		name   string
		reply  mockReply
		kind   EnvelopeModeKind
		reason LegacyModeReason
	}{
		{"absent", okJSON(`{"native_write_mode":"v2_only"}`), EnvelopeModeLegacy, LegacyNotReported},
		{"null", okJSON(`{"envelope_mode":null}`), EnvelopeModeLegacy, LegacyNotReported},
		{"hidden", mockReply{status: http.StatusNotFound, body: `{}`}, EnvelopeModeLegacy, LegacyStatusAnswered},
		{"server error", mockReply{status: http.StatusInternalServerError, body: `oops`}, EnvelopeModeLegacy, LegacyStatusAnswered},
		{"not json", okJSON(`<html>`), EnvelopeModeLegacy, LegacyUndecodable},
		{"not an object", okJSON(`["settled"]`), EnvelopeModeLegacy, LegacyUndecodable},
		{"gap", okJSON(`{"envelope_mode":"upgrade_gap"}`), EnvelopeModeUpgradeGap, ""},
		{"settled", okJSON(`{"envelope_mode":"settled"}`), EnvelopeModeSettled, ""},
		{"unknown", okJSON(`{"envelope_mode":"turbo"}`), EnvelopeModeUnrecognized, ""},
		{"not a string", okJSON(`{"envelope_mode":7}`), EnvelopeModeUnrecognized, ""},
		{"no content", mockReply{status: http.StatusNoContent}, EnvelopeModeLegacy, LegacyUndecodable},
		{"another 2xx reports settled", mockReply{status: http.StatusAccepted, body: `{"envelope_mode":"settled"}`}, EnvelopeModeSettled, ""},
		{"another 2xx without a mode", mockReply{status: http.StatusAccepted, body: `{}`}, EnvelopeModeLegacy, LegacyNotReported},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			node := newMockNode(t)
			node.setStatus(tc.reply)
			mode, err := node.client().GetEnvelopeMode(context.Background())
			if err != nil {
				t.Fatalf("err = %v", err)
			}
			if mode.Kind != tc.kind || mode.LegacyReason != tc.reason {
				t.Fatalf("mode = %+v, want %s/%s", mode, tc.kind, tc.reason)
			}
			if tc.reason == LegacyStatusAnswered && mode.HTTPStatus != tc.reply.status {
				t.Fatalf("HTTPStatus = %d, want %d", mode.HTTPStatus, tc.reply.status)
			}
			if tc.kind == EnvelopeModeUnrecognized && mode.Reported == "" {
				t.Fatal("an unrecognized mode keeps what the node reported")
			}
		})
	}
}

func TestGetEnvelopeModeWithNoAnswerReadsAsLegacy(t *testing.T) {
	node := newMockNode(t)
	client := node.client()
	node.server.Close()
	mode, err := client.GetEnvelopeMode(context.Background())
	if err != nil || mode.Kind != EnvelopeModeLegacy || mode.LegacyReason != LegacyNoAnswer || mode.Detail == "" {
		t.Fatalf("mode = %+v, err = %v", mode, err)
	}
}

func TestGetEnvelopeModeReturnsTheCallersCancellation(t *testing.T) {
	node := newMockNode(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := node.client().GetEnvelopeMode(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestAuthorityEnvelopeOfEachMode(t *testing.T) {
	if e, err := (EnvelopeMode{Kind: EnvelopeModeLegacy}).AuthorityEnvelope(); err != nil || e != AuthorityEnvelopeLegacy {
		t.Fatalf("legacy: %v %v", e, err)
	}
	if e, err := (EnvelopeMode{Kind: EnvelopeModeSettled}).AuthorityEnvelope(); err != nil || e != AuthorityEnvelopeSettled {
		t.Fatalf("settled: %v %v", e, err)
	}
	if _, err := (EnvelopeMode{Kind: EnvelopeModeUpgradeGap}).AuthorityEnvelope(); !errors.Is(err, ErrUpgradeGap) {
		t.Fatalf("gap: %v", err)
	}
	var unrecognized *UnrecognizedEnvelopeModeError
	if _, err := (EnvelopeMode{Kind: EnvelopeModeUnrecognized, Reported: `"turbo"`}).AuthorityEnvelope(); !errors.As(err, &unrecognized) || unrecognized.Reported != `"turbo"` {
		t.Fatalf("unrecognized: %v", err)
	}
	if _, err := (EnvelopeMode{}).AuthorityEnvelope(); err == nil {
		t.Fatal("a zero EnvelopeMode is not a reading of the node")
	}
}

func TestClawbackEnvelope(t *testing.T) {
	gap := EnvelopeMode{Kind: EnvelopeModeUpgradeGap}
	settled := EnvelopeMode{Kind: EnvelopeModeSettled}
	token := repeatAddr(0x01)
	if e, err := gap.ClawbackEnvelope(VerifyTokenAddress, ClawbackFromAccount); err != nil || e != AuthorityEnvelopeLegacy {
		t.Fatalf("VERIFY in the gap: %v %v", e, err)
	}
	if e, err := settled.ClawbackEnvelope(token, ClawbackFromHold); err != nil || e != AuthorityEnvelopeLegacy {
		t.Fatalf("hold when settled: %v %v", e, err)
	}
	if e, err := settled.ClawbackEnvelope(token, ClawbackFromAccount); err != nil || e != AuthorityEnvelopeSettled {
		t.Fatalf("account when settled: %v %v", e, err)
	}
	if _, err := gap.ClawbackEnvelope(token, ClawbackFromAccount); !errors.Is(err, ErrUpgradeGap) {
		t.Fatalf("account in the gap: %v", err)
	}
	if !ClawbackStaysLegacy(VerifyTokenAddress, ClawbackFromAccount) || !ClawbackStaysLegacy(token, ClawbackFromHold) || ClawbackStaysLegacy(token, ClawbackFromAccount) {
		t.Fatal("ClawbackStaysLegacy")
	}
}

func TestNativeWriteStatusCarriesTheEnvelopeMode(t *testing.T) {
	node := newMockNode(t)
	node.setStatus(okJSON(`{"native_write_mode":"v2_only","envelope_mode":"upgrade_gap"}`))
	status, err := node.client().GetNativeWriteStatus(context.Background())
	if err != nil || status.EnvelopeMode == nil || *status.EnvelopeMode != "upgrade_gap" {
		t.Fatalf("status = %+v, err = %v", status, err)
	}
}

func TestGetEnvelopeModeWithABodyThatCannotBeReadReadsAsNoAnswer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"envelope`))
	}))
	defer server.Close()
	mode, err := NewClientWithCustomUrl(server.URL).GetEnvelopeMode(context.Background())
	if err != nil || mode.Kind != EnvelopeModeLegacy || mode.LegacyReason != LegacyNoAnswer {
		t.Fatalf("mode = %+v, err = %v", mode, err)
	}
}
