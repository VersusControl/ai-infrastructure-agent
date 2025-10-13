package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	autoscalingtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/versus-control/ai-infrastructure-agent/pkg/types"

	"github.com/sirupsen/logrus"
)

// ========== Auto Scaling Group Methods ==========

// CreateLaunchTemplate creates a launch template for auto scaling
func (c *Client) CreateLaunchTemplate(ctx context.Context, params CreateLaunchTemplateParams) (*types.AWSResource, error) {
	// Prepare launch template data
	templateData := &ec2types.RequestLaunchTemplateData{
		ImageId:      aws.String(params.ImageID),
		InstanceType: ec2types.InstanceType(params.InstanceType),
	}

	if params.KeyName != "" {
		templateData.KeyName = aws.String(params.KeyName)
	}

	if len(params.SecurityGroupIDs) > 0 {
		templateData.SecurityGroupIds = params.SecurityGroupIDs
	}

	if params.UserData != "" {
		templateData.UserData = aws.String(params.UserData)
	}

	if params.IamInstanceProfile != "" {
		templateData.IamInstanceProfile = &ec2types.LaunchTemplateIamInstanceProfileSpecificationRequest{
			Name: aws.String(params.IamInstanceProfile),
		}
	}

	input := &ec2.CreateLaunchTemplateInput{
		LaunchTemplateName: aws.String(params.LaunchTemplateName),
		LaunchTemplateData: templateData,
	}

	// Add tag specifications during creation if tags are provided
	if len(params.Tags) > 0 {
		var ec2Tags []ec2types.Tag
		for key, value := range params.Tags {
			ec2Tags = append(ec2Tags, ec2types.Tag{
				Key:   aws.String(key),
				Value: aws.String(value),
			})
		}
		input.TagSpecifications = []ec2types.TagSpecification{
			{
				ResourceType: ec2types.ResourceTypeLaunchTemplate,
				Tags:         ec2Tags,
			},
		}
	}

	result, err := c.ec2.CreateLaunchTemplate(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to create launch template: %w", err)
	}

	templateID := *result.LaunchTemplate.LaunchTemplateId
	c.logger.WithFields(logrus.Fields{
		"templateId":   templateID,
		"templateName": params.LaunchTemplateName,
	}).Info("Launch template created successfully")

	resource := &types.AWSResource{
		ID:    templateID,
		Type:  "launch-template",
		State: "available",
		Details: map[string]interface{}{
			"name":         params.LaunchTemplateName,
			"imageId":      params.ImageID,
			"instanceType": params.InstanceType,
		},
		LastSeen: time.Now(),
	}

	return resource, nil
}

// CreateAutoScalingGroup creates an auto scaling group
func (c *Client) CreateAutoScalingGroup(ctx context.Context, params CreateAutoScalingGroupParams) (*types.AWSResource, error) {
	// Convert subnet IDs to comma-separated string
	var subnetIDs string
	if len(params.VPCZoneIdentifiers) > 0 {
		subnetIDs = params.VPCZoneIdentifiers[0]
		for i := 1; i < len(params.VPCZoneIdentifiers); i++ {
			subnetIDs += "," + params.VPCZoneIdentifiers[i]
		}
	}

	input := &autoscaling.CreateAutoScalingGroupInput{
		AutoScalingGroupName: aws.String(params.AutoScalingGroupName),
		MinSize:              aws.Int32(params.MinSize),
		MaxSize:              aws.Int32(params.MaxSize),
		DesiredCapacity:      aws.Int32(params.DesiredCapacity),
		VPCZoneIdentifier:    aws.String(subnetIDs),
		LaunchTemplate: &autoscalingtypes.LaunchTemplateSpecification{
			LaunchTemplateName: aws.String(params.LaunchTemplateName),
			Version:            aws.String("$Latest"),
		},
		HealthCheckType:        aws.String(params.HealthCheckType),
		HealthCheckGracePeriod: aws.Int32(params.HealthCheckGracePeriod),
	}

	// Add target group ARNs if provided
	if len(params.TargetGroupARNs) > 0 {
		input.TargetGroupARNs = params.TargetGroupARNs
	}

	_, err := c.autoscaling.CreateAutoScalingGroup(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to create auto scaling group: %w", err)
	}

	c.logger.WithField("asgName", params.AutoScalingGroupName).Info("Auto Scaling Group created successfully")

	resource := &types.AWSResource{
		ID:    params.AutoScalingGroupName,
		Type:  "auto-scaling-group",
		State: "active",
		Details: map[string]interface{}{
			"minSize":         params.MinSize,
			"maxSize":         params.MaxSize,
			"desiredCapacity": params.DesiredCapacity,
			"launchTemplate":  params.LaunchTemplateName,
		},
		LastSeen: time.Now(),
	}

	return resource, nil
}

