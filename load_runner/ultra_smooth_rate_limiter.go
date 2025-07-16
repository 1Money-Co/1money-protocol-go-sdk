package main

import (
	"context"
	"sync"
	"time"
)

// UltraSmoothRateLimiter implements a rate limiter with ultra-smooth token distribution
// Instead of releasing tokens in batches, it releases them one by one with precise timing
type UltraSmoothRateLimiter struct {
	ratePerSecond   int
	tokenInterval   time.Duration
	lastTokenTime   time.Time
	mu              sync.Mutex
	done            chan struct{}
}

// NewUltraSmoothRateLimiter creates a rate limiter that releases tokens individually
func NewUltraSmoothRateLimiter(ratePerSecond int) *UltraSmoothRateLimiter {
	// Calculate interval between individual tokens
	tokenInterval := time.Second / time.Duration(ratePerSecond)
	
	Logf("Ultra-smooth rate limiter: %d TPS = 1 token every %v\n", ratePerSecond, tokenInterval)
	
	return &UltraSmoothRateLimiter{
		ratePerSecond: ratePerSecond,
		tokenInterval: tokenInterval,
		lastTokenTime: time.Now(),
		done:         make(chan struct{}),
	}
}

// Wait blocks until the next token is available
func (rl *UltraSmoothRateLimiter) Wait(ctx context.Context) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	// Calculate when the next token should be available
	nextTokenTime := rl.lastTokenTime.Add(rl.tokenInterval)
	now := time.Now()
	
	// If we need to wait, calculate the duration
	if now.Before(nextTokenTime) {
		waitDuration := nextTokenTime.Sub(now)
		
		// Create a timer for the wait
		timer := time.NewTimer(waitDuration)
		defer timer.Stop()
		
		// Unlock while waiting
		rl.mu.Unlock()
		
		// Wait for either the timer or context cancellation
		select {
		case <-timer.C:
			// Timer expired, we can proceed
		case <-ctx.Done():
			rl.mu.Lock() // Re-lock before returning
			return ctx.Err()
		case <-rl.done:
			rl.mu.Lock() // Re-lock before returning
			return context.Canceled
		}
		
		// Re-lock after waiting
		rl.mu.Lock()
	}
	
	// Update last token time
	rl.lastTokenTime = time.Now()
	return nil
}

// Close stops the rate limiter
func (rl *UltraSmoothRateLimiter) Close() {
	close(rl.done)
}

// UltraSmoothGlobalRateLimiter manages ultra-smooth rate limiting across all nodes
type UltraSmoothGlobalRateLimiter struct {
	postLimiter *UltraSmoothRateLimiter
	getLimiter  *UltraSmoothRateLimiter
	nodeCount   int
}

// NewUltraSmoothGlobalRateLimiter creates a global rate limiter with ultra-smooth distribution
func NewUltraSmoothGlobalRateLimiter(nodeCount int, requestedPostConcurrency int, requestedGetConcurrency int) *UltraSmoothGlobalRateLimiter {
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
	
	return &UltraSmoothGlobalRateLimiter{
		postLimiter: NewUltraSmoothRateLimiter(effectivePostRate),
		getLimiter:  NewUltraSmoothRateLimiter(effectiveGetRate),
		nodeCount:   nodeCount,
	}
}

// WaitForPost waits for POST request rate limit
func (g *UltraSmoothGlobalRateLimiter) WaitForPost(ctx context.Context) error {
	return g.postLimiter.Wait(ctx)
}

// WaitForGet waits for GET request rate limit
func (g *UltraSmoothGlobalRateLimiter) WaitForGet(ctx context.Context) error {
	return g.getLimiter.Wait(ctx)
}

// GetEffectivePostConcurrency returns the effective concurrency for POST requests
func (g *UltraSmoothGlobalRateLimiter) GetEffectivePostConcurrency(requested int) int {
	return CalculateEffectiveConcurrency(g.nodeCount, requested, PostRateLimitPerNode)
}

// GetEffectiveGetConcurrency returns the effective concurrency for GET requests
func (g *UltraSmoothGlobalRateLimiter) GetEffectiveGetConcurrency(requested int) int {
	return CalculateEffectiveConcurrency(g.nodeCount, requested, GetRateLimitPerNode)
}

// Close stops all rate limiters
func (g *UltraSmoothGlobalRateLimiter) Close() {
	// Nothing to close for ultra-smooth rate limiters
}