package main

import (
	"encoding/binary"
	"math"
)

// ValueType is a 1-byte tag that tells the serializer
// how to encode/decode the value. Kept as uint8 = 1 byte on disk.
type ValueType uint8

const (
	TypeText   ValueType = 0
	TypeInt    ValueType = 1
	TypeFloat  ValueType = 2
	TypeBinary ValueType = 3
)

// Value holds a type tag and raw bytes.
// Everything is stored as []byte internally —
// the Type field tells us how to interpret those bytes.
type Value struct {
	Type ValueType
	Data []byte
}

// Store is the core data structure —
// a map from string keys to typed values.
type Store struct {
	entries map[string]Value
}

// NewStore initialises the map.
// Always use this — a zero-value Store has a nil map and will panic on Set.
func NewStore() *Store {
	return &Store{
		entries: make(map[string]Value),
	}
}

// ── Core methods ────────────────────────────────────────────────

func (s *Store) Set(key string, val Value) {
	s.entries[key] = val
}

func (s *Store) Get(key string) (Value, bool) {
	val, ok := s.entries[key]
	return val, ok
}

func (s *Store) Delete(key string) {
	delete(s.entries, key)
}

func (s *Store) Len() int {
	return len(s.entries)
}

// Entries exposes the map for the serializer to iterate.
// Returns a copy so the serializer can't mutate internal state.
func (s *Store) Entries() map[string]Value {
	out := make(map[string]Value, len(s.entries))
	for k, v := range s.entries {
		out[k] = v
	}
	return out
}

// ── Helper constructors ─────────────────────────────────────────
// These let you write TextValue("hello") instead of
// manually building Value{Type: TypeText, Data: []byte("hello")} every time.

func TextValue(s string) Value {
	return Value{Type: TypeText, Data: []byte(s)}
}

func IntValue(n int64) Value {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, uint64(n))
	return Value{Type: TypeInt, Data: buf}
}

func FloatValue(f float64) Value {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, math.Float64bits(f))
	return Value{Type: TypeFloat, Data: buf}
}

func BinaryValue(b []byte) Value {
	cp := make([]byte, len(b))
	copy(cp, b)
	return Value{Type: TypeBinary, Data: cp}
}

// ── Decode helpers ──────────────────────────────────────────────
// Mirror of the constructors — used in tests and main to read values back.

func AsText(v Value) string  { return string(v.Data) }
func AsInt(v Value) int64   { return int64(binary.LittleEndian.Uint64(v.Data)) }
func AsFloat(v Value) float64 { return math.Float64frombits(binary.LittleEndian.Uint64(v.Data)) }
func AsBinary(v Value) []byte { return v.Data }