package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	awsclient "github.com/versus-control/ai-infrastructure-agent/pkg/aws"
	"github.com/versus-control/ai-infrastructure-agent/pkg/interfaces"
)

// ========== EKS Cluster Tools ==========

// CreateEKSClusterTool creates a new EKS cluster
type CreateEKSClusterTool struct {
	client     *awsclient.Client
	actionType string
	logger     *logging.Logger
}

func NewCreateEKSClusterTool(client *awsclient.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	return &CreateEKSClusterTool{
		client:     client,
		actionType: actionType,
		logger:     logger,
	}
}

func (t *CreateEKSClusterTool) Execute(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	t.logger.Info("Creating EKS cluster")

	// Parse arguments
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return createErrorResult("name is required and must be a string"), nil
	}

	version, ok := args["version"].(string)
	if !ok || version == "" {
		version = "1.28" // Default to a stable version
	}

	roleArn, ok := args["roleArn"].(string)
	if !ok || roleArn == "" {
		return createErrorResult("roleArn is required and must be a string"), nil
	}

	// Parse VPC configuration
	vpcConfig := &types.VpcConfigRequest{}
	if vpcConfigRaw, exists := args["vpcConfig"]; exists {
		if vpcConfigMap, ok := vpcConfigRaw.(map[string]interface{}); ok {
			if subnetIds, exists := vpcConfigMap["subnetIds"]; exists {
				if subnetList, ok := subnetIds.([]interface{}); ok {
					for _, subnet := range subnetList {
						if subnetStr, ok := subnet.(string); ok {
							vpcConfig.SubnetIds = append(vpcConfig.SubnetIds, subnetStr)
						}
					}
				}
			}
			if securityGroupIds, exists := vpcConfigMap["securityGroupIds"]; exists {
				if sgList, ok := securityGroupIds.([]interface{}); ok {
					for _, sg := range sgList {
						if sgStr, ok := sg.(string); ok {
							vpcConfig.SecurityGroupIds = append(vpcConfig.SecurityGroupIds, sgStr)
						}
					}
				}
			}
			if endpointConfigPublic, exists := vpcConfigMap["endpointConfigPublic"]; exists {
				if publicAccess, ok := endpointConfigPublic.(bool); ok {
					vpcConfig.EndpointConfigResponse = &types.VpcConfigResponse{
						EndpointPublicAccess: &publicAccess,
					}
				}
			}
		}
	}

	// Parse tags
	tags := make(map[string]string)
	if tagsRaw, exists := args["tags"]; exists {
		if tagsMap, ok := tagsRaw.(map[string]interface{}); ok {
			for key, value := range tagsMap {
				if valueStr, ok := value.(string); ok {
					tags[key] = valueStr
				}
			}
		}
	}

	// Create cluster input
	input := &eks.CreateClusterInput{
		Name:               aws.String(name),
		Version:            aws.String(version),
		RoleArn:            aws.String(roleArn),
		ResourcesVpcConfig: vpcConfig,
		Tags:               tags,
	}

	// Parse optional encryption config
	if encryptionRaw, exists := args["encryptionConfig"]; exists {
		if encryptionList, ok := encryptionRaw.([]interface{}); ok {
			for _, encItem := range encryptionList {
				if encMap, ok := encItem.(map[string]interface{}); ok {
					encConfig := types.EncryptionConfig{}
					if resources, exists := encMap["resources"]; exists {
						if resourceList, ok := resources.([]interface{}); ok {
							for _, resource := range resourceList {
								if resourceStr, ok := resource.(string); ok {
									encConfig.Resources = append(encConfig.Resources, resourceStr)
								}
							}
						}
					}
					if provider, exists := encMap["provider"]; exists {
						if providerMap, ok := provider.(map[string]interface{}); ok {
							if keyArn, exists := providerMap["keyArn"]; exists {
								if keyArnStr, ok := keyArn.(string); ok {
									encConfig.Provider = &types.Provider{
										KeyArn: aws.String(keyArnStr),
									}
								}
							}
						}
					}
					input.EncryptionConfig = append(input.EncryptionConfig, encConfig)
				}
			}
		}
	}

	// Parse logging configuration
	if loggingRaw, exists := args["logging"]; exists {
		if loggingMap, ok := loggingRaw.(map[string]interface{}); ok {
			logging := &types.Logging{}
			if enable, exists := loggingMap["enable"]; exists {
				if enableList, ok := enable.([]interface{}); ok {
					for _, logType := range enableList {
						if logTypeStr, ok := logType.(string); ok {
							logging.Enable = append(logging.Enable, types.LogType(logTypeStr))
						}
					}
				}
			}
			input.Logging = logging
		}
	}

	// Create the cluster
	result, err := t.client.CreateEKSCluster(ctx, input)
	if err != nil {
		t.logger.WithError(err).Error("Failed to create EKS cluster")
		return createErrorResult(fmt.Sprintf("Failed to create EKS cluster: %v", err)), nil
	}

	// Return success result
	return &mcp.CallToolResult{
		Meta: map[string]interface{}{
			"cluster_name": name,
		},
		Content: []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": fmt.Sprintf("EKS cluster '%s' creation initiated successfully", name),
			},
		},
		IsError: false,
	}, nil
}

func (t *CreateEKSClusterTool) GetSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Name of the EKS cluster",
			},
			"version": map[string]interface{}{
				"type":        "string",
				"description": "Kubernetes version for the cluster (e.g., '1.28')",
			},
			"roleArn": map[string]interface{}{
				"type":        "string",
				"description": "ARN of the IAM role for the EKS cluster service",
			},
			"vpcConfig": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"subnetIds": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "string",
						},
						"description": "List of subnet IDs for the cluster",
					},
					"securityGroupIds": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "string",
						},
						"description": "List of security group IDs for the cluster",
					},
					"endpointConfigPublic": map[string]interface{}{
						"type":        "boolean",
						"description": "Whether the API server endpoint is publicly accessible",
					},
				},
				"description": "VPC configuration for the cluster",
			},
			"tags": map[string]interface{}{
				"type": "object",
				"additionalProperties": map[string]interface{}{
					"type": "string",
				},
				"description": "Tags to apply to the cluster",
			},
			"encryptionConfig": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"resources": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "string",
							},
						},
						"provider": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"keyArn": map[string]interface{}{
									"type": "string",
								},
							},
						},
					},
				},
				"description": "Encryption configuration for the cluster",
			},
			"logging": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"enable": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "string",
							"enum": []string{"api", "audit", "authenticator", "controllerManager", "scheduler"},
						},
						"description": "List of log types to enable",
					},
				},
				"description": "Logging configuration for the cluster",
			},
		},
		"required": []string{"name", "roleArn"},
	}
}

func (t *CreateEKSClusterTool) GetMetadata() ToolMetadata {
	return ToolMetadata{
		Name:        "create-eks-cluster",
		Description: "Creates a new Amazon EKS cluster with specified configuration",
		Category:    "EKS",
		Tags:        []string{"eks", "kubernetes", "cluster", "creation"},
	}
}

// ListEKSClustersTool lists all EKS clusters
type ListEKSClustersTool struct {
	client     *awsclient.Client
	actionType string
	logger     *logging.Logger
}

func NewListEKSClustersTool(client *awsclient.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	return &ListEKSClustersTool{
		client:     client,
		actionType: actionType,
		logger:     logger,
	}
}

func (t *ListEKSClustersTool) Execute(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	t.logger.Info("Listing EKS clusters")

	// List clusters
	result, err := t.client.ListEKSClusters(ctx)
	if err != nil {
		t.logger.WithError(err).Error("Failed to list EKS clusters")
		return createErrorResult(fmt.Sprintf("Failed to list EKS clusters: %v", err)), nil
	}

	// Get detailed information for each cluster if requested
	includeDetails := false
	if details, exists := args["includeDetails"]; exists {
		if detailsBool, ok := details.(bool); ok {
			includeDetails = detailsBool
		}
	}

	clusters := make([]map[string]interface{}, 0)
	for _, clusterName := range result.Clusters {
		clusterInfo := map[string]interface{}{
			"name": clusterName,
		}

		if includeDetails {
			// Get detailed cluster information
			describeResult, err := t.client.DescribeEKSCluster(ctx, clusterName)
			if err != nil {
				t.logger.WithError(err).WithField("cluster", clusterName).Warn("Failed to describe cluster")
				clusterInfo["error"] = fmt.Sprintf("Failed to get details: %v", err)
			} else if describeResult.Cluster != nil {
				cluster := describeResult.Cluster
				clusterInfo["version"] = *cluster.Version
				clusterInfo["status"] = string(cluster.Status)
				clusterInfo["endpoint"] = *cluster.Endpoint
				clusterInfo["platformVersion"] = *cluster.PlatformVersion
				clusterInfo["roleArn"] = *cluster.RoleArn
				clusterInfo["createdAt"] = cluster.CreatedAt.Format(time.RFC3339)

				if cluster.ResourcesVpcConfig != nil {
					clusterInfo["vpcId"] = *cluster.ResourcesVpcConfig.VpcId
					clusterInfo["subnetIds"] = cluster.ResourcesVpcConfig.SubnetIds
					clusterInfo["securityGroupIds"] = cluster.ResourcesVpcConfig.SecurityGroupIds
				}

				if cluster.Tags != nil {
					clusterInfo["tags"] = cluster.Tags
				}
			}
		}

		clusters = append(clusters, clusterInfo)
	}

	return &mcp.CallToolResult{
		Meta: map[string]interface{}{
			"cluster_count": len(clusters),
		},
		Content: []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": fmt.Sprintf("Found %d EKS clusters", len(clusters)),
			},
			map[string]interface{}{
				"type": "resource",
				"resource": map[string]interface{}{
					"uri":      "eks://clusters",
					"mimeType": "application/json",
					"text":     mustMarshalJSON(clusters),
				},
			},
		},
		IsError: false,
	}, nil
}

func (t *ListEKSClustersTool) GetSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"includeDetails": map[string]interface{}{
				"type":        "boolean",
				"description": "Whether to include detailed information for each cluster",
				"default":     false,
			},
		},
	}
}

