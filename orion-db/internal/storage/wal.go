package storage

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"math"
	"os"
	"sync"
)

const (
	walPayloadSize = 20
	walHeaderSize  = 8
	walFrameSize   = walHeaderSize + walPayloadSize
)

var ErrWALClosed = errors.New("WAL is closed")

type Record struct {
	Timestamp int64
	SeriesID  uint32
	Value     float64
}

type WAL struct {
	mu     sync.Mutex
	file   *os.File
	writer *bufio.Writer
	closed bool
}

func NewWAL(path string) (*WAL, error) {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open WAL: %w", err)
	}

	if _, err := recoverWAL(file); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("recover WAL: %w", err)
	}

	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("seek WAL end: %w", err)
	}

	return &WAL{
		file:   file,
		writer: bufio.NewWriterSize(file, 256*1024),
	}, nil
}

// recoverWAL validates every complete frame and truncates the first invalid,
// corrupted, or partially written tail.
func recoverWAL(file *os.File) (int64, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}

	var frame [walFrameSize]byte
	var validEnd int64

	for {
		n, err := io.ReadFull(file, frame[:])
		switch {
		case err == nil:
			// Continue validation.
		case errors.Is(err, io.EOF):
			// Clean end of file.
			if err := file.Truncate(validEnd); err != nil {
				return 0, err
			}
			return validEnd, nil
		case errors.Is(err, io.ErrUnexpectedEOF):
			// Torn tail.
			if err := file.Truncate(validEnd); err != nil {
				return 0, err
			}
			return validEnd, nil
		default:
			return 0, fmt.Errorf("read WAL at offset %d: %w", validEnd, err)
		}

		if n != walFrameSize {
			if err := file.Truncate(validEnd); err != nil {
				return 0, err
			}
			return validEnd, nil
		}

		payloadLength := binary.LittleEndian.Uint32(frame[0:4])
		if payloadLength != walPayloadSize {
			if err := file.Truncate(validEnd); err != nil {
				return 0, err
			}
			return validEnd, nil
		}

		expectedChecksum := binary.LittleEndian.Uint32(frame[4:8])
		actualChecksum := crc32.ChecksumIEEE(
			frame[walHeaderSize:walFrameSize],
		)
		if expectedChecksum != actualChecksum {
			if err := file.Truncate(validEnd); err != nil {
				return 0, err
			}
			return validEnd, nil
		}

		validEnd += walFrameSize
	}
}

func encodeRecord(frame *[walFrameSize]byte, record Record) {
	binary.LittleEndian.PutUint32(frame[0:4], walPayloadSize)
	binary.LittleEndian.PutUint64(
		frame[walHeaderSize:walHeaderSize+8],
		uint64(record.Timestamp),
	)
	binary.LittleEndian.PutUint32(
		frame[walHeaderSize+8:walHeaderSize+12],
		record.SeriesID,
	)
	binary.LittleEndian.PutUint64(
		frame[walHeaderSize+12:walHeaderSize+20],
		math.Float64bits(record.Value),
	)
	checksum := crc32.ChecksumIEEE(frame[walHeaderSize:walFrameSize])
	binary.LittleEndian.PutUint32(frame[4:8], checksum)
}

// Append writes one record to the process buffer.
// It does not constitute a durability boundary.
func (w *WAL) Append(record Record) error {
	var frame [walFrameSize]byte
	encodeRecord(&frame, record)

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return ErrWALClosed
	}

	if _, err := w.writer.Write(frame[:]); err != nil {
		return fmt.Errorf("buffer WAL record: %w", err)
	}
	return nil
}

// AppendBatch writes a batch of records to the process buffer only.
// Call Flush to move data into the OS kernel buffer, or Sync for full durability.
func (w *WAL) AppendBatch(records []Record) error {
	if len(records) == 0 {
		return nil
	}

	// Encode outside the WAL lock.
	encoded := make([]byte, len(records)*walFrameSize)
	for i, record := range records {
		var frame [walFrameSize]byte
		encodeRecord(&frame, record)
		start := i * walFrameSize
		copy(encoded[start:start+walFrameSize], frame[:])
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return ErrWALClosed
	}

	if _, err := w.writer.Write(encoded); err != nil {
		return fmt.Errorf("write WAL batch: %w", err)
	}
	return nil
}

// Flush moves the process buffer into the OS kernel buffer.
// Data written before Flush survives a process crash but not a kernel/power crash.
// Call Sync for full on-disk durability.
func (w *WAL) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return ErrWALClosed
	}

	if err := w.writer.Flush(); err != nil {
		return fmt.Errorf("flush WAL: %w", err)
	}
	return nil
}

// AppendBatchAndSync writes a complete batch and performs one fsync.
//
// Successful return means every record in this batch passed through
// bufio.Flush and os.File.Sync.
func (w *WAL) AppendBatchAndSync(records []Record) error {
	if len(records) == 0 {
		return nil
	}

	// Encode outside the WAL lock.
	encoded := make([]byte, len(records)*walFrameSize)
	for i, record := range records {
		var frame [walFrameSize]byte
		encodeRecord(&frame, record)
		start := i * walFrameSize
		copy(encoded[start:start+walFrameSize], frame[:])
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return ErrWALClosed
	}

	if _, err := w.writer.Write(encoded); err != nil {
		return fmt.Errorf("write WAL batch: %w", err)
	}
	if err := w.writer.Flush(); err != nil {
		return fmt.Errorf("flush WAL batch: %w", err)
	}
	if err := w.file.Sync(); err != nil {
		return fmt.Errorf("sync WAL batch: %w", err)
	}
	return nil
}

func (w *WAL) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return ErrWALClosed
	}

	if err := w.writer.Flush(); err != nil {
		return fmt.Errorf("flush WAL: %w", err)
	}
	if err := w.file.Sync(); err != nil {
		return fmt.Errorf("sync WAL: %w", err)
	}
	return nil
}

func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}
	w.closed = true

	var result error
	if err := w.writer.Flush(); err != nil {
		result = errors.Join(result, fmt.Errorf("flush WAL: %w", err))
	}
	if err := w.file.Sync(); err != nil {
		result = errors.Join(result, fmt.Errorf("sync WAL: %w", err))
	}
	if err := w.file.Close(); err != nil {
		result = errors.Join(result, fmt.Errorf("close WAL: %w", err))
	}
	return result
}
