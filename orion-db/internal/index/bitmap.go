package index

import (
	"hash/fnv"
	"sync"

	"github.com/RoaringBitmap/roaring"
)

type IndexShard struct {
	mu      sync.RWMutex
	bitmaps map[string]*roaring.Bitmap
}

type TagIndex struct {
	shards []*IndexShard
	mask   uint32
}

func NewTagIndex() *TagIndex {
	shardCount := uint32(16)
	shards := make([]*IndexShard, shardCount)
	for i := range shards {
		shards[i] = &IndexShard{bitmaps: make(map[string]*roaring.Bitmap)}
	}
	return &TagIndex{shards: shards, mask: shardCount - 1}
}

func fnv32a(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

func (ti *TagIndex) getShard(tag string) *IndexShard {
	return ti.shards[fnv32a(tag)&ti.mask]
}

func (ti *TagIndex) Add(tag string, seriesID uint32) {
	shard := ti.getShard(tag)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	bm, ok := shard.bitmaps[tag]
	if !ok {
		bm = roaring.New()
		shard.bitmaps[tag] = bm
	}
	bm.Add(seriesID)
}

func (ti *TagIndex) Intersect(tags ...string) *roaring.Bitmap {
	if len(tags) == 0 {
		return roaring.New()
	}

	// Clone each shard bitmap while holding its read lock. roaring.Bitmap is not
	// safe for concurrent read/write, so we must not retain a reference to the
	// live bitmap once the lock is released (a concurrent Add could mutate it).
	bitmaps := make([]*roaring.Bitmap, 0, len(tags))
	for _, tag := range tags {
		shard := ti.getShard(tag)
		shard.mu.RLock()
		bm := shard.bitmaps[tag]
		if bm == nil {
			shard.mu.RUnlock()
			return roaring.New()
		}
		bitmaps = append(bitmaps, bm.Clone())
		shard.mu.RUnlock()
	}

	result := bitmaps[0]
	for _, bm := range bitmaps[1:] {
		result.And(bm)
	}
	return result
}
