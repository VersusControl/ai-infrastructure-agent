package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/adapters"
	"github.com/versus-control/ai-infrastructure-agent/pkg/aws"
	"github.com/versus-control/ai-infrastructure-agent/pkg/interfaces"
)

// CreateLoadBalancerTool implements MCPTool for creating load balancers
type CreateLoadBalancerTool struct {
	*BaseTool
	adapter   interfaces.SpecializedOperations
	awsClient *aws.Client
}

// NewCreateLoadBalancerTool creates a new load balancer creation tool
func NewCreateLoadBalancerTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "The name for the load balancer",
			},
			"scheme": map[string]interface{}{
				"type":        "string",
				"description": "The scheme (internet-facing or internal)",
				"default":     "internet-facing",
			},
			"type": map[string]interface{}{
				"type":        "string",
				"description": "The type (application, network, gateway)",
				"default":     "application",
			},
			"subnetIds": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
				"description": "List of subnet IDs (minimum 2 subnets in different Availability Zones required). If not provided, subnets will be auto-selected.",
				"minItems":    2,
			},
			"securityGroupIds": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
				"description": "List of security group IDs",
			},
		},
		"required": []string{"name"},
	}

	baseTool := NewBaseTool(
		"create-load-balancer",
		"Create a new application load balancer",
		"load-balancer",
		actionType,
		inputSchema,
		logger,
	)

	return &CreateLoadBalancerTool{
		BaseTool:  baseTool,
		adapter:   adapters.NewALBSpecializedAdapter(awsClient, logger),
		awsClient: awsClient,
	}
}

func (t *CreateLoadBalancerTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	name, ok := arguments["name"].(string)
	if !ok || name == "" {
		return t.CreateErrorResponse("name is required")
	}

	scheme, _ := arguments["scheme"].(string)
	if scheme == "" {
		scheme = "internet-facing"
	}

	lbType, _ := arguments["type"].(string)
	if lbType == "" {
		lbType = "application"
	}

	// Check subnet requirements and auto-select if needed
	subnetIds, _ := arguments["subnetIds"].([]interface{})

	// Handle case where subnetIds might be passed as a JSON string (from dependency resolution)
	if len(subnetIds) == 0 {
		if subnetIdsStr, ok := arguments["subnetIds"].(string); ok && subnetIdsStr != "" {

			// Try to parse as JSON array
			var parsedSubnetIds []string
			if err := json.Unmarshal([]byte(subnetIdsStr), &parsedSubnetIds); err == nil {
				// Convert to []interface{}
				subnetIds = make([]interface{}, len(parsedSubnetIds))
				for i, subnetId := range parsedSubnetIds {
					subnetIds[i] = subnetId
				}
				t.logger.WithFields(map[string]interface{}{
					"parsed_subnet_count": len(subnetIds),
					"parsed_subnets":      parsedSubnetIds,
				}).Info("Successfully parsed subnetIds from JSON string")
			} else {
				t.logger.WithError(err).WithField("subnetIds_string", subnetIdsStr).Warn("Failed to parse subnetIds JSON string")
			}
		}
	}

	if len(subnetIds) < 2 {
		t.logger.Info("Insufficient subnets provided, auto-selecting subnets for ALB creation")

		// Create the subnet selection tool and call it
		subnetSelector := NewListSubnetsForALBTool(t.awsClient, "query", t.logger)
		selectionArgs := map[string]interface{}{
			"scheme": scheme,
		}

		// Copy VPC ID if provided
		if vpcId, exists := arguments["vpcId"]; exists {
			selectionArgs["vpcId"] = vpcId
		}

		selectionResult, err := subnetSelector.Execute(ctx, selectionArgs)
		if err != nil {
			return t.CreateErrorResponse(fmt.Sprintf("Failed to auto-select subnets: %v", err))
		}

		if selectionResult.IsError {
			return t.CreateErrorResponse(fmt.Sprintf("Subnet selection failed: %v", selectionResult.Content[0]))
		}

		t.logger.WithFields(map[string]interface{}{
			"selection_result_length": len(selectionResult.Content),
		}).Info("Received subnet selection result")

		// Extract subnet IDs from the selection result
		if len(selectionResult.Content) > 0 {

			// Try multiple approaches to extract text content
			var textData string
			var extractSuccess bool

			// Approach 1: Try TextContent type assertion
			if textContent, ok := selectionResult.Content[0].(*mcp.TextContent); ok {
				textData = textContent.Text
				extractSuccess = true
			} else if textContent, ok := selectionResult.Content[0].(mcp.TextContent); ok {
				// Approach 2: Try value type assertion
				textData = textContent.Text
				extractSuccess = true
			} else {
				// Approach 3: Try to extract from any content with Text field
				if contentInterface, ok := selectionResult.Content[0].(interface{ GetText() string }); ok {
					textData = contentInterface.GetText()
					extractSuccess = true
				}
			}

			if extractSuccess {

				// Parse the JSON response to extract subnet IDs
				var resultData map[string]interface{}
				if err := json.Unmarshal([]byte(textData), &resultData); err == nil {
					if subnetIdsData, exists := resultData["subnetIds"]; exists {
						if subnetIdsSlice, ok := subnetIdsData.([]interface{}); ok {
							arguments["subnetIds"] = subnetIdsSlice
							t.logger.WithFields(map[string]interface{}{
								"selected_subnets": subnetIdsSlice,
								"source":           "auto_selection",
							}).Info("Auto-selected subnets for ALB creation")
						} else {
							t.logger.Error("subnetIds field is not a slice")
							return t.CreateErrorResponse("Invalid format for selected subnet IDs")
						}
					} else {
						t.logger.Error("No subnetIds field found in response")
						return t.CreateErrorResponse("No subnet IDs found in selection response")
					}
				} else {
					t.logger.WithError(err).Error("Failed to parse subnet selection response")
					return t.CreateErrorResponse(fmt.Sprintf("Failed to parse subnet selection response: %v", err))
				}
			} else {
				t.logger.WithFields(map[string]interface{}{
					"actual_type":    fmt.Sprintf("%T", selectionResult.Content[0]),
					"content_string": fmt.Sprintf("%v", selectionResult.Content[0]),
				}).Error("Unable to extract text content from selection result")
				return t.CreateErrorResponse("Invalid content type in subnet selection response")
			}
		} else {
			return t.CreateErrorResponse("Empty response from subnet selection")
		}

		// Re-validate that we now have sufficient subnets
		subnetIds, _ = arguments["subnetIds"].([]interface{})
		if len(subnetIds) < 2 {
			return t.CreateErrorResponse(fmt.Sprintf("Failed to auto-select sufficient subnets for ALB creation. Got %d subnets, need at least 2", len(subnetIds)))
		}
	}

	// Use the ALB specialized adapter to create load balancer
	result, err := t.adapter.ExecuteSpecialOperation(ctx, "create-load-balancer", arguments)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to create load balancer: %v", err))
	}

	message := fmt.Sprintf("Load balancer %s created successfully", name)
	data := map[string]interface{}{
		"name":            name,
		"scheme":          scheme,
		"type":            lbType,
		"loadBalancer":    result,
		"loadBalancerId":  result.ID,
		"loadBalancerArn": result.ID, // The ID field contains the ARN for load balancers
		"arn":             result.ID, // Alternative key for ARN
	}

	return t.CreateSuccessResponse(message, data)
}

