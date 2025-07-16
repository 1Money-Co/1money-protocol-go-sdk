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