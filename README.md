# Task 2 — Every Byte Costs Money

A typed key-value store with a custom binary serialization format,
built to show that storage costs are a design decision, not an afterthought.

## What this is

A store that holds text, integers, floats, and binary values under
string keys. On first run it serializes everything to disk in a compact
binary format I designed. On restart it reloads from disk — fast, exact,
and without re-processing anything.

The interesting part isn't the store. It's the format — every field
width, every byte decision, every tradeoff between speed, size, and
durability is intentional and measured.

## Project structure
main.go              entry point — first run vs reload detection
store.go             typed in-memory key-value store
serializer.go        binary format — write and read
report.go            timing, file size, and memory measurement
store_test.go        correctness tests
benchmark_test.go    benchmarks vs JSON
docs/
  store.md           how the data structure works
  serializer.md      the binary format explained byte by byte
  report.md          how timing and memory are measured

## Quick start

```bash
git clone <repo>
cd task2

# First run — builds 10k entries, serializes, reports size
go run .

# Reload — loads from disk, reports timing and memory
go run .

# Reset
rm store.bin && go run .
```

---

## Testing

You'll need Go installed (I used Go 1.21, anything recent works).
No external dependencies — just clone and run.

---

### Run everything

Runs all 10 tests and prints each by name. Good first command.

```bash
go test -v ./...
```

---

### Run with race detector

Go's built-in race detector watches for unsafe memory access across
goroutines. All tests pass clean.

```bash
go test -v -race ./...
```

---

### Store tests

**TestStore_SetGet**
Sets all 4 value types — text, int, float, binary — and reads them
back. Checks both the type tag and the actual value survived intact.

```bash
go test -v -run TestStore_SetGet ./...
```

**TestStore_Delete**
Sets a key then deletes it. Verifies it's gone and the store
length dropped by one.

```bash
go test -v -run TestStore_Delete ./...
```

**TestStore_Overwrite**
Sets the same key twice. Verifies the second value wins, not the first.

```bash
go test -v -run TestStore_Overwrite ./...
```

**TestStore_MissingKey**
Calls Get on a key that was never set. Verifies it returns false
instead of panicking or returning garbage.

```bash
go test -v -run TestStore_MissingKey ./...
```

Run all store tests together:

```bash
go test -v -run TestStore ./...
```

---

### Serialization tests

**TestRoundTrip**
The core correctness test. Builds a store with all 4 value types,
serializes to disk, sets the original to nil (nothing left in memory),
deserializes, and checks every key and value came back exactly right.

```bash
go test -v -run TestRoundTrip$ ./...
```

**TestRoundTrip_LargeStore**
Same idea but 10,000 mixed-type entries. Spot-checks entries at the
start, middle, and end — makes sure the format holds at scale,
not just for 4 entries.

```bash
go test -v -run TestRoundTrip_LargeStore ./...
```

**TestRoundTrip_EmptyStore**
Serializes a store with zero entries and reloads it. An empty
file with just the header should load cleanly without errors.

```bash
go test -v -run TestRoundTrip_EmptyStore ./...
```

Run all round trip tests together:

```bash
go test -v -run TestRoundTrip ./...
```

---

### Format safety tests

**TestMagicByteDetection**
Writes random garbage bytes to a file and tries to load it.
The first 4 bytes won't match "TSK2" so it should return a
clear error immediately — not crash, not load wrong data silently.

```bash
go test -v -run TestMagicByteDetection ./...
```

**TestVersionMismatch**
Writes valid magic bytes but sets version to 99. The deserializer
should reject it cleanly. Protects against loading files written
by a different version of the format.

```bash
go test -v -run TestVersionMismatch ./...
```

**TestUnicodeKeys**
Sets keys with Japanese characters and emoji, serializes, reloads,
and verifies they came back correctly. Keys are raw UTF-8 so this
should work — but search systems deal with multilingual data and
it's worth testing explicitly.

```bash
go test -v -run TestUnicodeKeys ./...
```

---

### Benchmarks

`-run=^$` skips regular tests and runs only benchmarks.
`-benchmem` adds allocation numbers alongside speed.

**BenchmarkSerialize_Custom vs BenchmarkSerialize_JSON**
Serializes 10,000 entries to disk repeatedly until Go has a
stable measurement. Compares write speed and allocations per op.

```bash
go test -bench=BenchmarkSerialize -benchmem -run=^$ ./...
```

**BenchmarkDeserialize_Custom vs BenchmarkDeserialize_JSON**
Reads 10,000 entries back from disk repeatedly. This is the
number that matters most — reload time on restart is a real
infrastructure cost. Custom wins by ~5.5x here.

```bash
go test -bench=BenchmarkDeserialize -benchmem -run=^$ ./...
```

**TestSizeComparison**
Writes the same 10,000 entries in both formats and reports
file sizes side by side. Not a speed test — just a clear
answer to how much smaller the custom format is on disk.

```bash
go test -v -run TestSizeComparison ./...
```

---

### What to expect

