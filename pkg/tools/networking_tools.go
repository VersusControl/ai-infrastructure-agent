package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/adapters"
	"github.com/versus-control/ai-infrastructure-agent/pkg/aws"
	"github.com/versus-control/ai-infrastructure-agent/pkg/interfaces"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// CreatePrivateSubnetTool implements MCPTool for creating private subnets
type CreatePrivateSubnetTool struct {
	*BaseTool
	adapter *adapters.SubnetAdapter
}

// NewCreatePrivateSubnetTool creates a new private subnet creation tool
func NewCreatePrivateSubnetTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"vpcId": map[string]interface{}{
				"type":        "string",
				"description": "The VPC ID where the subnet will be created",
			},
			"cidrBlock": map[string]interface{}{
				"type":        "string",
				"description": "The CIDR block for the subnet",
			},
			"availabilityZone": map[string]interface{}{
				"type":        "string",
				"description": "The availability zone for the subnet",
			},
			"name": map[string]interface{}{
				"type":        "string",
				"description": "A name tag for the subnet",
			},
		},
		"required": []string{"vpcId", "cidrBlock"},
	}

	baseTool := NewBaseTool(
		"create-private-subnet",
		"Create a new private subnet",
		"networking",
		actionType,
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"Create a private subnet",
		map[string]interface{}{
			"vpcId":            "vpc-12345678",
			"cidrBlock":        "10.0.1.0/24",
			"availabilityZone": "us-east-1a",
			"name":             "private-subnet-1",
		},
		"Created private subnet subnet-87654321 in VPC vpc-12345678",
	)

	// Cast to SubnetAdapter for type safety
	adapter := adapters.NewSubnetAdapter(awsClient, logger).(*adapters.SubnetAdapter)

	return &CreatePrivateSubnetTool{
		BaseTool: baseTool,
		adapter:  adapter,
	}
}

func (t *CreatePrivateSubnetTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	vpcID, ok := arguments["vpcId"].(string)
	if !ok || vpcID == "" {
		return t.CreateErrorResponse("vpcId is required")
	}

	cidrBlock, ok := arguments["cidrBlock"].(string)
	if !ok || cidrBlock == "" {
		return t.CreateErrorResponse("cidrBlock is required")
	}

	availabilityZone, _ := arguments["availabilityZone"].(string)
	name, _ := arguments["name"].(string)

	// Create subnet parameters (private subnet doesn't map public IP)
	params := aws.CreateSubnetParams{
		VpcID:               vpcID,
		CidrBlock:           cidrBlock,
		AvailabilityZone:    availabilityZone,
		MapPublicIpOnLaunch: false, // Private subnet
		Name:                name,
		Tags:                map[string]string{"Type": "private"},
	}

	// Use the adapter to create the subnet
	subnet, err := t.adapter.Create(ctx, params)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to create private subnet: %s", err.Error()))
	}

	message := fmt.Sprintf("Created private subnet %s in VPC %s", subnet.ID, vpcID)
	data := map[string]interface{}{
		"subnetId":         subnet.ID,
		"vpcId":            vpcID,
		"cidrBlock":        cidrBlock,
		"availabilityZone": availabilityZone,
		"name":             name,
		"type":             "private",
		"resource":         subnet,
	}

	return t.CreateSuccessResponse(message, data)
}

// CreatePublicSubnetTool implements MCPTool for creating public subnets
type CreatePublicSubnetTool struct {
	*BaseTool
	adapter *adapters.SubnetAdapter
}

// NewCreatePublicSubnetTool creates a new public subnet creation tool
func NewCreatePublicSubnetTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"vpcId": map[string]interface{}{
				"type":        "string",
				"description": "The VPC ID where the subnet will be created",
			},
			"cidrBlock": map[string]interface{}{
				"type":        "string",
				"description": "The CIDR block for the subnet",
			},
			"availabilityZone": map[string]interface{}{
				"type":        "string",
				"description": "The availability zone for the subnet",
			},
			"name": map[string]interface{}{
				"type":        "string",
				"description": "A name tag for the subnet",
			},
		},
		"required": []string{"vpcId", "cidrBlock"},
	}

	baseTool := NewBaseTool(
		"create-public-subnet",
		"Create a new public subnet",
		"networking",
		actionType,
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"Create a public subnet",
		map[string]interface{}{
			"vpcId":            "vpc-12345678",
			"cidrBlock":        "10.0.2.0/24",
			"availabilityZone": "us-east-1a",
			"name":             "public-subnet-1",
		},
		"Created public subnet subnet-87654321 in VPC vpc-12345678",
	)

	// Cast to SubnetAdapter for type safety
	adapter := adapters.NewSubnetAdapter(awsClient, logger).(*adapters.SubnetAdapter)

	return &CreatePublicSubnetTool{
		BaseTool: baseTool,
		adapter:  adapter,
	}
}

