package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/versus-control/ai-infrastructure-agent/pkg/tools"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// ========== Interface defines ==========

// RetrievalInterface defines resource retrieval functionality
//
// Available Functions:
//   - retrieveLatestAMI()             : Get latest AMI for specified OS type
//   - retrieveDefaultVPC()            : Get default VPC for current region
//   - retrieveDefaultSubnet()         : Get default subnet information
//   - retrieveAvailabilityZones()     : Get available AZs for current region
//   - retrieveExistingVPC()           : Find existing VPC (default or first available)
//   - retrieveSubnetsInVPC()          : Get all subnets in specified VPC
//   - retrieveSelectSubnetsForALB()   : Select appropriate subnets for ALB creation
//   - retrieveLoadBalancerArn()       : Retrieve load balancer ARN from previous steps
//   - retrieveTargetGroupArn()        : Retrieve target group ARN from previous steps
//   - retrieveDefaultAMIForRegion()        : Get region-specific default AMI
//   - retrieveExistingResourceFromState()  : retrieves existing resources from the managed state file
//   - retrieveLaunchTemplateId()      : Retrieve launch template ID from previous steps
//   - retrieveSecurityGroupId()       : Retrieve security group ID from previous steps
//   - retrieveDBSubnetGroupName()     : Retrieve DB subnet group name from previous steps
//   - retrieveAutoScalingGroupArn()   : Retrieve Auto Scaling Group ARN from previous steps
//   - retrieveAutoScalingGroupName()  : Retrieve Auto Scaling Group name from previous steps
//   - retrieveRDSEndpoint()          : Retrieve RDS database endpoint from previous steps
//
// This file handles all AWS resource discovery and retrieval operations
// needed for infrastructure planning and dependency resolution.
//
// Usage Example:
//   1. amiInfo := agent.retrieveLatestAMI(ctx, planStep)
//   2. vpcInfo := agent.retrieveDefaultVPC(ctx, planStep)
//   3. // Use retrieved information for resource creation

// ========== AWS Resource Retrieval Functions ==========

