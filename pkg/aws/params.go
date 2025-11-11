package aws

// EC2 Instance Parameters
type CreateInstanceParams struct {
	ImageID         string
	InstanceType    string
	KeyName         string
	SecurityGroupID string
	SubnetID        string
	Name            string
}

// Key Pair Parameters
type CreateKeyPairParams struct {
	KeyName   string
	KeyType   string // "rsa" or "ed25519"
	TagSpecs  map[string]string
	KeyFormat string // "pem" or "ppk"
}

type ImportKeyPairParams struct {
	KeyName           string
	PublicKeyMaterial []byte
	TagSpecs          map[string]string
}

// VPC Configuration Parameters
type CreateVPCParams struct {
	Name               string
	CidrBlock          string
	EnableDnsHostnames bool
	EnableDnsSupport   bool
	Tags               map[string]string
}

// Subnet Information
type SubnetInfo struct {
	SubnetID string
	VpcID    string
}

type CreateSubnetParams struct {
	VpcID               string
	CidrBlock           string
	AvailabilityZone    string
	MapPublicIpOnLaunch bool
	Name                string
	Tags                map[string]string
}

type CreateInternetGatewayParams struct {
	Name string
	Tags map[string]string
}

type CreateNATGatewayParams struct {
	SubnetID string // Public subnet where NAT Gateway will be created
	Name     string
	Tags     map[string]string
}

// Auto Scaling Group Parameters
type CreateAutoScalingGroupParams struct {
	AutoScalingGroupName   string
	LaunchTemplateName     string
	LaunchTemplateVersion  string // Launch template version ($Latest, $Default, or version number)
	MinSize                int32
	MaxSize                int32
	DesiredCapacity        int32
	VPCZoneIdentifiers     []string // Subnet IDs
	TargetGroupARNs        []string // Load balancer target groups
	HealthCheckType        string   // "EC2" or "ELB"
	HealthCheckGracePeriod int32
	Tags                   map[string]string
}

type CreateLaunchTemplateParams struct {
	LaunchTemplateName string
	VersionDescription string // Description for this template version
	ImageID            string
	InstanceType       string
	KeyName            string
	SecurityGroupIDs   []string
	UserData           string // Base64 encoded user data
	IamInstanceProfile string
	Tags               map[string]string
	NetworkInterfaces  []map[string]interface{} // Network interface configurations
	TagSpecifications  []map[string]interface{} // Tag specifications for resources
}

// Application Load Balancer Parameters
type CreateLoadBalancerParams struct {
	Name           string   // Name of the load balancer
	Scheme         string   // "internet-facing" or "internal"
	Type           string   // "application", "network", or "gateway"
	IpAddressType  string   // "ipv4" or "dualstack"
	Subnets        []string // Subnet IDs
	SecurityGroups []string // Security Group IDs (for ALB only)
	Tags           map[string]string
}

type CreateTargetGroupParams struct {
	Name                       string // Name of the target group
	Protocol                   string // "HTTP", "HTTPS", "TCP", etc.
	Port                       int32
	VpcID                      string
	TargetType                 string // "instance", "ip", or "lambda"
	HealthCheckEnabled         bool
	HealthCheckPath            string
	HealthCheckProtocol        string
	HealthCheckIntervalSeconds int32
	HealthCheckTimeoutSeconds  int32
	HealthyThresholdCount      int32
	UnhealthyThresholdCount    int32
	Matcher                    string // HTTP codes (e.g., "200")
	Tags                       map[string]string
}

type CreateListenerParams struct {
	LoadBalancerArn       string // ARN of the load balancer
	Protocol              string // "HTTP", "HTTPS", "TCP", etc.
	Port                  int32
	DefaultTargetGroupArn string // ARN of the default target group
	CertificateArn        string // For HTTPS listeners
}

// RDS Parameters
type CreateDBSubnetGroupParams struct {
	DBSubnetGroupName        string
	DBSubnetGroupDescription string
	SubnetIDs                []string
	Tags                     map[string]string
}

type CreateDBInstanceParams struct {
	DBInstanceIdentifier       string
	DBInstanceClass            string
	Engine                     string
	EngineVersion              string
	MasterUsername             string
	MasterUserPassword         string
	AllocatedStorage           int32
	StorageType                string
	StorageEncrypted           bool
	VpcSecurityGroupIDs        []string
	DBSubnetGroupName          string
	BackupRetentionPeriod      int32
	PreferredBackupWindow      string
	PreferredMaintenanceWindow string
	MultiAZ                    bool
	PubliclyAccessible         bool
	PerformanceInsightsEnabled bool
	Tags                       map[string]string
}

type CreateDBSnapshotParams struct {
	DBInstanceIdentifier string
	DBSnapshotIdentifier string
	Tags                 map[string]string
}
