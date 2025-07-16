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

func SendTransactionsWithStrictRateLimit(nodePool *BalancedNodePool, accounts []Account, toAddress string, amount string, requestedRate int) []TransactionResult {
	// Create strict rate limiter
	rateLimiter := NewStrictGlobalRateLimiter(nodePool.Size(), requestedRate)
	
	// Results channel
	results := make([]TransactionResult, len(accounts))
	
	// Single worker to ensure strict sequential sending
	Logf("Using single worker for strict rate limiting at %d TPS\n", requestedRate)
	
	startTime := time.Now()
	
	for i, account := range accounts {
		// Wait for rate limit and get node assignment
		ctx := context.Background()
		nodeIndex, err := rateLimiter.WaitAndGetNode(ctx)
		if err != nil {
			results[i] = TransactionResult{
				AccountIndex: i,
				WalletIndex:  account.WalletIndex,
				Error:        fmt.Errorf("rate limit wait failed: %w", err),
				SendTime:     time.Now(),
				ResponseTime: time.Now(),
			}
			continue
		}
		
		// Get client for assigned node
		client, nodeURL := nodePool.GetClientForNode(nodeIndex)
		if client == nil {
			results[i] = TransactionResult{
				AccountIndex: i,
				WalletIndex:  account.WalletIndex,
				Error:        fmt.Errorf("no client available for node %d", nodeIndex),
				SendTime:     time.Now(),
				ResponseTime: time.Now(),
			}
			continue
		}
		
		// Send transaction
		result := sendSingleTransaction(client, nodeURL, nodeIndex, nodePool, account, toAddress, amount)
		result.AccountIndex = i
		results[i] = result
		
		// Log progress every 100 transactions
		if (i+1) % 100 == 0 {
			elapsed := time.Since(startTime)
			actualRate := float64(i+1) / elapsed.Seconds()
			Logf("Progress: %d/%d transactions sent (%.2f TPS actual)\n", i+1, len(accounts), actualRate)
		}
	}
	
	// Print final statistics
	rateLimiter.PrintStats()
	
	return results
}

func sendSingleTransaction(client *onemoney.Client, nodeURL string, nodeIndex int, nodePool *BalancedNodePool, account Account, toAddress string, amount string) TransactionResult {
	startTime := time.Now()
	result := TransactionResult{
		WalletIndex: account.WalletIndex,
		NodeIndex:   nodeIndex,
		NodeURL:     nodePool.GetNodeURL(nodeIndex),
	}
	
	// Increment node count
	nodeCount := nodePool.IncrementNodeCount(nodeIndex)
	result.NodeCount = nodeCount
	
	// Parse private key
	privateKeyHex := strings.TrimPrefix(account.PrivateKey, "0x")
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		result.SendTime = time.Now()
		result.ResponseTime = time.Now()
		result.Error = fmt.Errorf("failed to parse private key: %w", err)
		result.Duration = time.Since(startTime)
		return result
	}
	
	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		result.SendTime = time.Now()
		result.ResponseTime = time.Now()
		result.Error = fmt.Errorf("failed to cast public key to ECDSA")
		result.Duration = time.Since(startTime)
		return result
	}
	
	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
	result.FromAddress = fromAddress.Hex()
	
	// Prepare transaction
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
		return result
	}
	
	paymentReq := &onemoney.PaymentRequest{
		PaymentPayload: payload,
		Signature:      *signature,
	}
	
	// Send transaction
	ctx := context.Background()
	result.SendTime = time.Now()
	paymentResp, err := client.SendPayment(ctx, paymentReq)
	result.ResponseTime = time.Now()
	
	if err != nil {
		result.Error = fmt.Errorf("failed to send payment to %s: %w", nodeURL, err)
		result.Duration = time.Since(startTime)
		return result
	}
	
	result.TxHash = paymentResp.Hash
	result.Success = true
	result.Duration = time.Since(startTime)
	return result
}

func VerifyTransactionsWithStrictRateLimit(nodePool *BalancedNodePool, results []TransactionResult, requestedRate int) {
	// Create rate limiter for verification
	rateLimiter := NewStrictGlobalRateLimiter(nodePool.Size(), requestedRate)
	
	var wg sync.WaitGroup
	verifyQueue := make(chan int, len(results))
	
	// Queue up transactions to verify
	for i := range results {
		if results[i].Success && results[i].TxHash != "" {
			verifyQueue <- i
		}
	}
	close(verifyQueue)
	
	// Use limited workers for verification
	numWorkers := 10
	Logf("Using %d workers for verification at %d TPS\n", numWorkers, requestedRate)
	
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range verifyQueue {
				// Wait for rate limit and get node
				ctx := context.Background()
				nodeIndex, err := rateLimiter.WaitAndGetNode(ctx)
				if err != nil {
					results[idx].VerificationError = fmt.Errorf("rate limit wait failed: %w", err)
					continue
				}
				
				// Get client for node
				client, _ := nodePool.GetClientForNode(nodeIndex)
				if client == nil {
					results[idx].VerificationError = fmt.Errorf("no client available for node %d", nodeIndex)
					continue
				}
				
				// Verify transaction
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
	rateLimiter.PrintStats()
}