func (t *ListEKSClustersTool) GetMetadata() ToolMetadata {
	return ToolMetadata{
		Name:        "list-eks-clusters",
		Description: "Lists all Amazon EKS clusters in the current region",
		Category:    "EKS",
		Tags:        []string{"eks", "kubernetes", "cluster", "list"},
	}
}

// DescribeEKSClusterTool describes a specific EKS cluster
type DescribeEKSClusterTool struct {
	client     *awsclient.Client
	actionType string
	logger     *logging.Logger
}

func NewDescribeEKSClusterTool(client *awsclient.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	return &DescribeEKSClusterTool{
		client:     client,
		actionType: actionType,
		logger:     logger,
	}
}

func (t *DescribeEKSClusterTool) Execute(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	t.logger.Info("Describing EKS cluster")

	// Parse arguments
	clusterName, ok := args["clusterName"].(string)
	if !ok || clusterName == "" {
		return createErrorResult("clusterName is required and must be a string"), nil
	}

	// Describe the cluster
	result, err := t.client.DescribeEKSCluster(ctx, clusterName)
	if err != nil {
		t.logger.WithError(err).Error("Failed to describe EKS cluster")
		return createErrorResult(fmt.Sprintf("Failed to describe EKS cluster: %v", err)), nil
	}

	if result.Cluster == nil {
		return createErrorResult("Cluster not found"), nil
	}

	cluster := result.Cluster
	clusterInfo := map[string]interface{}{
		"name":            *cluster.Name,
		"version":         *cluster.Version,
		"status":          string(cluster.Status),
		"endpoint":        *cluster.Endpoint,
		"platformVersion": *cluster.PlatformVersion,
		"roleArn":         *cluster.RoleArn,
		"createdAt":       cluster.CreatedAt.Format(time.RFC3339),
	}

	if cluster.ResourcesVpcConfig != nil {
		clusterInfo["vpcConfig"] = map[string]interface{}{
			"vpcId":                    *cluster.ResourcesVpcConfig.VpcId,
			"subnetIds":                cluster.ResourcesVpcConfig.SubnetIds,
			"securityGroupIds":         cluster.ResourcesVpcConfig.SecurityGroupIds,
			"endpointPublicAccess":     *cluster.ResourcesVpcConfig.EndpointPublicAccess,
			"endpointPrivateAccess":    *cluster.ResourcesVpcConfig.EndpointPrivateAccess,
			"publicAccessCidrs":        cluster.ResourcesVpcConfig.PublicAccessCidrs,
		}
	}

	if cluster.Tags != nil {
		clusterInfo["tags"] = cluster.Tags
	}

	if cluster.Logging != nil {
		clusterInfo["logging"] = map[string]interface{}{
			"clusterLogging": cluster.Logging.ClusterLogging,
		}
	}

	return &mcp.CallToolResult{
		Meta: map[string]interface{}{
			"cluster_name": clusterName,
		},
		Content: []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": fmt.Sprintf("EKS cluster '%s' details retrieved successfully", clusterName),
			},
			map[string]interface{}{
				"type": "resource",
				"resource": map[string]interface{}{
					"uri":      fmt.Sprintf("eks://cluster/%s", clusterName),
					"mimeType": "application/json",
					"text":     mustMarshalJSON(clusterInfo),
				},
			},
		},
		IsError: false,
	}, nil
}

func (t *DescribeEKSClusterTool) GetSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"clusterName": map[string]interface{}{
				"type":        "string",
				"description": "Name of the EKS cluster to describe",
			},
		},
		"required": []string{"clusterName"},
	}
}

func (t *DescribeEKSClusterTool) GetMetadata() ToolMetadata {
	return ToolMetadata{
		Name:        "describe-eks-cluster",
		Description: "Describes a specific Amazon EKS cluster with detailed information",
		Category:    "EKS",
		Tags:        []string{"eks", "kubernetes", "cluster", "describe"},
	}
}

// DeleteEKSClusterTool deletes an EKS cluster
type DeleteEKSClusterTool struct {
	client     *awsclient.Client
	actionType string
	logger     *logging.Logger
}

func NewDeleteEKSClusterTool(client *awsclient.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	return &DeleteEKSClusterTool{
		client:     client,
		actionType: actionType,
		logger:     logger,
	}
}

func (t *DeleteEKSClusterTool) Execute(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	t.logger.Info("Deleting EKS cluster")

	// Parse arguments
	clusterName, ok := args["clusterName"].(string)
	if !ok || clusterName == "" {
		return createErrorResult("clusterName is required and must be a string"), nil
	}

	// Delete the cluster
	result, err := t.client.DeleteEKSCluster(ctx, clusterName)
	if err != nil {
		t.logger.WithError(err).Error("Failed to delete EKS cluster")
		return createErrorResult(fmt.Sprintf("Failed to delete EKS cluster: %v", err)), nil
	}

	return &mcp.CallToolResult{
		Meta: map[string]interface{}{
			"cluster_name": clusterName,
		},
		Content: []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": fmt.Sprintf("EKS cluster '%s' deletion initiated successfully", clusterName),
			},
		},
		IsError: false,
	}, nil
}

func (t *DeleteEKSClusterTool) GetSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"clusterName": map[string]interface{}{
				"type":        "string",
				"description": "Name of the EKS cluster to delete",
			},
		},
		"required": []string{"clusterName"},
	}
}

func (t *DeleteEKSClusterTool) GetMetadata() ToolMetadata {
	return ToolMetadata{
		Name:        "delete-eks-cluster",
		Description: "Deletes an Amazon EKS cluster",
		Category:    "EKS",
		Tags:        []string{"eks", "kubernetes", "cluster", "delete"},
	}
}

// ========== EKS Node Group Tools ==========

// CreateEKSNodeGroupTool creates a new EKS node group
type CreateEKSNodeGroupTool struct {
	client     *awsclient.Client
	actionType string
	logger     *logging.Logger
}

func NewCreateEKSNodeGroupTool(client *awsclient.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	return &CreateEKSNodeGroupTool{
		client:     client,
		actionType: actionType,
		logger:     logger,
	}
}

func (t *CreateEKSNodeGroupTool) Execute(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	t.logger.Info("Creating EKS node group")

	// Parse required arguments
	clusterName, ok := args["clusterName"].(string)
	if !ok || clusterName == "" {
		return createErrorResult("clusterName is required and must be a string"), nil
	}

	nodeGroupName, ok := args["nodeGroupName"].(string)
	if !ok || nodeGroupName == "" {
		return createErrorResult("nodeGroupName is required and must be a string"), nil
	}

	nodeRole, ok := args["nodeRole"].(string)
	if !ok || nodeRole == "" {
		return createErrorResult("nodeRole is required and must be a string"), nil
	}

	// Parse subnets
	var subnets []string
	if subnetsRaw, exists := args["subnets"]; exists {
		if subnetList, ok := subnetsRaw.([]interface{}); ok {
			for _, subnet := range subnetList {
				if subnetStr, ok := subnet.(string); ok {
					subnets = append(subnets, subnetStr)
				}
			}
		}
	}
	if len(subnets) == 0 {
		return createErrorResult("subnets is required and must be a non-empty array"), nil
	}

	// Create node group input
	input := &eks.CreateNodegroupInput{
		ClusterName:   aws.String(clusterName),
		NodegroupName: aws.String(nodeGroupName),
		NodeRole:      aws.String(nodeRole),
		Subnets:       subnets,
	}

	// Parse optional scaling configuration
	if scalingRaw, exists := args["scalingConfig"]; exists {
		if scalingMap, ok := scalingRaw.(map[string]interface{}); ok {
			scalingConfig := &types.NodegroupScalingConfig{}
			if minSize, exists := scalingMap["minSize"]; exists {
				if minSizeInt, ok := minSize.(float64); ok {
					scalingConfig.MinSize = aws.Int32(int32(minSizeInt))
				}
			}
			if maxSize, exists := scalingMap["maxSize"]; exists {
				if maxSizeInt, ok := maxSize.(float64); ok {
					scalingConfig.MaxSize = aws.Int32(int32(maxSizeInt))
				}
			}
			if desiredSize, exists := scalingMap["desiredSize"]; exists {
				if desiredSizeInt, ok := desiredSize.(float64); ok {
					scalingConfig.DesiredSize = aws.Int32(int32(desiredSizeInt))
				}
			}
			input.ScalingConfig = scalingConfig
		}
	}

	// Parse instance types
	if instanceTypesRaw, exists := args["instanceTypes"]; exists {
		if instanceTypeList, ok := instanceTypesRaw.([]interface{}); ok {
			for _, instanceType := range instanceTypeList {
				if instanceTypeStr, ok := instanceType.(string); ok {
					input.InstanceTypes = append(input.InstanceTypes, instanceTypeStr)
				}
			}
		}
	}

	// Parse AMI type
	if amiType, exists := args["amiType"]; exists {
		if amiTypeStr, ok := amiType.(string); ok {
			input.AmiType = types.AMITypes(amiTypeStr)
		}
	}

	// Parse capacity type
	if capacityType, exists := args["capacityType"]; exists {
		if capacityTypeStr, ok := capacityType.(string); ok {
			input.CapacityType = types.CapacityTypes(capacityTypeStr)
		}
	}

	// Parse tags
	if tagsRaw, exists := args["tags"]; exists {
		if tagsMap, ok := tagsRaw.(map[string]interface{}); ok {
			tags := make(map[string]string)
			for key, value := range tagsMap {
				if valueStr, ok := value.(string); ok {
					tags[key] = valueStr
				}
			}
			input.Tags = tags
		}
	}

	// Create the node group
	result, err := t.client.CreateEKSNodeGroup(ctx, input)
	if err != nil {
		t.logger.WithError(err).Error("Failed to create EKS node group")
		return createErrorResult(fmt.Sprintf("Failed to create EKS node group: %v", err)), nil
	}

	return &mcp.CallToolResult{
		Meta: map[string]interface{}{
			"cluster_name":    clusterName,
			"node_group_name": nodeGroupName,
		},
		Content: []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": fmt.Sprintf("EKS node group '%s' creation initiated successfully for cluster '%s'", nodeGroupName, clusterName),
			},
		},
		IsError: false,
	}, nil
}

