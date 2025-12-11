package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// LogStatStore manages in-memory storage of LogStat entries organized by time buckets
type LogStatStore struct {
	entries      map[string]*LogStat // key: "host:level:logger:bucketTS"
	nextID       int
	bucketSize   time.Duration
	appStartTime time.Time
	mu           sync.RWMutex
	dbPath       string // path to SQLite database file
	verbose      bool   // enable verbose output
}

// NewLogStatStore creates a new store instance with the specified bucket size
func NewLogStatStore(bucketSize time.Duration, dbPath string, verbose bool) *LogStatStore {
	return &LogStatStore{
		entries:      make(map[string]*LogStat),
		nextID:       1,
		bucketSize:   bucketSize,
		appStartTime: time.Now(),
		dbPath:       dbPath,
		verbose:      verbose,
	}
}

// InitDB ensures the database table exists
func (s *LogStatStore) InitDB() error {
	db, err := sql.Open("sqlite", s.dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	createTableSQL := `
	CREATE TABLE IF NOT EXISTS log_stats (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		hostname TEXT NOT NULL,
		bucket_ts TEXT NOT NULL,
		bucket_duration_s INTEGER NOT NULL,
		level TEXT NOT NULL,
		logger TEXT NOT NULL,
		n INTEGER NOT NULL,
		first_seen_ts TEXT NOT NULL DEFAULT '',
		UNIQUE(hostname, bucket_ts, level, logger)
	);
	`
	_, err = db.Exec(createTableSQL)
	if err != nil {
		return err
	}

	// Create index on bucket_ts for faster queries and cleanup operations
	indexSQL := `CREATE INDEX IF NOT EXISTS idx_bucket_ts ON log_stats(bucket_ts);`
	_, err = db.Exec(indexSQL)
	if err != nil {
		return err
	}

	// Set SQLite performance optimizations
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA cache_size=-64000",
		"PRAGMA temp_store=MEMORY",
	}
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			log.Printf("Warning: failed to set pragma during init: %v\n", err)
		}
	}

	return nil
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

// AddOrUpdate adds a log entry to the appropriate time bucket or updates an existing bucket entry
func (s *LogStatStore) AddOrUpdate(hostName, level string, logger string) *LogStat {
	s.mu.Lock()
	defer s.mu.Unlock()

	currentTime := time.Now()
	bucketStartTime := getBucketTime(currentTime, s.bucketSize)
	bucketTS := bucketStartTime.Format(time.RFC3339)

	// Create key including bucket timestamp
	key := hostName + ":" + logger + ":" + level + ":" + bucketTS

	if stat, exists := s.entries[key]; exists {

		// Update existing entry
		stat.N++
		return stat

	} else {

		// Create new entry
		var duration int
		if s.appStartTime.After(bucketStartTime) {
			// First bucket may be partial (from app start to now)
			duration = int(currentTime.Sub(s.appStartTime).Seconds())
		} else {
			// Other buckets have full size
			duration = int(s.bucketSize.Seconds())
		}

		stat := &LogStat{
			HostName:         hostName,
			BucketTS:         bucketTS,
			BucketDuration_S: duration,
			Level:            level,
			Logger:           logger,
			N:                1,
			FirstSeenTS:      currentTime.Format(time.RFC3339),
		}

		s.entries[key] = stat

		return stat
	}
}

