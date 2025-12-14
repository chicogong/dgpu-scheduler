package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/chicogong/dgpu-scheduler/pkg/logger"
	"github.com/chicogong/dgpu-scheduler/pkg/models"
	"github.com/chicogong/dgpu-scheduler/pkg/scheduler"
	"go.uber.org/zap"
)

// RESTServer implements the REST API server
type RESTServer struct {
	state  *scheduler.StateManager
	engine *scheduler.Engine
	logger *logger.Logger
	server *http.Server
}

// NewRESTServer creates a new REST API server
func NewRESTServer(
	state *scheduler.StateManager,
	engine *scheduler.Engine,
	log *logger.Logger,
) *RESTServer {
	return &RESTServer{
		state:  state,
		engine: engine,
		logger: log,
	}
}

// Start starts the REST API server
func (s *RESTServer) Start(address string) error {
	mux := http.NewServeMux()

	// Task endpoints
	mux.HandleFunc("/api/v1/tasks", s.handleTasks)
	mux.HandleFunc("/api/v1/tasks/", s.handleTaskByID)

	// GPU endpoints
	mux.HandleFunc("/api/v1/gpus", s.handleGPUs)

	// Quota endpoints
	mux.HandleFunc("/api/v1/quota", s.handleQuota)

	// Health check
	mux.HandleFunc("/health", s.handleHealth)

	s.server = &http.Server{
		Addr:    address,
		Handler: s.corsMiddleware(s.loggingMiddleware(mux)),
	}

	s.logger.Info("REST API server starting", zap.String("address", address))

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("REST API server failed", zap.Error(err))
		}
	}()

	return nil
}

// Stop stops the REST API server
func (s *RESTServer) Stop() error {
	if s.server != nil {
		return s.server.Close()
	}
	return nil
}

