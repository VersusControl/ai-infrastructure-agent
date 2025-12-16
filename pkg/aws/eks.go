package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
)

// EKS service methods for the AWS client

// GetEKSClient returns the EKS client
func (c *Client) GetEKSClient() *eks.Client {
	if c.eks == nil {
		c.eks = eks.NewFromConfig(c.cfg)
	}
	return c.eks
}

// EKS Cluster Operations

// CreateEKSCluster creates a new EKS cluster
func (c *Client) CreateEKSCluster(ctx context.Context, input *eks.CreateClusterInput) (*eks.CreateClusterOutput, error) {
	c.logger.WithField("clusterName", *input.Name).Info("Creating EKS cluster")
	return c.GetEKSClient().CreateCluster(ctx, input)
}

// DescribeEKSCluster describes an EKS cluster
func (c *Client) DescribeEKSCluster(ctx context.Context, clusterName string) (*eks.DescribeClusterOutput, error) {
	c.logger.WithField("clusterName", clusterName).Debug("Describing EKS cluster")
	return c.GetEKSClient().DescribeCluster(ctx, &eks.DescribeClusterInput{
		Name: aws.String(clusterName),
	})
}

// ListEKSClusters lists all EKS clusters
func (c *Client) ListEKSClusters(ctx context.Context) (*eks.ListClustersOutput, error) {
	c.logger.Debug("Listing EKS clusters")
	return c.GetEKSClient().ListClusters(ctx, &eks.ListClustersInput{})
}

// DeleteEKSCluster deletes an EKS cluster
func (c *Client) DeleteEKSCluster(ctx context.Context, clusterName string) (*eks.DeleteClusterOutput, error) {
	c.logger.WithField("clusterName", clusterName).Info("Deleting EKS cluster")
	return c.GetEKSClient().DeleteCluster(ctx, &eks.DeleteClusterInput{
		Name: aws.String(clusterName),
	})
}

// UpdateEKSClusterConfig updates EKS cluster configuration
func (c *Client) UpdateEKSClusterConfig(ctx context.Context, input *eks.UpdateClusterConfigInput) (*eks.UpdateClusterConfigOutput, error) {
	c.logger.WithField("clusterName", *input.Name).Info("Updating EKS cluster config")
	return c.GetEKSClient().UpdateClusterConfig(ctx, input)
}

// UpdateEKSClusterVersion updates EKS cluster version
func (c *Client) UpdateEKSClusterVersion(ctx context.Context, input *eks.UpdateClusterVersionInput) (*eks.UpdateClusterVersionOutput, error) {
	c.logger.WithField("clusterName", *input.Name).Info("Updating EKS cluster version")
	return c.GetEKSClient().UpdateClusterVersion(ctx, input)
}

// EKS Node Group Operations

// CreateEKSNodeGroup creates a new EKS node group
func (c *Client) CreateEKSNodeGroup(ctx context.Context, input *eks.CreateNodegroupInput) (*eks.CreateNodegroupOutput, error) {
	c.logger.WithFields(map[string]interface{}{
		"clusterName":   *input.ClusterName,
		"nodeGroupName": *input.NodegroupName,
	}).Info("Creating EKS node group")
	return c.GetEKSClient().CreateNodegroup(ctx, input)
}

// DescribeEKSNodeGroup describes an EKS node group
func (c *Client) DescribeEKSNodeGroup(ctx context.Context, clusterName, nodeGroupName string) (*eks.DescribeNodegroupOutput, error) {
	c.logger.WithFields(map[string]interface{}{
		"clusterName":   clusterName,
		"nodeGroupName": nodeGroupName,
	}).Debug("Describing EKS node group")
	return c.GetEKSClient().DescribeNodegroup(ctx, &eks.DescribeNodegroupInput{
		ClusterName:   aws.String(clusterName),
		NodegroupName: aws.String(nodeGroupName),
	})
}

// ListEKSNodeGroups lists all node groups for a cluster
func (c *Client) ListEKSNodeGroups(ctx context.Context, clusterName string) (*eks.ListNodegroupsOutput, error) {
	c.logger.WithField("clusterName", clusterName).Debug("Listing EKS node groups")
	return c.GetEKSClient().ListNodegroups(ctx, &eks.ListNodegroupsInput{
		ClusterName: aws.String(clusterName),
	})
}

