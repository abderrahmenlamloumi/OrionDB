package buffer

import (
	"runtime"
	"sync/atomic"

	"orion-db/internal/ingest"
)

type ringNode struct {
	seq  uint64
	item *ingest.Metric
}

// RingBuffer is a lock-free multi-producer, multi-consumer queue for high-throughput metric passing.
// It uses a sequence-based MPMC algorithm to prevent races between producers advancing head
// and consumers reading slot contents. Atomic operations such as Compare-And-Swap enable
// concurrent access without traditional mutex locks, so stalled threads do not block others.
type RingBuffer struct {
	capacity uint64
	mask     uint64
	_        [56]byte
	head     uint64
	_        [56]byte
	tail     uint64
	_        [56]byte
	nodes    []ringNode
}

// NewRingBuffer initializes an MPMC queue.
// Size must be a nonzero power of two.
func NewRingBuffer(size uint64) *RingBuffer {
	if size == 0 || size&(size-1) != 0 {
		panic("ring buffer size must be a nonzero power of two")
	}

	nodes := make([]ringNode, size)
	for i := uint64(0); i < size; i++ {
		nodes[i].seq = i
	}

	return &RingBuffer{
		capacity: size,
		mask:     size - 1,
		nodes:    nodes,
	}
}

// Enqueue attempts to publish an item.
// It returns false immediately when the queue is full.
func (rb *RingBuffer) Enqueue(item *ingest.Metric) bool {
	if item == nil {
		return false
	}

	for {
		head := atomic.LoadUint64(&rb.head)
		node := &rb.nodes[head&rb.mask]
		seq := atomic.LoadUint64(&node.seq)
		diff := int64(seq - head)

		switch {
		case diff == 0:
			if atomic.CompareAndSwapUint64(&rb.head, head, head+1) {
				node.item = item
				// Publishes node.item to the consumer.
				atomic.StoreUint64(&node.seq, head+1)
				return true
			}
		case diff < 0:
			return false
		default:
			runtime.Gosched()
		}
	}
}

// Dequeue returns nil when the queue is empty.
func (rb *RingBuffer) Dequeue() *ingest.Metric {
	for {
		tail := atomic.LoadUint64(&rb.tail)
		node := &rb.nodes[tail&rb.mask]
		seq := atomic.LoadUint64(&node.seq)
		diff := int64(seq - (tail + 1))

		switch {
		case diff == 0:
			if atomic.CompareAndSwapUint64(&rb.tail, tail, tail+1) {
				item := node.item
				node.item = nil
				// Makes the slot available for the next queue lap.
				atomic.StoreUint64(&node.seq, tail+rb.capacity)
				return item
			}
		case diff < 0:
			return nil
		default:
			runtime.Gosched()
		}
	}
}
