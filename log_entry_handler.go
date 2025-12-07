package main

import (
	"encoding/json"
	"fmt"
	"log"
)

func handleLogEntry(line string, store *LogStatStore) {
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
		stat := store.AddOrUpdate(hostName, level, loggerName)

		// Simple output
		log.Printf("[host: %s,  loggerName: %s, level:%s] = Count: %d\n", hostName, loggerName, level, stat.N)
	} else {
		// Parse error, just print the line
		log.Fatalf("[%s]", line)
	}
}
