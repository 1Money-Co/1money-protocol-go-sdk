package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

	// Initialize balanced node pool
	nodePool := NewBalancedNodePool()

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
	
	Logf("Chain ID: %d (hardcoded)\n", HardcodedChainID)
	Logf("Rate limits: POST %d TPS/node, GET %d TPS/node\n", PostRateLimitPerNode, GetRateLimitPerNode)
	Logf("Total rate limits: POST %d TPS, GET %d TPS\n", 
		nodePool.Size()*PostRateLimitPerNode, nodePool.Size()*GetRateLimitPerNode)
	Logf("Rate limiter: Strict sequential rate limiting\n")
	
	// Print node configuration stats
	printNodeUsageStats(nodePool)
	
	Logln("\nStarting transaction sending...")
	Logln(strings.Repeat("═", 60))

	// Calculate expected transactions per node
	expectedPerNode := nodePool.CalculateExpectedTransactionsPerNode(len(accounts))
	Logf("Expected transactions per node: %d\n", expectedPerNode)

	startTime := time.Now()
	results := SendTransactionsWithStrictRateLimit(nodePool, accounts, *toAddress, *amount, *concurrency)
	sendDuration := time.Since(startTime)

	// Log individual results with progress
	for _, result := range results {
		sendTime := ""
		responseTime := ""
		if !result.SendTime.IsZero() {
			sendTime = result.SendTime.Format("15:04:05.000")
		}
		if !result.ResponseTime.IsZero() {
			responseTime = result.ResponseTime.Format("15:04:05.000")
		}
		
		if result.Success {
			Logf("[Sent: %s, Response: %s] [%d/%d-%s](%dms) ✅ Wallet #%s: TX %s\n",
				sendTime, responseTime, result.NodeCount, expectedPerNode, result.NodeURL, result.Duration.Milliseconds(), result.WalletIndex, result.TxHash)
		} else {
			Logf("[Sent: %s, Response: %s] [%d/%d-%s](%dms) ❌ Wallet #%s: %v\n",
				sendTime, responseTime, result.NodeCount, expectedPerNode, result.NodeURL, result.Duration.Milliseconds(), result.WalletIndex, result.Error)
		}
	}

	// Calculate initial statistics
	stats := CalculateStatistics(results, sendDuration, 0)
	
	if stats.SuccessfulSends > 0 {
		Logln("\n⏳ Waiting 10 seconds before verifying transactions...")
		time.Sleep(10 * time.Second)

		Logln("\n🔍 Verifying transaction receipts...")
		Logln("Note: Using same nodes as configured, respecting GET rate limit (500 TPS/node)")
		Logln(strings.Repeat("─", 60))
		verifyStart := time.Now()
		VerifyTransactionsWithStrictRateLimit(nodePool, results, *concurrency)
		verifyDuration := time.Since(verifyStart)

		// Log verification results
		verifiedCount := 0
		for i, result := range results {
			if result.Verified {
				verifiedCount++
				if result.TxSuccess {
					Logf("[%d/%d] ✅ TX %s: Confirmed successful\n", verifiedCount, stats.SuccessfulSends, result.TxHash)
				} else {
					Logf("[%d/%d] ❌ TX %s: Failed on chain\n", verifiedCount, stats.SuccessfulSends, result.TxHash)
				}
			} else if result.Success && result.VerificationError != nil {
				Logf("[%d/%d] ⚠️  TX %s: Verification error: %v\n", i+1, stats.SuccessfulSends, result.TxHash, result.VerificationError)
			}
		}
		
		// Recalculate statistics with verification data
		stats = CalculateStatistics(results, sendDuration, verifyDuration)
	}
	
	// Print detailed statistics report
	stats.PrintDetailedReport()
	
	// Print node distribution statistics
	nodePool.PrintNodeDistribution()

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

	fmt.Fprintf(file, "wallet_index,from_address,tx_hash,success,error,duration_ms,send_time,response_time,node_url,node_count,verified,tx_success,verification_error\n")
	for _, result := range results {
		errorStr := ""
		if result.Error != nil {
			errorStr = result.Error.Error()
		}
		verifyErrorStr := ""
		if result.VerificationError != nil {
			verifyErrorStr = result.VerificationError.Error()
		}
		sendTimeStr := ""
		responseTimeStr := ""
		if !result.SendTime.IsZero() {
			sendTimeStr = result.SendTime.Format("2006-01-02 15:04:05.000")
		}
		if !result.ResponseTime.IsZero() {
			responseTimeStr = result.ResponseTime.Format("2006-01-02 15:04:05.000")
		}
		fmt.Fprintf(file, "%s,%s,%s,%t,%s,%d,%s,%s,%s,%d,%t,%t,%s\n",
			result.WalletIndex,
			result.FromAddress,
			result.TxHash,
			result.Success,
			errorStr,
			result.Duration.Milliseconds(),
			sendTimeStr,
			responseTimeStr,
			result.NodeURL,
			result.NodeCount,
			result.Verified,
			result.TxSuccess,
			verifyErrorStr,
		)
	}

	absPath, _ := filepath.Abs(filename)
	Logf("Results written to: %s\n", absPath)
	return nil
}

// Additional statistics helpers
func printNodeUsageStats(nodePool *BalancedNodePool) {
	Logln("\n┌────────────────── Node Configuration ──────────────────┐")
	Logf("│ Total Nodes: %-40d │\n", nodePool.Size())
	Logf("│ POST Rate Limit: %-33d TPS/node │\n", PostRateLimitPerNode)
	Logf("│ GET Rate Limit: %-34d TPS/node │\n", GetRateLimitPerNode)
	Logf("│ Max POST TPS: %-36d TPS │\n", nodePool.Size()*PostRateLimitPerNode)
	Logf("│ Max GET TPS: %-37d TPS │\n", nodePool.Size()*GetRateLimitPerNode)
	Logln("└────────────────────────────────────────────────────────┘")
}
