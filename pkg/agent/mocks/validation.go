package mocks

import (
	"fmt"
	"net"
	"regexp"
	"strings"

	"github.com/versus-control/ai-infrastructure-agent/pkg/aws"
)

// ValidationError represents a parameter validation error
type ValidationError struct {
	Parameter string
	Value     string
	Message   string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error for parameter '%s' (value: '%s'): %s", e.Parameter, e.Value, e.Message)
}

// ValidationErrors represents multiple validation errors
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	var messages []string
	for _, err := range e {
		messages = append(messages, err.Error())
	}
	return strings.Join(messages, "; ")
}

// HasErrors returns true if there are validation errors
func (e ValidationErrors) HasErrors() bool {
	return len(e) > 0
}

// Add adds a validation error
func (e *ValidationErrors) Add(parameter, value, message string) {
	*e = append(*e, ValidationError{
		Parameter: parameter,
		Value:     value,
		Message:   message,
	})
}

// ParameterValidator provides comprehensive AWS parameter validation
type ParameterValidator struct{}

// NewParameterValidator creates a new parameter validator
func NewParameterValidator() *ParameterValidator {
	return &ParameterValidator{}
}

// ValidateVPCParams validates VPC creation parameters
func (v *ParameterValidator) ValidateVPCParams(params aws.CreateVPCParams) error {
	var errors ValidationErrors

	// Validate CIDR block
	if params.CidrBlock == "" {
		errors.Add("CidrBlock", "", "CIDR block is required")
	} else {
		if err := v.validateCIDRBlock(params.CidrBlock); err != nil {
			errors.Add("CidrBlock", params.CidrBlock, err.Error())
		}
	}

	// Validate name
	if params.Name == "" {
		errors.Add("Name", "", "VPC name is required")
	} else if len(params.Name) > 255 {
		errors.Add("Name", params.Name, "VPC name must be 255 characters or less")
	}

	// Validate tags
	if err := v.validateTags(params.Tags); err != nil {
		errors.Add("Tags", "", err.Error())
	}

	if errors.HasErrors() {
		return errors
	}
	return nil
}

// ValidateSubnetParams validates subnet creation parameters
func (v *ParameterValidator) ValidateSubnetParams(params aws.CreateSubnetParams) error {
	var errors ValidationErrors

	// Validate VPC ID
	if params.VpcID == "" {
		errors.Add("VpcID", "", "VPC ID is required")
	} else if !v.isValidVPCID(params.VpcID) {
		errors.Add("VpcID", params.VpcID, "invalid VPC ID format (expected vpc-xxxxxxxx)")
	}

	// Validate CIDR block
	if params.CidrBlock == "" {
		errors.Add("CidrBlock", "", "CIDR block is required")
	} else {
		if err := v.validateCIDRBlock(params.CidrBlock); err != nil {
			errors.Add("CidrBlock", params.CidrBlock, err.Error())
		}
	}

	// Validate availability zone
	if params.AvailabilityZone == "" {
		errors.Add("AvailabilityZone", "", "availability zone is required")
	} else if !v.isValidAvailabilityZone(params.AvailabilityZone) {
		errors.Add("AvailabilityZone", params.AvailabilityZone, "invalid availability zone format")
	}

	// Validate name
	if params.Name == "" {
		errors.Add("Name", "", "subnet name is required")
	} else if len(params.Name) > 255 {
		errors.Add("Name", params.Name, "subnet name must be 255 characters or less")
	}

	if errors.HasErrors() {
		return errors
	}
	return nil
}

// ValidateInstanceParams validates EC2 instance creation parameters
func (v *ParameterValidator) ValidateInstanceParams(params aws.CreateInstanceParams) error {
	var errors ValidationErrors

	// Validate AMI ID
	if params.ImageID == "" {
		errors.Add("ImageID", "", "AMI ID is required")
	} else if !v.isValidAMIID(params.ImageID) {
		errors.Add("ImageID", params.ImageID, "invalid AMI ID format (expected ami-xxxxxxxx)")
	}

	// Validate instance type
	if params.InstanceType == "" {
		errors.Add("InstanceType", "", "instance type is required")
	} else if !v.isValidInstanceType(params.InstanceType) {
		errors.Add("InstanceType", params.InstanceType, "invalid instance type")
	}

	// Validate security group ID (if provided)
	if params.SecurityGroupID != "" && !v.isValidSecurityGroupID(params.SecurityGroupID) {
		errors.Add("SecurityGroupID", params.SecurityGroupID, "invalid security group ID format (expected sg-xxxxxxxx)")
	}

	// Validate subnet ID (if provided)
	if params.SubnetID != "" && !v.isValidSubnetID(params.SubnetID) {
		errors.Add("SubnetID", params.SubnetID, "invalid subnet ID format (expected subnet-xxxxxxxx)")
	}

	if errors.HasErrors() {
		return errors
	}
	return nil
}

