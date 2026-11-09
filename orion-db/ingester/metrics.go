package main

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"orion-db/internal/ingest"
)

type ingestMetrics struct {
	requests          atomic.Uint64
	requestWindow     atomic.Uint64
	requestsPerSecond atomic.Uint64
	samples           atomic.Uint64
	accepted          atomic.Uint64
	dropped           atomic.Uint64
}

func (metrics *ingestMetrics) recordRequest() {
	metrics.requests.Add(1)
	metrics.requestWindow.Add(1)
}

func (metrics *ingestMetrics) rateLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			metrics.requestsPerSecond.Store(metrics.requestWindow.Swap(0))
		}
	}
}

func (s *server) enqueueMetric(metric *ingest.Metric) bool {
	s.metrics.samples.Add(1)
	if !s.ring.Enqueue(metric) {
		s.metrics.dropped.Add(1)
		ingest.ReleaseMetric(metric)
		return false
	}
	s.metrics.accepted.Add(1)
	return true
}

func (s *server) prometheusMetricsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		_, _ = fmt.Fprintf(w, "# HELP orion_ingestion_requests_total Total telemetry ingestion requests.\n# TYPE orion_ingestion_requests_total counter\norion_ingestion_requests_total %d\n", s.metrics.requests.Load())
		_, _ = fmt.Fprintf(w, "# HELP orion_ingestion_requests_per_second Ingestion requests observed in the latest one-second window.\n# TYPE orion_ingestion_requests_per_second gauge\norion_ingestion_requests_per_second %d\n", s.metrics.requestsPerSecond.Load())
		_, _ = fmt.Fprintf(w, "# HELP orion_ingestion_samples_total Total telemetry samples submitted.\n# TYPE orion_ingestion_samples_total counter\norion_ingestion_samples_total %d\n", s.metrics.samples.Load())
		_, _ = fmt.Fprintf(w, "# HELP orion_ingestion_samples_accepted_total Samples accepted by the MPMC queue.\n# TYPE orion_ingestion_samples_accepted_total counter\norion_ingestion_samples_accepted_total %d\n", s.metrics.accepted.Load())
		_, _ = fmt.Fprintf(w, "# HELP orion_ingestion_samples_dropped_total Samples rejected because the MPMC queue was full.\n# TYPE orion_ingestion_samples_dropped_total counter\norion_ingestion_samples_dropped_total %d\n", s.metrics.dropped.Load())
		_, _ = fmt.Fprintf(w, "# HELP orion_ring_buffer_queue_depth Current MPMC queue depth.\n# TYPE orion_ring_buffer_queue_depth gauge\norion_ring_buffer_queue_depth %d\n", s.ring.Depth())
		_, _ = fmt.Fprintf(w, "# HELP orion_ring_buffer_capacity MPMC queue capacity.\n# TYPE orion_ring_buffer_capacity gauge\norion_ring_buffer_capacity %d\n", s.ring.Capacity())
	}
}
