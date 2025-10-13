package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/interfaces"
)

// ToolRegistryImpl implements the ToolRegistry interface
type ToolRegistryImpl struct {
	tools    map[string]interfaces.MCPTool
	category map[string][]interfaces.MCPTool
	mutex    sync.RWMutex
	logger   *logging.Logger
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry(logger *logging.Logger) interfaces.ToolRegistry {
	return &ToolRegistryImpl{
		tools:    make(map[string]interfaces.MCPTool),
		category: make(map[string][]interfaces.MCPTool),
		logger:   logger,
	}
}

// Register adds a tool to the registry
func (r *ToolRegistryImpl) Register(tool interfaces.MCPTool) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	name := tool.Name()
	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("tool %s already registered", name)
	}

	r.tools[name] = tool

	// Add to category index
	category := tool.Category()
	r.category[category] = append(r.category[category], tool)

	r.logger.WithField("toolName", name).WithField("category", category).Info("Registered MCP tool")
	return nil
}

// Unregister removes a tool from the registry
func (r *ToolRegistryImpl) Unregister(name string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	tool, exists := r.tools[name]
	if !exists {
		return fmt.Errorf("tool %s not found", name)
	}

	delete(r.tools, name)

	// Remove from category index
	category := tool.Category()
	categoryTools := r.category[category]
	for i, t := range categoryTools {
		if t.Name() == name {
			r.category[category] = append(categoryTools[:i], categoryTools[i+1:]...)
			break
		}
	}

	if len(r.category[category]) == 0 {
		delete(r.category, category)
	}

	r.logger.WithField("toolName", name).Info("Unregistered MCP tool")
	return nil
}

// GetTool retrieves a tool by name
func (r *ToolRegistryImpl) GetTool(name string) (interfaces.MCPTool, bool) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	tool, exists := r.tools[name]
	return tool, exists
}

// ListTools returns all registered tools
func (r *ToolRegistryImpl) ListTools() []interfaces.MCPTool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	var tools []interfaces.MCPTool
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

// ListToolsByCategory returns all tools in a specific category
func (r *ToolRegistryImpl) ListToolsByCategory(category string) []interfaces.MCPTool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	tools, exists := r.category[category]
	if !exists {
		return []interfaces.MCPTool{}
	}

	// Return a copy to prevent modification
	result := make([]interfaces.MCPTool, len(tools))
	copy(result, tools)
	return result
}

// ExecuteTool executes a tool by name
func (r *ToolRegistryImpl) ExecuteTool(ctx context.Context, name string, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	tool, exists := r.GetTool(name)
	if !exists {
		return nil, fmt.Errorf("tool %s not found", name)
	}

	// Validate arguments
	if err := tool.ValidateArguments(arguments); err != nil {
		return nil, fmt.Errorf("argument validation failed: %w", err)
	}

	r.logger.WithField("toolName", name).WithField("arguments", arguments).Info("Executing tool")

	return tool.Execute(ctx, arguments)
}

// GetToolSchemas returns schemas for all registered tools
func (r *ToolRegistryImpl) GetToolSchemas() map[string]interface{} {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	schemas := make(map[string]interface{})
	for name, tool := range r.tools {
		schemas[name] = map[string]interface{}{
			"name":         tool.Name(),
			"description":  tool.Description(),
			"category":     tool.Category(),
			"inputSchema":  tool.GetInputSchema(),
			"outputSchema": tool.GetOutputSchema(),
			"examples":     tool.GetExamples(),
		}
	}

	return schemas
}

// GetToolsForResourceType returns tools that can work with a specific resource type
func (r *ToolRegistryImpl) GetToolsForResourceType(resourceType string) []interfaces.MCPTool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	var matchingTools []interfaces.MCPTool

	// This is a simple implementation - in practice, you might want to add
	// metadata to tools indicating which resource types they work with
	for _, tool := range r.tools {
		// For now, we'll match based on category
		if tool.Category() == resourceType {
			matchingTools = append(matchingTools, tool)
		}
	}

	return matchingTools
}

// BaseTool provides common functionality for MCP tools
type BaseTool struct {
	name        string
	description string
	category    string
	actionType  string
	inputSchema map[string]interface{}
	examples    []interfaces.ToolExample
	logger      *logging.Logger
}

// NewBaseTool creates a new base tool
func NewBaseTool(name, description, category, actionType string, inputSchema map[string]interface{}, logger *logging.Logger) *BaseTool {
	return &BaseTool{
		name:        name,
		description: description,
		category:    category,
		actionType:  actionType,
		inputSchema: inputSchema,
		examples:    []interfaces.ToolExample{},
		logger:      logger,
	}
}

