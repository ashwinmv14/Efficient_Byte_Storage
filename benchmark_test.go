package main

import (
	"encoding/json"
	"math/rand"
	"os"
	"strconv"
	"testing"
)

// ── JSON-compatible representation ──────────────────────────────

type JSONValue struct {
	Type string `json:"type"`
	Data []byte `json:"data"`
}

type JSONStore struct {
	Entries map[string]JSONValue `json:"entries"`
}

func toJSONStore(s *Store) JSONStore {
	js := JSONStore{Entries: make(map[string]JSONValue, s.Len())}
	for k, v := range s.Entries() {
		typeName := map[ValueType]string{
			TypeText:   "text",
			TypeInt:    "int",
			TypeFloat:  "float",
			TypeBinary: "binary",
		}[v.Type]
		js.Entries[k] = JSONValue{Type: typeName, Data: v.Data}
	}
	return js
}

func serializeJSON(js JSONStore, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(js)
}

func deserializeJSON(path string) (JSONStore, error) {
	f, err := os.Open(path)
	if err != nil {
		return JSONStore{}, err
	}
	defer f.Close()
	var js JSONStore
	return js, json.NewDecoder(f).Decode(&js)
}

// ── Shared setup ─────────────────────────────────────────────────

func buildBenchStore(n int) *Store {
	s := NewStore()
	for i := 0; i < n; i++ {
		key := "key:" + strconv.Itoa(i)
		switch i % 4 {
		case 0:
			s.Set(key, TextValue("incident log entry for service "+strconv.Itoa(i)))
		case 1:
			s.Set(key, IntValue(int64(rand.Intn(1_000_000))))
		case 2:
			s.Set(key, FloatValue(rand.Float64()))
		case 3:
			payload := make([]byte, 128)
			rand.Read(payload)
			s.Set(key, BinaryValue(payload))
		}
	}
	return s
}

// ── Size comparison (not a benchmark, just a Test) ───────────────
// Run with: go test -v -run TestSizeComparison

func TestSizeComparison(t *testing.T) {
	s := buildBenchStore(10_000)
	js := toJSONStore(s)

	// Write both formats
	if err := Serialize(s, "bench_custom.bin"); err != nil {
		t.Fatal(err)
	}
	if err := serializeJSON(js, "bench_json.json"); err != nil {
		t.Fatal(err)
	}
	defer os.Remove("bench_custom.bin")
	defer os.Remove("bench_json.json")

	customInfo, _ := os.Stat("bench_custom.bin")
	jsonInfo, _ := os.Stat("bench_json.json")

	customSize := customInfo.Size()
	jsonSize := jsonInfo.Size()
	ratio := float64(jsonSize) / float64(customSize)

	t.Logf("Entries          : 10,000")
	t.Logf("Custom format    : %s", formatBytes(customSize))
	t.Logf("JSON format      : %s", formatBytes(jsonSize))
	t.Logf("JSON/Custom ratio: %.2fx larger", ratio)
	t.Logf("Space saved      : %s", formatBytes(jsonSize-customSize))
}

// ── Serialize benchmarks ─────────────────────────────────────────
// Run with: go test -bench=BenchmarkSerialize -benchmem -run=^$

func BenchmarkSerialize_Custom(b *testing.B) {
	s := buildBenchStore(10_000)
	b.ResetTimer() // don't count store build time
	for i := 0; i < b.N; i++ {
		Serialize(s, "bench_custom.bin")
	}
	b.Cleanup(func() { os.Remove("bench_custom.bin") })
}

func BenchmarkSerialize_JSON(b *testing.B) {
	s := buildBenchStore(10_000)
	js := toJSONStore(s)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		serializeJSON(js, "bench_json.json")
	}
	b.Cleanup(func() { os.Remove("bench_json.json") })
}

// ── Deserialize benchmarks ───────────────────────────────────────
// Run with: go test -bench=BenchmarkDeserialize -benchmem -run=^$

func BenchmarkDeserialize_Custom(b *testing.B) {
	s := buildBenchStore(10_000)
	Serialize(s, "bench_custom.bin")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Deserialize("bench_custom.bin")
	}
	b.Cleanup(func() { os.Remove("bench_custom.bin") })
}

func BenchmarkDeserialize_JSON(b *testing.B) {
	s := buildBenchStore(10_000)
	js := toJSONStore(s)
	serializeJSON(js, "bench_json.json")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		deserializeJSON("bench_json.json")
	}
	b.Cleanup(func() { os.Remove("bench_json.json") })
}
