package mocks

import (
	"context"
	"testing"

	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
)

func TestMockMCPServerWithAWSClientIntegration(t *testing.T) {
	logger := logging.NewLogger("test", "info")
	awsClient := NewMockAWSClient("us-east-1", logger)
	mcpServer := NewMockMCPServerWithAWSClient(logger, awsClient)

	ctx := context.Background()

	t.Run("VPC operations with AWS client", func(t *testing.T) {
		// Test VPC creation through MCP server that calls AWS client
		result, err := mcpServer.CallTool(ctx, "create-vpc", map[string]interface{}{
			"name":      "test-vpc-integration",
			"cidrBlock": "10.0.0.0/16",
		})

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if result.IsError {
			t.Errorf("Expected successful VPC creation, got error: %v", result.Content)
		}

		// Verify the VPC was actually created in the AWS client
		vpcs, err := awsClient.DescribeVPCs(ctx)
		if err != nil {
			t.Errorf("Failed to list VPCs: %v", err)
		}

		found := false
		for _, vpc := range vpcs {
			if vpc.Details["name"] == "test-vpc-integration" {
				found = true
				break
			}
		}

		if !found {
			t.Error("VPC not found in AWS client after creation through MCP server")
		}
	})

	t.Run("List VPCs through MCP server", func(t *testing.T) {
		// Test VPC listing through MCP server that calls AWS client
		result, err := mcpServer.CallTool(ctx, "list-vpcs", map[string]interface{}{})

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if result.IsError {
			t.Errorf("Expected successful VPC listing, got error: %v", result.Content)
		}

		// Check if we have the VPC we created previously
		// The result should contain the VPC from the AWS client
		// This verifies the integration is working
	})

	t.Run("Subnet creation with AWS client validation", func(t *testing.T) {
		// First create a VPC to put the subnet in
		vpcResult, err := mcpServer.CallTool(ctx, "create-vpc", map[string]interface{}{
			"name":      "vpc-for-subnet",
			"cidrBlock": "10.1.0.0/16",
		})

		if err != nil || vpcResult.IsError {
			t.Fatalf("Failed to create VPC for subnet test: %v", err)
		}

		// Test subnet creation with valid parameters
		result, err := mcpServer.CallTool(ctx, "create-subnet", map[string]interface{}{
			"name":             "test-subnet-integration",
			"vpcId":            "vpc-12345678", // Use mock format
			"cidrBlock":        "10.1.1.0/24",
			"availabilityZone": "us-east-1a",
		})

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if result.IsError {
			t.Errorf("Expected successful subnet creation, got error: %v", result.Content)
		}
	})

	t.Run("EC2 instance creation with validation", func(t *testing.T) {
		// Test EC2 instance creation with valid parameters
		result, err := mcpServer.CallTool(ctx, "create-ec2-instance", map[string]interface{}{
			"name":         "test-instance-integration",
			"imageId":      "ami-12345678", // Valid mock AMI format
			"instanceType": "t3.micro",
		})

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if result.IsError {
			t.Errorf("Expected successful EC2 instance creation, got error: %v", result.Content)
		}
	})

	t.Run("Parameter validation through MCP server", func(t *testing.T) {
		// Test that parameter validation works through the MCP server
		result, err := mcpServer.CallTool(ctx, "create-vpc", map[string]interface{}{
			"name":      "",             // Invalid - empty name
			"cidrBlock": "invalid-cidr", // Invalid CIDR
		})

		// This should fail due to validation
		if err == nil && !result.IsError {
			t.Error("Expected validation error for invalid VPC parameters")
		}
	})

	t.Run("MockMCPServer as MCPTool", func(t *testing.T) {
		// Test that MockMCPServer can be used as an MCPTool itself
		if mcpServer.Name() != "mock-mcp-server" {
			t.Error("MockMCPServer name method failed")
		}

		if mcpServer.Category() != "testing" {
			t.Error("MockMCPServer category method failed")
		}

		// Test validation
		err := mcpServer.ValidateArguments(map[string]interface{}{})
		if err == nil {
			t.Error("Expected validation error for missing tool_name")
		}

		err = mcpServer.ValidateArguments(map[string]interface{}{
			"tool_name": "create-vpc",
		})
		if err != nil {
			t.Errorf("Expected no validation error for valid arguments: %v", err)
		}

		// Test Execute method
		result, err := mcpServer.Execute(ctx, map[string]interface{}{
			"tool_name": "create-vpc",
			"name":      "test-vpc-execute",
			"cidrBlock": "10.2.0.0/16",
		})

		if err != nil {
			t.Errorf("Expected no error in Execute: %v", err)
		}

		if result.IsError {
			t.Errorf("Expected successful Execute: %v", result.Content)
		}
	})
}

