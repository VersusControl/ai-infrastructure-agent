package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/versus-control/ai-infrastructure-agent/pkg/utilities"
)

// ========== Interface defines ==========

// PromptHelpersInterface defines prompt generation and context building functionality
//
// Available Functions:
//   - getAvailableToolsContext()           : Get formatted string of available MCP tools
//   - generateMCPToolsSchema()             : Generate JSON schema for MCP tools
//   - loadTemplate()                       : Load prompt template from file
//   - loadTemplateWithPlaceholders()       : Load template and replace placeholders
//
// This file handles all prompt generation and context building for AI agent interactions.
// It manages tool descriptions, schemas, and template-based prompt construction.
//
// Usage Example:
//   1. toolsContext := agent.getAvailableToolsContext()
//   2. schema := agent.generateMCPToolsSchema()
//   3. template := agent.loadTemplate("prompts/decision.txt")
//   4. prompt := agent.loadTemplateWithPlaceholders(templatePath, replacements)

// ========== Prompt and Context Helper Functions ==========

// getAvailableToolsContext returns a formatted string of available tools for the AI to understand
func (a *StateAwareAgent) getAvailableToolsContext() (string, error) {
	a.capabilityMutex.RLock()
	toolsCount := len(a.mcpTools)
	a.capabilityMutex.RUnlock()

	if toolsCount == 0 {
		// Try to ensure capabilities are available
		if err := a.ensureMCPCapabilities(); err != nil {
			a.Logger.WithError(err).Error("Failed to ensure MCP capabilities in getAvailableToolsContext")
			return "", fmt.Errorf("failed to ensure MCP capabilities: %w", err)
		}

		// Re-check after ensuring capabilities
		a.capabilityMutex.RLock()
		toolsCount = len(a.mcpTools)
		a.capabilityMutex.RUnlock()
	}

	if toolsCount == 0 {
		return "", fmt.Errorf("no MCP tools discovered - MCP server may not be properly initialized")
	}

	// Generate dynamic MCP tools schema
	mcpToolsSchema := a.generateMCPToolsSchema()

	// Load template with MCP tools placeholder
	placeholders := map[string]string{
		"MCP_TOOLS_SCHEMAS": mcpToolsSchema,
	}

	// Use the new template-based approach
	executionContext, err := a.loadTemplateWithPlaceholders("settings/templates/tools-execution-context-optimized.txt", placeholders)
	if err != nil {
		a.Logger.WithError(err).Error("Failed to load tools execution template with placeholders")
		return "", fmt.Errorf("failed to load tools execution template: %w", err)
	}

	return executionContext, nil
}

// generateMCPToolsSchema generates the dynamic MCP tools schema section
func (a *StateAwareAgent) generateMCPToolsSchema() string {
	a.capabilityMutex.RLock()
	defer a.capabilityMutex.RUnlock()

	var context strings.Builder
	context.WriteString("=== AVAILABLE MCP TOOLS WITH FULL SCHEMAS ===\n\n")
	context.WriteString("You have direct access to these MCP tools. Use the exact tool names and parameter structures shown below.\n\n")

	// Get available categories from pattern matcher configuration
	availableCategories := a.patternMatcher.GetAvailableCategories()

	// Initialize categories with empty slices
	categories := make(map[string][]string)
	for _, category := range availableCategories {
		categories[category] = []string{}
	}

	// Ensure "Other" category exists as fallback
	if _, exists := categories["Other"]; !exists {
		categories["Other"] = []string{}
	}

	toolDetails := make(map[string]string)

	// Categorize tools using pattern matcher
	for toolName, toolInfo := range a.mcpTools {
		// Use pattern matcher to get category based on tool name and resource type patterns
		category := a.patternMatcher.GetCategoryForTool(toolName)

		// Fallback to "Other" if category not found
		if _, exists := categories[category]; !exists {
			category = "Other"
		}

		// Build detailed tool schema
		var toolDetail strings.Builder
		toolDetail.WriteString(fmt.Sprintf("  TOOL: %s\n", toolName))
		toolDetail.WriteString(fmt.Sprintf("  Description: %s\n", toolInfo.Description))

		if toolInfo.InputSchema != nil {
			if properties, ok := toolInfo.InputSchema["properties"].(map[string]interface{}); ok {
				toolDetail.WriteString("  Parameters:\n")

				// Get required fields
				requiredFields := make(map[string]bool)
				if required, ok := toolInfo.InputSchema["required"].([]interface{}); ok {
					for _, field := range required {
						if fieldStr, ok := field.(string); ok {
							requiredFields[fieldStr] = true
						}
					}
				}

				for paramName, paramSchema := range properties {
					if paramSchemaMap, ok := paramSchema.(map[string]interface{}); ok {
						requiredMark := ""
						if requiredFields[paramName] {
							requiredMark = " (REQUIRED)"
						}

						paramType := "string"
						if pType, exists := paramSchemaMap["type"]; exists {
							paramType = fmt.Sprintf("%v", pType)
						}

						description := ""
						if desc, exists := paramSchemaMap["description"]; exists {
							description = fmt.Sprintf(" - %v", desc)
						}

						toolDetail.WriteString(fmt.Sprintf("    - %s: %s%s%s\n", paramName, paramType, requiredMark, description))
					}
				}
			}
		}
		toolDetail.WriteString("\n")

		categories[category] = append(categories[category], toolName)
		toolDetails[toolName] = toolDetail.String()
	}

	// Write categorized tools with full schemas
	for category, tools := range categories {
		if len(tools) > 0 {
			context.WriteString(fmt.Sprintf("=== %s ===\n\n", category))
			for _, toolName := range tools {
				context.WriteString(toolDetails[toolName])
			}
		}
	}

	return context.String()
}

// loadTemplate loads a template file from the filesystem
func (a *StateAwareAgent) loadTemplate(templatePath string) (string, error) {
	// Try to read from the given path first
	data, err := os.ReadFile(templatePath)
	if err != nil {
		// If file not found then try to find project root and read from there
		if os.IsNotExist(err) {
			// Get current working directory
			cwd, _ := os.Getwd()

			// Try to find project root by looking for go.mod
			projectRoot := utilities.FindProjectRoot(cwd)
			if projectRoot != "" {
				absolutePath := filepath.Join(projectRoot, templatePath)
				data, err = os.ReadFile(absolutePath)
				if err == nil {
					return string(data), nil
				}
			}
		}

		return "", fmt.Errorf("failed to read template file %s: %w", templatePath, err)
	}

	return string(data), nil
}

// loadTemplateWithPlaceholders loads a template and processes placeholders
func (a *StateAwareAgent) loadTemplateWithPlaceholders(templatePath string, placeholders map[string]string) (string, error) {
	content, err := a.loadTemplate(templatePath)
	if err != nil {
		return "", err
	}

	// Replace placeholders
	for placeholder, value := range placeholders {
		content = strings.ReplaceAll(content, "{{"+placeholder+"}}", value)
	}

	return content, nil
}
