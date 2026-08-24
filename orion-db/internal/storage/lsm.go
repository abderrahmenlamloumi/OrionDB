package storage

import (
	"encoding/binary"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

var ErrNotFound = errors.New("key not found")

type Config struct {
	Dir                 string
	MemtableMaxEntries  int
	CompactionThreshold int
}

type Entry struct {
	Key       uint32
	Value     float64
	Timestamp int64
}

type SSTable struct {
	Path string
}

type manifest struct {
	SSTables []string
}

type LSM struct {
	mu sync.RWMutex

	dir                 string
	walPath             string
	manifestPath        string
	memtableMaxEntries  int
	compactionThreshold int
	done                chan struct{}
	compacting          bool

	memtable map[uint32]Entry
	sstables []SSTable
	wal      *WAL
}

func OpenLSM(cfg Config) (*LSM, error) {
	if cfg.Dir == "" {
		return nil, errors.New("storage directory is required")
	}
	if cfg.MemtableMaxEntries <= 0 {
		cfg.MemtableMaxEntries = 1024
	}
	if cfg.CompactionThreshold <= 0 {
		cfg.CompactionThreshold = 4
	}

	if err := os.MkdirAll(cfg.Dir, 0o755); err != nil {
		return nil, fmt.Errorf("create storage dir: %w", err)
	}

	walPath := filepath.Join(cfg.Dir, "db.wal")
	manifestPath := filepath.Join(cfg.Dir, "MANIFEST")

	wal, err := NewWAL(walPath)
	if err != nil {
		return nil, fmt.Errorf("open WAL: %w", err)
	}

	mf, err := readManifest(manifestPath)
	if err != nil {
		_ = wal.Close()
		return nil, err
	}

	sstables := make([]SSTable, 0, len(mf.SSTables))
	for _, p := range mf.SSTables {
		sstables = append(sstables, SSTable{Path: p})
	}

	memtable, err := replayWAL(walPath)
	if err != nil {
		_ = wal.Close()
		return nil, fmt.Errorf("replay WAL: %w", err)
	}

	db := &LSM{
		dir:                 cfg.Dir,
		walPath:             walPath,
		manifestPath:        manifestPath,
		memtableMaxEntries:  cfg.MemtableMaxEntries,
		compactionThreshold: cfg.CompactionThreshold,
		memtable:            memtable,
		sstables:            sstables,
		wal:                 wal,
		done:                make(chan struct{}),
	}

	go db.syncLoop()
	return db, nil
}

func (db *LSM) Close() error {
	db.mu.Lock()
	if db.wal == nil {
		db.mu.Unlock()
		return nil
	}
	wal := db.wal
	db.wal = nil
	close(db.done)
	db.mu.Unlock()

	return wal.Close()
}

func (db *LSM) syncLoop() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-db.done:
			return
		case <-ticker.C:
			db.mu.RLock()
			wal := db.wal
			db.mu.RUnlock()
			if wal != nil {
				_ = wal.Sync()
			}
		}
	}
}

func (db *LSM) Put(seriesID uint32, timestamp int64, value float64) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.wal == nil {
		return ErrWALClosed
	}

	rec := Record{Timestamp: timestamp, SeriesID: seriesID, Value: value}

	// FIX: Buffer the write in memory instead of forcing a blocking fsync.
	if err := db.wal.Append(rec); err != nil {
		return err
	}

	db.memtable[seriesID] = Entry{Key: seriesID, Value: value, Timestamp: timestamp}
	if len(db.memtable) >= db.memtableMaxEntries {
		if err := db.flushLocked(); err != nil {
			return err
		}
	}

	return nil
}

func (db *LSM) Get(seriesID uint32) (Entry, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if e, ok := db.memtable[seriesID]; ok {
		return e, nil
	}

	for i := len(db.sstables) - 1; i >= 0; i-- {
		e, found, err := getFromSSTable(db.sstables[i].Path, seriesID)
		if err != nil {
			return Entry{}, err
		}
		if found {
			return e, nil
		}
	}

	return Entry{}, ErrNotFound
}

func (db *LSM) Flush() error {
	db.mu.Lock()
	defer db.mu.Unlock()
	return db.flushLocked()
}

func (db *LSM) flushLocked() error {
	if len(db.memtable) == 0 {
		return nil
	}

	entries := make([]Entry, 0, len(db.memtable))
	for _, e := range db.memtable {
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Key < entries[j].Key
	})

	path := filepath.Join(db.dir, fmt.Sprintf("data-%d.sst", len(db.sstables)))
	if err := writeSSTable(path, entries); err != nil {
		return err
	}

	db.sstables = append(db.sstables, SSTable{Path: path})
	if err := db.writeManifestLocked(); err != nil {
		return err
	}

	db.memtable = make(map[uint32]Entry)
	if err := db.resetWALLocked(); err != nil {
		return err
	}

	return nil
}

