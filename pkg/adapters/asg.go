package adapters

import (
	"context"
	"fmt"

	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/aws"
	"github.com/versus-control/ai-infrastructure-agent/pkg/interfaces"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// ASGAdapter implements the AWSResourceAdapter interface for Auto Scaling Groups
type ASGAdapter struct {
	*BaseAWSAdapter
	client *aws.Client
}

// NewASGAdapter creates a new Auto Scaling Group adapter
func NewASGAdapter(client *aws.Client, logger *logging.Logger) interfaces.AWSResourceAdapter {
	base := NewBaseAWSAdapter(client, logger, "asg")
	return &ASGAdapter{
		BaseAWSAdapter: base,
		client:         client,
	}
}

// Create creates a new Auto Scaling Group
func (a *ASGAdapter) Create(ctx context.Context, params interface{}) (*types.AWSResource, error) {
	createParams, ok := params.(aws.CreateAutoScalingGroupParams)
	if !ok {
		return nil, fmt.Errorf("invalid parameters for ASG creation, expected aws.CreateAutoScalingGroupParams")
	}

	asg, err := a.client.CreateAutoScalingGroup(ctx, createParams)
	if err != nil {
		return nil, err
	}

	return asg, nil
}

// List returns all Auto Scaling Groups
func (a *ASGAdapter) List(ctx context.Context) ([]*types.AWSResource, error) {
	return a.client.DescribeAutoScalingGroups(ctx)
}

// Get retrieves a specific Auto Scaling Group
func (a *ASGAdapter) Get(ctx context.Context, id string) (*types.AWSResource, error) {
	return a.client.GetAutoScalingGroup(ctx, id)
}

// Update updates an Auto Scaling Group (only desired capacity supported)
func (a *ASGAdapter) Update(ctx context.Context, id string, params interface{}) (*types.AWSResource, error) {
	capacityParams, ok := params.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid parameters for ASG update, expected capacity parameters")
	}

	desiredCapacity, ok := capacityParams["desiredCapacity"].(int32)
	if !ok {
		return nil, fmt.Errorf("desiredCapacity is required for ASG update")
	}

	err := a.client.UpdateAutoScalingGroup(ctx, id, desiredCapacity)
	if err != nil {
		return nil, err
	}

	// Return the updated ASG
	return a.Get(ctx, id)
}

// Delete deletes an Auto Scaling Group
func (a *ASGAdapter) Delete(ctx context.Context, id string) error {
	return a.client.DeleteAutoScalingGroup(ctx, id, true) // Force delete
}

// GetSupportedOperations returns the operations supported by this adapter
func (a *ASGAdapter) GetSupportedOperations() []string {
	return []string{
		"create",
		"list",
		"get",
		"update",
		"delete",
		"scale-out",
		"scale-in",
		"set-desired-capacity",
	}
}

// ValidateParams validates ASG-specific parameters
func (a *ASGAdapter) ValidateParams(operation string, params interface{}) error {
	switch operation {
	case "create":
		createParams, ok := params.(aws.CreateAutoScalingGroupParams)
		if !ok {
			return fmt.Errorf("invalid parameters for create operation")
		}
		if createParams.AutoScalingGroupName == "" {
			return fmt.Errorf("autoScalingGroupName is required for ASG creation")
		}
		if createParams.MinSize < 0 || createParams.MaxSize < 0 || createParams.DesiredCapacity < 0 {
			return fmt.Errorf("size parameters must be non-negative")
		}
		if createParams.MinSize > createParams.MaxSize {
			return fmt.Errorf("minSize cannot be greater than maxSize")
		}
		return nil
	case "update":
		capacityParams, ok := params.(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid parameters for update operation")
		}
		desiredCapacity, ok := capacityParams["desiredCapacity"].(int32)
		if !ok {
			return fmt.Errorf("desiredCapacity is required for ASG update")
		}
		if desiredCapacity < 0 {
			return fmt.Errorf("desiredCapacity must be non-negative")
		}
		return nil
	case "get", "delete":
		if params == nil {
			return fmt.Errorf("AutoScaling Group name is required for %s operation", operation)
		}
		return nil
	default:
		return fmt.Errorf("unsupported operation: %s", operation)
	}
}

// ASGSpecializedAdapter adds ASG-specific operations
type ASGSpecializedAdapter struct {
	interfaces.AWSResourceAdapter
	client *aws.Client
}

