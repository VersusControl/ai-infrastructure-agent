package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	"github.com/versus-control/ai-infrastructure-agent/pkg/types"

	"github.com/sirupsen/logrus"
)

// ========== Load Balancer Methods ==========

// CreateApplicationLoadBalancer creates an Application Load Balancer
func (c *Client) CreateApplicationLoadBalancer(ctx context.Context, params CreateLoadBalancerParams) (*types.AWSResource, error) {
	// Validate AWS requirements before making the API call
	if len(params.Subnets) < 2 {
		return nil, fmt.Errorf("at least two subnets in different Availability Zones must be specified for Application Load Balancer creation")
	}

	input := &elasticloadbalancingv2.CreateLoadBalancerInput{
		Name:           aws.String(params.Name),
		Subnets:        params.Subnets,
		SecurityGroups: params.SecurityGroups,
		Scheme:         elbv2types.LoadBalancerSchemeEnum(params.Scheme),
		Type:           elbv2types.LoadBalancerTypeEnum(params.Type),
		IpAddressType:  elbv2types.IpAddressType(params.IpAddressType),
	}

	// Add tags
	if len(params.Tags) > 0 {
		var tags []elbv2types.Tag
		for k, v := range params.Tags {
			tags = append(tags, elbv2types.Tag{
				Key:   aws.String(k),
				Value: aws.String(v),
			})
		}
		input.Tags = tags
	}

	result, err := c.elbv2.CreateLoadBalancer(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to create load balancer: %w", err)
	}

	if len(result.LoadBalancers) == 0 {
		return nil, fmt.Errorf("no load balancer created")
	}

	lb := result.LoadBalancers[0]
	c.logger.WithFields(logrus.Fields{
		"lbArn":   *lb.LoadBalancerArn,
		"lbName":  *lb.LoadBalancerName,
		"dnsName": *lb.DNSName,
	}).Info("Application Load Balancer created successfully")

	resource := &types.AWSResource{
		ID:    *lb.LoadBalancerArn,
		Type:  "application-load-balancer",
		State: string(lb.State.Code),
		Details: map[string]interface{}{
			"name":    *lb.LoadBalancerName,
			"dnsName": *lb.DNSName,
			"scheme":  string(lb.Scheme),
			"type":    string(lb.Type),
		},
		LastSeen: time.Now(),
	}

	return resource, nil
}

// CreateTargetGroup creates a target group for the load balancer
func (c *Client) CreateTargetGroup(ctx context.Context, params CreateTargetGroupParams) (*types.AWSResource, error) {
	// Validate and set defaults for critical parameters
	if params.HealthCheckPath == "" {
		params.HealthCheckPath = "/"
	}
	if params.HealthCheckProtocol == "" {
		params.HealthCheckProtocol = params.Protocol
		if params.HealthCheckProtocol == "" {
			params.HealthCheckProtocol = "HTTP"
		}
	}
	if params.Matcher == "" {
		params.Matcher = "200"
	}
	if params.TargetType == "" {
		params.TargetType = "instance"
	}

	input := &elasticloadbalancingv2.CreateTargetGroupInput{
		Name:                       aws.String(params.Name),
		Protocol:                   elbv2types.ProtocolEnum(params.Protocol),
		Port:                       aws.Int32(params.Port),
		VpcId:                      aws.String(params.VpcID),
		TargetType:                 elbv2types.TargetTypeEnum(params.TargetType),
		HealthCheckEnabled:         aws.Bool(params.HealthCheckEnabled),
		HealthCheckPath:            aws.String(params.HealthCheckPath),
		HealthCheckProtocol:        elbv2types.ProtocolEnum(params.HealthCheckProtocol),
		HealthCheckIntervalSeconds: aws.Int32(params.HealthCheckIntervalSeconds),
		HealthCheckTimeoutSeconds:  aws.Int32(params.HealthCheckTimeoutSeconds),
		HealthyThresholdCount:      aws.Int32(params.HealthyThresholdCount),
		UnhealthyThresholdCount:    aws.Int32(params.UnhealthyThresholdCount),
		Matcher: &elbv2types.Matcher{
			HttpCode: aws.String(params.Matcher),
		},
	}

	// Add tags
	if len(params.Tags) > 0 {
		var tags []elbv2types.Tag
		for k, v := range params.Tags {
			tags = append(tags, elbv2types.Tag{
				Key:   aws.String(k),
				Value: aws.String(v),
			})
		}
		input.Tags = tags
	}

	result, err := c.elbv2.CreateTargetGroup(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to create target group: %w", err)
	}

	if len(result.TargetGroups) == 0 {
		return nil, fmt.Errorf("no target group created")
	}

	tg := result.TargetGroups[0]
	c.logger.WithFields(logrus.Fields{
		"tgArn":  *tg.TargetGroupArn,
		"tgName": *tg.TargetGroupName,
	}).Info("Target group created successfully")

	resource := &types.AWSResource{
		ID:    *tg.TargetGroupArn,
		Type:  "target-group",
		State: "active",
		Details: map[string]interface{}{
			"name":     *tg.TargetGroupName,
			"protocol": string(tg.Protocol),
			"port":     *tg.Port,
			"vpcId":    *tg.VpcId,
		},
		LastSeen: time.Now(),
	}

	return resource, nil
}