// CreateTargetGroupTool implements MCPTool for creating target groups
type CreateTargetGroupTool struct {
	*BaseTool
	adapter interfaces.SpecializedOperations
}

// NewCreateTargetGroupTool creates a new target group creation tool
func NewCreateTargetGroupTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "The name for the target group",
			},
			"protocol": map[string]interface{}{
				"type":        "string",
				"description": "The protocol (HTTP, HTTPS, TCP)",
				"default":     "HTTP",
			},
			"port": map[string]interface{}{
				"type":        "integer",
				"description": "The port number",
				"default":     80,
			},
			"vpcId": map[string]interface{}{
				"type":        "string",
				"description": "The VPC ID",
			},
			"targetType": map[string]interface{}{
				"type":        "string",
				"description": "The target type (instance, ip, lambda)",
				"default":     "instance",
			},
			"healthCheckPath": map[string]interface{}{
				"type":        "string",
				"description": "The health check path",
				"default":     "/",
			},
			"healthCheckProtocol": map[string]interface{}{
				"type":        "string",
				"description": "The health check protocol",
			},
			"healthCheckEnabled": map[string]interface{}{
				"type":        "boolean",
				"description": "Whether health checks are enabled",
				"default":     true,
			},
			"matcher": map[string]interface{}{
				"type":        "string",
				"description": "HTTP response codes that indicate healthy targets",
				"default":     "200",
			},
		},
		"required": []string{"name", "vpcId"},
	}

	baseTool := NewBaseTool(
		"create-target-group",
		"Create a new target group",
		"load-balancer",
		actionType,
		inputSchema,
		logger,
	)

	return &CreateTargetGroupTool{
		BaseTool: baseTool,
		adapter:  adapters.NewALBSpecializedAdapter(awsClient, logger),
	}
}

