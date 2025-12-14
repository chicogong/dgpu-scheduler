# Development Guide

This document provides guidelines for developers working on DGPU Scheduler.

## Development Environment Setup

### Prerequisites

1. **Go 1.19+**
   ```bash
   go version
   ```

2. **Protocol Buffers Compiler**
   ```bash
   # macOS
   brew install protobuf

   # Linux
   apt-get install protobuf-compiler
   ```

3. **Go Protobuf Plugins**
   ```bash
   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
   ```

4. **golangci-lint** (optional, for linting)
   ```bash
   # macOS
   brew install golangci-lint

   # Linux
   curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin
   ```

### Clone and Build

```bash
git clone https://github.com/chicogong/dgpu-scheduler.git
cd dgpu-scheduler

# Download dependencies
make deps

# Generate protobuf code
make proto

# Build binaries
make build
```

## Project Structure

```
dgpu-scheduler/
├── cmd/                    # Application entrypoints
│   ├── scheduler/          # Scheduler master
│   │   └── main.go
│   └── agent/              # Agent
│       └── main.go
├── pkg/                    # Core packages
│   ├── scheduler/          # Scheduler core logic
│   │   ├── engine.go       # Scheduling engine
│   │   ├── state.go        # State manager
│   │   └── replication.go  # Replication manager
│   ├── agent/              # Agent core logic
│   │   ├── gpu.go          # GPU detector
│   │   ├── heartbeat.go    # Heartbeat manager
│   │   └── executor.go     # Task executor
│   ├── api/                # API Gateway
│   │   ├── grpc.go         # gRPC server
│   │   └── rest.go         # REST server
│   ├── models/             # Data models
│   │   └── types.go
│   ├── config/             # Configuration
│   │   └── config.go
│   └── logger/             # Logging
│       └── logger.go
├── api/
│   └── proto/              # Protobuf definitions
│       └── scheduler.proto
├── configs/                # Configuration templates
│   ├── scheduler.yaml
│   └── agent.yaml
├── docs/                   # Documentation
│   ├── plans/              # Design documents
│   └── DEVELOPMENT.md      # This file
├── deployments/            # Deployment files
│   ├── docker/
│   └── k8s/
├── scripts/                # Utility scripts
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

## Development Workflow

### 1. Create a Feature Branch

```bash
git checkout -b feature/your-feature-name
```

### 2. Make Changes

Follow Go best practices:
- Use `gofmt` for formatting
- Write tests for new code
- Add comments for exported functions
- Keep functions small and focused

### 3. Run Tests

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run specific package tests
go test -v ./pkg/scheduler/...
```

### 4. Format and Lint

```bash
# Format code
make fmt

# Run linter
make lint
```

### 5. Commit Changes

Follow conventional commit messages:
```bash
git commit -m "feat: add GPU affinity scheduling"
git commit -m "fix: resolve race condition in state manager"
git commit -m "docs: update API documentation"
```

Commit types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `refactor`: Code refactoring
- `test`: Adding tests
- `chore`: Maintenance tasks

### 6. Push and Create PR

```bash
git push origin feature/your-feature-name
```

Then create a Pull Request on GitHub.

## Code Style Guidelines

### Go Code Style

