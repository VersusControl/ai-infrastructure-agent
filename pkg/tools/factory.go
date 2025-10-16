package tools

import (
	"fmt"

	"github.com/versus-control/ai-infrastructure-agent/internal/config"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/aws"
	"github.com/versus-control/ai-infrastructure-agent/pkg/conflict"
	"github.com/versus-control/ai-infrastructure-agent/pkg/discovery"
	"github.com/versus-control/ai-infrastructure-agent/pkg/graph"
	"github.com/versus-control/ai-infrastructure-agent/pkg/interfaces"
)

// ToolFactoryImpl implements the ToolFactory interface
type ToolFactoryImpl struct {
	awsClient *aws.Client
	logger    *logging.Logger
}

// ToolDependencies contains all dependencies needed to create tools
type ToolDependencies struct {
	AWSClient        *aws.Client
	StateManager     interfaces.StateManager
	DiscoveryScanner *discovery.Scanner
	GraphManager     *graph.Manager
	GraphAnalyzer    *graph.Analyzer
	ConflictResolver *conflict.Resolver
	Config           *config.Config
}

// NewToolFactory creates a new tool factory
func NewToolFactory(awsClient *aws.Client, logger *logging.Logger) interfaces.ToolFactory {
	return &ToolFactoryImpl{
		awsClient: awsClient,
		logger:    logger,
	}
}

