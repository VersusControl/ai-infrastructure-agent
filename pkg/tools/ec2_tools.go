package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/adapters"
	"github.com/versus-control/ai-infrastructure-agent/pkg/aws"
	"github.com/versus-control/ai-infrastructure-agent/pkg/interfaces"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// CreateEC2InstanceTool implements MCPTool for creating EC2 instances
type CreateEC2InstanceTool struct {
	*BaseTool
	adapter interfaces.AWSResourceAdapter
}

// NewCreateEC2InstanceTool creates a new EC2 instance creation tool
func NewCreateEC2InstanceTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"imageId": map[string]interface{}{
				"type":        "string",
				"description": "The AMI ID to use for the instance",
			},
			"instanceType": map[string]interface{}{
				"type":        "string",
				"description": "The instance type (e.g., t3.micro, t3.small)",
				"default":     "t3.micro",
			},
			"keyName": map[string]interface{}{
				"type":        "string",
				"description": "The name of the key pair for SSH access",
			},
			"securityGroupId": map[string]interface{}{
				"type":        "string",
				"description": "The security group ID to assign to the instance",
			},
			"subnetId": map[string]interface{}{
				"type":        "string",
				"description": "The subnet ID where the instance will be launched",
			},
			"name": map[string]interface{}{
				"type":        "string",
				"description": "A name tag for the instance",
			},
		},
		"required": []interface{}{"imageId", "instanceType"},
	}

	baseTool := NewBaseTool(
		"create-ec2-instance",
		"Create a new EC2 instance with specified configuration",
		"ec2",
		actionType,
		inputSchema,
		logger,
	)

	// Add examples
	baseTool.AddExample(
		"Create a basic web server instance",
		map[string]interface{}{
			"imageId":      "ami-0abcdef1234567890",
			"instanceType": "t3.micro",
			"keyName":      "my-key-pair",
			"name":         "web-server-1",
		},
		"Successfully created EC2 instance i-1234567890abcdef0",
	)

	adapter := adapters.NewEC2Adapter(awsClient, logger)

	return &CreateEC2InstanceTool{
		BaseTool: baseTool,
		adapter:  adapter,
	}
}

// Execute creates an EC2 instance
func (t *CreateEC2InstanceTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	// Extract and validate parameters
	imageID, _ := arguments["imageId"].(string)
	instanceType, _ := arguments["instanceType"].(string)
	keyName, _ := arguments["keyName"].(string)
	securityGroupID, _ := arguments["securityGroupId"].(string)
	subnetID, _ := arguments["subnetId"].(string)
	name, _ := arguments["name"].(string)

	// Create parameters struct
	params := aws.CreateInstanceParams{
		ImageID:         imageID,
		InstanceType:    instanceType,
		KeyName:         keyName,
		SecurityGroupID: securityGroupID,
		SubnetID:        subnetID,
		Name:            name,
	}

	// Validate parameters
	if err := t.adapter.ValidateParams("create", params); err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Parameter validation failed: %s", err.Error()))
	}

	// Create the instance using the adapter
	resource, err := t.adapter.Create(ctx, params)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to create EC2 instance: %s", err.Error()))
	}

	// Return success response
	message := fmt.Sprintf("Successfully created EC2 instance %s", resource.ID)
	data := map[string]interface{}{
		"instanceId":   resource.ID,
		"instanceType": instanceType,
		"state":        resource.State,
		"details":      resource.Details,
	}

	return t.CreateSuccessResponse(message, data)
}

// ListEC2InstancesTool implements MCPTool for listing EC2 instances
type ListEC2InstancesTool struct {
	*BaseTool
	adapter interfaces.AWSResourceAdapter
}

