package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/aws"
	"github.com/versus-control/ai-infrastructure-agent/pkg/interfaces"
)

// CreateIAMRoleTool implements MCPTool for creating IAM roles
type CreateIAMRoleTool struct {
	*BaseTool
	client *aws.Client
}

// NewCreateIAMRoleTool creates a tool for creating IAM roles
func NewCreateIAMRoleTool(client *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"roleName": map[string]interface{}{
				"type":        "string",
				"description": "The name of the IAM role to create",
			},
			"assumeRolePolicyDocument": map[string]interface{}{
				"type":        "string",
				"description": "The trust relationship policy document that grants an entity permission to assume the role (JSON string)",
			},
			"description": map[string]interface{}{
				"type":        "string",
				"description": "Description of the role",
			},
			"tags": map[string]interface{}{
				"type":        "object",
				"description": "Tags to assign to the role",
			},
		},
		"required": []interface{}{"roleName", "assumeRolePolicyDocument"},
	}

	baseTool := NewBaseTool(
		"create-iam-role",
		"Create a new IAM role",
		"iam",
		actionType,
		inputSchema,
		logger,
	)

	return &CreateIAMRoleTool{
		BaseTool: baseTool,
		client:   client,
	}
}

// Execute creates an IAM role
func (t *CreateIAMRoleTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	roleName, _ := arguments["roleName"].(string)
	assumeRolePolicyDocument, _ := arguments["assumeRolePolicyDocument"].(string)
	description, _ := arguments["description"].(string)

	var tags map[string]string
	if val, ok := arguments["tags"]; ok {
		if tagMap, ok := val.(map[string]interface{}); ok {
			tags = make(map[string]string)
			for k, v := range tagMap {
				tags[k] = fmt.Sprintf("%v", v)
			}
		}
	}

	params := aws.CreateIAMRoleParams{
		RoleName:                 roleName,
		AssumeRolePolicyDocument: assumeRolePolicyDocument,
		Description:              description,
		Tags:                     tags,
	}

	resource, err := t.client.CreateIAMRole(ctx, params)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to create IAM role: %s", err.Error()))
	}

	message := fmt.Sprintf("Successfully created IAM role %s", resource.ID)
	data := map[string]interface{}{
		"roleName": resource.ID,
		"arn":      resource.Details["arn"],
	}

	return t.CreateSuccessResponse(message, data)
}

// ListIAMRolesTool implements MCPTool for listing IAM roles
type ListIAMRolesTool struct {
	*BaseTool
	client *aws.Client
}

// NewListIAMRolesTool creates a tool for listing IAM roles
func NewListIAMRolesTool(client *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}

	baseTool := NewBaseTool(
		"list-iam-roles",
		"List all IAM roles",
		"iam",
		actionType,
		inputSchema,
		logger,
	)

	return &ListIAMRolesTool{
		BaseTool: baseTool,
		client:   client,
	}
}

// Execute lists IAM roles
func (t *ListIAMRolesTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	roles, err := t.client.ListIAMRoles(ctx)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to list IAM roles: %s", err.Error()))
	}

	message := fmt.Sprintf("Found %d IAM roles", len(roles))
	data := map[string]interface{}{
		"roles": roles,
		"count": len(roles),
	}

	return t.CreateSuccessResponse(message, data)
}
