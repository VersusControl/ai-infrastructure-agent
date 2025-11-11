package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"

	awstypes "github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// ========== RDS Database Management Methods ==========

// CreateDBSubnetGroup creates a database subnet group
func (c *Client) CreateDBSubnetGroup(ctx context.Context, params CreateDBSubnetGroupParams) (*types.DBSubnetGroup, error) {
	input := &rds.CreateDBSubnetGroupInput{
		DBSubnetGroupName:        aws.String(params.DBSubnetGroupName),
		DBSubnetGroupDescription: aws.String(params.DBSubnetGroupDescription),
		SubnetIds:                params.SubnetIDs,
	}

	// Add tag specifications during creation if tags are provided
	if len(params.Tags) > 0 {
		var rdsTags []types.Tag
		for key, value := range params.Tags {
			rdsTags = append(rdsTags, types.Tag{
				Key:   aws.String(key),
				Value: aws.String(value),
			})
		}
		input.Tags = rdsTags
	}

	result, err := c.rds.CreateDBSubnetGroup(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to create DB subnet group: %w", err)
	}

	c.logger.WithField("dbSubnetGroupName", aws.ToString(result.DBSubnetGroup.DBSubnetGroupName)).Info("DB subnet group created successfully")
	return result.DBSubnetGroup, nil
}

// CreateDBInstance creates a new RDS database instance
func (c *Client) CreateDBInstance(ctx context.Context, params CreateDBInstanceParams) (*types.DBInstance, error) {
	input := &rds.CreateDBInstanceInput{
		DBInstanceIdentifier: aws.String(params.DBInstanceIdentifier),
		DBInstanceClass:      aws.String(params.DBInstanceClass),
		Engine:               aws.String(params.Engine),
		MasterUsername:       aws.String(params.MasterUsername),
		MasterUserPassword:   aws.String(params.MasterUserPassword),
		AllocatedStorage:     aws.Int32(params.AllocatedStorage),
	}

	// Optional parameters
	if params.EngineVersion != "" {
		input.EngineVersion = aws.String(params.EngineVersion)
	}
	if params.StorageType != "" {
		input.StorageType = aws.String(params.StorageType)
	}
	if params.StorageEncrypted {
		input.StorageEncrypted = aws.Bool(params.StorageEncrypted)
	}
	if len(params.VpcSecurityGroupIDs) > 0 {
		input.VpcSecurityGroupIds = params.VpcSecurityGroupIDs
	}
	if params.DBSubnetGroupName != "" {
		input.DBSubnetGroupName = aws.String(params.DBSubnetGroupName)
	}
	if params.BackupRetentionPeriod > 0 {
		input.BackupRetentionPeriod = aws.Int32(params.BackupRetentionPeriod)
	}
	if params.PreferredBackupWindow != "" {
		input.PreferredBackupWindow = aws.String(params.PreferredBackupWindow)
	}
	if params.PreferredMaintenanceWindow != "" {
		input.PreferredMaintenanceWindow = aws.String(params.PreferredMaintenanceWindow)
	}
	if params.MultiAZ {
		input.MultiAZ = aws.Bool(params.MultiAZ)
	}
	if params.PubliclyAccessible {
		input.PubliclyAccessible = aws.Bool(params.PubliclyAccessible)
	}
	if params.PerformanceInsightsEnabled {
		input.EnablePerformanceInsights = aws.Bool(params.PerformanceInsightsEnabled)
	}

	// Add tag specifications during creation if tags are provided
	if len(params.Tags) > 0 {
		var rdsTags []types.Tag
		for key, value := range params.Tags {
			rdsTags = append(rdsTags, types.Tag{
				Key:   aws.String(key),
				Value: aws.String(value),
			})
		}
		input.Tags = rdsTags
	}

	result, err := c.rds.CreateDBInstance(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to create DB instance: %w", err)
	}

	c.logger.WithField("dbInstanceIdentifier", aws.ToString(result.DBInstance.DBInstanceIdentifier)).Info("DB instance creation initiated")
	return result.DBInstance, nil
}

