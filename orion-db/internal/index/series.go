package index

import (
	"sort"
	"strings"
	"sync"
)

type SeriesRegistry struct {
	mu   sync.RWMutex
	next uint32
	ids  map[string]uint32
}

func NewSeriesRegistry() *SeriesRegistry {
	return &SeriesRegistry{next: 1, ids: make(map[string]uint32)}
}

func canonicalKey(metric string, labels map[string]string) string {
	if len(labels) == 0 {
		return metric
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString(metric)
	for _, k := range keys {
		b.WriteByte('|')
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(labels[k])
	}
	return b.String()
}

// GetOrCreate returns a stable series ID for the metric+labels combination.
func (sr *SeriesRegistry) GetOrCreate(metric string, labels map[string]string) uint32 {
	key := canonicalKey(metric, labels)

	sr.mu.RLock()
	id, ok := sr.ids[key]
	sr.mu.RUnlock()
	if ok {
		return id
	}

	sr.mu.Lock()
	defer sr.mu.Unlock()
	// double-check
	if id, ok := sr.ids[key]; ok {
		return id
	}
	id = sr.next
	sr.next++
	sr.ids[key] = id
	return id
}
