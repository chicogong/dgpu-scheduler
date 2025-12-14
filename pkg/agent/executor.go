package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/chicogong/dgpu-scheduler/pkg/logger"
	"github.com/chicogong/dgpu-scheduler/pkg/models"
	"go.uber.org/zap"
)

// TaskExecutor executes tasks on the agent
type TaskExecutor struct {
	method      string
	workDir     string
	logger      *logger.Logger
	runningTasks sync.Map // task_id -> *exec.Cmd
	taskResults chan TaskResult
}

// TaskResult represents the result of a task execution
type TaskResult struct {
	TaskID string
	Status string
	Error  string
}

// NewTaskExecutor creates a new task executor
func NewTaskExecutor(method, workDir string, log *logger.Logger) *TaskExecutor {
	return &TaskExecutor{
		method:      method,
		workDir:     workDir,
		logger:      log,
		taskResults: make(chan TaskResult, 100),
	}
}

// ExecuteTask executes a task
func (e *TaskExecutor) ExecuteTask(ctx context.Context, task *models.Task, gpuIDs []string) error {
	e.logger.Info("Executing task",
		zap.String("task_id", task.ID),
		zap.String("command", task.Command),
		zap.Strings("gpu_ids", gpuIDs),
	)

	// Create work directory if needed
	if err := os.MkdirAll(e.workDir, 0755); err != nil {
		return fmt.Errorf("failed to create work directory: %w", err)
	}

	// Execute based on method
	switch e.method {
	case "process":
		return e.executeAsProcess(ctx, task, gpuIDs)
	case "docker":
		return e.executeAsDocker(ctx, task, gpuIDs)
	default:
		return fmt.Errorf("unsupported execution method: %s", e.method)
	}
}

// executeAsProcess executes task as a local process
func (e *TaskExecutor) executeAsProcess(ctx context.Context, task *models.Task, gpuIDs []string) error {
	// Parse command and arguments
	parts := strings.Fields(task.Command)
	if len(parts) == 0 {
		return fmt.Errorf("empty command")
	}

	command := parts[0]
	args := parts[1:]

	// Create command
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = e.workDir

	// Set environment variables
	cmd.Env = os.Environ()

	// Set CUDA_VISIBLE_DEVICES
	deviceIndices := make([]string, len(gpuIDs))
	for i, gpuID := range gpuIDs {
		// Extract device index from GPU ID (format: "node-X-gpu-Y")
		parts := strings.Split(gpuID, "-")
		if len(parts) >= 4 {
			deviceIndices[i] = parts[3]
		}
	}
	cmd.Env = append(cmd.Env, fmt.Sprintf("CUDA_VISIBLE_DEVICES=%s", strings.Join(deviceIndices, ",")))

	// Add custom environment variables
	for key, value := range task.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}

	// Set stdout/stderr
	logFile := fmt.Sprintf("%s/%s.log", e.workDir, task.ID)
	logWriter, err := os.Create(logFile)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}
	defer logWriter.Close()

	cmd.Stdout = logWriter
	cmd.Stderr = logWriter

	// Store running task
	e.runningTasks.Store(task.ID, cmd)
	defer e.runningTasks.Delete(task.ID)

	// Start task
	if err := cmd.Start(); err != nil {
		e.logger.Error("Failed to start task",
			zap.String("task_id", task.ID),
			zap.Error(err),
		)
		e.taskResults <- TaskResult{
			TaskID: task.ID,
			Status: "failed",
			Error:  err.Error(),
		}
		return err
	}

	e.logger.Info("Task started",
		zap.String("task_id", task.ID),
		zap.Int("pid", cmd.Process.Pid),
	)

	// Wait for task to complete in background
	go func() {
		err := cmd.Wait()

		var status string
		var errorMsg string

		if err != nil {
			status = "failed"
			errorMsg = err.Error()
			e.logger.Error("Task failed",
				zap.String("task_id", task.ID),
				zap.Error(err),
			)
		} else {
			status = "success"
			e.logger.Info("Task completed successfully",
				zap.String("task_id", task.ID),
			)
		}

		e.taskResults <- TaskResult{
			TaskID: task.ID,
			Status: status,
			Error:  errorMsg,
		}
	}()

	return nil
}

// executeAsDocker executes task in a Docker container
func (e *TaskExecutor) executeAsDocker(ctx context.Context, task *models.Task, gpuIDs []string) error {
	// Docker execution is similar to process but with docker run command
	// For now, fall back to process execution
	// TODO: Implement proper Docker execution with GPU support
	e.logger.Warn("Docker execution not fully implemented, using process execution")
	return e.executeAsProcess(ctx, task, gpuIDs)
}

// GetTaskResults returns the task results channel
func (e *TaskExecutor) GetTaskResults() <-chan TaskResult {
	return e.taskResults
}

// StopTask stops a running task
func (e *TaskExecutor) StopTask(taskID string) error {
	val, exists := e.runningTasks.Load(taskID)
	if !exists {
		return fmt.Errorf("task not found: %s", taskID)
	}

	cmd := val.(*exec.Cmd)
	if cmd.Process != nil {
		if err := cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill task: %w", err)
		}
		e.logger.Info("Task stopped", zap.String("task_id", taskID))
	}

	return nil
}

// GetRunningTasks returns the list of running task IDs
func (e *TaskExecutor) GetRunningTasks() []string {
	tasks := make([]string, 0)
	e.runningTasks.Range(func(key, value interface{}) bool {
		tasks = append(tasks, key.(string))
		return true
	})
	return tasks
}
