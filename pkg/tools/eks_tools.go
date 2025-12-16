package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	awsclient "github.com/versus-control/ai-infrastructure-agent/pkg/aws"
	"github.com/versus-control/ai-infrastructure-agent/pkg/interfaces"
)

// Helper function to create error results
func createErrorResult(message string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("Error: %s", message)),
		},
		IsError: true,
	}
}

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
					vpcConfig.EndpointPublicAccess = &publicAccess
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
	_, err := t.client.CreateEKSCluster(ctx, input)
	if err != nil {
		t.logger.WithError(err).Error("Failed to create EKS cluster")
		return createErrorResult(fmt.Sprintf("Failed to create EKS cluster: %v", err)), nil
	}

	// Return success result
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("EKS cluster '%s' creation initiated successfully", name)),
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

func (t *CreateEKSClusterTool) Name() string {
	return "create-eks-cluster"
}

func (t *CreateEKSClusterTool) Description() string {
	return "Creates a new Amazon EKS cluster with specified configuration"
}

func (t *CreateEKSClusterTool) Category() string {
	return "eks-mcp"
}

func (t *CreateEKSClusterTool) ActionType() string {
	return t.actionType
}

func (t *CreateEKSClusterTool) GetInputSchema() map[string]interface{} {
	return t.GetSchema()
}

func (t *CreateEKSClusterTool) GetOutputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"success": map[string]interface{}{
				"type":        "boolean",
				"description": "Whether the operation was successful",
			},
			"message": map[string]interface{}{
				"type":        "string",
				"description": "Human-readable message about the operation",
			},
		},
		"required": []string{"success", "message"},
	}
}

func (t *CreateEKSClusterTool) GetExamples() []interfaces.ToolExample {
	return []interfaces.ToolExample{}
}

func (t *CreateEKSClusterTool) ValidateArguments(arguments map[string]interface{}) error {
	// Basic validation
	if name, exists := arguments["name"]; !exists || name == "" {
		return fmt.Errorf("name is required")
	}
	if roleArn, exists := arguments["roleArn"]; !exists || roleArn == "" {
		return fmt.Errorf("roleArn is required")
	}
	return nil
}

// ========== Kubernetes Resource Management Tools ==========

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
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("Kubernetes resource management prepared for %s %s/%s in cluster '%s'", action, resourceType, resourceName, clusterName)),
			mcp.NewResourceContent(
				fmt.Sprintf("k8s://cluster/%s/resource/%s/%s", clusterName, resourceType, resourceName),
				"application/json",
				mustMarshalJSON(map[string]interface{}{
					"clusterName":     clusterName,
					"action":          action,
					"resourceType":    resourceType,
					"resourceName":    resourceName,
					"namespace":       namespace,
					"kubectlCommands": kubectlCommands,
				}),
			),
		},
		IsError: false,
	}, nil
}

func (t *ManageK8sResourceTool) Name() string {
	return "manage-k8s-resource"
}

func (t *ManageK8sResourceTool) Description() string {
	return "Manages Kubernetes resources (create, update, delete, get)"
}

func (t *ManageK8sResourceTool) Category() string {
	return "eks-mcp"
}

func (t *ManageK8sResourceTool) ActionType() string {
	return t.actionType
}

func (t *ManageK8sResourceTool) GetInputSchema() map[string]interface{} {
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

func (t *ManageK8sResourceTool) GetOutputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"success": map[string]interface{}{
				"type":        "boolean",
				"description": "Whether the operation was successful",
			},
			"message": map[string]interface{}{
				"type":        "string",
				"description": "Human-readable message about the operation",
			},
		},
		"required": []string{"success", "message"},
	}
}

func (t *ManageK8sResourceTool) GetExamples() []interfaces.ToolExample {
	return []interfaces.ToolExample{}
}

