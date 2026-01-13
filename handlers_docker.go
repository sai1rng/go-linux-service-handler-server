package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// List Containers: GET /containers/json?all=true
func listContainersHandler(w http.ResponseWriter, r *http.Request) {
	// We use "http://docker" as a dummy host; the transport hijacks it to the socket
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

	// Simplify response for client
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

// Status: GET /containers/{id}/json
func statusContainerHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("container_id")
	if id == "" {
		sendJSONResponse(w, http.StatusBadRequest, "Missing container_id", nil)
		return
	}

	resp, err := dockerClient.Get(fmt.Sprintf("http://docker/containers/%s/json", id))
	if err != nil || resp.StatusCode != 200 {
		sendJSONResponse(w, http.StatusNotFound, "Container not found or error", nil)
		return
	}
	defer resp.Body.Close()

	var inspect DockerInspectResponse
	json.NewDecoder(resp.Body).Decode(&inspect)
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