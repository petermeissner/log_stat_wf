package main

import (
	"database/sql"
	"log"
	"regexp"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// regexToLike converts a simple regex pattern to SQL LIKE pattern
func regexToLike(pattern string) string {
	if pattern == "" {
		return "%"
	}

	// Check for anchored patterns
	hasStart := strings.HasPrefix(pattern, "^")
	hasEnd := strings.HasSuffix(pattern, "$")

	// Remove anchors
	if hasStart {
		pattern = strings.TrimPrefix(pattern, "^")
	}
	if hasEnd {
		pattern = strings.TrimSuffix(pattern, "$")
	}

	// Replace regex patterns with LIKE wildcards
	pattern = strings.ReplaceAll(pattern, ".*", "%")
	pattern = strings.ReplaceAll(pattern, "\\.", ".") // Unescape dots

	// Add wildcards based on anchors
	if !hasStart && !hasEnd {
		// No anchors - match anywhere
		pattern = "%" + pattern + "%"
	} else if hasStart && !hasEnd {
		// Start anchor only - starts with
		if !strings.HasSuffix(pattern, "%") {
			pattern = pattern + "%"
		}
	} else if !hasStart && hasEnd {
		// End anchor only - ends with
		if !strings.HasPrefix(pattern, "%") {
			pattern = "%" + pattern
		}
	}
	// else both anchors - exact match, use pattern as-is

	return pattern
}

// QueryFilter holds filter criteria for querying log statistics
type QueryFilter struct {
	Level         string    // Filter by log level (empty = all levels)
	LoggerRegex   string    // Regex pattern to match logger names (empty = all loggers)
	StartTime     time.Time // Filter entries >= this time (zero = no start limit)
	EndTime       time.Time // Filter entries <= this time (zero = no end limit)
	MaxResults    int       // Maximum number of results to return (0 = unlimited)
	IncludeMemory bool      // Include in-memory entries
	IncludeDB     bool      // Include database entries
}

// AggregatedStat represents aggregated statistics across multiple loggers
type AggregatedStat struct {
	HostName    string
	BucketTS    string
	Level       string
	TotalCount  int
	LoggerCount int    // Number of unique loggers
	FirstSeenTS string // Earliest FirstSeenTS across aggregated entries
}

// QueryLogStats queries log statistics from both memory and database with filters
func (s *LogStatStore) QueryLogStats(filter QueryFilter) ([]*LogStat, error) {
	var allStats []*LogStat
	var loggerRegex *regexp.Regexp
	var err error

	// Compile regex if provided
	if filter.LoggerRegex != "" {
		loggerRegex, err = regexp.Compile(filter.LoggerRegex)
		if err != nil {
			return nil, err
		}
	}

	// Get in-memory entries
	if filter.IncludeMemory {
		s.mu.RLock()
		for _, stat := range s.entries {
			statCopy := *stat
			allStats = append(allStats, &statCopy)
		}
		s.mu.RUnlock()
	}

	// Get database entries
	if filter.IncludeDB {
		dbStats, err := s.queryDatabaseWithFilter(filter)
		if err != nil {
			log.Printf("Error querying database: %v\n", err)
		} else {
			allStats = append(allStats, dbStats...)
		}
	}

	// Apply filters
	var filtered []*LogStat
	for _, stat := range allStats {
		// Filter by level
		if filter.Level != "" && stat.Level != filter.Level {
			continue
		}

		// Filter by logger regex
		if loggerRegex != nil && !loggerRegex.MatchString(stat.Logger) {
			continue
		}

		// Filter by time range
		if !filter.StartTime.IsZero() || !filter.EndTime.IsZero() {
			bucketTime, err := time.Parse(time.RFC3339, stat.BucketTS)
			if err != nil {
				continue
			}

			if !filter.StartTime.IsZero() && bucketTime.Before(filter.StartTime) {
				continue
			}

			if !filter.EndTime.IsZero() && bucketTime.After(filter.EndTime) {
				continue
			}
		}

		filtered = append(filtered, stat)
	}

	// Apply max results limit
	if filter.MaxResults > 0 && len(filtered) > filter.MaxResults {
		filtered = filtered[:filter.MaxResults]
	}

	return filtered, nil
}

// queryDatabaseWithFilter queries the database with SQL-level filtering for efficiency
func (s *LogStatStore) queryDatabaseWithFilter(filter QueryFilter) ([]*LogStat, error) {
	db, err := sql.Open("sqlite", s.dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	// Build query with filters
	query := "SELECT id, hostname, bucket_ts, bucket_duration_s, level, logger, n, first_seen_ts FROM log_stats WHERE 1=1"
	var args []interface{}

	if filter.Level != "" {
		query += " AND level = ?"
		args = append(args, filter.Level)
	}

	// Convert pattern to SQL LIKE
	if filter.LoggerRegex != "" {
		likePattern := regexToLike(filter.LoggerRegex)
		query += " AND logger LIKE ?"
		args = append(args, likePattern)
	}

	if !filter.StartTime.IsZero() {
		query += " AND bucket_ts >= ?"
		args = append(args, filter.StartTime.Format(time.RFC3339))
	}

	if !filter.EndTime.IsZero() {
		query += " AND bucket_ts <= ?"
		args = append(args, filter.EndTime.Format(time.RFC3339))
	}

	query += " ORDER BY bucket_ts DESC"

	// Apply LIMIT
	if filter.MaxResults > 0 {
		query += " LIMIT ?"
		args = append(args, filter.MaxResults)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
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

	return stats, rows.Err()
}

// QueryAggregatedStats queries and aggregates statistics across loggers
func (s *LogStatStore) QueryAggregatedStats(filter QueryFilter) ([]*AggregatedStat, error) {
	// First get all matching log stats
	stats, err := s.QueryLogStats(filter)
	if err != nil {
		return nil, err
	}

	// Aggregate by hostname, bucket_ts, and level
	aggregateMap := make(map[string]*AggregatedStat)

	for _, stat := range stats {
		// Create key: hostname:bucket_ts:level
		key := stat.HostName + ":" + stat.BucketTS + ":" + stat.Level

		if agg, exists := aggregateMap[key]; exists {
			// Update existing aggregation
			agg.TotalCount += stat.N
			agg.LoggerCount++

			// Keep earliest FirstSeenTS
			if stat.FirstSeenTS != "" && (agg.FirstSeenTS == "" || stat.FirstSeenTS < agg.FirstSeenTS) {
				agg.FirstSeenTS = stat.FirstSeenTS
			}
		} else {
			// Create new aggregation
			aggregateMap[key] = &AggregatedStat{
				HostName:    stat.HostName,
				BucketTS:    stat.BucketTS,
				Level:       stat.Level,
				TotalCount:  stat.N,
				LoggerCount: 1,
				FirstSeenTS: stat.FirstSeenTS,
			}
		}
	}

	// Convert map to slice
	var results []*AggregatedStat
	for _, agg := range aggregateMap {
		results = append(results, agg)
	}

	return results, nil
}

// QueryAggregatedStatsOptimized queries and aggregates using SQL GROUP BY for better performance
func (s *LogStatStore) QueryAggregatedStatsOptimized(filter QueryFilter) ([]*AggregatedStat, error) {
	var allAggregates []*AggregatedStat

	// Aggregate in-memory data
	if filter.IncludeMemory {
		memoryStats, err := s.QueryLogStats(QueryFilter{
			Level:         filter.Level,
			LoggerRegex:   filter.LoggerRegex,
			StartTime:     filter.StartTime,
			EndTime:       filter.EndTime,
			IncludeMemory: true,
			IncludeDB:     false,
		})
		if err != nil {
			return nil, err
		}

		// Aggregate memory stats
		memoryAgg := aggregateStats(memoryStats)
		allAggregates = append(allAggregates, memoryAgg...)
	}

	// Aggregate database data using SQL
	if filter.IncludeDB {
		dbAgg, err := s.queryAggregatedFromDB(filter)
		if err != nil {
			log.Printf("Error querying aggregated database: %v\n", err)
		} else {
			allAggregates = append(allAggregates, dbAgg...)
		}
	}

	// Merge aggregates with same key
	return mergeAggregates(allAggregates), nil
}

func dbQueryInt(db *sql.DB, query string, args ...interface{}) int {
	var result int
	err := db.QueryRow(query, args...).Scan(&result)
	if err != nil {
		log.Printf("Error executing query '%s': %v\n", query, err)
		return 0
	}
	return result
}

func dbQueryInt64(db *sql.DB, query string, args ...interface{}) int64 {
	var result int64
	err := db.QueryRow(query, args...).Scan(&result)
	if err != nil {
		log.Printf("Error executing query '%s': %v\n", query, err)
		return 0
	}
	return result
}

// dbStats, returns map[string]interface{} with comprehensive database statistics
func (s LogStatStore) dbStats(retentionDays int) (map[string]interface{}, error) {

	// db connection
	db, err := sql.Open("sqlite", s.dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	// Basic counts using helper functions
	var oldestBucket, newestBucket string

	uniqueBuckets := dbQueryInt(db, "SELECT count(distinct bucket_ts) FROM log_stats")
	totalEntries := dbQueryInt(db, "SELECT count(*) FROM log_stats")
	uniqueLevels := dbQueryInt(db, "SELECT count(distinct level) FROM log_stats")
	uniqueLoggers := dbQueryInt(db, "SELECT count(distinct logger) FROM log_stats")
	uniqueHosts := dbQueryInt(db, "SELECT count(distinct hostname) FROM log_stats")
	totalMessages := dbQueryInt64(db, "SELECT COALESCE(SUM(n), 0) FROM log_stats")

	// Get date range
	query_date_range := "SELECT MIN(bucket_ts), MAX(bucket_ts) FROM log_stats"
	if err := db.QueryRow(query_date_range).Scan(&oldestBucket, &newestBucket); err != nil {
		// If no data, set to empty strings
		oldestBucket = ""
		newestBucket = ""
	}

	// Get database file size
	pageCount := dbQueryInt(db, "PRAGMA page_count")
	pageSize := dbQueryInt(db, "PRAGMA page_size")
	dbSizeMB := float64(pageCount*pageSize) / (1024 * 1024)

	// Collect results
	res := map[string]interface{}{}
	res["unique_loggers"] = uniqueLoggers
	res["unique_levels"] = uniqueLevels
	res["unique_hosts"] = uniqueHosts
	res["total_entries"] = totalEntries
	res["unique_buckets"] = uniqueBuckets
	res["total_messages"] = totalMessages
	res["oldest_bucket"] = oldestBucket
	res["newest_bucket"] = newestBucket
	res["db_size_mb"] = dbSizeMB
	res["retention_days"] = retentionDays

	// Message counts by level (sum of n, not count of rows)
	res["n_debug"] = dbQueryInt(db, "SELECT COALESCE(SUM(n), 0) FROM log_stats WHERE level='DEBUG'")
	res["n_trace"] = dbQueryInt(db, "SELECT COALESCE(SUM(n), 0) FROM log_stats WHERE level='TRACE'")
	res["n_info"] = dbQueryInt(db, "SELECT COALESCE(SUM(n), 0) FROM log_stats WHERE level='INFO'")
	res["n_warn"] = dbQueryInt(db, "SELECT COALESCE(SUM(n), 0) FROM log_stats WHERE level='WARN' OR level='WARNING'")
	res["n_error"] = dbQueryInt(db, "SELECT COALESCE(SUM(n), 0) FROM log_stats WHERE level='ERROR'")
	res["n_fatal"] = dbQueryInt(db, "SELECT COALESCE(SUM(n), 0) FROM log_stats WHERE level='FATAL'")

	// Recent activity by level for multiple time windows (24h, 8h, 1h)
	recentActivityQuery := `
		SELECT level, COALESCE(SUM(n), 0) as message_count 
		FROM log_stats 
		WHERE bucket_ts >= ? 
		GROUP BY level 
		ORDER BY message_count DESC
	`

	// 24-hour window
	cutoffTime24h := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
	rows24h, err := db.Query(recentActivityQuery, cutoffTime24h)
	if err == nil {
		defer rows24h.Close()
		recentActivity24h := make(map[string]int64)
		for rows24h.Next() {
			var level string
			var count int64
			if err := rows24h.Scan(&level, &count); err == nil {
				recentActivity24h[level] = count
			}
		}
		res["recent_activity_24h"] = recentActivity24h
	}

	// 8-hour window
	cutoffTime8h := time.Now().Add(-8 * time.Hour).Format(time.RFC3339)
	rows8h, err := db.Query(recentActivityQuery, cutoffTime8h)
	if err == nil {
		defer rows8h.Close()
		recentActivity8h := make(map[string]int64)
		for rows8h.Next() {
			var level string
			var count int64
			if err := rows8h.Scan(&level, &count); err == nil {
				recentActivity8h[level] = count
			}
		}
		res["recent_activity_8h"] = recentActivity8h
	}

	// 1-hour window
	cutoffTime1h := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	rows1h, err := db.Query(recentActivityQuery, cutoffTime1h)
	if err == nil {
		defer rows1h.Close()
		recentActivity1h := make(map[string]int64)
		for rows1h.Next() {
			var level string
			var count int64
			if err := rows1h.Scan(&level, &count); err == nil {
				recentActivity1h[level] = count
			}
		}
		res["recent_activity_1h"] = recentActivity1h
	}

	return res, nil
}

// queryAggregatedFromDB performs aggregation using SQL GROUP BY
func (s *LogStatStore) queryAggregatedFromDB(filter QueryFilter) ([]*AggregatedStat, error) {
	db, err := sql.Open("sqlite", s.dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	query := `
		SELECT 
			hostname,
			bucket_ts,
			level,
			SUM(n) as total_count,
			COUNT(DISTINCT logger) as logger_count,
			MIN(first_seen_ts) as first_seen_ts
		FROM log_stats
		WHERE 1=1
	`
	var args []interface{}

	if filter.Level != "" {
		query += " AND level = ?"
		args = append(args, filter.Level)
	}

	// Convert pattern to SQL LIKE
	if filter.LoggerRegex != "" {
		likePattern := regexToLike(filter.LoggerRegex)
		query += " AND logger LIKE ?"
		args = append(args, likePattern)
	}

	if !filter.StartTime.IsZero() {
		query += " AND bucket_ts >= ?"
		args = append(args, filter.StartTime.Format(time.RFC3339))
	}

	if !filter.EndTime.IsZero() {
		query += " AND bucket_ts <= ?"
		args = append(args, filter.EndTime.Format(time.RFC3339))
	}

	query += " GROUP BY hostname, bucket_ts, level"
	query += " ORDER BY bucket_ts DESC"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Read aggregated rows directly
	var aggregated []*AggregatedStat
	for rows.Next() {
		agg := &AggregatedStat{}
		if err := rows.Scan(&agg.HostName, &agg.BucketTS, &agg.Level, &agg.TotalCount, &agg.LoggerCount, &agg.FirstSeenTS); err != nil {
			log.Printf("Error scanning aggregated row: %v\n", err)
			continue
		}
		aggregated = append(aggregated, agg)
	}

	// Apply max results filter if specified
	if filter.MaxResults > 0 && len(aggregated) > filter.MaxResults {
		aggregated = aggregated[:filter.MaxResults]
	}

	return aggregated, rows.Err()
}

// aggregateStats aggregates a slice of LogStats
func aggregateStats(stats []*LogStat) []*AggregatedStat {
	aggregateMap := make(map[string]*AggregatedStat)

	for _, stat := range stats {
		key := stat.HostName + ":" + stat.BucketTS + ":" + stat.Level

		if agg, exists := aggregateMap[key]; exists {
			agg.TotalCount += stat.N
			agg.LoggerCount++
			if stat.FirstSeenTS != "" && (agg.FirstSeenTS == "" || stat.FirstSeenTS < agg.FirstSeenTS) {
				agg.FirstSeenTS = stat.FirstSeenTS
			}
		} else {
			aggregateMap[key] = &AggregatedStat{
				HostName:    stat.HostName,
				BucketTS:    stat.BucketTS,
				Level:       stat.Level,
				TotalCount:  stat.N,
				LoggerCount: 1,
				FirstSeenTS: stat.FirstSeenTS,
			}
		}
	}

	var results []*AggregatedStat
	for _, agg := range aggregateMap {
		results = append(results, agg)
	}
	return results
}

// mergeAggregates merges duplicate aggregates
func mergeAggregates(aggregates []*AggregatedStat) []*AggregatedStat {
	aggregateMap := make(map[string]*AggregatedStat)

	for _, agg := range aggregates {
		key := agg.HostName + ":" + agg.BucketTS + ":" + agg.Level

		if existing, exists := aggregateMap[key]; exists {
			existing.TotalCount += agg.TotalCount
			existing.LoggerCount += agg.LoggerCount
			if agg.FirstSeenTS != "" && (existing.FirstSeenTS == "" || agg.FirstSeenTS < existing.FirstSeenTS) {
				existing.FirstSeenTS = agg.FirstSeenTS
			}
		} else {
			aggregateMap[key] = agg
		}
	}

	var results []*AggregatedStat
	for _, agg := range aggregateMap {
		results = append(results, agg)
	}
	return results
}

// Helper functions for common query patterns

// QueryRecentStats returns recent log statistics from both memory and database
func (s *LogStatStore) QueryRecentStats(hours int, maxResults int) ([]*LogStat, error) {
	return s.QueryLogStats(QueryFilter{
		StartTime:     time.Now().Add(-time.Duration(hours) * time.Hour),
		MaxResults:    maxResults,
		IncludeMemory: true,
		IncludeDB:     true,
	})
}

// QueryByLevel returns all statistics for a specific log level
func (s *LogStatStore) QueryByLevel(level string, includeMemory bool, includeDB bool) ([]*LogStat, error) {
	return s.QueryLogStats(QueryFilter{
		Level:         level,
		IncludeMemory: includeMemory,
		IncludeDB:     includeDB,
	})
}

// QueryByLoggerPattern returns statistics matching a logger name pattern
func (s *LogStatStore) QueryByLoggerPattern(pattern string, includeMemory bool, includeDB bool) ([]*LogStat, error) {
	return s.QueryLogStats(QueryFilter{
		LoggerRegex:   pattern,
		IncludeMemory: includeMemory,
		IncludeDB:     includeDB,
	})
}

// QueryRecentAggregated returns aggregated statistics for recent time period
func (s *LogStatStore) QueryRecentAggregated(hours int) ([]*AggregatedStat, error) {
	return s.QueryAggregatedStatsOptimized(QueryFilter{
		StartTime:     time.Now().Add(-time.Duration(hours) * time.Hour),
		IncludeMemory: true,
		IncludeDB:     true,
	})
}
