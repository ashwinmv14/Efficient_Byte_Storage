# report.go

## Report struct

```go
type Report struct {
    EntryCount     int
    SerializeTime  time.Duration
    FileSizeBytes  int64
    ReloadTime     time.Duration
    HeapDeltaBytes uint64
}
```

First run populates SerializeTime and FileSizeBytes.
Reload populates ReloadTime and HeapDeltaBytes.

## MeasureSerialize

time.Now / time.Since wraps the Serialize call.
os.Stat gets file size from filesystem metadata — does not read
file contents.

## MeasureDeserialize

runtime.GC() runs before measurement. Without it, heap delta
includes uncollected garbage from the serialize step and overstates
the store's actual memory footprint.

Captures runtime.MemStats before and after Deserialize.
HeapDeltaBytes = after.HeapAlloc - before.HeapAlloc.

## Cold vs warm reload

First reload  : 2.54ms   OS reads from disk
Second reload : 1.25ms   OS serves from page cache

First restart pays the cold cost. Subsequent restarts are warm.

## formatBytes

Converts raw byte count to human readable string.
576121 → "562.62 KB (576121 bytes)"