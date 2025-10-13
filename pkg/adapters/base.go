package adapters

import (
	"context"
	"fmt"

	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/aws"
	"github.com/versus-control/ai-infrastructure-agent/pkg/interfaces"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// BaseAWSAdapter provides common functionality for all AWS service adapters
type BaseAWSAdapter struct {
	client       *aws.Client
	logger       *logging.Logger
	resourceType string
}

// NewBaseAWSAdapter creates a new base adapter
func NewBaseAWSAdapter(client *aws.Client, logger *logging.Logger, resourceType string) *BaseAWSAdapter {
	return &BaseAWSAdapter{
		client:       client,
		logger:       logger,
		resourceType: resourceType,
	}
}

// GetResourceType returns the AWS resource type this adapter manages
func (b *BaseAWSAdapter) GetResourceType() string {
	return b.resourceType
}

// HealthCheck verifies AWS connectivity
func (b *BaseAWSAdapter) HealthCheck(ctx context.Context) error {
	return b.client.HealthCheck(ctx)
}

// ValidateParams provides basic parameter validation
func (b *BaseAWSAdapter) ValidateParams(operation string, params interface{}) error {
	if params == nil {
		return fmt.Errorf("parameters cannot be nil for operation %s", operation)
	}
	return nil
}

// ListByTags provides default implementation for tag-based filtering
// Note: This is a fallback implementation. Concrete adapters should override for better performance
func (b *BaseAWSAdapter) ListByTags(ctx context.Context, tags map[string]string) ([]*types.AWSResource, error) {
	return nil, fmt.Errorf("ListByTags not implemented for %s adapter", b.resourceType)
}

// ListByFilter provides default implementation for general filtering
// Note: This is a fallback implementation. Concrete adapters should override for better performance
func (b *BaseAWSAdapter) ListByFilter(ctx context.Context, filters map[string]interface{}) ([]*types.AWSResource, error) {
	return nil, fmt.Errorf("ListByFilter not implemented for %s adapter", b.resourceType)
}

// Helper methods

func (b *BaseAWSAdapter) matchesTags(resourceTags, filterTags map[string]string) bool {
	for key, value := range filterTags {
		if resourceValue, exists := resourceTags[key]; !exists || resourceValue != value {
			return false
		}
	}
	return true
}

func (b *BaseAWSAdapter) matchesFilters(resource *types.AWSResource, filters map[string]interface{}) bool {
	for key, value := range filters {
		switch key {
		case "state":
			if resource.State != fmt.Sprintf("%v", value) {
				return false
			}
		case "type":
			if resource.Type != fmt.Sprintf("%v", value) {
				return false
			}
		case "region":
			if resource.Region != fmt.Sprintf("%v", value) {
				return false
			}
		default:
			// Check in Details map
			if detailValue, exists := resource.Details[key]; !exists || detailValue != value {
				return false
			}
		}
	}
	return true
}

// AdapterRegistry manages all AWS service adapters
type AdapterRegistry struct {
	adapters map[string]interfaces.AWSResourceAdapter
	logger   *logging.Logger
}

// NewAdapterRegistry creates a new adapter registry
func NewAdapterRegistry(logger *logging.Logger) *AdapterRegistry {
	return &AdapterRegistry{
		adapters: make(map[string]interfaces.AWSResourceAdapter),
		logger:   logger,
	}
}

// Register adds an adapter to the registry
func (r *AdapterRegistry) Register(resourceType string, adapter interfaces.AWSResourceAdapter) error {
	if _, exists := r.adapters[resourceType]; exists {
		return fmt.Errorf("adapter for resource type %s already registered", resourceType)
	}

	r.adapters[resourceType] = adapter
	r.logger.WithField("resourceType", resourceType).Info("Registered AWS adapter")
	return nil
}

// GetAdapter retrieves an adapter by resource type
func (r *AdapterRegistry) GetAdapter(resourceType string) (interfaces.AWSResourceAdapter, error) {
	adapter, exists := r.adapters[resourceType]
	if !exists {
		return nil, fmt.Errorf("no adapter registered for resource type: %s", resourceType)
	}
	return adapter, nil
}

// ListAdapters returns all registered adapters
func (r *AdapterRegistry) ListAdapters() map[string]interfaces.AWSResourceAdapter {
	result := make(map[string]interfaces.AWSResourceAdapter)
	for k, v := range r.adapters {
		result[k] = v
	}
	return result
}

// GetSupportedResourceTypes returns all supported resource types
func (r *AdapterRegistry) GetSupportedResourceTypes() []string {
	var types []string
	for resourceType := range r.adapters {
		types = append(types, resourceType)
	}
	return types
}
