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

	logger, err := InitLogger()
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Close()

	var client *onemoney.Client
	if *useTestnet {
		client = onemoney.NewTestClient()
		Logln("Using Test Network")
	} else {
		client = onemoney.NewClient()
		Logln("Using Main Network")
	}

	Logln("\n=== 1Money Load Runner ===")
	Logf("CSV File: %s\n", *csvFile)
	Logf("Target Address: %s\n", *toAddress)
	Logf("Amount per TX: %s\n", *amount)
	Logf("Concurrency: %d\n", *concurrency)
	Logln("=======================\n")

	accounts, err := ReadAccountsFromCSV(*csvFile)
	if err != nil {
		log.Fatalf("Failed to read CSV: %v", err)
	}

	if *maxAccounts > 0 && *maxAccounts < len(accounts) {
		accounts = accounts[:*maxAccounts]
	}

	Logf("Loaded %d accounts from CSV\n", len(accounts))
	Logln("Starting transaction sending...\n")

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
			Logf("✅ Account %s (Wallet #%s): TX %s (%.2fs)\n",
				result.FromAddress, result.WalletIndex, result.TxHash, result.Duration.Seconds())
		} else {
			failCount++
			Logf("❌ Account %s (Wallet #%s): %v\n",
				result.FromAddress, result.WalletIndex, result.Error)
		}
	}

	Logln("\n=== Summary ===")
	Logf("Total Accounts: %d\n", len(accounts))
	Logf("Successful: %d\n", successCount)
	Logf("Failed: %d\n", failCount)
	Logf("Success Rate: %.2f%%\n", float64(successCount)/float64(len(accounts))*100)
	Logf("Total Time: %.2fs\n", totalDuration.Seconds())
	if successCount > 0 {
		Logf("Avg TX Time: %.2fs\n", totalTxTime.Seconds()/float64(successCount))
		Logf("TPS: %.2f\n", float64(successCount)/totalDuration.Seconds())
	}

	if successCount > 0 {
		Logln("\n⏳ Waiting 10 seconds before verifying transactions...")
		time.Sleep(10 * time.Second)

		Logln("\n🔍 Verifying transaction receipts...\n")
		verifyStart := time.Now()
		VerifyTransactionsConcurrently(client, results, *concurrency)
		verifyDuration := time.Since(verifyStart)

		verifiedCount := 0
		txSuccessCount := 0
		for _, result := range results {
			if result.Verified {
				verifiedCount++
				if result.TxSuccess {
					txSuccessCount++
					Logf("✅ TX %s: Confirmed successful\n", result.TxHash)
				} else {
					Logf("❌ TX %s: Transaction failed on chain\n", result.TxHash)
				}
			} else if result.Success && result.VerificationError != nil {
				Logf("⚠️  TX %s: Verification error: %v\n", result.TxHash, result.VerificationError)
			}
		}

		Logln("\n=== Verification Summary ===")
		Logf("Verified: %d/%d\n", verifiedCount, successCount)
		Logf("On-chain Success: %d\n", txSuccessCount)
		Logf("On-chain Failed: %d\n", verifiedCount-txSuccessCount)
		Logf("Verification Time: %.2fs\n", verifyDuration.Seconds())
	}

	if err := WriteResultsToCSV(results); err != nil {
		Logf("Failed to write results CSV: %v\n", err)
	} else {
		Logf("\nResults saved to: load_results_%s.csv\n", time.Now().Format("20060102_150405"))
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

	fmt.Fprintf(file, "wallet_index,from_address,tx_hash,success,error,duration_ms,verified,tx_success,verification_error\n")
	for _, result := range results {
		errorStr := ""
		if result.Error != nil {
			errorStr = result.Error.Error()
		}
		verifyErrorStr := ""
		if result.VerificationError != nil {
			verifyErrorStr = result.VerificationError.Error()
		}
		fmt.Fprintf(file, "%s,%s,%s,%t,%s,%.0f,%t,%t,%s\n",
			result.WalletIndex,
			result.FromAddress,
			result.TxHash,
			result.Success,
			errorStr,
			result.Duration.Milliseconds(),
			result.Verified,
			result.TxSuccess,
			verifyErrorStr,
		)
	}

	absPath, _ := filepath.Abs(filename)
	Logf("Results written to: %s\n", absPath)
	return nil
}
