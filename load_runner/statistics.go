package main

import (
	"fmt"
	"sort"
	"time"
)

type Statistics struct {
	TotalAccounts      int
	SuccessfulSends    int
	FailedSends        int
	TotalSendDuration  time.Duration
	SuccessfulVerified int
	FailedVerified     int
	NotVerified        int
	TotalVerifyDuration time.Duration
	
	// Detailed timings
	MinSendTime    time.Duration
	MaxSendTime    time.Duration
	AvgSendTime    time.Duration
	
	// TPS calculations
	ActualSendTPS   float64
	ActualVerifyTPS float64
	
	// Per-second breakdown
	SendTPSBySecond    map[int]int
	VerifyTPSBySecond  map[int]int
}

func CalculateStatistics(results []TransactionResult, sendDuration, verifyDuration time.Duration) *Statistics {
	stats := &Statistics{
		TotalAccounts:     len(results),
		TotalSendDuration: sendDuration,
		TotalVerifyDuration: verifyDuration,
		MinSendTime:      time.Hour, // Initialize with large value
		SendTPSBySecond:  make(map[int]int),
		VerifyTPSBySecond: make(map[int]int),
	}
	
	var totalSendTime time.Duration
	
	for _, result := range results {
		if result.Success {
			stats.SuccessfulSends++
			totalSendTime += result.Duration
			
			// Track min/max send times
			if result.Duration < stats.MinSendTime {
				stats.MinSendTime = result.Duration
			}
			if result.Duration > stats.MaxSendTime {
				stats.MaxSendTime = result.Duration
			}
			
			// Calculate which second this transaction was sent
			secondOffset := int(result.Duration.Seconds())
			stats.SendTPSBySecond[secondOffset]++
		} else {
			stats.FailedSends++
		}
		
		// Verification stats
		if result.Verified {
			if result.TxSuccess {
				stats.SuccessfulVerified++
			} else {
				stats.FailedVerified++
			}
		} else if result.Success {
			stats.NotVerified++
		}
	}
	
	// Calculate averages
	if stats.SuccessfulSends > 0 {
		stats.AvgSendTime = totalSendTime / time.Duration(stats.SuccessfulSends)
		stats.ActualSendTPS = float64(stats.SuccessfulSends) / sendDuration.Seconds()
	}
	
	if stats.SuccessfulVerified+stats.FailedVerified > 0 {
		totalVerified := stats.SuccessfulVerified + stats.FailedVerified
		stats.ActualVerifyTPS = float64(totalVerified) / verifyDuration.Seconds()
	}
	
	return stats
}

func (s *Statistics) PrintDetailedReport() {
	Logln("\n╔══════════════════════════════════════════════════════════════════╗")
	Logln("║                    TRANSACTION STATISTICS REPORT                  ║")
	Logln("╚══════════════════════════════════════════════════════════════════╝")
	
	// Send Statistics
	Logln("\n┌─────────────────── Send Statistics ───────────────────┐")
	Logf("│ Total Accounts:        %-30d │\n", s.TotalAccounts)
	Logf("│ Successful Sends:      %-30d │\n", s.SuccessfulSends)
	Logf("│ Failed Sends:          %-30d │\n", s.FailedSends)
	Logf("│ Success Rate:          %-29.2f%% │\n", float64(s.SuccessfulSends)/float64(s.TotalAccounts)*100)
	Logln("├───────────────────────────────────────────────────────┤")
	Logf("│ Total Duration:        %-30s │\n", s.TotalSendDuration.Round(time.Millisecond))
	Logf("│ Min Send Time:         %-30s │\n", s.MinSendTime.Round(time.Millisecond))
	Logf("│ Max Send Time:         %-30s │\n", s.MaxSendTime.Round(time.Millisecond))
	Logf("│ Avg Send Time:         %-30s │\n", s.AvgSendTime.Round(time.Millisecond))
	Logln("├───────────────────────────────────────────────────────┤")
	Logf("│ Actual Send TPS:       %-29.2f │\n", s.ActualSendTPS)
	Logln("└───────────────────────────────────────────────────────┘")
	
	// Verification Statistics
	if s.SuccessfulVerified+s.FailedVerified > 0 {
		Logln("\n┌─────────────── Verification Statistics ────────────────┐")
		Logf("│ Total Verified:        %-30d │\n", s.SuccessfulVerified+s.FailedVerified)
		Logf("│ On-chain Success:      %-30d │\n", s.SuccessfulVerified)
		Logf("│ On-chain Failed:       %-30d │\n", s.FailedVerified)
		Logf("│ Not Verified:          %-30d │\n", s.NotVerified)
		Logln("├───────────────────────────────────────────────────────┤")
		Logf("│ Verification Duration: %-30s │\n", s.TotalVerifyDuration.Round(time.Millisecond))
		Logf("│ Actual Verify TPS:     %-29.2f │\n", s.ActualVerifyTPS)
		Logln("└───────────────────────────────────────────────────────┘")
	}
	
	// TPS Distribution (if we have enough data)
	if len(s.SendTPSBySecond) > 1 {
		s.printTPSDistribution()
	}
}

func (s *Statistics) printTPSDistribution() {
	Logln("\n┌──────────────── TPS Distribution ─────────────────┐")
	Logln("│ Second │ Transactions │ TPS                       │")
	Logln("├────────┼──────────────┼───────────────────────────┤")
	
	// Sort seconds
	seconds := make([]int, 0, len(s.SendTPSBySecond))
	for sec := range s.SendTPSBySecond {
		seconds = append(seconds, sec)
	}
	sort.Ints(seconds)
	
	// Show first 10 seconds
	maxRows := 10
	if len(seconds) < maxRows {
		maxRows = len(seconds)
	}
	
	for i := 0; i < maxRows; i++ {
		sec := seconds[i]
		count := s.SendTPSBySecond[sec]
		bar := generateBar(count, 20)
		Logf("│ %6d │ %12d │ %-25s │\n", sec, count, bar)
	}
	
	if len(seconds) > maxRows {
		Logf("│  ...   │     ...      │ (showing first %d seconds) │\n", maxRows)
	}
	
	Logln("└────────┴──────────────┴───────────────────────────┘")
}

func generateBar(value, maxWidth int) string {
	if value == 0 {
		return ""
	}
	
	// Scale to maxWidth
	barLength := value * maxWidth / 100
	if barLength < 1 && value > 0 {
		barLength = 1
	}
	if barLength > maxWidth {
		barLength = maxWidth
	}
	
	bar := ""
	for i := 0; i < barLength; i++ {
		bar += "█"
	}
	
	return fmt.Sprintf("%s %d", bar, value)
}

// CalculatePercentile calculates the nth percentile of durations
func CalculatePercentile(durations []time.Duration, percentile float64) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	
	sort.Slice(durations, func(i, j int) bool {
		return durations[i] < durations[j]
	})
	
	index := int(float64(len(durations)-1) * percentile / 100.0)
	return durations[index]
}