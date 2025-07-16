package main

import (
	"context"
	"sync"
	"time"
)

// SmoothRateLimiter implements a rate limiter that distributes requests evenly across time intervals
type SmoothRateLimiter struct {
	ratePerSecond    int
	intervalsPerSec  int
	tokensPerInterval int
	intervalDuration time.Duration
	
	intervalTicker   *time.Ticker
	availableTokens  int
	mu              sync.Mutex
	cond            *sync.Cond
	done            chan struct{}
	wg              sync.WaitGroup
}

// NewSmoothRateLimiter creates a rate limiter that distributes requests evenly
// Default: 20 intervals per second (50ms each)
func NewSmoothRateLimiter(ratePerSecond int) *SmoothRateLimiter {
	intervalsPerSec := 20 // 50ms intervals
	tokensPerInterval := ratePerSecond / intervalsPerSec
	if tokensPerInterval < 1 {
		tokensPerInterval = 1
	}
	
	rl := &SmoothRateLimiter{
		ratePerSecond:    ratePerSecond,
		intervalsPerSec:  intervalsPerSec,
		tokensPerInterval: tokensPerInterval,
		intervalDuration: time.Second / time.Duration(intervalsPerSec),
		intervalTicker:   time.NewTicker(time.Second / time.Duration(intervalsPerSec)),
		availableTokens:  0,
		done:            make(chan struct{}),
	}
	
	rl.cond = sync.NewCond(&rl.mu)
	
	// Start the token distributor
	rl.wg.Add(1)
	go rl.distributeTokens()
	
	return rl
}

// distributeTokens adds tokens at regular intervals
func (rl *SmoothRateLimiter) distributeTokens() {
	defer rl.wg.Done()
	
	// Initial tokens for first interval
	rl.mu.Lock()
	rl.availableTokens = rl.tokensPerInterval
	rl.cond.Broadcast()
	rl.mu.Unlock()
	
	for {
		select {
		case <-rl.intervalTicker.C:
			rl.mu.Lock()
			// Add tokens for this interval, but cap at the per-interval limit
			// This prevents token accumulation
			rl.availableTokens = rl.tokensPerInterval
			rl.cond.Broadcast()
			rl.mu.Unlock()
			
		case <-rl.done:
			rl.intervalTicker.Stop()
			return
		}
	}
}

// Wait blocks until a token is available
func (rl *SmoothRateLimiter) Wait(ctx context.Context) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	for rl.availableTokens <= 0 {
		// Create a channel to listen for context cancellation
		done := make(chan struct{})
		go func() {
			select {
			case <-ctx.Done():
				rl.mu.Lock()
				rl.cond.Broadcast()
				rl.mu.Unlock()
				close(done)
			case <-done:
				// Normal exit
			}
		}()
		
		rl.cond.Wait()
		
		// Check if context was cancelled
		select {
		case <-ctx.Done():
			close(done)
			return ctx.Err()
		default:
			close(done)
		}
	}
	
	rl.availableTokens--
	return nil
}

// Close stops the rate limiter
func (rl *SmoothRateLimiter) Close() {
	close(rl.done)
	rl.wg.Wait()
}

// SmoothGlobalRateLimiter manages smooth rate limiting across all nodes
type SmoothGlobalRateLimiter struct {
	postLimiter *SmoothRateLimiter
	getLimiter  *SmoothRateLimiter
	nodeCount   int
}

// NewSmoothGlobalRateLimiter creates a global rate limiter with smooth distribution
func NewSmoothGlobalRateLimiter(nodeCount int) *SmoothGlobalRateLimiter {
	return &SmoothGlobalRateLimiter{
		postLimiter: NewSmoothRateLimiter(nodeCount * PostRateLimitPerNode),
		getLimiter:  NewSmoothRateLimiter(nodeCount * GetRateLimitPerNode),
		nodeCount:   nodeCount,
	}
}

// WaitForPost waits for POST request rate limit
func (g *SmoothGlobalRateLimiter) WaitForPost(ctx context.Context) error {
	return g.postLimiter.Wait(ctx)
}

// WaitForGet waits for GET request rate limit
func (g *SmoothGlobalRateLimiter) WaitForGet(ctx context.Context) error {
	return g.getLimiter.Wait(ctx)
}

// GetEffectivePostConcurrency returns the effective concurrency for POST requests
func (g *SmoothGlobalRateLimiter) GetEffectivePostConcurrency(requested int) int {
	return CalculateEffectiveConcurrency(g.nodeCount, requested, PostRateLimitPerNode)
}

// GetEffectiveGetConcurrency returns the effective concurrency for GET requests
func (g *SmoothGlobalRateLimiter) GetEffectiveGetConcurrency(requested int) int {
	return CalculateEffectiveConcurrency(g.nodeCount, requested, GetRateLimitPerNode)
}

// Close stops all rate limiters
func (g *SmoothGlobalRateLimiter) Close() {
	g.postLimiter.Close()
	g.getLimiter.Close()
}