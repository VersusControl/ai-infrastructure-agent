package mocks

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/aws"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// MockAWSClient implements a mock AWS client for testing
type MockAWSClient struct {
	region string
	logger *logging.Logger
	mutex  sync.RWMutex

	// Parameter validator
	validator *ParameterValidator

	// Mock data stores
	vpcs              map[string]*types.AWSResource
	subnets           map[string]*types.AWSResource
	securityGroups    map[string]*types.AWSResource
	instances         map[string]*types.AWSResource
	loadBalancers     map[string]*types.AWSResource
	targetGroups      map[string]*types.AWSResource
	listeners         map[string]*types.AWSResource
	launchTemplates   map[string]*types.AWSResource
	autoScalingGroups map[string]*types.AWSResource
	amis              map[string]*types.AWSResource
	dbInstances       map[string]*types.AWSResource

	// Error simulation
	methodErrors         map[string]error
	shouldSimulateErrors bool
	errorRate            float32
}

// SubnetInfo represents subnet information
type SubnetInfo struct {
	SubnetID         string
	VPCID            string
	AvailabilityZone string
}

// NewMockAWSClient creates a new mock AWS client
func NewMockAWSClient(region string, logger *logging.Logger) *MockAWSClient {
	return &MockAWSClient{
		region:               region,
		logger:               logger,
		validator:            NewParameterValidator(),
		vpcs:                 make(map[string]*types.AWSResource),
		subnets:              make(map[string]*types.AWSResource),
		securityGroups:       make(map[string]*types.AWSResource),
		instances:            make(map[string]*types.AWSResource),
		loadBalancers:        make(map[string]*types.AWSResource),
		targetGroups:         make(map[string]*types.AWSResource),
		listeners:            make(map[string]*types.AWSResource),
		launchTemplates:      make(map[string]*types.AWSResource),
		autoScalingGroups:    make(map[string]*types.AWSResource),
		amis:                 make(map[string]*types.AWSResource),
		dbInstances:          make(map[string]*types.AWSResource),
		methodErrors:         make(map[string]error),
		shouldSimulateErrors: false,
		errorRate:            0.0,
	}
}

// ========== Error Simulation Methods ==========

// SimulateError sets up error simulation for a specific method
func (m *MockAWSClient) SimulateError(methodName string, err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.methodErrors[methodName] = err
}

// ClearError removes error simulation for a specific method
func (m *MockAWSClient) ClearError(methodName string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	delete(m.methodErrors, methodName)
}

// EnableErrorSimulation enables global error simulation
func (m *MockAWSClient) EnableErrorSimulation(rate float32) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.shouldSimulateErrors = true
	m.errorRate = rate
}

// DisableErrorSimulation disables global error simulation
func (m *MockAWSClient) DisableErrorSimulation() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.shouldSimulateErrors = false
	m.errorRate = 0.0
}

// checkError checks if an error should be simulated for the given method
func (m *MockAWSClient) checkError(methodName string) error {
	if err, exists := m.methodErrors[methodName]; exists {
		return err
	}
	if m.shouldSimulateErrors && m.errorRate > 0 {
		return fmt.Errorf("simulated AWS error for %s", methodName)
	}
	return nil
}

// ========== Basic AWS Client Methods ==========

// HealthCheck verifies AWS connectivity
func (m *MockAWSClient) HealthCheck(ctx context.Context) error {
	if err := m.checkError("HealthCheck"); err != nil {
		return err
	}
	return nil
}

// GetRegion returns the configured AWS region
func (m *MockAWSClient) GetRegion() string {
	return m.region
}

// ========== VPC Methods ==========

// CreateVPC creates a new VPC
func (m *MockAWSClient) CreateVPC(ctx context.Context, params aws.CreateVPCParams) (*types.AWSResource, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// First validate parameters
	if err := m.validator.ValidateVPCParams(params); err != nil {
		m.logger.Error("VPC parameter validation failed", "error", err, "params", params)
		return nil, fmt.Errorf("parameter validation failed: %w", err)
	}

	if err := m.checkError("CreateVPC"); err != nil {
		return nil, err
	}

	vpcID := fmt.Sprintf("vpc-%s", generateID())
	vpc := &types.AWSResource{
		ID:     vpcID,
		Type:   "vpc",
		Region: m.region,
		State:  "available",
		Details: map[string]interface{}{
			"cidrBlock": params.CidrBlock,
			"name":      params.Name,
		},
		LastSeen: time.Now(),
	}

	m.vpcs[vpcID] = vpc
	return vpc, nil
}

// DescribeVPCs lists all VPCs
func (m *MockAWSClient) DescribeVPCs(ctx context.Context) ([]*types.AWSResource, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if err := m.checkError("DescribeVPCs"); err != nil {
		return nil, err
	}

	var vpcs []*types.AWSResource
	for _, vpc := range m.vpcs {
		vpcs = append(vpcs, vpc)
	}

	return vpcs, nil
}

// GetVPC gets a specific VPC by ID
func (m *MockAWSClient) GetVPC(ctx context.Context, vpcID string) (*types.AWSResource, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if err := m.checkError("GetVPC"); err != nil {
		return nil, err
	}

	vpc, exists := m.vpcs[vpcID]
	if !exists {
		return nil, fmt.Errorf("vpc %s not found", vpcID)
	}

	return vpc, nil
}

