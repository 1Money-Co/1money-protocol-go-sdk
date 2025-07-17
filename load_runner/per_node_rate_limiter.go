package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// NodeRateLimiter handles rate limiting for a single node with micro-burst prevention
type NodeRateLimiter struct {
	nodeURL       string
	nodeIndex     int
	ratePerSecond int
	tokenInterval time.Duration
	nextTokenTime time.Time
	tokenCount    int64
	mu            sync.Mutex
	startTime     time.Time

	// Sliding window for micro-burst prevention
	recentRequests []time.Time   // Timestamps of recent requests
	windowSize     time.Duration // Time window for burst detection (e.g., 100ms)
	maxBurst       int           // Maximum requests allowed in window (2x rate)
}

// NewNodeRateLimiter creates a rate limiter for a single node
func NewNodeRateLimiter(nodeURL string, nodeIndex int, ratePerSecond int) *NodeRateLimiter {
	tokenInterval := time.Second / time.Duration(ratePerSecond)

	// Calculate window size and max burst for micro-burst prevention
	// Use 100ms window to detect micro-bursts
	windowSize := 100 * time.Millisecond
	// Allow max 2x rate in the window (server's micro-burst config)
	maxBurstRate := ratePerSecond * 2
	maxBurst := int(float64(maxBurstRate) * windowSize.Seconds())
	if maxBurst < 1 {
		maxBurst = 1 // Ensure at least 1 request allowed
	}

	return &NodeRateLimiter{
		nodeURL:        nodeURL,
		nodeIndex:      nodeIndex,
		ratePerSecond:  ratePerSecond,
		tokenInterval:  tokenInterval,
		nextTokenTime:  time.Now(),
		tokenCount:     0,
		startTime:      time.Now(),
		recentRequests: make([]time.Time, 0, maxBurst*2),
		windowSize:     windowSize,
		maxBurst:       maxBurst,
	}
}

