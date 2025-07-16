package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"time"

	onemoney "github.com/1Money-Co/1money-go-sdk"
)

var (
	csvFile     = flag.String("csv", "../stress_test/accounts_detail.csv", "Path to CSV file containing account details")
	toAddress   = flag.String("to", "", "Target address to send transactions to (required)")
	amount      = flag.String("amount", "1000000", "Amount to send in each transaction")
	concurrency = flag.Int("concurrency", 10, "Number of concurrent transactions")
	useTestnet  = flag.Bool("testnet", true, "Use testnet (true) or mainnet (false)")
	maxAccounts = flag.Int("max", 0, "Maximum number of accounts to process (0 = all)")
)

func main() {
	flag.Parse()

	if *toAddress == "" {
		log.Fatal("Target address (-to) is required")
	}

	if _, err := os.Stat(*csvFile); os.IsNotExist(err) {
		log.Fatalf("CSV file not found: %s", *csvFile)
	}

	var client *onemoney.Client
	if *useTestnet {
		client = onemoney.NewTestClient()
		fmt.Println("Using Test Network")
	} else {
		client = onemoney.NewClient()
		fmt.Println("Using Main Network")
	}

	fmt.Printf("\n=== 1Money Load Runner ===\n")
	fmt.Printf("CSV File: %s\n", *csvFile)
	fmt.Printf("Target Address: %s\n", *toAddress)
	fmt.Printf("Amount per TX: %s\n", *amount)
	fmt.Printf("Concurrency: %d\n", *concurrency)
	fmt.Printf("=======================\n\n")

	accounts, err := ReadAccountsFromCSV(*csvFile)
	if err != nil {
		log.Fatalf("Failed to read CSV: %v", err)
	}

	if *maxAccounts > 0 && *maxAccounts < len(accounts) {
		accounts = accounts[:*maxAccounts]
	}

	fmt.Printf("Loaded %d accounts from CSV\n", len(accounts))
	fmt.Printf("Starting transaction sending...\n\n")

	startTime := time.Now()
	results := SendTransactionsConcurrently(client, accounts, *toAddress, *amount, *concurrency)
	totalDuration := time.Since(startTime)

	successCount := 0
	failCount := 0
	var totalTxTime time.Duration

	for _, result := range results {
		if result.Success {
			successCount++
			totalTxTime += result.Duration
			fmt.Printf("✅ Account %s (Wallet #%s): TX %s (%.2fs)\n",
				result.FromAddress, result.WalletIndex, result.TxHash, result.Duration.Seconds())
		} else {
			failCount++
			fmt.Printf("❌ Account %s (Wallet #%s): %v\n",
				result.FromAddress, result.WalletIndex, result.Error)
		}
	}

	fmt.Printf("\n=== Summary ===\n")
	fmt.Printf("Total Accounts: %d\n", len(accounts))
	fmt.Printf("Successful: %d\n", successCount)
	fmt.Printf("Failed: %d\n", failCount)
	fmt.Printf("Success Rate: %.2f%%\n", float64(successCount)/float64(len(accounts))*100)
	fmt.Printf("Total Time: %.2fs\n", totalDuration.Seconds())
	if successCount > 0 {
		fmt.Printf("Avg TX Time: %.2fs\n", totalTxTime.Seconds()/float64(successCount))
		fmt.Printf("TPS: %.2f\n", float64(successCount)/totalDuration.Seconds())
	}

	if err := WriteResultsToCSV(results); err != nil {
		log.Printf("Failed to write results CSV: %v", err)
	} else {
		fmt.Printf("\nResults saved to: load_results_%s.csv\n", time.Now().Format("20060102_150405"))
	}
}

func WriteResultsToCSV(results []TransactionResult) error {
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("load_results_%s.csv", timestamp)

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	sort.Slice(results, func(i, j int) bool {
		return results[i].AccountIndex < results[j].AccountIndex
	})

	fmt.Fprintf(file, "wallet_index,from_address,tx_hash,success,error,duration_ms\n")
	for _, result := range results {
		errorStr := ""
		if result.Error != nil {
			errorStr = result.Error.Error()
		}
		fmt.Fprintf(file, "%s,%s,%s,%t,%s,%.0f\n",
			result.WalletIndex,
			result.FromAddress,
			result.TxHash,
			result.Success,
			errorStr,
			result.Duration.Milliseconds(),
		)
	}

	absPath, _ := filepath.Abs(filename)
	fmt.Printf("Results written to: %s\n", absPath)
	return nil
}
