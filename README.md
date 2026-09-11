# FrostDB

FrostDB is an embeddable, thread-safe, pure-Go key-value store with write-ahead persistence and zero external dependencies.

It is designed to be imported directly into Go applications that need durable, fast local storage without running external services like Redis or dealing with CGo toolchains like SQLite and RocksDB.

## Features

- **Pure Go**: Zero CGo, cross-compiles cleanly to any target architecture (`GOOS`/`GOARCH`).
- **Embeddable**: Run it directly in-process via `import "github.com/Lunaticfrost/frostdb"`.
- **Durable**: Append-only write-ahead log (WAL) with binary framing and CRC32 checksums for crash safety.
- **Thread-safe**: Concurrency control via reader-writer locks (`sync.RWMutex`).
- **Interactive CLI**: Standalone REPL binary for testing and manual inspection.
- **Zero Dependencies**: Standard library only.

## Quick Start

### As an Embedded Library

```go
package main

import (
	"fmt"
	"log"

	"github.com/Lunaticfrost/frostdb/internal/engine"
)

func main() {
	db := engine.NewStore()

	// Write
	if err := db.Set("session:user_123", "active"); err != nil {
		log.Fatal(err)
	}

	// Read
	val, exists := db.Get("session:user_123")
	if exists {
		fmt.Printf("Session status: %s\n", val)
	}

	// Check & Delete
	if db.Exists("session:user_123") {
		db.Delete("session:user_123")
	}
}
```

### Using the CLI

```bash
# Build binary
go build -o frostdb cmd/frostdb/main.go

# Start interactive shell
./frostdb
```

```text
frostdb> SET user:1 Alice
OK
frostdb> GET user:1
Alice
frostdb> EXISTS user:1
true
frostdb> KEYS
1) user:1
frostdb> DELETE user:1
OK
frostdb> EXIT
Goodbye! ❄️
```

## CLI Commands

| Command | Usage | Description |
| :--- | :--- | :--- |
| `SET` | `SET <key> <value>` | Insert or overwrite a key-value pair |
| `GET` | `GET <key>` | Retrieve value by key |
| `DELETE` / `DEL` | `DELETE <key>` | Remove key and value |
| `EXISTS` | `EXISTS <key>` | Check if a key exists |
| `KEYS` | `KEYS` | List all stored keys |
| `SIZE` | `SIZE` | Return total count of keys |
| `CLEAR` | `CLEAR` | Remove all stored entries |
| `HELP` | `HELP` | Show command reference |
| `EXIT` / `QUIT` | `EXIT` | Close session |

## Architecture & Storage Model

FrostDB combines an in-memory index with an append-only persistence layer:

```text
  Client / API
       │
       ▼
 ┌───────────┐      append       ┌───────────────────────────────┐
 │   Store   ├──────────────────►│ Write-Ahead Log (WAL)         │
 │ (In-RAM)  │                   │ [CRC][Timestamp][Op][KLen]... │
 └─────┬─────┘                   └───────────────┬───────────────┘
       │                                         │
 reads │                                  replay │ on boot
       ▼                                         ▼
   Immediate                                Recovered State
```

1. **Writes (`SET` / `DELETE`)**: Appended sequentially to the log on disk with a CRC32 checksum and synced according to the configured durability policy. The in-memory map updates immediately.
2. **Reads (`GET`)**: Served from memory in $O(1)$ time without disk seeks.
3. **Crash Recovery**: On boot, the log file is read sequentially. Records with valid checksums are replayed into memory. Incomplete or torn writes from sudden power cuts are detected and discarded.

## Development

### Run Tests & Race Detector

```bash
go test -v -race ./...
```

### Run Benchmarks

```bash
go test -bench=. -benchmem ./internal/engine
```

### Format Code

```bash
go fmt ./...
```

## Roadmap

- [x] **Phase 1: In-memory KV Store** — Concurrent read/write primitives, test suite, and REPL CLI.
- [ ] **Phase 2: Persistence Engine (In Progress)** — WAL with binary record framing, CRC32 verification, configurable `fsync` policies, and automatic crash recovery.
- [ ] **Phase 3: Compaction & KeyDir** — Log segment rotation, active file splitting, and background tombstone compaction.
- [ ] **Phase 4: Public API & Byte Slices** — Expose top-level `pkg/frostdb` supporting `[]byte` values, batch operations, and file locking.
- [ ] **Phase 5: Client/Server Mode** — Optional standalone server speaking the Redis (RESP) protocol.

## License

MIT