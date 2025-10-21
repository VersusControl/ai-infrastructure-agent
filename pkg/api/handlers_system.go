package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// ========== System/Basic Handlers ==========
// These handlers provide basic system operations like health checks,
// state retrieval, and web UI serving.

// webUiHandler serves the React web UI
func (ws *WebServer) webUiHandler(w http.ResponseWriter, r *http.Request) {
	indexPath := filepath.Join("web", "build", "index.html")

	// Check if the file exists
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		ws.aiAgent.Logger.WithError(err).Error("React build index.html not found")
		http.Error(w, "React build not found. Please run 'npm run build' in the web directory.", http.StatusNotFound)
		return
	}

	// Serve the React index.html file
	http.ServeFile(w, r, indexPath)
}

// healthHandler provides a health check endpoint
func (ws *WebServer) healthHandler(w http.ResponseWriter, r *http.Request) {
	// Return a simple health check response
	health := map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"service":   "ai-infrastructure-agent",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(health)
}

// Start starts the web server
func (ws *WebServer) Start(port int) error {
	addr := fmt.Sprintf(":%d", port)
	ws.aiAgent.Logger.WithField("port", port).Info("Starting web server")

	return http.ListenAndServe(addr, ws.router)
}
