package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
)

type Client struct {
	cfg         aws.Config
	ec2         *ec2.Client
	eks         *eks.Client
	iam         *iam.Client
	sts         *sts.Client
	autoscaling *autoscaling.Client
	elbv2       *elasticloadbalancingv2.Client
	rds         *rds.Client
	logger      *logging.Logger
}

func NewClient(region string, logger *logging.Logger) (*Client, error) {
	cfg, err := config.LoadDefaultConfig(
		context.Background(),
		config.WithRegion(region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &Client{
		cfg:         cfg,
		ec2:         ec2.NewFromConfig(cfg),
		eks:         eks.NewFromConfig(cfg),
		iam:         iam.NewFromConfig(cfg),
		sts:         sts.NewFromConfig(cfg),
		autoscaling: autoscaling.NewFromConfig(cfg),
		elbv2:       elasticloadbalancingv2.NewFromConfig(cfg),
		rds:         rds.NewFromConfig(cfg),
		logger:      logger,
	}, nil
}

// GetAccountID returns the AWS Account ID of the caller
func (c *Client) GetAccountID(ctx context.Context) (string, error) {
	input := &sts.GetCallerIdentityInput{}
	result, err := c.sts.GetCallerIdentity(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to get caller identity: %w", err)
	}
	return aws.ToString(result.Account), nil
}

// HealthCheck verifies AWS connectivity
func (c *Client) HealthCheck(ctx context.Context) error {
	_, err := c.ec2.DescribeRegions(ctx, &ec2.DescribeRegionsInput{})
	if err != nil {
		return fmt.Errorf("AWS health check failed: %w", err)
	}
	return nil
}

// GetRegion returns the configured AWS region
func (c *Client) GetRegion() string {
	return c.cfg.Region
}
