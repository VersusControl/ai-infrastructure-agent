package mocks

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/aws"
	"github.com/versus-control/ai-infrastructure-agent/pkg/interfaces"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// MockMCPServer implements a mock MCP server for testing
type MockMCPServer struct {
	logger       *logging.Logger
	tools        map[string]interfaces.MCPTool
	resources    map[string]*types.AWSResource
	stateManager *MockStateManager
	awsClient    *MockAWSClient
	mutex        sync.RWMutex

	// Response configuration
	simulateErrors  map[string]error
	customResponses map[string]*mcp.CallToolResult
}

// NewMockMCPServer creates a new mock MCP server
func NewMockMCPServer(logger *logging.Logger) *MockMCPServer {
	return &MockMCPServer{
		logger:          logger,
		tools:           make(map[string]interfaces.MCPTool),
		resources:       make(map[string]*types.AWSResource),
		stateManager:    NewMockStateManager(),
		awsClient:       nil, // Will be set via SetAWSClient
		simulateErrors:  make(map[string]error),
		customResponses: make(map[string]*mcp.CallToolResult),
	}
}

// NewMockMCPServerWithAWSClient creates a new mock MCP server with AWS client
func NewMockMCPServerWithAWSClient(logger *logging.Logger, awsClient *MockAWSClient) *MockMCPServer {
	return &MockMCPServer{
		logger:          logger,
		tools:           make(map[string]interfaces.MCPTool),
		resources:       make(map[string]*types.AWSResource),
		stateManager:    NewMockStateManager(),
		awsClient:       awsClient,
		simulateErrors:  make(map[string]error),
		customResponses: make(map[string]*mcp.CallToolResult),
	}
}

// SetAWSClient sets the AWS client for the mock server
func (m *MockMCPServer) SetAWSClient(awsClient *MockAWSClient) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.awsClient = awsClient
}

// Execute implements the MCPTool interface for the mock server itself
func (m *MockMCPServer) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	// This method allows the MockMCPServer to be used as an MCPTool
	// It delegates to CallTool with a default tool name if provided in arguments
	if toolName, ok := arguments["tool_name"].(string); ok {
		// Remove tool_name from arguments before calling the tool
		toolArgs := make(map[string]interface{})
		for k, v := range arguments {
			if k != "tool_name" {
				toolArgs[k] = v
			}
		}
		return m.CallTool(ctx, toolName, toolArgs)
	}

	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			&mcp.TextContent{
				Type: "text",
				Text: "MockMCPServer Execute requires 'tool_name' parameter",
			},
		},
	}, nil
}

// Name implements MCPTool interface
func (m *MockMCPServer) Name() string {
	return "mock-mcp-server"
}

// Description implements MCPTool interface
func (m *MockMCPServer) Description() string {
	return "Mock MCP server for testing"
}

// Category implements MCPTool interface
func (m *MockMCPServer) Category() string {
	return "testing"
}

// GetInputSchema implements MCPTool interface
func (m *MockMCPServer) GetInputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"tool_name": map[string]interface{}{
				"type":        "string",
				"description": "Name of the tool to execute",
			},
		},
		"required": []string{"tool_name"},
	}
}

// GetOutputSchema implements MCPTool interface
func (m *MockMCPServer) GetOutputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":        "object",
		"description": "Tool execution result",
	}
}

// GetExamples implements MCPTool interface
func (m *MockMCPServer) GetExamples() []interfaces.ToolExample {
	return []interfaces.ToolExample{
		{
			Description: "Execute a mock tool",
			Arguments: map[string]interface{}{
				"tool_name": "create-vpc",
				"name":      "test-vpc",
				"cidrBlock": "10.0.0.0/16",
			},
			Expected: "VPC creation result",
		},
	}
}

// ValidateArguments implements MCPTool interface
func (m *MockMCPServer) ValidateArguments(arguments map[string]interface{}) error {
	if _, ok := arguments["tool_name"]; !ok {
		return fmt.Errorf("tool_name is required")
	}
	return nil
}

// RegisterTool registers a tool with the mock server
func (m *MockMCPServer) RegisterTool(tool interfaces.MCPTool) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.tools[tool.Name()] = tool
	m.logger.Info("Registered mock tool", "tool", tool.Name())
	return nil
}

// CallTool simulates calling an MCP tool
func (m *MockMCPServer) CallTool(ctx context.Context, toolName string, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Check for simulated errors
	if err, exists := m.simulateErrors[toolName]; exists {
		return nil, err
	}

	// Check for custom responses
	if response, exists := m.customResponses[toolName]; exists {
		return response, nil
	}

	// Check if tool exists
	tool, exists := m.tools[toolName]
	if !exists {
		// Generate default mock response for common tools
		return m.generateMockToolResponse(toolName, arguments)
	}

	// Execute the actual tool with mocked dependencies
	return tool.Execute(ctx, arguments)
}

// generateMockToolResponse generates default responses for common AWS tools
func (m *MockMCPServer) generateMockToolResponse(toolName string, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	// If AWS client is available, use AWS client versions for better integration
	if m.awsClient != nil {
		return m.generateMockToolResponseWithAWSClient(toolName, arguments)
	}

	// Fallback to static responses when no AWS client is available
	return m.generateMockToolResponseFallback(toolName, arguments)
}

// generateMockToolResponseWithAWSClient uses AWS client for realistic mock responses
func (m *MockMCPServer) generateMockToolResponseWithAWSClient(toolName string, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	switch {
	// VPC Tools
	case toolName == "create-vpc":
		return m.mockCreateVPCWithAWSClient(arguments)
	case toolName == "list-vpcs":
		return m.mockListVPCsWithAWSClient(arguments)
	case toolName == "get-default-vpc":
		return m.mockGetDefaultVPC(arguments)

	// Subnet Tools
	case toolName == "create-subnet":
		return m.mockCreateSubnetWithAWSClient(arguments)
	case toolName == "create-private-subnet":
		return m.mockCreatePrivateSubnet(arguments)
	case toolName == "create-public-subnet":
		return m.mockCreatePublicSubnet(arguments)
	case toolName == "list-subnets":
		return m.mockListSubnets(arguments)
	case toolName == "list-subnets-for-alb":
		return m.mockSelectSubnetsForALB(arguments)

	// Networking Infrastructure Tools
	case toolName == "create-internet-gateway":
		return m.mockCreateInternetGateway(arguments)
	case toolName == "create-nat-gateway":
		return m.mockCreateNATGateway(arguments)
	case toolName == "create-public-route-table":
		return m.mockCreatePublicRouteTable(arguments)
	case toolName == "create-private-route-table":
		return m.mockCreatePrivateRouteTable(arguments)
	case toolName == "associate-route-table":
		return m.mockAssociateRouteTable(arguments)
	case toolName == "add-route":
		return m.mockAddRoute(arguments)

	// EC2 Tools
	case toolName == "create-ec2-instance":
		return m.mockCreateEC2InstanceWithAWSClient(arguments)
	case toolName == "list-ec2-instances":
		return m.mockListEC2Instances(arguments)
	case toolName == "start-ec2-instance":
		return m.mockStartEC2Instance(arguments)
	case toolName == "stop-ec2-instance":
		return m.mockStopEC2Instance(arguments)
	case toolName == "terminate-ec2-instance":
		return m.mockTerminateEC2Instance(arguments)
	case toolName == "create-ami-from-instance":
		return m.mockCreateAMIFromInstance(arguments)
	case toolName == "list-amis":
		return m.mockListAMIs(arguments)
	case toolName == "get-latest-ami":
		return m.mockGetLatestAMI(arguments)
	case toolName == "get-latest-amazon-linux-ami":
		return m.mockGetLatestAmazonLinuxAMI(arguments)
	case toolName == "get-latest-ubuntu-ami":
		return m.mockGetLatestUbuntuAMI(arguments)

	// Load Balancer Tools
	case toolName == "create-load-balancer":
		return m.mockCreateLoadBalancer(arguments)
	case toolName == "create-target-group":
		return m.mockCreateTargetGroup(arguments)
	case toolName == "create-listener":
		return m.mockCreateListener(arguments)
	case toolName == "list-load-balancers":
		return m.mockListLoadBalancers(arguments)
	case toolName == "list-target-groups":
		return m.mockListTargetGroups(arguments)
	case toolName == "register-targets":
		return m.mockRegisterTargets(arguments)
	case toolName == "deregister-targets":
		return m.mockDeregisterTargets(arguments)

	// RDS Tools
	case toolName == "create-db-subnet-group":
		return m.mockCreateDBSubnetGroup(arguments)
	case toolName == "create-db-instance":
		return m.mockCreateDBInstance(arguments)
	case toolName == "start-db-instance":
		return m.mockStartDBInstance(arguments)
	case toolName == "stop-db-instance":
		return m.mockStopDBInstance(arguments)
	case toolName == "delete-db-instance":
		return m.mockDeleteDBInstance(arguments)
	case toolName == "create-db-snapshot":
		return m.mockCreateDBSnapshot(arguments)
	case toolName == "list-db-instances":
		return m.mockListDBInstances(arguments)
	case toolName == "list-db-snapshots":
		return m.mockListDBSnapshots(arguments)

	// Security Group Tools
	case toolName == "create-security-group":
		return m.mockCreateSecurityGroup(arguments)
	case toolName == "list-security-groups":
		return m.mockListSecurityGroups(arguments)
	case toolName == "add-security-group-ingress-rule":
		return m.mockAddSecurityGroupIngressRule(arguments)
	case toolName == "add-security-group-egress-rule":
		return m.mockAddSecurityGroupEgressRule(arguments)
	case toolName == "delete-security-group":
		return m.mockDeleteSecurityGroup(arguments)

	// Auto Scaling Tools
	case toolName == "create-launch-template":
		return m.mockCreateLaunchTemplate(arguments)
	case toolName == "create-auto-scaling-group":
		return m.mockCreateAutoScalingGroup(arguments)
	case toolName == "list-auto-scaling-groups":
		return m.mockListAutoScalingGroups(arguments)
	case toolName == "list-launch-templates":
		return m.mockListLaunchTemplates(arguments)

	// State Tools
	case toolName == "export-infrastructure-state":
		return m.mockExportInfrastructureState(arguments)
	case toolName == "analyze-infrastructure-state":
		return m.mockAnalyzeInfrastructureState(arguments)
	case toolName == "visualize-dependency-graph":
		return m.mockVisualizeDependencyGraph(arguments)
	case toolName == "detect-infrastructure-conflicts":
		return m.mockDetectInfrastructureConflicts(arguments)
	case toolName == "plan-infrastructure-deployment":
		return m.mockPlanInfrastructureDeployment(arguments)
	case toolName == "add-resource-to-state":
		return m.mockAddResourceToState(arguments)
	case toolName == "save-state":
		return m.mockSaveState(arguments)

	// Utility Tools
	case toolName == "get-availability-zones":
		return m.mockGetAvailabilityZones(arguments)

	default:
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Mock tool '%s' not implemented", toolName),
				},
			},
		}, nil
	}
}

