package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// GPU represents a GPU in the system
type GPU struct {
	UUID      string    `json:"uuid"`
	GPUIndex  string    `json:"gpu_id"`
	Device    string    `json:"device"`
	ModelName string    `json:"model_name"`
	Hostname  string    `json:"hostname"`
	TimeStamp time.Time `json:"timestamp"`
}

// Telemetry represents a telemetry data point for a GPU
type Telemetry struct {
	Timestamp  time.Time `json:"timestamp"`
	MetricName string    `json:"metric_name"`
	GPUIndex   string    `json:"gpu_id"`
	Device     string    `json:"device"`
	UUID       string    `json:"uuid"`
	ModelName  string    `json:"model_name"`
	Hostname   string    `json:"hostname"`
	Container  string    `json:"container,omitempty"`
	Pod        string    `json:"pod,omitempty"`
	Namespace  string    `json:"namespace,omitempty"`
	Value      float64   `json:"value"`
	LabelsRaw  string    `json:"labels_raw,omitempty"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// listGPUs handles GET /gpus
func (s *Server) listGPUs(c *gin.Context) {
	gpuTelemetry, err := s.storage.ListGPUs(c.Request.Context())
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to list GPUs", err.Error())
		return
	}

	gpus := make([]GPU, 0, len(gpuTelemetry))
	for _, gpu := range gpuTelemetry {
		gpus = append(gpus, GPU{
			UUID:      gpu.UUID,
			GPUIndex:  gpu.GPUIndex,
			ModelName: gpu.ModelName,
			Hostname:  gpu.Hostname,
			// Set default values for fields not directly mapped
			Device:    gpu.Device,
			TimeStamp: gpu.Timestamp,
		})
	}

	c.JSON(http.StatusOK, gpus)
}

// getGPUTelemetry handles GET /gpus/:id/telemetry
func (s *Server) getGPUTelemetry(c *gin.Context) {
	gpuID := c.Param("id")
	if gpuID == "" {
		sendError(c, http.StatusBadRequest, "GPU ID is required", "")
		return
	}

	// Parse query parameters
	startTime, endTime, err := parseTimeRange(c)
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid time range", err.Error())
		return
	}

	// Get telemetry data from storage
	telemetryData, err := s.storage.GetGPUTelemetry(c.Request.Context(), gpuID, startTime, endTime)
	if err != nil {
		s.logger.Error().Err(err).Str("gpu_id", gpuID).Msg("Failed to get GPU telemetry")
		sendError(c, http.StatusInternalServerError, "Failed to get GPU telemetry", err.Error())
		return
	}

	// Convert to API response format
	result := make([]Telemetry, 0, len(telemetryData))
	for _, t := range telemetryData {
		result = append(result, Telemetry{
			Timestamp:  t.Timestamp,
			MetricName: t.MetricName,
			GPUIndex:   t.GPUIndex,
			Device:     t.Device,
			UUID:       t.UUID,
			ModelName:  t.ModelName,
			Hostname:   t.Hostname,
			Container:  t.Container,
			Pod:        t.Pod,
			Namespace:  t.Namespace,
			Value:      t.Value,
			LabelsRaw:  t.LabelsRaw,
		})
	}

	c.JSON(http.StatusOK, result)
}

// parseTimeRange parses start and end time from query parameters
func parseTimeRange(c *gin.Context) (time.Time, time.Time, error) {
	now := time.Now()
	defaultStart := now.Add(-24 * time.Hour) // Default to last 24 hours

	// Parse start time
	startTimeStr := c.DefaultQuery("start_time", "")
	if startTimeStr == "" {
		// Fallback to 'start' for backward compatibility
		startTimeStr = c.DefaultQuery("start", "")
	}
	startTime, err := parseTimeParam(startTimeStr)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid start_time: %v", err)
	}
	if startTime == nil {
		t := defaultStart
		startTime = &t
	}

	// Parse end time
	endTimeStr := c.DefaultQuery("end_time", "")
	if endTimeStr == "" {
		// Fallback to 'end' for backward compatibility
		endTimeStr = c.DefaultQuery("end", "")
	}
	endTime, err := parseTimeParam(endTimeStr)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid end_time: %v", err)
	}
	if endTime == nil {
		t := now
		endTime = &t
	}

	// Validate time range
	if endTime.Before(*startTime) {
		return time.Time{}, time.Time{}, fmt.Errorf("end time cannot be before start time")
	}

	return *startTime, *endTime, nil
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