// ListDBInstances returns all RDS instances in the region
func (c *Client) ListDBInstances(ctx context.Context) ([]awstypes.AWSResource, error) {
	input := &rds.DescribeDBInstancesInput{}

	result, err := c.rds.DescribeDBInstances(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe DB instances: %w", err)
	}

	var resources []awstypes.AWSResource
	for _, dbInstance := range result.DBInstances {
		resource := c.convertDBInstance(dbInstance)
		resources = append(resources, *resource)
	}

	c.logger.WithField("count", len(resources)).Info("Retrieved DB instances")
	return resources, nil
}

// GetDBInstance retrieves details about a specific DB instance
func (c *Client) GetDBInstance(ctx context.Context, dbInstanceIdentifier string) (*awstypes.AWSResource, error) {
	input := &rds.DescribeDBInstancesInput{
		DBInstanceIdentifier: aws.String(dbInstanceIdentifier),
	}

	result, err := c.rds.DescribeDBInstances(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe DB instance %s: %w", dbInstanceIdentifier, err)
	}

	if len(result.DBInstances) == 0 {
		return nil, fmt.Errorf("DB instance %s not found", dbInstanceIdentifier)
	}

	return c.convertDBInstance(result.DBInstances[0]), nil
}

// StartDBInstance starts a stopped RDS instance
func (c *Client) StartDBInstance(ctx context.Context, dbInstanceIdentifier string) error {
	input := &rds.StartDBInstanceInput{
		DBInstanceIdentifier: aws.String(dbInstanceIdentifier),
	}

	_, err := c.rds.StartDBInstance(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to start DB instance %s: %w", dbInstanceIdentifier, err)
	}

	c.logger.WithField("dbInstanceIdentifier", dbInstanceIdentifier).Info("DB instance start initiated")
	return nil
}

// StopDBInstance stops a running RDS instance
func (c *Client) StopDBInstance(ctx context.Context, dbInstanceIdentifier string) error {
	input := &rds.StopDBInstanceInput{
		DBInstanceIdentifier: aws.String(dbInstanceIdentifier),
	}

	_, err := c.rds.StopDBInstance(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to stop DB instance %s: %w", dbInstanceIdentifier, err)
	}

	c.logger.WithField("dbInstanceIdentifier", dbInstanceIdentifier).Info("DB instance stop initiated")
	return nil
}

// DeleteDBInstance deletes an RDS instance
func (c *Client) DeleteDBInstance(ctx context.Context, dbInstanceIdentifier string, skipFinalSnapshot bool, finalSnapshotIdentifier string) error {
	input := &rds.DeleteDBInstanceInput{
		DBInstanceIdentifier: aws.String(dbInstanceIdentifier),
		SkipFinalSnapshot:    aws.Bool(skipFinalSnapshot),
	}

	if !skipFinalSnapshot && finalSnapshotIdentifier != "" {
		input.FinalDBSnapshotIdentifier = aws.String(finalSnapshotIdentifier)
	}

	_, err := c.rds.DeleteDBInstance(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete DB instance %s: %w", dbInstanceIdentifier, err)
	}

	c.logger.WithField("dbInstanceIdentifier", dbInstanceIdentifier).Info("DB instance deletion initiated")
	return nil
}

// CreateDBSnapshot creates a snapshot of an RDS instance
func (c *Client) CreateDBSnapshot(ctx context.Context, params CreateDBSnapshotParams) (*types.DBSnapshot, error) {
	input := &rds.CreateDBSnapshotInput{
		DBInstanceIdentifier: aws.String(params.DBInstanceIdentifier),
		DBSnapshotIdentifier: aws.String(params.DBSnapshotIdentifier),
	}

	// Add tag specifications during creation if tags are provided
	if len(params.Tags) > 0 {
		var rdsTags []types.Tag
		for key, value := range params.Tags {
			rdsTags = append(rdsTags, types.Tag{
				Key:   aws.String(key),
				Value: aws.String(value),
			})
		}
		input.Tags = rdsTags
	}

	result, err := c.rds.CreateDBSnapshot(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to create DB snapshot: %w", err)
	}

	c.logger.WithField("dbSnapshotIdentifier", aws.ToString(result.DBSnapshot.DBSnapshotIdentifier)).Info("DB snapshot creation initiated")
	return result.DBSnapshot, nil
}