// CreateTool creates a tool by type
func (f *ToolFactoryImpl) CreateTool(toolType string, actionType string, dependencies interface{}) (interfaces.MCPTool, error) {

	// Handle regular tool dependencies
	deps, ok := dependencies.(*ToolDependencies)
	if !ok {
		// Try value type as well
		if depsVal, valOk := dependencies.(ToolDependencies); valOk {
			deps = &depsVal
		} else {
			return nil, fmt.Errorf("invalid dependencies type for tool %s", toolType)
		}
	}

	switch toolType {
	// EC2 Tools
	case "create-ec2-instance":
		return NewCreateEC2InstanceTool(deps.AWSClient, actionType, f.logger), nil
	case "list-ec2-instances":
		return NewListEC2InstancesTool(deps.AWSClient, actionType, f.logger), nil
	case "start-ec2-instance":
		return NewStartEC2InstanceTool(deps.AWSClient, actionType, f.logger), nil
	case "stop-ec2-instance":
		return NewStopEC2InstanceTool(deps.AWSClient, actionType, f.logger), nil
	case "terminate-ec2-instance":
		return NewTerminateEC2InstanceTool(deps.AWSClient, actionType, f.logger), nil
	case "create-ami-from-instance":
		return NewCreateAMIFromInstanceTool(deps.AWSClient, actionType, f.logger), nil
	case "list-amis":
		return NewListAMIsTool(deps.AWSClient, actionType, f.logger), nil

	// Key Pair Tools
	case "create-key-pair":
		return NewCreateKeyPairTool(deps.AWSClient, actionType, f.logger), nil
	case "list-key-pairs":
		return NewListKeyPairsTool(deps.AWSClient, actionType, f.logger), nil
	case "get-key-pair":
		return NewGetKeyPairTool(deps.AWSClient, actionType, f.logger), nil
	case "import-key-pair":
		return NewImportKeyPairTool(deps.AWSClient, actionType, f.logger), nil

	// VPC Tools
	case "create-vpc":
		return NewCreateVPCTool(deps.AWSClient, actionType, f.logger), nil
	case "list-vpcs":
		return NewListVPCsTool(deps.AWSClient, actionType, f.logger), nil
	case "get-default-vpc":
		return NewGetDefaultVPCTool(deps.AWSClient, actionType, f.logger), nil
	case "create-subnet":
		return NewCreateSubnetTool(deps.AWSClient, actionType, f.logger), nil
	case "create-private-subnet":
		return NewCreatePrivateSubnetTool(deps.AWSClient, actionType, f.logger), nil
	case "create-public-subnet":
		return NewCreatePublicSubnetTool(deps.AWSClient, actionType, f.logger), nil
	case "list-subnets":
		return NewListSubnetsTool(deps.AWSClient, actionType, f.logger), nil
	case "get-subnet":
		return NewGetSubnetTool(deps.AWSClient, actionType, f.logger), nil
	case "list-subnets-for-alb":
		return NewListSubnetsForALBTool(deps.AWSClient, actionType, f.logger), nil
	case "create-internet-gateway":
		return NewCreateInternetGatewayTool(deps.AWSClient, actionType, f.logger), nil
	case "create-nat-gateway":
		return NewCreateNATGatewayTool(deps.AWSClient, actionType, f.logger), nil
	case "describe-nat-gateways":
		return NewDescribeNATGatewaysTool(deps.AWSClient, actionType, f.logger), nil
	case "create-public-route-table":
		return NewCreatePublicRouteTableTool(deps.AWSClient, actionType, f.logger), nil
	case "create-private-route-table":
		return NewCreatePrivateRouteTableTool(deps.AWSClient, actionType, f.logger), nil
	case "associate-route-table":
		return NewAssociateRouteTableTool(deps.AWSClient, actionType, f.logger), nil
	case "add-route":
		return NewAddRouteTool(deps.AWSClient, actionType, f.logger), nil

	// Security Group Tools
	case "create-security-group":
		return NewCreateSecurityGroupTool(deps.AWSClient, actionType, f.logger), nil
	case "list-security-groups":
		return NewListSecurityGroupsTool(deps.AWSClient, actionType, f.logger), nil
	case "add-security-group-ingress-rule":
		return NewAddSecurityGroupIngressRuleTool(deps.AWSClient, actionType, f.logger), nil
	case "add-security-group-egress-rule":
		return NewAddSecurityGroupEgressRuleTool(deps.AWSClient, actionType, f.logger), nil
	case "delete-security-group":
		return NewDeleteSecurityGroupTool(deps.AWSClient, actionType, f.logger), nil

	// Auto Scaling Tools
	case "create-launch-template":
		return NewCreateLaunchTemplateTool(deps.AWSClient, actionType, f.logger), nil
	case "create-auto-scaling-group":
		return NewCreateAutoScalingGroupTool(deps.AWSClient, actionType, f.logger), nil
	case "list-auto-scaling-groups":
		return NewListAutoScalingGroupsTool(deps.AWSClient, actionType, f.logger), nil
	case "list-launch-templates":
		return NewListLaunchTemplatesTool(deps.AWSClient, actionType, f.logger), nil

	// Load Balancer Tools
	case "create-load-balancer":
		return NewCreateLoadBalancerTool(deps.AWSClient, actionType, f.logger), nil
	case "create-target-group":
		return NewCreateTargetGroupTool(deps.AWSClient, actionType, f.logger), nil
	case "create-listener":
		return NewCreateListenerTool(deps.AWSClient, actionType, f.logger), nil
	case "list-load-balancers":
		return NewListLoadBalancersTool(deps.AWSClient, actionType, f.logger), nil
	case "list-target-groups":
		return NewListTargetGroupsTool(deps.AWSClient, actionType, f.logger), nil
	case "register-targets":
		return NewRegisterTargetsTool(deps.AWSClient, actionType, f.logger), nil
	case "deregister-targets":
		return NewDeregisterTargetsTool(deps.AWSClient, actionType, f.logger), nil

	// AMI Tools
	case "get-latest-amazon-linux-ami":
		return NewGetLatestAmazonLinuxAMITool(deps.AWSClient, actionType, f.logger), nil
	case "get-latest-ubuntu-ami":
		return NewGetLatestUbuntuAMITool(deps.AWSClient, actionType, f.logger), nil
	case "get-latest-windows-ami":
		return NewGetLatestWindowsAMITool(deps.AWSClient, actionType, f.logger), nil

	// Zone Tools
	case "get-availability-zones":
		return NewGetAvailabilityZonesTool(deps.AWSClient, actionType, f.logger), nil

	// RDS Tools
	case "create-db-subnet-group":
		return NewCreateDBSubnetGroupTool(deps.AWSClient, actionType, f.logger), nil
	case "create-db-instance":
		return NewCreateDBInstanceTool(deps.AWSClient, actionType, f.logger), nil
	case "start-db-instance":
		return NewStartDBInstanceTool(deps.AWSClient, actionType, f.logger), nil
	case "stop-db-instance":
		return NewStopDBInstanceTool(deps.AWSClient, actionType, f.logger), nil
	case "delete-db-instance":
		return NewDeleteDBInstanceTool(deps.AWSClient, actionType, f.logger), nil
	case "create-db-snapshot":
		return NewCreateDBSnapshotTool(deps.AWSClient, actionType, f.logger), nil
	case "list-db-instances":
		return NewListDBInstancesTool(deps.AWSClient, actionType, f.logger), nil
	case "list-db-snapshots":
		return NewListDBSnapshotsTool(deps.AWSClient, actionType, f.logger), nil

	// State Management Tools
	case "analyze-infrastructure-state":
		return NewAnalyzeStateTool(deps, deps.AWSClient, actionType, f.logger), nil
	case "export-infrastructure-state":
		return NewExportStateTool(deps, deps.AWSClient, actionType, f.logger), nil

	// State-Aware Tools
	case "visualize-dependency-graph":
		return NewVisualizeDependencyGraphTool(deps, actionType, f.logger), nil
	case "detect-infrastructure-conflicts":
		return NewDetectConflictsTool(deps, actionType, f.logger), nil
	case "save-state":
		return NewSaveStateTool(deps, actionType, f.logger), nil
	case "add-resource-to-state":
		return NewAddResourceToStateTool(deps, actionType, f.logger), nil
	case "plan-infrastructure-deployment":
		return NewPlanDeploymentTool(deps, actionType, f.logger), nil

	default:
		return nil, fmt.Errorf("unsupported tool type: %s", toolType)
	}
}

