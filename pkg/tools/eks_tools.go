package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/aws"
	"github.com/versus-control/ai-infrastructure-agent/pkg/interfaces"
)

// CreateEKSClusterTool implements MCPTool for creating EKS clusters
type CreateEKSClusterTool struct {
	*BaseTool
	client *aws.Client
}

// NewCreateEKSClusterTool creates a new EKS cluster creation tool
func NewCreateEKSClusterTool(client *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "The unique name to give to your cluster",
			},
			"roleArn": map[string]interface{}{
				"type":        "string",
				"description": "The Amazon Resource Name (ARN) of the IAM role that provides permissions for the Kubernetes control plane to make calls to AWS API operations on your behalf",
			},
			"subnetIds": map[string]interface{}{
				"type":        "array",
				"items":       map[string]interface{}{"type": "string"},
				"description": "The VPC subnets to use for the cluster control plane. Must be in at least two different availability zones",
			},
			"securityGroupIds": map[string]interface{}{
				"type":        "array",
				"items":       map[string]interface{}{"type": "string"},
				"description": "Security groups for the cluster control plane communication",
			},
			"version": map[string]interface{}{
				"type":        "string",
				"description": "The desired Kubernetes version (e.g., '1.30')",
			},
		},
		"required": []interface{}{"name", "roleArn", "subnetIds"},
	}

	baseTool := NewBaseTool(
		"create-eks-cluster",
		"Create a new AWS EKS (Elastic Kubernetes Service) cluster",
		"eks",
		actionType,
		inputSchema,
		logger,
	)

	return &CreateEKSClusterTool{
		BaseTool: baseTool,
		client:   client,
	}
}

