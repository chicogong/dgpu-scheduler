package config

import (
	"os"
	"testing"
)

func TestLoadSchedulerConfig(t *testing.T) {
	// Create a temporary config file
	content := `
server:
  grpc_address: ":9090"
  http_address: ":8080"

scheduler:
  role: "master"
  schedule_interval: 5
  snapshot_interval: 30

replication:
  enabled: true
  peer_address: "standby:9090"
  heartbeat_interval: 2
  heartbeat_timeout: 6

quota:
  online_percent: 0.7
  batch_percent: 0.3

agent:
  heartbeat_timeout: 15

storage:
  snapshot_dir: "/tmp/dgpu-scheduler"
  shared_storage: "/tmp/shared"

logging:
  level: "info"
  format: "json"
  output: "stdout"
`

	tmpFile, err := os.CreateTemp("", "scheduler-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	tmpFile.Close()

	// Test loading config
	cfg, err := LoadSchedulerConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify config values
	if cfg.Server.GRPCAddress != ":9090" {
		t.Errorf("Expected GRPCAddress :9090, got %s", cfg.Server.GRPCAddress)
	}

	if cfg.Scheduler.Role != "master" {
		t.Errorf("Expected Role master, got %s", cfg.Scheduler.Role)
	}

	if cfg.Quota.OnlinePercent != 0.7 {
		t.Errorf("Expected OnlinePercent 0.7, got %f", cfg.Quota.OnlinePercent)
	}
}

func TestLoadAgentConfig(t *testing.T) {
	// Create a temporary config file
	content := `
agent:
  id: ""
  heartbeat_interval: 5

scheduler:
  master_address: "scheduler:9090"
  standby_address: "standby:9090"
  connection_timeout: 10
  retry_interval: 2
  max_retry_interval: 30

gpu:
  detection_method: "nvml"
  health_check_interval: 30

executor:
  execution_method: "process"
  work_dir: "/tmp/tasks"

logging:
  level: "info"
  format: "json"
  output: "stdout"
`

	tmpFile, err := os.CreateTemp("", "agent-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	tmpFile.Close()

	// Test loading config
	cfg, err := LoadAgentConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify config values
	if cfg.Scheduler.MasterAddress != "scheduler:9090" {
		t.Errorf("Expected MasterAddress scheduler:9090, got %s", cfg.Scheduler.MasterAddress)
	}

	if cfg.GPU.DetectionMethod != "nvml" {
		t.Errorf("Expected DetectionMethod nvml, got %s", cfg.GPU.DetectionMethod)
	}

	if cfg.Agent.ID == "" {
		t.Error("Expected Agent ID to be set (hostname)")
	}
}

func TestValidateSchedulerConfig(t *testing.T) {
	tests := []struct {
		name      string
		cfg       *SchedulerConfig
		shouldErr bool
	}{
		{
			name: "valid config",
			cfg: &SchedulerConfig{
				Server: struct {
					GRPCAddress string `yaml:"grpc_address"`
					HTTPAddress string `yaml:"http_address"`
				}{
					GRPCAddress: ":9090",
					HTTPAddress: ":8080",
				},
				Scheduler: struct {
					Role             string `yaml:"role"`
					ScheduleInterval int    `yaml:"schedule_interval"`
					SnapshotInterval int    `yaml:"snapshot_interval"`
				}{
					Role: "master",
				},
				Quota: struct {
					OnlinePercent float64 `yaml:"online_percent"`
					BatchPercent  float64 `yaml:"batch_percent"`
				}{
					OnlinePercent: 0.7,
					BatchPercent:  0.3,
				},
			},
			shouldErr: false,
		},
		{
			name: "invalid role",
			cfg: &SchedulerConfig{
				Server: struct {
					GRPCAddress string `yaml:"grpc_address"`
					HTTPAddress string `yaml:"http_address"`
				}{
					GRPCAddress: ":9090",
					HTTPAddress: ":8080",
				},
				Scheduler: struct {
					Role             string `yaml:"role"`
					ScheduleInterval int    `yaml:"schedule_interval"`
					SnapshotInterval int    `yaml:"snapshot_interval"`
				}{
					Role: "invalid",
				},
				Quota: struct {
					OnlinePercent float64 `yaml:"online_percent"`
					BatchPercent  float64 `yaml:"batch_percent"`
				}{
					OnlinePercent: 0.7,
					BatchPercent:  0.3,
				},
			},
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSchedulerConfig(tt.cfg)
			if (err != nil) != tt.shouldErr {
				t.Errorf("validateSchedulerConfig() error = %v, shouldErr %v", err, tt.shouldErr)
			}
		})
	}
}
