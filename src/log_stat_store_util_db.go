package main

import (
	"database/sql"
	"log"
	"time"

	_ "modernc.org/sqlite"
)

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
