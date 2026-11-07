---
id: troubleshooting
title: Troubleshooting
description: Solutions for common issues when running and querying OrionDB.
---

# Troubleshooting

## Ingestion Errors

### `ResourceExhausted` during heavy load
**Symptom:** The agent logs `ResourceExhausted` errors frequently.
**Cause:** The internal `RingBuffer` is full. The background consumers cannot write to the WAL/disk fast enough to keep up with the incoming network traffic.
**Solution:**
1. Increase the number of consumers by setting the environment variable `ORION_INGEST_CONSUMERS=8` (or higher, depending on your CPU cores).
2. Ensure your Agent implements an exponential backoff strategy instead of instantly retrying.
3. Check your disk I/O. If the WAL sync is blocking on a slow drive, the consumers will stall.

## Query Errors

### `NOT_FOUND` when reading data
**Symptom:** Calling `QueryTelemetry` returns a `NotFound` gRPC status.
**Cause:** 
- The metric has not been ingested yet.
- The `labels` provided in the query do not perfectly match the ingested labels. The metric name and *all* labels must match to generate the correct Series ID.
**Solution:** Verify the exact metric name and labels being emitted by your agent.

## Startup Issues

### Server fails to bind (Address already in use)
**Symptom:** `listen failed: listen tcp :9090: bind: address already in use`
**Cause:** Another process (or a zombie OrionDB instance) is already using port `9090` (gRPC) or `9091` (HTTP).
**Solution:** Find the process and kill it:
```bash
lsof -i :9090
kill -9 <PID>