func (t *CreateEKSNodeGroupTool) GetSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"clusterName": map[string]interface{}{
				"type":        "string",
				"description": "Name of the EKS cluster",
			},
			"nodeGroupName": map[string]interface{}{
				"type":        "string",
				"description": "Name of the node group",
			},
			"nodeRole": map[string]interface{}{
				"type":        "string",
				"description": "ARN of the IAM role for the node group",
			},
			"subnets": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
				"description": "List of subnet IDs for the node group",
			},
			"scalingConfig": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"minSize": map[string]interface{}{
						"type":        "number",
						"description": "Minimum number of nodes",
					},
					"maxSize": map[string]interface{}{
						"type":        "number",
						"description": "Maximum number of nodes",
					},
					"desiredSize": map[string]interface{}{
						"type":        "number",
						"description": "Desired number of nodes",
					},
				},
				"description": "Scaling configuration for the node group",
			},
			"instanceTypes": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
				"description": "List of instance types for the node group",
			},
			"amiType": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"AL2_x86_64", "AL2_x86_64_GPU", "AL2_ARM_64", "CUSTOM", "BOTTLEROCKET_ARM_64", "BOTTLEROCKET_x86_64"},
				"description": "AMI type for the node group",
			},
			"capacityType": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"ON_DEMAND", "SPOT"},
				"description": "Capacity type for the node group",
			},
			"tags": map[string]interface{}{
				"type": "object",
				"additionalProperties": map[string]interface{}{
					"type": "string",
				},
				"description": "Tags to apply to the node group",
			},
		},
		"required": []string{"clusterName", "nodeGroupName", "nodeRole", "subnets"},
	}
}

func (t *CreateEKSNodeGroupTool) GetMetadata() ToolMetadata {
	return ToolMetadata{
		Name:        "create-eks-nodegroup",
		Description: "Creates a new Amazon EKS node group with specified configuration",
		Category:    "EKS",
		Tags:        []string{"eks", "kubernetes", "nodegroup", "creation"},
	}
}

// ========== Kubernetes Deployment Tools ==========

// DeployKubernetesManifestTool deploys Kubernetes manifests to an EKS cluster
type DeployKubernetesManifestTool struct {
	client     *awsclient.Client
	actionType string
	logger     *logging.Logger
}

func NewDeployKubernetesManifestTool(client *awsclient.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	return &DeployKubernetesManifestTool{
		client:     client,
		actionType: actionType,
		logger:     logger,
	}
}

func (t *DeployKubernetesManifestTool) Execute(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	t.logger.Info("Deploying Kubernetes manifest")

	// Parse arguments
	clusterName, ok := args["clusterName"].(string)
	if !ok || clusterName == "" {
		return createErrorResult("clusterName is required and must be a string"), nil
	}

	manifestContent, ok := args["manifestContent"].(string)
	if !ok || manifestContent == "" {
		return createErrorResult("manifestContent is required and must be a string"), nil
	}

	// Optional namespace
	namespace := "default"
	if ns, exists := args["namespace"]; exists {
		if nsStr, ok := ns.(string); ok && nsStr != "" {
			namespace = nsStr
		}
	}

	// Verify cluster exists and is active
	clusterResult, err := t.client.DescribeEKSCluster(ctx, clusterName)
	if err != nil {
		t.logger.WithError(err).Error("Failed to describe EKS cluster")
		return createErrorResult(fmt.Sprintf("Failed to describe EKS cluster: %v", err)), nil
	}

	if clusterResult.Cluster == nil {
		return createErrorResult("Cluster not found"), nil
	}

	if clusterResult.Cluster.Status != types.ClusterStatusActive {
		return createErrorResult(fmt.Sprintf("Cluster is not active, current status: %s", clusterResult.Cluster.Status)), nil
	}

	// Parse and validate the manifest content
	manifestInfo, err := t.parseKubernetesManifest(manifestContent)
	if err != nil {
		return createErrorResult(fmt.Sprintf("Failed to parse manifest: %v", err)), nil
	}

	// For this implementation, we'll return instructions for kubectl deployment
	// In a production environment, you would integrate with the Kubernetes API directly
	kubectlCommands := t.generateKubectlCommands(clusterName, namespace, manifestContent, manifestInfo)

	return &mcp.CallToolResult{
		Meta: map[string]interface{}{
			"cluster_name":    clusterName,
			"namespace":       namespace,
			"manifest_type":   manifestInfo["kind"],
			"manifest_name":   manifestInfo["name"],
		},
		Content: []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": fmt.Sprintf("Kubernetes manifest deployment prepared for cluster '%s'", clusterName),
			},
			map[string]interface{}{
				"type": "resource",
				"resource": map[string]interface{}{
					"uri":      fmt.Sprintf("eks://cluster/%s/deployment", clusterName),
					"mimeType": "application/json",
					"text":     mustMarshalJSON(map[string]interface{}{
						"clusterName":      clusterName,
						"namespace":        namespace,
						"manifestInfo":     manifestInfo,
						"kubectlCommands":  kubectlCommands,
						"deploymentSteps":  t.getDeploymentSteps(clusterName, namespace),
					}),
				},
			},
		},
		IsError: false,
	}, nil
}

func (t *DeployKubernetesManifestTool) parseKubernetesManifest(manifestContent string) (map[string]interface{}, error) {
	// Simple YAML parsing to extract basic information
	lines := strings.Split(manifestContent, "\n")
	manifestInfo := make(map[string]interface{})
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "kind:") {
			manifestInfo["kind"] = strings.TrimSpace(strings.TrimPrefix(line, "kind:"))
		} else if strings.HasPrefix(line, "apiVersion:") {
			manifestInfo["apiVersion"] = strings.TrimSpace(strings.TrimPrefix(line, "apiVersion:"))
		} else if strings.Contains(line, "name:") && !strings.HasPrefix(line, "#") {
			// Try to extract name (this is a simplified approach)
			if strings.Contains(line, "name:") {
				parts := strings.Split(line, "name:")
				if len(parts) > 1 {
					name := strings.TrimSpace(parts[1])
					if manifestInfo["name"] == nil && name != "" {
						manifestInfo["name"] = name
					}
				}
			}
		}
	}

	// Validate required fields
	if manifestInfo["kind"] == nil {
		return nil, fmt.Errorf("manifest must specify 'kind'")
	}
	if manifestInfo["apiVersion"] == nil {
		return nil, fmt.Errorf("manifest must specify 'apiVersion'")
	}

	return manifestInfo, nil
}

func (t *DeployKubernetesManifestTool) generateKubectlCommands(clusterName, namespace, manifestContent string, manifestInfo map[string]interface{}) []string {
	commands := []string{
		fmt.Sprintf("# Update kubeconfig for EKS cluster"),
		fmt.Sprintf("aws eks update-kubeconfig --region %s --name %s", t.client.GetRegion(), clusterName),
		"",
		fmt.Sprintf("# Create namespace if it doesn't exist (optional)"),
		fmt.Sprintf("kubectl create namespace %s --dry-run=client -o yaml | kubectl apply -f -", namespace),
		"",
		fmt.Sprintf("# Apply the manifest"),
		fmt.Sprintf("kubectl apply -f - <<EOF"),
		manifestContent,
		"EOF",
		"",
		fmt.Sprintf("# Verify deployment"),
	}

	// Add verification commands based on manifest type
	if kind, ok := manifestInfo["kind"].(string); ok {
		switch strings.ToLower(kind) {
		case "deployment":
			commands = append(commands, fmt.Sprintf("kubectl get deployments -n %s", namespace))
			commands = append(commands, fmt.Sprintf("kubectl rollout status deployment/%s -n %s", manifestInfo["name"], namespace))
		case "service":
			commands = append(commands, fmt.Sprintf("kubectl get services -n %s", namespace))
		case "pod":
			commands = append(commands, fmt.Sprintf("kubectl get pods -n %s", namespace))
		default:
			commands = append(commands, fmt.Sprintf("kubectl get %s -n %s", strings.ToLower(kind), namespace))
		}
	}

	return commands
}

func (t *DeployKubernetesManifestTool) getDeploymentSteps(clusterName, namespace string) []map[string]interface{} {
	return []map[string]interface{}{
		{
			"step":        1,
			"description": "Configure kubectl for EKS cluster",
			"command":     fmt.Sprintf("aws eks update-kubeconfig --region %s --name %s", t.client.GetRegion(), clusterName),
			"required":    true,
		},
		{
			"step":        2,
			"description": "Verify cluster connectivity",
			"command":     "kubectl cluster-info",
			"required":    true,
		},
		{
			"step":        3,
			"description": "Create or verify namespace",
			"command":     fmt.Sprintf("kubectl create namespace %s --dry-run=client -o yaml | kubectl apply -f -", namespace),
			"required":    false,
		},
		{
			"step":        4,
			"description": "Apply Kubernetes manifest",
			"command":     "kubectl apply -f <manifest-file>",
			"required":    true,
		},
		{
			"step":        5,
			"description": "Verify deployment status",
			"command":     fmt.Sprintf("kubectl get all -n %s", namespace),
			"required":    false,
		},
	}
}

func (t *DeployKubernetesManifestTool) GetSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"clusterName": map[string]interface{}{
				"type":        "string",
				"description": "Name of the EKS cluster to deploy to",
			},
			"manifestContent": map[string]interface{}{
				"type":        "string",
				"description": "YAML content of the Kubernetes manifest to deploy",
			},
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Kubernetes namespace to deploy to (defaults to 'default')",
				"default":     "default",
			},
		},
		"required": []string{"clusterName", "manifestContent"},
	}
}

