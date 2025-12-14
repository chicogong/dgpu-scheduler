package agent

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/chicogong/dgpu-scheduler/pkg/models"
)

// GPUDetector detects and monitors GPU devices
type GPUDetector struct {
	method string
	nodeID string
}

// NewGPUDetector creates a new GPU detector
func NewGPUDetector(method, nodeID string) *GPUDetector {
	return &GPUDetector{
		method: method,
		nodeID: nodeID,
	}
}

// DetectGPUs detects all GPUs on the node
func (d *GPUDetector) DetectGPUs() ([]models.GPU, error) {
	switch d.method {
	case "nvml":
		return d.detectWithNVML()
	case "nvidia-smi":
		return d.detectWithNvidiaSMI()
	default:
		return nil, fmt.Errorf("unsupported detection method: %s", d.method)
	}
}

// detectWithNVML detects GPUs using NVML library
// NOTE: This is a placeholder. In production, use github.com/NVIDIA/go-nvml
func (d *GPUDetector) detectWithNVML() ([]models.GPU, error) {
	// For now, fall back to nvidia-smi
	// In production, implement proper NVML integration:
	// import "github.com/NVIDIA/go-nvml/pkg/nvml"
	return d.detectWithNvidiaSMI()
}

// detectWithNvidiaSMI detects GPUs using nvidia-smi command
func (d *GPUDetector) detectWithNvidiaSMI() ([]models.GPU, error) {
	// nvidia-smi --query-gpu=index,name,memory.total --format=csv,noheader,nounits
	cmd := exec.Command("nvidia-smi",
		"--query-gpu=index,name,memory.total",
		"--format=csv,noheader,nounits",
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run nvidia-smi: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	gpus := make([]models.GPU, 0, len(lines))

	for _, line := range lines {
		parts := strings.Split(line, ",")
		if len(parts) < 3 {
			continue
		}

		index, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			continue
		}

		name := strings.TrimSpace(parts[1])
		memory, err := strconv.ParseInt(strings.TrimSpace(parts[2]), 10, 64)
		if err != nil {
			continue
		}

		gpu := models.GPU{
			ID:          fmt.Sprintf("%s-gpu-%d", d.nodeID, index),
			NodeID:      d.nodeID,
			DeviceIndex: index,
			Model:       name,
			Memory:      memory,
			Status:      models.GPUStatusIdle,
			CurrentTask: nil,
			UpdatedAt:   time.Now(),
		}

		gpus = append(gpus, gpu)
	}

	if len(gpus) == 0 {
		return nil, fmt.Errorf("no GPUs detected")
	}

	return gpus, nil
}

// GetGPUStatus gets current status of all GPUs
func (d *GPUDetector) GetGPUStatus(gpus []models.GPU) ([]GPUStatus, error) {
	// nvidia-smi --query-gpu=index,utilization.gpu,memory.used --format=csv,noheader,nounits
	cmd := exec.Command("nvidia-smi",
		"--query-gpu=index,utilization.gpu,memory.used",
		"--format=csv,noheader,nounits",
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run nvidia-smi: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	statuses := make([]GPUStatus, 0, len(lines))

	for _, line := range lines {
		parts := strings.Split(line, ",")
		if len(parts) < 3 {
			continue
		}

		index, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			continue
		}

		utilization, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 32)
		if err != nil {
			continue
		}

		memoryUsed, err := strconv.ParseInt(strings.TrimSpace(parts[2]), 10, 64)
		if err != nil {
			continue
		}

		// Find corresponding GPU
		var gpuID string
		var gpuStatus models.GPUStatus
		for _, gpu := range gpus {
			if gpu.DeviceIndex == index {
				gpuID = gpu.ID
				gpuStatus = gpu.Status
				break
			}
		}

		if gpuID == "" {
			continue
		}

		status := GPUStatus{
			ID:           gpuID,
			Status:       string(gpuStatus),
			Utilization:  float32(utilization),
			MemoryUsed:   memoryUsed,
		}

		statuses = append(statuses, status)
	}

	return statuses, nil
}

// GPUStatus represents GPU status for reporting
type GPUStatus struct {
	ID          string
	Status      string
	Utilization float32
	MemoryUsed  int64
}

// CheckGPUHealth checks if GPUs are healthy
func (d *GPUDetector) CheckGPUHealth() error {
	cmd := exec.Command("nvidia-smi", "-L")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("GPU health check failed: %w", err)
	}
	return nil
}
