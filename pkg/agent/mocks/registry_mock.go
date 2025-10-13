package mocks

import (
	"context"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/versus-control/ai-infrastructure-agent/pkg/agent/retrieval"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// MockRetrievalRegistry provides comprehensive mock functionality that implements the same interface as RetrievalRegistry
// This allows it to be used as a drop-in replacement for the real registry in tests
type MockRetrievalRegistry struct {
	functions map[string]retrieval.RetrievalFunction
	patterns  map[string]*MockPatternEntry
	mu        sync.RWMutex
	awsClient *MockAWSClient // Use mock AWS client for realistic data structures
}

// MockPatternEntry contains a compiled regex pattern and its associated function
type MockPatternEntry struct {
	Pattern  *regexp.Regexp
	Function retrieval.RetrievalFunction
}

// NewMockRetrievalRegistry creates a new mock registry with MockAWSClient integration
func NewMockRetrievalRegistry(awsClient *MockAWSClient) *MockRetrievalRegistry {
	registry := &MockRetrievalRegistry{
		functions: make(map[string]retrieval.RetrievalFunction),
		patterns:  make(map[string]*MockPatternEntry),
		awsClient: awsClient,
	}

	// Register all mock functions matching agent_retrieval.go
	registry.registerAllMockFunctions()

	return registry
}

// RegisterRetrieval registers a mock retrieval function for exact value type matching
// This implements the same interface as the real RetrievalRegistry
func (r *MockRetrievalRegistry) RegisterRetrieval(valueType string, fn retrieval.RetrievalFunction) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.functions[valueType] = fn
}

// RegisterPattern registers a mock retrieval function for pattern-based matching
func (r *MockRetrievalRegistry) RegisterPattern(pattern string, fn retrieval.RetrievalFunction) error {
	compiledPattern, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid pattern %s: %w", pattern, err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.patterns[pattern] = &MockPatternEntry{
		Pattern:  compiledPattern,
		Function: fn,
	}

	return nil
}

// Execute executes the appropriate mock retrieval function for the given value type
func (r *MockRetrievalRegistry) Execute(ctx context.Context, valueType string, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Try exact match first
	if fn, exists := r.functions[valueType]; exists {
		return fn(ctx, planStep)
	}

	// Try pattern matching
	for _, entry := range r.patterns {
		if entry.Pattern.MatchString(valueType) {
			return entry.Function(ctx, planStep)
		}
	}

	// Return default mock response if no match
	return map[string]interface{}{
		"value":       fmt.Sprintf("mock-%s-value", valueType),
		"source":      "mock_registry",
		"value_type":  valueType,
		"resource_id": planStep.ResourceID,
		"action":      planStep.Name,
	}, nil
}

// GetRegisteredTypes returns all registered exact match types
// This implements the same interface as the real RetrievalRegistry
func (r *MockRetrievalRegistry) GetRegisteredTypes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]string, 0, len(r.functions))
	for valueType := range r.functions {
		types = append(types, valueType)
	}
	return types
}

// GetRegisteredPatterns returns all registered patterns
// This implements the same interface as the real RetrievalRegistry
func (r *MockRetrievalRegistry) GetRegisteredPatterns() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	patterns := make([]string, 0, len(r.patterns))
	for pattern := range r.patterns {
		patterns = append(patterns, pattern)
	}
	return patterns
}

