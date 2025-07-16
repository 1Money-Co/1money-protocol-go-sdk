package main

import (
	"context"
	"log"
	"sync"
	"time"
)

// NodeRateLimiter handles rate limiting for a single node
type NodeRateLimiter struct {
	nodeURL      string
	nodeIndex    int
	postRate     int
	getRate      int
	postInterval time.Duration
	getInterval  time.Duration
	nextPostTime time.Time
	nextGetTime  time.Time
	postCount    int64
	getCount     int64
	mu           sync.Mutex
	startTime    time.Time
}

// NewNodeRateLimiter creates a rate limiter for a single node
func NewNodeRateLimiter(nodeURL string, nodeIndex int, postRate int, getRate int) *NodeRateLimiter {
	postInterval := time.Second / time.Duration(postRate)
	getInterval := time.Second / time.Duration(getRate)

	return &NodeRateLimiter{
		nodeURL:      nodeURL,
		nodeIndex:    nodeIndex,
		postRate:     postRate,
		getRate:      getRate,
		postInterval: postInterval,
		getInterval:  getInterval,
		nextPostTime: time.Now(),
		nextGetTime:  time.Now(),
		postCount:    0,
		getCount:     0,
		startTime:    time.Now(),
	}
}

// WaitForPostToken blocks until the next POST token is available
func (nrl *NodeRateLimiter) WaitForPostToken(ctx context.Context) error {
	nrl.mu.Lock()
	defer nrl.mu.Unlock()

	now := time.Now()

	// If we need to wait for the next token
	if now.Before(nrl.nextPostTime) {
		waitDuration := nrl.nextPostTime.Sub(now)

		// Create timer
		timer := time.NewTimer(waitDuration)
		defer timer.Stop()

		// Unlock while waiting
		nrl.mu.Unlock()

		select {
		case <-timer.C:
			// Timer expired, continue
		case <-ctx.Done():
			nrl.mu.Lock()
			return ctx.Err()
		}

		// Re-lock after waiting
		nrl.mu.Lock()
		now = time.Now()
	}

	// Calculate next token time based on current token time to avoid drift
	nrl.nextPostTime = nrl.nextPostTime.Add(nrl.postInterval)

	// If we've fallen too far behind (more than 1 second), reset to current time
	if nrl.nextPostTime.Before(now.Add(-time.Second)) {
		nrl.nextPostTime = now
	}

	nrl.postCount++

	return nil
}

// WaitForGetToken blocks until the next GET token is available
func (nrl *NodeRateLimiter) WaitForGetToken(ctx context.Context) error {
	nrl.mu.Lock()
	defer nrl.mu.Unlock()

	now := time.Now()

	// If we need to wait for the next token
	if now.Before(nrl.nextGetTime) {
		waitDuration := nrl.nextGetTime.Sub(now)

		// Create timer
		timer := time.NewTimer(waitDuration)
		defer timer.Stop()

		// Unlock while waiting
		nrl.mu.Unlock()

		select {
		case <-timer.C:
			// Timer expired, continue
		case <-ctx.Done():
			nrl.mu.Lock()
			return ctx.Err()
		}

		// Re-lock after waiting
		nrl.mu.Lock()
		now = time.Now()
	}

	// Calculate next token time based on current token time to avoid drift
	nrl.nextGetTime = nrl.nextGetTime.Add(nrl.getInterval)

	// If we've fallen too far behind (more than 1 second), reset to current time
	if nrl.nextGetTime.Before(now.Add(-time.Second)) {
		nrl.nextGetTime = now
	}

	nrl.getCount++

	return nil
}

// GetStats returns statistics for this node's rate limiter
func (nrl *NodeRateLimiter) GetStats() (postCount int64, getCount int64, elapsed time.Duration, actualPostRate float64, actualGetRate float64) {
	nrl.mu.Lock()
	defer nrl.mu.Unlock()

	elapsed = time.Since(nrl.startTime)
	postCount = nrl.postCount
	getCount = nrl.getCount

	if elapsed.Seconds() > 0 {
		actualPostRate = float64(postCount) / elapsed.Seconds()
		actualGetRate = float64(getCount) / elapsed.Seconds()
	}
	return
}

// MultiNodeRateLimiter manages rate limiting across multiple nodes
type MultiNodeRateLimiter struct {
	nodeLimiters  []*NodeRateLimiter
	totalPostRate int
	totalGetRate  int
	nodeCount     int
}

