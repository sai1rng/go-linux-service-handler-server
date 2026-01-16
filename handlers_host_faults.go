package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strconv"
	"sync"
)

type SSEMessage struct {
	State string `json:"state"`
	Msg   string `json:"msg"`
}

func hostFaultSSEHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Setup SSE Headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// 2. Parse Query Params
	query := r.URL.Query()
	faultType := query.Get("type")
	subtype := query.Get("subtype") // delay, loss
	val := query.Get("val")         // e.g. 200ms
	iface := query.Get("interface")
	durationStr := query.Get("duration")

	duration, _ := strconv.Atoi(durationStr)
	if duration <= 0 {
		duration = 10
	}
	if iface == "" {
		iface = "eth0"
	}

	// Default Network Values
	if faultType == "network" && val == "" {
		if subtype == "loss" {
			val = "10%"
		} else {
			val = "200ms"
		}
	}

	var cmd *exec.Cmd
	var cleanupCmd *exec.Cmd

	// 3. Configure the Command
	switch faultType {
	case "cpu":
		// Logs come from stress-ng stdout/stderr
		cmd = exec.Command("stress-ng", "--cpu", "0", "--timeout", fmt.Sprintf("%ds", duration), "-v")

	case "memory":
		// Logs come from stress-ng stdout/stderr
		// --vm 2: start 2 workers
		// --vm-bytes 90%: use 90% RAM
		cmd = exec.Command("stress-ng", "--vm", "2", "--vm-bytes", "90%", "--timeout", fmt.Sprintf("%ds", duration), "-v")

	case "network":
		// A. Setup Network Rules (Immediate)
		if _, err := exec.LookPath("tc"); err != nil {
			sendSSE(w, "error", "tc command not found")
			return
		}

		// Clean previous rules
		exec.Command("tc", "qdisc", "del", "dev", iface, "root").Run()

		args := []string{"qdisc", "add", "dev", iface, "root", "netem"}
		if subtype == "loss" {
			args = append(args, "loss", val)
			sendSSE(w, "start", fmt.Sprintf("Network: Dropping %s packets on %s", val, iface))
		} else {
			args = append(args, "delay", val)
			sendSSE(w, "start", fmt.Sprintf("Network: Adding %s latency to %s", val, iface))
		}

		if out, err := exec.Command("tc", args...).CombinedOutput(); err != nil {
			sendSSE(w, "error", fmt.Sprintf("tc failed: %s", string(out)))
			return
		}

		// B. Define Cleanup Command (Runs after duration)
		cleanupCmd = exec.Command("tc", "qdisc", "del", "dev", iface, "root")

		// C. The "Main" Command to generate Logs
		// Instead of sleeping, we PING a reliable server (1.1.1.1) to visualize the lag/loss.
		// -c: count (duration)
		// -i 1: interval 1 second
		cmd = exec.Command("ping", "-c", fmt.Sprintf("%d", duration), "-i", "1", "1.1.1.1")

		// Log message to explain what's happening
		sendSSE(w, "log", "Running ping test to visualize fault...")

	default:
		sendSSE(w, "error", "Unknown fault type")
		return
	}

	// 4. Start the Main Command
	// For CPU/Mem: This starts stress-ng
	// For Network: This starts the Ping loop
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		sendSSE(w, "error", fmt.Sprintf("Failed to start process: %v", err))
		// Run cleanup if network setup succeeded but ping failed
		if cleanupCmd != nil {
			cleanupCmd.Run()
		}
		return
	}

	if faultType != "network" {
		sendSSE(w, "start", fmt.Sprintf("Started %s stress test for %ds", faultType, duration))
	}

	// 5. Stream Output (Concurrency Safe)
	// We read both stdout and stderr and pipe them to SSE
	var wg sync.WaitGroup
	outputChan := make(chan string)

	// Reader for Stdout
	wg.Add(1)
	go func() {
		defer wg.Done()
		streamPipe(stdout, outputChan)
	}()

	// Reader for Stderr
	wg.Add(1)
	go func() {
		defer wg.Done()
		streamPipe(stderr, outputChan)
	}()

	// Closer routine
	go func() {
		wg.Wait()
		close(outputChan)
	}()

	// Sender Loop
	for line := range outputChan {
		sendSSE(w, "log", line)
	}

	// Wait for process to exit
	cmd.Wait()

	// 6. Run Cleanup (If needed)
	if cleanupCmd != nil {
		sendSSE(w, "cleaning", "Restoring normal network conditions...")
		if out, err := cleanupCmd.CombinedOutput(); err != nil {
			sendSSE(w, "log", fmt.Sprintf("Cleanup warning: %s", string(out)))
		}
	}

	sendSSE(w, "completed", "Fault injection finished")
}

// Reads from a pipe line-by-line and sends to the channel
func streamPipe(pipe io.ReadCloser, ch chan string) {
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		ch <- scanner.Text()
	}
}

// Helper to write SSE data
func sendSSE(w http.ResponseWriter, state, msg string) {
	payload := SSEMessage{State: state, Msg: msg}
	jsonBytes, _ := json.Marshal(payload)

	fmt.Fprintf(w, "data: %s\n\n", jsonBytes)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}
