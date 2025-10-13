package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/interfaces"
	"github.com/versus-control/ai-infrastructure-agent/pkg/tools"
)

// ToolManager manages the integration between adapter-based tools and MCP server
type ToolManager struct {
	registry interfaces.ToolRegistry
}

// NewToolManager creates a new tool manager
func NewToolManager(logger *logging.Logger) *ToolManager {
	registry := tools.NewToolRegistry(logger)

	tm := &ToolManager{
		registry: registry,
	}

	return tm
}

func (tm *ToolManager) Register(tool interfaces.MCPTool) error {
	return tm.registry.Register(tool)
}

// ExecuteTool executes a tool by name with the given arguments
func (tm *ToolManager) ExecuteTool(ctx context.Context, name string, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	tool, exists := tm.registry.GetTool(name)
	if !exists {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				mcp.NewTextContent(fmt.Sprintf("Tool '%s' not found", name)),
			},
		}, nil
	}

	// Execute the tool
	return tool.Execute(ctx, arguments)
}
