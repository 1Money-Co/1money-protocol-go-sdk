package onemoney

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// TestTokensMetadataNamespace verifies the new Tokens().Metadata read routes to
// the token-metadata endpoint with the token query, and that the deprecated
// Client.GetTokenMetadata still works and hits the exact same URL.
func TestTokensMetadataNamespace(t *testing.T) {
	token := repeatAddr(0x01)
	var gotURL string
	hc := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotURL = r.URL.String()
		buf, _ := json.Marshal(map[string]string{"symbol": "USD1"})
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(buf)), Header: http.Header{}, Request: r}, nil
	})}
	c := NewClientWithCustomUrl("http://sdk.test", WithHTTPClient(hc))

	info, err := c.Tokens().Metadata(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	if info.Symbol != "USD1" {
		t.Errorf("symbol = %s, want USD1", info.Symbol)
	}
	if !strings.Contains(gotURL, "/v1/tokens/token_metadata") {
		t.Errorf("URL = %s, want the token_metadata endpoint", gotURL)
	}
	if !strings.Contains(gotURL, "token="+token.Hex()) {
		t.Errorf("URL = %s, want token=%s", gotURL, token.Hex())
	}
	namespaceURL := gotURL

	// The deprecated flat method must still work and hit the same URL.
	if _, err := c.GetTokenMetadata(context.Background(), token.Hex()); err != nil {
		t.Fatalf("deprecated GetTokenMetadata: %v", err)
	}
	if gotURL != namespaceURL {
		t.Errorf("deprecated URL %s != namespace URL %s", gotURL, namespaceURL)
	}
}
