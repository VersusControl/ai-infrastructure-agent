package agent

import (
	"fmt"

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
