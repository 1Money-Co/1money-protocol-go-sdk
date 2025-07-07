package main

import (
	"fmt"
	"log"
	"time"
)

// waitForTransactionReceipt waits for transaction receipt and validates success
func (st *StressTester) waitForTransactionReceipt(txHash string) error {
	timeout := time.After(RECEIPT_CHECK_TIMEOUT)
	ticker := time.NewTicker(RECEIPT_CHECK_INTERVAL)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for receipt of transaction %s", txHash)
		case <-ticker.C:
			// Apply rate limiting for GET request
			if err := st.getRateLimiter.Wait(st.ctx); err != nil {
				log.Printf("Rate limiting failed for GetTransactionReceipt: %v", err)
				continue
			}

			receipt, err := st.client.GetTransactionReceipt(st.ctx, txHash)
			if err != nil {
				log.Printf("Error getting receipt for transaction %s: %v", txHash, err)
				continue
			}

			if receipt != nil {
				if receipt.Success {
					log.Printf("Transaction %s confirmed successfully", txHash)
					return nil
				} else {
					return fmt.Errorf("transaction %s failed", txHash)
				}
			}
		}
	}
}

// validateNonceIncrement validates that the nonce has incremented by 1 after a transaction
func (st *StressTester) validateNonceIncrement(address string, expectedNonce uint64) error {
	timeout := time.After(NONCE_VALIDATION_TIMEOUT)
	ticker := time.NewTicker(NONCE_CHECK_INTERVAL)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for nonce validation for address %s", address)
		case <-ticker.C:
			// Apply rate limiting for GET request
			if err := st.getRateLimiter.Wait(st.ctx); err != nil {
				log.Printf("Rate limiting failed for GetAccountNonce in nonce validation: %v", err)
				continue
			}

			accountNonce, err := st.client.GetAccountNonce(st.ctx, address)
			if err != nil {
				log.Printf("Error getting nonce for address %s: %v", address, err)
				continue
			}

			if accountNonce.Nonce == expectedNonce {
				log.Printf("Nonce validated for address %s: %d", address, accountNonce.Nonce)
				return nil
			}

			if accountNonce.Nonce > expectedNonce {
				return fmt.Errorf("nonce validation failed for address %s: expected %d, got %d", address, expectedNonce, accountNonce.Nonce)
			}
		}
	}
}

// getAccountNonce retrieves the current nonce for an address
func (st *StressTester) getAccountNonce(address string) (uint64, error) {
	// Apply rate limiting for GET request
	if err := st.getRateLimiter.Wait(st.ctx); err != nil {
		return 0, fmt.Errorf("rate limiting failed for GetAccountNonce: %w", err)
	}

	accountNonce, err := st.client.GetAccountNonce(st.ctx, address)
	if err != nil {
		return 0, fmt.Errorf("failed to get account nonce for %s: %w", address, err)
	}
	return accountNonce.Nonce, nil
}
