package main

import (
	"context"
	"sync"
	"time"
)

const (
	// Server rate limits
	PostRateLimitPerNode = 250 // 250 TPS per node for POST requests
	GetRateLimitPerNode  = 500 // 500 TPS per node for GET requests
)

// RateLimiter implements a token bucket rate limiter
type RateLimiter struct {
	tokens    chan struct{}
	ticker    *time.Ticker
	done      chan struct{}
	wg        sync.WaitGroup
	ratePerSec int
}

// NewRateLimiter creates a new rate limiter with the specified rate per second
func NewRateLimiter(ratePerSec int) *RateLimiter {
	rl := &RateLimiter{
		tokens:     make(chan struct{}, ratePerSec),
		ticker:     time.NewTicker(time.Second / time.Duration(ratePerSec)),
		done:       make(chan struct{}),
		ratePerSec: ratePerSec,
	}
	
	// Fill initial tokens
	for i := 0; i < ratePerSec; i++ {
		rl.tokens <- struct{}{}
	}
	
	// Start token refill goroutine
	rl.wg.Add(1)
	go rl.refillTokens()
	
	return rl
}

// refillTokens adds tokens at the configured rate
func (rl *RateLimiter) refillTokens() {
	defer rl.wg.Done()
	
	for {
		select {
		case <-rl.ticker.C:
			select {
			case rl.tokens <- struct{}{}:
				// Token added
			default:
				// Bucket full, skip
			}
		case <-rl.done:
			rl.ticker.Stop()
			return
		}
	}
}

// Wait blocks until a token is available
func (rl *RateLimiter) Wait(ctx context.Context) error {
	select {
	case <-rl.tokens:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Close stops the rate limiter
func (rl *RateLimiter) Close() {
	close(rl.done)
	rl.wg.Wait()
	close(rl.tokens)
}

// CalculateEffectiveConcurrency calculates the maximum concurrency based on rate limits
func CalculateEffectiveConcurrency(nodeCount int, requestedConcurrency int, rateLimit int) int {
	maxConcurrency := nodeCount * rateLimit
	if requestedConcurrency > maxConcurrency {
		Logf("Concurrency %d exceeds max allowed (%d nodes * %d TPS = %d). Capping to %d\n", 
			requestedConcurrency, nodeCount, rateLimit, maxConcurrency, maxConcurrency)
		return maxConcurrency
	}
	return requestedConcurrency
}

// GlobalRateLimiter manages rate limiting across all nodes
type GlobalRateLimiter struct {
	postLimiter *RateLimiter
	getLimiter  *RateLimiter
	nodeCount   int
}

// NewGlobalRateLimiter creates a rate limiter that respects per-node limits
func NewGlobalRateLimiter(nodeCount int) *GlobalRateLimiter {
	return &GlobalRateLimiter{
		postLimiter: NewRateLimiter(nodeCount * PostRateLimitPerNode),
		getLimiter:  NewRateLimiter(nodeCount * GetRateLimitPerNode),
		nodeCount:   nodeCount,
	}
}

// WaitForPost waits for POST request rate limit
func (g *GlobalRateLimiter) WaitForPost(ctx context.Context) error {
	return g.postLimiter.Wait(ctx)
}

// WaitForGet waits for GET request rate limit
func (g *GlobalRateLimiter) WaitForGet(ctx context.Context) error {
	return g.getLimiter.Wait(ctx)
}

// GetEffectivePostConcurrency returns the effective concurrency for POST requests
func (g *GlobalRateLimiter) GetEffectivePostConcurrency(requested int) int {
	return CalculateEffectiveConcurrency(g.nodeCount, requested, PostRateLimitPerNode)
}

// GetEffectiveGetConcurrency returns the effective concurrency for GET requests
func (g *GlobalRateLimiter) GetEffectiveGetConcurrency(requested int) int {
	return CalculateEffectiveConcurrency(g.nodeCount, requested, GetRateLimitPerNode)
}

// Close stops all rate limiters
func (g *GlobalRateLimiter) Close() {
	g.postLimiter.Close()
	g.getLimiter.Close()
}