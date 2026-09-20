---
id: home
title: Home
description: Start here to understand and run OrionDB.
slug: /
---

# OrionDB documentation

OrionDB is an experimental Go telemetry ingestion and storage engine. The repository contains a working local pipeline for accepting telemetry over gRPC, buffering it, indexing series labels, and writing values through a WAL-backed LSM-style store.

## Start here

- [Project overview](./project-overview.md): scope, components, and support boundary.
- [Getting started](./getting-started.md): build, test, and run the local pipeline.
- [First working example](./first-working-example.md): send telemetry and observe the response.
- [Configuration](./configuration.md): ports, environment variables, and storage defaults.
- [API and command reference](./api-command-reference.md): protobuf contract and commands.

## Before deploying

This is a research prototype, not a production-ready telemetry backend. There is no authentication, TLS configuration, query API, retention policy, replication, metrics endpoint, or durability acknowledgment contract. The deployment files are development scaffolding and must be reviewed before use outside a local environment.

See [Architecture and concepts](./architecture-concepts.md) for the data path and [Troubleshooting](./troubleshooting.md) for known limitations.

## Documentation source of truth

Procedures in this site are derived from the repository, especially `orion-db/Makefile`, `orion-db/schema/telemetry.proto`, and the service entrypoints. Report discrepancies through the workflow in [Contributing to the documentation](./contributing-docs.md).
