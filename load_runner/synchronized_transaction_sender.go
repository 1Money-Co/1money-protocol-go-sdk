package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	onemoney "github.com/1Money-Co/1money-go-sdk"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func SendTransactionSynchronized(nodePool *BalancedNodePool, nodeIndex int, account Account, toAddress string, amount string) (*TransactionResult, error) {
	startTime := time.Now()
	result := &TransactionResult{
		WalletIndex: account.WalletIndex,
	}

	// Get specific client for this node
	client, nodeURL := nodePool.GetClientForNode(nodeIndex)
	if client == nil {
		result.SendTime = time.Now()
		result.ResponseTime = time.Now()
		result.Error = fmt.Errorf("no client available for node %d", nodeIndex)
		result.Duration = time.Since(startTime)
		result.NodeCount = 0
		return result, result.Error
	}
	
	result.NodeIndex = nodeIndex
	result.NodeURL = nodePool.GetNodeURL(nodeIndex)
	
	// Increment node count for this specific node
	nodeCount := nodePool.IncrementNodeCount(nodeIndex)
	result.NodeCount = nodeCount

	ctx := context.Background()

	privateKeyHex := strings.TrimPrefix(account.PrivateKey, "0x")
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		result.SendTime = time.Now()
		result.ResponseTime = time.Now()
		result.Error = fmt.Errorf("failed to parse private key: %w", err)
		result.Duration = time.Since(startTime)
		return result, result.Error
	}

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		result.SendTime = time.Now()
		result.ResponseTime = time.Now()
		result.Error = fmt.Errorf("failed to cast public key to ECDSA")
		result.Duration = time.Since(startTime)
		return result, result.Error
	}

	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
	result.FromAddress = fromAddress.Hex()

	amountBig := new(big.Int)
	amountBig.SetString(amount, 10)

	payload := onemoney.PaymentPayload{
		ChainID:   HardcodedChainID,
		Nonce:     uint64(0),
		Recipient: common.HexToAddress(toAddress),
		Value:     amountBig,
		Token:     common.HexToAddress(account.TokenAddress),
	}

	signature, err := client.SignMessage(payload, account.PrivateKey)
	if err != nil {
		result.SendTime = time.Now()
		result.ResponseTime = time.Now()
		result.Error = fmt.Errorf("failed to sign payment: %w", err)
		result.Duration = time.Since(startTime)
		return result, result.Error
	}

	paymentReq := &onemoney.PaymentRequest{
		PaymentPayload: payload,
		Signature:      *signature,
	}

	// Capture send time
	result.SendTime = time.Now()
	paymentResp, err := client.SendPayment(ctx, paymentReq)
	result.ResponseTime = time.Now()
	
	if err != nil {
		result.Error = fmt.Errorf("failed to send payment to %s: %w", nodeURL, err)
		result.Duration = time.Since(startTime)
		return result, result.Error
	}

	txHash := paymentResp.Hash

	result.TxHash = txHash
	result.Success = true
	result.Duration = time.Since(startTime)
	return result, nil
}

func SendTransactionsConcurrentlySynchronized(nodePool *BalancedNodePool, rateLimiter *SynchronizedGlobalRateLimiter, accounts []Account, toAddress string, amount string, concurrency int) []TransactionResult {
	var wg sync.WaitGroup
	resultsChan := make(chan TransactionResult, len(accounts))

	// Log rate limiting info
	effectiveConcurrency := rateLimiter.GetEffectivePostConcurrency(concurrency)
	if effectiveConcurrency != concurrency {
		Logf("Effective rate limit for transactions: %d TPS\n", effectiveConcurrency)
	}
	
	// Use a smaller worker pool to prevent thundering herd
	// For synchronized rate limiting, we can use fewer workers
	numWorkers := 20 // Fixed small number of workers
	
	Logf("Using %d workers for synchronized %d TPS rate limit\n", numWorkers, effectiveConcurrency)
	
	// Create work queue
	workQueue := make(chan int, len(accounts))
	for i := range accounts {
		workQueue <- i
	}
	close(workQueue)

	// Start workers
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range workQueue {
				// Wait for rate limit and get which node to use
				ctx := context.Background()
				nodeIndex, err := rateLimiter.WaitForPostAndGetNode(ctx)
				if err != nil {
					result := TransactionResult{
						AccountIndex: idx,
						WalletIndex: accounts[idx].WalletIndex,
						Error: fmt.Errorf("rate limit wait failed: %w", err),
						SendTime: time.Now(),
						ResponseTime: time.Now(),
					}
					resultsChan <- result
					continue
				}
				
				// Send to the specific node determined by rate limiter
				result, _ := SendTransactionSynchronized(nodePool, nodeIndex, accounts[idx], toAddress, amount)
				result.AccountIndex = idx
				resultsChan <- *result
			}
		}()
	}

	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	var results []TransactionResult
	for result := range resultsChan {
		results = append(results, result)
	}

	return results
}

func VerifyTransactionsConcurrentlySynchronized(nodePool *BalancedNodePool, rateLimiter *SynchronizedGlobalRateLimiter, results []TransactionResult, concurrency int) {
	var wg sync.WaitGroup

	// Log rate limiting info
	effectiveConcurrency := rateLimiter.GetEffectiveGetConcurrency(concurrency)
	if effectiveConcurrency != concurrency {
		Logf("Effective rate limit for verification: %d TPS\n", effectiveConcurrency)
	}
	
	// Use a smaller worker pool for verification
	numWorkers := 40 // Fixed number for verification
	
	Logf("Using %d workers for synchronized verification at %d TPS\n", numWorkers, effectiveConcurrency)
	
	// Create work queue for indices to verify
	workQueue := make(chan int, len(results))
	for i := range results {
		if results[i].Success && results[i].TxHash != "" {
			workQueue <- i
		}
	}
	close(workQueue)

	// Start workers
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range workQueue {
				// Wait for rate limit and get which node to use
				ctx := context.Background()
				nodeIndex, err := rateLimiter.WaitForGetAndGetNode(ctx)
				if err != nil {
					results[idx].VerificationError = fmt.Errorf("rate limit wait failed: %w", err)
					continue
				}
				
				// Get client for specific node
				client, _ := nodePool.GetClientForNode(nodeIndex)
				if client == nil {
					results[idx].VerificationError = fmt.Errorf("no client available for node %d", nodeIndex)
					continue
				}

				success, err := VerifyTransaction(client, results[idx].TxHash)
				results[idx].Verified = true
				results[idx].VerificationError = err
				if err == nil {
					results[idx].TxSuccess = success
				}
			}
		}()
	}

	wg.Wait()
}