// generateMockToolResponseFallback provides static responses when no AWS client is available
func (m *MockMCPServer) generateMockToolResponseFallback(toolName string, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	switch {
	// VPC Tools
	case toolName == "create-vpc":
		return m.mockCreateVPC(arguments)
	case toolName == "list-vpcs":
		return m.mockListVPCs(arguments)
	case toolName == "get-default-vpc":
		return m.mockGetDefaultVPC(arguments)

	// Subnet Tools
	case toolName == "create-subnet":
		return m.mockCreateSubnet(arguments)
	case toolName == "create-private-subnet":
		return m.mockCreatePrivateSubnet(arguments)
	case toolName == "create-public-subnet":
		return m.mockCreatePublicSubnet(arguments)
	case toolName == "list-subnets":
		return m.mockListSubnets(arguments)
	case toolName == "list-subnets-for-alb":
		return m.mockSelectSubnetsForALB(arguments)

	// Networking Infrastructure Tools
	case toolName == "create-internet-gateway":
		return m.mockCreateInternetGateway(arguments)
	case toolName == "create-nat-gateway":
		return m.mockCreateNATGateway(arguments)
	case toolName == "create-public-route-table":
		return m.mockCreatePublicRouteTable(arguments)
	case toolName == "create-private-route-table":
		return m.mockCreatePrivateRouteTable(arguments)
	case toolName == "associate-route-table":
		return m.mockAssociateRouteTable(arguments)
	case toolName == "add-route":
		return m.mockAddRoute(arguments)

	// EC2 Tools
	case toolName == "create-ec2-instance":
		return m.mockCreateEC2Instance(arguments)
	case toolName == "list-ec2-instances":
		return m.mockListEC2Instances(arguments)
	case toolName == "start-ec2-instance":
		return m.mockStartEC2Instance(arguments)
	case toolName == "stop-ec2-instance":
		return m.mockStopEC2Instance(arguments)
	case toolName == "terminate-ec2-instance":
		return m.mockTerminateEC2Instance(arguments)
	case toolName == "create-ami-from-instance":
		return m.mockCreateAMIFromInstance(arguments)
	case toolName == "list-amis":
		return m.mockListAMIs(arguments)
	case toolName == "get-latest-ami":
		return m.mockGetLatestAMI(arguments)
	case toolName == "get-latest-amazon-linux-ami":
		return m.mockGetLatestAmazonLinuxAMI(arguments)
	case toolName == "get-latest-ubuntu-ami":
		return m.mockGetLatestUbuntuAMI(arguments)

	// Load Balancer Tools
	case toolName == "create-load-balancer":
		return m.mockCreateLoadBalancer(arguments)
	case toolName == "create-target-group":
		return m.mockCreateTargetGroup(arguments)
	case toolName == "create-listener":
		return m.mockCreateListener(arguments)
	case toolName == "list-load-balancers":
		return m.mockListLoadBalancers(arguments)
	case toolName == "list-target-groups":
		return m.mockListTargetGroups(arguments)
	case toolName == "register-targets":
		return m.mockRegisterTargets(arguments)
	case toolName == "deregister-targets":
		return m.mockDeregisterTargets(arguments)

	// RDS Tools
	case toolName == "create-db-subnet-group":
		return m.mockCreateDBSubnetGroup(arguments)
	case toolName == "create-db-instance":
		return m.mockCreateDBInstance(arguments)
	case toolName == "start-db-instance":
		return m.mockStartDBInstance(arguments)
	case toolName == "stop-db-instance":
		return m.mockStopDBInstance(arguments)
	case toolName == "delete-db-instance":
		return m.mockDeleteDBInstance(arguments)
	case toolName == "create-db-snapshot":
		return m.mockCreateDBSnapshot(arguments)
	case toolName == "list-db-instances":
		return m.mockListDBInstances(arguments)
	case toolName == "list-db-snapshots":
		return m.mockListDBSnapshots(arguments)

	// Security Group Tools
	case toolName == "create-security-group":
		return m.mockCreateSecurityGroup(arguments)
	case toolName == "list-security-groups":
		return m.mockListSecurityGroups(arguments)
	case toolName == "add-security-group-ingress-rule":
		return m.mockAddSecurityGroupIngressRule(arguments)
	case toolName == "add-security-group-egress-rule":
		return m.mockAddSecurityGroupEgressRule(arguments)
	case toolName == "delete-security-group":
		return m.mockDeleteSecurityGroup(arguments)

	// Auto Scaling Tools
	case toolName == "create-launch-template":
		return m.mockCreateLaunchTemplate(arguments)
	case toolName == "create-auto-scaling-group":
		return m.mockCreateAutoScalingGroup(arguments)
	case toolName == "list-auto-scaling-groups":
		return m.mockListAutoScalingGroups(arguments)
	case toolName == "list-launch-templates":
		return m.mockListLaunchTemplates(arguments)

	// State Tools
	case toolName == "export-infrastructure-state":
		return m.mockExportInfrastructureState(arguments)
	case toolName == "analyze-infrastructure-state":
		return m.mockAnalyzeInfrastructureState(arguments)
	case toolName == "visualize-dependency-graph":
		return m.mockVisualizeDependencyGraph(arguments)
	case toolName == "detect-infrastructure-conflicts":
		return m.mockDetectInfrastructureConflicts(arguments)
	case toolName == "plan-infrastructure-deployment":
		return m.mockPlanInfrastructureDeployment(arguments)
	case toolName == "add-resource-to-state":
		return m.mockAddResourceToState(arguments)
	case toolName == "save-state":
		return m.mockSaveState(arguments)

	// Utility Tools
	case toolName == "get-availability-zones":
		return m.mockGetAvailabilityZones(arguments)

	default:
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Mock tool '%s' not implemented", toolName),
				},
			},
		}, nil
	}
}

// VPC Mock Responses
func (m *MockMCPServer) mockCreateVPC(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	name, _ := arguments["name"].(string)
	cidrBlock, _ := arguments["cidrBlock"].(string)

	if name == "" {
		name = "test-vpc"
	}
	if cidrBlock == "" {
		cidrBlock = "10.0.0.0/16"
	}

	vpcId := m.generateMockResourceId("vpc")

	response := map[string]interface{}{
		"vpcId":     vpcId,
		"name":      name,
		"cidrBlock": cidrBlock,
		"state":     "available",
		"isDefault": false,
		"resource": map[string]interface{}{
			"id":   vpcId,
			"type": "vpc",
			"name": name,
		},
	}

	// Store resource for later retrieval
	m.resources[vpcId] = &types.AWSResource{
		ID:   vpcId,
		Type: "vpc",
		Details: map[string]interface{}{
			"name":      name,
			"cidrBlock": cidrBlock,
			"state":     "available",
		},
	}

	return m.createSuccessResponse(fmt.Sprintf("VPC '%s' created successfully", name), response)
}

// mockCreateVPCWithAWSClient uses the MockAWSClient for VPC creation
func (m *MockMCPServer) mockCreateVPCWithAWSClient(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	name, _ := arguments["name"].(string)
	cidrBlock, _ := arguments["cidrBlock"].(string)

	if name == "" {
		name = "test-vpc"
	}
	if cidrBlock == "" {
		cidrBlock = "10.0.0.0/16"
	}

	// Use the actual AWS mock client
	params := aws.CreateVPCParams{
		CidrBlock: cidrBlock,
		Name:      name,
		Tags: map[string]string{
			"Name": name,
		},
	}

	ctx := context.Background()
	awsResource, err := m.awsClient.CreateVPC(ctx, params)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Failed to create VPC: %v", err),
				},
			},
		}, err
	}

	response := map[string]interface{}{
		"vpcId":     awsResource.ID,
		"name":      name,
		"cidrBlock": cidrBlock,
		"state":     awsResource.State,
		"isDefault": false,
		"resource": map[string]interface{}{
			"id":     awsResource.ID,
			"type":   awsResource.Type,
			"name":   name,
			"region": awsResource.Region,
		},
	}

	return m.createSuccessResponse(fmt.Sprintf("VPC '%s' created successfully", name), response)
}