// GetDefaultVPC finds the default VPC
func (m *MockAWSClient) GetDefaultVPC(ctx context.Context) (string, error) {
	if err := m.checkError("GetDefaultVPC"); err != nil {
		return "", err
	}

	// Return mock default VPC ID
	defaultVPCID := "vpc-default-12345"

	// Ensure default VPC exists in mock data
	m.mutex.Lock()
	if _, exists := m.vpcs[defaultVPCID]; !exists {
		defaultVPC := &types.AWSResource{
			ID:     defaultVPCID,
			Type:   "vpc",
			Region: m.region,
			State:  "available",
			Details: map[string]interface{}{
				"cidrBlock": "172.31.0.0/16",
				"isDefault": true,
			},
			LastSeen: time.Now(),
		}
		m.vpcs[defaultVPCID] = defaultVPC
	}
	m.mutex.Unlock()

	return defaultVPCID, nil
}

// ========== Subnet Methods ==========

// CreateSubnet creates a new subnet
func (m *MockAWSClient) CreateSubnet(ctx context.Context, params aws.CreateSubnetParams) (*types.AWSResource, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// First validate parameters
	if err := m.validator.ValidateSubnetParams(params); err != nil {
		m.logger.Error("Subnet parameter validation failed", "error", err, "params", params)
		return nil, fmt.Errorf("parameter validation failed: %w", err)
	}

	if err := m.checkError("CreateSubnet"); err != nil {
		return nil, err
	}

	subnetID := fmt.Sprintf("subnet-%s", generateID())
	subnet := &types.AWSResource{
		ID:     subnetID,
		Type:   "subnet",
		Region: m.region,
		State:  "available",
		Details: map[string]interface{}{
			"vpcId":               params.VpcID,
			"cidrBlock":           params.CidrBlock,
			"availabilityZone":    params.AvailabilityZone,
			"mapPublicIpOnLaunch": params.MapPublicIpOnLaunch,
			"name":                params.Name,
		},
		LastSeen: time.Now(),
	}

	m.subnets[subnetID] = subnet
	return subnet, nil
}

// DescribeSubnets lists all subnets
func (m *MockAWSClient) DescribeSubnets(ctx context.Context) ([]*types.AWSResource, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if err := m.checkError("DescribeSubnets"); err != nil {
		return nil, err
	}

	var subnets []*types.AWSResource
	for _, subnet := range m.subnets {
		subnets = append(subnets, subnet)
	}

	return subnets, nil
}

// GetSubnet gets a specific subnet by ID
func (m *MockAWSClient) GetSubnet(ctx context.Context, subnetID string) (*types.AWSResource, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if err := m.checkError("GetSubnet"); err != nil {
		return nil, err
	}

	subnet, exists := m.subnets[subnetID]
	if !exists {
		return nil, fmt.Errorf("subnet %s not found", subnetID)
	}

	return subnet, nil
}

// GetDefaultSubnet gets the default subnet info
func (m *MockAWSClient) GetDefaultSubnet(ctx context.Context) (*SubnetInfo, error) {
	if err := m.checkError("GetDefaultSubnet"); err != nil {
		return nil, err
	}

	return &SubnetInfo{
		SubnetID:         "subnet-12345678",
		VPCID:            "vpc-default-12345",
		AvailabilityZone: m.region + "a",
	}, nil
}

// GetSubnetsInVPC gets all subnets in a specific VPC
func (m *MockAWSClient) GetSubnetsInVPC(ctx context.Context, vpcID string) ([]string, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if err := m.checkError("GetSubnetsInVPC"); err != nil {
		return nil, err
	}

	var subnetIDs []string
	for _, subnet := range m.subnets {
		if subnetVpcID, ok := subnet.Details["vpcId"].(string); ok && subnetVpcID == vpcID {
			subnetIDs = append(subnetIDs, subnet.ID)
		}
	}

	return subnetIDs, nil
}

// ========== Security Group Methods ==========

// CreateSecurityGroup creates a new security group
func (m *MockAWSClient) CreateSecurityGroup(ctx context.Context, params aws.SecurityGroupParams) (*types.AWSResource, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// First validate parameters
	if err := m.validator.ValidateSecurityGroupParams(params); err != nil {
		m.logger.Error("Security group parameter validation failed", "error", err, "params", params)
		return nil, fmt.Errorf("parameter validation failed: %w", err)
	}

	if err := m.checkError("CreateSecurityGroup"); err != nil {
		return nil, err
	}

	sgID := fmt.Sprintf("sg-%s", generateID())
	securityGroup := &types.AWSResource{
		ID:     sgID,
		Type:   "security-group",
		Region: m.region,
		State:  "available",
		Details: map[string]interface{}{
			"groupName":   params.GroupName,
			"description": params.Description,
			"vpcId":       params.VpcID,
		},
		LastSeen: time.Now(),
	}

	m.securityGroups[sgID] = securityGroup
	return securityGroup, nil
}

// DescribeSecurityGroups lists all security groups
func (m *MockAWSClient) DescribeSecurityGroups(ctx context.Context) ([]*types.AWSResource, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if err := m.checkError("DescribeSecurityGroups"); err != nil {
		return nil, err
	}

	var securityGroups []*types.AWSResource
	for _, sg := range m.securityGroups {
		securityGroups = append(securityGroups, sg)
	}

	return securityGroups, nil
}

// GetSecurityGroup gets a specific security group by ID
func (m *MockAWSClient) GetSecurityGroup(ctx context.Context, sgID string) (*types.AWSResource, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if err := m.checkError("GetSecurityGroup"); err != nil {
		return nil, err
	}

	sg, exists := m.securityGroups[sgID]
	if !exists {
		return nil, fmt.Errorf("security group %s not found", sgID)
	}

	return sg, nil
}

// AuthorizeSecurityGroupIngress authorizes ingress rules for a security group
func (m *MockAWSClient) AuthorizeSecurityGroupIngress(ctx context.Context, params aws.SecurityGroupRuleParams) error {
	// First validate parameters
	if err := m.validator.ValidateSecurityGroupRuleParams(params); err != nil {
		m.logger.Error("Security group rule parameter validation failed", "error", err, "params", params)
		return fmt.Errorf("parameter validation failed: %w", err)
	}

	if err := m.checkError("AuthorizeSecurityGroupIngress"); err != nil {
		return err
	}

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if _, exists := m.securityGroups[params.GroupID]; !exists {
		return fmt.Errorf("security group %s not found", params.GroupID)
	}

	return nil
}