// NewListEC2InstancesTool creates a new tool for listing EC2 instances
func NewListEC2InstancesTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"tags": map[string]interface{}{
				"type":        "object",
				"description": "Filter instances by tags (optional)",
			},
			"state": map[string]interface{}{
				"type":        "string",
				"description": "Filter by instance state (running, stopped, etc.)",
			},
		},
	}

	baseTool := NewBaseTool(
		"list-ec2-instances",
		"List EC2 instances with optional filtering",
		"ec2",
		actionType,
		inputSchema,
		logger,
	)

	// Add examples
	baseTool.AddExample(
		"List all EC2 instances",
		map[string]interface{}{},
		"Retrieved 5 EC2 instances",
	)

	baseTool.AddExample(
		"List running instances with specific tag",
		map[string]interface{}{
			"tags":  map[string]interface{}{"Environment": "production"},
			"state": "running",
		},
		"Retrieved 2 running production instances",
	)

	adapter := adapters.NewEC2Adapter(awsClient, logger)

	return &ListEC2InstancesTool{
		BaseTool: baseTool,
		adapter:  adapter,
	}
}

// Execute lists EC2 instances
func (t *ListEC2InstancesTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	var instances []*types.AWSResource
	var err error

	// Check if filtering by tags
	if tagsFilter, exists := arguments["tags"]; exists {
		if tags, ok := tagsFilter.(map[string]interface{}); ok {
			// Convert to map[string]string
			tagMap := make(map[string]string)
			for k, v := range tags {
				tagMap[k] = fmt.Sprintf("%v", v)
			}
			instances, err = t.adapter.ListByTags(ctx, tagMap)
		} else {
			return t.CreateErrorResponse("Invalid tags filter format")
		}
	} else {
		// List all instances
		instances, err = t.adapter.List(ctx)
	}

	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to list EC2 instances: %s", err.Error()))
	}

	// Apply state filter if specified
	if stateFilter, exists := arguments["state"]; exists {
		if state, ok := stateFilter.(string); ok {
			var filtered []*types.AWSResource
			for _, instance := range instances {
				if instance.State == state {
					filtered = append(filtered, instance)
				}
			}
			instances = filtered
		}
	}

	// Format response
	message := fmt.Sprintf("Retrieved %d EC2 instances", len(instances))
	data := map[string]interface{}{
		"count":     len(instances),
		"instances": instances,
	}

	return t.CreateSuccessResponse(message, data)
}

// GetEC2InstanceTool implements MCPTool for getting a specific EC2 instance
type GetEC2InstanceTool struct {
	*BaseTool
	adapter interfaces.AWSResourceAdapter
}

// NewGetEC2InstanceTool creates a tool for getting a specific EC2 instance by ID
func NewGetEC2InstanceTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"instanceId": map[string]interface{}{
				"type":        "string",
				"description": "The ID of the EC2 instance to retrieve (e.g., i-1234567890abcdef0)",
			},
		},
		"required": []interface{}{"instanceId"},
	}

	baseTool := NewBaseTool(
		"get-ec2-instance",
		"Get details of a specific EC2 instance by its instance ID",
		"ec2",
		actionType,
		inputSchema,
		logger,
	)

	// Add example
	baseTool.AddExample(
		"Get details of a specific EC2 instance",
		map[string]interface{}{
			"instanceId": "i-1234567890abcdef0",
		},
		"Successfully retrieved instance details for i-1234567890abcdef0",
	)

	adapter := adapters.NewEC2Adapter(awsClient, logger)

	return &GetEC2InstanceTool{
		BaseTool: baseTool,
		adapter:  adapter,
	}
}

// Execute retrieves a specific EC2 instance
func (t *GetEC2InstanceTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	// Extract instance ID
	instanceID, ok := arguments["instanceId"].(string)
	if !ok || instanceID == "" {
		return t.CreateErrorResponse("instanceId is required and must be a string")
	}

	// Get the instance using adapter
	instance, err := t.adapter.Get(ctx, instanceID)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to get EC2 instance: %s", err.Error()))
	}

	// Format response
	message := fmt.Sprintf("Successfully retrieved instance %s (state: %s)", instanceID, instance.State)
	data := map[string]interface{}{
		"instanceId": instance.ID,
		"state":      instance.State,
		"type":       instance.Type,
		"details":    instance.Details,
		"tags":       instance.Tags,
		"region":     instance.Region,
	}

	return t.CreateSuccessResponse(message, data)
}

