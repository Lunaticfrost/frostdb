package protocol

import (
	"bufio"
	"bytes"
	"testing"
)

func TestReadCommandRESPArray(t *testing.T) {
	input := "*3\r\n$3\r\nSET\r\n$4\r\nname\r\n$5\r\nAlice\r\n"
	r := bufio.NewReader(bytes.NewBufferString(input))

	args, err := ReadCommand(r)
	if err != nil {
		t.Fatalf("ReadCommand failed: %v", err)
	}

	if len(args) != 3 {
		t.Fatalf("expected 3 args, got %d", len(args))
	}
	if string(args[0]) != "SET" || string(args[1]) != "name" || string(args[2]) != "Alice" {
		t.Errorf("unexpected args: %q", args)
	}
}

func TestReadCommandInline(t *testing.T) {
	input := "SET key Hello World\r\n"
	r := bufio.NewReader(bytes.NewBufferString(input))

	args, err := ReadCommand(r)
	if err != nil {
		t.Fatalf("ReadCommand inline failed: %v", err)
	}

	if len(args) != 4 {
		t.Fatalf("expected 4 args, got %d", len(args))
	}
	if string(args[0]) != "SET" || string(args[1]) != "key" || string(args[2]) != "Hello" || string(args[3]) != "World" {
		t.Errorf("unexpected args: %q", args)
	}
}

func TestRESPWriters(t *testing.T) {
	var buf bytes.Buffer

	_ = WriteSimpleString(&buf, "OK")
	if buf.String() != "+OK\r\n" {
		t.Errorf("WriteSimpleString got %q", buf.String())
	}
	buf.Reset()

	_ = WriteError(&buf, "ERR message")
	if buf.String() != "-ERR ERR message\r\n" {
		t.Errorf("WriteError got %q", buf.String())
	}
	buf.Reset()

	_ = WriteInteger(&buf, 42)
	if buf.String() != ":42\r\n" {
		t.Errorf("WriteInteger got %q", buf.String())
	}
	buf.Reset()

	_ = WriteBulkString(&buf, []byte("hello"))
	if buf.String() != "$5\r\nhello\r\n" {
		t.Errorf("WriteBulkString got %q", buf.String())
	}
	buf.Reset()

	_ = WriteNull(&buf)
	if buf.String() != "$-1\r\n" {
		t.Errorf("WriteNull got %q", buf.String())
	}
	buf.Reset()

	_ = WriteArray(&buf, [][]byte{[]byte("foo"), []byte("bar")})
	expected := "*2\r\n$3\r\nfoo\r\n$3\r\nbar\r\n"
	if buf.String() != expected {
		t.Errorf("WriteArray got %q, expected %q", buf.String(), expected)
	}
}
