package api

import (
	"encoding/json"
	"net/http"
	"time"
)

// ========== Infrastructure Discovery Handlers ==========
// These handlers provide infrastructure discovery, dependency analysis,
// and conflict detection capabilities.

// discoverInfrastructureHandler discovers AWS infrastructure resources
// Returns a list of all discovered resources with their current state
func (ws *WebServer) discoverInfrastructureHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	ctx := r.Context()
	// Use MCP server to analyze infrastructure state
	_, discoveredResources, _, err := ws.aiAgent.AnalyzeInfrastructureState(ctx, true)
	if err != nil {
		ws.aiAgent.Logger.WithError(err).Error("Failed to discover infrastructure")
		http.Error(w, "Discovery failed", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"resources": discoveredResources,
		"count":     len(discoveredResources),
		"timestamp": time.Now(),
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		ws.aiAgent.Logger.WithError(err).Error("Failed to encode discovery response")
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// getGraphHandler generates a dependency graph visualization
// Supports query parameters:
//   - source=state (default): Use state file for graph generation
//   - source=live: Discover from AWS and generate graph
//
// Returns a Mermaid diagram and structured graph data
func (ws *WebServer) getGraphHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Force format to mermaid for front-end visualization
	format := "mermaid"

	// Get source parameter (state or live)
	// Default: state (from state file)
	// live: discover from AWS
	source := r.URL.Query().Get("source")
	if source == "" {
		source = "state" // default to state file
	}

	if source != "state" && source != "live" {
		http.Error(w, `Invalid source parameter. Use "state" (default) or "live"`, http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Check if AI agent is available
	if ws.aiAgent == nil {
		ws.aiAgent.Logger.Error("AI agent not available for graph visualization")
		http.Error(w, "Graph visualization requires AI agent", http.StatusServiceUnavailable)
		return
	}

	// Use MCP server to visualize dependency graph with source parameter
	graphData, err := ws.aiAgent.VisualizeDependencyGraph(ctx, format, true, source)
	if err != nil {
		ws.aiAgent.Logger.WithError(err).Error("Failed to visualize dependency graph")
		http.Error(w, "Graph visualization failed", http.StatusInternalServerError)
		return
	}

	// Build response from the graph data
	response := map[string]interface{}{
		"format":    format,
		"source":    source,
		"timestamp": time.Now(),
	}

	// Extract and add mermaid visualization
	if visualization, ok := graphData["visualization"].(string); ok {
		response["mermaid"] = visualization
	}

	// Add graph structure
	if graph, ok := graphData["graph"]; ok {
		response["graph"] = graph
	}

	// Add metadata
	if metadata, ok := graphData["metadata"]; ok {
		response["metadata"] = metadata
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		ws.aiAgent.Logger.WithError(err).Error("Failed to encode graph response")
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// getConflictsHandler detects resource conflicts in the infrastructure
// Supports query parameters:
//   - auto_resolve=true: Automatically resolve detected conflicts
//
// Returns a list of conflicts with details and resolution suggestions
func (ws *WebServer) getConflictsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	autoResolve := r.URL.Query().Get("auto_resolve") == "true"

	ctx := r.Context()
	// Use MCP server to detect conflicts
	conflicts, err := ws.aiAgent.DetectInfrastructureConflicts(ctx, autoResolve)
	if err != nil {
		ws.aiAgent.Logger.WithError(err).Error("Failed to detect conflicts")
		http.Error(w, "Conflict detection failed", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"conflicts":    conflicts,
		"count":        len(conflicts),
		"auto_resolve": autoResolve,
		"timestamp":    time.Now(),
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		ws.aiAgent.Logger.WithError(err).Error("Failed to encode conflicts response")
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}