// registerAllMockFunctions registers all mock retrieval functions matching initializeRetrievalRegistry
func (r *MockRetrievalRegistry) registerAllMockFunctions() {
	// Direct registrations for exact matches (matching agent_plan_executor.go)
	r.RegisterRetrieval("latest_ami", r.mockRetrieveLatestAMI)
	r.RegisterRetrieval("default_vpc", r.mockRetrieveDefaultVPC)
	r.RegisterRetrieval("existing_vpc", r.mockRetrieveExistingVPC)
	r.RegisterRetrieval("default_subnet", r.mockRetrieveDefaultSubnet)
	r.RegisterRetrieval("subnets_in_vpc", r.mockRetrieveSubnetsInVPC)
	r.RegisterRetrieval("available_azs", r.mockRetrieveAvailabilityZones)
	r.RegisterRetrieval("select_subnets_for_alb", r.mockRetrieveSelectSubnetsForALB)
	r.RegisterRetrieval("load_balancer_arn", r.mockRetrieveLoadBalancerArn)
	r.RegisterRetrieval("target_group_arn", r.mockRetrieveTargetGroupArn)
	r.RegisterRetrieval("launch_template_id", r.mockRetrieveLaunchTemplateId)
	r.RegisterRetrieval("security_group_id_ref", r.mockRetrieveSecurityGroupId)
	r.RegisterRetrieval("db_subnet_group_name", r.mockRetrieveDBSubnetGroupName)
	r.RegisterRetrieval("auto_scaling_group_arn", r.mockRetrieveAutoScalingGroupArn)
	r.RegisterRetrieval("auto_scaling_group_name", r.mockRetrieveAutoScalingGroupName)
	r.RegisterRetrieval("rds_endpoint", r.mockRetrieveRDSEndpoint)

	// State-based retrievals
	r.RegisterRetrieval("vpc_id", func(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
		return r.mockRetrieveExistingResourceFromState(planStep, "vpc")
	})
	r.RegisterRetrieval("subnet_id", func(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
		return r.mockRetrieveExistingResourceFromState(planStep, "subnet")
	})
	r.RegisterRetrieval("security_group_id", func(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
		return r.mockRetrieveExistingResourceFromState(planStep, "security_group")
	})
	r.RegisterRetrieval("instance_id", func(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
		return r.mockRetrieveExistingResourceFromState(planStep, "ec2_instance")
	})
	r.RegisterRetrieval("existing_resource", func(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
		return r.mockRetrieveExistingResourceFromState(planStep, "")
	})

	// Register pattern-based mocks
	r.registerPatternMocks()
}

// ========== Mock Retrieval Functions (matching agent_retrieval.go) ==========

// mockRetrieveLatestAMI gets the latest AMI using MockAWSClient
func (r *MockRetrievalRegistry) mockRetrieveLatestAMI(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	// Get the OS type from parameters (default to Amazon Linux 2) to match real function behavior
	osType := "amazon-linux-2"
	if osParam, exists := planStep.Parameters["os_type"]; exists {
		osType = fmt.Sprintf("%v", osParam)
	}

	// Get the architecture (default to x86_64) to match real function behavior
	architecture := "x86_64"
	if archParam, exists := planStep.Parameters["architecture"]; exists {
		architecture = fmt.Sprintf("%v", archParam)
	}

	// Use MockAWSClient to get realistic AMI data
	amiID, err := r.awsClient.GetLatestAmazonLinux2AMI(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve latest AMI: %w", err)
	}

	// Try to get detailed AMI info, but provide fallback if not found
	_, err = r.awsClient.GetAMI(ctx, amiID)
	if err != nil {
		// Fallback to basic AMI information if detailed lookup fails - match real function format
		return map[string]interface{}{
			"value":        amiID,
			"type":         "ami",
			"os_type":      osType,
			"architecture": architecture,
			"retrieved_at": time.Now().Format(time.RFC3339),
			"description":  fmt.Sprintf("Latest %s AMI for %s architecture", osType, architecture),
			"source":       "aws_api_call", // Match real function source
		}, nil
	}

	// Full details available - match real function format exactly
	return map[string]interface{}{
		"value":        amiID,
		"type":         "ami",
		"os_type":      osType,
		"architecture": architecture,
		"retrieved_at": time.Now().Format(time.RFC3339),
		"description":  fmt.Sprintf("Latest %s AMI for %s architecture", osType, architecture),
		"source":       "aws_api_call", // Match real function source
	}, nil
}

// mockRetrieveDefaultVPC retrieves default VPC using MockAWSClient
func (r *MockRetrievalRegistry) mockRetrieveDefaultVPC(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	defaultVPCID, err := r.awsClient.GetDefaultVPC(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve default VPC: %w", err)
	}

	return map[string]interface{}{
		"value":        defaultVPCID,
		"type":         "vpc",
		"is_default":   true,
		"retrieved_at": time.Now().Format(time.RFC3339),
		"description":  "Default VPC for the current region",
		"source":       "aws_api_call",
	}, nil
}

// mockRetrieveExistingVPC finds existing VPC (default or first available)
func (r *MockRetrievalRegistry) mockRetrieveExistingVPC(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	// Try default VPC first
	vpcID, err := r.awsClient.GetDefaultVPC(ctx)
	if err == nil && vpcID != "" {
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
	vpcs, err := r.awsClient.DescribeVPCs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to describe VPCs: %w", err)
	}

	if len(vpcs) == 0 {
		return nil, fmt.Errorf("no VPCs found in region")
	}

	firstVPC := vpcs[0]
	return map[string]interface{}{
		"value":        firstVPC.ID,
		"type":         "vpc",
		"is_default":   false,
		"retrieved_at": time.Now().Format(time.RFC3339),
		"description":  "First available VPC in the current region",
		"source":       "aws_api_call",
	}, nil
}

