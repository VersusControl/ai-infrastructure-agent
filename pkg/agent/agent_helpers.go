package agent

import (
	"fmt"
	"strings"

	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// ========== Interface defines ==========

// AgentHelpersInterface defines utility and helper functionality
//
// Available Functions:
//   - extractResourceTypeFromStep()      : Extract resource type from plan step
//   - extractResourceIDFromResponse()    : Extract resource ID from MCP tool response
//
// This file provides utility functions for resource type and ID extraction.
// These helpers support plan execution and resource management operations.
//
// Usage Example:
//   1. resourceType := agent.extractResourceTypeFromStep(planStep)
//   2. resourceID := agent.extractResourceIDFromResponse(result, toolName, stepID)

// ========== Helper Functions ==========

// Helper function to extract resource type from plan step
func (a *StateAwareAgent) extractResourceTypeFromStep(planStep *types.ExecutionPlanStep) string {
	// First try the resource_type parameter
	if rt, exists := planStep.Parameters["resource_type"]; exists {
		if rtStr, ok := rt.(string); ok {
			return rtStr
		}
	}

	// Try to infer from MCP tool name using pattern matcher
	if planStep.MCPTool != "" {
		resourceType := a.patternMatcher.IdentifyResourceTypeFromToolName(planStep.MCPTool)
		if resourceType != "" && resourceType != "unknown" {
			return resourceType
		}
	}

	// Try to infer from ResourceID field using pattern matcher
	if planStep.ResourceID != "" {
		// Use pattern matcher to identify resource type from ID
		resourceType := a.patternMatcher.IdentifyResourceType(planStep)
		if resourceType != "" && resourceType != "unknown" {
			return resourceType
		}
	}

	// Try to infer from step name or description using pattern matcher
	resourceType := a.patternMatcher.InferResourceTypeFromDescription(planStep.Name + " " + planStep.Description)
	if resourceType != "" && resourceType != "unknown" {
		return resourceType
	}

	return ""
}

// extractResourceIDFromResponse extracts the actual AWS resource ID from MCP response
// Note: result should be the processed result with concatenated arrays
func (a *StateAwareAgent) extractResourceIDFromResponse(result map[string]interface{}, toolName string, stepID string) (string, error) {
	// Use configuration-driven extraction for primary resource ID
	resourceType := a.patternMatcher.IdentifyResourceTypeFromToolName(toolName)
	if resourceType == "" || resourceType == "unknown" {
		return "", fmt.Errorf("could not identify resource type for tool %s", toolName)
	}

	extractedID, err := a.idExtractor.ExtractResourceID(toolName, resourceType, nil, result)
	if err != nil {
		a.Logger.WithFields(map[string]interface{}{
			"tool_name":     toolName,
			"resource_type": resourceType,
			"error":         err.Error(),
		}).Error("Failed to extract resource ID using configuration-driven approach")
		return "", fmt.Errorf("could not extract resource ID from MCP response for tool %s: %w", toolName, err)
	}

	if extractedID == "" {
		return "", fmt.Errorf("extracted empty resource ID for tool %s", toolName)
	}

	if a.config.EnableDebug {
		a.Logger.WithFields(map[string]interface{}{
			"tool_name":     toolName,
			"resource_type": resourceType,
			"resource_id":   extractedID,
			"step_id":       stepID,
		}).Info("Successfully extracted resource ID")
	}

	return extractedID, nil
}

// extractUnifiedResourceInfo creates a unified resource information structure from MCP response
// This provides standardized resource information regardless of the tool or resource type
func (a *StateAwareAgent) extractUnifiedResourceInfo(
	planStep *types.ExecutionPlanStep,
	mcpResponse map[string]interface{},
) *types.UnifiedResourceInfo {
	if mcpResponse == nil {
		return nil
	}

	unified := &types.UnifiedResourceInfo{}

	// 1. Extract Resource Type
	resourceType := a.extractResourceTypeFromStep(planStep)

	if resourceType == "" {
		return nil
	}

	unified.ResourceType = resourceType

	// 2. Extract Resource Id
	resourceID, err := a.idExtractor.ExtractResourceID(planStep.MCPTool, resourceType, planStep.ToolParameters, mcpResponse)
	if err != nil {
		a.Logger.WithFields(map[string]interface{}{
			"tool_name":     planStep.MCPTool,
			"resource_type": resourceType,
			"error":         err.Error(),
		}).Warn("Failed to extract resource ID for unified info")

		return nil
	}

	unified.ResourceId = resourceID

	// If resourceId contains "arn:", assign it to ResourceArn
	if strings.HasPrefix(resourceID, "arn:") {
		unified.ResourceArn = resourceID
	}

	// 3. Extract Resource Name
	unified.ResourceName = a.extractResourceName(mcpResponse)

	// 4. Determine Dependency Reference Type and extract IDs
	a.extractDependencyInfo(unified, mcpResponse, unified.ResourceId)

	return unified
}

// extractResourceName extracts a human-readable name from the response
func (a *StateAwareAgent) extractResourceName(mcpResponse map[string]interface{}) string {
	// Try common name fields in priority order

	if name, ok := mcpResponse["name"].(string); ok && name != "" {
		return name
	}

	// Try to extract from Tags["Name"]
	if tags, ok := mcpResponse["tags"].(map[string]interface{}); ok {
		if name, ok := tags["Name"].(string); ok && name != "" {
			return name
		}
	}

	// Try nested resource.tags
	if resource, ok := mcpResponse["resource"].(map[string]interface{}); ok {
		if tags, ok := resource["tags"].(map[string]interface{}); ok {
			if name, ok := tags["Name"].(string); ok && name != "" {
				return name
			}
		}
	}

	// Return empty string if no name found
	return ""
}

// extractDependencyInfo determines how the resource should be referenced and extracts relevant IDs/ARN
func (a *StateAwareAgent) extractDependencyInfo(unified *types.UnifiedResourceInfo, mcpResponse map[string]interface{}, resourceID string) {
	// Check if this is an array resource by looking for fields ending with "_array"
	for field, value := range mcpResponse {
		if strings.HasSuffix(field, "_array") {
			if arrayValue, ok := value.([]interface{}); ok && len(arrayValue) > 0 {
				stringArray := make([]string, 0, len(arrayValue))
				for _, item := range arrayValue {
					if str, ok := item.(string); ok {
						stringArray = append(stringArray, str)
					} else {
						// Convert non-string items to string
						stringArray = append(stringArray, fmt.Sprintf("%v", item))
					}
				}
				unified.ResourceIds = stringArray
				unified.DependencyReferenceType = "array"

				return
			}
		}
	}

	// Default to single resource reference
	unified.DependencyReferenceType = "single"
}
