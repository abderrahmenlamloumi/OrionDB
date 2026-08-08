package ingest

import "testing"

// Run with: go test -bench=. -benchmem
func BenchmarkMetricAllocation(b *testing.B) {
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		m := AcquireMetric()
		m.Name = "cpu_usage"
		m.Value = 99.9

		m.Tags = append(m.Tags, Tag{Key: "env", Value: "prod"})
		m.Tags = append(m.Tags, Tag{Key: "service", Value: "billing"})

		ReleaseMetric(m)
	}
}
