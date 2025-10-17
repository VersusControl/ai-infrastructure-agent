package api

import (
	"net/http"
	"time"
)

// getStateHandler retrieves the current infrastructure state
// Supports query parameters:
//   - cache_only=true: Skip fresh discovery, use cached state only
//   - discovered_only=true: Exclude managed state, show only discovered resources
func (ws *WebServer) getStateHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Check if AI agent is available
	if ws.aiAgent == nil {
		// Return empty state if AI agent is not available
		emptyState := `{"resources": [], "metadata": {"timestamp": "` + time.Now().Format(time.RFC3339) + `", "source": "demo_mode", "message": "AI agent not available"}}`
		w.Write([]byte(emptyState))
		return
	}

	// Check if client wants fresh discovery (default to true for real-time state)
	includeDiscovered := true
	if r.URL.Query().Get("cache_only") == "true" {
		includeDiscovered = false
	}

	// Check if client wants to exclude managed state (useful when state file is cleared)
	includeManaged := true
	if r.URL.Query().Get("discovered_only") == "true" {
		includeManaged = false
	}

	ws.aiAgent.Logger.WithFields(map[string]interface{}{
		"include_discovered": includeDiscovered,
		"include_managed":    includeManaged,
	}).Info("Getting infrastructure state")

	// Use MCP server to get state with fresh discovery
	stateJSON, err := ws.aiAgent.ExportInfrastructureStateWithOptions(r.Context(), includeDiscovered, includeManaged)
	if err != nil {
		ws.aiAgent.Logger.WithError(err).Error("Failed to get state from MCP server")
		http.Error(w, "Failed to get state", http.StatusInternalServerError)
		return
	}

	// Write the JSON response directly
	w.Write([]byte(stateJSON))
}

// exportStateHandler exports infrastructure state as a downloadable JSON file
// Supports query parameters:
//   - include_discovered=true: Include freshly discovered resources
//   - include_managed=false: Exclude managed resources
func (ws *WebServer) exportStateHandler(w http.ResponseWriter, r *http.Request) {
	includeDiscovered := r.URL.Query().Get("include_discovered") == "true"
	includeManaged := r.URL.Query().Get("include_managed") != "false" // default to true

	ctx := r.Context()
	// Use MCP server to export infrastructure state
	stateJSON, err := ws.aiAgent.ExportInfrastructureStateWithOptions(ctx, includeDiscovered, includeManaged)
	if err != nil {
		ws.aiAgent.Logger.WithError(err).Error("Failed to export infrastructure state")
		http.Error(w, "Export failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=infrastructure-state.json")

	// Write the JSON response directly
	w.Write([]byte(stateJSON))
}