func (m *MockMCPServer) mockListVPCs(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	vpcs := []map[string]interface{}{
		{
			"vpcId":     "vpc-default123",
			"name":      "default-vpc",
			"cidrBlock": "172.31.0.0/16",
			"state":     "available",
			"isDefault": true,
		},
		{
			"vpcId":     "vpc-prod123",
			"name":      "production-vpc",
			"cidrBlock": "10.0.0.0/16",
			"state":     "available",
			"isDefault": false,
		},
	}

	response := map[string]interface{}{
		"vpcs":  vpcs,
		"count": len(vpcs),
	}

	return m.createSuccessResponse(fmt.Sprintf("Found %d VPCs", len(vpcs)), response)
}

// mockListVPCsWithAWSClient uses the MockAWSClient for VPC listing
func (m *MockMCPServer) mockListVPCsWithAWSClient(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	ctx := context.Background()
	awsVPCs, err := m.awsClient.DescribeVPCs(ctx)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Failed to list VPCs: %v", err),
				},
			},
		}, err
	}

	vpcs := make([]map[string]interface{}, 0, len(awsVPCs))
	for _, vpc := range awsVPCs {
		vpcs = append(vpcs, map[string]interface{}{
			"vpcId":     vpc.ID,
			"name":      vpc.Details["name"],
			"cidrBlock": vpc.Details["cidrBlock"],
			"state":     vpc.State,
			"isDefault": false, // Mock doesn't track default VPCs
		})
	}

	response := map[string]interface{}{
		"vpcs":  vpcs,
		"count": len(vpcs),
	}

	return m.createSuccessResponse(fmt.Sprintf("Found %d VPCs", len(vpcs)), response)
}

func (m *MockMCPServer) mockGetDefaultVPC(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	response := map[string]interface{}{
		"value":     "vpc-default123",
		"vpcId":     "vpc-default123",
		"name":      "default-vpc",
		"cidrBlock": "172.31.0.0/16",
		"state":     "available",
		"source":    "aws_api_call",
	}

	return m.createSuccessResponse("Found default VPC", response)
}

// Subnet Mock Responses
func (m *MockMCPServer) mockCreateSubnet(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	name, _ := arguments["name"].(string)
	vpcId, _ := arguments["vpcId"].(string)
	cidrBlock, _ := arguments["cidrBlock"].(string)
	availabilityZone, _ := arguments["availabilityZone"].(string)

	if name == "" {
		name = "test-subnet"
	}
	if vpcId == "" {
		vpcId = "vpc-default123"
	}
	if cidrBlock == "" {
		cidrBlock = "10.0.1.0/24"
	}
	if availabilityZone == "" {
		availabilityZone = "us-west-2a"
	}

	subnetId := m.generateMockResourceId("subnet")

	response := map[string]interface{}{
		"subnetId":         subnetId,
		"name":             name,
		"vpcId":            vpcId,
		"cidrBlock":        cidrBlock,
		"availabilityZone": availabilityZone,
		"state":            "available",
		"resource": map[string]interface{}{
			"id":   subnetId,
			"type": "subnet",
			"name": name,
		},
	}

	return m.createSuccessResponse(fmt.Sprintf("Subnet '%s' created successfully", name), response)
}

// mockCreateSubnetWithAWSClient uses the MockAWSClient for subnet creation
func (m *MockMCPServer) mockCreateSubnetWithAWSClient(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	name, _ := arguments["name"].(string)
	vpcId, _ := arguments["vpcId"].(string)
	cidrBlock, _ := arguments["cidrBlock"].(string)
	availabilityZone, _ := arguments["availabilityZone"].(string)

	if name == "" {
		name = "test-subnet"
	}
	if vpcId == "" {
		vpcId = "vpc-default123"
	}
	if cidrBlock == "" {
		cidrBlock = "10.0.1.0/24"
	}
	if availabilityZone == "" {
		availabilityZone = "us-west-2a"
	}

	// Use the actual AWS mock client
	params := aws.CreateSubnetParams{
		VpcID:            vpcId,
		CidrBlock:        cidrBlock,
		AvailabilityZone: availabilityZone,
		Name:             name,
	}

	ctx := context.Background()
	awsResource, err := m.awsClient.CreateSubnet(ctx, params)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Failed to create subnet: %v", err),
				},
			},
		}, err
	}

	response := map[string]interface{}{
		"subnetId":         awsResource.ID,
		"name":             name,
		"vpcId":            vpcId,
		"cidrBlock":        cidrBlock,
		"availabilityZone": availabilityZone,
		"state":            awsResource.State,
		"resource": map[string]interface{}{
			"id":     awsResource.ID,
			"type":   awsResource.Type,
			"name":   name,
			"region": awsResource.Region,
		},
	}

	return m.createSuccessResponse(fmt.Sprintf("Subnet '%s' created successfully", name), response)
}

func (m *MockMCPServer) mockSelectSubnetsForALB(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	scheme, _ := arguments["scheme"].(string)
	vpcId, _ := arguments["vpcId"].(string)

	if vpcId == "" {
		vpcId = "vpc-default123"
	}

	subnetIds := []string{"subnet-12345", "subnet-67890"}

	response := map[string]interface{}{
		"subnetIds": subnetIds,
		"vpcId":     vpcId,
		"scheme":    scheme,
		"count":     len(subnetIds),
	}

	responseText, _ := json.Marshal(response)
	return &mcp.CallToolResult{
		IsError: false,
		Content: []mcp.Content{
			&mcp.TextContent{
				Type: "text",
				Text: string(responseText),
			},
		},
	}, nil
}

// EC2 Mock Responses
func (m *MockMCPServer) mockGetLatestAMI(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	response := map[string]interface{}{
		"value":       "ami-latest123",
		"imageId":     "ami-latest123",
		"name":        "amzn2-ami-hvm-2.0.20231101.0-x86_64-gp2",
		"description": "Amazon Linux 2 AMI",
		"source":      "aws_api_call",
	}

	return m.createSuccessResponse("Found latest AMI", response)
}

func (m *MockMCPServer) mockCreateEC2Instance(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	name, _ := arguments["name"].(string)
	imageId, _ := arguments["imageId"].(string)
	instanceType, _ := arguments["instanceType"].(string)

	if name == "" {
		name = "test-instance"
	}
	if imageId == "" {
		imageId = "ami-latest123"
	}
	if instanceType == "" {
		instanceType = "t3.medium"
	}

	instanceId := m.generateMockResourceId("i")

	response := map[string]interface{}{
		"instanceId":   instanceId,
		"name":         name,
		"imageId":      imageId,
		"instanceType": instanceType,
		"state":        "running",
		"resource": map[string]interface{}{
			"id":   instanceId,
			"type": "ec2_instance",
			"name": name,
		},
	}

	return m.createSuccessResponse(fmt.Sprintf("EC2 instance '%s' created successfully", name), response)
}

// mockCreateEC2InstanceWithAWSClient uses the MockAWSClient for EC2 instance creation
func (m *MockMCPServer) mockCreateEC2InstanceWithAWSClient(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	name, _ := arguments["name"].(string)
	imageId, _ := arguments["imageId"].(string)
	instanceType, _ := arguments["instanceType"].(string)
	keyName, _ := arguments["keyName"].(string)
	securityGroupId, _ := arguments["securityGroupId"].(string)
	subnetId, _ := arguments["subnetId"].(string)

	if name == "" {
		name = "test-instance"
	}
	if imageId == "" {
		imageId = "ami-12345678" // Valid mock AMI ID format
	}
	if instanceType == "" {
		instanceType = "t3.micro"
	}

	// Use the actual AWS mock client
	params := aws.CreateInstanceParams{
		ImageID:         imageId,
		InstanceType:    instanceType,
		KeyName:         keyName,
		SecurityGroupID: securityGroupId,
		SubnetID:        subnetId,
	}

	ctx := context.Background()
	awsResource, err := m.awsClient.CreateEC2Instance(ctx, params)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Failed to create EC2 instance: %v", err),
				},
			},
		}, err
	}

	response := map[string]interface{}{
		"instanceId":   awsResource.ID,
		"name":         name,
		"imageId":      imageId,
		"instanceType": instanceType,
		"state":        awsResource.State,
		"resource": map[string]interface{}{
			"id":     awsResource.ID,
			"type":   awsResource.Type,
			"name":   name,
			"region": awsResource.Region,
		},
	}

	return m.createSuccessResponse(fmt.Sprintf("EC2 instance '%s' created successfully", name), response)
}

