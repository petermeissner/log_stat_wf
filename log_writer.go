package main

import (
	"fmt"
	"os"
	"time"
)

// logWriter implements io.Writer with custom timestamp format
type logWriter struct{}

func (w *logWriter) Write(p []byte) (n int, err error) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(os.Stderr, "[%s] %s", timestamp, string(p))
	return len(p), nil
}
