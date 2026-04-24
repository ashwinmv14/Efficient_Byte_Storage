package main

import (
	"fmt"
	"os"
	"runtime"
	"time"
)

// First run fills SerializeTime + FileSizeBytes.
// Future runs fill ReloadTime + HeapDeltaBytes.
type Report struct {
	EntryCount     int
	SerializeTime  time.Duration
	FileSizeBytes  int64
	ReloadTime     time.Duration
	HeapDeltaBytes uint64
}

func MeasureSerialize(s *Store, path string) (Report, error) {
	start := time.Now()

	if err := Serialize(s, path); err != nil {
		return Report{}, err
	}

	elapsed := time.Since(start)

	info, err := os.Stat(path)
	if err != nil {
		return Report{}, err
	}

	return Report{
		EntryCount:    s.Len(),
		SerializeTime: elapsed,
		FileSizeBytes: info.Size(),
	}, nil
}

func MeasureDeserialize(path string) (*Store, Report, error) {
	runtime.GC()

	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	start := time.Now()

	s, err := Deserialize(path)
	if err != nil {
		return nil, Report{}, err
	}

	elapsed := time.Since(start)

	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	return s, Report{
		EntryCount:     s.Len(),
		ReloadTime:     elapsed,
		HeapDeltaBytes: after.HeapAlloc - before.HeapAlloc,
	}, nil
}

// PrintFirstRun prints the first-run report.
func PrintFirstRun(r Report) {
	fmt.Println("=== FIRST RUN ===")
	fmt.Printf("Entries written  : %d\n", r.EntryCount)
	fmt.Printf("Serialization    : %v\n", r.SerializeTime)
	fmt.Printf("File size        : %s\n", formatBytes(r.FileSizeBytes))
	fmt.Printf("Bytes per entry  : %.1f\n", float64(r.FileSizeBytes)/float64(r.EntryCount))
	fmt.Println()
	fmt.Println("Run again to see reload report.")
}

// PrintReload prints the reload report.
func PrintReload(r Report) {
	fmt.Println("=== RELOAD (subsequent run) ===")
	fmt.Printf("Entries loaded   : %d\n", r.EntryCount)
	fmt.Printf("Reload time      : %v\n", r.ReloadTime)
	fmt.Printf("Heap delta       : %s\n", formatBytes(int64(r.HeapDeltaBytes)))
	fmt.Printf("Bytes/entry RAM  : %.1f\n", float64(r.HeapDeltaBytes)/float64(r.EntryCount))
	fmt.Println()
	fmt.Println("Delete store.bin to run first-run again.")
}

// formatBytes turns raw byte counts into human-readable strings.
func formatBytes(b int64) string {
	switch {
	case b >= 1<<20:
		return fmt.Sprintf("%.2f MB (%d bytes)", float64(b)/float64(1<<20), b)
	case b >= 1<<10:
		return fmt.Sprintf("%.2f KB (%d bytes)", float64(b)/float64(1<<10), b)
	default:
		return fmt.Sprintf("%d bytes", b)
	}
}
