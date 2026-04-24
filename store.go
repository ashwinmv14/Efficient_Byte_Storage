package main

import (
	"encoding/binary"
	"math"
)

type ValueType uint8

const (
	TypeText   ValueType = 0
	TypeInt    ValueType = 1
	TypeFloat  ValueType = 2
	TypeBinary ValueType = 3
)

type Value struct {
	Type ValueType
	Data []byte
}

type Store struct {
	entries map[string]Value
}

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

func (s *Store) Entries() map[string]Value {
	out := make(map[string]Value, len(s.entries))
	for k, v := range s.entries {
		out[k] = v
	}
	return out
}

// ── Helper constructors ─────────────────────────────────────────

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

func AsText(v Value) string  { return string(v.Data) }
func AsInt(v Value) int64   { return int64(binary.LittleEndian.Uint64(v.Data)) }
func AsFloat(v Value) float64 { return math.Float64frombits(binary.LittleEndian.Uint64(v.Data)) }
func AsBinary(v Value) []byte { return v.Data }