func (t *ManageK8sResourceTool) ValidateArguments(arguments map[string]interface{}) error {
	required := []string{"clusterName", "action", "resourceType", "resourceName"}
	for _, field := range required {
		if val, exists := arguments[field]; !exists || val == "" {
			return fmt.Errorf("%s is required", field)
		}
	}
	return nil
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

func (t *ApplyYamlTool) Name() string {
	return "apply-yaml"
}

func (t *ApplyYamlTool) Description() string {
	return "Applies YAML manifests to a Kubernetes cluster"
}

func (t *ApplyYamlTool) Category() string {
	return "eks-mcp"
}

func (t *ApplyYamlTool) ActionType() string {
	return t.actionType
}

func (t *ApplyYamlTool) GetInputSchema() map[string]interface{} {
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

func (t *ApplyYamlTool) GetOutputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"success": map[string]interface{}{
				"type":        "boolean",
				"description": "Whether the operation was successful",
			},
			"message": map[string]interface{}{
				"type":        "string",
				"description": "Human-readable message about the operation",
			},
		},
		"required": []string{"success", "message"},
	}
}

func (t *ApplyYamlTool) GetExamples() []interfaces.ToolExample {
	return []interfaces.ToolExample{}
}

func (t *ApplyYamlTool) ValidateArguments(arguments map[string]interface{}) error {
	required := []string{"clusterName", "yamlContent"}
	for _, field := range required {
		if val, exists := arguments[field]; !exists || val == "" {
			return fmt.Errorf("%s is required", field)
		}
	}
	return nil
}

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

func (t *GetK8sEventsTool) Name() string {
	return "get-k8s-events"
}

func (t *GetK8sEventsTool) Description() string {
	return "Gets Kubernetes events from an EKS cluster"
}

func (t *GetK8sEventsTool) Category() string {
	return "eks-mcp"
}

func (t *GetK8sEventsTool) ActionType() string {
	return t.actionType
}

func (t *GetK8sEventsTool) GetInputSchema() map[string]interface{} {
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

func (t *GetK8sEventsTool) GetOutputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"success": map[string]interface{}{
				"type":        "boolean",
				"description": "Whether the operation was successful",
			},
			"message": map[string]interface{}{
				"type":        "string",
				"description": "Human-readable message about the operation",
			},
		},
		"required": []string{"success", "message"},
	}
}

func (t *GetK8sEventsTool) GetExamples() []interfaces.ToolExample {
	return []interfaces.ToolExample{}
}

func (t *GetK8sEventsTool) ValidateArguments(arguments map[string]interface{}) error {
	if clusterName, exists := arguments["clusterName"]; !exists || clusterName == "" {
		return fmt.Errorf("clusterName is required")
	}
	return nil
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

func (t *GetPodLogsTool) Name() string {
	return "get-pod-logs"
}

func (t *GetPodLogsTool) Description() string {
	return "Gets logs from Kubernetes pods in an EKS cluster"
}

func (t *GetPodLogsTool) Category() string {
	return "eks-mcp"
}

func (t *GetPodLogsTool) ActionType() string {
	return t.actionType
}

func (t *GetPodLogsTool) GetInputSchema() map[string]interface{} {
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

func (t *GetPodLogsTool) GetOutputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"success": map[string]interface{}{
				"type":        "boolean",
				"description": "Whether the operation was successful",
			},
			"message": map[string]interface{}{
				"type":        "string",
				"description": "Human-readable message about the operation",
			},
		},
		"required": []string{"success", "message"},
	}
}

func (t *GetPodLogsTool) GetExamples() []interfaces.ToolExample {
	return []interfaces.ToolExample{}
}

func (t *GetPodLogsTool) ValidateArguments(arguments map[string]interface{}) error {
	required := []string{"clusterName", "podName"}
	for _, field := range required {
		if val, exists := arguments[field]; !exists || val == "" {
			return fmt.Errorf("%s is required", field)
		}
	}
	return nil
}

// Helper function for JSON marshaling
func mustMarshalJSON(v interface{}) string {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error": "Failed to marshal JSON: %v"}`, err)
	}
	return string(data)
}
