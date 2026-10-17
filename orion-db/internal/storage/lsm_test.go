package storage

import (
	"encoding/gob"
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

func TestLSMMigratesLegacyGobSSTable(t *testing.T) {
	dir := t.TempDir()
	legacyPath := filepath.Join(dir, "data-0.sst")
	legacyFile, err := os.Create(legacyPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := gob.NewEncoder(legacyFile).Encode(Entry{Key: 71, Timestamp: 710, Value: 7.1}); err != nil {
		t.Fatal(err)
	}
	if err := legacyFile.Close(); err != nil {
		t.Fatal(err)
	}
	manifestFile, err := os.Create(filepath.Join(dir, "MANIFEST"))
	if err != nil {
		t.Fatal(err)
	}
	if err := gob.NewEncoder(manifestFile).Encode(manifest{SSTables: []string{legacyPath}}); err != nil {
		t.Fatal(err)
	}
	if err := manifestFile.Close(); err != nil {
		t.Fatal(err)
	}

	db, err := OpenLSM(Config{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	entry, err := db.Get(71)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Timestamp != 710 || entry.Value != 7.1 {
		t.Fatalf("legacy value changed during migration: %#v", entry)
	}
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Fatalf("legacy table should be removed after manifest commit, stat error=%v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenLSM(Config{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	entry, err = reopened.Get(71)
	if err != nil || entry.Timestamp != 710 || entry.Value != 7.1 {
		t.Fatalf("migrated binary table did not survive reopen: entry=%#v err=%v", entry, err)
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
