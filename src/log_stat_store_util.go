package main

import (
	"fmt"
	"time"
)

// PrintSummary prints all entries to console
func (s *LogStatStore) PrintSummary() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.entries) == 0 {
		fmt.Println("No log statistics yet")
		return
	}

	fmt.Println("\n=== Log Statistics Summary ===")
	fmt.Printf("Total unique patterns: %d\n", len(s.entries))
	fmt.Printf("Bucket size: %v\n\n", s.bucketSize)

	for _, stat := range s.entries {
		fmt.Println(stat.String())
	}
	fmt.Println()
}

// getBucketTime returns the start time of the bucket that contains the given timestamp
// Buckets align to clock boundaries (e.g., for 5m buckets: 00:00, 05:00, 10:00, etc.)
func getBucketTime(ts time.Time, bucketSize time.Duration) time.Time {
	// Get the start of the day
	year, month, day := ts.Date()
	dayStart := time.Date(year, month, day, 0, 0, 0, 0, ts.Location())

	// Calculate bucket index since start of day
	secondsSinceDayStart := ts.Sub(dayStart).Seconds()
	bucketSizeSeconds := bucketSize.Seconds()
	bucketIndex := int(secondsSinceDayStart / bucketSizeSeconds)

	// Return the bucket start time
	bucketStartTime := dayStart.Add(time.Duration(bucketIndex) * bucketSize)
	return bucketStartTime
}