func (t *DeployKubernetesManifestTool) GetMetadata() ToolMetadata {
	return ToolMetadata{
		Name:        "deploy-kubernetes-manifest",
		Description: "Deploys a Kubernetes manifest (deployment.yaml) to an Amazon EKS cluster",
		Category:    "EKS",
		Tags:        []string{"eks", "kubernetes", "deployment", "manifest", "yaml"},
	}
}

// ========== IAM Policy Management Tools ==========

// AddInlinePolicyTool adds an inline policy to an IAM role
type AddInlinePolicyTool struct {
	client     *awsclient.Client
	actionType string
	logger     *logging.Logger
}

func NewAddInlinePolicyTool(client *awsclient.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	return &AddInlinePolicyTool{
		client:     client,
		actionType: actionType,
		logger:     logger,
	}
}

func (t *AddInlinePolicyTool) Execute(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	t.logger.Info("Adding inline policy to IAM role")

	// Parse arguments
	roleName, ok := args["roleName"].(string)
	if !ok || roleName == "" {
		return createErrorResult("roleName is required and must be a string"), nil
	}

	policyName, ok := args["policyName"].(string)
	if !ok || policyName == "" {
		return createErrorResult("policyName is required and must be a string"), nil
	}

	policyDocument, ok := args["policyDocument"].(string)
	if !ok || policyDocument == "" {
		return createErrorResult("policyDocument is required and must be a string"), nil
	}

	// This would require IAM client integration - for now return instructions
	return &mcp.CallToolResult{
		Meta: map[string]interface{}{
			"role_name":    roleName,
			"policy_name":  policyName,
		},
		Content: []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": fmt.Sprintf("To add inline policy '%s' to role '%s', use AWS CLI:", policyName, roleName),
			},
			map[string]interface{}{
				"type": "resource",
				"resource": map[string]interface{}{
					"uri":      fmt.Sprintf("iam://role/%s/policy/%s", roleName, policyName),
					"mimeType": "application/json",
					"text":     mustMarshalJSON(map[string]interface{}{
						"command": fmt.Sprintf("aws iam put-role-policy --role-name %s --policy-name %s --policy-document '%s'", roleName, policyName, policyDocument),
						"roleName": roleName,
						"policyName": policyName,
						"policyDocument": policyDocument,
					}),
				},
			},
		},
		IsError: false,
	}, nil
}

func (t *AddInlinePolicyTool) GetSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"roleName": map[string]interface{}{
				"type":        "string",
				"description": "Name of the IAM role",
			},
			"policyName": map[string]interface{}{
				"type":        "string",
				"description": "Name of the inline policy",
			},
			"policyDocument": map[string]interface{}{
				"type":        "string",
				"description": "JSON policy document",
			},
		},
		"required": []string{"roleName", "policyName", "policyDocument"},
	}
}

func (t *AddInlinePolicyTool) GetMetadata() ToolMetadata {
	return ToolMetadata{
		Name:        "add-inline-policy",
		Description: "Adds an inline policy to an IAM role",
		Category:    "IAM",
		Tags:        []string{"iam", "policy", "role", "security"},
	}
}

// GetPoliciesForRoleTool lists policies attached to an IAM role
type GetPoliciesForRoleTool struct {
	client     *awsclient.Client
	actionType string
	logger     *logging.Logger
}

func NewGetPoliciesForRoleTool(client *awsclient.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	return &GetPoliciesForRoleTool{
		client:     client,
		actionType: actionType,
		logger:     logger,
	}
}

func (t *GetPoliciesForRoleTool) Execute(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	t.logger.Info("Getting policies for IAM role")

	// Parse arguments
	roleName, ok := args["roleName"].(string)
	if !ok || roleName == "" {
		return createErrorResult("roleName is required and must be a string"), nil
	}

	// This would require IAM client integration - for now return instructions
	return &mcp.CallToolResult{
		Meta: map[string]interface{}{
			"role_name": roleName,
		},
		Content: []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": fmt.Sprintf("To list policies for role '%s', use AWS CLI:", roleName),
			},
			map[string]interface{}{
				"type": "resource",
				"resource": map[string]interface{}{
					"uri":      fmt.Sprintf("iam://role/%s/policies", roleName),
					"mimeType": "application/json",
					"text":     mustMarshalJSON(map[string]interface{}{
						"attachedPoliciesCommand": fmt.Sprintf("aws iam list-attached-role-policies --role-name %s", roleName),
						"inlinePoliciesCommand": fmt.Sprintf("aws iam list-role-policies --role-name %s", roleName),
						"roleName": roleName,
					}),
				},
			},
		},
		IsError: false,
	}, nil
}

func (t *GetPoliciesForRoleTool) GetSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"roleName": map[string]interface{}{
				"type":        "string",
				"description": "Name of the IAM role",
			},
		},
		"required": []string{"roleName"},
	}
}

func (t *GetPoliciesForRoleTool) GetMetadata() ToolMetadata {
	return ToolMetadata{
		Name:        "get-policies-for-role",
		Description: "Lists all policies attached to an IAM role",
		Category:    "IAM",
		Tags:        []string{"iam", "policy", "role", "list"},
	}
}

// ========== Kubernetes Resource Management Tools ==========

// ListK8sResourcesTool lists Kubernetes resources in a cluster
type ListK8sResourcesTool struct {
	client     *awsclient.Client
	actionType string
	logger     *logging.Logger
}

func NewListK8sResourcesTool(client *awsclient.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	return &ListK8sResourcesTool{
		client:     client,
		actionType: actionType,
		logger:     logger,
	}
}

func (t *ListK8sResourcesTool) Execute(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	t.logger.Info("Listing Kubernetes resources")

	// Parse arguments
	clusterName, ok := args["clusterName"].(string)
	if !ok || clusterName == "" {
		return createErrorResult("clusterName is required and must be a string"), nil
	}

	resourceType := "all"
	if rt, exists := args["resourceType"]; exists {
		if rtStr, ok := rt.(string); ok {
			resourceType = rtStr
		}
	}

	namespace := "default"
	if ns, exists := args["namespace"]; exists {
		if nsStr, ok := ns.(string); ok && nsStr != "" {
			namespace = nsStr
		}
	}

	// Verify cluster exists
	_, err := t.client.DescribeEKSCluster(ctx, clusterName)
	if err != nil {
		return createErrorResult(fmt.Sprintf("Failed to verify cluster: %v", err)), nil
	}

	kubectlCommands := []string{
		fmt.Sprintf("# Update kubeconfig for cluster %s", clusterName),
		fmt.Sprintf("aws eks update-kubeconfig --region %s --name %s", t.client.GetRegion(), clusterName),
		"",
		fmt.Sprintf("# List %s resources in namespace %s", resourceType, namespace),
	}

	if resourceType == "all" {
		kubectlCommands = append(kubectlCommands, fmt.Sprintf("kubectl get all -n %s", namespace))
	} else {
		kubectlCommands = append(kubectlCommands, fmt.Sprintf("kubectl get %s -n %s", resourceType, namespace))
	}

	return &mcp.CallToolResult{
		Meta: map[string]interface{}{
			"cluster_name":  clusterName,
			"resource_type": resourceType,
			"namespace":     namespace,
		},
		Content: []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": fmt.Sprintf("Kubernetes resource listing prepared for cluster '%s'", clusterName),
			},
			map[string]interface{}{
				"type": "resource",
				"resource": map[string]interface{}{
					"uri":      fmt.Sprintf("k8s://cluster/%s/resources", clusterName),
					"mimeType": "application/json",
					"text":     mustMarshalJSON(map[string]interface{}{
						"clusterName":      clusterName,
						"resourceType":     resourceType,
						"namespace":        namespace,
						"kubectlCommands":  kubectlCommands,
					}),
				},
			},
		},
		IsError: false,
	}, nil
}

func (t *ListK8sResourcesTool) GetSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"clusterName": map[string]interface{}{
				"type":        "string",
				"description": "Name of the EKS cluster",
			},
			"resourceType": map[string]interface{}{
				"type":        "string",
				"description": "Type of Kubernetes resource to list (e.g., pods, services, deployments, all)",
				"default":     "all",
			},
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Kubernetes namespace",
				"default":     "default",
			},
		},
		"required": []string{"clusterName"},
	}
}

func (t *ListK8sResourcesTool) GetMetadata() ToolMetadata {
	return ToolMetadata{
		Name:        "list-k8s-resources",
		Description: "Lists Kubernetes resources in an EKS cluster",
		Category:    "Kubernetes",
		Tags:        []string{"kubernetes", "k8s", "resources", "list"},
	}
}

// ListApiVersionsTool lists available Kubernetes API versions
type ListApiVersionsTool struct {
	client     *awsclient.Client
	actionType string
	logger     *logging.Logger
}

func NewListApiVersionsTool(client *awsclient.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	return &ListApiVersionsTool{
		client:     client,
		actionType: actionType,
		logger:     logger,
	}
}

func (t *ListApiVersionsTool) Execute(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	t.logger.Info("Listing Kubernetes API versions")

	// Parse arguments
	clusterName, ok := args["clusterName"].(string)
	if !ok || clusterName == "" {
		return createErrorResult("clusterName is required and must be a string"), nil
	}

	// Verify cluster exists
	_, err := t.client.DescribeEKSCluster(ctx, clusterName)
	if err != nil {
		return createErrorResult(fmt.Sprintf("Failed to verify cluster: %v", err)), nil
	}

	kubectlCommands := []string{
		fmt.Sprintf("# Update kubeconfig for cluster %s", clusterName),
		fmt.Sprintf("aws eks update-kubeconfig --region %s --name %s", t.client.GetRegion(), clusterName),
		"",
		"# List API versions",
		"kubectl api-versions",
		"",
		"# List API resources",
		"kubectl api-resources",
	}

	return &mcp.CallToolResult{
		Meta: map[string]interface{}{
			"cluster_name": clusterName,
		},
		Content: []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": fmt.Sprintf("Kubernetes API versions listing prepared for cluster '%s'", clusterName),
			},
			map[string]interface{}{
				"type": "resource",
				"resource": map[string]interface{}{
					"uri":      fmt.Sprintf("k8s://cluster/%s/api-versions", clusterName),
					"mimeType": "application/json",
					"text":     mustMarshalJSON(map[string]interface{}{
						"clusterName":     clusterName,
						"kubectlCommands": kubectlCommands,
					}),
				},
			},
		},
		IsError: false,
	}, nil
}

