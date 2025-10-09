package adapters

import (
	"context"
	"fmt"

	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/aws"
	"github.com/versus-control/ai-infrastructure-agent/pkg/interfaces"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// VPCAdapter implements the AWSResourceAdapter interface for VPC resources
type VPCAdapter struct {
	*BaseAWSAdapter
	client *aws.Client
}

// NewVPCAdapter creates a new VPC adapter
func NewVPCAdapter(client *aws.Client, logger *logging.Logger) interfaces.AWSResourceAdapter {
	base := NewBaseAWSAdapter(client, logger, "vpc")
	return &VPCAdapter{
		BaseAWSAdapter: base,
		client:         client,
	}
}

// Create creates a new VPC
func (v *VPCAdapter) Create(ctx context.Context, params interface{}) (*types.AWSResource, error) {
	createParams, ok := params.(aws.CreateVPCParams)
	if !ok {
		return nil, fmt.Errorf("invalid parameters for VPC creation, expected aws.CreateVPCParams")
	}

	return v.client.CreateVPC(ctx, createParams)
}

// List returns all VPCs
func (v *VPCAdapter) List(ctx context.Context) ([]*types.AWSResource, error) {
	return v.client.DescribeVPCs(ctx)
}

// Get retrieves a specific VPC
func (v *VPCAdapter) Get(ctx context.Context, id string) (*types.AWSResource, error) {
	vpcs, err := v.client.DescribeVPCs(ctx)
	if err != nil {
		return nil, err
	}

	for _, vpc := range vpcs {
		if vpc.ID == id {
			return vpc, nil
		}
	}

	return nil, fmt.Errorf("VPC %s not found", id)
}

// Update updates a VPC (limited operations available)
func (v *VPCAdapter) Update(ctx context.Context, id string, params interface{}) (*types.AWSResource, error) {
	// VPCs have very limited update operations
	return nil, fmt.Errorf("VPC updates not supported via standard interface, use specialized operations")
}

// Delete deletes a VPC (not implemented in AWS client)
func (v *VPCAdapter) Delete(ctx context.Context, id string) error {
	// VPC deletion is complex and requires removing all dependencies first
	// For now, return not implemented error
	return fmt.Errorf("VPC deletion not implemented - requires manual dependency cleanup")
}

// GetDefaultVPC finds and returns the default VPC in the current region
func (v *VPCAdapter) GetDefaultVPC(ctx context.Context) (*types.AWSResource, error) {
	vpcID, err := v.client.GetDefaultVPC(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get default VPC: %w", err)
	}

	// Get the full VPC details
	vpc, err := v.Get(ctx, vpcID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve default VPC details for %s: %w", vpcID, err)
	}

	return vpc, nil
}

// GetSupportedOperations returns the operations supported by this adapter
func (v *VPCAdapter) GetSupportedOperations() []string {
	return []string{
		"create",
		"list",
		"get",
		"delete",
		"get-default-vpc",
		"create-subnet",
		"create-internet-gateway",
		"create-route-table",
		"associate-route-table",
		"add-route",
	}
}

// ValidateParams validates VPC-specific parameters
func (v *VPCAdapter) ValidateParams(operation string, params interface{}) error {
	switch operation {
	case "create":
		createParams, ok := params.(aws.CreateVPCParams)
		if !ok {
			return fmt.Errorf("invalid parameters for create operation")
		}
		if createParams.CidrBlock == "" {
			return fmt.Errorf("cidrBlock is required for VPC creation")
		}
		return nil
	case "get", "delete":
		if params == nil {
			return fmt.Errorf("VPC ID is required for %s operation", operation)
		}
		return nil
	case "get-default-vpc":
		// No parameters required for get-default-vpc
		return nil
	default:
		return fmt.Errorf("unsupported operation: %s", operation)
	}
}

// SubnetAdapter implements the AWSResourceAdapter interface for Subnet resources
type SubnetAdapter struct {
	*BaseAWSAdapter
	client *aws.Client
}

// NewSubnetAdapter creates a new Subnet adapter
func NewSubnetAdapter(client *aws.Client, logger *logging.Logger) interfaces.AWSResourceAdapter {
	base := NewBaseAWSAdapter(client, logger, "subnet")
	return &SubnetAdapter{
		BaseAWSAdapter: base,
		client:         client,
	}
}

// Create creates a new subnet
func (s *SubnetAdapter) Create(ctx context.Context, params interface{}) (*types.AWSResource, error) {
	createParams, ok := params.(aws.CreateSubnetParams)
	if !ok {
		return nil, fmt.Errorf("invalid parameters for subnet creation, expected aws.CreateSubnetParams")
	}

	return s.client.CreateSubnet(ctx, createParams)
}

// List returns all subnets
func (s *SubnetAdapter) List(ctx context.Context) ([]*types.AWSResource, error) {
	return s.client.DescribeSubnetsAll(ctx)
}

// Get retrieves a specific subnet
func (s *SubnetAdapter) Get(ctx context.Context, id string) (*types.AWSResource, error) {
	subnets, err := s.client.DescribeSubnetsAll(ctx)
	if err != nil {
		return nil, err
	}

	for _, subnet := range subnets {
		if subnet.ID == id {
			return subnet, nil
		}
	}

	return nil, fmt.Errorf("subnet %s not found", id)
}

// Update updates a subnet (limited operations available)
func (s *SubnetAdapter) Update(ctx context.Context, id string, params interface{}) (*types.AWSResource, error) {
	return nil, fmt.Errorf("subnet updates not supported via standard interface")
}

// Delete deletes a subnet (not implemented in AWS client)
func (s *SubnetAdapter) Delete(ctx context.Context, id string) error {
	// Subnet deletion not implemented in current AWS client
	return fmt.Errorf("subnet deletion not implemented in AWS client")
}

// GetDefaultSubnet finds and returns the default subnet in the current region
func (s *SubnetAdapter) GetDefaultSubnet(ctx context.Context) (*types.AWSResource, error) {
	subnetInfo, err := s.client.GetDefaultSubnet(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get default subnet: %w", err)
	}

	// Get the full subnet details
	subnet, err := s.Get(ctx, subnetInfo.SubnetID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve default subnet details for %s: %w", subnetInfo.SubnetID, err)
	}

	return subnet, nil
}

// GetSupportedOperations returns the operations supported by this adapter
func (s *SubnetAdapter) GetSupportedOperations() []string {
	return []string{
		"create",
		"list",
		"get",
		"delete",
	}
}

// ValidateParams validates subnet-specific parameters
func (s *SubnetAdapter) ValidateParams(operation string, params interface{}) error {
	switch operation {
	case "create":
		createParams, ok := params.(aws.CreateSubnetParams)
		if !ok {
			return fmt.Errorf("invalid parameters for create operation")
		}
		if createParams.VpcID == "" {
			return fmt.Errorf("vpcId is required for subnet creation")
		}
		if createParams.CidrBlock == "" {
			return fmt.Errorf("cidrBlock is required for subnet creation")
		}
		return nil
	case "get", "delete":
		if params == nil {
			return fmt.Errorf("subnet ID is required for %s operation", operation)
		}
		return nil
	default:
		return fmt.Errorf("unsupported operation: %s", operation)
	}
}

// VPCSpecializedAdapter adds VPC-specific operations
type VPCSpecializedAdapter struct {
	interfaces.AWSResourceAdapter
	client *aws.Client
}

// NewVPCSpecializedAdapter creates an adapter with specialized VPC operations
func NewVPCSpecializedAdapter(client *aws.Client, logger *logging.Logger) interfaces.SpecializedOperations {
	baseAdapter := NewVPCAdapter(client, logger)
	return &VPCSpecializedAdapter{
		AWSResourceAdapter: baseAdapter,
		client:             client,
	}
}

// ExecuteSpecialOperation handles VPC-specific operations
func (v *VPCSpecializedAdapter) ExecuteSpecialOperation(ctx context.Context, operation string, params interface{}) (*types.AWSResource, error) {
	switch operation {
	case "create-subnet":
		// Handle both parameter formats for flexibility
		if subnetParams, ok := params.(aws.CreateSubnetParams); ok {
			// Direct parameter struct format
			return v.client.CreateSubnet(ctx, subnetParams)
		}

		// Map format (for MCP tools or other callers)
		paramsMap, ok := params.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("subnet parameters required for create-subnet operation")
		}

		vpcID, ok := paramsMap["vpcId"].(string)
		if !ok || vpcID == "" {
			return nil, fmt.Errorf("vpcId is required for subnet creation")
		}

		cidrBlock, ok := paramsMap["cidrBlock"].(string)
		if !ok || cidrBlock == "" {
			return nil, fmt.Errorf("cidrBlock is required for subnet creation")
		}

		availabilityZone, _ := paramsMap["availabilityZone"].(string)
		name, _ := paramsMap["name"].(string)
		mapPublicIpOnLaunch, _ := paramsMap["mapPublicIpOnLaunch"].(bool)

		// Extract tags if provided
		tags := make(map[string]string)
		if tagsMap, ok := paramsMap["tags"].(map[string]interface{}); ok {
			for k, v := range tagsMap {
				if strVal, ok := v.(string); ok {
					tags[k] = strVal
				}
			}
		}

		// Add name tag if provided
		if name != "" {
			tags["Name"] = name
		}

		// Create subnet parameters
		subnetParams := aws.CreateSubnetParams{
			VpcID:               vpcID,
			CidrBlock:           cidrBlock,
			AvailabilityZone:    availabilityZone,
			MapPublicIpOnLaunch: mapPublicIpOnLaunch,
			Name:                name,
			Tags:                tags,
		}

		return v.client.CreateSubnet(ctx, subnetParams)

	case "create-internet-gateway":
		paramsMap, ok := params.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("internet gateway parameters required as map")
		}

		vpcID, ok := paramsMap["vpcId"].(string)
		if !ok || vpcID == "" {
			return nil, fmt.Errorf("vpcId is required for internet gateway creation")
		}

		name, _ := paramsMap["name"].(string)

		// Create the IGW parameters
		igwParams := aws.CreateInternetGatewayParams{
			Name: name,
			Tags: map[string]string{},
		}

		return v.client.CreateInternetGateway(ctx, igwParams, vpcID)

	case "create-route-table":
		routeParams, ok := params.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("route table parameters required")
		}
		vpcID, _ := routeParams["vpcId"].(string)
		name, _ := routeParams["name"].(string)

		if vpcID == "" {
			return nil, fmt.Errorf("vpcId is required for route table creation")
		}

		return v.client.CreateRouteTable(ctx, vpcID, name)

	case "create-nat-gateway":
		natParams, ok := params.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("NAT gateway parameters required")
		}

		subnetID, ok := natParams["subnetId"].(string)
		if !ok || subnetID == "" {
			return nil, fmt.Errorf("subnetId is required for NAT gateway creation")
		}

		name, _ := natParams["name"].(string)

		// Create NAT gateway parameters
		natGwParams := aws.CreateNATGatewayParams{
			SubnetID: subnetID,
			Name:     name,
			Tags: map[string]string{
				"Name": name,
			},
		}

		return v.client.CreateNATGateway(ctx, natGwParams)

	case "describe-nat-gateways":
		// Handle NAT gateway description
		var natGatewayIDs []string

		if params != nil {
			if paramsMap, ok := params.(map[string]interface{}); ok {
				if ids, exists := paramsMap["natGatewayIds"]; exists {
					if idList, ok := ids.([]interface{}); ok {
						for _, id := range idList {
							if idStr, ok := id.(string); ok {
								natGatewayIDs = append(natGatewayIDs, idStr)
							}
						}
					} else if idList, ok := ids.([]string); ok {
						natGatewayIDs = idList
					}
				}
			}
		}

		// This operation returns multiple resources, but we need to return a single resource
		// We'll return a composite resource with the results
		natGateways, err := v.client.DescribeNATGateways(ctx, natGatewayIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to describe NAT gateways: %w", err)
		}

		// Create a composite resource containing all NAT gateways
		return &types.AWSResource{
			ID:    "nat-gateways-list",
			Type:  "nat-gateways-list",
			State: "available",
			Details: map[string]interface{}{
				"count":       len(natGateways),
				"natGateways": natGateways,
			},
		}, nil

	case "associate-route-table":
		assocParams, ok := params.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("route table association parameters required")
		}

		routeTableID, ok := assocParams["routeTableId"].(string)
		if !ok || routeTableID == "" {
			return nil, fmt.Errorf("routeTableId is required for route table association")
		}

		subnetID, ok := assocParams["subnetId"].(string)
		if !ok || subnetID == "" {
			return nil, fmt.Errorf("subnetId is required for route table association")
		}

		err := v.client.AssociateRouteTable(ctx, routeTableID, subnetID)
		if err != nil {
			return nil, err
		}

		// Return a resource representing the association
		return &types.AWSResource{
			ID:     fmt.Sprintf("%s-%s", routeTableID, subnetID),
			Type:   "route-table-association",
			Region: v.client.GetRegion(),
			State:  "associated",
			Details: map[string]interface{}{
				"routeTableId": routeTableID,
				"subnetId":     subnetID,
			},
		}, nil

	case "add-route":
		routeParams, ok := params.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("route parameters required")
		}

		routeTableID, ok := routeParams["routeTableId"].(string)
		if !ok || routeTableID == "" {
			return nil, fmt.Errorf("routeTableId is required for route creation")
		}

		destinationCidr, ok := routeParams["destinationCidrBlock"].(string)
		if !ok || destinationCidr == "" {
			return nil, fmt.Errorf("destinationCidrBlock is required for route creation")
		}

		natGatewayID, _ := routeParams["natGatewayId"].(string)
		internetGatewayID, _ := routeParams["internetGatewayId"].(string)

		if natGatewayID == "" && internetGatewayID == "" {
			return nil, fmt.Errorf("either natGatewayId or internetGatewayId is required for route creation")
		}

		if natGatewayID != "" && internetGatewayID != "" {
			return nil, fmt.Errorf("specify either natGatewayId or internetGatewayId, not both")
		}

		var err error
		var targetID string
		var targetType string

		if natGatewayID != "" {
			err = v.client.CreateRouteForNAT(ctx, routeTableID, destinationCidr, natGatewayID)
			targetID = natGatewayID
			targetType = "nat-gateway"
		} else {
			err = v.client.CreateRoute(ctx, routeTableID, destinationCidr, internetGatewayID)
			targetID = internetGatewayID
			targetType = "internet-gateway"
		}

		if err != nil {
			return nil, err
		}

		// Return a resource representing the route
		return &types.AWSResource{
			ID:     fmt.Sprintf("%s-%s", routeTableID, destinationCidr),
			Type:   "route",
			Region: v.client.GetRegion(),
			State:  "active",
			Details: map[string]interface{}{
				"routeTableId":         routeTableID,
				"destinationCidrBlock": destinationCidr,
				"targetId":             targetID,
				"targetType":           targetType,
			},
		}, nil

	default:
		return nil, fmt.Errorf("unsupported specialized operation: %s", operation)
	}
}

// GetSpecialOperations returns the specialized operations available
func (v *VPCSpecializedAdapter) GetSpecialOperations() []string {
	return []string{"create-subnet", "create-internet-gateway", "create-route-table", "create-nat-gateway", "associate-route-table", "add-route"}
}
