package models

import "time"

// GPUStatus represents the status of a GPU
type GPUStatus string

const (
	GPUStatusIdle    GPUStatus = "idle"
	GPUStatusBusy    GPUStatus = "busy"
	GPUStatusOffline GPUStatus = "offline"
)

// GPU represents a GPU resource
type GPU struct {
	ID          string    `json:"id"`
	NodeID      string    `json:"node_id"`
	DeviceIndex int       `json:"device_index"`
	Model       string    `json:"model"`
	Memory      int64     `json:"memory"`
	Status      GPUStatus `json:"status"`
	CurrentTask *string   `json:"current_task,omitempty"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Priority represents task priority
type Priority string

const (
	PriorityHigh Priority = "high"
	PriorityLow  Priority = "low"
)

// TaskStatus represents the status of a task
type TaskStatus string

const (
	TaskStatusPending TaskStatus = "pending"
	TaskStatusRunning TaskStatus = "running"
	TaskStatusSuccess TaskStatus = "success"
	TaskStatusFailed  TaskStatus = "failed"
)

// Task represents a scheduling task
type Task struct {
	ID           string            `json:"id"`
	Priority     Priority          `json:"priority"`
	GPUCount     int               `json:"gpu_count"`
	GPUModel     *string           `json:"gpu_model,omitempty"`
	Command      string            `json:"command"`
	Env          map[string]string `json:"env,omitempty"`
	Status       TaskStatus        `json:"status"`
	AssignedGPUs []string          `json:"assigned_gpus,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	StartedAt    *time.Time        `json:"started_at,omitempty"`
	FinishedAt   *time.Time        `json:"finished_at,omitempty"`
	Error        *string           `json:"error,omitempty"`
}

// AgentStatus represents the status of an agent
type AgentStatus string

const (
	AgentStatusOnline  AgentStatus = "online"
	AgentStatusOffline AgentStatus = "offline"
)

// Agent represents a GPU node agent
type Agent struct {
	ID            string      `json:"id"`
	Address       string      `json:"address"`
	GPUs          []GPU       `json:"gpus"`
	LastHeartbeat time.Time   `json:"last_heartbeat"`
	Status        AgentStatus `json:"status"`
}

// Quota represents resource quota configuration
type Quota struct {
	TotalGPUs   int `json:"total_gpus"`
	OnlineQuota int `json:"online_quota"`
	BatchQuota  int `json:"batch_quota"`
	OnlineUsed  int `json:"online_used"`
	BatchUsed   int `json:"batch_used"`
}