// StartEC2InstanceTool implements specialized operation for starting instances
type StartEC2InstanceTool struct {
	*BaseTool
	specializedAdapter interfaces.SpecializedOperations
}

// NewStartEC2InstanceTool creates a tool for starting EC2 instances
func NewStartEC2InstanceTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"instanceId": map[string]interface{}{
				"type":        "string",
				"description": "The ID of the instance to start",
			},
		},
		"required": []interface{}{"instanceId"},
	}

	baseTool := NewBaseTool(
		"start-ec2-instance",
		"Start a stopped EC2 instance",
		"ec2",
		actionType,
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"Start an EC2 instance",
		map[string]interface{}{
			"instanceId": "i-1234567890abcdef0",
		},
		"Successfully started instance i-1234567890abcdef0",
	)

	specializedAdapter := adapters.NewEC2SpecializedAdapter(awsClient, logger)

	return &StartEC2InstanceTool{
		BaseTool:           baseTool,
		specializedAdapter: specializedAdapter,
	}
}

// Execute starts an EC2 instance
func (t *StartEC2InstanceTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	instanceID, ok := arguments["instanceId"].(string)
	if !ok || instanceID == "" {
		return t.CreateErrorResponse("instanceId is required")
	}

	resource, err := t.specializedAdapter.ExecuteSpecialOperation(ctx, "start", instanceID)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to start instance: %s", err.Error()))
	}

	message := fmt.Sprintf("Successfully started instance %s", instanceID)
	data := map[string]interface{}{
		"instanceId": instanceID,
		"state":      resource.State,
	}

	return t.CreateSuccessResponse(message, data)
}

// StopEC2InstanceTool implements specialized operation for stopping instances
type StopEC2InstanceTool struct {
	*BaseTool
	specializedAdapter interfaces.SpecializedOperations
}

// NewStopEC2InstanceTool creates a tool for stopping EC2 instances
func NewStopEC2InstanceTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"instanceId": map[string]interface{}{
				"type":        "string",
				"description": "The ID of the instance to stop",
			},
		},
		"required": []interface{}{"instanceId"},
	}

	baseTool := NewBaseTool(
		"stop-ec2-instance",
		"Stop a running EC2 instance",
		"ec2",
		actionType,
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"Stop an EC2 instance",
		map[string]interface{}{
			"instanceId": "i-1234567890abcdef0",
		},
		"Successfully stopped instance i-1234567890abcdef0",
	)

	specializedAdapter := adapters.NewEC2SpecializedAdapter(awsClient, logger)

	return &StopEC2InstanceTool{
		BaseTool:           baseTool,
		specializedAdapter: specializedAdapter,
	}
}

// Execute stops an EC2 instance
func (t *StopEC2InstanceTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	instanceID, ok := arguments["instanceId"].(string)
	if !ok || instanceID == "" {
		return t.CreateErrorResponse("instanceId is required")
	}

	resource, err := t.specializedAdapter.ExecuteSpecialOperation(ctx, "stop", instanceID)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to stop instance: %s", err.Error()))
	}

	message := fmt.Sprintf("Successfully stopped instance %s", instanceID)
	data := map[string]interface{}{
		"instanceId": instanceID,
		"state":      resource.State,
	}

	return t.CreateSuccessResponse(message, data)
}

// ModifyEC2InstanceTypeTool implements operation for modifying instance type
type ModifyEC2InstanceTypeTool struct {
	*BaseTool
	specializedAdapter interfaces.SpecializedOperations
}