// ValidateLaunchTemplateParams validates launch template creation parameters
func (v *ParameterValidator) ValidateLaunchTemplateParams(params aws.CreateLaunchTemplateParams) error {
	var errors ValidationErrors

	// Validate template name
	if params.LaunchTemplateName == "" {
		errors.Add("LaunchTemplateName", "", "launch template name is required")
	} else if len(params.LaunchTemplateName) > 128 {
		errors.Add("LaunchTemplateName", params.LaunchTemplateName, "launch template name must be 128 characters or less")
	} else if !v.isValidLaunchTemplateName(params.LaunchTemplateName) {
		errors.Add("LaunchTemplateName", params.LaunchTemplateName, "invalid launch template name format")
	}

	// Validate AMI ID
	if params.ImageID == "" {
		errors.Add("ImageID", "", "AMI ID is required")
	} else if !v.isValidAMIID(params.ImageID) {
		errors.Add("ImageID", params.ImageID, "invalid AMI ID format (expected ami-xxxxxxxx)")
	}

	// Validate instance type
	if params.InstanceType == "" {
		errors.Add("InstanceType", "", "instance type is required")
	} else if !v.isValidInstanceType(params.InstanceType) {
		errors.Add("InstanceType", params.InstanceType, "invalid instance type")
	}

	// Validate security group IDs
	for i, sgID := range params.SecurityGroupIDs {
		if !v.isValidSecurityGroupID(sgID) {
			errors.Add(fmt.Sprintf("SecurityGroupIDs[%d]", i), sgID, "invalid security group ID format (expected sg-xxxxxxxx)")
		}
	}

	if errors.HasErrors() {
		return errors
	}
	return nil
}

// ValidateAutoScalingGroupParams validates auto scaling group creation parameters
func (v *ParameterValidator) ValidateAutoScalingGroupParams(params aws.CreateAutoScalingGroupParams) error {
	var errors ValidationErrors

	// Validate ASG name
	if params.AutoScalingGroupName == "" {
		errors.Add("AutoScalingGroupName", "", "auto scaling group name is required")
	} else if len(params.AutoScalingGroupName) > 255 {
		errors.Add("AutoScalingGroupName", params.AutoScalingGroupName, "auto scaling group name must be 255 characters or less")
	}

	// Validate launch template name
	if params.LaunchTemplateName == "" {
		errors.Add("LaunchTemplateName", "", "launch template name is required")
	} else if !v.isValidLaunchTemplateName(params.LaunchTemplateName) {
		errors.Add("LaunchTemplateName", params.LaunchTemplateName, "invalid launch template name format")
	}

	// Validate launch template version
	if params.LaunchTemplateVersion == "" {
		errors.Add("LaunchTemplateVersion", "", "launch template version is required")
	} else if !v.isValidLaunchTemplateVersion(params.LaunchTemplateVersion) {
		errors.Add("LaunchTemplateVersion", params.LaunchTemplateVersion, "invalid launch template version (expected $Latest, $Default, or version number)")
	}

	// Validate capacity values
	if params.MinSize < 0 {
		errors.Add("MinSize", fmt.Sprintf("%d", params.MinSize), "minimum size must be non-negative")
	}

	if params.MaxSize < 0 {
		errors.Add("MaxSize", fmt.Sprintf("%d", params.MaxSize), "maximum size must be non-negative")
	}

	if params.MinSize > params.MaxSize {
		errors.Add("MinSize", fmt.Sprintf("%d", params.MinSize), "minimum size cannot be greater than maximum size")
	}

	if params.DesiredCapacity < params.MinSize || params.DesiredCapacity > params.MaxSize {
		errors.Add("DesiredCapacity", fmt.Sprintf("%d", params.DesiredCapacity), "desired capacity must be between minimum and maximum size")
	}

	// Validate subnet IDs
	if len(params.VPCZoneIdentifiers) == 0 {
		errors.Add("VPCZoneIdentifiers", "", "at least one subnet ID is required")
	} else {
		for i, subnetID := range params.VPCZoneIdentifiers {
			if !v.isValidSubnetID(subnetID) {
				errors.Add(fmt.Sprintf("VPCZoneIdentifiers[%d]", i), subnetID, "invalid subnet ID format (expected subnet-xxxxxxxx)")
			}
		}
	}

	// Validate health check type
	if params.HealthCheckType != "" && params.HealthCheckType != "EC2" && params.HealthCheckType != "ELB" {
		errors.Add("HealthCheckType", params.HealthCheckType, "health check type must be either 'EC2' or 'ELB'")
	}

	if errors.HasErrors() {
		return errors
	}
	return nil
}