func (t *CreatePublicSubnetTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	vpcID, ok := arguments["vpcId"].(string)
	if !ok || vpcID == "" {
		return t.CreateErrorResponse("vpcId is required")
	}

	cidrBlock, ok := arguments["cidrBlock"].(string)
	if !ok || cidrBlock == "" {
		return t.CreateErrorResponse("cidrBlock is required")
	}

	availabilityZone, _ := arguments["availabilityZone"].(string)
	name, _ := arguments["name"].(string)

	// Create subnet parameters (public subnet maps public IP)
	params := aws.CreateSubnetParams{
		VpcID:               vpcID,
		CidrBlock:           cidrBlock,
		AvailabilityZone:    availabilityZone,
		MapPublicIpOnLaunch: true, // Public subnet
		Name:                name,
		Tags:                map[string]string{"Type": "public"},
	}

	// Use the adapter to create the subnet
	subnet, err := t.adapter.Create(ctx, params)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to create public subnet: %s", err.Error()))
	}

	message := fmt.Sprintf("Created public subnet %s in VPC %s", subnet.ID, vpcID)
	data := map[string]interface{}{
		"subnetId":         subnet.ID,
		"vpcId":            vpcID,
		"cidrBlock":        cidrBlock,
		"availabilityZone": availabilityZone,
		"name":             name,
		"type":             "public",
		"resource":         subnet,
	}

	return t.CreateSuccessResponse(message, data)
}

// ListSubnetsTool implements MCPTool for listing subnets
type ListSubnetsTool struct {
	*BaseTool
	adapter *adapters.SubnetAdapter
}

// NewListSubnetsTool creates a new subnet listing tool
func NewListSubnetsTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"vpcId": map[string]interface{}{
				"type":        "string",
				"description": "Filter subnets by VPC ID",
			},
		},
	}

	baseTool := NewBaseTool(
		"list-subnets",
		"List subnets in the current region to get subnet IDs. IMPORTANT: Returns ONLY subnet information (subnetId, subnet_ids, subnets).",
		"networking",
		actionType,
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"List all subnets",
		map[string]interface{}{},
		"Found 5 subnets. Returns: { subnetId: \"subnet-xxx\" (first subnet), subnet_ids: [array of all subnet IDs], subnets: [detailed info] }",
	)

	baseTool.AddExample(
		"List subnets in specific VPC",
		map[string]interface{}{
			"vpcId": "vpc-12345678",
		},
		"Found 3 subnets in VPC vpc-12345678. Use {{step-id.subnetId}} to reference the first subnet ID in subsequent steps.",
	)

	// Cast to SubnetAdapter for type safety
	adapter := adapters.NewSubnetAdapter(awsClient, logger).(*adapters.SubnetAdapter)

	return &ListSubnetsTool{
		BaseTool: baseTool,
		adapter:  adapter,
	}
}