// AuthorizeSecurityGroupEgress authorizes egress rules for a security group
func (m *MockAWSClient) AuthorizeSecurityGroupEgress(ctx context.Context, params aws.SecurityGroupRuleParams) error {
	// First validate parameters
	if err := m.validator.ValidateSecurityGroupRuleParams(params); err != nil {
		m.logger.Error("Security group rule parameter validation failed", "error", err, "params", params)
		return fmt.Errorf("parameter validation failed: %w", err)
	}

	if err := m.checkError("AuthorizeSecurityGroupEgress"); err != nil {
		return err
	}

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if _, exists := m.securityGroups[params.GroupID]; !exists {
		return fmt.Errorf("security group %s not found", params.GroupID)
	}

	return nil
}

// ========== EC2 Instance Methods ==========

// CreateEC2Instance creates a new EC2 instance
func (m *MockAWSClient) CreateEC2Instance(ctx context.Context, params aws.CreateInstanceParams) (*types.AWSResource, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// First validate parameters
	if err := m.validator.ValidateInstanceParams(params); err != nil {
		m.logger.Error("EC2 instance parameter validation failed", "error", err, "params", params)
		return nil, fmt.Errorf("parameter validation failed: %w", err)
	}

	if err := m.checkError("CreateEC2Instance"); err != nil {
		return nil, err
	}

	instanceID := fmt.Sprintf("i-%s", generateID())
	instance := &types.AWSResource{
		ID:     instanceID,
		Type:   "ec2-instance",
		Region: m.region,
		State:  "running",
		Details: map[string]interface{}{
			"imageId":          params.ImageID,
			"instanceType":     params.InstanceType,
			"keyName":          params.KeyName,
			"securityGroupId":  params.SecurityGroupID,
			"subnetId":         params.SubnetID,
			"name":             params.Name,
			"publicIpAddress":  "1.2.3.4",
			"privateIpAddress": "10.0.0.4",
		},
		LastSeen: time.Now(),
	}

	m.instances[instanceID] = instance
	return instance, nil
}

// StartEC2Instance starts a stopped EC2 instance
func (m *MockAWSClient) StartEC2Instance(ctx context.Context, instanceID string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if err := m.checkError("StartEC2Instance"); err != nil {
		return err
	}

	instance, exists := m.instances[instanceID]
	if !exists {
		return fmt.Errorf("instance %s not found", instanceID)
	}

	instance.State = "running"
	return nil
}

// StopEC2Instance stops a running EC2 instance
func (m *MockAWSClient) StopEC2Instance(ctx context.Context, instanceID string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if err := m.checkError("StopEC2Instance"); err != nil {
		return err
	}

	instance, exists := m.instances[instanceID]
	if !exists {
		return fmt.Errorf("instance %s not found", instanceID)
	}

	instance.State = "stopped"
	return nil
}

// TerminateEC2Instance terminates an EC2 instance
func (m *MockAWSClient) TerminateEC2Instance(ctx context.Context, instanceID string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if err := m.checkError("TerminateEC2Instance"); err != nil {
		return err
	}

	instance, exists := m.instances[instanceID]
	if !exists {
		return fmt.Errorf("instance %s not found", instanceID)
	}

	instance.State = "terminated"
	return nil
}

// ListEC2Instances lists all EC2 instances (alias for DescribeInstances)
func (m *MockAWSClient) ListEC2Instances(ctx context.Context) ([]*types.AWSResource, error) {
	return m.DescribeInstances(ctx)
}

// DescribeInstances lists all EC2 instances
func (m *MockAWSClient) DescribeInstances(ctx context.Context) ([]*types.AWSResource, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if err := m.checkError("DescribeInstances"); err != nil {
		return nil, err
	}

	var instances []*types.AWSResource
	for _, instance := range m.instances {
		instances = append(instances, instance)
	}

	return instances, nil
}

// GetEC2Instance gets a specific EC2 instance by ID
func (m *MockAWSClient) GetEC2Instance(ctx context.Context, instanceID string) (*types.AWSResource, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if err := m.checkError("GetEC2Instance"); err != nil {
		return nil, err
	}

	instance, exists := m.instances[instanceID]
	if !exists {
		return nil, fmt.Errorf("instance %s not found", instanceID)
	}

	return instance, nil
}

// ========== AMI Methods ==========

// CreateAMI creates an Amazon Machine Image from an EC2 instance
func (m *MockAWSClient) CreateAMI(ctx context.Context, instanceID, name, description string) (*types.AWSResource, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if err := m.checkError("CreateAMI"); err != nil {
		return nil, err
	}

	// Verify instance exists
	if _, exists := m.instances[instanceID]; !exists {
		return nil, fmt.Errorf("instance %s not found", instanceID)
	}

	amiID := fmt.Sprintf("ami-%s", generateID())
	ami := &types.AWSResource{
		ID:     amiID,
		Type:   "ami",
		Region: m.region,
		State:  "available",
		Details: map[string]interface{}{
			"name":        name,
			"description": description,
			"instanceId":  instanceID,
		},
		LastSeen: time.Now(),
	}

	m.amis[amiID] = ami
	return ami, nil
}

// WaitForAMI waits for an AMI to become available
func (m *MockAWSClient) WaitForAMI(ctx context.Context, amiID string) error {
	if err := m.checkError("WaitForAMI"); err != nil {
		return err
	}

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if _, exists := m.amis[amiID]; !exists {
		return fmt.Errorf("ami %s not found", amiID)
	}

	return nil
}

