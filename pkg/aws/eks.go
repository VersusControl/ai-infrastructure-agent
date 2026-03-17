package aws

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"

	"github.com/sirupsen/logrus"
)

// ========== EKS Cluster Management Methods ==========

// CreateEKSCluster creates a new EKS cluster
func (c *Client) CreateEKSCluster(ctx context.Context, params CreateEKSClusterParams) (*types.AWSResource, error) {
	c.logger.WithFields(logrus.Fields{
		"name":    params.Name,
		"version": params.Version,
		"subnets": params.SubnetIDs,
	}).Info("CreateEKSCluster called with parameters")

	input := &eks.CreateClusterInput{
		Name:    aws.String(params.Name),
		RoleArn: aws.String(params.RoleArn),
		ResourcesVpcConfig: &ekstypes.VpcConfigRequest{
			SubnetIds:        params.SubnetIDs,
			SecurityGroupIds: params.SecurityGroupIDs,
		},
	}

	if params.Version != "" {
		input.Version = aws.String(params.Version)
	}

	if len(params.Tags) > 0 {
		input.Tags = params.Tags
	}

	result, err := c.eks.CreateCluster(ctx, input)
	if err != nil {
		c.logger.WithError(err).Error("Failed to create EKS cluster")
		return nil, fmt.Errorf("failed to create cluster: %w", err)
	}

	if result.Cluster == nil {
		return nil, fmt.Errorf("cluster creation initiated but no cluster returned")
	}

	resource := c.convertEKSCluster(result.Cluster)
	c.logger.WithField("clusterName", params.Name).Info("EKS cluster creation initiated")
	return resource, nil
}

// DescribeEKSCluster gets details of a specific EKS cluster
func (c *Client) DescribeEKSCluster(ctx context.Context, name string) (*types.AWSResource, error) {
	result, err := c.eks.DescribeCluster(ctx, &eks.DescribeClusterInput{
		Name: aws.String(name),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe cluster %s: %w", name, err)
	}

	if result.Cluster == nil {
		return nil, fmt.Errorf("cluster %s not found", name)
	}

	return c.convertEKSCluster(result.Cluster), nil
}

// ListEKSClusters lists all EKS clusters in the region
func (c *Client) ListEKSClusters(ctx context.Context) ([]string, error) {
	input := &eks.ListClustersInput{}
	var clusterNames []string

	paginator := eks.NewListClustersPaginator(c.eks, input)
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list clusters: %w", err)
		}
		clusterNames = append(clusterNames, output.Clusters...)
	}

	return clusterNames, nil
}

// DeleteEKSCluster deletes an EKS cluster
func (c *Client) DeleteEKSCluster(ctx context.Context, name string) error {
	_, err := c.eks.DeleteCluster(ctx, &eks.DeleteClusterInput{
		Name: aws.String(name),
	})
	if err != nil {
		return fmt.Errorf("failed to delete cluster %s: %w", name, err)
	}

	c.logger.WithField("clusterName", name).Info("EKS cluster deletion initiated")
	return nil
}

// ========== EKS Node Group Management Methods ==========

// CreateEKSNodeGroup creates a managed node group for an EKS cluster
func (c *Client) CreateEKSNodeGroup(ctx context.Context, params CreateEKSNodeGroupParams) (*types.AWSResource, error) {
	c.logger.WithFields(logrus.Fields{
		"clusterName":   params.ClusterName,
		"nodeGroupName": params.NodeGroupName,
	}).Info("CreateEKSNodeGroup called")

	input := &eks.CreateNodegroupInput{
		ClusterName:   aws.String(params.ClusterName),
		NodegroupName: aws.String(params.NodeGroupName),
		NodeRole:      aws.String(params.NodeRoleArn),
		Subnets:       params.SubnetIDs,
		ScalingConfig: &ekstypes.NodegroupScalingConfig{
			MinSize:     aws.Int32(params.MinSize),
			MaxSize:     aws.Int32(params.MaxSize),
			DesiredSize: aws.Int32(params.DesiredSize),
		},
	}

	if len(params.InstanceTypes) > 0 {
		input.InstanceTypes = params.InstanceTypes
	}

	if params.DiskSize > 0 {
		input.DiskSize = aws.Int32(params.DiskSize)
	}

	if params.AmiType != "" {
		input.AmiType = ekstypes.AMITypes(params.AmiType)
	}

	if len(params.Tags) > 0 {
		input.Tags = params.Tags
	}

	result, err := c.eks.CreateNodegroup(ctx, input)
	if err != nil {
		c.logger.WithError(err).Error("Failed to create EKS node group")
		return nil, fmt.Errorf("failed to create node group: %w", err)
	}

	if result.Nodegroup == nil {
		return nil, fmt.Errorf("node group creation initiated but no node group returned")
	}

	resource := c.convertEKSNodeGroup(result.Nodegroup)
	c.logger.WithField("nodeGroupName", params.NodeGroupName).Info("EKS node group creation initiated")
	return resource, nil
}

