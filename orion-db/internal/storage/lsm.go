package storage

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var ErrNotFound = errors.New("key not found")

var compactionCrashCheckpoint = func(string) {}

var manifestMagic = [8]byte{'O', 'R', 'I', 'O', 'N', 'M', 'F', '1'}

const (
	manifestVersion     = 1
	manifestHeaderSize  = 22
	manifestTrailerSize = 4
)

type Config struct {
	Dir                 string
	MemtableMaxEntries  int
	CompactionThreshold int
	LevelBaseBytes      int64
	LevelSizeMultiplier int
}

type Entry struct {
	Key       uint32
	Value     float64
	Timestamp int64
}

type SSTable struct {
	Path  string
	Level int
	table *mappedSSTable
}

type manifest struct {
	SSTables []string
	Levels   []int
	NextID   uint64
}

type LSM struct {
	mu sync.RWMutex

	dir                 string
	walPath             string
	manifestPath        string
	memtableMaxEntries  int
	compactionThreshold int
	levelBaseBytes      int64
	levelSizeMultiplier int
	nextSSTableID       uint64
	done                chan struct{}
	compactionQueue     chan struct{}
	background          sync.WaitGroup
	compacting          bool
	walSyncLatencyNanos atomic.Int64
	walSyncCount        atomic.Uint64

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
	if cfg.LevelBaseBytes <= 0 {
		cfg.LevelBaseBytes = 64 << 20
	}
	if cfg.LevelSizeMultiplier < 2 {
		cfg.LevelSizeMultiplier = 10
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
	for _, path := range mf.SSTables {
		base := filepath.Base(path)
		if strings.HasPrefix(base, "sstable-") && strings.HasSuffix(base, ".sst") {
			idText := strings.TrimSuffix(strings.TrimPrefix(base, "sstable-"), ".sst")
			if id, parseErr := strconv.ParseUint(idText, 10, 64); parseErr == nil && id >= mf.NextID {
				mf.NextID = id + 1
			}
		}
	}

	sstables := make([]SSTable, 0, len(mf.SSTables))
	legacyPaths := make([]string, 0)
	changedManifest := false
	for i, p := range mf.SSTables {
		table, err := openSSTable(p)
		if err != nil {
			entries, legacyErr := readLegacySSTable(p)
			if legacyErr != nil {
				for i := range sstables {
					sstables[i].table.close()
				}
				_ = wal.Close()
				return nil, fmt.Errorf("open SSTable %q: %w (legacy decode: %v)", p, err, legacyErr)
			}
			newPath := filepath.Join(cfg.Dir, fmt.Sprintf("sstable-%020d.sst", mf.NextID))
			mf.NextID++
			if writeErr := writeSSTable(newPath, entries); writeErr != nil {
				for i := range sstables {
					sstables[i].table.close()
				}
				_ = wal.Close()
				return nil, fmt.Errorf("migrate legacy SSTable %q: %w", p, writeErr)
			}
			table, err = openSSTable(newPath)
			if err != nil {
				_ = os.Remove(newPath)
				for i := range sstables {
					sstables[i].table.close()
				}
				_ = wal.Close()
				return nil, fmt.Errorf("open migrated SSTable %q: %w", newPath, err)
			}
			p = newPath
			legacyPaths = append(legacyPaths, mf.SSTables[i])
			changedManifest = true
		}
		level := 0
		if i < len(mf.Levels) && mf.Levels[i] >= 0 {
			level = mf.Levels[i]
		}
		sstables = append(sstables, SSTable{Path: p, Level: level, table: table})
	}
	if changedManifest {
		mf.SSTables = make([]string, len(sstables))
		mf.Levels = make([]int, len(sstables))
		for i, table := range sstables {
			mf.SSTables[i] = table.Path
			mf.Levels[i] = table.Level
		}
		if err := writeManifest(manifestPath, mf); err != nil {
			for i := range sstables {
				sstables[i].table.close()
			}
			_ = wal.Close()
			return nil, fmt.Errorf("commit migrated SSTable manifest: %w", err)
		}
		for _, path := range legacyPaths {
			_ = os.Remove(path)
		}
	}
	cleanupOrphanSSTables(cfg.Dir, sstables)

	memtable, err := replayWAL(walPath)
	if err != nil {
		for i := range sstables {
			sstables[i].table.close()
		}
		_ = wal.Close()
		return nil, fmt.Errorf("replay WAL: %w", err)
	}
	sortSSTablesByLevel(sstables)

	db := &LSM{
		dir:                 cfg.Dir,
		walPath:             walPath,
		manifestPath:        manifestPath,
		memtableMaxEntries:  cfg.MemtableMaxEntries,
		compactionThreshold: cfg.CompactionThreshold,
		levelBaseBytes:      cfg.LevelBaseBytes,
		levelSizeMultiplier: cfg.LevelSizeMultiplier,
		nextSSTableID:       mf.NextID,
		memtable:            memtable,
		sstables:            sstables,
		wal:                 wal,
		done:                make(chan struct{}),
		compactionQueue:     make(chan struct{}, 1),
	}
	db.background.Add(2)
	go db.syncLoop()
	go db.compactionLoop()
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

	db.background.Wait()
	db.mu.Lock()
	for i := range db.sstables {
		db.sstables[i].table.close()
	}
	db.mu.Unlock()

	return wal.Close()
}

func (db *LSM) syncLoop() {
	defer db.background.Done()
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
				started := time.Now()
				if err := wal.Sync(); err == nil {
					db.walSyncCount.Add(1)
				}
				db.walSyncLatencyNanos.Store(int64(time.Since(started)))
			}
		}
	}
}

