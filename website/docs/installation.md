---
id: installation
title: Installation
description: Clone and set up OrionDB for local development.
---

# Installation

## 1. Clone the repository

```bash
git clone https://github.com/abderrahmenlamloumi/OrionDB.git
cd OrionDB/orion-db
```

## 2. Fetch Go module dependencies

```bash
go mod download
```

Expected result:

- Go dependencies in `go.mod` and `go.sum` are available locally.

## Install documentation dependencies

The documentation site is an independent Node.js project:

```bash
cd ../website
npm ci
```

Use `npm ci` in CI and release workflows because it installs the versions recorded in `package-lock.json`. Use `npm install` only when intentionally changing dependencies.

## Verify the checkout

```bash
cd ../orion-db
go test ./...
python -m compileall mlops
cd ../website
npm run typecheck
npm run build
```

The generated site is written to `website/build/`.
