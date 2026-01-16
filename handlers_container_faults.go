package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// SSE Handler for Container Faults
func containerFaultSSEHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Parse Params
	query := r.URL.Query()
	containerID := query.Get("container_id")
	faultType := query.Get("fault_type")

	msgChan := make(chan string)

	go func() {
		defer close(msgChan)

		if containerID == "" {
			msgChan <- `{"state": "error", "msg": "Missing container_id"}`
			return
		}

		msgChan <- fmt.Sprintf(`{"state": "start", "msg": "Preparing %s for container %s"}`, faultType, containerID[:10])

		// Prepare Config
		updateConfig := DockerUpdateConfig{}

		switch faultType {
		case "cpu_choke":
			updateConfig.CpuPeriod = 100000
			updateConfig.CpuQuota = 10000 // 10%
		case "mem_limit":
			updateConfig.Memory = 64 * 1024 * 1024 // 64MB
			updateConfig.MemorySwap = -1
		case "restore":
			updateConfig.CpuQuota = -1
			updateConfig.Memory = 0
		default:
			msgChan <- `{"state": "error", "msg": "Unknown fault type"}`
			return
		}

		// Send Request to Docker Socket
		msgChan <- `{"state": "injecting", "msg": "Applying resource limits via Docker API..."}`

		jsonBody, _ := json.Marshal(updateConfig)
		url := fmt.Sprintf("http://docker/containers/%s/update", containerID)
		req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")

		resp, err := dockerClient.Do(req)
		if err != nil {
			msgChan <- fmt.Sprintf(`{"state": "error", "msg": "Socket error: %s"}`, err.Error())
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode == 200 {
			msgChan <- `{"state": "completed", "msg": "Container limits updated successfully"}`
		} else {
			msgChan <- fmt.Sprintf(`{"state": "error", "msg": "Docker API returned status %s"}`, resp.Status)
		}
	}()

	// Stream Loop
	for msg := range msgChan {
		fmt.Fprintf(w, "data: %s\n\n", msg)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
}
