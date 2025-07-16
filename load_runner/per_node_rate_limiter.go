package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// NodeRateLimiter handles rate limiting for a single node
type NodeRateLimiter struct {
	nodeURL       string
	nodeIndex     int
	ratePerSecond int
	tokenInterval time.Duration
	nextTokenTime time.Time
	tokenCount    int64
	mu            sync.Mutex
	startTime     time.Time
}

// NewNodeRateLimiter creates a rate limiter for a single node
func NewNodeRateLimiter(nodeURL string, nodeIndex int, ratePerSecond int) *NodeRateLimiter {
	tokenInterval := time.Second / time.Duration(ratePerSecond)
	
	return &NodeRateLimiter{
		nodeURL:       nodeURL,
		nodeIndex:     nodeIndex,
		ratePerSecond: ratePerSecond,
		tokenInterval: tokenInterval,
		nextTokenTime: time.Now(),
		tokenCount:    0,
		startTime:     time.Now(),
	}
}

// WaitForToken blocks until the next token is available for this node
func (nrl *NodeRateLimiter) WaitForToken(ctx context.Context) error {
	nrl.mu.Lock()
	defer nrl.mu.Unlock()
	
	now := time.Now()
	
	// If we need to wait for the next token
	if now.Before(nrl.nextTokenTime) {
		waitDuration := nrl.nextTokenTime.Sub(now)
		
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
	nrl.nextTokenTime = nrl.nextTokenTime.Add(nrl.tokenInterval)
	
	// If we've fallen too far behind (more than 1 second), reset to current time
	if nrl.nextTokenTime.Before(now.Add(-time.Second)) {
		nrl.nextTokenTime = now
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

// MultiNodeRateLimiter manages rate limiting across multiple nodes
type MultiNodeRateLimiter struct {
	nodeLimiters []*NodeRateLimiter
	totalRate    int
	nodeCount    int
}

// NewMultiNodeRateLimiter creates a rate limiter that distributes rate across multiple nodes
func NewMultiNodeRateLimiter(nodeURLs []string, totalRate int) *MultiNodeRateLimiter {
	nodeCount := len(nodeURLs)
	if nodeCount == 0 {
		panic("No nodes provided")
	}
	
	// Calculate rate per node
	baseRate := totalRate / nodeCount
	remainder := totalRate % nodeCount
	
	nodeLimiters := make([]*NodeRateLimiter, nodeCount)
	
	Logf("\n=== Rate Limiter Configuration ===\n")
	Logf("Total requested rate: %d TPS\n", totalRate)
	Logf("Number of nodes: %d\n", nodeCount)
	Logf("Base rate per node: %d TPS\n", baseRate)
	if remainder > 0 {
		Logf("Remainder distribution: %d nodes will get +1 TPS\n", remainder)
	}
	
	// Create individual rate limiters for each node
	for i, nodeURL := range nodeURLs {
		nodeRate := baseRate
		// Distribute remainder tokens to first few nodes
		if i < remainder {
			nodeRate++
		}
		
		nodeLimiters[i] = NewNodeRateLimiter(nodeURL, i, nodeRate)
		tokenInterval := time.Second / time.Duration(nodeRate)
		Logf("Node %d (%s): %d TPS (1 token every %v)\n", i, nodeURL, nodeRate, tokenInterval)
	}
	Logf("==================================\n")
	
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
	Logln("\n┌─────────────────── Rate Limiter Statistics ───────────────────┐")
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
	
	Logf("\nTarget total rate: %d TPS, Actual total rate: %.2f TPS\n", 
		mnrl.totalRate, totalActualRate)
}

// NodeWorkerPool manages workers for a specific node
type NodeWorkerPool struct {
	nodeIndex    int
	nodeURL      string
	rateLimiter  *NodeRateLimiter
	workerCount  int
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