// Execute creates an EKS cluster
func (t *CreateEKSClusterTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	name, _ := arguments["name"].(string)
	roleArn, _ := arguments["roleArn"].(string)
	version, _ := arguments["version"].(string)

	// Validate and potentially fix roleArn
	var roleName string
	if roleArn != "" {
		accountID, err := t.client.GetAccountID(ctx)
		if err == nil {
			if !strings.HasPrefix(roleArn, "arn:") {
				roleName = roleArn
				// It's likely a role name, construct full ARN
				roleArn = fmt.Sprintf("arn:aws:iam::%s:role/%s", accountID, roleArn)
				t.GetLogger().Infof("Constructed ARN from role name: %s", roleArn)
			} else {
				// Try to extract role name from ARN
				parts := strings.Split(roleArn, "/")
				if len(parts) > 0 {
					roleName = parts[len(parts)-1]
				}

				// Check if ARN has correct account ID
				arnParts := strings.Split(roleArn, ":")
				if len(arnParts) >= 5 {
					arnAccountID := arnParts[4]
					// Check if account ID matches (and is not empty/wildcard)
					if arnAccountID != "" && arnAccountID != "aws" && arnAccountID != accountID {
						t.GetLogger().Warnf("Role ARN account ID %s does not match current account %s. Attempting to correct.", arnAccountID, accountID)
						arnParts[4] = accountID
						roleArn = strings.Join(arnParts, ":")
						t.GetLogger().Infof("Corrected Role ARN: %s", roleArn)
					}
				}
			}

			// Check if role exists and auto-create if missing
			if roleName != "" {
				_, err := t.client.GetIAMRole(ctx, roleName)
				if err != nil {
					t.GetLogger().Warnf("Role %s not found or error accessing it: %v. Attempting to create it.", roleName, err)

					assumeRolePolicy := `{
						"Version": "2012-10-17",
						"Statement": [
							{
								"Effect": "Allow",
								"Principal": {
									"Service": "eks.amazonaws.com"
								},
								"Action": "sts:AssumeRole"
							}
						]
					}`

					createParams := aws.CreateIAMRoleParams{
						RoleName:                 roleName,
						AssumeRolePolicyDocument: assumeRolePolicy,
						Description:              "Auto-created EKS Cluster Role",
					}

					_, createErr := t.client.CreateIAMRole(ctx, createParams)
					if createErr != nil {
						t.GetLogger().Errorf("Failed to auto-create role %s: %v", roleName, createErr)
					} else {
						t.GetLogger().Infof("Successfully created role %s", roleName)

						// Attach AmazonEKSClusterPolicy
						policyArn := "arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"
						if attachErr := t.client.AttachRolePolicy(ctx, roleName, policyArn); attachErr != nil {
							t.GetLogger().Errorf("Failed to attach policy %s to role %s: %v", policyArn, roleName, attachErr)
						}
					}
				}
			}
		} else {
			t.GetLogger().Warnf("Could not get account ID to validate role ARN: %v", err)
		}
	}

	var subnetIDs []string
	if val, ok := arguments["subnetIds"]; ok {
		if subnets, ok := val.([]interface{}); ok {
			for _, s := range subnets {
				if str, ok := s.(string); ok {
					subnetIDs = append(subnetIDs, str)
				}
			}
		} else if subnetStr, ok := val.(string); ok {
			// Handle both comma and underscore (agent internal format) delimiters
			normalized := strings.ReplaceAll(subnetStr, "_", ",")
			parts := strings.Split(normalized, ",")
			for _, p := range parts {
				if trimmed := strings.TrimSpace(p); trimmed != "" {
					subnetIDs = append(subnetIDs, trimmed)
				}
			}
		}
	}

	var securityGroupIDs []string
	if val, ok := arguments["securityGroupIds"]; ok {
		if sgs, ok := val.([]interface{}); ok {
			for _, sg := range sgs {
				if str, ok := sg.(string); ok {
					securityGroupIDs = append(securityGroupIDs, str)
				}
			}
		} else if sgStr, ok := val.(string); ok {
			// Handle both comma and underscore (agent internal format) delimiters
			normalized := strings.ReplaceAll(sgStr, "_", ",")
			parts := strings.Split(normalized, ",")
			for _, p := range parts {
				if trimmed := strings.TrimSpace(p); trimmed != "" {
					securityGroupIDs = append(securityGroupIDs, trimmed)
				}
			}
		}
	}

	params := aws.CreateEKSClusterParams{
		Name:             name,
		RoleArn:          roleArn,
		SubnetIDs:        subnetIDs,
		SecurityGroupIDs: securityGroupIDs,
		Version:          version,
	}

	resource, err := t.client.CreateEKSCluster(ctx, params)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to create EKS cluster: %s", err.Error()))
	}

	message := fmt.Sprintf("Successfully initiated creation of EKS cluster %s", resource.ID)
	data := map[string]interface{}{
		"clusterName": resource.ID,
		"state":       resource.State,
		"details":     resource.Details,
	}

	return t.CreateSuccessResponse(message, data)
}

// ListEKSClustersTool implements MCPTool for listing EKS clusters
type ListEKSClustersTool struct {
	*BaseTool
	client *aws.Client
}

// NewListEKSClustersTool creates a tool for listing EKS clusters
func NewListEKSClustersTool(client *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}

	baseTool := NewBaseTool(
		"list-eks-clusters",
		"List all EKS clusters in the current region",
		"eks",
		actionType,
		inputSchema,
		logger,
	)

	return &ListEKSClustersTool{
		BaseTool: baseTool,
		client:   client,
	}
}

// Execute lists EKS clusters
func (t *ListEKSClustersTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	clusters, err := t.client.ListEKSClusters(ctx)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to list EKS clusters: %s", err.Error()))
	}

	message := fmt.Sprintf("Found %d EKS clusters", len(clusters))
	data := map[string]interface{}{
		"clusters": clusters,
		"count":    len(clusters),
	}

	return t.CreateSuccessResponse(message, data)
}

// GetEKSClusterTool implements MCPTool for getting EKS cluster details
type GetEKSClusterTool struct {
	*BaseTool
	client *aws.Client
}

// NewGetEKSClusterTool creates a tool for getting EKS cluster details
func NewGetEKSClusterTool(client *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "The name of the EKS cluster",
			},
		},
		"required": []interface{}{"name"},
	}

	baseTool := NewBaseTool(
		"get-eks-cluster",
		"Get details of a specific EKS cluster",
		"eks",
		actionType,
		inputSchema,
		logger,
	)

	return &GetEKSClusterTool{
		BaseTool: baseTool,
		client:   client,
	}
}

