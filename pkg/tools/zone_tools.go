package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/aws"
	"github.com/versus-control/ai-infrastructure-agent/pkg/interfaces"
)

// GetAvailabilityZonesTool implements MCPTool for retrieving availability zones
type GetAvailabilityZonesTool struct {
	*BaseTool
	awsClient *aws.Client
}

// NewGetAvailabilityZonesTool creates a new availability zones retrieval tool
func NewGetAvailabilityZonesTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"region": map[string]interface{}{
				"type":        "string",
				"description": "The AWS region (optional, uses current client region if not specified)",
			},
			"maxAZs": map[string]interface{}{
				"type":        "number",
				"description": "Maximum number of AZs to return (optional)",
				"minimum":     1,
			},
		},
	}

	baseTool := NewBaseTool(
		"get-availability-zones",
		"Get available availability zones in the current or specified region",
		"aws-info",
		actionType,
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"Get first 3 availability zones",
		map[string]interface{}{
			"maxAZs": 3,
		},
		"Retrieved 3 availability zones",
	)

	return &GetAvailabilityZonesTool{
		BaseTool:  baseTool,
		awsClient: awsClient,
	}
}

func (t *GetAvailabilityZonesTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	t.logger.Info("Getting availability zones")

	zones, err := t.awsClient.GetAvailabilityZones(ctx)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to get availability zones: %s", err.Error()))
	}

	// Handle maxAZs parameter
	maxAZs := len(zones)
	if maxAZsParam, exists := arguments["maxAZs"]; exists {
		if maxAZsFloat, ok := maxAZsParam.(float64); ok && int(maxAZsFloat) > 0 && int(maxAZsFloat) < len(zones) {
			maxAZs = int(maxAZsFloat)
		}
	}

	// Limit zones if requested
	limitedZones := zones
	if maxAZs < len(zones) {
		limitedZones = zones[:maxAZs]
	}

	message := fmt.Sprintf("Retrieved %d availability zones", len(limitedZones))
	data := map[string]interface{}{
		"availabilityZones": limitedZones,
		"count":             len(limitedZones),
		"allZones":          zones,
		"allCount":          len(zones),
	}

	return t.CreateSuccessResponse(message, data)
}
