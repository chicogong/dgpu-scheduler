.PHONY: all build clean test proto scheduler agent docker-build docker-push help deps fmt lint

# Variables
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

BINARY_SCHEDULER=bin/scheduler
BINARY_AGENT=bin/agent

# Git info
GIT_COMMIT=$(shell git rev-parse --short HEAD)
GIT_TAG=$(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')

# Build flags
LDFLAGS=-ldflags "-X main.Version=$(GIT_TAG) -X main.GitCommit=$(GIT_COMMIT) -X main.BuildTime=$(BUILD_TIME)"

all: test build

## build: Build scheduler and agent binaries
build: scheduler agent

## scheduler: Build scheduler binary
scheduler:
	@echo "Building scheduler..."
	@mkdir -p bin
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_SCHEDULER) ./cmd/scheduler

## agent: Build agent binary
agent:
	@echo "Building agent..."
	@mkdir -p bin
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_AGENT) ./cmd/agent

## test: Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...

## test-coverage: Run tests with coverage report
test-coverage: test
	@echo "Generating coverage report..."
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## clean: Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf bin/
	rm -f coverage.out coverage.html

## proto: Generate protobuf code
proto:
	@echo "Generating protobuf code..."
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		api/proto/*.proto

## deps: Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

## fmt: Format code
fmt:
	@echo "Formatting code..."
	$(GOCMD) fmt ./...

## lint: Run linter
lint:
	@echo "Running linter..."
	golangci-lint run ./...

## docker-build: Build Docker images
docker-build:
	@echo "Building Docker images..."
	docker build -t dgpu-scheduler:$(GIT_TAG) -f deployments/docker/Dockerfile.scheduler .
	docker build -t dgpu-agent:$(GIT_TAG) -f deployments/docker/Dockerfile.agent .

## docker-push: Push Docker images
docker-push:
	@echo "Pushing Docker images..."
	docker push dgpu-scheduler:$(GIT_TAG)
	docker push dgpu-agent:$(GIT_TAG)

## run-scheduler: Run scheduler locally
run-scheduler: scheduler
	@echo "Running scheduler..."
	./$(BINARY_SCHEDULER) -config configs/scheduler.yaml

## run-agent: Run agent locally
run-agent: agent
	@echo "Running agent..."
	./$(BINARY_AGENT) -config configs/agent.yaml

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' Makefile | column -t -s ':' | sed -e 's/^/ /'