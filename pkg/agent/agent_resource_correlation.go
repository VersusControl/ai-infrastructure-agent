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

// // ========== Resource Capability Extraction Functions ==========

// // extractSecurityGroupCapabilities extracts security group specific capabilities
// func (a *StateAwareAgent) extractSecurityGroupCapabilities(resource *types.ResourceState, capabilities map[string]interface{}) {
// 	if vpcId, exists := resource.Properties["vpcId"]; exists {
// 		capabilities["vpc_id"] = vpcId
// 	}
// 	if groupName, exists := resource.Properties["groupName"]; exists {
// 		capabilities["group_name"] = groupName
// 	}
// 	if description, exists := resource.Properties["description"]; exists {
// 		capabilities["description"] = description
// 	}

// 	// Extract ingress rules dynamically
// 	if ingressRules, exists := resource.Properties["ingress_rules"]; exists {
// 		capabilities["ingress_rules"] = ingressRules
// 		capabilities["ingress_rule_count"] = a.countRules(ingressRules)

// 		// Analyze ports dynamically
// 		openPorts := a.extractOpenPorts(ingressRules)
// 		capabilities["open_ports"] = openPorts
// 		capabilities["port_count"] = len(openPorts)

// 		// Check for common ports dynamically
// 		commonPorts := map[string]int{
// 			"http": 80, "https": 443, "ssh": 22, "ftp": 21, "smtp": 25,
// 			"dns": 53, "dhcp": 67, "pop3": 110, "imap": 143, "ldap": 389,
// 			"mysql": 3306, "postgresql": 5432, "redis": 6379, "mongodb": 27017,
// 		}

// 		for service, port := range commonPorts {
// 			capabilities[fmt.Sprintf("allows_%s", service)] = a.hasPortInRules(ingressRules, port)
// 		}
// 	}

// 	// Extract egress rules dynamically
// 	if egressRules, exists := resource.Properties["egress_rules"]; exists {
// 		capabilities["egress_rules"] = egressRules
// 		capabilities["egress_rule_count"] = a.countRules(egressRules)
// 	}
// }

// // extractVPCCapabilities extracts VPC specific capabilities
// func (a *StateAwareAgent) extractVPCCapabilities(resource *types.ResourceState, capabilities map[string]interface{}) {
// 	if cidr, exists := resource.Properties["cidrBlock"]; exists {
// 		capabilities["cidr_block"] = cidr
// 	}
// 	if state, exists := resource.Properties["state"]; exists {
// 		capabilities["state"] = state
// 	}
// 	if isDefault, exists := resource.Properties["isDefault"]; exists {
// 		capabilities["is_default"] = isDefault
// 	}
// 	if dhcpOptionsId, exists := resource.Properties["dhcpOptionsId"]; exists {
// 		capabilities["dhcp_options_id"] = dhcpOptionsId
// 	}
// }

// // extractEC2Capabilities extracts EC2 instance specific capabilities
// func (a *StateAwareAgent) extractEC2Capabilities(resource *types.ResourceState, capabilities map[string]interface{}) {
// 	properties := []string{
// 		"instanceType", "state", "vpcId", "subnetId", "availabilityZone",
// 		"privateIpAddress", "publicIpAddress", "keyName", "platform",
// 		"architecture", "virtualizationType", "hypervisor",
// 	}

// 	for _, prop := range properties {
// 		if value, exists := resource.Properties[prop]; exists {
// 			capabilities[strings.ToLower(prop)] = value
// 		}
// 	}

// 	// Extract security groups
// 	if securityGroups, exists := resource.Properties["securityGroups"]; exists {
// 		capabilities["security_groups"] = securityGroups
// 	}
// }

// // extractSubnetCapabilities extracts subnet specific capabilities
// func (a *StateAwareAgent) extractSubnetCapabilities(resource *types.ResourceState, capabilities map[string]interface{}) {
// 	properties := []string{
// 		"vpcId", "cidrBlock", "availabilityZone", "state",
// 		"mapPublicIpOnLaunch", "assignIpv6AddressOnCreation",
// 	}

// 	for _, prop := range properties {
// 		if value, exists := resource.Properties[prop]; exists {
// 			capabilities[strings.ToLower(prop)] = value
// 		}
// 	}
// }

// // extractLoadBalancerCapabilities extracts load balancer specific capabilities
// func (a *StateAwareAgent) extractLoadBalancerCapabilities(resource *types.ResourceState, capabilities map[string]interface{}) {
// 	properties := []string{
// 		"type", "scheme", "state", "vpcId", "ipAddressType",
// 		"dnsName", "canonicalHostedZoneId",
// 	}

// 	for _, prop := range properties {
// 		if value, exists := resource.Properties[prop]; exists {
// 			capabilities[strings.ToLower(prop)] = value
// 		}
// 	}

// 	if securityGroups, exists := resource.Properties["securityGroups"]; exists {
// 		capabilities["security_groups"] = securityGroups
// 	}
// 	if subnets, exists := resource.Properties["subnets"]; exists {
// 		capabilities["subnets"] = subnets
// 	}
// }

// // extractTargetGroupCapabilities extracts target group specific capabilities
// func (a *StateAwareAgent) extractTargetGroupCapabilities(resource *types.ResourceState, capabilities map[string]interface{}) {
// 	properties := []string{
// 		"port", "protocol", "vpcId", "healthCheckPath", "healthCheckProtocol",
// 		"healthCheckIntervalSeconds", "healthCheckTimeoutSeconds", "targetType",
// 	}

// 	for _, prop := range properties {
// 		if value, exists := resource.Properties[prop]; exists {
// 			capabilities[strings.ToLower(prop)] = value
// 		}
// 	}
// }

