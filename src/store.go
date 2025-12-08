package main

import (
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// LogStatStore manages in-memory storage of LogStat entries
type LogStatStore struct {
	entries map[string]*LogStat // key: "host:level:logger"
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

// FlushToDb writes all LogStat entries to SQLite database and clears the store
func (s *LogStatStore) FlushToDb(dbPath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	log.Printf("Flushing %d entries to database: %s\n", len(s.entries), dbPath)

	// Calculate final intervals for all entries
	for _, stat := range s.entries {
		stat.TS_Interval_S = s.calculateInterval(stat.TS_Start)
	}

	// Open or create database
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Printf("Error opening database: %v\n", err)
		s.entries = make(map[string]*LogStat)
		return err
	}
	defer db.Close()

	// Create table if it doesn't exist
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS log_stats (
		id INTEGER PRIMARY KEY,
		hostname TEXT NOT NULL,
		ts_start TEXT NOT NULL,
		ts_interval_s INTEGER NOT NULL,
		level TEXT NOT NULL,
		logger TEXT NOT NULL,
		n INTEGER NOT NULL
	);
	`

	if _, err := db.Exec(createTableSQL); err != nil {
		log.Printf("Error creating table: %v\n", err)
		s.entries = make(map[string]*LogStat)
		return err
	}

	// Insert all entries into database
	insertSQL := `
	INSERT INTO log_stats (hostname, ts_start, ts_interval_s, level, logger, n)
	VALUES (?, ?, ?, ?, ?, ?);
	`

	for _, stat := range s.entries {
		if _, err := db.Exec(insertSQL, stat.HostName, stat.TS_Start, stat.TS_Interval_S, stat.Level, stat.Logger, stat.N); err != nil {
			log.Printf("Error inserting log stat: %v\n", err)
		}
	}

	// Clear the store
	s.entries = make(map[string]*LogStat)
	s.nextID = 1

	log.Printf("Successfully flushed data to database and cleared store\n")
	return nil
}
