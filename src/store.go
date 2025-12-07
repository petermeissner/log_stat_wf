package main

import (
	"fmt"
	"sync"
	"time"
)

// LogStatStore manages in-memory storage of LogStat entries
type LogStatStore struct {
	entries map[string]*LogStat // key: "level:logger"
	nextID  int
	mu      sync.RWMutex
}

// NewLogStatStore creates a new store instance
func NewLogStatStore() *LogStatStore {
	return &LogStatStore{
		entries: make(map[string]*LogStat),
		nextID:  1,
	}
}

// AddOrUpdate adds a new log stat or updates an existing one
func (s *LogStatStore) AddOrUpdate(hostName, level string, logger string) *LogStat {

	// concurrent access protection
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create key
	key := hostName + ":" + level + ":" + logger

	// create current timestamp
	timestamp := time.Now().Format(time.RFC3339)

	// Check if entry exists
	if stat, exists := s.entries[key]; exists {
		// Update existing entry
		stat.N++
		stat.TS_Interval_S = s.calculateInterval(stat.TS_Start)
		return stat
	}

	// Create new entry
	stat := &LogStat{
		ID:            s.nextID,
		HostName:      hostName,
		TS_Start:      timestamp,
		TS_Interval_S: 0,
		Level:         level,
		Logger:        logger,
		N:             1,
	}

	s.entries[key] = stat
	s.nextID++

	return stat
}

// GetAll returns all log stat entries
func (s *LogStatStore) GetAll() []*LogStat {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := make([]*LogStat, 0, len(s.entries))
	for _, stat := range s.entries {
		stats = append(stats, stat)
	}

	return stats
}

// GetCount returns the total number of entries
func (s *LogStatStore) GetCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.entries)
}

// PrintSummary prints all entries to console
func (s *LogStatStore) PrintSummary() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.entries) == 0 {
		fmt.Println("No log statistics yet")
		return
	}

	fmt.Println("\n=== Log Statistics Summary ===")
	fmt.Printf("Total unique patterns: %d\n\n", len(s.entries))

	for _, stat := range s.entries {
		fmt.Println(stat.String())
	}
	fmt.Println()
}

// calculateInterval calculates seconds between start timestamp and now
func (s *LogStatStore) calculateInterval(startTS string) int {
	// Parse timestamp format: "2024-12-07T14:33:34Z" (RFC3339)
	startTime, err := time.Parse(time.RFC3339, startTS)
	if err != nil {
		return 0
	}

	elapsed := time.Since(startTime)
	return int(elapsed.Seconds())
}
