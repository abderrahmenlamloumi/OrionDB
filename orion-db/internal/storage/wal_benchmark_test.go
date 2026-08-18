package storage

import (
	"path/filepath"
	"testing"
)

func BenchmarkWALAppendBuffered(b *testing.B) {
	wal, err := NewWAL(filepath.Join(b.TempDir(), "benchmark.wal"))
	if err != nil {
		b.Fatal(err)
	}
	defer wal.Close()

	record := Record{
		Timestamp: 1,
		SeriesID:  1,
		Value:     42,
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		record.Timestamp = int64(i)
		if err := wal.Append(record); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkWALGroupCommit256(b *testing.B) {
	wal, err := NewWAL(filepath.Join(b.TempDir(), "benchmark.wal"))
	if err != nil {
		b.Fatal(err)
	}
	defer wal.Close()

	records := make([]Record, 256)
	for i := range records {
		records[i] = Record{
			Timestamp: int64(i),
			SeriesID:  uint32(i + 1),
			Value:     float64(i),
		}
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if err := wal.AppendBatchAndSync(records); err != nil {
			b.Fatal(err)
		}
	}

	b.SetBytes(int64(len(records) * walFrameSize))
}
