package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strconv"
	"sync"
)

// Endpoint: GET /host/inject
func hostFaultHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	query := r.URL.Query()
	faultType := query.Get("type")
	subtype := query.Get("subtype")
	val := query.Get("val")
	iface := query.Get("interface")
	duration, _ := strconv.Atoi(query.Get("duration"))

	if duration <= 0 {
		duration = 10
	}
	if iface == "" {
		iface = "eth0"
	}

	if faultType == "network" && val == "" {
		if subtype == "loss" {
			val = "10%"
		} else {
			val = "200ms"
		}
	}

	var cmd *exec.Cmd
	var cleanupCmd *exec.Cmd

	switch faultType {
	case "cpu":
		cmd = exec.Command("stress-ng", "--cpu", "0", "--timeout", fmt.Sprintf("%ds", duration), "-v")
	case "memory":
		cmd = exec.Command("stress-ng", "--vm", "2", "--vm-bytes", "90%", "--timeout", fmt.Sprintf("%ds", duration), "-v")
	case "network":
		if _, err := exec.LookPath("tc"); err != nil {
			sendSSE(w, "error", "tc command not found")
			return
		}
		exec.Command("tc", "qdisc", "del", "dev", iface, "root").Run() // Clear old rules

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

		cleanupCmd = exec.Command("tc", "qdisc", "del", "dev", iface, "root")
		cmd = exec.Command("ping", "-c", fmt.Sprintf("%d", duration), "-i", "1", "1.1.1.1")
		sendSSE(w, "log", "Running ping test to visualize fault...")

	default:
		sendSSE(w, "error", "Unknown fault type")
		return
	}

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		sendSSE(w, "error", fmt.Sprintf("Failed to start: %v", err))
		if cleanupCmd != nil {
			cleanupCmd.Run()
		}
		return
	}

	if faultType != "network" {
		sendSSE(w, "start", fmt.Sprintf("Started %s stress test for %ds", faultType, duration))
	}

	var wg sync.WaitGroup
	outputChan := make(chan string)

	wg.Add(2)
	go func() { defer wg.Done(); streamPipe(stdout, outputChan) }()
	go func() { defer wg.Done(); streamPipe(stderr, outputChan) }()
	go func() { wg.Wait(); close(outputChan) }()

	for line := range outputChan {
		sendSSE(w, "log", line)
	}

	cmd.Wait()

	if cleanupCmd != nil {
		sendSSE(w, "cleaning", "Restoring network...")
		cleanupCmd.Run()
	}

	sendSSE(w, "completed", "Fault injection finished")
}

func streamPipe(pipe io.ReadCloser, ch chan string) {
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		ch <- scanner.Text()
	}
}
