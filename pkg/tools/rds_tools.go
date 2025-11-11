package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/adapters"
	"github.com/versus-control/ai-infrastructure-agent/pkg/aws"
	"github.com/versus-control/ai-infrastructure-agent/pkg/interfaces"
)

// CreateDBSubnetGroupTool implements MCPTool for creating DB subnet groups
type CreateDBSubnetGroupTool struct {
	*BaseTool
	adapter interfaces.SpecializedOperations
}

// NewCreateDBSubnetGroupTool creates a new DB subnet group creation tool
func NewCreateDBSubnetGroupTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"dbSubnetGroupName": map[string]interface{}{
				"type":        "string",
				"description": "The name for the DB subnet group",
			},
			"dbSubnetGroupDescription": map[string]interface{}{
				"type":        "string",
				"description": "The description for the DB subnet group",
			},
			"subnetIds": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
				"description": "List of subnet IDs",
			},
		},
		"required": []string{"dbSubnetGroupName", "dbSubnetGroupDescription", "subnetIds"},
	}

	baseTool := NewBaseTool(
		"create-db-subnet-group",
		"Create a new DB subnet group",
		"database",
		actionType,
		inputSchema,
		logger,
	)

	return &CreateDBSubnetGroupTool{
		BaseTool: baseTool,
		adapter:  adapters.NewRDSSpecializedAdapter(awsClient, logger),
	}
}

func (t *CreateDBSubnetGroupTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	dbSubnetGroupName, ok := arguments["dbSubnetGroupName"].(string)
	if !ok || dbSubnetGroupName == "" {
		return t.CreateErrorResponse("dbSubnetGroupName is required")
	}

	dbSubnetGroupDescription, ok := arguments["dbSubnetGroupDescription"].(string)
	if !ok || dbSubnetGroupDescription == "" {
		return t.CreateErrorResponse("dbSubnetGroupDescription is required")
	}

	// Use the RDS specialized adapter to create DB subnet group
	result, err := t.adapter.ExecuteSpecialOperation(ctx, "create-db-subnet-group", arguments)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to create DB subnet group: %v", err))
	}

	message := fmt.Sprintf("DB subnet group %s created successfully", dbSubnetGroupName)
	data := map[string]interface{}{
		"dbSubnetGroupName":        dbSubnetGroupName,
		"dbSubnetGroupDescription": dbSubnetGroupDescription,
		"result":                   result,
		"subnetGroupId":            result.ID,
	}

	return t.CreateSuccessResponse(message, data)
}

// CreateDBInstanceTool implements MCPTool for creating DB instances
type CreateDBInstanceTool struct {
	*BaseTool
	adapter interfaces.SpecializedOperations
}

// NewCreateDBInstanceTool creates a new DB instance creation tool
func NewCreateDBInstanceTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"dbInstanceIdentifier": map[string]interface{}{
				"type":        "string",
				"description": "The DB instance identifier",
			},
			"dbInstanceClass": map[string]interface{}{
				"type":        "string",
				"description": "The DB instance class",
				"default":     "db.t3.micro",
			},
			"engine": map[string]interface{}{
				"type":        "string",
				"description": "The database engine",
				"default":     "mysql",
			},
			"engineVersion": map[string]interface{}{
				"type":        "string",
				"description": "The database engine version",
			},
			"masterUsername": map[string]interface{}{
				"type":        "string",
				"description": "The master username",
			},
			"masterUserPassword": map[string]interface{}{
				"type":        "string",
				"description": "The master user password",
			},
			"allocatedStorage": map[string]interface{}{
				"type":        "integer",
				"description": "The allocated storage in GB",
				"default":     20,
			},
			"storageType": map[string]interface{}{
				"type":        "string",
				"description": "The storage type (gp2, gp3, io1, io2, standard)",
				"default":     "gp2",
			},
			"storageEncrypted": map[string]interface{}{
				"type":        "boolean",
				"description": "Whether storage is encrypted",
				"default":     false,
			},
			"dbSubnetGroupName": map[string]interface{}{
				"type":        "string",
				"description": "The DB subnet group name",
			},
			"vpcSecurityGroupIds": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
				"description": "List of VPC security group IDs",
			},
			"backupRetentionPeriod": map[string]interface{}{
				"type":        "integer",
				"description": "Number of days to retain backups (0-35)",
				"default":     1,
			},
			"preferredBackupWindow": map[string]interface{}{
				"type":        "string",
				"description": "Daily time range for backups (HH:MM-HH:MM UTC)",
			},
			"preferredMaintenanceWindow": map[string]interface{}{
				"type":        "string",
				"description": "Weekly time range for maintenance (ddd:hh24:mi-ddd:hh24:mi)",
			},
			"multiAz": map[string]interface{}{
				"type":        "boolean",
				"description": "Whether to deploy Multi-AZ",
				"default":     false,
			},
			"publiclyAccessible": map[string]interface{}{
				"type":        "boolean",
				"description": "Whether the instance is publicly accessible",
				"default":     false,
			},
			"performanceInsightsEnabled": map[string]interface{}{
				"type":        "boolean",
				"description": "Whether to enable Performance Insights",
				"default":     false,
			},
		},
		"required": []string{"dbInstanceIdentifier", "masterUsername", "masterUserPassword"},
	}

	baseTool := NewBaseTool(
		"create-db-instance",
		"Create a new RDS DB instance",
		"database",
		actionType,
		inputSchema,
		logger,
	)

	return &CreateDBInstanceTool{
		BaseTool: baseTool,
		adapter:  adapters.NewRDSSpecializedAdapter(awsClient, logger),
	}
}

