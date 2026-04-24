# Every Byte Costs Money

A typed key-value store with a custom binary serialization format,
built to show that storage costs are a design decision, not an afterthought.

## What this is

A store that holds text, integers, floats, and binary values under
string keys. On first run it serializes everything to disk in a compact
binary format I designed. On restart it reloads from disk > fast, exact,
and without re-processing anything.


## Project structure
main.go -              entry point — first run vs reload detection

store.go -       typed in-memory key-value store

serialize.go -       binary format — write and read

report.go -           timing, file size, and memory measurement

store_test.go -       correctness tests

benchmark_test.go -   benchmarks vs JSON

docs/
  store.md -        how the data structure works
  
  serialize.md -    the binary format explained byte by byte
  
  report.md -        how timing and memory are measured

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
No external dependencies, just clone and run.

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
Sets all 4 value types - text, int, float, binary and reads them
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
start, middle, and end, makes sure the format holds at scale,
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
should work, but search systems deal with multilingual data and
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
number that matters most > reload time on restart is a real
infrastructure cost. Custom wins by ~5.5x here.

```bash
go test -bench=BenchmarkDeserialize -benchmem -run=^$ ./...
```

**TestSizeComparison**
Writes the same 10,000 entries in both formats and reports
file sizes side by side. Not a speed test, just a clear
answer to how much smaller the custom format is on disk.

```bash
go test -v -run TestSizeComparison ./...
```

**TestReloadLatency**
Deserializes the same file 5 times in a row and prints each
duration. Shows the OS page cache effect — first load hits disk,
subsequent loads are served from memory and are noticeably faster.

```bash
go test -v -run TestReloadLatency ./...
```

---

### What to expect

All 10 tests pass. Race detector comes back clean.
Numbers vary by machine but the relative gap stays consistent.

Results on my machine (Apple M4):

| Metric | Custom | JSON |
|---|---|---|
| File size | 562 KB | 960 KB |
| Serialize | ~6.1ms/op | ~2.4ms/op* |
| Deserialize | ~1ms/op | ~5.7ms/op |
| RAM on reload | 1.71 MB | 5.06 MB |
| Serialize allocs | 10,038/op | 20,004/op |
| Deserialize allocs | 55,043/op | 40,107/op |

*Custom serialize includes `f.Sync()` — forces data to physical
disk before returning. JSON benchmark skips this. With fsync on both,
serialize speed is comparable.

The deserialize alloc count looks higher for custom but each
alloc is precisely sized to the value. JSON allocates fewer
times but wastes more memory per alloc — hence 5MB heap vs 1.71MB.

---

## Cold vs Warm Reload

Running `go run .` multiple times shows the OS page cache effect.
First reload hits disk. Subsequent reloads are served from memory.

```bash
rm store.bin && go run .    # first run — serialize
go run .                    # cold reload
go run .                    # warm reload
```

Results from my machine (Apple M4):

| Run | Time | What happened |
|---|---|---|
| Serialize | 8.5ms | built 10k entries, wrote to disk, fsynced |
| Cold reload | 7.73ms | file read from physical disk |
| Warm reload | 1.26ms | OS already had file in page cache |
| Warm reload | 1.21ms | stable |

Cold to warm is ~6x faster — same file, same data, same code.
The difference is entirely the OS page cache. No disk I/O on
warm runs.

This matters in production. First restart after a deploy pays
the cold cost. Every restart after that pays the warm cost.
A search index that reloads in 1.2ms is effectively instant.

## Tradeoffs

### Space efficiency

Every field width is a deliberate choice, not a default.

- Key length is 2 bytes (uint16) not 4 — max 65535 chars is
  enough for any real key, saves 2 bytes per entry
- Integers and floats are always 8 bytes fixed — no varint,
  predictable decode speed, no branching
- Type tag is 1 byte — 4 types fit in 4 values, no string labels

Result: 57.6 bytes/entry on disk vs 98.4 bytes/entry for JSON.

The data structure is Go's built-in map — a language primitive,
not an imported library. The cost is ~94 bytes of runtime metadata
per entry for O(1) lookup. At 10k entries that's acceptable.
At 10M entries you'd want a flat array with binary search instead.

### Speed

Three things that made a measurable difference:

- **Scratch buffer** — one 8-byte slice reused for all fixed-width
  writes. Dropped serialize allocs from 35,040 to 10,038 per op
  by removing binary.Write's reflection overhead
- **bufio** — batches thousands of small writes into one syscall.
  Same on the read side. Without it, every field is a separate
  OS call
- **io.ReadFull** — plain Read() doesn't guarantee returning all
  bytes you asked for. Hit this as a real bug in testing — entry 73
  read a stray byte as a type tag and corrupted everything after it.
  io.ReadFull loops until it has exactly what you asked for

### Durability

f.Sync() is called before Serialize returns. This forces the OS
to flush its write buffer to physical disk — not just "handed off
to the OS." Without it, a crash after Serialize returns could
still lose the file.

Cost: 1-10ms per serialize. That's why custom serialize looks
slower than JSON in benchmarks — JSON skips this call entirely.
Durability is a design decision, not a format property.

Magic bytes + version field add 5 bytes per file total.
Cheap insurance against loading a wrong file or an old format
silently. Both return a clear error immediately.

## Where I'd use this in FailureChain

My existing project — FailureChain — is an LLM-based system that
debugs infrastructure failures. It ingests incident logs and metrics,
builds a GraphRAG for structural dependencies between services, and
a RAG for historical failure analogs. Then fuses both to give an
LLM context about what broke and why.

The problem this store solves for that system:

**Incident metadata store** — every incident chunk has mixed metadata.
Timestamp (int), service name (text), severity score (float),
raw log payload (binary). Right now that lives in memory and
disappears on restart. This store would persist it to disk in
a compact format and reload it in under 2ms — no re-processing
on startup.

**Embedding cache** — generating embeddings for 500k log chunks
costs real money in API calls or GPU time. An embedding is just
a []float32 — raw bytes. TypeBinary in this store handles that
exactly. Serialize once, reload fast. Avoid re-embedding on
every restart.

**GraphRAG node store** — each node in the dependency graph has
a name (text), a type like "service" or "database" (text), a
failure count (int), and a risk score (float). That's all 4
value types in one node. This store holds that naturally.

The format I built here is essentially what sits under a RAG
index between restarts. Lucene solves this for text search in Java.
This is a Go-native version of the same idea, scoped to what
FailureChain actually needs.

---

## Concept deep dives

For detailed explanations of the smaller concepts used in each file:

- [store.md](docs/store.md) — typed values, why raw bytes internally,
  the map tradeoff
- [serialize.md](docs/serialize.md) — the wire format byte by byte,
  scratch buffer, io.ReadFull, fsync
- [report.md](docs/report.md) — runtime.MemStats, os.Stat,
  cold vs warm reload, what heap delta actually measures