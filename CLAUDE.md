# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

DGPU Scheduler is a distributed GPU scheduling system built in Go for medium-scale GPU clusters (50-200 nodes). It provides mixed workload support (online inference + batch processing), strict quota management, and high availability through active-standby scheduler architecture.

**Architecture**: Three-layer design with API Gateway (REST + gRPC), Scheduler Master (active-standby with in-memory state), and distributed Agents (one per GPU node).

**Communication**:
- gRPC for internal Agent ↔ Scheduler communication (high performance)
- HTTP REST for external user/service API (ease of use)
- Protobuf-based master-standby replication

## Essential Commands

### Build and Development
```bash
# Install dependencies
make deps

# Generate protobuf code (required after modifying .proto files)
make proto

# Build both scheduler and agent binaries
make build

# Build individual components
make scheduler    # Build scheduler binary only
make agent       # Build agent binary only

# Format code before commits
make fmt

# Run linter
make lint
```

### Testing
```bash
# Run all tests with race detection and coverage
make test

# Generate HTML coverage report
make test-coverage

# Run tests for specific package
go test -v ./pkg/scheduler/...
go test -v ./pkg/agent/...

# Run single test function
go test -v -run TestSchedulerEngine_ScheduleTask ./pkg/scheduler/
```

### Running Locally
```bash
# Run scheduler master
make run-scheduler
# Or with custom config:
./bin/scheduler -config test-local/scheduler.yaml

# Run agent
make run-agent
# Or with custom config:
./bin/agent -config test-local/agent.yaml
```

### Docker
```bash
# Build Docker images (tagged with git version)
make docker-build

# Images created:
# - dgpu-scheduler:<version>
# - dgpu-agent:<version>
```

## Code Architecture

### Key Packages

**pkg/models/types.go**: Core data models
- `GPU`: Resource representation with status (idle/busy/offline)
- `Task`: Scheduling task with priority (high/low), status (pending/running/success/failed)
- `Agent`: GPU node with heartbeat tracking
- `Quota`: Resource quota management (online vs batch)

**pkg/scheduler/engine.go**: Scheduling engine
- FIFO + priority queue scheduling algorithm
- Quota checking logic (`CanScheduleTask`)
- GPU allocation (simple round-robin or random selection)
- Event-driven + periodic (5s) scheduling loop

**pkg/scheduler/state.go**: State manager
- In-memory global state (GPUs, tasks, quotas, agents)
- Thread-safe read/write with locks
- Periodic snapshots to local files (every 30s)
- Failure recovery from snapshots

