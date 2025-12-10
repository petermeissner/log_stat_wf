package main

import "fmt"

// LogStat represents statistics for a log pattern in a time bucket
type LogStat struct {
	ID               int    // unique identifier
	HostName         string // host name
	BucketTS         string // bucket start time (RFC3339 format, aligned to clock)
	FirstSeenTS      string // timestamp of the first message in this bucket
	BucketDuration_S int    // actual duration of this bucket in seconds (may be less for first bucket)
	Level            string // log level (8 character string)
	Logger           string // logger name
	N                int    // counter of occurrences in this bucket
}

// String returns a formatted string representation of LogStat
func (ls *LogStat) String() string {
	return fmt.Sprintf("ID:%d | Host:%-10s | BucketTS:%s | FirstSeen:%s | Duration:%ds | Level:%-8s | Logger:%-30s | Count:%d",
		ls.ID, ls.HostName, ls.BucketTS, ls.FirstSeenTS, ls.BucketDuration_S, ls.Level, ls.Logger, ls.N)
}