// ValidateLoadBalancerParams validates load balancer creation parameters
func (v *ParameterValidator) ValidateLoadBalancerParams(params aws.CreateLoadBalancerParams) error {
	var errors ValidationErrors

	// Validate name
	if params.Name == "" {
		errors.Add("Name", "", "load balancer name is required")
	} else if len(params.Name) > 32 {
		errors.Add("Name", params.Name, "load balancer name must be 32 characters or less")
	} else if !v.isValidLoadBalancerName(params.Name) {
		errors.Add("Name", params.Name, "invalid load balancer name format")
	}

	// Validate scheme
	if params.Scheme != "" && params.Scheme != "internet-facing" && params.Scheme != "internal" {
		errors.Add("Scheme", params.Scheme, "scheme must be either 'internet-facing' or 'internal'")
	}

	// Validate type
	if params.Type != "" && params.Type != "application" && params.Type != "network" && params.Type != "gateway" {
		errors.Add("Type", params.Type, "type must be 'application', 'network', or 'gateway'")
	}

	// Validate subnets
	if len(params.Subnets) < 2 {
		errors.Add("Subnets", "", "at least 2 subnets are required for load balancer")
	} else {
		for i, subnetID := range params.Subnets {
			if !v.isValidSubnetID(subnetID) {
				errors.Add(fmt.Sprintf("Subnets[%d]", i), subnetID, "invalid subnet ID format (expected subnet-xxxxxxxx)")
			}
		}
	}

	// Validate security groups (for ALB only)
	if params.Type == "application" {
		for i, sgID := range params.SecurityGroups {
			if !v.isValidSecurityGroupID(sgID) {
				errors.Add(fmt.Sprintf("SecurityGroups[%d]", i), sgID, "invalid security group ID format (expected sg-xxxxxxxx)")
			}
		}
	}

	if errors.HasErrors() {
		return errors
	}
	return nil
}

// ValidateSecurityGroupParams validates security group creation parameters
func (v *ParameterValidator) ValidateSecurityGroupParams(params aws.SecurityGroupParams) error {
	var errors ValidationErrors

	// Validate group name
	if params.GroupName == "" {
		errors.Add("GroupName", "", "security group name is required")
	} else if len(params.GroupName) > 255 {
		errors.Add("GroupName", params.GroupName, "security group name must be 255 characters or less")
	} else if !v.isValidSecurityGroupName(params.GroupName) {
		errors.Add("GroupName", params.GroupName, "invalid security group name format")
	}

	// Validate description
	if params.Description == "" {
		errors.Add("Description", "", "security group description is required")
	} else if len(params.Description) > 255 {
		errors.Add("Description", params.Description, "security group description must be 255 characters or less")
	}

	// Validate VPC ID (if provided)
	if params.VpcID != "" && !v.isValidVPCID(params.VpcID) {
		errors.Add("VpcID", params.VpcID, "invalid VPC ID format (expected vpc-xxxxxxxx)")
	}

	// Validate tags
	if err := v.validateTags(params.Tags); err != nil {
		errors.Add("Tags", "", err.Error())
	}

	if errors.HasErrors() {
		return errors
	}
	return nil
}

// ValidateSecurityGroupRuleParams validates security group rule parameters
func (v *ParameterValidator) ValidateSecurityGroupRuleParams(params aws.SecurityGroupRuleParams) error {
	var errors ValidationErrors

	// Validate group ID
	if params.GroupID == "" {
		errors.Add("GroupID", "", "security group ID is required")
	} else if !v.isValidSecurityGroupID(params.GroupID) {
		errors.Add("GroupID", params.GroupID, "invalid security group ID format (expected sg-xxxxxxxx)")
	}

	// Validate type
	if params.Type == "" {
		errors.Add("Type", "", "rule type is required")
	} else if params.Type != "ingress" && params.Type != "egress" {
		errors.Add("Type", params.Type, "rule type must be either 'ingress' or 'egress'")
	}

	// Validate protocol
	if params.Protocol == "" {
		errors.Add("Protocol", "", "protocol is required")
	} else if !v.isValidProtocol(params.Protocol) {
		errors.Add("Protocol", params.Protocol, "invalid protocol (must be tcp, udp, icmp, or -1)")
	}

	// Validate port range
	if params.Protocol == "tcp" || params.Protocol == "udp" {
		if params.FromPort < 0 || params.FromPort > 65535 {
			errors.Add("FromPort", fmt.Sprintf("%d", params.FromPort), "port must be between 0 and 65535")
		}
		if params.ToPort < 0 || params.ToPort > 65535 {
			errors.Add("ToPort", fmt.Sprintf("%d", params.ToPort), "port must be between 0 and 65535")
		}
		if params.FromPort > params.ToPort {
			errors.Add("FromPort", fmt.Sprintf("%d", params.FromPort), "from port cannot be greater than to port")
		}
	}

	// Validate CIDR blocks or source security group
	if len(params.CidrBlocks) == 0 && params.SourceSG == "" {
		errors.Add("CidrBlocks", "", "either CIDR blocks or source security group must be specified")
	}

	if params.SourceSG != "" && !v.isValidSecurityGroupID(params.SourceSG) {
		errors.Add("SourceSG", params.SourceSG, "invalid source security group ID format (expected sg-xxxxxxxx)")
	}

	for i, cidr := range params.CidrBlocks {
		if err := v.validateCIDRBlockForSecurityGroup(cidr); err != nil {
			errors.Add(fmt.Sprintf("CidrBlocks[%d]", i), cidr, err.Error())
		}
	}

	if errors.HasErrors() {
		return errors
	}
	return nil
}

