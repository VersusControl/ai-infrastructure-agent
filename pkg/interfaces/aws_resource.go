package interfaces

import (
	"context"

	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// AWSResourceManager defines the common interface for all AWS resource operations
// This interface enables uniform handling of different AWS services (EC2, VPC, RDS, etc.)
// and makes it easy for AI agents to discover and use AWS operations consistently.
type AWSResourceManager interface {
	// Core CRUD operations
	Create(ctx context.Context, params interface{}) (*types.AWSResource, error)
	List(ctx context.Context) ([]*types.AWSResource, error)
	Get(ctx context.Context, id string) (*types.AWSResource, error)
	Update(ctx context.Context, id string, params interface{}) (*types.AWSResource, error)
	Delete(ctx context.Context, id string) error

	// Metadata operations
	GetResourceType() string
	GetSupportedOperations() []string
	ValidateParams(operation string, params interface{}) error
}

// AWSResourceAdapter provides base functionality for AWS resource adapters
// Concrete adapters (EC2Adapter, VPCAdapter, etc.) will embed this
type AWSResourceAdapter interface {
	AWSResourceManager

	// Health and connectivity
	HealthCheck(ctx context.Context) error

	// Resource discovery and filtering
	ListByTags(ctx context.Context, tags map[string]string) ([]*types.AWSResource, error)
	ListByFilter(ctx context.Context, filters map[string]interface{}) ([]*types.AWSResource, error)
}

// SpecializedOperations defines service-specific operations that don't fit the CRUD pattern
// Each service can implement additional operations beyond the base interface
type SpecializedOperations interface {
	// Service-specific operations (e.g., StartInstance, StopInstance for EC2)
	ExecuteSpecialOperation(ctx context.Context, operation string, params interface{}) (*types.AWSResource, error)
	GetSpecialOperations() []string
}

// ResourceConverter defines the interface for converting AWS SDK types to our internal format
type ResourceConverter interface {
	ConvertToAWSResource(sdkResource interface{}) *types.AWSResource
	ConvertFromAWSResource(resource *types.AWSResource) (interface{}, error)
}
