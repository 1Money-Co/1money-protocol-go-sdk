package main

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"

	onemoney "github.com/1Money-Co/1money-go-sdk"
)

// NodeInfo represents information about a single node
type NodeInfo struct {
	URL           string
	Client        *onemoney.Client
	MintCount     int64
	TransferCount int64
	GetCount      int64
}

// NodePool manages multiple nodes with round-robin distribution
type NodePool struct {
	nodes           []*NodeInfo
	mintCounter     uint64
	transferCounter uint64
	getCounter      uint64
	mu              sync.RWMutex
}

// NewNodePool creates a new node pool
func NewNodePool() *NodePool {
	return &NodePool{
		nodes: make([]*NodeInfo, 0),
	}
}

// AddNode adds a new node to the pool
func (np *NodePool) AddNode(url string) error {
	np.mu.Lock()
	defer np.mu.Unlock()

	// Create client for this node
	client := onemoney.NewTestClient()

	node := &NodeInfo{
		URL:    url,
		Client: client,
	}

	np.nodes = append(np.nodes, node)
	log.Printf("Added node: %s", url)

	return nil
}

// GetNodeForMint returns the next node for mint operations using round-robin
func (np *NodePool) GetNodeForMint() (*onemoney.Client, string, int, error) {
	np.mu.RLock()
	defer np.mu.RUnlock()

	if len(np.nodes) == 0 {
		return nil, "", 0, fmt.Errorf("no nodes available")
	}

	// Round-robin selection
	counter := atomic.AddUint64(&np.mintCounter, 1)
	index := int((counter - 1) % uint64(len(np.nodes)))
	node := np.nodes[index]

	atomic.AddInt64(&node.MintCount, 1)

	return node.Client, node.URL, index, nil
}

// GetNodeForTransfer returns the next node for transfer operations using round-robin
func (np *NodePool) GetNodeForTransfer() (*onemoney.Client, string, int, error) {
	np.mu.RLock()
	defer np.mu.RUnlock()

	if len(np.nodes) == 0 {
		return nil, "", 0, fmt.Errorf("no nodes available")
	}

	// Round-robin selection
	counter := atomic.AddUint64(&np.transferCounter, 1)
	index := int((counter - 1) % uint64(len(np.nodes)))
	node := np.nodes[index]

	atomic.AddInt64(&node.TransferCount, 1)

	return node.Client, node.URL, index, nil
}

// GetNodeForGet returns the next node for GET operations using round-robin
func (np *NodePool) GetNodeForGet() (*onemoney.Client, string, int, error) {
	np.mu.RLock()
	defer np.mu.RUnlock()

	if len(np.nodes) == 0 {
		return nil, "", 0, fmt.Errorf("no nodes available")
	}

	// Round-robin selection
	counter := atomic.AddUint64(&np.getCounter, 1)
	index := int((counter - 1) % uint64(len(np.nodes)))
	node := np.nodes[index]

	atomic.AddInt64(&node.GetCount, 1)

	return node.Client, node.URL, index, nil
}


// Size returns the number of nodes in the pool
func (np *NodePool) Size() int {
	np.mu.RLock()
	defer np.mu.RUnlock()
	return len(np.nodes)
}

// GetNodes returns all node URLs
func (np *NodePool) GetNodes() []string {
	np.mu.RLock()
	defer np.mu.RUnlock()

	urls := make([]string, len(np.nodes))
	for i, node := range np.nodes {
		urls[i] = node.URL
	}
	return urls
}

// GetNodeURL returns the URL for a specific node index
func (np *NodePool) GetNodeURL(index int) string {
	np.mu.RLock()
	defer np.mu.RUnlock()

	if index < 0 || index >= len(np.nodes) {
		return ""
	}
	return np.nodes[index].URL
}

// PrintDistribution prints operation distribution statistics
func (np *NodePool) PrintDistribution() {
	np.mu.RLock()
	defer np.mu.RUnlock()

	log.Println("\nNode Distribution:")
	log.Println("Node | URL                  | Mints | Transfers | GETs")
	log.Println("-----|----------------------|-------|-----------|-----")

	totalMints := int64(0)
	totalTransfers := int64(0)
	totalGets := int64(0)

	for i, node := range np.nodes {
		mints := atomic.LoadInt64(&node.MintCount)
		transfers := atomic.LoadInt64(&node.TransferCount)
		gets := atomic.LoadInt64(&node.GetCount)

		totalMints += mints
		totalTransfers += transfers
		totalGets += gets

		url := node.URL
		if len(url) > 23 {
			url = url[:20] + "..."
		}

		log.Printf("%4d | %-20s | %5d | %9d | %5d",
			i, url, mints, transfers, gets)
	}

	log.Println("-----|----------------------|-------|-----------|-----")
	log.Printf("TOTAL|                      | %5d | %9d | %5d",
		totalMints, totalTransfers, totalGets)
}
