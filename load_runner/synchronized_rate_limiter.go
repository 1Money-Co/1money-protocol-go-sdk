package main

import (
	"context"
	"sync"
	"time"
)

// SynchronizedPerNodeRateLimiter ensures proper rate limiting across all nodes
// by synchronizing token distribution
type SynchronizedPerNodeRateLimiter struct {
	totalRate        int
	nodeCount        int
	tokensPerNode    int
	tokenInterval    time.Duration
	lastTokenTime    time.Time
	nodeTokenCounts  []int64
	mu               sync.Mutex
	done             chan struct{}
}

// NewSynchronizedPerNodeRateLimiter creates a rate limiter that properly synchronizes across nodes
func NewSynchronizedPerNodeRateLimiter(nodeCount int, totalRate int) *SynchronizedPerNodeRateLimiter {
	tokensPerNode := totalRate / nodeCount
	// For 800 TPS across 4 nodes = 200 TPS per node = 1 token every 5ms per node
	// But we need to ensure total rate doesn't exceed 800 TPS
	tokenInterval := time.Second / time.Duration(totalRate)
	
	Logf("Synchronized rate limiter: %d TPS total, %d nodes, %d TPS/node\n", 
		totalRate, nodeCount, tokensPerNode)
	Logf("Token interval: %v (ensures %d TPS total)\n", tokenInterval, totalRate)
	
	return &SynchronizedPerNodeRateLimiter{
		totalRate:       totalRate,
		nodeCount:       nodeCount,
		tokensPerNode:   tokensPerNode,
		tokenInterval:   tokenInterval,
		lastTokenTime:   time.Now().Add(-tokenInterval), // Start one interval in the past so first token is immediate
		nodeTokenCounts: make([]int64, nodeCount),
		done:           make(chan struct{}),
	}
}

// WaitForNode waits for the next available token for any node
// Returns the node index that should be used
func (s *SynchronizedPerNodeRateLimiter) WaitForNode(ctx context.Context) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Calculate when the next token should be available
	nextTokenTime := s.lastTokenTime.Add(s.tokenInterval)
	now := time.Now()
	
	// If we need to wait, calculate the duration
	if now.Before(nextTokenTime) {
		waitDuration := nextTokenTime.Sub(now)
		
		// Create a timer for the wait
		timer := time.NewTimer(waitDuration)
		defer timer.Stop()
		
		// Unlock while waiting
		s.mu.Unlock()
		
		// Wait for either the timer or context cancellation
		select {
		case <-timer.C:
			// Timer expired, we can proceed
		case <-ctx.Done():
			s.mu.Lock() // Re-lock before returning
			return -1, ctx.Err()
		case <-s.done:
			s.mu.Lock() // Re-lock before returning
			return -1, context.Canceled
		}
		
		// Re-lock after waiting
		s.mu.Lock()
	}
	
	// Update last token time to maintain precise intervals
	// Use nextTokenTime instead of time.Now() to prevent drift
	s.lastTokenTime = nextTokenTime
	
	// Find the node with the least tokens (round-robin with balance)
	minTokens := s.nodeTokenCounts[0]
	selectedNode := 0
	for i := 1; i < s.nodeCount; i++ {
		if s.nodeTokenCounts[i] < minTokens {
			minTokens = s.nodeTokenCounts[i]
			selectedNode = i
		}
	}
	
	// Increment token count for selected node
	s.nodeTokenCounts[selectedNode]++
	
	return selectedNode, nil
}

// Close stops the rate limiter
func (s *SynchronizedPerNodeRateLimiter) Close() {
	close(s.done)
}

// SynchronizedGlobalRateLimiter manages synchronized rate limiting for POST and GET
type SynchronizedGlobalRateLimiter struct {
	postLimiter *SynchronizedPerNodeRateLimiter
	getLimiter  *SynchronizedPerNodeRateLimiter
	nodeCount   int
}

// NewSynchronizedGlobalRateLimiter creates a global rate limiter with synchronized distribution
func NewSynchronizedGlobalRateLimiter(nodeCount int, requestedPostConcurrency int, requestedGetConcurrency int) *SynchronizedGlobalRateLimiter {
	// Calculate maximum allowed rates
	maxPostRate := nodeCount * PostRateLimitPerNode
	maxGetRate := nodeCount * GetRateLimitPerNode
	
	// Use the minimum of requested and maximum allowed
	effectivePostRate := requestedPostConcurrency
	if effectivePostRate > maxPostRate {
		Logf("POST concurrency %d exceeds max allowed (%d nodes × %d TPS = %d). Using %d TPS\n", 
			requestedPostConcurrency, nodeCount, PostRateLimitPerNode, maxPostRate, maxPostRate)
		effectivePostRate = maxPostRate
	} else {
		Logf("Using requested POST rate: %d TPS (max allowed: %d TPS)\n", effectivePostRate, maxPostRate)
	}
	
	effectiveGetRate := requestedGetConcurrency
	if effectiveGetRate > maxGetRate {
		Logf("GET concurrency %d exceeds max allowed (%d nodes × %d TPS = %d). Using %d TPS\n", 
			requestedGetConcurrency, nodeCount, GetRateLimitPerNode, maxGetRate, maxGetRate)
		effectiveGetRate = maxGetRate
	} else {
		Logf("Using requested GET rate: %d TPS (max allowed: %d TPS)\n", effectiveGetRate, maxGetRate)
	}
	
	return &SynchronizedGlobalRateLimiter{
		postLimiter: NewSynchronizedPerNodeRateLimiter(nodeCount, effectivePostRate),
		getLimiter:  NewSynchronizedPerNodeRateLimiter(nodeCount, effectiveGetRate),
		nodeCount:   nodeCount,
	}
}

// WaitForPostAndGetNode waits for POST rate limit and returns which node to use
func (g *SynchronizedGlobalRateLimiter) WaitForPostAndGetNode(ctx context.Context) (int, error) {
	return g.postLimiter.WaitForNode(ctx)
}

// WaitForGetAndGetNode waits for GET rate limit and returns which node to use
func (g *SynchronizedGlobalRateLimiter) WaitForGetAndGetNode(ctx context.Context) (int, error) {
	return g.getLimiter.WaitForNode(ctx)
}

// GetEffectivePostConcurrency returns the effective concurrency for POST requests
func (g *SynchronizedGlobalRateLimiter) GetEffectivePostConcurrency(requested int) int {
	return CalculateEffectiveConcurrency(g.nodeCount, requested, PostRateLimitPerNode)
}

// GetEffectiveGetConcurrency returns the effective concurrency for GET requests
func (g *SynchronizedGlobalRateLimiter) GetEffectiveGetConcurrency(requested int) int {
	return CalculateEffectiveConcurrency(g.nodeCount, requested, GetRateLimitPerNode)
}

// Close stops all rate limiters
func (g *SynchronizedGlobalRateLimiter) Close() {
	g.postLimiter.Close()
	g.getLimiter.Close()
}