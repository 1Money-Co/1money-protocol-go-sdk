package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"
)

// main function for standalone execution
func main() {
	// Parse command line flags
	flag.Parse()

	// Create log file with timestamp
	timestamp := time.Now().Format("20060102_150405")
	logFileName := fmt.Sprintf("stress_test_%s.log", timestamp)
	logFile, err := os.Create(logFileName)
	if err != nil {
		log.Fatalf("Failed to create log file: %v", err)
	}
	defer logFile.Close()

	// Create custom logger that writes to file
	fileLogger := log.New(logFile, "", log.LstdFlags|log.Lmicroseconds)

	// Redirect default log output to file
	originalLogOutput := log.Writer()
	log.SetOutput(logFile)
	defer log.SetOutput(originalLogOutput)

	// Helper function to log only to file (same behavior as test method)
	logToFile := func(format string, args ...interface{}) {
		message := fmt.Sprintf(format, args...)
		fileLogger.Println(message)
		// No console output - logs only go to file like in test method
	}

	// Check if multi-node mode is requested
	if *nodeList != "" {
		// Multi-node mode
		logToFile("Initializing 1Money Multi-Node Batch Mint Stress Tester...")
		logToFile("Log file created: %s", logFileName)

		// Parse node URLs
		nodeURLs, err := ParseNodeURLs(*nodeList)
		if err != nil {
			errorMsg := fmt.Sprintf("Failed to parse node list: %v", err)
			fileLogger.Println("FATAL: " + errorMsg)
			fmt.Printf("FATAL: %s\n", errorMsg)
			os.Exit(1)
		}

		// Run the multi-node stress test
		if err := runCompleteStressTest(logToFile, fileLogger, nodeURLs, *postRate, *getRate); err != nil {
			errorMsg := fmt.Sprintf("Multi-node batch mint stress test failed: %v", err)
			fileLogger.Println("FATAL: " + errorMsg)
			fmt.Printf("FATAL: Multi-node stress test failed. Check log file for details: %s\n", logFileName)
			os.Exit(1)
		}

		fmt.Printf("Complete multi-node multi-tier stress test completed successfully!\n")
	} else {
		// Single node mode (default)
		logToFile("Initializing 1Money Batch Mint Stress Tester...")
		logToFile("Log file created: %s", logFileName)

		// Run the complete stress test using shared function
		if err := runCompleteStressTestLegacy(logToFile, fileLogger); err != nil {
			errorMsg := fmt.Sprintf("Batch mint stress test failed: %v", err)
			fileLogger.Println("FATAL: " + errorMsg)
			fmt.Printf("FATAL: Stress test failed. Check log file for details: %s\n", logFileName)
			os.Exit(1)
		}

		fmt.Printf("Complete multi-tier stress test completed successfully!\n")
	}

	// Final log message with file location (only to file)
	logToFile("All logs have been written to: %s", logFileName)
	// Show completion message on console with log file location
	fmt.Printf("Detailed logs saved to: %s\n", logFileName)
}