func (db *LSM) compactAsync() {
	for {
		db.mu.Lock()
		if db.compacting || len(db.sstables) < db.compactionThreshold {
			db.mu.Unlock()
			return
		}
		db.compacting = true
		db.mu.Unlock()

		if err := db.compactLocked(); err == nil {
			db.mu.Lock()
			db.compacting = false
			db.mu.Unlock()
			return
		}

		db.mu.Lock()
		db.compacting = false
		db.mu.Unlock()
		return
	}
}

func (db *LSM) compactLocked() error {
	if len(db.sstables) < 2 {
		return nil
	}

	latest := make(map[uint32]Entry)
	for i := 0; i < len(db.sstables); i++ {
		entries, err := readSSTable(db.sstables[i].Path)
		if err != nil {
			return err
		}
		for _, e := range entries {
			latest[e.Key] = e
		}
	}

	merged := make([]Entry, 0, len(latest))
	for _, e := range latest {
		merged = append(merged, e)
	}
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Key < merged[j].Key
	})

	compactPath := filepath.Join(db.dir, fmt.Sprintf("data-compacted-%d.sst", len(db.sstables)))
	if err := writeSSTable(compactPath, merged); err != nil {
		return err
	}

	oldSSTables := db.sstables
	db.sstables = []SSTable{{Path: compactPath}}
	if err := db.writeManifestLocked(); err != nil {
		db.sstables = oldSSTables
		return err
	}

	for _, s := range oldSSTables {
		if s.Path == compactPath {
			continue
		}
		_ = os.Remove(s.Path)
	}

	return nil
}

func (db *LSM) writeManifestLocked() error {
	m := manifest{SSTables: make([]string, 0, len(db.sstables))}
	for _, s := range db.sstables {
		m.SSTables = append(m.SSTables, s.Path)
	}
	return writeManifest(db.manifestPath, m)
}

func (db *LSM) resetWALLocked() error {
	if err := db.wal.Close(); err != nil {
		return err
	}
	if err := os.Remove(db.walPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	nextWAL, err := NewWAL(db.walPath)
	if err != nil {
		return err
	}
	db.wal = nextWAL
	return nil
}

func writeSSTable(path string, entries []Entry) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := gob.NewEncoder(f)
	for _, e := range entries {
		if err := enc.Encode(e); err != nil {
			return err
		}
	}
	return nil
}

func readSSTable(path string) ([]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	dec := gob.NewDecoder(f)
	entries := make([]Entry, 0, 128)
	for {
		var e Entry
		if err := dec.Decode(&e); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func getFromSSTable(path string, key uint32) (Entry, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return Entry{}, false, err
	}
	defer f.Close()

	dec := gob.NewDecoder(f)
	for {
		var e Entry
		if err := dec.Decode(&e); err != nil {
			if errors.Is(err, io.EOF) {
				return Entry{}, false, nil
			}
			return Entry{}, false, err
		}
		if e.Key == key {
			return e, true, nil
		}
		if e.Key > key {
			return Entry{}, false, nil
		}
	}
}

func readManifest(path string) (manifest, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return manifest{SSTables: []string{}}, nil
		}
		return manifest{}, err
	}
	defer f.Close()

	var m manifest
	if err := gob.NewDecoder(f).Decode(&m); err != nil {
		return manifest{}, err
	}
	return m, nil
}

func writeManifest(path string, m manifest) error {
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	enc := gob.NewEncoder(f)
	if err := enc.Encode(m); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func replayWAL(path string) (map[uint32]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return make(map[uint32]Entry), nil
		}
		return nil, err
	}
	defer f.Close()

	memtable := make(map[uint32]Entry)
	var frame [walFrameSize]byte
	for {
		_, err := io.ReadFull(f, frame[:])
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				break
			}
			return nil, err
		}

		payloadLength := binary.LittleEndian.Uint32(frame[0:4])
		if payloadLength != walPayloadSize {
			break
		}

		ts := int64(binary.LittleEndian.Uint64(frame[walHeaderSize : walHeaderSize+8]))
		seriesID := binary.LittleEndian.Uint32(frame[walHeaderSize+8 : walHeaderSize+12])
		valueBits := binary.LittleEndian.Uint64(frame[walHeaderSize+12 : walHeaderSize+20])

		memtable[seriesID] = Entry{
			Key:       seriesID,
			Value:     math.Float64frombits(valueBits),
			Timestamp: ts,
		}
	}

	return memtable, nil
}