// DeleteEKSNodeGroup deletes an EKS node group
func (c *Client) DeleteEKSNodeGroup(ctx context.Context, clusterName, nodeGroupName string) (*eks.DeleteNodegroupOutput, error) {
	c.logger.WithFields(map[string]interface{}{
		"clusterName":   clusterName,
		"nodeGroupName": nodeGroupName,
	}).Info("Deleting EKS node group")
	return c.GetEKSClient().DeleteNodegroup(ctx, &eks.DeleteNodegroupInput{
		ClusterName:   aws.String(clusterName),
		NodegroupName: aws.String(nodeGroupName),
	})
}

// UpdateEKSNodeGroupConfig updates EKS node group configuration
func (c *Client) UpdateEKSNodeGroupConfig(ctx context.Context, input *eks.UpdateNodegroupConfigInput) (*eks.UpdateNodegroupConfigOutput, error) {
	c.logger.WithFields(map[string]interface{}{
		"clusterName":   *input.ClusterName,
		"nodeGroupName": *input.NodegroupName,
	}).Info("Updating EKS node group config")
	return c.GetEKSClient().UpdateNodegroupConfig(ctx, input)
}

// UpdateEKSNodeGroupVersion updates EKS node group version
func (c *Client) UpdateEKSNodeGroupVersion(ctx context.Context, input *eks.UpdateNodegroupVersionInput) (*eks.UpdateNodegroupVersionOutput, error) {
	c.logger.WithFields(map[string]interface{}{
		"clusterName":   *input.ClusterName,
		"nodeGroupName": *input.NodegroupName,
	}).Info("Updating EKS node group version")
	return c.GetEKSClient().UpdateNodegroupVersion(ctx, input)
}

// EKS Add-on Operations

// CreateEKSAddon creates an EKS add-on
func (c *Client) CreateEKSAddon(ctx context.Context, input *eks.CreateAddonInput) (*eks.CreateAddonOutput, error) {
	c.logger.WithFields(map[string]interface{}{
		"clusterName": *input.ClusterName,
		"addonName":   *input.AddonName,
	}).Info("Creating EKS add-on")
	return c.GetEKSClient().CreateAddon(ctx, input)
}

// DescribeEKSAddon describes an EKS add-on
func (c *Client) DescribeEKSAddon(ctx context.Context, clusterName, addonName string) (*eks.DescribeAddonOutput, error) {
	c.logger.WithFields(map[string]interface{}{
		"clusterName": clusterName,
		"addonName":   addonName,
	}).Debug("Describing EKS add-on")
	return c.GetEKSClient().DescribeAddon(ctx, &eks.DescribeAddonInput{
		ClusterName: aws.String(clusterName),
		AddonName:   aws.String(addonName),
	})
}

// ListEKSAddons lists all add-ons for a cluster
func (c *Client) ListEKSAddons(ctx context.Context, clusterName string) (*eks.ListAddonsOutput, error) {
	c.logger.WithField("clusterName", clusterName).Debug("Listing EKS add-ons")
	return c.GetEKSClient().ListAddons(ctx, &eks.ListAddonsInput{
		ClusterName: aws.String(clusterName),
	})
}

// DeleteEKSAddon deletes an EKS add-on
func (c *Client) DeleteEKSAddon(ctx context.Context, clusterName, addonName string) (*eks.DeleteAddonOutput, error) {
	c.logger.WithFields(map[string]interface{}{
		"clusterName": clusterName,
		"addonName":   addonName,
	}).Info("Deleting EKS add-on")
	return c.GetEKSClient().DeleteAddon(ctx, &eks.DeleteAddonInput{
		ClusterName: aws.String(clusterName),
		AddonName:   aws.String(addonName),
	})
}

// UpdateEKSAddon updates an EKS add-on
func (c *Client) UpdateEKSAddon(ctx context.Context, input *eks.UpdateAddonInput) (*eks.UpdateAddonOutput, error) {
	c.logger.WithFields(map[string]interface{}{
		"clusterName": *input.ClusterName,
		"addonName":   *input.AddonName,
	}).Info("Updating EKS add-on")
	return c.GetEKSClient().UpdateAddon(ctx, input)
}