// Name returns the tool name
func (b *BaseTool) Name() string {
	return b.name
}

// Description returns the tool description
func (b *BaseTool) Description() string {
	return b.description
}

// Category returns the tool category
func (b *BaseTool) Category() string {
	return b.category
}

// ActionType returns the tool action type
func (b *BaseTool) ActionType() string {
	return b.actionType
}

// GetInputSchema returns the input schema
func (b *BaseTool) GetInputSchema() map[string]interface{} {
	return b.inputSchema
}

// GetOutputSchema returns a default output schema
func (b *BaseTool) GetOutputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"success": map[string]interface{}{
				"type":        "boolean",
				"description": "Whether the operation was successful",
			},
			"message": map[string]interface{}{
				"type":        "string",
				"description": "Human-readable message about the operation",
			},
			"data": map[string]interface{}{
				"type":        "object",
				"description": "Operation-specific data",
			},
		},
		"required": []string{"success", "message"},
	}
}

// GetExamples returns usage examples
func (b *BaseTool) GetExamples() []interfaces.ToolExample {
	return b.examples
}

// AddExample adds a usage example
func (b *BaseTool) AddExample(description string, arguments map[string]interface{}, expected string) {
	b.examples = append(b.examples, interfaces.ToolExample{
		Description: description,
		Arguments:   arguments,
		Expected:    expected,
	})
}

// GetLogger returns the logger for this tool
func (b *BaseTool) GetLogger() *logging.Logger {
	return b.logger
}

// ValidateArguments provides basic argument validation
func (b *BaseTool) ValidateArguments(arguments map[string]interface{}) error {
	// Basic validation - check required fields based on input schema
	if properties, ok := b.inputSchema["properties"].(map[string]interface{}); ok {
		if required, ok := b.inputSchema["required"].([]interface{}); ok {
			for _, requiredField := range required {
				fieldName := requiredField.(string)
				if _, exists := arguments[fieldName]; !exists {
					return fmt.Errorf("required field %s is missing", fieldName)
				}

				// Check field type if specified
				if fieldSchema, ok := properties[fieldName].(map[string]interface{}); ok {
					if expectedType, ok := fieldSchema["type"].(string); ok {
						if !b.validateType(arguments[fieldName], expectedType) {
							return fmt.Errorf("field %s has invalid type, expected %s", fieldName, expectedType)
						}
					}
				}
			}
		}
	}

	return nil
}

// Helper method to validate types
func (b *BaseTool) validateType(value interface{}, expectedType string) bool {
	switch expectedType {
	case "string":
		_, ok := value.(string)
		return ok
	case "number":
		switch value.(type) {
		case int, int32, int64, float32, float64:
			return true
		default:
			return false
		}
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "object":
		_, ok := value.(map[string]interface{})
		return ok
	case "array":
		_, ok := value.([]interface{})
		return ok
	default:
		return true // Unknown type, assume valid
	}
}

// CreateSuccessResponse creates a standardized success response
func (b *BaseTool) CreateSuccessResponse(message string, data map[string]interface{}) (*mcp.CallToolResult, error) {
	// Create structured response that the agent expects
	response := map[string]interface{}{
		"success": true,
		"message": message,
	}

	// Add data fields to the response
	for key, value := range data {
		response[key] = value
	}

	// Marshal to JSON string for the text content
	jsonBytes, err := json.Marshal(response)
	if err != nil {
		b.logger.Error("Failed to marshal success response", "error", err)
		// Fallback to simple message
		content := []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf(`{"success": true, "message": %q, "error": "failed to marshal response data"}`, message)),
		}
		return &mcp.CallToolResult{
			Content: content,
			IsError: false,
		}, nil
	}

	content := []mcp.Content{
		mcp.NewTextContent(string(jsonBytes)),
	}

	return &mcp.CallToolResult{
		Content: content,
		IsError: false,
	}, nil
}

// CreateErrorResponse creates a standardized error response
func (b *BaseTool) CreateErrorResponse(message string) (*mcp.CallToolResult, error) {
	// Create structured error response that the agent expects
	response := map[string]interface{}{
		"success": false,
		"error":   message,
	}

	// Marshal to JSON string for the text content
	jsonBytes, err := json.Marshal(response)
	if err != nil {
		b.logger.Error("Failed to marshal error response", "error", err)
		// Fallback to simple error message
		content := []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf(`{"success": false, "error": %q}`, message)),
		}
		return &mcp.CallToolResult{
			Content: content,
			IsError: true,
		}, nil
	}

	content := []mcp.Content{
		mcp.NewTextContent(string(jsonBytes)),
	}

	return &mcp.CallToolResult{
		Content: content,
		IsError: true,
	}, nil
}
