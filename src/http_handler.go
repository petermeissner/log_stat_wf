package main

import (
	"embed"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
)

//go:embed web/*
var webFiles embed.FS

// logRequest logs HTTP request parameters and execution time
func logRequest(endpoint string, params map[string]string, start time.Time, resultCount int, err error) {
	duration := time.Since(start)
	status := "OK"
	if err != nil {
		status = fmt.Sprintf("ERROR: %v", err)
	}

	paramStr := ""
	for k, v := range params {
		if paramStr != "" {
			paramStr += ", "
		}
		paramStr += fmt.Sprintf("%s=%s", k, v)
	}

	log.Printf("[HTTP] %s | %s | Results: %d | Duration: %v | Params: {%s}",
		endpoint, status, resultCount, duration, paramStr)
}

func filterStatsByTimestamp(stats []*LogStat, minTS, maxTS string) []*LogStat {
	if minTS == "" && maxTS == "" {
		return stats
	}

	var minTime, maxTime time.Time
	var err error

	if minTS != "" {
		minTime, err = time.Parse(time.RFC3339, minTS)
		if err != nil {
			return stats // If parsing fails, return unfiltered
		}
	}

	if maxTS != "" {
		maxTime, err = time.Parse(time.RFC3339, maxTS)
		if err != nil {
			return stats // If parsing fails, return unfiltered
		}
	}

	var filtered []*LogStat
	for _, stat := range stats {
		statTime, err := time.Parse(time.RFC3339, stat.BucketTS)
		if err != nil {
			continue
		}

		includeRow := true
		if minTS != "" && statTime.Before(minTime) {
			includeRow = false
		}
		if maxTS != "" && statTime.After(maxTime) {
			includeRow = false
		}

		if includeRow {
			filtered = append(filtered, stat)
		}
	}

	return filtered
}

