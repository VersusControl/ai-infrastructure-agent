package discovery

import (
	"context"
	"fmt"
	"time"

	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/aws"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// Scanner handles infrastructure discovery and scanning
type Scanner struct {
	awsClient *aws.Client
	logger    *logging.Logger
}

// NewScanner creates a new infrastructure scanner
func NewScanner(awsClient *aws.Client, logger *logging.Logger) *Scanner {
	return &Scanner{
		awsClient: awsClient,
		logger:    logger,
	}
}

// DiscoverInfrastructure performs a comprehensive scan of existing infrastructure
func (s *Scanner) DiscoverInfrastructure(ctx context.Context) ([]*types.ResourceState, error) {
	s.logger.Info("Starting infrastructure discovery")

	var resources []*types.ResourceState

	// Discover VPCs
	vpcs, err := s.discoverVPCs(ctx)
	if err != nil {
		s.logger.WithError(err).Error("Failed to discover VPCs")
		return nil, err
	}
	resources = append(resources, vpcs...)

	// Discover EC2 instances
	instances, err := s.discoverEC2Instances(ctx)
	if err != nil {
		s.logger.WithError(err).Error("Failed to discover EC2 instances")
		return nil, err
	}
	resources = append(resources, instances...)

	// Discover Security Groups
	securityGroups, err := s.discoverSecurityGroups(ctx)
	if err != nil {
		s.logger.WithError(err).Error("Failed to discover security groups")
		return nil, err
	}
	resources = append(resources, securityGroups...)

	// Discover Load Balancers
	loadBalancers, err := s.discoverLoadBalancers(ctx)
	if err != nil {
		s.logger.WithError(err).Error("Failed to discover load balancers")
		return nil, err
	}
	resources = append(resources, loadBalancers...)

	// Discover Auto Scaling Groups
	autoScalingGroups, err := s.discoverAutoScalingGroups(ctx)
	if err != nil {
		s.logger.WithError(err).Error("Failed to discover auto scaling groups")
		return nil, err
	}
	resources = append(resources, autoScalingGroups...)

	// Discover Subnets
	subnets, err := s.discoverSubnets(ctx)
	if err != nil {
		s.logger.WithError(err).Error("Failed to discover subnets")
		return nil, err
	}
	resources = append(resources, subnets...)

	// Discover NAT Gateways
	natGateways, err := s.discoverNATGateways(ctx)
	if err != nil {
		s.logger.WithError(err).Error("Failed to discover NAT gateways")
		return nil, err
	}
	resources = append(resources, natGateways...)

	// Discover Target Groups
	targetGroups, err := s.discoverTargetGroups(ctx)
	if err != nil {
		s.logger.WithError(err).Error("Failed to discover target groups")
		return nil, err
	}
	resources = append(resources, targetGroups...)

	// Discover RDS DB Instances
	rdsInstances, err := s.discoverRDSInstances(ctx)
	if err != nil {
		s.logger.WithError(err).Error("Failed to discover RDS instances")
		return nil, err
	}
	resources = append(resources, rdsInstances...)

	// Discover Launch Templates
	launchTemplates, err := s.discoverLaunchTemplates(ctx)
	if err != nil {
		s.logger.WithError(err).Error("Failed to discover launch templates")
		return nil, err
	}
	resources = append(resources, launchTemplates...)

	s.logger.WithField("resource_count", len(resources)).Info("Infrastructure discovery completed")
	return resources, nil
}

// discoverVPCs discovers all VPCs in the region
func (s *Scanner) discoverVPCs(ctx context.Context) ([]*types.ResourceState, error) {
	s.logger.Debug("Discovering VPCs")

	vpcs, err := s.awsClient.DescribeVPCs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to describe VPCs: %w", err)
	}

	var resources []*types.ResourceState
	for _, vpc := range vpcs {
		name := ""
		if nameVal, exists := vpc.Details["name"]; exists && nameVal != nil {
			name = nameVal.(string)
		}

		resource := &types.ResourceState{
			ID:           vpc.ID,
			Name:         name,
			Type:         "vpc",
			Status:       vpc.State,
			DesiredState: "available",
			CurrentState: vpc.State,
			Tags:         vpc.Tags,
			Properties:   vpc.Details,
			Dependencies: []string{},
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		resources = append(resources, resource)
	}

	s.logger.WithField("vpc_count", len(resources)).Debug("VPC discovery completed")
	return resources, nil
}

// discoverEC2Instances discovers all EC2 instances in the region
func (s *Scanner) discoverEC2Instances(ctx context.Context) ([]*types.ResourceState, error) {
	s.logger.Debug("Discovering EC2 instances")

	instances, err := s.awsClient.DescribeInstances(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to describe instances: %w", err)
	}

	var resources []*types.ResourceState
	for _, instance := range instances {
		// Skip terminated instances
		if instance.State == "terminated" {
			continue
		}

		name := ""
		if nameVal, exists := instance.Details["name"]; exists && nameVal != nil {
			name = nameVal.(string)
		}

		var dependencies []string
		if vpcID, exists := instance.Details["vpcId"]; exists && vpcID != nil {
			dependencies = append(dependencies, vpcID.(string))
		}
		if subnetID, exists := instance.Details["subnetId"]; exists && subnetID != nil {
			dependencies = append(dependencies, subnetID.(string))
		}

		resource := &types.ResourceState{
			ID:           instance.ID,
			Name:         name,
			Type:         "ec2_instance",
			Status:       instance.State,
			DesiredState: "running",
			CurrentState: instance.State,
			Tags:         instance.Tags,
			Properties:   instance.Details,
			Dependencies: dependencies,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		resources = append(resources, resource)
	}

	s.logger.WithField("instance_count", len(resources)).Debug("EC2 instance discovery completed")
	return resources, nil
}

// discoverSecurityGroups discovers all security groups in the region
func (s *Scanner) discoverSecurityGroups(ctx context.Context) ([]*types.ResourceState, error) {
	s.logger.Debug("Discovering security groups")

	// Get all VPCs first to discover security groups across all VPCs
	vpcs, err := s.awsClient.DescribeVPCs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to describe VPCs for security group discovery: %w", err)
	}

	var resources []*types.ResourceState
	for _, vpc := range vpcs {
		securityGroups, err := s.awsClient.ListSecurityGroups(ctx, vpc.ID)
		if err != nil {
			s.logger.WithError(err).WithField("vpc_id", vpc.ID).Warn("Failed to list security groups for VPC")
			continue
		}

		for _, sg := range securityGroups {
			// Convert AWS SDK SecurityGroup to our ResourceState
			name := ""
			if sg.GroupName != nil {
				name = *sg.GroupName
			}

			tags := make(map[string]string)
			for _, tag := range sg.Tags {
				if tag.Key != nil && tag.Value != nil {
					tags[*tag.Key] = *tag.Value
				}
			}

			// Convert ingress rules
			ingressRules := make([]map[string]interface{}, len(sg.IpPermissions))
			for i, rule := range sg.IpPermissions {
				var ipRanges []string
				for _, r := range rule.IpRanges {
					if r.CidrIp != nil {
						ipRanges = append(ipRanges, *r.CidrIp)
					}
				}

				ingressRules[i] = map[string]interface{}{
					"from_port":   rule.FromPort,
					"to_port":     rule.ToPort,
					"ip_protocol": *rule.IpProtocol,
					"ip_ranges":   ipRanges,
				}
			}

			// Convert egress rules
			egressRules := make([]map[string]interface{}, len(sg.IpPermissionsEgress))
			for i, rule := range sg.IpPermissionsEgress {
				var ipRanges []string
				for _, r := range rule.IpRanges {
					if r.CidrIp != nil {
						ipRanges = append(ipRanges, *r.CidrIp)
					}
				}

				egressRules[i] = map[string]interface{}{
					"from_port":   rule.FromPort,
					"to_port":     rule.ToPort,
					"ip_protocol": *rule.IpProtocol,
					"ip_ranges":   ipRanges,
				}
			}

			vpcID := ""
			if sg.VpcId != nil {
				vpcID = *sg.VpcId
			}

			description := ""
			if sg.Description != nil {
				description = *sg.Description
			}

			resource := &types.ResourceState{
				ID:           *sg.GroupId,
				Name:         name,
				Type:         "security_group",
				Status:       "active",
				DesiredState: "active",
				CurrentState: "active",
				Tags:         tags,
				Properties: map[string]interface{}{
					"group_name":    name,
					"description":   description,
					"vpc_id":        vpcID,
					"ingress_rules": ingressRules,
					"egress_rules":  egressRules,
				},
				Dependencies: []string{vpcID},
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
			}

			resources = append(resources, resource)
		}
	}

	s.logger.WithField("security_group_count", len(resources)).Debug("Security group discovery completed")
	return resources, nil
}

// discoverLoadBalancers discovers all application load balancers
func (s *Scanner) discoverLoadBalancers(ctx context.Context) ([]*types.ResourceState, error) {
	s.logger.Debug("Discovering load balancers")

	loadBalancers, err := s.awsClient.DescribeLoadBalancers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to describe load balancers: %w", err)
	}

	var resources []*types.ResourceState
	for _, lb := range loadBalancers {
		name := ""
		if nameVal, exists := lb.Details["name"]; exists && nameVal != nil {
			name = nameVal.(string)
		}

		var dependencies []string
		if vpcID, exists := lb.Details["vpcId"]; exists && vpcID != nil {
			dependencies = append(dependencies, vpcID.(string))
		}

		// Add subnet dependencies
		if subnets, exists := lb.Details["subnets"]; exists && subnets != nil {
			if subnetList, ok := subnets.([]string); ok {
				dependencies = append(dependencies, subnetList...)
			}
		}

		// Add security group dependencies
		if securityGroups, exists := lb.Details["securityGroups"]; exists && securityGroups != nil {
			if sgList, ok := securityGroups.([]string); ok {
				dependencies = append(dependencies, sgList...)
			}
		}

		resource := &types.ResourceState{
			ID:           lb.ID,
			Name:         name,
			Type:         "load_balancer",
			Status:       lb.State,
			DesiredState: "active",
			CurrentState: lb.State,
			Tags:         lb.Tags,
			Properties:   lb.Details,
			Dependencies: dependencies,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		resources = append(resources, resource)
	}

	s.logger.WithField("load_balancer_count", len(resources)).Debug("Load balancer discovery completed")
	return resources, nil
}

// discoverAutoScalingGroups discovers all auto scaling groups
func (s *Scanner) discoverAutoScalingGroups(ctx context.Context) ([]*types.ResourceState, error) {
	s.logger.Debug("Discovering auto scaling groups")

	autoScalingGroups, err := s.awsClient.DescribeAutoScalingGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to describe auto scaling groups: %w", err)
	}

	var resources []*types.ResourceState
	for _, asg := range autoScalingGroups {
		name := ""
		if nameVal, exists := asg.Details["name"]; exists && nameVal != nil {
			name = nameVal.(string)
		}

		var dependencies []string

		// Add load balancer dependencies
		if loadBalancers, exists := asg.Details["loadBalancerNames"]; exists && loadBalancers != nil {
			if lbList, ok := loadBalancers.([]string); ok {
				dependencies = append(dependencies, lbList...)
			}
		}

		// Add target group dependencies
		if targetGroups, exists := asg.Details["targetGroupARNs"]; exists && targetGroups != nil {
			if tgList, ok := targetGroups.([]string); ok {
				dependencies = append(dependencies, tgList...)
			}
		}

		resource := &types.ResourceState{
			ID:           asg.ID,
			Name:         name,
			Type:         "auto_scaling_group",
			Status:       "active",
			DesiredState: "active",
			CurrentState: "active",
			Tags:         asg.Tags,
			Properties:   asg.Details,
			Dependencies: dependencies,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		resources = append(resources, resource)
	}

	s.logger.WithField("auto_scaling_group_count", len(resources)).Debug("Auto scaling group discovery completed")
	return resources, nil
}

// discoverSubnets discovers all subnets across all VPCs
func (s *Scanner) discoverSubnets(ctx context.Context) ([]*types.ResourceState, error) {
	s.logger.Debug("Discovering subnets")

	subnets, err := s.awsClient.DescribeSubnetsAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to describe subnets: %w", err)
	}

	var resources []*types.ResourceState
	for _, subnet := range subnets {
		name := ""
		if nameVal, exists := subnet.Details["name"]; exists && nameVal != nil {
			name = nameVal.(string)
		}

		var dependencies []string
		if vpcID, exists := subnet.Details["vpcId"]; exists && vpcID != nil {
			dependencies = append(dependencies, vpcID.(string))
		}

		resource := &types.ResourceState{
			ID:           subnet.ID,
			Name:         name,
			Type:         "subnet",
			Status:       subnet.State,
			DesiredState: "available",
			CurrentState: subnet.State,
			Tags:         subnet.Tags,
			Properties:   subnet.Details,
			Dependencies: dependencies,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		resources = append(resources, resource)
	}

	s.logger.WithField("subnet_count", len(resources)).Debug("Subnet discovery completed")
	return resources, nil
}

// discoverNATGateways discovers all NAT gateways
func (s *Scanner) discoverNATGateways(ctx context.Context) ([]*types.ResourceState, error) {
	s.logger.Debug("Discovering NAT gateways")

	// Pass empty slice to get all NAT gateways
	natGateways, err := s.awsClient.DescribeNATGateways(ctx, []string{})
	if err != nil {
		return nil, fmt.Errorf("failed to describe NAT gateways: %w", err)
	}

	var resources []*types.ResourceState
	for _, natGateway := range natGateways {
		name := ""
		if nameVal, exists := natGateway.Details["name"]; exists && nameVal != nil {
			name = nameVal.(string)
		}

		var dependencies []string
		if vpcID, exists := natGateway.Details["vpcId"]; exists && vpcID != nil {
			dependencies = append(dependencies, vpcID.(string))
		}
		if subnetID, exists := natGateway.Details["subnetId"]; exists && subnetID != nil {
			dependencies = append(dependencies, subnetID.(string))
		}

		resource := &types.ResourceState{
			ID:           natGateway.ID,
			Name:         name,
			Type:         "nat_gateway",
			Status:       natGateway.State,
			DesiredState: "available",
			CurrentState: natGateway.State,
			Tags:         natGateway.Tags,
			Properties:   natGateway.Details,
			Dependencies: dependencies,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		resources = append(resources, resource)
	}

	s.logger.WithField("nat_gateway_count", len(resources)).Debug("NAT gateway discovery completed")
	return resources, nil
}

// discoverTargetGroups discovers all target groups
func (s *Scanner) discoverTargetGroups(ctx context.Context) ([]*types.ResourceState, error) {
	s.logger.Debug("Discovering target groups")

	targetGroups, err := s.awsClient.DescribeTargetGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to describe target groups: %w", err)
	}

	var resources []*types.ResourceState
	for _, tg := range targetGroups {
		name := ""
		if nameVal, exists := tg.Details["name"]; exists && nameVal != nil {
			name = nameVal.(string)
		}

		var dependencies []string
		if vpcID, exists := tg.Details["vpcId"]; exists && vpcID != nil {
			dependencies = append(dependencies, vpcID.(string))
		}

		resource := &types.ResourceState{
			ID:           tg.ID,
			Name:         name,
			Type:         "target_group",
			Status:       "active",
			DesiredState: "active",
			CurrentState: "active",
			Tags:         tg.Tags,
			Properties:   tg.Details,
			Dependencies: dependencies,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		resources = append(resources, resource)
	}

	s.logger.WithField("target_group_count", len(resources)).Debug("Target group discovery completed")
	return resources, nil
}

// discoverRDSInstances discovers all RDS database instances
func (s *Scanner) discoverRDSInstances(ctx context.Context) ([]*types.ResourceState, error) {
	s.logger.Debug("Discovering RDS instances")

	rdsInstances, err := s.awsClient.ListDBInstances(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list RDS instances: %w", err)
	}

	var resources []*types.ResourceState
	for _, rds := range rdsInstances {
		name := ""
		if nameVal, exists := rds.Details["name"]; exists && nameVal != nil {
			name = nameVal.(string)
		}

		var dependencies []string
		if vpcID, exists := rds.Details["vpcId"]; exists && vpcID != nil {
			dependencies = append(dependencies, vpcID.(string))
		}

		// Add subnet group dependencies if available
		if subnetGroup, exists := rds.Details["dbSubnetGroup"]; exists && subnetGroup != nil {
			if subnetGroupMap, ok := subnetGroup.(map[string]interface{}); ok {
				if subnets, ok := subnetGroupMap["subnets"].([]interface{}); ok {
					for _, subnet := range subnets {
						if subnetMap, ok := subnet.(map[string]interface{}); ok {
							if subnetID, ok := subnetMap["subnetIdentifier"].(string); ok {
								dependencies = append(dependencies, subnetID)
							}
						}
					}
				}
			}
		}

		// Add security group dependencies
		if securityGroups, exists := rds.Details["vpcSecurityGroups"]; exists && securityGroups != nil {
			if sgList, ok := securityGroups.([]interface{}); ok {
				for _, sg := range sgList {
					if sgMap, ok := sg.(map[string]interface{}); ok {
						if sgID, ok := sgMap["vpcSecurityGroupId"].(string); ok {
							dependencies = append(dependencies, sgID)
						}
					}
				}
			}
		}

		status := "unknown"
		if statusVal, exists := rds.Details["status"]; exists && statusVal != nil {
			status = statusVal.(string)
		}

		resource := &types.ResourceState{
			ID:           rds.ID,
			Name:         name,
			Type:         "rds_instance",
			Status:       status,
			DesiredState: "available",
			CurrentState: status,
			Tags:         rds.Tags,
			Properties:   rds.Details,
			Dependencies: dependencies,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		resources = append(resources, resource)
	}

	s.logger.WithField("rds_instance_count", len(resources)).Debug("RDS instance discovery completed")
	return resources, nil
}

// discoverLaunchTemplates discovers all EC2 launch templates
func (s *Scanner) discoverLaunchTemplates(ctx context.Context) ([]*types.ResourceState, error) {
	s.logger.Debug("Discovering launch templates")

	launchTemplates, err := s.awsClient.DescribeLaunchTemplates(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to describe launch templates: %w", err)
	}

	var resources []*types.ResourceState
	for _, lt := range launchTemplates {
		name := ""
		if nameVal, exists := lt.Details["name"]; exists && nameVal != nil {
			name = nameVal.(string)
		}

		resource := &types.ResourceState{
			ID:           lt.ID,
			Name:         name,
			Type:         "launch_template",
			Status:       "active",
			DesiredState: "active",
			CurrentState: "active",
			Tags:         lt.Tags,
			Properties:   lt.Details,
			Dependencies: []string{},
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		resources = append(resources, resource)
	}

	s.logger.WithField("launch_template_count", len(resources)).Debug("Launch template discovery completed")
	return resources, nil
}