// handleTasks handles task creation and listing
func (s *RESTServer) handleTasks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.createTask(w, r)
	case http.MethodGet:
		s.listTasks(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// createTask creates a new task
func (s *RESTServer) createTask(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Priority  string            `json:"priority"`
		GPUCount  int               `json:"gpu_count"`
		GPUModel  *string           `json:"gpu_model,omitempty"`
		Command   string            `json:"command"`
		Env       map[string]string `json:"env,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate request
	if req.Command == "" {
		s.sendError(w, http.StatusBadRequest, "Command is required")
		return
	}
	if req.GPUCount <= 0 {
		s.sendError(w, http.StatusBadRequest, "GPU count must be positive")
		return
	}

	priority := models.Priority(req.Priority)
	if priority != models.PriorityHigh && priority != models.PriorityLow {
		s.sendError(w, http.StatusBadRequest, "Priority must be 'high' or 'low'")
		return
	}

	// Create task
	task := &models.Task{
		ID:       generateTaskID(),
		Priority: priority,
		GPUCount: req.GPUCount,
		GPUModel: req.GPUModel,
		Command:  req.Command,
		Env:      req.Env,
		Status:   models.TaskStatusPending,
		CreatedAt: time.Now(),
	}

	s.state.AddTask(task)

	// Trigger scheduling
	s.engine.TriggerSchedule()

	s.logger.Info("Task created",
		zap.String("task_id", task.ID),
		zap.String("priority", string(task.Priority)),
	)

	s.sendJSON(w, http.StatusCreated, map[string]interface{}{
		"task_id":    task.ID,
		"status":     task.Status,
		"created_at": task.CreatedAt,
	})
}

// listTasks lists all tasks
func (s *RESTServer) listTasks(w http.ResponseWriter, r *http.Request) {
	state := s.state.GetState()
	state.mu.RLock()
	defer state.mu.RUnlock()

	tasks := make([]*models.Task, 0, len(state.Tasks))
	for _, task := range state.Tasks {
		tasks = append(tasks, task)
	}

	s.sendJSON(w, http.StatusOK, map[string]interface{}{
		"tasks": tasks,
		"total": len(tasks),
	})
}

// handleTaskByID handles task operations by ID
func (s *RESTServer) handleTaskByID(w http.ResponseWriter, r *http.Request) {
	// Extract task ID from path
	taskID := r.URL.Path[len("/api/v1/tasks/"):]
	if taskID == "" {
		s.sendError(w, http.StatusBadRequest, "Task ID is required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.getTask(w, r, taskID)
	case http.MethodDelete:
		s.deleteTask(w, r, taskID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// getTask gets a task by ID
func (s *RESTServer) getTask(w http.ResponseWriter, r *http.Request, taskID string) {
	task, err := s.state.GetTask(taskID)
	if err != nil {
		s.sendError(w, http.StatusNotFound, "Task not found")
		return
	}

	s.sendJSON(w, http.StatusOK, task)
}

// deleteTask cancels a task
func (s *RESTServer) deleteTask(w http.ResponseWriter, r *http.Request, taskID string) {
	task, err := s.state.GetTask(taskID)
	if err != nil {
		s.sendError(w, http.StatusNotFound, "Task not found")
		return
	}

	if task.Status == models.TaskStatusRunning {
		s.sendError(w, http.StatusBadRequest, "Cannot cancel running task")
		return
	}

	// TODO: Actually cancel the task
	s.sendJSON(w, http.StatusOK, map[string]string{
		"message": "Task cancelled",
	})
}

// handleGPUs handles GPU listing
func (s *RESTServer) handleGPUs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	state := s.state.GetState()
	state.mu.RLock()
	defer state.mu.RUnlock()

	gpus := make([]*models.GPU, 0, len(state.GPUs))
	var idle, busy, offline int

	for _, gpu := range state.GPUs {
		gpus = append(gpus, gpu)
		switch gpu.Status {
		case models.GPUStatusIdle:
			idle++
		case models.GPUStatusBusy:
			busy++
		case models.GPUStatusOffline:
			offline++
		}
	}

	s.sendJSON(w, http.StatusOK, map[string]interface{}{
		"total":   len(gpus),
		"idle":    idle,
		"busy":    busy,
		"offline": offline,
		"gpus":    gpus,
	})
}

// handleQuota handles quota operations
func (s *RESTServer) handleQuota(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.getQuota(w, r)
	case http.MethodPut:
		s.updateQuota(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// getQuota gets current quota
func (s *RESTServer) getQuota(w http.ResponseWriter, r *http.Request) {
	state := s.state.GetState()
	state.mu.RLock()
	defer state.mu.RUnlock()

	s.sendJSON(w, http.StatusOK, map[string]interface{}{
		"total_gpus": state.Quota.TotalGPUs,
		"online": map[string]int{
			"quota":     state.Quota.OnlineQuota,
			"used":      state.Quota.OnlineUsed,
			"available": state.Quota.OnlineQuota - state.Quota.OnlineUsed,
		},
		"batch": map[string]int{
			"quota":     state.Quota.BatchQuota,
			"used":      state.Quota.BatchUsed,
			"available": state.Quota.BatchQuota - state.Quota.BatchUsed,
		},
	})
}

// updateQuota updates quota configuration
func (s *RESTServer) updateQuota(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OnlinePercent float64 `json:"online_percent"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.OnlinePercent < 0 || req.OnlinePercent > 1 {
		s.sendError(w, http.StatusBadRequest, "Online percent must be between 0 and 1")
		return
	}

	batchPercent := 1.0 - req.OnlinePercent
	s.state.SetQuota(req.OnlinePercent, batchPercent)

	s.sendJSON(w, http.StatusOK, map[string]string{
		"message": "Quota updated",
	})
}

// handleHealth handles health check
func (s *RESTServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.sendJSON(w, http.StatusOK, map[string]string{
		"status": "healthy",
	})
}

// loggingMiddleware logs all requests
func (s *RESTServer) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		s.logger.Info("HTTP request",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.Duration("duration", time.Since(start)),
		)
	})
}

// corsMiddleware adds CORS headers
func (s *RESTServer) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// sendJSON sends JSON response
func (s *RESTServer) sendJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// sendError sends error response
func (s *RESTServer) sendError(w http.ResponseWriter, status int, message string) {
	s.sendJSON(w, status, map[string]string{
		"error": message,
	})
}

// generateTaskID generates a unique task ID
func generateTaskID() string {
	return fmt.Sprintf("task-%d", time.Now().UnixNano())
}
