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

type TransactionResult struct {
	AccountIndex int
	WalletIndex  string
	FromAddress  string
	TxHash       string
	Success      bool
	Error        error
	Duration     time.Duration
}

func SendTransaction(client *onemoney.Client, account Account, toAddress string, amount string) (*TransactionResult, error) {
	startTime := time.Now()
	result := &TransactionResult{
		WalletIndex: account.WalletIndex,
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

	ctx := context.Background()

	chainIDResp, err := client.GetChainId(ctx)
	if err != nil {
		result.Error = fmt.Errorf("failed to get chain ID: %w", err)
		result.Duration = time.Since(startTime)
		return result, result.Error
	}

	amountBig := new(big.Int)
	amountBig.SetString(amount, 10)

	payload := onemoney.PaymentPayload{
		ChainID:   uint64(chainIDResp.ChainId),
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
		result.Error = fmt.Errorf("failed to send payment: %w", err)
		result.Duration = time.Since(startTime)
		return result, result.Error
	}

	txHash := paymentResp.Hash

	result.TxHash = txHash
	result.Success = true
	result.Duration = time.Since(startTime)
	return result, nil
}

func SendTransactionsConcurrently(client *onemoney.Client, accounts []Account, toAddress string, amount string, concurrency int) []TransactionResult {
	var wg sync.WaitGroup
	resultsChan := make(chan TransactionResult, len(accounts))
	semaphore := make(chan struct{}, concurrency)

	for i, account := range accounts {
		wg.Add(1)
		go func(idx int, acc Account) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			result, _ := SendTransaction(client, acc, toAddress, amount)
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