1. **Follow Standard Go Style**
   - Use `gofmt` for formatting
   - Follow [Effective Go](https://golang.org/doc/effective_go.html)
   - Follow [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)

2. **Naming Conventions**
   - Use `camelCase` for variables and functions
   - Use `PascalCase` for exported names
   - Use descriptive names, avoid abbreviations

3. **Error Handling**
   ```go
   // Good
   if err != nil {
       return fmt.Errorf("failed to process task: %w", err)
   }

   // Bad
   if err != nil {
       panic(err)
   }
   ```

4. **Logging**
   ```go
   // Use structured logging
   log.Info("Task scheduled",
       logger.String("task_id", task.ID),
       logger.Int("gpu_count", task.GPUCount),
   )
   ```

### Protobuf Style

1. Use lowercase with underscores for field names
2. Add comments for all messages and fields
3. Use appropriate data types (int32, int64, string)

## Testing Guidelines

### Unit Tests

1. **Test File Naming**
   - Test files should end with `_test.go`
   - Place tests in the same package as the code

2. **Test Function Naming**
   ```go
   func TestSchedulerEngine_ScheduleTask(t *testing.T) {
       // Test implementation
   }
   ```

3. **Table-Driven Tests**
   ```go
   func TestQuotaCheck(t *testing.T) {
       tests := []struct {
           name     string
           task     *Task
           quota    *Quota
           expected bool
       }{
           {
               name: "sufficient quota",
               task: &Task{Priority: PriorityHigh, GPUCount: 2},
               quota: &Quota{OnlineQuota: 10, OnlineUsed: 5},
               expected: true,
           },
           // More test cases...
       }

       for _, tt := range tests {
           t.Run(tt.name, func(t *testing.T) {
               result := CanScheduleTask(tt.task, tt.quota)
               if result != tt.expected {
                   t.Errorf("expected %v, got %v", tt.expected, result)
               }
           })
       }
   }
   ```

### Integration Tests

Place integration tests in a separate package:
```
pkg/scheduler/
├── engine.go
├── engine_test.go          # Unit tests
└── integration_test.go     # Integration tests
```

## Debugging

### Local Development

1. **Run Scheduler Locally**
   ```bash
   go run cmd/scheduler/main.go -config configs/scheduler.yaml
   ```

2. **Run Agent Locally**
   ```bash
   go run cmd/agent/main.go -config configs/agent.yaml
   ```

3. **Enable Debug Logging**
   ```yaml
   # configs/scheduler.yaml
   logging:
     level: "debug"
   ```

### Using Delve Debugger

```bash
# Install delve
go install github.com/go-delve/delve/cmd/dlv@latest

# Debug scheduler
dlv debug cmd/scheduler/main.go -- -config configs/scheduler.yaml

# Set breakpoint
(dlv) break pkg/scheduler/engine.go:42
(dlv) continue
```

## Building and Deployment

### Build Binaries

```bash
# Build all binaries
make build

# Build specific binary
make scheduler
make agent
```

### Build Docker Images

```bash
make docker-build
```

### Cross-Platform Builds

```bash
# Build for Linux
GOOS=linux GOARCH=amd64 go build -o bin/scheduler-linux ./cmd/scheduler

# Build for ARM
GOOS=linux GOARCH=arm64 go build -o bin/scheduler-arm ./cmd/scheduler
```

## Protobuf Development

### Modify Protobuf Definitions

1. Edit `api/proto/scheduler.proto`
2. Regenerate code:
   ```bash
   make proto
   ```

### Add New Service

```protobuf
service NewService {
  rpc NewMethod(NewRequest) returns (NewResponse);
}

message NewRequest {
  string field = 1;
}

message NewResponse {
  bool success = 1;
}
```

## Common Tasks

### Add New Configuration Option

1. Add field to config struct in `pkg/config/config.go`
2. Add field to YAML template in `configs/scheduler.yaml`
3. Update documentation

### Add New API Endpoint

1. Define protobuf message in `api/proto/scheduler.proto`
2. Implement handler in `pkg/api/grpc.go` or `pkg/api/rest.go`
3. Add tests
4. Update API documentation

### Add New Metric

1. Define metric in `pkg/metrics/metrics.go`
2. Instrument code to collect metric
3. Expose via `/metrics` endpoint

## Resources

- [Go Documentation](https://golang.org/doc/)
- [gRPC Go Tutorial](https://grpc.io/docs/languages/go/)
- [Protocol Buffers Guide](https://developers.google.com/protocol-buffers/docs/proto3)
- [Zap Logger Documentation](https://pkg.go.dev/go.uber.org/zap)

## Getting Help

- Read the [Design Document](plans/2025-12-14-dgpu-scheduler-design.md)
- Check existing issues on GitHub
- Ask questions in GitHub Discussions
