package main

import (
	"bytes"
	"os"
	"testing"
	"time"
)

// ── helpers ──────────────────────────────────────────────────────
func tmpFile(t *testing.T) string {
	t.Helper()
	f, err := os.CreateTemp("", "store_test_*.bin")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })
	return f.Name()
}

func seedStore() *Store {
	s := NewStore()
	s.Set("name", TextValue("alice"))
	s.Set("age", IntValue(30))
	s.Set("score", FloatValue(98.6))
	s.Set("payload", BinaryValue([]byte{0xDE, 0xAD, 0xBE, 0xEF}))
	return s
}

// ── store tests ──────────────────────────────────────────────────
func TestStore_SetGet(t *testing.T) {
	s := seedStore()

	// text
	v, ok := s.Get("name")
	if !ok || AsText(v) != "alice" {
		t.Errorf("name: got %q, want alice", AsText(v))
	}

	// int
	v, ok = s.Get("age")
	if !ok || AsInt(v) != 30 {
		t.Errorf("age: got %d, want 30", AsInt(v))
	}

	// float
	v, ok = s.Get("score")
	if !ok || AsFloat(v) != 98.6 {
		t.Errorf("score: got %f, want 98.6", AsFloat(v))
	}

	// binary
	v, ok = s.Get("payload")
	if !ok || !bytes.Equal(v.Data, []byte{0xDE, 0xAD, 0xBE, 0xEF}) {
		t.Errorf("payload: got %v, want DEADBEEF", v.Data)
	}
}

func TestStore_Delete(t *testing.T) {
	s := seedStore()
	s.Delete("name")

	if _, ok := s.Get("name"); ok {
		t.Error("expected name to be deleted")
	}
	if s.Len() != 3 {
		t.Errorf("expected len 3 after delete, got %d", s.Len())
	}
}

func TestStore_Overwrite(t *testing.T) {
	s := NewStore()
	s.Set("key", IntValue(1))
	s.Set("key", IntValue(99))

	v, _ := s.Get("key")
	if AsInt(v) != 99 {
		t.Errorf("overwrite: got %d, want 99", AsInt(v))
	}
}

func TestStore_MissingKey(t *testing.T) {
	s := NewStore()
	if _, ok := s.Get("ghost"); ok {
		t.Error("expected miss for unknown key")
	}
}

// ── serializer tests ─────────────────────────────────────────────

func TestRoundTrip(t *testing.T) {
	path := tmpFile(t)
	original := seedStore()

	if err := Serialize(original, path); err != nil {
		t.Fatalf("serialize: %v", err)
	}

	original = nil

	loaded, err := Deserialize(path)
	if err != nil {
		t.Fatalf("deserialize: %v", err)
	}

	// Verify all 4 types survived the round trip exactly.
	checks := []struct {
		key      string
		wantType ValueType
	}{
		{"name", TypeText},
		{"age", TypeInt},
		{"score", TypeFloat},
		{"payload", TypeBinary},
	}

	for _, c := range checks {
		v, ok := loaded.Get(c.key)
		if !ok {
			t.Errorf("key %q missing after reload", c.key)
			continue
		}
		if v.Type != c.wantType {
			t.Errorf("key %q: type got %d want %d", c.key, v.Type, c.wantType)
		}
	}

	v, _ := loaded.Get("name")
	if AsText(v) != "alice" {
		t.Errorf("name: got %q want alice", AsText(v))
	}

	v, _ = loaded.Get("age")
	if AsInt(v) != 30 {
		t.Errorf("age: got %d want 30", AsInt(v))
	}

	v, _ = loaded.Get("score")
	if AsFloat(v) != 98.6 {
		t.Errorf("score: got %f want 98.6", AsFloat(v))
	}

	v, _ = loaded.Get("payload")
	if !bytes.Equal(v.Data, []byte{0xDE, 0xAD, 0xBE, 0xEF}) {
		t.Errorf("payload mismatch after reload")
	}
}

func TestRoundTrip_LargeStore(t *testing.T) {
	path := tmpFile(t)
	original := buildBenchStore(10_000)

	if err := Serialize(original, path); err != nil {
		t.Fatalf("serialize: %v", err)
	}

	loaded, err := Deserialize(path)
	if err != nil {
		t.Fatalf("deserialize: %v", err)
	}

	if loaded.Len() != original.Len() {
		t.Errorf("entry count: got %d want %d", loaded.Len(), original.Len())
	}

	for _, key := range []string{
		"key:0", "key:1", "key:2", "key:3",
		"key:999", "key:5000", "key:9999",
	} {
		orig, ok1 := original.Get(key)
		load, ok2 := loaded.Get(key)

		if !ok1 || !ok2 {
			t.Errorf("key %q missing: original=%v loaded=%v", key, ok1, ok2)
			continue
		}
		if orig.Type != load.Type {
			t.Errorf("key %q type mismatch", key)
		}
		if !bytes.Equal(orig.Data, load.Data) {
			t.Errorf("key %q data mismatch", key)
		}
	}
}

func TestRoundTrip_EmptyStore(t *testing.T) {
	path := tmpFile(t)
	s := NewStore()

	if err := Serialize(s, path); err != nil {
		t.Fatalf("serialize empty: %v", err)
	}

	loaded, err := Deserialize(path)
	if err != nil {
		t.Fatalf("deserialize empty: %v", err)
	}
	if loaded.Len() != 0 {
		t.Errorf("expected 0 entries, got %d", loaded.Len())
	}
}

func TestMagicByteDetection(t *testing.T) {
	path := tmpFile(t)

	if err := os.WriteFile(path, []byte("this is not a store file"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Deserialize(path)
	if err == nil {
		t.Error("expected error for garbage file, got nil")
	}
}

func TestVersionMismatch(t *testing.T) {
	path := tmpFile(t)

	data := []byte{'T', 'S', 'K', '2', 99}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Deserialize(path)
	if err == nil {
		t.Error("expected error for version mismatch, got nil")
	}
}

func TestUnicodeKeys(t *testing.T) {
	path := tmpFile(t)
	s := NewStore()
	s.Set("日本語", TextValue("japanese"))
	s.Set("emoji_🔥", TextValue("fire"))

	if err := Serialize(s, path); err != nil {
		t.Fatal(err)
	}
	loaded, err := Deserialize(path)
	if err != nil {
		t.Fatal(err)
	}

	v, ok := loaded.Get("日本語")
	if !ok || AsText(v) != "japanese" {
		t.Error("unicode key failed round trip")
	}
}

func TestReloadLatency(t *testing.T) {
	// build and serialize once
	s := buildBenchStore(10_000)
	path := "latency_test.bin"
	defer os.Remove(path)

	if err := Serialize(s, path); err != nil {
		t.Fatal(err)
	}

	// run 5 reloads and record each time
	times := make([]time.Duration, 5)
	for i := 0; i < 5; i++ {
		start := time.Now()
		loaded, err := Deserialize(path)
		times[i] = time.Since(start)
		if err != nil {
			t.Fatal(err)
		}
		_ = loaded
	}

	t.Logf("Reload latencies across 5 runs:")
	for i, d := range times {
		label := "cold"
		if i > 0 {
			label = "warm"
		}
		t.Logf("  run %d (%s): %v", i+1, label, d)
	}
	t.Logf("cold: %v  →  warm: %v  (%.2fx faster)",
		times[0],
		times[len(times)-1],
		float64(times[0])/float64(times[len(times)-1]),
	)
}