// EKS Fargate Profile Operations

// CreateEKSFargateProfile creates an EKS Fargate profile
func (c *Client) CreateEKSFargateProfile(ctx context.Context, input *eks.CreateFargateProfileInput) (*eks.CreateFargateProfileOutput, error) {
	c.logger.WithFields(map[string]interface{}{
		"clusterName":        *input.ClusterName,
		"fargateProfileName": *input.FargateProfileName,
	}).Info("Creating EKS Fargate profile")
	return c.GetEKSClient().CreateFargateProfile(ctx, input)
}

// DescribeEKSFargateProfile describes an EKS Fargate profile
func (c *Client) DescribeEKSFargateProfile(ctx context.Context, clusterName, fargateProfileName string) (*eks.DescribeFargateProfileOutput, error) {
	c.logger.WithFields(map[string]interface{}{
		"clusterName":        clusterName,
		"fargateProfileName": fargateProfileName,
	}).Debug("Describing EKS Fargate profile")
	return c.GetEKSClient().DescribeFargateProfile(ctx, &eks.DescribeFargateProfileInput{
		ClusterName:        aws.String(clusterName),
		FargateProfileName: aws.String(fargateProfileName),
	})
}

// ListEKSFargateProfiles lists all Fargate profiles for a cluster
func (c *Client) ListEKSFargateProfiles(ctx context.Context, clusterName string) (*eks.ListFargateProfilesOutput, error) {
	c.logger.WithField("clusterName", clusterName).Debug("Listing EKS Fargate profiles")
	return c.GetEKSClient().ListFargateProfiles(ctx, &eks.ListFargateProfilesInput{
		ClusterName: aws.String(clusterName),
	})
}

// DeleteEKSFargateProfile deletes an EKS Fargate profile
func (c *Client) DeleteEKSFargateProfile(ctx context.Context, clusterName, fargateProfileName string) (*eks.DeleteFargateProfileOutput, error) {
	c.logger.WithFields(map[string]interface{}{
		"clusterName":        clusterName,
		"fargateProfileName": fargateProfileName,
	}).Info("Deleting EKS Fargate profile")
	return c.GetEKSClient().DeleteFargateProfile(ctx, &eks.DeleteFargateProfileInput{
		ClusterName:        aws.String(clusterName),
		FargateProfileName: aws.String(fargateProfileName),
	})
}

// EKS Identity Provider Config Operations

// AssociateEKSIdentityProviderConfig associates an identity provider config with a cluster
func (c *Client) AssociateEKSIdentityProviderConfig(ctx context.Context, input *eks.AssociateIdentityProviderConfigInput) (*eks.AssociateIdentityProviderConfigOutput, error) {
	c.logger.WithField("clusterName", *input.ClusterName).Info("Associating EKS identity provider config")
	return c.GetEKSClient().AssociateIdentityProviderConfig(ctx, input)
}

// DescribeEKSIdentityProviderConfig describes an identity provider config
func (c *Client) DescribeEKSIdentityProviderConfig(ctx context.Context, input *eks.DescribeIdentityProviderConfigInput) (*eks.DescribeIdentityProviderConfigOutput, error) {
	c.logger.WithField("clusterName", *input.ClusterName).Debug("Describing EKS identity provider config")
	return c.GetEKSClient().DescribeIdentityProviderConfig(ctx, input)
}

// ListEKSIdentityProviderConfigs lists all identity provider configs for a cluster
func (c *Client) ListEKSIdentityProviderConfigs(ctx context.Context, clusterName string) (*eks.ListIdentityProviderConfigsOutput, error) {
	c.logger.WithField("clusterName", clusterName).Debug("Listing EKS identity provider configs")
	return c.GetEKSClient().ListIdentityProviderConfigs(ctx, &eks.ListIdentityProviderConfigsInput{
		ClusterName: aws.String(clusterName),
	})
}