// GetAMI gets a specific AMI by ID
func (m *MockAWSClient) GetAMI(ctx context.Context, amiID string) (*types.AWSResource, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if err := m.checkError("GetAMI"); err != nil {
		return nil, err
	}

	ami, exists := m.amis[amiID]
	if !exists {
		return nil, fmt.Errorf("ami %s not found", amiID)
	}

	return ami, nil
}

// ListAMIs lists Amazon Machine Images owned by the specified owner
func (m *MockAWSClient) ListAMIs(ctx context.Context, owner string) ([]*types.AWSResource, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if err := m.checkError("ListAMIs"); err != nil {
		return nil, err
	}

	var amis []*types.AWSResource
	for _, ami := range m.amis {
		amis = append(amis, ami)
	}

	return amis, nil
}

// DescribeAMIs lists all AMIs owned by the account
func (m *MockAWSClient) DescribeAMIs(ctx context.Context) ([]*types.AWSResource, error) {
	return m.ListAMIs(ctx, "self")
}

// DescribePublicAMIs lists public AMIs
func (m *MockAWSClient) DescribePublicAMIs(ctx context.Context, namePattern string) ([]*types.AWSResource, error) {
	return m.ListAMIs(ctx, "amazon")
}

// GetLatestAmazonLinux2AMI finds the latest Amazon Linux 2 AMI
func (m *MockAWSClient) GetLatestAmazonLinux2AMI(ctx context.Context) (string, error) {
	if err := m.checkError("GetLatestAmazonLinux2AMI"); err != nil {
		return "", err
	}

	return "ami-0abcdef1234567890", nil // Mock AMI ID
}

// GetLatestUbuntuAMI finds the latest Ubuntu LTS AMI
func (m *MockAWSClient) GetLatestUbuntuAMI(ctx context.Context, architecture string) (string, error) {
	if err := m.checkError("GetLatestUbuntuAMI"); err != nil {
		return "", err
	}

	return "ami-0abc123def456789", nil // Mock AMI ID
}

// GetLatestWindowsAMI finds the latest Windows Server AMI
func (m *MockAWSClient) GetLatestWindowsAMI(ctx context.Context, architecture string) (string, error) {
	if err := m.checkError("GetLatestWindowsAMI"); err != nil {
		return "", err
	}

	return "ami-windows1234567890", nil // Mock AMI ID
}

// GetAvailabilityZones retrieves all availability zones in the current region
func (m *MockAWSClient) GetAvailabilityZones(ctx context.Context) ([]string, error) {
	if err := m.checkError("GetAvailabilityZones"); err != nil {
		return nil, err
	}

	return []string{
		m.region + "a",
		m.region + "b",
		m.region + "c",
	}, nil
}

// ========== Application Load Balancer Methods ==========

// CreateApplicationLoadBalancer creates an Application Load Balancer
func (m *MockAWSClient) CreateApplicationLoadBalancer(ctx context.Context, params aws.CreateLoadBalancerParams) (*types.AWSResource, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// First validate parameters
	if err := m.validator.ValidateLoadBalancerParams(params); err != nil {
		m.logger.Error("Load balancer parameter validation failed", "error", err, "params", params)
		return nil, fmt.Errorf("parameter validation failed: %w", err)
	}

	if err := m.checkError("CreateApplicationLoadBalancer"); err != nil {
		return nil, err
	}

	elbArn := fmt.Sprintf("arn:aws:elasticloadbalancing:%s:123456789012:loadbalancer/app/%s/%s",
		m.region, params.Name, generateID())

	loadBalancer := &types.AWSResource{
		ID:     elbArn,
		Type:   "application-load-balancer",
		Region: m.region,
		State:  "active",
		Details: map[string]interface{}{
			"name":           params.Name,
			"scheme":         params.Scheme,
			"type":           params.Type,
			"dnsName":        fmt.Sprintf("%s-12345.%s.elb.amazonaws.com", params.Name, m.region),
			"subnets":        params.Subnets,
			"securityGroups": params.SecurityGroups,
		},
		LastSeen: time.Now(),
	}

	m.loadBalancers[elbArn] = loadBalancer
	return loadBalancer, nil
}

// CreateTargetGroup creates a target group for the load balancer
func (m *MockAWSClient) CreateTargetGroup(ctx context.Context, params aws.CreateTargetGroupParams) (*types.AWSResource, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// First validate parameters
	if err := m.validator.ValidateTargetGroupParams(params); err != nil {
		m.logger.Error("Target group parameter validation failed", "error", err, "params", params)
		return nil, fmt.Errorf("parameter validation failed: %w", err)
	}

	if err := m.checkError("CreateTargetGroup"); err != nil {
		return nil, err
	}

	tgArn := fmt.Sprintf("arn:aws:elasticloadbalancing:%s:123456789012:targetgroup/%s/%s",
		m.region, params.Name, generateID())

	targetGroup := &types.AWSResource{
		ID:     tgArn,
		Type:   "target-group",
		Region: m.region,
		State:  "active",
		Details: map[string]interface{}{
			"name":       params.Name,
			"protocol":   params.Protocol,
			"port":       params.Port,
			"vpcId":      params.VpcID,
			"targetType": params.TargetType,
		},
		LastSeen: time.Now(),
	}

	m.targetGroups[tgArn] = targetGroup
	return targetGroup, nil
}