// ValidateTargetGroupParams validates target group creation parameters
func (v *ParameterValidator) ValidateTargetGroupParams(params aws.CreateTargetGroupParams) error {
	var errors ValidationErrors

	// Validate name
	if params.Name == "" {
		errors.Add("Name", "", "target group name is required")
	} else if len(params.Name) > 32 {
		errors.Add("Name", params.Name, "target group name must be 32 characters or less")
	} else if !v.isValidTargetGroupName(params.Name) {
		errors.Add("Name", params.Name, "invalid target group name format")
	}

	// Validate protocol
	if params.Protocol == "" {
		errors.Add("Protocol", "", "protocol is required")
	} else if !v.isValidTargetGroupProtocol(params.Protocol) {
		errors.Add("Protocol", params.Protocol, "invalid protocol (must be HTTP, HTTPS, TCP, TLS, UDP, TCP_UDP, or GENEVE)")
	}

	// Validate port
	if params.Port <= 0 || params.Port > 65535 {
		errors.Add("Port", fmt.Sprintf("%d", params.Port), "port must be between 1 and 65535")
	}

	// Validate VPC ID
	if params.VpcID == "" {
		errors.Add("VpcID", "", "VPC ID is required")
	} else if !v.isValidVPCID(params.VpcID) {
		errors.Add("VpcID", params.VpcID, "invalid VPC ID format (expected vpc-xxxxxxxx)")
	}

	// Validate target type
	if params.TargetType != "" && params.TargetType != "instance" && params.TargetType != "ip" && params.TargetType != "lambda" {
		errors.Add("TargetType", params.TargetType, "target type must be 'instance', 'ip', or 'lambda'")
	}

	// Validate health check parameters
	if params.HealthCheckEnabled && params.Protocol == "HTTP" || params.Protocol == "HTTPS" {
		if params.HealthCheckPath == "" {
			errors.Add("HealthCheckPath", "", "health check path is required for HTTP/HTTPS protocols")
		} else if !strings.HasPrefix(params.HealthCheckPath, "/") {
			errors.Add("HealthCheckPath", params.HealthCheckPath, "health check path must start with '/'")
		}
	}

	if params.HealthCheckIntervalSeconds > 0 && (params.HealthCheckIntervalSeconds < 5 || params.HealthCheckIntervalSeconds > 300) {
		errors.Add("HealthCheckIntervalSeconds", fmt.Sprintf("%d", params.HealthCheckIntervalSeconds), "health check interval must be between 5 and 300 seconds")
	}

	// Validate tags
	if err := v.validateTags(params.Tags); err != nil {
		errors.Add("Tags", "", err.Error())
	}

	if errors.HasErrors() {
		return errors
	}
	return nil
}

// ValidateListenerParams validates listener creation parameters
func (v *ParameterValidator) ValidateListenerParams(params aws.CreateListenerParams) error {
	var errors ValidationErrors

	// Validate load balancer ARN
	if params.LoadBalancerArn == "" {
		errors.Add("LoadBalancerArn", "", "load balancer ARN is required")
	} else if !v.isValidLoadBalancerArn(params.LoadBalancerArn) {
		errors.Add("LoadBalancerArn", params.LoadBalancerArn, "invalid load balancer ARN format")
	}

	// Validate protocol
	if params.Protocol == "" {
		errors.Add("Protocol", "", "protocol is required")
	} else if !v.isValidListenerProtocol(params.Protocol) {
		errors.Add("Protocol", params.Protocol, "invalid protocol (must be HTTP, HTTPS, TCP, TLS, UDP, or TCP_UDP)")
	}

	// Validate port
	if params.Port <= 0 || params.Port > 65535 {
		errors.Add("Port", fmt.Sprintf("%d", params.Port), "port must be between 1 and 65535")
	}

	// Validate default target group ARN
	if params.DefaultTargetGroupArn == "" {
		errors.Add("DefaultTargetGroupArn", "", "default target group ARN is required")
	} else if !v.isValidTargetGroupArn(params.DefaultTargetGroupArn) {
		errors.Add("DefaultTargetGroupArn", params.DefaultTargetGroupArn, "invalid target group ARN format")
	}

	// Validate certificate ARN for HTTPS
	if params.Protocol == "HTTPS" || params.Protocol == "TLS" {
		if params.CertificateArn == "" {
			errors.Add("CertificateArn", "", "certificate ARN is required for HTTPS/TLS listeners")
		} else if !v.isValidCertificateArn(params.CertificateArn) {
			errors.Add("CertificateArn", params.CertificateArn, "invalid certificate ARN format")
		}
	}

	if errors.HasErrors() {
		return errors
	}
	return nil
}