func (t *ListApiVersionsTool) GetSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"clusterName": map[string]interface{}{
				"type":        "string",
				"description": "Name of the EKS cluster",
			},
		},
		"required": []string{"clusterName"},
	}
}

func (t *ListApiVersionsTool) GetMetadata() ToolMetadata {
	return ToolMetadata{
		Name:        "list-api-versions",
		Description: "Lists available Kubernetes API versions and resources",
		Category:    "Kubernetes",
		Tags:        []string{"kubernetes", "k8s", "api", "versions"},
	}
}

// ManageK8sResourceTool manages Kubernetes resources (create, update, delete)
type ManageK8sResourceTool struct {
	client     *awsclient.Client
	actionType string
	logger     *logging.Logger
}

func NewManageK8sResourceTool(client *awsclient.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	return &ManageK8sResourceTool{
		client:     client,
		actionType: actionType,
		logger:     logger,
	}
}

func (t *ManageK8sResourceTool) Execute(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	t.logger.Info("Managing Kubernetes resource")

	// Parse arguments
	clusterName, ok := args["clusterName"].(string)
	if !ok || clusterName == "" {
		return createErrorResult("clusterName is required and must be a string"), nil
	}

	action, ok := args["action"].(string)
	if !ok || action == "" {
		return createErrorResult("action is required and must be a string (create, update, delete, get)"), nil
	}

	resourceType, ok := args["resourceType"].(string)
	if !ok || resourceType == "" {
		return createErrorResult("resourceType is required and must be a string"), nil
	}

	resourceName, ok := args["resourceName"].(string)
	if !ok || resourceName == "" {
		return createErrorResult("resourceName is required and must be a string"), nil
	}

	namespace := "default"
	if ns, exists := args["namespace"]; exists {
		if nsStr, ok := ns.(string); ok && nsStr != "" {
			namespace = nsStr
		}
	}

	// Verify cluster exists
	_, err := t.client.DescribeEKSCluster(ctx, clusterName)
	if err != nil {
		return createErrorResult(fmt.Sprintf("Failed to verify cluster: %v", err)), nil
	}

	var kubectlCommands []string
	switch action {
	case "create":
		kubectlCommands = []string{
			fmt.Sprintf("# Update kubeconfig for cluster %s", clusterName),
			fmt.Sprintf("aws eks update-kubeconfig --region %s --name %s", t.client.GetRegion(), clusterName),
			"",
			fmt.Sprintf("# Create %s resource", resourceType),
			fmt.Sprintf("kubectl create %s %s -n %s", resourceType, resourceName, namespace),
		}
	case "update":
		kubectlCommands = []string{
			fmt.Sprintf("# Update kubeconfig for cluster %s", clusterName),
			fmt.Sprintf("aws eks update-kubeconfig --region %s --name %s", t.client.GetRegion(), clusterName),
			"",
			fmt.Sprintf("# Update %s resource", resourceType),
			fmt.Sprintf("kubectl patch %s %s -n %s --patch '{}'", resourceType, resourceName, namespace),
		}
	case "delete":
		kubectlCommands = []string{
			fmt.Sprintf("# Update kubeconfig for cluster %s", clusterName),
			fmt.Sprintf("aws eks update-kubeconfig --region %s --name %s", t.client.GetRegion(), clusterName),
			"",
			fmt.Sprintf("# Delete %s resource", resourceType),
			fmt.Sprintf("kubectl delete %s %s -n %s", resourceType, resourceName, namespace),
		}
	case "get":
		kubectlCommands = []string{
			fmt.Sprintf("# Update kubeconfig for cluster %s", clusterName),
			fmt.Sprintf("aws eks update-kubeconfig --region %s --name %s", t.client.GetRegion(), clusterName),
			"",
			fmt.Sprintf("# Get %s resource", resourceType),
			fmt.Sprintf("kubectl get %s %s -n %s -o yaml", resourceType, resourceName, namespace),
		}
	default:
		return createErrorResult("Invalid action. Must be one of: create, update, delete, get"), nil
	}

	return &mcp.CallToolResult{
		Meta: map[string]interface{}{
			"cluster_name":  clusterName,
			"action":        action,
			"resource_type": resourceType,
			"resource_name": resourceName,
			"namespace":     namespace,
		},
		Content: []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": fmt.Sprintf("Kubernetes resource management prepared for %s %s/%s in cluster '%s'", action, resourceType, resourceName, clusterName),
			},
			map[string]interface{}{
				"type": "resource",
				"resource": map[string]interface{}{
					"uri":      fmt.Sprintf("k8s://cluster/%s/resource/%s/%s", clusterName, resourceType, resourceName),
					"mimeType": "application/json",
					"text":     mustMarshalJSON(map[string]interface{}{
						"clusterName":     clusterName,
						"action":          action,
						"resourceType":    resourceType,
						"resourceName":    resourceName,
						"namespace":       namespace,
						"kubectlCommands": kubectlCommands,
					}),
				},
			},
		},
		IsError: false,
	}, nil
}

func (t *ManageK8sResourceTool) GetSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"clusterName": map[string]interface{}{
				"type":        "string",
				"description": "Name of the EKS cluster",
			},
			"action": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"create", "update", "delete", "get"},
				"description": "Action to perform on the resource",
			},
			"resourceType": map[string]interface{}{
				"type":        "string",
				"description": "Type of Kubernetes resource (e.g., deployment, service, pod)",
			},
			"resourceName": map[string]interface{}{
				"type":        "string",
				"description": "Name of the resource",
			},
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Kubernetes namespace",
				"default":     "default",
			},
		},
		"required": []string{"clusterName", "action", "resourceType", "resourceName"},
	}
}

func (t *ManageK8sResourceTool) GetMetadata() ToolMetadata {
	return ToolMetadata{
		Name:        "manage-k8s-resource",
		Description: "Manages Kubernetes resources (create, update, delete, get)",
		Category:    "Kubernetes",
		Tags:        []string{"kubernetes", "k8s", "resource", "management"},
	}
}

// ApplyYamlTool applies YAML manifests to a Kubernetes cluster
type ApplyYamlTool struct {
	client     *awsclient.Client
	actionType string
	logger     *logging.Logger
}

func NewApplyYamlTool(client *awsclient.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	return &ApplyYamlTool{
		client:     client,
		actionType: actionType,
		logger:     logger,
	}
}

func (t *ApplyYamlTool) Execute(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	t.logger.Info("Applying YAML to Kubernetes cluster")

	// Parse arguments
	clusterName, ok := args["clusterName"].(string)
	if !ok || clusterName == "" {
		return createErrorResult("clusterName is required and must be a string"), nil
	}

	yamlContent, ok := args["yamlContent"].(string)
	if !ok || yamlContent == "" {
		return createErrorResult("yamlContent is required and must be a string"), nil
	}

	namespace := "default"
	if ns, exists := args["namespace"]; exists {
		if nsStr, ok := ns.(string); ok && nsStr != "" {
			namespace = nsStr
		}
	}

	// Verify cluster exists
	_, err := t.client.DescribeEKSCluster(ctx, clusterName)
	if err != nil {
		return createErrorResult(fmt.Sprintf("Failed to verify cluster: %v", err)), nil
	}

	kubectlCommands := []string{
		fmt.Sprintf("# Update kubeconfig for cluster %s", clusterName),
		fmt.Sprintf("aws eks update-kubeconfig --region %s --name %s", t.client.GetRegion(), clusterName),
		"",
		fmt.Sprintf("# Apply YAML to namespace %s", namespace),
		"kubectl apply -f - <<EOF",
		yamlContent,
		"EOF",
		"",
		"# Verify applied resources",
		fmt.Sprintf("kubectl get all -n %s", namespace),
	}

	return &mcp.CallToolResult{
		Meta: map[string]interface{}{
			"cluster_name": clusterName,
			"namespace":    namespace,
		},
		Content: []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": fmt.Sprintf("YAML application prepared for cluster '%s'", clusterName),
			},
			map[string]interface{}{
				"type": "resource",
				"resource": map[string]interface{}{
					"uri":      fmt.Sprintf("k8s://cluster/%s/apply", clusterName),
					"mimeType": "application/json",
					"text":     mustMarshalJSON(map[string]interface{}{
						"clusterName":     clusterName,
						"namespace":       namespace,
						"yamlContent":     yamlContent,
						"kubectlCommands": kubectlCommands,
					}),
				},
			},
		},
		IsError: false,
	}, nil
}

func (t *ApplyYamlTool) GetSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"clusterName": map[string]interface{}{
				"type":        "string",
				"description": "Name of the EKS cluster",
			},
			"yamlContent": map[string]interface{}{
				"type":        "string",
				"description": "YAML content to apply",
			},
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Kubernetes namespace",
				"default":     "default",
			},
		},
		"required": []string{"clusterName", "yamlContent"},
	}
}

func (t *ApplyYamlTool) GetMetadata() ToolMetadata {
	return ToolMetadata{
		Name:        "apply-yaml",
		Description: "Applies YAML manifests to a Kubernetes cluster",
		Category:    "Kubernetes",
		Tags:        []string{"kubernetes", "k8s", "yaml", "apply"},
	}
}

// ========== Kubernetes Monitoring and Logging Tools ==========

// GetK8sEventsTool gets Kubernetes events from a cluster
type GetK8sEventsTool struct {
	client     *awsclient.Client
	actionType string
	logger     *logging.Logger
}

func NewGetK8sEventsTool(client *awsclient.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	return &GetK8sEventsTool{
		client:     client,
		actionType: actionType,
		logger:     logger,
	}
}