// NewModifyEC2InstanceTypeTool creates a tool for modifying EC2 instance type
func NewModifyEC2InstanceTypeTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"instanceId": map[string]interface{}{
				"type":        "string",
				"description": "The ID of the instance to modify (must be in stopped state)",
			},
			"instanceType": map[string]interface{}{
				"type":        "string",
				"description": "The new instance type (e.g., t3.small, t3.medium, t3.large)",
			},
		},
		"required": []interface{}{"instanceId", "instanceType"},
	}

	baseTool := NewBaseTool(
		"modify-ec2-instance-type",
		"Modify the instance type of a stopped EC2 instance. The instance must be stopped before modification.",
		"ec2",
		actionType,
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"Change instance type from t3.micro to t3.small",
		map[string]interface{}{
			"instanceId":   "i-1234567890abcdef0",
			"instanceType": "t3.small",
		},
		"Successfully modified instance i-1234567890abcdef0 to type t3.small",
	)

	specializedAdapter := adapters.NewEC2SpecializedAdapter(awsClient, logger)

	return &ModifyEC2InstanceTypeTool{
		BaseTool:           baseTool,
		specializedAdapter: specializedAdapter,
	}
}

// Execute modifies an EC2 instance type
func (t *ModifyEC2InstanceTypeTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	instanceID, ok := arguments["instanceId"].(string)
	if !ok || instanceID == "" {
		return t.CreateErrorResponse("instanceId is required")
	}

	instanceType, ok := arguments["instanceType"].(string)
	if !ok || instanceType == "" {
		return t.CreateErrorResponse("instanceType is required")
	}

	params := map[string]interface{}{
		"instanceId":   instanceID,
		"instanceType": instanceType,
	}

	resource, err := t.specializedAdapter.ExecuteSpecialOperation(ctx, "modify-instance-type", params)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to modify instance type: %s", err.Error()))
	}

	message := fmt.Sprintf("Successfully modified instance %s to type %s", instanceID, instanceType)
	data := map[string]interface{}{
		"instanceId":   instanceID,
		"instanceType": instanceType,
		"state":        resource.State,
	}

	return t.CreateSuccessResponse(message, data)
}

// TerminateEC2InstanceTool implements operation for terminating instances
type TerminateEC2InstanceTool struct {
	*BaseTool
	adapter interfaces.AWSResourceAdapter
}

// NewTerminateEC2InstanceTool creates a tool for terminating EC2 instances
func NewTerminateEC2InstanceTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"instanceId": map[string]interface{}{
				"type":        "string",
				"description": "The ID of the instance to terminate",
			},
		},
		"required": []interface{}{"instanceId"},
	}

	baseTool := NewBaseTool(
		"terminate-ec2-instance",
		"Terminate an EC2 instance (permanent deletion)",
		"ec2",
		actionType,
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"Terminate an EC2 instance",
		map[string]interface{}{
			"instanceId": "i-1234567890abcdef0",
		},
		"Successfully terminated instance i-1234567890abcdef0",
	)

	adapter := adapters.NewEC2Adapter(awsClient, logger)

	return &TerminateEC2InstanceTool{
		BaseTool: baseTool,
		adapter:  adapter,
	}
}

// Execute terminates an EC2 instance
func (t *TerminateEC2InstanceTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	instanceID, ok := arguments["instanceId"].(string)
	if !ok || instanceID == "" {
		return t.CreateErrorResponse("instanceId is required")
	}

	err := t.adapter.Delete(ctx, instanceID)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to terminate instance: %s", err.Error()))
	}

	message := fmt.Sprintf("Successfully terminated instance %s", instanceID)
	data := map[string]interface{}{
		"instanceId": instanceID,
		"status":     "terminating",
	}

	return t.CreateSuccessResponse(message, data)
}

// CreateAMIFromInstanceTool implements MCPTool for creating AMI from instance
type CreateAMIFromInstanceTool struct {
	*BaseTool
	specializedAdapter interfaces.SpecializedOperations
}