// DisassociateEKSIdentityProviderConfig disassociates an identity provider config from a cluster
func (c *Client) DisassociateEKSIdentityProviderConfig(ctx context.Context, input *eks.DisassociateIdentityProviderConfigInput) (*eks.DisassociateIdentityProviderConfigOutput, error) {
	c.logger.WithField("clusterName", *input.ClusterName).Info("Disassociating EKS identity provider config")
	return c.GetEKSClient().DisassociateIdentityProviderConfig(ctx, input)
}

// EKS Access Entry Operations (for EKS Access Management)

// CreateEKSAccessEntry creates an access entry for a cluster
func (c *Client) CreateEKSAccessEntry(ctx context.Context, input *eks.CreateAccessEntryInput) (*eks.CreateAccessEntryOutput, error) {
	c.logger.WithFields(map[string]interface{}{
		"clusterName":    *input.ClusterName,
		"principalArn":   *input.PrincipalArn,
	}).Info("Creating EKS access entry")
	return c.GetEKSClient().CreateAccessEntry(ctx, input)
}

// DescribeEKSAccessEntry describes an access entry
func (c *Client) DescribeEKSAccessEntry(ctx context.Context, clusterName, principalArn string) (*eks.DescribeAccessEntryOutput, error) {
	c.logger.WithFields(map[string]interface{}{
		"clusterName":  clusterName,
		"principalArn": principalArn,
	}).Debug("Describing EKS access entry")
	return c.GetEKSClient().DescribeAccessEntry(ctx, &eks.DescribeAccessEntryInput{
		ClusterName:  aws.String(clusterName),
		PrincipalArn: aws.String(principalArn),
	})
}

// ListEKSAccessEntries lists all access entries for a cluster
func (c *Client) ListEKSAccessEntries(ctx context.Context, clusterName string) (*eks.ListAccessEntriesOutput, error) {
	c.logger.WithField("clusterName", clusterName).Debug("Listing EKS access entries")
	return c.GetEKSClient().ListAccessEntries(ctx, &eks.ListAccessEntriesInput{
		ClusterName: aws.String(clusterName),
	})
}

// DeleteEKSAccessEntry deletes an access entry
func (c *Client) DeleteEKSAccessEntry(ctx context.Context, clusterName, principalArn string) (*eks.DeleteAccessEntryOutput, error) {
	c.logger.WithFields(map[string]interface{}{
		"clusterName":  clusterName,
		"principalArn": principalArn,
	}).Info("Deleting EKS access entry")
	return c.GetEKSClient().DeleteAccessEntry(ctx, &eks.DeleteAccessEntryInput{
		ClusterName:  aws.String(clusterName),
		PrincipalArn: aws.String(principalArn),
	})
}

// UpdateEKSAccessEntry updates an access entry
func (c *Client) UpdateEKSAccessEntry(ctx context.Context, input *eks.UpdateAccessEntryInput) (*eks.UpdateAccessEntryOutput, error) {
	c.logger.WithFields(map[string]interface{}{
		"clusterName":  *input.ClusterName,
		"principalArn": *input.PrincipalArn,
	}).Info("Updating EKS access entry")
	return c.GetEKSClient().UpdateAccessEntry(ctx, input)
}

// EKS Pod Identity Association Operations

// CreateEKSPodIdentityAssociation creates a pod identity association
func (c *Client) CreateEKSPodIdentityAssociation(ctx context.Context, input *eks.CreatePodIdentityAssociationInput) (*eks.CreatePodIdentityAssociationOutput, error) {
	c.logger.WithFields(map[string]interface{}{
		"clusterName": *input.ClusterName,
		"namespace":   *input.Namespace,
		"serviceAccount": *input.ServiceAccount,
	}).Info("Creating EKS pod identity association")
	return c.GetEKSClient().CreatePodIdentityAssociation(ctx, input)
}

// DescribeEKSPodIdentityAssociation describes a pod identity association
func (c *Client) DescribeEKSPodIdentityAssociation(ctx context.Context, clusterName, associationId string) (*eks.DescribePodIdentityAssociationOutput, error) {
	c.logger.WithFields(map[string]interface{}{
		"clusterName":   clusterName,
		"associationId": associationId,
	}).Debug("Describing EKS pod identity association")
	return c.GetEKSClient().DescribePodIdentityAssociation(ctx, &eks.DescribePodIdentityAssociationInput{
		ClusterName:   aws.String(clusterName),
		AssociationId: aws.String(associationId),
	})
}