func (t *GetK8sEventsTool) Execute(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	t.logger.Info("Getting Kubernetes events")

	// Parse arguments
	clusterName, ok := args["clusterName"].(string)
	if !ok || clusterName == "" {
		return createErrorResult("clusterName is required and must be a string"), nil
	}

	namespace := "default"
	if ns, exists := args["namespace"]; exists {
		if nsStr, ok := ns.(string); ok && nsStr != "" {
			namespace = nsStr
		}
	}

	// Verify cluster exists
	_, err := t.client.DescribeEKSCluster(ctx, clusterName)
	if err != nil {
		return createErrorResult(fmt.Sprintf("Failed to verify cluster: %v", err)), nil
	}

	kubectlCommands := []string{
		fmt.Sprintf("# Update kubeconfig for cluster %s", clusterName),
		fmt.Sprintf("aws eks update-kubeconfig --region %s --name %s", t.client.GetRegion(), clusterName),
		"",
		fmt.Sprintf("# Get events in namespace %s", namespace),
		fmt.Sprintf("kubectl get events -n %s --sort-by='.lastTimestamp'", namespace),
		"",
		"# Get events across all namespaces",
		"kubectl get events --all-namespaces --sort-by='.lastTimestamp'",
	}

	return &mcp.CallToolResult{
		Meta: map[string]interface{}{
			"cluster_name": clusterName,
			"namespace":    namespace,
		},
		Content: []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": fmt.Sprintf("Kubernetes events retrieval prepared for cluster '%s'", clusterName),
			},
			map[string]interface{}{
				"type": "resource",
				"resource": map[string]interface{}{
					"uri":      fmt.Sprintf("k8s://cluster/%s/events", clusterName),
					"mimeType": "application/json",
					"text":     mustMarshalJSON(map[string]interface{}{
						"clusterName":     clusterName,
						"namespace":       namespace,
						"kubectlCommands": kubectlCommands,
					}),
				},
			},
		},
		IsError: false,
	}, nil
}

func (t *GetK8sEventsTool) GetSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"clusterName": map[string]interface{}{
				"type":        "string",
				"description": "Name of the EKS cluster",
			},
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Kubernetes namespace (optional, defaults to 'default')",
				"default":     "default",
			},
		},
		"required": []string{"clusterName"},
	}
}

func (t *GetK8sEventsTool) GetMetadata() ToolMetadata {
	return ToolMetadata{
		Name:        "get-k8s-events",
		Description: "Gets Kubernetes events from an EKS cluster",
		Category:    "Kubernetes",
		Tags:        []string{"kubernetes", "k8s", "events", "monitoring"},
	}
}

// GetPodLogsTool gets logs from Kubernetes pods
type GetPodLogsTool struct {
	client     *awsclient.Client
	actionType string
	logger     *logging.Logger
}

func NewGetPodLogsTool(client *awsclient.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	return &GetPodLogsTool{
		client:     client,
		actionType: actionType,
		logger:     logger,
	}
}

func (t *GetPodLogsTool) Execute(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	t.logger.Info("Getting pod logs")

	// Parse arguments
	clusterName, ok := args["clusterName"].(string)
	if !ok || clusterName == "" {
		return createErrorResult("clusterName is required and must be a string"), nil
	}

	podName, ok := args["podName"].(string)
	if !ok || podName == "" {
		return createErrorResult("podName is required and must be a string"), nil
	}

	namespace := "default"
	if ns, exists := args["namespace"]; exists {
		if nsStr, ok := ns.(string); ok && nsStr != "" {
			namespace = nsStr
		}
	}

	// Optional container name
	containerName := ""
	if cn, exists := args["containerName"]; exists {
		if cnStr, ok := cn.(string); ok {
			containerName = cnStr
		}
	}

	// Optional tail lines
	tailLines := "100"
	if tl, exists := args["tailLines"]; exists {
		if tlInt, ok := tl.(float64); ok {
			tailLines = fmt.Sprintf("%.0f", tlInt)
		}
	}

	// Verify cluster exists
	_, err := t.client.DescribeEKSCluster(ctx, clusterName)
	if err != nil {
		return createErrorResult(fmt.Sprintf("Failed to verify cluster: %v", err)), nil
	}

	kubectlCommands := []string{
		fmt.Sprintf("# Update kubeconfig for cluster %s", clusterName),
		fmt.Sprintf("aws eks update-kubeconfig --region %s --name %s", t.client.GetRegion(), clusterName),
		"",
		fmt.Sprintf("# Get logs for pod %s", podName),
	}

	if containerName != "" {
		kubectlCommands = append(kubectlCommands, fmt.Sprintf("kubectl logs %s -c %s -n %s --tail=%s", podName, containerName, namespace, tailLines))
	} else {
		kubectlCommands = append(kubectlCommands, fmt.Sprintf("kubectl logs %s -n %s --tail=%s", podName, namespace, tailLines))
	}

	kubectlCommands = append(kubectlCommands, []string{
		"",
		"# Follow logs (optional)",
		fmt.Sprintf("kubectl logs %s -n %s -f", podName, namespace),
	}...)

	return &mcp.CallToolResult{
		Meta: map[string]interface{}{
			"cluster_name":   clusterName,
			"pod_name":       podName,
			"namespace":      namespace,
			"container_name": containerName,
		},
		Content: []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": fmt.Sprintf("Pod logs retrieval prepared for pod '%s' in cluster '%s'", podName, clusterName),
			},
			map[string]interface{}{
				"type": "resource",
				"resource": map[string]interface{}{
					"uri":      fmt.Sprintf("k8s://cluster/%s/pod/%s/logs", clusterName, podName),
					"mimeType": "application/json",
					"text":     mustMarshalJSON(map[string]interface{}{
						"clusterName":     clusterName,
						"podName":         podName,
						"namespace":       namespace,
						"containerName":   containerName,
						"tailLines":       tailLines,
						"kubectlCommands": kubectlCommands,
					}),
				},
			},
		},
		IsError: false,
	}, nil
}

func (t *GetPodLogsTool) GetSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"clusterName": map[string]interface{}{
				"type":        "string",
				"description": "Name of the EKS cluster",
			},
			"podName": map[string]interface{}{
				"type":        "string",
				"description": "Name of the pod",
			},
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Kubernetes namespace",
				"default":     "default",
			},
			"containerName": map[string]interface{}{
				"type":        "string",
				"description": "Name of the container (optional for single-container pods)",
			},
			"tailLines": map[string]interface{}{
				"type":        "number",
				"description": "Number of lines to tail from the end of the logs",
				"default":     100,
			},
		},
		"required": []string{"clusterName", "podName"},
	}
}

func (t *GetPodLogsTool) GetMetadata() ToolMetadata {
	return ToolMetadata{
		Name:        "get-pod-logs",
		Description: "Gets logs from Kubernetes pods in an EKS cluster",
		Category:    "Kubernetes",
		Tags:        []string{"kubernetes", "k8s", "logs", "pods", "monitoring"},
	}
}

// ========== CloudWatch Integration Tools ==========

// GetCloudWatchLogsTool gets CloudWatch logs for EKS
type GetCloudWatchLogsTool struct {
	client     *awsclient.Client
	actionType string
	logger     *logging.Logger
}

func NewGetCloudWatchLogsTool(client *awsclient.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	return &GetCloudWatchLogsTool{
		client:     client,
		actionType: actionType,
		logger:     logger,
	}
}

func (t *GetCloudWatchLogsTool) Execute(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	t.logger.Info("Getting CloudWatch logs")

	// Parse arguments
	logGroupName, ok := args["logGroupName"].(string)
	if !ok || logGroupName == "" {
		return createErrorResult("logGroupName is required and must be a string"), nil
	}

	// Optional parameters
	startTime := ""
	if st, exists := args["startTime"]; exists {
		if stStr, ok := st.(string); ok {
			startTime = stStr
		}
	}

	endTime := ""
	if et, exists := args["endTime"]; exists {
		if etStr, ok := et.(string); ok {
			endTime = etStr
		}
	}

	filterPattern := ""
	if fp, exists := args["filterPattern"]; exists {
		if fpStr, ok := fp.(string); ok {
			filterPattern = fpStr
		}
	}

	// Build AWS CLI commands for CloudWatch logs
	awsCommands := []string{
		fmt.Sprintf("# Get CloudWatch logs for log group: %s", logGroupName),
		fmt.Sprintf("aws logs describe-log-streams --log-group-name %s", logGroupName),
		"",
		"# Get log events",
	}

	logCommand := fmt.Sprintf("aws logs filter-log-events --log-group-name %s", logGroupName)
	if startTime != "" {
		logCommand += fmt.Sprintf(" --start-time %s", startTime)
	}
	if endTime != "" {
		logCommand += fmt.Sprintf(" --end-time %s", endTime)
	}
	if filterPattern != "" {
		logCommand += fmt.Sprintf(" --filter-pattern '%s'", filterPattern)
	}

	awsCommands = append(awsCommands, logCommand)

	return &mcp.CallToolResult{
		Meta: map[string]interface{}{
			"log_group_name": logGroupName,
		},
		Content: []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": fmt.Sprintf("CloudWatch logs retrieval prepared for log group '%s'", logGroupName),
			},
			map[string]interface{}{
				"type": "resource",
				"resource": map[string]interface{}{
					"uri":      fmt.Sprintf("cloudwatch://logs/%s", logGroupName),
					"mimeType": "application/json",
					"text":     mustMarshalJSON(map[string]interface{}{
						"logGroupName":  logGroupName,
						"startTime":     startTime,
						"endTime":       endTime,
						"filterPattern": filterPattern,
						"awsCommands":   awsCommands,
					}),
				},
			},
		},
		IsError: false,
	}, nil
}

func (t *GetCloudWatchLogsTool) GetSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"logGroupName": map[string]interface{}{
				"type":        "string",
				"description": "Name of the CloudWatch log group",
			},
			"startTime": map[string]interface{}{
				"type":        "string",
				"description": "Start time for log retrieval (Unix timestamp or ISO 8601)",
			},
			"endTime": map[string]interface{}{
				"type":        "string",
				"description": "End time for log retrieval (Unix timestamp or ISO 8601)",
			},
			"filterPattern": map[string]interface{}{
				"type":        "string",
				"description": "Filter pattern for log events",
			},
		},
		"required": []string{"logGroupName"},
	}
}

