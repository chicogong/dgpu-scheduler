package scheduler

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/chicogong/dgpu-scheduler/pkg/models"
)

// State represents the scheduler's global state
type State struct {
	mu sync.RWMutex

	// GPU resources
	GPUs map[string]*models.GPU // GPU ID -> GPU

	// Task queues
	HighPriorityQueue []*models.Task
	LowPriorityQueue  []*models.Task

	// All tasks (for tracking)
	Tasks map[string]*models.Task // Task ID -> Task

	// Agents
	Agents map[string]*models.Agent // Agent ID -> Agent

	// Quota
	Quota *models.Quota

	// Metadata
	Version   int64     // State version for replication
	UpdatedAt time.Time // Last update time
}

// StateManager manages the scheduler's global state
type StateManager struct {
	state        *State
	snapshotDir  string
	snapshotChan chan struct{}
	stopChan     chan struct{}
}

// NewStateManager creates a new state manager
func NewStateManager(snapshotDir string) *StateManager {
	return &StateManager{
		state: &State{
			GPUs:              make(map[string]*models.GPU),
			HighPriorityQueue: make([]*models.Task, 0),
			LowPriorityQueue:  make([]*models.Task, 0),
			Tasks:             make(map[string]*models.Task),
			Agents:            make(map[string]*models.Agent),
			Quota: &models.Quota{
				TotalGPUs:   0,
				OnlineQuota: 0,
				BatchQuota:  0,
				OnlineUsed:  0,
				BatchUsed:   0,
			},
			Version:   0,
			UpdatedAt: time.Now(),
		},
		snapshotDir:  snapshotDir,
		snapshotChan: make(chan struct{}, 1),
		stopChan:     make(chan struct{}),
	}
}

// GetState returns a read-only snapshot of the current state
func (sm *StateManager) GetState() *State {
	sm.state.mu.RLock()
	defer sm.state.mu.RUnlock()

	// Return a copy to prevent external modifications
	return sm.state
}

// AddGPU adds a GPU to the state
func (sm *StateManager) AddGPU(gpu *models.GPU) {
	sm.state.mu.Lock()
	defer sm.state.mu.Unlock()

	sm.state.GPUs[gpu.ID] = gpu
	sm.state.TotalGPUs++
	sm.incrementVersion()
	sm.triggerSnapshot()
}

// RemoveGPU removes a GPU from the state
func (sm *StateManager) RemoveGPU(gpuID string) {
	sm.state.mu.Lock()
	defer sm.state.mu.Unlock()

	if _, exists := sm.state.GPUs[gpuID]; exists {
		delete(sm.state.GPUs, gpuID)
		sm.state.Quota.TotalGPUs--
		sm.incrementVersion()
		sm.triggerSnapshot()
	}
}

// UpdateGPUStatus updates GPU status
func (sm *StateManager) UpdateGPUStatus(gpuID string, status models.GPUStatus) error {
	sm.state.mu.Lock()
	defer sm.state.mu.Unlock()

	gpu, exists := sm.state.GPUs[gpuID]
	if !exists {
		return fmt.Errorf("GPU not found: %s", gpuID)
	}

	gpu.Status = status
	gpu.UpdatedAt = time.Now()
	sm.incrementVersion()
	return nil
}

// AddTask adds a task to the appropriate queue
func (sm *StateManager) AddTask(task *models.Task) {
	sm.state.mu.Lock()
	defer sm.state.mu.Unlock()

	sm.state.Tasks[task.ID] = task

	// Add to priority queue
	if task.Priority == models.PriorityHigh {
		sm.state.HighPriorityQueue = append(sm.state.HighPriorityQueue, task)
	} else {
		sm.state.LowPriorityQueue = append(sm.state.LowPriorityQueue, task)
	}

	sm.incrementVersion()
	sm.triggerSnapshot()
}

// GetTask retrieves a task by ID
func (sm *StateManager) GetTask(taskID string) (*models.Task, error) {
	sm.state.mu.RLock()
	defer sm.state.mu.RUnlock()

	task, exists := sm.state.Tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}
	return task, nil
}

// UpdateTaskStatus updates task status
func (sm *StateManager) UpdateTaskStatus(taskID string, status models.TaskStatus) error {
	sm.state.mu.Lock()
	defer sm.state.mu.Unlock()

	task, exists := sm.state.Tasks[taskID]
	if !exists {
		return fmt.Errorf("task not found: %s", taskID)
	}

	task.Status = status
	now := time.Now()

	if status == models.TaskStatusRunning && task.StartedAt == nil {
		task.StartedAt = &now
	} else if (status == models.TaskStatusSuccess || status == models.TaskStatusFailed) && task.FinishedAt == nil {
		task.FinishedAt = &now
	}

	sm.incrementVersion()
	return nil
}

