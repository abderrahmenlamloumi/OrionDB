---
id: faq
title: Frequently Asked Questions
description: Common questions about OrionDB's design and usage.
---

# Frequently Asked Questions

### Why does OrionDB use a single process instead of microservices?
While earlier designs separated the ingestion API and the storage consumer over the network, we unified them into a single process. This avoids massive network overhead, serialization costs, and latency. The in-memory Lock-Free Ring Buffer securely bridges the network layer (gRPC handlers) and the storage layer (LSM tree) in RAM.

### How do I query data out of OrionDB?
OrionDB provides a `QueryTelemetry` gRPC endpoint on port `9090`. You pass the exact metric name and a map of labels, and it returns the latest ingested data point by searching the Memtable and SSTables. See the [API Reference](api-command-reference) for examples.

### Is OrionDB data persistent?
Yes. Although data is initially queued in memory (Ring Buffer) and held in a Memtable, the storage engine utilizes a Write-Ahead Log (WAL). If the server crashes, the WAL is replayed on startup to reconstruct the Memtable, ensuring no data loss.

### Can I run OrionDB in a Docker container?
Absolutely. A `docker-compose.yml` file is provided in the repository to spin up the database and the test agent instantly.