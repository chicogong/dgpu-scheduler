package api

import (
	"context"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/chicogong/dgpu-scheduler/api/proto"
	"github.com/chicogong/dgpu-scheduler/pkg/logger"
	"github.com/chicogong/dgpu-scheduler/pkg/models"
	"github.com/chicogong/dgpu-scheduler/pkg/scheduler"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// GRPCServer implements the gRPC server for scheduler-agent communication
type GRPCServer struct {
	proto.UnimplementedSchedulerServiceServer
	proto.UnimplementedReplicationServiceServer

	state    *scheduler.StateManager
	engine   *scheduler.Engine
	logger   *logger.Logger
	server   *grpc.Server
	isMaster bool
}

// NewGRPCServer creates a new gRPC server
func NewGRPCServer(
	state *scheduler.StateManager,
	engine *scheduler.Engine,
	log *logger.Logger,
	isMaster bool,
) *GRPCServer {
	return &GRPCServer{
		state:    state,
		engine:   engine,
		logger:   log,
		isMaster: isMaster,
	}
}

// Start starts the gRPC server
func (s *GRPCServer) Start(address string) error {
	lis, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	s.server = grpc.NewServer()
	proto.RegisterSchedulerServiceServer(s.server, s)
	proto.RegisterReplicationServiceServer(s.server, s)

	s.logger.Info("gRPC server starting", zap.String("address", address))

	go func() {
		if err := s.server.Serve(lis); err != nil {
			s.logger.Error("gRPC server failed", zap.Error(err))
		}
	}()

	return nil
}

// Stop stops the gRPC server
func (s *GRPCServer) Stop() {
	if s.server != nil {
		s.server.GracefulStop()
	}
}

// RegisterAgent handles agent registration
func (s *GRPCServer) RegisterAgent(ctx context.Context, req *proto.RegisterRequest) (*proto.RegisterResponse, error) {
	s.logger.Info("Agent registering",
		zap.String("agent_id", req.AgentId),
		zap.String("address", req.Address),
		zap.Int("gpu_count", len(req.Gpus)),
	)

	// Convert proto GPUs to model GPUs
	gpus := make([]models.GPU, len(req.Gpus))
	for i, protoGPU := range req.Gpus {
		gpus[i] = models.GPU{
			ID:          protoGPU.Id,
			NodeID:      req.AgentId,
			DeviceIndex: int(protoGPU.DeviceIndex),
			Model:       protoGPU.Model,
			Memory:      protoGPU.Memory,
			Status:      models.GPUStatusIdle,
			UpdatedAt:   time.Now(),
		}
	}

	// Register agent
	agent := &models.Agent{
		ID:            req.AgentId,
		Address:       req.Address,
		GPUs:          gpus,
		LastHeartbeat: time.Now(),
		Status:        models.AgentStatusOnline,
	}

	s.state.RegisterAgent(agent)

	s.logger.Info("Agent registered successfully",
		zap.String("agent_id", req.AgentId),
	)

	return &proto.RegisterResponse{
		Success: true,
		Message: "Agent registered successfully",
	}, nil
}

// Heartbeat handles bidirectional heartbeat streaming
func (s *GRPCServer) Heartbeat(stream proto.SchedulerService_HeartbeatServer) error {
	var agentID string

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			s.logger.Debug("Heartbeat stream closed", zap.String("agent_id", agentID))
			return nil
		}
		if err != nil {
			s.logger.Error("Heartbeat receive error",
				zap.String("agent_id", agentID),
				zap.Error(err),
			)
			return err
		}

		agentID = req.AgentId

		// Update agent heartbeat
		if err := s.state.UpdateAgentHeartbeat(agentID); err != nil {
			s.logger.Warn("Failed to update agent heartbeat",
				zap.String("agent_id", agentID),
				zap.Error(err),
			)
		}

		// Update GPU status
		for _, gpuStatus := range req.GpuStatus {
			status := models.GPUStatusIdle
			if gpuStatus.Status == "busy" {
				status = models.GPUStatusBusy
			} else if gpuStatus.Status == "offline" {
				status = models.GPUStatusOffline
			}

			if err := s.state.UpdateGPUStatus(gpuStatus.Id, status); err != nil {
				s.logger.Debug("Failed to update GPU status",
					zap.String("gpu_id", gpuStatus.Id),
					zap.Error(err),
				)
			}
		}

		// Send response
		resp := &proto.HeartbeatResponse{
			IsMaster:  s.isMaster,
			Tasks:     []*proto.Task{}, // TODO: Send pending tasks to agent
			Timestamp: time.Now().Unix(),
		}

		if err := stream.Send(resp); err != nil {
			s.logger.Error("Heartbeat send error",
				zap.String("agent_id", agentID),
				zap.Error(err),
			)
			return err
		}
	}
}

// TaskFinished handles task completion notification
func (s *GRPCServer) TaskFinished(ctx context.Context, req *proto.TaskFinishedRequest) (*proto.TaskFinishedResponse, error) {
	s.logger.Info("Task finished",
		zap.String("task_id", req.TaskId),
		zap.String("status", req.Status),
	)

	// Determine task status
	var status models.TaskStatus
	switch req.Status {
	case "success":
		status = models.TaskStatusSuccess
	case "failed":
		status = models.TaskStatusFailed
	default:
		return &proto.TaskFinishedResponse{
			Success: false,
			Message: fmt.Sprintf("invalid status: %s", req.Status),
		}, nil
	}

	// Error message
	var errorMsg *string
	if req.Error != "" {
		errorMsg = &req.Error
	}

	// Release task resources
	if err := s.engine.ReleaseTask(req.TaskId, status, errorMsg); err != nil {
		s.logger.Error("Failed to release task",
			zap.String("task_id", req.TaskId),
			zap.Error(err),
		)
		return &proto.TaskFinishedResponse{
			Success: false,
			Message: fmt.Sprintf("failed to release task: %v", err),
		}, nil
	}

	return &proto.TaskFinishedResponse{
		Success: true,
		Message: "Task finished successfully",
	}, nil
}

// SyncState handles master-standby state synchronization
func (s *GRPCServer) SyncState(stream proto.ReplicationService_SyncStateServer) error {
	// TODO: Implement state synchronization for HA
	s.logger.Warn("SyncState not implemented yet")
	return fmt.Errorf("not implemented")
}

// Ping handles master-standby heartbeat
func (s *GRPCServer) Ping(ctx context.Context, req *proto.PingRequest) (*proto.PingResponse, error) {
	return &proto.PingResponse{
		ResponderId: "scheduler", // TODO: Use actual scheduler ID
		IsMaster:    s.isMaster,
		Timestamp:   time.Now().Unix(),
	}, nil
}
