package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"os"

	"github.com/versus-control/ai-infrastructure-agent/internal/config"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/aws"
	"github.com/versus-control/ai-infrastructure-agent/pkg/conflict"
	"github.com/versus-control/ai-infrastructure-agent/pkg/discovery"
	"github.com/versus-control/ai-infrastructure-agent/pkg/graph"
	"github.com/versus-control/ai-infrastructure-agent/pkg/state"

	"github.com/mark3labs/mcp-go/server"
)

type Server struct {
	mcpServer *server.MCPServer

	Config           *config.Config
	AWSClient        *aws.Client
	Logger           *logging.Logger
	StateManager     *state.Manager
	DiscoveryScanner *discovery.Scanner
	GraphManager     *graph.Manager
	GraphAnalyzer    *graph.Analyzer
	ConflictResolver *conflict.Resolver
	ToolManager      *ToolManager
}

func NewServer(cfg *config.Config, awsClient *aws.Client, logger *logging.Logger) *Server {
	// Initialize individual components
	stateManager := state.NewManager(cfg.State.FilePath, cfg.AWS.Region, logger)
	discoveryScanner := discovery.NewScanner(awsClient, logger)
	graphManager := graph.NewManager(logger)
	graphAnalyzer := graph.NewAnalyzer(graphManager)
	conflictResolver := conflict.NewResolver(logger)
	toolManager := NewToolManager(logger)

	// Create MCP server
	mcpServer := server.NewMCPServer(
		cfg.MCP.ServerName,
		cfg.MCP.Version,
		server.WithResourceCapabilities(true, true),
		server.WithToolCapabilities(true),
	)

	s := &Server{
		mcpServer: mcpServer,

		Config:           cfg,
		AWSClient:        awsClient,
		Logger:           logger,
		StateManager:     stateManager,
		DiscoveryScanner: discoveryScanner,
		GraphManager:     graphManager,
		GraphAnalyzer:    graphAnalyzer,
		ConflictResolver: conflictResolver,
		ToolManager:      toolManager,
	}

	// Register resources using the new registry-based approach
	s.registerResources()

	// Register modern adapter-based tools (replaces legacy registerTools)
	s.registerServerTools()

	// Load existing state from file
	if err := s.StateManager.LoadState(context.Background()); err != nil {
		logger.WithError(err).Error("Failed to load infrastructure state, continuing with empty state")
		// Don't fail initialization, just log the error and continue with empty state
	}

	return s
}

// Start begins the stdio message loop for the MCP server
func (s *Server) Start(ctx context.Context) error {
	s.Logger.Info("Starting MCP server message loop on stdio...")
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			s.Logger.Info("Shutdown signal received, stopping server")
			return ctx.Err()
		default:
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			// Handle the JSON-RPC message
			response := s.mcpServer.HandleMessage(ctx, line)

			// Write response to stdout
			if response != nil {
				responseBytes, err := json.Marshal(response)
				if err != nil {
					s.Logger.WithError(err).Error("Failed to marshal response")
					continue
				}

				os.Stdout.Write(responseBytes)
				os.Stdout.Write([]byte("\n"))
			}
		}
	}

	if err := scanner.Err(); err != nil {
		s.Logger.WithError(err).Error("Error reading from stdin")
		return err
	}

	return nil
}
