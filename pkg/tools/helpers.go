package tools

import (
	"fmt"
	"strings"
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
	fieldResolver    *resources.FieldResolver
	creationPatterns []config.ExtractionPattern
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

	// Get creation patterns directly for step_reference extraction
	creationPatterns := extractionConfig.ResourceIDExtraction.CreationTools.Patterns

	return &StepReferenceResolver{
		fieldResolver:    fieldResolver,
		creationPatterns: creationPatterns,
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
// This uses configuration-driven extraction patterns from creation tools
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

		// Normalize resource type: replace hyphens with underscores
		// (mcp_response uses hyphens like "security-group", but config uses underscores like "security_group")
		resourceType = strings.ReplaceAll(resourceType, "-", "_")

		// Try creation patterns for this resource type
		// step_reference always contains creation results
		if resourceType != "" {
			for _, pattern := range r.creationPatterns {
				if r.matchesResourceType(pattern.ResourceTypes, resourceType) {
					for _, fieldPath := range pattern.FieldPaths {
						if value := r.extractFromPath(mcpResponse, fieldPath); value != "" {
							return value
						}
					}
				}
			}
		}
	}

	return ""
}

// matchesResourceType checks if a resource type matches the pattern's resource types
func (r *StepReferenceResolver) matchesResourceType(patternTypes []string, resourceType string) bool {
	for _, patternType := range patternTypes {
		if patternType == "*" || patternType == resourceType {
			return true
		}
	}
	return false
}

// extractFromPath extracts a value from nested data using dot notation
func (r *StepReferenceResolver) extractFromPath(data map[string]interface{}, path string) string {
	if data == nil {
		return ""
	}

	parts := strings.Split(path, ".")
	current := data

	for _, part := range parts {
		switch v := current[part].(type) {
		case map[string]interface{}:
			current = v
		case string:
			if len(parts) == 1 || part == parts[len(parts)-1] {
				return v
			}
			return ""
		default:
			return ""
		}
	}

	return ""
}

// ExtractResourceIDFromStepReference is a helper function that extracts resource ID from step_reference
// This is a convenience function that uses the global resolver instance
func ExtractResourceIDFromStepReference(stepRef *types.ResourceState) (string, error) {
	resolver := GetStepReferenceResolver()
	if resolver == nil {
		return "", fmt.Errorf("step reference resolver failed to initialize")
	}
	return resolver.ExtractResourceID(stepRef), nil
}