// GetAll returns all log stat entries
func (s *LogStatStore) GetAll() []*LogStat {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := make([]*LogStat, 0, len(s.entries))
	for _, stat := range s.entries {
		// Make a copy to avoid race conditions
		statCopy := *stat
		stats = append(stats, &statCopy)
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
	fmt.Printf("Total unique patterns: %d\n", len(s.entries))
	fmt.Printf("Bucket size: %v\n\n", s.bucketSize)

	for _, stat := range s.entries {
		fmt.Println(stat.String())
	}
	fmt.Println()
}

// FlushToDb writes all LogStat entries to SQLite database and clears the store
func (s *LogStatStore) FlushToDb() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// log time taken for flush
	defer func(start time.Time) {
		log.Printf("    "+"FlushToDb took %v", time.Since(start))
		log.Printf("=== Successfully flushed data to database and cleared store ===\n")
	}(time.Now())

	log.Printf("=== Flushing %d entries to database: %s ===\n", len(s.entries), s.dbPath)
	log.Print("    " + GetMemoryStatsString())

	// Open or create database
	db, err := sql.Open("sqlite", s.dbPath)
	if err != nil {
		log.Printf("Error opening database: %v\n", err)
		s.entries = make(map[string]*LogStat)
		return err
	}
	defer db.Close()

	// Enable performance optimizations for SQLite
	pragmas := []string{
		"PRAGMA journal_mode=WAL",   // Write-Ahead Logging for better concurrency
		"PRAGMA synchronous=NORMAL", // Faster writes with reasonable durability
		"PRAGMA cache_size=-64000",  // 64MB cache
		"PRAGMA temp_store=MEMORY",  // Use memory for temp tables
	}
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			log.Printf("Warning: failed to set pragma: %v\n", err)
		}
	}

	// Begin transaction for batch insert (HUGE performance boost)
	tx, err := db.Begin()
	if err != nil {
		log.Printf("Error beginning transaction: %v\n", err)
		s.entries = make(map[string]*LogStat)
		return err
	}

	// Prepare statement once for reuse (performance optimization)
	upsertSQL := `
	INSERT INTO log_stats (hostname, bucket_ts, bucket_duration_s, level, logger, n, first_seen_ts)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(hostname, bucket_ts, level, logger) 
	DO UPDATE SET 
		n = log_stats.n + excluded.n,
		bucket_duration_s = excluded.bucket_duration_s,
		first_seen_ts = CASE 
			WHEN log_stats.first_seen_ts = '' THEN excluded.first_seen_ts
			WHEN excluded.first_seen_ts = '' THEN log_stats.first_seen_ts
			WHEN log_stats.first_seen_ts < excluded.first_seen_ts THEN log_stats.first_seen_ts
			ELSE excluded.first_seen_ts
		END;
	`
	stmt, err := tx.Prepare(upsertSQL)
	if err != nil {
		tx.Rollback()
		log.Printf("Error preparing statement: %v\n", err)
		s.entries = make(map[string]*LogStat)
		return err
	}
	defer stmt.Close()

	// Execute all inserts within the transaction
	errorCount := 0
	for _, stat := range s.entries {
		if _, err := stmt.Exec(stat.HostName, stat.BucketTS, stat.BucketDuration_S, stat.Level, stat.Logger, stat.N, stat.FirstSeenTS); err != nil {
			log.Printf("Error upserting log stat: %v\n", err)
			errorCount++
		}
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		log.Printf("Error committing transaction: %v\n", err)
		s.entries = make(map[string]*LogStat)
		return err
	}

	if errorCount > 0 {
		log.Printf("Warning: %d errors occurred during flush\n", errorCount)
	}

	// Clear the store
	s.entries = make(map[string]*LogStat)

	log.Print("    " + GetMemoryStatsString())

	return nil
}

// QueryDatabase retrieves all LogStat entries from the SQLite database
func (s *LogStatStore) QueryDatabase() ([]*LogStat, error) {
	db, err := sql.Open("sqlite", s.dbPath)
	if err != nil {
		log.Printf("Error opening database: %v\n", err)
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query("SELECT id, hostname, bucket_ts, bucket_duration_s, level, logger, n, first_seen_ts FROM log_stats ORDER BY bucket_ts DESC")
	if err != nil {
		log.Printf("Error querying database: %v\n", err)
		return nil, err
	}
	defer rows.Close()

	var stats []*LogStat
	for rows.Next() {
		stat := &LogStat{}
		if err := rows.Scan(&stat.ID, &stat.HostName, &stat.BucketTS, &stat.BucketDuration_S, &stat.Level, &stat.Logger, &stat.N, &stat.FirstSeenTS); err != nil {
			log.Printf("Error scanning row: %v\n", err)
			continue
		}
		stats = append(stats, stat)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Error iterating rows: %v\n", err)
		return nil, err
	}

	return stats, nil
}

func (s *LogStatStore) handleJsonLogEntry(line string) {
	// Try to parse as JSON
	var logEntry map[string]interface{}
	err := json.Unmarshal([]byte(line), &logEntry)

	if err == nil {
		// Extract fields
		level := ""
		loggerName := ""
		hostName := ""

		if lvl, ok := logEntry["level"]; ok {
			level = fmt.Sprintf("%v", lvl)
		}
		if log, ok := logEntry["loggerName"]; ok {
			loggerName = fmt.Sprintf("%v", log)
		}
		if h, ok := logEntry["hostName"]; ok {
			hostName = fmt.Sprintf("%v", h)
		}
		// Add or update in store
		stat := s.AddOrUpdate(hostName, level, loggerName)

		// Simple output
		if s.verbose {
			log.Printf("[host: %s,  loggerName: %s, level:%s] = Count: %d\n", hostName, loggerName, level, stat.N)
		}
	} else {
		// Parse error, just print the line
		log.Fatalf("[%s]", line)
	}
}
