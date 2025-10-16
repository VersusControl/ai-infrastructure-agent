package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// ========== Interface defines ==========

// DependencyResolverInterface defines dependency resolution and parameter management functionality
//
// Available Functions:
//   - resolveDependencyReference()      : Resolve references like {{step-1.resourceId}}
//   - resolveDefaultValue()                 : Provide intelligent default values
//   - addMissingRequiredParameters()    : Add defaults for missing required parameters
//   - validateNativeMCPArguments()      : Validate arguments against tool schema
//
// This file handles dependency resolution between plan steps, parameter
// validation, and intelligent default value provisioning for infrastructure operations.
//
// Usage Example:
//   1. resolvedValue := agent.resolveDependencyReference("{{step-1.resourceId}}")
//   2. defaultValue := agent.resolveDefaultValue("create-ec2-instance", "instanceType", params)
//   3. // Use resolved values in infrastructure operations

// ========== Dependency Resolution and Parameter Management Functions ==========

// resolveDependencyReference resolves references like {{step-1.resourceId}} to actual resource IDs
func (a *StateAwareAgent) resolveDependencyReference(reference string) (string, error) {
	a.Logger.WithField("reference", reference).Debug("Starting dependency reference resolution")

	// Extract step ID from reference like {{step-1.resourceId}} or {{step-1.resourceId}}[0]
	if !strings.HasPrefix(reference, "{{") || (!strings.HasSuffix(reference, "}}") && !strings.Contains(reference, "}[")) {
		return reference, nil // Not a reference
	}

	// Handle bracket notation first: {{step-1.resourceId}}[0] -> convert to {{step-1.resourceId.0}}
	if strings.Contains(reference, "}[") {
		// Pattern: {{step-1.resourceId}}[0]
		bracketPos := strings.Index(reference, "}[")
		if bracketPos > 0 {
			beforeBracket := reference[:bracketPos+1] // {{step-1.resourceId}}
			afterBracket := reference[bracketPos+2:]  // 0]

			if strings.HasSuffix(afterBracket, "]") {
				indexStr := strings.TrimSuffix(afterBracket, "]")
				if _, err := strconv.Atoi(indexStr); err == nil {
					// Convert {{step-1.resourceId}}[0] to {{step-1.resourceId.0}}
					convertedRef := strings.TrimSuffix(beforeBracket, "}}") + "." + indexStr + "}}"

					return a.resolveDependencyReference(convertedRef)
				}
			}
		}
	}

	refContent := strings.TrimSuffix(strings.TrimPrefix(reference, "{{"), "}}")

	parts := strings.Split(refContent, ".")

	// Support multiple reference formats: {{step-1.resourceId}}, {{step-1}}, {{step-1.targetGroupArn}}, {{step-1.resourceId.0}}, {{step-1.resourceId.[0]}}, etc.
	var stepID string
	var requestedField string
	var arrayIndex int = -1

	if len(parts) == 3 {
		// Format: {{step-1.resourceId.0}} or {{step-1.resourceId.[0]}} - array indexing
		stepID = parts[0]
		requestedField = parts[1]

		// Handle both formats: "0" and "[0]"
		indexPart := parts[2]
		if strings.HasPrefix(indexPart, "[") && strings.HasSuffix(indexPart, "]") {
			// Format: {{step-1.resourceId.[0]}}
			indexStr := strings.TrimPrefix(strings.TrimSuffix(indexPart, "]"), "[")
			if idx, err := strconv.Atoi(indexStr); err == nil {
				arrayIndex = idx
			} else {
				return "", fmt.Errorf("invalid array index in reference: %s (expected numeric index)", reference)
			}
		} else if idx, err := strconv.Atoi(indexPart); err == nil {
			// Format: {{step-1.resourceId.0}}
			arrayIndex = idx
		} else {
			return "", fmt.Errorf("invalid array index in reference: %s (expected numeric index)", reference)
		}
	} else if len(parts) == 2 {
		stepID = parts[0]
		requestedField = parts[1]
	} else if len(parts) == 1 {
		stepID = parts[0]
		requestedField = "resourceId" // Default to resourceId for backward compatibility
	} else {
		return "", fmt.Errorf("invalid reference format: %s (expected {{step-id.field}}, {{step-id.field.index}}, or {{step-id}})", reference)
	}

	a.mappingsMutex.RLock()

	// Handle array indexing - check for specific indexed mapping first
	if arrayIndex >= 0 {
		indexedKey := fmt.Sprintf("%s.%d", stepID, arrayIndex)
		if indexedValue, indexedExists := a.resourceMappings[indexedKey]; indexedExists {
			a.mappingsMutex.RUnlock()

			return indexedValue, nil
		}
	}

	// Check for field-specific mapping (e.g., step-id.zones)
	if requestedField != "resourceId" {
		fieldKey := fmt.Sprintf("%s.%s", stepID, requestedField)
		if fieldValue, fieldExists := a.resourceMappings[fieldKey]; fieldExists {
			a.mappingsMutex.RUnlock()

			// If this is an array field (concatenated) and we have an index, split it
			if arrayIndex >= 0 && strings.Contains(fieldValue, "_") {
				parts := strings.Split(fieldValue, "_")
				if arrayIndex < len(parts) {
					return parts[arrayIndex], nil
				}
				return "", fmt.Errorf("array index %d out of bounds for field %s (length: %d)", arrayIndex, requestedField, len(parts))
			}

			return fieldValue, nil
		}
	}

	// Check for primary resource ID mapping
	resourceID, exists := a.resourceMappings[stepID]
	a.mappingsMutex.RUnlock()

	// If mapping exists return it directly
	if exists {
		return resourceID, nil
	}

	// For all other cases (no mapping exists OR specific field requested), resolve from infrastructure state
	if a.testMode && !exists {
		// In test mode, avoid accessing real state - rely only on stored mappings
		return "", fmt.Errorf("dependency reference not found in test mode: %s (step ID: %s not found in resource mappings)", reference, stepID)
	}

	// Resolve from infrastructure state (handles both missing mappings and specific field requests)
	resolvedID, err := a.resolveFromInfrastructureState(stepID, requestedField, reference, arrayIndex)
	if err != nil {
		return "", fmt.Errorf("failed to resolve dependency %s for step %s: %w", reference, stepID, err)
	}

	return resolvedID, nil
}