// NewCreateAMIFromInstanceTool creates a new AMI creation tool
func NewCreateAMIFromInstanceTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"instanceId": map[string]interface{}{
				"type":        "string",
				"description": "The ID of the instance to create AMI from",
			},
			"name": map[string]interface{}{
				"type":        "string",
				"description": "The name for the AMI",
			},
			"description": map[string]interface{}{
				"type":        "string",
				"description": "The description for the AMI",
			},
		},
		"required": []string{"instanceId", "name"},
	}

	baseTool := NewBaseTool(
		"create-ami-from-instance",
		"Create an AMI from an existing EC2 instance",
		"ec2",
		actionType,
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"Create AMI from an instance",
		map[string]interface{}{
			"instanceId":  "i-1234567890abcdef0",
			"name":        "my-web-server-ami",
			"description": "Custom AMI for web server",
		},
		"Successfully created AMI ami-0abcdef1234567890",
	)

	specializedAdapter := adapters.NewEC2SpecializedAdapter(awsClient, logger)

	return &CreateAMIFromInstanceTool{
		BaseTool:           baseTool,
		specializedAdapter: specializedAdapter,
	}
}

func (t *CreateAMIFromInstanceTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	instanceID, ok := arguments["instanceId"].(string)
	if !ok || instanceID == "" {
		return t.CreateErrorResponse("instanceId is required")
	}

	name, ok := arguments["name"].(string)
	if !ok || name == "" {
		return t.CreateErrorResponse("name is required")
	}

	description, _ := arguments["description"].(string)

	// Create parameters for specialized operation
	amiParams := map[string]interface{}{
		"instanceId":  instanceID,
		"name":        name,
		"description": description,
	}

	// Use the specialized adapter for AMI creation
	resource, err := t.specializedAdapter.ExecuteSpecialOperation(ctx, "create-ami", amiParams)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to create AMI: %s", err.Error()))
	}

	message := fmt.Sprintf("Successfully created AMI %s from instance %s", resource.ID, instanceID)
	data := map[string]interface{}{
		"amiId":       resource.ID,
		"instanceId":  instanceID,
		"name":        name,
		"description": description,
		"state":       resource.State,
		"details":     resource.Details,
	}

	return t.CreateSuccessResponse(message, data)
}

// ListAMIsTool implements MCPTool for listing AMIs
type ListAMIsTool struct {
	*BaseTool
	adapter *adapters.EC2Adapter
}

// NewListAMIsTool creates a new AMI listing tool
func NewListAMIsTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"owner": map[string]interface{}{
				"type":        "string",
				"description": "The owner of the AMIs (self, amazon, aws-marketplace)",
				"default":     "self",
			},
		},
	}

	baseTool := NewBaseTool(
		"list-amis",
		"List available Amazon Machine Images",
		"ec2",
		actionType,
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"List your own AMIs",
		map[string]interface{}{
			"owner": "self",
		},
		"Found 3 AMIs owned by self",
	)

	baseTool.AddExample(
		"List Amazon-provided AMIs",
		map[string]interface{}{
			"owner": "amazon",
		},
		"Found 1500 AMIs owned by amazon",
	)

	// Note: We need to cast to EC2Adapter to access ListAMIs method
	adapter := adapters.NewEC2Adapter(awsClient, logger).(*adapters.EC2Adapter)

	return &ListAMIsTool{
		BaseTool: baseTool,
		adapter:  adapter,
	}
}

func (t *ListAMIsTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	owner, _ := arguments["owner"].(string)
	if owner == "" {
		owner = "self"
	}

	// Use the adapter to list AMIs
	amis, err := t.adapter.ListAMIs(ctx, owner)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to list AMIs: %s", err.Error()))
	}

	message := fmt.Sprintf("Found %d AMIs for owner: %s", len(amis), owner)
	data := map[string]interface{}{
		"amis":  amis,
		"owner": owner,
		"count": len(amis),
	}

	return t.CreateSuccessResponse(message, data)
}
