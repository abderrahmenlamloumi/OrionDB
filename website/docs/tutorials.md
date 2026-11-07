---
id: tutorials
title: Tutorials
description: Step-by-step implementations using OrionDB.
---

# Tutorials

Here are comprehensive tutorials to help you integrate OrionDB into your wider system architecture.

## 1. MLOps: Connecting Python to OrionDB

Machine Learning models often need real-time telemetry (like CPU load, memory usage, or active connections) to make scaling decisions or anomaly detections.

**Goal:** Fetch the latest data point from OrionDB inside a Python script.

*See the detailed setup and code in our [Task-Oriented Guides: Querying Data with Python](task-oriented-guides).*

## 2. Load Testing with the Agent

You can use the built-in OrionDB Agent to benchmark your hardware and understand how the LSM tree behaves under pressure.

**Goal:** Push 10,000 requests per second and observe backpressure.

1. Start the Ingester: `make run-ingester`
2. Start the Agent with high concurrency: `go run ./agent --workers 500 --duration 60s`
3. Observe the CLI output. You will notice `Success Rate` and `Error Rate`. If errors occur, they will likely be `ResourceExhausted`, proving the internal Ring Buffer is successfully shielding the disk from network spikes.