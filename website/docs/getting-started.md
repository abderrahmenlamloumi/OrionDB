---
id: getting-started
title: Getting Started
description: Quick path to build, test, and run OrionDB locally.
---

# Getting started

This guide runs the implemented local ingestion path: the Go ingester accepts gRPC requests and the Go agent generates synthetic requests.

## Prerequisites

- Go 1.24 or newer
- `make`
- a checkout of the repository

## Build and test

From the repository root:

```bash
cd orion-db
make build
make test
```

## Run the ingester

```bash
cd orion-db
make run-ingester
```

This starts the gRPC service on TCP port `9090` and creates the default `data/` storage directory.

## Run the load generator

```bash
cd orion-db
make run-agent
```

You can tune concurrency with the standard agent flags:

```bash
go run ./agent --addr 127.0.0.1:9090 --workers 100 --duration 20s
```

## Useful environment variables

```bash
ORION_INGEST_CONSUMERS=4
ORION_STORAGE_DIR=data
```

These tune the worker pool and the LSM storage folder used by the ingester.

## Expected flow

1. the agent emits `TelemetryPoint` messages.
2. the ingester accepts `SubmitTelemetry` and returns the request on success.
3. the request enters the bounded ring buffer.
4. a consumer assigns a series ID and updates label indexes.
5. the value is appended to the WAL and tracked by the LSM storage path.

The successful RPC response means the point was accepted into the in-memory queue. It does not mean the point has been fsynced or replicated.

## Troubleshooting

If the ingester fails to bind to the port, check whether another process is already listening on `:9090`.

If throughput appears to collapse, inspect `ResourceExhausted` counts from the agent. That status means the bounded queue is full. The current implementation buffers WAL writes and syncs them asynchronously; it does not expose a durability acknowledgment to clients.
