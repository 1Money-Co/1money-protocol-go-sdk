package main

import (
	"context"
)

// PerNodeRateLimiter implements rate limiting per individual node
type PerNodeRateLimiter struct {
	nodeRateLimiters []*UltraSmoothRateLimiter
	nodeCount        int
	ratePerNode      int
}

// NewPerNodeRateLimiter creates a rate limiter with separate limits per node
func NewPerNodeRateLimiter(nodeCount int, totalRate int) *PerNodeRateLimiter {
	ratePerNode := totalRate / nodeCount
	remainder := totalRate % nodeCount
	
	Logf("Per-node rate limiter: %d TPS total = %d TPS per node (remainder: %d)\n", 
		totalRate, ratePerNode, remainder)
	
	rateLimiters := make([]*UltraSmoothRateLimiter, nodeCount)
	for i := 0; i < nodeCount; i++ {
		nodeRate := ratePerNode
		// Distribute remainder across first few nodes
		if i < remainder {
			nodeRate++
		}
		rateLimiters[i] = NewUltraSmoothRateLimiter(nodeRate)
		Logf("Node %d rate limiter: %d TPS\n", i, nodeRate)
	}
	
	return &PerNodeRateLimiter{
		nodeRateLimiters: rateLimiters,
		nodeCount:        nodeCount,
		ratePerNode:      ratePerNode,
	}
}

// WaitForNode waits for rate limit for a specific node
func (pnrl *PerNodeRateLimiter) WaitForNode(ctx context.Context, nodeIndex int) error {
	if nodeIndex < 0 || nodeIndex >= len(pnrl.nodeRateLimiters) {
		return context.Canceled
	}
	return pnrl.nodeRateLimiters[nodeIndex].Wait(ctx)
}

// Close stops all rate limiters
func (pnrl *PerNodeRateLimiter) Close() {
	for _, limiter := range pnrl.nodeRateLimiters {
		limiter.Close()
	}
}

// PerNodeGlobalRateLimiter manages per-node rate limiting for POST and GET
type PerNodeGlobalRateLimiter struct {
	postLimiter *PerNodeRateLimiter
	getLimiter  *PerNodeRateLimiter
	nodeCount   int
}

// NewPerNodeGlobalRateLimiter creates a global rate limiter with per-node distribution
func NewPerNodeGlobalRateLimiter(nodeCount int, requestedPostConcurrency int, requestedGetConcurrency int) *PerNodeGlobalRateLimiter {
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
	
	return &PerNodeGlobalRateLimiter{
		postLimiter: NewPerNodeRateLimiter(nodeCount, effectivePostRate),
		getLimiter:  NewPerNodeRateLimiter(nodeCount, effectiveGetRate),
		nodeCount:   nodeCount,
	}
}

// WaitForPost waits for POST request rate limit for a specific node
func (g *PerNodeGlobalRateLimiter) WaitForPost(ctx context.Context, nodeIndex int) error {
	return g.postLimiter.WaitForNode(ctx, nodeIndex)
}

// WaitForGet waits for GET request rate limit for a specific node
func (g *PerNodeGlobalRateLimiter) WaitForGet(ctx context.Context, nodeIndex int) error {
	return g.getLimiter.WaitForNode(ctx, nodeIndex)
}

// GetEffectivePostConcurrency returns the effective concurrency for POST requests
func (g *PerNodeGlobalRateLimiter) GetEffectivePostConcurrency(requested int) int {
	return CalculateEffectiveConcurrency(g.nodeCount, requested, PostRateLimitPerNode)
}

// GetEffectiveGetConcurrency returns the effective concurrency for GET requests
func (g *PerNodeGlobalRateLimiter) GetEffectiveGetConcurrency(requested int) int {
	return CalculateEffectiveConcurrency(g.nodeCount, requested, GetRateLimitPerNode)
}

// Close stops all rate limiters
func (g *PerNodeGlobalRateLimiter) Close() {
	g.postLimiter.Close()
	g.getLimiter.Close()
}