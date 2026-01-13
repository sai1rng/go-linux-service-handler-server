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

// Global HTTP Client configured for Unix Socket
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

	// 2. Check Root (Required for accessing the socket usually)
	if os.Geteuid() != 0 {
		fmt.Println("⚠️  WARNING: Not running as root. Permission to /var/run/docker.sock might fail.")
	}

	// 3. Define Routes
	setupRoutes()

	// 4. Start Server
	port := ":8080"
	fmt.Printf("🔥 Socket-Based Chaos Server listening on %s\n", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal(err)
	}
}

func setupRoutes() {
	http.HandleFunc("/docker/start", startContainerHandler)
	http.HandleFunc("/docker/stop", stopContainerHandler)
	http.HandleFunc("/docker/status", statusContainerHandler)
	http.HandleFunc("/docker/list", listContainersHandler)
	http.HandleFunc("/docker/fault", containerFaultHandler)
	http.HandleFunc("/host/inject", hostFaultHandler)
}

func sendJSONResponse(w http.ResponseWriter, code int, msg string, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	resp := ResponsePayload{Message: msg}
	if code >= 400 {
		resp.Error = msg
		resp.Message = "error"
	}
	if data != nil {
		resp.Data = data
	}
	json.NewEncoder(w).Encode(resp)
}