// Load Balancer Mock Responses
func (m *MockMCPServer) mockCreateLoadBalancer(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	name, _ := arguments["name"].(string)
	scheme, _ := arguments["scheme"].(string)
	lbType, _ := arguments["type"].(string)

	if name == "" {
		name = "test-alb"
	}
	if scheme == "" {
		scheme = "internet-facing"
	}
	if lbType == "" {
		lbType = "application"
	}

	loadBalancerArn := fmt.Sprintf("arn:aws:elasticloadbalancing:us-west-2:123456789012:loadbalancer/app/%s/%s",
		name, m.generateMockResourceId(""))

	response := map[string]interface{}{
		"name":            name,
		"scheme":          scheme,
		"type":            lbType,
		"loadBalancerId":  loadBalancerArn,
		"loadBalancerArn": loadBalancerArn,
		"arn":             loadBalancerArn,
		"resource": map[string]interface{}{
			"id":   loadBalancerArn,
			"type": "load_balancer",
			"name": name,
		},
	}

	return m.createSuccessResponse(fmt.Sprintf("Load balancer '%s' created successfully", name), response)
}

func (m *MockMCPServer) mockCreateTargetGroup(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	name, _ := arguments["name"].(string)
	vpcId, _ := arguments["vpcId"].(string)
	protocol, _ := arguments["protocol"].(string)
	port, _ := arguments["port"].(float64)

	if name == "" {
		name = "test-tg"
	}
	if vpcId == "" {
		vpcId = "vpc-default123"
	}
	if protocol == "" {
		protocol = "HTTP"
	}
	if port == 0 {
		port = 80
	}

	targetGroupArn := fmt.Sprintf("arn:aws:elasticloadbalancing:us-west-2:123456789012:targetgroup/%s/%s",
		name, m.generateMockResourceId(""))

	response := map[string]interface{}{
		"name":           name,
		"vpcId":          vpcId,
		"protocol":       protocol,
		"port":           int(port),
		"targetGroupId":  targetGroupArn,
		"targetGroupArn": targetGroupArn,
		"arn":            targetGroupArn,
		"resource": map[string]interface{}{
			"id":   targetGroupArn,
			"type": "target_group",
			"name": name,
		},
	}

	return m.createSuccessResponse(fmt.Sprintf("Target group '%s' created successfully", name), response)
}

// State Mock Responses
func (m *MockMCPServer) mockExportInfrastructureState(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	return m.stateManager.ExportState(arguments)
}

// Utility Mock Responses
func (m *MockMCPServer) mockGetAvailabilityZones(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	zones := []string{"ap-southeast-1a", "ap-southeast-1b", "ap-southeast-1c"}

	response := map[string]interface{}{
		"zones":            zones,
		"count":            len(zones),
		"allZones":         zones,
		"allCount":         len(zones),
		"availabilityZone": zones[0], // First AZ for {{step-id.availabilityZone}} resolution
		"value":            zones[0], // For {{step-id.resourceId}} resolution
		"zone_names":       zones,    // Alternative access pattern
	}

	return m.createSuccessResponse(fmt.Sprintf("Retrieved %d availability zones", len(zones)), response)
}

// Utility methods
func (m *MockMCPServer) generateMockResourceId(prefix string) string {
	if prefix == "" {
		return fmt.Sprintf("%08d", len(m.resources)+1)
	}
	return fmt.Sprintf("%s-%08d", prefix, len(m.resources)+1)
}

func (m *MockMCPServer) createSuccessResponse(message string, data interface{}) (*mcp.CallToolResult, error) {
	responseData, _ := json.Marshal(data)

	return &mcp.CallToolResult{
		IsError: false,
		Content: []mcp.Content{
			&mcp.TextContent{
				Type: "text",
				Text: string(responseData),
			},
		},
	}, nil
}

// Configuration methods
func (m *MockMCPServer) SimulateError(toolName string, err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.simulateErrors[toolName] = err
}

func (m *MockMCPServer) SetCustomResponse(toolName string, response *mcp.CallToolResult) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.customResponses[toolName] = response
}

func (m *MockMCPServer) ClearError(toolName string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	delete(m.simulateErrors, toolName)
}

func (m *MockMCPServer) ClearAllErrors() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.simulateErrors = make(map[string]error)
}

// Additional mock methods for missing implementations
func (m *MockMCPServer) mockListSubnets(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	subnets := []map[string]interface{}{
		{
			"subnetId":         "subnet-0def123abc456789",
			"name":             "default-subnet",
			"vpcId":            "vpc-default123",
			"cidrBlock":        "172.31.1.0/24",
			"availabilityZone": "us-west-2a",
			"state":            "available",
		},
		{
			"subnetId":         "subnet-12345",
			"name":             "public-subnet-1",
			"vpcId":            "vpc-prod123",
			"cidrBlock":        "10.0.1.0/24",
			"availabilityZone": "us-west-2a",
			"state":            "available",
		},
	}

	// Extract subnet IDs for dependency resolution
	subnetIDs := []string{"subnet-0def123abc456789", "subnet-0abc12345678"}

	response := map[string]interface{}{
		"subnets":    subnets,      // Subnet details
		"subnetId":   subnetIDs[0], // First subnet for dependency resolution
		"value":      subnetIDs[0], // For {{step-id.resourceId}} resolution
		"subnet_ids": subnetIDs,    // Full list for comprehensive access
	}

	return m.createSuccessResponse(fmt.Sprintf("Found %d subnets", len(subnets)), response)
}

func (m *MockMCPServer) mockListEC2Instances(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	instances := []map[string]interface{}{
		{
			"instanceId":   "i-sample123",
			"name":         "sample-instance",
			"imageId":      "ami-latest123",
			"instanceType": "t3.medium",
			"state":        "running",
		},
	}

	response := map[string]interface{}{
		"instances": instances,
		"count":     len(instances),
	}

	return m.createSuccessResponse(fmt.Sprintf("Found %d instances", len(instances)), response)
}

func (m *MockMCPServer) mockCreateSecurityGroup(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	name, _ := arguments["name"].(string)
	vpcId, _ := arguments["vpcId"].(string)
	description, _ := arguments["description"].(string)

	if name == "" {
		name = "test-sg"
	}
	if vpcId == "" {
		vpcId = "vpc-default123"
	}
	if description == "" {
		description = "Test security group"
	}

	groupId := m.generateMockResourceId("sg")

	response := map[string]interface{}{
		"groupId":         groupId,
		"securityGroupId": groupId,
		"name":            name,
		"vpcId":           vpcId,
		"description":     description,
		"resource": map[string]interface{}{
			"id":   groupId,
			"type": "security_group",
			"name": name,
		},
	}

	return m.createSuccessResponse(fmt.Sprintf("Security group '%s' created successfully", name), response)
}

func (m *MockMCPServer) mockCreateListener(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	loadBalancerArn, _ := arguments["loadBalancerArn"].(string)
	targetGroupArn, _ := arguments["targetGroupArn"].(string)
	protocol, _ := arguments["protocol"].(string)
	port, _ := arguments["port"].(float64)

	if protocol == "" {
		protocol = "HTTP"
	}
	if port == 0 {
		port = 80
	}

	// Validate ARN formats - be lenient for testing, allow mock IDs
	if loadBalancerArn != "" && !strings.HasPrefix(loadBalancerArn, "arn:aws:elasticloadbalancing:") && !strings.HasPrefix(loadBalancerArn, "mock-") {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("loadBalancerArn must be in ARN format, received: '%s'", loadBalancerArn),
				},
			},
		}, nil
	}

	if targetGroupArn != "" && !strings.HasPrefix(targetGroupArn, "arn:aws:elasticloadbalancing:") && !strings.HasPrefix(targetGroupArn, "mock-") {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("targetGroupArn must be in ARN format, received: '%s'", targetGroupArn),
				},
			},
		}, nil
	}

	listenerId := m.generateMockResourceId("listener")

	response := map[string]interface{}{
		"loadBalancerArn": loadBalancerArn,
		"targetGroupArn":  targetGroupArn,
		"protocol":        protocol,
		"port":            int(port),
		"listenerId":      listenerId,
		"resource": map[string]interface{}{
			"id":   listenerId,
			"type": "listener",
		},
	}

	return m.createSuccessResponse("Listener created successfully", response)
}

// ==== Missing EC2 Mock Methods ====

func (m *MockMCPServer) mockStartEC2Instance(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	instanceId, _ := arguments["instanceId"].(string)

	if instanceId == "" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{
					Type: "text",
					Text: "instanceId is required",
				},
			},
		}, nil
	}

	response := map[string]interface{}{
		"instanceId":    instanceId,
		"previousState": "stopped",
		"currentState":  "running",
		"resource": map[string]interface{}{
			"id":   instanceId,
			"type": "ec2_instance",
		},
	}

	return m.createSuccessResponse(fmt.Sprintf("Instance %s started successfully", instanceId), response)
}

func (m *MockMCPServer) mockStopEC2Instance(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	instanceId, _ := arguments["instanceId"].(string)

	if instanceId == "" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{
					Type: "text",
					Text: "instanceId is required",
				},
			},
		}, nil
	}

	response := map[string]interface{}{
		"instanceId":    instanceId,
		"previousState": "running",
		"currentState":  "stopped",
		"resource": map[string]interface{}{
			"id":   instanceId,
			"type": "ec2_instance",
		},
	}

	return m.createSuccessResponse(fmt.Sprintf("Instance %s stopped successfully", instanceId), response)
}

func (m *MockMCPServer) mockTerminateEC2Instance(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	instanceId, _ := arguments["instanceId"].(string)

	if instanceId == "" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{
					Type: "text",
					Text: "instanceId is required",
				},
			},
		}, nil
	}

	response := map[string]interface{}{
		"instanceId":    instanceId,
		"previousState": "running",
		"currentState":  "terminated",
		"resource": map[string]interface{}{
			"id":   instanceId,
			"type": "ec2_instance",
		},
	}

	return m.createSuccessResponse(fmt.Sprintf("Instance %s terminated successfully", instanceId), response)
}