// CreateListener creates a listener for the load balancer
func (c *Client) CreateListener(ctx context.Context, params CreateListenerParams) (*types.AWSResource, error) {
	input := &elasticloadbalancingv2.CreateListenerInput{
		LoadBalancerArn: aws.String(params.LoadBalancerArn),
		Protocol:        elbv2types.ProtocolEnum(params.Protocol),
		Port:            aws.Int32(params.Port),
		DefaultActions: []elbv2types.Action{
			{
				Type:           elbv2types.ActionTypeEnumForward,
				TargetGroupArn: aws.String(params.DefaultTargetGroupArn),
			},
		},
	}

	// Add SSL certificate for HTTPS
	if params.Protocol == "HTTPS" && params.CertificateArn != "" {
		input.Certificates = []elbv2types.Certificate{
			{
				CertificateArn: aws.String(params.CertificateArn),
			},
		}
	}

	result, err := c.elbv2.CreateListener(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to create listener: %w", err)
	}

	if len(result.Listeners) == 0 {
		return nil, fmt.Errorf("no listener created")
	}

	listener := result.Listeners[0]
	c.logger.WithFields(logrus.Fields{
		"listenerArn": *listener.ListenerArn,
		"protocol":    string(listener.Protocol),
		"port":        *listener.Port,
	}).Info("Listener created successfully")

	resource := &types.AWSResource{
		ID:    *listener.ListenerArn,
		Type:  "listener",
		State: "active",
		Details: map[string]interface{}{
			"protocol":        string(listener.Protocol),
			"port":            *listener.Port,
			"loadBalancerArn": params.LoadBalancerArn,
		},
		LastSeen: time.Now(),
	}

	return resource, nil
}

// RegisterTargets registers targets with a target group
func (c *Client) RegisterTargets(ctx context.Context, targetGroupArn string, targetIDs []string) error {
	c.logger.WithFields(logrus.Fields{
		"targetGroupArn": targetGroupArn,
		"targetIDs":      targetIDs,
		"targetCount":    len(targetIDs),
	}).Info("AWS RegisterTargets API call - Starting target registration")

	if len(targetIDs) == 0 {
		c.logger.Warn("No target IDs provided for registration")
		return fmt.Errorf("no target IDs provided")
	}

	var targets []elbv2types.TargetDescription
	for i, targetID := range targetIDs {
		targets = append(targets, elbv2types.TargetDescription{
			Id: aws.String(targetID),
		})
		c.logger.WithFields(logrus.Fields{
			"targetIndex": i,
			"targetId":    targetID,
		}).Debug("Prepared target for registration")
	}

	input := &elasticloadbalancingv2.RegisterTargetsInput{
		TargetGroupArn: aws.String(targetGroupArn),
		Targets:        targets,
	}

	c.logger.WithFields(logrus.Fields{
		"targetGroupArn": targetGroupArn,
		"targetCount":    len(targets),
		"awsInput":       input,
	}).Info("Calling AWS ELBv2 RegisterTargets API")

	output, err := c.elbv2.RegisterTargets(ctx, input)
	if err != nil {
		c.logger.WithError(err).WithFields(logrus.Fields{
			"targetGroupArn": targetGroupArn,
			"targetIDs":      targetIDs,
		}).Error("AWS RegisterTargets API call failed")
		return fmt.Errorf("failed to register targets: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"targetGroupArn": targetGroupArn,
		"targetCount":    len(targetIDs),
		"awsOutput":      output,
	}).Info("AWS RegisterTargets API call completed successfully")

	return nil
}

// DeregisterTargets deregisters targets from a target group
func (c *Client) DeregisterTargets(ctx context.Context, targetGroupArn string, targetIDs []string) error {
	var targets []elbv2types.TargetDescription
	for _, targetID := range targetIDs {
		targets = append(targets, elbv2types.TargetDescription{
			Id: aws.String(targetID),
		})
	}

	input := &elasticloadbalancingv2.DeregisterTargetsInput{
		TargetGroupArn: aws.String(targetGroupArn),
		Targets:        targets,
	}

	_, err := c.elbv2.DeregisterTargets(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to deregister targets: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"targetGroupArn": targetGroupArn,
		"targetCount":    len(targetIDs),
	}).Info("Targets deregistered successfully")

	return nil
}

// ========== Load Balancer and Target Group Listing Methods ==========

// DescribeLoadBalancers lists all Load Balancers in the region
func (c *Client) DescribeLoadBalancers(ctx context.Context) ([]*types.AWSResource, error) {
	result, err := c.elbv2.DescribeLoadBalancers(ctx, &elasticloadbalancingv2.DescribeLoadBalancersInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to describe Load Balancers: %w", err)
	}

	var resources []*types.AWSResource
	for _, lb := range result.LoadBalancers {
		resources = append(resources, c.convertLoadBalancer(lb))
	}

	return resources, nil
}