// UpdateAutoScalingGroup updates the desired capacity of an auto scaling group
func (c *Client) UpdateAutoScalingGroup(ctx context.Context, asgName string, desiredCapacity int32) error {
	input := &autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName: aws.String(asgName),
		DesiredCapacity:      aws.Int32(desiredCapacity),
	}

	_, err := c.autoscaling.UpdateAutoScalingGroup(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to update auto scaling group: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"asgName":         asgName,
		"desiredCapacity": desiredCapacity,
	}).Info("Auto Scaling Group updated successfully")

	return nil
}

// DeleteAutoScalingGroup deletes an auto scaling group
func (c *Client) DeleteAutoScalingGroup(ctx context.Context, asgName string, forceDelete bool) error {
	input := &autoscaling.DeleteAutoScalingGroupInput{
		AutoScalingGroupName: aws.String(asgName),
		ForceDelete:          aws.Bool(forceDelete),
	}

	_, err := c.autoscaling.DeleteAutoScalingGroup(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete auto scaling group: %w", err)
	}

	c.logger.WithField("asgName", asgName).Info("Auto Scaling Group deleted successfully")

	return nil
}

// ========== Auto Scaling Group Listing Methods ==========

// DescribeAutoScalingGroups lists all Auto Scaling Groups in the region
func (c *Client) DescribeAutoScalingGroups(ctx context.Context) ([]*types.AWSResource, error) {
	result, err := c.autoscaling.DescribeAutoScalingGroups(ctx, &autoscaling.DescribeAutoScalingGroupsInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to describe Auto Scaling Groups: %w", err)
	}

	var resources []*types.AWSResource
	for _, asg := range result.AutoScalingGroups {
		resources = append(resources, c.convertAutoScalingGroup(asg))
	}

	return resources, nil
}

// GetAutoScalingGroup gets a specific Auto Scaling Group by name
func (c *Client) GetAutoScalingGroup(ctx context.Context, groupName string) (*types.AWSResource, error) {
	result, err := c.autoscaling.DescribeAutoScalingGroups(ctx, &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: []string{groupName},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe Auto Scaling Group %s: %w", groupName, err)
	}

	if len(result.AutoScalingGroups) == 0 {
		return nil, fmt.Errorf("auto Scaling Group %s not found", groupName)
	}

	return c.convertAutoScalingGroup(result.AutoScalingGroups[0]), nil
}

// DescribeLaunchTemplates lists all Launch Templates in the region
func (c *Client) DescribeLaunchTemplates(ctx context.Context) ([]*types.AWSResource, error) {
	result, err := c.ec2.DescribeLaunchTemplates(ctx, &ec2.DescribeLaunchTemplatesInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to describe Launch Templates: %w", err)
	}

	var resources []*types.AWSResource
	for _, lt := range result.LaunchTemplates {
		resources = append(resources, c.convertLaunchTemplate(lt))
	}

	return resources, nil
}

