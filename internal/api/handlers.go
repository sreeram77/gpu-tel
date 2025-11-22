package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// GPU represents a GPU device
type GPU struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Model         string    `json:"model"`
	MemoryMB      int       `json:"memory_mb,omitempty"`
	DriverVersion string    `json:"driver_version,omitempty"`
	FirstSeen     time.Time `json:"first_seen,omitempty"`
}

// Telemetry represents a single telemetry data point
type Telemetry struct {
	Timestamp       time.Time `json:	imestamp"`
	GpuID           string    `json:"gpu_id"`
	GpuUtilization  float64   `json:"gpu_utilization"`
	MemoryUsedMB    float64   `json:"memory_used_mb"`
	MemoryTotalMB   float64   `json:"memory_total_mb"`
	TemperatureC    float64   `json:"temperature_c"`
	PowerUsageW     float64   `json:"power_usage_w"`
	FanSpeedPercent float64   `json:"fan_speed_percent"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// listGPUs handles GET /api/v1/gpus
func (s *Server) listGPUs(c *gin.Context) {
	// Get list of GPU IDs from storage
	gpuIDs, err := s.storage.ListGPUs(c.Request.Context())
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to list GPUs", err.Error())
		return
	}

	// Convert to response format
	gpus := make([]GPU, 0, len(gpuIDs))
	for _, id := range gpuIDs {
		// For now, we only have the ID. In a real implementation, we might want to store
		// more metadata about each GPU in the storage layer.
		gpus = append(gpus, GPU{
			ID:        id,
			FirstSeen: time.Now().Add(-24 * time.Hour), // This would come from storage in a real implementation
		})
	}

	c.JSON(http.StatusOK, gpus)
}

// getGPUTelemetry handles GET /api/v1/gpus/:id/telemetry
func (s *Server) getGPUTelemetry(c *gin.Context) {
	gpuID := c.Param("id")
	if gpuID == "" {
		sendError(c, http.StatusBadRequest, "GPU ID is required", "")
		return
	}

	// Parse query parameters
	startTime, err := parseTimeParam(c.Query("start_time"))
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid start_time format", err.Error())
		return
	}

	endTime, err := parseTimeParam(c.Query("end_time"))
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid end_time format", err.Error())
		return
	}

	// Get telemetry data from storage
	telemetry, err := s.getTelemetryData(gpuID, startTime, endTime)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to get telemetry data", err.Error())
		return
	}

	c.JSON(http.StatusOK, telemetry)
}

// getTelemetryData retrieves telemetry data from storage
func (s *Server) getTelemetryData(gpuID string, startTime, endTime *time.Time) ([]Telemetry, error) {
	// Get telemetry data from storage
	telemetryData, err := s.storage.GetGPUTelemetry(context.Background(), gpuID, *startTime, *endTime)
	if err != nil {
		return nil, err
	}

	// Convert to response format
	result := make([]Telemetry, 0, len(telemetryData))
	for _, t := range telemetryData {
		result = append(result, Telemetry{
			Timestamp:       t.Timestamp,
			GpuID:           t.ID,
			GpuUtilization:  t.GPULoad,
			MemoryUsedMB:    float64(t.MemoryUsed) / (1024 * 1024),  // Convert bytes to MB
			MemoryTotalMB:   float64(t.MemoryTotal) / (1024 * 1024), // Convert bytes to MB
			TemperatureC:    t.GPUTemperature,
			PowerUsageW:     t.PowerDraw,
			FanSpeedPercent: t.FanSpeed,
		})
	}

	return result, nil
}

// parseTimeParam parses a time parameter from a string
func parseTimeParam(timeStr string) (*time.Time, error) {
	if timeStr == "" {
		return nil, nil
	}

	t, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return nil, err
	}

	return &t, nil
}

// sendError sends an error response
func sendError(c *gin.Context, code int, message string, details ...string) {
	err := ErrorResponse{
		Code:    code,
		Message: message,
	}

	if len(details) > 0 {
		err.Details = details[0]
	}

	c.JSON(code, err)
}
