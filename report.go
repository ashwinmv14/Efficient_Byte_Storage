package main

import (
	"fmt"
	"os"
	"runtime"
	"time"
)

// Report holds the numbers we care about.
// First run fills SerializeTime + FileSizeBytes.
// Future runs fill ReloadTime + HeapDeltaBytes.
type Report struct {
	EntryCount     int
	SerializeTime  time.Duration
	FileSizeBytes  int64
	ReloadTime     time.Duration
	HeapDeltaBytes uint64
}

// MeasureSerialize times the Serialize call and
// stats the file immediately after to get exact disk size.
func MeasureSerialize(s *Store, path string) (Report, error) {
	start := time.Now()

	if err := Serialize(s, path); err != nil {
		return Report{}, err
	}

	elapsed := time.Since(start)

	// os.Stat gives us the file size without reading the file.
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

// MeasureDeserialize captures heap before and after loading
// so we can report exactly how many bytes the store occupies in RAM.
func MeasureDeserialize(path string) (*Store, Report, error) {
	// Force GC before measuring so leftover allocations
	// from serialization don't pollute our heap delta number.
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
// e.g. 48392 → "47.3 KB"
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
