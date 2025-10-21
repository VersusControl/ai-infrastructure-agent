package agent

// import (
// 	"fmt"
// 	"strings"

// 	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
// )

// // ========== Interface defines ==========

// // CapabilityExtractorsInterface defines resource capability extraction functionality
// //
// // Available Functions:
// //   - extractSecurityGroupCapabilities() : Extract security group capabilities
// //   - extractVPCCapabilities()           : Extract VPC capabilities
// //   - extractEC2Capabilities()           : Extract EC2 instance capabilities
// //   - extractSubnetCapabilities()        : Extract subnet capabilities
// //   - extractLoadBalancerCapabilities()  : Extract load balancer capabilities
// //   - extractTargetGroupCapabilities()   : Extract target group capabilities
// //   - extractAutoScalingGroupCapabilities() : Extract ASG capabilities
// //   - extractLaunchTemplateCapabilities() : Extract launch template capabilities
// //   - extractDBInstanceCapabilities()    : Extract RDS instance capabilities
// //   - extractGenericCapabilities()       : Extract generic resource capabilities
// //
// // This file provides specialized capability extraction for different AWS
// // resource types to enable intelligent infrastructure analysis and planning.
// //
// // Usage Example:
// //   1. capabilities := make(map[string]interface{})
// //   2. agent.extractSecurityGroupCapabilities(resource, capabilities)
// //   3. Use capabilities for resource correlation and decision making

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
