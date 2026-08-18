package storage

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"math"
	"os"
	"sync"
)

const (
	// payload: timestamp(8) + seriesID(4) + value(8) = 20
	walPayloadSize = 20
	walHeaderSize  = 8 // payloadLen(4) + checksum(4)
	walFrameSize   = walHeaderSize + walPayloadSize
)

type WAL struct {
	mu     sync.Mutex
	file   *os.File
	writer *bufio.Writer
	buf    [walFrameSize]byte
}

func NewWAL(path string) (*WAL, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, err
	}

	if err := recoverWAL(f); err != nil {
		f.Close()
		return nil, err
	}

	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		f.Close()
		return nil, err
	}

	return &WAL{
		file:   f,
		writer: bufio.NewWriterSize(f, 64*1024),
	}, nil
}

func recoverWAL(f *os.File) error {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}

	buf := make([]byte, walFrameSize)
	validEnd := int64(0)

	for {
		if _, err := io.ReadFull(f, buf[:walHeaderSize]); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			return err
		}

		payloadLen := binary.LittleEndian.Uint32(buf[0:4])
		checksum := binary.LittleEndian.Uint32(buf[4:8])
		if payloadLen != walPayloadSize {
			return fmt.Errorf("wal record size mismatch: %d", payloadLen)
		}

		if _, err := io.ReadFull(f, buf[walHeaderSize:walFrameSize]); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			return err
		}

		if crc32.ChecksumIEEE(buf[walHeaderSize:walFrameSize]) != checksum {
			return fmt.Errorf("wal checksum mismatch at offset %d", validEnd)
		}

		validEnd += walFrameSize
	}

	return f.Truncate(validEnd)
}

// Append writes a framed WAL record containing timestamp, seriesID, and value.
func (w *WAL) Append(timestamp int64, seriesID uint32, value float64) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	binary.LittleEndian.PutUint32(w.buf[0:4], walPayloadSize)
	// checksum placeholder at [4:8]

	// payload: timestamp(8) | seriesID(4) | value(8)
	binary.LittleEndian.PutUint64(w.buf[walHeaderSize+0:walHeaderSize+8], uint64(timestamp))
	binary.LittleEndian.PutUint32(w.buf[walHeaderSize+8:walHeaderSize+12], seriesID)
	binary.LittleEndian.PutUint64(w.buf[walHeaderSize+12:walHeaderSize+20], math.Float64bits(value))

	checksum := crc32.ChecksumIEEE(w.buf[walHeaderSize:walFrameSize])
	binary.LittleEndian.PutUint32(w.buf[4:8], checksum)

	_, err := w.writer.Write(w.buf[:])
	return err
}

func (w *WAL) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.writer.Flush(); err != nil {
		return err
	}
	return w.file.Sync()
}

func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.writer.Flush(); err != nil {
		w.file.Close()
		return err
	}
	return w.file.Close()
}