func (t *GetCloudWatchLogsTool) GetMetadata() ToolMetadata {
	return ToolMetadata{
		Name:        "get-cloudwatch-logs",
		Description: "Gets CloudWatch logs for EKS clusters and applications",
		Category:    "Monitoring",
		Tags:        []string{"cloudwatch", "logs", "monitoring", "eks"},
	}
}

// GetCloudWatchMetricsTool gets CloudWatch metrics for EKS
type GetCloudWatchMetricsTool struct {
	client     *awsclient.Client
	actionType string
	logger     *logging.Logger
}

func NewGetCloudWatchMetricsTool(client *awsclient.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	return &GetCloudWatchMetricsTool{
		client:     client,
		actionType: actionType,
		logger:     logger,
	}
}

func (t *GetCloudWatchMetricsTool) Execute(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	t.logger.Info("Getting CloudWatch metrics")

	// Parse arguments
	namespace, ok := args["namespace"].(string)
	if !ok || namespace == "" {
		return createErrorResult("namespace is required and must be a string"), nil
	}

	metricName, ok := args["metricName"].(string)
	if !ok || metricName == "" {
		return createErrorResult("metricName is required and must be a string"), nil
	}

	// Optional parameters
	startTime := ""
	if st, exists := args["startTime"]; exists {
		if stStr, ok := st.(string); ok {
			startTime = stStr
		}
	}

	endTime := ""
	if et, exists := args["endTime"]; exists {
		if etStr, ok := et.(string); ok {
			endTime = etStr
		}
	}

	period := "300"
	if p, exists := args["period"]; exists {
		if pInt, ok := p.(float64); ok {
			period = fmt.Sprintf("%.0f", pInt)
		}
	}

	statistic := "Average"
	if s, exists := args["statistic"]; exists {
		if sStr, ok := s.(string); ok {
			statistic = sStr
		}
	}

	// Build AWS CLI commands for CloudWatch metrics
	awsCommands := []string{
		fmt.Sprintf("# Get CloudWatch metrics for namespace: %s", namespace),
		fmt.Sprintf("aws cloudwatch list-metrics --namespace %s", namespace),
		"",
		fmt.Sprintf("# Get metric statistics for %s", metricName),
	}

	metricsCommand := fmt.Sprintf("aws cloudwatch get-metric-statistics --namespace %s --metric-name %s --statistic %s --period %s", namespace, metricName, statistic, period)
	if startTime != "" {
		metricsCommand += fmt.Sprintf(" --start-time %s", startTime)
	}
	if endTime != "" {
		metricsCommand += fmt.Sprintf(" --end-time %s", endTime)
	}

	awsCommands = append(awsCommands, metricsCommand)

	return &mcp.CallToolResult{
		Meta: map[string]interface{}{
			"namespace":   namespace,
			"metric_name": metricName,
		},
		Content: []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": fmt.Sprintf("CloudWatch metrics retrieval prepared for %s/%s", namespace, metricName),
			},
			map[string]interface{}{
				"type": "resource",
				"resource": map[string]interface{}{
					"uri":      fmt.Sprintf("cloudwatch://metrics/%s/%s", namespace, metricName),
					"mimeType": "application/json",
					"text":     mustMarshalJSON(map[string]interface{}{
						"namespace":   namespace,
						"metricName":  metricName,
						"startTime":   startTime,
						"endTime":     endTime,
						"period":      period,
						"statistic":   statistic,
						"awsCommands": awsCommands,
					}),
				},
			},
		},
		IsError: false,
	}, nil
}

func (t *GetCloudWatchMetricsTool) GetSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "CloudWatch metrics namespace (e.g., AWS/EKS, AWS/ContainerInsights)",
			},
			"metricName": map[string]interface{}{
				"type":        "string",
				"description": "Name of the metric to retrieve",
			},
			"startTime": map[string]interface{}{
				"type":        "string",
				"description": "Start time for metrics (ISO 8601 format)",
			},
			"endTime": map[string]interface{}{
				"type":        "string",
				"description": "End time for metrics (ISO 8601 format)",
			},
			"period": map[string]interface{}{
				"type":        "number",
				"description": "Period in seconds for metric aggregation",
				"default":     300,
			},
			"statistic": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"Average", "Sum", "Maximum", "Minimum", "SampleCount"},
				"description": "Statistic to retrieve",
				"default":     "Average",
			},
		},
		"required": []string{"namespace", "metricName"},
	}
}

func (t *GetCloudWatchMetricsTool) GetMetadata() ToolMetadata {
	return ToolMetadata{
		Name:        "get-cloudwatch-metrics",
		Description: "Gets CloudWatch metrics for EKS clusters and applications",
		Category:    "Monitoring",
		Tags:        []string{"cloudwatch", "metrics", "monitoring", "eks"},
	}
}

// ========== EKS Troubleshooting and Insights Tools ==========

// SearchEKSTroubleshootGuideTool searches EKS troubleshooting guide
type SearchEKSTroubleshootGuideTool struct {
	client     *awsclient.Client
	actionType string
	logger     *logging.Logger
}

func NewSearchEKSTroubleshootGuideTool(client *awsclient.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	return &SearchEKSTroubleshootGuideTool{
		client:     client,
		actionType: actionType,
		logger:     logger,
	}
}

func (t *SearchEKSTroubleshootGuideTool) Execute(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	t.logger.Info("Searching EKS troubleshooting guide")

	// Parse arguments
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return createErrorResult("query is required and must be a string"), nil
	}

	// Optional category filter
	category := ""
	if c, exists := args["category"]; exists {
		if cStr, ok := c.(string); ok {
			category = cStr
		}
	}

	// Simulate troubleshooting guide search results
	troubleshootingGuide := t.getTroubleshootingGuide(query, category)

	return &mcp.CallToolResult{
		Meta: map[string]interface{}{
			"query":    query,
			"category": category,
		},
		Content: []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": fmt.Sprintf("EKS troubleshooting guide search results for: '%s'", query),
			},
			map[string]interface{}{
				"type": "resource",
				"resource": map[string]interface{}{
					"uri":      fmt.Sprintf("eks://troubleshoot/%s", query),
					"mimeType": "application/json",
					"text":     mustMarshalJSON(troubleshootingGuide),
				},
			},
		},
		IsError: false,
	}, nil
}

func (t *SearchEKSTroubleshootGuideTool) getTroubleshootingGuide(query, category string) map[string]interface{} {
	// Common EKS troubleshooting scenarios
	troubleshootingScenarios := map[string]interface{}{
		"cluster_creation_failed": map[string]interface{}{
			"title":       "EKS Cluster Creation Failed",
			"description": "Common issues and solutions for EKS cluster creation failures",
			"solutions": []string{
				"Check IAM role permissions for EKS service",
				"Verify VPC and subnet configuration",
				"Ensure security groups allow required traffic",
				"Check AWS service limits and quotas",
			},
			"commands": []string{
				"aws eks describe-cluster --name <cluster-name>",
				"aws iam get-role --role-name <eks-service-role>",
				"aws ec2 describe-subnets --subnet-ids <subnet-id>",
			},
		},
		"node_group_issues": map[string]interface{}{
			"title":       "EKS Node Group Issues",
			"description": "Troubleshooting node group creation and scaling problems",
			"solutions": []string{
				"Verify node group IAM role has required policies",
				"Check instance type availability in selected AZs",
				"Review security group rules for node communication",
				"Validate subnet configuration and capacity",
			},
			"commands": []string{
				"aws eks describe-nodegroup --cluster-name <cluster> --nodegroup-name <nodegroup>",
				"kubectl get nodes",
				"kubectl describe node <node-name>",
			},
		},
		"pod_scheduling": map[string]interface{}{
			"title":       "Pod Scheduling Issues",
			"description": "Resolving pod scheduling and resource allocation problems",
			"solutions": []string{
				"Check node capacity and resource requests",
				"Verify node selectors and affinity rules",
				"Review taints and tolerations",
				"Ensure sufficient cluster autoscaler configuration",
			},
			"commands": []string{
				"kubectl describe pod <pod-name>",
				"kubectl get events --sort-by='.lastTimestamp'",
				"kubectl top nodes",
				"kubectl top pods",
			},
		},
		"networking": map[string]interface{}{
			"title":       "EKS Networking Issues",
			"description": "Troubleshooting connectivity and DNS problems",
			"solutions": []string{
				"Verify VPC CNI plugin configuration",
				"Check security group rules for pod communication",
				"Validate CoreDNS configuration",
				"Review load balancer and ingress settings",
			},
			"commands": []string{
				"kubectl get pods -n kube-system",
				"kubectl logs -n kube-system -l k8s-app=aws-node",
				"kubectl get svc",
				"nslookup kubernetes.default.svc.cluster.local",
			},
		},
	}

	// Filter results based on query
	results := make(map[string]interface{})
	for key, scenario := range troubleshootingScenarios {
		if strings.Contains(strings.ToLower(key), strings.ToLower(query)) ||
			strings.Contains(strings.ToLower(scenario.(map[string]interface{})["title"].(string)), strings.ToLower(query)) {
			results[key] = scenario
		}
	}

	if len(results) == 0 {
		results["general"] = map[string]interface{}{
			"title":       "General EKS Troubleshooting",
			"description": "General troubleshooting steps for EKS issues",
			"solutions": []string{
				"Check AWS CloudTrail logs for API errors",
				"Review EKS cluster and node group status",
				"Verify IAM permissions and policies",
				"Check AWS service health dashboard",
			},
			"commands": []string{
				"aws eks list-clusters",
				"aws eks describe-cluster --name <cluster-name>",
				"kubectl cluster-info",
				"kubectl get all --all-namespaces",
			},
		}
	}

	return map[string]interface{}{
		"query":     query,
		"category":  category,
		"scenarios": results,
		"resources": []string{
			"https://docs.aws.amazon.com/eks/latest/userguide/troubleshooting.html",
			"https://aws.github.io/aws-eks-best-practices/",
			"https://kubernetes.io/docs/tasks/debug-application-cluster/",
		},
	}
}

