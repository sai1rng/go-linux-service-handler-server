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
		fmt.Println("⚠️  WARNING: Not running as root. Host faults/Docker socket may fail.")
	}

	setupRoutes()

	port := ":8080"
	fmt.Printf("🔥 Worker Node (SSE Enabled) listening on %s\n", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal(err)
	}
}

func setupRoutes() {
	// Standard Docker Management (Keep these as JSON/REST)
	http.HandleFunc("/docker/start", startContainerHandler)
	http.HandleFunc("/docker/stop", stopContainerHandler)
	http.HandleFunc("/docker/list", listContainersHandler)
	http.HandleFunc("/docker/status", statusContainerHandler)

	// --- NEW SSE ENDPOINTS ---
	// Usage: GET /host/inject/stream?type=cpu&duration=10
	http.HandleFunc("/host/inject/stream", hostFaultSSEHandler)
	
	// Usage: GET /docker/fault/stream?container_id=xxx&fault_type=cpu_choke
	http.HandleFunc("/docker/fault/stream", containerFaultSSEHandler)
}

// Helper for standard JSON responses (used by management endpoints)
func sendJSONResponse(w http.ResponseWriter, code int, msg string, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	resp := map[string]any{"message": msg, "data": data}
	if code >= 400 {
		resp["error"] = msg
	}
	json.NewEncoder(w).Encode(resp)
}