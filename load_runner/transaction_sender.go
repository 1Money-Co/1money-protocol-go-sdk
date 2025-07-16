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
	Verified          bool
	VerificationError error
	TxSuccess         bool
}

func SendTransaction(nodePool *NodePool, rateLimiter *GlobalRateLimiter, account Account, toAddress string, amount string) (*TransactionResult, error) {
	startTime := time.Now()
	result := &TransactionResult{
		WalletIndex: account.WalletIndex,
	}

	// Get client from node pool
	client, nodeURL, err := nodePool.GetNextClient()
	if err != nil {
		result.Error = fmt.Errorf("failed to get client from pool: %w", err)
		result.Duration = time.Since(startTime)
		return result, result.Error
	}

	ctx := context.Background()

	// Apply rate limiting for POST request
	if err := rateLimiter.WaitForPost(ctx); err != nil {
		result.Error = fmt.Errorf("rate limit wait failed: %w", err)
		result.Duration = time.Since(startTime)
		return result, result.Error
	}

	privateKeyHex := strings.TrimPrefix(account.PrivateKey, "0x")
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		result.Error = fmt.Errorf("failed to parse private key: %w", err)
		result.Duration = time.Since(startTime)
		return result, result.Error
	}

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
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
		result.Error = fmt.Errorf("failed to sign payment: %w", err)
		result.Duration = time.Since(startTime)
		return result, result.Error
	}

	paymentReq := &onemoney.PaymentRequest{
		PaymentPayload: payload,
		Signature:      *signature,
	}

	paymentResp, err := client.SendPayment(ctx, paymentReq)
	if err != nil {
		result.Error = fmt.Errorf("failed to send payment to %s: %w", nodeURL, err)
		result.Duration = time.Since(startTime)
		return result, result.Error
	}

	txHash := paymentResp.Hash
	Logf("Transaction %s sent to node: %s\n", txHash, nodeURL)

	result.TxHash = txHash
	result.Success = true
	result.Duration = time.Since(startTime)
	return result, nil
}

func SendTransactionsConcurrently(nodePool *NodePool, rateLimiter *GlobalRateLimiter, accounts []Account, toAddress string, amount string, concurrency int) []TransactionResult {
	var wg sync.WaitGroup
	resultsChan := make(chan TransactionResult, len(accounts))

	// Apply effective concurrency based on rate limits
	effectiveConcurrency := rateLimiter.GetEffectivePostConcurrency(concurrency)
	if effectiveConcurrency != concurrency {
		Logf("Effective concurrency for transactions: %d\n", effectiveConcurrency)
	}
	semaphore := make(chan struct{}, effectiveConcurrency)

	for i, account := range accounts {
		wg.Add(1)
		go func(idx int, acc Account) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			result, _ := SendTransaction(nodePool, rateLimiter, acc, toAddress, amount)
			result.AccountIndex = idx
			resultsChan <- *result
		}(i, account)
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

func VerifyTransactionsConcurrently(nodePool *NodePool, rateLimiter *GlobalRateLimiter, results []TransactionResult, concurrency int) {
	var wg sync.WaitGroup

	// Apply effective concurrency based on rate limits
	effectiveConcurrency := rateLimiter.GetEffectiveGetConcurrency(concurrency)
	if effectiveConcurrency != concurrency {
		Logf("Effective concurrency for verification: %d\n", effectiveConcurrency)
	}
	semaphore := make(chan struct{}, effectiveConcurrency)

	for i := range results {
		if !results[i].Success || results[i].TxHash == "" {
			continue
		}

		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			client, _, _ := nodePool.GetNextClient()
			if client == nil {
				results[idx].VerificationError = fmt.Errorf("no client available")
				return
			}

			// Apply rate limiting for GET request
			ctx := context.Background()
			if err := rateLimiter.WaitForGet(ctx); err != nil {
				results[idx].VerificationError = fmt.Errorf("rate limit wait failed: %w", err)
				return
			}

			success, err := VerifyTransaction(client, results[idx].TxHash)
			results[idx].Verified = true
			results[idx].VerificationError = err
			if err == nil {
				results[idx].TxSuccess = success
			}
		}(i)
	}

	wg.Wait()
}
