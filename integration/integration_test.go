package integration

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	testConfigPath = "../configs/test-config.yaml"
	mqGRPCPort     = 50051
	apiHTTPPort    = 8080
	testTopic      = "gpu_metrics"
)

func TestIntegration(t *testing.T) {
	// Get the absolute path to the test config file
	configPath, err := filepath.Abs("../configs/test-config.yaml")
	if err != nil {
		t.Fatalf("Failed to get absolute path to test config: %v", err)
	}

	// Start MQ service
	mqCmd := startService(t, "mq-service", mqGRPCPort, fmt.Sprintf("--config=%s", configPath))
	defer stopService(t, mqCmd)

	// Start sink service
	sinkCmd := startService(t, "sink", apiHTTPPort, fmt.Sprintf("--config=%s", configPath))
	defer stopService(t, sinkCmd)

	// Start telemetry streamer
	streamerCmd := startService(t, "telemetry-streamer", 0, fmt.Sprintf("--config=%s", configPath))
	defer stopService(t, streamerCmd)

	// Wait for services to start and initialize
	time.Sleep(3 * time.Second)

	// Test 1: Verify MQ service is running
	t.Run("MQ Service Health Check", func(t *testing.T) {
		conn, err := grpc.Dial(
			fmt.Sprintf("localhost:%d", mqGRPCPort),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		require.NoError(t, err, "Failed to connect to MQ service")
		defer conn.Close()

		// Connection successful means the service is running
		assert.NotNil(t, conn)
	})

	// Test 2: Verify API is responding
	t.Run("API Health Check", func(t *testing.T) {
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/health", apiHTTPPort))
		require.NoError(t, err, "Failed to call health endpoint")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "Unexpected status code")

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err, "Failed to decode response")
		assert.Equal(t, "ok", result["status"], "Unexpected status in response")
	})

	// Test 3: Wait for streamer to process some data
	t.Log("Waiting for telemetry streamer to process data...")
	time.Sleep(5 * time.Second)

	// Test 4: Test GPU telemetry API
	t.Run("GPU Telemetry API", func(t *testing.T) {
		// First, get list of GPUs
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/v1/gpus", apiHTTPPort))
		require.NoError(t, err, "Failed to get GPU list")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "Unexpected status code")

		var gpus []map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&gpus)
		require.NoError(t, err, "Failed to decode GPU list response")

		// If there are GPUs, test getting telemetry for the first one
		if len(gpus) > 0 {
			gpuID := gpus[0]["id"].(string)
			t.Logf("Testing telemetry for GPU: %s", gpuID)

			// Test getting telemetry with time range
			now := time.Now()
			start := now.Add(-5 * time.Minute).Format(time.RFC3339)
			end := now.Format(time.RFC3339)

			telemetryURL := fmt.Sprintf("http://localhost:%d/api/v1/gpus/%s/telemetry?start=%s&end=%s",
				apiHTTPPort, gpuID, start, end)

			telemetryResp, err := http.Get(telemetryURL)
			require.NoError(t, err, "Failed to get GPU telemetry")
			defer telemetryResp.Body.Close()

			assert.Equal(t, http.StatusOK, telemetryResp.StatusCode, "Unexpected status code for telemetry")

			var telemetry []map[string]interface{}
			err = json.NewDecoder(telemetryResp.Body).Decode(&telemetry)
			assert.NoError(t, err, "Failed to decode telemetry response")
			t.Logf("Received %d telemetry data points", len(telemetry))
		}
	})
}

func startService(t *testing.T, name string, port int, args string) *exec.Cmd {
	t.Helper()

	// Get the absolute path to the test config file
	configPath, err := filepath.Abs("../configs/test-config.yaml")
	if err != nil {
		t.Fatalf("Failed to get absolute path to test config: %v", err)
	}

	// Set the CONFIG_PATH environment variable
	os.Setenv("CONFIG_PATH", configPath)

	// Build the service binary if it doesn't exist
	binaryPath := filepath.Join("bin", name)
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		// Build the service
		buildCmd := exec.Command("go", "build", "-o", binaryPath, filepath.Join("../cmd", name))
		buildCmd.Stdout = os.Stdout
		buildCmd.Stderr = os.Stderr
		if err := buildCmd.Run(); err != nil {
			t.Fatalf("Failed to build %s: %v", name, err)
		}
	}

	// Prepare the command
	cmd := exec.Command(binaryPath, "--config="+configPath)

	// Set up output redirection
	logFile, err := os.Create(fmt.Sprintf("%s.log", name))
	if err != nil {
		t.Fatalf("Failed to create log file for %s: %v", name, err)
	}

	// Set environment variables
	cmd.Env = append(os.Environ(), 
		"CONFIG_PATH="+configPath,
	)

	cmd.Stdout = logFile
	cmd.Stderr = logFile

	// Start the service
	t.Logf("Starting %s service...", name)
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start %s: %v", name, err)
	}

	// Wait for the service to be ready
	if port > 0 {
		maxRetries := 30
		for i := 0; i < maxRetries; i++ {
			conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 100*time.Millisecond)
			if err == nil {
				conn.Close()
				t.Logf("%s service is up and running", name)
				return cmd
			}
			time.Sleep(100 * time.Millisecond)
		}
		// Read the log file to help with debugging
		logContent, _ := os.ReadFile(fmt.Sprintf("%s.log", name))
		t.Logf("%s service logs:\n%s", name, string(logContent))
		t.Fatalf("Timed out waiting for %s to start on port %d", name, port)
	}

	t.Logf("%s service started", name)

	// For services without a health check, just wait a moment
	time.Sleep(2 * time.Second)
	return cmd
}

func stopService(t *testing.T, cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}

	t.Logf("Stopping process %d...", cmd.Process.Pid)

	// Try to stop the process gracefully first
	err := cmd.Process.Signal(os.Interrupt)
	if err != nil {
		t.Logf("Error sending interrupt signal: %v", err)
		cmd.Process.Kill()
		return
	}

	// Wait for the process to exit, but don't wait forever
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-time.After(5 * time.Second):
		t.Log("Process did not exit gracefully, forcing kill...")
		cmd.Process.Kill()
	case err := <-done:
		if err != nil {
			t.Logf("Process exited with error: %v", err)
		} else {
			t.Log("Process stopped gracefully")
		}
	}
}
