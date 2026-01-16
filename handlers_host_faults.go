package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strconv"
)

// SSE Structure for JSON messages
type SSEMessage struct {
	State string `json:"state"` // start, injecting, log, completed, error
	Msg   string `json:"msg"`
	Time  string `json:"time,omitempty"`
}

func hostFaultSSEHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Setup Headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	query := r.URL.Query()
	faultType := query.Get("type")
	durationStr := query.Get("duration")
	duration, _ := strconv.Atoi(durationStr)
	if duration <= 0 {
		duration = 10
	}

	// 2. Prepare Command
	var cmd *exec.Cmd
	switch faultType {
	case "cpu":
		// verbose mode (-v) ensures stress-ng prints output we can capture
		cmd = exec.Command("stress-ng", "--cpu", "0", "--timeout", fmt.Sprintf("%ds", duration), "-v")
	case "memory":
		cmd = exec.Command("stress-ng", "--vm", "2", "--vm-bytes", "90%", "--timeout", fmt.Sprintf("%ds", duration), "-v")
	// ... add other cases ...
	default:
		sendSSE(w, "error", "Unknown fault type")
		return
	}

	// 3. Get Pipes for Stdout and Stderr
	// We merge both because usually tools print errors to stderr and info to stdout
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	// 4. Start the Command
	if err := cmd.Start(); err != nil {
		sendSSE(w, "error", fmt.Sprintf("Failed to start command: %v", err))
		return
	}

	sendSSE(w, "start", fmt.Sprintf("Command started: %s", faultType))

	// 5. Stream Output in Real-Time
	// We need to read from both pipes concurrently
	outputChan := make(chan string)

	go streamPipe(stdout, outputChan)
	go streamPipe(stderr, outputChan)

	// Close channel when command finishes
	go func() {
		cmd.Wait()
		close(outputChan)
	}()

	// 6. Loop over output lines and send to UI
	for line := range outputChan {
		// Send as a "log" event
		sendSSE(w, "log", line)
	}

	sendSSE(w, "completed", "Fault injection finished")
}

// Helper to read a pipe line-by-line and send to channel
func streamPipe(pipe io.ReadCloser, ch chan string) {
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		ch <- scanner.Text()
	}
}

// Helper to format and flush SSE
func sendSSE(w http.ResponseWriter, state, msg string) {
	payload := SSEMessage{State: state, Msg: msg}
	jsonBytes, _ := json.Marshal(payload)

	fmt.Fprintf(w, "data: %s\n\n", jsonBytes)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}
