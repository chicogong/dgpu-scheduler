package scheduler

import (
	"os"
	"testing"
	"time"

	"github.com/chicogong/dgpu-scheduler/pkg/logger"
	"github.com/chicogong/dgpu-scheduler/pkg/models"
)

func TestCheckQuota(t *testing.T) {
	log, _ := logger.New(logger.Config{
		Level:  "error",
		Format: "json",
		Output: "stderr",
	})

	stateManager := NewStateManager("/tmp/test-scheduler")
	engine := NewEngine(stateManager, log)

	// Set quota
	stateManager.SetQuota(0.7, 0.3)
	state := stateManager.GetState()
	state.Quota.TotalGPUs = 100
	state.Quota.OnlineQuota = 70
	state.Quota.BatchQuota = 30

	tests := []struct {
		name     string
		task     *models.Task
		quota    *models.Quota
		expected bool
	}{
		{
			name: "sufficient online quota",
			task: &models.Task{
				Priority: models.PriorityHigh,
				GPUCount: 10,
			},
			quota: &models.Quota{
				OnlineQuota: 70,
				OnlineUsed:  50,
			},
			expected: true,
		},
		{
			name: "insufficient online quota",
			task: &models.Task{
				Priority: models.PriorityHigh,
				GPUCount: 30,
			},
			quota: &models.Quota{
				OnlineQuota: 70,
				OnlineUsed:  50,
			},
			expected: false,
		},
		{
			name: "sufficient batch quota",
			task: &models.Task{
				Priority: models.PriorityLow,
				GPUCount: 10,
			},
			quota: &models.Quota{
				BatchQuota: 30,
				BatchUsed:  10,
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.checkQuota(tt.task, tt.quota)
			if result != tt.expected {
				t.Errorf("checkQuota() = %v, want %v", result, tt.expected)
			}
		})
	}

	// Cleanup
	os.RemoveAll("/tmp/test-scheduler")
}

func TestScheduleTask(t *testing.T) {
	log, _ := logger.New(logger.Config{
		Level:  "error",
		Format: "json",
		Output: "stderr",
	})

	stateManager := NewStateManager("/tmp/test-scheduler")
	engine := NewEngine(stateManager, log)

	// Setup: Add some GPUs
	for i := 0; i < 10; i++ {
		gpu := &models.GPU{
			ID:          string(rune('A' + i)),
			DeviceIndex: i,
			Model:       "TestGPU",
			Memory:      16000,
			Status:      models.GPUStatusIdle,
			UpdatedAt:   time.Now(),
		}
		stateManager.AddGPU(gpu)
	}

	// Set quota
	stateManager.SetQuota(0.7, 0.3)

	// Create a task
	task := &models.Task{
		ID:       "task-1",
		Priority: models.PriorityHigh,
		GPUCount: 2,
		Command:  "test command",
		Status:   models.TaskStatusPending,
	}

	// Try to schedule the task
	err := engine.scheduleTask(task)
	if err != nil {
		t.Fatalf("Failed to schedule task: %v", err)
	}

	// Verify task was scheduled
	if task.Status != models.TaskStatusRunning {
		t.Errorf("Expected task status Running, got %s", task.Status)
	}

	if len(task.AssignedGPUs) != 2 {
		t.Errorf("Expected 2 assigned GPUs, got %d", len(task.AssignedGPUs))
	}

	// Verify GPUs are busy
	state := stateManager.GetState()
	busyCount := 0
	for _, gpu := range state.GPUs {
		if gpu.Status == models.GPUStatusBusy {
			busyCount++
		}
	}

	if busyCount != 2 {
		t.Errorf("Expected 2 busy GPUs, got %d", busyCount)
	}

	// Cleanup
	os.RemoveAll("/tmp/test-scheduler")
}

func TestReleaseTask(t *testing.T) {
	log, _ := logger.New(logger.Config{
		Level:  "error",
		Format: "json",
		Output: "stderr",
	})

	stateManager := NewStateManager("/tmp/test-scheduler")
	engine := NewEngine(stateManager, log)

	// Setup: Add GPUs
	for i := 0; i < 5; i++ {
		gpu := &models.GPU{
			ID:          string(rune('A' + i)),
			DeviceIndex: i,
			Model:       "TestGPU",
			Memory:      16000,
			Status:      models.GPUStatusIdle,
			UpdatedAt:   time.Now(),
		}
		stateManager.AddGPU(gpu)
	}

	stateManager.SetQuota(0.7, 0.3)

	// Create and schedule a task
	task := &models.Task{
		ID:       "task-1",
		Priority: models.PriorityHigh,
		GPUCount: 2,
		Command:  "test command",
		Status:   models.TaskStatusPending,
	}

	stateManager.AddTask(task)
	err := engine.scheduleTask(task)
	if err != nil {
		t.Fatalf("Failed to schedule task: %v", err)
	}

	// Release the task
	err = engine.ReleaseTask("task-1", models.TaskStatusSuccess, nil)
	if err != nil {
		t.Fatalf("Failed to release task: %v", err)
	}

	// Verify task status
	releasedTask, _ := stateManager.GetTask("task-1")
	if releasedTask.Status != models.TaskStatusSuccess {
		t.Errorf("Expected task status Success, got %s", releasedTask.Status)
	}

	// Verify GPUs are idle
	state := stateManager.GetState()
	for _, gpu := range state.GPUs {
		if gpu.Status != models.GPUStatusIdle {
			t.Errorf("Expected GPU %s to be idle, got %s", gpu.ID, gpu.Status)
		}
	}

	// Cleanup
	os.RemoveAll("/tmp/test-scheduler")
}