// ValidateDBInstanceParams validates RDS DB instance creation parameters
func (v *ParameterValidator) ValidateDBInstanceParams(params aws.CreateDBInstanceParams) error {
	var errors ValidationErrors

	// Validate DB instance identifier
	if params.DBInstanceIdentifier == "" {
		errors.Add("DBInstanceIdentifier", "", "DB instance identifier is required")
	} else if len(params.DBInstanceIdentifier) > 63 {
		errors.Add("DBInstanceIdentifier", params.DBInstanceIdentifier, "DB instance identifier must be 63 characters or less")
	} else if !v.isValidDBInstanceIdentifier(params.DBInstanceIdentifier) {
		errors.Add("DBInstanceIdentifier", params.DBInstanceIdentifier, "invalid DB instance identifier format")
	}

	// Validate DB instance class
	if params.DBInstanceClass == "" {
		errors.Add("DBInstanceClass", "", "DB instance class is required")
	} else if !v.isValidDBInstanceClass(params.DBInstanceClass) {
		errors.Add("DBInstanceClass", params.DBInstanceClass, "invalid DB instance class")
	}

	// Validate engine
	if params.Engine == "" {
		errors.Add("Engine", "", "database engine is required")
	} else if !v.isValidDBEngine(params.Engine) {
		errors.Add("Engine", params.Engine, "invalid database engine")
	}

	// Validate master username
	if params.MasterUsername == "" {
		errors.Add("MasterUsername", "", "master username is required")
	} else if len(params.MasterUsername) > 16 {
		errors.Add("MasterUsername", params.MasterUsername, "master username must be 16 characters or less")
	} else if !v.isValidDBUsername(params.MasterUsername) {
		errors.Add("MasterUsername", params.MasterUsername, "invalid master username format")
	}

	// Validate master password
	if params.MasterUserPassword == "" {
		errors.Add("MasterUserPassword", "", "master password is required")
	} else if len(params.MasterUserPassword) < 8 || len(params.MasterUserPassword) > 128 {
		errors.Add("MasterUserPassword", "", "master password must be between 8 and 128 characters")
	}

	// Validate allocated storage
	if params.AllocatedStorage <= 0 {
		errors.Add("AllocatedStorage", fmt.Sprintf("%d", params.AllocatedStorage), "allocated storage must be greater than 0")
	} else if params.AllocatedStorage < 20 {
		errors.Add("AllocatedStorage", fmt.Sprintf("%d", params.AllocatedStorage), "minimum allocated storage is 20 GB")
	}

	// Validate storage type
	if params.StorageType != "" && !v.isValidStorageType(params.StorageType) {
		errors.Add("StorageType", params.StorageType, "invalid storage type (must be gp2, io1, or magnetic)")
	}

	// Validate VPC security group IDs
	for i, sgID := range params.VpcSecurityGroupIDs {
		if !v.isValidSecurityGroupID(sgID) {
			errors.Add(fmt.Sprintf("VpcSecurityGroupIDs[%d]", i), sgID, "invalid security group ID format (expected sg-xxxxxxxx)")
		}
	}

	// Validate backup retention period
	if params.BackupRetentionPeriod < 0 || params.BackupRetentionPeriod > 35 {
		errors.Add("BackupRetentionPeriod", fmt.Sprintf("%d", params.BackupRetentionPeriod), "backup retention period must be between 0 and 35 days")
	}

	// Validate tags
	if err := v.validateTags(params.Tags); err != nil {
		errors.Add("Tags", "", err.Error())
	}

	if errors.HasErrors() {
		return errors
	}
	return nil
}

// Helper validation methods

// validateCIDRBlock validates a CIDR block format and range
func (v *ParameterValidator) validateCIDRBlock(cidr string) error {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("invalid CIDR format")
	}

	// Check if it's a valid VPC CIDR range
	ones, bits := ipnet.Mask.Size()
	if bits != 32 {
		return fmt.Errorf("CIDR block must be IPv4")
	}

	// VPC CIDR blocks must be between /16 and /28
	if ones < 16 || ones > 28 {
		return fmt.Errorf("CIDR block netmask must be between /16 and /28")
	}

	return nil
}

// validateTags validates AWS resource tags
func (v *ParameterValidator) validateTags(tags map[string]string) error {
	if len(tags) > 50 {
		return fmt.Errorf("maximum of 50 tags allowed")
	}

	for key, value := range tags {
		if len(key) == 0 {
			return fmt.Errorf("tag key cannot be empty")
		}
		if len(key) > 128 {
			return fmt.Errorf("tag key '%s' exceeds maximum length of 128 characters", key)
		}
		if len(value) > 256 {
			return fmt.Errorf("tag value for key '%s' exceeds maximum length of 256 characters", key)
		}
		// AWS reserved prefixes
		if strings.HasPrefix(key, "aws:") {
			return fmt.Errorf("tag key '%s' uses reserved prefix 'aws:'", key)
		}
	}

	return nil
}

