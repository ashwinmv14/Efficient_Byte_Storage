package main

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
)

const storePath = "store.bin"
const entryCount = 10_000

func main() {
	// Check if store.bin already exists.
	// First run: build + serialize. Subsequent runs: reload.
	_, err := os.Stat(storePath)
	firstRun := os.IsNotExist(err)

	if firstRun {
		runFirstRun()
	} else {
		runReload()
	}
}

// runFirstRun builds a large store with mixed types,
// serializes it, and prints the first-run report.
func runFirstRun() {
	fmt.Printf("Building store with %d entries...\n\n", entryCount)

	s := buildStore(entryCount)

	report, err := MeasureSerialize(s, storePath)
	if err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}

	PrintFirstRun(report)
}

// runReload loads from disk and prints the reload report.
func runReload() {
	fmt.Printf("Found %s — reloading...\n\n", storePath)

	_, report, err := MeasureDeserialize(storePath)
	if err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}

	PrintReload(report)
}

// buildStore generates a realistic mixed-type dataset.
// 4 value types distributed evenly across entries.
// This simulates a real index: doc text, numeric metadata,
// float scores, and binary payloads (e.g. embeddings).
func buildStore(n int) *Store {
	s := NewStore()

	for i := 0; i < n; i++ {
		key := "key:" + strconv.Itoa(i)

		switch i % 4 {
		case 0:
			// Simulates a text chunk (e.g. log line or doc fragment)
			s.Set(key, TextValue(
				"incident log entry for service "+strconv.Itoa(i),
			))
		case 1:
			// Simulates numeric metadata (timestamp, page, token count)
			s.Set(key, IntValue(int64(rand.Intn(1_000_000))))
		case 2:
			// Simulates a relevance score or embedding similarity
			s.Set(key, FloatValue(rand.Float64()))
		case 3:
			// Simulates a binary payload (compressed embedding, bitmap)
			payload := make([]byte, 128)
			rand.Read(payload)
			s.Set(key, BinaryValue(payload))
		}
	}

	return s
}
