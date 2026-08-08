package index

import (
	"fmt"
	"sync"
	"testing"
)

func TestIntersectEmptyTags(t *testing.T) {
	ti := NewTagIndex()
	if got := ti.Intersect(); got.GetCardinality() != 0 {
		t.Fatalf("expected empty bitmap for no tags, got cardinality %d", got.GetCardinality())
	}
}

func TestIntersectMissingTagReturnsEmpty(t *testing.T) {
	ti := NewTagIndex()
	ti.Add("service=checkout", 1)

	if got := ti.Intersect("service=checkout", "region=eu-west"); got.GetCardinality() != 0 {
		t.Fatalf("expected empty intersection when a tag is absent, got %v", got.ToArray())
	}
}

func TestIntersectSingleTag(t *testing.T) {
	ti := NewTagIndex()
	ti.Add("service=checkout", 1)
	ti.Add("service=checkout", 5)

	got := ti.Intersect("service=checkout").ToArray()
	want := []uint32{1, 5}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("Intersect single tag = %v, want %v", got, want)
	}
}

func TestIntersectMultipleTags(t *testing.T) {
	ti := NewTagIndex()
	// series 1: checkout + eu-west
	ti.Add("service=checkout", 1)
	ti.Add("region=eu-west", 1)
	// series 2: checkout only
	ti.Add("service=checkout", 2)
	// series 3: eu-west only
	ti.Add("region=eu-west", 3)

	got := ti.Intersect("service=checkout", "region=eu-west").ToArray()
	if len(got) != 1 || got[0] != 1 {
		t.Fatalf("Intersect(checkout, eu-west) = %v, want [1]", got)
	}
}

// TestIntersectDoesNotMutateStoredBitmaps ensures the returned bitmap is a
// clone: intersecting must not shrink the underlying per-tag bitmaps.
func TestIntersectDoesNotMutateStoredBitmaps(t *testing.T) {
	ti := NewTagIndex()
	ti.Add("service=checkout", 1)
	ti.Add("service=checkout", 2)
	ti.Add("region=eu-west", 1)

	_ = ti.Intersect("service=checkout", "region=eu-west")

	if got := ti.Intersect("service=checkout").ToArray(); len(got) != 2 {
		t.Fatalf("stored bitmap was mutated by Intersect: got %v, want [1 2]", got)
	}
}

// TestIntersectConcurrentWithAdd exercises the read path concurrently with the
// write path; run with -race to detect unsafe concurrent bitmap access.
func TestIntersectConcurrentWithAdd(t *testing.T) {
	ti := NewTagIndex()
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10000; i++ {
			ti.Add("service=checkout", uint32(i))
			ti.Add(fmt.Sprintf("region=r%d", i%8), uint32(i))
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10000; i++ {
			_ = ti.Intersect("service=checkout", "region=r0").ToArray()
		}
	}()

	wg.Wait()
}
