package adapters

import (
	"context"
	"fmt"

	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/aws"
	"github.com/versus-control/ai-infrastructure-agent/pkg/interfaces"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// RDSAdapter implements the AWSResourceAdapter interface for RDS instances
type RDSAdapter struct {
	*BaseAWSAdapter
	client *aws.Client
}

// NewRDSAdapter creates a new RDS adapter
func NewRDSAdapter(client *aws.Client, logger *logging.Logger) interfaces.AWSResourceAdapter {
	base := NewBaseAWSAdapter(client, logger, "rds-instance")
	return &RDSAdapter{
		BaseAWSAdapter: base,
		client:         client,
	}
}

// Create creates a new RDS instance
func (r *RDSAdapter) Create(ctx context.Context, params interface{}) (*types.AWSResource, error) {
	createParams, ok := params.(aws.CreateDBInstanceParams)
	if !ok {
		return nil, fmt.Errorf("invalid parameters for RDS creation, expected aws.CreateDBInstanceParams")
	}

	dbInstance, err := r.client.CreateDBInstance(ctx, createParams)
	if err != nil {
		return nil, err
	}

	// Convert RDS DBInstance to AWSResource
	resource := &types.AWSResource{
		ID:     *dbInstance.DBInstanceIdentifier,
		Type:   "rds-instance",
		Region: "unknown", // TODO: Get region from client config
		State:  *dbInstance.DBInstanceStatus,
		Tags:   make(map[string]string), // RDS tags need separate API call
		Details: map[string]interface{}{
			"engine":           *dbInstance.Engine,
			"engineVersion":    *dbInstance.EngineVersion,
			"instanceClass":    *dbInstance.DBInstanceClass,
			"allocatedStorage": *dbInstance.AllocatedStorage,
			"endpoint":         dbInstance.Endpoint,
			"port":             *dbInstance.DbInstancePort,
		},
	}

	return resource, nil
}

// List returns all RDS instances (converted to AWSResource format)
func (r *RDSAdapter) List(ctx context.Context) ([]*types.AWSResource, error) {
	dbInstances, err := r.client.ListDBInstances(ctx)
	if err != nil {
		return nil, err
	}

	// Convert []types.AWSResource to []*types.AWSResource
	var resources []*types.AWSResource
	for i := range dbInstances {
		resources = append(resources, &dbInstances[i])
	}

	return resources, nil
}

// Get retrieves a specific RDS instance
func (r *RDSAdapter) Get(ctx context.Context, id string) (*types.AWSResource, error) {
	dbInstance, err := r.client.GetDBInstance(ctx, id)
	if err != nil {
		return nil, err
	}

	return dbInstance, nil
}

// Update updates an RDS instance (limited operations available)
func (r *RDSAdapter) Update(ctx context.Context, id string, params interface{}) (*types.AWSResource, error) {
	// RDS instances can be modified but it's complex
	return nil, fmt.Errorf("RDS instance updates should use specialized operations")
}

// Delete deletes an RDS instance
func (r *RDSAdapter) Delete(ctx context.Context, id string) error {
	// RDS instances require additional parameters for deletion
	return r.client.DeleteDBInstance(ctx, id, true, "")
}

// GetSupportedOperations returns the operations supported by this adapter
func (r *RDSAdapter) GetSupportedOperations() []string {
	return []string{
		"create",
		"list",
		"get",
		"delete",
		"start",
		"stop",
		"create-snapshot",
	}
}

// ValidateParams validates RDS-specific parameters
func (r *RDSAdapter) ValidateParams(operation string, params interface{}) error {
	switch operation {
	case "create":
		createParams, ok := params.(aws.CreateDBInstanceParams)
		if !ok {
			return fmt.Errorf("invalid parameters for create operation")
		}
		if createParams.DBInstanceIdentifier == "" {
			return fmt.Errorf("dbInstanceIdentifier is required for RDS creation")
		}
		if createParams.Engine == "" {
			return fmt.Errorf("engine is required for RDS creation")
		}
		if createParams.DBInstanceClass == "" {
			return fmt.Errorf("dbInstanceClass is required for RDS creation")
		}
		return nil
	case "get", "delete", "start", "stop":
		if params == nil {
			return fmt.Errorf("DB instance identifier is required for %s operation", operation)
		}
		return nil
	default:
		return fmt.Errorf("unsupported operation: %s", operation)
	}
}

// RDSSpecializedAdapter adds RDS-specific operations
type RDSSpecializedAdapter struct {
	interfaces.AWSResourceAdapter
	client *aws.Client
}

// NewRDSSpecializedAdapter creates an adapter with specialized RDS operations
func NewRDSSpecializedAdapter(client *aws.Client, logger *logging.Logger) interfaces.SpecializedOperations {
	baseAdapter := NewRDSAdapter(client, logger)
	return &RDSSpecializedAdapter{
		AWSResourceAdapter: baseAdapter,
		client:             client,
	}
}

// ExecuteSpecialOperation handles RDS-specific operations
func (r *RDSSpecializedAdapter) ExecuteSpecialOperation(ctx context.Context, operation string, params interface{}) (*types.AWSResource, error) {
	switch operation {
	case "start":
		dbIdentifier, ok := params.(string)
		if !ok {
			return nil, fmt.Errorf("DB instance identifier required for start operation")
		}
		err := r.client.StartDBInstance(ctx, dbIdentifier)
		if err != nil {
			return nil, err
		}
		return r.Get(ctx, dbIdentifier)

	case "stop":
		dbIdentifier, ok := params.(string)
		if !ok {
			return nil, fmt.Errorf("DB instance identifier required for stop operation")
		}
		err := r.client.StopDBInstance(ctx, dbIdentifier)
		if err != nil {
			return nil, err
		}
		return r.Get(ctx, dbIdentifier)

	case "delete":
		deleteParams, ok := params.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("delete parameters required")
		}
		dbIdentifier := ""
		if val, ok := deleteParams["dbInstanceIdentifier"].(string); ok {
			dbIdentifier = val
		}
		skipFinalSnapshot := false
		if val, ok := deleteParams["skipFinalSnapshot"].(bool); ok {
			skipFinalSnapshot = val
		}
		finalSnapshotIdentifier := ""
		if val, ok := deleteParams["finalSnapshotIdentifier"].(string); ok {
			finalSnapshotIdentifier = val
		}

		if dbIdentifier == "" {
			return nil, fmt.Errorf("dbInstanceIdentifier is required for delete operation")
		}

		err := r.client.DeleteDBInstance(ctx, dbIdentifier, skipFinalSnapshot, finalSnapshotIdentifier)
		if err != nil {
			return nil, err
		}

		return &types.AWSResource{
			ID:    dbIdentifier,
			Type:  "rds-instance",
			State: "deleting",
			Details: map[string]interface{}{
				"dbInstanceIdentifier":    dbIdentifier,
				"skipFinalSnapshot":       skipFinalSnapshot,
				"finalSnapshotIdentifier": finalSnapshotIdentifier,
			},
		}, nil

	case "create-db-instance":
		createParams, ok := params.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("create parameters required")
		}

		// Extract parameters with type checking
		var dbIdentifier, dbClass, engine, username, password, subnetGroup string
		var allocatedStorage int32 = 20
		var securityGroupIds []string

		if val, ok := createParams["dbInstanceIdentifier"].(string); ok {
			dbIdentifier = val
		}
		if val, ok := createParams["dbInstanceClass"].(string); ok {
			dbClass = val
		}
		if val, ok := createParams["engine"].(string); ok {
			engine = val
		}
		if val, ok := createParams["masterUsername"].(string); ok {
			username = val
		}
		if val, ok := createParams["masterUserPassword"].(string); ok {
			password = val
		}
		if val, ok := createParams["allocatedStorage"].(float64); ok {
			allocatedStorage = int32(val)
		}
		if val, ok := createParams["dbSubnetGroupName"].(string); ok {
			subnetGroup = val
		}
		if val, ok := createParams["vpcSecurityGroupIds"].([]interface{}); ok {
			for _, id := range val {
				if strId, ok := id.(string); ok {
					securityGroupIds = append(securityGroupIds, strId)
				}
			}
		}

		dbParams := aws.CreateDBInstanceParams{
			DBInstanceIdentifier: dbIdentifier,
			DBInstanceClass:      dbClass,
			Engine:               engine,
			MasterUsername:       username,
			MasterUserPassword:   password,
			AllocatedStorage:     allocatedStorage,
			DBSubnetGroupName:    subnetGroup,
			VpcSecurityGroupIDs:  securityGroupIds,
		}

		result, err := r.client.CreateDBInstance(ctx, dbParams)
		if err != nil {
			return nil, err
		}

		// Convert to AWSResource
		return &types.AWSResource{
			ID:    *result.DBInstanceIdentifier,
			Type:  "rds-instance",
			State: *result.DBInstanceStatus,
			Details: map[string]interface{}{
				"engine":           *result.Engine,
				"instanceClass":    *result.DBInstanceClass,
				"allocatedStorage": result.AllocatedStorage,
			},
		}, nil

	case "create-db-subnet-group":
		subnetParams, ok := params.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("subnet group parameters required")
		}

		// Extract parameters with type checking
		var dbSubnetGroupName, dbSubnetGroupDescription string
		var subnetIds []string
		var tags map[string]string = make(map[string]string)

		if val, ok := subnetParams["dbSubnetGroupName"].(string); ok {
			dbSubnetGroupName = val
		}
		if val, ok := subnetParams["dbSubnetGroupDescription"].(string); ok {
			dbSubnetGroupDescription = val
		}
		if val, ok := subnetParams["subnetIds"].([]interface{}); ok {
			for _, id := range val {
				if strId, ok := id.(string); ok {
					subnetIds = append(subnetIds, strId)
				}
			}
		}
		if val, ok := subnetParams["tags"].(map[string]interface{}); ok {
			for k, v := range val {
				if strVal, ok := v.(string); ok {
					tags[k] = strVal
				}
			}
		}

		sgParams := aws.CreateDBSubnetGroupParams{
			DBSubnetGroupName:        dbSubnetGroupName,
			DBSubnetGroupDescription: dbSubnetGroupDescription,
			SubnetIDs:                subnetIds,
			Tags:                     tags,
		}

		result, err := r.client.CreateDBSubnetGroup(ctx, sgParams)
		if err != nil {
			return nil, err
		}

		// Convert to AWSResource
		return &types.AWSResource{
			ID:    *result.DBSubnetGroupName,
			Type:  "db-subnet-group",
			State: "available",
			Details: map[string]interface{}{
				"description": *result.DBSubnetGroupDescription,
				"vpcId":       *result.VpcId,
			},
		}, nil

	case "list-db-snapshots":
		// List all DB Snapshots
		snapshots, err := r.client.ListDBSnapshots(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list db snapshots: %w", err)
		}

		return &types.AWSResource{
			ID:    "db-snapshot-list",
			Type:  "db-snapshot-list",
			State: "available",
			Details: map[string]interface{}{
				"count":       len(snapshots),
				"dbSnapshots": snapshots,
			},
		}, nil

	case "create-snapshot":
		snapshotParams, ok := params.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("snapshot parameters required")
		}
		dbIdentifier, _ := snapshotParams["dbInstanceIdentifier"].(string)
		snapshotId, _ := snapshotParams["snapshotIdentifier"].(string)

		if dbIdentifier == "" || snapshotId == "" {
			return nil, fmt.Errorf("dbInstanceIdentifier and snapshotIdentifier are required for snapshot creation")
		}

		// Create snapshot using proper parameters
		snapshotParamsStruct := aws.CreateDBSnapshotParams{
			DBInstanceIdentifier: dbIdentifier,
			DBSnapshotIdentifier: snapshotId,
		}

		snapshot, err := r.client.CreateDBSnapshot(ctx, snapshotParamsStruct)
		if err != nil {
			return nil, err
		}

		// Convert DBSnapshot to AWSResource format
		resource := &types.AWSResource{
			ID:   *snapshot.DBSnapshotIdentifier,
			Type: "rds-snapshot",
			Details: map[string]interface{}{
				"dbInstanceIdentifier": *snapshot.DBInstanceIdentifier,
				"snapshotCreateTime":   snapshot.SnapshotCreateTime,
				"status":               *snapshot.Status,
			},
		}
		return resource, nil

	default:
		return nil, fmt.Errorf("unsupported specialized operation: %s", operation)
	}
}

// GetSpecialOperations returns the specialized operations available
func (r *RDSSpecializedAdapter) GetSpecialOperations() []string {
	return []string{
		"start",
		"stop",
		"delete",
		"create-db-instance",
		"create-db-subnet-group",
		"list-db-snapshots",
		"create-snapshot",
	}
}
