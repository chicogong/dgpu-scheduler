package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/chicogong/dgpu-scheduler/api/proto"
	"github.com/chicogong/dgpu-scheduler/pkg/logger"
	"github.com/chicogong/dgpu-scheduler/pkg/models"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client is the gRPC client for agent-scheduler communication
type Client struct {
	agentID         string
	masterAddr      string
	standbyAddr     string
	currentAddr     string
	conn            *grpc.ClientConn
	client          proto.SchedulerServiceClient
	logger          *logger.Logger
	heartbeatStream proto.SchedulerService_HeartbeatClient
	executor        *TaskExecutor
	stopCh          chan struct{}
}

// NewClient creates a new gRPC client
func NewClient(agentID, masterAddr, standbyAddr string, log *logger.Logger) *Client {
	return &Client{
		agentID:     agentID,
		masterAddr:  masterAddr,
		standbyAddr: standbyAddr,
		currentAddr: masterAddr, // Start with master
		logger:      log,
		stopCh:      make(chan struct{}),
	}
}

// Connect connects to the scheduler
func (c *Client) Connect(ctx context.Context) error {
	c.logger.Info("Connecting to scheduler", zap.String("address", c.currentAddr))

	conn, err := grpc.NewClient(
		c.currentAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	c.conn = conn
	c.client = proto.NewSchedulerServiceClient(conn)

	c.logger.Info("Connected to scheduler", zap.String("address", c.currentAddr))
	return nil
}

// Register registers the agent with the scheduler
func (c *Client) Register(ctx context.Context, gpus []models.GPU) error {
	// Convert model GPUs to proto GPUs
	protoGPUs := make([]*proto.GPU, len(gpus))
	for i, gpu := range gpus {
		protoGPUs[i] = &proto.GPU{
			Id:          gpu.ID,
			DeviceIndex: int32(gpu.DeviceIndex),
			Model:       gpu.Model,
			Memory:      gpu.Memory,
		}
	}

	req := &proto.RegisterRequest{
		AgentId: c.agentID,
		Address: c.currentAddr,
		Gpus:    protoGPUs,
	}

	resp, err := c.client.RegisterAgent(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to register: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("registration failed: %s", resp.Message)
	}

	c.logger.Info("Agent registered successfully",
		zap.String("agent_id", c.agentID),
		zap.String("message", resp.Message),
	)

	return nil
}

// StartHeartbeat starts the heartbeat loop
func (c *Client) StartHeartbeat(ctx context.Context, interval time.Duration, gpus []models.GPU, executor *TaskExecutor) error {
	stream, err := c.client.Heartbeat(ctx)
	if err != nil {
		return fmt.Errorf("failed to start heartbeat: %w", err)
	}

	c.heartbeatStream = stream
	c.executor = executor

	// Start heartbeat sender
	go c.heartbeatSender(ctx, interval, gpus)

	// Start heartbeat receiver
	go c.heartbeatReceiver(ctx)

	return nil
}

// heartbeatSender sends periodic heartbeats
func (c *Client) heartbeatSender(ctx context.Context, interval time.Duration, gpus []models.GPU) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := c.sendHeartbeat(gpus); err != nil {
				c.logger.Error("Failed to send heartbeat", zap.Error(err))
				// TODO: Implement reconnection logic
			}
		case <-c.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// sendHeartbeat sends a single heartbeat
func (c *Client) sendHeartbeat(gpus []models.GPU) error {
	// Get GPU status
	gpuStatuses := make([]*proto.GPUStatus, len(gpus))
	for i, gpu := range gpus {
		gpuStatuses[i] = &proto.GPUStatus{
			Id:          gpu.ID,
			Status:      string(gpu.Status),
			Utilization: 0, // TODO: Get actual utilization
			MemoryUsed:  0, // TODO: Get actual memory usage
		}
	}

	req := &proto.HeartbeatRequest{
		AgentId:   c.agentID,
		GpuStatus: gpuStatuses,
		Timestamp: time.Now().Unix(),
	}

	if err := c.heartbeatStream.Send(req); err != nil {
		return fmt.Errorf("failed to send heartbeat: %w", err)
	}

	c.logger.Debug("Heartbeat sent", zap.String("agent_id", c.agentID))
	return nil
}

// heartbeatReceiver receives heartbeat responses
func (c *Client) heartbeatReceiver(ctx context.Context) {
	for {
		select {
		case <-c.stopCh:
			return
		case <-ctx.Done():
			return
		default:
			resp, err := c.heartbeatStream.Recv()
			if err != nil {
				c.logger.Error("Failed to receive heartbeat response", zap.Error(err))
				// TODO: Implement reconnection logic
				time.Sleep(time.Second)
				continue
			}

			c.logger.Debug("Heartbeat response received",
				zap.Bool("is_master", resp.IsMaster),
				zap.Int("task_count", len(resp.Tasks)),
			)

			// Handle master failover
			if !resp.IsMaster && c.currentAddr == c.masterAddr {
				c.logger.Warn("Master is down, switching to standby")
				// TODO: Implement failover to standby
			}

			// Handle new tasks
			if len(resp.Tasks) > 0 {
				c.logger.Info("Received tasks from scheduler",
					zap.Int("task_count", len(resp.Tasks)),
				)

				// Execute each task
				for _, protoTask := range resp.Tasks {
					// Convert proto task to models.Task
					task := &models.Task{
						ID:       protoTask.Id,
						Priority: models.Priority(protoTask.Priority),
						GPUCount: int(protoTask.GpuCount),
						Command:  protoTask.Command,
						Env:      protoTask.Env,
						Status:   models.TaskStatusRunning,
					}

					// Extract GPU IDs
					gpuIDs := protoTask.AssignedGpus

					c.logger.Info("Executing task",
						zap.String("task_id", task.ID),
						zap.String("command", task.Command),
						zap.Strings("gpu_ids", gpuIDs),
					)

					// Execute task
					if err := c.executor.ExecuteTask(ctx, task, gpuIDs); err != nil {
						c.logger.Error("Failed to execute task",
							zap.String("task_id", task.ID),
							zap.Error(err),
						)
						// Report failure to scheduler
						if err := c.ReportTaskFinished(ctx, task.ID, "failed", err.Error()); err != nil {
							c.logger.Error("Failed to report task failure",
								zap.String("task_id", task.ID),
								zap.Error(err),
							)
						}
					}
				}
			}
		}
	}
}

// ReportTaskFinished reports task completion to scheduler
func (c *Client) ReportTaskFinished(ctx context.Context, taskID, status, errorMsg string) error {
	req := &proto.TaskFinishedRequest{
		TaskId:    taskID,
		Status:    status,
		Error:     errorMsg,
		Timestamp: time.Now().Unix(),
	}

	resp, err := c.client.TaskFinished(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to report task finished: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("report failed: %s", resp.Message)
	}

	c.logger.Info("Task finished reported",
		zap.String("task_id", taskID),
		zap.String("status", status),
	)

	return nil
}

// Stop stops the client
func (c *Client) Stop() {
	close(c.stopCh)
	if c.heartbeatStream != nil {
		_ = c.heartbeatStream.CloseSend()
	}
	if c.conn != nil {
		_ = c.conn.Close()
	}
}
