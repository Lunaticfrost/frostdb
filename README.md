# FrostDB ❄️

[![Go Reference](https://pkg.go.dev/badge/github.com/Lunaticfrost/frostdb.svg)](https://pkg.go.dev/github.com/Lunaticfrost/frostdb)
[![CI](https://github.com/Lunaticfrost/frostdb/actions/workflows/ci.yml/badge.svg)](https://github.com/Lunaticfrost/frostdb/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/Lunaticfrost/frostdb?color=blue)](https://github.com/Lunaticfrost/frostdb/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/Lunaticfrost/frostdb)](https://goreportcard.com/report/github.com/Lunaticfrost/frostdb)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

FrostDB is an embeddable, thread-safe, pure-Go key-value store with write-ahead persistence and zero external dependencies.

It is designed to be imported directly into Go applications that need durable, fast local storage without running external services like Redis or dealing with CGo toolchains like SQLite and RocksDB.

## Features

- **Pure Go**: Zero CGo, cross-compiles cleanly to any target architecture (`GOOS`/`GOARCH`).
- **Embeddable**: Run it directly in-process via `import "github.com/Lunaticfrost/frostdb"`.
- **Durable**: Append-only write-ahead log (WAL) with binary framing and CRC32 checksums for crash safety.
- **Thread-safe**: Concurrency control via reader-writer locks (`sync.RWMutex`).
- **Interactive CLI & Redis Server**: Standalone REPL binary and streaming Redis RESP2 TCP server.
- **Zero Dependencies**: Standard library only.

📖 *Curious about the engine design, binary framing, or zero-downtime compaction? Read the [FrostDB Internals Guide](docs/INTERNALS.md).*

## Quick Start

### As an Embedded Library

```go
package main

import (
	"fmt"
	"log"

	"github.com/Lunaticfrost/frostdb"
)

func main() {
	// Open or create a persistent database with 1-second background fsync
	db, err := frostdb.Open("./frostdata", frostdb.DefaultOptions)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Binary-safe write
	if err := db.Set("session:user_123", []byte("active")); err != nil {
		log.Fatal(err)
	}

	// Binary-safe read
	val, exists := db.Get("session:user_123")
	if exists {
		fmt.Printf("Session status: %s\n", string(val))
	}

	// Atomic multi-operation batch
	batch := frostdb.NewWriteBatch()
	batch.Set("k1", []byte("v1"))
	batch.Set("k2", []byte("v2"))
	batch.Delete("session:user_123")
	if err := db.Write(batch); err != nil {
		log.Fatal(err)
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
| `INFO` | `INFO` | Display storage mode, data directory, and key count |
| `SYNC` | `SYNC` | Force fsync all pending writes to disk |
| `COMPACT` | `COMPACT` | Reclaim disk space by purging overwritten and deleted records |
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
4. **Log Compaction**: Running `COMPACT` takes a snapshot of active keys, writes them to a temporary WAL, and performs an atomic POSIX rename (`os.Rename`), purging all stale historical updates and tombstones without downtime.

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

### As a Redis-Compatible Server

FrostDB can run as a standalone TCP network daemon speaking standard Redis (RESP2):

```bash
# Launch server on port 7379 with disk persistence
./frostdb -l :7379 -d ./frostdata
```

Connect using standard Redis tooling and SDKs:

```bash
# Using redis-cli
redis-cli -p 7379 PING
# Output: PONG

redis-cli -p 7379 SET user:101 "Alice"
# Output: OK

redis-cli -p 7379 GET user:101
# Output: "Alice"
```

## Roadmap

- [x] **Phase 1: In-memory KV Store** — Concurrent read/write primitives, test suite, and REPL CLI.
- [x] **Phase 2: Persistence Engine** — WAL with binary record framing, CRC32 verification, configurable `fsync` policies, and automatic crash recovery.
- [x] **Phase 3: Compaction & Space Reclamation** — Atomic log compaction, dead space recovery, and `COMPACT` CLI command.
- [x] **Phase 4: Public API & Byte Slices** — Root package API (`import "github.com/Lunaticfrost/frostdb"`), binary-safe `[]byte` values, atomic `WriteBatch`, and process file locking (`flock`).
- [x] **Phase 5: Client/Server Mode** — Standalone TCP server with streaming Redis (RESP2) protocol compatibility.

## License

MIT