func (t *ListSubnetsTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	vpcID, _ := arguments["vpcId"].(string)

	// Use the adapter to list subnets
	subnets, err := t.adapter.List(ctx)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to list subnets: %s", err.Error()))
	}

	// Filter by VPC if specified
	var filteredSubnets []*types.AWSResource
	if vpcID != "" {
		for _, subnet := range subnets {
			if vpcId, ok := subnet.Details["vpcId"].(string); ok && vpcId == vpcID {
				filteredSubnets = append(filteredSubnets, subnet)
			}
		}
		subnets = filteredSubnets
	}

	if len(subnets) == 0 {
		return t.CreateErrorResponse(fmt.Sprintf("Found 0 subnets in VPC %s", vpcID))
	}

	message := fmt.Sprintf("Found %d subnets", len(subnets))

	// Create subnet IDs list for dependency resolution
	var subnetIDs []string
	for _, subnet := range subnets {
		subnetIDs = append(subnetIDs, subnet.ID)
	}

	data := map[string]interface{}{}

	// Add dependency resolution fields (following retrieveSubnetsInVPC pattern)
	if len(subnetIDs) > 0 {
		data["subnetId"] = subnetIDs[0] // First subnet ID for {{step-id.subnetId}} resolution
		data["value"] = subnetIDs[0]    // For {{step-id.resourceId}} resolution
		data["subnet_ids"] = subnetIDs  // Full list for comprehensive access
		data["subnets"] = subnets       // Subnet details
	}

	if vpcID != "" {
		data["vpcId"] = vpcID // VPC ID
	}

	return t.CreateSuccessResponse(message, data)
}

// CreateInternetGatewayTool implements MCPTool for creating internet gateways
type CreateInternetGatewayTool struct {
	*BaseTool
	adapter *adapters.VPCSpecializedAdapter
}

// NewCreateInternetGatewayTool creates a new internet gateway creation tool
func NewCreateInternetGatewayTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"vpcId": map[string]interface{}{
				"type":        "string",
				"description": "The VPC ID where the internet gateway will be created and attached",
			},
			"name": map[string]interface{}{
				"type":        "string",
				"description": "A name tag for the internet gateway",
			},
		},
		"required": []string{"vpcId"},
	}

	baseTool := NewBaseTool(
		"create-internet-gateway",
		"Create an internet gateway and automatically attach it to the specified VPC in a single operation. NOTE: There is no separate attach operation needed.",
		"networking",
		actionType,
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"Create internet gateway",
		map[string]interface{}{
			"vpcId": "vpc-12345678",
			"name":  "main-igw",
		},
		"Created internet gateway igw-87654321 and attached to VPC vpc-12345678",
	)

	// Cast to VPCSpecializedAdapter for type safety
	adapter := adapters.NewVPCSpecializedAdapter(awsClient, logger).(*adapters.VPCSpecializedAdapter)

	return &CreateInternetGatewayTool{
		BaseTool: baseTool,
		adapter:  adapter,
	}
}

func (t *CreateInternetGatewayTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	vpcID, ok := arguments["vpcId"].(string)
	if !ok || vpcID == "" {
		return t.CreateErrorResponse("vpcId is required")
	}

	name, _ := arguments["name"].(string)

	// Create parameters map that includes both IGW params and vpcId
	params := map[string]interface{}{
		"vpcId": vpcID,
		"name":  name,
		"tags":  map[string]string{},
	}

	// Use the specialized adapter to create and attach the internet gateway
	igw, err := t.adapter.ExecuteSpecialOperation(ctx, "create-internet-gateway", params)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to create internet gateway: %s", err.Error()))
	}

	message := fmt.Sprintf("Created internet gateway %s and attached to VPC %s", igw.ID, vpcID)
	data := map[string]interface{}{
		"internetGatewayId": igw.ID,
		"vpcId":             vpcID,
		"name":              name,
		"resource":          igw,
	}

	return t.CreateSuccessResponse(message, data)
}

// CreateNATGatewayTool implements MCPTool for creating NAT gateways
type CreateNATGatewayTool struct {
	*BaseTool
	adapter interfaces.SpecializedOperations
}

// NewCreateNATGatewayTool creates a new NAT gateway creation tool
func NewCreateNATGatewayTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"subnetId": map[string]interface{}{
				"type":        "string",
				"description": "The subnet ID for the NAT gateway",
			},
			"name": map[string]interface{}{
				"type":        "string",
				"description": "A name tag for the NAT gateway",
			},
		},
		"required": []string{"subnetId"},
	}

	baseTool := NewBaseTool(
		"create-nat-gateway",
		"Create a NAT gateway and automatically allocate an Elastic IP for it in a single operation. The Elastic IP is allocated internally - no separate allocation step is needed.",
		"networking",
		actionType,
		inputSchema,
		logger,
	)

	return &CreateNATGatewayTool{
		BaseTool: baseTool,
		adapter:  adapters.NewVPCSpecializedAdapter(awsClient, logger),
	}
}

