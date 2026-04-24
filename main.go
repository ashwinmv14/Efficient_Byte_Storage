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
	_, err := os.Stat(storePath)
	firstRun := os.IsNotExist(err)

	if firstRun {
		runFirstRun()
	} else {
		runReload()
	}
}

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

func runReload() {
	fmt.Printf("Found %s — reloading...\n\n", storePath)

	_, report, err := MeasureDeserialize(storePath)
	if err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}

	PrintReload(report)
}

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
