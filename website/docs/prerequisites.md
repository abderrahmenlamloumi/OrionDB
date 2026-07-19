---
id: prerequisites
title: Prerequisites
description: Tools required to build and run OrionDB from this repository.
---

# Prerequisites

The project README and module files imply the following baseline requirements.

## Required tools

- Go 1.22 (from `orion-db/go.mod`)
- Docker and Docker Compose (used by `make build` and `deploy/docker-compose.yml`)
- GNU Make (used by primary local workflows)
- Python 3.11+ recommended for `mlops/` container parity

## Optional but useful

- `protoc` plus Go protobuf plugins if you plan to regenerate schema code

## Verify installed versions

```bash
go version
docker --version
docker compose version
make --version
python --version
```
