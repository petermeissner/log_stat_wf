package main

import (
	"embed"
	"log"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
)

//go:embed web/*
var webFiles embed.FS

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

	// Unified API endpoint for stats with optional timestamp filtering
	app.Get("/api/stats", func(c *fiber.Ctx) error {
		minTS := c.Query("min_ts")
		maxTS := c.Query("max_ts")

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

		return c.JSON(allStats)
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
