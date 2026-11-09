package storage

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"math"
	"os"
	"path/filepath"
)

const (
	sstHeaderSize = 16
	lllllll
	sstTrailerSize     = 56
	sstRecordSize      = 20
	sstIndexEntrySize  = 24
	sstBlockEntries    = 128
	sstFormatVersion   = 1
	sstBloomHashes     = 7
	sstMinimumBloomLen = 8
)

var (
	sstHeaderMagic  = [8]byte{'O', 'R', 'I', 'O', 'N', 'S', 'S', 'T'}
	sstTrailerMagic = [8]byte{'O', 'R', 'I', 'O', 'N', 'E', 'N', 'D'}
)

type sstIndexEntry struct {
	firstKey uint32
	lastKey  uint32
	offset   uint64
	count    uint32
	checksum uint32
}

func writeSSTable(path string, entries []Entry) error {
	if len(entries) == 0 {
		return errors.New("cannot write empty SSTable")
	}
	for i := 1; i < len(entries); i++ {
		if entries[i-1].Key >= entries[i].Key {
			return errors.New("SSTable entries must have unique, sorted keys")
		}
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create SSTable directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".sst-*.tmp")
	if err != nil {
		return fmt.Errorf("create SSTable temporary file: %w", err)
	}
	tmpPath := tmp.Name()
	committed := false
	defer func() {
		_ = tmp.Close()
		if !committed {
			_ = os.Remove(tmpPath)
		}
	}()

	var header [sstHeaderSize]byte
	copy(header[:8], sstHeaderMagic[:])
	binary.LittleEndian.PutUint16(header[8:10], sstFormatVersion)
	binary.LittleEndian.PutUint16(header[10:12], sstBlockEntries)
	if _, err := tmp.Write(header[:]); err != nil {
		return fmt.Errorf("write SSTable header: %w", err)
	}

	index := make([]sstIndexEntry, 0, (len(entries)+sstBlockEntries-1)/sstBlockEntries)
	filter := newBloomFilter(len(entries))
	var offset uint64 = sstHeaderSize
	var block [sstBlockEntries * sstRecordSize]byte
	for start := 0; start < len(entries); start += sstBlockEntries {
		end := start + sstBlockEntries
		if end > len(entries) {
			end = len(entries)
		}
		blockBytes := block[:(end-start)*sstRecordSize]
		for i, entry := range entries[start:end] {
			encodeSSTEntry(blockBytes[i*sstRecordSize:], entry)
			filter.add(entry.Key)
		}
		if _, err := tmp.Write(blockBytes); err != nil {
			return fmt.Errorf("write SSTable data block: %w", err)
		}
		index = append(index, sstIndexEntry{
			firstKey: entries[start].Key,
			lastKey:  entries[end-1].Key,
			offset:   offset,
			count:    uint32(end - start),
			checksum: crc32.ChecksumIEEE(blockBytes),
		})
		offset += uint64(len(blockBytes))
	}

	bloomOffset := offset
	if _, err := tmp.Write(filter.bits); err != nil {
		return fmt.Errorf("write SSTable Bloom filter: %w", err)
	}
	offset += uint64(len(filter.bits))
	indexOffset := offset
	var indexBytes [sstIndexEntrySize]byte
	indexChecksum := crc32.NewIEEE()
	for _, item := range index {
		binary.LittleEndian.PutUint32(indexBytes[0:4], item.firstKey)
		binary.LittleEndian.PutUint32(indexBytes[4:8], item.lastKey)
		binary.LittleEndian.PutUint64(indexBytes[8:16], item.offset)
		binary.LittleEndian.PutUint32(indexBytes[16:20], item.count)
		binary.LittleEndian.PutUint32(indexBytes[20:24], item.checksum)
		if _, err := tmp.Write(indexBytes[:]); err != nil {
			return fmt.Errorf("write SSTable index: %w", err)
		}
		_, _ = indexChecksum.Write(indexBytes[:])
	}

	var trailer [sstTrailerSize]byte
	copy(trailer[:8], sstTrailerMagic[:])
	binary.LittleEndian.PutUint64(trailer[8:16], indexOffset)
	binary.LittleEndian.PutUint32(trailer[16:20], uint32(len(index)))
	binary.LittleEndian.PutUint64(trailer[20:28], bloomOffset)
	binary.LittleEndian.PutUint32(trailer[28:32], uint32(len(filter.bits)))
	trailer[32] = sstBloomHashes
	binary.LittleEndian.PutUint64(trailer[36:44], uint64(len(entries)))
	binary.LittleEndian.PutUint32(trailer[44:48], crc32.ChecksumIEEE(filter.bits))
	binary.LittleEndian.PutUint32(trailer[48:52], indexChecksum.Sum32())
	binary.LittleEndian.PutUint32(trailer[52:56], crc32.ChecksumIEEE(trailer[:52]))
	if _, err := tmp.Write(trailer[:]); err != nil {
		return fmt.Errorf("write SSTable trailer: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync SSTable: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close SSTable: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("publish SSTable: %w", err)
	}
	directory, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("open SSTable directory: %w", err)
	}
	err = directory.Sync()
	closeErr := directory.Close()
	if err != nil {
		return fmt.Errorf("sync SSTable directory: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("close SSTable directory: %w", closeErr)
	}
	committed = true
	return nil
}

func readSSTable(path string) ([]Entry, error) {
	table, err := openSSTable(path)
	if err != nil {
		return nil, err
	}
	defer table.close()
	entries := make([]Entry, 0, table.metadata.entryCount)
	for _, item := range table.metadata.index {
		block := table.data[item.offset : item.offset+uint64(item.count)*sstRecordSize]
		if crc32.ChecksumIEEE(block) != item.checksum {
			return nil, errors.New("SSTable data block checksum mismatch")
		}
		for offset := 0; offset < len(block); offset += sstRecordSize {
			entries = append(entries, decodeSSTEntry(block[offset:]))
		}
	}
	return entries, nil
}

func getFromSSTable(path string, key uint32) (Entry, bool, error) {
	table, err := openSSTable(path)
	if err != nil {
		return Entry{}, false, err
	}
	defer table.close()
	return table.get(key)
}

type sstMetadata struct {
	index      []sstIndexEntry
	filter     bloomFilter
	entryCount int
}

type mappedSSTable struct {
	data         []byte
	closeMapping func()
	metadata     sstMetadata
}

func openSSTable(path string) (*mappedSSTable, error) {
	data, closeMapping, err := mapSSTable(path)
	if err != nil {
		return nil, err
	}
	metadata, err := parseSSTable(data)
	if err != nil {
		closeMapping()
		return nil, err
	}
	return &mappedSSTable{data: data, closeMapping: closeMapping, metadata: metadata}, nil
}

func (table *mappedSSTable) close() {
	if table != nil && table.closeMapping != nil {
		table.closeMapping()
		table.closeMapping = nil
		table.data = nil
	}
}

func (table *mappedSSTable) get(key uint32) (Entry, bool, error) {
	metadata := table.metadata
	if !metadata.filter.mayContain(key) {
		return Entry{}, false, nil
	}

	low, high := 0, len(metadata.index)
	for low < high {
		middle := int(uint(low+high) >> 1)
		if metadata.index[middle].lastKey < key {
			low = middle + 1
		} else {
			high = middle
		}
	}
	if low == len(metadata.index) {
		return Entry{}, false, nil
	}
	item := metadata.index[low]
	if key < item.firstKey {
		return Entry{}, false, nil
	}
	block := table.data[item.offset : item.offset+uint64(item.count)*sstRecordSize]
	if crc32.ChecksumIEEE(block) != item.checksum {
		return Entry{}, false, errors.New("SSTable data block checksum mismatch")
	}
	left, right := 0, int(item.count)
	for left < right {
		middle := int(uint(left+right) >> 1)
		encodedKey := binary.LittleEndian.Uint32(block[middle*sstRecordSize : middle*sstRecordSize+4])
		if encodedKey < key {
			left = middle + 1
		} else {
			right = middle
		}
	}
	if left == int(item.count) {
		return Entry{}, false, nil
	}
	entry := decodeSSTEntry(block[left*sstRecordSize:])
	return entry, entry.Key == key, nil
}

func parseSSTable(data []byte) (sstMetadata, error) {
	if len(data) < sstHeaderSize+sstTrailerSize {
		return sstMetadata{}, errors.New("SSTable is truncated")
	}
	if string(data[:8]) != string(sstHeaderMagic[:]) {
		return sstMetadata{}, errors.New("invalid SSTable header magic")
	}
	if binary.LittleEndian.Uint16(data[8:10]) != sstFormatVersion {
		return sstMetadata{}, errors.New("unsupported SSTable format version")
	}
	if binary.LittleEndian.Uint16(data[10:12]) != sstBlockEntries {
		return sstMetadata{}, errors.New("invalid SSTable block size")
	}

	trailer := data[len(data)-sstTrailerSize:]
	if string(trailer[:8]) != string(sstTrailerMagic[:]) {
		return sstMetadata{}, errors.New("invalid SSTable trailer magic")
	}
	if crc32.ChecksumIEEE(trailer[:52]) != binary.LittleEndian.Uint32(trailer[52:56]) {
		return sstMetadata{}, errors.New("SSTable trailer checksum mismatch")
	}
	indexOffset := binary.LittleEndian.Uint64(trailer[8:16])
	indexCount := binary.LittleEndian.Uint32(trailer[16:20])
	bloomOffset := binary.LittleEndian.Uint64(trailer[20:28])
	bloomLength := binary.LittleEndian.Uint32(trailer[28:32])
	entryCount := binary.LittleEndian.Uint64(trailer[36:44])
	trailerOffset := uint64(len(data) - sstTrailerSize)
	if bloomOffset < sstHeaderSize || bloomOffset > trailerOffset || bloomLength < sstMinimumBloomLen ||
		uint64(bloomLength) > trailerOffset-bloomOffset ||
		bloomOffset+uint64(bloomLength) != indexOffset || indexOffset > trailerOffset ||
		uint64(indexCount) > (trailerOffset-indexOffset)/sstIndexEntrySize ||
		indexOffset+uint64(indexCount)*sstIndexEntrySize != trailerOffset ||
		entryCount == 0 || entryCount > uint64(math.MaxInt) {
		return sstMetadata{}, errors.New("invalid SSTable trailer offsets")
	}
	if trailer[32] == 0 || trailer[32] > 16 {
		return sstMetadata{}, errors.New("invalid SSTable Bloom filter hash count")
	}

	index := make([]sstIndexEntry, int(indexCount))
	indexBytes := data[indexOffset:trailerOffset]
	if crc32.ChecksumIEEE(data[bloomOffset:indexOffset]) != binary.LittleEndian.Uint32(trailer[44:48]) ||
		crc32.ChecksumIEEE(indexBytes) != binary.LittleEndian.Uint32(trailer[48:52]) {
		return sstMetadata{}, errors.New("SSTable metadata checksum mismatch")
	}
	var previousLast uint32
	var indexedEntries uint64
	expectedBlockOffset := uint64(sstHeaderSize)
	for i := range index {
		start := int(indexOffset) + i*sstIndexEntrySize
		encoded := data[start : start+sstIndexEntrySize]
		item := sstIndexEntry{
			firstKey: binary.LittleEndian.Uint32(encoded[0:4]),
			lastKey:  binary.LittleEndian.Uint32(encoded[4:8]),
			offset:   binary.LittleEndian.Uint64(encoded[8:16]),
			count:    binary.LittleEndian.Uint32(encoded[16:20]),
			checksum: binary.LittleEndian.Uint32(encoded[20:24]),
		}
		blockSize := uint64(item.count) * sstRecordSize
		if item.count == 0 || item.count > sstBlockEntries || item.firstKey > item.lastKey ||
			(i > 0 && item.firstKey <= previousLast) || item.offset != expectedBlockOffset ||
			item.offset > bloomOffset || blockSize > bloomOffset-item.offset {
			return sstMetadata{}, errors.New("invalid SSTable index entry")
		}
		index[i] = item
		previousLast = item.lastKey
		indexedEntries += uint64(item.count)
		expectedBlockOffset += blockSize
	}
	if indexedEntries != entryCount || len(index) == 0 || expectedBlockOffset != bloomOffset {
		return sstMetadata{}, errors.New("SSTable entry count mismatch")
	}
	filter := bloomFilter{bits: data[bloomOffset : bloomOffset+uint64(bloomLength)], hashes: trailer[32]}
	return sstMetadata{index: index, filter: filter, entryCount: int(entryCount)}, nil
}

func encodeSSTEntry(dst []byte, entry Entry) {
	binary.LittleEndian.PutUint32(dst[0:4], entry.Key)
	binary.LittleEndian.PutUint64(dst[4:12], uint64(entry.Timestamp))
	binary.LittleEndian.PutUint64(dst[12:20], math.Float64bits(entry.Value))
}

func decodeSSTEntry(src []byte) Entry {
	return Entry{
		Key:       binary.LittleEndian.Uint32(src[0:4]),
		Timestamp: int64(binary.LittleEndian.Uint64(src[4:12])),
		Value:     math.Float64frombits(binary.LittleEndian.Uint64(src[12:20])),
	}
}

type bloomFilter struct {
	bits   []byte
	hashes uint8
}

func newBloomFilter(entries int) bloomFilter {
	length := (entries*10 + 7) / 8
	if length < sstMinimumBloomLen {
		length = sstMinimumBloomLen
	}
	return bloomFilter{bits: make([]byte, length), hashes: sstBloomHashes}
}

func (filter bloomFilter) add(key uint32) {
	first, second := bloomHashes(key)
	bitCount := uint64(len(filter.bits) * 8)
	for i := uint8(0); i < filter.hashes; i++ {
		bit := (first + uint64(i)*second) % bitCount
		filter.bits[bit/8] |= byte(1 << (bit % 8))
	}
}

func (filter bloomFilter) mayContain(key uint32) bool {
	first, second := bloomHashes(key)
	bitCount := uint64(len(filter.bits) * 8)
	for i := uint8(0); i < filter.hashes; i++ {
		bit := (first + uint64(i)*second) % bitCount
		if filter.bits[bit/8]&(1<<(bit%8)) == 0 {
			return false
		}
	}
	return true
}

func bloomHashes(key uint32) (uint64, uint64) {
	value := uint64(key) + 0x9e3779b97f4a7c15
	value = (value ^ (value >> 30)) * 0xbf58476d1ce4e5b9
	value = (value ^ (value >> 27)) * 0x94d049bb133111eb
	first := value ^ (value >> 31)
	second := first>>17 | first<<47
	return first, second | 1
}
