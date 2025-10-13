package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/versus-control/ai-infrastructure-agent/internal/config"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/aws"
	"github.com/versus-control/ai-infrastructure-agent/pkg/mcp"
)

func main() {
	// Create context that cancels on interrupt
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger
	logger := logging.NewLogger("info", "text")
	logger.Info("Starting AWS MCP Server...")

	// Initialize AWS client
	awsClient, err := aws.NewClient(cfg.AWS.Region, logger)
	if err != nil {
		logger.WithError(err).Fatal("Failed to initialize AWS client")
	}

	// Test AWS connectivity
	if err := awsClient.HealthCheck(ctx); err != nil {
		logger.WithError(err).Fatal("AWS health check failed")
	}
	logger.Info("AWS connectivity verified")

	// Create our MCP server wrapper (resources are registered automatically)
	mcpServer := mcp.NewServer(cfg, awsClient, logger)

	logger.WithField("server_name", cfg.MCP.ServerName).
		WithField("version", cfg.MCP.Version).
		Info("MCP server configured successfully")

	// Start the server
	logger.Info("Starting MCP server...")
	if err := mcpServer.Start(ctx); err != nil && err != context.Canceled {
		logger.WithError(err).Fatal("Server failed")
	}

	logger.Info("MCP server shutdown complete")
}
