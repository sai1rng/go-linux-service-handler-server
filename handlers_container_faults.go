package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Endpoint: GET /docker/fault
func containerFaultHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	containerID := r.URL.Query().Get("container_id")
	faultType := r.URL.Query().Get("fault_type")

	if containerID == "" || faultType == "" {
		sendSSE(w, "error", "Missing container_id or fault_type")
		return
	}

	msgChan := make(chan string)
	go func() {
		defer close(msgChan)

		sendSSE(w, "start", fmt.Sprintf("Preparing %s fault...", faultType))

		updateConfig := DockerUpdateConfig{}
		switch faultType {
		case "cpu_choke":
			updateConfig.CpuPeriod = 100000
			updateConfig.CpuQuota = 10000
			sendSSE(w, "injecting", "Applying 10% CPU Limit...")
		case "mem_limit":
			updateConfig.Memory = 64 * 1024 * 1024
			updateConfig.MemorySwap = -1
			sendSSE(w, "injecting", "Applying 64MB Memory Limit...")
		case "restore":
			updateConfig.CpuQuota = -1
			updateConfig.Memory = 0
			sendSSE(w, "cleaning", "Restoring resources...")
		default:
			sendSSE(w, "error", "Unknown fault type")
			return
		}

		jsonBody, _ := json.Marshal(updateConfig)
		url := fmt.Sprintf("http://docker/containers/%s/update", containerID)
		req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")

		time.Sleep(500 * time.Millisecond) // UI Visual Delay

		resp, err := dockerClient.Do(req)
		if err != nil {
			sendSSE(w, "error", fmt.Sprintf("Docker error: %v", err))
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode == 200 {
			sendSSE(w, "completed", "Fault applied successfully")
		} else {
			sendSSE(w, "error", fmt.Sprintf("Docker API Status: %s", resp.Status))
		}
	}()

	<-msgChan // Wait for goroutine (actually we just keep connection open)
}
