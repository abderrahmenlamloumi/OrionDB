package storage

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestWALAppendBatchAndRecover(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.wal")

	wal, err := NewWAL(path)
	if err != nil {
		t.Fatal(err)
	}

	records := []Record{
		{Timestamp: 1, SeriesID: 10, Value: 1.5},
		{Timestamp: 2, SeriesID: 11, Value: 2.5},
		{Timestamp: 3, SeriesID: 12, Value: 3.5},
	}

	if err := wal.AppendBatchAndSync(records); err != nil {
		t.Fatal(err)
	}
	if err := wal.Close(); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	expected := int64(len(records) * walFrameSize)
	if info.Size() != expected {
		t.Fatalf("size=%d expected=%d", info.Size(), expected)
	}

	reopened, err := NewWAL(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
}

func TestWALTruncatesTornTail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "torn.wal")

	wal, err := NewWAL(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := wal.AppendBatchAndSync([]Record{
		{Timestamp: 1, SeriesID: 1, Value: 10},
		{Timestamp: 2, SeriesID: 2, Value: 20},
	}); err != nil {
		t.Fatal(err)
	}
	if err := wal.Close(); err != nil {
		t.Fatal(err)
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a crash in the middle of a frame.
	if _, err := file.Write(make([]byte, 13)); err != nil {
		t.Fatal(err)
	}
	_ = file.Close()

	recovered, err := NewWAL(path)
	if err != nil {
		t.Fatal(err)
	}
	_ = recovered.Close()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	expected := int64(2 * walFrameSize)
	if info.Size() != expected {
		t.Fatalf("size=%d expected=%d", info.Size(), expected)
	}
}

func TestWALTruncatesChecksumCorruption(t *testing.T) {
	path := filepath.Join(t.TempDir(), "corrupt.wal")

	wal, err := NewWAL(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := wal.AppendBatchAndSync([]Record{
		{Timestamp: 1, SeriesID: 1, Value: math.Pi},
		{Timestamp: 2, SeriesID: 2, Value: math.E},
	}); err != nil {
		t.Fatal(err)
	}
	if err := wal.Close(); err != nil {
		t.Fatal(err)
	}

	file, err := os.OpenFile(path, os.O_RDWR, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	// Corrupt the value in the second frame without updating CRC32.
	offset := int64(walFrameSize + walHeaderSize + 12)
	var corruptValue [8]byte
	binary.LittleEndian.PutUint64(corruptValue[:], 0xDEADBEEF)
	if _, err := file.WriteAt(corruptValue[:], offset); err != nil {
		t.Fatal(err)
	}
	_ = file.Close()

	recovered, err := NewWAL(path)
	if err != nil {
		t.Fatal(err)
	}
	_ = recovered.Close()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if info.Size() != walFrameSize {
		t.Fatalf(
			"size=%d expected=%d",
			info.Size(),
			walFrameSize,
		)
	}
}
