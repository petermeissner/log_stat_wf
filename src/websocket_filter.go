package main

import (
	"regexp"
	"strings"

	"github.com/gobwas/glob"
)

// ClientSubscription defines what log messages a client wants to receive
type ClientSubscription struct {
	// Host filtering
	HostPatterns []string `json:"host_patterns"` // e.g., ["prod-*", "server-01"]

	// Logger filtering
	LoggerPatterns []string `json:"logger_patterns"` // e.g., ["com.example.*", "timer"]

	// Level filtering
	Levels []string `json:"levels"` // e.g., ["ERROR", "FATAL"]

	// Message content filtering
	MessageContains []string `json:"message_contains"` // e.g., ["timeout", "failed"]
	MessageExcludes []string `json:"message_excludes"` // e.g., ["debug info"]
	MessageRegex    string   `json:"message_regex"`    // Optional regex

	// Stack trace control
	StackTraceMode    string   `json:"stack_trace_mode"`    // "summary", "filtered"
	StackTraceInclude []string `json:"stack_trace_include"` // Package patterns to include
	StackTraceExclude []string `json:"stack_trace_exclude"` // Package patterns to exclude

	// Rate limiting
	MaxMessagesPerSecond int `json:"max_rate"` // 0 = unlimited

	// Batching
	BatchTimeoutMs int `json:"batch_timeout_ms"` // Send batch after timeout
}

// MessageFilter performs efficient filtering using compiled patterns
type MessageFilter struct {
	subscription *ClientSubscription

	// Compiled patterns for performance
	hostGlobs    []glob.Glob
	loggerGlobs  []glob.Glob
	messageRegex *regexp.Regexp
	stackInclude []glob.Glob
	stackExclude []glob.Glob
}

// NewMessageFilter creates a filter with compiled patterns
func NewMessageFilter(sub *ClientSubscription) (*MessageFilter, error) {
	filter := &MessageFilter{
		subscription: sub,
	}

	// Compile host patterns
	for _, pattern := range sub.HostPatterns {
		g, err := glob.Compile(pattern)
		if err != nil {
			return nil, err
		}
		filter.hostGlobs = append(filter.hostGlobs, g)
	}

	// Compile logger patterns
	for _, pattern := range sub.LoggerPatterns {
		g, err := glob.Compile(pattern)
		if err != nil {
			return nil, err
		}
		filter.loggerGlobs = append(filter.loggerGlobs, g)
	}

	// Compile message regex if provided
	if sub.MessageRegex != "" {
		re, err := regexp.Compile(sub.MessageRegex)
		if err != nil {
			return nil, err
		}
		filter.messageRegex = re
	}

	// Compile stack trace include patterns
	for _, pattern := range sub.StackTraceInclude {
		g, err := glob.Compile(pattern)
		if err != nil {
			return nil, err
		}
		filter.stackInclude = append(filter.stackInclude, g)
	}

	// Compile stack trace exclude patterns
	for _, pattern := range sub.StackTraceExclude {
		g, err := glob.Compile(pattern)
		if err != nil {
			return nil, err
		}
		filter.stackExclude = append(filter.stackExclude, g)
	}

	return filter, nil
}