func (t *CreateNATGatewayTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	subnetID, ok := arguments["subnetId"].(string)
	if !ok || subnetID == "" {
		return t.CreateErrorResponse("subnetId is required")
	}

	name, _ := arguments["name"].(string)

	// Create NAT gateway using VPC adapter
	natGateway, err := t.adapter.ExecuteSpecialOperation(ctx, "create-nat-gateway", arguments)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to create NAT gateway: %s", err.Error()))
	}

	message := fmt.Sprintf("Successfully created NAT gateway %s in subnet %s", natGateway.ID, subnetID)
	data := map[string]interface{}{
		"natGatewayId": natGateway.ID,
		"subnetId":     subnetID,
		"name":         name,
		"type":         "nat-gateway",
		"resource":     natGateway,
	}

	return t.CreateSuccessResponse(message, data)
}

// CreatePublicRouteTableTool implements MCPTool for creating public route tables
type CreatePublicRouteTableTool struct {
	*BaseTool
	adapter interfaces.SpecializedOperations
}

// NewCreatePublicRouteTableTool creates a new public route table creation tool
func NewCreatePublicRouteTableTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"vpcId": map[string]interface{}{
				"type":        "string",
				"description": "The VPC ID for the route table",
			},
			"name": map[string]interface{}{
				"type":        "string",
				"description": "A name tag for the route table",
			},
		},
		"required": []string{"vpcId"},
	}

	baseTool := NewBaseTool(
		"create-public-route-table",
		"Create a new public route table",
		"networking",
		actionType,
		inputSchema,
		logger,
	)

	return &CreatePublicRouteTableTool{
		BaseTool: baseTool,
		adapter:  adapters.NewVPCSpecializedAdapter(awsClient, logger),
	}
}

func (t *CreatePublicRouteTableTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	vpcID, ok := arguments["vpcId"].(string)
	if !ok || vpcID == "" {
		return t.CreateErrorResponse("vpcId is required")
	}

	name, _ := arguments["name"].(string)

	// Create route table using VPC adapter
	routeTable, err := t.adapter.ExecuteSpecialOperation(ctx, "create-route-table", arguments)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to create public route table: %s", err.Error()))
	}

	message := fmt.Sprintf("Successfully created public route table %s in VPC %s", routeTable.ID, vpcID)
	data := map[string]interface{}{
		"routeTableId": routeTable.ID,
		"resourceId":   routeTable.ID, // Keep for backward compatibility
		"vpcId":        vpcID,
		"name":         name,
		"type":         "public",
		"resource":     routeTable,
	}

	return t.CreateSuccessResponse(message, data)
}

// CreatePrivateRouteTableTool implements MCPTool for creating private route tables
type CreatePrivateRouteTableTool struct {
	*BaseTool
	adapter interfaces.SpecializedOperations
}

// NewCreatePrivateRouteTableTool creates a new private route table creation tool
func NewCreatePrivateRouteTableTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"vpcId": map[string]interface{}{
				"type":        "string",
				"description": "The VPC ID for the route table",
			},
			"name": map[string]interface{}{
				"type":        "string",
				"description": "A name tag for the route table",
			},
		},
		"required": []string{"vpcId"},
	}

	baseTool := NewBaseTool(
		"create-private-route-table",
		"Create a new private route table",
		"networking",
		actionType,
		inputSchema,
		logger,
	)

	return &CreatePrivateRouteTableTool{
		BaseTool: baseTool,
		adapter:  adapters.NewVPCSpecializedAdapter(awsClient, logger),
	}
}

func (t *CreatePrivateRouteTableTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	vpcID, ok := arguments["vpcId"].(string)
	if !ok || vpcID == "" {
		return t.CreateErrorResponse("vpcId is required")
	}

	name, _ := arguments["name"].(string)

	// Use the adapter to create the route table
	params := map[string]interface{}{
		"vpcId": vpcID,
		"name":  name,
		"type":  "private",
	}

	result, err := t.adapter.ExecuteSpecialOperation(ctx, "create-route-table", params)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to create private route table: %v", err))
	}

	responseData := map[string]interface{}{
		"routeTableId": result.ID,
		"resourceId":   result.ID, // Keep for backward compatibility
		"type":         result.Type,
		"region":       result.Region,
		"state":        result.State,
		"vpcId":        vpcID,
		"name":         name,
		"routeType":    "private",
	}

	return t.CreateSuccessResponse("Private route table created successfully", responseData)
}

// AssociateRouteTableTool implements MCPTool for associating route tables with subnets
type AssociateRouteTableTool struct {
	*BaseTool
	adapter interfaces.SpecializedOperations
}

// NewAssociateRouteTableTool creates a new route table association tool
func NewAssociateRouteTableTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"routeTableId": map[string]interface{}{
				"type":        "string",
				"description": "The route table ID",
			},
			"subnetId": map[string]interface{}{
				"type":        "string",
				"description": "The subnet ID to associate with the route table",
			},
		},
		"required": []string{"routeTableId", "subnetId"},
	}

	baseTool := NewBaseTool(
		"associate-route-table",
		"Associate a route table with a subnet",
		"networking",
		actionType,
		inputSchema,
		logger,
	)

	return &AssociateRouteTableTool{
		BaseTool: baseTool,
		adapter:  adapters.NewVPCSpecializedAdapter(awsClient, logger),
	}
}

func (t *AssociateRouteTableTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	routeTableID, ok := arguments["routeTableId"].(string)
	if !ok || routeTableID == "" {
		return t.CreateErrorResponse("routeTableId is required")
	}

	subnetID, ok := arguments["subnetId"].(string)
	if !ok || subnetID == "" {
		return t.CreateErrorResponse("subnetId is required")
	}

	params := map[string]interface{}{
		"routeTableId": routeTableID,
		"subnetId":     subnetID,
	}

	result, err := t.adapter.ExecuteSpecialOperation(ctx, "associate-route-table", params)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to associate route table with subnet: %v", err))
	}

	message := fmt.Sprintf("Successfully associated route table %s with subnet %s", routeTableID, subnetID)
	data := map[string]interface{}{
		"routeTableId":  routeTableID,
		"subnetId":      subnetID,
		"associationId": result.ID,
	}

	return t.CreateSuccessResponse(message, data)
}

// AddRouteTool implements MCPTool for adding routes to route tables
type AddRouteTool struct {
	*BaseTool
	adapter interfaces.SpecializedOperations
}

// NewAddRouteTool creates a new route addition tool
func NewAddRouteTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"routeTableId": map[string]interface{}{
				"type":        "string",
				"description": "The route table ID",
			},
			"destinationCidrBlock": map[string]interface{}{
				"type":        "string",
				"description": "The destination CIDR block for the route",
			},
			"natGatewayId": map[string]interface{}{
				"type":        "string",
				"description": "The NAT gateway ID for the route target (for private routes)",
			},
			"internetGatewayId": map[string]interface{}{
				"type":        "string",
				"description": "The internet gateway ID for the route target (for public routes)",
			},
		},
		"required": []string{"routeTableId", "destinationCidrBlock"},
	}

	baseTool := NewBaseTool(
		"add-route",
		"Add a route to a route table. Use this to attach internet gateways (IGW) to public route tables or NAT gateways to private route tables.",
		"networking",
		actionType,
		inputSchema,
		logger,
	)

	return &AddRouteTool{
		BaseTool: baseTool,
		adapter:  adapters.NewVPCSpecializedAdapter(awsClient, logger),
	}
}

