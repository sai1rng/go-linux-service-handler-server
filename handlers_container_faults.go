package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// usage: GET /docker/fault/stream?container_id=...&fault_type=cpu_choke
func containerFaultSSEHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Setup SSE Headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// 2. Parse Query Parameters
	query := r.URL.Query()
	containerID := query.Get("container_id")
	faultType := query.Get("fault_type")

	// 3. Validation
	if containerID == "" {
		sendSSE(w, "error", "Missing container_id parameter")
		return
	}
	if faultType == "" {
		sendSSE(w, "error", "Missing fault_type parameter")
		return
	}

	// 4. Create a channel to handle the fault logic asynchronously
	// This allows us to stream updates while blocking on the Docker API call
	msgChan := make(chan string)

	go func() {
		defer close(msgChan)

		// Notify Start
		// We truncate ID to 12 chars for readability
		shortID := containerID
		if len(containerID) > 12 {
			shortID = containerID[:12]
		}

		sendSSE(w, "start", fmt.Sprintf("Preparing %s fault for container %s...", faultType, shortID))

		// 5. Configure Resource Limits based on Fault Type
		updateConfig := DockerUpdateConfig{}

		switch faultType {
		case "cpu_choke":
			// Limit CPU to 10% (10,000 quota / 100,000 period)
			updateConfig.CpuPeriod = 100000
			updateConfig.CpuQuota = 10000
			sendSSE(w, "injecting", "Applying CPU throttle (10%)...")

		case "mem_limit":
			// Limit Memory to 64MB
			updateConfig.Memory = 64 * 1024 * 1024
			updateConfig.MemorySwap = -1 // Prevent swap issues
			sendSSE(w, "injecting", "Applying Hard Memory Limit (64MB)...")

		case "restore":
			// Remove all limits
			updateConfig.CpuQuota = -1
			updateConfig.Memory = 0
			sendSSE(w, "cleaning", "Restoring original container resources...")

		default:
			sendSSE(w, "error", fmt.Sprintf("Unknown fault type: %s", faultType))
			return
		}

		// 6. Call Docker Engine API (Socket)
		// POST /containers/{id}/update
		jsonBody, err := json.Marshal(updateConfig)
		if err != nil {
			sendSSE(w, "error", "Failed to encode config")
			return
		}

		url := fmt.Sprintf("http://docker/containers/%s/update", containerID)
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
		if err != nil {
			sendSSE(w, "error", "Failed to create request")
			return
		}
		req.Header.Set("Content-Type", "application/json")

		// Execute Request
		// We simulate a small delay so the UI shows the "injecting" state briefly
		time.Sleep(500 * time.Millisecond)

		resp, err := dockerClient.Do(req)
		if err != nil {
			sendSSE(w, "error", fmt.Sprintf("Docker Socket connection failed: %v", err))
			return
		}
		defer resp.Body.Close()

		// 7. Check Response Status
		if resp.StatusCode == 200 {
			sendSSE(w, "completed", "Container resources updated successfully")
		} else {
			// Try to read error message from body
			var errResp struct {
				Message string `json:"message"`
			}
			json.NewDecoder(resp.Body).Decode(&errResp)

			errMsg := errResp.Message
			if errMsg == "" {
				errMsg = resp.Status
			}

			sendSSE(w, "error", fmt.Sprintf("Docker API Error: %s", errMsg))
		}
	}()

	// 8. Keep Connection Open (The function blocks here until 'go func' finishes)
	// Note: In this specific handler, we are writing directly to 'w' inside the goroutine
	// via the helper function because the helper handles flushing.
	// Just need to ensure the main handler doesn't exit before the goroutine.
	// We use a channel simply to wait.
	<-msgChan
}
