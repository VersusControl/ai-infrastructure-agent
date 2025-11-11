package adapters

import (
	"context"
	"fmt"
	"time"

	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/aws"
	"github.com/versus-control/ai-infrastructure-agent/pkg/interfaces"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// SecurityGroupAdapter implements the AWSResourceAdapter interface for Security Group resources
type SecurityGroupAdapter struct {
	*BaseAWSAdapter
	client *aws.Client
}

// NewSecurityGroupAdapter creates a new Security Group adapter
func NewSecurityGroupAdapter(client *aws.Client, logger *logging.Logger) interfaces.AWSResourceAdapter {
	base := NewBaseAWSAdapter(client, logger, "security-group")
	return &SecurityGroupAdapter{
		BaseAWSAdapter: base,
		client:         client,
	}
}

// Create creates a new security group
func (s *SecurityGroupAdapter) Create(ctx context.Context, params interface{}) (*types.AWSResource, error) {
	createParams, ok := params.(aws.SecurityGroupParams)
	if !ok {
		return nil, fmt.Errorf("invalid parameters for security group creation, expected aws.SecurityGroupParams")
	}

	result, err := s.client.CreateSecurityGroup(ctx, createParams)
	if err != nil {
		return nil, fmt.Errorf("failed to create security group: %w", err)
	}

	// Convert to AWSResource
	resource := &types.AWSResource{
		ID:     *result.GroupId,
		Type:   "security-group",
		Region: s.client.GetRegion(),
		State:  "available",
		Tags:   createParams.Tags,
		Details: map[string]interface{}{
			"GroupName":   createParams.GroupName,
			"Description": createParams.Description,
			"VpcId":       createParams.VpcID,
		},
		LastSeen: time.Now(),
	}

	s.logger.Infof("Created security group %s", resource.ID)
	return resource, nil
}

// List returns all security groups
func (s *SecurityGroupAdapter) List(ctx context.Context) ([]*types.AWSResource, error) {
	// Use the AWS client's ListSecurityGroups method
	sgs, err := s.client.ListSecurityGroups(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("failed to list security groups: %w", err)
	}

	var resources []*types.AWSResource
	for _, sg := range sgs {
		resource := &types.AWSResource{
			ID:     *sg.GroupId,
			Type:   "security-group",
			Region: s.client.GetRegion(),
			State:  "available",
			Tags:   make(map[string]string),
			Details: map[string]interface{}{
				"GroupName":   *sg.GroupName,
				"Description": *sg.Description,
				"VpcId":       sg.VpcId,
			},
			LastSeen: time.Now(),
		}

		// Convert tags
		for _, tag := range sg.Tags {
			resource.Tags[*tag.Key] = *tag.Value
		}

		resources = append(resources, resource)
	}

	return resources, nil
}

// Get retrieves a specific security group
func (s *SecurityGroupAdapter) Get(ctx context.Context, id string) (*types.AWSResource, error) {
	sg, err := s.client.GetSecurityGroup(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get security group %s: %w", id, err)
	}

	resource := &types.AWSResource{
		ID:     *sg.GroupId,
		Type:   "security-group",
		Region: s.client.GetRegion(),
		State:  "available",
		Tags:   make(map[string]string),
		Details: map[string]interface{}{
			"GroupName":   *sg.GroupName,
			"Description": *sg.Description,
			"VpcId":       sg.VpcId,
		},
		LastSeen: time.Now(),
	}

	// Convert tags
	for _, tag := range sg.Tags {
		resource.Tags[*tag.Key] = *tag.Value
	}

	return resource, nil
}

// Update updates a security group (limited operations available)
func (s *SecurityGroupAdapter) Update(ctx context.Context, id string, params interface{}) (*types.AWSResource, error) {
	// Security groups have limited update operations (mainly rule management)
	return nil, fmt.Errorf("security group updates not supported via standard interface, use specialized operations for rule management")
}

// Delete deletes a security group
func (s *SecurityGroupAdapter) Delete(ctx context.Context, id string) error {
	err := s.client.DeleteSecurityGroup(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to delete security group %s: %w", id, err)
	}

	s.logger.Infof("Deleted security group %s", id)
	return nil
}

// GetSupportedOperations returns the operations supported by this adapter
func (s *SecurityGroupAdapter) GetSupportedOperations() []string {
	return []string{
		"create",
		"list",
		"get",
		"delete",
	}
}

// ValidateParams validates security group-specific parameters
func (s *SecurityGroupAdapter) ValidateParams(operation string, params interface{}) error {
	switch operation {
	case "create":
		createParams, ok := params.(aws.SecurityGroupParams)
		if !ok {
			return fmt.Errorf("invalid parameters for create operation")
		}
		if createParams.GroupName == "" {
			return fmt.Errorf("groupName is required for security group creation")
		}
		if createParams.Description == "" {
			return fmt.Errorf("description is required for security group creation")
		}
		return nil
	case "get", "delete":
		if params == nil {
			return fmt.Errorf("security group ID is required for %s operation", operation)
		}
		return nil
	default:
		return fmt.Errorf("unsupported operation: %s", operation)
	}
}

// ExecuteSpecialOperation executes security group-specific operations
func (s *SecurityGroupAdapter) ExecuteSpecialOperation(ctx context.Context, operation string, params interface{}) (*types.AWSResource, error) {
	switch operation {
	case "add-ingress-rule":
		ruleParams, ok := params.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("add ingress rule parameters required")
		}

		groupID, _ := ruleParams["groupId"].(string)
		protocol, _ := ruleParams["protocol"].(string)
		fromPort, _ := ruleParams["fromPort"].(int)
		toPort, _ := ruleParams["toPort"].(int)
		cidrBlock, _ := ruleParams["cidrBlock"].(string)
		sourceSecurityGroupId, _ := ruleParams["sourceSecurityGroupId"].(string)

		if groupID == "" || protocol == "" {
			return nil, fmt.Errorf("groupId and protocol are required for add ingress rule operation")
		}

		if cidrBlock == "" && sourceSecurityGroupId == "" {
			return nil, fmt.Errorf("either cidrBlock or sourceSecurityGroupId is required for add ingress rule operation")
		}

		if cidrBlock != "" && sourceSecurityGroupId != "" {
			return nil, fmt.Errorf("cannot specify both cidrBlock and sourceSecurityGroupId - use one or the other")
		}

		// Create rule parameters
		sgRuleParams := aws.SecurityGroupRuleParams{
			GroupID:  groupID,
			Type:     "ingress",
			Protocol: protocol,
			FromPort: int32(fromPort),
			ToPort:   int32(toPort),
		}

		// Add either CIDR block or source security group
		if cidrBlock != "" {
			sgRuleParams.CidrBlocks = []string{cidrBlock}
		} else {
			sgRuleParams.SourceSG = sourceSecurityGroupId
		}

		err := s.client.AddSecurityGroupRule(ctx, sgRuleParams)
		if err != nil {
			return nil, fmt.Errorf("failed to add ingress rule: %w", err)
		}

		// Prepare details for response
		details := map[string]interface{}{
			"direction": "ingress",
			"protocol":  protocol,
			"fromPort":  fromPort,
			"toPort":    toPort,
		}

		if cidrBlock != "" {
			details["cidrBlock"] = cidrBlock
		} else {
			details["sourceSecurityGroupId"] = sourceSecurityGroupId
		}

		// Return result resource
		return &types.AWSResource{
			ID:      groupID,
			Type:    "security-group-rule",
			State:   "available",
			Details: details,
		}, nil

	case "add-egress-rule":
		ruleParams, ok := params.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("add egress rule parameters required")
		}

		groupID, _ := ruleParams["groupId"].(string)
		protocol, _ := ruleParams["protocol"].(string)
		fromPort, _ := ruleParams["fromPort"].(int)
		toPort, _ := ruleParams["toPort"].(int)
		cidrBlock, _ := ruleParams["cidrBlock"].(string)
		sourceSecurityGroupId, _ := ruleParams["sourceSecurityGroupId"].(string)

		if groupID == "" || protocol == "" {
			return nil, fmt.Errorf("groupId and protocol are required for add egress rule operation")
		}

		if cidrBlock == "" && sourceSecurityGroupId == "" {
			return nil, fmt.Errorf("either cidrBlock or sourceSecurityGroupId is required for add egress rule operation")
		}

		if cidrBlock != "" && sourceSecurityGroupId != "" {
			return nil, fmt.Errorf("cannot specify both cidrBlock and sourceSecurityGroupId - use one or the other")
		}

		// Create rule parameters
		sgRuleParams := aws.SecurityGroupRuleParams{
			GroupID:  groupID,
			Type:     "egress",
			Protocol: protocol,
			FromPort: int32(fromPort),
			ToPort:   int32(toPort),
		}

		// Add either CIDR block or source security group
		if cidrBlock != "" {
			sgRuleParams.CidrBlocks = []string{cidrBlock}
		} else {
			sgRuleParams.SourceSG = sourceSecurityGroupId
		}

		err := s.client.AddSecurityGroupRule(ctx, sgRuleParams)
		if err != nil {
			return nil, fmt.Errorf("failed to add egress rule: %w", err)
		}

		// Prepare details for response
		details := map[string]interface{}{
			"direction": "egress",
			"protocol":  protocol,
			"fromPort":  fromPort,
			"toPort":    toPort,
		}

		if cidrBlock != "" {
			details["cidrBlock"] = cidrBlock
		} else {
			details["sourceSecurityGroupId"] = sourceSecurityGroupId
		}

		// Return result resource
		return &types.AWSResource{
			ID:      groupID,
			Type:    "security-group-rule",
			State:   "available",
			Details: details,
		}, nil

	case "delete-security-group":
		groupID, ok := params.(string)
		if !ok {
			return nil, fmt.Errorf("security group ID required for delete operation")
		}

		err := s.client.DeleteSecurityGroup(ctx, groupID)
		if err != nil {
			return nil, fmt.Errorf("failed to delete security group: %w", err)
		}

		// Return result resource
		return &types.AWSResource{
			ID:    groupID,
			Type:  "security-group",
			State: "deleted",
			Details: map[string]interface{}{
				"status": "deleted",
			},
		}, nil

	default:
		return nil, fmt.Errorf("unsupported special operation: %s", operation)
	}
}

// GetSpecialOperations returns the list of supported special operations
func (s *SecurityGroupAdapter) GetSpecialOperations() []string {
	return []string{
		"add-ingress-rule",
		"add-egress-rule",
		"delete-security-group",
	}
}

// NewSecurityGroupSpecializedAdapter creates a Security Group adapter with specialized operations
func NewSecurityGroupSpecializedAdapter(client *aws.Client, logger *logging.Logger) interfaces.SpecializedOperations {
	return &SecurityGroupAdapter{
		BaseAWSAdapter: NewBaseAWSAdapter(client, logger, "security-group"),
		client:         client,
	}
}