// NewMultiNodeRateLimiter creates a rate limiter that distributes rate across multiple nodes
func NewMultiNodeRateLimiter(nodeURLs []string, totalPostRate int, totalGetRate int) *MultiNodeRateLimiter {
	nodeCount := len(nodeURLs)
	if nodeCount == 0 {
		panic("No nodes provided")
	}

	// Calculate rate per node
	basePostRate := totalPostRate / nodeCount
	postRemainder := totalPostRate % nodeCount

	baseGetRate := totalGetRate / nodeCount
	getRemainder := totalGetRate % nodeCount

	nodeLimiters := make([]*NodeRateLimiter, nodeCount)

	log.Printf("\n=== Rate Limiter Configuration ===")
	log.Printf("Total requested POST rate: %d TPS", totalPostRate)
	log.Printf("Total requested GET rate: %d TPS", totalGetRate)
	log.Printf("Number of nodes: %d", nodeCount)
	log.Printf("Base POST rate per node: %d TPS", basePostRate)
	log.Printf("Base GET rate per node: %d TPS", baseGetRate)

	// Create individual rate limiters for each node
	for i, nodeURL := range nodeURLs {
		nodePostRate := basePostRate
		nodeGetRate := baseGetRate

		// Distribute remainder tokens to first few nodes
		if i < postRemainder {
			nodePostRate++
		}
		if i < getRemainder {
			nodeGetRate++
		}

		nodeLimiters[i] = NewNodeRateLimiter(nodeURL, i, nodePostRate, nodeGetRate)

		postInterval := time.Second / time.Duration(nodePostRate)
		getInterval := time.Second / time.Duration(nodeGetRate)

		log.Printf("Node %d (%s): POST %d TPS (1 token every %v), GET %d TPS (1 token every %v)",
			i, nodeURL, nodePostRate, postInterval, nodeGetRate, getInterval)
	}
	log.Printf("==================================\n")

	return &MultiNodeRateLimiter{
		nodeLimiters:  nodeLimiters,
		totalPostRate: totalPostRate,
		totalGetRate:  totalGetRate,
		nodeCount:     nodeCount,
	}
}

// GetNodeRateLimiter returns the rate limiter for a specific node
func (mnrl *MultiNodeRateLimiter) GetNodeRateLimiter(nodeIndex int) *NodeRateLimiter {
	if nodeIndex < 0 || nodeIndex >= len(mnrl.nodeLimiters) {
		return nil
	}
	return mnrl.nodeLimiters[nodeIndex]
}

// PrintStats prints statistics for all nodes
func (mnrl *MultiNodeRateLimiter) PrintStats() {
	log.Println("\n┌─────────────────── Rate Limiter Statistics ───────────────────┐")
	log.Println("│ Node │ URL                  │ POST Tokens │ GET Tokens │ Elapsed  │ POST TPS │ GET TPS │")
	log.Println("├──────┼──────────────────────┼─────────────┼────────────┼──────────┼──────────┼─────────┤")

	totalPostTokens := int64(0)
	totalGetTokens := int64(0)
	var maxElapsed time.Duration

	for i, limiter := range mnrl.nodeLimiters {
		postTokens, getTokens, elapsed, postRate, getRate := limiter.GetStats()
		totalPostTokens += postTokens
		totalGetTokens += getTokens

		if elapsed > maxElapsed {
			maxElapsed = elapsed
		}

		url := limiter.nodeURL
		if len(url) > 20 {
			url = url[:17] + "..."
		}

		log.Printf("│ %4d │ %-20s │ %11d │ %10d │ %8.2fs │ %8.2f │ %7.2f │",
			i, url, postTokens, getTokens, elapsed.Seconds(), postRate, getRate)
	}

	log.Println("├──────┼──────────────────────┼─────────────┼────────────┼──────────┼──────────┼─────────┤")

	totalActualPostRate := float64(totalPostTokens) / maxElapsed.Seconds()
	totalActualGetRate := float64(totalGetTokens) / maxElapsed.Seconds()

	log.Printf("│ TOTAL│                      │ %11d │ %10d │ %8.2fs │ %8.2f │ %7.2f │",
		totalPostTokens, totalGetTokens, maxElapsed.Seconds(), totalActualPostRate, totalActualGetRate)

	log.Println("└──────┴──────────────────────┴─────────────┴────────────┴──────────┴──────────┴─────────┘")

	log.Printf("\nTarget POST rate: %d TPS, Actual POST rate: %.2f TPS", mnrl.totalPostRate, totalActualPostRate)
	log.Printf("Target GET rate: %d TPS, Actual GET rate: %.2f TPS", mnrl.totalGetRate, totalActualGetRate)
}

// NodeWorkerPool manages workers for a specific node
type NodeWorkerPool struct {
	nodeIndex   int
	nodeURL     string
	rateLimiter *NodeRateLimiter
	workerCount int
}

// NewNodeWorkerPool creates a worker pool for a specific node
func NewNodeWorkerPool(nodeIndex int, nodeURL string, rateLimiter *NodeRateLimiter, workerCount int) *NodeWorkerPool {
	return &NodeWorkerPool{
		nodeIndex:   nodeIndex,
		nodeURL:     nodeURL,
		rateLimiter: rateLimiter,
		workerCount: workerCount,
	}
}
