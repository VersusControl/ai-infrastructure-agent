package agent

// import (
// 	"fmt"
// 	"strings"

// 	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
// )

// // ========== Interface defines ==========

// // ResourceCorrelationInterface defines resource correlation and matching functionality
// //
// // Available Functions:
// //   - analyzeResourceCorrelation()  : Analyze correlation between managed and discovered AWS resources
// //   - extractAWSResourceID()        : Extract AWS resource ID from managed resource properties
// //   - extractResourceCapabilities() : Extract resource capabilities dynamically
// //
// // This file handles correlation between managed infrastructure state and
// // discovered AWS resources for intelligent decision making and resource reuse.
// //
// // Usage Example:
// //   1. correlation := agent.analyzeResourceCorrelation(currentState, discoveredResources)
// //   2. match := correlation["managed-resource-id"]
// //   3. Use match.DiscoveredResource.ID for resource reuse in infrastructure decisions

// // ========== Resource Correlation and Matching Functions ==========

// // analyzeResourceCorrelation analyzes correlation between managed and discovered resources
// func (a *StateAwareAgent) analyzeResourceCorrelation(currentState *types.InfrastructureState, discoveredResources []*types.ResourceState) map[string]*ResourceMatch {
// 	correlation := make(map[string]*ResourceMatch)

// 	// For each managed resource, try to find its corresponding discovered resource
// 	for managedID, managedResource := range currentState.Resources {

// 		// Extract AWS resource ID from managed resource properties
// 		awsResourceID := a.extractAWSResourceID(managedResource)
// 		if awsResourceID == "" {
// 			continue
// 		}

// 		// Find the corresponding discovered resource
// 		for _, discoveredResource := range discoveredResources {
// 			if discoveredResource.ID == awsResourceID {

// 				// Extract capabilities based on resource type dynamically
// 				capabilities := a.extractResourceCapabilities(discoveredResource)

// 				correlation[managedID] = &ResourceMatch{
// 					ManagedResource:    managedResource,
// 					DiscoveredResource: discoveredResource,
// 					MatchConfidence:    1.0, // Perfect match by AWS ID
// 					MatchReason:        fmt.Sprintf("AWS ID match: %s", awsResourceID),
// 					Capabilities:       capabilities,
// 				}
// 				break
// 			}
// 		}
// 	}

// 	return correlation
// }

// // extractAWSResourceID extracts the actual AWS resource ID from managed resource properties
// func (a *StateAwareAgent) extractAWSResourceID(resource *types.ResourceState) string {
// 	if resource.Properties == nil {
// 		return ""
// 	}

// 	// Check for MCP response which usually contains the actual AWS resource ID
// 	if mcpResponse, exists := resource.Properties["mcp_response"]; exists {
// 		if mcpMap, ok := mcpResponse.(map[string]interface{}); ok {
// 			// Define mappings for all supported AWS resource types
// 			resourceIDMappings := map[string][]string{
// 				"SECURITY_GROUP":     {"groupId", "securityGroupId"},
// 				"VPC":                {"vpcId"},
// 				"EC2_INSTANCE":       {"instanceId"},
// 				"SUBNET":             {"subnetId"},
// 				"INTERNET_GATEWAY":   {"internetGatewayId", "gatewayId"},
// 				"NAT_GATEWAY":        {"natGatewayId"},
// 				"ROUTE_TABLE":        {"routeTableId"},
// 				"LAUNCH_TEMPLATE":    {"launchTemplateId"},
// 				"AUTO_SCALING_GROUP": {"autoScalingGroupName", "asgName"},
// 				"LOAD_BALANCER":      {"loadBalancerArn", "arn"},
// 				"TARGET_GROUP":       {"targetGroupArn", "arn"},
// 				"LISTENER":           {"listenerArn", "arn"},
// 				"DB_INSTANCE":        {"dbInstanceIdentifier"},
// 				"DB_SUBNET_GROUP":    {"dbSubnetGroupName"},
// 				"DB_SNAPSHOT":        {"dbSnapshotIdentifier"},
// 				"AMI":                {"imageId", "amiId"},
// 				"KEY_PAIR":           {"keyName"},
// 			}

// 			resourceType := strings.ToUpper(resource.Type)
// 			if possibleKeys, exists := resourceIDMappings[resourceType]; exists {
// 				for _, key := range possibleKeys {
// 					if value, exists := mcpMap[key]; exists {
// 						if id, ok := value.(string); ok && id != "" {
// 							return id
// 						}
// 					}
// 				}
// 			}
// 		}
// 	}

// 	return ""
// }

// // extractResourceCapabilities extracts meaningful capabilities from a discovered resource dynamically
// func (a *StateAwareAgent) extractResourceCapabilities(resource *types.ResourceState) map[string]interface{} {
// 	capabilities := make(map[string]interface{})

// 	if resource.Properties == nil {
// 		return capabilities
// 	}

// 	// Add basic resource information
// 	capabilities["resource_type"] = resource.Type
// 	capabilities["resource_status"] = resource.Status

// 	// Extract capabilities based on resource type dynamically
// 	switch strings.ToUpper(resource.Type) {
// 	case "SECURITY_GROUP":
// 		a.extractSecurityGroupCapabilities(resource, capabilities)
// 	case "VPC":
// 		a.extractVPCCapabilities(resource, capabilities)
// 	case "EC2_INSTANCE":
// 		a.extractEC2Capabilities(resource, capabilities)
// 	case "SUBNET":
// 		a.extractSubnetCapabilities(resource, capabilities)
// 	case "LOAD_BALANCER":
// 		a.extractLoadBalancerCapabilities(resource, capabilities)
// 	case "TARGET_GROUP":
// 		a.extractTargetGroupCapabilities(resource, capabilities)
// 	case "AUTO_SCALING_GROUP":
// 		a.extractAutoScalingGroupCapabilities(resource, capabilities)
// 	case "LAUNCH_TEMPLATE":
// 		a.extractLaunchTemplateCapabilities(resource, capabilities)
// 	case "DB_INSTANCE":
// 		a.extractDBInstanceCapabilities(resource, capabilities)
// 	default:
// 		// For any other resource type, extract all available properties
// 		a.extractGenericCapabilities(resource, capabilities)
// 	}

// 	return capabilities
// }