func (t *CreateDBInstanceTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	dbInstanceIdentifier, ok := arguments["dbInstanceIdentifier"].(string)
	if !ok || dbInstanceIdentifier == "" {
		return t.CreateErrorResponse("dbInstanceIdentifier is required")
	}

	masterUsername, ok := arguments["masterUsername"].(string)
	if !ok || masterUsername == "" {
		return t.CreateErrorResponse("masterUsername is required")
	}

	masterUserPassword, ok := arguments["masterUserPassword"].(string)
	if !ok || masterUserPassword == "" {
		return t.CreateErrorResponse("masterUserPassword is required")
	}

	dbInstanceClass, _ := arguments["dbInstanceClass"].(string)
	if dbInstanceClass == "" {
		dbInstanceClass = "db.t3.micro"
	}

	engine, _ := arguments["engine"].(string)
	if engine == "" {
		engine = "mysql"
	}

	// Use the RDS specialized adapter to create DB instance
	result, err := t.adapter.ExecuteSpecialOperation(ctx, "create-db-instance", arguments)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to create DB instance: %v", err))
	}

	message := fmt.Sprintf("DB instance %s created successfully", dbInstanceIdentifier)
	data := map[string]interface{}{
		"dbInstanceIdentifier": dbInstanceIdentifier,
		"dbInstanceClass":      dbInstanceClass,
		"engine":               engine,
		"masterUsername":       masterUsername,
		"result":               result,
		"dbInstanceId":         result.ID,
	}

	return t.CreateSuccessResponse(message, data)
}

// StartDBInstanceTool implements MCPTool for starting DB instances
type StartDBInstanceTool struct {
	*BaseTool
	adapter interfaces.SpecializedOperations
}

// NewStartDBInstanceTool creates a new DB instance start tool
func NewStartDBInstanceTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"dbInstanceIdentifier": map[string]interface{}{
				"type":        "string",
				"description": "The DB instance identifier",
			},
		},
		"required": []string{"dbInstanceIdentifier"},
	}

	baseTool := NewBaseTool(
		"start-db-instance",
		"Start a stopped DB instance",
		"database",
		actionType,
		inputSchema,
		logger,
	)

	return &StartDBInstanceTool{
		BaseTool: baseTool,
		adapter:  adapters.NewRDSSpecializedAdapter(awsClient, logger),
	}
}

