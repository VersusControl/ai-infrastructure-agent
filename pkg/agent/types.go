package agent

import (
	"bufio"
	"context"
	"os/exec"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/versus-control/ai-infrastructure-agent/internal/config"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/agent/resources"
	"github.com/versus-control/ai-infrastructure-agent/pkg/aws"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"

	"github.com/tmc/langchaingo/llms"
)

// ========== Interface defines ==========
// Type definitions for agent components and shared data structures
// Core structures: Agent, ToolCall, ResponseData, ToolRegistry
// Communication: MCPRequest/Response, stream processing
// Execution: Command processing, state management

// AgentTypesInterface defines all data structures and types used by the AI agent system
//
// Available Types:
//   - StateAwareAgent             : Main agent struct with LLM, config, and MCP capabilities
//   - MCPProcess                  : Running MCP server process representation
//   - MCPToolInfo                 : Information about available MCP tool capabilities
//   - MCPResourceInfo             : Information about available MCP resources
//   - DecisionContext             : Context data for agent decision-making process
//
// Key Features:
//   - MCP Server Integration      : Direct communication with Model Context Protocol servers
//   - Multi-LLM Support          : OpenAI, Google AI, and other LLM providers
//   - Resource Management        : Track mappings between plan steps and actual AWS resource IDs
//   - Thread-Safe Operations     : Mutex protection for concurrent access
//
// Usage Example:
//   agent := &StateAwareAgent{...}  // Main agent instance
//   context := &DecisionContext{...} // For decision making

// ========== Agent Type Definitions ==========

// MCPProcess represents a running MCP server process
type MCPProcess struct {
	cmd    *exec.Cmd
	stdin  *bufio.Writer
	stdout *bufio.Scanner
	mutex  sync.Mutex
	reqID  int64
}

// StateAwareAgent represents an AI agent with state management capabilities
type StateAwareAgent struct {
	llm       llms.Model
	config    *config.AgentConfig
	awsConfig *config.AWSConfig
	awsClient *aws.Client
	Logger    *logging.Logger

	// MCP properties
	mcpProcess       *MCPProcess
	resourceMappings map[string]string
	mappingsMutex    sync.RWMutex
	mcpTools         map[string]MCPToolInfo
	mcpResources     map[string]MCPResourceInfo
	capabilityMutex  sync.RWMutex

	// Configuration-driven components
	fieldResolver  *resources.FieldResolver
	patternMatcher *resources.PatternMatcher

	// Extractor for resource identification
	extractionConfig *config.ResourceExtractionConfig
	idExtractor      *resources.IDExtractor

	// Test mode flag to bypass real MCP server startup
	testMode bool

	// Mock MCP server for testing (only used when testMode is true)
	mockMCPServer interface {
		CallTool(ctx context.Context, toolName string, arguments map[string]interface{}) (*mcp.CallToolResult, error)
	}
}

// MCPToolInfo represents information about an available MCP tool
type MCPToolInfo struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
	Type        string                 `json:"type"`
}

// MCPResourceInfo represents information about an available MCP resource
type MCPResourceInfo struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description"`
	MimeType    string `json:"mimeType"`
}

// DecisionContext contains context for agent decision-making
type DecisionContext struct {
	Request             string                      `json:"request"`
	CurrentState        *types.InfrastructureState  `json:"current_state"`
	DiscoveredState     []*types.ResourceState      `json:"discovered_state"`
	Conflicts           []*types.ConflictResolution `json:"conflicts"`
	DependencyGraph     *types.DependencyGraph      `json:"dependency_graph"`
	DeploymentOrder     []string                    `json:"deployment_order"`
	ResourceCorrelation map[string]*ResourceMatch   `json:"resource_correlation"`
}

// ResourceMatch represents correlation between managed and discovered resources
type ResourceMatch struct {
	ManagedResource    *types.ResourceState   `json:"managed_resource"`
	DiscoveredResource *types.ResourceState   `json:"discovered_resource"`
	MatchConfidence    float64                `json:"match_confidence"`
	MatchReason        string                 `json:"match_reason"`
	Capabilities       map[string]interface{} `json:"capabilities"`
}