// resolveFromInfrastructureState attempts to resolve a dependency reference by parsing the infrastructure state
func (a *StateAwareAgent) resolveFromInfrastructureState(stepID, requestedField, reference string, arrayIndex int) (string, error) {
	// Parse the state and look for the step ID
	stateJSON, err := a.ExportInfrastructureState(context.Background(), false) // Only managed state
	if err != nil {
		return "", fmt.Errorf("failed to export infrastructure state: %w", err)
	}

	var stateData map[string]interface{}
	if err := json.Unmarshal([]byte(stateJSON), &stateData); err != nil {
		return "", fmt.Errorf("failed to parse state JSON: %w", err)
	}

	managedState, ok := stateData["managed_state"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("managed_state not found in state data")
	}

	resources, ok := managedState["resources"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("resources not found in managed_state")
	}

	resource, ok := resources[stepID].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("resource not found for step ID: %s", stepID)
	}

	// Extract AWS resource ID from the resource properties
	properties, ok := resource["properties"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("properties not found in resource")
	}

	mcpResponse, ok := properties["mcp_response"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("mcp_response not found in properties")
	}

	// Handle array indexing for the requested field
	if arrayIndex >= 0 {
		// First, check if we have a concatenated string value for this field
		if concatenatedValue, ok := mcpResponse[requestedField].(string); ok && concatenatedValue != "" {
			// Check if it looks like a concatenated array (contains _)
			if strings.Contains(concatenatedValue, "_") {
				// Split by _ delimiter to get array elements
				parts := strings.Split(concatenatedValue, "_")

				if arrayIndex < len(parts) {
					id := parts[arrayIndex]

					// Cache it for future use
					indexedKey := fmt.Sprintf("%s.%d", stepID, arrayIndex)
					a.mappingsMutex.Lock()
					a.resourceMappings[indexedKey] = id
					a.mappingsMutex.Unlock()

					a.Logger.WithFields(map[string]interface{}{
						"reference":       reference,
						"step_id":         stepID,
						"resource_id":     id,
						"source":          "state_concatenated_array",
						"requested_field": requestedField,
						"array_index":     arrayIndex,
						"total_elements":  len(parts),
					}).Info("Resolved indexed dependency from concatenated array")

					return id, nil
				} else {
					return "", fmt.Errorf("array index %d out of bounds for concatenated field %s (length: %d)", arrayIndex, requestedField, len(parts))
				}
			}
		}

		// Fallback: try to find the field as an original array (for backward compatibility)
		if arrayField, ok := mcpResponse[requestedField+"_array"].([]interface{}); ok {
			if arrayIndex < len(arrayField) {
				if id, ok := arrayField[arrayIndex].(string); ok && id != "" {
					// Cache it for future use
					indexedKey := fmt.Sprintf("%s.%d", stepID, arrayIndex)
					a.mappingsMutex.Lock()
					a.resourceMappings[indexedKey] = id
					a.mappingsMutex.Unlock()

					a.Logger.WithFields(map[string]interface{}{
						"reference":       reference,
						"step_id":         stepID,
						"resource_id":     id,
						"source":          "state_original_array",
						"requested_field": requestedField,
						"array_index":     arrayIndex,
					}).Info("Resolved array field dependency from original array (backward compatibility)")

					return id, nil
				}
			} else {
				return "", fmt.Errorf("array index %d out of bounds for field %s (length: %d)", arrayIndex, requestedField, len(arrayField))
			}
		}

		return "", fmt.Errorf("array field %s[%d] not found or invalid in mcp_response", requestedField, arrayIndex)
	}

	// Try to find the field directly in the MCP response
	if fieldValue, exists := mcpResponse[requestedField]; exists {
		// Handle string field
		if id, ok := fieldValue.(string); ok && id != "" {
			// Cache it for future use
			a.mappingsMutex.Lock()
			a.resourceMappings[stepID] = id
			a.mappingsMutex.Unlock()

			a.Logger.WithFields(map[string]interface{}{
				"reference":       reference,
				"step_id":         stepID,
				"resource_id":     id,
				"source":          "state_direct_field",
				"requested_field": requestedField,
			}).Info("Resolved field dependency directly from state")

			return id, nil
		}

		// Handle array field - serialize to JSON for use in other tools
		if arrayValue, ok := fieldValue.([]interface{}); ok && len(arrayValue) > 0 {
			jsonBytes, err := json.Marshal(arrayValue)
			if err == nil {
				jsonStr := string(jsonBytes)

				// Cache it for future use
				a.mappingsMutex.Lock()
				a.resourceMappings[stepID] = jsonStr
				a.mappingsMutex.Unlock()

				a.Logger.WithFields(map[string]interface{}{
					"reference":       reference,
					"step_id":         stepID,
					"resource_id":     jsonStr,
					"source":          "state_array_field",
					"requested_field": requestedField,
					"array_length":    len(arrayValue),
				}).Info("Resolved array field dependency from state")

				return jsonStr, nil
			}
		}
	}

	// Use configuration-driven field resolver with resource type detection
	fieldsToTry := a.fieldResolver.GetFieldsForRequestWithContext(requestedField, mcpResponse)

	for _, field := range fieldsToTry {
		if id, ok := mcpResponse[field].(string); ok && id != "" {
			// Cache it for future use
			a.mappingsMutex.Lock()
			a.resourceMappings[stepID] = id
			a.mappingsMutex.Unlock()

			return id, nil
		}
	}

	return "", fmt.Errorf("field %s not found in mcp_response for step %s", requestedField, stepID)
}

// validateNativeMCPArguments validates arguments against the tool's schema
func (a *StateAwareAgent) validateNativeMCPArguments(toolName string, arguments map[string]interface{}, toolInfo MCPToolInfo) error {
	if toolInfo.InputSchema == nil {
		return nil // No schema to validate against
	}

	properties, ok := toolInfo.InputSchema["properties"].(map[string]interface{})
	if !ok {
		return nil
	}

	// Get required fields
	requiredFields := make(map[string]bool)
	if required, ok := toolInfo.InputSchema["required"].([]interface{}); ok {
		for _, field := range required {
			if fieldStr, ok := field.(string); ok {
				requiredFields[fieldStr] = true
			}
		}
	}

	// Validate required fields are present and non-empty
	for paramName := range properties {
		if requiredFields[paramName] {
			val, exists := arguments[paramName]
			if !exists || val == nil {
				return fmt.Errorf("required parameter %s is missing for tool %s", paramName, toolName)
			}
			// Check for empty strings
			if strVal, ok := val.(string); ok && strVal == "" {
				return fmt.Errorf("required parameter %s is empty for tool %s", paramName, toolName)
			}
		}
	}

	return nil
}