func (t *StartDBInstanceTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	dbInstanceIdentifier, ok := arguments["dbInstanceIdentifier"].(string)
	if !ok || dbInstanceIdentifier == "" {
		return t.CreateErrorResponse("dbInstanceIdentifier is required")
	}

	// Use the RDS specialized adapter to start DB instance
	result, err := t.adapter.ExecuteSpecialOperation(ctx, "start", dbInstanceIdentifier)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to start DB instance: %v", err))
	}

	message := fmt.Sprintf("DB instance %s started successfully", dbInstanceIdentifier)
	data := map[string]interface{}{
		"dbInstanceIdentifier": dbInstanceIdentifier,
		"status":               "starting",
		"result":               result,
	}

	return t.CreateSuccessResponse(message, data)
}

// StopDBInstanceTool implements MCPTool for stopping DB instances
type StopDBInstanceTool struct {
	*BaseTool
	adapter interfaces.SpecializedOperations
}

// NewStopDBInstanceTool creates a new DB instance stop tool
func NewStopDBInstanceTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"dbInstanceIdentifier": map[string]interface{}{
				"type":        "string",
				"description": "The DB instance identifier",
			},
		},
		"required": []string{"dbInstanceIdentifier"},
	}

	baseTool := NewBaseTool(
		"stop-db-instance",
		"Stop a running DB instance",
		"database",
		actionType,
		inputSchema,
		logger,
	)

	return &StopDBInstanceTool{
		BaseTool: baseTool,
		adapter:  adapters.NewRDSSpecializedAdapter(awsClient, logger),
	}
}

func (t *StopDBInstanceTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	dbInstanceIdentifier, ok := arguments["dbInstanceIdentifier"].(string)
	if !ok || dbInstanceIdentifier == "" {
		return t.CreateErrorResponse("dbInstanceIdentifier is required")
	}

	// Use the RDS specialized adapter to stop DB instance
	result, err := t.adapter.ExecuteSpecialOperation(ctx, "stop", dbInstanceIdentifier)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to stop DB instance: %v", err))
	}

	message := fmt.Sprintf("DB instance %s stopped successfully", dbInstanceIdentifier)
	data := map[string]interface{}{
		"dbInstanceIdentifier": dbInstanceIdentifier,
		"status":               "stopping",
		"result":               result,
	}

	return t.CreateSuccessResponse(message, data)
}

// DeleteDBInstanceTool implements MCPTool for deleting DB instances
type DeleteDBInstanceTool struct {
	*BaseTool
	adapter interfaces.SpecializedOperations
}

// NewDeleteDBInstanceTool creates a new DB instance deletion tool
func NewDeleteDBInstanceTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"dbInstanceIdentifier": map[string]interface{}{
				"type":        "string",
				"description": "The DB instance identifier",
			},
			"skipFinalSnapshot": map[string]interface{}{
				"type":        "boolean",
				"description": "Skip the final snapshot",
				"default":     false,
			},
		},
		"required": []string{"dbInstanceIdentifier"},
	}

	baseTool := NewBaseTool(
		"delete-db-instance",
		"Delete a DB instance",
		"database",
		actionType,
		inputSchema,
		logger,
	)

	return &DeleteDBInstanceTool{
		BaseTool: baseTool,
		adapter:  adapters.NewRDSSpecializedAdapter(awsClient, logger),
	}
}

func (t *DeleteDBInstanceTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	dbInstanceIdentifier, ok := arguments["dbInstanceIdentifier"].(string)
	if !ok || dbInstanceIdentifier == "" {
		return t.CreateErrorResponse("dbInstanceIdentifier is required")
	}

	skipFinalSnapshot, _ := arguments["skipFinalSnapshot"].(bool)

	// Use the RDS specialized adapter to delete DB instance
	result, err := t.adapter.ExecuteSpecialOperation(ctx, "delete", arguments)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to delete DB instance: %v", err))
	}

	message := fmt.Sprintf("DB instance %s deletion initiated successfully", dbInstanceIdentifier)
	data := map[string]interface{}{
		"dbInstanceIdentifier": dbInstanceIdentifier,
		"skipFinalSnapshot":    skipFinalSnapshot,
		"status":               "deleting",
		"result":               result,
	}

	return t.CreateSuccessResponse(message, data)
}

// CreateDBSnapshotTool implements MCPTool for creating DB snapshots
type CreateDBSnapshotTool struct {
	*BaseTool
	adapter interfaces.SpecializedOperations
}

