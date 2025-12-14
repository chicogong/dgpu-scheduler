package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/chicogong/dgpu-scheduler/pkg/logger"
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

	// Initialize logger
	log, err := logger.New(logger.Config{
		Level:  "info",
		Format: "json",
		Output: "stdout",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	log.Info("Starting DGPU Agent",
		logger.String("version", Version),
		logger.String("config", *configFile),
	)

	// TODO: Load configuration
	// TODO: Detect GPUs
	// TODO: Connect to scheduler
	// TODO: Start heartbeat
	// TODO: Wait for shutdown signal

	log.Info("DGPU Agent stopped")
}
