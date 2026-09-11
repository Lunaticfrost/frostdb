package engine

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"time"
)

// OpType represents the database operation type in the WAL.
type OpType uint8

const (
	// OpSet represents an insert or update.
	OpSet OpType = 1
	// OpDelete represents a key deletion.
	OpDelete OpType = 2
	// OpClear represents clearing all keys.
	OpClear OpType = 3
)

const (
	// HeaderSize is the fixed size of a record header in bytes:
	// CRC (4) + Timestamp (8) + OpType (1) + KeyLen (2) + ValLen (4) = 19 bytes
	HeaderSize = 19
	// MaxKeySize is the maximum permitted key length (64 KB).
	MaxKeySize = 65535
	// MaxValueSize is the maximum permitted value length (16 MB).
	MaxValueSize = 16 * 1024 * 1024
)

var (
	// ErrCorruptRecord indicates a checksum mismatch or invalid header length.
	ErrCorruptRecord = errors.New("corrupt record: checksum mismatch or invalid length")
	// ErrKeyTooLarge indicates the key exceeds MaxKeySize.
	ErrKeyTooLarge = errors.New("key exceeds maximum permitted size")
	// ErrValueTooLarge indicates the value exceeds MaxValueSize.
	ErrValueTooLarge = errors.New("value exceeds maximum permitted size")
)

// Record represents a single framed database mutation.
type Record struct {
	CRC       uint32
	Timestamp uint64
	Op        OpType
	Key       string
	Value     string
}

// NewRecord creates a new Record instance stamped with the current UTC timestamp.
func NewRecord(op OpType, key, value string) *Record {
	return &Record{
		Timestamp: uint64(time.Now().UnixNano()),
		Op:        op,
		Key:       key,
		Value:     value,
	}
}

// EncodeRecord serializes a record into a framed byte slice with CRC32 verification.
//
// Wire format:
// +----------------+-------------------+---------------+------------------+------------------+---------------+-----------------+
// | CRC32 (4 bytes)| Timestamp (8 bytes)| OpType (1 byte)| KeyLen (2 bytes) | ValLen (4 bytes) | Key (N bytes) | Value (M bytes) |
// +----------------+-------------------+---------------+------------------+------------------+---------------+-----------------+
func EncodeRecord(rec *Record) ([]byte, error) {
	keyBytes := []byte(rec.Key)
	valBytes := []byte(rec.Value)

	if len(keyBytes) > MaxKeySize {
		return nil, ErrKeyTooLarge
	}
	if len(valBytes) > MaxValueSize {
		return nil, ErrValueTooLarge
	}

	keyLen := uint16(len(keyBytes))
	valLen := uint32(len(valBytes))
	totalSize := HeaderSize + len(keyBytes) + len(valBytes)
	buf := make([]byte, totalSize)

	// Encode payload fields starting at offset 4 (after CRC)
	binary.BigEndian.PutUint64(buf[4:12], rec.Timestamp)
	buf[12] = byte(rec.Op)
	binary.BigEndian.PutUint16(buf[13:15], keyLen)
	binary.BigEndian.PutUint32(buf[15:19], valLen)

	copy(buf[19:19+len(keyBytes)], keyBytes)
	copy(buf[19+len(keyBytes):], valBytes)

	// Calculate CRC32 (IEEE) over everything after the CRC field (buf[4:])
	checksum := crc32.ChecksumIEEE(buf[4:])
	binary.BigEndian.PutUint32(buf[0:4], checksum)
	rec.CRC = checksum

	return buf, nil
}

// DecodeRecord reads and decodes the next Record from an io.Reader.
// Returns io.EOF when no more bytes are available.
func DecodeRecord(r io.Reader) (*Record, error) {
	header := make([]byte, HeaderSize)
	_, err := io.ReadFull(r, header)
	if err != nil {
		return nil, err // Returns io.EOF or io.ErrUnexpectedEOF
	}

	expectedCRC := binary.BigEndian.Uint32(header[0:4])
	timestamp := binary.BigEndian.Uint64(header[4:12])
	op := OpType(header[12])
	keyLen := binary.BigEndian.Uint16(header[13:15])
	valLen := binary.BigEndian.Uint32(header[15:19])

	if int(valLen) > MaxValueSize {
		return nil, fmt.Errorf("%w: value length %d exceeds max %d", ErrCorruptRecord, valLen, MaxValueSize)
	}

	body := make([]byte, int(keyLen)+int(valLen))
	if len(body) > 0 {
		if _, err := io.ReadFull(r, body); err != nil {
			return nil, err
		}
	}

	// Verify CRC32
	crc := crc32.NewIEEE()
	crc.Write(header[4:])
	crc.Write(body)
	actualCRC := crc.Sum32()

	if actualCRC != expectedCRC {
		return nil, fmt.Errorf("%w: expected %x, got %x", ErrCorruptRecord, expectedCRC, actualCRC)
	}

	key := string(body[:keyLen])
	value := string(body[keyLen:])

	return &Record{
		CRC:       expectedCRC,
		Timestamp: timestamp,
		Op:        op,
		Key:       key,
		Value:     value,
	}, nil
}