func (t *SearchEKSTroubleshootGuideTool) GetSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Search query for troubleshooting topics",
			},
			"category": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"cluster", "nodegroup", "networking", "pods", "security", "performance"},
				"description": "Category to filter troubleshooting topics",
			},
		},
		"required": []string{"query"},
	}
}

func (t *SearchEKSTroubleshootGuideTool) GetMetadata() ToolMetadata {
	return ToolMetadata{
		Name:        "search-eks-troubleshoot-guide",
		Description: "Searches EKS troubleshooting guide for common issues and solutions",
		Category:    "Troubleshooting",
		Tags:        []string{"eks", "troubleshooting", "guide", "help"},
	}
}

// GetEKSInsightsTool gets EKS insights and recommendations
type GetEKSInsightsTool struct {
	client     *awsclient.Client
	actionType string
	logger     *logging.Logger
}

func NewGetEKSInsightsTool(client *awsclient.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	return &GetEKSInsightsTool{
		client:     client,
		actionType: actionType,
		logger:     logger,
	}
}

func (t *GetEKSInsightsTool) Execute(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	t.logger.Info("Getting EKS insights")

	// Parse arguments
	clusterName, ok := args["clusterName"].(string)
	if !ok || clusterName == "" {
		return createErrorResult("clusterName is required and must be a string"), nil
	}

	// Verify cluster exists
	clusterResult, err := t.client.DescribeEKSCluster(ctx, clusterName)
	if err != nil {
		return createErrorResult(fmt.Sprintf("Failed to verify cluster: %v", err)), nil
	}

	if clusterResult.Cluster == nil {
		return createErrorResult("Cluster not found"), nil
	}

	// Generate insights based on cluster configuration
	insights := t.generateEKSInsights(clusterResult.Cluster)

	return &mcp.CallToolResult{
		Meta: map[string]interface{}{
			"cluster_name": clusterName,
		},
		Content: []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": fmt.Sprintf("EKS insights and recommendations for cluster '%s'", clusterName),
			},
			map[string]interface{}{
				"type": "resource",
				"resource": map[string]interface{}{
					"uri":      fmt.Sprintf("eks://cluster/%s/insights", clusterName),
					"mimeType": "application/json",
					"text":     mustMarshalJSON(insights),
				},
			},
		},
		IsError: false,
	}, nil
}

func (t *GetEKSInsightsTool) generateEKSInsights(cluster *types.Cluster) map[string]interface{} {
	insights := map[string]interface{}{
		"clusterName": *cluster.Name,
		"version":     *cluster.Version,
		"status":      string(cluster.Status),
		"recommendations": []map[string]interface{}{},
		"security":        map[string]interface{}{},
		"performance":     map[string]interface{}{},
		"cost":           map[string]interface{}{},
	}

	recommendations := []map[string]interface{}{}

	// Version recommendations
	if *cluster.Version < "1.27" {
		recommendations = append(recommendations, map[string]interface{}{
			"type":        "version",
			"priority":    "high",
			"title":       "Upgrade Kubernetes Version",
			"description": "Consider upgrading to a newer Kubernetes version for security and feature improvements",
			"action":      "Plan cluster upgrade to latest supported version",
		})
	}

	// Security recommendations
	securityInsights := map[string]interface{}{
		"endpointAccess": "unknown",
		"encryption":     "unknown",
		"logging":        "unknown",
	}

	if cluster.ResourcesVpcConfig != nil {
		if cluster.ResourcesVpcConfig.EndpointPublicAccess != nil && *cluster.ResourcesVpcConfig.EndpointPublicAccess {
			securityInsights["endpointAccess"] = "public"
			recommendations = append(recommendations, map[string]interface{}{
				"type":        "security",
				"priority":    "medium",
				"title":       "Review Public Endpoint Access",
				"description": "Cluster API endpoint is publicly accessible. Consider restricting access.",
				"action":      "Review and configure endpoint access restrictions",
			})
		} else {
			securityInsights["endpointAccess"] = "private"
		}
	}

	// Encryption recommendations
	if len(cluster.EncryptionConfig) == 0 {
		securityInsights["encryption"] = "disabled"
		recommendations = append(recommendations, map[string]interface{}{
			"type":        "security",
			"priority":    "high",
			"title":       "Enable Encryption at Rest",
			"description": "Cluster secrets are not encrypted at rest. Enable encryption for better security.",
			"action":      "Configure KMS encryption for cluster secrets",
		})
	} else {
		securityInsights["encryption"] = "enabled"
	}

	// Logging recommendations
	if cluster.Logging == nil || len(cluster.Logging.ClusterLogging) == 0 {
		securityInsights["logging"] = "disabled"
		recommendations = append(recommendations, map[string]interface{}{
			"type":        "monitoring",
			"priority":    "medium",
			"title":       "Enable Control Plane Logging",
			"description": "Control plane logging is not enabled. Enable for better observability.",
			"action":      "Enable audit, api, authenticator, controllerManager, and scheduler logs",
		})
	} else {
		securityInsights["logging"] = "enabled"
	}

	insights["recommendations"] = recommendations
	insights["security"] = securityInsights

	// Performance insights
	insights["performance"] = map[string]interface{}{
		"recommendations": []string{
			"Monitor cluster autoscaler performance",
			"Review node group instance types for workload requirements",
			"Consider using spot instances for cost optimization",
			"Implement horizontal pod autoscaling for applications",
		},
	}

	// Cost optimization insights
	insights["cost"] = map[string]interface{}{
		"recommendations": []string{
			"Review node group sizing and utilization",
			"Consider using Fargate for serverless workloads",
			"Implement cluster autoscaler for dynamic scaling",
			"Use spot instances for non-critical workloads",
		},
	}

	return insights
}

func (t *GetEKSInsightsTool) GetSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"clusterName": map[string]interface{}{
				"type":        "string",
				"description": "Name of the EKS cluster to analyze",
			},
		},
		"required": []string{"clusterName"},
	}
}

func (t *GetEKSInsightsTool) GetMetadata() ToolMetadata {
	return ToolMetadata{
		Name:        "get-eks-insights",
		Description: "Gets EKS cluster insights and recommendations for security, performance, and cost optimization",
		Category:    "Analysis",
		Tags:        []string{"eks", "insights", "recommendations", "analysis"},
	}
}

// GetEKSVpcConfigTool gets VPC configuration for EKS cluster
type GetEKSVpcConfigTool struct {
	client     *awsclient.Client
	actionType string
	logger     *logging.Logger
}

func NewGetEKSVpcConfigTool(client *awsclient.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	return &GetEKSVpcConfigTool{
		client:     client,
		actionType: actionType,
		logger:     logger,
	}
}

func (t *GetEKSVpcConfigTool) Execute(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	t.logger.Info("Getting EKS VPC configuration")

	// Parse arguments
	clusterName, ok := args["clusterName"].(string)
	if !ok || clusterName == "" {
		return createErrorResult("clusterName is required and must be a string"), nil
	}

	// Verify cluster exists and get VPC config
	clusterResult, err := t.client.DescribeEKSCluster(ctx, clusterName)
	if err != nil {
		return createErrorResult(fmt.Sprintf("Failed to describe cluster: %v", err)), nil
	}

	if clusterResult.Cluster == nil {
		return createErrorResult("Cluster not found"), nil
	}

	cluster := clusterResult.Cluster
	vpcConfig := map[string]interface{}{
		"clusterName": *cluster.Name,
	}

	if cluster.ResourcesVpcConfig != nil {
		vpc := cluster.ResourcesVpcConfig
		vpcConfig["vpcId"] = *vpc.VpcId
		vpcConfig["subnetIds"] = vpc.SubnetIds
		vpcConfig["securityGroupIds"] = vpc.SecurityGroupIds
		vpcConfig["clusterSecurityGroupId"] = *vpc.ClusterSecurityGroupId
		vpcConfig["endpointPublicAccess"] = *vpc.EndpointPublicAccess
		vpcConfig["endpointPrivateAccess"] = *vpc.EndpointPrivateAccess
		vpcConfig["publicAccessCidrs"] = vpc.PublicAccessCidrs

		// Add recommendations based on VPC configuration
		recommendations := []string{}
		if *vpc.EndpointPublicAccess {
			recommendations = append(recommendations, "Consider restricting public access CIDRs for better security")
		}
		if !*vpc.EndpointPrivateAccess {
			recommendations = append(recommendations, "Consider enabling private endpoint access for better security")
		}
		if len(vpc.SubnetIds) < 2 {
			recommendations = append(recommendations, "Use multiple subnets across different AZs for high availability")
		}

		vpcConfig["recommendations"] = recommendations
	}

	return &mcp.CallToolResult{
		Meta: map[string]interface{}{
			"cluster_name": clusterName,
		},
		Content: []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": fmt.Sprintf("EKS VPC configuration for cluster '%s'", clusterName),
			},
			map[string]interface{}{
				"type": "resource",
				"resource": map[string]interface{}{
					"uri":      fmt.Sprintf("eks://cluster/%s/vpc-config", clusterName),
					"mimeType": "application/json",
					"text":     mustMarshalJSON(vpcConfig),
				},
			},
		},
		IsError: false,
	}, nil
}

func (t *GetEKSVpcConfigTool) GetSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"clusterName": map[string]interface{}{
				"type":        "string",
				"description": "Name of the EKS cluster",
			},
		},
		"required": []string{"clusterName"},
	}
}

func (t *GetEKSVpcConfigTool) GetMetadata() ToolMetadata {
	return ToolMetadata{
		Name:        "get-eks-vpc-config",
		Description: "Gets VPC configuration details for an EKS cluster",
		Category:    "EKS",
		Tags:        []string{"eks", "vpc", "networking", "configuration"},
	}
}

// Helper function for JSON marshaling
func mustMarshalJSON(v interface{}) string {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error": "Failed to marshal JSON: %v"}`, err)
	}
	return string(data)
}
