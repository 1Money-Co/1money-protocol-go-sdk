package main

import (
	"fmt"
	"log"
	"os"
	"time"
)

// main function for standalone execution
func main() {
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

	logToFile("Initializing 1Money Batch Mint Stress Tester...")
	logToFile("Log file created: %s", logFileName)

	// Run the complete stress test using shared function
	if err := runCompleteStressTest(logToFile, fileLogger); err != nil {
		errorMsg := fmt.Sprintf("Batch mint stress test failed: %v", err)
		fileLogger.Println("FATAL: " + errorMsg)
		// Only show critical errors on console, but log details to file
		fmt.Printf("FATAL: Stress test failed. Check log file for details: %s\n", logFileName)
		os.Exit(1)
	}

	// Final log message with file location (only to file)
	logToFile("All logs have been written to: %s", logFileName)
	// Only show completion message on console with log file location
	fmt.Printf("Complete multi-tier stress test completed successfully!\n")
	fmt.Printf("Detailed logs saved to: %s\n", logFileName)
}
