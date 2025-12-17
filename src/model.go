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

// SystemInfo represents runtime and memory statistics
type SystemInfo struct {
	Hostname     string `json:"hostname"`
	NumGoroutine int    `json:"num_goroutine"`
	NumCPU       int    `json:"num_cpu"`
	GoVersion    string `json:"go_version"`

	// Memory statistics (in bytes)
	Alloc      uint64 `json:"alloc"`       // Bytes allocated and in use
	TotalAlloc uint64 `json:"total_alloc"` // Bytes allocated (even if freed)
	Sys        uint64 `json:"sys"`         // Bytes obtained from system
	Lookups    uint64 `json:"lookups"`     // Number of pointer lookups
	Mallocs    uint64 `json:"mallocs"`     // Number of mallocs
	Frees      uint64 `json:"frees"`       // Number of frees

	// Heap memory statistics
	HeapAlloc    uint64 `json:"heap_alloc"`    // Bytes allocated and in use
	HeapSys      uint64 `json:"heap_sys"`      // Bytes obtained from system
	HeapIdle     uint64 `json:"heap_idle"`     // Bytes in idle spans
	HeapInuse    uint64 `json:"heap_inuse"`    // Bytes in non-idle span
	HeapReleased uint64 `json:"heap_released"` // Bytes released to the OS
	HeapObjects  uint64 `json:"heap_objects"`  // Total number of allocated objects

	// Stack memory
	StackInuse uint64 `json:"stack_inuse"` // Bytes used by stack allocator
	StackSys   uint64 `json:"stack_sys"`   // Bytes obtained from system for stack

	// GC statistics
	NumGC        uint32 `json:"num_gc"`         // Number of completed GC cycles
	LastGC       uint64 `json:"last_gc"`        // Time of last GC (Unix timestamp in nanoseconds)
	PauseTotalNs uint64 `json:"pause_total_ns"` // Cumulative nanoseconds in GC stop-the-world pauses
}
