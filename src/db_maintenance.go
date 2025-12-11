package main

import (
	"database/sql"
	"log"
	"time"

	_ "modernc.org/sqlite"
)

// CleanupOldData deletes log entries older than the specified retention period
// This helps reduce database size by removing old statistics
func CleanupOldData(dbPath string, retentionDays int) error {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Printf("Error opening database: %v\n", err)
		return err
	}
	defer db.Close()

	cutoffDate := time.Now().AddDate(0, 0, -retentionDays).Format(time.RFC3339)

	result, err := db.Exec("DELETE FROM log_stats WHERE bucket_ts < ?", cutoffDate)
	if err != nil {
		log.Printf("Error cleaning up old data: %v\n", err)
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	log.Printf("Cleanup: deleted %d rows older than %d days\n", rowsAffected, retentionDays)

	return nil
}

// VacuumDatabase reclaims unused space and optimizes the database file
// Should be run periodically (e.g., after cleanup operations)
func VacuumDatabase(dbPath string) error {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Printf("Error opening database: %v\n", err)
		return err
	}
	defer db.Close()

	log.Printf("Running VACUUM to reclaim disk space...\n")
	start := time.Now()

	_, err = db.Exec("VACUUM")
	if err != nil {
		log.Printf("Error running VACUUM: %v\n", err)
		return err
	}

	log.Printf("VACUUM completed in %v\n", time.Since(start))
	return nil
}

// GetDatabaseStats returns statistics about the database
func GetDatabaseStats(dbPath string) (map[string]interface{}, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	stats := make(map[string]interface{})

	// Get total row count
	var rowCount int
	err = db.QueryRow("SELECT COUNT(*) FROM log_stats").Scan(&rowCount)
	if err != nil {
		return nil, err
	}
	stats["total_rows"] = rowCount

	// Get date range
	var oldestBucket, newestBucket string
	err = db.QueryRow("SELECT MIN(bucket_ts), MAX(bucket_ts) FROM log_stats").Scan(&oldestBucket, &newestBucket)
	if err == nil {
		stats["oldest_bucket"] = oldestBucket
		stats["newest_bucket"] = newestBucket
	}

	// Get database file size (page_count * page_size)
	var pageCount, pageSize int
	db.QueryRow("PRAGMA page_count").Scan(&pageCount)
	db.QueryRow("PRAGMA page_size").Scan(&pageSize)
	stats["db_size_mb"] = float64(pageCount*pageSize) / (1024 * 1024)

	// Get number of unique hosts
	var hostCount int
	db.QueryRow("SELECT COUNT(DISTINCT hostname) FROM log_stats").Scan(&hostCount)
	stats["unique_hosts"] = hostCount

	return stats, nil
}