// ListEKSNodeGroups lists node groups for a specific cluster
func (c *Client) ListEKSNodeGroups(ctx context.Context, clusterName string) ([]string, error) {
	input := &eks.ListNodegroupsInput{
		ClusterName: aws.String(clusterName),
	}
	var nodeGroups []string

	paginator := eks.NewListNodegroupsPaginator(c.eks, input)
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list node groups for cluster %s: %w", clusterName, err)
		}
		nodeGroups = append(nodeGroups, output.Nodegroups...)
	}

	return nodeGroups, nil
}

// ========== Kubernetes Operations ==========

// ApplyKubernetesYAML applies a YAML configuration to the EKS cluster
func (c *Client) ApplyKubernetesYAML(ctx context.Context, clusterName string, yamlContent string) error {
	c.logger.WithField("clusterName", clusterName).Info("Applying Kubernetes YAML")

	// Update kubeconfig
	// We assume 'aws' and 'kubectl' are in the PATH
	updateCmd := exec.CommandContext(ctx, "aws", "eks", "update-kubeconfig", "--name", clusterName, "--region", c.cfg.Region)
	if output, err := updateCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to update kubeconfig: %s: %w", string(output), err)
	}

	// Write YAML to temp file
	tmpFile, err := os.CreateTemp("", "k8s-*.yaml")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(yamlContent); err != nil {
		return fmt.Errorf("failed to write YAML to temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Apply YAML
	applyCmd := exec.CommandContext(ctx, "kubectl", "apply", "-f", tmpFile.Name())
	if output, err := applyCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to apply YAML: %s: %w", string(output), err)
	}

	return nil
}

// ========== Helpers ==========

// convertEKSCluster converts an EKS cluster to our internal resource representation
func (c *Client) convertEKSCluster(cluster *ekstypes.Cluster) *types.AWSResource {
	details := map[string]interface{}{
		"version":         aws.ToString(cluster.Version),
		"roleArn":         aws.ToString(cluster.RoleArn),
		"endpoint":        aws.ToString(cluster.Endpoint),
		"status":          string(cluster.Status),
		"platformVersion": aws.ToString(cluster.PlatformVersion),
	}

	if cluster.ResourcesVpcConfig != nil {
		details["vpcId"] = aws.ToString(cluster.ResourcesVpcConfig.VpcId)
		details["subnetIds"] = cluster.ResourcesVpcConfig.SubnetIds
		details["securityGroupIds"] = cluster.ResourcesVpcConfig.SecurityGroupIds
		details["clusterSecurityGroupId"] = aws.ToString(cluster.ResourcesVpcConfig.ClusterSecurityGroupId)
	}

	return &types.AWSResource{
		ID:       aws.ToString(cluster.Name), // Use Name as ID for EKS clusters usually, or cluster.Arn
		Type:     "eks-cluster",
		Region:   c.cfg.Region,
		State:    string(cluster.Status),
		Tags:     cluster.Tags,
		Details:  details,
		LastSeen: time.Now(),
	}
}

// convertEKSNodeGroup converts an EKS node group to our internal resource representation
func (c *Client) convertEKSNodeGroup(ng *ekstypes.Nodegroup) *types.AWSResource {
	details := map[string]interface{}{
		"clusterName":    aws.ToString(ng.ClusterName),
		"nodeRole":       aws.ToString(ng.NodeRole),
		"status":         string(ng.Status),
		"amiType":        string(ng.AmiType),
		"instanceTypes":  ng.InstanceTypes,
		"diskSize":       aws.ToInt32(ng.DiskSize),
		"capacityType":   string(ng.CapacityType),
		"version":        aws.ToString(ng.Version),
		"releaseVersion": aws.ToString(ng.ReleaseVersion),
	}

	if ng.ScalingConfig != nil {
		details["scalingConfig"] = map[string]int32{
			"minSize":     aws.ToInt32(ng.ScalingConfig.MinSize),
			"maxSize":     aws.ToInt32(ng.ScalingConfig.MaxSize),
			"desiredSize": aws.ToInt32(ng.ScalingConfig.DesiredSize),
		}
	}

	return &types.AWSResource{
		ID:       aws.ToString(ng.NodegroupName),
		Type:     "eks-nodegroup",
		Region:   c.cfg.Region,
		State:    string(ng.Status),
		Tags:     ng.Tags,
		Details:  details,
		LastSeen: time.Now(),
	}
}