func (t *CreateTargetGroupTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	name, ok := arguments["name"].(string)
	if !ok || name == "" {
		return t.CreateErrorResponse("name is required")
	}

	vpcID, ok := arguments["vpcId"].(string)
	if !ok || vpcID == "" {
		return t.CreateErrorResponse("vpcId is required")
	}

	protocol, _ := arguments["protocol"].(string)
	if protocol == "" {
		protocol = "HTTP"
	}

	port := 80
	if portArg, exists := arguments["port"]; exists {
		if portFloat, ok := portArg.(float64); ok {
			port = int(portFloat)
		}
	}

	targetType, _ := arguments["targetType"].(string)
	if targetType == "" {
		targetType = "instance"
	}

	// Use the ALB specialized adapter to create target group
	result, err := t.adapter.ExecuteSpecialOperation(ctx, "create-target-group", arguments)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to create target group: %v", err))
	}

	message := fmt.Sprintf("Target group %s created successfully", name)
	data := map[string]interface{}{
		"name":           name,
		"protocol":       protocol,
		"port":           port,
		"vpcId":          vpcID,
		"targetType":     targetType,
		"targetGroup":    result,
		"targetGroupId":  result.ID,
		"targetGroupArn": result.ID, // The ID field contains the ARN for target groups
		"arn":            result.ID, // Alternative key for ARN
	}

	return t.CreateSuccessResponse(message, data)
}

// CreateListenerTool implements MCPTool for creating listeners
type CreateListenerTool struct {
	*BaseTool
	adapter interfaces.SpecializedOperations
}

// NewCreateListenerTool creates a new listener creation tool
func NewCreateListenerTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"loadBalancerArn": map[string]interface{}{
				"type":        "string",
				"description": "The ARN of the load balancer",
			},
			"protocol": map[string]interface{}{
				"type":        "string",
				"description": "The protocol (HTTP, HTTPS)",
				"default":     "HTTP",
			},
			"port": map[string]interface{}{
				"type":        "integer",
				"description": "The port number",
				"default":     80,
			},
			"targetGroupArn": map[string]interface{}{
				"type":        "string",
				"description": "The ARN of the target group",
			},
		},
		"required": []string{"loadBalancerArn", "targetGroupArn"},
	}

	baseTool := NewBaseTool(
		"create-listener",
		"Create a new listener for a load balancer",
		"load-balancer",
		actionType,
		inputSchema,
		logger,
	)

	return &CreateListenerTool{
		BaseTool: baseTool,
		adapter:  adapters.NewALBSpecializedAdapter(awsClient, logger),
	}
}

func (t *CreateListenerTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	loadBalancerArn, ok := arguments["loadBalancerArn"].(string)
	if !ok || loadBalancerArn == "" {
		return t.CreateErrorResponse("loadBalancerArn is required")
	}

	// Check if the loadBalancerArn is actually an ARN format, if not, it might be a name that needs validation
	if !strings.HasPrefix(loadBalancerArn, "arn:aws:elasticloadbalancing:") {
		t.logger.WithFields(map[string]interface{}{
			"provided_value": loadBalancerArn,
		}).Warn("loadBalancerArn does not appear to be in ARN format - this may cause AWS API errors")

		// If it looks like a load balancer name instead of ARN, return a helpful error
		if !strings.Contains(loadBalancerArn, ":") {
			return t.CreateErrorResponse(fmt.Sprintf("loadBalancerArn must be in ARN format, received what appears to be a name: '%s'. Expected format: arn:aws:elasticloadbalancing:region:account:loadbalancer/type/name/id", loadBalancerArn))
		}
	}

	targetGroupArn, ok := arguments["targetGroupArn"].(string)
	if !ok || targetGroupArn == "" {
		return t.CreateErrorResponse("targetGroupArn is required")
	}

	// Check if the targetGroupArn is actually an ARN format
	if !strings.HasPrefix(targetGroupArn, "arn:aws:elasticloadbalancing:") {
		t.logger.WithFields(map[string]interface{}{
			"provided_value": targetGroupArn,
		}).Warn("targetGroupArn does not appear to be in ARN format - this may cause AWS API errors")

		// If it looks like a target group name instead of ARN, return a helpful error
		if !strings.Contains(targetGroupArn, ":") {
			return t.CreateErrorResponse(fmt.Sprintf("targetGroupArn must be in ARN format, received what appears to be a name: '%s'. Expected format: arn:aws:elasticloadbalancing:region:account:targetgroup/name/id", targetGroupArn))
		}
	}

	protocol, _ := arguments["protocol"].(string)
	if protocol == "" {
		protocol = "HTTP"
	}

	port := 80
	if portArg, exists := arguments["port"]; exists {
		if portFloat, ok := portArg.(float64); ok {
			port = int(portFloat)
		}
	}

	// Use the ALB specialized adapter to create listener
	t.logger.WithFields(map[string]interface{}{
		"loadBalancerArn": loadBalancerArn,
		"targetGroupArn":  targetGroupArn,
		"protocol":        protocol,
		"port":            port,
	}).Info("Creating listener with validated parameters")

	result, err := t.adapter.ExecuteSpecialOperation(ctx, "create-listener", arguments)
	if err != nil {
		t.logger.WithError(err).WithFields(map[string]interface{}{
			"loadBalancerArn": loadBalancerArn,
			"targetGroupArn":  targetGroupArn,
		}).Error("Failed to create listener")
		return t.CreateErrorResponse(fmt.Sprintf("Failed to create listener: %v", err))
	}

	message := "Listener created successfully for load balancer"
	data := map[string]interface{}{
		"loadBalancerArn": loadBalancerArn,
		"targetGroupArn":  targetGroupArn,
		"protocol":        protocol,
		"port":            port,
		"listener":        result,
		"listenerId":      result.ID,
	}

	return t.CreateSuccessResponse(message, data)
}

