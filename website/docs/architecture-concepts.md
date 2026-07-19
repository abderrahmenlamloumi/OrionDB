---
id: architecture-concepts
title: Architecture and Concepts
description: Component layout and core data path concepts.
---

# Architecture and concepts

## High-level pipeline

Current architecture documents and code describe a four-stage flow:

```text
Agent -> Ingester -> Consumer -> MLOps (WIP)
```

The schema contract (`schema/telemetry.proto`) defines shared telemetry message types.

## Architecture diagram

![OrionDB architecture showing agent, ingester, consumer, and MLOps stages connected in sequence.](/img/OrionDbArchitecture.png)

## Core concepts to be implemented

## gRPC ingestion

## Series identity

## Bitmap tag indexing

## WAL framing and recovery

## Internal buffering primitives
