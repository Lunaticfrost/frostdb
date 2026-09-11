package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/Lunaticfrost/frostdb/internal/engine"
)

const banner = `
  ______              _   ____  ____  
 |  ____|            | | |  _ \|  _ \ 
 | |__ _ __ ___  ___ | |_| | | | |_) |
 |  __| '__/ _ \/ __|| __| | | |  _ < 
 | |  | | | (_) \__ \| |_| |_| | |_) |
 |_|  |_|  \___/|___/ \__|____/|____/ 
                                      
FrostDB v0.1.0 - Educational Key-Value Database
Type 'HELP' for available commands
`

func main() {
	fmt.Print(banner)

	store := engine.NewStore()
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
		os.Exit(1)
	}
}

func handleCommand(store *engine.Store, line string) {
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
	case "HELP":
		printHelp()
	case "EXIT", "QUIT":
		fmt.Println("Goodbye! ❄️")
		os.Exit(0)
	default:
		fmt.Printf("Unknown command: %s. Type 'HELP' for available commands.\n", command)
	}
}

func handleSet(store *engine.Store, parts []string) {
	if len(parts) < 3 {
		fmt.Println("Usage: SET key value")
		return
	}

	key := parts[1]
	// Join remaining parts as value (allows values with spaces)
	value := strings.Join(parts[2:], " ")

	err := store.Set(key, value)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Println("OK")
}

func handleGet(store *engine.Store, parts []string) {
	if len(parts) != 2 {
		fmt.Println("Usage: GET key")
		return
	}

	key := parts[1]
	value, exists := store.Get(key)

	if !exists {
		fmt.Println("(nil)")
		return
	}

	fmt.Println(value)
}

func handleDelete(store *engine.Store, parts []string) {
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

func handleExists(store *engine.Store, parts []string) {
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

func handleKeys(store *engine.Store) {
	keys := store.Keys()

	if len(keys) == 0 {
		fmt.Println("(empty)")
		return
	}

	for i, key := range keys {
		fmt.Printf("%d) %s\n", i+1, key)
	}
}

func handleClear(store *engine.Store) {
	store.Clear()
	fmt.Println("OK - All keys removed")
}

func handleSize(store *engine.Store) {
	size := store.Size()
	fmt.Printf("%d key(s)\n", size)
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
  CLEAR              Remove all keys
  HELP               Show this help message
  EXIT               Quit FrostDB

Examples:
  frostdb> SET name Alice
  OK
  frostdb> GET name
  Alice
  frostdb> SET greeting Hello World
  OK
  frostdb> KEYS
  1) name
  2) greeting
  frostdb> DELETE name
  OK
  frostdb> CLEAR
  OK - All keys removed
`
	fmt.Println(help)
}