// ListEKSPodIdentityAssociations lists all pod identity associations for a cluster
func (c *Client) ListEKSPodIdentityAssociations(ctx context.Context, clusterName string) (*eks.ListPodIdentityAssociationsOutput, error) {
	c.logger.WithField("clusterName", clusterName).Debug("Listing EKS pod identity associations")
	return c.GetEKSClient().ListPodIdentityAssociations(ctx, &eks.ListPodIdentityAssociationsInput{
		ClusterName: aws.String(clusterName),
	})
}

// DeleteEKSPodIdentityAssociation deletes a pod identity association
func (c *Client) DeleteEKSPodIdentityAssociation(ctx context.Context, clusterName, associationId string) (*eks.DeletePodIdentityAssociationOutput, error) {
	c.logger.WithFields(map[string]interface{}{
		"clusterName":   clusterName,
		"associationId": associationId,
	}).Info("Deleting EKS pod identity association")
	return c.GetEKSClient().DeletePodIdentityAssociation(ctx, &eks.DeletePodIdentityAssociationInput{
		ClusterName:   aws.String(clusterName),
		AssociationId: aws.String(associationId),
	})
}

// UpdateEKSPodIdentityAssociation updates a pod identity association
func (c *Client) UpdateEKSPodIdentityAssociation(ctx context.Context, input *eks.UpdatePodIdentityAssociationInput) (*eks.UpdatePodIdentityAssociationOutput, error) {
	c.logger.WithFields(map[string]interface{}{
		"clusterName":   *input.ClusterName,
		"associationId": *input.AssociationId,
	}).Info("Updating EKS pod identity association")
	return c.GetEKSClient().UpdatePodIdentityAssociation(ctx, input)
}

// EKS Utility Operations

// DescribeEKSUpdate describes an update operation
func (c *Client) DescribeEKSUpdate(ctx context.Context, clusterName, updateId string, nodeGroupName *string) (*eks.DescribeUpdateOutput, error) {
	input := &eks.DescribeUpdateInput{
		Name:     aws.String(clusterName),
		UpdateId: aws.String(updateId),
	}
	if nodeGroupName != nil {
		input.NodegroupName = nodeGroupName
	}
	
	c.logger.WithFields(map[string]interface{}{
		"clusterName": clusterName,
		"updateId":    updateId,
	}).Debug("Describing EKS update")
	return c.GetEKSClient().DescribeUpdate(ctx, input)
}

// ListEKSUpdates lists all updates for a cluster
func (c *Client) ListEKSUpdates(ctx context.Context, clusterName string, nodeGroupName *string) (*eks.ListUpdatesOutput, error) {
	input := &eks.ListUpdatesInput{
		Name: aws.String(clusterName),
	}
	if nodeGroupName != nil {
		input.NodegroupName = nodeGroupName
	}
	
	c.logger.WithField("clusterName", clusterName).Debug("Listing EKS updates")
	return c.GetEKSClient().ListUpdates(ctx, input)
}

// TagEKSResource tags an EKS resource
func (c *Client) TagEKSResource(ctx context.Context, input *eks.TagResourceInput) (*eks.TagResourceOutput, error) {
	c.logger.WithField("resourceArn", *input.ResourceArn).Info("Tagging EKS resource")
	return c.GetEKSClient().TagResource(ctx, input)
}

// UntagEKSResource removes tags from an EKS resource
func (c *Client) UntagEKSResource(ctx context.Context, input *eks.UntagResourceInput) (*eks.UntagResourceOutput, error) {
	c.logger.WithField("resourceArn", *input.ResourceArn).Info("Untagging EKS resource")
	return c.GetEKSClient().UntagResource(ctx, input)
}

// ListEKSTagsForResource lists tags for an EKS resource
func (c *Client) ListEKSTagsForResource(ctx context.Context, resourceArn string) (*eks.ListTagsForResourceOutput, error) {
	c.logger.WithField("resourceArn", resourceArn).Debug("Listing tags for EKS resource")
	return c.GetEKSClient().ListTagsForResource(ctx, &eks.ListTagsForResourceInput{
		ResourceArn: aws.String(resourceArn),
	})
}