func (t *AddRouteTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	routeTableID, ok := arguments["routeTableId"].(string)
	if !ok || routeTableID == "" {
		return t.CreateErrorResponse("routeTableId is required")
	}

	// Validate that routeTableId looks like a route table ID
	if !strings.HasPrefix(routeTableID, "rtb-") {
		return t.CreateErrorResponse(fmt.Sprintf("Invalid route table ID format: %s (expected format: rtb-xxxxxxxx). Make sure you're using the correct dependency reference for route table creation", routeTableID))
	}

	destinationCidr, ok := arguments["destinationCidrBlock"].(string)
	if !ok || destinationCidr == "" {
		return t.CreateErrorResponse("destinationCidrBlock is required")
	}

	natGatewayID, _ := arguments["natGatewayId"].(string)
	internetGatewayID, _ := arguments["internetGatewayId"].(string)

	if natGatewayID == "" && internetGatewayID == "" {
		return t.CreateErrorResponse("Either natGatewayId or internetGatewayId is required for route creation")
	}

	if natGatewayID != "" && internetGatewayID != "" {
		return t.CreateErrorResponse("Specify either natGatewayId or internetGatewayId, not both")
	}

	params := map[string]interface{}{
		"routeTableId":         routeTableID,
		"destinationCidrBlock": destinationCidr,
	}

	var targetType string
	var targetID string
	if natGatewayID != "" {
		// Validate NAT gateway ID format
		if !strings.HasPrefix(natGatewayID, "nat-") {
			return t.CreateErrorResponse(fmt.Sprintf("Invalid NAT gateway ID format: %s (expected format: nat-xxxxxxxx)", natGatewayID))
		}
		params["natGatewayId"] = natGatewayID
		targetType = "NAT Gateway"
		targetID = natGatewayID
	} else {
		// Validate internet gateway ID format
		if !strings.HasPrefix(internetGatewayID, "igw-") {
			return t.CreateErrorResponse(fmt.Sprintf("Invalid internet gateway ID format: %s (expected format: igw-xxxxxxxx)", internetGatewayID))
		}
		params["internetGatewayId"] = internetGatewayID
		targetType = "Internet Gateway"
		targetID = internetGatewayID
	}

	result, err := t.adapter.ExecuteSpecialOperation(ctx, "add-route", params)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to add route: %v", err))
	}

	message := fmt.Sprintf("Successfully added route %s -> %s %s in route table %s",
		destinationCidr, targetType, targetID, routeTableID)

	data := map[string]interface{}{
		"routeTableId":         routeTableID,
		"destinationCidrBlock": destinationCidr,
		"targetId":             targetID,
		"targetType":           targetType,
		"result":               result,
	}

	return t.CreateSuccessResponse(message, data)
} // SelectSubnetsForALBTool implements MCPTool for selecting suitable subnets for ALB creation
type SelectSubnetsForALBTool struct {
	*BaseTool
	awsClient *aws.Client
}

// NewListSubnetsForALBTool creates a new subnet selection tool for ALB
func NewListSubnetsForALBTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"vpcId": map[string]interface{}{
				"type":        "string",
				"description": "The VPC ID to select subnets from (optional - defaults to default VPC)",
			},
			"scheme": map[string]interface{}{
				"type":        "string",
				"description": "Load balancer scheme (internet-facing or internal)",
				"default":     "internet-facing",
			},
		},
	}

	baseTool := NewBaseTool(
		"list-subnets-for-alb",
		"Select at least two subnets in different Availability Zones for ALB creation",
		"networking",
		actionType,
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"Select subnets for internet-facing ALB",
		map[string]interface{}{
			"scheme": "internet-facing",
		},
		"Selected 2 public subnets in different AZs",
	)

	baseTool.AddExample(
		"Select subnets for internal ALB in specific VPC",
		map[string]interface{}{
			"vpcId":  "vpc-12345678",
			"scheme": "internal",
		},
		"Selected 2 private subnets in different AZs",
	)

	return &SelectSubnetsForALBTool{
		BaseTool:  baseTool,
		awsClient: awsClient,
	}
}

