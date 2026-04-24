package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// Magic bytes — first 4 bytes of every file we write.
// "TSK2" in ASCII. Detects wrong or garbage files on reload.
var magic = [4]byte{'T', 'S', 'K', '2'}

// formatVersion lets us change the format later
// without silently misreading old files.
const formatVersion uint8 = 1

// Serialize writes the store to disk in this layout:
//
//	HEADER
//	  [magic:         4B ]  "TSK2"
//	  [version:       1B ]  format version
//	  [entry_count:   4B ]  number of key/value pairs
//
//	PER ENTRY  (repeated entry_count times)
//	  [key_len:       2B ]  uint16 — max 65535, saves 2B vs uint32
//	  [key:     key_len B]  raw UTF-8
//	  [type_tag:      1B ]  0=text 1=int 2=float 3=binary
//	  value encoding:
//	    text   → [len: 4B][utf-8 bytes]
//	    int    → [8B little-endian int64]
//	    float  → [8B little-endian float64 bits]
//	    binary → [len: 4B][raw bytes]
func Serialize(s *Store, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	w := bufio.NewWriter(f)

	// scratch is a single 8-byte buffer reused for every
	// fixed-width field write (uint16, uint32, uint64).
	// This eliminates the reflection + alloc cost of binary.Write
	// which was making custom serialize slower than JSON.
	scratch := make([]byte, 8)

	// ── HEADER ──────────────────────────────────────────────────

	// Magic bytes — 4B, written as-is
	if _, err := w.Write(magic[:]); err != nil {
		return err
	}

	// Version — 1B
	scratch[0] = formatVersion
	if _, err := w.Write(scratch[:1]); err != nil {
		return err
	}

	// Entry count — 4B uint32
	binary.LittleEndian.PutUint32(scratch[:4], uint32(s.Len()))
	if _, err := w.Write(scratch[:4]); err != nil {
		return err
	}

	// ── ENTRIES ─────────────────────────────────────────────────

	for key, val := range s.Entries() {

		// Key length — 2B uint16
		// uint16 max = 65535, enough for any real key, saves 2B vs uint32
		binary.LittleEndian.PutUint16(scratch[:2], uint16(len(key)))
		if _, err := w.Write(scratch[:2]); err != nil {
			return err
		}

		// Key bytes — raw UTF-8
		if _, err := w.Write([]byte(key)); err != nil {
			return err
		}

		// Type tag — 1B
		scratch[0] = byte(val.Type)
		if _, err := w.Write(scratch[:1]); err != nil {
			return err
		}

		// Value — encoding depends on type
		if err := writeValue(w, val, scratch); err != nil {
			return err
		}
	}

	// Flush Go buffer → OS buffer
	if err := w.Flush(); err != nil {
		return err
	}

	// Sync OS buffer → physical disk.
	// Without this, a power loss after Serialize() returns
	// could still lose the file. Cost: ~1-10ms. Worth it.
	return f.Sync()
}

// writeValue encodes a single value into w.
// scratch is passed in — same 8-byte buffer reused across all entries.
// Zero new allocations per call.
func writeValue(w *bufio.Writer, val Value, scratch []byte) error {
	switch val.Type {

	case TypeInt, TypeFloat:
		// Already exactly 8 bytes from IntValue/FloatValue constructors.
		// Direct write — no length prefix needed, no alloc.
		_, err := w.Write(val.Data)
		return err

	case TypeText, TypeBinary:
		// Write 4B length prefix using scratch — no alloc.
		binary.LittleEndian.PutUint32(scratch[:4], uint32(len(val.Data)))
		if _, err := w.Write(scratch[:4]); err != nil {
			return err
		}
		_, err := w.Write(val.Data)
		return err

	default:
		return fmt.Errorf("unknown value type: %d", val.Type)
	}
}

// ── READ SIDE ───────────────────────────────────────────────────

// Deserialize reads the file at path and reconstructs
// the store exactly as it was when Serialize was called.
// Read order mirrors write order — top to bottom, zero seeking.
func Deserialize(path string) (*Store, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	r := bufio.NewReader(f)

	// ── HEADER ──────────────────────────────────────────────────

	// Magic check — first thing, before reading anything else.
	// Wrong magic = wrong file = immediate error, no silent bad load.
	var gotMagic [4]byte
	if _, err := io.ReadFull(r, gotMagic[:]); err != nil {
		return nil, fmt.Errorf("read magic: %w", err)
	}
	if gotMagic != magic {
		return nil, fmt.Errorf("invalid file: wrong magic bytes, got %v", gotMagic)
	}

	// Version check
	var version uint8
	if err := binary.Read(r, binary.LittleEndian, &version); err != nil {
		return nil, fmt.Errorf("read version: %w", err)
	}
	if version != formatVersion {
		return nil, fmt.Errorf("unsupported format version: %d", version)
	}

	// Entry count
	var entryCount uint32
	if err := binary.Read(r, binary.LittleEndian, &entryCount); err != nil {
		return nil, fmt.Errorf("read entry count: %w", err)
	}

	// ── ENTRIES ─────────────────────────────────────────────────

	// Pre-allocate to exact size — eliminates all map resize allocs.
	s := &Store{entries: make(map[string]Value, entryCount)}

	for i := uint32(0); i < entryCount; i++ {

		// Key length — 2B
		var keyLen uint16
		if err := binary.Read(r, binary.LittleEndian, &keyLen); err != nil {
			return nil, fmt.Errorf("entry %d: read key len: %w", i, err)
		}

		// Key bytes
		keyBuf := make([]byte, keyLen)
		if _, err := io.ReadFull(r, keyBuf); err != nil {
			return nil, fmt.Errorf("entry %d: read key: %w", i, err)
		}
		key := string(keyBuf)

		// Type tag — 1B
		var typeTag ValueType
		if err := binary.Read(r, binary.LittleEndian, &typeTag); err != nil {
			return nil, fmt.Errorf("entry %d: read type: %w", i, err)
		}

		// Value
		val, err := readValue(r, typeTag)
		if err != nil {
			return nil, fmt.Errorf("entry %d: read value: %w", i, err)
		}

		s.entries[key] = val
	}

	return s, nil
}

// readValue is the exact mirror of writeValue.
// io.ReadFull guarantees we read exactly the bytes we ask for —
// plain Read() can return fewer bytes than requested, causing
// silent misalignment on large files.
func readValue(r *bufio.Reader, t ValueType) (Value, error) {
	switch t {

	case TypeInt, TypeFloat:
		buf := make([]byte, 8)
		if _, err := io.ReadFull(r, buf); err != nil {
			return Value{}, err
		}
		return Value{Type: t, Data: buf}, nil

	case TypeText, TypeBinary:
		var dataLen uint32
		if err := binary.Read(r, binary.LittleEndian, &dataLen); err != nil {
			return Value{}, err
		}
		buf := make([]byte, dataLen)
		if _, err := io.ReadFull(r, buf); err != nil {
			return Value{}, err
		}
		return Value{Type: t, Data: buf}, nil

	default:
		return Value{}, fmt.Errorf("unknown type tag: %d", t)
	}
}
