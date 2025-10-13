package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/versus-control/ai-infrastructure-agent/pkg/types"

	"github.com/sirupsen/logrus"
)

// ========== VPC Management Methods ==========

// CreateVPC creates a new VPC with the specified parameters
func (c *Client) CreateVPC(ctx context.Context, params CreateVPCParams) (*types.AWSResource, error) {
	input := &ec2.CreateVpcInput{
		CidrBlock: aws.String(params.CidrBlock),
	}

	// Add tag specifications during creation
	if params.Name != "" || len(params.Tags) > 0 {
		tags := make(map[string]string)
		for k, v := range params.Tags {
			tags[k] = v
		}
		if params.Name != "" {
			tags["Name"] = params.Name
		}

		var ec2Tags []ec2types.Tag
		for key, value := range tags {
			ec2Tags = append(ec2Tags, ec2types.Tag{
				Key:   aws.String(key),
				Value: aws.String(value),
			})
		}

		input.TagSpecifications = []ec2types.TagSpecification{
			{
				ResourceType: ec2types.ResourceTypeVpc,
				Tags:         ec2Tags,
			},
		}
	}

	result, err := c.ec2.CreateVpc(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to create VPC: %w", err)
	}

	vpcID := *result.Vpc.VpcId
	c.logger.WithField("vpcId", vpcID).Info("VPC created successfully")

	// Enable DNS hostnames and support if requested
	if params.EnableDnsHostnames {
		_, err = c.ec2.ModifyVpcAttribute(ctx, &ec2.ModifyVpcAttributeInput{
			VpcId:              aws.String(vpcID),
			EnableDnsHostnames: &ec2types.AttributeBooleanValue{Value: aws.Bool(true)},
		})
		if err != nil {
			c.logger.WithError(err).Warn("Failed to enable DNS hostnames")
		}
	}

	if params.EnableDnsSupport {
		_, err = c.ec2.ModifyVpcAttribute(ctx, &ec2.ModifyVpcAttributeInput{
			VpcId:            aws.String(vpcID),
			EnableDnsSupport: &ec2types.AttributeBooleanValue{Value: aws.Bool(true)},
		})
		if err != nil {
			c.logger.WithError(err).Warn("Failed to enable DNS support")
		}
	}

	resource := &types.AWSResource{
		ID:    vpcID,
		Type:  "vpc",
		State: "available",
		Details: map[string]interface{}{
			"cidrBlock": params.CidrBlock,
		},
		LastSeen: time.Now(),
	}

	return resource, nil
}

// CreateSubnet creates a subnet in the specified VPC
func (c *Client) CreateSubnet(ctx context.Context, params CreateSubnetParams) (*types.AWSResource, error) {
	input := &ec2.CreateSubnetInput{
		VpcId:            aws.String(params.VpcID),
		CidrBlock:        aws.String(params.CidrBlock),
		AvailabilityZone: aws.String(params.AvailabilityZone),
	}

	// Add tag specifications during creation
	if params.Name != "" || len(params.Tags) > 0 {
		tags := make(map[string]string)
		for k, v := range params.Tags {
			tags[k] = v
		}
		if params.Name != "" {
			tags["Name"] = params.Name
		}

		var ec2Tags []ec2types.Tag
		for key, value := range tags {
			ec2Tags = append(ec2Tags, ec2types.Tag{
				Key:   aws.String(key),
				Value: aws.String(value),
			})
		}

		input.TagSpecifications = []ec2types.TagSpecification{
			{
				ResourceType: ec2types.ResourceTypeSubnet,
				Tags:         ec2Tags,
			},
		}
	}

	result, err := c.ec2.CreateSubnet(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to create subnet: %w", err)
	}

	subnetID := *result.Subnet.SubnetId
	c.logger.WithFields(logrus.Fields{
		"subnetId": subnetID,
		"vpcId":    params.VpcID,
		"az":       params.AvailabilityZone,
	}).Info("Subnet created successfully")

	// Enable auto-assign public IP if requested
	if params.MapPublicIpOnLaunch {
		_, err = c.ec2.ModifySubnetAttribute(ctx, &ec2.ModifySubnetAttributeInput{
			SubnetId:            aws.String(subnetID),
			MapPublicIpOnLaunch: &ec2types.AttributeBooleanValue{Value: aws.Bool(true)},
		})
		if err != nil {
			c.logger.WithError(err).Warn("Failed to enable auto-assign public IP")
		}
	}

	resource := &types.AWSResource{
		ID:    subnetID,
		Type:  "subnet",
		State: "available",
		Details: map[string]interface{}{
			"vpcId":     params.VpcID,
			"cidrBlock": params.CidrBlock,
			"az":        params.AvailabilityZone,
			"isPublic":  params.MapPublicIpOnLaunch,
		},
		LastSeen: time.Now(),
	}

	return resource, nil
}

