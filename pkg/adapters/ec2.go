package adapters

import (
	"context"
	"fmt"

	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/aws"
	"github.com/versus-control/ai-infrastructure-agent/pkg/interfaces"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// EC2Adapter implements the AWSResourceAdapter interface for EC2 instances
type EC2Adapter struct {
	*BaseAWSAdapter
	client *aws.Client
}

// NewEC2Adapter creates a new EC2 adapter
func NewEC2Adapter(client *aws.Client, logger *logging.Logger) interfaces.AWSResourceAdapter {
	base := NewBaseAWSAdapter(client, logger, "ec2-instance")
	return &EC2Adapter{
		BaseAWSAdapter: base,
		client:         client,
	}
}

// Create creates a new EC2 instance
func (e *EC2Adapter) Create(ctx context.Context, params interface{}) (*types.AWSResource, error) {
	createParams, ok := params.(aws.CreateInstanceParams)
	if !ok {
		return nil, fmt.Errorf("invalid parameters for EC2 instance creation, expected aws.CreateInstanceParams")
	}

	return e.client.CreateEC2Instance(ctx, createParams)
}

// List returns all EC2 instances
func (e *EC2Adapter) List(ctx context.Context) ([]*types.AWSResource, error) {
	return e.client.ListEC2Instances(ctx)
}

// Get retrieves a specific EC2 instance
func (e *EC2Adapter) Get(ctx context.Context, id string) (*types.AWSResource, error) {
	return e.client.GetEC2Instance(ctx, id)
}

// Update updates an EC2 instance (limited operations available)
func (e *EC2Adapter) Update(ctx context.Context, id string, params interface{}) (*types.AWSResource, error) {
	// EC2 instances have limited update operations
	// For now, we'll return an error - specific operations should use specialized methods
	return nil, fmt.Errorf("EC2 instance updates should use specialized operations (start, stop, modify attributes)")
}

// Delete terminates an EC2 instance
func (e *EC2Adapter) Delete(ctx context.Context, id string) error {
	return e.client.TerminateEC2Instance(ctx, id)
}

// GetSupportedOperations returns the operations supported by this adapter
func (e *EC2Adapter) GetSupportedOperations() []string {
	return []string{
		"create",
		"list",
		"get",
		"delete",
		"start",
		"stop",
		"terminate",
		"create-ami",
	}
}

// ListByTags overrides the base implementation with EC2-specific logic
func (e *EC2Adapter) ListByTags(ctx context.Context, tags map[string]string) ([]*types.AWSResource, error) {
	// Get all instances and filter by tags
	allInstances, err := e.client.ListEC2Instances(ctx)
	if err != nil {
		return nil, err
	}

	var filtered []*types.AWSResource
	for _, instance := range allInstances {
		if e.matchesTags(instance.Tags, tags) {
			filtered = append(filtered, instance)
		}
	}

	return filtered, nil
}

// ListByFilter overrides the base implementation with EC2-specific logic
func (e *EC2Adapter) ListByFilter(ctx context.Context, filters map[string]interface{}) ([]*types.AWSResource, error) {
	// Get all instances and filter
	allInstances, err := e.client.ListEC2Instances(ctx)
	if err != nil {
		return nil, err
	}

	var filtered []*types.AWSResource
	for _, instance := range allInstances {
		if e.matchesFilters(instance, filters) {
			filtered = append(filtered, instance)
		}
	}

	return filtered, nil
}

// ValidateParams validates EC2-specific parameters
func (e *EC2Adapter) ValidateParams(operation string, params interface{}) error {
	switch operation {
	case "create":
		createParams, ok := params.(aws.CreateInstanceParams)
		if !ok {
			return fmt.Errorf("invalid parameters for create operation")
		}
		if createParams.ImageID == "" {
			return fmt.Errorf("imageId is required for EC2 instance creation")
		}
		if createParams.InstanceType == "" {
			return fmt.Errorf("instanceType is required for EC2 instance creation")
		}
		return nil
	case "start", "stop", "terminate", "get", "delete":
		// These operations need an instance ID
		if params == nil {
			return fmt.Errorf("instance ID is required for %s operation", operation)
		}
		return nil
	default:
		return fmt.Errorf("unsupported operation: %s", operation)
	}
}

// EC2SpecializedAdapter adds EC2-specific operations
type EC2SpecializedAdapter struct {
	interfaces.AWSResourceAdapter
	client *aws.Client
}

// NewEC2SpecializedAdapter creates an adapter with specialized EC2 operations
func NewEC2SpecializedAdapter(client *aws.Client, logger *logging.Logger) interfaces.SpecializedOperations {
	baseAdapter := NewEC2Adapter(client, logger)
	return &EC2SpecializedAdapter{
		AWSResourceAdapter: baseAdapter,
		client:             client,
	}
}

// ExecuteSpecialOperation handles EC2-specific operations
func (e *EC2SpecializedAdapter) ExecuteSpecialOperation(ctx context.Context, operation string, params interface{}) (*types.AWSResource, error) {
	switch operation {
	case "start":
		instanceID, ok := params.(string)
		if !ok {
			return nil, fmt.Errorf("instance ID required for start operation")
		}
		err := e.client.StartEC2Instance(ctx, instanceID)
		if err != nil {
			return nil, err
		}
		return e.client.GetEC2Instance(ctx, instanceID)

	case "stop":
		instanceID, ok := params.(string)
		if !ok {
			return nil, fmt.Errorf("instance ID required for stop operation")
		}
		err := e.client.StopEC2Instance(ctx, instanceID)
		if err != nil {
			return nil, err
		}
		return e.client.GetEC2Instance(ctx, instanceID)

	case "modify-instance-type":
		modifyParams, ok := params.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("modify parameters required for modify-instance-type operation")
		}
		instanceID, _ := modifyParams["instanceId"].(string)
		instanceType, _ := modifyParams["instanceType"].(string)

		if instanceID == "" || instanceType == "" {
			return nil, fmt.Errorf("instanceId and instanceType are required for instance type modification")
		}

		err := e.client.ModifyEC2InstanceType(ctx, instanceID, instanceType)
		if err != nil {
			return nil, err
		}
		return e.client.GetEC2Instance(ctx, instanceID)

	case "create-ami":
		amiParams, ok := params.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("AMI parameters required")
		}
		instanceID, _ := amiParams["instanceId"].(string)
		name, _ := amiParams["name"].(string)
		description, _ := amiParams["description"].(string)

		if instanceID == "" || name == "" {
			return nil, fmt.Errorf("instanceId and name are required for AMI creation")
		}

		return e.client.CreateAMI(ctx, instanceID, name, description)

	case "list-launch-templates":
		// List all Launch Templates
		templates, err := e.client.DescribeLaunchTemplates(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list launch templates: %w", err)
		}

		// Since this operation returns multiple resources, we'll return the first one
		// or a summary resource. For listing operations, we might want to handle this differently
		if len(templates) == 0 {
			// Return a summary resource indicating no templates found
			return &types.AWSResource{
				ID:    "no-templates",
				Type:  "launch-template-list",
				State: "empty",
				Details: map[string]interface{}{
					"count":     0,
					"templates": templates,
				},
			}, nil
		}

		// Return a summary resource with all templates
		return &types.AWSResource{
			ID:    "launch-template-list",
			Type:  "launch-template-list",
			State: "available",
			Details: map[string]interface{}{
				"count":     len(templates),
				"templates": templates,
			},
		}, nil

	case "get-latest-amazon-linux-ami":
		// Get latest Amazon Linux 2 AMI
		amiParams, ok := params.(map[string]interface{})
		if !ok {
			amiParams = map[string]interface{}{"architecture": "x86_64"}
		}

		architecture, ok := amiParams["architecture"].(string)
		if !ok {
			architecture = "x86_64"
		}

		amiID, err := e.client.GetLatestAmazonLinux2AMI(ctx, architecture)
		if err != nil {
			return nil, fmt.Errorf("failed to get latest Amazon Linux 2 AMI: %w", err)
		}

		return &types.AWSResource{
			ID:    amiID,
			Type:  "ami",
			State: "available",
			Details: map[string]interface{}{
				"amiId":        amiID,
				"architecture": architecture,
				"description":  "Amazon Linux 2 AMI (HVM) - Kernel 5.x, SSD Volume Type",
				"osType":       "Linux",
				"platform":     "Amazon Linux 2",
			},
		}, nil

	case "get-latest-ubuntu-ami":
		// Get latest Ubuntu LTS AMI
		amiParams, ok := params.(map[string]interface{})
		if !ok {
			amiParams = map[string]interface{}{"architecture": "x86_64"}
		}

		architecture, ok := amiParams["architecture"].(string)
		if !ok {
			architecture = "x86_64"
		}

		amiID, err := e.client.GetLatestUbuntuAMI(ctx, architecture)
		if err != nil {
			return nil, fmt.Errorf("failed to get latest Ubuntu LTS AMI: %w", err)
		}

		return &types.AWSResource{
			ID:    amiID,
			Type:  "ami",
			State: "available",
			Details: map[string]interface{}{
				"amiId":        amiID,
				"architecture": architecture,
				"description":  "Ubuntu Server LTS",
				"osType":       "Linux",
				"platform":     "Ubuntu",
			},
		}, nil

	case "get-latest-windows-ami":
		// Get latest Windows Server AMI
		amiParams, ok := params.(map[string]interface{})
		if !ok {
			amiParams = map[string]interface{}{"architecture": "x86_64"}
		}

		architecture, ok := amiParams["architecture"].(string)
		if !ok {
			architecture = "x86_64"
		}

		amiID, err := e.client.GetLatestWindowsAMI(ctx, architecture)
		if err != nil {
			return nil, fmt.Errorf("failed to get latest Windows Server AMI: %w", err)
		}

		return &types.AWSResource{
			ID:    amiID,
			Type:  "ami",
			State: "available",
			Details: map[string]interface{}{
				"amiId":        amiID,
				"architecture": architecture,
				"description":  "Microsoft Windows Server",
				"osType":       "Windows",
				"platform":     "Windows Server",
			},
		}, nil

	default:
		return nil, fmt.Errorf("unsupported specialized operation: %s", operation)
	}
}

// GetSpecialOperations returns the specialized operations available
func (e *EC2SpecializedAdapter) GetSpecialOperations() []string {
	return []string{"start", "stop", "create-ami", "list-launch-templates", "get-latest-amazon-linux-ami", "get-latest-ubuntu-ami", "get-latest-windows-ami"}
}

// ListAMIs lists AMIs using the adapter pattern
func (e *EC2Adapter) ListAMIs(ctx context.Context, owner string) ([]*types.AWSResource, error) {
	return e.client.ListAMIs(ctx, owner)
}
