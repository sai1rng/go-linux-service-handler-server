package main

// --- Internal Docker API Structures ---

// Used for listing containers (GET /containers/json)
type DockerContainer struct {
	ID     string   `json:"Id"`
	Names  []string `json:"Names"`
	State  string   `json:"State"`
	Status string   `json:"Status"`
}

// Used for inspecting containers (GET /containers/{id}/json)
type DockerInspectResponse struct {
	State struct {
		Status string `json:"Status"` // running, exited, etc.
	} `json:"State"`
}

// Used for updating resources (POST /containers/{id}/update)
type DockerUpdateConfig struct {
	CpuPeriod  int64 `json:"CpuPeriod,omitempty"`
	CpuQuota   int64 `json:"CpuQuota,omitempty"`
	Memory     int64 `json:"Memory,omitempty"`
	MemorySwap int64 `json:"MemorySwap,omitempty"`
}

// --- Server Request/Response Structures ---

type ResponsePayload struct {
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
	Data    any    `json:"data,omitempty"`
	Time    string `json:"timestamp"`
}

type ContainerRequest struct {
	ContainerID string `json:"container_id"`
}

type ContainerFaultRequest struct {
	ContainerID string `json:"container_id"`
	FaultType   string `json:"fault_type"`
}

type HostFaultRequest struct {
	Type        string `json:"type"`
	Duration    int    `json:"duration"`
	LoadPercent int    `json:"load_percent,omitempty"`
	Interface   string `json:"interface,omitempty"`
	Latency     string `json:"latency,omitempty"`
	Jitter      string `json:"jitter,omitempty"`
	Loss        string `json:"loss,omitempty"`
}

// --- SSE Streaming Structure ---
type SSEMessage struct {
	State string `json:"state"` // start, injecting, log, completed, error
	Msg   string `json:"msg"`
	Time  string `json:"timestamp"` // <--- New Field
}
