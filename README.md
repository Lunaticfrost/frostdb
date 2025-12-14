markdown# FrostDB ❄️

A lightweight, educational key-value database built in Go to demonstrate fundamental database concepts.

## Features (Phase 1)
- ✅ In-memory key-value storage
- ✅ Thread-safe operations
- ✅ Interactive CLI
- ✅ Basic CRUD operations

## Installation
```bash
go build -o frostdb cmd/frostdb/main.go
./frostdb
```

## Usage
frostdb> SET name Alice
OK
frostdb> GET name
Alice
frostdb> DELETE name
OK
frostdb> EXISTS name
false
frostdb> EXIT
Goodbye!

## Commands
- `SET key value` - Store a key-value pair
- `GET key` - Retrieve value by key
- `DELETE key` - Remove a key-value pair
- `EXISTS key` - Check if key exists
- `KEYS` - List all keys
- `CLEAR` - Remove all keys
- `EXIT` - Quit the CLI

## Development

### Run Tests
```bash
go test ./...
```

### Run with Race Detector
```bash
go run -race cmd/frostdb/main.go
```

### Benchmarks
```bash
go test -bench=. ./internal/engine
```

## Roadmap
- [x] Phase 1: In-memory key-value store
- [ ] Phase 2: Persistence layer
- [ ] Phase 3: Indexing & compaction
- [ ] Phase 4: WAL & durability
- [ ] Phase 5: Advanced features

## License
MIT
Quick Commands
Build the project
bashgo build -o frostdb cmd/frostdb/main.go
Run the CLI
bash./frostdb
Run tests
bashgo test ./...
Run tests with coverage
bashgo test -cover ./...
Run tests with race detector
bashgo test -race ./...
Format code
bashgo fmt ./...
Run linter (install first: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
bashgolangci-lint run