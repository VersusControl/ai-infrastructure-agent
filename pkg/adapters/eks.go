package adapters

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/aws"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// EKSAdapter handles EKS cluster and node group operations
type EKSAdapter struct {
	*BaseAWSAdapter
	client *aws.Client
}

// NewEKSAdapter creates a new EKS adapter
func NewEKSAdapter(client *aws.Client, logger *logging.Logger) *EKSAdapter {
	return &EKSAdapter{
		BaseAWSAdapter: NewBaseAWSAdapter(client, logger, "eks"),
		client:         client,
	}
}

// ValidateResource validates an EKS resource state
func (a *EKSAdapter) ValidateResource(resource *types.ResourceState) error {
	if resource == nil {
		return fmt.Errorf("resource cannot be nil")
	}

	if resource.Type != "eks-cluster" && resource.Type != "eks-nodegroup" && 
	   resource.Type != "eks-addon" && resource.Type != "eks-fargate-profile" {
		return fmt.Errorf("invalid resource type for EKS adapter: %s", resource.Type)
	}

	if resource.ID == "" {
		return fmt.Errorf("resource ID cannot be empty")
	}

	return nil
}

// NormalizeResource normalizes an EKS resource from AWS API response
func (a *EKSAdapter) NormalizeResource(rawResource interface{}) (*types.ResourceState, error) {
	switch resource := rawResource.(type) {
	case *eks.DescribeClusterOutput:
		return a.normalizeCluster(resource)
	case *eks.DescribeNodegroupOutput:
		return a.normalizeNodeGroup(resource)
	case *eks.DescribeAddonOutput:
		return a.normalizeAddon(resource)
	case *eks.DescribeFargateProfileOutput:
		return a.normalizeFargateProfile(resource)
	default:
		return nil, fmt.Errorf("unsupported EKS resource type: %T", rawResource)
	}
}

// ExtractMetadata extracts metadata from an EKS resource
func (a *EKSAdapter) ExtractMetadata(resource interface{}) map[string]interface{} {
	metadata := make(map[string]interface{})

	switch r := resource.(type) {
	case *eks.DescribeClusterOutput:
		if r.Cluster != nil {
			cluster := r.Cluster
			metadata["name"] = *cluster.Name
			metadata["version"] = *cluster.Version
			metadata["status"] = string(cluster.Status)
			metadata["endpoint"] = *cluster.Endpoint
			metadata["platform_version"] = *cluster.PlatformVersion
			metadata["created_at"] = cluster.CreatedAt.String()
			
			if cluster.ResourcesVpcConfig != nil {
				metadata["vpc_id"] = *cluster.ResourcesVpcConfig.VpcId
				if len(cluster.ResourcesVpcConfig.SubnetIds) > 0 {
					metadata["subnet_ids"] = cluster.ResourcesVpcConfig.SubnetIds
				}
				if len(cluster.ResourcesVpcConfig.SecurityGroupIds) > 0 {
					metadata["security_group_ids"] = cluster.ResourcesVpcConfig.SecurityGroupIds
				}
			}
			
			if cluster.RoleArn != nil {
				metadata["role_arn"] = *cluster.RoleArn
			}
		}

	case *eks.DescribeNodegroupOutput:
		if r.Nodegroup != nil {
			nodegroup := r.Nodegroup
			metadata["name"] = *nodegroup.NodegroupName
			metadata["cluster_name"] = *nodegroup.ClusterName
			metadata["status"] = string(nodegroup.Status)
			metadata["instance_types"] = nodegroup.InstanceTypes
			metadata["ami_type"] = string(nodegroup.AmiType)
			metadata["capacity_type"] = string(nodegroup.CapacityType)
			metadata["node_role"] = *nodegroup.NodeRole
			metadata["created_at"] = nodegroup.CreatedAt.String()
			
			if nodegroup.ScalingConfig != nil {
				metadata["min_size"] = *nodegroup.ScalingConfig.MinSize
				metadata["max_size"] = *nodegroup.ScalingConfig.MaxSize
				metadata["desired_size"] = *nodegroup.ScalingConfig.DesiredSize
			}
			
			if len(nodegroup.Subnets) > 0 {
				metadata["subnets"] = nodegroup.Subnets
			}
		}

	case *eks.DescribeAddonOutput:
		if r.Addon != nil {
			addon := r.Addon
			metadata["name"] = *addon.AddonName
			metadata["cluster_name"] = *addon.ClusterName
			metadata["status"] = string(addon.Status)
			metadata["addon_version"] = *addon.AddonVersion
			metadata["created_at"] = addon.CreatedAt.String()
			
			if addon.ServiceAccountRoleArn != nil {
				metadata["service_account_role_arn"] = *addon.ServiceAccountRoleArn
			}
		}

	case *eks.DescribeFargateProfileOutput:
		if r.FargateProfile != nil {
			profile := r.FargateProfile
			metadata["name"] = *profile.FargateProfileName
			metadata["cluster_name"] = *profile.ClusterName
			metadata["status"] = string(profile.Status)
			metadata["pod_execution_role_arn"] = *profile.PodExecutionRoleArn
			metadata["created_at"] = profile.CreatedAt.String()
			
			if len(profile.Subnets) > 0 {
				metadata["subnets"] = profile.Subnets
			}
		}
	}

	return metadata
}

