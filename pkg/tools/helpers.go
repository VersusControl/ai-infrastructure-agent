package tools

import (
	"sync"

	"github.com/versus-control/ai-infrastructure-agent/internal/config"
	"github.com/versus-control/ai-infrastructure-agent/pkg/agent/resources"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

var (
	// Global resolver instance for step reference resolution
	globalResolver     *StepReferenceResolver
	globalResolverOnce sync.Once
)

// StepReferenceResolver handles resolution of step references to actual resource IDs
type StepReferenceResolver struct {
	fieldResolver *resources.FieldResolver
	idExtractor   *resources.IDExtractor
}

// newStepReferenceResolver creates a new step reference resolver
func newStepReferenceResolver() (*StepReferenceResolver, error) {
	// Load field mapping configuration to reuse the extraction logic
	configLoader := config.NewConfigLoader("./settings")

	fieldMappingConfig, err := configLoader.LoadFieldMappings()
	if err != nil {
		return nil, err
	}

	// Load extraction configuration
	extractionConfig, err := configLoader.LoadResourceExtraction()
	if err != nil {
		return nil, err
	}

	fieldResolver := resources.NewFieldResolver(fieldMappingConfig)

	idExtractor, err := resources.NewIDExtractor(extractionConfig)
	if err != nil {
		return nil, err
	}

	return &StepReferenceResolver{
		fieldResolver: fieldResolver,
		idExtractor:   idExtractor,
	}, nil
}

// GetStepReferenceResolver returns a singleton instance of the step reference resolver
func GetStepReferenceResolver() *StepReferenceResolver {
	globalResolverOnce.Do(func() {
		resolver, err := newStepReferenceResolver()
		if err != nil {
			// If initialization fails, create a nil resolver
			// The extraction function will handle this gracefully
			globalResolver = nil
		} else {
			globalResolver = resolver
		}
	})
	return globalResolver
}

// ExtractResourceID extracts the actual AWS resource ID from a step_reference resource
// This uses the IDExtractor's configuration-driven extraction patterns
func (r *StepReferenceResolver) ExtractResourceID(stepRef *types.ResourceState) string {
	if stepRef == nil || stepRef.Type != "step_reference" {
		return ""
	}

	// Try to extract resource ID from properties.mcp_response
	if mcpResponse, ok := stepRef.Properties["mcp_response"].(map[string]interface{}); ok {
		// Determine resource type from the mcp_response
		var resourceType string
		if resType, ok := mcpResponse["type"].(string); ok {
			resourceType = resType
		} else if resource, ok := mcpResponse["resource"].(map[string]interface{}); ok {
			if resType, ok := resource["type"].(string); ok {
				resourceType = resType
			}
		}

		// Use IDExtractor with creation patterns (step_reference contains creation results)
		// We'll use a dummy tool name since we're extracting from stored results
		if resourceType != "" {
			// Try to extract using configured patterns for this resource type
			// We look at creation_tools patterns which contain all the field paths
			extractedID, err := r.idExtractor.ExtractResourceID("", resourceType, nil, mcpResponse)
			if err == nil && extractedID != "" {
				return extractedID
			}
		}

		// Fallback: try nested resource.id field using FieldResolver
		value := r.fieldResolver.ExtractFromPath(mcpResponse, "resource.id")
		if strValue, ok := value.(string); ok && strValue != "" {
			return strValue
		}
	}

	// Fallback: check if there's a direct resource_id field
	if resourceID, ok := stepRef.Properties["resource_id"].(string); ok && resourceID != "" {
		return resourceID
	}

	return ""
}

// ExtractResourceIDFromStepReference is a helper function that extracts resource ID from step_reference
// This is a convenience function that uses the global resolver instance
func ExtractResourceIDFromStepReference(stepRef *types.ResourceState) string {
	resolver := GetStepReferenceResolver()
	if resolver == nil {
		// Fallback: try direct extraction without resolver
		return extractResourceIDFallback(stepRef)
	}
	return resolver.ExtractResourceID(stepRef)
}

// extractResourceIDFallback provides a simple fallback when resolver is not available
func extractResourceIDFallback(stepRef *types.ResourceState) string {
	if stepRef == nil || stepRef.Type != "step_reference" {
		return ""
	}

	// Try to extract from common fields in mcp_response
	if mcpResponse, ok := stepRef.Properties["mcp_response"].(map[string]interface{}); ok {
		// Try direct ID fields
		idFields := []string{
			"subnetId", "vpcId", "instanceId", "groupId", "gatewayId",
			"natGatewayId", "routeTableId", "allocationId", "associationId",
		}

		for _, field := range idFields {
			if id, ok := mcpResponse[field].(string); ok && id != "" {
				return id
			}
		}

		// Try nested resource.id
		if resource, ok := mcpResponse["resource"].(map[string]interface{}); ok {
			if id, ok := resource["id"].(string); ok && id != "" {
				return id
			}
		}
	}

	return ""
}
