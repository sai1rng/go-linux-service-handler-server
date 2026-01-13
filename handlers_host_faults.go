package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

func hostFaultHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req HostFaultRequest
	json.NewDecoder(r.Body).Decode(&req)

	if req.Duration <= 0 { req.Duration = 60 }
	if req.Interface == "" { req.Interface = getDefaultInterface() }

	go executeHostFault(req)

	sendJSONResponse(w, http.StatusOK, fmt.Sprintf("Host Fault '%s' triggered for %ds", req.Type, req.Duration), nil)
}

func executeHostFault(req HostFaultRequest) {
	log.Printf("⚡ Host Fault: %s (%ds)", req.Type, req.Duration)
	
	switch req.Type {
	case "cpu":
		args := []string{"--cpu", "0", "--timeout", fmt.Sprintf("%ds", req.Duration)}
		if req.LoadPercent > 0 { args = append(args, "--cpu-load", fmt.Sprintf("%d", req.LoadPercent)) }
		exec.Command("stress-ng", args...).Run()
	case "memory":
		args := []string{"--vm", "2", "--vm-bytes", "90%", "--timeout", fmt.Sprintf("%ds", req.Duration)}
		exec.Command("stress-ng", args...).Run()
	case "disk":
		defer os.Remove("/tmp/chaos_test.dat")
		exec.Command("fio", "--name=chaos", "--ioengine=libaio", "--rw=randwrite", "--bs=64k", "--size=1G", "--numjobs=2", "--direct=1", "--time_based", fmt.Sprintf("--runtime=%d", req.Duration), "--filename=/tmp/chaos_test.dat").Run()
	case "network":
		if req.Interface != "" {
			args := []string{"qdisc", "add", "dev", req.Interface, "root", "netem"}
			if req.Latency != "" { args = append(args, "delay", req.Latency, req.Jitter) }
			if req.Loss != "" { args = append(args, "loss", req.Loss) }
			exec.Command("tc", args...).Run()
			time.Sleep(time.Duration(req.Duration) * time.Second)
			exec.Command("tc", "qdisc", "del", "dev", req.Interface, "root").Run()
		}
	}
	log.Printf("✅ Host Fault Completed")
}

func getDefaultInterface() string {
	out, _ := exec.Command("bash", "-c", "ip route | grep default | awk '{print $5}' | head -n1").Output()
	return strings.TrimSpace(string(out))
}