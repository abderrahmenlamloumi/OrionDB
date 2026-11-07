---
id: prerequisites
title: Prerequisites
description: System requirements for building and running OrionDB.
---

# Prerequisites

To build, run, and develop OrionDB, ensure your system has the following tools installed.

## Core Requirements

- **[Go (1.24+)](https://go.dev/dl/):** OrionDB is written entirely in Go.
- **Make:** Used for running build scripts (standard on Linux/macOS. For Windows, use WSL or Mingw).

## Optional Requirements (For Development)

- **[Docker & Docker Compose](https://docs.docker.com/get-docker/):** If you prefer running the pre-packaged containers via `docker-compose`.
- **[gRPC Tooling](https://grpc.io/docs/languages/go/quickstart/):** `protoc`, `protoc-gen-go`, and `protoc-gen-go-grpc` are required only if you modify `schema/telemetry.proto`.
- **[grpcurl](https://github.com/fullstorydev/grpcurl):** A highly recommended CLI tool for sending test queries to the database.
- **[Node.js (18+)](https://nodejs.org/):** Only required if you intend to modify and build this Docusaurus documentation website locally.