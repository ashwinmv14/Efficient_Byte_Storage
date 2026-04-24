## Wire format

HEADER
magic        4B   54 53 4B 32 ("TSK2")
version      1B   currently 1
entry_count  4B   uint32
PER ENTRY
key_len      2B   uint16, max key size 65535 bytes
key          NB   raw UTF-8
type_tag     1B   0=text 1=int 2=float 3=binary
value
int/float  → 8B fixed, little-endian
text/binary → 4B length + raw bytes

## Serialize(s *Store, path string) error

Writes store to disk in the format above.

Uses bufio.Writer — batches all field writes into one syscall
instead of one syscall per field.

Uses a single 8-byte scratch buffer reused across all entries
for fixed-width fields. Replaces binary.Write (which uses
reflection and allocates per call). Reduced allocations from
35,040 to 10,038 per operation.

Calls w.Flush() then f.Sync() before returning. Flush moves
data from Go's buffer to the OS. Sync forces OS to write to
physical disk. Without Sync, a crash after Serialize returns
can still lose the file.

## Deserialize(path string) (*Store, error)

Reads file, validates magic and version, reconstructs store.

Reads top to bottom in the same order Serialize wrote — no
seeking, sequential access, OS read-ahead works in your favour.

Uses io.ReadFull instead of Read for all byte slice reads.
Read is not guaranteed to return all requested bytes in one
call — buffer boundary can fall mid-value, leaving the reader
offset at the wrong position. This caused a real bug in testing:
error: entry 73: read value: unknown type tag: 148

io.ReadFull loops until it has exactly the bytes requested.

Pre-allocates the map with entry count from header:
make(map[string]Value, entryCount). Eliminates ~7 resize
operations Go would otherwise perform as the map fills.

## Magic bytes

```go
var magic = [4]byte{'T', 'S', 'K', '2'}
```

Checked before anything else on load. Wrong bytes → immediate
error. Prevents silently loading a wrong or corrupted file.

## Version

```go
const formatVersion uint8 = 1
```

Checked after magic. Version mismatch → immediate error.
Prevents misreading files written by a different format version.