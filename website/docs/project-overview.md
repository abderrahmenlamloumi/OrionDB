---
id: project-overview
title: Project Overview
description: Purpose, scope, and intended users of OrionDB.
---

# Project overview

OrionDB is a Go-based telemetry ingestion and storage prototype. Its purpose is to explore the trade-offs involved in high-volume writes, high-cardinality labels, bounded queues, and sequential storage. It is useful for local experiments, benchmarks, and systems-engineering study.

## What runs today

The supported end-to-end path is the `ingester` service plus the synthetic `agent` client:

```text
agent
  -> gRPC SubmitTelemetry
  -> bounded ring buffer
  -> ingester consumer workers
  -> series registry and tag bitmap index
  -> WAL-backed LSM storage
```

The `consumer` binary is a separate in-memory example and is not connected to the ingester. The `mlops` directory contains a standalone Python `IsolationForest` example and is not called by the Go service.

## Repository components

| Path | Role |
| --- | --- |
| `orion-db/ingester` | gRPC server, queue, indexing, and storage write path |
| `orion-db/agent` | Concurrent synthetic telemetry load generator |
| `orion-db/consumer` | Standalone bounded in-memory `TSDB` example |
| `orion-db/internal/buffer` | Bounded lock-free MPMC ring buffer |
| `orion-db/internal/index` | Series identity and label bitmap indexes |
| `orion-db/internal/storage` | WAL, memtable, SSTable, and compaction implementation |
| `orion-db/mlops` | Standalone Python anomaly-detection example |
| `orion-db/schema` | Protobuf source and generated Go bindings |
| `orion-db/deploy` | Docker Compose, Helm, and Terraform scaffolding |

## Support boundary

The project does not currently provide authentication, TLS, a query/read API, retention, replication, tenant isolation, schema validation beyond protobuf decoding, or production SLOs. Treat local benchmarks as exploratory measurements rather than capacity guarantees. See [Architecture and concepts](./architecture-concepts.md) and [FAQ](./faq.md).
