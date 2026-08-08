package ingest

import "sync"

// Tag models a single label attached to a metric.
type Tag struct {
	Key   string
	Value string
}

// Metric represents a single time-series data point.
type Metric struct {
	Name      string
	Value     float64
	Timestamp int64
	Tags      []Tag
}

var metricPool = sync.Pool{
	New: func() any {
		return &Metric{
			Tags: make([]Tag, 0, 16),
		}
	},
}

// AcquireMetric returns an empty reusable metric.
func AcquireMetric() *Metric {
	m := metricPool.Get().(*Metric)
	m.Name = ""
	m.Value = 0
	m.Timestamp = 0
	if m.Tags == nil {
		m.Tags = make([]Tag, 0, 16)
	} else {
		m.Tags = m.Tags[:0]
	}
	return m
}

// ReleaseMetric removes references held by the metric and returns it to the pool.
func ReleaseMetric(m *Metric) {
	if m == nil {
		return
	}
	m.Name = ""
	m.Value = 0
	m.Timestamp = 0
	// Clear string references before retaining the backing array.
	for i := range m.Tags {
		m.Tags[i] = Tag{}
	}
	if cap(m.Tags) > 128 {
		m.Tags = nil
	} else {
		m.Tags = m.Tags[:0]
	}
	metricPool.Put(m)
}