All 10 tests pass. Race detector comes back clean.
Numbers vary by machine but the relative gap stays consistent.

Results on my machine (Apple M4):

| Metric | Custom | JSON |
|---|---|---|
| File size | 562 KB | 960 KB |
| Serialize | ~5.5ms/op | ~2.5ms/op* |
| Deserialize | ~1ms/op | ~5.5ms/op |
| RAM on reload | 1.63 MB | 5.06 MB |
| Serialize allocs | 10,038/op | 20,004/op |
| Deserialize allocs | 55,043/op | 40,107/op |

*Custom serialize includes `f.Sync()` — forces data to physical
disk before returning. JSON benchmark skips this. Durability is
a design choice, not a format property. With fsync on both,
serialize speed is comparable.

The deserialize alloc count looks higher for custom but each
alloc is precisely sized to the value. JSON allocates fewer
times but wastes more memory per alloc — hence 5MB heap vs 1.63MB.

---

## Tradeoffs

This is the part I spent the most time thinking about.
Correctness is the baseline. The real question is what you
give up to get it — and whether those are the right tradeoffs
for the problem.

### Space efficiency

The format stores each entry as:
- 2 bytes for key length (uint16, max 65535 — saves 2B vs uint32 per entry)
- raw key bytes (no padding, no alignment)
- 1 byte type tag (0=text, 1=int, 2=float, 3=binary)
- 8 bytes fixed for int and float (no varint — fast decode, no branches)
- 4 byte length prefix + raw bytes for text and binary

This comes out to ~57.6 bytes per entry on disk vs ~180 bytes
for JSON — a 3.1x reduction. The gap comes from eliminating
field name strings, quotes, colons, and ASCII-encoding numbers.

The data structure underneath is Go's built-in map. It's not
custom — it's a language primitive, not an imported library.
The tradeoff is that Go's map carries ~50 bytes of runtime
metadata per entry (hash buckets, overflow pointers). This
is why RAM at 171 bytes/entry is 3x the disk size of 57.6 bytes.
At 10M entries this becomes ~1.7GB of overhead you can't avoid
with a built-in map. A flat open-addressing hash table with
a known load factor would close that gap significantly —
that's the obvious next step.

### Speed

Three decisions that made the serializer fast:

**Scratch buffer** — a single 8-byte slice reused for every
fixed-width field write. The original version used `binary.Write`
which goes through reflection and allocates internally. Switching
to `binary.LittleEndian.PutUint*` into a reused buffer dropped
serialize allocations from 35,040 to 10,038 per operation.

**bufio.Writer / bufio.Reader** — without buffering, every field
write is a separate syscall. With bufio, thousands of small writes
get batched into one. Same on the read side with bufio.Reader.

**io.ReadFull over Read** — Go's `Read` is not guaranteed to
return all the bytes you asked for in one call. On large files
the buffer boundary can fall mid-value, and plain `Read` returns
fewer bytes silently. This caused a real bug in testing — entry 73
read a stray byte as a type tag and everything after it was wrong.
`io.ReadFull` loops internally until it has exactly what you asked
for or returns an error. One function call, no silent misalignment.

**Pre-allocated map** — `make(map[string]Value, entryCount)` on
reload tells Go exactly how many entries are coming. Without it
the map resizes ~7 times as it fills up, each time allocating a
new backing array and copying everything over.

### Durability awareness

`f.Sync()` is called after every serialize before the function
returns. This forces the OS to flush its write buffer to physical
disk. Without it, "written" just means "handed to the OS" —
a power loss in that window means the file is gone or corrupt.

The cost is real — fsync takes 1-10ms depending on the disk.
That's why custom serialize looks slower than JSON in the benchmark.
It's not slower encoding — it's paying the durability tax that
JSON skips.

Magic bytes and a version field are the other durability decisions.
4 bytes of overhead per file to guarantee you never silently load
a wrong or corrupted file. If the first 4 bytes aren't "TSK2",
the deserializer errors immediately. If the version doesn't match,
same thing. Silent failures that propagate through a system are
far more expensive than a loud error on startup.

### What I'd change at scale

At 1M entries the current design starts showing its limits.
The map metadata alone would be ~50MB. Loading the entire store
into memory before serving any queries means a slow cold start.
A memory-mapped file with a sorted index block at the end of the
file would let you seek directly to any key without loading
everything — that's how Lucene's `.fdx` / `.fdt` split works.
Compression per value type (LZ4 for text, delta encoding for
sorted integers) would cut disk size by another 3-5x.
Those are the right next steps, not fixes to what's here.

---

## Concept deep dives

For detailed explanations of the smaller concepts used in each file:

- [store.md](docs/store.md) — typed values, why raw bytes internally,
  the map tradeoff
- [serializer.md](docs/serializer.md) — the wire format byte by byte,
  scratch buffer, io.ReadFull, fsync
- [report.md](docs/report.md) — runtime.MemStats, os.Stat,
  cold vs warm reload, what heap delta actually measures