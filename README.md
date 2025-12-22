# DGPU Scheduler

<div align="center">

[![Release](https://img.shields.io/github/v/release/chicogong/dgpu-scheduler)](https://github.com/chicogong/dgpu-scheduler/releases)
[![Build Status](https://github.com/chicogong/dgpu-scheduler/workflows/CI%2FCD%20Pipeline/badge.svg)](https://github.com/chicogong/dgpu-scheduler/actions)
[![Go Version](https://img.shields.io/badge/Go-%3E%3D%201.21-blue.svg)](https://golang.org/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Report Card](https://goreportcard.com/badge/github.com/chicogong/dgpu-scheduler)](https://goreportcard.com/report/github.com/chicogong/dgpu-scheduler)

**Distributed, GPU-aware workload scheduler for heterogeneous clusters**

Mixed Workloads • Resource Quotas • High Availability

[简体中文](README_CN.md) | English

</div>

---

## Overview

DGPU Scheduler is a distributed GPU scheduling system designed for medium-scale GPU clusters (50-200 nodes). It provides:

- **Mixed Workload Support**: Online inference services and batch processing tasks
- **Resource Isolation**: Strict quota management with configurable allocation ratios
- **High Availability**: Active-standby scheduler with automatic failover
- **Dual API**: gRPC for internal agents, HTTP REST for external users
- **No External Dependencies**: Self-contained architecture without Redis/etcd requirements

## Architecture

```
┌─────────────────────────────────────────────────┐
│           Users/Services (HTTP REST)             │
└─────────────────┬───────────────────────────────┘
                  │
        ┌─────────▼─────────┐
        │   API Gateway     │
        │  (REST + gRPC)    │
        └─────────┬─────────┘
                  │
    ┌─────────────▼──────────────┐
    │   Scheduler Master (HA)     │
    │   - Scheduling Engine       │
    │   - State Management        │
    │   - Quota Management        │
    └─────────────┬──────────────┘
                  │ gRPC
         ┌────────┼────────┐
         │        │        │
    ┌────▼───┐ ┌─▼────┐ ┌─▼────┐
    │ Agent  │ │Agent │ │Agent │
    │ GPU节点 │ │GPU节点│ │GPU节点│
    └────────┘ └──────┘ └──────┘
```

See [Design Document](docs/plans/2025-12-14-dgpu-scheduler-design.md) for detailed architecture.

## Quick Start

### Prerequisites

- Go 1.19+
- Protocol Buffers compiler (protoc)
- NVIDIA GPU with CUDA support (for agents)

### Build

```bash
# Clone the repository
git clone https://github.com/chicogong/dgpu-scheduler.git
cd dgpu-scheduler

# Install dependencies
make deps

# Generate protobuf code
make proto

# Build binaries
make build
```

### Run Scheduler

```bash
# Edit configuration
vim configs/scheduler.yaml

# Run scheduler master
./bin/scheduler -config configs/scheduler.yaml
```

### Run Agent

```bash
# Edit configuration
vim configs/agent.yaml

# Run agent on GPU node
./bin/agent -config configs/agent.yaml
```

## Project Structure

```
dgpu-scheduler/
├── cmd/                # Application entrypoints
│   ├── scheduler/      # Scheduler master binary
│   └── agent/          # Agent binary
├── pkg/                # Core packages
│   ├── scheduler/      # Scheduler logic
│   ├── agent/          # Agent logic
│   ├── api/            # API Gateway
│   ├── models/         # Data models
│   ├── config/         # Configuration
│   └── logger/         # Logging
├── api/
│   └── proto/          # Protobuf definitions
├── configs/            # Configuration templates
├── docs/               # Documentation
│   └── plans/          # Design documents
├── deployments/        # Deployment files
└── scripts/            # Utility scripts
```

## Configuration

### Scheduler Configuration

See [configs/scheduler.yaml](configs/scheduler.yaml) for configuration options:

- Server addresses (gRPC, HTTP)
- Scheduler role (master/standby)
- Quota percentages
- Replication settings

### Agent Configuration

See [configs/agent.yaml](configs/agent.yaml) for configuration options:

- Scheduler addresses
- GPU detection method
- Task execution method
- Heartbeat intervals

## Development

### Run Tests

```bash
make test
```

### Run Tests with Coverage

```bash
make test-coverage
```

### Format Code

```bash
make fmt
```

### Run Linter

```bash
make lint
```

## API Documentation

### REST API

Submit a task:

```bash
curl -X POST http://localhost:8080/api/v1/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "priority": "high",
    "gpu_count": 2,
    "command": "python train.py"
  }'
```

Query task status:

```bash
curl http://localhost:8080/api/v1/tasks/{task_id}
```

See [Design Document](docs/plans/2025-12-14-dgpu-scheduler-design.md#8-api接口设计) for complete API reference.

## Deployment

### Docker

```bash
# Build Docker images
make docker-build

# Run with Docker Compose
docker-compose up -d
```

### Kubernetes

```bash
# Apply Kubernetes manifests
kubectl apply -f deployments/k8s/
```

## Roadmap

- [x] System design
- [x] Project structure
- [ ] Core scheduler implementation (Week 1-4)
- [ ] High availability features (Week 5-7)
- [ ] Monitoring and observability (Week 8-9)
- [ ] Auto-scaling support (Future)

## Contributing

Contributions are welcome! Please read our contributing guidelines before submitting PRs.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Documentation

- [Design Document](docs/plans/2025-12-14-dgpu-scheduler-design.md)
- API Reference (Coming soon)
- Deployment Guide (Coming soon)
- Troubleshooting Guide (Coming soon)
