package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"
)

var dockerClient *http.Client

func main() {
	// 1. Configure the Transport to talk to /var/run/docker.sock
	dockerClient = &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", "/var/run/docker.sock")
			},
		},
		Timeout: 30 * time.Second,
	}

	if os.Geteuid() != 0 {
		log.Println("⚠️  WARNING: Not running as root. Host faults/Docker socket may fail.")
	}

	setupRoutes()

	port := ":8080"
	log.Printf("🔥 Worker Node listening on %s\n", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal(err)
	}
}

func setupRoutes() {
	// Utility
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		sendJSONResponse(w, http.StatusOK, "Server is running", nil)
	})

	// Docker Management (Standard JSON)
	http.HandleFunc("/docker/start", startContainerHandler)
	http.HandleFunc("/docker/stop", stopContainerHandler)
	http.HandleFunc("/docker/list", listContainersHandler)
	http.HandleFunc("/docker/status", statusContainerHandler)

	// Fault Injection (Streaming SSE)
	// Usage: GET /host/inject?type=cpu&duration=10
	http.HandleFunc("/host/inject", hostFaultHandler)

	// Usage: GET /docker/fault?container_id=...&fault_type=cpu_choke
	http.HandleFunc("/docker/fault", containerFaultHandler)
}

// --- Shared Helpers ---

// Helper for standard JSON responses
func sendJSONResponse(w http.ResponseWriter, code int, msg string, data any) {
	w.Header().Set("Content-Type", "application/json")
	resp := APIResponse{
		Message:   msg,
		Timestamp: time.Now().Format(time.RFC3339),
		Data:      data,
	}
	if code >= 400 {
		resp.Error = msg
		resp.Message = "error"
	}

	jsonBytes, err := json.Marshal(resp)
	if err != nil {
		log.Printf("ERROR: Failed to marshal JSON response: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"failed to build response"}`))
		return
	}

	log.Printf("Sending JSON Response: %s", string(jsonBytes))

	w.WriteHeader(code)
	w.Write(jsonBytes)
}

// Helper for SSE responses (Streaming)
func sendSSE(w http.ResponseWriter, state, msg string) {
	payload := APIResponse{
		State:     state,
		Message:   msg,
		Timestamp: time.Now().Format(time.RFC3339),
	}
	if state == "error" {
		payload.Error = msg
		payload.Message = ""
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		log.Printf("ERROR: Failed to marshal SSE payload: %v", err)
		return
	}

	log.Printf("Sending SSE: data: %s", string(jsonBytes))

	// Format: data: <json>\n\n
	fmt.Fprintf(w, "data: %s\n\n", jsonBytes)

	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}
