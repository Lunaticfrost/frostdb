package server

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"

	"github.com/Lunaticfrost/frostdb/internal/engine"
	"github.com/Lunaticfrost/frostdb/internal/protocol"
)

// Server represents a high-performance TCP server speaking the Redis RESP protocol.
type Server struct {
	addr     string
	store    *engine.Store
	listener net.Listener
	quit     chan struct{}
	wg       sync.WaitGroup
	conns    map[net.Conn]struct{}
	connsMu  sync.Mutex
}

// NewServer initializes a new Server listening on the given address.
func NewServer(addr string, store *engine.Store) *Server {
	return &Server{
		addr:  addr,
		store: store,
		quit:  make(chan struct{}),
		conns: make(map[net.Conn]struct{}),
	}
}

// Start begins listening for incoming client TCP connections.
func (s *Server) Start() error {
	l, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("failed to bind to %s: %w", s.addr, err)
	}
	s.listener = l

	s.wg.Add(1)
	go s.acceptLoop()

	return nil
}

// Addr returns the actual resolved listening address (e.g. for dynamic ports).
func (s *Server) Addr() string {
	if s.listener == nil {
		return s.addr
	}
	return s.listener.Addr().String()
}

func (s *Server) acceptLoop() {
	defer s.wg.Done()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.quit:
				return
			default:
				continue
			}
		}

		s.trackConn(conn, true)
		s.wg.Add(1)
		go func(c net.Conn) {
			defer s.wg.Done()
			defer s.trackConn(c, false)
			defer c.Close()
			s.handleConnection(c)
		}(conn)
	}
}

func (s *Server) trackConn(c net.Conn, add bool) {
	s.connsMu.Lock()
	defer s.connsMu.Unlock()
	if add {
		s.conns[c] = struct{}{}
	} else {
		delete(s.conns, c)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	r := bufio.NewReader(conn)
	w := bufio.NewWriter(conn)

	for {
		args, err := protocol.ReadCommand(r)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				return
			}
			_ = protocol.WriteError(w, err.Error())
			_ = w.Flush()
			return
		}

		if len(args) == 0 {
			continue
		}

		shouldQuit := s.dispatch(w, args)
		if err := w.Flush(); err != nil {
			return
		}

		if shouldQuit {
			return
		}
	}
}

func (s *Server) dispatch(w io.Writer, args [][]byte) bool {
	cmd := strings.ToUpper(string(args[0]))

	switch cmd {
	case "PING":
		if len(args) > 1 {
			_ = protocol.WriteBulkString(w, args[1])
		} else {
			_ = protocol.WriteSimpleString(w, "PONG")
		}

	case "ECHO":
		if len(args) < 2 {
			_ = protocol.WriteError(w, "wrong number of arguments for 'echo' command")
		} else {
			_ = protocol.WriteBulkString(w, args[1])
		}

	case "SET":
		if len(args) < 3 {
			_ = protocol.WriteError(w, "wrong number of arguments for 'set' command")
			return false
		}
		key := string(args[1])
		value := args[2]
		if err := s.store.Set(key, value); err != nil {
			_ = protocol.WriteError(w, err.Error())
		} else {
			_ = protocol.WriteSimpleString(w, "OK")
		}

	case "GET":
		if len(args) != 2 {
			_ = protocol.WriteError(w, "wrong number of arguments for 'get' command")
			return false
		}
		key := string(args[1])
		val, exists := s.store.Get(key)
		if !exists {
			_ = protocol.WriteNull(w)
		} else {
			_ = protocol.WriteBulkString(w, val)
		}

	case "DEL", "DELETE":
		if len(args) < 2 {
			_ = protocol.WriteError(w, "wrong number of arguments for 'del' command")
			return false
		}
		var count int64
		for _, arg := range args[1:] {
			if s.store.Delete(string(arg)) {
				count++
			}
		}
		_ = protocol.WriteInteger(w, count)

	case "EXISTS":
		if len(args) < 2 {
			_ = protocol.WriteError(w, "wrong number of arguments for 'exists' command")
			return false
		}
		var count int64
		for _, arg := range args[1:] {
			if s.store.Exists(string(arg)) {
				count++
			}
		}
		_ = protocol.WriteInteger(w, count)

	case "KEYS":
		keys := s.store.Keys()
		byteKeys := make([][]byte, 0, len(keys))
		for _, k := range keys {
			byteKeys = append(byteKeys, []byte(k))
		}
		_ = protocol.WriteArray(w, byteKeys)

	case "DBSIZE":
		_ = protocol.WriteInteger(w, int64(s.store.Size()))

	case "FLUSHDB", "CLEAR":
		s.store.Clear()
		_ = protocol.WriteSimpleString(w, "OK")

	case "INFO":
		info := s.buildInfo()
		_ = protocol.WriteBulkString(w, []byte(info))

	case "COMPACT", "BGSAVE", "SAVE":
		if !s.store.IsPersistent() {
			_ = protocol.WriteError(w, "compaction not supported on in-memory store")
			return false
		}
		_, err := s.store.Compact()
		if err != nil {
			_ = protocol.WriteError(w, err.Error())
		} else {
			_ = protocol.WriteSimpleString(w, "OK")
		}

	case "QUIT":
		_ = protocol.WriteSimpleString(w, "OK")
		return true

	default:
		_ = protocol.WriteError(w, fmt.Sprintf("unknown command '%s'", bytes.ToLower(args[0])))
	}

	return false
}

func (s *Server) buildInfo() string {
	mode := "in-memory"
	if s.store.IsPersistent() {
		mode = "persistent"
	}
	return fmt.Sprintf("# Server\r\nfrostdb_version:0.3.0\r\nmode:%s\r\ntotal_keys:%d\r\n", mode, s.store.Size())
}

// Stop gracefully stops the server, disconnects clients, and closes the listener.
func (s *Server) Stop() error {
	close(s.quit)

	var err error
	if s.listener != nil {
		err = s.listener.Close()
	}

	s.connsMu.Lock()
	for c := range s.conns {
		_ = c.Close()
	}
	s.connsMu.Unlock()

	s.wg.Wait()
	return err
}
