package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/chicogong/dgpu-scheduler/pkg/api"
	"github.com/chicogong/dgpu-scheduler/pkg/config"
	"github.com/chicogong/dgpu-scheduler/pkg/logger"
	"github.com/chicogong/dgpu-scheduler/pkg/scheduler"
	"go.uber.org/zap"
)

var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

func main() {
	var (
		configFile  = flag.String("config", "configs/scheduler.yaml", "Path to config file")
		showVersion = flag.Bool("version", false, "Show version information")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("DGPU Scheduler\n")
		fmt.Printf("  Version:    %s\n", Version)
		fmt.Printf("  Git Commit: %s\n", GitCommit)
		fmt.Printf("  Build Time: %s\n", BuildTime)
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.LoadSchedulerConfig(*configFile)
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
	defer func() { _ = log.Sync() }()

	log.Info("Starting DGPU Scheduler",
		zap.String("version", Version),
		zap.String("config", *configFile),
		zap.String("role", cfg.Scheduler.Role),
	)

	// Initialize state manager
	stateManager := scheduler.NewStateManager(cfg.Storage.SnapshotDir)

	// Load snapshot if exists
	if err := stateManager.LoadSnapshot(); err != nil {
		log.Warn("Failed to load snapshot, starting with empty state", zap.Error(err))
	}

	// Set initial quota
	stateManager.SetQuota(cfg.Quota.OnlinePercent, cfg.Quota.BatchPercent)

	// Start periodic snapshot
	snapshotInterval := time.Duration(cfg.Scheduler.SnapshotInterval) * time.Second
	stateManager.StartPeriodicSnapshot(snapshotInterval)

	// Initialize scheduling engine
	engine := scheduler.NewEngine(stateManager, log)

	// Start scheduling loop
	scheduleInterval := time.Duration(cfg.Scheduler.ScheduleInterval) * time.Second
	engine.Start(scheduleInterval)

	// Start gRPC server
	isMaster := cfg.Scheduler.Role == "master"
	grpcServer := api.NewGRPCServer(stateManager, engine, log, isMaster)
	if err := grpcServer.Start(cfg.Server.GRPCAddress); err != nil {
		log.Fatal("Failed to start gRPC server", zap.Error(err))
	}

	// Start REST API server
	restServer := api.NewRESTServer(stateManager, engine, log)
	if err := restServer.Start(cfg.Server.HTTPAddress); err != nil {
		log.Fatal("Failed to start REST API server", zap.Error(err))
	}

	log.Info("DGPU Scheduler started successfully",
		zap.String("grpc_address", cfg.Server.GRPCAddress),
		zap.String("http_address", cfg.Server.HTTPAddress),
	)

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Info("Shutting down DGPU Scheduler...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Stop components
	engine.Stop()
	grpcServer.Stop()
	_ = restServer.Stop()
	stateManager.Stop()

	// Wait for shutdown
	<-ctx.Done()

	log.Info("DGPU Scheduler stopped")
}
