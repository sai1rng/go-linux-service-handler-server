package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"time"
)

// SSE Handler for Host Faults
func hostFaultSSEHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Set SSE Headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// 2. Parse Query Params (Since EventSource uses GET)
	query := r.URL.Query()
	faultType := query.Get("type")
	durationStr := query.Get("duration")

	// Default duration
	duration, _ := strconv.Atoi(durationStr)
	if duration <= 0 {
		duration = 10
	}

	// Create a channel to communicate status updates
	msgChan := make(chan string)

	// 3. Run Fault Logic in Background Goroutine
	go func() {
		defer close(msgChan)

		// Initial State
		msgChan <- fmt.Sprintf(`{"state": "start", "msg": "Worker received %s fault request"}`, faultType)

		var err error

		switch faultType {
		case "cpu":
			msgChan <- fmt.Sprintf(`{"state": "injecting", "msg": "Spiking CPU load for %ds..."}`, duration)
			// Runs stress-ng blocking
			err = exec.Command("stress-ng", "--cpu", "0", "--timeout", fmt.Sprintf("%ds", duration)).Run()

		case "memory":
			msgChan <- fmt.Sprintf(`{"state": "injecting", "msg": "Consuming Memory for %ds..."}`, duration)
			err = exec.Command("stress-ng", "--vm", "2", "--vm-bytes", "90%", "--timeout", fmt.Sprintf("%ds", duration)).Run()

		case "disk":
			msgChan <- fmt.Sprintf(`{"state": "injecting", "msg": "Thrashing Disk I/O for %ds..."}`, duration)
			defer os.Remove("/tmp/chaos_test.dat")
			err = exec.Command("fio", "--name=chaos", "--ioengine=libaio", "--rw=randwrite", "--bs=64k",
				"--size=512M", "--numjobs=2", "--direct=1", "--time_based",
				fmt.Sprintf("--runtime=%d", duration), "--filename=/tmp/chaos_test.dat").Run()

		case "network":
			iface := "eth0" // Ideally fetch this dynamically
			msgChan <- fmt.Sprintf(`{"state": "injecting", "msg": "Adding Network Latency on %s..."}`, iface)

			// Add delay
			exec.Command("tc", "qdisc", "add", "dev", iface, "root", "netem", "delay", "200ms").Run()

			// Wait for duration
			time.Sleep(time.Duration(duration) * time.Second)

			// Remove delay
			msgChan <- `{"state": "cleaning", "msg": "Restoring network rules..."}`
			exec.Command("tc", "qdisc", "del", "dev", iface, "root").Run()

		default:
			msgChan <- fmt.Sprintf(`{"state": "error", "msg": "Unknown fault type: %s"}`, faultType)
			return
		}

		// Final State
		if err != nil {
			msgChan <- fmt.Sprintf(`{"state": "error", "msg": "Fault execution failed: %s"}`, err.Error())
		} else {
			msgChan <- `{"state": "completed", "msg": "Fault injection finished successfully"}`
		}
	}()

	// 4. Stream Loop: Pipe messages to the HTTP response
	for msg := range msgChan {
		// SSE format: data: <payload>\n\n
		fmt.Fprintf(w, "data: %s\n\n", msg)

		// Flush immediately
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
}
