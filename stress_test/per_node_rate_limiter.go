package main

import (
	"context"
	"log"
	"sync"
	"time"
)

// NodeRateLimiter handles rate limiting for a single node
type NodeRateLimiter struct {
	nodeURL      string
	nodeIndex    int
	postRate     int
	getRate      int
	postInterval time.Duration
	getInterval  time.Duration
	nextPostTime time.Time
	nextGetTime  time.Time
	postCount    int64
	getCount     int64
	mu           sync.Mutex
	startTime    time.Time
	
	// Sliding window for micro-burst prevention
	recentPostRequests []time.Time      // Timestamps of recent POST requests
	recentGetRequests  []time.Time      // Timestamps of recent GET requests
	windowSize         time.Duration    // Time window for burst detection (e.g., 100ms)
	maxPostBurst       int              // Maximum POST requests allowed in window (2x rate)
	maxGetBurst        int              // Maximum GET requests allowed in window (2x rate)
}

// NewNodeRateLimiter creates a rate limiter for a single node
func NewNodeRateLimiter(nodeURL string, nodeIndex int, postRate int, getRate int) *NodeRateLimiter {
	postInterval := time.Second / time.Duration(postRate)
	getInterval := time.Second / time.Duration(getRate)
	
	// Configure sliding window for micro-burst prevention (100ms window)
	windowSize := 100 * time.Millisecond
	
	// Calculate max burst as 2x the rate in the window period
	// For 100ms window: maxBurst = 2 * rate * 0.1
	maxPostBurst := int(float64(postRate) * 0.1 * 2)
	if maxPostBurst < 1 {
		maxPostBurst = 1
	}
	
	maxGetBurst := int(float64(getRate) * 0.1 * 2)
	if maxGetBurst < 1 {
		maxGetBurst = 1
	}

	return &NodeRateLimiter{
		nodeURL:            nodeURL,
		nodeIndex:          nodeIndex,
		postRate:           postRate,
		getRate:            getRate,
		postInterval:       postInterval,
		getInterval:        getInterval,
		nextPostTime:       time.Now(),
		nextGetTime:        time.Now(),
		postCount:          0,
		getCount:           0,
		startTime:          time.Now(),
		recentPostRequests: make([]time.Time, 0, maxPostBurst*2),
		recentGetRequests:  make([]time.Time, 0, maxGetBurst*2),
		windowSize:         windowSize,
		maxPostBurst:       maxPostBurst,
		maxGetBurst:        maxGetBurst,
	}
}

// WaitForPostToken blocks until the next POST token is available
func (nrl *NodeRateLimiter) WaitForPostToken(ctx context.Context) error {
	nrl.mu.Lock()
	defer nrl.mu.Unlock()

	for {
		now := time.Now()
		
		// Clean up old entries from sliding window
		cutoff := now.Add(-nrl.windowSize)
		i := 0
		for i < len(nrl.recentPostRequests) && nrl.recentPostRequests[i].Before(cutoff) {
			i++
		}
		nrl.recentPostRequests = nrl.recentPostRequests[i:]
		
		// Check if we're within burst limit
		if len(nrl.recentPostRequests) < nrl.maxPostBurst {
			// Check token bucket rate limit
			if now.Before(nrl.nextPostTime) {
				waitDuration := nrl.nextPostTime.Sub(now)
				
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
				continue // Re-check burst limit after waiting
			}
			
			// We can proceed - update sliding window
			nrl.recentPostRequests = append(nrl.recentPostRequests, now)
			
			// Calculate next token time
			nrl.nextPostTime = nrl.nextPostTime.Add(nrl.postInterval)
			
			// If we've fallen too far behind (more than 1 second), reset to current time
			if nrl.nextPostTime.Before(now.Add(-time.Second)) {
				nrl.nextPostTime = now.Add(nrl.postInterval)
			}
			
			nrl.postCount++
			return nil
		}
		
		// Burst limit exceeded, wait until the oldest request expires
		oldestRequest := nrl.recentPostRequests[0]
		waitUntil := oldestRequest.Add(nrl.windowSize).Add(time.Millisecond)
		waitDuration := waitUntil.Sub(now)
		
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
		
		// Re-lock and continue loop
		nrl.mu.Lock()
	}
}

// WaitForGetToken blocks until the next GET token is available
func (nrl *NodeRateLimiter) WaitForGetToken(ctx context.Context) error {
	nrl.mu.Lock()
	defer nrl.mu.Unlock()

	for {
		now := time.Now()
		
		// Clean up old entries from sliding window
		cutoff := now.Add(-nrl.windowSize)
		i := 0
		for i < len(nrl.recentGetRequests) && nrl.recentGetRequests[i].Before(cutoff) {
			i++
		}
		nrl.recentGetRequests = nrl.recentGetRequests[i:]
		
		// Check if we're within burst limit
		if len(nrl.recentGetRequests) < nrl.maxGetBurst {
			// Check token bucket rate limit
			if now.Before(nrl.nextGetTime) {
				waitDuration := nrl.nextGetTime.Sub(now)
				
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
				continue // Re-check burst limit after waiting
			}
			
			// We can proceed - update sliding window
			nrl.recentGetRequests = append(nrl.recentGetRequests, now)
			
			// Calculate next token time
			nrl.nextGetTime = nrl.nextGetTime.Add(nrl.getInterval)
			
			// If we've fallen too far behind (more than 1 second), reset to current time
			if nrl.nextGetTime.Before(now.Add(-time.Second)) {
				nrl.nextGetTime = now.Add(nrl.getInterval)
			}
			
			nrl.getCount++
			return nil
		}
		
		// Burst limit exceeded, wait until the oldest request expires
		oldestRequest := nrl.recentGetRequests[0]
		waitUntil := oldestRequest.Add(nrl.windowSize).Add(time.Millisecond)
		waitDuration := waitUntil.Sub(now)
		
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
		
		// Re-lock and continue loop
		nrl.mu.Lock()
	}
}