// Resource ID validation patterns - Updated to support both real AWS format and configuration-driven test IDs
var (
	vpcIDRegex    = regexp.MustCompile(`^vpc-[0-9a-fA-F]{8,17}$`)     // Support both hex and longer numeric IDs
	subnetIDRegex = regexp.MustCompile(`^subnet-[0-9a-fA-F]{8,17}$`)  // Support both hex and longer numeric IDs
	sgIDRegex     = regexp.MustCompile(`^sg-[0-9a-fA-F]{8,17}$`)      // Support both hex and longer numeric IDs
	amiIDRegex    = regexp.MustCompile(`^ami-[0-9a-fA-F]{8,17}$`)     // Support both hex and longer numeric IDs
	azRegex       = regexp.MustCompile(`^[a-z]+-[a-z]+-[0-9]+[a-z]$`) // Keep original AZ format

	validInstanceTypes = map[string]bool{
		// General Purpose
		"t2.nano": true, "t2.micro": true, "t2.small": true, "t2.medium": true, "t2.large": true, "t2.xlarge": true, "t2.2xlarge": true,
		"t3.nano": true, "t3.micro": true, "t3.small": true, "t3.medium": true, "t3.large": true, "t3.xlarge": true, "t3.2xlarge": true,
		"t3a.nano": true, "t3a.micro": true, "t3a.small": true, "t3a.medium": true, "t3a.large": true, "t3a.xlarge": true, "t3a.2xlarge": true,
		"t4g.nano": true, "t4g.micro": true, "t4g.small": true, "t4g.medium": true, "t4g.large": true, "t4g.xlarge": true, "t4g.2xlarge": true,
		"m5.large": true, "m5.xlarge": true, "m5.2xlarge": true, "m5.4xlarge": true, "m5.8xlarge": true, "m5.12xlarge": true, "m5.16xlarge": true, "m5.24xlarge": true,
		"m5a.large": true, "m5a.xlarge": true, "m5a.2xlarge": true, "m5a.4xlarge": true, "m5a.8xlarge": true, "m5a.12xlarge": true, "m5a.16xlarge": true, "m5a.24xlarge": true,
		"m6i.large": true, "m6i.xlarge": true, "m6i.2xlarge": true, "m6i.4xlarge": true, "m6i.8xlarge": true, "m6i.12xlarge": true, "m6i.16xlarge": true, "m6i.24xlarge": true, "m6i.32xlarge": true,
		// Compute Optimized
		"c5.large": true, "c5.xlarge": true, "c5.2xlarge": true, "c5.4xlarge": true, "c5.9xlarge": true, "c5.12xlarge": true, "c5.18xlarge": true, "c5.24xlarge": true,
		"c5a.large": true, "c5a.xlarge": true, "c5a.2xlarge": true, "c5a.4xlarge": true, "c5a.8xlarge": true, "c5a.12xlarge": true, "c5a.16xlarge": true, "c5a.24xlarge": true,
		"c6i.large": true, "c6i.xlarge": true, "c6i.2xlarge": true, "c6i.4xlarge": true, "c6i.8xlarge": true, "c6i.12xlarge": true, "c6i.16xlarge": true, "c6i.24xlarge": true, "c6i.32xlarge": true,
		// Memory Optimized
		"r5.large": true, "r5.xlarge": true, "r5.2xlarge": true, "r5.4xlarge": true, "r5.8xlarge": true, "r5.12xlarge": true, "r5.16xlarge": true, "r5.24xlarge": true,
		"r5a.large": true, "r5a.xlarge": true, "r5a.2xlarge": true, "r5a.4xlarge": true, "r5a.8xlarge": true, "r5a.12xlarge": true, "r5a.16xlarge": true, "r5a.24xlarge": true,
		"r6i.large": true, "r6i.xlarge": true, "r6i.2xlarge": true, "r6i.4xlarge": true, "r6i.8xlarge": true, "r6i.12xlarge": true, "r6i.16xlarge": true, "r6i.24xlarge": true, "r6i.32xlarge": true,
	}
)

func (v *ParameterValidator) isValidVPCID(id string) bool {
	return vpcIDRegex.MatchString(id)
}

func (v *ParameterValidator) isValidSubnetID(id string) bool {
	return subnetIDRegex.MatchString(id)
}

func (v *ParameterValidator) isValidSecurityGroupID(id string) bool {
	return sgIDRegex.MatchString(id)
}

func (v *ParameterValidator) isValidAMIID(id string) bool {
	return amiIDRegex.MatchString(id)
}

func (v *ParameterValidator) isValidInstanceType(instanceType string) bool {
	return validInstanceTypes[instanceType]
}

