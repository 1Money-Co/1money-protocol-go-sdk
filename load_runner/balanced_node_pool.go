package main

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	onemoney "github.com/1Money-Co/1money-go-sdk"
)

type NodeStats struct {
	URL         string
	Client      *onemoney.Client
	SendCount   int64
	VerifyCount int64
}

type BalancedNodePool struct {
	nodes        []NodeStats
	nodeCount    int
	sendCounter  uint64
	verifyCounter uint64
	mu           sync.RWMutex
}

func NewBalancedNodePool() *BalancedNodePool {
	return &BalancedNodePool{
		nodes: make([]NodeStats, 0),
	}
}

func (np *BalancedNodePool) AddNode(url string) error {
	np.mu.Lock()
	defer np.mu.Unlock()

	// Validate URL format
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("invalid URL format: %s (must start with http:// or https://)", url)
	}

	// For now, we'll use the default client since the SDK doesn't support custom URLs
	var client *onemoney.Client
	if strings.Contains(url, "test") {
		client = onemoney.NewTestClient()
	} else {
		client = onemoney.NewClient()
	}

	np.nodes = append(np.nodes, NodeStats{
		URL:    url,
		Client: client,
	})
	np.nodeCount = len(np.nodes)

	Logf("Added node: %s (Note: SDK currently uses default URL)\n", url)
	return nil
}

// GetNextClientForSend returns the next client for sending transactions
// Uses strict round-robin to ensure even distribution
func (np *BalancedNodePool) GetNextClientForSend() (*onemoney.Client, string, int, error) {
	np.mu.RLock()
	defer np.mu.RUnlock()

	if len(np.nodes) == 0 {
		return nil, "", 0, fmt.Errorf("no nodes available in pool")
	}

	// Strict round-robin selection
	counter := atomic.AddUint64(&np.sendCounter, 1)
	index := int((counter - 1) % uint64(len(np.nodes)))
	
	// Increment send count for this node
	atomic.AddInt64(&np.nodes[index].SendCount, 1)
	
	return np.nodes[index].Client, np.nodes[index].URL, index, nil
}

// GetNextClientForVerify returns the next client for verifying transactions
// Uses separate counter for verification to ensure even distribution
func (np *BalancedNodePool) GetNextClientForVerify() (*onemoney.Client, string, int, error) {
	np.mu.RLock()
	defer np.mu.RUnlock()

	if len(np.nodes) == 0 {
		return nil, "", 0, fmt.Errorf("no nodes available in pool")
	}

	// Strict round-robin selection for verification
	counter := atomic.AddUint64(&np.verifyCounter, 1)
	index := int((counter - 1) % uint64(len(np.nodes)))
	
	// Increment verify count for this node
	atomic.AddInt64(&np.nodes[index].VerifyCount, 1)
	
	return np.nodes[index].Client, np.nodes[index].URL, index, nil
}

func (np *BalancedNodePool) Size() int {
	np.mu.RLock()
	defer np.mu.RUnlock()
	return len(np.nodes)
}

func (np *BalancedNodePool) GetNodes() []string {
	np.mu.RLock()
	defer np.mu.RUnlock()

	urls := make([]string, len(np.nodes))
	for i, node := range np.nodes {
		urls[i] = node.URL
	}
	return urls
}

// GetNodeStats returns statistics for all nodes
func (np *BalancedNodePool) GetNodeStats() []NodeStats {
	np.mu.RLock()
	defer np.mu.RUnlock()

	stats := make([]NodeStats, len(np.nodes))
	for i, node := range np.nodes {
		stats[i] = NodeStats{
			URL:         node.URL,
			SendCount:   atomic.LoadInt64(&node.SendCount),
			VerifyCount: atomic.LoadInt64(&node.VerifyCount),
		}
	}
	return stats
}

// PrintNodeDistribution prints the distribution of requests across nodes
func (np *BalancedNodePool) PrintNodeDistribution() {
	stats := np.GetNodeStats()
	
	Logln("\n┌─────────────────── Node Distribution ───────────────────┐")
	Logln("│ Node URL                          │ Sends  │ Verifies │")
	Logln("├───────────────────────────────────┼────────┼──────────┤")
	
	totalSends := int64(0)
	totalVerifies := int64(0)
	
	for _, stat := range stats {
		totalSends += stat.SendCount
		totalVerifies += stat.VerifyCount
		
		// Truncate URL if too long
		url := stat.URL
		if len(url) > 33 {
			url = url[:30] + "..."
		}
		
		Logf("│ %-33s │ %6d │ %8d │\n", url, stat.SendCount, stat.VerifyCount)
	}
	
	Logln("├───────────────────────────────────┼────────┼──────────┤")
	Logf("│ %-33s │ %6d │ %8d │\n", "TOTAL", totalSends, totalVerifies)
	Logln("└───────────────────────────────────┴────────┴──────────┘")
}

// GetNodeURL returns the URL for a specific node index
func (np *BalancedNodePool) GetNodeURL(index int) string {
	np.mu.RLock()
	defer np.mu.RUnlock()
	
	if index < 0 || index >= len(np.nodes) {
		return "unknown"
	}
	
	// Extract just the host:port part from the URL
	url := np.nodes[index].URL
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimPrefix(url, "https://")
	return url
}