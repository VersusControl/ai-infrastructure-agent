package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// SecurityGroupParams contains parameters for creating a security group
type SecurityGroupParams struct {
	GroupName   string
	Description string
	VpcID       string
	Tags        map[string]string
}

// SecurityGroupRuleParams contains parameters for adding rules to a security group
type SecurityGroupRuleParams struct {
	GroupID    string
	Type       string // "ingress" or "egress"
	Protocol   string // "tcp", "udp", "icmp", or "-1" for all
	FromPort   int32
	ToPort     int32
	CidrBlocks []string
	SourceSG   string // Source security group ID for SG-to-SG rules
}

// CreateSecurityGroup creates a new security group
func (c *Client) CreateSecurityGroup(ctx context.Context, params SecurityGroupParams) (*ec2.CreateSecurityGroupOutput, error) {
	input := &ec2.CreateSecurityGroupInput{
		GroupName:   aws.String(params.GroupName),
		Description: aws.String(params.Description),
	}

	if params.VpcID != "" {
		input.VpcId = aws.String(params.VpcID)
	}

	// Add tag specifications during creation
	if len(params.Tags) > 0 {
		var ec2Tags []types.Tag
		for key, value := range params.Tags {
			ec2Tags = append(ec2Tags, types.Tag{
				Key:   aws.String(key),
				Value: aws.String(value),
			})
		}
		input.TagSpecifications = []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeSecurityGroup,
				Tags:         ec2Tags,
			},
		}
	}

	result, err := c.ec2.CreateSecurityGroup(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to create security group: %w", err)
	}

	c.logger.WithField("groupId", aws.ToString(result.GroupId)).Info("Security group created successfully")
	return result, nil
}

// AddSecurityGroupRule adds an ingress or egress rule to a security group
func (c *Client) AddSecurityGroupRule(ctx context.Context, params SecurityGroupRuleParams) error {
	var err error

	if params.Type == "ingress" {
		err = c.addIngressRule(ctx, params)
	} else if params.Type == "egress" {
		err = c.addEgressRule(ctx, params)
	} else {
		return fmt.Errorf("invalid rule type: %s (must be 'ingress' or 'egress')", params.Type)
	}

	if err != nil {
		return fmt.Errorf("failed to add %s rule: %w", params.Type, err)
	}

	c.logger.WithField("groupId", params.GroupID).WithField("type", params.Type).Info("Security group rule added successfully")
	return nil
}

// addIngressRule adds an ingress rule to the security group
func (c *Client) addIngressRule(ctx context.Context, params SecurityGroupRuleParams) error {
	var ipPermissions []types.IpPermission

	permission := types.IpPermission{
		IpProtocol: aws.String(params.Protocol),
	}

	// Set ports only for TCP/UDP protocols
	if params.Protocol == "tcp" || params.Protocol == "udp" {
		permission.FromPort = aws.Int32(params.FromPort)
		permission.ToPort = aws.Int32(params.ToPort)
	}

	// Add CIDR blocks if provided
	if len(params.CidrBlocks) > 0 {
		for _, cidr := range params.CidrBlocks {
			permission.IpRanges = append(permission.IpRanges, types.IpRange{
				CidrIp: aws.String(cidr),
			})
		}
	}

	// Add source security group if provided
	if params.SourceSG != "" {
		permission.UserIdGroupPairs = append(permission.UserIdGroupPairs, types.UserIdGroupPair{
			GroupId: aws.String(params.SourceSG),
		})
	}

	ipPermissions = append(ipPermissions, permission)

	input := &ec2.AuthorizeSecurityGroupIngressInput{
		GroupId:       aws.String(params.GroupID),
		IpPermissions: ipPermissions,
	}

	_, err := c.ec2.AuthorizeSecurityGroupIngress(ctx, input)
	return err
}

// addEgressRule adds an egress rule to the security group
func (c *Client) addEgressRule(ctx context.Context, params SecurityGroupRuleParams) error {
	var ipPermissions []types.IpPermission

	permission := types.IpPermission{
		IpProtocol: aws.String(params.Protocol),
	}

	// Set ports only for TCP/UDP protocols
	if params.Protocol == "tcp" || params.Protocol == "udp" {
		permission.FromPort = aws.Int32(params.FromPort)
		permission.ToPort = aws.Int32(params.ToPort)
	}

	// Add CIDR blocks if provided
	if len(params.CidrBlocks) > 0 {
		for _, cidr := range params.CidrBlocks {
			permission.IpRanges = append(permission.IpRanges, types.IpRange{
				CidrIp: aws.String(cidr),
			})
		}
	}

	// Add source security group if provided
	if params.SourceSG != "" {
		permission.UserIdGroupPairs = append(permission.UserIdGroupPairs, types.UserIdGroupPair{
			GroupId: aws.String(params.SourceSG),
		})
	}

	ipPermissions = append(ipPermissions, permission)

	input := &ec2.AuthorizeSecurityGroupEgressInput{
		GroupId:       aws.String(params.GroupID),
		IpPermissions: ipPermissions,
	}

	_, err := c.ec2.AuthorizeSecurityGroupEgress(ctx, input)
	return err
}

// ListSecurityGroups lists all security groups in the region
func (c *Client) ListSecurityGroups(ctx context.Context, vpcID string) ([]types.SecurityGroup, error) {
	input := &ec2.DescribeSecurityGroupsInput{}

	// Filter by VPC if specified
	if vpcID != "" {
		input.Filters = []types.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{vpcID},
			},
		}
	}

	result, err := c.ec2.DescribeSecurityGroups(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to list security groups: %w", err)
	}

	c.logger.WithField("count", len(result.SecurityGroups)).Info("Listed security groups")
	return result.SecurityGroups, nil
}

// GetSecurityGroup gets details of a specific security group
func (c *Client) GetSecurityGroup(ctx context.Context, groupID string) (*types.SecurityGroup, error) {
	input := &ec2.DescribeSecurityGroupsInput{
		GroupIds: []string{groupID},
	}

	result, err := c.ec2.DescribeSecurityGroups(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get security group %s: %w", groupID, err)
	}

	if len(result.SecurityGroups) == 0 {
		return nil, fmt.Errorf("security group %s not found", groupID)
	}

	return &result.SecurityGroups[0], nil
}

// DeleteSecurityGroup deletes a security group
func (c *Client) DeleteSecurityGroup(ctx context.Context, groupID string) error {
	input := &ec2.DeleteSecurityGroupInput{
		GroupId: aws.String(groupID),
	}

	_, err := c.ec2.DeleteSecurityGroup(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete security group %s: %w", groupID, err)
	}

	c.logger.WithField("groupId", groupID).Info("Security group deleted successfully")
	return nil
}
