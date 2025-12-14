package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/chicogong/dgpu-scheduler/pkg/agent"
	"github.com/chicogong/dgpu-scheduler/pkg/config"
	"github.com/chicogong/dgpu-scheduler/pkg/logger"
	"go.uber.org/zap"
)

var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

func main() {
	var (
		configFile  = flag.String("config", "configs/agent.yaml", "Path to config file")
		showVersion = flag.Bool("version", false, "Show version information")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("DGPU Agent\n")
		fmt.Printf("  Version:    %s\n", Version)
		fmt.Printf("  Git Commit: %s\n", GitCommit)
		fmt.Printf("  Build Time: %s\n", BuildTime)
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.LoadAgentConfig(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	log, err := logger.New(logger.Config{
		Level:  cfg.Logging.Level,
		Format: cfg.Logging.Format,
		Output: cfg.Logging.Output,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	log.Info("Starting DGPU Agent",
		zap.String("version", Version),
		zap.String("config", *configFile),
		zap.String("agent_id", cfg.Agent.ID),
	)

	// Initialize GPU detector
	detector := agent.NewGPUDetector(cfg.GPU.DetectionMethod, cfg.Agent.ID)

	// Detect GPUs
	log.Info("Detecting GPUs...")
	gpus, err := detector.DetectGPUs()
	if err != nil {
		log.Fatal("Failed to detect GPUs", zap.Error(err))
	}

	log.Info("GPUs detected",
		zap.Int("count", len(gpus)),
	)
	for _, gpu := range gpus {
		log.Info("GPU found",
			zap.String("id", gpu.ID),
			zap.String("model", gpu.Model),
			zap.Int64("memory_mb", gpu.Memory),
		)
	}

	// Initialize gRPC client
	client := agent.NewClient(
		cfg.Agent.ID,
		cfg.Scheduler.MasterAddress,
		cfg.Scheduler.StandbyAddress,
		log,
	)

	// Connect to scheduler
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Info("Connecting to scheduler...",
		zap.String("master_address", cfg.Scheduler.MasterAddress),
	)

	if err := client.Connect(ctx); err != nil {
		log.Fatal("Failed to connect to scheduler", zap.Error(err))
	}

	// Register agent
	log.Info("Registering agent...")
	if err := client.Register(ctx, gpus); err != nil {
		log.Fatal("Failed to register agent", zap.Error(err))
	}

	log.Info("Agent registered successfully")

	// Start heartbeat
	heartbeatInterval := time.Duration(cfg.Agent.HeartbeatInterval) * time.Second
	log.Info("Starting heartbeat...",
		zap.Duration("interval", heartbeatInterval),
	)

	if err := client.StartHeartbeat(ctx, heartbeatInterval, gpus); err != nil {
		log.Fatal("Failed to start heartbeat", zap.Error(err))
	}

	// Initialize task executor
	executor := agent.NewTaskExecutor(
		cfg.Executor.ExecutionMethod,
		cfg.Executor.WorkDir,
		log,
	)

	// Monitor task results
	go func() {
		for result := range executor.GetTaskResults() {
			log.Info("Task result",
				zap.String("task_id", result.TaskID),
				zap.String("status", result.Status),
				zap.String("error", result.Error),
			)

			// Report to scheduler
			if err := client.ReportTaskFinished(ctx, result.TaskID, result.Status, result.Error); err != nil {
				log.Error("Failed to report task finished",
					zap.String("task_id", result.TaskID),
					zap.Error(err),
				)
			}
		}
	}()

	log.Info("DGPU Agent started successfully")

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Info("Shutting down DGPU Agent...")

	// Graceful shutdown
	cancel()
	client.Stop()

	log.Info("DGPU Agent stopped")
}