func (t *SelectSubnetsForALBTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	scheme, _ := arguments["scheme"].(string)
	if scheme == "" {
		scheme = "internet-facing"
	}

	vpcID, exists := arguments["vpcId"].(string)
	if !exists || vpcID == "" {
		return t.CreateErrorResponse(fmt.Sprintln("No VPC ID provided for the ALB."))
	}

	// Get all subnets in the VPC
	subnets, err := t.awsClient.DescribeSubnetsAll(ctx)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to describe subnets: %v", err))
	}

	// Filter subnets by VPC and collect by availability zone
	subnetsByAZ := make(map[string][]*types.AWSResource)
	for _, subnet := range subnets {
		if subnetVPC, ok := subnet.Details["vpcId"].(string); ok && subnetVPC == vpcID {
			if az, ok := subnet.Details["availabilityZone"].(string); ok {
				subnetsByAZ[az] = append(subnetsByAZ[az], subnet)
			}
		}
	}

	// Ensure we have at least 2 different AZs
	if len(subnetsByAZ) < 2 {
		return t.CreateErrorResponse(fmt.Sprintf("Need at least 2 subnets in different Availability Zones, found %d AZs in VPC %s", len(subnetsByAZ), vpcID))
	}

	// Select subnets with preference for the right type, but be flexible
	var selectedSubnets []string
	var selectedAZs []string
	count := 0

	// Select preferred subnet type (public for internet-facing, private for internal)
	for az, subnetsInAZ := range subnetsByAZ {
		if count >= 2 {
			break
		}
		if len(subnetsInAZ) > 0 {
			var bestSubnet *types.AWSResource

			// Look for the preferred subnet type
			for _, subnet := range subnetsInAZ {
				isPublic := false
				if mapPublic, ok := subnet.Details["mapPublicIpOnLaunch"].(bool); ok {
					isPublic = mapPublic
				}

				if (scheme == "internet-facing" && isPublic) || (scheme == "internal" && !isPublic) {
					bestSubnet = subnet
					break
				}
			}

			// If no preferred type found, use the first available subnet in this AZ
			if bestSubnet == nil {
				bestSubnet = subnetsInAZ[0]
			}

			selectedSubnets = append(selectedSubnets, bestSubnet.ID)
			selectedAZs = append(selectedAZs, az)
			count++

			t.logger.WithFields(map[string]interface{}{
				"subnet_id": bestSubnet.ID,
				"az":        az,
				"scheme":    scheme,
			}).Info("Selected subnet for ALB")
		}
	}

	if len(selectedSubnets) < 2 {
		return t.CreateErrorResponse("Could not find at least 2 suitable subnets in different AZs for ALB creation")
	}

	message := fmt.Sprintf("Selected %d subnets in %d different Availability Zones for %s ALB", len(selectedSubnets), len(selectedAZs), scheme)
	data := map[string]interface{}{
		"subnetIds":         selectedSubnets,
		"availabilityZones": selectedAZs,
		"vpcId":             vpcID,
		"scheme":            scheme,
		"count":             len(selectedSubnets),
	}

	return t.CreateSuccessResponse(message, data)
}

// DescribeNATGatewaysTool implements MCPTool for describing NAT gateways
type DescribeNATGatewaysTool struct {
	*BaseTool
	adapter interfaces.SpecializedOperations
}

// NewDescribeNATGatewaysTool creates a new NAT gateway description tool
func NewDescribeNATGatewaysTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"natGatewayIds": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
				"description": "Optional list of NAT Gateway IDs to describe. If not provided, all NAT gateways will be returned",
			},
		},
	}

	baseTool := NewBaseTool(
		"describe-nat-gateways",
		"Describe NAT gateways in the current AWS region",
		"networking",
		actionType,
		inputSchema,
		logger,
	)

	adapter := adapters.NewVPCSpecializedAdapter(awsClient, logger)

	return &DescribeNATGatewaysTool{
		BaseTool: baseTool,
		adapter:  adapter,
	}
}

// Execute describes NAT gateways using the VPC adapter
func (t *DescribeNATGatewaysTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	// Use the adapter's ExecuteSpecialOperation
	result, err := t.adapter.ExecuteSpecialOperation(ctx, "describe-nat-gateways", arguments)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to describe NAT gateways: %s", err.Error()))
	}

	// Extract the NAT gateways from the composite resource
	natGateways, ok := result.Details["natGateways"].([]*types.AWSResource)
	if !ok {
		return t.CreateErrorResponse("Failed to extract NAT gateway data")
	}

	count, ok := result.Details["count"].(int)
	if !ok {
		count = len(natGateways)
	}

	// Format response
	message := fmt.Sprintf("Retrieved %d NAT gateways", count)
	data := map[string]interface{}{
		"count":       count,
		"natGateways": natGateways,
	}

	return t.CreateSuccessResponse(message, data)
}
