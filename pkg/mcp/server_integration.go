package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/versus-control/ai-infrastructure-agent/pkg/interfaces"
	"github.com/versus-control/ai-infrastructure-agent/pkg/tools"
)

// registerServerToolsModern registers all modern tools with the MCP server using the factory pattern
func (s *Server) registerServerTools() {
	s.Logger.Info("Registering modern tools with factory pattern")

	// Create factory instance
	factory := tools.NewToolFactory(s.AWSClient, s.Logger)

	// Get all supported tool types from the factory (grouped by action type)
	supportedToolsByAction := factory.GetSupportedToolTypes()

	// Flatten the map to get all tool names
	for actionType, toolNames := range supportedToolsByAction {
		for _, toolName := range toolNames {
			// Create tool instance using factory with basic dependencies
			tool, err := factory.CreateTool(toolName, actionType, &tools.ToolDependencies{
				AWSClient:        s.AWSClient,
				StateManager:     s.StateManager,
				DiscoveryScanner: s.DiscoveryScanner,
				GraphManager:     s.GraphManager,
				GraphAnalyzer:    s.GraphAnalyzer,
				ConflictResolver: s.ConflictResolver,
				Config:           s.Config,
			})
			if err != nil {
				s.Logger.WithField("toolName", toolName).WithError(err).Debug("Skipping tool (may require additional dependencies)")
				continue
			}

			// Register the tool with dynamic registration
			s.registerToolDynamic(tool)

			s.Logger.WithField("toolName", tool.Name()).Debug("Registered tool via factory pattern")
		}
	}
}

// registerToolDynamic registers a single modern tool with the MCP server using dynamic registration
func (s *Server) registerToolDynamic(tool interfaces.MCPTool) {
	name := tool.Name()
	description := tool.Description()

	// Register tool in the tool manager registry
	if err := s.ToolManager.Register(tool); err != nil {
		s.Logger.WithError(err).WithField("toolName", name).Error("Failed to register tool")
		return
	}

	s.Logger.WithField("toolName", name).Debug("Registering modern tool dynamically")

	// Start with basic tool configuration
	mcpOptions := []mcp.ToolOption{mcp.WithDescription(description)}

	// Extract input schema and convert to MCP parameters
	inputSchema := tool.GetInputSchema()
	if inputSchema != nil {
		mcpOptions = append(mcpOptions, s.convertSchemaToMCPOptions(inputSchema)...)
	}

	// Create MCP tool with dynamic parameters
	mcpTool := mcp.NewTool(name, mcpOptions...)

	// Create handler that delegates to tool manager
	handler := s.createToolHandler(name)

	// Register with MCP server
	s.mcpServer.AddTool(mcpTool, handler)

	s.Logger.WithField("toolName", name).Info("Successfully registered modern tool")
}

// convertSchemaToMCPOptions converts JSON Schema to MCP tool options
func (s *Server) convertSchemaToMCPOptions(schema map[string]interface{}) []mcp.ToolOption {
	var options []mcp.ToolOption

	// Extract properties and required fields
	properties, hasProperties := schema["properties"].(map[string]interface{})
	if !hasProperties {
		return options
	}

	required, _ := schema["required"].([]interface{})
	requiredSet := make(map[string]bool)
	for _, req := range required {
		if reqStr, ok := req.(string); ok {
			requiredSet[reqStr] = true
		}
	}

	// Convert each property to MCP parameter
	for propName, propDef := range properties {
		propMap, ok := propDef.(map[string]interface{})
		if !ok {
			continue
		}

		propType, _ := propMap["type"].(string)
		propDesc, _ := propMap["description"].(string)

		// Create parameter options
		var paramOptions []mcp.PropertyOption
		if propDesc != "" {
			paramOptions = append(paramOptions, mcp.Description(propDesc))
		}
		if requiredSet[propName] {
			paramOptions = append(paramOptions, mcp.Required())
		}

		// Add parameter based on type
		switch propType {
		case "string":
			options = append(options, mcp.WithString(propName, paramOptions...))
		case "number", "integer":
			options = append(options, mcp.WithNumber(propName, paramOptions...))
		case "boolean":
			options = append(options, mcp.WithBoolean(propName, paramOptions...))
		case "object":
			options = append(options, mcp.WithObject(propName, paramOptions...))
		case "array":
			options = append(options, mcp.WithArray(propName, paramOptions...))
		default:
			// Default to string for unknown types
			options = append(options, mcp.WithString(propName, paramOptions...))
		}
	}

	return options
}

// createToolHandler creates a handler function that delegates to the tool manager
func (s *Server) createToolHandler(toolName string) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		arguments, ok := request.Params.Arguments.(map[string]interface{})
		if !ok {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					mcp.NewTextContent("Invalid arguments format"),
				},
			}, nil
		}

		s.Logger.WithField("toolName", toolName).WithField("arguments", arguments).Info("Executing modern tool via tool manager")
		return s.ToolManager.ExecuteTool(ctx, toolName, arguments)
	}
}