func TestMockMCPServerFallbackBehavior(t *testing.T) {
	// Test that the mock server works even without AWS client (fallback mode)
	logger := logging.NewLogger("test", "info")
	mcpServer := NewMockMCPServer(logger) // No AWS client

	ctx := context.Background()

	t.Run("Fallback to static responses", func(t *testing.T) {
		// This should use static mock responses since no AWS client is set
		result, err := mcpServer.CallTool(ctx, "create-vpc", map[string]interface{}{
			"name":      "fallback-vpc",
			"cidrBlock": "10.0.0.0/16",
		})

		if err != nil {
			t.Errorf("Expected no error in fallback mode: %v", err)
		}

		if result.IsError {
			t.Errorf("Expected successful fallback response: %v", result.Content)
		}
	})

	t.Run("Set AWS client after creation", func(t *testing.T) {
		// Test that we can set AWS client after mock server creation
		awsClient := NewMockAWSClient("us-west-2", logger)
		mcpServer.SetAWSClient(awsClient)

		// Now it should use the AWS client
		result, err := mcpServer.CallTool(ctx, "create-vpc", map[string]interface{}{
			"name":      "late-bound-vpc",
			"cidrBlock": "10.3.0.0/16",
		})

		if err != nil {
			t.Errorf("Expected no error after setting AWS client: %v", err)
		}

		if result.IsError {
			t.Errorf("Expected successful response after setting AWS client: %v", result.Content)
		}

		// Verify the VPC was created in the AWS client
		vpcs, err := awsClient.DescribeVPCs(ctx)
		if err != nil {
			t.Errorf("Failed to list VPCs: %v", err)
		}

		found := false
		for _, vpc := range vpcs {
			if vpc.Details["name"] == "late-bound-vpc" {
				found = true
				break
			}
		}

		if !found {
			t.Error("VPC not found in AWS client after late binding")
		}
	})
}

func TestMockTestSuiteIntegration(t *testing.T) {
	// Test that the MockTestSuite properly integrates everything
	suite, err := NewMockTestSuite("us-east-1")
	if err != nil {
		t.Fatalf("Failed to create mock test suite: %v", err)
	}

	ctx := context.Background()

	t.Run("Integrated test suite", func(t *testing.T) {
		// The suite should have MCP server connected to AWS client
		result, err := suite.MCPServer.CallTool(ctx, "create-vpc", map[string]interface{}{
			"name":      "suite-vpc",
			"cidrBlock": "10.4.0.0/16",
		})

		if err != nil {
			t.Errorf("Expected no error in test suite: %v", err)
		}

		if result.IsError {
			t.Errorf("Expected successful response in test suite: %v", result.Content)
		}

		// Verify it's in the AWS client
		vpcs, err := suite.AWSClient.DescribeVPCs(ctx)
		if err != nil {
			t.Errorf("Failed to list VPCs in test suite: %v", err)
		}

		found := false
		for _, vpc := range vpcs {
			if vpc.Details["name"] == "suite-vpc" {
				found = true
				break
			}
		}

		if !found {
			t.Error("VPC not found in test suite AWS client")
		}
	})
}