// mockRetrieveDefaultSubnet retrieves default subnet using MockAWSClient
func (r *MockRetrievalRegistry) mockRetrieveDefaultSubnet(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	subnetInfo, err := r.awsClient.GetDefaultSubnet(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve default subnet: %w", err)
	}

	return map[string]interface{}{
		"value":        subnetInfo.SubnetID, // For {{step-id.resourceId}} resolution (subnet ID)
		"subnet_id":    subnetInfo.SubnetID, // Explicit subnet ID
		"vpc_id":       subnetInfo.VPCID,    // Explicit VPC ID for security groups
		"type":         "subnet",
		"is_default":   true,
		"retrieved_at": time.Now().Format(time.RFC3339),
		"description":  fmt.Sprintf("Default subnet (%s) in VPC (%s)", subnetInfo.SubnetID, subnetInfo.VPCID),
		"source":       "aws_api_call",
	}, nil
}

// mockRetrieveAvailabilityZones retrieves available AZs using MockAWSClient
func (r *MockRetrievalRegistry) mockRetrieveAvailabilityZones(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	azs, err := r.awsClient.GetAvailabilityZones(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve availability zones: %w", err)
	}

	// Store the first AZ as the resource value for dependency resolution
	primaryAZ := ""
	if len(azs) > 0 {
		primaryAZ = azs[0]
	}

	return map[string]interface{}{
		"value":        primaryAZ, // For {{step-id.resourceId}} resolution
		"all_zones":    azs,       // Full list available in result
		"count":        len(azs),
		"type":         "availability_zones",
		"retrieved_at": time.Now().Format(time.RFC3339),
		"description":  fmt.Sprintf("Available AZs in current region (primary: %s)", primaryAZ),
		"source":       "aws_api_call",
	}, nil
}

// mockRetrieveSubnetsInVPC retrieves all subnets in specified VPC using MockAWSClient
func (r *MockRetrievalRegistry) mockRetrieveSubnetsInVPC(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	// Extract VPC ID from plan step parameters
	vpcID, ok := planStep.Parameters["vpc_id"].(string)
	if !ok {
		return nil, fmt.Errorf("vpc_id parameter not found or not a string")
	}

	subnetIDs, err := r.awsClient.GetSubnetsInVPC(ctx, vpcID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve subnets in VPC %s: %w", vpcID, err)
	}

	// Get detailed subnet information
	var subnets []map[string]interface{}
	for _, subnetID := range subnetIDs {
		subnetResource, err := r.awsClient.GetSubnet(ctx, subnetID)
		if err != nil {
			continue // Skip failed subnet lookups
		}
		subnets = append(subnets, map[string]interface{}{
			"subnet_id":         subnetID,
			"vpc_id":            vpcID,
			"availability_zone": subnetResource.Details["AvailabilityZone"],
			"cidr_block":        subnetResource.Details["CidrBlock"],
			"state":             subnetResource.State,
		})
	}

	return map[string]interface{}{
		"subnets":    subnets,
		"subnet_ids": subnetIDs,
		"vpc_id":     vpcID,
		"count":      len(subnetIDs),
		"source":     "aws_api_call",
		"region":     r.awsClient.GetRegion(),
	}, nil
}

// mockRetrieveSelectSubnetsForALB selects appropriate subnets for ALB creation
func (r *MockRetrievalRegistry) mockRetrieveSelectSubnetsForALB(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	// Get ALB scheme from parameters (default to internet-facing)
	scheme := "internet-facing"
	if schemeParam, exists := planStep.Parameters["scheme"]; exists {
		if schemeStr, ok := schemeParam.(string); ok {
			scheme = schemeStr
		}
	}

	// Mock subnet selection for ALB (minimum 2 subnets from different AZs required)
	selectedSubnets := []string{"subnet-mock123456", "subnet-mock789012"}

	return map[string]interface{}{
		"value":        selectedSubnets,      // For {{step-id.resourceId}} resolution
		"subnet_ids":   selectedSubnets,      // Full list of subnet IDs
		"scheme":       scheme,               // ALB scheme for reference
		"count":        len(selectedSubnets), // Number of selected subnets
		"type":         "alb_subnets",        // Resource type
		"retrieved_at": time.Now().Format(time.RFC3339),
		"description":  fmt.Sprintf("Selected %d subnets for %s ALB", len(selectedSubnets), scheme),
		"source":       "subnet_selection_tool",
	}, nil
}

