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

type SSEMessage struct {
	State string `json:"state"`
	Msg   string `json:"msg"`
}

func hostFaultSSEHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// 1. Parse Parameters
	query := r.URL.Query()
	faultType := query.Get("type")  // cpu, memory, network
	subtype := query.Get("subtype") // delay, loss (for network)
	val := query.Get("val")         // 200ms, 10%
	iface := query.Get("interface") // eth0
	durationStr := query.Get("duration")

	duration, _ := strconv.Atoi(durationStr)
	if duration <= 0 {
		duration = 10
	}
	if iface == "" {
		iface = "eth0"
	} // Default interface
	if val == "" && faultType == "network" {
		if subtype == "loss" {
			val = "10%"
		} else {
			val = "200ms"
		}
	}

	// 2. Prepare Command
	var cmd *exec.Cmd
	var cleanupCmd *exec.Cmd

	switch faultType {
	case "memory":
		// Consume 90% of RAM
		cmd = exec.Command("stress-ng", "--vm", "2", "--vm-bytes", "90%", "--timeout", fmt.Sprintf("%ds", duration), "-v")

	case "network":
		// Check if tc exists
		if _, err := exec.LookPath("tc"); err != nil {
			sendSSE(w, "error", "tc command not found. Install iproute2.")
			return
		}

		// Clean up previous rules first (ignore errors)
		exec.Command("tc", "qdisc", "del", "dev", iface, "root").Run()

		// Construct 'tc' command
		// Syntax: tc qdisc add dev eth0 root netem delay 200ms
		args := []string{"qdisc", "add", "dev", iface, "root", "netem"}

		if subtype == "loss" {
			args = append(args, "loss", val) // val = "10%"
			sendSSE(w, "start", fmt.Sprintf("Network Loss: Dropping %s packets on %s", val, iface))
		} else {
			// Default to delay
			args = append(args, "delay", val) // val = "200ms"
			sendSSE(w, "start", fmt.Sprintf("Network Latency: Adding %s delay on %s", val, iface))
		}

		// Execute the setup immediately (Blocking but fast)
		if output, err := exec.Command("tc", args...).CombinedOutput(); err != nil {
			sendSSE(w, "error", fmt.Sprintf("tc failed: %s", string(output)))
			return
		}

		// Set cleanup command to run after sleep
		cleanupCmd = exec.Command("tc", "qdisc", "del", "dev", iface, "root")

		// Create a dummy sleep command just to keep the "injecting" state active for 'duration'
		cmd = exec.Command("sleep", fmt.Sprintf("%d", duration))

	default:
		// Fallback for CPU, Disk, etc.
		cmd = exec.Command("stress-ng", "--cpu", "0", "--timeout", fmt.Sprintf("%ds", duration), "-v")
	}

	// 3. Run the "Wait" Command (stress-ng or sleep)
	if err := cmd.Start(); err != nil {
		sendSSE(w, "error", fmt.Sprintf("Failed to start: %v", err))
		return
	}

	if faultType != "network" {
		sendSSE(w, "start", fmt.Sprintf("Command started: %s", faultType))
	}
	sendSSE(w, "injecting", fmt.Sprintf("Fault active for %ds...", duration))

	// Stream logs if available
	stdout, _ := cmd.StdoutPipe()
	if stdout != nil {
		outputChan := make(chan string)
		go streamPipe(stdout, outputChan)
		go func() { cmd.Wait(); close(outputChan) }()
		for line := range outputChan {
			sendSSE(w, "log", line)
		}
	} else {
		cmd.Wait()
	}

	// 4. Cleanup (For Network)
	if cleanupCmd != nil {
		sendSSE(w, "cleaning", "Restoring network settings...")
		if out, err := cleanupCmd.CombinedOutput(); err != nil {
			sendSSE(w, "error", fmt.Sprintf("Cleanup failed: %s", string(out)))
		}
	}

	sendSSE(w, "completed", "Fault injection finished")
}

func streamPipe(pipe io.ReadCloser, ch chan string) {
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		ch <- scanner.Text()
	}
}

func sendSSE(w http.ResponseWriter, state, msg string) {
	payload := SSEMessage{State: state, Msg: msg}
	jsonBytes, _ := json.Marshal(payload)
	fmt.Fprintf(w, "data: %s\n\n", jsonBytes)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}