// RegisterAgent registers a new agent
func (sm *StateManager) RegisterAgent(agent *models.Agent) {
	sm.state.mu.Lock()
	defer sm.state.mu.Unlock()

	sm.state.Agents[agent.ID] = agent

	// Add agent's GPUs to the global GPU pool
	for i := range agent.GPUs {
		gpu := &agent.GPUs[i]
		sm.state.GPUs[gpu.ID] = gpu
		sm.state.Quota.TotalGPUs++
	}

	sm.incrementVersion()
	sm.triggerSnapshot()
}

// UpdateAgentHeartbeat updates agent's last heartbeat time
func (sm *StateManager) UpdateAgentHeartbeat(agentID string) error {
	sm.state.mu.Lock()
	defer sm.state.mu.Unlock()

	agent, exists := sm.state.Agents[agentID]
	if !exists {
		return fmt.Errorf("agent not found: %s", agentID)
	}

	agent.LastHeartbeat = time.Now()
	agent.Status = models.AgentStatusOnline
	sm.incrementVersion()
	return nil
}

// SetQuota sets the quota configuration
func (sm *StateManager) SetQuota(onlinePercent, batchPercent float64) {
	sm.state.mu.Lock()
	defer sm.state.mu.Unlock()

	total := sm.state.Quota.TotalGPUs
	sm.state.Quota.OnlineQuota = int(float64(total) * onlinePercent)
	sm.state.Quota.BatchQuota = int(float64(total) * batchPercent)

	sm.incrementVersion()
	sm.triggerSnapshot()
}

// incrementVersion increments the state version (must hold lock)
func (sm *StateManager) incrementVersion() {
	sm.state.Version++
	sm.state.UpdatedAt = time.Now()
}

// triggerSnapshot triggers a snapshot save
func (sm *StateManager) triggerSnapshot() {
	select {
	case sm.snapshotChan <- struct{}{}:
	default:
		// Channel full, snapshot already pending
	}
}

// SaveSnapshot saves the current state to disk
func (sm *StateManager) SaveSnapshot() error {
	sm.state.mu.RLock()
	defer sm.state.mu.RUnlock()

	if err := os.MkdirAll(sm.snapshotDir, 0755); err != nil {
		return fmt.Errorf("failed to create snapshot directory: %w", err)
	}

	snapshotFile := filepath.Join(sm.snapshotDir, "state.json")
	tempFile := snapshotFile + ".tmp"

	data, err := json.MarshalIndent(sm.state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write snapshot: %w", err)
	}

	if err := os.Rename(tempFile, snapshotFile); err != nil {
		return fmt.Errorf("failed to rename snapshot: %w", err)
	}

	return nil
}

// LoadSnapshot loads state from disk
func (sm *StateManager) LoadSnapshot() error {
	snapshotFile := filepath.Join(sm.snapshotDir, "state.json")

	data, err := os.ReadFile(snapshotFile)
	if err != nil {
		if os.IsNotExist(err) {
			// No snapshot exists, start with empty state
			return nil
		}
		return fmt.Errorf("failed to read snapshot: %w", err)
	}

	sm.state.mu.Lock()
	defer sm.state.mu.Unlock()

	if err := json.Unmarshal(data, sm.state); err != nil {
		return fmt.Errorf("failed to unmarshal state: %w", err)
	}

	return nil
}

// StartPeriodicSnapshot starts periodic snapshot saving
func (sm *StateManager) StartPeriodicSnapshot(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-ticker.C:
				if err := sm.SaveSnapshot(); err != nil {
					// Log error (would use logger in production)
					fmt.Fprintf(os.Stderr, "Failed to save snapshot: %v\n", err)
				}
			case <-sm.snapshotChan:
				if err := sm.SaveSnapshot(); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to save snapshot: %v\n", err)
				}
			case <-sm.stopChan:
				ticker.Stop()
				return
			}
		}
	}()
}

// Stop stops the state manager
func (sm *StateManager) Stop() {
	close(sm.stopChan)
	// Save final snapshot
	if err := sm.SaveSnapshot(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save final snapshot: %v\n", err)
	}
}
