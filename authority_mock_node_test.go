package onemoney

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

// mockReply is one answer of the mock node.
type mockReply struct {
	status int
	body   string
}

func okJSON(body string) mockReply { return mockReply{status: http.StatusOK, body: body} }

func problemReply(status int, problemType, action string, members map[string]any) mockReply {
	body := map[string]any{
		"type":   ProblemTypePrefix + problemType,
		"title":  problemType,
		"status": status,
		"action": action,
	}
	for k, v := range members {
		body[k] = v
	}
	raw, _ := json.Marshal(body)
	return mockReply{status: status, body: string(raw)}
}

var notFound = problemReply(http.StatusNotFound, "transaction_not_found", "check_status", nil)

// mockNode is a node of the release that answers the reads and writes of the
// token authority model. Queues answer in order and repeat their last entry.
// Every POST that it accepts answers with the hash rebuilt from its body.
type mockNode struct {
	t      *testing.T
	server *httptest.Server

	mu          sync.Mutex
	status      []mockReply
	checkpoints []uint64
	nonces      map[common.Address][]uint64
	private     bool
	receipts    map[string]mockReply
	statuses    map[string]mockReply
	posts       map[string][]mockReply
	events      []string
	posted      []postedRequest
}

type postedRequest struct {
	path string
	body map[string]any
	hash string
}

func newMockNode(t *testing.T) *mockNode {
	t.Helper()
	n := &mockNode{
		t:           t,
		status:      []mockReply{okJSON(`{"envelope_mode":"settled"}`)},
		checkpoints: []uint64{100},
		nonces:      map[common.Address][]uint64{},
		receipts:    map[string]mockReply{},
		statuses:    map[string]mockReply{},
		posts:       map[string][]mockReply{},
	}
	n.server = httptest.NewServer(http.HandlerFunc(n.serve))
	t.Cleanup(n.server.Close)
	return n
}

func (n *mockNode) client(opts ...ClientOption) *Client {
	return NewClientWithCustomUrl(n.server.URL, opts...)
}

// note records an event of the test, such as a store, in the same order as
// the node's requests.
func (n *mockNode) note(event string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.events = append(n.events, event)
}

func (n *mockNode) eventLog() []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]string(nil), n.events...)
}

func (n *mockNode) postedRequests() []postedRequest {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]postedRequest(nil), n.posted...)
}

func next[T any](queue *[]T) T {
	head := (*queue)[0]
	if len(*queue) > 1 {
		*queue = (*queue)[1:]
	}
	return head
}

func (n *mockNode) serve(w http.ResponseWriter, r *http.Request) {
	n.mu.Lock()
	defer n.mu.Unlock()
	reply := n.answer(r)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(reply.status)
	_, _ = io.WriteString(w, reply.body)
}

func (n *mockNode) answer(r *http.Request) mockReply {
	query := r.URL.Query()
	n.events = append(n.events, r.Method+" "+r.URL.Path)
	switch {
	case r.Method == http.MethodPost:
		return n.acceptPost(r)
	case r.URL.Path == "/api/status":
		return next(&n.status)
	case r.URL.Path == endpointCheckpointsNumber:
		return okJSON(fmt.Sprintf(`{"number":%d}`, next(&n.checkpoints)))
	case r.URL.Path == endpointAccountsNonce:
		queue, ok := n.nonces[common.HexToAddress(query.Get("address"))]
		if !ok {
			return okJSON(`{"nonce":0}`)
		}
		nonce := next(&queue)
		n.nonces[common.HexToAddress(query.Get("address"))] = queue
		return okJSON(fmt.Sprintf(`{"nonce":%d}`, nonce))
	case r.URL.Path == endpointTokensMetadata:
		return okJSON(fmt.Sprintf(`{"is_private":%t}`, n.private))
	case r.URL.Path == endpointTransactionsReceiptByHashV1:
		return replyOr(n.receipts, query.Get("hash"))
	case r.URL.Path == endpointTransactionsStatusByHashV1:
		return replyOr(n.statuses, query.Get("hash"))
	}
	n.t.Errorf("mock node: unexpected %s %s", r.Method, r.URL)
	return mockReply{status: http.StatusTeapot, body: `{}`}
}

func replyOr(replies map[string]mockReply, hash string) mockReply {
	if reply, ok := replies[strings.ToLower(hash)]; ok {
		return reply
	}
	return notFound
}

func (n *mockNode) acceptPost(r *http.Request) mockReply {
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		n.t.Errorf("mock node: read body: %v", err)
	}
	var body map[string]any
	_ = json.Unmarshal(raw, &body)
	request := postedRequest{path: r.URL.Path, body: body}
	if authorized, err := authorizedFromBody(r.URL.Path, raw); err == nil {
		request.hash = hexLower(authorized.txHash)
	} else {
		n.t.Errorf("mock node: %s body does not rebuild: %v", r.URL.Path, err)
	}
	n.posted = append(n.posted, request)
	if queue := n.posts[r.URL.Path]; len(queue) > 0 {
		reply := queue[0]
		n.posts[r.URL.Path] = queue[1:]
		if reply.status != 0 {
			return reply
		}
	}
	return okJSON(fmt.Sprintf(`{"hash":%q}`, request.hash))
}

func (n *mockNode) setStatus(replies ...mockReply) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.status = replies
}

func (n *mockNode) setNonces(account common.Address, nonces ...uint64) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.nonces[account] = nonces
}

func (n *mockNode) setCheckpoints(numbers ...uint64) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.checkpoints = numbers
}

// queuePost queues answers for the next POSTs to path. A zero mockReply
// accepts the request.
func (n *mockNode) queuePost(path string, replies ...mockReply) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.posts[path] = append(n.posts[path], replies...)
}

func (n *mockNode) setReceipt(hash string, reply mockReply) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.receipts[strings.ToLower(hash)] = reply
}

func (n *mockNode) setTxStatus(hash string, reply mockReply) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.statuses[strings.ToLower(hash)] = reply
}

func (n *mockNode) countEvents(event string) int {
	count := 0
	for _, e := range n.eventLog() {
		if e == event {
			count++
		}
	}
	return count
}
