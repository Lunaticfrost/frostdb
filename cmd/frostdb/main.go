package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Lunaticfrost/frostdb"
)

const banner = `
  ______              _   ____  ____  
 |  ____|            | | |  _ \|  _ \ 
 | |__ _ __ ___  ___ | |_| | | | |_) |
 |  __| '__/ _ \/ __|| __| | | |  _ < 
 | |  | | | (_) \__ \| |_| |_| | |_) |
 |_|  |_|  \___/|___/ \__|____/|____/ 
                                      
FrostDB v0.4.0 - Embeddable Key-Value Database & Redis Server
Type 'HELP' for available commands
`

func main() {
	dataDir := flag.String("data-dir", "./frostdata", "Path to data directory for persistence")
	flag.StringVar(dataDir, "d", "./frostdata", "Path to data directory for persistence (shorthand)")
	inMemory := flag.Bool("in-memory", false, "Run in purely in-memory mode (no persistence)")
	syncMode := flag.String("sync", "periodic", "Sync policy: 'always', 'periodic', or 'none'")
	listenAddr := flag.String("listen", "", "TCP address to listen on for Redis (RESP) clients (e.g. :7379)")
	flag.StringVar(listenAddr, "l", "", "TCP address to listen on (shorthand)")
	flag.Parse()

	fmt.Print(banner)

	var store *frostdb.DB
	var err error

	if *inMemory {
		store = frostdb.New()
		fmt.Println("Storage: In-Memory (data will not be persisted to disk)")
	} else {
		policy := frostdb.SyncPeriodic
		switch strings.ToLower(*syncMode) {
		case "always":
			policy = frostdb.SyncAlways
		case "none":
			policy = frostdb.SyncNone
		default:
			policy = frostdb.SyncPeriodic
		}

		store, err = frostdb.Open(*dataDir, frostdb.Options{SyncPolicy: policy})
		if err != nil {
			if errors.Is(err, frostdb.ErrDatabaseLocked) {
				fmt.Fprintf(os.Stderr, "Error: database at %s is locked by another running process\n", *dataDir)
			} else {
				fmt.Fprintf(os.Stderr, "Failed to initialize storage at %s: %v\n", *dataDir, err)
			}
			os.Exit(1)
		}
		fmt.Printf("Storage: Persistent [Dir: %s, Sync: %s, Keys Recovered: %d]\n", *dataDir, *syncMode, store.Size())
	}
	fmt.Println()

	// Signal handling channel
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// Mode 1: Redis Server Mode
	if *listenAddr != "" {
		srv := frostdb.NewServer(*listenAddr, store)
		if err := srv.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to start server on %s: %v\n", *listenAddr, err)
			_ = store.Close()
			os.Exit(1)
		}
		fmt.Printf("Redis RESP Server listening on %s\n", srv.Addr())
		fmt.Println("Accepting connections from redis-cli, SDKs, and netcat.")
		fmt.Println("Press Ctrl+C to shut down.")

		<-sigCh
		fmt.Println("\nReceived shutdown signal. Stopping server...")
		_ = srv.Stop()
		_ = store.Close()
		fmt.Println("Server stopped gracefully. ❄️")
		return
	}

	// Mode 2: Interactive REPL CLI Mode
	go func() {
		<-sigCh
		fmt.Println("\nReceived shutdown signal. Flushing and exiting...")
		_ = store.Close()
		os.Exit(0)
	}()

	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("frostdb> ")

		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		handleCommand(store, line)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
		_ = store.Close()
		os.Exit(1)
	}

	_ = store.Close()
}

func handleCommand(store *frostdb.DB, line string) {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return
	}

	command := strings.ToUpper(parts[0])

	switch command {
	case "SET":
		handleSet(store, parts)
	case "GET":
		handleGet(store, parts)
	case "DELETE", "DEL":
		handleDelete(store, parts)
	case "EXISTS":
		handleExists(store, parts)
	case "KEYS":
		handleKeys(store)
	case "CLEAR":
		handleClear(store)
	case "SIZE":
		handleSize(store)
	case "INFO":
		handleInfo(store)
	case "SYNC":
		handleSync(store)
	case "COMPACT":
		handleCompact(store)
	case "HELP":
		printHelp()
	case "EXIT", "QUIT":
		_ = store.Close()
		fmt.Println("Goodbye! ❄️")
		os.Exit(0)
	default:
		fmt.Printf("Unknown command: %s. Type 'HELP' for available commands.\n", command)
	}
}

