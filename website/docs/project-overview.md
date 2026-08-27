---
id: project-overview
title: Project Overview
description: Purpose, scope, and intended users of OrionDB.
---

# Project overview

OrionDB is a telemetry ingestion and storage project written primarily in Go. It is designed as a systems engineering project for studying the architecture trade-offs behind modern observability platforms, especially the interaction between high-volume ingestion, indexing, and durable storage.

The repository is intentionally built around real engineering constraints rather than as a toy demo. The current design emphasizes:

- bounded ingestion pressure
- predictable hot-path behavior
- controlled disk write amplification
- low-latency label indexing
- crash-safe WAL recovery

## Target problem

Modern telemetry systems must absorb a large number of high-cardinality events while keeping the write path stable under concurrency. OrionDB exercises the patterns used to solve this problem at a smaller scale: a ring buffer, in-memory series registry, bitmap-based tags, append-only WAL writes, and an LSM-style storage layer.

## Current implementation

The active pipeline is:

```text
Agent
  -> gRPC SubmitTelemetry
  -> Ring Buffer (bounded)
  -> Consumer worker
  -> Series registry + tag index
  -> WAL append
  -> Memtable
  -> SSTable flush
  -> Compaction
```

This is not a purely theoretical project anymore; the repository contains the working pieces for an end-to-end telemetry pipeline and the documentation reflects the implemented architecture.

For safe usage, rely on implemented code paths and the current command set in [API and command reference](./api-command-reference.md).
