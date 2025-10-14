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

// CreateSecurityGroupTool implements security group creation
type CreateSecurityGroupTool struct {
	*BaseTool
	adapter *adapters.SecurityGroupAdapter
}

// NewCreateSecurityGroupTool creates a new security group creation tool
func NewCreateSecurityGroupTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"groupName": map[string]interface{}{
				"type":        "string",
				"description": "Name of the security group",
			},
			"description": map[string]interface{}{
				"type":        "string",
				"description": "Description of the security group",
			},
			"vpcId": map[string]interface{}{
				"type":        "string",
				"description": "VPC ID where the security group will be created",
			},
		},
		"required": []interface{}{"groupName", "description", "vpcId"},
	}

	baseTool := NewBaseTool(
		"create-security-group",
		"Create a new security group",
		"security",
		actionType,
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"Create security group",
		map[string]interface{}{
			"groupName":   "web-server-sg",
			"description": "Security group for web servers",
			"vpcId":       "vpc-12345678",
		},
		"Created security group sg-87654321 for web servers",
	)

	// Cast to SecurityGroupAdapter for type safety
	adapter := adapters.NewSecurityGroupAdapter(awsClient, logger).(*adapters.SecurityGroupAdapter)

	return &CreateSecurityGroupTool{
		BaseTool: baseTool,
		adapter:  adapter,
	}
}

func (t *CreateSecurityGroupTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	groupName, ok := arguments["groupName"].(string)
	if !ok || groupName == "" {
		return t.CreateErrorResponse("groupName is required")
	}

	description, ok := arguments["description"].(string)
	if !ok || description == "" {
		return t.CreateErrorResponse("description is required")
	}

	vpcID, ok := arguments["vpcId"].(string)
	if !ok || vpcID == "" {
		return t.CreateErrorResponse("vpcId is required")
	}

	// Create security group parameters
	params := aws.SecurityGroupParams{
		GroupName:   groupName,
		Description: description,
		VpcID:       vpcID,
		Tags:        map[string]string{},
	}

	// Use the adapter to create the security group
	sg, err := t.adapter.Create(ctx, params)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to create security group: %s", err.Error()))
	}

	message := fmt.Sprintf("Created security group %s (%s) in VPC %s", sg.ID, groupName, vpcID)
	data := map[string]interface{}{
		"securityGroupId": sg.ID,
		"groupName":       groupName,
		"description":     description,
		"vpcId":           vpcID,
		"resource":        sg,
	}

	return t.CreateSuccessResponse(message, data)
}

// ListSecurityGroupsTool implements security group listing
type ListSecurityGroupsTool struct {
	*BaseTool
	adapter *adapters.SecurityGroupAdapter
}

// NewListSecurityGroupsTool creates a new security group listing tool
func NewListSecurityGroupsTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}

	baseTool := NewBaseTool(
		"list-security-groups",
		"List all security groups in the region",
		"security",
		actionType,
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"List all security groups",
		map[string]interface{}{},
		"Found 5 security groups",
	)

	// Cast to SecurityGroupAdapter for type safety
	adapter := adapters.NewSecurityGroupAdapter(awsClient, logger).(*adapters.SecurityGroupAdapter)

	return &ListSecurityGroupsTool{
		BaseTool: baseTool,
		adapter:  adapter,
	}
}

func (t *ListSecurityGroupsTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	// Use the adapter to list security groups
	sgs, err := t.adapter.List(ctx)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to list security groups: %s", err.Error()))
	}

	// Extract security group IDs for dependency resolution
	var sgIDs []string
	for _, sg := range sgs {
		sgIDs = append(sgIDs, sg.ID)
	}

	message := fmt.Sprintf("Found %d security groups", len(sgs))
	data := map[string]interface{}{
		"securityGroups":   sgs,
		"securityGroupIds": sgIDs, // Array of all security group IDs
		"count":            len(sgs),
	}

	// Add single ID fields for extraction pattern matching
	if len(sgIDs) > 0 {
		data["securityGroupId"] = sgIDs[0] // First security group ID
		data["groupId"] = sgIDs[0]         // Alias for compatibility
		data["value"] = sgIDs[0]           // For resource ID resolution
	}

	return t.CreateSuccessResponse(message, data)
}

// AddSecurityGroupIngressRuleTool implements adding ingress rules to security groups
type AddSecurityGroupIngressRuleTool struct {
	*BaseTool
	adapter interfaces.SpecializedOperations
}

