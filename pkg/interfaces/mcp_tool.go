package interfaces

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
)

// MCPTool defines the interface for all MCP (Model Context Protocol) tools
// This enables uniform registration, discovery, and execution of tools by AI agents
type MCPTool interface {
	// Tool identification
	Name() string
	Description() string
	Category() string   // e.g., "ec2", "vpc", "rds", "state-management"
	ActionType() string // e.g., "creation", "modification", "association", "deletion", "query"

	// Tool execution
	Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error)

	// Tool metadata for AI agent discovery
	GetInputSchema() map[string]interface{}
	GetOutputSchema() map[string]interface{}
	GetExamples() []ToolExample

	// Validation
	ValidateArguments(arguments map[string]interface{}) error
}

// ToolExample provides usage examples for AI agents
type ToolExample struct {
	Description string                 `json:"description"`
	Arguments   map[string]interface{} `json:"arguments"`
	Expected    string                 `json:"expected"`
}

// ToolRegistry manages registration and discovery of MCP tools
type ToolRegistry interface {
	// Registration
	Register(tool MCPTool) error
	Unregister(name string) error

	// Discovery
	GetTool(name string) (MCPTool, bool)
	ListTools() []MCPTool
	ListToolsByCategory(category string) []MCPTool

	// Execution
	ExecuteTool(ctx context.Context, name string, arguments map[string]interface{}) (*mcp.CallToolResult, error)

	// AI Agent helpers
	GetToolSchemas() map[string]interface{}
	GetToolsForResourceType(resourceType string) []MCPTool
}

// StateAwareTool extends MCPTool with state management capabilities
type StateAwareTool interface {
	MCPTool

	// State dependencies
	RequiresState() bool
	GetStateDependencies() []string

	// State validation
	ValidateState(state interface{}) error

	// Conflict detection
	DetectConflicts(ctx context.Context, arguments map[string]interface{}) ([]string, error)
}

// ToolFactory creates tools with proper dependencies
type ToolFactory interface {
	CreateTool(toolType string, actionType string, dependencies interface{}) (MCPTool, error)
	GetSupportedToolTypes() map[string][]string
	GetToolActionType(toolName string) string
}
