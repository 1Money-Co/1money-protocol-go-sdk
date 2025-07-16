package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"time"
)

var (
	csvFile     = flag.String("csv", "../stress_test/accounts_detail.csv", "Path to CSV file containing account details")
	toAddress   = flag.String("to", "", "Target address to send transactions to (required)")
	amount      = flag.String("amount", "1000000", "Amount to send in each transaction")
	concurrency = flag.Int("concurrency", 10, "Number of concurrent transactions")
	useTestnet  = flag.Bool("testnet", true, "Use testnet (true) or mainnet (false)")
	maxAccounts = flag.Int("max", 0, "Maximum number of accounts to process (0 = all)")
	nodeList    = flag.String("nodes", "", "Comma-separated list of node URLs (e.g. '192.168.1.1:8080,192.168.1.2:8080')")
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

	// Initialize node pool
	nodePool := NewNodePool()

	if *nodeList != "" {
		// Use custom nodes
		nodeURLs, err := ParseNodeURLs(*nodeList)
		if err != nil {
			log.Fatalf("Failed to parse node list: %v", err)
		}

		for _, url := range nodeURLs {
			if err := nodePool.AddNode(url); err != nil {
				log.Fatalf("Failed to add node %s: %v", url, err)
			}
		}
		Logf("Using %d custom nodes\n", nodePool.Size())
	} else {
		// Use default test/main network nodes
		if *useTestnet {
			// Add default testnet nodes (you can configure these)
			defaultNodes := []string{
				"https://testapi.1moneynetwork.com",
				"https://testapi2.1moneynetwork.com",
				"https://testapi3.1moneynetwork.com",
				"https://testapi4.1moneynetwork.com",
			}
			for _, url := range defaultNodes {
				nodePool.AddNode(url)
			}
			Logln("Using Test Network nodes")
		} else {
			// Add default mainnet nodes
			defaultNodes := []string{
				"https://api.1moneynetwork.com",
				"https://api2.1moneynetwork.com",
				"https://api3.1moneynetwork.com",
				"https://api4.1moneynetwork.com",
			}
			for _, url := range defaultNodes {
				nodePool.AddNode(url)
			}
			Logln("Using Main Network nodes")
		}
	}

	Logln("\n=== 1Money Load Runner ===")
	Logf("CSV File: %s\n", *csvFile)
	Logf("Target Address: %s\n", *toAddress)
	Logf("Amount per TX: %s\n", *amount)
	Logf("Concurrency: %d\n", *concurrency)
	Logf("Node Count: %d\n", nodePool.Size())
	Logln("Nodes:")
	for i, node := range nodePool.GetNodes() {
		Logf("  [%d] %s\n", i+1, node)
	}
	Logln("=======================")

	accounts, err := ReadAccountsFromCSV(*csvFile)
	if err != nil {
		log.Fatalf("Failed to read CSV: %v", err)
	}

	if *maxAccounts > 0 && *maxAccounts < len(accounts) {
		accounts = accounts[:*maxAccounts]
	}

	Logf("Loaded %d accounts from CSV\n", len(accounts))
	
	// Create rate limiter based on node count
	rateLimiter := NewGlobalRateLimiter(nodePool.Size())
	defer rateLimiter.Close()
	
	Logf("Rate limits: POST %d TPS/node, GET %d TPS/node\n", PostRateLimitPerNode, GetRateLimitPerNode)
	Logf("Total rate limits: POST %d TPS, GET %d TPS\n", 
		nodePool.Size()*PostRateLimitPerNode, nodePool.Size()*GetRateLimitPerNode)
	
	Logln("Starting transaction sending...")

	startTime := time.Now()
	results := SendTransactionsConcurrently(nodePool, rateLimiter, accounts, *toAddress, *amount, *concurrency)
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

		Logln("\n🔍 Verifying transaction receipts...")
		verifyStart := time.Now()
		VerifyTransactionsConcurrently(nodePool, rateLimiter, results, *concurrency)
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
		fmt.Fprintf(file, "%s,%s,%s,%t,%s,%d,%t,%t,%s\n",
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