// mockRetrieveLoadBalancerArn retrieves load balancer ARN from previous steps
func (r *MockRetrievalRegistry) mockRetrieveLoadBalancerArn(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	// Mock step reference resolution - simulate getting ARN from a previous step
	stepRef := "mock-alb-step"
	if stepRefParam, exists := planStep.Parameters["step_ref"]; exists {
		if stepRefStr, ok := stepRefParam.(string); ok {
			stepRef = stepRefStr
		}
	}

	// Mock load balancer ARN
	mockArn := fmt.Sprintf("arn:aws:elasticloadbalancing:us-west-2:123456789012:loadbalancer/app/mock-alb-%s/1234567890abcdef", planStep.ResourceID)

	return map[string]interface{}{
		"value":           mockArn,             // For {{step-id.resourceId}} resolution
		"loadBalancerArn": mockArn,             // Explicit ARN field
		"arn":             mockArn,             // Alternative key for ARN
		"type":            "load_balancer_arn", // Resource type
		"retrieved_at":    time.Now().Format(time.RFC3339),
		"description":     fmt.Sprintf("Load balancer ARN resolved from %s", stepRef),
		"source":          "step_reference",
	}, nil
}

// mockRetrieveTargetGroupArn retrieves target group ARN from previous steps
func (r *MockRetrievalRegistry) mockRetrieveTargetGroupArn(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	// Mock step reference resolution - simulate getting ARN from a previous step
	stepRef := "mock-tg-step"
	if stepRefParam, exists := planStep.Parameters["step_ref"]; exists {
		if stepRefStr, ok := stepRefParam.(string); ok {
			stepRef = stepRefStr
		}
	}

	// Mock target group ARN
	mockArn := fmt.Sprintf("arn:aws:elasticloadbalancing:us-west-2:123456789012:targetgroup/mock-tg-%s/1234567890abcdef", planStep.ResourceID)

	return map[string]interface{}{
		"value":          mockArn,            // For {{step-id.resourceId}} resolution
		"targetGroupArn": mockArn,            // Explicit ARN field
		"arn":            mockArn,            // Alternative key for ARN
		"type":           "target_group_arn", // Resource type
		"retrieved_at":   time.Now().Format(time.RFC3339),
		"description":    fmt.Sprintf("Target group ARN resolved from %s", stepRef),
		"source":         "step_reference",
	}, nil
}

// mockRetrieveLaunchTemplateId retrieves launch template ID from previous steps
func (r *MockRetrievalRegistry) mockRetrieveLaunchTemplateId(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	// Mock step reference resolution - simulate getting ID from a previous step
	stepRef := "mock-lt-step"
	if stepRefParam, exists := planStep.Parameters["step_ref"]; exists {
		if stepRefStr, ok := stepRefParam.(string); ok {
			stepRef = stepRefStr
		}
	}

	// Mock launch template ID
	mockId := fmt.Sprintf("lt-mock%s123456", planStep.ResourceID)

	return map[string]interface{}{
		"value":            mockId,               // For {{step-id.resourceId}} resolution
		"launchTemplateId": mockId,               // Explicit ID field
		"templateId":       mockId,               // Alternative key
		"type":             "launch_template_id", // Resource type
		"retrieved_at":     time.Now().Format(time.RFC3339),
		"description":      fmt.Sprintf("Launch template ID resolved from %s", stepRef),
		"source":           "step_reference",
	}, nil
}

// mockRetrieveSecurityGroupId retrieves security group ID from previous steps
func (r *MockRetrievalRegistry) mockRetrieveSecurityGroupId(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	// Mock step reference resolution - simulate getting ID from a previous step
	stepRef := "mock-sg-step"
	if stepRefParam, exists := planStep.Parameters["step_ref"]; exists {
		if stepRefStr, ok := stepRefParam.(string); ok {
			stepRef = stepRefStr
		}
	}

	// Mock security group ID
	mockId := fmt.Sprintf("sg-mock%s123456", planStep.ResourceID)

	return map[string]interface{}{
		"value":           mockId,              // For {{step-id.resourceId}} resolution
		"securityGroupId": mockId,              // Explicit ID field
		"groupId":         mockId,              // Alternative key
		"type":            "security_group_id", // Resource type
		"retrieved_at":    time.Now().Format(time.RFC3339),
		"description":     fmt.Sprintf("Security group ID resolved from %s", stepRef),
		"source":          "step_reference",
	}, nil
}