type StorageStats struct {
	MemtableEntries      int
	WALSyncLatencyNanos  int64
	WALSyncCount         uint64
	CompactionQueueDepth int
	SSTableCount         int
}

func (db *LSM) Stats() StorageStats {
	db.mu.RLock()
	stats := StorageStats{
		MemtableEntries:      len(db.memtable),
		WALSyncLatencyNanos:  db.walSyncLatencyNanos.Load(),
		WALSyncCount:         db.walSyncCount.Load(),
		CompactionQueueDepth: len(db.compactionQueue),
		SSTableCount:         len(db.sstables),
	}
	db.mu.RUnlock()
	return stats
}

func (db *LSM) compactionLoop() {
	defer db.background.Done()
	for {
		select {
		case <-db.done:
			return
		case <-db.compactionQueue:
			db.compactAsync()
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

	maxLevel := 0
	for i := range db.sstables {
		if db.sstables[i].Level > maxLevel {
			maxLevel = db.sstables[i].Level
		}
	}
	for level := 0; level <= maxLevel; level++ {
		if level == 0 {
			for i := len(db.sstables) - 1; i >= 0; i-- {
				if db.sstables[i].Level != 0 {
					continue
				}
				first, last := sstableRange(db.sstables[i])
				if seriesID < first || seriesID > last {
					continue
				}
				e, found, err := db.sstables[i].table.get(seriesID)
				if err != nil {
					return Entry{}, err
				}
				if found {
					return e, nil
				}
			}
			continue
		}
		start := sort.Search(len(db.sstables), func(i int) bool { return db.sstables[i].Level >= level })
		end := sort.Search(len(db.sstables), func(i int) bool { return db.sstables[i].Level > level })
		if start == end {
			continue
		}
		candidate := start + sort.Search(end-start, func(i int) bool {
			first, _ := sstableRange(db.sstables[start+i])
			return first > seriesID
		}) - 1
		if candidate < start {
			continue
		}
		first, last := sstableRange(db.sstables[candidate])
		if seriesID < first || seriesID > last {
			continue
		}
		e, found, err := db.sstables[candidate].table.get(seriesID)
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

	path := db.nextSSTablePathLocked()
	if err := writeSSTable(path, entries); err != nil {
		return err
	}
	table, err := openSSTable(path)
	if err != nil {
		_ = os.Remove(path)
		return err
	}

	previousTables := db.sstables
	nextTables := make([]SSTable, len(previousTables), len(previousTables)+1)
	copy(nextTables, previousTables)
	nextTables = append(nextTables, SSTable{Path: path, Level: 0, table: table})
	sortSSTablesByLevel(nextTables)
	db.sstables = nextTables
	if err := db.writeManifestLocked(); err != nil {
		table.close()
		db.sstables = previousTables
		_ = os.Remove(path)
		return err
	}

	db.memtable = make(map[uint32]Entry)
	if err := db.resetWALLocked(); err != nil {
		return err
	}
	db.scheduleCompactionLocked()

	return nil
}

func (db *LSM) compactAsync() {
	db.mu.Lock()
	if db.compacting {
		db.mu.Unlock()
		return
	}
	db.compacting = true
	db.mu.Unlock()
	defer func() {
		db.mu.Lock()
		db.compacting = false
		db.mu.Unlock()
	}()

	for {
		compacted, err := db.compactOne()
		if err != nil || !compacted {
			return
		}
	}
}

func (db *LSM) compactOne() (bool, error) {
	db.mu.Lock()
	selected, destinationLevel := db.pickCompactionLocked()
	if len(selected) == 0 {
		db.mu.Unlock()
		return false, nil
	}
	inputs := make([]SSTable, len(selected))
	for i, index := range selected {
		inputs[i] = db.sstables[index]
	}
	db.mu.Unlock()

	// Process older destination levels first so newer source-level values win.
	sort.SliceStable(inputs, func(i, j int) bool {
		return inputs[i].Level > inputs[j].Level
	})
	latest := make(map[uint32]Entry)
	for _, input := range inputs {
		entries, err := readSSTable(input.Path)
		if err != nil {
			return false, err
		}
		for _, entry := range entries {
			latest[entry.Key] = entry
		}
	}
	merged := make([]Entry, 0, len(latest))
	for _, entry := range latest {
		merged = append(merged, entry)
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i].Key < merged[j].Key })

	const outputTableEntries = sstBlockEntries * 128
	outputCount := (len(merged) + outputTableEntries - 1) / outputTableEntries
	db.mu.Lock()
	paths := make([]string, outputCount)
	for i := range paths {
		paths[i] = db.nextSSTablePathLocked()
	}
	db.mu.Unlock()

	outputs := make([]SSTable, 0, outputCount)
	for i, path := range paths {
		start := i * outputTableEntries
		end := start + outputTableEntries
		if end > len(merged) {
			end = len(merged)
		}
		if err := writeSSTable(path, merged[start:end]); err != nil {
			closeAndRemoveSSTables(outputs)
			return false, err
		}
		table, err := openSSTable(path)
		if err != nil {
			_ = os.Remove(path)
			closeAndRemoveSSTables(outputs)
			return false, err
		}
		outputs = append(outputs, SSTable{Path: path, Level: destinationLevel, table: table})
	}

	compactionCrashCheckpoint("before-manifest")
	db.mu.Lock()
	selectedPaths := make(map[string]struct{}, len(inputs))
	for _, input := range inputs {
		selectedPaths[input.Path] = struct{}{}
	}
	current := db.sstables
	next := make([]SSTable, 0, len(current)-len(inputs)+len(outputs))
	for _, table := range current {
		if _, remove := selectedPaths[table.Path]; !remove {
			next = append(next, table)
		}
	}
	if len(current)-len(next) != len(inputs) {
		db.mu.Unlock()
		closeAndRemoveSSTables(outputs)
		return false, errors.New("compaction inputs changed before manifest commit")
	}
	next = append(next, outputs...)
	sortSSTablesByLevel(next)
	db.sstables = next
	if err := db.writeManifestLocked(); err != nil {
		db.sstables = current
		db.mu.Unlock()
		closeAndRemoveSSTables(outputs)
		return false, err
	}
	db.mu.Unlock()
	compactionCrashCheckpoint("after-manifest")

	for _, input := range inputs {
		input.table.close()
		_ = os.Remove(input.Path)
	}
	return true, nil
}

func (db *LSM) pickCompactionLocked() ([]int, int) {
	selected := make([]int, 0)
	minKey, maxKey := uint32(0), uint32(0)
	sourceLevel := -1
	levelZeroCount := 0
	for i := range db.sstables {
		if db.sstables[i].Level == 0 {
			levelZeroCount++
		}
	}
	if levelZeroCount >= db.compactionThreshold {
		sourceLevel = 0
		for i := range db.sstables {
			if db.sstables[i].Level != 0 {
				continue
			}
			selected = append(selected, i)
			first, last := sstableRange(db.sstables[i])
			if len(selected) == 1 || first < minKey {
				minKey = first
			}
			if len(selected) == 1 || last > maxKey {
				maxKey = last
			}
		}
	} else {
		maxLevel := 0
		for i := range db.sstables {
			if db.sstables[i].Level > maxLevel {
				maxLevel = db.sstables[i].Level
			}
		}
		for level := 1; level <= maxLevel; level++ {
			var bytesAtLevel int64
			firstAtLevel := -1
			for i := range db.sstables {
				if db.sstables[i].Level != level {
					continue
				}
				if firstAtLevel < 0 {
					firstAtLevel = i
				}
				bytesAtLevel += int64(len(db.sstables[i].table.data))
			}
			if firstAtLevel < 0 || bytesAtLevel <= db.levelLimit(level) {
				continue
			}
			sourceLevel = level
			selected = append(selected, firstAtLevel)
			minKey, maxKey = sstableRange(db.sstables[firstAtLevel])
			break
		}
	}
	if sourceLevel < 0 {
		return nil, 0
	}
	destinationLevel := sourceLevel + 1
	for i := range db.sstables {
		if db.sstables[i].Level != destinationLevel {
			continue
		}
		first, last := sstableRange(db.sstables[i])
		if first <= maxKey && last >= minKey {
			selected = append(selected, i)
		}
	}
	return selected, destinationLevel
}

func (db *LSM) levelLimit(level int) int64 {
	limit := db.levelBaseBytes
	for current := 1; current < level; current++ {
		if limit > int64(^uint64(0)>>1)/int64(db.levelSizeMultiplier) {
			return int64(^uint64(0) >> 1)
		}
		limit *= int64(db.levelSizeMultiplier)
	}
	return limit
}

func sstableRange(table SSTable) (uint32, uint32) {
	index := table.table.metadata.index
	return index[0].firstKey, index[len(index)-1].lastKey
}

func sortSSTablesByLevel(tables []SSTable) {
	sort.SliceStable(tables, func(i, j int) bool {
		if tables[i].Level != tables[j].Level {
			return tables[i].Level < tables[j].Level
		}
		if tables[i].Level == 0 {
			return false
		}
		firstI, _ := sstableRange(tables[i])
		firstJ, _ := sstableRange(tables[j])
		return firstI < firstJ
	})
}

func cleanupOrphanSSTables(dir string, tables []SSTable) {
	referenced := make(map[string]struct{}, len(tables))
	for _, table := range tables {
		path, err := filepath.Abs(table.Path)
		if err == nil {
			referenced[filepath.Clean(path)] = struct{}{}
		}
	}
	directory, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range directory {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, "sstable-") || !strings.HasSuffix(name, ".sst") {
			continue
		}
		path, err := filepath.Abs(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		if _, ok := referenced[filepath.Clean(path)]; !ok {
			_ = os.Remove(filepath.Join(dir, name))
		}
	}
}

func closeAndRemoveSSTables(tables []SSTable) {
	for _, table := range tables {
		table.table.close()
		_ = os.Remove(table.Path)
	}
}

func (db *LSM) nextSSTablePathLocked() string {
	path := filepath.Join(db.dir, fmt.Sprintf("sstable-%020d.sst", db.nextSSTableID))
	db.nextSSTableID++
	return path
}

func (db *LSM) scheduleCompactionLocked() {
	select {
	case db.compactionQueue <- struct{}{}:
	default:
	}
}

func (db *LSM) writeManifestLocked() error {
	m := manifest{
		SSTables: make([]string, 0, len(db.sstables)),
		Levels:   make([]int, 0, len(db.sstables)),
		NextID:   db.nextSSTableID,
	}
	for _, s := range db.sstables {
		m.SSTables = append(m.SSTables, s.Path)
		m.Levels = append(m.Levels, s.Level)
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

func readManifest(path string) (manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return manifest{SSTables: []string{}}, nil
		}
		return manifest{}, err
	}
	if len(data) >= len(manifestMagic) && bytes.Equal(data[:len(manifestMagic)], manifestMagic[:]) {
		return decodeBinaryManifest(data)
	}
	var m manifest
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&m); err != nil {
		return manifest{}, err
	}
	if err := writeManifest(path, m); err != nil {
		return manifest{}, fmt.Errorf("migrate legacy manifest: %w", err)
	}
	return m, nil
}