// WaitForToken blocks until the next token is available for this node
func (nrl *NodeRateLimiter) WaitForToken(ctx context.Context) error {
	nrl.mu.Lock()
	defer nrl.mu.Unlock()

	now := time.Now()

	// Clean up old requests outside the window
	cutoff := now.Add(-nrl.windowSize)
	validRequests := make([]time.Time, 0, len(nrl.recentRequests))
	for _, reqTime := range nrl.recentRequests {
		if reqTime.After(cutoff) {
			validRequests = append(validRequests, reqTime)
		}
	}
	nrl.recentRequests = validRequests

	// Check if we would exceed burst limit
	if len(nrl.recentRequests) >= nrl.maxBurst {
		// Find the oldest request in the window
		oldestInWindow := nrl.recentRequests[0]
		// Calculate when we can send the next request
		nextAllowedTime := oldestInWindow.Add(nrl.windowSize)

		// Also respect the token bucket timing
		if nrl.nextTokenTime.After(nextAllowedTime) {
			nextAllowedTime = nrl.nextTokenTime
		}

		// Wait until we can send
		waitDuration := nextAllowedTime.Sub(now)
		if waitDuration > 0 {
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

			// Clean up again after waiting
			cutoff = now.Add(-nrl.windowSize)
			validRequests = make([]time.Time, 0, len(nrl.recentRequests))
			for _, reqTime := range nrl.recentRequests {
				if reqTime.After(cutoff) {
					validRequests = append(validRequests, reqTime)
				}
			}
			nrl.recentRequests = validRequests
		}
	}

	// Also check the regular token bucket timing
	if now.Before(nrl.nextTokenTime) {
		waitDuration := nrl.nextTokenTime.Sub(now)

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

	// Record this request
	nrl.recentRequests = append(nrl.recentRequests, now)

	// Update next token time
	nrl.nextTokenTime = nrl.nextTokenTime.Add(nrl.tokenInterval)

	// If we've fallen too far behind (more than 1 second), reset to current time
	if nrl.nextTokenTime.Before(now.Add(-time.Second)) {
		nrl.nextTokenTime = now.Add(nrl.tokenInterval)
	}

	nrl.tokenCount++

	return nil
}

// GetStats returns statistics for this node's rate limiter
func (nrl *NodeRateLimiter) GetStats() (tokensIssued int64, elapsed time.Duration, actualRate float64) {
	nrl.mu.Lock()
	defer nrl.mu.Unlock()

	elapsed = time.Since(nrl.startTime)
	tokensIssued = nrl.tokenCount
	if elapsed.Seconds() > 0 {
		actualRate = float64(tokensIssued) / elapsed.Seconds()
	}
	return
}

// GetBurstInfo returns information about the current burst window
func (nrl *NodeRateLimiter) GetBurstInfo() (currentBurst int, maxBurst int, windowSize time.Duration) {
	nrl.mu.Lock()
	defer nrl.mu.Unlock()

	// Clean up old requests
	now := time.Now()
	cutoff := now.Add(-nrl.windowSize)
	validRequests := make([]time.Time, 0, len(nrl.recentRequests))
	for _, reqTime := range nrl.recentRequests {
		if reqTime.After(cutoff) {
			validRequests = append(validRequests, reqTime)
		}
	}
	nrl.recentRequests = validRequests

	return len(nrl.recentRequests), nrl.maxBurst, nrl.windowSize
}

// MultiNodeRateLimiter manages rate limiting across multiple nodes
type MultiNodeRateLimiter struct {
	nodeLimiters []*NodeRateLimiter
	totalRate    int
	nodeCount    int
}

// NewMultiNodeRateLimiter creates a rate limiter that distributes rate across multiple nodes
func NewMultiNodeRateLimiter(nodeURLs []string, totalRate int) *MultiNodeRateLimiter {
	return NewMultiNodeRateLimiterWithType(nodeURLs, totalRate, "POST")
}

// NewMultiNodeRateLimiterWithType creates a rate limiter with operation type
func NewMultiNodeRateLimiterWithType(nodeURLs []string, totalRate int, operationType string) *MultiNodeRateLimiter {
	nodeCount := len(nodeURLs)
	if nodeCount == 0 {
		panic("No nodes provided")
	}

	// Calculate rate per node
	baseRate := totalRate / nodeCount
	remainder := totalRate % nodeCount

	nodeLimiters := make([]*NodeRateLimiter, nodeCount)

	Logf("\n=== %s Rate Limiter Configuration ===\n", operationType)
	Logf("Total requested rate: %d TPS\n", totalRate)
	Logf("Number of nodes: %d\n", nodeCount)
	Logf("Base rate per node: %d TPS\n", baseRate)
	if remainder > 0 {
		Logf("Remainder distribution: %d nodes will get +1 TPS\n", remainder)
	}
	Logf("Micro-burst prevention: 2x rate limit in 100ms windows\n")

	// Create individual rate limiters for each node
	for i, nodeURL := range nodeURLs {
		nodeRate := baseRate
		// Distribute remainder tokens to first few nodes
		if i < remainder {
			nodeRate++
		}

		nodeLimiters[i] = NewNodeRateLimiter(nodeURL, i, nodeRate)
		tokenInterval := time.Second / time.Duration(nodeRate)
		// Calculate burst info for logging
		windowSize := 100 * time.Millisecond
		maxBurstRate := nodeRate * 2
		maxBurst := int(float64(maxBurstRate) * windowSize.Seconds())
		if maxBurst < 1 {
			maxBurst = 1
		}
		Logf("Node %d (%s): %d TPS (1 token every %v, max %d reqs/100ms)\n", i, nodeURL, nodeRate, tokenInterval, maxBurst)
	}
	Logf("==================================%s\n", strings.Repeat("=", len(operationType)))

	return &MultiNodeRateLimiter{
		nodeLimiters: nodeLimiters,
		totalRate:    totalRate,
		nodeCount:    nodeCount,
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
	Logln("┌─────────────────── Rate Limiter Statistics ─────────────────────┐")
	Logln("│ Node │ URL                     │ Tokens │ Elapsed  │ Actual TPS │")
	Logln("├──────┼─────────────────────────┼────────┼──────────┼────────────┤")

	totalTokens := int64(0)
	var maxElapsed time.Duration

	for i, limiter := range mnrl.nodeLimiters {
		tokens, elapsed, rate := limiter.GetStats()
		totalTokens += tokens
		if elapsed > maxElapsed {
			maxElapsed = elapsed
		}

		url := limiter.nodeURL
		if len(url) > 23 {
			url = url[:20] + "..."
		}

		Logf("│ %4d │ %-23s │ %6d │ %8.2fs │ %10.2f │\n",
			i, url, tokens, elapsed.Seconds(), rate)
	}

	Logln("├──────┼─────────────────────────┼────────┼──────────┼────────────┤")

	totalActualRate := float64(totalTokens) / maxElapsed.Seconds()
	Logf("│ TOTAL│                         │ %6d │ %8.2fs │ %10.2f │\n",
		totalTokens, maxElapsed.Seconds(), totalActualRate)

	Logln("└──────┴─────────────────────────┴────────┴──────────┴────────────┘")

	Logf("Target total rate: %d TPS, Actual total rate: %.2f TPS\n",
		mnrl.totalRate, totalActualRate)
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

// ProcessTransactions processes transactions for this node
func (nwp *NodeWorkerPool) ProcessTransactions(
	ctx context.Context,
	nodePool *BalancedNodePool,
	accounts []Account,
	toAddress string,
	amount string,
	workQueue <-chan int,
	results chan<- TransactionResult,
	wg *sync.WaitGroup,
) {
	// Start workers for this node
	for w := 0; w < nwp.workerCount; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for accountIndex := range workQueue {
				// Wait for rate limit token
				if err := nwp.rateLimiter.WaitForToken(ctx); err != nil {
					results <- TransactionResult{
						AccountIndex: accountIndex,
						WalletIndex:  accounts[accountIndex].WalletIndex,
						Error:        fmt.Errorf("rate limit wait failed: %w", err),
						SendTime:     time.Now(),
						ResponseTime: time.Now(),
						NodeIndex:    nwp.nodeIndex,
						NodeURL:      nwp.nodeURL,
					}
					continue
				}

				// Get client for this specific node
				client, _ := nodePool.GetClientForNode(nwp.nodeIndex)
				if client == nil {
					results <- TransactionResult{
						AccountIndex: accountIndex,
						WalletIndex:  accounts[accountIndex].WalletIndex,
						Error:        fmt.Errorf("no client available for node %d", nwp.nodeIndex),
						SendTime:     time.Now(),
						ResponseTime: time.Now(),
						NodeIndex:    nwp.nodeIndex,
						NodeURL:      nwp.nodeURL,
					}
					continue
				}

				// Send transaction
				result := SendSingleTransactionToNode(client, nwp.nodeURL, nwp.nodeIndex, nodePool, accounts[accountIndex], toAddress, amount)
				result.AccountIndex = accountIndex
				results <- result
			}
		}(w)
	}
}
