package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// ListIAMRoles lists IAM roles
func (c *Client) ListIAMRoles(ctx context.Context) ([]*types.AWSResource, error) {
	c.logger.Info("ListIAMRoles called")

	input := &iam.ListRolesInput{}
	var roles []*types.AWSResource

	paginator := iam.NewListRolesPaginator(c.iam, input)
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list IAM roles: %w", err)
		}

		for _, role := range output.Roles {
			roles = append(roles, c.convertIAMRole(role))
		}
	}

	c.logger.WithField("count", len(roles)).Info("IAM roles listed successfully")
	return roles, nil
}

// CreateIAMRole creates a new IAM role
func (c *Client) CreateIAMRole(ctx context.Context, params CreateIAMRoleParams) (*types.AWSResource, error) {
	c.logger.WithField("roleName", params.RoleName).Info("CreateIAMRole called")

	input := &iam.CreateRoleInput{
		RoleName:                 aws.String(params.RoleName),
		AssumeRolePolicyDocument: aws.String(params.AssumeRolePolicyDocument),
	}

	if params.Description != "" {
		input.Description = aws.String(params.Description)
	}

	if len(params.Tags) > 0 {
		var tags []iamtypes.Tag
		for k, v := range params.Tags {
			tags = append(tags, iamtypes.Tag{
				Key:   aws.String(k),
				Value: aws.String(v),
			})
		}
		input.Tags = tags
	}

	result, err := c.iam.CreateRole(ctx, input)
	if err != nil {
		c.logger.WithError(err).Error("Failed to create IAM role")
		return nil, fmt.Errorf("failed to create IAM role: %w", err)
	}

	if result.Role == nil {
		return nil, fmt.Errorf("role creation initiated but no role returned")
	}

	resource := c.convertIAMRole(*result.Role)
	c.logger.WithField("roleName", params.RoleName).Info("IAM role created successfully")
	return resource, nil
}

// convertIAMRole converts an IAM role to our internal resource representation
func (c *Client) convertIAMRole(role iamtypes.Role) *types.AWSResource {
	tags := make(map[string]string)
	for _, tag := range role.Tags {
		if tag.Key != nil && tag.Value != nil {
			tags[*tag.Key] = *tag.Value
		}
	}

	details := map[string]interface{}{
		"arn":                      aws.ToString(role.Arn),
		"roleName":                 aws.ToString(role.RoleName),
		"roleId":                   aws.ToString(role.RoleId),
		"createDate":               role.CreateDate,
		"assumeRolePolicyDocument": aws.ToString(role.AssumeRolePolicyDocument),
		"description":              aws.ToString(role.Description),
		"maxSessionDuration":       aws.ToInt32(role.MaxSessionDuration),
		"path":                     aws.ToString(role.Path),
	}

	return &types.AWSResource{
		ID:       aws.ToString(role.RoleName), // Use RoleName as ID
		Type:     "iam-role",
		Region:   "global", // IAM is global
		State:    "active", // IAM roles don't have a state like EC2
		Tags:     tags,
		Details:  details,
		LastSeen: time.Now(),
	}
}

// GetIAMRole retrieves an IAM role by name
func (c *Client) GetIAMRole(ctx context.Context, roleName string) (*types.AWSResource, error) {
	c.logger.WithField("roleName", roleName).Info("GetIAMRole called")

	input := &iam.GetRoleInput{
		RoleName: aws.String(roleName),
	}

	result, err := c.iam.GetRole(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get IAM role: %w", err)
	}

	if result.Role == nil {
		return nil, fmt.Errorf("role not found")
	}

	return c.convertIAMRole(*result.Role), nil
}

// AttachRolePolicy attaches a policy to an IAM role
func (c *Client) AttachRolePolicy(ctx context.Context, roleName string, policyArn string) error {
	c.logger.WithFields(map[string]interface{}{
		"roleName":  roleName,
		"policyArn": policyArn,
	}).Info("AttachRolePolicy called")

	input := &iam.AttachRolePolicyInput{
		RoleName:  aws.String(roleName),
		PolicyArn: aws.String(policyArn),
	}

	_, err := c.iam.AttachRolePolicy(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to attach policy to role: %w", err)
	}

	c.logger.Info("Policy attached successfully")
	return nil
}