func (m *MockMCPServer) mockCreateAMIFromInstance(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	instanceId, _ := arguments["instanceId"].(string)
	name, _ := arguments["name"].(string)
	description, _ := arguments["description"].(string)

	if instanceId == "" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{
					Type: "text",
					Text: "instanceId is required",
				},
			},
		}, nil
	}

	if name == "" {
		name = "custom-ami-" + m.generateMockResourceId("ami")
	}

	amiId := m.generateMockResourceId("ami")

	response := map[string]interface{}{
		"imageId":          amiId,
		"name":             name,
		"description":      description,
		"sourceInstanceId": instanceId,
		"state":            "available",
		"resource": map[string]interface{}{
			"id":   amiId,
			"type": "ami",
			"name": name,
		},
	}

	return m.createSuccessResponse(fmt.Sprintf("AMI %s created from instance %s", amiId, instanceId), response)
}

func (m *MockMCPServer) mockListAMIs(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	amis := []map[string]interface{}{
		{
			"imageId":     "ami-latest123",
			"name":        "amzn2-ami-hvm-2.0.20231101.0-x86_64-gp2",
			"description": "Amazon Linux 2 AMI",
			"state":       "available",
			"public":      true,
		},
		{
			"imageId":     "ami-0abc123def456789",
			"name":        "ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-20231020",
			"description": "Canonical, Ubuntu, 22.04 LTS, amd64 jammy image",
			"state":       "available",
			"public":      true,
		},
	}

	response := map[string]interface{}{
		"images": amis,
		"count":  len(amis),
	}

	return m.createSuccessResponse(fmt.Sprintf("Found %d AMIs", len(amis)), response)
}

func (m *MockMCPServer) mockGetLatestAmazonLinuxAMI(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	response := map[string]interface{}{
		"amiId":        "ami-latest123",
		"imageId":      "ami-latest123",
		"name":         "amzn2-ami-hvm-2.0.20231101.0-x86_64-gp2",
		"description":  "Amazon Linux 2 AMI (HVM) - Kernel 5.10, SSD Volume Type",
		"osType":       "Linux",
		"platform":     "amazon-linux",
		"architecture": "x86_64",
		"state":        "available",
		"resource": map[string]interface{}{
			"id":   "ami-latest123",
			"type": "ami",
		},
	}

	return m.createSuccessResponse("Found latest Amazon Linux 2 AMI: ami-latest123", response)
}

func (m *MockMCPServer) mockGetLatestUbuntuAMI(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	architecture, _ := arguments["architecture"].(string)
	if architecture == "" {
		architecture = "x86_64"
	}

	response := map[string]interface{}{
		"amiId":        "ami-0abc123def456789",
		"imageId":      "ami-0abc123def456789",
		"name":         "ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-20231020",
		"description":  "Canonical, Ubuntu, 22.04 LTS, amd64 jammy image build on 2023-10-20",
		"osType":       "Linux",
		"platform":     "ubuntu",
		"architecture": architecture,
		"state":        "available",
		"resource": map[string]interface{}{
			"id":   "ami-0abc123def456789",
			"type": "ami",
		},
	}

	return m.createSuccessResponse("Found latest Ubuntu LTS AMI: ami-0abc123def456789", response)
}

// ==== Missing Networking Mock Methods ====

func (m *MockMCPServer) mockCreatePrivateSubnet(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	vpcId, _ := arguments["vpcId"].(string)
	cidrBlock, _ := arguments["cidrBlock"].(string)
	availabilityZone, _ := arguments["availabilityZone"].(string)
	name, _ := arguments["name"].(string)

	if vpcId == "" || cidrBlock == "" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{
					Type: "text",
					Text: "vpcId and cidrBlock are required",
				},
			},
		}, nil
	}

	if name == "" {
		name = "private-subnet"
	}
	if availabilityZone == "" {
		availabilityZone = "us-west-2a"
	}

	subnetId := m.generateMockResourceId("subnet")

	response := map[string]interface{}{
		"subnetId":            subnetId,
		"vpcId":               vpcId,
		"cidrBlock":           cidrBlock,
		"availabilityZone":    availabilityZone,
		"name":                name,
		"mapPublicIpOnLaunch": false,
		"state":               "available",
		"subnetType":          "private",
		"resource": map[string]interface{}{
			"id":   subnetId,
			"type": "subnet",
			"name": name,
		},
	}

	return m.createSuccessResponse(fmt.Sprintf("Private subnet '%s' created successfully", name), response)
}

func (m *MockMCPServer) mockCreatePublicSubnet(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	vpcId, _ := arguments["vpcId"].(string)
	cidrBlock, _ := arguments["cidrBlock"].(string)
	availabilityZone, _ := arguments["availabilityZone"].(string)
	name, _ := arguments["name"].(string)

	if vpcId == "" || cidrBlock == "" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{
					Type: "text",
					Text: "vpcId and cidrBlock are required",
				},
			},
		}, nil
	}

	if name == "" {
		name = "public-subnet"
	}
	if availabilityZone == "" {
		availabilityZone = "us-west-2a"
	}

	subnetId := m.generateMockResourceId("subnet")

	response := map[string]interface{}{
		"subnetId":            subnetId,
		"vpcId":               vpcId,
		"cidrBlock":           cidrBlock,
		"availabilityZone":    availabilityZone,
		"name":                name,
		"mapPublicIpOnLaunch": true,
		"state":               "available",
		"subnetType":          "public",
		"resource": map[string]interface{}{
			"id":   subnetId,
			"type": "subnet",
			"name": name,
		},
	}

	return m.createSuccessResponse(fmt.Sprintf("Public subnet '%s' created successfully", name), response)
}

func (m *MockMCPServer) mockCreateInternetGateway(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	name, _ := arguments["name"].(string)
	vpcId, _ := arguments["vpcId"].(string)

	if name == "" {
		name = "test-igw"
	}

	igwId := m.generateMockResourceId("igw")

	response := map[string]interface{}{
		"internetGatewayId": igwId,
		"name":              name,
		"state":             "available",
		"resource": map[string]interface{}{
			"id":   igwId,
			"type": "internet_gateway",
			"name": name,
		},
	}

	if vpcId != "" {
		response["vpcId"] = vpcId
		response["attachmentState"] = "attached"
	}

	return m.createSuccessResponse(fmt.Sprintf("Internet gateway '%s' created successfully", name), response)
}

func (m *MockMCPServer) mockCreateNATGateway(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	subnetId, _ := arguments["subnetId"].(string)
	name, _ := arguments["name"].(string)

	if subnetId == "" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{
					Type: "text",
					Text: "subnetId is required",
				},
			},
		}, nil
	}

	if name == "" {
		name = "test-nat-gw"
	}

	natGwId := m.generateMockResourceId("nat")
	eipId := m.generateMockResourceId("eip")

	response := map[string]interface{}{
		"natGatewayId": natGwId,
		"subnetId":     subnetId,
		"name":         name,
		"state":        "available",
		"publicIp":     "1.2.3.4",
		"allocationId": eipId,
		"resource": map[string]interface{}{
			"id":   natGwId,
			"type": "nat_gateway",
			"name": name,
		},
	}

	return m.createSuccessResponse(fmt.Sprintf("NAT gateway '%s' created successfully", name), response)
}

func (m *MockMCPServer) mockCreatePublicRouteTable(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	vpcId, _ := arguments["vpcId"].(string)
	name, _ := arguments["name"].(string)

	if vpcId == "" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{
					Type: "text",
					Text: "vpcId is required",
				},
			},
		}, nil
	}

	if name == "" {
		name = "public-route-table"
	}

	routeTableId := m.generateMockResourceId("rtb")

	response := map[string]interface{}{
		"routeTableId": routeTableId,
		"vpcId":        vpcId,
		"name":         name,
		"routeType":    "public",
		"resource": map[string]interface{}{
			"id":   routeTableId,
			"type": "route_table",
			"name": name,
		},
	}

	return m.createSuccessResponse(fmt.Sprintf("Public route table '%s' created successfully", name), response)
}

func (m *MockMCPServer) mockCreatePrivateRouteTable(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	vpcId, _ := arguments["vpcId"].(string)
	name, _ := arguments["name"].(string)

	if vpcId == "" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{
					Type: "text",
					Text: "vpcId is required",
				},
			},
		}, nil
	}

	if name == "" {
		name = "private-route-table"
	}

	routeTableId := m.generateMockResourceId("rtb")

	response := map[string]interface{}{
		"routeTableId": routeTableId,
		"vpcId":        vpcId,
		"name":         name,
		"routeType":    "private",
		"resource": map[string]interface{}{
			"id":   routeTableId,
			"type": "route_table",
			"name": name,
		},
	}

	return m.createSuccessResponse(fmt.Sprintf("Private route table '%s' created successfully", name), response)
}