// CreateListener creates a listener for the load balancer
func (m *MockAWSClient) CreateListener(ctx context.Context, params aws.CreateListenerParams) (*types.AWSResource, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// First validate parameters
	if err := m.validator.ValidateListenerParams(params); err != nil {
		m.logger.Error("Listener parameter validation failed", "error", err, "params", params)
		return nil, fmt.Errorf("parameter validation failed: %w", err)
	}

	if err := m.checkError("CreateListener"); err != nil {
		return nil, err
	}

	listenerArn := fmt.Sprintf("arn:aws:elasticloadbalancing:%s:123456789012:listener/app/%s/%s/%s",
		m.region, "load-balancer", generateID(), generateID())

	listener := &types.AWSResource{
		ID:     listenerArn,
		Type:   "listener",
		Region: m.region,
		State:  "active",
		Details: map[string]interface{}{
			"loadBalancerArn": params.LoadBalancerArn,
			"protocol":        params.Protocol,
			"port":            params.Port,
			"targetGroupArn":  params.DefaultTargetGroupArn,
		},
		LastSeen: time.Now(),
	}

	m.listeners[listenerArn] = listener
	return listener, nil
}

// DescribeLoadBalancers lists all Load Balancers
func (m *MockAWSClient) DescribeLoadBalancers(ctx context.Context) ([]*types.AWSResource, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if err := m.checkError("DescribeLoadBalancers"); err != nil {
		return nil, err
	}

	var loadBalancers []*types.AWSResource
	for _, lb := range m.loadBalancers {
		loadBalancers = append(loadBalancers, lb)
	}

	return loadBalancers, nil
}

// GetLoadBalancer gets a specific Load Balancer by ARN
func (m *MockAWSClient) GetLoadBalancer(ctx context.Context, loadBalancerArn string) (*types.AWSResource, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if err := m.checkError("GetLoadBalancer"); err != nil {
		return nil, err
	}

	lb, exists := m.loadBalancers[loadBalancerArn]
	if !exists {
		return nil, fmt.Errorf("load balancer %s not found", loadBalancerArn)
	}

	return lb, nil
}

// DescribeTargetGroups lists all Target Groups
func (m *MockAWSClient) DescribeTargetGroups(ctx context.Context) ([]*types.AWSResource, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if err := m.checkError("DescribeTargetGroups"); err != nil {
		return nil, err
	}

	var targetGroups []*types.AWSResource
	for _, tg := range m.targetGroups {
		targetGroups = append(targetGroups, tg)
	}

	return targetGroups, nil
}

// GetTargetGroup gets a specific Target Group by ARN
func (m *MockAWSClient) GetTargetGroup(ctx context.Context, targetGroupArn string) (*types.AWSResource, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if err := m.checkError("GetTargetGroup"); err != nil {
		return nil, err
	}

	tg, exists := m.targetGroups[targetGroupArn]
	if !exists {
		return nil, fmt.Errorf("target group %s not found", targetGroupArn)
	}

	return tg, nil
}

// RegisterTargets registers targets with a target group
func (m *MockAWSClient) RegisterTargets(ctx context.Context, targetGroupArn string, targetIDs []string) error {
	if err := m.checkError("RegisterTargets"); err != nil {
		return err
	}

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if _, exists := m.targetGroups[targetGroupArn]; !exists {
		return fmt.Errorf("target group %s not found", targetGroupArn)
	}

	return nil
}

// DeregisterTargets deregisters targets from a target group
func (m *MockAWSClient) DeregisterTargets(ctx context.Context, targetGroupArn string, targetIDs []string) error {
	if err := m.checkError("DeregisterTargets"); err != nil {
		return err
	}

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if _, exists := m.targetGroups[targetGroupArn]; !exists {
		return fmt.Errorf("target group %s not found", targetGroupArn)
	}

	return nil
}

// ========== Auto Scaling Group Methods ==========

// CreateLaunchTemplate creates a launch template for auto scaling
func (m *MockAWSClient) CreateLaunchTemplate(ctx context.Context, params aws.CreateLaunchTemplateParams) (*types.AWSResource, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// First validate parameters
	if err := m.validator.ValidateLaunchTemplateParams(params); err != nil {
		m.logger.Error("Launch template parameter validation failed", "error", err, "params", params)
		return nil, fmt.Errorf("parameter validation failed: %w", err)
	}

	if err := m.checkError("CreateLaunchTemplate"); err != nil {
		return nil, err
	}

	templateID := fmt.Sprintf("lt-%s", generateID())
	launchTemplate := &types.AWSResource{
		ID:     templateID,
		Type:   "launch-template",
		Region: m.region,
		State:  "available",
		Details: map[string]interface{}{
			"name":             params.LaunchTemplateName,
			"imageId":          params.ImageID,
			"instanceType":     params.InstanceType,
			"keyName":          params.KeyName,
			"securityGroupIds": params.SecurityGroupIDs,
		},
		LastSeen: time.Now(),
	}

	m.launchTemplates[templateID] = launchTemplate
	return launchTemplate, nil
}

// CreateAutoScalingGroup creates an auto scaling group
func (m *MockAWSClient) CreateAutoScalingGroup(ctx context.Context, params aws.CreateAutoScalingGroupParams) (*types.AWSResource, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// First validate parameters
	if err := m.validator.ValidateAutoScalingGroupParams(params); err != nil {
		m.logger.Error("Auto scaling group parameter validation failed", "error", err, "params", params)
		return nil, fmt.Errorf("parameter validation failed: %w", err)
	}

	if err := m.checkError("CreateAutoScalingGroup"); err != nil {
		return nil, err
	}

	asg := &types.AWSResource{
		ID:     params.AutoScalingGroupName,
		Type:   "auto-scaling-group",
		Region: m.region,
		State:  "active",
		Details: map[string]interface{}{
			"minSize":            params.MinSize,
			"maxSize":            params.MaxSize,
			"desiredCapacity":    params.DesiredCapacity,
			"launchTemplateName": params.LaunchTemplateName,
			"vpcZoneIdentifiers": params.VPCZoneIdentifiers,
			"targetGroupARNs":    params.TargetGroupARNs,
			"healthCheckType":    params.HealthCheckType,
		},
		LastSeen: time.Now(),
	}

	m.autoScalingGroups[params.AutoScalingGroupName] = asg
	return asg, nil
}