// ListByTags lists EKS resources filtered by tags
func (a *EKSAdapter) ListByTags(ctx context.Context, tags map[string]string) ([]*types.AWSResource, error) {
	var resources []*types.AWSResource

	// List clusters and filter by tags
	clusters, err := a.listClusters(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list EKS clusters: %w", err)
	}

	for _, cluster := range clusters {
		if a.matchesTags(cluster.Tags, tags) {
			resources = append(resources, cluster)
		}
	}

	return resources, nil
}

// ListByFilter lists EKS resources filtered by general criteria
func (a *EKSAdapter) ListByFilter(ctx context.Context, filters map[string]interface{}) ([]*types.AWSResource, error) {
	var resources []*types.AWSResource

	// List all clusters first
	clusters, err := a.listClusters(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list EKS clusters: %w", err)
	}

	for _, cluster := range clusters {
		if a.matchesFilters(cluster, filters) {
			resources = append(resources, cluster)
		}
	}

	return resources, nil
}

// Private helper methods

func (a *EKSAdapter) normalizeCluster(output *eks.DescribeClusterOutput) (*types.ResourceState, error) {
	if output.Cluster == nil {
		return nil, fmt.Errorf("cluster data is nil")
	}

	cluster := output.Cluster
	
	return &types.ResourceState{
		ID:     *cluster.Name,
		Type:   "eks-cluster",
		Status: string(cluster.Status),
		Region: a.client.GetRegion(),
		Properties: map[string]interface{}{
			"name":             *cluster.Name,
			"version":          *cluster.Version,
			"status":           string(cluster.Status),
			"endpoint":         *cluster.Endpoint,
			"platform_version": *cluster.PlatformVersion,
			"role_arn":         *cluster.RoleArn,
			"created_at":       cluster.CreatedAt.String(),
			"mcp_response":     a.ExtractMetadata(output),
		},
	}, nil
}

func (a *EKSAdapter) normalizeNodeGroup(output *eks.DescribeNodegroupOutput) (*types.ResourceState, error) {
	if output.Nodegroup == nil {
		return nil, fmt.Errorf("nodegroup data is nil")
	}

	nodegroup := output.Nodegroup
	
	return &types.ResourceState{
		ID:     fmt.Sprintf("%s/%s", *nodegroup.ClusterName, *nodegroup.NodegroupName),
		Type:   "eks-nodegroup",
		Status: string(nodegroup.Status),
		Region: a.client.GetRegion(),
		Properties: map[string]interface{}{
			"name":         *nodegroup.NodegroupName,
			"cluster_name": *nodegroup.ClusterName,
			"status":       string(nodegroup.Status),
			"node_role":    *nodegroup.NodeRole,
			"created_at":   nodegroup.CreatedAt.String(),
			"mcp_response": a.ExtractMetadata(output),
		},
	}, nil
}