// CreateInternetGateway creates an internet gateway and attaches it to a VPC
func (c *Client) CreateInternetGateway(ctx context.Context, params CreateInternetGatewayParams, vpcID string) (*types.AWSResource, error) {
	input := &ec2.CreateInternetGatewayInput{}

	// Add tag specifications during creation
	if params.Name != "" || len(params.Tags) > 0 {
		tags := make(map[string]string)
		for k, v := range params.Tags {
			tags[k] = v
		}
		if params.Name != "" {
			tags["Name"] = params.Name
		}

		var ec2Tags []ec2types.Tag
		for key, value := range tags {
			ec2Tags = append(ec2Tags, ec2types.Tag{
				Key:   aws.String(key),
				Value: aws.String(value),
			})
		}

		input.TagSpecifications = []ec2types.TagSpecification{
			{
				ResourceType: ec2types.ResourceTypeInternetGateway,
				Tags:         ec2Tags,
			},
		}
	}

	// Create Internet Gateway
	createResult, err := c.ec2.CreateInternetGateway(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to create internet gateway: %w", err)
	}

	igwID := *createResult.InternetGateway.InternetGatewayId
	c.logger.WithField("igwId", igwID).Info("Internet Gateway created successfully")

	// Attach to VPC
	_, err = c.ec2.AttachInternetGateway(ctx, &ec2.AttachInternetGatewayInput{
		InternetGatewayId: aws.String(igwID),
		VpcId:             aws.String(vpcID),
	})
	if err != nil {
		// Try to clean up the created IGW
		c.ec2.DeleteInternetGateway(ctx, &ec2.DeleteInternetGatewayInput{
			InternetGatewayId: aws.String(igwID),
		})
		return nil, fmt.Errorf("failed to attach internet gateway to VPC: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"igwId": igwID,
		"vpcId": vpcID,
	}).Info("Internet Gateway attached to VPC")

	resource := &types.AWSResource{
		ID:    igwID,
		Type:  "internet-gateway",
		State: "attached",
		Details: map[string]interface{}{
			"vpcId": vpcID,
		},
		LastSeen: time.Now(),
	}

	return resource, nil
}

// CreateRouteTable creates a route table for the VPC
func (c *Client) CreateRouteTable(ctx context.Context, vpcID, name string) (*types.AWSResource, error) {
	input := &ec2.CreateRouteTableInput{
		VpcId: aws.String(vpcID),
	}

	// Add tag specifications during creation
	if name != "" {
		var ec2Tags []ec2types.Tag
		ec2Tags = append(ec2Tags, ec2types.Tag{
			Key:   aws.String("Name"),
			Value: aws.String(name),
		})

		input.TagSpecifications = []ec2types.TagSpecification{
			{
				ResourceType: ec2types.ResourceTypeRouteTable,
				Tags:         ec2Tags,
			},
		}
	}

	result, err := c.ec2.CreateRouteTable(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to create route table: %w", err)
	}

	routeTableID := *result.RouteTable.RouteTableId
	c.logger.WithFields(logrus.Fields{
		"routeTableId": routeTableID,
		"vpcId":        vpcID,
	}).Info("Route table created successfully")

	resource := &types.AWSResource{
		ID:    routeTableID,
		Type:  "route-table",
		State: "available",
		Details: map[string]interface{}{
			"vpcId": vpcID,
			"name":  name,
		},
		LastSeen: time.Now(),
	}

	return resource, nil
}

// CreateRoute creates a route in the specified route table
func (c *Client) CreateRoute(ctx context.Context, routeTableID, destinationCidr, gatewayID string) error {
	input := &ec2.CreateRouteInput{
		RouteTableId:         aws.String(routeTableID),
		DestinationCidrBlock: aws.String(destinationCidr),
		GatewayId:            aws.String(gatewayID),
	}

	_, err := c.ec2.CreateRoute(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to create route: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"routeTableId": routeTableID,
		"destination":  destinationCidr,
		"gatewayId":    gatewayID,
	}).Info("Route created successfully")

	return nil
}