// UpdateAutoScalingGroup updates the desired capacity of an auto scaling group
func (m *MockAWSClient) UpdateAutoScalingGroup(ctx context.Context, asgName string, desiredCapacity int32) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if err := m.checkError("UpdateAutoScalingGroup"); err != nil {
		return err
	}

	asg, exists := m.autoScalingGroups[asgName]
	if !exists {
		return fmt.Errorf("auto scaling group %s not found", asgName)
	}

	asg.Details["desiredCapacity"] = desiredCapacity
	return nil
}

// DeleteAutoScalingGroup deletes an auto scaling group
func (m *MockAWSClient) DeleteAutoScalingGroup(ctx context.Context, asgName string, forceDelete bool) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if err := m.checkError("DeleteAutoScalingGroup"); err != nil {
		return err
	}

	if _, exists := m.autoScalingGroups[asgName]; !exists {
		return fmt.Errorf("auto scaling group %s not found", asgName)
	}

	delete(m.autoScalingGroups, asgName)
	return nil
}

// DescribeAutoScalingGroups lists all Auto Scaling Groups
func (m *MockAWSClient) DescribeAutoScalingGroups(ctx context.Context) ([]*types.AWSResource, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if err := m.checkError("DescribeAutoScalingGroups"); err != nil {
		return nil, err
	}

	var asgs []*types.AWSResource
	for _, asg := range m.autoScalingGroups {
		asgs = append(asgs, asg)
	}

	return asgs, nil
}

// GetAutoScalingGroup gets a specific Auto Scaling Group by name
func (m *MockAWSClient) GetAutoScalingGroup(ctx context.Context, groupName string) (*types.AWSResource, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if err := m.checkError("GetAutoScalingGroup"); err != nil {
		return nil, err
	}

	asg, exists := m.autoScalingGroups[groupName]
	if !exists {
		return nil, fmt.Errorf("auto scaling group %s not found", groupName)
	}

	return asg, nil
}

// DescribeLaunchTemplates lists all Launch Templates
func (m *MockAWSClient) DescribeLaunchTemplates(ctx context.Context) ([]*types.AWSResource, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if err := m.checkError("DescribeLaunchTemplates"); err != nil {
		return nil, err
	}

	var templates []*types.AWSResource
	for _, template := range m.launchTemplates {
		templates = append(templates, template)
	}

	return templates, nil
}

// GetLaunchTemplate gets a specific Launch Template by ID
func (m *MockAWSClient) GetLaunchTemplate(ctx context.Context, templateID string) (*types.AWSResource, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if err := m.checkError("GetLaunchTemplate"); err != nil {
		return nil, err
	}

	template, exists := m.launchTemplates[templateID]
	if !exists {
		return nil, fmt.Errorf("launch template %s not found", templateID)
	}

	return template, nil
}

// ========== RDS Methods ==========

// CreateDBInstance creates a new RDS database instance
func (m *MockAWSClient) CreateDBInstance(ctx context.Context, params aws.CreateDBInstanceParams) (*types.AWSResource, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// First validate parameters
	if err := m.validator.ValidateDBInstanceParams(params); err != nil {
		m.logger.Error("DB instance parameter validation failed", "error", err, "params", params)
		return nil, fmt.Errorf("parameter validation failed: %w", err)
	}

	if err := m.checkError("CreateDBInstance"); err != nil {
		return nil, err
	}

	dbInstance := &types.AWSResource{
		ID:     params.DBInstanceIdentifier,
		Type:   "db-instance",
		Region: m.region,
		State:  "available",
		Details: map[string]interface{}{
			"engine":           params.Engine,
			"engineVersion":    params.EngineVersion,
			"dbInstanceClass":  params.DBInstanceClass,
			"allocatedStorage": params.AllocatedStorage,
			"masterUsername":   params.MasterUsername,
		},
		LastSeen: time.Now(),
	}

	m.dbInstances[params.DBInstanceIdentifier] = dbInstance
	return dbInstance, nil
}

// DescribeDBInstances lists all RDS database instances
func (m *MockAWSClient) DescribeDBInstances(ctx context.Context) ([]*types.AWSResource, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if err := m.checkError("DescribeDBInstances"); err != nil {
		return nil, err
	}

	var dbInstances []*types.AWSResource
	for _, db := range m.dbInstances {
		dbInstances = append(dbInstances, db)
	}

	return dbInstances, nil
}

// GetDBInstance gets a specific RDS database instance by identifier
func (m *MockAWSClient) GetDBInstance(ctx context.Context, dbInstanceIdentifier string) (*types.AWSResource, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if err := m.checkError("GetDBInstance"); err != nil {
		return nil, err
	}

	db, exists := m.dbInstances[dbInstanceIdentifier]
	if !exists {
		return nil, fmt.Errorf("db instance %s not found", dbInstanceIdentifier)
	}

	return db, nil
}

// DeleteDBInstance deletes an RDS database instance
func (m *MockAWSClient) DeleteDBInstance(ctx context.Context, dbInstanceIdentifier string, skipFinalSnapshot bool) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if err := m.checkError("DeleteDBInstance"); err != nil {
		return err
	}

	if _, exists := m.dbInstances[dbInstanceIdentifier]; !exists {
		return fmt.Errorf("db instance %s not found", dbInstanceIdentifier)
	}

	delete(m.dbInstances, dbInstanceIdentifier)
	return nil
}

// ========== Utility Methods ==========