// Execute gets EKS cluster details
func (t *GetEKSClusterTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	name, _ := arguments["name"].(string)
	if name == "" {
		return t.CreateErrorResponse("name is required")
	}

	resource, err := t.client.DescribeEKSCluster(ctx, name)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to describe EKS cluster: %s", err.Error()))
	}

	message := fmt.Sprintf("Successfully retrieved details for cluster %s", name)
	data := map[string]interface{}{
		"cluster":     resource,
		"clusterName": resource.ID,
	}

	return t.CreateSuccessResponse(message, data)
}

// CreateEKSNodeGroupTool implements MCPTool for creating EKS node groups
type CreateEKSNodeGroupTool struct {
	*BaseTool
	client *aws.Client
}

// NewCreateEKSNodeGroupTool creates a tool for creating EKS node groups
func NewCreateEKSNodeGroupTool(client *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"clusterName": map[string]interface{}{
				"type":        "string",
				"description": "The name of the EKS cluster",
			},
			"nodeGroupName": map[string]interface{}{
				"type":        "string",
				"description": "The unique name to give to your node group",
			},
			"nodeRoleArn": map[string]interface{}{
				"type":        "string",
				"description": "The Amazon Resource Name (ARN) of the IAM role to associate with your node group",
			},
			"subnetIds": map[string]interface{}{
				"type":        "array",
				"items":       map[string]interface{}{"type": "string"},
				"description": "The subnets to use for the Auto Scaling group that is created for your node group",
			},
			"instanceTypes": map[string]interface{}{
				"type":        "array",
				"items":       map[string]interface{}{"type": "string"},
				"description": "The instance types to use for your node group",
			},
			"minSize": map[string]interface{}{
				"type":        "integer",
				"description": "The minimum number of nodes that the managed node group can scale in to",
			},
			"maxSize": map[string]interface{}{
				"type":        "integer",
				"description": "The maximum number of nodes that the managed node group can scale out to",
			},
			"desiredSize": map[string]interface{}{
				"type":        "integer",
				"description": "The current number of nodes that the managed node group should maintain",
			},
		},
		"required": []interface{}{"clusterName", "nodeGroupName", "nodeRoleArn", "subnetIds"},
	}

	baseTool := NewBaseTool(
		"create-eks-node-group",
		"Create a managed node group for an EKS cluster",
		"eks",
		actionType,
		inputSchema,
		logger,
	)

	return &CreateEKSNodeGroupTool{
		BaseTool: baseTool,
		client:   client,
	}
}

