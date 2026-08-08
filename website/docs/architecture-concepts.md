---
id: architecture-concepts
title: Architecture and Concepts
description: Component layout and core data path concepts.
---

# Architecture and concepts

## High-level pipeline

Current architecture documents and code describe a four-stage flow:

```text
Agent -> Ingester -> Consumer -> MLOps
```

The schema contract (`schema/telemetry.proto`) defines shared telemetry message types.

## Architecture diagram

![OrionDB architecture showing agent, ingester, consumer, and MLOps stages connected in sequence.](/img/OrionDbArchitecture.png)

## Core concepts to be implemented

## gRPC ingestion

## Series identity

The index package in `orion-db/internal/index` assigns stable numeric IDs to metric series. A series key combines the metric name with its labels in sorted key order, so the same metric and labels produce the same ID regardless of map iteration order. IDs are created lazily and protected by a read/write mutex.

## Bitmap tag indexing

`TagIndex` stores each `key=value` tag as a Roaring Bitmap of matching series IDs. The index uses 16 FNV-hash shards, each with its own lock, so independent tag updates can proceed concurrently. Queries clone the relevant bitmaps under read locks and intersect them to return series matching all requested tags. A missing tag or an empty query returns an empty bitmap.

The ingester creates the series ID first, adds every metric tag to the bitmap index, and then appends the value to the WAL. This keeps label filtering separate from the series registry and avoids scanning every series during a tag query.

## WAL framing and recovery

## Internal buffering primitives

### Lock-free MPMC ring buffer

The ingester uses the bounded lock-free MPMC queue in `orion-db/internal/buffer/ring.go` to move metrics from gRPC handlers to consumer workers:

```text
Telemetry request
	  |
	  v
  Enqueue  --->  bounded ring buffer  --->  Dequeue
                                    |
                                    v
                        series index + tag index
                                    |
                                    v
                                    WAL
```

Each slot stores a metric pointer and sequence number. Producers and consumers claim slots with atomic compare-and-swap operations, then publish or release them by updating the sequence number. This keeps the queue bounded and avoids mutex contention.

The queue lifecycle is:

1. `NewRingBuffer` initializes a nonzero power-of-two capacity.
2. `Enqueue` claims a free slot and publishes the metric.
3. `Dequeue` claims a published metric, processes it through indexing and WAL append, then releases the slot.
4. A full queue returns `false` and becomes gRPC `ResourceExhausted`; an empty queue returns `nil`.

The current capacity is 1024 slots. Consumer count is configurable with `ORION_INGEST_CONSUMERS` and defaults to one.
