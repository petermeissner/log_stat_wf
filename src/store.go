package main

import (
	"database/sql"
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
}

// NewLogStatStore creates a new store instance with the specified bucket size
func NewLogStatStore(bucketSize time.Duration, dbPath string) *LogStatStore {
	return &LogStatStore{
		entries:      make(map[string]*LogStat),
		nextID:       1,
		bucketSize:   bucketSize,
		appStartTime: time.Now(),
		dbPath:       dbPath,
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
	return err
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

	log.Printf("Flushing %d entries to database: %s\n", len(s.entries), s.dbPath)

	// Open or create database
	db, err := sql.Open("sqlite", s.dbPath)
	if err != nil {
		log.Printf("Error opening database: %v\n", err)
		s.entries = make(map[string]*LogStat)
		return err
	}
	defer db.Close()

	// Insert or update entries in database (UPSERT)
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

	for _, stat := range s.entries {
		if _, err := db.Exec(upsertSQL, stat.HostName, stat.BucketTS, stat.BucketDuration_S, stat.Level, stat.Logger, stat.N, stat.FirstSeenTS); err != nil {
			log.Printf("Error upserting log stat: %v\n", err)
		}
	}

	// Clear the store
	s.entries = make(map[string]*LogStat)

	log.Printf("Successfully flushed data to database and cleared store\n")
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
