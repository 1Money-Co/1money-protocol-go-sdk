package main

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// SendTransactionsMultiNode sends transactions across multiple nodes with per-node rate limiting
func SendTransactionsMultiNode(
	nodePool *BalancedNodePool,
	accounts []Account,
	toAddress string,
	amount string,
	totalRate int,
) []TransactionResult {
	startTime := time.Now()

	// Get node URLs
	nodeURLs := nodePool.GetNodes()
	nodeCount := len(nodeURLs)

	// Create multi-node rate limiter
	rateLimiter := NewMultiNodeRateLimiter(nodeURLs, totalRate)

	// Calculate expected transactions per node
	expectedPerNode := len(accounts) / nodeCount
	remainder := len(accounts) % nodeCount

	Logf("\n=== Transaction Distribution ===\n")
	Logf("Total accounts: %d\n", len(accounts))
	Logf("Accounts per node: %d (remainder: %d)\n", expectedPerNode, remainder)

	// Distribute accounts to nodes
	nodeQueues := make([]chan int, nodeCount)
	for i := 0; i < nodeCount; i++ {
		nodeQueues[i] = make(chan int, expectedPerNode+1)
	}

	// Round-robin distribution of accounts to nodes
	for i, _ := range accounts {
		nodeIndex := i % nodeCount
		nodeQueues[nodeIndex] <- i
	}

	// Close all queues
	for i := 0; i < nodeCount; i++ {
		close(nodeQueues[i])
	}

	// Results channel
	results := make(chan TransactionResult, len(accounts))

	// Create context for cancellation
	ctx := context.Background()

	// WaitGroup for all workers
	var wg sync.WaitGroup

	// Calculate workers per node (minimum 1, maximum 5)
	workersPerNode := 2
	if totalRate/nodeCount > 100 {
		workersPerNode = 3
	}

	Logf("Workers per node: %d\n", workersPerNode)
	Logf("===============================\n")

	// Start worker pools for each node
	for i := 0; i < nodeCount; i++ {
		nodeRateLimiter := rateLimiter.GetNodeRateLimiter(i)
		workerPool := NewNodeWorkerPool(i, nodeURLs[i], nodeRateLimiter, workersPerNode)

		// Process transactions for this node
		workerPool.ProcessTransactions(
			ctx,
			nodePool,
			accounts,
			toAddress,
			amount,
			nodeQueues[i],
			results,
			&wg,
		)
	}

	// Create a separate channel for progress monitoring
	progressChan := make(chan TransactionResult, len(accounts))
	
	// Start progress monitor
	progressDone := make(chan bool)
	go func() {
		monitorProgress(len(accounts), progressChan, startTime)
		close(progressDone)
	}()

	// Wait for all workers to complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results and forward to progress monitor
	allResults := make([]TransactionResult, 0, len(accounts))
	for result := range results {
		allResults = append(allResults, result)
		// Send copy to progress monitor
		select {
		case progressChan <- result:
		default:
			// If progress channel is full, skip (shouldn't happen with buffer)
		}
	}
	
	// Close progress channel and wait for monitor to finish
	close(progressChan)
	<-progressDone

	// Print final statistics
	rateLimiter.PrintStats()

	return allResults
}