// mockRetrieveDBSubnetGroupName retrieves DB subnet group name from previous steps
func (r *MockRetrievalRegistry) mockRetrieveDBSubnetGroupName(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	// Mock step reference resolution - simulate getting name from a previous step
	stepRef := "mock-dbsg-step"
	if stepRefParam, exists := planStep.Parameters["step_ref"]; exists {
		if stepRefStr, ok := stepRefParam.(string); ok {
			stepRef = stepRefStr
		}
	}

	// Mock DB subnet group name
	mockName := fmt.Sprintf("mock-db-subnet-group-%s", planStep.ResourceID)

	return map[string]interface{}{
		"value":             mockName,               // For {{step-id.resourceId}} resolution
		"dbSubnetGroupName": mockName,               // Explicit name field
		"subnetGroupName":   mockName,               // Alternative key
		"type":              "db_subnet_group_name", // Resource type
		"retrieved_at":      time.Now().Format(time.RFC3339),
		"description":       fmt.Sprintf("DB subnet group name resolved from %s", stepRef),
		"source":            "step_reference",
	}, nil
}

// mockRetrieveAutoScalingGroupArn retrieves Auto Scaling Group ARN from previous steps
func (r *MockRetrievalRegistry) mockRetrieveAutoScalingGroupArn(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	// Mock step reference resolution - simulate getting ARN from a previous step
	stepRef := "mock-asg-step"
	if stepRefParam, exists := planStep.Parameters["step_ref"]; exists {
		if stepRefStr, ok := stepRefParam.(string); ok {
			stepRef = stepRefStr
		}
	}

	// Mock Auto Scaling Group ARN
	mockArn := fmt.Sprintf("arn:aws:autoscaling:us-west-2:123456789012:autoScalingGroup:1234567890abcdef:autoScalingGroupName/mock-asg-%s", planStep.ResourceID)

	return map[string]interface{}{
		"value":               mockArn,                  // For {{step-id.resourceId}} resolution
		"autoScalingGroupArn": mockArn,                  // Explicit ARN field
		"asgArn":              mockArn,                  // Alternative key
		"arn":                 mockArn,                  // Generic ARN field
		"type":                "auto_scaling_group_arn", // Resource type
		"retrieved_at":        time.Now().Format(time.RFC3339),
		"description":         fmt.Sprintf("Auto Scaling Group ARN resolved from %s", stepRef),
		"source":              "step_reference",
	}, nil
}

// mockRetrieveAutoScalingGroupName retrieves Auto Scaling Group name from previous steps
func (r *MockRetrievalRegistry) mockRetrieveAutoScalingGroupName(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	// Mock step reference resolution - simulate getting name from a previous step
	stepRef := "mock-asg-step"
	if stepRefParam, exists := planStep.Parameters["step_ref"]; exists {
		if stepRefStr, ok := stepRefParam.(string); ok {
			stepRef = stepRefStr
		}
	}

	// Mock Auto Scaling Group name
	mockName := fmt.Sprintf("mock-asg-name-%s", planStep.ResourceID)

	return map[string]interface{}{
		"value":                mockName,                  // For {{step-id.resourceId}} resolution
		"autoScalingGroupName": mockName,                  // Explicit name field
		"asgName":              mockName,                  // Alternative key
		"name":                 mockName,                  // Generic name field
		"type":                 "auto_scaling_group_name", // Resource type
		"retrieved_at":         time.Now().Format(time.RFC3339),
		"description":          fmt.Sprintf("Auto Scaling Group name resolved from %s", stepRef),
		"source":               "step_reference",
	}, nil
}

