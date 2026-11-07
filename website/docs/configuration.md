---
id: configuration
title: Configuration
description: Environment variables and tuning parameters for OrionDB.
---

# Configuration

OrionDB is designed to be easily configured via environment variables. Since the Ingester and Storage Engine (LSM) run in the same process, all configuration is passed directly to the `ingester` binary.

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `ORION_STORAGE_DIR` | `data` | The directory where the LSM engine writes the WAL (Write-Ahead Log), Memtable backups, and SSTables. |
| `ORION_INGEST_CONSUMERS` | `4` | The number of background worker goroutines that drain the in-memory Ring Buffer and write to the LSM storage. Increase this if you experience frequent `ResourceExhausted` errors on modern multi-core machines. |
| `ORION_HTTP_ADDR` | `:9091` | The port used for Prometheus metrics and profiling (pprof). |

## Internal Hardcoded Limits

For advanced tuning, some constants are currently defined directly in the code (e.g., `internal/storage/lsm.go` and `internal/buffer/ring.go`):

- **Ring Buffer Capacity:** Default `4096` slots.
- **Memtable Max Entries:** Default `1024` entries before flushing to an SSTable.
- **Compaction Threshold:** Default `4` Level-0 SSTables before triggering a compaction to Level 1.

*Note: In future releases, these parameters will be exposed as environment variables or via a YAML configuration file.*