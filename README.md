# 🌌 OrionDB

# Overview

**OrionDB** is a high-performance, experimental Go-based telemetry database capable of sustaining ten of thousands of requests per second while exploring the storage architectures that power modern observability platforms.

The project explores the engineering challenges behind large-scale observability systems, including:

- ingesting millions of telemetry events efficiently
- handling extreme metric cardinality
- minimizing garbage collection pressure
- building storage engines optimized for sequential writes
- designing low-latency indexing structures

**Rather than recreating an existing observability platform, OrionDB is designed as a systems engineering project to understand and implement the architectural principles behind modern telemetry infrastructure.**

---

# Why OrionDB?

Modern observability platforms face a fundamental scaling problem:

> **The number of telemetry dimensions grows exponentially faster than the number of machines producing them.**

In ephemeral environments such as Kubernetes and serverless platforms, a single metric can generate thousands of unique tag combinations throughout its lifetime.

```text
service=checkout-api
container_id=8f92ab31
pod_name=checkout-api-7d9f
git_hash=a91f3c
region=eu-west-3
customer_tier=enterprise
```

This high-cardinality explosion causes conventional telemetry pipelines to struggle. Generic time-series databases incur significant memory overhead from indexing raw strings, while naïve Go ingestion services suffer from excessive garbage collection when allocating millions of telemetry objects every second.

OrionDB investigates techniques used in production observability systems to address these bottlenecks.

| Architectural Challenge | System Impact | OrionDB Solution |
| ----------------------- | ------------- | ---------------- |
| **High-throughput ingestion** | CPU thrashing and GC pauses | Allocation-aware pipeline (`sync.Pool`) |
| **Millions of unique labels** | Massive index memory growth | Roaring Bitmap Inverted Index |
| **Cross-thread event passing** | Mutex contention | Lock-free atomic ring buffers |
| **Continuous writes** | Storage amplification | Custom LSM-Tree + Append-only WAL |

## Core Design Decisions

### Zero-Allocation OTLP Ingestion

OrionDB minimizes allocations on the network hot path through:

- reusable object pools (`sync.Pool`)
- zero-copy byte parsing
- lock-free MPMC ring buffers
- controlled object ownership

The objective is predictable latency while minimizing heap allocation pressure on the ingestion hot path via sync.Pool

### Lock-Free MPMC Ring Buffer

Metrics move from gRPC handlers to consumers through the bounded, lock-free MPMC queue. Each slot has a metric pointer and sequence number; producers and consumers claim slots with atomic compare-and-swap operations, then publish or release slots by updating the sequence.

### Roaring Bitmap Inverted Index

Rather than storing millions of tag strings directly, OrionDB converts labels into compressed bitmap indexes.

Queries such as:

```text
service=checkout
AND
region=eu-west
```

become fast bitmap intersections, enabling efficient filtering without scanning every metric.

### Custom LSM-Tree Storage

Instead of relying on generic storage libraries, OrionDB implements:

- append-only Write-Ahead Log (WAL)
- MemTables
- immutable SSTables
- streaming compaction

---

# Architecture

![Alt text](./website/static/img/OrionDbArchitecture.png)

# Deployment & Operations Guide

For user and deployment documentation, see [docs](https://abderrahmenlamloumi.github.io/OrionDB/docs/getting-started).

---

# Engineering Topics Explored

Building OrionDB explores:

- Go runtime behavior
- memory ownership
- garbage collector optimization
- lock-free programming
- bitmap indexing
- storage engine internals
- telemetry architecture

---

# Status

I am building OrionDB as a systems engineering project. My focus is on understanding the architectural trade-offs behind modern observability 
platforms from first principles, rather than simply gluing together established tools.

# Current implementation status

## Ingestion

- [x] gRPC telemetry ingestion endpoint
- [x] metric validation and conversion into internal structs
- [x] buffered ingestion path with queue-based backpressure
- [ ] OpenTelemetry Collector integration
- [ ] Prometheus remote-write compatibility
- [ ] native OTLP payload support beyond the custom gRPC contract

## Pipeline

- [x] Lock-free MPMC ring buffer
  - Reference used for the design: [A simple lock-free ring buffer](https://kmdreko.github.io/posts/20191003/a-simple-lock-free-ring-buffer/)
- [x] ingester → ring buffer → consumer → storage hot path
- [x] consumer workers processing telemetry concurrently

## Index

- [x] Series registry with stable numeric IDs
- [x] tag bitmap index for label-based filtering
- [x] deterministic series identity from metric name + labels

## Storage

Reference used for the design: [How to Build an LSM Tree Storage Engine from Scratch Full Handbook](https://www.freecodecamp.org/news/build-an-lsm-tree-storage-engine-from-scratch-handbook/)

- [x] append-only WAL with framed records
- [x] CRC32-checked record validation
- [x] WAL crash recovery and truncated-tail recovery
- [x] LSM-style memtable tracking
- [x] SSTable flush path
- [x] async WAL sync loop to avoid fsync on the hot path
- [x] background compaction trigger outside the write critical path
- [ ] Bloom filters
- [ ] advanced multi-level compaction strategy


## Processing & MLOps

- [x] consumer pipeline for ingest processing
- [ ] anomaly detection
- [ ] downstream analytics workflows

## Observability

- [x] `pprof` profiling endpoints mounted
- [x] live load stats in the benchmarking agent

## Deployment

- [x] Docker Compose configuration present
- [ ] Terraform / Helm manifests

## Validation & Benchmarks

- [x] load-testing agent for multi-worker traffic generation
- [x] throughput benchmarking under concurrent load
- [x] backpressure behavior via `ResourceExhausted` when the queue fills
- [ ] sustained production-like benchmarking on larger hardware
- [ ] formal resilience validation under long-running stress

---

## License

OrionDB is released under the [MIT License](./LICENSE).