func (a *EKSAdapter) normalizeAddon(output *eks.DescribeAddonOutput) (*types.ResourceState, error) {
	if output.Addon == nil {
		return nil, fmt.Errorf("addon data is nil")
	}

	addon := output.Addon
	
	return &types.ResourceState{
		ID:     fmt.Sprintf("%s/%s", *addon.ClusterName, *addon.AddonName),
		Type:   "eks-addon",
		Status: string(addon.Status),
		Region: a.client.GetRegion(),
		Properties: map[string]interface{}{
			"name":           *addon.AddonName,
			"cluster_name":   *addon.ClusterName,
			"status":         string(addon.Status),
			"addon_version":  *addon.AddonVersion,
			"created_at":     addon.CreatedAt.String(),
			"mcp_response":   a.ExtractMetadata(output),
		},
	}, nil
}

func (a *EKSAdapter) normalizeFargateProfile(output *eks.DescribeFargateProfileOutput) (*types.ResourceState, error) {
	if output.FargateProfile == nil {
		return nil, fmt.Errorf("fargate profile data is nil")
	}

	profile := output.FargateProfile
	
	return &types.ResourceState{
		ID:     fmt.Sprintf("%s/%s", *profile.ClusterName, *profile.FargateProfileName),
		Type:   "eks-fargate-profile",
		Status: string(profile.Status),
		Region: a.client.GetRegion(),
		Properties: map[string]interface{}{
			"name":                     *profile.FargateProfileName,
			"cluster_name":             *profile.ClusterName,
			"status":                   string(profile.Status),
			"pod_execution_role_arn":   *profile.PodExecutionRoleArn,
			"created_at":               profile.CreatedAt.String(),
			"mcp_response":             a.ExtractMetadata(output),
		},
	}, nil
}

func (a *EKSAdapter) listClusters(ctx context.Context) ([]*types.AWSResource, error) {
	var resources []*types.AWSResource

	// List all clusters
	listOutput, err := a.client.ListEKSClusters(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list EKS clusters: %w", err)
	}

	// Get detailed information for each cluster
	for _, clusterName := range listOutput.Clusters {
		describeOutput, err := a.client.DescribeEKSCluster(ctx, clusterName)
		if err != nil {
			a.logger.WithError(err).WithField("cluster", clusterName).Warn("Failed to describe EKS cluster")
			continue
		}

		if describeOutput.Cluster == nil {
			continue
		}

		cluster := describeOutput.Cluster
		
		// Extract tags
		tags := make(map[string]string)
		if cluster.Tags != nil {
			for key, value := range cluster.Tags {
				tags[key] = value
			}
		}

		resource := &types.AWSResource{
			ID:     *cluster.Name,
			Type:   "eks-cluster",
			State:  string(cluster.Status),
			Region: a.client.GetRegion(),
			Tags:   tags,
			Details: map[string]interface{}{
				"name":             *cluster.Name,
				"version":          *cluster.Version,
				"status":           string(cluster.Status),
				"endpoint":         *cluster.Endpoint,
				"platform_version": *cluster.PlatformVersion,
				"role_arn":         *cluster.RoleArn,
				"created_at":       cluster.CreatedAt.String(),
			},
		}

		// Add VPC configuration if available
		if cluster.ResourcesVpcConfig != nil {
			resource.Details["vpc_id"] = *cluster.ResourcesVpcConfig.VpcId
			if len(cluster.ResourcesVpcConfig.SubnetIds) > 0 {
				resource.Details["subnet_ids"] = cluster.ResourcesVpcConfig.SubnetIds
			}
			if len(cluster.ResourcesVpcConfig.SecurityGroupIds) > 0 {
				resource.Details["security_group_ids"] = cluster.ResourcesVpcConfig.SecurityGroupIds
			}
		}

		resources = append(resources, resource)
	}

	return resources, nil
}

// Helper method to check if cluster status matches filter
func (a *EKSAdapter) matchesClusterStatus(status string, filterStatus interface{}) bool {
	filterStr, ok := filterStatus.(string)
	if !ok {
		return false
	}
	return strings.EqualFold(status, filterStr)
}