func (m *MockMCPServer) mockAssociateRouteTable(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	routeTableId, _ := arguments["routeTableId"].(string)
	subnetId, _ := arguments["subnetId"].(string)

	if routeTableId == "" || subnetId == "" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{
					Type: "text",
					Text: "routeTableId and subnetId are required",
				},
			},
		}, nil
	}

	associationId := m.generateMockResourceId("rtbassoc")

	response := map[string]interface{}{
		"associationId": associationId,
		"routeTableId":  routeTableId,
		"subnetId":      subnetId,
		"resource": map[string]interface{}{
			"id":   associationId,
			"type": "route_table_association",
		},
	}

	return m.createSuccessResponse("Route table associated successfully", response)
}

func (m *MockMCPServer) mockAddRoute(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	routeTableId, _ := arguments["routeTableId"].(string)
	destinationCidrBlock, _ := arguments["destinationCidrBlock"].(string)
	gatewayId, _ := arguments["gatewayId"].(string)
	natGatewayId, _ := arguments["natGatewayId"].(string)

	if routeTableId == "" || destinationCidrBlock == "" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{
					Type: "text",
					Text: "routeTableId and destinationCidrBlock are required",
				},
			},
		}, nil
	}

	if gatewayId == "" && natGatewayId == "" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{
					Type: "text",
					Text: "either gatewayId or natGatewayId must be specified",
				},
			},
		}, nil
	}

	response := map[string]interface{}{
		"routeTableId":         routeTableId,
		"destinationCidrBlock": destinationCidrBlock,
		"state":                "active",
	}

	if gatewayId != "" {
		response["gatewayId"] = gatewayId
		response["targetType"] = "gateway"
	}
	if natGatewayId != "" {
		response["natGatewayId"] = natGatewayId
		response["targetType"] = "nat-gateway"
	}

	return m.createSuccessResponse("Route added successfully", response)
}

// ==== Missing Security Group Mock Methods ====

func (m *MockMCPServer) mockListSecurityGroups(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	vpcId, _ := arguments["vpcId"].(string)

	securityGroups := []map[string]interface{}{
		{
			"groupId":     "sg-default123",
			"groupName":   "default",
			"description": "default VPC security group",
			"vpcId":       "vpc-default123",
			"inboundRules": []map[string]interface{}{
				{
					"protocol": "-1",
					"fromPort": -1,
					"toPort":   -1,
					"source":   "sg-default123",
				},
			},
			"outboundRules": []map[string]interface{}{
				{
					"protocol":    "-1",
					"fromPort":    -1,
					"toPort":      -1,
					"destination": "0.0.0.0/0",
				},
			},
		},
		{
			"groupId":     "sg-web123",
			"groupName":   "web-tier",
			"description": "Web tier security group",
			"vpcId":       "vpc-prod123",
			"inboundRules": []map[string]interface{}{
				{
					"protocol": "tcp",
					"fromPort": 80,
					"toPort":   80,
					"source":   "0.0.0.0/0",
				},
				{
					"protocol": "tcp",
					"fromPort": 443,
					"toPort":   443,
					"source":   "0.0.0.0/0",
				},
			},
		},
	}

	// Filter by VPC if specified
	if vpcId != "" {
		filteredGroups := []map[string]interface{}{}
		for _, sg := range securityGroups {
			if sg["vpcId"].(string) == vpcId {
				filteredGroups = append(filteredGroups, sg)
			}
		}
		securityGroups = filteredGroups
	}

	response := map[string]interface{}{
		"securityGroups": securityGroups,
		"count":          len(securityGroups),
	}

	return m.createSuccessResponse(fmt.Sprintf("Found %d security groups", len(securityGroups)), response)
}

func (m *MockMCPServer) mockAddSecurityGroupIngressRule(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	groupId, _ := arguments["groupId"].(string)
	protocol, _ := arguments["protocol"].(string)
	fromPort, _ := arguments["fromPort"].(float64)
	toPort, _ := arguments["toPort"].(float64)
	cidrBlock, _ := arguments["cidrBlock"].(string)
	sourceGroupId, _ := arguments["sourceGroupId"].(string)

	if groupId == "" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{
					Type: "text",
					Text: "groupId is required",
				},
			},
		}, nil
	}

	if protocol == "" {
		protocol = "tcp"
	}

	rule := map[string]interface{}{
		"protocol": protocol,
		"fromPort": int(fromPort),
		"toPort":   int(toPort),
	}

	if cidrBlock != "" {
		rule["source"] = cidrBlock
	} else if sourceGroupId != "" {
		rule["source"] = sourceGroupId
	} else {
		rule["source"] = "0.0.0.0/0"
	}

	response := map[string]interface{}{
		"groupId":   groupId,
		"direction": "ingress",
		"rule":      rule,
	}

	return m.createSuccessResponse("Ingress rule added successfully", response)
}

func (m *MockMCPServer) mockAddSecurityGroupEgressRule(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	groupId, _ := arguments["groupId"].(string)
	protocol, _ := arguments["protocol"].(string)
	fromPort, _ := arguments["fromPort"].(float64)
	toPort, _ := arguments["toPort"].(float64)
	cidrBlock, _ := arguments["cidrBlock"].(string)
	destinationGroupId, _ := arguments["destinationGroupId"].(string)

	if groupId == "" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{
					Type: "text",
					Text: "groupId is required",
				},
			},
		}, nil
	}

	if protocol == "" {
		protocol = "tcp"
	}

	rule := map[string]interface{}{
		"protocol": protocol,
		"fromPort": int(fromPort),
		"toPort":   int(toPort),
	}

	if cidrBlock != "" {
		rule["destination"] = cidrBlock
	} else if destinationGroupId != "" {
		rule["destination"] = destinationGroupId
	} else {
		rule["destination"] = "0.0.0.0/0"
	}

	response := map[string]interface{}{
		"groupId":   groupId,
		"direction": "egress",
		"rule":      rule,
	}

	return m.createSuccessResponse("Egress rule added successfully", response)
}

func (m *MockMCPServer) mockDeleteSecurityGroup(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	groupId, _ := arguments["groupId"].(string)

	if groupId == "" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{
					Type: "text",
					Text: "groupId is required",
				},
			},
		}, nil
	}

	// Simulate validation - cannot delete default group
	if strings.Contains(groupId, "default") {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{
					Type: "text",
					Text: "Cannot delete default security group",
				},
			},
		}, nil
	}

	response := map[string]interface{}{
		"groupId": groupId,
		"status":  "deleted",
	}

	return m.createSuccessResponse(fmt.Sprintf("Security group %s deleted successfully", groupId), response)
}

// ==== Missing Auto Scaling Mock Methods ====

func (m *MockMCPServer) mockCreateLaunchTemplate(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	name, _ := arguments["name"].(string)
	imageId, _ := arguments["imageId"].(string)
	instanceType, _ := arguments["instanceType"].(string)
	securityGroupIds, _ := arguments["securityGroupIds"].([]interface{})

	if name == "" {
		name = "test-launch-template"
	}
	if imageId == "" {
		imageId = "ami-latest123"
	}
	if instanceType == "" {
		instanceType = "t3.micro"
	}

	templateId := m.generateMockResourceId("lt")

	// Convert securityGroupIds to string slice
	var sgIds []string
	for _, sgId := range securityGroupIds {
		if id, ok := sgId.(string); ok {
			sgIds = append(sgIds, id)
		}
	}

	response := map[string]interface{}{
		"launchTemplateId":    templateId,
		"name":                name,
		"imageId":             imageId,
		"instanceType":        instanceType,
		"securityGroupIds":    sgIds,
		"latestVersionNumber": 1,
		"resource": map[string]interface{}{
			"id":   templateId,
			"type": "launch_template",
			"name": name,
		},
	}

	return m.createSuccessResponse(fmt.Sprintf("Launch template '%s' created successfully", name), response)
}

func (m *MockMCPServer) mockCreateAutoScalingGroup(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	name, _ := arguments["name"].(string)
	launchTemplateId, _ := arguments["launchTemplateId"].(string)
	subnetIds, _ := arguments["subnetIds"].([]interface{})
	minSize, _ := arguments["minSize"].(float64)
	maxSize, _ := arguments["maxSize"].(float64)
	desiredCapacity, _ := arguments["desiredCapacity"].(float64)

	if name == "" {
		name = "test-asg"
	}
	// Be lenient for testing - generate a mock template ID if not provided
	if launchTemplateId == "" {
		launchTemplateId = m.generateMockResourceId("launch-template")
	}

	asgArn := fmt.Sprintf("arn:aws:autoscaling:us-west-2:123456789012:autoScalingGroup:%s:autoScalingGroupName/%s",
		m.generateMockResourceId("asg"), name)

	// Convert subnetIds to string slice
	var subnets []string
	for _, subnetId := range subnetIds {
		if id, ok := subnetId.(string); ok {
			subnets = append(subnets, id)
		}
	}

	response := map[string]interface{}{
		"autoScalingGroupName": name,
		"autoScalingGroupArn":  asgArn,
		"launchTemplateId":     launchTemplateId,
		"subnetIds":            subnets,
		"minSize":              int(minSize),
		"maxSize":              int(maxSize),
		"desiredCapacity":      int(desiredCapacity),
		"resource": map[string]interface{}{
			"id":   asgArn,
			"type": "auto_scaling_group",
			"name": name,
		},
	}

	return m.createSuccessResponse(fmt.Sprintf("Auto scaling group '%s' created successfully", name), response)
}

