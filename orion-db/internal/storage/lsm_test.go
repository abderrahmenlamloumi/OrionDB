package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLSMPutGet(t *testing.T) {
	db, err := OpenLSM(Config{
		Dir:                 t.TempDir(),
		MemtableMaxEntries:  4,
		CompactionThreshold: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.Put(42, 1000, 1.5); err != nil {
		t.Fatal(err)
	}

	entry, err := db.Get(42)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Key != 42 || entry.Timestamp != 1000 || entry.Value != 1.5 {
		t.Fatalf("unexpected entry: %#v", entry)
	}
}

func TestLSMPutBuffersWALBeforeSync(t *testing.T) {
	db, err := OpenLSM(Config{
		Dir:                 t.TempDir(),
		MemtableMaxEntries:  100,
		CompactionThreshold: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.Put(42, 1000, 1.5); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(db.walPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != 0 {
		t.Fatalf("Put should buffer WAL data in memory before the background syncer flushes it; size=%d", info.Size())
	}
}

func TestLSMRecoveryFromWAL(t *testing.T) {
	dir := t.TempDir()

	db, err := OpenLSM(Config{
		Dir:                 dir,
		MemtableMaxEntries:  100,
		CompactionThreshold: 3,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := db.Put(7, 111, 7.7); err != nil {
		t.Fatal(err)
	}
	if err := db.Put(7, 222, 8.8); err != nil {
		t.Fatal(err)
	}

	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenLSM(Config{
		Dir:                 dir,
		MemtableMaxEntries:  100,
		CompactionThreshold: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()

	entry, err := reopened.Get(7)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Timestamp != 222 || entry.Value != 8.8 {
		t.Fatalf("expected latest WAL value, got %#v", entry)
	}
}

func TestLSMFlushAndCompaction(t *testing.T) {
	dir := t.TempDir()

	db, err := OpenLSM(Config{
		Dir:                 dir,
		MemtableMaxEntries:  2,
		CompactionThreshold: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.Put(1, 1, 1.0); err != nil {
		t.Fatal(err)
	}
	if err := db.Put(2, 2, 2.0); err != nil {
		t.Fatal(err)
	}

	if err := db.Put(1, 3, 3.0); err != nil {
		t.Fatal(err)
	}
	if err := db.Put(3, 4, 4.0); err != nil {
		t.Fatal(err)
	}

	if err := db.Flush(); err != nil {
		t.Fatal(err)
	}

	entry, err := db.Get(1)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Timestamp != 3 || entry.Value != 3.0 {
		t.Fatalf("expected compacted latest value, got %#v", entry)
	}

	manifestPath := filepath.Join(dir, "MANIFEST")
	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("manifest should exist: %v", err)
	}
}