func (v *ParameterValidator) isValidAvailabilityZone(az string) bool {
	// Accept template references during test mode (they will be resolved later)
	if strings.Contains(az, "{{") && strings.Contains(az, "}}") {
		return true
	}
	// Standard AWS availability zone format
	return azRegex.MatchString(az)
}

func (v *ParameterValidator) isValidLaunchTemplateName(name string) bool {
	// Launch template names must be 3-128 characters, start with letter, and contain only letters, numbers, hyphens, underscores, periods, and spaces
	if len(name) < 3 || len(name) > 128 {
		return false
	}

	// Must start with a letter
	if !regexp.MustCompile(`^[a-zA-Z]`).MatchString(name) {
		return false
	}

	// Can contain letters, numbers, hyphens, underscores, periods, and spaces
	return regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9\-_. ]*$`).MatchString(name)
}

func (v *ParameterValidator) isValidLaunchTemplateVersion(version string) bool {
	// Valid versions: $Latest, $Default, or a positive integer
	if version == "$Latest" || version == "$Default" {
		return true
	}

	// Check if it's a positive integer
	return regexp.MustCompile(`^[1-9]\d*$`).MatchString(version)
}

func (v *ParameterValidator) isValidLoadBalancerName(name string) bool {
	// Load balancer names must be 1-32 characters, start with letter or number, and contain only alphanumeric characters and hyphens
	if len(name) < 1 || len(name) > 32 {
		return false
	}

	// Must start with alphanumeric character
	if !regexp.MustCompile(`^[a-zA-Z0-9]`).MatchString(name) {
		return false
	}

	// Must end with alphanumeric character
	if !regexp.MustCompile(`[a-zA-Z0-9]$`).MatchString(name) {
		return false
	}

	// Can contain letters, numbers, and hyphens (but not consecutive hyphens)
	if regexp.MustCompile(`--`).MatchString(name) {
		return false
	}

	return regexp.MustCompile(`^[a-zA-Z0-9-]*$`).MatchString(name)
}

// Additional validation helper methods

func (v *ParameterValidator) isValidSecurityGroupName(name string) bool {
	// Security group names must be 1-255 characters, start with letter or number, and contain only alphanumeric characters, spaces, periods, hyphens, and underscores
	if len(name) < 1 || len(name) > 255 {
		return false
	}

	// Must start with alphanumeric character
	if !regexp.MustCompile(`^[a-zA-Z0-9]`).MatchString(name) {
		return false
	}

	// Can contain letters, numbers, spaces, periods, hyphens, and underscores
	return regexp.MustCompile(`^[a-zA-Z0-9 ._-]*$`).MatchString(name)
}

func (v *ParameterValidator) isValidProtocol(protocol string) bool {
	validProtocols := map[string]bool{
		"tcp":  true,
		"udp":  true,
		"icmp": true,
		"-1":   true, // All protocols
	}
	return validProtocols[protocol]
}

func (v *ParameterValidator) isValidTargetGroupName(name string) bool {
	// Target group names must be 1-32 characters, start with letter or number, and contain only alphanumeric characters and hyphens
	if len(name) < 1 || len(name) > 32 {
		return false
	}

	// Must start with alphanumeric character
	if !regexp.MustCompile(`^[a-zA-Z0-9]`).MatchString(name) {
		return false
	}

	// Must end with alphanumeric character
	if !regexp.MustCompile(`[a-zA-Z0-9]$`).MatchString(name) {
		return false
	}

	// Can contain letters, numbers, and hyphens (but not consecutive hyphens)
	if regexp.MustCompile(`--`).MatchString(name) {
		return false
	}

	return regexp.MustCompile(`^[a-zA-Z0-9-]*$`).MatchString(name)
}

func (v *ParameterValidator) isValidTargetGroupProtocol(protocol string) bool {
	validProtocols := map[string]bool{
		"HTTP":    true,
		"HTTPS":   true,
		"TCP":     true,
		"TLS":     true,
		"UDP":     true,
		"TCP_UDP": true,
		"GENEVE":  true,
	}
	return validProtocols[protocol]
}

func (v *ParameterValidator) isValidListenerProtocol(protocol string) bool {
	validProtocols := map[string]bool{
		"HTTP":    true,
		"HTTPS":   true,
		"TCP":     true,
		"TLS":     true,
		"UDP":     true,
		"TCP_UDP": true,
	}
	return validProtocols[protocol]
}

func (v *ParameterValidator) isValidLoadBalancerArn(arn string) bool {
	// Load balancer ARNs have the format: arn:aws:elasticloadbalancing:region:account-id:loadbalancer/app/load-balancer-name/load-balancer-id
	return regexp.MustCompile(`^arn:aws:elasticloadbalancing:[a-z0-9-]+:[0-9]{12}:loadbalancer/(app|net|gwy)/[a-zA-Z0-9-]+/[a-f0-9]{16}$`).MatchString(arn)
}

func (v *ParameterValidator) isValidTargetGroupArn(arn string) bool {
	// Target group ARNs have the format: arn:aws:elasticloadbalancing:region:account-id:targetgroup/target-group-name/target-group-id
	return regexp.MustCompile(`^arn:aws:elasticloadbalancing:[a-z0-9-]+:[0-9]{12}:targetgroup/[a-zA-Z0-9-]+/[a-f0-9]{16}$`).MatchString(arn)
}

func (v *ParameterValidator) isValidCertificateArn(arn string) bool {
	// Certificate ARNs have the format: arn:aws:acm:region:account-id:certificate/certificate-id
	return regexp.MustCompile(`^arn:aws:acm:[a-z0-9-]+:[0-9]{12}:certificate/[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`).MatchString(arn)
}

func (v *ParameterValidator) isValidDBInstanceIdentifier(identifier string) bool {
	// DB instance identifiers must be 1-63 characters, start with letter, and contain only lowercase letters, numbers, and hyphens
	if len(identifier) < 1 || len(identifier) > 63 {
		return false
	}

	// Must start with a letter
	if !regexp.MustCompile(`^[a-z]`).MatchString(identifier) {
		return false
	}

	// Can contain lowercase letters, numbers, and hyphens
	return regexp.MustCompile(`^[a-z][a-z0-9-]*$`).MatchString(identifier)
}

func (v *ParameterValidator) isValidDBInstanceClass(instanceClass string) bool {
	// Common RDS instance classes
	validClasses := map[string]bool{
		// General Purpose
		"db.t2.micro": true, "db.t2.small": true, "db.t2.medium": true, "db.t2.large": true, "db.t2.xlarge": true, "db.t2.2xlarge": true,
		"db.t3.micro": true, "db.t3.small": true, "db.t3.medium": true, "db.t3.large": true, "db.t3.xlarge": true, "db.t3.2xlarge": true,
		"db.t4g.micro": true, "db.t4g.small": true, "db.t4g.medium": true, "db.t4g.large": true, "db.t4g.xlarge": true, "db.t4g.2xlarge": true,
		"db.m5.large": true, "db.m5.xlarge": true, "db.m5.2xlarge": true, "db.m5.4xlarge": true, "db.m5.8xlarge": true, "db.m5.12xlarge": true, "db.m5.16xlarge": true, "db.m5.24xlarge": true,
		"db.m6i.large": true, "db.m6i.xlarge": true, "db.m6i.2xlarge": true, "db.m6i.4xlarge": true, "db.m6i.8xlarge": true, "db.m6i.12xlarge": true, "db.m6i.16xlarge": true, "db.m6i.24xlarge": true, "db.m6i.32xlarge": true,
		// Memory Optimized
		"db.r5.large": true, "db.r5.xlarge": true, "db.r5.2xlarge": true, "db.r5.4xlarge": true, "db.r5.8xlarge": true, "db.r5.12xlarge": true, "db.r5.16xlarge": true, "db.r5.24xlarge": true,
		"db.r6i.large": true, "db.r6i.xlarge": true, "db.r6i.2xlarge": true, "db.r6i.4xlarge": true, "db.r6i.8xlarge": true, "db.r6i.12xlarge": true, "db.r6i.16xlarge": true, "db.r6i.24xlarge": true, "db.r6i.32xlarge": true,
		// Compute Optimized
		"db.c5.large": true, "db.c5.xlarge": true, "db.c5.2xlarge": true, "db.c5.4xlarge": true, "db.c5.9xlarge": true, "db.c5.12xlarge": true, "db.c5.18xlarge": true, "db.c5.24xlarge": true,
	}
	return validClasses[instanceClass]
}

func (v *ParameterValidator) isValidDBEngine(engine string) bool {
	validEngines := map[string]bool{
		"mysql":         true,
		"postgres":      true,
		"mariadb":       true,
		"oracle-ee":     true,
		"oracle-se2":    true,
		"oracle-se1":    true,
		"sqlserver-ee":  true,
		"sqlserver-se":  true,
		"sqlserver-ex":  true,
		"sqlserver-web": true,
	}
	return validEngines[engine]
}

func (v *ParameterValidator) isValidDBUsername(username string) bool {
	// DB usernames must be 1-16 characters, start with letter, and contain only letters and numbers
	if len(username) < 1 || len(username) > 16 {
		return false
	}

	// Must start with a letter
	if !regexp.MustCompile(`^[a-zA-Z]`).MatchString(username) {
		return false
	}

	// Can contain letters and numbers
	return regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9]*$`).MatchString(username)
}

func (v *ParameterValidator) isValidStorageType(storageType string) bool {
	validTypes := map[string]bool{
		"gp2":      true,
		"io1":      true,
		"magnetic": true,
	}
	return validTypes[storageType]
}

// validateCIDRBlockForSecurityGroup validates a CIDR block for security group rules (less restrictive than VPC CIDRs)
func (v *ParameterValidator) validateCIDRBlockForSecurityGroup(cidr string) error {
	_, _, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("invalid CIDR format")
	}
	return nil
}