// retrieveLatestAMI gets the latest Amazon Linux 2 AMI for the current region
func (a *StateAwareAgent) retrieveLatestAMI(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	// Get the OS type from parameters (default to Amazon Linux 2)
	osType := "amazon-linux-2"
	if osParam, exists := planStep.Parameters["os_type"]; exists {
		osType = fmt.Sprintf("%v", osParam)
	}

	// Get the architecture (default to x86_64)
	architecture := "x86_64"
	if archParam, exists := planStep.Parameters["architecture"]; exists {
		architecture = fmt.Sprintf("%v", archParam)
	}

	a.Logger.WithFields(map[string]interface{}{
		"os_type":      osType,
		"architecture": architecture,
		"step_id":      planStep.ID,
	}).Info("Starting API retrieval for latest AMI")

	var amiID string
	var err error

	switch osType {
	case "amazon-linux-2":
		a.Logger.Info("Calling AWS API via awsClient.GetLatestAmazonLinux2AMI")
		amiID, err = a.awsClient.GetLatestAmazonLinux2AMI(ctx)
		if err != nil {
			a.Logger.WithError(err).Error("AWS API call failed for GetLatestAmazonLinux2AMI")
		} else {
			a.Logger.WithField("ami_id", amiID).Info("AWS API call successful, received AMI ID")
		}
	case "ubuntu":
		a.Logger.Info("Calling AWS API via awsClient.GetLatestUbuntuAMI")
		amiID, err = a.awsClient.GetLatestUbuntuAMI(ctx, architecture)
		if err != nil {
			a.Logger.WithError(err).Error("AWS API call failed for GetLatestUbuntuAMI")
		} else {
			a.Logger.WithField("ami_id", amiID).Info("AWS API call successful, received Ubuntu AMI ID")
		}
	case "windows":
		a.Logger.Info("Calling AWS API via awsClient.GetLatestWindowsAMI")
		amiID, err = a.awsClient.GetLatestWindowsAMI(ctx, architecture)
		if err != nil {
			a.Logger.WithError(err).Error("AWS API call failed for GetLatestWindowsAMI")
		} else {
			a.Logger.WithField("ami_id", amiID).Info("AWS API call successful, received Windows AMI ID")
		}
	default:
		return nil, fmt.Errorf("unsupported OS type: %s. Supported types: amazon-linux-2, ubuntu, windows", osType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get latest %s AMI: %w", osType, err)
	}

	a.Logger.WithFields(map[string]interface{}{
		"ami_id":       amiID,
		"os_type":      osType,
		"architecture": architecture,
		"source":       "aws_api_call",
	}).Info("API retrieval completed successfully")

	return map[string]interface{}{
		"value":        amiID,
		"type":         "ami",
		"os_type":      osType,
		"architecture": architecture,
		"retrieved_at": time.Now().Format(time.RFC3339),
		"description":  fmt.Sprintf("Latest %s AMI for %s architecture", osType, architecture),
		"source":       "aws_api_call", // Confirm this came from API
	}, nil
}

// retrieveDefaultVPC gets the default VPC for the current region
func (a *StateAwareAgent) retrieveDefaultVPC(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	a.Logger.WithField("step_id", planStep.ID).Info("Starting API retrieval for default VPC")

	a.Logger.Info("Calling AWS API via awsClient.GetDefaultVPC")
	vpcID, err := a.awsClient.GetDefaultVPC(ctx)
	if err != nil {
		a.Logger.WithError(err).Error("AWS API call failed for GetDefaultVPC")
		return nil, fmt.Errorf("failed to get default VPC: %w", err)
	}

	a.Logger.WithField("vpc_id", vpcID).Info("AWS API call successful, received VPC ID")

	return map[string]interface{}{
		"value":        vpcID,
		"type":         "vpc",
		"is_default":   true,
		"retrieved_at": time.Now().Format(time.RFC3339),
		"description":  "Default VPC for the current region",
		"source":       "aws_api_call",
	}, nil
}

// retrieveDefaultSubnet gets the default subnet for the current region
func (a *StateAwareAgent) retrieveDefaultSubnet(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	a.Logger.WithField("step_id", planStep.ID).Info("Starting API retrieval for default subnet")

	a.Logger.Info("Calling AWS API via awsClient.GetDefaultSubnet")
	subnetInfo, err := a.awsClient.GetDefaultSubnet(ctx)
	if err != nil {
		a.Logger.WithError(err).Error("AWS API call failed for GetDefaultSubnet")
		return nil, fmt.Errorf("failed to get default subnet: %w", err)
	}

	a.Logger.WithFields(map[string]interface{}{
		"subnet_id": subnetInfo.SubnetID,
		"vpc_id":    subnetInfo.VpcID,
	}).Info("AWS API call successful, received subnet and VPC IDs")

	return map[string]interface{}{
		"value":        subnetInfo.SubnetID, // For {{step-id.resourceId}} resolution (subnet ID)
		"subnet_id":    subnetInfo.SubnetID, // Explicit subnet ID
		"vpc_id":       subnetInfo.VpcID,    // Explicit VPC ID for security groups
		"type":         "subnet",
		"is_default":   true,
		"retrieved_at": time.Now().Format(time.RFC3339),
		"description":  fmt.Sprintf("Default subnet (%s) in VPC (%s)", subnetInfo.SubnetID, subnetInfo.VpcID),
		"source":       "aws_api_call",
	}, nil
}

// retrieveAvailabilityZones gets available AZs for the current region
func (a *StateAwareAgent) retrieveAvailabilityZones(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	a.Logger.WithField("step_id", planStep.ID).Info("Starting API retrieval for availability zones")

	// Check if user wants a specific number of AZs
	maxAZs := 0
	if maxParam, exists := planStep.Parameters["max_azs"]; exists {
		if maxFloat, ok := maxParam.(float64); ok {
			maxAZs = int(maxFloat)
		}
	}

	a.Logger.Info("Calling AWS API via awsClient.GetAvailabilityZones")
	azList, err := a.awsClient.GetAvailabilityZones(ctx)
	if err != nil {
		a.Logger.WithError(err).Error("AWS API call failed for GetAvailabilityZones")
		return nil, fmt.Errorf("failed to get availability zones: %w", err)
	}

	// Limit AZs if requested
	if maxAZs > 0 && len(azList) > maxAZs {
		azList = azList[:maxAZs]
		a.Logger.WithField("limited_to", maxAZs).Info("Limited AZ list to requested maximum")
	}

	a.Logger.WithFields(map[string]interface{}{
		"availability_zones": azList,
		"count":              len(azList),
	}).Info("AWS API call successful, received availability zones")

	// Store the first AZ as the resource value for dependency resolution
	primaryAZ := ""
	if len(azList) > 0 {
		primaryAZ = azList[0]
	}

	return map[string]interface{}{
		"value":        primaryAZ, // For {{step-id.resourceId}} resolution
		"all_zones":    azList,    // Full list available in result
		"count":        len(azList),
		"type":         "availability_zones",
		"retrieved_at": time.Now().Format(time.RFC3339),
		"description":  fmt.Sprintf("Available AZs in current region (primary: %s)", primaryAZ),
		"source":       "aws_api_call",
	}, nil
}

// retrieveExistingVPC gets an existing VPC ID (first available VPC or default VPC)
func (a *StateAwareAgent) retrieveExistingVPC(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	a.Logger.WithField("step_id", planStep.ID).Info("Starting API retrieval for existing VPC")

	// First try to get the default VPC
	a.Logger.Info("Attempting to find default VPC first")
	vpcID, err := a.awsClient.GetDefaultVPC(ctx)
	if err == nil && vpcID != "" {
		a.Logger.WithField("vpc_id", vpcID).Info("Found default VPC")
		return map[string]interface{}{
			"value":        vpcID,
			"type":         "vpc",
			"is_default":   true,
			"retrieved_at": time.Now().Format(time.RFC3339),
			"description":  "Default VPC for the current region",
			"source":       "aws_api_call",
		}, nil
	}

	// If no default VPC, get the first available VPC
	a.Logger.Info("No default VPC found, looking for any available VPC")
	vpcs, err := a.awsClient.DescribeVPCs(ctx)
	if err != nil {
		a.Logger.WithError(err).Error("AWS API call failed for DescribeVPCs")
		return nil, fmt.Errorf("failed to describe VPCs: %w", err)
	}

	if len(vpcs) == 0 {
		return nil, fmt.Errorf("no VPCs found in the region")
	}

	// Use the first available VPC
	firstVPC := vpcs[0]
	a.Logger.WithField("vpc_id", firstVPC.ID).Info("Using first available VPC")

	return map[string]interface{}{
		"value":        firstVPC.ID,
		"type":         "vpc",
		"is_default":   false,
		"retrieved_at": time.Now().Format(time.RFC3339),
		"description":  "First available VPC in the current region",
		"source":       "aws_api_call",
	}, nil
}

// retrieveSubnetsInVPC gets all subnets in a specified VPC
func (a *StateAwareAgent) retrieveSubnetsInVPC(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	a.Logger.WithField("step_id", planStep.ID).Info("Starting API retrieval for subnets in VPC")

	// Get VPC ID from parameters
	vpcID, exists := planStep.Parameters["vpc_id"]
	if !exists {
		// Try to get from resource mapping using a previous step
		if vpcIDParam, exists := planStep.Parameters["vpc_id_step"]; exists {
			if stepID, ok := vpcIDParam.(string); ok {
				a.mappingsMutex.RLock()
				mappedVPC, mappingExists := a.resourceMappings[stepID]
				a.mappingsMutex.RUnlock()
				if mappingExists && mappedVPC != "" {
					vpcID = mappedVPC
					a.Logger.WithFields(map[string]interface{}{
						"vpc_id":      vpcID,
						"source_step": stepID,
					}).Info("Retrieved VPC ID from previous step mapping")
				}
			}
		}
	}

	if vpcID == nil {
		return nil, fmt.Errorf("vpc_id parameter is required for subnets_in_vpc retrieval")
	}

	vpcIDStr, ok := vpcID.(string)
	if !ok {
		return nil, fmt.Errorf("vpc_id must be a string")
	}

	a.Logger.WithField("vpc_id", vpcIDStr).Info("Calling AWS API via awsClient.GetSubnetsInVPC")
	subnetIDs, err := a.awsClient.GetSubnetsInVPC(ctx, vpcIDStr)
	if err != nil {
		a.Logger.WithError(err).Error("AWS API call failed for GetSubnetsInVPC")
		return nil, fmt.Errorf("failed to get subnets in VPC %s: %w", vpcIDStr, err)
	}

	a.Logger.WithFields(map[string]interface{}{
		"vpc_id":     vpcIDStr,
		"subnet_ids": subnetIDs,
		"count":      len(subnetIDs),
	}).Info("AWS API call successful, received subnet IDs")

	// Use the first subnet as the primary value for dependency resolution
	primarySubnet := ""
	if len(subnetIDs) > 0 {
		primarySubnet = subnetIDs[0]
	}

	return map[string]interface{}{
		"value":        primarySubnet, // For {{step-id.resourceId}} resolution (first subnet)
		"subnet_ids":   subnetIDs,     // Full list of subnet IDs
		"vpc_id":       vpcIDStr,      // VPC ID for reference
		"count":        len(subnetIDs),
		"type":         "subnets",
		"retrieved_at": time.Now().Format(time.RFC3339),
		"description":  fmt.Sprintf("Found %d subnets in VPC %s (primary: %s)", len(subnetIDs), vpcIDStr, primarySubnet),
		"source":       "aws_api_call",
	}, nil
}

// retrieveSelectSubnetsForALB retrieves subnets suitable for ALB creation
func (a *StateAwareAgent) retrieveSelectSubnetsForALB(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	a.Logger.WithField("step_id", planStep.ID).Info("Starting subnet selection for ALB")

	// Get scheme from parameters (default to "internet-facing")
	scheme := "internet-facing"
	if schemeParam, exists := planStep.Parameters["scheme"]; exists {
		if schemeStr, ok := schemeParam.(string); ok {
			scheme = schemeStr
		}
	}

	// Get VPC ID from parameters if provided
	var vpcID string
	var vpcIDParam interface{}
	var exists bool

	// Try snake_case first (from AI plans)
	if vpcIDParam, exists = planStep.Parameters["vpc_id"]; exists {
		a.Logger.WithField("vpc_id_param_type", "snake_case").Debug("Found vpc_id parameter")
	} else if vpcIDParam, exists = planStep.Parameters["vpcId"]; exists {
		a.Logger.WithField("vpc_id_param_type", "camelCase").Debug("Found vpcId parameter")
	}

	if exists {
		if vpcIDStr, ok := vpcIDParam.(string); ok {
			// Check if it's a step reference that needs resolution
			if strings.Contains(vpcIDStr, "{{") && strings.Contains(vpcIDStr, "}}") {
				resolvedVPCID, err := a.resolveDependencyReference(vpcIDStr)
				if err != nil {
					a.Logger.WithError(err).WithField("vpc_id_reference", vpcIDStr).Error("Failed to resolve VPC ID reference")
					return nil, fmt.Errorf("failed to resolve VPC ID reference %s: %w", vpcIDStr, err)
				}
				vpcID = resolvedVPCID
				a.Logger.WithFields(map[string]interface{}{
					"vpc_id_reference": vpcIDStr,
					"resolved_vpc_id":  vpcID,
				}).Info("Resolved VPC ID from step reference")
			} else {
				vpcID = vpcIDStr
			}
		}
	}

	// Create the subnet selection tool and call it directly
	subnetSelector := tools.NewSelectSubnetsForALBTool(a.awsClient, "query", a.Logger)
	selectionArgs := map[string]interface{}{
		"scheme": scheme,
	}

	if vpcID != "" {
		selectionArgs["vpcId"] = vpcID
	}

	a.Logger.WithFields(map[string]interface{}{
		"scheme": scheme,
		"vpc_id": vpcID,
	}).Info("Calling subnet selection tool for ALB")

	selectionResult, err := subnetSelector.Execute(ctx, selectionArgs)
	if err != nil {
		a.Logger.WithError(err).Error("Failed to execute subnet selection tool")
		return nil, fmt.Errorf("failed to select subnets for ALB: %w", err)
	}

	if selectionResult.IsError {
		a.Logger.Error("Subnet selection tool returned error", "error", selectionResult.Content[0])
		return nil, fmt.Errorf("subnet selection failed: %v", selectionResult.Content[0])
	}

	// Parse the tool result
	if len(selectionResult.Content) > 0 {
		var textData string
		var extractSuccess bool

		// Try multiple approaches to extract text content, similar to ALB tools
		if textContent, ok := selectionResult.Content[0].(*mcp.TextContent); ok {
			textData = textContent.Text
			extractSuccess = true
			a.Logger.Debug("Successfully extracted text using TextContent pointer type assertion")
		} else if textContent, ok := selectionResult.Content[0].(mcp.TextContent); ok {
			// Try value type assertion
			textData = textContent.Text
			extractSuccess = true
			a.Logger.Debug("Successfully extracted text using TextContent value type assertion")
		} else {
			// Try to extract from any content with GetText method
			if contentInterface, ok := selectionResult.Content[0].(interface{ GetText() string }); ok {
				textData = contentInterface.GetText()
				extractSuccess = true
				a.Logger.Debug("Successfully extracted text using GetText method")
			}
		}

		if extractSuccess {
			var resultData map[string]interface{}
			if err := json.Unmarshal([]byte(textData), &resultData); err != nil {
				a.Logger.WithError(err).WithField("text_data", textData).Error("Failed to parse subnet selection response")
				return nil, fmt.Errorf("failed to parse subnet selection response: %w", err)
			}

			a.Logger.WithFields(map[string]interface{}{
				"result_data": resultData,
			}).Info("Successfully parsed subnet selection result")

			// Extract subnet IDs from the result
			var subnetIDs []string
			if subnetIDsData, exists := resultData["subnetIds"]; exists {
				if subnetIDsSlice, ok := subnetIDsData.([]interface{}); ok {
					subnetIDs = make([]string, len(subnetIDsSlice))
					for i, subnetID := range subnetIDsSlice {
						if subnetIDStr, ok := subnetID.(string); ok {
							subnetIDs[i] = subnetIDStr
						}
					}
				}
			}

			return map[string]interface{}{
				"value":        subnetIDs,      // For {{step-id.resourceId}} resolution
				"subnet_ids":   subnetIDs,      // Full list of subnet IDs
				"scheme":       scheme,         // ALB scheme for reference
				"count":        len(subnetIDs), // Number of selected subnets
				"type":         "alb_subnets",  // Resource type
				"retrieved_at": time.Now().Format(time.RFC3339),
				"description":  fmt.Sprintf("Selected %d subnets for %s ALB", len(subnetIDs), scheme),
				"source":       "subnet_selection_tool",
			}, nil
		} else {
			a.Logger.WithFields(map[string]interface{}{
				"actual_type":    fmt.Sprintf("%T", selectionResult.Content[0]),
				"content_string": fmt.Sprintf("%v", selectionResult.Content[0]),
			}).Error("Unable to extract text content from subnet selection result")
		}
	}

	return nil, fmt.Errorf("invalid or empty response from subnet selection tool")
}

// retrieveLoadBalancerArn retrieves load balancer ARN from a previous step
func (a *StateAwareAgent) retrieveLoadBalancerArn(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	a.Logger.WithField("step_id", planStep.ID).Info("Starting load balancer ARN retrieval")

	// Get the step reference from parameters
	stepRef, exists := planStep.Parameters["step_ref"]
	if !exists {
		return nil, fmt.Errorf("step_ref parameter is required for load_balancer_arn retrieval")
	}

	stepRefStr, ok := stepRef.(string)
	if !ok {
		return nil, fmt.Errorf("step_ref must be a string")
	}

	a.Logger.WithField("step_ref", stepRefStr).Info("Resolving load balancer ARN from step reference")

	// Use dependency resolution to get the ARN
	loadBalancerArn, err := a.resolveDependencyReference(stepRefStr)
	if err != nil {
		a.Logger.WithError(err).Error("Failed to resolve load balancer ARN reference")
		return nil, fmt.Errorf("failed to resolve load balancer ARN reference %s: %w", stepRefStr, err)
	}

	a.Logger.WithFields(map[string]interface{}{
		"step_ref":          stepRefStr,
		"load_balancer_arn": loadBalancerArn,
	}).Info("Successfully resolved load balancer ARN")

	return map[string]interface{}{
		"value":           loadBalancerArn,     // For {{step-id.resourceId}} resolution
		"loadBalancerArn": loadBalancerArn,     // Explicit ARN field
		"arn":             loadBalancerArn,     // Alternative key for ARN
		"type":            "load_balancer_arn", // Resource type
		"retrieved_at":    time.Now().Format(time.RFC3339),
		"description":     fmt.Sprintf("Load balancer ARN resolved from %s", stepRefStr),
		"source":          "step_reference",
	}, nil
}

// retrieveTargetGroupArn retrieves the target group ARN from a previous step
func (a *StateAwareAgent) retrieveTargetGroupArn(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	a.Logger.WithField("step_id", planStep.ID).Info("Retrieving target group ARN")

	// Extract the step_ref parameter
	stepRef, exists := planStep.Parameters["step_ref"]
	if !exists {
		return nil, fmt.Errorf("step_ref parameter is required for target_group_arn retrieval")
	}

	stepRefStr, ok := stepRef.(string)
	if !ok {
		return nil, fmt.Errorf("step_ref must be a string")
	}

	a.Logger.WithField("step_ref", stepRefStr).Info("Resolving target group ARN from step reference")

	// Use the existing dependency resolution mechanism
	targetGroupArn, err := a.resolveDependencyReference(stepRefStr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve target group ARN from step reference %s: %w", stepRefStr, err)
	}

	a.Logger.WithFields(map[string]interface{}{
		"target_group_arn": targetGroupArn,
		"step_ref":         stepRefStr,
	}).Info("Successfully resolved target group ARN")

	return map[string]interface{}{
		"value":          targetGroupArn,     // For {{step-id.resourceId}} resolution
		"targetGroupArn": targetGroupArn,     // Explicit ARN field
		"arn":            targetGroupArn,     // Alternative key for ARN
		"type":           "target_group_arn", // Resource type
		"retrieved_at":   time.Now().Format(time.RFC3339),
		"description":    fmt.Sprintf("Target group ARN resolved from %s", stepRefStr),
		"source":         "step_reference",
	}, nil
}

// retrieveDefaultAMIForRegion returns the default AMI ID for the current region by dynamically looking up the latest Amazon Linux 2 AMI
func (a *StateAwareAgent) retrieveDefaultAMIForRegion() string {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Try to get the latest Amazon Linux 2 AMI dynamically
	amiID, err := a.awsClient.GetLatestAmazonLinux2AMI(ctx)
	if err != nil {
		a.Logger.WithError(err).Warn("Failed to get latest Amazon Linux 2 AMI, using fallback")

		// Final fallback
		return ""
	}

	a.Logger.WithField("amiId", amiID).Info("Using dynamically discovered Amazon Linux 2 AMI")
	return amiID
}

// retrieveExistingResourceFromState retrieves existing resources from the managed state file
func (a *StateAwareAgent) retrieveExistingResourceFromState(planStep *types.ExecutionPlanStep, resourceType string) (map[string]interface{}, error) {
	a.Logger.WithFields(map[string]interface{}{
		"step_id":       planStep.ID,
		"resource_type": resourceType,
		"parameters":    planStep.Parameters,
	}).Info("Retrieving existing resource from state file")

	// Get resource name or identifier from parameters
	resourceName, _ := planStep.Parameters["resource_name"].(string)
	resourceID, _ := planStep.Parameters["resource_id"].(string)

	// If no specific name or ID provided, try to infer from step description or other parameters
	if resourceName == "" && resourceID == "" {
		// Try to extract from step name or description
		description := strings.ToLower(planStep.Description + " " + planStep.Name)
		if strings.Contains(description, "production-vpc") || strings.Contains(description, "production vpc") {
			resourceName = "production-vpc"
			a.Logger.WithField("step_id", planStep.ID).Debug("Inferred resource name 'production-vpc' from step description")
		} else if strings.Contains(description, "default-vpc") || strings.Contains(description, "default vpc") {
			resourceName = "default-vpc"
			a.Logger.WithField("step_id", planStep.ID).Debug("Inferred resource name 'default-vpc' from step description")
		}

		// Try to get from other common parameter names
		if resourceName == "" {
			if vpcName, ok := planStep.Parameters["vpc_name"].(string); ok && vpcName != "" {
				resourceName = vpcName
				a.Logger.WithField("step_id", planStep.ID).Debug("Using vpc_name parameter as resource_name")
			} else if name, ok := planStep.Parameters["name"].(string); ok && name != "" {
				resourceName = name
				a.Logger.WithField("step_id", planStep.ID).Debug("Using name parameter as resource_name")
			}
		}
	}

	// Use export-infrastructure-state tool to get current managed state
	stateResult, err := a.callMCPTool("export-infrastructure-state", map[string]interface{}{
		"format":             "json",
		"include_managed":    true,
		"include_discovered": false, // Only need managed state for this operation
	})
	if err != nil {
		return nil, fmt.Errorf("failed to export infrastructure state from MCP server: %w", err)
	}

	// Extract managed resources from the exported state
	var managedResources []interface{}
	if managedState, ok := stateResult["managed_state"].(map[string]interface{}); ok {
		if resources, ok := managedState["resources"].(map[string]interface{}); ok {
			// Convert map to slice for easier iteration
			for _, resource := range resources {
				managedResources = append(managedResources, resource)
			}
		}
	}

	a.Logger.WithFields(map[string]interface{}{
		"managed_resources_count": len(managedResources),
		"searching_for_name":      resourceName,
		"searching_for_id":        resourceID,
		"target_resource_type":    resourceType,
	}).Debug("Searching through managed resources")

	// Search through managed resources
	for _, resource := range managedResources {
		if resourceMap, ok := resource.(map[string]interface{}); ok {
			resourceTypeFromState, _ := resourceMap["type"].(string)
			resourceNameFromState, _ := resourceMap["name"].(string)
			resourceIDFromState, _ := resourceMap["id"].(string)

			// Check if this is a step_reference that contains actual resource info
			var actualResourceType, actualResourceName, actualAwsResourceID string
			isStepReference := resourceTypeFromState == "step_reference"

			if isStepReference {
				// Extract resource info from mcp_response for step references
				if properties, ok := resourceMap["properties"].(map[string]interface{}); ok {
					if mcpResponse, ok := properties["mcp_response"].(map[string]interface{}); ok {
						// Get the actual resource name from mcp_response
						if name, ok := mcpResponse["name"].(string); ok {
							actualResourceName = name
						}

						// Determine resource type and AWS ID from mcp_response structure
						if subnetID, ok := mcpResponse["subnetId"].(string); ok {
							actualResourceType = "subnet"
							actualAwsResourceID = subnetID
						} else if vpcID, ok := mcpResponse["vpcId"].(string); ok {
							actualResourceType = "vpc"
							actualAwsResourceID = vpcID
						} else if sgID, ok := mcpResponse["securityGroupId"].(string); ok {
							actualResourceType = "security_group"
							actualAwsResourceID = sgID
						} else if instanceID, ok := mcpResponse["instanceId"].(string); ok {
							actualResourceType = "ec2_instance"
							actualAwsResourceID = instanceID
						} else if resource, ok := mcpResponse["resource"].(map[string]interface{}); ok {
							if resourceType, ok := resource["type"].(string); ok {
								actualResourceType = resourceType
							}
							if resourceAwsID, ok := resource["id"].(string); ok {
								actualAwsResourceID = resourceAwsID
							}
						}
					}
				}
			}

			a.Logger.WithFields(map[string]interface{}{
				"state_resource_type":  resourceTypeFromState,
				"state_resource_name":  resourceNameFromState,
				"state_resource_id":    resourceIDFromState,
				"is_step_reference":    isStepReference,
				"actual_resource_type": actualResourceType,
				"actual_resource_name": actualResourceName,
				"actual_aws_id":        actualAwsResourceID,
			}).Debug("Checking resource in state")

			// Match by resource type if specified
			effectiveType := resourceTypeFromState
			effectiveName := resourceNameFromState
			if isStepReference && actualResourceType != "" {
				effectiveType = actualResourceType
				effectiveName = actualResourceName
			}

			if resourceType != "" && effectiveType != resourceType {
				continue
			} // Enhanced matching logic with support for subnet name-CIDR patterns
			nameMatch := false
			idMatch := false
			awsIdMatch := false

			// Basic name and ID matching - check both original and effective names
			if resourceName != "" {
				nameMatch = resourceNameFromState == resourceName ||
					strings.Contains(resourceNameFromState, resourceName) ||
					resourceIDFromState == resourceName

				// Also check effective name from step_reference mcp_response
				if isStepReference && effectiveName != "" {
					nameMatch = nameMatch ||
						effectiveName == resourceName ||
						strings.Contains(effectiveName, resourceName)
				}

				// Enhanced matching for ALB security groups
				if resourceType == "security_group" && resourceName == "alb-security-group" {
					// Check if this is the ALB security group by various patterns
					nameMatch = nameMatch ||
						strings.Contains(strings.ToLower(resourceNameFromState), "alb") ||
						strings.Contains(strings.ToLower(resourceNameFromState), "load balancer") ||
						strings.Contains(strings.ToLower(resourceIDFromState), "alb-security") ||
						resourceIDFromState == "step-create-alb-security-group"
				}
			}
			if resourceID != "" {
				idMatch = resourceIDFromState == resourceID
				// For step references, also check the actual AWS ID
				if isStepReference && actualAwsResourceID != "" {
					idMatch = idMatch || actualAwsResourceID == resourceID
				}
			}

			// Special handling for subnet resources with AWS ID cross-referencing
			if resourceType == "subnet" && resourceID != "" {
				// Check if the search ID is an AWS subnet ID and we need to find the matching custom-named resource
				if strings.HasPrefix(resourceID, "subnet-") {
					// Extract the actual AWS resource ID from properties.mcp_response
					if properties, ok := resourceMap["properties"].(map[string]interface{}); ok {
						if mcpResponse, ok := properties["mcp_response"].(map[string]interface{}); ok {
							if awsSubnetID, ok := mcpResponse["subnetId"].(string); ok && awsSubnetID == resourceID {
								awsIdMatch = true
								a.Logger.WithFields(map[string]interface{}{
									"search_aws_subnet_id": resourceID,
									"found_aws_subnet_id":  awsSubnetID,
									"custom_name":          resourceNameFromState,
								}).Debug("Found subnet by AWS ID cross-reference")
							} else if resource, ok := mcpResponse["resource"].(map[string]interface{}); ok {
								if awsSubnetID, ok := resource["id"].(string); ok && awsSubnetID == resourceID {
									awsIdMatch = true
									a.Logger.WithFields(map[string]interface{}{
										"search_aws_subnet_id": resourceID,
										"found_aws_subnet_id":  awsSubnetID,
										"custom_name":          resourceNameFromState,
									}).Debug("Found subnet by AWS ID cross-reference in resource object")
								}
							}
						}
					}
				}
			}

			if nameMatch || idMatch || awsIdMatch {
				// Extract the actual AWS resource ID from properties.mcp_response
				finalAwsResourceID := ""

				// For step references, we already extracted the AWS ID above
				if isStepReference && actualAwsResourceID != "" {
					finalAwsResourceID = actualAwsResourceID
				} else if properties, ok := resourceMap["properties"].(map[string]interface{}); ok {
					// Fallback to the original extraction logic for non-step-reference resources
					if mcpResponse, ok := properties["mcp_response"].(map[string]interface{}); ok {
						// Try different possible AWS resource ID fields based on resource type
						switch resourceType {
						case "vpc":
							// For VPC, try vpcId first, then resource.id
							if vpcID, ok := mcpResponse["vpcId"].(string); ok && vpcID != "" {
								finalAwsResourceID = vpcID
							} else if resource, ok := mcpResponse["resource"].(map[string]interface{}); ok {
								if resourceAwsID, ok := resource["id"].(string); ok && resourceAwsID != "" {
									finalAwsResourceID = resourceAwsID
								}
							}
						case "subnet":
							// For subnet, try subnetId first, then resource.id
							if subnetID, ok := mcpResponse["subnetId"].(string); ok && subnetID != "" {
								finalAwsResourceID = subnetID
							} else if resource, ok := mcpResponse["resource"].(map[string]interface{}); ok {
								if resourceAwsID, ok := resource["id"].(string); ok && resourceAwsID != "" {
									finalAwsResourceID = resourceAwsID
								}
							}
						case "security_group":
							// For security group, try securityGroupId or groupId, then resource.id
							if sgID, ok := mcpResponse["securityGroupId"].(string); ok && sgID != "" {
								finalAwsResourceID = sgID
							} else if groupID, ok := mcpResponse["groupId"].(string); ok && groupID != "" {
								finalAwsResourceID = groupID
							} else if resource, ok := mcpResponse["resource"].(map[string]interface{}); ok {
								if resourceAwsID, ok := resource["id"].(string); ok && resourceAwsID != "" {
									finalAwsResourceID = resourceAwsID
								}
							}
						case "ec2_instance":
							// For EC2 instance, try instanceId first, then resource.id
							if instanceID, ok := mcpResponse["instanceId"].(string); ok && instanceID != "" {
								finalAwsResourceID = instanceID
							} else if resource, ok := mcpResponse["resource"].(map[string]interface{}); ok {
								if resourceAwsID, ok := resource["id"].(string); ok && resourceAwsID != "" {
									finalAwsResourceID = resourceAwsID
								}
							}
						default:
							// For unknown types, try common patterns
							if resource, ok := mcpResponse["resource"].(map[string]interface{}); ok {
								if resourceAwsID, ok := resource["id"].(string); ok && resourceAwsID != "" {
									finalAwsResourceID = resourceAwsID
								}
							}
							// Try common AWS ID field patterns
							commonFields := []string{"vpcId", "subnetId", "instanceId", "securityGroupId", "groupId"}
							for _, field := range commonFields {
								if awsID, ok := mcpResponse[field].(string); ok && awsID != "" {
									finalAwsResourceID = awsID
									break
								}
							}
						}
					}
				}

				// Use the actual AWS resource ID if found, otherwise fall back to step ID
				finalResourceID := finalAwsResourceID
				if finalResourceID == "" {
					finalResourceID = resourceIDFromState
					a.Logger.WithFields(map[string]interface{}{
						"step_id":       resourceIDFromState,
						"resource_type": effectiveType,
						"warning":       "no_aws_id_found",
					}).Warn("Could not extract AWS resource ID from mcp_response, using step ID as fallback")
				}

				// Use effective name for step references
				finalResourceName := resourceNameFromState
				if isStepReference && effectiveName != "" {
					finalResourceName = effectiveName
				}

				a.Logger.WithFields(map[string]interface{}{
					"found_resource_id":   finalResourceID,
					"found_resource_name": finalResourceName,
					"found_resource_type": effectiveType,
					"step_id":             resourceIDFromState,
					"actual_aws_id":       finalAwsResourceID,
					"is_step_reference":   isStepReference,
				}).Info("Found matching resource in state file")

				return map[string]interface{}{
					"value":         finalResourceID,
					"resource_id":   finalResourceID,
					"resource_name": finalResourceName,
					"resource_type": effectiveType,
					"source":        "state_file",
					"step_id":       resourceIDFromState,
				}, nil
			}
		}
	}

	// If no exact match found, try partial matching for VPC names
	if resourceType == "vpc" || resourceType == "" {
		for _, resource := range managedResources {
			if resourceMap, ok := resource.(map[string]interface{}); ok {
				resourceTypeFromState, _ := resourceMap["type"].(string)
				resourceNameFromState, _ := resourceMap["name"].(string)
				resourceIDFromState, _ := resourceMap["id"].(string)

				// Check for VPC type and partial name match
				if (resourceTypeFromState == "vpc" || strings.Contains(resourceTypeFromState, "vpc")) &&
					(strings.Contains(strings.ToLower(resourceNameFromState), "production") ||
						strings.Contains(strings.ToLower(resourceNameFromState), "vpc")) {

					// Extract the actual AWS VPC ID from properties.mcp_response
					actualAwsResourceID := ""
					if properties, ok := resourceMap["properties"].(map[string]interface{}); ok {
						if mcpResponse, ok := properties["mcp_response"].(map[string]interface{}); ok {
							// Try vpcId first, then resource.id
							if vpcID, ok := mcpResponse["vpcId"].(string); ok && vpcID != "" {
								actualAwsResourceID = vpcID
							} else if resource, ok := mcpResponse["resource"].(map[string]interface{}); ok {
								if resourceAwsID, ok := resource["id"].(string); ok && resourceAwsID != "" {
									actualAwsResourceID = resourceAwsID
								}
							}
						}
					}

					// Use the actual AWS resource ID if found, otherwise fall back to step ID
					finalResourceID := actualAwsResourceID
					if finalResourceID == "" {
						finalResourceID = resourceIDFromState
						a.Logger.WithFields(map[string]interface{}{
							"step_id": resourceIDFromState,
							"warning": "no_aws_id_found_partial",
						}).Warn("Could not extract AWS VPC ID from mcp_response for partial match, using step ID as fallback")
					}

					a.Logger.WithFields(map[string]interface{}{
						"found_resource_id":   finalResourceID,
						"found_resource_name": resourceNameFromState,
						"found_resource_type": resourceTypeFromState,
						"step_id":             resourceIDFromState,
						"actual_aws_id":       actualAwsResourceID,
					}).Info("Found VPC resource with partial name match")

					return map[string]interface{}{
						"value":         finalResourceID,
						"resource_id":   finalResourceID,
						"resource_name": resourceNameFromState,
						"resource_type": resourceTypeFromState,
						"source":        "state_file",
						"match_type":    "partial",
						"step_id":       resourceIDFromState,
					}, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("resource not found in state file - name: '%s', id: '%s', type: '%s'", resourceName, resourceID, resourceType)
}

// retrieveLaunchTemplateId retrieves launch template ID from a previous step
func (a *StateAwareAgent) retrieveLaunchTemplateId(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	a.Logger.WithField("step_id", planStep.ID).Info("Starting launch template ID retrieval")

	// Get the step reference from parameters
	stepRef, exists := planStep.Parameters["step_ref"]
	if !exists {
		return nil, fmt.Errorf("step_ref parameter is required for launch_template_id retrieval")
	}

	stepRefStr, ok := stepRef.(string)
	if !ok {
		return nil, fmt.Errorf("step_ref must be a string")
	}

	a.Logger.WithField("step_ref", stepRefStr).Info("Resolving launch template ID from step reference")

	// Use dependency resolution to get the ID
	launchTemplateId, err := a.resolveDependencyReference(stepRefStr)
	if err != nil {
		a.Logger.WithError(err).Error("Failed to resolve launch template ID reference")
		return nil, fmt.Errorf("failed to resolve launch template ID reference %s: %w", stepRefStr, err)
	}

	a.Logger.WithFields(map[string]interface{}{
		"step_ref":           stepRefStr,
		"launch_template_id": launchTemplateId,
	}).Info("Successfully resolved launch template ID")

	return map[string]interface{}{
		"value":            launchTemplateId,     // For {{step-id.resourceId}} resolution
		"launchTemplateId": launchTemplateId,     // Explicit ID field
		"templateId":       launchTemplateId,     // Alternative key
		"type":             "launch_template_id", // Resource type
		"retrieved_at":     time.Now().Format(time.RFC3339),
		"description":      fmt.Sprintf("Launch template ID resolved from %s", stepRefStr),
		"source":           "step_reference",
	}, nil
}

// retrieveSecurityGroupId retrieves security group ID from a previous step
func (a *StateAwareAgent) retrieveSecurityGroupId(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	a.Logger.WithField("step_id", planStep.ID).Info("Starting security group ID retrieval")

	// Get the step reference from parameters
	stepRef, exists := planStep.Parameters["step_ref"]
	if !exists {
		return nil, fmt.Errorf("step_ref parameter is required for security_group_id retrieval")
	}

	stepRefStr, ok := stepRef.(string)
	if !ok {
		return nil, fmt.Errorf("step_ref must be a string")
	}

	a.Logger.WithField("step_ref", stepRefStr).Info("Resolving security group ID from step reference")

	// Use dependency resolution to get the ID
	securityGroupId, err := a.resolveDependencyReference(stepRefStr)
	if err != nil {
		a.Logger.WithError(err).Error("Failed to resolve security group ID reference")
		return nil, fmt.Errorf("failed to resolve security group ID reference %s: %w", stepRefStr, err)
	}

	a.Logger.WithFields(map[string]interface{}{
		"step_ref":          stepRefStr,
		"security_group_id": securityGroupId,
	}).Info("Successfully resolved security group ID")

	return map[string]interface{}{
		"value":           securityGroupId,     // For {{step-id.resourceId}} resolution
		"securityGroupId": securityGroupId,     // Explicit ID field
		"groupId":         securityGroupId,     // Alternative key
		"type":            "security_group_id", // Resource type
		"retrieved_at":    time.Now().Format(time.RFC3339),
		"description":     fmt.Sprintf("Security group ID resolved from %s", stepRefStr),
		"source":          "step_reference",
	}, nil
}

// retrieveDBSubnetGroupName retrieves DB subnet group name from a previous step
func (a *StateAwareAgent) retrieveDBSubnetGroupName(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	a.Logger.WithField("step_id", planStep.ID).Info("Starting DB subnet group name retrieval")

	// Get the step reference from parameters
	stepRef, exists := planStep.Parameters["step_ref"]
	if !exists {
		return nil, fmt.Errorf("step_ref parameter is required for db_subnet_group_name retrieval")
	}

	stepRefStr, ok := stepRef.(string)
	if !ok {
		return nil, fmt.Errorf("step_ref must be a string")
	}

	a.Logger.WithField("step_ref", stepRefStr).Info("Resolving DB subnet group name from step reference")

	// For DB subnet group, we might need to get the name rather than ID
	// First try to resolve normally
	dbSubnetGroupId, err := a.resolveDependencyReference(stepRefStr)
	if err != nil {
		a.Logger.WithError(err).Error("Failed to resolve DB subnet group reference")
		return nil, fmt.Errorf("failed to resolve DB subnet group reference %s: %w", stepRefStr, err)
	}

	// The DB subnet group creation typically returns the name as the ID
	// but we should also check if we have the name stored separately
	dbSubnetGroupName := dbSubnetGroupId

	a.Logger.WithFields(map[string]interface{}{
		"step_ref":             stepRefStr,
		"db_subnet_group_name": dbSubnetGroupName,
	}).Info("Successfully resolved DB subnet group name")

	return map[string]interface{}{
		"value":             dbSubnetGroupName,      // For {{step-id.resourceId}} resolution
		"dbSubnetGroupName": dbSubnetGroupName,      // Explicit name field
		"subnetGroupName":   dbSubnetGroupName,      // Alternative key
		"type":              "db_subnet_group_name", // Resource type
		"retrieved_at":      time.Now().Format(time.RFC3339),
		"description":       fmt.Sprintf("DB subnet group name resolved from %s", stepRefStr),
		"source":            "step_reference",
	}, nil
}

// retrieveAutoScalingGroupArn retrieves Auto Scaling Group ARN from a previous step
func (a *StateAwareAgent) retrieveAutoScalingGroupArn(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	a.Logger.WithField("step_id", planStep.ID).Info("Starting Auto Scaling Group ARN retrieval")

	// Get the step reference from parameters
	stepRef, exists := planStep.Parameters["step_ref"]
	if !exists {
		return nil, fmt.Errorf("step_ref parameter is required for auto_scaling_group_arn retrieval")
	}

	stepRefStr, ok := stepRef.(string)
	if !ok {
		return nil, fmt.Errorf("step_ref must be a string")
	}

	a.Logger.WithField("step_ref", stepRefStr).Info("Resolving Auto Scaling Group ARN from step reference")

	// Use dependency resolution to get the ARN
	asgArn, err := a.resolveDependencyReference(stepRefStr)
	if err != nil {
		a.Logger.WithError(err).Error("Failed to resolve Auto Scaling Group ARN reference")
		return nil, fmt.Errorf("failed to resolve Auto Scaling Group ARN reference %s: %w", stepRefStr, err)
	}

	a.Logger.WithFields(map[string]interface{}{
		"step_ref": stepRefStr,
		"asg_arn":  asgArn,
	}).Info("Successfully resolved Auto Scaling Group ARN")

	return map[string]interface{}{
		"value":               asgArn,                   // For {{step-id.resourceId}} resolution
		"autoScalingGroupArn": asgArn,                   // Explicit ARN field
		"asgArn":              asgArn,                   // Alternative key
		"arn":                 asgArn,                   // Generic ARN field
		"type":                "auto_scaling_group_arn", // Resource type
		"retrieved_at":        time.Now().Format(time.RFC3339),
		"description":         fmt.Sprintf("Auto Scaling Group ARN resolved from %s", stepRefStr),
		"source":              "step_reference",
	}, nil
}

// retrieveAutoScalingGroupName retrieves Auto Scaling Group name from a previous step
func (a *StateAwareAgent) retrieveAutoScalingGroupName(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	a.Logger.WithField("step_id", planStep.ID).Info("Starting Auto Scaling Group name retrieval")

	// Get the step reference from parameters
	stepRef, exists := planStep.Parameters["step_ref"]
	if !exists {
		return nil, fmt.Errorf("step_ref parameter is required for auto_scaling_group_name retrieval")
	}

	stepRefStr, ok := stepRef.(string)
	if !ok {
		return nil, fmt.Errorf("step_ref must be a string")
	}

	a.Logger.WithField("step_ref", stepRefStr).Info("Resolving Auto Scaling Group name from step reference")

	// For ASG, we might need to get the name from the resource properties
	// First try to resolve the reference
	asgId, err := a.resolveDependencyReference(stepRefStr)
	if err != nil {
		a.Logger.WithError(err).Error("Failed to resolve Auto Scaling Group reference")
		return nil, fmt.Errorf("failed to resolve Auto Scaling Group reference %s: %w", stepRefStr, err)
	}

	// The ASG creation typically returns the name as the ID
	asgName := asgId

	a.Logger.WithFields(map[string]interface{}{
		"step_ref": stepRefStr,
		"asg_name": asgName,
	}).Info("Successfully resolved Auto Scaling Group name")

	return map[string]interface{}{
		"value":                asgName,                   // For {{step-id.resourceId}} resolution
		"autoScalingGroupName": asgName,                   // Explicit name field
		"asgName":              asgName,                   // Alternative key
		"name":                 asgName,                   // Generic name field
		"type":                 "auto_scaling_group_name", // Resource type
		"retrieved_at":         time.Now().Format(time.RFC3339),
		"description":          fmt.Sprintf("Auto Scaling Group name resolved from %s", stepRefStr),
		"source":               "step_reference",
	}, nil
}

// retrieveRDSEndpoint retrieves RDS database endpoint from a previous step
func (a *StateAwareAgent) retrieveRDSEndpoint(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	a.Logger.WithField("step_id", planStep.ID).Info("Starting RDS endpoint retrieval")

	// Get the step reference from parameters
	stepRef, exists := planStep.Parameters["step_ref"]
	if !exists {
		return nil, fmt.Errorf("step_ref parameter is required for rds_endpoint retrieval")
	}

	stepRefStr, ok := stepRef.(string)
	if !ok {
		return nil, fmt.Errorf("step_ref must be a string")
	}

	// Get the RDS instance identifier from the dependency reference
	rdsInstanceId, err := a.resolveDependencyReference(stepRefStr)
	if err != nil {
		a.Logger.WithError(err).Error("Failed to resolve RDS instance reference")
		return nil, fmt.Errorf("failed to resolve RDS instance reference %s: %w", stepRefStr, err)
	}

	a.Logger.WithField("rds_instance_id", rdsInstanceId).Info("Calling AWS API to get RDS endpoint")

	// Call AWS API to get the RDS instance details including the endpoint
	dbInstance, err := a.awsClient.GetDBInstance(ctx, rdsInstanceId)
	if err != nil {
		a.Logger.WithError(err).Error("AWS API call failed for GetDBInstance")
		return nil, fmt.Errorf("failed to get RDS instance details for %s: %w", rdsInstanceId, err)
	}

	// Extract the endpoint from the DB instance Details
	endpoint := ""
	if dbInstance.Details != nil {
		if endpointVal, ok := dbInstance.Details["endpoint"].(string); ok {
			endpoint = endpointVal
		}
	}

	if endpoint == "" {
		return nil, fmt.Errorf("could not extract RDS endpoint from instance %s", rdsInstanceId)
	}

	a.Logger.WithFields(map[string]interface{}{
		"step_ref":        stepRefStr,
		"rds_instance_id": rdsInstanceId,
		"rds_endpoint":    endpoint,
	}).Info("Successfully resolved RDS endpoint via AWS API")

	return map[string]interface{}{
		"value":        endpoint,       // For {{step-id.resourceId}} resolution
		"endpoint":     endpoint,       // Explicit endpoint field
		"rdsEndpoint":  endpoint,       // Alternative key
		"address":      endpoint,       // Generic address field
		"type":         "rds_endpoint", // Resource type
		"retrieved_at": time.Now().Format(time.RFC3339),
		"description":  fmt.Sprintf("RDS endpoint resolved from %s", stepRefStr),
		"source":       "aws_api_call",
	}, nil
}