// NewAddSecurityGroupIngressRuleTool creates a new ingress rule addition tool
func NewAddSecurityGroupIngressRuleTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"groupId": map[string]interface{}{
				"type":        "string",
				"description": "The security group ID",
			},
			"protocol": map[string]interface{}{
				"type":        "string",
				"description": "The protocol (tcp, udp, icmp)",
			},
			"fromPort": map[string]interface{}{
				"type":        "integer",
				"description": "The start port number",
			},
			"toPort": map[string]interface{}{
				"type":        "integer",
				"description": "The end port number",
			},
			"cidrBlock": map[string]interface{}{
				"type":        "string",
				"description": "The CIDR block to allow",
			},
		},
		"required": []string{"groupId", "protocol", "fromPort", "toPort", "cidrBlock"},
	}

	baseTool := NewBaseTool(
		"add-security-group-ingress-rule",
		"Add an ingress rule to a security group",
		"security",
		actionType,
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"Add HTTP ingress rule",
		map[string]interface{}{
			"groupId":   "sg-123456789",
			"protocol":  "tcp",
			"fromPort":  80,
			"toPort":    80,
			"cidrBlock": "0.0.0.0/0",
		},
		"Added ingress rule to security group sg-123456789: tcp 80-80 from 0.0.0.0/0",
	)

	// Create specialized adapter for Security Group operations
	adapter := adapters.NewSecurityGroupSpecializedAdapter(awsClient, logger)

	return &AddSecurityGroupIngressRuleTool{
		BaseTool: baseTool,
		adapter:  adapter,
	}
}

func (t *AddSecurityGroupIngressRuleTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	groupID, ok := arguments["groupId"].(string)
	if !ok || groupID == "" {
		return t.CreateErrorResponse("groupId is required")
	}

	protocol, ok := arguments["protocol"].(string)
	if !ok || protocol == "" {
		return t.CreateErrorResponse("protocol is required")
	}

	fromPort, ok := arguments["fromPort"].(float64)
	if !ok {
		return t.CreateErrorResponse("fromPort is required")
	}

	toPort, ok := arguments["toPort"].(float64)
	if !ok {
		return t.CreateErrorResponse("toPort is required")
	}

	cidrBlock, ok := arguments["cidrBlock"].(string)
	if !ok || cidrBlock == "" {
		return t.CreateErrorResponse("cidrBlock is required")
	}

	// Prepare parameters for the adapter
	params := map[string]interface{}{
		"groupId":   groupID,
		"protocol":  protocol,
		"fromPort":  int(fromPort),
		"toPort":    int(toPort),
		"cidrBlock": cidrBlock,
	}

	// Add ingress rule using the Security Group specialized adapter
	result, err := t.adapter.ExecuteSpecialOperation(ctx, "add-ingress-rule", params)
	if err != nil {
		t.logger.Error("Failed to add ingress rule", "error", err)
		return t.CreateErrorResponse(fmt.Sprintf("Failed to add ingress rule: %v", err))
	}

	message := fmt.Sprintf("Added ingress rule to security group %s: %s %d-%d from %s",
		groupID, protocol, int(fromPort), int(toPort), cidrBlock)
	data := map[string]interface{}{
		"groupId":   groupID,
		"protocol":  protocol,
		"fromPort":  int(fromPort),
		"toPort":    int(toPort),
		"cidrBlock": cidrBlock,
		"direction": "ingress",
		"resource":  result,
	}

	return t.CreateSuccessResponse(message, data)
}

// AddSecurityGroupEgressRuleTool implements adding egress rules to security groups
type AddSecurityGroupEgressRuleTool struct {
	*BaseTool
	adapter interfaces.SpecializedOperations
}