// ClearAllData clears all mock data for clean testing
func (m *MockAWSClient) ClearAllData() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.vpcs = make(map[string]*types.AWSResource)
	m.subnets = make(map[string]*types.AWSResource)
	m.securityGroups = make(map[string]*types.AWSResource)
	m.instances = make(map[string]*types.AWSResource)
	m.loadBalancers = make(map[string]*types.AWSResource)
	m.targetGroups = make(map[string]*types.AWSResource)
	m.listeners = make(map[string]*types.AWSResource)
	m.launchTemplates = make(map[string]*types.AWSResource)
	m.autoScalingGroups = make(map[string]*types.AWSResource)
	m.amis = make(map[string]*types.AWSResource)
	m.dbInstances = make(map[string]*types.AWSResource)
	m.methodErrors = make(map[string]error)
}

// AddDefaultTestData adds some default test data
func (m *MockAWSClient) AddDefaultTestData() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Add default VPC
	defaultVPC := &types.AWSResource{
		ID:     "vpc-default-12345",
		Type:   "vpc",
		Region: m.region,
		State:  "available",
		Details: map[string]interface{}{
			"cidrBlock": "172.31.0.0/16",
			"isDefault": true,
		},
		LastSeen: time.Now(),
	}
	m.vpcs["vpc-default-12345"] = defaultVPC

	// Add default subnet
	defaultSubnet := &types.AWSResource{
		ID:     "subnet-12345678",
		Type:   "subnet",
		Region: m.region,
		State:  "available",
		Details: map[string]interface{}{
			"vpcId":               "vpc-default-12345",
			"cidrBlock":           "172.31.1.0/24",
			"availabilityZone":    m.region + "a",
			"mapPublicIpOnLaunch": true,
		},
		LastSeen: time.Now(),
	}
	m.subnets["subnet-12345678"] = defaultSubnet

	// Add default security group
	defaultSG := &types.AWSResource{
		ID:     "sg-default-12345",
		Type:   "security-group",
		Region: m.region,
		State:  "available",
		Details: map[string]interface{}{
			"groupName":   "default",
			"description": "Default security group",
			"vpcId":       "vpc-default-12345",
		},
		LastSeen: time.Now(),
	}
	m.securityGroups["sg-default-12345"] = defaultSG
}

// generateID generates a simple ID for testing purposes
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano()%1000000000000)
}

// ==== Enhanced Mock Client Methods ====

//
// GetRegion returns the configured region
//
// (Method already exists, removing duplicate)

// SetRegion updates the configured region
func (m *MockAWSClient) SetRegion(region string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.region = region
}

// GetResourceByID retrieves a resource by ID regardless of type
func (m *MockAWSClient) GetResourceByID(resourceId string) *types.AWSResource {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Check all resource types
	if resource, exists := m.vpcs[resourceId]; exists {
		return resource
	}
	if resource, exists := m.subnets[resourceId]; exists {
		return resource
	}
	if resource, exists := m.securityGroups[resourceId]; exists {
		return resource
	}
	if resource, exists := m.instances[resourceId]; exists {
		return resource
	}
	if resource, exists := m.loadBalancers[resourceId]; exists {
		return resource
	}
	if resource, exists := m.targetGroups[resourceId]; exists {
		return resource
	}
	if resource, exists := m.launchTemplates[resourceId]; exists {
		return resource
	}
	if resource, exists := m.autoScalingGroups[resourceId]; exists {
		return resource
	}
	if resource, exists := m.dbInstances[resourceId]; exists {
		return resource
	}

	return nil
}

// GetResourcesByType retrieves all resources of a specific type
func (m *MockAWSClient) GetResourcesByType(resourceType string) map[string]*types.AWSResource {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	result := make(map[string]*types.AWSResource)

	switch resourceType {
	case "vpc":
		for k, v := range m.vpcs {
			result[k] = v
		}
	case "subnet":
		for k, v := range m.subnets {
			result[k] = v
		}
	case "security_group":
		for k, v := range m.securityGroups {
			result[k] = v
		}
	case "ec2_instance":
		for k, v := range m.instances {
			result[k] = v
		}
	case "load_balancer":
		for k, v := range m.loadBalancers {
			result[k] = v
		}
	case "target_group":
		for k, v := range m.targetGroups {
			result[k] = v
		}
	case "launch_template":
		for k, v := range m.launchTemplates {
			result[k] = v
		}
	case "auto_scaling_group":
		for k, v := range m.autoScalingGroups {
			result[k] = v
		}
	case "db_instance":
		for k, v := range m.dbInstances {
			result[k] = v
		}
	}

	return result
}

// GetAllResources returns all resources across all types
func (m *MockAWSClient) GetAllResources() map[string]*types.AWSResource {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	result := make(map[string]*types.AWSResource)

	// Collect all resources
	for k, v := range m.vpcs {
		result[k] = v
	}
	for k, v := range m.subnets {
		result[k] = v
	}
	for k, v := range m.securityGroups {
		result[k] = v
	}
	for k, v := range m.instances {
		result[k] = v
	}
	for k, v := range m.loadBalancers {
		result[k] = v
	}
	for k, v := range m.targetGroups {
		result[k] = v
	}
	for k, v := range m.launchTemplates {
		result[k] = v
	}
	for k, v := range m.autoScalingGroups {
		result[k] = v
	}
	for k, v := range m.dbInstances {
		result[k] = v
	}

	return result
}