// Matches checks if a log message passes all subscription filters
func (f *MessageFilter) Matches(msg *RawLogEntry) bool {
	// Host filtering
	if len(f.hostGlobs) > 0 {
		matched := false
		for _, g := range f.hostGlobs {
			if g.Match(msg.Host) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Logger filtering
	if len(f.loggerGlobs) > 0 {
		matched := false
		for _, g := range f.loggerGlobs {
			if g.Match(msg.Logger) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Level filtering
	if len(f.subscription.Levels) > 0 {
		matched := false
		for _, level := range f.subscription.Levels {
			if strings.EqualFold(level, msg.Level) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Message contains filtering
	if len(f.subscription.MessageContains) > 0 {
		matched := false
		lowerMsg := strings.ToLower(msg.Message)
		for _, substr := range f.subscription.MessageContains {
			if strings.Contains(lowerMsg, strings.ToLower(substr)) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Message excludes filtering
	if len(f.subscription.MessageExcludes) > 0 {
		lowerMsg := strings.ToLower(msg.Message)
		for _, substr := range f.subscription.MessageExcludes {
			if strings.Contains(lowerMsg, strings.ToLower(substr)) {
				return false
			}
		}
	}

	// Message regex filtering
	if f.messageRegex != nil {
		if !f.messageRegex.MatchString(msg.Message) {
			return false
		}
	}

	return true
}

// ProcessStackTrace transforms stack trace based on subscription mode
func (f *MessageFilter) ProcessStackTrace(stackTrace string) interface{} {
	if stackTrace == "" {
		return nil
	}

	mode := f.subscription.StackTraceMode
	if mode == "" {
		mode = "summary" // Default mode
	}

	hash := computeStackTraceHash(stackTrace)

	switch mode {
	case "summary":
		return &StackTraceSummary{
			Hash:       hash,
			FirstLine:  extractFirstRelevantFrame(stackTrace),
			FrameCount: countStackFrames(stackTrace),
		}

	case "filtered":
		frames := f.filterStackTraceFrames(stackTrace)
		totalFrames := countStackFrames(stackTrace)
		omitted := totalFrames - len(frames)
		if omitted < 0 {
			omitted = 0
		}

		return &StackTraceFiltered{
			Hash:           hash,
			RelevantFrames: frames,
			OmittedCount:   omitted,
		}

	default:
		// Fallback to summary
		return &StackTraceSummary{
			Hash:       hash,
			FirstLine:  extractFirstRelevantFrame(stackTrace),
			FrameCount: countStackFrames(stackTrace),
		}
	}
}

// filterStackTraceFrames applies include/exclude patterns to extract relevant frames
func (f *MessageFilter) filterStackTraceFrames(stackTrace string) []string {
	lines := strings.Split(stackTrace, "\n")
	var result []string
	var firstFrame string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Skip exception header lines
		if strings.Contains(trimmed, "Exception:") || strings.Contains(trimmed, "Error:") {
			continue
		}

		// Check if it looks like a stack frame
		isFrame := strings.Contains(trimmed, ".java:") ||
			strings.Contains(trimmed, ".kt:") ||
			(strings.Contains(trimmed, "(") && strings.Contains(trimmed, ")"))

		if !isFrame {
			continue
		}

		// Keep track of first frame (always include)
		if firstFrame == "" {
			firstFrame = trimmed
		}

		// Extract the class/package name from the stack frame
		// E.g., "at org.jboss.as.ejb3.component.EJBComponent.invoke(EJBComponent.java:123)"
		// should extract "org.jboss.as.ejb3.component.EJBComponent"
		className := extractClassName(trimmed)

		// Apply include patterns (if specified, only include matching frames)
		if len(f.stackInclude) > 0 {
			matched := false
			for _, g := range f.stackInclude {
				// Match against both full line and class name
				if g.Match(trimmed) || g.Match(className) {
					matched = true
					break
				}
			}
			if !matched && trimmed != firstFrame {
				continue
			}
		}

		// Apply exclude patterns
		if len(f.stackExclude) > 0 {
			excluded := false
			for _, g := range f.stackExclude {
				// Match against both full line and class name
				if g.Match(trimmed) || g.Match(className) {
					excluded = true
					break
				}
			}
			if excluded && trimmed != firstFrame {
				continue
			}
		}

		result = append(result, trimmed)
	}

	// Ensure first frame is always included
	if firstFrame != "" && len(result) == 0 {
		result = append(result, firstFrame)
	}

	return result
}

// extractClassName extracts the class/package name from a stack trace line
// E.g., "at org.jboss.as.ejb3.component.EJBComponent.invoke(EJBComponent.java:123)"
// returns "org.jboss.as.ejb3.component.EJBComponent"
func extractClassName(stackLine string) string {
	// Remove leading "at " if present
	line := strings.TrimPrefix(stackLine, "at ")
	line = strings.TrimSpace(line)

	// Find the opening parenthesis
	parenIdx := strings.Index(line, "(")
	if parenIdx > 0 {
		line = line[:parenIdx]
	}

	// Remove method name (everything after last dot before parenthesis)
	lastDot := strings.LastIndex(line, ".")
	if lastDot > 0 {
		line = line[:lastDot]
	}

	return line
}

// GetDefaultSubscription returns the default subscription (INFO and above)
func GetDefaultSubscription() *ClientSubscription {
	return &ClientSubscription{
		Levels:         []string{"INFO", "WARN", "ERROR", "FATAL"},
		StackTraceMode: "summary",
	}
}