**pkg/agent/**: Agent components
- `gpu.go`: GPU detection via NVML or nvidia-smi
- `executor.go`: Task execution (Docker containers or local processes)
- `client.go`: gRPC client for scheduler communication with automatic failover

**pkg/api/**: API layer
- `grpc_server.go`: gRPC service for agents (RegisterAgent, Heartbeat, TaskFinished)
- `rest_server.go`: HTTP REST API for users (task submission, status queries)

**api/proto/scheduler.proto**: Protocol definitions
- `SchedulerService`: Agent communication
- `ReplicationService`: Master-standby synchronization

### State Management

The scheduler maintains all state in memory for performance:
- GPU pool: Each GPU's ID, model, status, current task
- Task queues: Separate high/low priority FIFO queues
- Quota counters: Online/batch used vs quota limits
- Agent registry: Last heartbeat, GPU list, status

State persistence:
- Periodic snapshots to `snapshot_dir` (default: /var/lib/dgpu-scheduler/state)
- JSON format for human readability
- Used for crash recovery

### Scheduling Flow

1. **Task submission** (REST API) → validate → quota check → enqueue by priority → trigger scheduling
2. **Scheduling decision**:
   - Pick from high priority queue first
   - Check quota availability
   - Find idle GPUs matching requirements
   - Allocate GPUs and update task status
   - Send task to agent via gRPC
3. **Task completion**: Agent reports → release GPUs → update quota → trigger next scheduling round

### High Availability

**Active-standby model**:
- Primary master handles all requests
- Standby replicates state via gRPC stream
- Heartbeat-based failure detection (2s interval, 6s timeout)
- Automatic failover when master fails
- Agents automatically reconnect to new master

**Brain-split prevention**:
- Shared storage liveness marker
- Standby checks shared storage before promotion

## Configuration

### Scheduler Config (configs/scheduler.yaml)
- `scheduler.role`: "master" or "standby"
- `quota.online_percent`: GPU percentage for online services (default: 0.7)
- `quota.batch_percent`: GPU percentage for batch processing (default: 0.3)
- `replication.enabled`: Enable master-standby replication
- `agent.heartbeat_timeout`: Agent offline threshold (default: 15s)

### Agent Config (configs/agent.yaml)
- `gpu.detection_method`: "nvml" (preferred) or "nvidia-smi"
- `executor.execution_method`: "docker" or "process"
- `scheduler.master_address` and `scheduler.standby_address`: Scheduler endpoints
- `agent.heartbeat_interval`: How often to send heartbeat (default: 5s)

## Local Testing

The `test-local/` directory provides configurations for local development:
- `test-local/scheduler.yaml`: Local scheduler config (ports 19090/18080)
- `test-local/agent.yaml`: Local agent config
- `test-local/fake-nvidia-smi.sh`: Mock GPU detection script (4 fake V100 GPUs)

To test without real GPUs, configure agent to use the fake script:
```yaml
gpu:
  detection_method: "nvidia-smi"
```
And set `PATH` to include `test-local/` directory.

## Protobuf Development

After modifying `api/proto/scheduler.proto`:
1. Run `make proto` to regenerate Go code
2. Generated files appear in same directory with `.pb.go` suffix
3. Never manually edit generated files

## Testing Guidelines

**Unit tests**: Test individual functions/methods in same package
- Use table-driven tests for multiple scenarios
- Mock external dependencies (GPU detection, Docker execution)

**Integration tests**: Test component interactions
- Agent ↔ Scheduler communication
- Task full lifecycle (submit → schedule → execute → complete)
- Master-standby failover

## API Endpoints

### REST API (default port 8080)
```
POST   /api/v1/tasks          # Submit task
GET    /api/v1/tasks/{id}     # Query task status
DELETE /api/v1/tasks/{id}     # Cancel task
GET    /api/v1/tasks          # List tasks (with filters)
GET    /api/v1/gpus           # Query GPU resources
GET    /api/v1/quota          # Query quota status
PUT    /api/v1/quota          # Update quota ratio (admin)
```

### gRPC API (default port 9090)
- `RegisterAgent`: Agent registration with GPU info
- `Heartbeat`: Bidirectional streaming for health monitoring and task dispatch
- `TaskFinished`: Task completion notification from agent

## Logging

Structured logging with configurable format (JSON or text):
- Key events: task_submitted, task_scheduled, task_finished, agent_registered, agent_offline
- Include context: task_id, agent_id, gpu_ids, priority
- Output to stdout by default (container-friendly)

## Common Development Patterns

**Adding a new task field**:
1. Update `Task` struct in `pkg/models/types.go`
2. Update protobuf `Task` message in `api/proto/scheduler.proto`
3. Run `make proto`
4. Update REST API request/response handling
5. Update tests

**Modifying scheduling logic**:
- Focus on `pkg/scheduler/engine.go`
- Maintain thread safety (use state manager locks)
- Update quota checks if adding new priority levels
- Test with various quota configurations

**Agent failure handling**:
- Agents detect scheduler failure via heartbeat timeout
- Automatic reconnection with exponential backoff (1s → 30s max)
- Scheduler detects agent failure via 15s heartbeat timeout
- Failed agent's GPUs automatically released

## Important Notes

- Never manually modify state snapshot files - they are auto-generated
- GPU allocation is stateful - releasing GPUs must update both GPU status and quota counters
- All gRPC streaming connections must handle reconnection gracefully
- Task IDs must be globally unique (use timestamp + random for generation)
- Quota percentages must sum to ≤1.0