func handleSet(store *frostdb.DB, parts []string) {
	if len(parts) < 3 {
		fmt.Println("Usage: SET key value")
		return
	}

	key := parts[1]
	value := strings.Join(parts[2:], " ")

	err := store.SetString(key, value)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Println("OK")
}

func handleGet(store *frostdb.DB, parts []string) {
	if len(parts) != 2 {
		fmt.Println("Usage: GET key")
		return
	}

	key := parts[1]
	value, exists := store.GetString(key)

	if !exists {
		fmt.Println("(nil)")
		return
	}

	fmt.Println(value)
}

func handleDelete(store *frostdb.DB, parts []string) {
	if len(parts) != 2 {
		fmt.Println("Usage: DELETE key")
		return
	}

	key := parts[1]
	deleted := store.Delete(key)

	if deleted {
		fmt.Println("OK")
	} else {
		fmt.Println("Key not found")
	}
}

func handleExists(store *frostdb.DB, parts []string) {
	if len(parts) != 2 {
		fmt.Println("Usage: EXISTS key")
		return
	}

	key := parts[1]
	exists := store.Exists(key)

	if exists {
		fmt.Println("true")
	} else {
		fmt.Println("false")
	}
}

func handleKeys(store *frostdb.DB) {
	keys := store.Keys()

	if len(keys) == 0 {
		fmt.Println("(empty)")
		return
	}

	for i, key := range keys {
		fmt.Printf("%d) %s\n", i+1, key)
	}
}

func handleClear(store *frostdb.DB) {
	store.Clear()
	fmt.Println("OK - All keys removed")
}

func handleSize(store *frostdb.DB) {
	size := store.Size()
	fmt.Printf("%d key(s)\n", size)
}

func handleInfo(store *frostdb.DB) {
	if store.IsPersistent() {
		fmt.Printf("Mode:       Persistent\n")
		fmt.Printf("Data Dir:   %s\n", store.DataDir())
	} else {
		fmt.Printf("Mode:       In-Memory\n")
	}
	fmt.Printf("Total Keys: %d\n", store.Size())
}

func handleSync(store *frostdb.DB) {
	if !store.IsPersistent() {
		fmt.Println("Store is in-memory only (sync not required)")
		return
	}

	if err := store.Sync(); err != nil {
		fmt.Printf("Sync error: %v\n", err)
		return
	}
	fmt.Println("OK - Synced to disk")
}

func handleCompact(store *frostdb.DB) {
	if !store.IsPersistent() {
		fmt.Println("Error: Compaction is only supported for persistent stores")
		return
	}

	stats, err := store.Compact()
	if err != nil {
		fmt.Printf("Compaction error: %v\n", err)
		return
	}

	fmt.Printf("OK - Compacted %d active key(s) in %s\n", stats.KeysCompacted, stats.Duration.Round(100*time.Microsecond))
	fmt.Printf("Disk space: %s -> %s (reclaimed %s / %.1f%%)\n",
		formatBytes(stats.BeforeBytes),
		formatBytes(stats.AfterBytes),
		formatBytes(stats.ReclaimedBytes),
		stats.ReclaimedPct,
	)
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func printHelp() {
	help := `
Available Commands:
  SET key value      Store a key-value pair
  GET key            Retrieve value by key
  DELETE key         Remove a key-value pair
  EXISTS key         Check if key exists
  KEYS               List all keys
  SIZE               Show number of keys
  INFO               Show engine mode and statistics
  SYNC               Force flush pending writes to disk
  COMPACT            Reclaim disk space by purging deleted/stale records
  CLEAR              Remove all keys
  HELP               Show this help message
  EXIT               Quit FrostDB

Server Mode:
  Start TCP server:  ./frostdb -l :7379
  Connect via CLI:   redis-cli -p 7379
`
	fmt.Println(help)
}
