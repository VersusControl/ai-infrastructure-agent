package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/versus-control/ai-infrastructure-agent/internal/config"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/agent"
	"github.com/versus-control/ai-infrastructure-agent/pkg/aws"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

// WebSocket connection wrapper
type wsConnection struct {
	conn       *websocket.Conn
	lastPong   time.Time
	writeMutex sync.Mutex // Protects concurrent writes to WebSocket
}

// WebServer handles HTTP requests for the AI agent UI
type WebServer struct {
	router *mux.Router

	upgrader websocket.Upgrader
	aiAgent  *agent.StateAwareAgent

	// WebSocket connection management
	connections map[string]*wsConnection
	connMutex   sync.RWMutex

	// Decision storage for plan confirmations
	decisions      map[string]*StoredDecision
	decisionsMutex sync.RWMutex

	// Plan-level recovery coordination
	planRecoveryRequests map[string]*PlanRecoveryRequest
	planRecoveryMutex    sync.RWMutex
}

// PlanRecoveryRequest represents a pending plan-level recovery decision
type PlanRecoveryRequest struct {
	ExecutionID    string                      `json:"executionId"`
	FailureContext *agent.PlanFailureContext   `json:"failureContext"`
	Strategy       *agent.PlanRecoveryStrategy `json:"strategy"` // Single strategy now
	ResponseChan   chan *PlanRecoveryResponse  `json:"-"`
	Timestamp      time.Time                   `json:"timestamp"`
}

// PlanRecoveryResponse represents the user's plan recovery decision
type PlanRecoveryResponse struct {
	Approved bool `json:"approved"` // User approved (true) or rejected (false) the recovery plan
}

// StoredDecision stores a decision along with its execution parameters
type StoredDecision struct {
	Decision *types.AgentDecision `json:"decision"`
	DryRun   bool                 `json:"dry_run"`
}

// NewWebServer creates a new web server instance
func NewWebServer(cfg *config.Config, awsClient *aws.Client, logger *logging.Logger) *WebServer {
	ws := &WebServer{
		router:               mux.NewRouter(),
		connections:          make(map[string]*wsConnection),
		decisions:            make(map[string]*StoredDecision),
		planRecoveryRequests: make(map[string]*PlanRecoveryRequest),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins in development
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
	}

	// Initialize AI agent with all infrastructure components
	ws.initializeAIAgent(cfg, awsClient, logger)

	ws.setupRoutes()

	return ws
}

// initializeAIAgent initializes the AI agent if possible
func (ws *WebServer) initializeAIAgent(cfg *config.Config, awsClient *aws.Client, logger *logging.Logger) {
	// Check if the required API key is available based on provider
	provider := strings.ToLower(cfg.Agent.Provider)
	var hasAPIKey bool

	switch provider {
	case "openai":
		hasAPIKey = cfg.Agent.OpenAIAPIKey != ""
	case "gemini", "googleai":
		hasAPIKey = cfg.Agent.GeminiAPIKey != ""
	case "anthropic":
		hasAPIKey = cfg.Agent.AnthropicAPIKey != ""
	case "bedrock", "nova":
		// For Bedrock, AWS credentials are handled by default credential chain
		hasAPIKey = true
	default:
		logger.WithField("provider", provider).Warn("Unknown AI provider - AI agent will run in demo mode")
		return
	}

	if !hasAPIKey {
		logger.WithField("provider", provider).Warn("API key not set for provider - AI agent will run in demo mode")
		return
	}

	// Create AI agent using centralized config
	aiAgent, err := agent.NewStateAwareAgent(
		&cfg.Agent,
		awsClient,
		cfg.GetStateFilePath(),
		cfg.AWS.Region,
		logger,
		&cfg.AWS,
	)
	if err != nil {
		logger.WithError(err).Error("Failed to create AI agent - running in demo mode")
		return
	}

	// Initialize the agent
	if err := aiAgent.Initialize(context.Background()); err != nil {
		logger.WithError(err).Error("Failed to initialize AI agent - running in demo mode")
		return
	}

	ws.aiAgent = aiAgent
	logger.Info("AI agent initialized successfully")
}

