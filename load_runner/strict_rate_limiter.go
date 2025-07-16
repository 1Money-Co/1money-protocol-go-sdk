package main

import (
	"context"
	"sync"
	"time"
)

// StrictRateLimiter ensures strict sequential rate limiting with no concurrent token distribution
type StrictRateLimiter struct {
	tokenInterval   time.Duration
	nextTokenTime   time.Time
	mu              sync.Mutex
	startTime       time.Time
	tokenCount      int64
}

// NewStrictRateLimiter creates a rate limiter that strictly enforces token intervals
func NewStrictRateLimiter(ratePerSecond int) *StrictRateLimiter {
	tokenInterval := time.Second / time.Duration(ratePerSecond)
	
	Logf("Strict rate limiter: %d TPS = 1 token every %v\n", ratePerSecond, tokenInterval)
	
	return &StrictRateLimiter{
		tokenInterval: tokenInterval,
		nextTokenTime: time.Now(),
		startTime:     time.Now(),
		tokenCount:    0,
	}
}

// Wait blocks until the next token is available
func (rl *StrictRateLimiter) Wait(ctx context.Context) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	now := time.Now()
	
	// If we need to wait
	if now.Before(rl.nextTokenTime) {
		waitDuration := rl.nextTokenTime.Sub(now)
		
		// Sleep while holding the lock to ensure strict ordering
		select {
		case <-time.After(waitDuration):
			// Continue after wait
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	
	// Update next token time
	rl.tokenCount++
	rl.nextTokenTime = rl.nextTokenTime.Add(rl.tokenInterval)
	
	// If we've fallen behind, catch up
	now = time.Now()
	if rl.nextTokenTime.Before(now) {
		rl.nextTokenTime = now
	}
	
	return nil
}

// GetStats returns statistics about the rate limiter
func (rl *StrictRateLimiter) GetStats() (tokensIssued int64, elapsed time.Duration, actualRate float64) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	elapsed = time.Since(rl.startTime)
	tokensIssued = rl.tokenCount
	if elapsed.Seconds() > 0 {
		actualRate = float64(tokensIssued) / elapsed.Seconds()
	}
	return
}

// StrictGlobalRateLimiter manages strict rate limiting with node assignment
type StrictGlobalRateLimiter struct {
	limiter         *StrictRateLimiter
	nodeCount       int
	nodeAssignments []int64
	mu              sync.Mutex
}

// NewStrictGlobalRateLimiter creates a global rate limiter with strict rate enforcement
func NewStrictGlobalRateLimiter(nodeCount int, requestedRate int) *StrictGlobalRateLimiter {
	// Calculate maximum allowed rate
	maxRate := nodeCount * PostRateLimitPerNode
	
	effectiveRate := requestedRate
	if effectiveRate > maxRate {
		Logf("Requested rate %d exceeds max allowed (%d nodes × %d TPS = %d). Using %d TPS\n", 
			requestedRate, nodeCount, PostRateLimitPerNode, maxRate, maxRate)
		effectiveRate = maxRate
	} else {
		Logf("Using requested rate: %d TPS (max allowed: %d TPS)\n", effectiveRate, maxRate)
	}
	
	return &StrictGlobalRateLimiter{
		limiter:         NewStrictRateLimiter(effectiveRate),
		nodeCount:       nodeCount,
		nodeAssignments: make([]int64, nodeCount),
	}
}

// WaitAndGetNode waits for the next token and returns which node to use
func (g *StrictGlobalRateLimiter) WaitAndGetNode(ctx context.Context) (int, error) {
	// First wait for rate limit
	if err := g.limiter.Wait(ctx); err != nil {
		return -1, err
	}
	
	// Then assign a node (round-robin)
	g.mu.Lock()
	defer g.mu.Unlock()
	
	// Find node with least assignments
	minAssignments := g.nodeAssignments[0]
	selectedNode := 0
	
	for i := 1; i < g.nodeCount; i++ {
		if g.nodeAssignments[i] < minAssignments {
			minAssignments = g.nodeAssignments[i]
			selectedNode = i
		}
	}
	
	g.nodeAssignments[selectedNode]++
	return selectedNode, nil
}

// GetStats returns rate limiter statistics
func (g *StrictGlobalRateLimiter) GetStats() (tokensIssued int64, elapsed time.Duration, actualRate float64) {
	return g.limiter.GetStats()
}

// PrintStats prints current statistics
func (g *StrictGlobalRateLimiter) PrintStats() {
	tokens, elapsed, rate := g.GetStats()
	Logf("Rate limiter stats: %d tokens in %v = %.2f TPS\n", tokens, elapsed, rate)
}