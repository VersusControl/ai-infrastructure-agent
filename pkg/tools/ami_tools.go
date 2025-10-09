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

// GetLatestAmazonLinuxAMITool implements MCPTool for finding latest Amazon Linux 2 AMI
type GetLatestAmazonLinuxAMITool struct {
	*BaseTool
	adapter interfaces.SpecializedOperations
}

// NewGetLatestAmazonLinuxAMITool creates a new tool for finding latest Amazon Linux 2 AMI
func NewGetLatestAmazonLinuxAMITool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}

	baseTool := NewBaseTool(
		"get-latest-amazon-linux-ami",
		"Find the latest Amazon Linux 2 AMI ID to use for launching EC2 instances.",
		"ec2",
		actionType,
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"Get latest Amazon Linux 2 AMI",
		map[string]interface{}{},
		"Found latest Amazon Linux 2 AMI: ami-0abcdef1234567890. Returns: { amiId: \"ami-0abcdef1234567890\", osType: \"Linux\", platform: \"Amazon Linux\" }.",
	)

	// Use EC2 specialized adapter for AMI operations
	adapter := adapters.NewEC2SpecializedAdapter(awsClient, logger)

	return &GetLatestAmazonLinuxAMITool{
		BaseTool: baseTool,
		adapter:  adapter,
	}
}

func (t *GetLatestAmazonLinuxAMITool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	t.logger.Info("Getting latest Amazon Linux 2 AMI...")

	// Use the EC2 specialized adapter to get latest Amazon Linux 2 AMI
	result, err := t.adapter.ExecuteSpecialOperation(ctx, "get-latest-amazon-linux-ami", nil)
	if err != nil {
		t.logger.Error("Failed to get latest Amazon Linux 2 AMI", "error", err)
		return t.CreateErrorResponse(fmt.Sprintf("Failed to get latest Amazon Linux 2 AMI: %v", err))
	}

	// Extract the AMI details from the result
	amiID := result.ID
	details := result.Details

	message := fmt.Sprintf("Found latest Amazon Linux 2 AMI: %s", amiID)
	data := map[string]interface{}{
		"amiId":       amiID,
		"description": details["description"],
		"osType":      details["osType"],
		"platform":    details["platform"],
	}

	return t.CreateSuccessResponse(message, data)
}

// GetLatestUbuntuAMITool implements MCPTool for finding latest Ubuntu LTS AMI
type GetLatestUbuntuAMITool struct {
	*BaseTool
	adapter interfaces.SpecializedOperations
}

// NewGetLatestUbuntuAMITool creates a new tool for finding latest Ubuntu LTS AMI
func NewGetLatestUbuntuAMITool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"architecture": map[string]interface{}{
				"type":        "string",
				"description": "The architecture (x86_64, arm64)",
				"default":     "x86_64",
			},
		},
	}

	baseTool := NewBaseTool(
		"get-latest-ubuntu-ami",
		"Find the latest Ubuntu LTS AMI ID to use for launching EC2 instances.",
		"ec2",
		actionType,
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"Get latest Ubuntu LTS AMI for x86_64",
		map[string]interface{}{
			"architecture": "x86_64",
		},
		"Found latest Ubuntu LTS AMI: ami-0987654321abcdef0. Returns: { amiId: \"ami-0987654321abcdef0\", architecture: \"x86_64\", osType: \"Linux\", platform: \"Ubuntu\" }.",
	)

	// Use EC2 specialized adapter for AMI operations
	adapter := adapters.NewEC2SpecializedAdapter(awsClient, logger)

	return &GetLatestUbuntuAMITool{
		BaseTool: baseTool,
		adapter:  adapter,
	}
}

func (t *GetLatestUbuntuAMITool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	t.logger.Info("Getting latest Ubuntu LTS AMI...")

	architecture, ok := arguments["architecture"].(string)
	if !ok {
		architecture = "x86_64" // Default to x86_64
	}

	// Prepare parameters for the adapter
	params := map[string]interface{}{
		"architecture": architecture,
	}

	// Use the EC2 specialized adapter to get latest Ubuntu LTS AMI
	result, err := t.adapter.ExecuteSpecialOperation(ctx, "get-latest-ubuntu-ami", params)
	if err != nil {
		t.logger.Error("Failed to get latest Ubuntu LTS AMI", "error", err)
		return t.CreateErrorResponse(fmt.Sprintf("Failed to get latest Ubuntu LTS AMI: %v", err))
	}

	// Extract the AMI details from the result
	amiID := result.ID
	details := result.Details

	message := fmt.Sprintf("Found latest Ubuntu LTS AMI for %s: %s", architecture, amiID)
	data := map[string]interface{}{
		"amiId":        amiID,
		"architecture": details["architecture"],
		"description":  details["description"],
		"osType":       details["osType"],
		"platform":     details["platform"],
	}

	return t.CreateSuccessResponse(message, data)
}

// GetLatestWindowsAMITool implements MCPTool for finding latest Windows Server AMI
type GetLatestWindowsAMITool struct {
	*BaseTool
	adapter interfaces.SpecializedOperations
}

// NewGetLatestWindowsAMITool creates a new tool for finding latest Windows Server AMI
func NewGetLatestWindowsAMITool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"architecture": map[string]interface{}{
				"type":        "string",
				"description": "The architecture (x86_64, arm64)",
				"default":     "x86_64",
			},
		},
	}

	baseTool := NewBaseTool(
		"get-latest-windows-ami",
		"Find the latest Windows Server AMI ID to use for launching EC2 instances.",
		"ec2",
		actionType,
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"Get latest Windows Server AMI for x86_64",
		map[string]interface{}{
			"architecture": "x86_64",
		},
		"Found latest Windows Server AMI: ami-0fedcba9876543210. Returns: { amiId: \"ami-0fedcba9876543210\", architecture: \"x86_64\", osType: \"Windows\", platform: \"Windows Server\" }.",
	)

	// Use EC2 specialized adapter for AMI operations
	adapter := adapters.NewEC2SpecializedAdapter(awsClient, logger)

	return &GetLatestWindowsAMITool{
		BaseTool: baseTool,
		adapter:  adapter,
	}
}

func (t *GetLatestWindowsAMITool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	t.logger.Info("Getting latest Windows Server AMI...")

	architecture, ok := arguments["architecture"].(string)
	if !ok {
		architecture = "x86_64" // Default to x86_64
	}

	// Prepare parameters for the adapter
	params := map[string]interface{}{
		"architecture": architecture,
	}

	// Use the EC2 specialized adapter to get latest Windows Server AMI
	result, err := t.adapter.ExecuteSpecialOperation(ctx, "get-latest-windows-ami", params)
	if err != nil {
		t.logger.Error("Failed to get latest Windows Server AMI", "error", err)
		return t.CreateErrorResponse(fmt.Sprintf("Failed to get latest Windows Server AMI: %v", err))
	}

	// Extract the AMI details from the result
	amiID := result.ID
	details := result.Details

	message := fmt.Sprintf("Found latest Windows Server AMI for %s: %s", architecture, amiID)
	data := map[string]interface{}{
		"amiId":        amiID,
		"architecture": details["architecture"],
		"description":  details["description"],
		"osType":       details["osType"],
		"platform":     details["platform"],
	}

	return t.CreateSuccessResponse(message, data)
}
