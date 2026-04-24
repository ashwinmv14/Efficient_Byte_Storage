package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// Magic bytes — first 4 bytes of every file being writen.
var magic = [4]byte{'T', 'S', 'K', '2'}

const formatVersion uint8 = 1

func Serialize(s *Store, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	w := bufio.NewWriter(f)

	scratch := make([]byte, 8)

	// ── HEADER ──────────────────────────────────────────────────

	if _, err := w.Write(magic[:]); err != nil {
		return err
	}

	scratch[0] = formatVersion
	if _, err := w.Write(scratch[:1]); err != nil {
		return err
	}

	binary.LittleEndian.PutUint32(scratch[:4], uint32(s.Len()))
	if _, err := w.Write(scratch[:4]); err != nil {
		return err
	}

	// ── ENTRIES ─────────────────────────────────────────────────

	for key, val := range s.Entries() {

		binary.LittleEndian.PutUint16(scratch[:2], uint16(len(key)))
		if _, err := w.Write(scratch[:2]); err != nil {
			return err
		}

		if _, err := w.Write([]byte(key)); err != nil {
			return err
		}

		scratch[0] = byte(val.Type)
		if _, err := w.Write(scratch[:1]); err != nil {
			return err
		}

		if err := writeValue(w, val, scratch); err != nil {
			return err
		}
	}

	if err := w.Flush(); err != nil {
		return err
	}

	return f.Sync()
}

func writeValue(w *bufio.Writer, val Value, scratch []byte) error {
	switch val.Type {

	case TypeInt, TypeFloat:
		_, err := w.Write(val.Data)
		return err

	case TypeText, TypeBinary:
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

func Deserialize(path string) (*Store, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	r := bufio.NewReader(f)

	// ── HEADER ──────────────────────────────────────────────────

	var gotMagic [4]byte
	if _, err := io.ReadFull(r, gotMagic[:]); err != nil {
		return nil, fmt.Errorf("read magic: %w", err)
	}
	if gotMagic != magic {
		return nil, fmt.Errorf("invalid file: wrong magic bytes, got %v", gotMagic)
	}

	var version uint8
	if err := binary.Read(r, binary.LittleEndian, &version); err != nil {
		return nil, fmt.Errorf("read version: %w", err)
	}
	if version != formatVersion {
		return nil, fmt.Errorf("unsupported format version: %d", version)
	}

	var entryCount uint32
	if err := binary.Read(r, binary.LittleEndian, &entryCount); err != nil {
		return nil, fmt.Errorf("read entry count: %w", err)
	}

	// ── ENTRIES ─────────────────────────────────────────────────

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