// ListLoadBalancersTool implements MCPTool for listing load balancers
type ListLoadBalancersTool struct {
	*BaseTool
	adapter interfaces.AWSResourceAdapter
}

// NewListLoadBalancersTool creates a new load balancer listing tool
func NewListLoadBalancersTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}

	baseTool := NewBaseTool(
		"list-load-balancers",
		"List all load balancers",
		"alb",
		actionType,
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"List all load balancers",
		map[string]interface{}{},
		"Retrieved 3 load balancers",
	)

	adapter := adapters.NewALBAdapter(awsClient, logger)

	return &ListLoadBalancersTool{
		BaseTool: baseTool,
		adapter:  adapter,
	}

}

// Execute lists all Load Balancers
func (t *ListLoadBalancersTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	t.logger.Info("Listing Load Balancers...")

	// List all Load Balancers using the adapter
	loadBalancers, err := t.adapter.List(ctx)
	if err != nil {
		t.logger.Error("Failed to list Load Balancers", "error", err)
		return t.CreateErrorResponse(fmt.Sprintf("Failed to list Load Balancers: %v", err))
	}

	message := fmt.Sprintf("Successfully retrieved %d Load Balancers", len(loadBalancers))
	data := map[string]interface{}{
		"loadBalancers": loadBalancers,
		"count":         len(loadBalancers),
	}

	return t.CreateSuccessResponse(message, data)
}

// ListTargetGroupsTool implements MCPTool for listing target groups
type ListTargetGroupsTool struct {
	*BaseTool
	adapter interfaces.SpecializedOperations
}

// NewListTargetGroupsTool creates a new target group listing tool
func NewListTargetGroupsTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}

	baseTool := NewBaseTool(
		"list-target-groups",
		"List all target groups",
		"alb",
		actionType,
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"List all target groups",
		map[string]interface{}{},
		"Retrieved 2 target groups",
	)

	// Use ALB specialized adapter for Target Group operations
	adapter := adapters.NewALBSpecializedAdapter(awsClient, logger)

	return &ListTargetGroupsTool{
		BaseTool: baseTool,
		adapter:  adapter,
	}
}

func (t *ListTargetGroupsTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	t.logger.Info("Listing Target Groups...")

	// Use the ALB specialized adapter for target group operations
	result, err := t.adapter.ExecuteSpecialOperation(ctx, "list-target-groups", map[string]interface{}{})
	if err != nil {
		t.logger.Error("Failed to list Target Groups", "error", err)
		return t.CreateErrorResponse(fmt.Sprintf("Failed to list Target Groups: %v", err))
	}

	// Extract target groups from adapter result Details
	var targetGroups []interface{}
	var count int
	if result.Details != nil {
		if tgList, exists := result.Details["targetGroups"]; exists {
			targetGroups, _ = tgList.([]interface{})
		}
		if countVal, exists := result.Details["count"]; exists {
			count, _ = countVal.(int)
		}
	}

	message := fmt.Sprintf("Successfully retrieved %d Target Groups", count)
	data := map[string]interface{}{
		"targetGroups": targetGroups,
		"count":        count,
	}

	return t.CreateSuccessResponse(message, data)
}

// RegisterTargetsTool implements MCPTool for registering targets with a target group
type RegisterTargetsTool struct {
	*BaseTool
	adapter interfaces.SpecializedOperations
}

