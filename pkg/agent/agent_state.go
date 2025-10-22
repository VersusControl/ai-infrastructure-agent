package agent

import (
	"fmt"
	"time"

	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// ========== Interface defines ==========

// StateManagementInterface defines state tracking and management functionality
//
// Available Functions:
//   - addStateFromMCPResult()    : Add new resources to state from MCP operation results
//   - updateStateFromMCPResult() : Update existing resources in state from MCP operation results
//   - persistCurrentState()      : Save current state to disk
//
// This file manages infrastructure state tracking throughout the agent lifecycle.
// It handles state updates from MCP operations and maintains resource state consistency.
//
// Usage Example:
//   1. err := agent.addStateFromMCPResult(planStep, mcpResult)
//   2. err := agent.updateStateFromMCPResult(planStep, mcpResult)
//   3. err := agent.persistCurrentState()

// ========== State Management Functions ==========

// addStateFromMCPResult updates the state manager with results from MCP operations
func (a *StateAwareAgent) addStateFromMCPResult(planStep *types.ExecutionPlanStep, result map[string]interface{}) error {
	a.Logger.WithFields(map[string]interface{}{
		"step_id":      planStep.ID,
		"step_name":    planStep.Name,
		"resource_id":  planStep.ResourceID,
		"mcp_response": result,
	}).Info("Starting state update from MCP result")

	// Note: result is already processed (arrays concatenated) from executeNativeMCPTool
	// Create a simple properties map from MCP result
	resultData := map[string]interface{}{
		"mcp_response": result,
		"status":       "created_via_mcp",
	}

	// Extract resource type
	resourceType := a.extractResourceTypeFromStep(planStep)

	// Create a resource state entry
	resourceState := &types.ResourceState{
		ID:           planStep.ResourceID,
		Name:         planStep.Name,
		Description:  planStep.Description,
		Type:         resourceType,
		Status:       "created",
		Properties:   resultData,
		Dependencies: planStep.DependsOn,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	a.Logger.WithFields(map[string]interface{}{
		"step_id":           planStep.ID,
		"resource_state_id": resourceState.ID,
		"resource_type":     resourceState.Type,
		"dependencies":      resourceState.Dependencies,
	}).Info("Calling AddResourceToState for main AWS resource")

	// Add to state manager via MCP server
	if err := a.AddResourceToState(resourceState); err != nil {
		a.Logger.WithError(err).WithFields(map[string]interface{}{
			"step_id":           planStep.ID,
			"resource_state_id": resourceState.ID,
			"resource_type":     resourceState.Type,
		}).Error("Failed to add resource to state via MCP server")
		return fmt.Errorf("failed to add resource %s to state: %w", resourceState.ID, err)
	}

	a.Logger.WithFields(map[string]interface{}{
		"step_id":           planStep.ID,
		"resource_state_id": resourceState.ID,
		"resource_type":     resourceState.Type,
	}).Info("Successfully added main AWS resource to state")

	// Also store the resource with the step ID as the key for dependency resolution
	// This ensures that {{step-create-xxx.resourceId}} references can be resolved
	if planStep.ID != planStep.ResourceID {

		stepResourceState := &types.ResourceState{
			ID:           planStep.ID, // Use step ID as the key
			Name:         planStep.Name,
			Description:  planStep.Description + " (Step Reference)",
			Type:         "step_reference",
			Status:       "created",
			Properties:   resultData,
			Dependencies: planStep.DependsOn,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		if err := a.AddResourceToState(stepResourceState); err != nil {
			if a.config.EnableDebug {
				a.Logger.WithError(err).WithFields(map[string]interface{}{
					"step_id":          planStep.ID,
					"step_resource_id": stepResourceState.ID,
				}).Warn("Failed to add step-based resource to state - dependency resolution may be affected")
			}
			// Don't fail the whole operation for this, just log the warning
		} else {
			a.Logger.WithFields(map[string]interface{}{
				"step_id":          planStep.ID,
				"step_resource_id": stepResourceState.ID,
				"type":             "step_reference",
			}).Info("Successfully added step reference to state for dependency resolution")
		}
	} else {
		a.Logger.WithFields(map[string]interface{}{
			"step_id":     planStep.ID,
			"resource_id": planStep.ResourceID,
			"reason":      "step_id equals resource_id",
		}).Info("Skipping step reference creation - not needed for dependency resolution")
	}

	a.Logger.WithFields(map[string]interface{}{
		"step_id":                planStep.ID,
		"main_resource_id":       resourceState.ID,
		"main_resource_type":     resourceState.Type,
		"step_reference_created": planStep.ID != planStep.ResourceID,
	}).Info("Successfully completed state update from MCP result")

	return nil
}

// updateStateFromMCPResult updates the state manager with results from modify operations
// This ONLY updates existing resources that are already managed by the AI Agent.
// For security, it rejects attempts to modify resources not in the managed state.
func (a *StateAwareAgent) updateStateFromMCPResult(planStep *types.ExecutionPlanStep, result map[string]interface{}) error {
	resourceID := planStep.ResourceID

	// Check if resource exists in current state
	existingResource, err := a.GetResourceFromState(resourceID)

	if err == nil && existingResource != nil {
		// Build properties from MCP result
		resultData := map[string]interface{}{
			"mcp_response": result,
			"status":       "modified_via_mcp",
		}

		// Merge with existing properties (new properties override old)
		mergedProperties := make(map[string]interface{})
		if existingResource.Properties != nil {
			for k, v := range existingResource.Properties {
				mergedProperties[k] = v
			}
		}
		for k, v := range resultData {
			mergedProperties[k] = v
		}

		// Update resource in state
		if err := a.UpdateResourceInState(resourceID, mergedProperties, "modified"); err != nil {
			return fmt.Errorf("failed to update existing resource %s in state: %w", resourceID, err)
		}

	} else {
		return fmt.Errorf("cannot modify resource %s: resource not found in managed state. "+
			"The modify action can only be used on resources created by the AI Agent. "+
			"If you want to create a new resource, use the 'create' action instead. "+
			"If this resource exists in AWS but is not tracked, consider importing it first", resourceID)
	}

	// Also handle step reference
	if planStep.ID != planStep.ResourceID {
		stepResourceState := &types.ResourceState{
			ID:          planStep.ID,
			Name:        planStep.Name,
			Description: planStep.Description + " (Step Reference)",
			Type:        "step_reference",
			Status:      "created",
			Properties: map[string]interface{}{
				"mcp_response": result,
				"status":       "modified_via_mcp",
			},
			Dependencies: planStep.DependsOn,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		if err := a.AddResourceToState(stepResourceState); err != nil {
			a.Logger.WithError(err).Warn("Failed to add step reference - non-critical")
		} else {
			a.Logger.WithField("step_id", planStep.ID).Debug("Added step reference for dependency resolution")
		}
	}

	a.Logger.WithField("resource_id", resourceID).Info("Successfully completed modify state update")
	return nil
}

// persistCurrentState saves the current infrastructure state to persistent storage
// This ensures that successfully completed steps are not lost if later steps fail
func (a *StateAwareAgent) persistCurrentState() error {
	a.Logger.Info("Starting state persistence via MCP server")

	// Use MCP server to save the current state
	result, err := a.callMCPTool("save-state", map[string]interface{}{
		"force": true, // Force save even if state hasn't changed much
	})
	if err != nil {
		a.Logger.WithError(err).Error("Failed to call save-state MCP tool")
		return fmt.Errorf("failed to save state via MCP: %w", err)
	}

	a.Logger.WithField("result", result).Info("State persistence completed successfully via MCP server")
	return nil
}