// Execute creates an EKS node group
func (t *CreateEKSNodeGroupTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	clusterName, _ := arguments["clusterName"].(string)
	nodeGroupName, _ := arguments["nodeGroupName"].(string)
	nodeRoleArn, _ := arguments["nodeRoleArn"].(string)

	// Validate and potentially fix nodeRoleArn
	var roleName string
	if nodeRoleArn != "" {
		accountID, err := t.client.GetAccountID(ctx)
		if err == nil {
			if !strings.HasPrefix(nodeRoleArn, "arn:") {
				roleName = nodeRoleArn
				// It's likely a role name, construct full ARN
				nodeRoleArn = fmt.Sprintf("arn:aws:iam::%s:role/%s", accountID, nodeRoleArn)
				t.GetLogger().Infof("Constructed ARN from role name: %s", nodeRoleArn)
			} else {
				// Try to extract role name from ARN
				parts := strings.Split(nodeRoleArn, "/")
				if len(parts) > 0 {
					roleName = parts[len(parts)-1]
				}

				// Check if ARN has correct account ID
				arnParts := strings.Split(nodeRoleArn, ":")
				if len(arnParts) >= 5 {
					arnAccountID := arnParts[4]
					// Check if account ID matches (and is not empty/wildcard)
					if arnAccountID != "" && arnAccountID != "aws" && arnAccountID != accountID {
						t.GetLogger().Warnf("Role ARN account ID %s does not match current account %s. Attempting to correct.", arnAccountID, accountID)
						arnParts[4] = accountID
						nodeRoleArn = strings.Join(arnParts, ":")
						t.GetLogger().Infof("Corrected Role ARN: %s", nodeRoleArn)
					}
				}
			}

			// Check if role exists and auto-create if missing
			if roleName != "" {
				_, err := t.client.GetIAMRole(ctx, roleName)
				if err != nil {
					t.GetLogger().Warnf("Role %s not found or error accessing it: %v. Attempting to create it.", roleName, err)

					assumeRolePolicy := `{
						"Version": "2012-10-17",
						"Statement": [
							{
								"Effect": "Allow",
								"Principal": {
									"Service": "ec2.amazonaws.com"
								},
								"Action": "sts:AssumeRole"
							}
						]
					}`

					createParams := aws.CreateIAMRoleParams{
						RoleName:                 roleName,
						AssumeRolePolicyDocument: assumeRolePolicy,
						Description:              "Auto-created EKS Node Group Role",
					}

					_, createErr := t.client.CreateIAMRole(ctx, createParams)
					if createErr != nil {
						t.GetLogger().Errorf("Failed to auto-create role %s: %v", roleName, createErr)
					} else {
						t.GetLogger().Infof("Successfully created role %s", roleName)

						policies := []string{
							"arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy",
							"arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly",
							"arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy",
						}

						for _, policyArn := range policies {
							if attachErr := t.client.AttachRolePolicy(ctx, roleName, policyArn); attachErr != nil {
								t.GetLogger().Errorf("Failed to attach policy %s to role %s: %v", policyArn, roleName, attachErr)
							}
						}
					}
				}
			}
		} else {
			t.GetLogger().Warnf("Could not get account ID to validate role ARN: %v", err)
		}
	}

	var subnetIDs []string
	if val, ok := arguments["subnetIds"]; ok {
		if subnets, ok := val.([]interface{}); ok {
			for _, s := range subnets {
				if str, ok := s.(string); ok {
					subnetIDs = append(subnetIDs, str)
				}
			}
		} else if subnetStr, ok := val.(string); ok {
			// Handle both comma and underscore (agent internal format) delimiters
			normalized := strings.ReplaceAll(subnetStr, "_", ",")
			parts := strings.Split(normalized, ",")
			for _, p := range parts {
				if trimmed := strings.TrimSpace(p); trimmed != "" {
					subnetIDs = append(subnetIDs, trimmed)
				}
			}
		}
	}

	var instanceTypes []string
	if val, ok := arguments["instanceTypes"]; ok {
		if types, ok := val.([]interface{}); ok {
			for _, typ := range types {
				if str, ok := typ.(string); ok {
					instanceTypes = append(instanceTypes, str)
				}
			}
		} else if typeStr, ok := val.(string); ok {
			parts := strings.Split(typeStr, ",")
			for _, p := range parts {
				if trimmed := strings.TrimSpace(p); trimmed != "" {
					instanceTypes = append(instanceTypes, trimmed)
				}
			}
		}
	}

	minSize, _ := arguments["minSize"].(float64)
	maxSize, _ := arguments["maxSize"].(float64)
	desiredSize, _ := arguments["desiredSize"].(float64)

	params := aws.CreateEKSNodeGroupParams{
		ClusterName:   clusterName,
		NodeGroupName: nodeGroupName,
		NodeRoleArn:   nodeRoleArn,
		SubnetIDs:     subnetIDs,
		InstanceTypes: instanceTypes,
		MinSize:       int32(minSize),
		MaxSize:       int32(maxSize),
		DesiredSize:   int32(desiredSize),
	}

	resource, err := t.client.CreateEKSNodeGroup(ctx, params)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to create EKS node group: %s", err.Error()))
	}

	message := fmt.Sprintf("Successfully initiated creation of EKS node group %s", resource.ID)
	data := map[string]interface{}{
		"nodeGroupName": resource.ID,
		"clusterName":   clusterName,
		"state":         resource.State,
		"details":       resource.Details,
	}

	return t.CreateSuccessResponse(message, data)
}

// ListEKSNodeGroupsTool implements MCPTool for listing EKS node groups
type ListEKSNodeGroupsTool struct {
	*BaseTool
	client *aws.Client
}