// ListDBSnapshots returns all RDS snapshots
func (c *Client) ListDBSnapshots(ctx context.Context) ([]awstypes.AWSResource, error) {
	input := &rds.DescribeDBSnapshotsInput{
		SnapshotType: aws.String("manual"), // Only manual snapshots
	}

	result, err := c.rds.DescribeDBSnapshots(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe DB snapshots: %w", err)
	}

	var resources []awstypes.AWSResource
	for _, snapshot := range result.DBSnapshots {
		resource := c.convertDBSnapshot(snapshot)
		resources = append(resources, *resource)
	}

	c.logger.WithField("count", len(resources)).Info("Retrieved DB snapshots")
	return resources, nil
}

// convertDBInstance converts an RDS DB instance to our standard resource format
func (c *Client) convertDBInstance(dbInstance types.DBInstance) *awstypes.AWSResource {
	tags := make(map[string]string)
	for _, tag := range dbInstance.TagList {
		if tag.Key != nil && tag.Value != nil {
			tags[*tag.Key] = *tag.Value
		}
	}

	var endpoint string
	if dbInstance.Endpoint != nil {
		endpoint = fmt.Sprintf("%s:%d", aws.ToString(dbInstance.Endpoint.Address), aws.ToInt32(dbInstance.Endpoint.Port))
	}

	details := map[string]interface{}{
		"engine":                       aws.ToString(dbInstance.Engine),
		"engine_version":               aws.ToString(dbInstance.EngineVersion),
		"instance_class":               aws.ToString(dbInstance.DBInstanceClass),
		"allocated_storage":            aws.ToInt32(dbInstance.AllocatedStorage),
		"storage_type":                 aws.ToString(dbInstance.StorageType),
		"storage_encrypted":            aws.ToBool(dbInstance.StorageEncrypted),
		"multi_az":                     aws.ToBool(dbInstance.MultiAZ),
		"publicly_accessible":          aws.ToBool(dbInstance.PubliclyAccessible),
		"endpoint":                     endpoint,
		"availability_zone":            aws.ToString(dbInstance.AvailabilityZone),
		"backup_retention":             aws.ToInt32(dbInstance.BackupRetentionPeriod),
		"preferred_backup_window":      aws.ToString(dbInstance.PreferredBackupWindow),
		"preferred_maintenance_window": aws.ToString(dbInstance.PreferredMaintenanceWindow),
	}

	if dbInstance.DBSubnetGroup != nil {
		details["subnet_group"] = aws.ToString(dbInstance.DBSubnetGroup.DBSubnetGroupName)
	}

	return &awstypes.AWSResource{
		ID:       aws.ToString(dbInstance.DBInstanceIdentifier),
		Type:     "rds-instance",
		Region:   c.cfg.Region,
		State:    aws.ToString(dbInstance.DBInstanceStatus),
		Tags:     tags,
		Details:  details,
		LastSeen: time.Now(),
	}
}

// convertDBSnapshot converts an RDS DB snapshot to our standard resource format
func (c *Client) convertDBSnapshot(snapshot types.DBSnapshot) *awstypes.AWSResource {
	tags := make(map[string]string)
	for _, tag := range snapshot.TagList {
		if tag.Key != nil && tag.Value != nil {
			tags[*tag.Key] = *tag.Value
		}
	}

	details := map[string]interface{}{
		"engine":               aws.ToString(snapshot.Engine),
		"engine_version":       aws.ToString(snapshot.EngineVersion),
		"instance_identifier":  aws.ToString(snapshot.DBInstanceIdentifier),
		"allocated_storage":    aws.ToInt32(snapshot.AllocatedStorage),
		"storage_type":         aws.ToString(snapshot.StorageType),
		"snapshot_create_time": snapshot.SnapshotCreateTime,
		"instance_create_time": snapshot.InstanceCreateTime,
		"availability_zone":    aws.ToString(snapshot.AvailabilityZone),
		"percent_progress":     aws.ToInt32(snapshot.PercentProgress),
		"source_region":        aws.ToString(snapshot.SourceRegion),
	}

	return &awstypes.AWSResource{
		ID:       aws.ToString(snapshot.DBSnapshotIdentifier),
		Type:     "rds-snapshot",
		Region:   c.cfg.Region,
		State:    aws.ToString(snapshot.Status),
		Tags:     tags,
		Details:  details,
		LastSeen: time.Now(),
	}
}
