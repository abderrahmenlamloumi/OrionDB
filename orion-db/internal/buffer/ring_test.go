package buffer

// import (
// 	"sync"
// 	"sync/atomic"
// 	"testing"

// 	"orion-db/internal/ingest"
// )

// func TestRingBufferWraparound(t *testing.T) {
// 	ring := NewRingBuffer(2)

// 	for i := 0; i < 10000; i++ {
// 		metric := ingest.AcquireMetric()
// 		metric.Timestamp = int64(i)
// 		request := ingest.NewRequest(metric)

// 		if !ring.Enqueue(request) {
// 			t.Fatalf("enqueue failed at iteration %d", i)
// 		}

// 		result := ring.Dequeue()
// 		if result == nil {
// 			t.Fatalf("dequeue returned nil at iteration %d", i)
// 		}

// 		if result.Metric.Timestamp != int64(i) {
// 			t.Fatalf(
// 				"timestamp=%d expected=%d",
// 				result.Metric.Timestamp,
// 				i,
// 			)
// 		}

// 		ingest.ReleaseMetric(result.Metric)
// 	}
// }

// func TestRingBufferMPMC(t *testing.T) {
// 	const (
// 		producers        = 8
// 		consumers        = 8
// 		itemsPerProducer = 2500
// 		totalItems       = producers * itemsPerProducer
// 	)

// 	ring := NewRingBuffer(1024)

// 	seen := make([]uint32, totalItems)

// 	var producerWG sync.WaitGroup
// 	var consumerWG sync.WaitGroup
// 	var consumed atomic.Uint64

// 	producerWG.Add(producers)
// 	for producer := 0; producer < producers; producer++ {
// 		go func(producerID int) {
// 			defer producerWG.Done()
// 			start := producerID * itemsPerProducer
// 			for i := 0; i < itemsPerProducer; i++ {
// 				id := start + i
// 				metric := ingest.AcquireMetric()
// 				metric.Timestamp = int64(id)
// 				request := ingest.NewRequest(metric)
// 				for !ring.Enqueue(request) {
// 				}
// 			}
// 		}(producer)
// 	}

// 	consumerWG.Add(consumers)
// 	for consumer := 0; consumer < consumers; consumer++ {
// 		go func() {
// 			defer consumerWG.Done()
// 			for {
// 				position := consumed.Load()
// 				if position >= totalItems {
// 					return
// 				}
// 				request := ring.Dequeue()
// 				if request == nil {
// 					continue
// 				}
// 				id := int(request.Metric.Timestamp)
// 				if id < 0 || id >= totalItems {
// 					t.Errorf("invalid item ID: %d", id)
// 				} else {
// 					atomic.AddUint32(&seen[id], 1)
// 				}
// 				ingest.ReleaseMetric(request.Metric)
// 				consumed.Add(1)
// 			}
// 		}()
// 	}

// 	producerWG.Wait()
// 	consumerWG.Wait()

// 	if consumed.Load() != totalItems {
// 		t.Fatalf(
// 			"consumed=%d expected=%d",
// 			consumed.Load(),
// 			totalItems,
// 		)
// 	}

// 	for id, count := range seen {
// 		if count != 1 {
// 			t.Fatalf("item=%d observed=%d times", id, count)
// 		}
// 	}
// }
