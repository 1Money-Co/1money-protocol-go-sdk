package main

// IMPORTANT NOTE:
// The current 1money-go-sdk doesn't support custom API URLs directly.
// To properly implement multi-node support, the SDK needs to be modified to:
// 1. Add a WithBaseURL option or
// 2. Export the newClientInternal function or
// 3. Add a NewClientWithURL constructor
//
// This implementation shows the intended architecture, but currently all
// clients will use the default SDK URL (http://127.0.0.1:18555).
//
// To make this work properly, you would need to modify the SDK's 1money.go file.

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	onemoney "github.com/1Money-Co/1money-go-sdk"
)

type NodeInfo struct {
	URL    string
	Client *onemoney.Client
}

type NodePool struct {
	nodes   []NodeInfo
	counter uint64
	mu      sync.RWMutex
}

func NewNodePool() *NodePool {
	return &NodePool{
		nodes: make([]NodeInfo, 0),
	}
}

func (np *NodePool) AddNode(url string) error {
	np.mu.Lock()
	defer np.mu.Unlock()

	// Validate URL format
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("invalid URL format: %s (must start with http:// or https://)", url)
	}

	// For now, we'll use the default client since the SDK doesn't support custom URLs
	// In a real implementation, you would need to modify the SDK or use a different approach
	// This is a placeholder that demonstrates the intended functionality
	var client *onemoney.Client
	if strings.Contains(url, "test") {
		client = onemoney.NewTestClient()
	} else {
		client = onemoney.NewClient()
	}

	np.nodes = append(np.nodes, NodeInfo{
		URL:    url,
		Client: client,
	})

	Logf("Added node: %s (Note: SDK currently uses default URL)\n", url)
	return nil
}

func (np *NodePool) GetNextClient() (*onemoney.Client, string, error) {
	np.mu.RLock()
	defer np.mu.RUnlock()

	if len(np.nodes) == 0 {
		return nil, "", fmt.Errorf("no nodes available in pool")
	}

	// Round-robin selection
	index := atomic.AddUint64(&np.counter, 1) % uint64(len(np.nodes))
	node := np.nodes[index]

	return node.Client, node.URL, nil
}

func (np *NodePool) Size() int {
	np.mu.RLock()
	defer np.mu.RUnlock()
	return len(np.nodes)
}

func (np *NodePool) GetNodes() []string {
	np.mu.RLock()
	defer np.mu.RUnlock()

	urls := make([]string, len(np.nodes))
	for i, node := range np.nodes {
		urls[i] = node.URL
	}
	return urls
}

// ParseNodeURLs parses comma-separated node URLs
// Minimum: 1 node, Maximum: 13 nodes
// Supports both scenarios:
// 1. Same domain/IP with different ports: "192.168.1.1:8080,192.168.1.1:8081,192.168.1.1:8082"
// 2. Different domains/IPs: "node1.com:8080,node2.com:8080,192.168.1.10:9000"
// 3. Mixed: "localhost:8080,localhost:8081,node2.com:8080,192.168.1.1:9000"
func ParseNodeURLs(nodeList string) ([]string, error) {
	if nodeList == "" {
		return nil, fmt.Errorf("node list cannot be empty")
	}

	// Split by comma and trim spaces
	parts := strings.Split(nodeList, ",")
	urls := make([]string, 0, len(parts))
	uniqueURLs := make(map[string]bool)

	for _, part := range parts {
		url := strings.TrimSpace(part)
		if url != "" {
			// Add http:// if no protocol specified
			if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
				url = "http://" + url
			}
			
			// Check for duplicates
			if uniqueURLs[url] {
				return nil, fmt.Errorf("duplicate node URL found: %s", url)
			}
			
			// Validate URL has host and port
			if !strings.Contains(url, ":") || strings.Count(url, ":") < 2 {
				return nil, fmt.Errorf("invalid URL format: %s (must include port, e.g., host:port)", part)
			}
			
			uniqueURLs[url] = true
			urls = append(urls, url)
		}
	}

	if len(urls) < 1 {
		return nil, fmt.Errorf("at least 1 node is required, got %d", len(urls))
	}

	if len(urls) > 13 {
		return nil, fmt.Errorf("maximum 13 nodes are allowed, got %d", len(urls))
	}

	return urls, nil
}