// GetLoadBalancer gets a specific Load Balancer by ARN
func (c *Client) GetLoadBalancer(ctx context.Context, loadBalancerArn string) (*types.AWSResource, error) {
	result, err := c.elbv2.DescribeLoadBalancers(ctx, &elasticloadbalancingv2.DescribeLoadBalancersInput{
		LoadBalancerArns: []string{loadBalancerArn},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe Load Balancer %s: %w", loadBalancerArn, err)
	}

	if len(result.LoadBalancers) == 0 {
		return nil, fmt.Errorf("LoadBalancer %s not found", loadBalancerArn)
	}

	return c.convertLoadBalancer(result.LoadBalancers[0]), nil
}

// DescribeTargetGroups lists all Target Groups in the region
func (c *Client) DescribeTargetGroups(ctx context.Context) ([]*types.AWSResource, error) {
	result, err := c.elbv2.DescribeTargetGroups(ctx, &elasticloadbalancingv2.DescribeTargetGroupsInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to describe Target Groups: %w", err)
	}

	var resources []*types.AWSResource
	for _, tg := range result.TargetGroups {
		resources = append(resources, c.convertTargetGroup(tg))
	}

	return resources, nil
}

// GetTargetGroup gets a specific Target Group by ARN
func (c *Client) GetTargetGroup(ctx context.Context, targetGroupArn string) (*types.AWSResource, error) {
	result, err := c.elbv2.DescribeTargetGroups(ctx, &elasticloadbalancingv2.DescribeTargetGroupsInput{
		TargetGroupArns: []string{targetGroupArn},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe Target Group %s: %w", targetGroupArn, err)
	}

	if len(result.TargetGroups) == 0 {
		return nil, fmt.Errorf("TargetGroup %s not found", targetGroupArn)
	}

	return c.convertTargetGroup(result.TargetGroups[0]), nil
}

// convertLoadBalancer converts a Load Balancer to our internal resource representation
func (c *Client) convertLoadBalancer(lb elbv2types.LoadBalancer) *types.AWSResource {
	details := map[string]interface{}{
		"loadBalancerName":      aws.ToString(lb.LoadBalancerName),
		"dnsName":               aws.ToString(lb.DNSName),
		"canonicalHostedZoneId": aws.ToString(lb.CanonicalHostedZoneId),
		"createdTime":           lb.CreatedTime,
		"scheme":                string(lb.Scheme),
		"vpcId":                 aws.ToString(lb.VpcId),
		"type":                  string(lb.Type),
		"availabilityZones":     lb.AvailabilityZones,
		"securityGroups":        lb.SecurityGroups,
		"ipAddressType":         string(lb.IpAddressType),
		"customerOwnedIpv4Pool": aws.ToString(lb.CustomerOwnedIpv4Pool),
	}

	return &types.AWSResource{
		ID:       aws.ToString(lb.LoadBalancerArn),
		Type:     "load-balancer",
		Region:   c.cfg.Region,
		State:    string(lb.State.Code),
		Tags:     make(map[string]string), // Tags need to be fetched separately
		Details:  details,
		LastSeen: time.Now(),
	}
}

// convertTargetGroup converts a Target Group to our internal resource representation
func (c *Client) convertTargetGroup(tg elbv2types.TargetGroup) *types.AWSResource {
	details := map[string]interface{}{
		"targetGroupName":            aws.ToString(tg.TargetGroupName),
		"protocol":                   string(tg.Protocol),
		"port":                       aws.ToInt32(tg.Port),
		"vpcId":                      aws.ToString(tg.VpcId),
		"healthCheckProtocol":        string(tg.HealthCheckProtocol),
		"healthCheckPort":            aws.ToString(tg.HealthCheckPort),
		"healthCheckEnabled":         aws.ToBool(tg.HealthCheckEnabled),
		"healthCheckIntervalSeconds": aws.ToInt32(tg.HealthCheckIntervalSeconds),
		"healthCheckTimeoutSeconds":  aws.ToInt32(tg.HealthCheckTimeoutSeconds),
		"healthyThresholdCount":      aws.ToInt32(tg.HealthyThresholdCount),
		"unhealthyThresholdCount":    aws.ToInt32(tg.UnhealthyThresholdCount),
		"healthCheckPath":            aws.ToString(tg.HealthCheckPath),
		"matcher":                    tg.Matcher,
		"loadBalancerArns":           tg.LoadBalancerArns,
		"targetType":                 string(tg.TargetType),
		"protocolVersion":            aws.ToString(tg.ProtocolVersion),
		"ipAddressType":              string(tg.IpAddressType),
	}

	return &types.AWSResource{
		ID:       aws.ToString(tg.TargetGroupArn),
		Type:     "target-group",
		Region:   c.cfg.Region,
		State:    "active",                // Target Groups don't have explicit state
		Tags:     make(map[string]string), // Tags need to be fetched separately
		Details:  details,
		LastSeen: time.Now(),
	}
}
