package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// SchedulerConfig represents the scheduler configuration
type SchedulerConfig struct {
	Server struct {
		GRPCAddress string `yaml:"grpc_address"`
		HTTPAddress string `yaml:"http_address"`
	} `yaml:"server"`

	Scheduler struct {
		Role             string `yaml:"role"`
		ScheduleInterval int    `yaml:"schedule_interval"`
		SnapshotInterval int    `yaml:"snapshot_interval"`
	} `yaml:"scheduler"`

	Replication struct {
		Enabled          bool   `yaml:"enabled"`
		PeerAddress      string `yaml:"peer_address"`
		HeartbeatInterval int   `yaml:"heartbeat_interval"`
		HeartbeatTimeout int   `yaml:"heartbeat_timeout"`
	} `yaml:"replication"`

	Quota struct {
		OnlinePercent float64 `yaml:"online_percent"`
		BatchPercent  float64 `yaml:"batch_percent"`
	} `yaml:"quota"`

	Agent struct {
		HeartbeatTimeout int `yaml:"heartbeat_timeout"`
	} `yaml:"agent"`

	Storage struct {
		SnapshotDir   string `yaml:"snapshot_dir"`
		SharedStorage string `yaml:"shared_storage"`
	} `yaml:"storage"`

	Logging struct {
		Level  string `yaml:"level"`
		Format string `yaml:"format"`
		Output string `yaml:"output"`
	} `yaml:"logging"`
}

// AgentConfig represents the agent configuration
type AgentConfig struct {
	Agent struct {
		ID                string `yaml:"id"`
		HeartbeatInterval int    `yaml:"heartbeat_interval"`
	} `yaml:"agent"`

	Scheduler struct {
		MasterAddress     string `yaml:"master_address"`
		StandbyAddress    string `yaml:"standby_address"`
		ConnectionTimeout int    `yaml:"connection_timeout"`
		RetryInterval     int    `yaml:"retry_interval"`
		MaxRetryInterval  int    `yaml:"max_retry_interval"`
	} `yaml:"scheduler"`

	GPU struct {
		DetectionMethod      string `yaml:"detection_method"`
		HealthCheckInterval int    `yaml:"health_check_interval"`
	} `yaml:"gpu"`

	Executor struct {
		ExecutionMethod string `yaml:"execution_method"`
		WorkDir         string `yaml:"work_dir"`
		Docker          struct {
			Socket       string `yaml:"socket"`
			DefaultImage string `yaml:"default_image"`
		} `yaml:"docker"`
	} `yaml:"executor"`

	Logging struct {
		Level  string `yaml:"level"`
		Format string `yaml:"format"`
		Output string `yaml:"output"`
	} `yaml:"logging"`
}

// LoadSchedulerConfig loads scheduler configuration from file
func LoadSchedulerConfig(path string) (*SchedulerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg SchedulerConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Validate configuration
	if err := validateSchedulerConfig(&cfg); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// LoadAgentConfig loads agent configuration from file
func LoadAgentConfig(path string) (*AgentConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg AgentConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set default agent ID if not specified
	if cfg.Agent.ID == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return nil, fmt.Errorf("failed to get hostname: %w", err)
		}
		cfg.Agent.ID = hostname
	}

	// Validate configuration
	if err := validateAgentConfig(&cfg); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// validateSchedulerConfig validates scheduler configuration
func validateSchedulerConfig(cfg *SchedulerConfig) error {
	if cfg.Server.GRPCAddress == "" {
		return fmt.Errorf("server.grpc_address is required")
	}
	if cfg.Server.HTTPAddress == "" {
		return fmt.Errorf("server.http_address is required")
	}
	if cfg.Scheduler.Role != "master" && cfg.Scheduler.Role != "standby" {
		return fmt.Errorf("scheduler.role must be 'master' or 'standby'")
	}
	if cfg.Quota.OnlinePercent+cfg.Quota.BatchPercent != 1.0 {
		return fmt.Errorf("quota percentages must sum to 1.0")
	}
	if cfg.Quota.OnlinePercent < 0 || cfg.Quota.OnlinePercent > 1 {
		return fmt.Errorf("quota.online_percent must be between 0 and 1")
	}
	return nil
}

// validateAgentConfig validates agent configuration
func validateAgentConfig(cfg *AgentConfig) error {
	if cfg.Agent.ID == "" {
		return fmt.Errorf("agent.id is required")
	}
	if cfg.Scheduler.MasterAddress == "" {
		return fmt.Errorf("scheduler.master_address is required")
	}
	if cfg.GPU.DetectionMethod != "nvml" && cfg.GPU.DetectionMethod != "nvidia-smi" {
		return fmt.Errorf("gpu.detection_method must be 'nvml' or 'nvidia-smi'")
	}
	if cfg.Executor.ExecutionMethod != "docker" && cfg.Executor.ExecutionMethod != "process" {
		return fmt.Errorf("executor.execution_method must be 'docker' or 'process'")
	}
	return nil
}
