package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

// logWriter implements io.Writer with custom timestamp format
type logWriter struct {
	writer io.Writer
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	message := fmt.Sprintf("[%s] %s", timestamp, string(p))
	return w.writer.Write([]byte(message))
}

// setupLogging configures logging to both console and rotating file
func setupLogging(logFilePath string) {
	// Create lumberjack logger for file rotation
	fileLogger := &lumberjack.Logger{
		Filename:   logFilePath,
		MaxSize:    10,    // megabytes
		MaxBackups: 3,     // keep last 3 files
		MaxAge:     0,     // don't delete by age
		Compress:   false, // set to true if you want .gz compression
	}

	// Write to both console (stderr) and rotating file
	multiWriter := io.MultiWriter(os.Stderr, fileLogger)

	// Wrap with custom timestamp writer
	customWriter := &logWriter{writer: multiWriter}

	// Set the logger output
	log.SetOutput(customWriter)
	log.SetFlags(0) // Disable default flags since we handle timestamps
}
