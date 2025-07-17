package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	onemoney "github.com/1Money-Co/1money-go-sdk"
)

// generateAccountsDetailCSV generates a CSV file with account details for all wallet tiers (multi-node compatible)
func (st *StressTester) generateAccountsDetailCSV(timestamp string) error {
	csvFileName := fmt.Sprintf("accounts_detail_%s.csv", timestamp)

	// Create CSV file
	file, err := os.Create(csvFileName)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer file.Close()

	// Create CSV writer
	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write CSV header with additional columns for multi-tier tracking
	header := []string{"privatekey", "token_address", "decimal", "balance", "wallet_tier", "wallet_index", "source_wallet"}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	log.Printf("Generating accounts detail CSV file: %s", csvFileName)
	totalWallets := len(st.transferWallets) + len(st.distributionWallets)
	log.Printf("Collecting balance information for %d wallets...", totalWallets)
	log.Printf("CSV balance query rate limit: %d queries/second", st.csvRateLimit)

	processedCount := 0

	// Create rate limiter for CSV balance queries
	rateLimiter := time.NewTicker(time.Second / time.Duration(st.csvRateLimit))
	defer rateLimiter.Stop()

	// Write data for transfer wallets
	log.Printf("Processing transfer wallets...")
	for i, wallet := range st.transferWallets {
		// Wait for rate limiter
		<-rateLimiter.C

		// Get a node for GET operation
		client, _, _, err := st.nodePool.GetNodeForGet()
		if err != nil {
			log.Printf("Failed to get node for balance check (wallet %d): %v", i+1, err)
			continue
		}

		// Get token account balance
		startTime := time.Now()

		tokenAccount, err := client.GetTokenAccount(st.ctx, wallet.Address, st.tokenAddress)
		queryDuration := time.Since(startTime)

		if err != nil {
			// Failed to get balance
			log.Printf("⚠️  CSV WARNING: GetTokenAccount failed | Wallet: %d | Address: %s | Token: %s | Duration: %v | Error: %v | Using zero balance", i+1, wallet.Address, st.tokenAddress, queryDuration, err)
			// Continue with zero balance if account doesn't exist or has error
			tokenAccount = &onemoney.TokenAccountResponse{Balance: "0"}
		}

		// Prepare CSV row for transfer wallet
		row := []string{
			"0x" + wallet.PrivateKey,
			st.tokenAddress,
			strconv.Itoa(int(TOKEN_DECIMALS)),
			tokenAccount.Balance,
			"transfer",
			strconv.Itoa(i + 1),
			"mint_wallet", // Transfer wallets are funded by mint wallets
		}

		// Write row to CSV
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("failed to write CSV row for wallet %d: %w", i+1, err)
		}

		processedCount++
		// Log progress every CSV_PROGRESS_INTERVAL_WALLETS wallets
		if processedCount%CSV_PROGRESS_INTERVAL_WALLETS == 0 {
			log.Printf("Processed %d/%d total wallets for CSV generation", processedCount, totalWallets)
		}
	}

	log.Printf("Processing distribution wallets...")
	for i, wallet := range st.distributionWallets {
		// Wait for rate limiter
		<-rateLimiter.C

		// Get a node for GET operation
		client, _, _, err := st.nodePool.GetNodeForGet()
		if err != nil {
			log.Printf("Failed to get node for balance check (dist wallet %d): %v", i+1, err)
			continue
		}

		// Get token account balance
		startTime := time.Now()

		tokenAccount, err := client.GetTokenAccount(st.ctx, wallet.Address, st.tokenAddress)
		queryDuration := time.Since(startTime)

		if err != nil {
			// Failed to get balance
			log.Printf("⚠️  CSV WARNING: GetTokenAccount failed | Dist Wallet: %d | Address: %s | Token: %s | Duration: %v | Error: %v | Using zero balance", i+1, wallet.Address, st.tokenAddress, queryDuration, err)
			// Continue with zero balance if account doesn't exist or has error
			tokenAccount = &onemoney.TokenAccountResponse{Balance: "0"}
		}

		// Calculate which transfer wallet this distribution wallet belongs to
		transferWalletIndex := i / DIST_WALLETS_PER_TRANSFER

		// Prepare CSV row for distribution wallet
		row := []string{
			"0x" + wallet.PrivateKey,
			st.tokenAddress,
			strconv.Itoa(int(TOKEN_DECIMALS)),
			tokenAccount.Balance,
			"distribution",
			strconv.Itoa(i + 1),
			fmt.Sprintf("transfer_wallet_%d", transferWalletIndex+1), // Source is the transfer wallet that sent tokens
		}

		// Write row to CSV
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("failed to write CSV row for dist wallet %d: %w", i+1, err)
		}

		processedCount++
		// Log progress every CSV_PROGRESS_INTERVAL_DIST wallets
		if processedCount%CSV_PROGRESS_INTERVAL_DIST == 0 {
			log.Printf("Processed %d/%d total wallets for CSV generation", processedCount, totalWallets)
		}
	}

	log.Printf("✓ CSV generated: %s (%d entries)", csvFileName, processedCount)

	return nil
}
