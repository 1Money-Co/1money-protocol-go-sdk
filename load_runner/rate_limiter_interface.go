package main

import "context"

// RateLimiterInterface defines the common interface for rate limiters
type RateLimiterInterface interface {
	WaitForPost(ctx context.Context) error
	WaitForGet(ctx context.Context) error
	GetEffectivePostConcurrency(requested int) int
	GetEffectiveGetConcurrency(requested int) int
	Close()
}

// PerNodeRateLimiterInterface defines interface for per-node rate limiting
type PerNodeRateLimiterInterface interface {
	WaitForPost(ctx context.Context, nodeIndex int) error
	WaitForGet(ctx context.Context, nodeIndex int) error
	GetEffectivePostConcurrency(requested int) int
	GetEffectiveGetConcurrency(requested int) int
	Close()
}