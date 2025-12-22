package scheduler

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/chicogong/dgpu-scheduler/pkg/logger"
	"github.com/chicogong/dgpu-scheduler/pkg/models"
	"go.uber.org/zap"
)

// Engine is the core scheduling engine
type Engine struct {
	state  *StateManager
	logger *logger.Logger
	stopCh chan struct{}
}

// NewEngine creates a new scheduling engine
func NewEngine(state *StateManager, log *logger.Logger) *Engine {
	return &Engine{
		state:  state,
		logger: log,
		stopCh: make(chan struct{}),
	}
}

// Start starts the scheduling loop
func (e *Engine) Start(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-ticker.C:
				e.runSchedulingCycle()
			case <-e.stopCh:
				ticker.Stop()
				return
			}
		}
	}()
}

// Stop stops the scheduling engine
func (e *Engine) Stop() {
	close(e.stopCh)
}

// TriggerSchedule manually triggers a scheduling cycle
func (e *Engine) TriggerSchedule() {
	e.runSchedulingCycle()
}

// runSchedulingCycle executes one scheduling cycle
func (e *Engine) runSchedulingCycle() {
	state := e.state.GetState()

	// Process high priority queue first
	e.processQueue(state.HighPriorityQueue, models.PriorityHigh)

	// Then process low priority queue
	e.processQueue(state.LowPriorityQueue, models.PriorityLow)
}

// processQueue processes tasks in a priority queue
func (e *Engine) processQueue(queue []*models.Task, priority models.Priority) {
	for _, task := range queue {
		if task.Status != models.TaskStatusPending {
			continue
		}

		// Try to schedule the task
		if err := e.scheduleTask(task); err != nil {
			e.logger.Debug("Failed to schedule task",
				zap.String("task_id", task.ID),
				zap.String("priority", string(priority)),
				zap.Error(err),
			)
		}
	}
}

// scheduleTask attempts to schedule a single task
func (e *Engine) scheduleTask(task *models.Task) error {
	state := e.state.GetState()

	// Step 1: Check quota
	if !e.checkQuota(task, state.Quota) {
		return fmt.Errorf("insufficient quota")
	}

	// Step 2: Find available GPUs
	gpus, err := e.findAvailableGPUs(task, state.GPUs)
	if err != nil {
		return err
	}

	// Step 3: Allocate GPUs to task
	if err := e.allocateGPUs(task, gpus); err != nil {
		return err
	}

	e.logger.Info("Task scheduled",
		zap.String("task_id", task.ID),
		zap.String("priority", string(task.Priority)),
		zap.Int("gpu_count", task.GPUCount),
		zap.Strings("assigned_gpus", task.AssignedGPUs),
	)

	return nil
}

// checkQuota checks if there's sufficient quota for the task
func (e *Engine) checkQuota(task *models.Task, quota *models.Quota) bool {
	switch task.Priority {
	case models.PriorityHigh:
		return quota.OnlineUsed+task.GPUCount <= quota.OnlineQuota
	case models.PriorityLow:
		return quota.BatchUsed+task.GPUCount <= quota.BatchQuota
	default:
		return false
	}
}

// findAvailableGPUs finds available GPUs for a task
func (e *Engine) findAvailableGPUs(task *models.Task, allGPUs map[string]*models.GPU) ([]*models.GPU, error) {
	available := make([]*models.GPU, 0)

	// Filter idle GPUs
	for _, gpu := range allGPUs {
		if gpu.Status != models.GPUStatusIdle {
			continue
		}

		// If task specifies GPU model, check match
		if task.GPUModel != nil && *task.GPUModel != gpu.Model {
			continue
		}

		available = append(available, gpu)
	}

	// Check if we have enough GPUs
	if len(available) < task.GPUCount {
		return nil, fmt.Errorf("insufficient GPUs: need %d, have %d", task.GPUCount, len(available))
	}

	// Select GPUs (simple random selection)
	selected := e.selectGPUs(available, task.GPUCount)
	return selected, nil
}

// selectGPUs selects N GPUs from available pool
func (e *Engine) selectGPUs(available []*models.GPU, count int) []*models.GPU {
	// Shuffle for random selection (Go 1.20+ auto-seeds global RNG)
	rand.Shuffle(len(available), func(i, j int) {
		available[i], available[j] = available[j], available[i]
	})

	return available[:count]
}

// allocateGPUs allocates GPUs to a task
func (e *Engine) allocateGPUs(task *models.Task, gpus []*models.GPU) error {
	state := e.state.GetState()
	state.mu.Lock()
	defer state.mu.Unlock()

	// Update GPU status
	assignedIDs := make([]string, len(gpus))
	for i, gpu := range gpus {
		gpu.Status = models.GPUStatusBusy
		gpu.CurrentTask = &task.ID
		gpu.UpdatedAt = time.Now()
		assignedIDs[i] = gpu.ID
	}

	// Update task
	task.AssignedGPUs = assignedIDs
	task.Status = models.TaskStatusRunning
	now := time.Now()
	task.StartedAt = &now

	// Update quota
	if task.Priority == models.PriorityHigh {
		state.Quota.OnlineUsed += task.GPUCount
	} else {
		state.Quota.BatchUsed += task.GPUCount
	}

	// Increment version
	state.Version++
	state.UpdatedAt = time.Now()

	return nil
}

// ReleaseTask releases resources when a task finishes
func (e *Engine) ReleaseTask(taskID string, status models.TaskStatus, errorMsg *string) error {
	task, err := e.state.GetTask(taskID)
	if err != nil {
		return err
	}

	state := e.state.GetState()
	state.mu.Lock()
	defer state.mu.Unlock()

	// Release GPUs
	for _, gpuID := range task.AssignedGPUs {
		if gpu, exists := state.GPUs[gpuID]; exists {
			gpu.Status = models.GPUStatusIdle
			gpu.CurrentTask = nil
			gpu.UpdatedAt = time.Now()
		}
	}

	// Update quota
	if task.Priority == models.PriorityHigh {
		state.Quota.OnlineUsed -= task.GPUCount
	} else {
		state.Quota.BatchUsed -= task.GPUCount
	}

	// Update task status
	task.Status = status
	now := time.Now()
	task.FinishedAt = &now
	if errorMsg != nil {
		task.Error = errorMsg
	}

	// Increment version
	state.Version++
	state.UpdatedAt = time.Now()

	e.logger.Info("Task released",
		zap.String("task_id", taskID),
		zap.String("status", string(status)),
	)

	// Trigger scheduling to process pending tasks
	go e.TriggerSchedule()

	return nil
}