// corsMiddleware adds CORS headers to responses
func (ws *WebServer) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, Authorization")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Continue to next handler
		next.ServeHTTP(w, r)
	})
}

// setupRoutes configures HTTP routes
func (ws *WebServer) setupRoutes() {
	// Serve React build files
	buildDir := "web/build"

	// Serve static assets (CSS, JS, images) from React build
	fs := http.FileServer(http.Dir(buildDir))
	ws.router.PathPrefix("/static/").Handler(http.StripPrefix("/", fs))
	ws.router.PathPrefix("/aws-service-icons/").Handler(http.StripPrefix("/", fs))
	ws.router.PathPrefix("/manifest.json").Handler(fs)
	ws.router.PathPrefix("/robots.txt").Handler(fs)
	ws.router.PathPrefix("/ai-infrastructure-agent.svg").Handler(fs)

	// Health check endpoint
	ws.router.HandleFunc("/health", ws.healthHandler).Methods("GET")

	// API routes with CORS middleware
	api := ws.router.PathPrefix("/api").Subrouter()
	api.Use(ws.corsMiddleware) // Apply CORS middleware to all API routes
	api.HandleFunc("/health", ws.healthHandler).Methods("GET")
	api.HandleFunc("/state", ws.getStateHandler).Methods("GET")
	api.HandleFunc("/discover", ws.discoverInfrastructureHandler).Methods("POST")
	api.HandleFunc("/graph", ws.getGraphHandler).Methods("GET")
	api.HandleFunc("/conflicts", ws.getConflictsHandler).Methods("GET")
	api.HandleFunc("/plan", ws.getPlanHandler).Methods("POST")
	api.HandleFunc("/agent/process", ws.processRequestHandler).Methods("POST")
	api.HandleFunc("/agent/execute-with-plan-recovery", ws.executeWithPlanRecoveryHandler).Methods("POST")
	api.HandleFunc("/export", ws.exportStateHandler).Methods("GET")

	// Handle OPTIONS requests for all API routes
	api.HandleFunc("/{path:.*}", func(w http.ResponseWriter, r *http.Request) {
		// CORS middleware will handle this
		w.WriteHeader(http.StatusOK)
	}).Methods("OPTIONS")

	// WebSocket for real-time updates
	ws.router.HandleFunc("/ws", ws.websocketHandler)

	// Handler for React Router
	// This serves index.html for all non-API routes to support client-side routing
	ws.router.PathPrefix("/").HandlerFunc(ws.webUiHandler).Methods("GET")
}

// webUiHandler serves the React app for all non-API routes
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

// Start starts the web server
func (ws *WebServer) Start(port int) error {
	addr := fmt.Sprintf(":%d", port)
	ws.aiAgent.Logger.WithField("port", port).Info("Starting web server")

	return http.ListenAndServe(addr, ws.router)
}

// Handlers

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

// API Handlers

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

