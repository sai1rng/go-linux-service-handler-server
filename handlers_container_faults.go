package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

func containerFaultHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload ContainerFaultRequest
	json.NewDecoder(r.Body).Decode(&payload)

	updateConfig := DockerUpdateConfig{}
	msg := ""

	switch payload.FaultType {
	case "cpu_choke":
		// 10% CPU (10k quota / 100k period)
		updateConfig.CpuPeriod = 100000
		updateConfig.CpuQuota = 10000
		msg = "Container CPU throttled to 10%"
	case "mem_limit":
		// 64MB
		updateConfig.Memory = 64 * 1024 * 1024
		updateConfig.MemorySwap = -1 // Keep swap default
		msg = "Container Memory limited to 64MB"
	case "restore":
		// Restore (-1 means unlimited)
		updateConfig.CpuQuota = -1
		updateConfig.Memory = 0
		msg = "Container resources restored"
	default:
		sendJSONResponse(w, http.StatusBadRequest, "Unknown fault_type", nil)
		return
	}

	// Send POST /containers/{id}/update
	jsonBody, _ := json.Marshal(updateConfig)
	url := fmt.Sprintf("http://docker/containers/%s/update", payload.ContainerID)
	
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := dockerClient.Do(req)
	if err != nil {
		sendJSONResponse(w, http.StatusInternalServerError, "Socket Error", err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		log.Printf("Fault [%s] -> %s", payload.FaultType, payload.ContainerID)
		sendJSONResponse(w, http.StatusOK, msg, nil)
	} else {
		sendJSONResponse(w, http.StatusInternalServerError, "Docker Update Failed", fmt.Sprintf("Status: %s", resp.Status))
	}
}