// NewAddSecurityGroupEgressRuleTool creates a new egress rule addition tool
func NewAddSecurityGroupEgressRuleTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"groupId": map[string]interface{}{
				"type":        "string",
				"description": "The security group ID",
			},
			"protocol": map[string]interface{}{
				"type":        "string",
				"description": "The protocol (tcp, udp, icmp)",
			},
			"fromPort": map[string]interface{}{
				"type":        "integer",
				"description": "The start port number",
			},
			"toPort": map[string]interface{}{
				"type":        "integer",
				"description": "The end port number",
			},
			"cidrBlock": map[string]interface{}{
				"type":        "string",
				"description": "The CIDR block to allow",
			},
		},
		"required": []string{"groupId", "protocol", "fromPort", "toPort", "cidrBlock"},
	}

	baseTool := NewBaseTool(
		"add-security-group-egress-rule",
		"Add an egress rule to a security group",
		"security",
		actionType,
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"Add HTTPS egress rule",
		map[string]interface{}{
			"groupId":   "sg-123456789",
			"protocol":  "tcp",
			"fromPort":  443,
			"toPort":    443,
			"cidrBlock": "0.0.0.0/0",
		},
		"Added egress rule to security group sg-123456789: tcp 443-443 to 0.0.0.0/0",
	)

	// Create specialized adapter for Security Group operations
	adapter := adapters.NewSecurityGroupSpecializedAdapter(awsClient, logger)

	return &AddSecurityGroupEgressRuleTool{
		BaseTool: baseTool,
		adapter:  adapter,
	}
}

func (t *AddSecurityGroupEgressRuleTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	groupID, ok := arguments["groupId"].(string)
	if !ok || groupID == "" {
		return t.CreateErrorResponse("groupId is required")
	}

	protocol, ok := arguments["protocol"].(string)
	if !ok || protocol == "" {
		return t.CreateErrorResponse("protocol is required")
	}

	fromPort, ok := arguments["fromPort"].(float64)
	if !ok {
		return t.CreateErrorResponse("fromPort is required")
	}

	toPort, ok := arguments["toPort"].(float64)
	if !ok {
		return t.CreateErrorResponse("toPort is required")
	}

	cidrBlock, ok := arguments["cidrBlock"].(string)
	if !ok || cidrBlock == "" {
		return t.CreateErrorResponse("cidrBlock is required")
	}

	// Prepare parameters for the adapter
	params := map[string]interface{}{
		"groupId":   groupID,
		"protocol":  protocol,
		"fromPort":  int(fromPort),
		"toPort":    int(toPort),
		"cidrBlock": cidrBlock,
	}

	// Add egress rule using the Security Group specialized adapter
	result, err := t.adapter.ExecuteSpecialOperation(ctx, "add-egress-rule", params)
	if err != nil {
		t.logger.Error("Failed to add egress rule", "error", err)
		return t.CreateErrorResponse(fmt.Sprintf("Failed to add egress rule: %v", err))
	}

	message := fmt.Sprintf("Added egress rule to security group %s: %s %d-%d to %s",
		groupID, protocol, int(fromPort), int(toPort), cidrBlock)
	data := map[string]interface{}{
		"groupId":   groupID,
		"protocol":  protocol,
		"fromPort":  int(fromPort),
		"toPort":    int(toPort),
		"cidrBlock": cidrBlock,
		"direction": "egress",
		"resource":  result,
	}

	return t.CreateSuccessResponse(message, data)
}

// DeleteSecurityGroupTool implements security group deletion
type DeleteSecurityGroupTool struct {
	*BaseTool
	adapter interfaces.SpecializedOperations
}

// NewDeleteSecurityGroupTool creates a new security group deletion tool
func NewDeleteSecurityGroupTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"groupId": map[string]interface{}{
				"type":        "string",
				"description": "The security group ID to delete",
			},
		},
		"required": []string{"groupId"},
	}

	baseTool := NewBaseTool(
		"delete-security-group",
		"Delete a security group",
		"security",
		actionType,
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"Delete security group",
		map[string]interface{}{
			"groupId": "sg-123456789",
		},
		"Security group sg-123456789 deleted successfully",
	)

	// Create specialized adapter for Security Group operations
	adapter := adapters.NewSecurityGroupSpecializedAdapter(awsClient, logger)

	return &DeleteSecurityGroupTool{
		BaseTool: baseTool,
		adapter:  adapter,
	}
}

func (t *DeleteSecurityGroupTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	groupID, ok := arguments["groupId"].(string)
	if !ok || groupID == "" {
		return t.CreateErrorResponse("groupId is required")
	}

	// Delete security group using the Security Group specialized adapter
	result, err := t.adapter.ExecuteSpecialOperation(ctx, "delete-security-group", groupID)
	if err != nil {
		t.logger.Error("Failed to delete security group", "error", err)
		return t.CreateErrorResponse(fmt.Sprintf("Failed to delete security group: %v", err))
	}

	message := fmt.Sprintf("Security group %s deleted successfully", groupID)
	data := map[string]interface{}{
		"groupId":  groupID,
		"status":   "deleted",
		"resource": result,
	}

	return t.CreateSuccessResponse(message, data)
}
