package api

import (
	"context"
	"net/http"
	"strings"
	"sync"

	"github.com/versus-control/ai-infrastructure-agent/internal/config"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/agent"
	"github.com/versus-control/ai-infrastructure-agent/pkg/aws"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

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

// ========== HTTP Handlers ==========
// All HTTP handlers have been organized into separate files:
// - handlers_system.go: System operations (health, web UI)
// - handlers_agent.go: AI agent operations (plan, process, execute)
// - handlers_state.go: Infrastructure state operations (state, export)
// - handlers_discovery.go: Infrastructure discovery (discover, graph, conflicts)

// ========== Plan-Level Recovery Coordinator ==========
// recovery_coordinator.go
