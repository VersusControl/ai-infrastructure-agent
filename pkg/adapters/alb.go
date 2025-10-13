package adapters

import (
	"context"
	"fmt"

	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/aws"
	"github.com/versus-control/ai-infrastructure-agent/pkg/interfaces"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
	util "github.com/versus-control/ai-infrastructure-agent/pkg/utilities"
)

// ALBAdapter implements the AWSResourceAdapter interface for Application Load Balancers
type ALBAdapter struct {
	*BaseAWSAdapter
	client *aws.Client
}

// NewALBAdapter creates a new ALB adapter
func NewALBAdapter(client *aws.Client, logger *logging.Logger) interfaces.AWSResourceAdapter {
	base := NewBaseAWSAdapter(client, logger, "alb")
	return &ALBAdapter{
		BaseAWSAdapter: base,
		client:         client,
	}
}

// Create creates a new Application Load Balancer
func (a *ALBAdapter) Create(ctx context.Context, params interface{}) (*types.AWSResource, error) {
	createParams, ok := params.(aws.CreateLoadBalancerParams)
	if !ok {
		return nil, fmt.Errorf("invalid parameters for ALB creation, expected aws.CreateLoadBalancerParams")
	}

	alb, err := a.client.CreateApplicationLoadBalancer(ctx, createParams)
	if err != nil {
		return nil, err
	}

	return alb, nil
}

// List returns all Application Load Balancers
func (a *ALBAdapter) List(ctx context.Context) ([]*types.AWSResource, error) {
	return a.client.DescribeLoadBalancers(ctx)
}

// Get retrieves a specific Application Load Balancer
func (a *ALBAdapter) Get(ctx context.Context, id string) (*types.AWSResource, error) {
	return a.client.GetLoadBalancer(ctx, id)
}

// Update updates an Application Load Balancer (limited operations available)
func (a *ALBAdapter) Update(ctx context.Context, id string, params interface{}) (*types.AWSResource, error) {
	// ALB modifications are complex, should use specialized operations
	return nil, fmt.Errorf("ALB updates should use specialized operations")
}

// Delete deletes an Application Load Balancer
func (a *ALBAdapter) Delete(ctx context.Context, id string) error {
	// Note: ALB deletion is not implemented in the AWS client yet
	return fmt.Errorf("ALB deletion not yet implemented in AWS client")
}

// GetSupportedOperations returns the operations supported by this adapter
func (a *ALBAdapter) GetSupportedOperations() []string {
	return []string{
		"create",
		"list",
		"get",
		"delete",
		"create-target-group",
		"attach-target-group",
		"create-listener",
	}
}

// ValidateParams validates ALB-specific parameters
func (a *ALBAdapter) ValidateParams(operation string, params interface{}) error {
	switch operation {
	case "create":
		createParams, ok := params.(aws.CreateLoadBalancerParams)
		if !ok {
			return fmt.Errorf("invalid parameters for create operation")
		}
		if createParams.Name == "" {
			return fmt.Errorf("name is required for ALB creation")
		}
		if len(createParams.Subnets) == 0 {
			return fmt.Errorf("subnets are required for ALB creation")
		}
		if len(createParams.Subnets) < 2 {
			return fmt.Errorf("at least two subnets in different Availability Zones are required for ALB creation")
		}
		return nil
	case "get", "delete":
		if params == nil {
			return fmt.Errorf("load balancer ARN is required for %s operation", operation)
		}
		return nil
	default:
		return fmt.Errorf("unsupported operation: %s", operation)
	}
}

// ALBSpecializedAdapter adds ALB-specific operations
type ALBSpecializedAdapter struct {
	interfaces.AWSResourceAdapter
	client *aws.Client
}

// NewALBSpecializedAdapter creates an adapter with specialized ALB operations
func NewALBSpecializedAdapter(client *aws.Client, logger *logging.Logger) interfaces.SpecializedOperations {
	baseAdapter := NewALBAdapter(client, logger)
	return &ALBSpecializedAdapter{
		AWSResourceAdapter: baseAdapter,
		client:             client,
	}
}

