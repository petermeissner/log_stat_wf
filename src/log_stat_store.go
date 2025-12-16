package main

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
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

		// handle timer loggers
		if !strings.Contains(strings.ToLower(loggerName), "peter") && strings.Contains(strings.ToLower(loggerName), "timer") {
			// extract timer id from message field, pattern = "timedObjectId=restjms19.restjms19.SchedMe"
			timerID := "Unknown"
			if msg, ok := logEntry["message"].(string); ok {
				// Use regex to extract timedObjectId value
				timer_regex := regexp.MustCompile(`timedObjectId=([^\s\)]+)`)
				matches := timer_regex.FindStringSubmatch(msg)
				if len(matches) > 1 {
					timerID = matches[1]
				}
			}
			loggerName = loggerName + ":" + timerID
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
