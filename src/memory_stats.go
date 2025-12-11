package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/shirou/gopsutil/v3/process"
)

// MemoryStats holds memory usage information
type MemoryStats struct {
	// Go runtime memory stats
	HeapAllocMB  float64 // Currently allocated heap memory (MB)
	HeapSysMB    float64 // Total heap memory from OS (MB)
	StackMB      float64 // Stack memory (MB)
	NumGoroutine int     // Number of goroutines

	// Process memory stats (from OS perspective)
	RSSMB float64 // Resident Set Size - actual physical memory used (MB)
	VMSMB float64 // Virtual Memory Size (MB)
}

// GetMemoryStats returns comprehensive memory usage for the current process
// Works cross-platform (Windows/Linux/Mac) without CGO
func GetMemoryStats() (*MemoryStats, error) {
	stats := &MemoryStats{}

	// Get Go runtime memory stats
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	stats.HeapAllocMB = float64(m.Alloc) / 1024 / 1024
	stats.HeapSysMB = float64(m.Sys) / 1024 / 1024
	stats.StackMB = float64(m.StackInuse) / 1024 / 1024
	stats.NumGoroutine = runtime.NumGoroutine()

	// Get process memory stats (RSS/VMS)
	proc, err := process.NewProcess(int32(os.Getpid()))
	if err != nil {
		return stats, err // Return partial stats if process info fails
	}

	memInfo, err := proc.MemoryInfo()
	if err != nil {
		return stats, err // Return partial stats if memory info fails
	}

	stats.RSSMB = float64(memInfo.RSS) / 1024 / 1024
	stats.VMSMB = float64(memInfo.VMS) / 1024 / 1024

	return stats, nil
}

// String returns a formatted string of memory stats
func (m *MemoryStats) String() string {
	return fmt.Sprintf("RSS=%.1fMB VMS=%.1fMB HeapAlloc=%.1fMB HeapSys=%.1fMB Stack=%.1fMB Goroutines=%d",
		m.RSSMB, m.VMSMB, m.HeapAllocMB, m.HeapSysMB, m.StackMB, m.NumGoroutine)
}

func GetMemoryStatsString() string {
	stats, err := GetMemoryStats()
	if err != nil {
		return fmt.Sprintf("Error getting memory stats: %v", err)
	}
	return stats.String()
}
