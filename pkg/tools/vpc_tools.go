package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/adapters"
	"github.com/versus-control/ai-infrastructure-agent/pkg/aws"
	"github.com/versus-control/ai-infrastructure-agent/pkg/interfaces"
)

// CreateVPCTool implements VPC creation using the VPC adapter
type CreateVPCTool struct {
	*BaseTool
	adapter interfaces.AWSResourceAdapter
}

// NewCreateVPCTool creates a new VPC creation tool
func NewCreateVPCTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"cidrBlock": map[string]interface{}{
				"type":        "string",
				"description": "CIDR block for the VPC (e.g., '10.0.0.0/16')",
			},
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Name tag for the VPC (will appear in AWS Console)",
			},
			"enableDnsHostnames": map[string]interface{}{
				"type":        "boolean",
				"description": "Enable DNS hostnames for the VPC",
				"default":     true,
			},
			"enableDnsSupport": map[string]interface{}{
				"type":        "boolean",
				"description": "Enable DNS support for the VPC",
				"default":     true,
			},
		},
		"required": []interface{}{"cidrBlock"},
	}

	baseTool := NewBaseTool(
		"create-vpc",
		"Create a new VPC (Virtual Private Cloud)",
		"networking",
		actionType,
		inputSchema,
		logger,
	)

	adapter := adapters.NewVPCAdapter(awsClient, logger)

	return &CreateVPCTool{
		BaseTool: baseTool,
		adapter:  adapter,
	}
}

// Execute creates a VPC using the adapter
func (t *CreateVPCTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	// Extract parameters
	cidrBlock, _ := arguments["cidrBlock"].(string)
	name, _ := arguments["name"].(string)
	enableDnsHostnames := getBoolValue(arguments, "enableDnsHostnames", true)
	enableDnsSupport := getBoolValue(arguments, "enableDnsSupport", true)

	// Build AWS parameters
	createParams := aws.CreateVPCParams{
		CidrBlock:          cidrBlock,
		Name:               name,
		EnableDnsHostnames: enableDnsHostnames,
		EnableDnsSupport:   enableDnsSupport,
		Tags:               make(map[string]string),
	}

	// Create VPC using adapter
	vpc, err := t.adapter.Create(ctx, createParams)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to create VPC: %s", err.Error()))
	}

	// Return success result with structured data
	message := fmt.Sprintf("Successfully created VPC %s with CIDR block %s", vpc.ID, cidrBlock)
	data := map[string]interface{}{
		"vpcId":              vpc.ID,
		"cidrBlock":          cidrBlock,
		"name":               name,
		"enableDnsHostnames": enableDnsHostnames,
		"enableDnsSupport":   enableDnsSupport,
		"resource":           vpc,
	}

	return t.CreateSuccessResponse(message, data)
}

// ListVPCsTool implements VPC listing using the VPC adapter
type ListVPCsTool struct {
	*BaseTool
	adapter interfaces.AWSResourceAdapter
}

// NewListVPCsTool creates a new VPC listing tool
func NewListVPCsTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}

	baseTool := NewBaseTool(
		"list-vpcs",
		"List all VPCs in the region",
		"networking",
		actionType,
		inputSchema,
		logger,
	)

	adapter := adapters.NewVPCAdapter(awsClient, logger)

	return &ListVPCsTool{
		BaseTool: baseTool,
		adapter:  adapter,
	}
}

// Execute lists VPCs using the adapter
func (t *ListVPCsTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	// List VPCs using adapter
	vpcs, err := t.adapter.List(ctx)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to list VPCs: %s", err.Error()))
	}

	// Return success result with structured data
	message := fmt.Sprintf("Successfully retrieved %d VPCs", len(vpcs))
	data := map[string]interface{}{
		"vpcs":  vpcs,
		"count": len(vpcs),
	}

	return t.CreateSuccessResponse(message, data)
}

// CreateSubnetTool implements subnet creation using the Subnet adapter
type CreateSubnetTool struct {
	*BaseTool
	adapter interfaces.AWSResourceAdapter
}