// AddMockResource adds a custom resource for testing
func (m *MockAWSClient) AddMockResource(resourceType, resourceId string, details map[string]interface{}) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	resource := &types.AWSResource{
		ID:       resourceId,
		Type:     resourceType,
		Region:   m.region,
		State:    "available",
		Details:  details,
		LastSeen: time.Now(),
	}

	switch resourceType {
	case "vpc":
		m.vpcs[resourceId] = resource
	case "subnet":
		m.subnets[resourceId] = resource
	case "security_group":
		m.securityGroups[resourceId] = resource
	case "ec2_instance":
		m.instances[resourceId] = resource
	case "load_balancer":
		m.loadBalancers[resourceId] = resource
	case "target_group":
		m.targetGroups[resourceId] = resource
	case "launch_template":
		m.launchTemplates[resourceId] = resource
	case "auto_scaling_group":
		m.autoScalingGroups[resourceId] = resource
	case "db_instance":
		m.dbInstances[resourceId] = resource
	default:
		return fmt.Errorf("unsupported resource type: %s", resourceType)
	}

	return nil
}

// RemoveMockResource removes a resource for testing
func (m *MockAWSClient) RemoveMockResource(resourceType, resourceId string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	switch resourceType {
	case "vpc":
		if _, exists := m.vpcs[resourceId]; !exists {
			return fmt.Errorf("VPC %s not found", resourceId)
		}
		delete(m.vpcs, resourceId)
	case "subnet":
		if _, exists := m.subnets[resourceId]; !exists {
			return fmt.Errorf("subnet %s not found", resourceId)
		}
		delete(m.subnets, resourceId)
	case "security_group":
		if _, exists := m.securityGroups[resourceId]; !exists {
			return fmt.Errorf("security group %s not found", resourceId)
		}
		delete(m.securityGroups, resourceId)
	case "ec2_instance":
		if _, exists := m.instances[resourceId]; !exists {
			return fmt.Errorf("instance %s not found", resourceId)
		}
		delete(m.instances, resourceId)
	case "load_balancer":
		if _, exists := m.loadBalancers[resourceId]; !exists {
			return fmt.Errorf("load balancer %s not found", resourceId)
		}
		delete(m.loadBalancers, resourceId)
	case "target_group":
		if _, exists := m.targetGroups[resourceId]; !exists {
			return fmt.Errorf("target group %s not found", resourceId)
		}
		delete(m.targetGroups, resourceId)
	case "launch_template":
		if _, exists := m.launchTemplates[resourceId]; !exists {
			return fmt.Errorf("launch template %s not found", resourceId)
		}
		delete(m.launchTemplates, resourceId)
	case "auto_scaling_group":
		if _, exists := m.autoScalingGroups[resourceId]; !exists {
			return fmt.Errorf("auto scaling group %s not found", resourceId)
		}
		delete(m.autoScalingGroups, resourceId)
	case "db_instance":
		if _, exists := m.dbInstances[resourceId]; !exists {
			return fmt.Errorf("DB instance %s not found", resourceId)
		}
		delete(m.dbInstances, resourceId)
	default:
		return fmt.Errorf("unsupported resource type: %s", resourceType)
	}

	return nil
}

// GetResourceCount returns the total count of all resources
func (m *MockAWSClient) GetResourceCount() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	count := len(m.vpcs) + len(m.subnets) + len(m.securityGroups) + len(m.instances) +
		len(m.loadBalancers) + len(m.targetGroups) + len(m.launchTemplates) +
		len(m.autoScalingGroups) + len(m.dbInstances)

	return count
}

// GetResourceCountByType returns count of resources by type
func (m *MockAWSClient) GetResourceCountByType() map[string]int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return map[string]int{
		"vpc":                len(m.vpcs),
		"subnet":             len(m.subnets),
		"security_group":     len(m.securityGroups),
		"ec2_instance":       len(m.instances),
		"load_balancer":      len(m.loadBalancers),
		"target_group":       len(m.targetGroups),
		"launch_template":    len(m.launchTemplates),
		"auto_scaling_group": len(m.autoScalingGroups),
		"db_instance":        len(m.dbInstances),
	}
}

// ClearAllResources removes all resources (useful for test cleanup)
func (m *MockAWSClient) ClearAllResources() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.vpcs = make(map[string]*types.AWSResource)
	m.subnets = make(map[string]*types.AWSResource)
	m.securityGroups = make(map[string]*types.AWSResource)
	m.instances = make(map[string]*types.AWSResource)
	m.loadBalancers = make(map[string]*types.AWSResource)
	m.targetGroups = make(map[string]*types.AWSResource)
	m.launchTemplates = make(map[string]*types.AWSResource)
	m.autoScalingGroups = make(map[string]*types.AWSResource)
	m.dbInstances = make(map[string]*types.AWSResource)
}

// ResetToDefaults resets the mock client to its default state with default resources
func (m *MockAWSClient) ResetToDefaults() {
	m.ClearAllResources()
	m.AddDefaultTestData()
}

// SetResourceState updates the state of a specific resource
func (m *MockAWSClient) SetResourceState(resourceType, resourceId, state string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	var resource *types.AWSResource
	switch resourceType {
	case "vpc":
		resource = m.vpcs[resourceId]
	case "subnet":
		resource = m.subnets[resourceId]
	case "security_group":
		resource = m.securityGroups[resourceId]
	case "ec2_instance":
		resource = m.instances[resourceId]
	case "load_balancer":
		resource = m.loadBalancers[resourceId]
	case "target_group":
		resource = m.targetGroups[resourceId]
	case "launch_template":
		resource = m.launchTemplates[resourceId]
	case "auto_scaling_group":
		resource = m.autoScalingGroups[resourceId]
	case "db_instance":
		resource = m.dbInstances[resourceId]
	default:
		return fmt.Errorf("unsupported resource type: %s", resourceType)
	}

	if resource == nil {
		return fmt.Errorf("resource %s of type %s not found", resourceId, resourceType)
	}

	resource.State = state
	resource.LastSeen = time.Now()
	return nil
}