func (ws *WebServer) getPlanHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var requestBody map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	includeLevels := true
	if val, ok := requestBody["include_levels"].(bool); ok {
		includeLevels = val
	}

	ctx := r.Context()
	// Use MCP server to plan deployment
	deploymentOrder, deploymentLevels, err := ws.aiAgent.PlanInfrastructureDeployment(ctx, nil, includeLevels)
	if err != nil {
		ws.aiAgent.Logger.WithError(err).Error("Failed to plan deployment")
		http.Error(w, "Deployment planning failed", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"deployment_order": deploymentOrder,
		"resource_count":   len(deploymentOrder),
		"timestamp":        time.Now(),
	}

	if includeLevels {
		response["deployment_levels"] = deploymentLevels
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		ws.aiAgent.Logger.WithError(err).Error("Failed to encode plan response")
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (ws *WebServer) processRequestHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var requestBody map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	request, ok := requestBody["request"].(string)
	if !ok {
		http.Error(w, "Request field is required", http.StatusBadRequest)
		return
	}

	dryRun := true
	if val, ok := requestBody["dry_run"].(bool); ok {
		dryRun = val
	}

	ctx := r.Context()

	// Check if AI agent is available
	if ws.aiAgent == nil {
		// Return demo response if AI agent is not available
		response := map[string]interface{}{
			"request":    request,
			"dry_run":    dryRun,
			"response":   "AI Agent not available. Please set OPENAI_API_KEY environment variable to enable real AI processing.",
			"confidence": 0.0,
			"mode":       "demo",
			"timestamp":  time.Now(),
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			ws.aiAgent.Logger.WithError(err).Error("Failed to encode demo response")
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
		return
	}

	ws.aiAgent.Logger.WithFields(map[string]interface{}{
		"request": request,
		"dry_run": dryRun,
	}).Info("Processing request with AI agent")

	// Notify WebSocket clients that processing has started
	ws.broadcastUpdate(map[string]interface{}{
		"type":      "processing_started",
		"request":   request,
		"dry_run":   dryRun,
		"timestamp": time.Now(),
	})

	// Process the request
	decision, err := ws.aiAgent.ProcessRequest(ctx, request)
	if err != nil {
		ws.aiAgent.Logger.WithError(err).Error("AI agent request processing failed")
		http.Error(w, fmt.Sprintf("AI processing failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Store the decision for later execution
	ws.storeDecisionWithDryRun(decision, dryRun)

	// Build response with execution plan (without executing yet)
	response := map[string]interface{}{
		"request":              request,
		"dry_run":              dryRun,
		"mode":                 "live",
		"decision":             decision,
		"executionPlan":        decision.ExecutionPlan,
		"confidence":           decision.Confidence,
		"action":               decision.Action,
		"reasoning":            decision.Reasoning,
		"requiresConfirmation": true,
		"timestamp":            time.Now(),
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		ws.aiAgent.Logger.WithError(err).Error("Failed to encode AI response")
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	// Notify WebSocket clients that processing has completed
	ws.broadcastUpdate(map[string]interface{}{
		"type":                 "processing_completed",
		"request":              request,
		"decisionId":           decision.ID,
		"success":              true,
		"requiresConfirmation": true,
		"timestamp":            time.Now(),
	})
}

// executeWithPlanRecoveryHandler executes a confirmed plan with plan-level ReAct recovery
func (ws *WebServer) executeWithPlanRecoveryHandler(w http.ResponseWriter, r *http.Request) {
	var executeRequest struct {
		DecisionID string `json:"decisionId"`
	}

	if err := json.NewDecoder(r.Body).Decode(&executeRequest); err != nil {
		ws.aiAgent.Logger.WithError(err).Error("Failed to decode execute request")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if executeRequest.DecisionID == "" {
		http.Error(w, "Decision ID is required", http.StatusBadRequest)
		return
	}

	ws.aiAgent.Logger.WithField("decision_id", executeRequest.DecisionID).Info("Executing plan with plan-level recovery")

	// Retrieve the stored decision
	decision, _, exists := ws.getStoredDecisionWithDryRun(executeRequest.DecisionID)
	if !exists {
		ws.aiAgent.Logger.WithField("decision_id", executeRequest.DecisionID).Error("Decision not found")
		http.Error(w, "Decision not found", http.StatusNotFound)
		return
	}

	// Create a buffered progress channel to avoid blocking
	progressChan := make(chan *types.ExecutionUpdate, 100)

	// Start execution in a goroutine
	go func() {
		defer close(progressChan)
		defer ws.removeStoredDecision(executeRequest.DecisionID) // Cleanup after execution

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute*20) // 20 minutes for plan-level recovery
		defer cancel()

		// Use new plan-level recovery execution
		execution, err := ws.aiAgent.ExecutePlanWithReActRecovery(ctx, decision, progressChan, ws)
		if err != nil {
			ws.aiAgent.Logger.WithError(err).Error("Plan execution with recovery failed")
			// Send error update
			select {
			case progressChan <- &types.ExecutionUpdate{
				Type:        "execution_failed",
				ExecutionID: "failed",
				Message:     fmt.Sprintf("Execution failed: %v", err),
				Error:       err.Error(),
				Timestamp:   time.Now(),
			}:
			default:
			}
		} else {
			ws.aiAgent.Logger.WithFields(map[string]interface{}{
				"execution_id": execution.ID,
				"status":       execution.Status,
				"total_steps":  len(execution.Steps),
			}).Info("Plan execution with plan-level recovery completed")
		}
	}()

	// Start progress streaming in another goroutine
	go func() {
		for update := range progressChan {
			ws.aiAgent.Logger.WithFields(map[string]interface{}{
				"type":    update.Type,
				"message": update.Message,
			}).Debug("Broadcasting execution update")

			// Broadcast update via WebSocket
			ws.broadcastUpdate(map[string]interface{}{
				"type":        update.Type,
				"executionId": update.ExecutionID,
				"stepId":      update.StepID,
				"message":     update.Message,
				"error":       update.Error,
				"timestamp":   update.Timestamp,
			})
		}
	}()

	// Return immediate response
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"success":      true,
		"message":      "Plan execution started with plan-level recovery",
		"executionId":  "exec-" + executeRequest.DecisionID,
		"recoveryType": "plan-level",
		"timestamp":    time.Now(),
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		ws.aiAgent.Logger.WithError(err).Error("Failed to encode execute response")
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

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

// WebSocket handler for real-time updates
func (ws *WebServer) websocketHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := ws.upgrader.Upgrade(w, r, nil)
	if err != nil {
		ws.aiAgent.Logger.WithError(err).Error("Failed to upgrade WebSocket")
		return
	}

	// Generate unique connection ID
	connID := fmt.Sprintf("%s-%d", r.RemoteAddr, time.Now().UnixNano())

	// Store connection
	ws.connMutex.Lock()
	ws.connections[connID] = &wsConnection{
		conn:     conn,
		lastPong: time.Now(),
	}
	ws.connMutex.Unlock()

	ws.aiAgent.Logger.WithField("conn_id", connID).Info("WebSocket connection established")

	// Setup connection cleanup
	defer func() {
		ws.connMutex.Lock()
		delete(ws.connections, connID)
		ws.connMutex.Unlock()
		conn.Close()
		ws.aiAgent.Logger.WithField("conn_id", connID).Info("WebSocket connection closed")
	}()

	// Configure connection timeouts
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))

	// Setup ping/pong for connection health
	conn.SetPongHandler(func(string) error {
		ws.connMutex.Lock()
		if wsConn, exists := ws.connections[connID]; exists {
			wsConn.lastPong = time.Now()
		}
		ws.connMutex.Unlock()
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// Send periodic updates and ping
	ticker := time.NewTicker(30 * time.Second)
	pingTicker := time.NewTicker(45 * time.Second)
	defer ticker.Stop()
	defer pingTicker.Stop()

	// Send initial state
	ws.sendStateUpdate(connID)

	// Handle connection lifecycle
	done := make(chan struct{})

	// Handle incoming messages (if any)
	go func() {
		defer close(done)
		for {
			_, msgData, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					ws.aiAgent.Logger.WithError(err).WithField("conn_id", connID).Error("WebSocket read error")
				}
				return
			}

			// Process incoming message
			ws.handleIncomingMessage(connID, msgData)
		}
	}()

	// Main event loop
	for {
		select {
		case <-ticker.C:
			// Send state update
			ws.sendStateUpdate(connID)

		case <-pingTicker.C:
			// Send ping to check connection health
			ws.connMutex.Lock()
			wsConn, exists := ws.connections[connID]
			ws.connMutex.Unlock()

			if !exists {
				return
			}

			// Lock write mutex to prevent concurrent writes
			wsConn.writeMutex.Lock()
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			err := conn.WriteMessage(websocket.PingMessage, nil)
			wsConn.writeMutex.Unlock()

			if err != nil {
				ws.aiAgent.Logger.WithError(err).WithField("conn_id", connID).Error("Failed to send ping")
				return
			}

			// Check if we've received a recent pong
			if time.Since(wsConn.lastPong) > 90*time.Second {
				ws.aiAgent.Logger.WithField("conn_id", connID).Warn("Connection seems stale, closing")
				return
			}

		case <-done:
			return

		case <-r.Context().Done():
			return
		}
	}
}

// sendStateUpdate sends a state update to a specific connection
func (ws *WebServer) sendStateUpdate(connID string) {
	ws.connMutex.RLock()
	wsConn, exists := ws.connections[connID]
	ws.connMutex.RUnlock()

	if !exists {
		return
	}

	// Use MCP server to get current state with fresh discovery
	stateJSON, err := ws.aiAgent.ExportInfrastructureStateWithOptions(context.Background(), true, true)
	if err != nil {
		ws.aiAgent.Logger.WithError(err).Error("Failed to get state for WebSocket update")
		return
	}

	update := map[string]interface{}{
		"type":      "state_update",
		"data":      stateJSON,
		"timestamp": time.Now(),
	}

	// Lock the write mutex to prevent concurrent writes
	wsConn.writeMutex.Lock()
	wsConn.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	err = wsConn.conn.WriteJSON(update)
	wsConn.writeMutex.Unlock()

	if err != nil {
		ws.aiAgent.Logger.WithError(err).WithField("conn_id", connID).Debug("Failed to send state update, connection likely closed")
		// Connection is broken, it will be cleaned up by the main handler
	}
}

// broadcastUpdate sends an update to all active WebSocket connections
func (ws *WebServer) broadcastUpdate(update map[string]interface{}) {
	ws.connMutex.RLock()
	connections := make(map[string]*wsConnection)
	for id, conn := range ws.connections {
		connections[id] = conn
	}
	ws.connMutex.RUnlock()

	for connID, wsConn := range connections {
		// Lock the write mutex for this connection to prevent concurrent writes
		wsConn.writeMutex.Lock()

		wsConn.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		err := wsConn.conn.WriteJSON(update)

		wsConn.writeMutex.Unlock()

		if err != nil {
			ws.aiAgent.Logger.WithError(err).WithField("conn_id", connID).Debug("Failed to broadcast update, removing connection")
			// Remove broken connection
			ws.connMutex.Lock()
			delete(ws.connections, connID)
			ws.connMutex.Unlock()
			wsConn.conn.Close()
		}
	}
}

// storeDecisionWithDryRun stores a decision with dry run flag for later execution
func (ws *WebServer) storeDecisionWithDryRun(decision *types.AgentDecision, dryRun bool) {
	ws.decisionsMutex.Lock()
	defer ws.decisionsMutex.Unlock()
	ws.decisions[decision.ID] = &StoredDecision{
		Decision: decision,
		DryRun:   dryRun,
	}
	ws.aiAgent.Logger.WithFields(map[string]interface{}{
		"decision_id": decision.ID,
		"dry_run":     dryRun,
	}).Debug("Stored decision for execution")
}

// getStoredDecisionWithDryRun retrieves a stored decision with its dry run flag
func (ws *WebServer) getStoredDecisionWithDryRun(decisionID string) (*types.AgentDecision, bool, bool) {
	ws.decisionsMutex.RLock()
	defer ws.decisionsMutex.RUnlock()
	storedDecision, exists := ws.decisions[decisionID]
	if !exists {
		return nil, false, false
	}
	return storedDecision.Decision, storedDecision.DryRun, true
}

// removeStoredDecision removes a stored decision after execution
func (ws *WebServer) removeStoredDecision(decisionID string) {
	ws.decisionsMutex.Lock()
	defer ws.decisionsMutex.Unlock()
	delete(ws.decisions, decisionID)
	ws.aiAgent.Logger.WithField("decision_id", decisionID).Debug("Removed stored decision")
}

// RecoveryMessage represents incoming recovery-related messages from WebSocket clients
type RecoveryMessage struct {
	Type        string    `json:"type"`
	ExecutionID string    `json:"executionId,omitempty"` // For plan-level recovery
	Approved    bool      `json:"approved,omitempty"`    // For plan-level recovery (approved/rejected)
	Timestamp   time.Time `json:"timestamp"`
}

// handleIncomingMessage processes messages received from WebSocket clients
func (ws *WebServer) handleIncomingMessage(connID string, msgData []byte) {
	var message RecoveryMessage
	if err := json.Unmarshal(msgData, &message); err != nil {
		ws.aiAgent.Logger.WithError(err).WithField("conn_id", connID).Error("Failed to parse WebSocket message")
		return
	}

	ws.aiAgent.Logger.WithFields(logrus.Fields{
		"conn_id":      connID,
		"type":         message.Type,
		"execution_id": message.ExecutionID,
	}).Debug("Received WebSocket message")

	switch message.Type {
	case "plan_recovery_decision":
		ws.handlePlanRecoveryDecision(message)
	case "plan_recovery_abort":
		ws.handlePlanRecoveryAbort(message)
	default:
		ws.aiAgent.Logger.WithFields(logrus.Fields{
			"conn_id": connID,
			"type":    message.Type,
		}).Warn("Unknown WebSocket message type")
	}
}

// handlePlanRecoveryDecision processes user's plan recovery decision
func (ws *WebServer) handlePlanRecoveryDecision(message RecoveryMessage) {
	ws.aiAgent.Logger.WithFields(logrus.Fields{
		"execution_id": message.ExecutionID,
		"approved":     message.Approved,
	}).Info("Processing plan recovery decision")

	// Find the pending plan recovery request
	ws.planRecoveryMutex.Lock()
	request, exists := ws.planRecoveryRequests[message.ExecutionID]
	ws.planRecoveryMutex.Unlock()

	if !exists {
		ws.aiAgent.Logger.WithField("execution_id", message.ExecutionID).Warn("No pending plan recovery request found")
		return
	}

	// Send response back to waiting goroutine
	response := &PlanRecoveryResponse{
		Approved: message.Approved,
	}

	select {
	case request.ResponseChan <- response:
		ws.aiAgent.Logger.WithFields(logrus.Fields{
			"execution_id": message.ExecutionID,
			"approved":     message.Approved,
		}).Info("Plan recovery decision sent to execution")
	default:
		ws.aiAgent.Logger.WithField("execution_id", message.ExecutionID).Error("Failed to send plan recovery decision - channel full or closed")
	}
}

// handlePlanRecoveryAbort processes user's plan recovery abort decision
func (ws *WebServer) handlePlanRecoveryAbort(message RecoveryMessage) {
	ws.aiAgent.Logger.WithField("execution_id", message.ExecutionID).Info("Processing plan recovery abort")

	// Find the pending plan recovery request
	ws.planRecoveryMutex.Lock()
	request, exists := ws.planRecoveryRequests[message.ExecutionID]
	ws.planRecoveryMutex.Unlock()

	if !exists {
		ws.aiAgent.Logger.WithField("execution_id", message.ExecutionID).Warn("No pending plan recovery request found for abort")
		return
	}

	// Send abort response back to waiting goroutine
	response := &PlanRecoveryResponse{
		Approved: false, // User rejected the recovery plan
	}

	select {
	case request.ResponseChan <- response:
		ws.aiAgent.Logger.WithField("execution_id", message.ExecutionID).Info("Plan recovery abort sent to execution")
	default:
		ws.aiAgent.Logger.WithField("execution_id", message.ExecutionID).Error("Failed to send plan recovery abort - channel full or closed")
	}
}

// ========== Plan-Level Recovery Coordinator Implementation ==========

// RequestPlanRecoveryDecision implements PlanRecoveryCoordinator interface
// RequestPlanRecoveryDecision implements PlanRecoveryCoordinator interface
// This is the main method called by the agent when plan-level recovery is needed
func (ws *WebServer) RequestPlanRecoveryDecision(
	executionID string,
	context *agent.PlanFailureContext,
	strategy *agent.PlanRecoveryStrategy,
) (approved bool, err error) {
	ws.aiAgent.Logger.WithFields(map[string]interface{}{
		"execution_id": executionID,
		"failed_step":  context.FailedStepID,
		"attempt":      context.AttemptNumber,
		"total_steps":  strategy.TotalSteps,
	}).Info("Plan recovery decision requested from UI")

	// Create response channel for this request
	responseChan := make(chan *PlanRecoveryResponse, 1)

	// Store the recovery request
	ws.planRecoveryMutex.Lock()
	ws.planRecoveryRequests[executionID] = &PlanRecoveryRequest{
		ExecutionID:    executionID,
		FailureContext: context,
		Strategy:       strategy, // Single strategy now
		ResponseChan:   responseChan,
		Timestamp:      time.Now(),
	}
	ws.planRecoveryMutex.Unlock()

	// Convert strategy to JSON-friendly format for UI (same structure as AgentDecision)
	strategyJSON := map[string]interface{}{
		"action":             strategy.Action,
		"reasoning":          strategy.Reasoning,
		"confidence":         strategy.Confidence,
		"successProbability": strategy.SuccessProbability,
		"riskLevel":          strategy.RiskLevel,
		"estimatedDuration":  strategy.EstimatedDuration,
		"executionPlan":      strategy.ExecutionPlan, // Standard ExecutionPlanStep array
		"totalSteps":         strategy.TotalSteps,
		"preservedCount":     strategy.PreservedCount,
		"newStepsCount":      strategy.NewStepsCount,
		"recoveryNotes":      strategy.RecoveryNotes,
	}

	// Convert failure context to JSON-friendly format
	contextJSON := map[string]interface{}{
		"executionId":      context.ExecutionID,
		"failedStepId":     context.FailedStepID,
		"failedStepIndex":  context.FailedStepIndex,
		"failureError":     context.FailureError,
		"attemptNumber":    context.AttemptNumber,
		"completedSteps":   len(context.CompletedSteps),
		"remainingSteps":   len(context.RemainingSteps),
		"failedStepName":   context.FailedStep.Name,
		"failedStepAction": context.FailedStep.Action,
	}

	// Add completed steps details
	completedStepsJSON := make([]map[string]interface{}, len(context.CompletedSteps))
	for i, step := range context.CompletedSteps {
		completedStepsJSON[i] = map[string]interface{}{
			"stepId":       step.StepID,
			"stepName":     step.StepName,
			"stepIndex":    step.StepIndex,
			"resourceId":   step.ResourceID,
			"resourceType": step.ResourceType,
			"status":       step.Status,
		}
	}
	contextJSON["completedStepsDetails"] = completedStepsJSON

	// Send plan recovery request to UI (single strategy)
	ws.broadcastUpdate(map[string]interface{}{
		"type":           "plan_recovery_request",
		"executionId":    executionID,
		"failureContext": contextJSON,
		"strategy":       strategyJSON, // Single strategy now
		"timestamp":      time.Now(),
	})

	// Log the complete execution plan being sent to UI for debugging
	if len(strategy.ExecutionPlan) > 0 {
		// Marshal the complete strategy JSON (including full execution plan with parameters)
		if strategyFullJSON, err := json.MarshalIndent(strategyJSON, "", "  "); err == nil {
			ws.aiAgent.Logger.WithFields(map[string]interface{}{
				"execution_id": executionID,
				"total_steps":  len(strategy.ExecutionPlan),
			}).Infof("📋 Complete strategy sent to front-end:\n%s", string(strategyFullJSON))
		}
	}

	ws.aiAgent.Logger.WithField("execution_id", executionID).Info("Plan recovery request sent to UI, waiting for response")

	// Wait for user response with timeout
	select {
	case response := <-responseChan:
		// Cleanup
		ws.planRecoveryMutex.Lock()
		delete(ws.planRecoveryRequests, executionID)
		ws.planRecoveryMutex.Unlock()

		ws.aiAgent.Logger.WithFields(map[string]interface{}{
			"execution_id": executionID,
			"approved":     response.Approved,
		}).Info("Plan recovery decision received from user")

		return response.Approved, nil

	case <-time.After(15 * time.Minute): // 15 minute timeout for plan-level decisions
		// Cleanup
		ws.planRecoveryMutex.Lock()
		delete(ws.planRecoveryRequests, executionID)
		ws.planRecoveryMutex.Unlock()

		ws.aiAgent.Logger.WithField("execution_id", executionID).Error("Plan recovery decision timeout")
		return false, fmt.Errorf("plan recovery decision timeout after 15 minutes")
	}
}

// NotifyRecoveryAnalyzing implements PlanRecoveryCoordinator interface
func (ws *WebServer) NotifyRecoveryAnalyzing(
	executionID string,
	failedStepID string,
	failedIndex int,
	completedCount int,
	totalSteps int,
) error {
	ws.aiAgent.Logger.WithFields(map[string]interface{}{
		"execution_id":    executionID,
		"failed_step":     failedStepID,
		"failed_index":    failedIndex,
		"completed_count": completedCount,
		"total_steps":     totalSteps,
	}).Info("Notifying UI: Analyzing recovery options")

	ws.broadcastUpdate(map[string]interface{}{
		"type":           "plan_recovery_analyzing",
		"executionId":    executionID,
		"failedStepId":   failedStepID,
		"failedIndex":    failedIndex,
		"completedCount": completedCount,
		"totalSteps":     totalSteps,
		"message":        fmt.Sprintf("Plan failed at step %d/%d, analyzing recovery options...", failedIndex+1, totalSteps),
		"timestamp":      time.Now(),
	})

	return nil
}

// NotifyRecoveryExecuting implements PlanRecoveryCoordinator interface
func (ws *WebServer) NotifyRecoveryExecuting(executionID string, strategy *agent.PlanRecoveryStrategy) error {
	ws.aiAgent.Logger.WithFields(map[string]interface{}{
		"execution_id": executionID,
		"action":       strategy.Action,
		"total_steps":  strategy.TotalSteps,
	}).Info("Notifying UI: Executing recovery plan")

	ws.broadcastUpdate(map[string]interface{}{
		"type":        "plan_recovery_executing",
		"executionId": executionID,
		"action":      strategy.Action,
		"reasoning":   strategy.Reasoning,
		"totalSteps":  strategy.TotalSteps,
		"message":     fmt.Sprintf("Executing recovery plan with %d steps", strategy.TotalSteps),
		"timestamp":   time.Now(),
	})

	return nil
}

// NotifyRecoveryCompleted implements PlanRecoveryCoordinator interface
func (ws *WebServer) NotifyRecoveryCompleted(executionID string) error {
	ws.aiAgent.Logger.WithField("execution_id", executionID).Info("Notifying UI: Recovery completed")

	ws.broadcastUpdate(map[string]interface{}{
		"type":        "plan_recovery_completed",
		"executionId": executionID,
		"message":     "Recovery completed successfully, continuing execution",
		"timestamp":   time.Now(),
	})

	return nil
}

// NotifyRecoveryFailed implements PlanRecoveryCoordinator interface
func (ws *WebServer) NotifyRecoveryFailed(executionID string, reason string) error {
	ws.aiAgent.Logger.WithFields(map[string]interface{}{
		"execution_id": executionID,
		"reason":       reason,
	}).Warn("Notifying UI: Recovery failed")

	ws.broadcastUpdate(map[string]interface{}{
		"type":        "plan_recovery_failed",
		"executionId": executionID,
		"reason":      reason,
		"message":     fmt.Sprintf("Recovery attempt failed: %s", reason),
		"timestamp":   time.Now(),
	})

	return nil
}