// // extractAutoScalingGroupCapabilities extracts ASG specific capabilities
// func (a *StateAwareAgent) extractAutoScalingGroupCapabilities(resource *types.ResourceState, capabilities map[string]interface{}) {
// 	properties := []string{
// 		"minSize", "maxSize", "desiredCapacity", "launchTemplateName",
// 		"healthCheckType", "healthCheckGracePeriod",
// 	}

// 	for _, prop := range properties {
// 		if value, exists := resource.Properties[prop]; exists {
// 			capabilities[strings.ToLower(prop)] = value
// 		}
// 	}

// 	if zones, exists := resource.Properties["availabilityZones"]; exists {
// 		capabilities["availability_zones"] = zones
// 	}
// 	if subnets, exists := resource.Properties["vpcZoneIdentifiers"]; exists {
// 		capabilities["subnets"] = subnets
// 	}
// }

// // extractLaunchTemplateCapabilities extracts launch template specific capabilities
// func (a *StateAwareAgent) extractLaunchTemplateCapabilities(resource *types.ResourceState, capabilities map[string]interface{}) {
// 	properties := []string{
// 		"imageId", "instanceType", "keyName", "userData",
// 	}

// 	for _, prop := range properties {
// 		if value, exists := resource.Properties[prop]; exists {
// 			capabilities[strings.ToLower(prop)] = value
// 		}
// 	}

// 	if securityGroups, exists := resource.Properties["securityGroups"]; exists {
// 		capabilities["security_groups"] = securityGroups
// 	}
// }

// // extractDBInstanceCapabilities extracts RDS instance specific capabilities
// func (a *StateAwareAgent) extractDBInstanceCapabilities(resource *types.ResourceState, capabilities map[string]interface{}) {
// 	properties := []string{
// 		"engine", "engineVersion", "dbInstanceClass", "allocatedStorage",
// 		"storageType", "storageEncrypted", "multiAZ", "publiclyAccessible",
// 		"endpoint", "port", "masterUsername",
// 	}

// 	for _, prop := range properties {
// 		if value, exists := resource.Properties[prop]; exists {
// 			capabilities[strings.ToLower(prop)] = value
// 		}
// 	}
// }

// // extractGenericCapabilities extracts capabilities for any resource type
// func (a *StateAwareAgent) extractGenericCapabilities(resource *types.ResourceState, capabilities map[string]interface{}) {
// 	// Add all properties as capabilities for unknown resource types
// 	for key, value := range resource.Properties {
// 		if key != "Tags" && key != "tags" { // Skip tags to reduce noise
// 			capabilities[strings.ToLower(key)] = value
// 		}
// 	}
// }

// // countRules counts the number of rules in a rule set
// func (a *StateAwareAgent) countRules(rulesInterface interface{}) int {
// 	if rules, ok := rulesInterface.([]interface{}); ok {
// 		return len(rules)
// 	}
// 	if rules, ok := rulesInterface.([]map[string]interface{}); ok {
// 		return len(rules)
// 	}
// 	return 0
// }

// // extractOpenPorts extracts all open ports from security group rules
// func (a *StateAwareAgent) extractOpenPorts(rulesInterface interface{}) []int {
// 	var ports []int
// 	portSet := make(map[int]bool)

// 	if rules, ok := rulesInterface.([]interface{}); ok {
// 		for _, rule := range rules {
// 			if ruleMap, ok := rule.(map[string]interface{}); ok {
// 				a.addPortsFromRule(ruleMap, portSet)
// 			}
// 		}
// 	} else if rules, ok := rulesInterface.([]map[string]interface{}); ok {
// 		for _, rule := range rules {
// 			a.addPortsFromRule(rule, portSet)
// 		}
// 	}

// 	for port := range portSet {
// 		ports = append(ports, port)
// 	}

// 	return ports
// }

// // addPortsFromRule adds ports from a single rule to the port set
// func (a *StateAwareAgent) addPortsFromRule(rule map[string]interface{}, portSet map[int]bool) {
// 	if fromPort, exists := rule["from_port"]; exists {
// 		if toPort, exists := rule["to_port"]; exists {
// 			if from, ok := fromPort.(int); ok {
// 				if to, ok := toPort.(int); ok {
// 					for port := from; port <= to; port++ {
// 						portSet[port] = true
// 					}
// 				}
// 			}
// 		}
// 	}
// }

// // hasPortInRules checks if security group rules include a specific port
// func (a *StateAwareAgent) hasPortInRules(rulesInterface interface{}, targetPort int) bool {
// 	if rules, ok := rulesInterface.([]interface{}); ok {
// 		for _, rule := range rules {
// 			if ruleMap, ok := rule.(map[string]interface{}); ok {
// 				if a.ruleIncludesPort(ruleMap, targetPort) {
// 					return true
// 				}
// 			}
// 		}
// 	} else if rules, ok := rulesInterface.([]map[string]interface{}); ok {
// 		for _, rule := range rules {
// 			if a.ruleIncludesPort(rule, targetPort) {
// 				return true
// 			}
// 		}
// 	}
// 	return false
// }

// // ruleIncludesPort checks if a single rule includes the target port
// func (a *StateAwareAgent) ruleIncludesPort(rule map[string]interface{}, targetPort int) bool {
// 	if fromPort, exists := rule["from_port"]; exists {
// 		if toPort, exists := rule["to_port"]; exists {
// 			if from, ok := fromPort.(int); ok {
// 				if to, ok := toPort.(int); ok {
// 					return from <= targetPort && targetPort <= to
// 				}
// 			}
// 		}
// 	}
// 	return false
// }