// monitorProgress monitors and logs transaction progress
func monitorProgress(totalAccounts int, results <-chan TransactionResult, startTime time.Time) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	processed := int32(0)
	successful := int32(0)
	failed := int32(0)

	// Create a channel to signal when counting is done
	countingDone := make(chan bool)
	
	// Count results in background
	go func() {
		for result := range results {
			atomic.AddInt32(&processed, 1)
			if result.Success {
				atomic.AddInt32(&successful, 1)
			} else {
				atomic.AddInt32(&failed, 1)
			}
		}
		close(countingDone)
	}()

	// Monitor progress until all are processed
	for {
		select {
		case <-ticker.C:
			p := atomic.LoadInt32(&processed)
			s := atomic.LoadInt32(&successful)
			f := atomic.LoadInt32(&failed)

			elapsed := time.Since(startTime)
			rate := float64(p) / elapsed.Seconds()
			
			if p < int32(totalAccounts) {
				remaining := totalAccounts - int(p)
				eta := time.Duration(float64(remaining) / rate * float64(time.Second))
				
				Logf("Progress: %d/%d (%.1f%%) | Success: %d | Failed: %d | Rate: %.2f TPS | ETA: %v\n",
					p, totalAccounts, float64(p)/float64(totalAccounts)*100,
					s, f, rate, eta.Round(time.Second))
			}
		case <-countingDone:
			// Print final progress
			p := atomic.LoadInt32(&processed)
			s := atomic.LoadInt32(&successful)
			f := atomic.LoadInt32(&failed)
			
			elapsed := time.Since(startTime)
			rate := float64(p) / elapsed.Seconds()
			
			Logf("Progress: %d/%d (%.1f%%) | Success: %d | Failed: %d | Rate: %.2f TPS | Completed\n",
				p, totalAccounts, float64(p)/float64(totalAccounts)*100,
				s, f, rate)
			return
		}
	}
}

// VerifyTransactionsMultiNode verifies transactions with per-node rate limiting
func VerifyTransactionsMultiNode(
	nodePool *BalancedNodePool,
	results []TransactionResult,
	totalRate int,
) {
	// Get node URLs
	nodeURLs := nodePool.GetNodes()
	nodeCount := len(nodeURLs)

	// Create multi-node rate limiter for verification
	rateLimiter := NewMultiNodeRateLimiter(nodeURLs, totalRate)

	// Count transactions to verify
	toVerify := 0
	for i := range results {
		if results[i].Success && results[i].TxHash != "" {
			toVerify++
		}
	}

	Logf("\n=== Verification Configuration ===\n")
	Logf("Transactions to verify: %d\n", toVerify)
	Logf("Verification rate: %d TPS total (%d TPS per node)\n", totalRate, totalRate/nodeCount)

	// Create work queues for each node
	nodeQueues := make([]chan int, nodeCount)
	for i := 0; i < nodeCount; i++ {
		nodeQueues[i] = make(chan int, toVerify/nodeCount+1)
	}

	// Distribute verification work to nodes (round-robin)
	nodeIndex := 0
	for i := range results {
		if results[i].Success && results[i].TxHash != "" {
			nodeQueues[nodeIndex] <- i
			nodeIndex = (nodeIndex + 1) % nodeCount
		}
	}

	// Close all queues
	for i := 0; i < nodeCount; i++ {
		close(nodeQueues[i])
	}

	// Create context
	ctx := context.Background()

	// WaitGroup for all workers
	var wg sync.WaitGroup

	// Start verification workers for each node
	workersPerNode := 3

	for i := 0; i < nodeCount; i++ {
		nodeRateLimiter := rateLimiter.GetNodeRateLimiter(i)

		for w := 0; w < workersPerNode; w++ {
			wg.Add(1)
			go func(nodeIdx int) {
				defer wg.Done()

				for resultIdx := range nodeQueues[nodeIdx] {
					// Wait for rate limit token
					if err := nodeRateLimiter.WaitForToken(ctx); err != nil {
						results[resultIdx].VerificationError = fmt.Errorf("rate limit wait failed: %w", err)
						continue
					}

					// Get client for this node
					client, _ := nodePool.GetClientForNode(nodeIdx)
					if client == nil {
						results[resultIdx].VerificationError = fmt.Errorf("no client available for node %d", nodeIdx)
						continue
					}

					// Verify transaction
					success, err := VerifyTransaction(client, results[resultIdx].TxHash)
					results[resultIdx].Verified = true
					results[resultIdx].VerificationError = err
					if err == nil {
						results[resultIdx].TxSuccess = success
					}
				}
			}(i)
		}
	}

	// Wait for all verification to complete
	wg.Wait()

	// Print verification statistics
	rateLimiter.PrintStats()
}