// NewASGSpecializedAdapter creates an adapter with specialized ASG operations
func NewASGSpecializedAdapter(client *aws.Client, logger *logging.Logger) interfaces.SpecializedOperations {
	baseAdapter := NewASGAdapter(client, logger)
	return &ASGSpecializedAdapter{
		AWSResourceAdapter: baseAdapter,
		client:             client,
	}
}

// ExecuteSpecialOperation handles ASG-specific operations
func (a *ASGSpecializedAdapter) ExecuteSpecialOperation(ctx context.Context, operation string, params interface{}) (*types.AWSResource, error) {
	switch operation {
	case "create-launch-template":
		templateParams, ok := params.(aws.CreateLaunchTemplateParams)
		if !ok {
			return nil, fmt.Errorf("launch template parameters required for create-launch-template operation")
		}

		return a.client.CreateLaunchTemplate(ctx, templateParams)

	case "scale-out":
		scaleParams, ok := params.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("scale parameters required")
		}

		asgName, _ := scaleParams["autoScalingGroupName"].(string)
		increment, _ := scaleParams["increment"].(int32)

		if asgName == "" {
			return nil, fmt.Errorf("autoScalingGroupName is required for scale-out operation")
		}
		if increment <= 0 {
			increment = 1 // Default increment
		}

		// Get current ASG to determine current capacity
		currentASG, err := a.client.GetAutoScalingGroup(ctx, asgName)
		if err != nil {
			return nil, err
		}

		currentCapacity, ok := currentASG.Details["desiredCapacity"].(int32)
		if !ok {
			return nil, fmt.Errorf("unable to determine current desired capacity")
		}

		newCapacity := currentCapacity + increment
		err = a.client.UpdateAutoScalingGroup(ctx, asgName, newCapacity)
		if err != nil {
			return nil, err
		}

		return a.client.GetAutoScalingGroup(ctx, asgName)

	case "scale-in":
		scaleParams, ok := params.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("scale parameters required")
		}

		asgName, _ := scaleParams["autoScalingGroupName"].(string)
		decrement, _ := scaleParams["decrement"].(int32)

		if asgName == "" {
			return nil, fmt.Errorf("autoScalingGroupName is required for scale-in operation")
		}
		if decrement <= 0 {
			decrement = 1 // Default decrement
		}

		// Get current ASG to determine current capacity
		currentASG, err := a.client.GetAutoScalingGroup(ctx, asgName)
		if err != nil {
			return nil, err
		}

		currentCapacity, ok := currentASG.Details["desiredCapacity"].(int32)
		if !ok {
			return nil, fmt.Errorf("unable to determine current desired capacity")
		}

		newCapacity := currentCapacity - decrement
		if newCapacity < 0 {
			newCapacity = 0
		}

		err = a.client.UpdateAutoScalingGroup(ctx, asgName, newCapacity)
		if err != nil {
			return nil, err
		}

		return a.client.GetAutoScalingGroup(ctx, asgName)

	case "set-desired-capacity":
		capacityParams, ok := params.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("capacity parameters required")
		}

		asgName, _ := capacityParams["autoScalingGroupName"].(string)
		desiredCapacity, _ := capacityParams["desiredCapacity"].(int32)

		if asgName == "" {
			return nil, fmt.Errorf("autoScalingGroupName is required")
		}
		if desiredCapacity < 0 {
			return nil, fmt.Errorf("desiredCapacity must be non-negative")
		}

		err := a.client.UpdateAutoScalingGroup(ctx, asgName, desiredCapacity)
		if err != nil {
			return nil, err
		}

		return a.client.GetAutoScalingGroup(ctx, asgName)

	case "attach-target-groups":
		attachParams, ok := params.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("attach parameters required")
		}

		asgName, _ := attachParams["autoScalingGroupName"].(string)
		targetGroupArns, _ := attachParams["targetGroupArns"].([]string)

		if asgName == "" {
			return nil, fmt.Errorf("autoScalingGroupName is required for attach-target-groups operation")
		}
		if len(targetGroupArns) == 0 {
			return nil, fmt.Errorf("targetGroupArns is required and must not be empty")
		}

		err := a.client.AttachLoadBalancerTargetGroups(ctx, asgName, targetGroupArns)
		if err != nil {
			return nil, err
		}

		return a.client.GetAutoScalingGroup(ctx, asgName)

	default:
		return nil, fmt.Errorf("unsupported specialized operation: %s", operation)
	}
}

// GetSpecialOperations returns the specialized operations available
func (a *ASGSpecializedAdapter) GetSpecialOperations() []string {
	return []string{"create-launch-template", "scale-out", "scale-in", "set-desired-capacity", "attach-target-groups"}
}