// ExecuteSpecialOperation handles ALB-specific operations
func (a *ALBSpecializedAdapter) ExecuteSpecialOperation(ctx context.Context, operation string, params interface{}) (*types.AWSResource, error) {
	switch operation {
	case "create-load-balancer":
		// Extract parameters for load balancer creation
		paramsMap, ok := params.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid parameters for create-load-balancer")
		}

		// Validate subnet requirements before proceeding
		subnetIds := util.GetStringSlice(paramsMap, "subnetIds")
		if len(subnetIds) == 0 {
			return nil, fmt.Errorf("subnetIds are required for load balancer creation")
		}
		if len(subnetIds) < 2 {
			return nil, fmt.Errorf("at least two subnets in different Availability Zones must be specified for load balancer creation")
		}

		albParams := aws.CreateLoadBalancerParams{
			Name:           util.GetStringFromMap(paramsMap, "name"),
			Scheme:         util.GetStringFromMap(paramsMap, "scheme"),
			Type:           util.GetStringFromMap(paramsMap, "type"),
			IpAddressType:  util.GetStringFromMap(paramsMap, "ipAddressType"),
			Subnets:        subnetIds,
			SecurityGroups: util.GetStringSlice(paramsMap, "securityGroupIds"),
			Tags:           util.GetStringMap(paramsMap, "tags"),
		}

		loadBalancer, err := a.client.CreateApplicationLoadBalancer(ctx, albParams)
		if err != nil {
			return nil, fmt.Errorf("failed to create load balancer: %w", err)
		}

		return loadBalancer, nil

	case "create-target-group":
		// Handle both old parameter format and new format
		if tgParams, ok := params.(aws.CreateTargetGroupParams); ok {
			return a.client.CreateTargetGroup(ctx, tgParams)
		}

		// Extract parameters from map for new format
		paramsMap, ok := params.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid parameters for create-target-group")
		}

		tgParams := aws.CreateTargetGroupParams{
			Name:                       util.GetStringFromMap(paramsMap, "name"),
			Protocol:                   util.GetStringFromMap(paramsMap, "protocol"),
			Port:                       util.GetInt32FromMap(paramsMap, "port", 80),
			VpcID:                      util.GetStringFromMap(paramsMap, "vpcId"),
			TargetType:                 util.GetStringFromMap(paramsMap, "targetType"),
			HealthCheckEnabled:         util.GetBoolFromMap(paramsMap, "healthCheckEnabled", true),
			HealthCheckPath:            util.GetStringFromMap(paramsMap, "healthCheckPath"),
			HealthCheckProtocol:        util.GetStringFromMap(paramsMap, "healthCheckProtocol"),
			HealthCheckIntervalSeconds: util.GetInt32FromMap(paramsMap, "healthCheckIntervalSeconds", 30),
			HealthCheckTimeoutSeconds:  util.GetInt32FromMap(paramsMap, "healthCheckTimeoutSeconds", 5),
			HealthyThresholdCount:      util.GetInt32FromMap(paramsMap, "healthyThresholdCount", 2),
			UnhealthyThresholdCount:    util.GetInt32FromMap(paramsMap, "unhealthyThresholdCount", 2),
			Matcher:                    util.GetStringFromMap(paramsMap, "matcher"),
			Tags:                       util.GetStringMap(paramsMap, "tags"),
		}

		targetGroup, err := a.client.CreateTargetGroup(ctx, tgParams)
		if err != nil {
			return nil, fmt.Errorf("failed to create target group: %w", err)
		}

		return targetGroup, nil

	case "attach-target-group":
		attachParams, ok := params.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("attach parameters required")
		}

		loadBalancerArn, _ := attachParams["loadBalancerArn"].(string)
		targetGroupArn, _ := attachParams["targetGroupArn"].(string)
		port, _ := attachParams["port"].(int32)

		if loadBalancerArn == "" || targetGroupArn == "" {
			return nil, fmt.Errorf("loadBalancerArn and targetGroupArn are required")
		}

		listenerParams := aws.CreateListenerParams{
			LoadBalancerArn:       loadBalancerArn,
			Port:                  port,
			Protocol:              "HTTP", // Default protocol
			DefaultTargetGroupArn: targetGroupArn,
		}

		return a.client.CreateListener(ctx, listenerParams)

	case "create-listener":
		// Handle both old parameter format and new format
		if listenerParams, ok := params.(aws.CreateListenerParams); ok {
			return a.client.CreateListener(ctx, listenerParams)
		}

		// Extract parameters from map for new format
		paramsMap, ok := params.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid parameters for create-listener")
		}

		listenerParams := aws.CreateListenerParams{
			LoadBalancerArn:       util.GetStringFromMap(paramsMap, "loadBalancerArn"),
			Protocol:              util.GetStringFromMap(paramsMap, "protocol"),
			Port:                  util.GetInt32FromMap(paramsMap, "port", 80),
			DefaultTargetGroupArn: getTargetGroupArn(paramsMap),
			CertificateArn:        util.GetStringFromMap(paramsMap, "certificateArn"),
		}

		listener, err := a.client.CreateListener(ctx, listenerParams)
		if err != nil {
			return nil, fmt.Errorf("failed to create listener: %w", err)
		}

		return listener, nil

	case "list-target-groups":
		// List all Target Groups
		targetGroups, err := a.client.DescribeTargetGroups(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list target groups: %w", err)
		}

		// Return a summary resource with all target groups
		return &types.AWSResource{
			ID:    "target-group-list",
			Type:  "target-group-list",
			State: "available",
			Details: map[string]interface{}{
				"count":        len(targetGroups),
				"targetGroups": targetGroups,
			},
		}, nil

	case "register-targets":
		// Extract parameters for target registration
		paramsMap, ok := params.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid parameters for register-targets")
		}

		targetGroupArn := util.GetStringFromMap(paramsMap, "targetGroupArn")
		instanceIds := util.GetStringSlice(paramsMap, "instanceIds")

		if targetGroupArn == "" {
			return nil, fmt.Errorf("targetGroupArn is required")
		}

		if len(instanceIds) == 0 {
			return nil, fmt.Errorf("at least one instance ID is required")
		}

		err := a.client.RegisterTargets(ctx, targetGroupArn, instanceIds)
		if err != nil {
			return nil, fmt.Errorf("failed to register targets: %w", err)
		}

		return &types.AWSResource{
			ID:    targetGroupArn,
			Type:  "target-registration",
			State: "registered",
			Details: map[string]interface{}{
				"targetGroupArn": targetGroupArn,
				"instanceIds":    instanceIds,
				"count":          len(instanceIds),
			},
		}, nil

	case "deregister-targets":
		// Extract parameters for target deregistration
		paramsMap, ok := params.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid parameters for deregister-targets")
		}

		targetGroupArn := util.GetStringFromMap(paramsMap, "targetGroupArn")
		instanceIds := util.GetStringSlice(paramsMap, "instanceIds")

		err := a.client.DeregisterTargets(ctx, targetGroupArn, instanceIds)
		if err != nil {
			return nil, fmt.Errorf("failed to deregister targets: %w", err)
		}

		return &types.AWSResource{
			ID:    targetGroupArn,
			Type:  "target-deregistration",
			State: "deregistered",
			Details: map[string]interface{}{
				"targetGroupArn": targetGroupArn,
				"instanceIds":    instanceIds,
				"count":          len(instanceIds),
			},
		}, nil

	default:
		return nil, fmt.Errorf("unsupported specialized operation: %s", operation)
	}
}

// GetSpecialOperations returns the specialized operations available
func (a *ALBSpecializedAdapter) GetSpecialOperations() []string {
	return []string{
		"create-load-balancer",
		"create-target-group",
		"attach-target-group",
		"create-listener",
		"list-target-groups",
		"register-targets",
		"deregister-targets",
	}
}

// getTargetGroupArn handles both targetGroupArn and defaultTargetGroupArn parameter names
func getTargetGroupArn(params map[string]interface{}) string {
	// First try the new parameter name from CreateListener tool
	if val, ok := params["targetGroupArn"].(string); ok && val != "" {
		return val
	}
	// Fallback to the old parameter name for backward compatibility
	if val, ok := params["defaultTargetGroupArn"].(string); ok && val != "" {
		return val
	}
	return ""
}