// NewCreateSubnetTool creates a new subnet creation tool
func NewCreateSubnetTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"vpcId": map[string]interface{}{
				"type":        "string",
				"description": "VPC ID where the subnet will be created",
			},
			"cidrBlock": map[string]interface{}{
				"type":        "string",
				"description": "CIDR block for the subnet (e.g., '10.0.1.0/24')",
			},
			"availabilityZone": map[string]interface{}{
				"type":        "string",
				"description": "Availability zone for the subnet",
			},
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Name tag for the subnet (will appear in AWS Console)",
			},
			"mapPublicIpOnLaunch": map[string]interface{}{
				"type":        "boolean",
				"description": "Auto-assign public IP on instance launch",
				"default":     false,
			},
		},
		"required": []interface{}{"vpcId", "cidrBlock", "availabilityZone"},
	}

	baseTool := NewBaseTool(
		"create-subnet",
		"Create a new subnet in a VPC",
		"networking",
		actionType,
		inputSchema,
		logger,
	)

	adapter := adapters.NewSubnetAdapter(awsClient, logger)

	return &CreateSubnetTool{
		BaseTool: baseTool,
		adapter:  adapter,
	}
}

// Execute creates a subnet using the adapter
func (t *CreateSubnetTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	// Extract parameters
	vpcId, _ := arguments["vpcId"].(string)
	cidrBlock, _ := arguments["cidrBlock"].(string)
	availabilityZone, _ := arguments["availabilityZone"].(string)
	name, _ := arguments["name"].(string)
	mapPublicIpOnLaunch := getBoolValue(arguments, "mapPublicIpOnLaunch", false)

	// Build AWS parameters
	createParams := aws.CreateSubnetParams{
		VpcID:               vpcId,
		CidrBlock:           cidrBlock,
		AvailabilityZone:    availabilityZone,
		Name:                name,
		MapPublicIpOnLaunch: mapPublicIpOnLaunch,
		Tags:                make(map[string]string),
	}

	// Create subnet using adapter
	subnet, err := t.adapter.Create(ctx, createParams)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to create subnet: %s", err.Error()))
	}

	// Return success result with structured data
	message := fmt.Sprintf("Successfully created subnet %s in VPC %s with CIDR block %s", subnet.ID, vpcId, cidrBlock)
	data := map[string]interface{}{
		"subnetId":            subnet.ID,
		"vpcId":               vpcId,
		"cidrBlock":           cidrBlock,
		"availabilityZone":    availabilityZone,
		"name":                name,
		"mapPublicIpOnLaunch": mapPublicIpOnLaunch,
		"resource":            subnet,
	}

	return t.CreateSuccessResponse(message, data)
}

// GetDefaultVPCTool implements getting the default VPC using the VPC adapter
type GetDefaultVPCTool struct {
	*BaseTool
	client *aws.Client
}

// NewGetDefaultVPCTool creates a new get default VPC tool
func NewGetDefaultVPCTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}

	baseTool := NewBaseTool(
		"get-default-vpc",
		"Get the default VPC in the current region",
		"networking",
		actionType,
		inputSchema,
		logger,
	)

	return &GetDefaultVPCTool{
		BaseTool: baseTool,
		client:   awsClient,
	}
}

// Execute gets the default VPC using the AWS client
func (t *GetDefaultVPCTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	// Get default VPC ID using AWS client
	vpcID, err := t.client.GetDefaultVPC(ctx)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to get default VPC: %s", err.Error()))
	}

	// Get full VPC details using the adapter
	adapter := adapters.NewVPCAdapter(t.client, t.logger)
	vpc, err := adapter.Get(ctx, vpcID)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to get VPC details for %s: %s", vpcID, err.Error()))
	}

	// Return success result with structured data
	message := fmt.Sprintf("Successfully retrieved default VPC %s", vpc.ID)
	data := map[string]interface{}{
		"vpcId":    vpc.ID,
		"vpc":      vpc,
		"resource": vpc,
	}

	return t.CreateSuccessResponse(message, data)
}

// Helper function for boolean values
func getBoolValue(params map[string]interface{}, key string, defaultValue bool) bool {
	if value, ok := params[key].(bool); ok {
		return value
	}
	return defaultValue
}