// NewRegisterTargetsTool creates a new target registration tool
func NewRegisterTargetsTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"targetGroupArn": map[string]interface{}{
				"type":        "string",
				"description": "The ARN of the target group",
			},
			"targetIds": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
				"description": "List of target IDs (instance IDs, IP addresses, etc.)",
			},
		},
		"required": []interface{}{"targetGroupArn", "targetIds"},
	}

	baseTool := NewBaseTool(
		"register-targets",
		"Register targets with a load balancer target group",
		"load-balancer",
		actionType,
		inputSchema,
		logger,
	)

	return &RegisterTargetsTool{
		BaseTool: baseTool,
		adapter:  adapters.NewALBSpecializedAdapter(awsClient, logger),
	}
}

func (t *RegisterTargetsTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	targetGroupArn, ok := arguments["targetGroupArn"].(string)
	if !ok {
		return t.CreateErrorResponse("targetGroupArn is required and must be a string")
	}

	targetIdsInterface, ok := arguments["targetIds"].([]interface{})
	if !ok {
		return t.CreateErrorResponse("targetIds is required and must be an array")
	}

	// Convert interface slice to string slice
	targetIds := make([]string, len(targetIdsInterface))
	for i, id := range targetIdsInterface {
		if str, ok := id.(string); ok {
			targetIds[i] = str
		} else {
			return t.CreateErrorResponse(fmt.Sprintf("target ID at index %d must be a string", i))
		}
	}

	// Use the ALB specialized adapter to register targets
	params := map[string]interface{}{
		"targetGroupArn": targetGroupArn,
		"instanceIds":    targetIds,
	}

	result, err := t.adapter.ExecuteSpecialOperation(ctx, "register-targets", params)
	if err != nil {
		t.logger.WithError(err).Error("Failed to register targets")
		return t.CreateErrorResponse(fmt.Sprintf("Failed to register targets: %v", err))
	}

	message := fmt.Sprintf("Successfully registered %d targets with target group %s", len(targetIds), targetGroupArn)
	data := map[string]interface{}{
		"targetGroupArn": targetGroupArn,
		"targetIds":      targetIds,
		"status":         "registered",
		"result":         result,
	}

	return t.CreateSuccessResponse(message, data)
}

// DeregisterTargetsTool implements MCPTool for deregistering targets from a target group
type DeregisterTargetsTool struct {
	*BaseTool
	adapter interfaces.SpecializedOperations
}

// NewDeregisterTargetsTool creates a new target deregistration tool
func NewDeregisterTargetsTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"targetGroupArn": map[string]interface{}{
				"type":        "string",
				"description": "The ARN of the target group",
			},
			"targetIds": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
				"description": "List of target IDs (instance IDs, IP addresses, etc.)",
			},
		},
		"required": []interface{}{"targetGroupArn", "targetIds"},
	}

	baseTool := NewBaseTool(
		"deregister-targets",
		"Deregister targets from a load balancer target group",
		"load-balancer",
		actionType,
		inputSchema,
		logger,
	)

	return &DeregisterTargetsTool{
		BaseTool: baseTool,
		adapter:  adapters.NewALBSpecializedAdapter(awsClient, logger),
	}
}

func (t *DeregisterTargetsTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	targetGroupArn, ok := arguments["targetGroupArn"].(string)
	if !ok {
		return t.CreateErrorResponse("targetGroupArn is required and must be a string")
	}

	targetIdsInterface, ok := arguments["targetIds"].([]interface{})
	if !ok {
		return t.CreateErrorResponse("targetIds is required and must be an array")
	}

	// Convert interface slice to string slice
	targetIds := make([]string, len(targetIdsInterface))
	for i, id := range targetIdsInterface {
		if str, ok := id.(string); ok {
			targetIds[i] = str
		} else {
			return t.CreateErrorResponse(fmt.Sprintf("target ID at index %d must be a string", i))
		}
	}

	// Use the ALB specialized adapter to deregister targets
	params := map[string]interface{}{
		"targetGroupArn": targetGroupArn,
		"instanceIds":    targetIds,
	}

	result, err := t.adapter.ExecuteSpecialOperation(ctx, "deregister-targets", params)
	if err != nil {
		t.logger.WithError(err).Error("Failed to deregister targets")
		return t.CreateErrorResponse(fmt.Sprintf("Failed to deregister targets: %v", err))
	}

	message := fmt.Sprintf("Successfully deregistered %d targets from target group %s", len(targetIds), targetGroupArn)
	data := map[string]interface{}{
		"targetGroupArn": targetGroupArn,
		"targetIds":      targetIds,
		"status":         "deregistered",
		"result":         result,
	}

	return t.CreateSuccessResponse(message, data)
}