func readLegacySSTable(path string) ([]Entry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	decoder := gob.NewDecoder(file)
	entries := make([]Entry, 0, 128)
	for {
		var entry Entry
		if err := decoder.Decode(&entry); err != nil {
			if errors.Is(err, io.EOF) && len(entries) > 0 {
				return entries, nil
			}
			return nil, err
		}
		entries = append(entries, entry)
	}
}
func writeManifest(path string, m manifest) error {
	if uint64(len(m.SSTables)) > uint64(^uint32(0)) || len(m.SSTables) != len(m.Levels) && len(m.Levels) != 0 {
		return errors.New("invalid manifest table metadata")
	}
	encoded := make([]byte, manifestHeaderSize, manifestHeaderSize+manifestTrailerSize+len(m.SSTables)*64)
	copy(encoded[:8], manifestMagic[:])
	binary.LittleEndian.PutUint16(encoded[8:10], manifestVersion)
	binary.LittleEndian.PutUint32(encoded[10:14], uint32(len(m.SSTables)))
	binary.LittleEndian.PutUint64(encoded[14:22], m.NextID)
	for i, path := range m.SSTables {
		if len(path) == 0 || len(path) > int(^uint16(0)) {
			return fmt.Errorf("invalid manifest table path length: %d", len(path))
		}
		level := 0
		if len(m.Levels) != 0 {
			level = m.Levels[i]
		}
		if level < 0 || uint64(level) > uint64(^uint32(0)>>1) {
			return fmt.Errorf("invalid SSTable level: %d", level)
		}
		var entryHeader [6]byte
		binary.LittleEndian.PutUint16(entryHeader[:2], uint16(len(path)))
		binary.LittleEndian.PutUint32(entryHeader[2:6], uint32(level))
		encoded = append(encoded, entryHeader[:]...)
		encoded = append(encoded, path...)
	}
	var checksum [manifestTrailerSize]byte
	binary.LittleEndian.PutUint32(checksum[:], crc32.ChecksumIEEE(encoded))
	encoded = append(encoded, checksum[:]...)

	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(encoded); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	directory, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	err = directory.Sync()
	closeErr := directory.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func decodeBinaryManifest(data []byte) (manifest, error) {
	if len(data) < manifestHeaderSize+manifestTrailerSize {
		return manifest{}, errors.New("manifest is truncated")
	}
	if binary.LittleEndian.Uint16(data[8:10]) != manifestVersion {
		return manifest{}, errors.New("unsupported manifest version")
	}
	checksumOffset := len(data) - manifestTrailerSize
	if crc32.ChecksumIEEE(data[:checksumOffset]) != binary.LittleEndian.Uint32(data[checksumOffset:]) {
		return manifest{}, errors.New("manifest checksum mismatch")
	}
	count := binary.LittleEndian.Uint32(data[10:14])
	if uint64(count) > uint64(checksumOffset-manifestHeaderSize)/7 {
		return manifest{}, errors.New("manifest entry count exceeds encoded data")
	}
	m := manifest{
		SSTables: make([]string, 0, count),
		Levels:   make([]int, 0, count),
		NextID:   binary.LittleEndian.Uint64(data[14:22]),
	}
	position := manifestHeaderSize
	for i := uint32(0); i < count; i++ {
		if position+6 > checksumOffset {
			return manifest{}, errors.New("manifest entry header is truncated")
		}
		pathLength := int(binary.LittleEndian.Uint16(data[position : position+2]))
		level := binary.LittleEndian.Uint32(data[position+2 : position+6])
		position += 6
		if pathLength == 0 || uint64(level) > uint64(^uint(0)>>1) || pathLength > checksumOffset-position {
			return manifest{}, errors.New("invalid manifest entry")
		}
		m.SSTables = append(m.SSTables, string(data[position:position+pathLength]))
		m.Levels = append(m.Levels, int(level))
		position += pathLength
	}
	if position != checksumOffset {
		return manifest{}, errors.New("manifest contains trailing data")
	}
	return m, nil
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
