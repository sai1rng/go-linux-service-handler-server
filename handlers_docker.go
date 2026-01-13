package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// List Containers: GET /containers/json?all=true
func listContainersHandler(w http.ResponseWriter, r *http.Request) {
	resp, err := dockerClient.Get("http://docker/containers/json?all=true")
	if err != nil {
		sendJSONResponse(w, http.StatusInternalServerError, "Socket connection failed", err.Error())
		return
	}
	defer resp.Body.Close()

	var containers []DockerContainer
	if err := json.NewDecoder(resp.Body).Decode(&containers); err != nil {
		sendJSONResponse(w, http.StatusInternalServerError, "Failed to decode Docker response", err.Error())
		return
	}

	var list []map[string]string
	for _, c := range containers {
		name := "unknown"
		if len(c.Names) > 0 {
			name = c.Names[0][1:] // Strip leading slash
		}
		list = append(list, map[string]string{
			"id":     c.ID[:12],
			"name":   name,
			"status": c.Status,
			"state":  c.State,
		})
	}
	sendJSONResponse(w, http.StatusOK, "success", map[string]interface{}{"containers": list, "count": len(list)})
}

// Status: Supports GET (query param) and POST (json body)
func statusContainerHandler(w http.ResponseWriter, r *http.Request) {
	var containerID string

	// 1. Check Method and Extract ID
	switch r.Method {
	case http.MethodGet:
		containerID = r.URL.Query().Get("container_id")

	case http.MethodPost:
		var payload ContainerRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			sendJSONResponse(w, http.StatusBadRequest, "Invalid JSON body", err.Error())
			return
		}
		containerID = payload.ContainerID

	default:
		http.Error(w, "Method not allowed. Use GET or POST", http.StatusMethodNotAllowed)
		return
	}

	// 2. Validate ID
	if containerID == "" {
		sendJSONResponse(w, http.StatusBadRequest, "container_id is required", nil)
		return
	}

	// 3. Call Docker API
	resp, err := dockerClient.Get(fmt.Sprintf("http://docker/containers/%s/json", containerID))
	if err != nil {
		sendJSONResponse(w, http.StatusInternalServerError, "Docker socket error", err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		sendJSONResponse(w, http.StatusNotFound, "Container not found", nil)
		return
	}

	var inspect DockerInspectResponse
	if err := json.NewDecoder(resp.Body).Decode(&inspect); err != nil {
		sendJSONResponse(w, http.StatusInternalServerError, "Failed to decode Docker response", nil)
		return
	}

	sendJSONResponse(w, http.StatusOK, inspect.State.Status, nil)
}

// Start: POST /containers/{id}/start
func startContainerHandler(w http.ResponseWriter, r *http.Request) {
	lifecycleHelper(w, r, "start")
}

// Stop: POST /containers/{id}/stop
func stopContainerHandler(w http.ResponseWriter, r *http.Request) {
	lifecycleHelper(w, r, "stop")
}

func lifecycleHelper(w http.ResponseWriter, r *http.Request, action string) {
	var payload ContainerRequest
	json.NewDecoder(r.Body).Decode(&payload)

	if payload.ContainerID == "" {
		sendJSONResponse(w, http.StatusBadRequest, "Container ID required", nil)
		return
	}

	url := fmt.Sprintf("http://docker/containers/%s/%s", payload.ContainerID, action)
	req, _ := http.NewRequest("POST", url, nil)
	
	resp, err := dockerClient.Do(req)
	if err != nil {
		sendJSONResponse(w, http.StatusInternalServerError, "Request failed", err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 204 || resp.StatusCode == 304 {
		sendJSONResponse(w, http.StatusOK, fmt.Sprintf("Container %sed", action), nil)
	} else {
		sendJSONResponse(w, http.StatusInternalServerError, fmt.Sprintf("Docker API Error: %s", resp.Status), nil)
	}
}