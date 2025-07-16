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

const (
	// HardcodedChainID is the fixed chain ID for 1Money network
	// This avoids REST API calls to get chain ID
	HardcodedChainID = 1212101
)

type TransactionResult struct {
	AccountIndex      int
	WalletIndex       string
	FromAddress       string
	TxHash            string
	Success           bool
	Error             error
	Duration          time.Duration
	SendTime          time.Time    // When the transaction was sent
	ResponseTime      time.Time    // When the response was received
	Verified          bool
	VerificationError error
	TxSuccess         bool
	NodeIndex         int          // Which node was used
	NodeURL           string       // Node URL for logging
	NodeCount         int64        // Count for this specific node
}

func SendTransaction(nodePool *BalancedNodePool, rateLimiter PerNodeRateLimiterInterface, account Account, toAddress string, amount string) (*TransactionResult, error) {
	startTime := time.Now()
	result := &TransactionResult{
		WalletIndex: account.WalletIndex,
	}

	// Get client from node pool
	client, nodeURL, nodeIndex, nodeCount, err := nodePool.GetNextClientForSend()
	if err != nil {
		result.SendTime = time.Now() // Mark attempt time
		result.ResponseTime = time.Now() // Same as send time for immediate failures
		result.Error = fmt.Errorf("failed to get client from pool: %w", err)
		result.Duration = time.Since(startTime)
		result.NodeCount = 0 // No node assigned yet
		return result, result.Error
	}
	
	result.NodeIndex = nodeIndex
	result.NodeURL = nodePool.GetNodeURL(nodeIndex)
	result.NodeCount = nodeCount

	ctx := context.Background()

	// Apply rate limiting for POST request for this specific node
	if err := rateLimiter.WaitForPost(ctx, nodeIndex); err != nil {
		result.SendTime = time.Now() // Mark attempt time
		result.ResponseTime = time.Now() // Same as send time for immediate failures
		result.Error = fmt.Errorf("rate limit wait failed: %w", err)
		result.Duration = time.Since(startTime)
		return result, result.Error
	}

	privateKeyHex := strings.TrimPrefix(account.PrivateKey, "0x")
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		result.SendTime = time.Now() // Mark attempt time
		result.ResponseTime = time.Now() // Same as send time for immediate failures
		result.Error = fmt.Errorf("failed to parse private key: %w", err)
		result.Duration = time.Since(startTime)
		return result, result.Error
	}

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		result.SendTime = time.Now() // Mark attempt time
		result.ResponseTime = time.Now() // Same as send time for immediate failures
		result.Error = fmt.Errorf("failed to cast public key to ECDSA")
		result.Duration = time.Since(startTime)
		return result, result.Error
	}

	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
	result.FromAddress = fromAddress.Hex()

	// Use hardcoded chainId to avoid API call

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
		result.SendTime = time.Now() // Mark attempt time
		result.ResponseTime = time.Now() // Same as send time for immediate failures
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

func SendTransactionsConcurrently(nodePool *BalancedNodePool, rateLimiter PerNodeRateLimiterInterface, accounts []Account, toAddress string, amount string, concurrency int) []TransactionResult {
	var wg sync.WaitGroup
	resultsChan := make(chan TransactionResult, len(accounts))

	// Log rate limiting info
	effectiveConcurrency := rateLimiter.GetEffectivePostConcurrency(concurrency)
	if effectiveConcurrency != concurrency {
		Logf("Effective rate limit for transactions: %d TPS\n", effectiveConcurrency)
	}
	
	// Use a smaller worker pool to prevent thundering herd
	// Workers should be limited to prevent too many concurrent rate limit waits
	numWorkers := effectiveConcurrency / 10
	if numWorkers < 10 {
		numWorkers = 10
	}
	if numWorkers > 100 {
		numWorkers = 100
	}
	
	Logf("Using %d workers for %d TPS rate limit\n", numWorkers, effectiveConcurrency)
	
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
				result, _ := SendTransaction(nodePool, rateLimiter, accounts[idx], toAddress, amount)
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

func VerifyTransaction(client *onemoney.Client, txHash string) (bool, error) {
	ctx := context.Background()
	receipt, err := client.GetTransactionReceipt(ctx, txHash)
	if err != nil {
		return false, err
	}
	return receipt.Success, nil
}

func VerifyTransactionsConcurrently(nodePool *BalancedNodePool, rateLimiter PerNodeRateLimiterInterface, results []TransactionResult, concurrency int) {
	var wg sync.WaitGroup

	// Log rate limiting info
	effectiveConcurrency := rateLimiter.GetEffectiveGetConcurrency(concurrency)
	if effectiveConcurrency != concurrency {
		Logf("Effective rate limit for verification: %d TPS\n", effectiveConcurrency)
	}
	
	// Use a smaller worker pool for verification too
	numWorkers := effectiveConcurrency / 10
	if numWorkers < 20 {
		numWorkers = 20
	}
	if numWorkers > 200 {
		numWorkers = 200
	}
	
	Logf("Using %d workers for verification at %d TPS\n", numWorkers, effectiveConcurrency)
	
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

				client, _, nodeIndex, err := nodePool.GetNextClientForVerify()
				if err != nil {
					results[idx].VerificationError = fmt.Errorf("no client available: %w", err)
					continue
				}

				// Apply rate limiting for GET request for this specific node
				ctx := context.Background()
				if err := rateLimiter.WaitForGet(ctx, nodeIndex); err != nil {
					results[idx].VerificationError = fmt.Errorf("rate limit wait failed: %w", err)
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