func startHTTPServer(addr string, store *LogStatStore) {
	app := fiber.New(fiber.Config{
		AppName: "WildFly Log Statistics",
	})

	// Legacy API endpoint (kept for backward compatibility)
	app.Get("/api/stats", func(c *fiber.Ctx) error {
		start := time.Now()
		minTS := c.Query("min_ts")
		maxTS := c.Query("max_ts")

		params := map[string]string{
			"min_ts": minTS,
			"max_ts": maxTS,
		}

		// Get all current stats
		current := store.GetAll()

		// Get all historical stats
		historical, err := store.QueryDatabase()
		if err != nil {
			historical = []*LogStat{}
		}

		// Merge current and historical
		var allStats []*LogStat
		allStats = append(allStats, current...)
		allStats = append(allStats, historical...)

		// Filter by timestamp if provided
		allStats = filterStatsByTimestamp(allStats, minTS, maxTS)

		logRequest("/api/stats", params, start, len(allStats), nil)
		return c.JSON(allStats)
	})

	// New query API with filters
	app.Get("/api/query/stats", func(c *fiber.Ctx) error {
		start := time.Now()

		filter := QueryFilter{
			Level:         c.Query("level"),
			LoggerRegex:   c.Query("logger_regex"),
			IncludeMemory: c.QueryBool("include_memory", true),
			IncludeDB:     c.QueryBool("include_db", true),
		}

		params := map[string]string{
			"level":          c.Query("level"),
			"logger_regex":   c.Query("logger_regex"),
			"start_time":     c.Query("start_time"),
			"end_time":       c.Query("end_time"),
			"max_results":    c.Query("max_results"),
			"include_memory": fmt.Sprintf("%v", filter.IncludeMemory),
			"include_db":     fmt.Sprintf("%v", filter.IncludeDB),
		}

		// Parse time filters
		if startTime := c.Query("start_time"); startTime != "" {
			if t, err := time.Parse(time.RFC3339, startTime); err == nil {
				filter.StartTime = t
			}
		}
		if endTime := c.Query("end_time"); endTime != "" {
			if t, err := time.Parse(time.RFC3339, endTime); err == nil {
				filter.EndTime = t
			}
		}

		// Parse max results
		if maxResults := c.Query("max_results"); maxResults != "" {
			if n, err := strconv.Atoi(maxResults); err == nil {
				filter.MaxResults = n
			}
		}

		stats, err := store.QueryLogStats(filter)
		if err != nil {
			logRequest("/api/query/stats", params, start, 0, err)
			return c.Status(500).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		logRequest("/api/query/stats", params, start, len(stats), nil)
		return c.JSON(stats)
	})

	// Aggregated stats API
	app.Get("/api/query/aggregated", func(c *fiber.Ctx) error {
		start := time.Now()

		filter := QueryFilter{
			Level:         c.Query("level"),
			LoggerRegex:   c.Query("logger_regex"),
			IncludeMemory: c.QueryBool("include_memory", true),
			IncludeDB:     c.QueryBool("include_db", true),
		}

		params := map[string]string{
			"level":          c.Query("level"),
			"logger_regex":   c.Query("logger_regex"),
			"start_time":     c.Query("start_time"),
			"end_time":       c.Query("end_time"),
			"max_results":    c.Query("max_results"),
			"include_memory": fmt.Sprintf("%v", filter.IncludeMemory),
			"include_db":     fmt.Sprintf("%v", filter.IncludeDB),
		}

		// Parse time filters
		if startTime := c.Query("start_time"); startTime != "" {
			if t, err := time.Parse(time.RFC3339, startTime); err == nil {
				filter.StartTime = t
			}
		}
		if endTime := c.Query("end_time"); endTime != "" {
			if t, err := time.Parse(time.RFC3339, endTime); err == nil {
				filter.EndTime = t
			}
		}

		// Parse max results
		if maxResults := c.Query("max_results"); maxResults != "" {
			if n, err := strconv.Atoi(maxResults); err == nil {
				filter.MaxResults = n
			}
		}

		aggregated, err := store.QueryAggregatedStatsOptimized(filter)
		if err != nil {
			logRequest("/api/query/aggregated", params, start, 0, err)
			return c.Status(500).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		logRequest("/api/query/aggregated", params, start, len(aggregated), nil)
		return c.JSON(aggregated)
	})

	// Quick helpers
	app.Get("/api/query/recent", func(c *fiber.Ctx) error {
		start := time.Now()
		hours := c.QueryInt("hours", 24)
		maxResults := c.QueryInt("max_results", 1000)

		params := map[string]string{
			"hours":       fmt.Sprintf("%d", hours),
			"max_results": fmt.Sprintf("%d", maxResults),
		}

		stats, err := store.QueryRecentStats(hours, maxResults)
		if err != nil {
			logRequest("/api/query/recent", params, start, 0, err)
			return c.Status(500).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		logRequest("/api/query/recent", params, start, len(stats), nil)
		return c.JSON(stats)
	})

	app.Get("/api/query/by_level", func(c *fiber.Ctx) error {
		start := time.Now()
		level := c.Query("level")
		if level == "" {
			logRequest("/api/query/by_level", map[string]string{"level": "missing"}, start, 0, fmt.Errorf("level parameter required"))
			return c.Status(400).JSON(fiber.Map{
				"error": "level parameter is required",
			})
		}

		includeMemory := c.QueryBool("include_memory", true)
		includeDB := c.QueryBool("include_db", true)

		params := map[string]string{
			"level":          level,
			"include_memory": fmt.Sprintf("%v", includeMemory),
			"include_db":     fmt.Sprintf("%v", includeDB),
		}

		stats, err := store.QueryByLevel(level, includeMemory, includeDB)
		if err != nil {
			logRequest("/api/query/by_level", params, start, 0, err)
			return c.Status(500).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		logRequest("/api/query/by_level", params, start, len(stats), nil)
		return c.JSON(stats)
	})

	app.Get("/api/query/aggregated_recent", func(c *fiber.Ctx) error {
		start := time.Now()
		hours := c.QueryInt("hours", 24)

		params := map[string]string{
			"hours": fmt.Sprintf("%d", hours),
		}

		aggregated, err := store.QueryRecentAggregated(hours)
		if err != nil {
			logRequest("/api/query/aggregated_recent", params, start, 0, err)
			return c.Status(500).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		logRequest("/api/query/aggregated_recent", params, start, len(aggregated), nil)
		return c.JSON(aggregated)
	})

	// Prometheus metrics endpoint
	app.Get("/metrics", func(c *fiber.Ctx) error {
		start := time.Now()

		// Get stats from the last completed bucket
		stats, err := store.QueryRecentStats(1, 10000) // Last 1 hour, max 10k results
		if err != nil {
			logRequest("/metrics", map[string]string{}, start, 0, err)
			return c.Status(500).SendString("# Error retrieving metrics\n")
		}

		// Find the most recent complete bucket timestamp
		var latestBucket string
		bucketCounts := make(map[string]int)
		for _, stat := range stats {
			bucketCounts[stat.BucketTS]++
			if latestBucket == "" || stat.BucketTS > latestBucket {
				latestBucket = stat.BucketTS
			}
		}

		// If we have multiple buckets, use the second most recent (last completed)
		var targetBucket string
		if len(bucketCounts) > 1 {
			sortedBuckets := make([]string, 0, len(bucketCounts))
			for bucket := range bucketCounts {
				sortedBuckets = append(sortedBuckets, bucket)
			}
			// Simple sort by comparing strings (RFC3339 is sortable)
			for i := 0; i < len(sortedBuckets); i++ {
				for j := i + 1; j < len(sortedBuckets); j++ {
					if sortedBuckets[i] < sortedBuckets[j] {
						sortedBuckets[i], sortedBuckets[j] = sortedBuckets[j], sortedBuckets[i]
					}
				}
			}
			targetBucket = sortedBuckets[1] // Second most recent
		} else if len(bucketCounts) == 1 {
			targetBucket = latestBucket
		} else {
			logRequest("/metrics", map[string]string{}, start, 0, nil)
			c.Set("Content-Type", "text/plain; version=0.0.4")
			return c.SendString("# No metrics available\n")
		}

		// Filter stats for target bucket only
		var bucketStats []*LogStat
		for _, stat := range stats {
			if stat.BucketTS == targetBucket {
				bucketStats = append(bucketStats, stat)
			}
		}

		// Generate Prometheus metrics format
		output := "# HELP wildfly_log_messages_total Total number of log messages by level and logger\n"
		output += "# TYPE wildfly_log_messages_total counter\n"

		for _, stat := range bucketStats {
			// Escape label values for Prometheus format
			hostname := stat.HostName
			level := stat.Level
			logger := stat.Logger

			output += fmt.Sprintf("wildfly_log_messages_total{hostname=\"%s\",level=\"%s\",logger=\"%s\"} %d\n",
				hostname, level, logger, stat.N)
		}

		// Add bucket timestamp as metadata
		output += "\n# HELP wildfly_log_bucket_timestamp_seconds Timestamp of the metrics bucket\n"
		output += "# TYPE wildfly_log_bucket_timestamp_seconds gauge\n"

		bucketTime, err := time.Parse(time.RFC3339, targetBucket)
		if err == nil {
			output += fmt.Sprintf("wildfly_log_bucket_timestamp_seconds %d\n", bucketTime.Unix())
		}

		logRequest("/metrics", map[string]string{"bucket": targetBucket}, start, len(bucketStats), nil)
		c.Set("Content-Type", "text/plain; version=0.0.4")
		return c.SendString(output)
	})

	// Serve embedded static files (CSS, JS)
	app.Use("/", filesystem.New(filesystem.Config{
		Root:       http.FS(webFiles),
		PathPrefix: "web",
		Browse:     false,
	}))

	log.Printf("=== Fiber HTTP server starting on %s ===\n", addr)
	if err := app.Listen(addr); err != nil {
		log.Fatalf("HTTP server error: %v\n", err)
	}
}