// AssociateRouteTable associates a route table with a subnet
func (c *Client) AssociateRouteTable(ctx context.Context, routeTableID, subnetID string) error {
	input := &ec2.AssociateRouteTableInput{
		RouteTableId: aws.String(routeTableID),
		SubnetId:     aws.String(subnetID),
	}

	_, err := c.ec2.AssociateRouteTable(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to associate route table: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"routeTableId": routeTableID,
		"subnetId":     subnetID,
	}).Info("Route table associated with subnet")

	return nil
}

// CreateNATGateway creates a NAT Gateway in the specified public subnet
func (c *Client) CreateNATGateway(ctx context.Context, params CreateNATGatewayParams) (*types.AWSResource, error) {
	// First, allocate an Elastic IP
	eipResult, err := c.ec2.AllocateAddress(ctx, &ec2.AllocateAddressInput{
		Domain: ec2types.DomainTypeVpc,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to allocate Elastic IP: %w", err)
	}

	eipID := *eipResult.AllocationId
	c.logger.WithField("eipId", eipID).Info("Elastic IP allocated for NAT Gateway")

	// Create the NAT Gateway
	input := &ec2.CreateNatGatewayInput{
		SubnetId:     aws.String(params.SubnetID),
		AllocationId: aws.String(eipID),
	}

	// Add tag specifications during creation
	tags := make(map[string]string)
	if params.Name != "" || len(params.Tags) > 0 {
		for k, v := range params.Tags {
			tags[k] = v
		}
		if params.Name != "" {
			tags["Name"] = params.Name
		}

		var ec2Tags []ec2types.Tag
		for key, value := range tags {
			ec2Tags = append(ec2Tags, ec2types.Tag{
				Key:   aws.String(key),
				Value: aws.String(value),
			})
		}

		input.TagSpecifications = []ec2types.TagSpecification{
			{
				ResourceType: ec2types.ResourceTypeNatgateway,
				Tags:         ec2Tags,
			},
		}
	}

	result, err := c.ec2.CreateNatGateway(ctx, input)
	if err != nil {
		// Clean up the EIP if NAT Gateway creation fails
		_, cleanupErr := c.ec2.ReleaseAddress(ctx, &ec2.ReleaseAddressInput{
			AllocationId: aws.String(eipID),
		})
		if cleanupErr != nil {
			c.logger.WithError(cleanupErr).Warn("Failed to clean up Elastic IP after NAT Gateway creation failure")
		}
		return nil, fmt.Errorf("failed to create NAT Gateway: %w", err)
	}

	natGatewayID := *result.NatGateway.NatGatewayId
	c.logger.WithFields(logrus.Fields{
		"natGatewayId": natGatewayID,
		"subnetId":     params.SubnetID,
		"eipId":        eipID,
	}).Info("NAT Gateway created successfully")

	resource := &types.AWSResource{
		ID:    natGatewayID,
		Type:  "nat-gateway",
		State: "pending",
		Details: map[string]interface{}{
			"subnetId":         params.SubnetID,
			"eipId":            eipID,
			"publicIp":         aws.ToString(eipResult.PublicIp),
			"privateIp":        aws.ToString(result.NatGateway.NatGatewayAddresses[0].PrivateIp),
			"connectivityType": string(result.NatGateway.ConnectivityType),
		},
		Tags:     tags,
		LastSeen: time.Now(),
	}

	return resource, nil
}

// WaitForNATGateway waits for a NAT Gateway to become available
func (c *Client) WaitForNATGateway(ctx context.Context, natGatewayID string) error {
	maxWaitTime := 10 * time.Minute
	pollInterval := 30 * time.Second

	ctxWithTimeout, cancel := context.WithTimeout(ctx, maxWaitTime)
	defer cancel()

	for {
		select {
		case <-ctxWithTimeout.Done():
			return fmt.Errorf("timeout waiting for NAT Gateway %s to become available", natGatewayID)
		default:
			result, err := c.ec2.DescribeNatGateways(ctx, &ec2.DescribeNatGatewaysInput{
				NatGatewayIds: []string{natGatewayID},
			})
			if err != nil {
				return fmt.Errorf("failed to describe NAT Gateway %s: %w", natGatewayID, err)
			}

			if len(result.NatGateways) == 0 {
				return fmt.Errorf("NAT Gateway %s not found", natGatewayID)
			}

			state := result.NatGateways[0].State
			c.logger.WithFields(logrus.Fields{
				"natGatewayId": natGatewayID,
				"state":        state,
			}).Info("NAT Gateway status check")

			switch state {
			case ec2types.NatGatewayStateAvailable:
				return nil
			case ec2types.NatGatewayStateFailed:
				return fmt.Errorf("NAT Gateway %s creation failed", natGatewayID)
			case ec2types.NatGatewayStatePending:
				time.Sleep(pollInterval)
			default:
				time.Sleep(pollInterval)
			}
		}
	}
}

// CreateRouteForNAT creates a route to a NAT Gateway
func (c *Client) CreateRouteForNAT(ctx context.Context, routeTableID, destinationCidr, natGatewayID string) error {
	input := &ec2.CreateRouteInput{
		RouteTableId:         aws.String(routeTableID),
		DestinationCidrBlock: aws.String(destinationCidr),
		NatGatewayId:         aws.String(natGatewayID),
	}

	_, err := c.ec2.CreateRoute(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to create route to NAT Gateway: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"routeTableId": routeTableID,
		"destination":  destinationCidr,
		"natGatewayId": natGatewayID,
	}).Info("Route to NAT Gateway created successfully")

	return nil
}

