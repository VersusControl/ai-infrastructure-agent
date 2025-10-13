package mcp

import (
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/adapters"
	"github.com/versus-control/ai-infrastructure-agent/pkg/aws"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// CreateResourceDefinitions creates all the resource definitions for the system
func CreateResourceDefinitions(awsClient *aws.Client, logger *logging.Logger) []ResourceDefinition {
	// Create adapters
	ec2Adapter := adapters.NewEC2Adapter(awsClient, logger)
	vpcAdapter := adapters.NewVPCAdapter(awsClient, logger)
	rdsAdapter := adapters.NewRDSAdapter(awsClient, logger)
	asgAdapter := adapters.NewASGAdapter(awsClient, logger)
	albAdapter := adapters.NewALBAdapter(awsClient, logger)

	return []ResourceDefinition{
		// EC2 Resources
		{
			BaseURI:           "aws://ec2/instances",
			Name:              "EC2 Instances",
			Description:       "List all EC2 instances in the region",
			DetailTemplate:    "aws://ec2/instances/{instanceId}",
			DetailName:        "EC2 Instance Details",
			DetailDescription: "Detailed information about a specific EC2 instance",
			Adapter:           ec2Adapter,
			ListFormatter:     formatInstancesForAI,
			DetailFormatter:   formatInstanceForAI,
		},

		// VPC Resources
		{
			BaseURI:           "aws://vpc/vpcs",
			Name:              "VPCs",
			Description:       "List all VPCs in the region",
			DetailTemplate:    "aws://vpc/vpcs/{vpcId}",
			DetailName:        "VPC Details",
			DetailDescription: "Detailed information about a specific VPC",
			Adapter:           vpcAdapter,
			ListFormatter:     formatVPCsForAI,
			DetailFormatter:   formatVPCForAI,
		},

		{
			BaseURI:           "aws://vpc/subnets",
			Name:              "Subnets",
			Description:       "List all subnets in the region",
			DetailTemplate:    "aws://vpc/subnets/{subnetId}",
			DetailName:        "Subnet Details",
			DetailDescription: "Detailed information about a specific subnet",
			Adapter:           vpcAdapter,
			ListFormatter:     formatSubnetsForAI,
			DetailFormatter:   formatSubnetForAI,
		},

		// Auto Scaling Groups
		{
			BaseURI:           "aws://autoscaling/groups",
			Name:              "Auto Scaling Groups",
			Description:       "List all Auto Scaling Groups in the region",
			DetailTemplate:    "aws://autoscaling/groups/{groupName}",
			DetailName:        "Auto Scaling Group Details",
			DetailDescription: "Detailed information about a specific Auto Scaling Group",
			Adapter:           asgAdapter,
			ListFormatter:     formatASGsForAI,
			DetailFormatter:   formatASGForAI,
		},

		// Application Load Balancers
		{
			BaseURI:           "aws://elbv2/loadbalancers",
			Name:              "Load Balancers",
			Description:       "List all Application Load Balancers in the region",
			DetailTemplate:    "aws://elbv2/loadbalancers/{loadBalancerArn}",
			DetailName:        "Load Balancer Details",
			DetailDescription: "Detailed information about a specific Load Balancer",
			Adapter:           albAdapter,
			ListFormatter:     formatLoadBalancersForAI,
			DetailFormatter:   formatLoadBalancerForAI,
		},

		{
			BaseURI:           "aws://elbv2/targetgroups",
			Name:              "Target Groups",
			Description:       "List all Target Groups in the region",
			DetailTemplate:    "aws://elbv2/targetgroups/{targetGroupArn}",
			DetailName:        "Target Group Details",
			DetailDescription: "Detailed information about a specific Target Group",
			Adapter:           albAdapter,
			ListFormatter:     formatTargetGroupsForAI,
			DetailFormatter:   formatTargetGroupForAI,
		},

		// RDS Resources
		{
			BaseURI:           "aws://rds/instances",
			Name:              "RDS Instances",
			Description:       "List all RDS database instances in the region",
			DetailTemplate:    "aws://rds/instances/{dbInstanceIdentifier}",
			DetailName:        "RDS Instance Details",
			DetailDescription: "Detailed information about a specific RDS instance",
			Adapter:           rdsAdapter,
			ListFormatter:     formatRDSInstancesForAI,
			DetailFormatter:   formatRDSInstanceForAI,
		},

		{
			BaseURI:           "aws://rds/snapshots",
			Name:              "RDS Snapshots",
			Description:       "List all RDS snapshots in the region",
			DetailTemplate:    "aws://rds/snapshots/{snapshotId}",
			DetailName:        "RDS Snapshot Details",
			DetailDescription: "Detailed information about a specific RDS snapshot",
			Adapter:           rdsAdapter,
			ListFormatter:     formatRDSSnapshotsForAI,
			DetailFormatter:   formatRDSSnapshotForAI,
		},

		// EC2 Additional Resources
		{
			BaseURI:           "aws://ec2/launchtemplates",
			Name:              "Launch Templates",
			Description:       "List all Launch Templates in the region",
			DetailTemplate:    "aws://ec2/launchtemplates/{templateId}",
			DetailName:        "Launch Template Details",
			DetailDescription: "Detailed information about a specific Launch Template",
			Adapter:           ec2Adapter,
			ListFormatter:     formatLaunchTemplatesForAI,
			DetailFormatter:   formatLaunchTemplateForAI,
		},

		{
			BaseURI:           "aws://ec2/images",
			Name:              "AMIs",
			Description:       "List all AMIs (Amazon Machine Images) in the region",
			DetailTemplate:    "aws://ec2/images/{imageId}",
			DetailName:        "AMI Details",
			DetailDescription: "Detailed information about a specific AMI",
			Adapter:           ec2Adapter,
			ListFormatter:     formatAMIsForAI,
			DetailFormatter:   formatAMIForAI,
		},
	}
}

// Custom formatters from the original implementation

// formatInstancesForAI formats instance data optimally for AI processing
func formatInstancesForAI(instances []*types.AWSResource) map[string]interface{} {
	summary := map[string]interface{}{
		"total_instances":  len(instances),
		"instances":        make([]map[string]interface{}, 0, len(instances)),
		"summary_by_state": make(map[string]int),
		"summary_by_type":  make(map[string]int),
	}

	stateCount := make(map[string]int)
	typeCount := make(map[string]int)

	for _, instance := range instances {
		formatted := map[string]interface{}{
			"id":     instance.ID,
			"state":  instance.State,
			"type":   instance.Details["instanceType"],
			"region": instance.Region,
		}

		// Add name if available from tags
		if name, exists := instance.Tags["Name"]; exists {
			formatted["name"] = name
		}

		// Add IP addresses if available
		if publicIP := instance.Details["publicIpAddress"]; publicIP != nil {
			formatted["public_ip"] = publicIP
		}

		if privateIP := instance.Details["privateIpAddress"]; privateIP != nil {
			formatted["private_ip"] = privateIP
		}

		summary["instances"] = append(summary["instances"].([]map[string]interface{}), formatted)

		// Update counters
		stateCount[instance.State]++
		if instanceType, ok := instance.Details["instanceType"].(string); ok {
			typeCount[instanceType]++
		}
	}

	summary["summary_by_state"] = stateCount
	summary["summary_by_type"] = typeCount

	return summary
}

// formatVPCsForAI formats VPC data optimally for AI processing
func formatVPCsForAI(vpcs []*types.AWSResource) map[string]interface{} {
	summary := map[string]interface{}{
		"total_vpcs":       len(vpcs),
		"vpcs":             make([]map[string]interface{}, 0, len(vpcs)),
		"summary_by_state": make(map[string]int),
	}

	stateCount := make(map[string]int)

	for _, vpc := range vpcs {
		formatted := map[string]interface{}{
			"id":     vpc.ID,
			"state":  vpc.State,
			"region": vpc.Region,
		}

		// Add name if available from tags
		if name, exists := vpc.Tags["Name"]; exists {
			formatted["name"] = name
		}

		// Add CIDR block if available
		if cidrBlock := vpc.Details["cidrBlock"]; cidrBlock != nil {
			formatted["cidr_block"] = cidrBlock
		}

		summary["vpcs"] = append(summary["vpcs"].([]map[string]interface{}), formatted)

		// Update counters
		stateCount[vpc.State]++
	}

	summary["summary_by_state"] = stateCount

	return summary
}

// formatInstanceForAI formats a single instance with comprehensive details
func formatInstanceForAI(instance types.AWSResource) map[string]interface{} {
	formatted := map[string]interface{}{
		"id":        instance.ID,
		"type":      instance.Type,
		"state":     instance.State,
		"region":    instance.Region,
		"tags":      instance.Tags,
		"details":   instance.Details,
		"last_seen": instance.LastSeen.Format("2006-01-02T15:04:05Z07:00"),
	}

	// Add computed fields that AI systems find useful
	if name, exists := instance.Tags["Name"]; exists {
		formatted["name"] = name
	} else {
		formatted["name"] = instance.ID
	}

	return map[string]interface{}{
		"instance": formatted,
	}
}

// formatVPCForAI formats a single VPC with comprehensive details
func formatVPCForAI(vpc types.AWSResource) map[string]interface{} {
	formatted := map[string]interface{}{
		"id":        vpc.ID,
		"type":      vpc.Type,
		"state":     vpc.State,
		"region":    vpc.Region,
		"tags":      vpc.Tags,
		"details":   vpc.Details,
		"last_seen": vpc.LastSeen.Format("2006-01-02T15:04:05Z07:00"),
	}

	// Add computed fields that AI systems find useful
	if name, exists := vpc.Tags["Name"]; exists {
		formatted["name"] = name
	} else {
		formatted["name"] = vpc.ID
	}

	return map[string]interface{}{
		"vpc": formatted,
	}
}

// formatSubnetsForAI formats subnet data optimally for AI processing
func formatSubnetsForAI(subnets []*types.AWSResource) map[string]interface{} {
	summary := map[string]interface{}{
		"total_subnets":    len(subnets),
		"subnets":          make([]map[string]interface{}, 0, len(subnets)),
		"summary_by_state": make(map[string]int),
		"summary_by_az":    make(map[string]int),
	}

	stateCount := make(map[string]int)
	azCount := make(map[string]int)

	for _, subnet := range subnets {
		formatted := map[string]interface{}{
			"id":     subnet.ID,
			"state":  subnet.State,
			"region": subnet.Region,
		}

		if name, exists := subnet.Tags["Name"]; exists {
			formatted["name"] = name
		}
		if cidrBlock := subnet.Details["cidrBlock"]; cidrBlock != nil {
			formatted["cidr_block"] = cidrBlock
		}
		if az := subnet.Details["availabilityZone"]; az != nil {
			formatted["availability_zone"] = az
			if azStr, ok := az.(string); ok {
				azCount[azStr]++
			}
		}

		summary["subnets"] = append(summary["subnets"].([]map[string]interface{}), formatted)
		stateCount[subnet.State]++
	}

	summary["summary_by_state"] = stateCount
	summary["summary_by_az"] = azCount
	return summary
}

// formatSubnetForAI formats a single subnet with comprehensive details
func formatSubnetForAI(subnet types.AWSResource) map[string]interface{} {
	formatted := map[string]interface{}{
		"id":        subnet.ID,
		"type":      subnet.Type,
		"state":     subnet.State,
		"region":    subnet.Region,
		"tags":      subnet.Tags,
		"details":   subnet.Details,
		"last_seen": subnet.LastSeen.Format("2006-01-02T15:04:05Z07:00"),
	}

	if name, exists := subnet.Tags["Name"]; exists {
		formatted["name"] = name
	} else {
		formatted["name"] = subnet.ID
	}

	return map[string]interface{}{
		"subnet": formatted,
	}
}

// formatASGsForAI formats Auto Scaling Groups data optimally for AI processing
func formatASGsForAI(asgs []*types.AWSResource) map[string]interface{} {
	summary := map[string]interface{}{
		"total_asgs":       len(asgs),
		"asgs":             make([]map[string]interface{}, 0, len(asgs)),
		"summary_by_state": make(map[string]int),
	}

	stateCount := make(map[string]int)

	for _, asg := range asgs {
		formatted := map[string]interface{}{
			"id":     asg.ID,
			"state":  asg.State,
			"region": asg.Region,
		}

		if name, exists := asg.Tags["Name"]; exists {
			formatted["name"] = name
		}
		if desired := asg.Details["desiredCapacity"]; desired != nil {
			formatted["desired_capacity"] = desired
		}
		if min := asg.Details["minSize"]; min != nil {
			formatted["min_size"] = min
		}
		if max := asg.Details["maxSize"]; max != nil {
			formatted["max_size"] = max
		}

		summary["asgs"] = append(summary["asgs"].([]map[string]interface{}), formatted)
		stateCount[asg.State]++
	}

	summary["summary_by_state"] = stateCount
	return summary
}

// formatASGForAI formats a single Auto Scaling Group with comprehensive details
func formatASGForAI(asg types.AWSResource) map[string]interface{} {
	formatted := map[string]interface{}{
		"id":        asg.ID,
		"type":      asg.Type,
		"state":     asg.State,
		"region":    asg.Region,
		"tags":      asg.Tags,
		"details":   asg.Details,
		"last_seen": asg.LastSeen.Format("2006-01-02T15:04:05Z07:00"),
	}

	if name, exists := asg.Tags["Name"]; exists {
		formatted["name"] = name
	} else {
		formatted["name"] = asg.ID
	}

	return map[string]interface{}{
		"autoscaling_group": formatted,
	}
}

// formatLoadBalancersForAI formats Load Balancer data optimally for AI processing
func formatLoadBalancersForAI(albs []*types.AWSResource) map[string]interface{} {
	summary := map[string]interface{}{
		"total_load_balancers": len(albs),
		"load_balancers":       make([]map[string]interface{}, 0, len(albs)),
		"summary_by_state":     make(map[string]int),
		"summary_by_scheme":    make(map[string]int),
	}

	stateCount := make(map[string]int)
	schemeCount := make(map[string]int)

	for _, alb := range albs {
		formatted := map[string]interface{}{
			"id":     alb.ID,
			"state":  alb.State,
			"region": alb.Region,
		}

		if name, exists := alb.Tags["Name"]; exists {
			formatted["name"] = name
		}
		if scheme := alb.Details["scheme"]; scheme != nil {
			formatted["scheme"] = scheme
			if schemeStr, ok := scheme.(string); ok {
				schemeCount[schemeStr]++
			}
		}
		if dnsName := alb.Details["dnsName"]; dnsName != nil {
			formatted["dns_name"] = dnsName
		}

		summary["load_balancers"] = append(summary["load_balancers"].([]map[string]interface{}), formatted)
		stateCount[alb.State]++
	}

	summary["summary_by_state"] = stateCount
	summary["summary_by_scheme"] = schemeCount
	return summary
}

// formatLoadBalancerForAI formats a single Load Balancer with comprehensive details
func formatLoadBalancerForAI(alb types.AWSResource) map[string]interface{} {
	formatted := map[string]interface{}{
		"id":        alb.ID,
		"type":      alb.Type,
		"state":     alb.State,
		"region":    alb.Region,
		"tags":      alb.Tags,
		"details":   alb.Details,
		"last_seen": alb.LastSeen.Format("2006-01-02T15:04:05Z07:00"),
	}

	if name, exists := alb.Tags["Name"]; exists {
		formatted["name"] = name
	} else {
		formatted["name"] = alb.ID
	}

	return map[string]interface{}{
		"load_balancer": formatted,
	}
}

// formatTargetGroupsForAI formats Target Group data optimally for AI processing
func formatTargetGroupsForAI(tgs []*types.AWSResource) map[string]interface{} {
	summary := map[string]interface{}{
		"total_target_groups": len(tgs),
		"target_groups":       make([]map[string]interface{}, 0, len(tgs)),
		"summary_by_protocol": make(map[string]int),
	}

	protocolCount := make(map[string]int)

	for _, tg := range tgs {
		formatted := map[string]interface{}{
			"id":     tg.ID,
			"state":  tg.State,
			"region": tg.Region,
		}

		if name, exists := tg.Tags["Name"]; exists {
			formatted["name"] = name
		}
		if protocol := tg.Details["protocol"]; protocol != nil {
			formatted["protocol"] = protocol
			if protocolStr, ok := protocol.(string); ok {
				protocolCount[protocolStr]++
			}
		}
		if port := tg.Details["port"]; port != nil {
			formatted["port"] = port
		}

		summary["target_groups"] = append(summary["target_groups"].([]map[string]interface{}), formatted)
	}

	summary["summary_by_protocol"] = protocolCount
	return summary
}

// formatTargetGroupForAI formats a single Target Group with comprehensive details
func formatTargetGroupForAI(tg types.AWSResource) map[string]interface{} {
	formatted := map[string]interface{}{
		"id":        tg.ID,
		"type":      tg.Type,
		"state":     tg.State,
		"region":    tg.Region,
		"tags":      tg.Tags,
		"details":   tg.Details,
		"last_seen": tg.LastSeen.Format("2006-01-02T15:04:05Z07:00"),
	}

	if name, exists := tg.Tags["Name"]; exists {
		formatted["name"] = name
	} else {
		formatted["name"] = tg.ID
	}

	return map[string]interface{}{
		"target_group": formatted,
	}
}

// formatRDSInstancesForAI formats RDS instance data optimally for AI processing
func formatRDSInstancesForAI(instances []*types.AWSResource) map[string]interface{} {
	summary := map[string]interface{}{
		"total_instances":   len(instances),
		"instances":         make([]map[string]interface{}, 0, len(instances)),
		"summary_by_state":  make(map[string]int),
		"summary_by_engine": make(map[string]int),
	}

	stateCount := make(map[string]int)
	engineCount := make(map[string]int)

	for _, instance := range instances {
		formatted := map[string]interface{}{
			"id":     instance.ID,
			"state":  instance.State,
			"region": instance.Region,
		}

		if name, exists := instance.Tags["Name"]; exists {
			formatted["name"] = name
		}
		if engine := instance.Details["engine"]; engine != nil {
			formatted["engine"] = engine
			if engineStr, ok := engine.(string); ok {
				engineCount[engineStr]++
			}
		}
		if dbClass := instance.Details["dbInstanceClass"]; dbClass != nil {
			formatted["instance_class"] = dbClass
		}

		summary["instances"] = append(summary["instances"].([]map[string]interface{}), formatted)
		stateCount[instance.State]++
	}

	summary["summary_by_state"] = stateCount
	summary["summary_by_engine"] = engineCount
	return summary
}

// formatRDSInstanceForAI formats a single RDS instance with comprehensive details
func formatRDSInstanceForAI(instance types.AWSResource) map[string]interface{} {
	formatted := map[string]interface{}{
		"id":        instance.ID,
		"type":      instance.Type,
		"state":     instance.State,
		"region":    instance.Region,
		"tags":      instance.Tags,
		"details":   instance.Details,
		"last_seen": instance.LastSeen.Format("2006-01-02T15:04:05Z07:00"),
	}

	if name, exists := instance.Tags["Name"]; exists {
		formatted["name"] = name
	} else {
		formatted["name"] = instance.ID
	}

	return map[string]interface{}{
		"rds_instance": formatted,
	}
}

// formatRDSSnapshotsForAI formats RDS snapshot data optimally for AI processing
func formatRDSSnapshotsForAI(snapshots []*types.AWSResource) map[string]interface{} {
	summary := map[string]interface{}{
		"total_snapshots":  len(snapshots),
		"snapshots":        make([]map[string]interface{}, 0, len(snapshots)),
		"summary_by_state": make(map[string]int),
		"summary_by_type":  make(map[string]int),
	}

	stateCount := make(map[string]int)
	typeCount := make(map[string]int)

	for _, snapshot := range snapshots {
		formatted := map[string]interface{}{
			"id":     snapshot.ID,
			"state":  snapshot.State,
			"region": snapshot.Region,
		}

		if name, exists := snapshot.Tags["Name"]; exists {
			formatted["name"] = name
		}
		if snapshotType := snapshot.Details["snapshotType"]; snapshotType != nil {
			formatted["snapshot_type"] = snapshotType
			if typeStr, ok := snapshotType.(string); ok {
				typeCount[typeStr]++
			}
		}

		summary["snapshots"] = append(summary["snapshots"].([]map[string]interface{}), formatted)
		stateCount[snapshot.State]++
	}

	summary["summary_by_state"] = stateCount
	summary["summary_by_type"] = typeCount
	return summary
}

// formatRDSSnapshotForAI formats a single RDS snapshot with comprehensive details
func formatRDSSnapshotForAI(snapshot types.AWSResource) map[string]interface{} {
	formatted := map[string]interface{}{
		"id":        snapshot.ID,
		"type":      snapshot.Type,
		"state":     snapshot.State,
		"region":    snapshot.Region,
		"tags":      snapshot.Tags,
		"details":   snapshot.Details,
		"last_seen": snapshot.LastSeen.Format("2006-01-02T15:04:05Z07:00"),
	}

	if name, exists := snapshot.Tags["Name"]; exists {
		formatted["name"] = name
	} else {
		formatted["name"] = snapshot.ID
	}

	return map[string]interface{}{
		"rds_snapshot": formatted,
	}
}

// formatLaunchTemplatesForAI formats Launch Template data optimally for AI processing
func formatLaunchTemplatesForAI(templates []*types.AWSResource) map[string]interface{} {
	summary := map[string]interface{}{
		"total_templates":  len(templates),
		"templates":        make([]map[string]interface{}, 0, len(templates)),
		"summary_by_state": make(map[string]int),
	}

	stateCount := make(map[string]int)

	for _, template := range templates {
		formatted := map[string]interface{}{
			"id":     template.ID,
			"state":  template.State,
			"region": template.Region,
		}

		if name, exists := template.Tags["Name"]; exists {
			formatted["name"] = name
		}
		if version := template.Details["latestVersionNumber"]; version != nil {
			formatted["latest_version"] = version
		}

		summary["templates"] = append(summary["templates"].([]map[string]interface{}), formatted)
		stateCount[template.State]++
	}

	summary["summary_by_state"] = stateCount
	return summary
}

// formatLaunchTemplateForAI formats a single Launch Template with comprehensive details
func formatLaunchTemplateForAI(template types.AWSResource) map[string]interface{} {
	formatted := map[string]interface{}{
		"id":        template.ID,
		"type":      template.Type,
		"state":     template.State,
		"region":    template.Region,
		"tags":      template.Tags,
		"details":   template.Details,
		"last_seen": template.LastSeen.Format("2006-01-02T15:04:05Z07:00"),
	}

	if name, exists := template.Tags["Name"]; exists {
		formatted["name"] = name
	} else {
		formatted["name"] = template.ID
	}

	return map[string]interface{}{
		"launch_template": formatted,
	}
}

// formatAMIsForAI formats AMI data optimally for AI processing
func formatAMIsForAI(amis []*types.AWSResource) map[string]interface{} {
	summary := map[string]interface{}{
		"total_amis":       len(amis),
		"amis":             make([]map[string]interface{}, 0, len(amis)),
		"summary_by_state": make(map[string]int),
		"summary_by_arch":  make(map[string]int),
	}

	stateCount := make(map[string]int)
	archCount := make(map[string]int)

	for _, ami := range amis {
		formatted := map[string]interface{}{
			"id":     ami.ID,
			"state":  ami.State,
			"region": ami.Region,
		}

		if name, exists := ami.Tags["Name"]; exists {
			formatted["name"] = name
		}
		if arch := ami.Details["architecture"]; arch != nil {
			formatted["architecture"] = arch
			if archStr, ok := arch.(string); ok {
				archCount[archStr]++
			}
		}

		summary["amis"] = append(summary["amis"].([]map[string]interface{}), formatted)
		stateCount[ami.State]++
	}

	summary["summary_by_state"] = stateCount
	summary["summary_by_arch"] = archCount
	return summary
}

// formatAMIForAI formats a single AMI with comprehensive details
func formatAMIForAI(ami types.AWSResource) map[string]interface{} {
	formatted := map[string]interface{}{
		"id":        ami.ID,
		"type":      ami.Type,
		"state":     ami.State,
		"region":    ami.Region,
		"tags":      ami.Tags,
		"details":   ami.Details,
		"last_seen": ami.LastSeen.Format("2006-01-02T15:04:05Z07:00"),
	}

	if name, exists := ami.Tags["Name"]; exists {
		formatted["name"] = name
	} else {
		formatted["name"] = ami.ID
	}

	return map[string]interface{}{
		"ami": formatted,
	}
}
