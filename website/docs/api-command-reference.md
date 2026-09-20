---
id: api-command-reference
title: API and Command Reference
description: Verified commands, service interfaces, and deployment file references.
---

# API and command reference

## Repository commands

These are the current local commands used by the project:

```bash
cd orion-db
make build
make test
make run-ingester
make run-agent
```

## Service endpoints

The ingester exposes the gRPC collector service used by the agent.

Main call:

```proto
rpc SubmitTelemetry(TelemetryPoint) returns (TelemetryPoint);
```

The message contains:

- `metric` name
- numeric `value`
- `timestamp`
- `labels` map

## Environment variables

```bash
ORION_INGEST_CONSUMERS=4
ORION_STORAGE_DIR=data
```

- `ORION_INGEST_CONSUMERS`: number of consumer goroutines draining the ring buffer
- `ORION_STORAGE_DIR`: directory used by the LSM storage engine

## Typical local workflow

```bash
cd orion-db
make build
make run-ingester
make run-agent
```

The ingester listens on `:9090`, and the agent sends high-volume telemetry traffic against that endpoint.

## Benchmarking notes

The current agent supports concurrent workers and a duration flag for synthetic load generation:

```bash
go run ./agent --workers 100 --duration 20s
```

This is intended for throughput and backpressure evaluation under heavy concurrency.

