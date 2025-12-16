package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"
)

// LogMessage represents a log entry sent to WebSocket clients
type LogMessage struct {
	Timestamp  string      `json:"timestamp"`
	Host       string      `json:"host"`
	Logger     string      `json:"logger"`
	Level      string      `json:"level"`
	Message    string      `json:"message"`
	StackTrace interface{} `json:"stack_trace,omitempty"` // Can be StackTraceSummary or StackTraceFiltered
}

// StackTraceSummary provides minimal stack trace info for low bandwidth
type StackTraceSummary struct {
	Hash       string `json:"hash"`        // SHA-256 hash for deduplication
	FirstLine  string `json:"first_line"`  // Most relevant frame
	FrameCount int    `json:"frame_count"` // Total number of frames
}

// StackTraceFiltered provides smart filtered stack trace
type StackTraceFiltered struct {
	Hash           string   `json:"hash"`    // SHA-256 hash for deduplication
	RelevantFrames []string `json:"frames"`  // Filtered frames
	OmittedCount   int      `json:"omitted"` // Number of frames filtered out
}

// ClientMessage represents a message from client to server
type ClientMessage struct {
	Action string          `json:"action"` // "subscribe", "update", "ping"
	Data   json.RawMessage `json:"data"`
}

// ServerMessage represents a message from server to client
type ServerMessage struct {
	Type string      `json:"type"` // "log", "batch", "stats", "error", "pong"
	Data interface{} `json:"data"`
}

// BatchMessage contains multiple log messages
type BatchMessage struct {
	Messages []*LogMessage `json:"messages"`
	Count    int           `json:"count"`
}

// StatsMessage provides client statistics
type StatsMessage struct {
	Connected      int `json:"connected"`     // Number of connected clients
	TotalClients   int `json:"total_clients"` // Max clients (20)
	MessagesQueued int `json:"queued"`        // Messages in send buffer
	Dropped        int `json:"dropped"`       // Messages dropped due to rate limiting
}

// ErrorMessage provides error information
type ErrorMessage struct {
	Code    string `json:"code"`    // Error code
	Message string `json:"message"` // Human-readable message
}

// RawLogEntry represents the incoming log data structure
type RawLogEntry struct {
	Timestamp  time.Time
	Host       string
	Logger     string
	Level      string
	Message    string
	StackTrace string // Single string field as specified
}

// TransformMessage converts a raw log entry to a WebSocket message with filtered stack trace
func TransformMessage(raw *RawLogEntry, filter *MessageFilter) *LogMessage {
	msg := &LogMessage{
		Timestamp: raw.Timestamp.Format(time.RFC3339),
		Host:      raw.Host,
		Logger:    raw.Logger,
		Level:     raw.Level,
		Message:   raw.Message,
	}

	// Process stack trace based on mode
	if raw.StackTrace != "" && filter != nil {
		msg.StackTrace = filter.ProcessStackTrace(raw.StackTrace)
	}

	return msg
}

// computeStackTraceHash generates a SHA-256 hash of the stack trace for deduplication
func computeStackTraceHash(stackTrace string) string {
	hash := sha256.Sum256([]byte(stackTrace))
	return hex.EncodeToString(hash[:])
}

// extractFirstRelevantFrame extracts the first meaningful frame from a stack trace
func extractFirstRelevantFrame(stackTrace string) string {
	lines := strings.Split(stackTrace, "\n")

	// Look for the first line that looks like a stack frame
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Skip common exception header lines
		if strings.Contains(trimmed, "Exception:") || strings.Contains(trimmed, "Error:") {
			continue
		}

		// Look for typical stack frame patterns
		if strings.Contains(trimmed, ".java:") ||
			strings.Contains(trimmed, ".kt:") ||
			strings.Contains(trimmed, "(") && strings.Contains(trimmed, ")") {
			return trimmed
		}
	}

	// If no frame found, return first non-empty line
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			return trimmed
		}
	}

	return "Unknown"
}

// countStackFrames counts the number of frames in a stack trace
func countStackFrames(stackTrace string) int {
	lines := strings.Split(stackTrace, "\n")
	count := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Count lines that look like stack frames
		if strings.Contains(trimmed, ".java:") ||
			strings.Contains(trimmed, ".kt:") ||
			(strings.Contains(trimmed, "(") && strings.Contains(trimmed, ")")) {
			count++
		}
	}

	return count
}