// ========== VPC and Subnet Listing Methods ==========

// DescribeVPCs lists all VPCs in the region
func (c *Client) DescribeVPCs(ctx context.Context) ([]*types.AWSResource, error) {
	result, err := c.ec2.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to describe VPCs: %w", err)
	}

	var resources []*types.AWSResource
	for _, vpc := range result.Vpcs {
		resources = append(resources, c.convertVPC(vpc))
	}

	return resources, nil
}

// GetVPC gets a specific VPC by ID
func (c *Client) GetVPC(ctx context.Context, vpcID string) (*types.AWSResource, error) {
	result, err := c.ec2.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{
		VpcIds: []string{vpcID},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe VPC %s: %w", vpcID, err)
	}

	if len(result.Vpcs) == 0 {
		return nil, fmt.Errorf("VPC %s not found", vpcID)
	}

	return c.convertVPC(result.Vpcs[0]), nil
}

// DescribeSubnetsAll lists all subnets in the region
func (c *Client) DescribeSubnetsAll(ctx context.Context) ([]*types.AWSResource, error) {
	result, err := c.ec2.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to describe subnets: %w", err)
	}

	var resources []*types.AWSResource
	for _, subnet := range result.Subnets {
		resources = append(resources, c.convertSubnet(subnet))
	}

	return resources, nil
}

// GetSubnet gets a specific subnet by ID
func (c *Client) GetSubnet(ctx context.Context, subnetID string) (*types.AWSResource, error) {
	result, err := c.ec2.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{
		SubnetIds: []string{subnetID},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe subnet %s: %w", subnetID, err)
	}

	if len(result.Subnets) == 0 {
		return nil, fmt.Errorf("subnet %s not found", subnetID)
	}

	return c.convertSubnet(result.Subnets[0]), nil
}

// convertVPC converts an EC2 VPC to our internal resource representation
func (c *Client) convertVPC(vpc ec2types.Vpc) *types.AWSResource {
	tags := make(map[string]string)
	for _, tag := range vpc.Tags {
		if tag.Key != nil && tag.Value != nil {
			tags[*tag.Key] = *tag.Value
		}
	}

	details := map[string]interface{}{
		"cidrBlock":                   aws.ToString(vpc.CidrBlock),
		"dhcpOptionsId":               aws.ToString(vpc.DhcpOptionsId),
		"instanceTenancy":             string(vpc.InstanceTenancy),
		"isDefault":                   aws.ToBool(vpc.IsDefault),
		"cidrBlockAssociationSet":     vpc.CidrBlockAssociationSet,
		"ipv6CidrBlockAssociationSet": vpc.Ipv6CidrBlockAssociationSet,
	}

	return &types.AWSResource{
		ID:       aws.ToString(vpc.VpcId),
		Type:     "vpc",
		Region:   c.cfg.Region,
		State:    string(vpc.State),
		Tags:     tags,
		Details:  details,
		LastSeen: time.Now(),
	}
}

