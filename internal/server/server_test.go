package server

import (
	"bufio"
	"net"
	"strings"
	"testing"

	"github.com/Lunaticfrost/frostdb/internal/engine"
)

func TestServerE2E(t *testing.T) {
	tempDir := t.TempDir()

	db, err := engine.Open(tempDir, engine.Options{SyncPolicy: engine.SyncAlways})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	srv := NewServer("127.0.0.1:0", db)
	if err := srv.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer srv.Stop()

	conn, err := net.Dial("tcp", srv.Addr())
	if err != nil {
		t.Fatalf("failed to connect to server: %v", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)

	sendAndAssert := func(cmd string, expected string) {
		t.Helper()
		if _, err := conn.Write([]byte(cmd)); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("ReadString failed: %v", err)
		}
		line = strings.TrimRight(line, "\r\n")
		if line != expected {
			t.Errorf("cmd %q: expected %q, got %q", cmd, expected, line)
		}
	}

	// 1. PING
	sendAndAssert("PING\r\n", "+PONG")

	// 2. SET via RESP
	sendAndAssert("*3\r\n$3\r\nSET\r\n$4\r\nuser\r\n$5\r\nAlice\r\n", "+OK")

	// 3. GET via RESP
	// First line is $5, second line is Alice
	if _, err := conn.Write([]byte("*2\r\n$3\r\nGET\r\n$4\r\nuser\r\n")); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	header, _ := reader.ReadString('\n')
	if strings.TrimRight(header, "\r\n") != "$5" {
		t.Errorf("expected $5, got %q", header)
	}
	body, _ := reader.ReadString('\n')
	if strings.TrimRight(body, "\r\n") != "Alice" {
		t.Errorf("expected 'Alice', got %q", body)
	}

	// 4. EXISTS
	sendAndAssert("*2\r\n$6\r\nEXISTS\r\n$4\r\nuser\r\n", ":1")

	// 5. DBSIZE
	sendAndAssert("DBSIZE\r\n", ":1")

	// 6. DEL
	sendAndAssert("*2\r\n$3\r\nDEL\r\n$4\r\nuser\r\n", ":1")

	// 7. GET non-existent
	sendAndAssert("*2\r\n$3\r\nGET\r\n$4\r\nuser\r\n", "$-1")

	// 8. QUIT
	sendAndAssert("QUIT\r\n", "+OK")
}