// NewCreateDBSnapshotTool creates a new DB snapshot creation tool
func NewCreateDBSnapshotTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"dbInstanceIdentifier": map[string]interface{}{
				"type":        "string",
				"description": "The DB instance identifier",
			},
			"dbSnapshotIdentifier": map[string]interface{}{
				"type":        "string",
				"description": "The DB snapshot identifier",
			},
			"name": map[string]interface{}{
				"type":        "string",
				"description": "The name for the DB snapshot (for AWS Console display)",
			},
		},
		"required": []string{"dbInstanceIdentifier", "dbSnapshotIdentifier"},
	}

	baseTool := NewBaseTool(
		"create-db-snapshot",
		"Create a snapshot of a DB instance",
		"database",
		actionType,
		inputSchema,
		logger,
	)

	return &CreateDBSnapshotTool{
		BaseTool: baseTool,
		adapter:  adapters.NewRDSSpecializedAdapter(awsClient, logger),
	}
}

func (t *CreateDBSnapshotTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	dbInstanceIdentifier, ok := arguments["dbInstanceIdentifier"].(string)
	if !ok || dbInstanceIdentifier == "" {
		return t.CreateErrorResponse("dbInstanceIdentifier is required")
	}

	dbSnapshotIdentifier, ok := arguments["dbSnapshotIdentifier"].(string)
	if !ok || dbSnapshotIdentifier == "" {
		return t.CreateErrorResponse("dbSnapshotIdentifier is required")
	}

	name, _ := arguments["name"].(string)

	// Prepare parameters for snapshot creation
	params := map[string]interface{}{
		"dbInstanceIdentifier": dbInstanceIdentifier,
		"snapshotIdentifier":   dbSnapshotIdentifier,
	}

	// Add name parameter if provided
	if name != "" {
		params["name"] = name
	}

	// Use the RDS specialized adapter to create DB snapshot
	result, err := t.adapter.ExecuteSpecialOperation(ctx, "create-snapshot", params)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to create DB snapshot: %v", err))
	}

	message := fmt.Sprintf("DB snapshot %s created successfully from instance %s", dbSnapshotIdentifier, dbInstanceIdentifier)
	data := map[string]interface{}{
		"dbInstanceIdentifier": dbInstanceIdentifier,
		"dbSnapshotIdentifier": dbSnapshotIdentifier,
		"status":               "creating",
		"result":               result,
		"snapshotId":           result.ID,
	}

	return t.CreateSuccessResponse(message, data)
}

// ListDBInstancesTool implements MCPTool for listing DB instances
type ListDBInstancesTool struct {
	*BaseTool
	adapter interfaces.AWSResourceAdapter
}

// NewListDBInstancesTool creates a new DB instance listing tool
func NewListDBInstancesTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}

	baseTool := NewBaseTool(
		"list-db-instances",
		"List all RDS DB instances",
		"rds",
		actionType,
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"List all DB instances",
		map[string]interface{}{},
		"Retrieved 2 DB instances",
	)

	adapter := adapters.NewRDSAdapter(awsClient, logger)

	return &ListDBInstancesTool{
		BaseTool: baseTool,
		adapter:  adapter,
	}
}

func (t *ListDBInstancesTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	t.logger.Info("Listing DB Instances...")

	// List all DB Instances using the adapter
	dbInstances, err := t.adapter.List(ctx)
	if err != nil {
		t.logger.Error("Failed to list DB Instances", "error", err)
		return t.CreateErrorResponse(fmt.Sprintf("Failed to list DB Instances: %v", err))
	}

	message := fmt.Sprintf("Successfully retrieved %d DB Instances", len(dbInstances))
	data := map[string]interface{}{
		"dbInstances": dbInstances,
		"count":       len(dbInstances),
	}

	return t.CreateSuccessResponse(message, data)
}

// ListDBSnapshotsTool implements MCPTool for listing DB snapshots
type ListDBSnapshotsTool struct {
	*BaseTool
	adapter interfaces.SpecializedOperations
}

