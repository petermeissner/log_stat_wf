package main

import "fmt"

// LogStat represents statistics for a log pattern
type LogStat struct {
	ID            int    // unique identifier
	TS_Start      string // first occurrence timestamp (12 character string)
	TS_Interval_S int    // elapsed seconds since first occurrence
	Level         string // log level (8 character string)
	Logger        string // logger name
	N             int    // counter of occurrences
}

// String returns a formatted string representation of LogStat
func (ls *LogStat) String() string {
	return fmt.Sprintf("ID:%d | Start:%s | Interval:%ds | Level:%-8s | Logger:%-30s | Count:%d",
		ls.ID, ls.TS_Start, ls.TS_Interval_S, ls.Level, ls.Logger, ls.N)
}