// NewListEKSNodeGroupsTool creates a tool for listing EKS node groups
func NewListEKSNodeGroupsTool(client *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"clusterName": map[string]interface{}{
				"type":        "string",
				"description": "The name of the EKS cluster",
			},
		},
		"required": []interface{}{"clusterName"},
	}

	baseTool := NewBaseTool(
		"list-eks-node-groups",
		"List node groups in a specific EKS cluster",
		"eks",
		actionType,
		inputSchema,
		logger,
	)

	return &ListEKSNodeGroupsTool{
		BaseTool: baseTool,
		client:   client,
	}
}

// Execute lists EKS node groups
func (t *ListEKSNodeGroupsTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	clusterName, _ := arguments["clusterName"].(string)
	if clusterName == "" {
		return t.CreateErrorResponse("clusterName is required")
	}

	nodeGroups, err := t.client.ListEKSNodeGroups(ctx, clusterName)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to list EKS node groups: %s", err.Error()))
	}

	message := fmt.Sprintf("Found %d node groups in cluster %s", len(nodeGroups), clusterName)
	data := map[string]interface{}{
		"nodeGroups":  nodeGroups,
		"clusterName": clusterName,
		"count":       len(nodeGroups),
	}

	return t.CreateSuccessResponse(message, data)
}

// DeleteEKSClusterTool implements MCPTool for deleting EKS clusters
type DeleteEKSClusterTool struct {
	*BaseTool
	client *aws.Client
}

// NewDeleteEKSClusterTool creates a tool for deleting EKS clusters
func NewDeleteEKSClusterTool(client *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "The name of the EKS cluster to delete",
			},
		},
		"required": []interface{}{"name"},
	}

	baseTool := NewBaseTool(
		"delete-eks-cluster",
		"Delete an AWS EKS cluster",
		"eks",
		actionType,
		inputSchema,
		logger,
	)

	return &DeleteEKSClusterTool{
		BaseTool: baseTool,
		client:   client,
	}
}

// Execute deletes an EKS cluster
func (t *DeleteEKSClusterTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	name, _ := arguments["name"].(string)
	if name == "" {
		return t.CreateErrorResponse("name is required")
	}

	err := t.client.DeleteEKSCluster(ctx, name)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to delete EKS cluster: %s", err.Error()))
	}

	message := fmt.Sprintf("Successfully initiated deletion of EKS cluster %s", name)
	data := map[string]interface{}{
		"clusterName": name,
		"status":      "deleting",
	}

	return t.CreateSuccessResponse(message, data)
}

// ApplyKubernetesYAMLTool implements MCPTool for applying Kubernetes YAML to an EKS cluster
type ApplyKubernetesYAMLTool struct {
	*BaseTool
	client *aws.Client
}

// NewApplyKubernetesYAMLTool creates a tool for applying Kubernetes YAML
func NewApplyKubernetesYAMLTool(client *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"clusterName": map[string]interface{}{
				"type":        "string",
				"description": "The name of the EKS cluster",
			},
			"yaml": map[string]interface{}{
				"type":        "string",
				"description": "The YAML configuration to apply",
			},
		},
		"required": []interface{}{"clusterName", "yaml"},
	}

	baseTool := NewBaseTool(
		"apply-yaml",
		"Apply a Kubernetes YAML configuration to an EKS cluster",
		"eks",
		actionType,
		inputSchema,
		logger,
	)

	return &ApplyKubernetesYAMLTool{
		BaseTool: baseTool,
		client:   client,
	}
}

// Execute applies Kubernetes YAML
func (t *ApplyKubernetesYAMLTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	clusterName, _ := arguments["clusterName"].(string)
	yamlContent, _ := arguments["yaml"].(string)

	if clusterName == "" {
		return t.CreateErrorResponse("clusterName is required")
	}
	if yamlContent == "" {
		return t.CreateErrorResponse("yaml is required")
	}

	err := t.client.ApplyKubernetesYAML(ctx, clusterName, yamlContent)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to apply Kubernetes YAML: %s", err.Error()))
	}

	message := fmt.Sprintf("Successfully applied YAML to cluster %s", clusterName)
	data := map[string]interface{}{
		"clusterName": clusterName,
		"status":      "applied",
	}

	return t.CreateSuccessResponse(message, data)
}
