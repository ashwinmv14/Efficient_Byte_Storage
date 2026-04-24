package main

import (
	"bytes"
	"os"
	"testing"
)

// ── helpers ──────────────────────────────────────────────────────
// tmpFile returns a temp path and registers cleanup so the
// test file is always deleted when the test finishes.
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

// seedStore builds a store with all 4 value types.
// Used by multiple tests so the dataset is consistent.
func seedStore() *Store {
	s := NewStore()
	s.Set("name",    TextValue("alice"))
	s.Set("age",     IntValue(30))
	s.Set("score",   FloatValue(98.6))
	s.Set("payload", BinaryValue([]byte{0xDE, 0xAD, 0xBE, 0xEF}))
	return s
}

// ── store tests ──────────────────────────────────────────────────

// TestStore_SetGet verifies basic set and get for all 4 value types.
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

// TestStore_Delete verifies that deleted keys are gone.
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

// TestStore_Overwrite verifies that setting the same key twice
// keeps the latest value, not the first.
func TestStore_Overwrite(t *testing.T) {
	s := NewStore()
	s.Set("key", IntValue(1))
	s.Set("key", IntValue(99))

	v, _ := s.Get("key")
	if AsInt(v) != 99 {
		t.Errorf("overwrite: got %d, want 99", AsInt(v))
	}
}

// TestStore_MissingKey verifies Get returns false for unknown keys.
func TestStore_MissingKey(t *testing.T) {
	s := NewStore()
	if _, ok := s.Get("ghost"); ok {
		t.Error("expected miss for unknown key")
	}
}

// ── serializer tests ─────────────────────────────────────────────

// TestRoundTrip is the core correctness test.
// Serialize a store → wipe it → deserialize → verify every key/value.
func TestRoundTrip(t *testing.T) {
	path := tmpFile(t)
	original := seedStore()

	if err := Serialize(original, path); err != nil {
		t.Fatalf("serialize: %v", err)
	}

	// Deliberately set original to nil to prove we're loading
	// from disk, not from any leftover in-memory state.
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

	// Verify specific values — not just types.
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

// TestRoundTrip_LargeStore verifies correctness at scale —
// 10k mixed-type entries all survive serialize → deserialize.
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

	// Spot-check a sample of entries — not every one,
	// but enough to catch format bugs.
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

// TestRoundTrip_EmptyStore verifies that an empty store
// serializes and reloads without error.
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

// TestMagicByteDetection verifies that loading a garbage file
// returns a clear error instead of silently loading bad data.
func TestMagicByteDetection(t *testing.T) {
	path := tmpFile(t)

	// Write garbage — not a valid store file.
	if err := os.WriteFile(path, []byte("this is not a store file"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Deserialize(path)
	if err == nil {
		t.Error("expected error for garbage file, got nil")
	}
}

// TestVersionMismatch verifies that a file with a different
// version byte is rejected cleanly.
func TestVersionMismatch(t *testing.T) {
	path := tmpFile(t)

	// Write magic bytes + wrong version (99)
	data := []byte{'T', 'S', 'K', '2', 99}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Deserialize(path)
	if err == nil {
		t.Error("expected error for version mismatch, got nil")
	}
}

// TestUnicodeKeys verifies keys with unicode characters
// survive the round trip — key bytes are raw UTF-8.
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