// convertSubnet converts an EC2 Subnet to our internal resource representation
func (c *Client) convertSubnet(subnet ec2types.Subnet) *types.AWSResource {
	tags := make(map[string]string)
	for _, tag := range subnet.Tags {
		if tag.Key != nil && tag.Value != nil {
			tags[*tag.Key] = *tag.Value
		}
	}

	details := map[string]interface{}{
		"vpcId":                         aws.ToString(subnet.VpcId),
		"cidrBlock":                     aws.ToString(subnet.CidrBlock),
		"availabilityZone":              aws.ToString(subnet.AvailabilityZone),
		"availabilityZoneId":            aws.ToString(subnet.AvailabilityZoneId),
		"availableIpAddressCount":       aws.ToInt32(subnet.AvailableIpAddressCount),
		"defaultForAz":                  aws.ToBool(subnet.DefaultForAz),
		"mapPublicIpOnLaunch":           aws.ToBool(subnet.MapPublicIpOnLaunch),
		"mapCustomerOwnedIpOnLaunch":    aws.ToBool(subnet.MapCustomerOwnedIpOnLaunch),
		"customerOwnedIpv4Pool":         aws.ToString(subnet.CustomerOwnedIpv4Pool),
		"outpostArn":                    aws.ToString(subnet.OutpostArn),
		"assignIpv6AddressOnCreation":   aws.ToBool(subnet.AssignIpv6AddressOnCreation),
		"ipv6CidrBlockAssociationSet":   subnet.Ipv6CidrBlockAssociationSet,
		"ipv6Native":                    aws.ToBool(subnet.Ipv6Native),
		"privateDnsNameOptionsOnLaunch": subnet.PrivateDnsNameOptionsOnLaunch,
	}

	return &types.AWSResource{
		ID:       aws.ToString(subnet.SubnetId),
		Type:     "subnet",
		Region:   c.cfg.Region,
		State:    string(subnet.State),
		Tags:     tags,
		Details:  details,
		LastSeen: time.Now(),
	}
}

// DescribeNATGateways retrieves NAT Gateway information
func (c *Client) DescribeNATGateways(ctx context.Context, natGatewayIDs []string) ([]*types.AWSResource, error) {
	input := &ec2.DescribeNatGatewaysInput{}

	if len(natGatewayIDs) > 0 {
		input.NatGatewayIds = natGatewayIDs
	}

	result, err := c.ec2.DescribeNatGateways(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe NAT gateways: %w", err)
	}

	var natGateways []*types.AWSResource
	for _, natGateway := range result.NatGateways {
		resource := c.convertNATGatewayToResource(natGateway)
		natGateways = append(natGateways, resource)
	}

	return natGateways, nil
}

// convertNATGatewayToResource converts an EC2 NAT Gateway to our AWSResource format
func (c *Client) convertNATGatewayToResource(natGateway ec2types.NatGateway) *types.AWSResource {
	tags := make(map[string]string)
	for _, tag := range natGateway.Tags {
		if tag.Key != nil && tag.Value != nil {
			tags[*tag.Key] = *tag.Value
		}
	}

	// Extract NAT Gateway addresses information
	var addresses []map[string]interface{}
	for _, addr := range natGateway.NatGatewayAddresses {
		address := map[string]interface{}{
			"allocationId":       aws.ToString(addr.AllocationId),
			"networkInterfaceId": aws.ToString(addr.NetworkInterfaceId),
			"privateIp":          aws.ToString(addr.PrivateIp),
			"publicIp":           aws.ToString(addr.PublicIp),
			"associationId":      aws.ToString(addr.AssociationId),
			"isPrimary":          aws.ToBool(addr.IsPrimary),
			"status":             string(addr.Status),
		}
		addresses = append(addresses, address)
	}

	details := map[string]interface{}{
		"subnetId":             aws.ToString(natGateway.SubnetId),
		"vpcId":                aws.ToString(natGateway.VpcId),
		"state":                string(natGateway.State),
		"failureCode":          aws.ToString(natGateway.FailureCode),
		"failureMessage":       aws.ToString(natGateway.FailureMessage),
		"provisionedBandwidth": natGateway.ProvisionedBandwidth,
		"connectivityType":     string(natGateway.ConnectivityType),
		"natGatewayAddresses":  addresses,
		"createTime":           natGateway.CreateTime,
		"deleteTime":           natGateway.DeleteTime,
	}

	return &types.AWSResource{
		ID:       aws.ToString(natGateway.NatGatewayId),
		Type:     "nat-gateway",
		Region:   c.cfg.Region,
		State:    string(natGateway.State),
		Tags:     tags,
		Details:  details,
		LastSeen: time.Now(),
	}
}
