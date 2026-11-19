package onemoney

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const problemTypeAboutBlank = "about:blank"

// newAPIError is the error of an HTTP answer that is not a success.
func newAPIError(status int, body []byte) *APIError {
	var errorResp ErrorResponse
	if err := json.Unmarshal(body, &errorResp); err != nil {
		return &APIError{
			StatusCode: status,
			Message:    fmt.Sprintf("unexpected status code: %d, body: %s", status, body),
			Problem:    ParseProblem(body),
			Body:       body,
		}
	}
	return &APIError{StatusCode: status, ErrorCode: errorResp.ErrorCode, Message: errorResp.Message, Problem: errorResp.Problem, Body: body}
}

// postAnswer sends body as JSON and keeps whatever came back. The error is a
// local failure before the send.
func (client *Client) postAnswer(ctx context.Context, path string, body any) (answer, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return answer{}, fmt.Errorf("failed to marshal request: %w", err)
	}
	return client.exchange(ctx, http.MethodPost, path, data)
}

func (client *Client) exchange(ctx context.Context, method, path string, data []byte) (answer, error) {
	fullURL := client.baseHost + path
	var body io.Reader
	if data != nil {
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return answer{}, fmt.Errorf("failed to create request for %s %s: %w", method, path, err)
	}
	if data != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client.debugf("--> %s %s body=%s", method, fullURL, data)
	for _, hook := range client.hooks {
		hook.PreRequest(ctx, method, fullURL, data)
	}
	got := client.roundTrip(req)
	var hookErr error
	switch {
	case got.err != nil:
		hookErr = got.err
	case !isSuccessStatus(got.status):
		hookErr = newAPIError(got.status, got.body)
	}
	if got.err != nil && client.logger != nil {
		client.logger.Errorf("API %s request to %s failed: %v", method, fullURL, got.err)
	}
	client.debugf("<-- %s %s status=%d body=%s", method, fullURL, got.status, got.body)
	for _, hook := range client.hooks {
		hook.PostRequest(ctx, method, fullURL, got.status, got.body, hookErr)
	}
	return got, nil
}

func (client *Client) roundTrip(req *http.Request) answer {
	resp, err := client.httpclient.Do(req)
	if err != nil {
		return answer{err: err}
	}
	defer func() { _ = resp.Body.Close() }()
	// Request.Response is set only on a request that a redirect created, so it
	// also catches a redirect back to the same URL. A client from
	// WithHTTPClient can follow redirects.
	got := answer{status: resp.StatusCode, redirected: resp.Request != nil && resp.Request.Response != nil}
	got.body, got.err = io.ReadAll(resp.Body)
	return got
}