// GetSupportedToolTypes returns all supported tool types grouped by action type
func (f *ToolFactoryImpl) GetSupportedToolTypes() map[string][]string {
	return map[string][]string{
		"creation": {
			"create-ec2-instance",
			"create-ami-from-instance",
			"create-key-pair",
			"import-key-pair",
			"create-vpc",
			"create-subnet",
			"create-private-subnet",
			"create-public-subnet",
			"create-internet-gateway",
			"create-nat-gateway",
			"create-public-route-table",
			"create-private-route-table",
			"create-security-group",
			"create-launch-template",
			"create-auto-scaling-group",
			"create-load-balancer",
			"create-target-group",
			"create-listener",
			"create-db-subnet-group",
			"create-db-instance",
			"create-db-snapshot",
		},
		"query": {
			"list-ec2-instances",
			"list-amis",
			"list-key-pairs",
			"get-key-pair",
			"list-vpcs",
			"get-default-vpc",
			"list-subnets",
			"get-subnet",
			"list-subnets-for-alb",
			"describe-nat-gateways",
			"list-security-groups",
			"list-auto-scaling-groups",
			"list-launch-templates",
			"list-load-balancers",
			"list-target-groups",
			"get-latest-amazon-linux-ami",
			"get-latest-ubuntu-ami",
			"get-latest-windows-ami",
			"get-availability-zones",
			"list-db-instances",
			"list-db-snapshots",
		},
		"modification": {
			"start-ec2-instance",
			"stop-ec2-instance",
			"start-db-instance",
			"stop-db-instance",
		},
		"deletion": {
			"terminate-ec2-instance",
			"delete-security-group",
			"delete-db-instance",
		},
		"association": {
			"associate-route-table",
			"add-route",
			"add-security-group-ingress-rule",
			"add-security-group-egress-rule",
			"register-targets",
			"deregister-targets",
		},
		"state": {
			"analyze-infrastructure-state",
			"export-infrastructure-state",
			"visualize-dependency-graph",
			"detect-infrastructure-conflicts",
			"plan-infrastructure-deployment",
			"add-resource-to-state",
			"save-state",
		},
	}
}

// GetToolActionType returns the action type for a given tool name
func (f *ToolFactoryImpl) GetToolActionType(toolName string) string {
	toolsByAction := f.GetSupportedToolTypes()
	for actionType, toolNames := range toolsByAction {
		for _, name := range toolNames {
			if name == toolName {
				return actionType
			}
		}
	}
	return "unknown"
}

// ToolRegistrationHelper helps register all standard tools
type ToolRegistrationHelper struct {
	factory  interfaces.ToolFactory
	registry interfaces.ToolRegistry
	logger   *logging.Logger
}

// NewToolRegistrationHelper creates a new registration helper
func NewToolRegistrationHelper(factory interfaces.ToolFactory, registry interfaces.ToolRegistry, logger *logging.Logger) *ToolRegistrationHelper {
	return &ToolRegistrationHelper{
		factory:  factory,
		registry: registry,
		logger:   logger,
	}
}