func (m *MockMCPServer) mockListAutoScalingGroups(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	groups := []map[string]interface{}{
		{
			"autoScalingGroupName": "web-tier-asg",
			"autoScalingGroupArn":  "arn:aws:autoscaling:us-west-2:123456789012:autoScalingGroup:uuid:autoScalingGroupName/web-tier-asg",
			"launchTemplateId":     "lt-sample123",
			"minSize":              1,
			"maxSize":              3,
			"desiredCapacity":      2,
			"availabilityZones":    []string{"us-west-2a", "us-west-2b"},
		},
	}

	response := map[string]interface{}{
		"autoScalingGroups": groups,
		"count":             len(groups),
	}

	return m.createSuccessResponse(fmt.Sprintf("Found %d auto scaling groups", len(groups)), response)
}

func (m *MockMCPServer) mockListLaunchTemplates(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	templates := []map[string]interface{}{
		{
			"launchTemplateId":     "lt-sample123",
			"name":                 "web-server-template",
			"latestVersionNumber":  1,
			"defaultVersionNumber": 1,
			"imageId":              "ami-latest123",
			"instanceType":         "t3.medium",
		},
	}

	response := map[string]interface{}{
		"launchTemplates": templates,
		"count":           len(templates),
	}

	return m.createSuccessResponse(fmt.Sprintf("Found %d launch templates", len(templates)), response)
}

// ==== Missing Load Balancer Mock Methods ====

func (m *MockMCPServer) mockListLoadBalancers(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	loadBalancers := []map[string]interface{}{
		{
			"loadBalancerArn":  "arn:aws:elasticloadbalancing:us-west-2:123456789012:loadbalancer/app/test-alb/1234567890123456",
			"loadBalancerName": "test-alb",
			"scheme":           "internet-facing",
			"vpcId":            "vpc-prod123",
			"type":             "application",
			"state":            "active",
			"dnsName":          "test-alb-123456789.us-west-2.elb.amazonaws.com",
		},
	}

	response := map[string]interface{}{
		"loadBalancers": loadBalancers,
		"count":         len(loadBalancers),
	}

	return m.createSuccessResponse(fmt.Sprintf("Found %d load balancers", len(loadBalancers)), response)
}

func (m *MockMCPServer) mockListTargetGroups(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	targetGroups := []map[string]interface{}{
		{
			"targetGroupArn":  "arn:aws:elasticloadbalancing:us-west-2:123456789012:targetgroup/web-servers/1234567890123456",
			"targetGroupName": "web-servers",
			"protocol":        "HTTP",
			"port":            80,
			"vpcId":           "vpc-prod123",
			"targetType":      "instance",
			"healthCheckPath": "/health",
		},
	}

	response := map[string]interface{}{
		"targetGroups": targetGroups,
		"count":        len(targetGroups),
	}

	return m.createSuccessResponse(fmt.Sprintf("Found %d target groups", len(targetGroups)), response)
}

func (m *MockMCPServer) mockRegisterTargets(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	targetGroupArn, _ := arguments["targetGroupArn"].(string)
	targets, _ := arguments["targets"].([]interface{})

	if targetGroupArn == "" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{
					Type: "text",
					Text: "targetGroupArn is required",
				},
			},
		}, nil
	}

	var targetList []string
	for _, target := range targets {
		if targetStr, ok := target.(string); ok {
			targetList = append(targetList, targetStr)
		}
	}

	response := map[string]interface{}{
		"targetGroupArn": targetGroupArn,
		"targets":        targetList,
		"status":         "registered",
	}

	return m.createSuccessResponse(fmt.Sprintf("Registered %d targets", len(targetList)), response)
}

func (m *MockMCPServer) mockDeregisterTargets(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	targetGroupArn, _ := arguments["targetGroupArn"].(string)
	targets, _ := arguments["targets"].([]interface{})

	if targetGroupArn == "" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{
					Type: "text",
					Text: "targetGroupArn is required",
				},
			},
		}, nil
	}

	var targetList []string
	for _, target := range targets {
		if targetStr, ok := target.(string); ok {
			targetList = append(targetList, targetStr)
		}
	}

	response := map[string]interface{}{
		"targetGroupArn": targetGroupArn,
		"targets":        targetList,
		"status":         "deregistered",
	}

	return m.createSuccessResponse(fmt.Sprintf("Deregistered %d targets", len(targetList)), response)
}

// ==== Missing RDS Mock Methods ====

func (m *MockMCPServer) mockCreateDBSubnetGroup(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	name, _ := arguments["name"].(string)
	description, _ := arguments["description"].(string)
	subnetIds, _ := arguments["subnetIds"].([]interface{})

	if name == "" {
		name = "test-db-subnet-group"
	}
	if description == "" {
		description = "Test database subnet group"
	}

	// Convert subnetIds to string slice
	var subnets []string
	for _, subnetId := range subnetIds {
		if id, ok := subnetId.(string); ok {
			subnets = append(subnets, id)
		}
	}

	if len(subnets) == 0 {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{
					Type: "text",
					Text: "At least one subnet ID is required",
				},
			},
		}, nil
	}

	response := map[string]interface{}{
		"dbSubnetGroupName":   name,
		"description":         description,
		"subnetIds":           subnets,
		"vpcId":               "vpc-prod123",
		"dbSubnetGroupStatus": "Complete",
		"resource": map[string]interface{}{
			"id":   name,
			"type": "db_subnet_group",
			"name": name,
		},
	}

	return m.createSuccessResponse(fmt.Sprintf("DB subnet group '%s' created successfully", name), response)
}

func (m *MockMCPServer) mockCreateDBInstance(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	dbInstanceIdentifier, _ := arguments["dbInstanceIdentifier"].(string)
	dbInstanceClass, _ := arguments["dbInstanceClass"].(string)
	engine, _ := arguments["engine"].(string)
	masterUsername, _ := arguments["masterUsername"].(string)
	dbSubnetGroupName, _ := arguments["dbSubnetGroupName"].(string)
	allocatedStorage, _ := arguments["allocatedStorage"].(float64)

	if dbInstanceIdentifier == "" {
		dbInstanceIdentifier = "test-db-instance"
	}
	if dbInstanceClass == "" {
		dbInstanceClass = "db.t3.micro"
	}
	if engine == "" {
		engine = "mysql"
	}
	if masterUsername == "" {
		masterUsername = "admin"
	}
	if allocatedStorage == 0 {
		allocatedStorage = 20
	}

	endpoint := fmt.Sprintf("%s.cluster-xyz.us-west-2.rds.amazonaws.com", dbInstanceIdentifier)
	dbArn := fmt.Sprintf("arn:aws:rds:us-west-2:123456789012:db:%s", dbInstanceIdentifier)

	response := map[string]interface{}{
		"dbInstanceIdentifier": dbInstanceIdentifier,
		"dbInstanceClass":      dbInstanceClass,
		"engine":               engine,
		"dbInstanceStatus":     "available",
		"masterUsername":       masterUsername,
		"allocatedStorage":     int(allocatedStorage),
		"dbSubnetGroupName":    dbSubnetGroupName,
		"endpoint":             endpoint,
		"port":                 3306,
		"dbInstanceArn":        dbArn,
		"resource": map[string]interface{}{
			"id":   dbInstanceIdentifier,
			"type": "db_instance",
			"name": dbInstanceIdentifier,
		},
	}

	return m.createSuccessResponse(fmt.Sprintf("DB instance '%s' created successfully", dbInstanceIdentifier), response)
}

func (m *MockMCPServer) mockStartDBInstance(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	dbInstanceIdentifier, _ := arguments["dbInstanceIdentifier"].(string)

	if dbInstanceIdentifier == "" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{
					Type: "text",
					Text: "dbInstanceIdentifier is required",
				},
			},
		}, nil
	}

	response := map[string]interface{}{
		"dbInstanceIdentifier": dbInstanceIdentifier,
		"previousStatus":       "stopped",
		"currentStatus":        "starting",
		"resource": map[string]interface{}{
			"id":   dbInstanceIdentifier,
			"type": "db_instance",
		},
	}

	return m.createSuccessResponse(fmt.Sprintf("DB instance '%s' started successfully", dbInstanceIdentifier), response)
}

func (m *MockMCPServer) mockStopDBInstance(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	dbInstanceIdentifier, _ := arguments["dbInstanceIdentifier"].(string)

	if dbInstanceIdentifier == "" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{
					Type: "text",
					Text: "dbInstanceIdentifier is required",
				},
			},
		}, nil
	}

	response := map[string]interface{}{
		"dbInstanceIdentifier": dbInstanceIdentifier,
		"previousStatus":       "available",
		"currentStatus":        "stopping",
		"resource": map[string]interface{}{
			"id":   dbInstanceIdentifier,
			"type": "db_instance",
		},
	}

	return m.createSuccessResponse(fmt.Sprintf("DB instance '%s' stopped successfully", dbInstanceIdentifier), response)
}

func (m *MockMCPServer) mockDeleteDBInstance(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	dbInstanceIdentifier, _ := arguments["dbInstanceIdentifier"].(string)
	skipFinalSnapshot, _ := arguments["skipFinalSnapshot"].(bool)

	if dbInstanceIdentifier == "" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{
					Type: "text",
					Text: "dbInstanceIdentifier is required",
				},
			},
		}, nil
	}

	response := map[string]interface{}{
		"dbInstanceIdentifier": dbInstanceIdentifier,
		"currentStatus":        "deleting",
		"skipFinalSnapshot":    skipFinalSnapshot,
		"resource": map[string]interface{}{
			"id":   dbInstanceIdentifier,
			"type": "db_instance",
		},
	}

	return m.createSuccessResponse(fmt.Sprintf("DB instance '%s' deletion initiated", dbInstanceIdentifier), response)
}

