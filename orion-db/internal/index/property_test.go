package index

import (
	"math/rand"
	"testing"
	"testing/quick"
)

func TestTagIndexPropertyIntersections(t *testing.T) {
	err := quick.Check(func(seed int64, operations []byte) bool {
		index := NewTagIndex()
		model := make(map[string]map[uint32]struct{})
		random := rand.New(rand.NewSource(seed))
		tags := []string{"region=west", "region=east", "env=prod", "env=stage", "service=api"}
		for _, operation := range operations {
			seriesID := uint32(random.Intn(128))
			tag := tags[int(operation)%len(tags)]
			index.Add(tag, seriesID)
			if model[tag] == nil {
				model[tag] = make(map[uint32]struct{})
			}
			model[tag][seriesID] = struct{}{}
		}
		for _, firstTag := range tags {
			for _, secondTag := range tags {
				actual := index.Intersect(firstTag, secondTag)
				for seriesID := uint32(0); seriesID < 128; seriesID++ {
					_, inFirst := model[firstTag][seriesID]
					_, inSecond := model[secondTag][seriesID]
					if actual.Contains(seriesID) != (inFirst && inSecond) {
						return false
					}
				}
			}
		}
		return true
	}, &quick.Config{MaxCount: 100})
	if err != nil {
		t.Fatalf("tag index model property failed: %v", err)
	}
}

func BenchmarkTagIndexBitmapIntersection(b *testing.B) {
	index := NewTagIndex()
	for seriesID := uint32(0); seriesID < 100_000; seriesID++ {
		index.Add("region=west", seriesID)
		if seriesID%10 != 0 {
			index.Add("service=api", seriesID)
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = index.Intersect("region=west", "service=api")
	}
}