// mockRetrieveRDSEndpoint retrieves RDS database endpoint from previous steps
func (r *MockRetrievalRegistry) mockRetrieveRDSEndpoint(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	// Mock step reference resolution to get RDS instance ID, then make AWS call
	stepRef := "mock-rds-step"
	if stepRefParam, exists := planStep.Parameters["step_ref"]; exists {
		if stepRefStr, ok := stepRefParam.(string); ok {
			stepRef = stepRefStr
		}
	}

	// Mock AWS API call to get RDS endpoint (simulating the real function behavior)
	mockEndpoint := fmt.Sprintf("mock-rds-%s.123456789012.us-west-2.rds.amazonaws.com", planStep.ResourceID)

	return map[string]interface{}{
		"value":        mockEndpoint,   // For {{step-id.resourceId}} resolution
		"endpoint":     mockEndpoint,   // Explicit endpoint field
		"rdsEndpoint":  mockEndpoint,   // Alternative key
		"address":      mockEndpoint,   // Generic address field
		"type":         "rds_endpoint", // Resource type
		"retrieved_at": time.Now().Format(time.RFC3339),
		"description":  fmt.Sprintf("RDS endpoint resolved from %s", stepRef),
		"source":       "aws_api_call",
	}, nil
}

// mockRetrieveExistingResourceFromState retrieves existing resources from the managed state
func (r *MockRetrievalRegistry) mockRetrieveExistingResourceFromState(planStep *types.ExecutionPlanStep, resourceType string) (map[string]interface{}, error) {
	// Generate proper AWS resource ID format instead of generic mock ID
	var resourceID string
	switch resourceType {
	case "vpc":
		resourceID = fmt.Sprintf("vpc-%08x", len(planStep.ResourceID)*12345) // vpc-xxxxxxxx
	case "subnet":
		resourceID = fmt.Sprintf("subnet-%08x", len(planStep.ResourceID)*23456) // subnet-xxxxxxxx
	case "security_group":
		resourceID = fmt.Sprintf("sg-%08x", len(planStep.ResourceID)*34567) // sg-xxxxxxxx
	case "ec2_instance":
		resourceID = fmt.Sprintf("i-%08x", len(planStep.ResourceID)*45678) // i-xxxxxxxx
	default:
		resourceID = fmt.Sprintf("mock-%s-%s", resourceType, planStep.ResourceID)
	}

	baseResource := map[string]interface{}{
		"value":         resourceID,
		"resource_id":   resourceID,
		"resource_type": resourceType,
		"state":         "available",
		"source":        "state_file",
		"region":        r.awsClient.GetRegion(),
	}

	// Add resource-specific properties to match real function behavior
	switch resourceType {
	case "vpc":
		baseResource["vpc_id"] = resourceID
		baseResource["cidr_block"] = "10.0.0.0/16"
		baseResource["is_default"] = false
	case "subnet":
		baseResource["subnet_id"] = resourceID
		baseResource["vpc_id"] = "vpc-mock1234"
		baseResource["availability_zone"] = "us-west-2a"
		baseResource["cidr_block"] = "10.0.1.0/24"
	case "security_group":
		baseResource["group_id"] = resourceID
		baseResource["group_name"] = fmt.Sprintf("mock-sg-%s", planStep.ResourceID)
		baseResource["vpc_id"] = "vpc-mock1234"
	case "ec2_instance":
		baseResource["instance_id"] = resourceID
		baseResource["instance_type"] = "t3.micro"
		baseResource["subnet_id"] = "subnet-mock123"
		baseResource["vpc_id"] = "vpc-mock1234"
	}

	return baseResource, nil
}

// registerPatternMocks registers pattern-based mock functions
func (r *MockRetrievalRegistry) registerPatternMocks() {
	r.RegisterPattern(".*_id$", func(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
		return map[string]interface{}{
			"id":          fmt.Sprintf("mock-%s-123456", planStep.ResourceID),
			"source":      "mock_pattern_api",
			"pattern":     ".*_id$",
			"resource_id": planStep.ResourceID,
		}, nil
	})

	r.RegisterPattern(".*_arn$", func(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
		return map[string]interface{}{
			"arn":         fmt.Sprintf("arn:aws:service:us-west-2:123456789012:resource/mock-%s", planStep.ResourceID),
			"source":      "mock_pattern_api",
			"pattern":     ".*_arn$",
			"resource_id": planStep.ResourceID,
		}, nil
	})

	r.RegisterPattern(".*_name$", func(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
		return map[string]interface{}{
			"name":        fmt.Sprintf("mock-%s-name", planStep.ResourceID),
			"source":      "mock_pattern_api",
			"pattern":     ".*_name$",
			"resource_id": planStep.ResourceID,
		}, nil
	})
}