// GetStats returns statistics for this node's rate limiter
func (nrl *NodeRateLimiter) GetStats() (postCount int64, getCount int64, elapsed time.Duration, actualPostRate float64, actualGetRate float64) {
	nrl.mu.Lock()
	defer nrl.mu.Unlock()

	elapsed = time.Since(nrl.startTime)
	postCount = nrl.postCount
	getCount = nrl.getCount

	if elapsed.Seconds() > 0 {
		actualPostRate = float64(postCount) / elapsed.Seconds()
		actualGetRate = float64(getCount) / elapsed.Seconds()
	}
	return
}

// MultiNodeRateLimiter manages rate limiting across multiple nodes
type MultiNodeRateLimiter struct {
	nodeLimiters  []*NodeRateLimiter
	totalPostRate int
	totalGetRate  int
	nodeCount     int
}

// NewMultiNodeRateLimiter creates a rate limiter that distributes rate across multiple nodes
func NewMultiNodeRateLimiter(nodeURLs []string, totalPostRate int, totalGetRate int) *MultiNodeRateLimiter {
	nodeCount := len(nodeURLs)
	if nodeCount == 0 {
		panic("No nodes provided")
	}

	// Calculate rate per node
	basePostRate := totalPostRate / nodeCount
	postRemainder := totalPostRate % nodeCount

	baseGetRate := totalGetRate / nodeCount
	getRemainder := totalGetRate % nodeCount

	nodeLimiters := make([]*NodeRateLimiter, nodeCount)

	log.Printf("Rate Limiter: %d nodes, POST: %d TPS (%d/node), GET: %d TPS (%d/node)",
		nodeCount, totalPostRate, basePostRate, totalGetRate, baseGetRate)
	log.Printf("Micro-burst prevention: Max burst = 2x rate in 100ms window")

	// Create individual rate limiters for each node
	for i, nodeURL := range nodeURLs {
		nodePostRate := basePostRate
		nodeGetRate := baseGetRate

		// Distribute remainder tokens to first few nodes
		if i < postRemainder {
			nodePostRate++
		}
		if i < getRemainder {
			nodeGetRate++
		}

		nodeLimiters[i] = NewNodeRateLimiter(nodeURL, i, nodePostRate, nodeGetRate)
		
		log.Printf("  Node %d: POST %d TPS (burst %d/100ms), GET %d TPS (burst %d/100ms)",
			i, nodePostRate, nodeLimiters[i].maxPostBurst, nodeGetRate, nodeLimiters[i].maxGetBurst)
	}
	// Rate limiter initialized

	return &MultiNodeRateLimiter{
		nodeLimiters:  nodeLimiters,
		totalPostRate: totalPostRate,
		totalGetRate:  totalGetRate,
		nodeCount:     nodeCount,
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
	log.Println("\nRate Limiter Stats:")
	log.Println("Node | URL            | POST | GET  | Time  | POST TPS | GET TPS")
	log.Println("-----|----------------|------|------|-------|---------|--------")

	totalPostTokens := int64(0)
	totalGetTokens := int64(0)
	var maxElapsed time.Duration

	for i, limiter := range mnrl.nodeLimiters {
		postTokens, getTokens, elapsed, postRate, getRate := limiter.GetStats()
		totalPostTokens += postTokens
		totalGetTokens += getTokens

		if elapsed > maxElapsed {
			maxElapsed = elapsed
		}

		url := limiter.nodeURL
		if len(url) > 20 {
			url = url[:17] + "..."
		}

		log.Printf("%4d | %-14s | %4d | %4d | %5.1fs | %7.1f | %6.1f",
			i, url, postTokens, getTokens, elapsed.Seconds(), postRate, getRate)
	}

	log.Println("-----|----------------|------|------|-------|---------|--------")

	totalActualPostRate := float64(totalPostTokens) / maxElapsed.Seconds()
	totalActualGetRate := float64(totalGetTokens) / maxElapsed.Seconds()

	log.Printf("TOTAL|                | %4d | %4d | %5.1fs | %7.1f | %6.1f",
		totalPostTokens, totalGetTokens, maxElapsed.Seconds(), totalActualPostRate, totalActualGetRate)

	log.Printf("\nActual rates - POST: %.1f/%d TPS, GET: %.1f/%d TPS", 
		totalActualPostRate, mnrl.totalPostRate, totalActualGetRate, mnrl.totalGetRate)
}