// NewListDBSnapshotsTool creates a new DB snapshot listing tool
func NewListDBSnapshotsTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"dbInstanceIdentifier": map[string]interface{}{
				"type":        "string",
				"description": "Filter by DB instance identifier",
			},
		},
	}

	baseTool := NewBaseTool(
		"list-db-snapshots",
		"List all DB snapshots",
		"database",
		actionType,
		inputSchema,
		logger,
	)

	return &ListDBSnapshotsTool{
		BaseTool: baseTool,
		adapter:  adapters.NewRDSSpecializedAdapter(awsClient, logger),
	}
}

func (t *ListDBSnapshotsTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	t.logger.Info("Listing DB Snapshots...")

	dbInstanceIdentifier, _ := arguments["dbInstanceIdentifier"].(string)

	// Prepare parameters for the adapter
	params := map[string]interface{}{
		"dbInstanceIdentifier": dbInstanceIdentifier,
	}

	// List all DB Snapshots using the RDS specialized adapter
	result, err := t.adapter.ExecuteSpecialOperation(ctx, "list-db-snapshots", params)
	if err != nil {
		t.logger.Error("Failed to list DB Snapshots", "error", err)
		return t.CreateErrorResponse(fmt.Sprintf("Failed to list DB Snapshots: %v", err))
	}

	// Extract the snapshots from the result Details
	snapshotsData, exists := result.Details["dbSnapshots"]
	if !exists {
		t.logger.Error("No snapshots data found in result")
		return t.CreateErrorResponse("No snapshots data found in result")
	}

	// The snapshots are returned as aws.AWSResource from the aws client
	dbSnapshots := snapshotsData

	count, exists := result.Details["count"]
	if !exists {
		count = 0
	}

	message := fmt.Sprintf("Successfully retrieved %v DB Snapshots", count)
	if dbInstanceIdentifier != "" {
		message = fmt.Sprintf("Successfully retrieved %v DB Snapshots for instance %s", count, dbInstanceIdentifier)
	}

	data := map[string]interface{}{
		"dbSnapshots":          dbSnapshots,
		"dbInstanceIdentifier": dbInstanceIdentifier,
		"count":                count,
	}

	return t.CreateSuccessResponse(message, data)
}

// DescribeDBInstanceTool implements MCPTool for describing a single DB instance
type DescribeDBInstanceTool struct {
	*BaseTool
	adapter interfaces.AWSResourceAdapter
}

// NewDescribeDBInstanceTool creates a new DB instance description tool
func NewDescribeDBInstanceTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"dbInstanceIdentifier": map[string]interface{}{
				"type":        "string",
				"description": "The DB instance identifier to describe",
			},
		},
		"required": []string{"dbInstanceIdentifier"},
	}

	baseTool := NewBaseTool(
		"describe-db-instance",
		"Describe a specific RDS DB instance and get its current status",
		"database",
		actionType,
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"Describe a DB instance",
		map[string]interface{}{
			"dbInstanceIdentifier": "my-database",
		},
		"Retrieved DB instance details with status 'available'",
	)

	adapter := adapters.NewRDSAdapter(awsClient, logger)

	return &DescribeDBInstanceTool{
		BaseTool: baseTool,
		adapter:  adapter,
	}
}

func (t *DescribeDBInstanceTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	dbInstanceIdentifier, ok := arguments["dbInstanceIdentifier"].(string)
	if !ok || dbInstanceIdentifier == "" {
		return t.CreateErrorResponse("dbInstanceIdentifier is required")
	}

	t.logger.Info("Describing DB Instance", "dbInstanceIdentifier", dbInstanceIdentifier)

	// Get the DB instance using the adapter
	dbInstance, err := t.adapter.Get(ctx, dbInstanceIdentifier)
	if err != nil {
		t.logger.Error("Failed to describe DB Instance", "error", err)
		return t.CreateErrorResponse(fmt.Sprintf("Failed to describe DB instance: %v", err))
	}

	message := fmt.Sprintf("Successfully retrieved DB instance %s with status '%s'", dbInstanceIdentifier, dbInstance.State)
	data := map[string]interface{}{
		"dbInstanceIdentifier": dbInstanceIdentifier,
		"dbInstance":           dbInstance,
		"state":                dbInstance.State,
		"id":                   dbInstance.ID,
		"type":                 dbInstance.Type,
		"region":               dbInstance.Region,
	}

	return t.CreateSuccessResponse(message, data)
}