func (m *MockMCPServer) mockCreateDBSnapshot(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	dbSnapshotIdentifier, _ := arguments["dbSnapshotIdentifier"].(string)
	dbInstanceIdentifier, _ := arguments["dbInstanceIdentifier"].(string)

	if dbSnapshotIdentifier == "" || dbInstanceIdentifier == "" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{
					Type: "text",
					Text: "Both dbSnapshotIdentifier and dbInstanceIdentifier are required",
				},
			},
		}, nil
	}

	snapshotArn := fmt.Sprintf("arn:aws:rds:us-west-2:123456789012:snapshot:%s", dbSnapshotIdentifier)

	response := map[string]interface{}{
		"dbSnapshotIdentifier": dbSnapshotIdentifier,
		"dbInstanceIdentifier": dbInstanceIdentifier,
		"status":               "creating",
		"snapshotType":         "manual",
		"dbSnapshotArn":        snapshotArn,
		"resource": map[string]interface{}{
			"id":   dbSnapshotIdentifier,
			"type": "db_snapshot",
			"name": dbSnapshotIdentifier,
		},
	}

	return m.createSuccessResponse(fmt.Sprintf("DB snapshot '%s' creation initiated", dbSnapshotIdentifier), response)
}

func (m *MockMCPServer) mockListDBInstances(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	instances := []map[string]interface{}{
		{
			"dbInstanceIdentifier": "production-mysql",
			"dbInstanceClass":      "db.t3.medium",
			"engine":               "mysql",
			"dbInstanceStatus":     "available",
			"allocatedStorage":     100,
			"endpoint":             "production-mysql.cluster-xyz.us-west-2.rds.amazonaws.com",
			"port":                 3306,
		},
		{
			"dbInstanceIdentifier": "test-postgres",
			"dbInstanceClass":      "db.t3.micro",
			"engine":               "postgres",
			"dbInstanceStatus":     "available",
			"allocatedStorage":     20,
			"endpoint":             "test-postgres.cluster-abc.us-west-2.rds.amazonaws.com",
			"port":                 5432,
		},
	}

	response := map[string]interface{}{
		"dbInstances": instances,
		"count":       len(instances),
	}

	return m.createSuccessResponse(fmt.Sprintf("Found %d DB instances", len(instances)), response)
}

func (m *MockMCPServer) mockListDBSnapshots(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	snapshots := []map[string]interface{}{
		{
			"dbSnapshotIdentifier": "production-mysql-snapshot-2023-10-01",
			"dbInstanceIdentifier": "production-mysql",
			"status":               "available",
			"snapshotType":         "manual",
			"allocatedStorage":     100,
		},
	}

	response := map[string]interface{}{
		"dbSnapshots": snapshots,
		"count":       len(snapshots),
	}

	return m.createSuccessResponse(fmt.Sprintf("Found %d DB snapshots", len(snapshots)), response)
}

// ==== Missing State Management Mock Methods ====

func (m *MockMCPServer) mockAnalyzeInfrastructureState(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	includeMetrics, _ := arguments["includeMetrics"].(bool)

	analysis := map[string]interface{}{
		"totalResources": 15,
		"resourcesByType": map[string]int{
			"vpc":            2,
			"subnet":         4,
			"ec2_instance":   3,
			"security_group": 3,
			"load_balancer":  1,
			"target_group":   1,
			"db_instance":    1,
		},
		"healthStatus": "healthy",
		"warnings": []string{
			"Security group sg-web123 has overly permissive ingress rules",
		},
		"recommendations": []string{
			"Consider enabling VPC Flow Logs for better network monitoring",
			"Review security group rules for least privilege access",
		},
	}

	if includeMetrics {
		analysis["metrics"] = map[string]interface{}{
			"averageResourceAge":  "30 days",
			"resourceUtilization": "75%",
			"complianceScore":     85,
		}
	}

	response := map[string]interface{}{
		"analysis": analysis,
	}

	return m.createSuccessResponse("Infrastructure state analyzed successfully", response)
}

func (m *MockMCPServer) mockVisualizeDependencyGraph(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	format, _ := arguments["format"].(string)
	if format == "" {
		format = "mermaid"
	}

	var visualization string
	if format == "mermaid" {
		visualization = `graph TD
    VPC[vpc-prod123] --> Subnet1[subnet-web-1]
    VPC --> Subnet2[subnet-web-2]
    Subnet1 --> ALB[test-alb]
    Subnet2 --> ALB
    ALB --> TG[web-servers]
    TG --> EC2_1[i-web-1]
    TG --> EC2_2[i-web-2]
    VPC --> SG[sg-web123]
    SG --> EC2_1
    SG --> EC2_2`
	} else {
		visualization = "Dependency graph visualization not available for format: " + format
	}

	response := map[string]interface{}{
		"format":        format,
		"visualization": visualization,
		"nodeCount":     8,
		"edgeCount":     10,
	}

	return m.createSuccessResponse("Dependency graph generated successfully", response)
}

func (m *MockMCPServer) mockDetectInfrastructureConflicts(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	conflicts := []map[string]interface{}{
		{
			"type":           "security_group_overlap",
			"severity":       "medium",
			"resources":      []string{"sg-web123", "sg-app123"},
			"description":    "Security groups have overlapping port ranges that may cause confusion",
			"recommendation": "Review and consolidate security group rules",
		},
		{
			"type":           "subnet_cidr_conflict",
			"severity":       "low",
			"resources":      []string{"subnet-web-1", "subnet-db-1"},
			"description":    "Subnets are in different availability zones but have similar naming",
			"recommendation": "Use consistent naming convention for subnets",
		},
	}

	response := map[string]interface{}{
		"conflicts":     conflicts,
		"conflictCount": len(conflicts),
		"severityCounts": map[string]int{
			"high":   0,
			"medium": 1,
			"low":    1,
		},
	}

	return m.createSuccessResponse(fmt.Sprintf("Found %d infrastructure conflicts", len(conflicts)), response)
}

func (m *MockMCPServer) mockPlanInfrastructureDeployment(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	dryRun, _ := arguments["dryRun"].(bool)

	plan := map[string]interface{}{
		"totalSteps":        8,
		"estimatedDuration": "15 minutes",
		"steps": []map[string]interface{}{
			{
				"step":         1,
				"action":       "create",
				"resourceType": "vpc",
				"resourceName": "production-vpc",
				"dependencies": []string{},
			},
			{
				"step":         2,
				"action":       "create",
				"resourceType": "subnet",
				"resourceName": "web-subnet-1",
				"dependencies": []string{"production-vpc"},
			},
			{
				"step":         3,
				"action":       "create",
				"resourceType": "subnet",
				"resourceName": "web-subnet-2",
				"dependencies": []string{"production-vpc"},
			},
			{
				"step":         4,
				"action":       "create",
				"resourceType": "security_group",
				"resourceName": "web-sg",
				"dependencies": []string{"production-vpc"},
			},
			{
				"step":         5,
				"action":       "create",
				"resourceType": "load_balancer",
				"resourceName": "web-alb",
				"dependencies": []string{"web-subnet-1", "web-subnet-2", "web-sg"},
			},
		},
		"warnings": []string{
			"This deployment will create resources that incur AWS charges",
		},
	}

	if dryRun {
		plan["mode"] = "dry-run"
		plan["status"] = "planned"
	} else {
		plan["mode"] = "execute"
		plan["status"] = "ready"
	}

	response := map[string]interface{}{
		"deploymentPlan": plan,
	}

	return m.createSuccessResponse("Deployment plan generated successfully", response)
}

func (m *MockMCPServer) mockAddResourceToState(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	resourceId, _ := arguments["resource_id"].(string)
	resourceType, _ := arguments["resource_type"].(string)
	resourceData, _ := arguments["properties"].(map[string]interface{})

	if resourceId == "" || resourceType == "" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{
					Type: "text",
					Text: "resourceId and resourceType are required",
				},
			},
		}, nil
	}

	// Add to mock state manager
	resource := &types.AWSResource{
		ID:      resourceId,
		Type:    resourceType,
		Details: resourceData,
	}

	m.resources[resourceId] = resource

	response := map[string]interface{}{
		"resourceId":   resourceId,
		"resourceType": resourceType,
		"status":       "added_to_state",
		"resource":     resource,
	}

	return m.createSuccessResponse(fmt.Sprintf("Resource %s added to state successfully", resourceId), response)
}

func (m *MockMCPServer) mockSaveState(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	filePath, _ := arguments["filePath"].(string)
	if filePath == "" {
		filePath = "infrastructure-state-test.json"
	}

	resourceCount := len(m.resources)

	response := map[string]interface{}{
		"filePath":      filePath,
		"resourceCount": resourceCount,
		"timestamp":     "2023-10-20T10:30:00Z",
		"status":        "saved",
	}

	return m.createSuccessResponse(fmt.Sprintf("State saved to %s with %d resources", filePath, resourceCount), response)
}

// ==== Mock Configuration Methods ====

// SetError configures error simulation for a specific tool
func (m *MockMCPServer) SetError(toolName string, err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.simulateErrors[toolName] = err
}
