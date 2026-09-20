---
id: prerequisites
title: Prerequisites
description: Tools required to build and run OrionDB from this repository.
---

# Prerequisites

The following requirements are for the documented local workflows.

## Required tools

- Go 1.24 or newer (required by `orion-db/go.mod`)
- Docker and Docker Compose v2 (required for the Compose build)
- GNU Make (used by primary local workflows)
- Python 3.11 or newer for the local MLOps example
- Node.js 20 or newer and npm for the documentation site

## Optional tools

- `protoc` plus Go protobuf plugins if you plan to regenerate schema code
- `curl` or `grpcurl` for manual endpoint checks

## Verify installed versions

```bash
go version
docker --version
docker compose version
make --version
python --version
node --version
npm --version
```

The Dockerfiles currently use Go 1.22 builder images while the module declares Go 1.24. Use the local Go workflow until those image tags are aligned; see [Troubleshooting](./troubleshooting.md).
