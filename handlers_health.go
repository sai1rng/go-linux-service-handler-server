package main

import (
	"net/http"
)

// Simple Health Check: GET /health
func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	// You can add more logic here (e.g., check if Docker socket is actually responsive)
	// For now, just returning 200 OK means the HTTP server is up.
	sendJSONResponse(w, http.StatusOK, "Server is running", nil)
}