# store.go

## Types

**ValueType** — uint8 enum: TypeText(0), TypeInt(1), TypeFloat(2),
TypeBinary(3). 1 byte on disk per entry.

**Value** — Type tag + raw bytes. All 4 types stored as []byte
internally. Type tag determines how Data is encoded and decoded.

**Store** — wraps map[string]Value. O(1) insert and lookup.

## Functions

**NewStore()** — initialises the internal map. Store{} has a nil
map and panics on first Set.

**Get(key)** — returns (Value, bool). bool is false if key missing.

**Entries()** — returns a copy of the internal map, not a reference.
Prevents external mutation of the store during serialization.

**BinaryValue(b []byte)** — makes a defensive copy of the input.
Caller modifying the original slice after this call has no effect
on the stored value.

## Decoders

AsText, AsFloat, AsInt, AsBinary reverse what the constructors wrote.
AsFloat uses math.Float64frombits — reconstructs the exact IEEE 754
bit pattern, no precision loss.

## Tradeoff

Go's map carries ~94 bytes of runtime overhead per entry (hash
buckets, tophash array, overflow pointer). At 10k entries that's
~900KB of metadata on top of actual data. This shows up in
HeapDeltaBytes after reload, not in file size.