// GetLaunchTemplate gets a specific Launch Template by ID
func (c *Client) GetLaunchTemplate(ctx context.Context, templateID string) (*types.AWSResource, error) {
	result, err := c.ec2.DescribeLaunchTemplates(ctx, &ec2.DescribeLaunchTemplatesInput{
		LaunchTemplateIds: []string{templateID},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe Launch Template %s: %w", templateID, err)
	}

	if len(result.LaunchTemplates) == 0 {
		return nil, fmt.Errorf("launch Template %s not found", templateID)
	}

	return c.convertLaunchTemplate(result.LaunchTemplates[0]), nil
}

// convertAutoScalingGroup converts an Auto Scaling Group to our internal resource representation
func (c *Client) convertAutoScalingGroup(asg autoscalingtypes.AutoScalingGroup) *types.AWSResource {
	tags := make(map[string]string)
	for _, tag := range asg.Tags {
		if tag.Key != nil && tag.Value != nil {
			tags[*tag.Key] = *tag.Value
		}
	}

	// Extract instance IDs
	var instanceIds []string
	for _, instance := range asg.Instances {
		if instance.InstanceId != nil {
			instanceIds = append(instanceIds, *instance.InstanceId)
		}
	}

	details := map[string]interface{}{
		"minSize":                          aws.ToInt32(asg.MinSize),
		"maxSize":                          aws.ToInt32(asg.MaxSize),
		"desiredCapacity":                  aws.ToInt32(asg.DesiredCapacity),
		"defaultCooldown":                  aws.ToInt32(asg.DefaultCooldown),
		"healthCheckType":                  aws.ToString(asg.HealthCheckType),
		"healthCheckGracePeriod":           aws.ToInt32(asg.HealthCheckGracePeriod),
		"launchTemplate":                   asg.LaunchTemplate,
		"vpcZoneIdentifier":                aws.ToString(asg.VPCZoneIdentifier),
		"targetGroupARNs":                  asg.TargetGroupARNs,
		"loadBalancerNames":                asg.LoadBalancerNames,
		"serviceLinkedRoleARN":             aws.ToString(asg.ServiceLinkedRoleARN),
		"instances":                        instanceIds,
		"availabilityZones":                asg.AvailabilityZones,
		"capacityRebalance":                aws.ToBool(asg.CapacityRebalance),
		"newInstancesProtectedFromScaleIn": aws.ToBool(asg.NewInstancesProtectedFromScaleIn),
	}

	return &types.AWSResource{
		ID:       aws.ToString(asg.AutoScalingGroupName),
		Type:     "auto-scaling-group",
		Region:   c.cfg.Region,
		State:    "active", // ASGs don't have a state field like EC2
		Tags:     tags,
		Details:  details,
		LastSeen: time.Now(),
	}
}

// convertLaunchTemplate converts a Launch Template to our internal resource representation
func (c *Client) convertLaunchTemplate(lt ec2types.LaunchTemplate) *types.AWSResource {
	tags := make(map[string]string)
	for _, tag := range lt.Tags {
		if tag.Key != nil && tag.Value != nil {
			tags[*tag.Key] = *tag.Value
		}
	}

	details := map[string]interface{}{
		"launchTemplateName":   aws.ToString(lt.LaunchTemplateName),
		"versionNumber":        aws.ToInt64(lt.LatestVersionNumber),
		"defaultVersionNumber": aws.ToInt64(lt.DefaultVersionNumber),
		"createdBy":            aws.ToString(lt.CreatedBy),
		"createTime":           lt.CreateTime,
	}

	return &types.AWSResource{
		ID:       aws.ToString(lt.LaunchTemplateId),
		Type:     "launch-template",
		Region:   c.cfg.Region,
		State:    "available",
		Tags:     tags,
		Details:  details,
		LastSeen: time.Now(),
	}
}

// AttachLoadBalancerTargetGroups attaches an Auto Scaling Group to load balancer target groups
func (c *Client) AttachLoadBalancerTargetGroups(ctx context.Context, asgName string, targetGroupARNs []string) error {
	input := &autoscaling.AttachLoadBalancerTargetGroupsInput{
		AutoScalingGroupName: aws.String(asgName),
		TargetGroupARNs:      targetGroupARNs,
	}

	_, err := c.autoscaling.AttachLoadBalancerTargetGroups(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to attach load balancer target groups to ASG %s: %w", asgName, err)
	}

	c.logger.WithFields(map[string]interface{}{
		"asgName":          asgName,
		"targetGroupCount": len(targetGroupARNs),
		"targetGroupARNs":  targetGroupARNs,
	}).Info("Successfully attached Auto Scaling Group to target groups")

	return nil
}

// DetachLoadBalancerTargetGroups detaches an Auto Scaling Group from load balancer target groups
func (c *Client) DetachLoadBalancerTargetGroups(ctx context.Context, asgName string, targetGroupARNs []string) error {
	input := &autoscaling.DetachLoadBalancerTargetGroupsInput{
		AutoScalingGroupName: aws.String(asgName),
		TargetGroupARNs:      targetGroupARNs,
	}

	_, err := c.autoscaling.DetachLoadBalancerTargetGroups(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to detach load balancer target groups from ASG %s: %w", asgName, err)
	}

	c.logger.WithFields(map[string]interface{}{
		"asgName":          asgName,
		"targetGroupCount": len(targetGroupARNs),
		"targetGroupARNs":  targetGroupARNs,
	}).Info("Successfully detached Auto Scaling Group from target groups")

	return nil
}
