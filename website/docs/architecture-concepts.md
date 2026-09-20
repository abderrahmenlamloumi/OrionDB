---
id: architecture-concepts
title: Architecture and Concepts
description: Component layout and core data path concepts.
---

# Architecture and concepts

## High-level pipeline

The current implementation follows a practical telemetry pipeline:

```text
Agent -> Ingester -> Ring Buffer -> Consumer -> WAL -> Memtable -> SSTable -> Compaction
                          \-> Series registry
                          \-> Tag bitmap index
```

The schema contract in `schema/telemetry.proto` defines the shared telemetry message type.

## Architecture diagram

![OrionDB architecture showing agent, ingester, consumer, and MLOps stages connected in sequence.](/img/OrionDbArchitecture.png)

## gRPC ingestion

The ingester exposes a gRPC endpoint that accepts `TelemetryPoint` payloads. Each request is mapped into an internal metric object, then enqueued into a bounded ring buffer.

This keeps the network hot path lightweight and avoids blocking the gRPC handler on storage IO. The queue is designed to fail fast with `ResourceExhausted` when the downstream workers cannot keep pace, which is a clear and explicit backpressure signal.

## Series identity

The index package assigns stable numeric IDs to metric series. A series key combines the metric name with the labels in sorted key order, so the same metric and labels produce the same ID regardless of map iteration order.

This ensures deterministic identity and makes label-based lookups efficient.

## Bitmap tag indexing

`TagIndex` stores each `key=value` tag as a bitmap of matching series IDs. The index is sharded by hash to reduce lock contention, and the ingester updates the relevant shard as each metric enters the pipeline.

The important design point is that this indexing layer is intentionally separated from durability. The storage engine persists values without requiring a full scan of all series metadata.

## WAL framing and recovery

The WAL is a write-ahead log that stores framed records with:

- 32-bit payload length
- 32-bit CRC32 checksum
- timestamp
- series ID
- float64 value

On startup, the WAL is replayed and invalid or partial trailing frames are truncated. This allows the system to recover from a crash without corrupting the log.

## LSM storage engine

The storage path follows Log-Structured Merge Tree principles:

```text
Write Request
   |
   v
WAL append
   |
   v
Memtable
   |
   +--> flush threshold reached
             |
             v
          SSTable
             |
             v
         Compaction
```

The key optimization is that the write path does not block on every individual fsync. Instead, writes go to the WAL buffer immediately, and a background worker periodically calls `Sync()` so the ingestion path can stay fast under load.

## Internal buffering primitives

### Lock-free MPMC ring buffer

The ingester uses the bounded lock-free MPMC queue in `internal/buffer` to move metrics from gRPC handlers to consumer workers.

Each slot stores a metric pointer and sequence number. Producers and consumers claim slots with atomic compare-and-swap operations, then publish or release them by updating the sequence number.

The queue lifecycle is:

1. `NewRingBuffer` initializes a nonzero power-of-two capacity.
2. `Enqueue` claims a free slot and publishes a metric.
3. `Dequeue` claims a published metric, processes it, and releases the slot.
4. A full queue returns `false` and becomes gRPC `ResourceExhausted`.

The current default capacity is 1024 slots. Consumer count is also configurable via `ORION_INGEST_CONSUMERS`.

## Background sync and compaction

The system deliberately separates the hot path from expensive disk work:

- `Put()` appends to the WAL buffer without waiting for a blocking fsync.
- a background `syncLoop()` periodically flushes the buffer to disk
- flush and compaction are triggered when memtable or SSTable thresholds are reached, but not on every per-event write

This is the main architectural decision that keeps the write path from collapsing under load.
