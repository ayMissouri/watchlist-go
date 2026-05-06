// this means this file belongs to a different package from main.
package handlers

import (
	// encoding/json provides functions for encoding and decoding JSON data.
	"encoding/json"
	// net/http provides HTTP client and server implementations
	"net/http"
)

// HealthResponse is a struct that represents the JSON response for the health endpoint.
type HealthResponse struct {
	Status string `json:"status"`
}

// Health starting with a capital letter means this function is exported and can be used by other packages.
func Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	resp := HealthResponse{
		Status: "ok",
	}

	_ = json.NewEncoder(w).Encode(resp)
}