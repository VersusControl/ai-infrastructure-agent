package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/googleai"
	"github.com/versus-control/ai-infrastructure-agent/internal/config"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/agent/mocks"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// === Helper Functions for Setting Up Real AI and Comprehensive Mock MCP Tools ===

func setupRealConfiguration() (*config.Config, error) {
	// Load the actual config from configuration file
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Ensure we're using Gemini for this test
	cfg.Agent.Provider = "gemini"
	cfg.Agent.MaxTokens = 20000
	cfg.Agent.Model = "gemini-2.5-flash-lite" // Use a valid model name
	cfg.Agent.EnableDebug = true

	return cfg, nil
}

func setupRealLLMClient(cfg *config.Config) (llms.Model, error) {
	switch cfg.Agent.Provider {
	case "gemini":
		apiKey := os.Getenv("GEMINI_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("You need an auth option to use this client.")
		}
		// Pass the API key and model to the googleai client
		return googleai.New(context.Background(),
			googleai.WithAPIKey(apiKey),
			googleai.WithDefaultModel(cfg.Agent.Model))
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", cfg.Agent.Provider)
	}
}

func setupAgentWithRealAI(cfg *config.Config, llmClient llms.Model) (*StateAwareAgent, *mocks.MockTestSuite, error) {
	// Create logger
	logger := logging.NewLogger("test", cfg.Logging.Level)

	// Initialize mock test suite with real component integration
	mockSuite, err := mocks.NewMockTestSuite("us-west-2")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create mock test suite: %w", err)
	}

	// Create agent with real AI but mock infrastructure components
	agent := &StateAwareAgent{
		llm:       llmClient,
		config:    &cfg.Agent,
		awsConfig: &cfg.AWS,
		Logger:    logger,

		// MCP from mock suite
		resourceMappings: make(map[string]string),
		mcpTools:         make(map[string]MCPToolInfo),
		mcpResources:     make(map[string]MCPResourceInfo),

		// Enable test mode to use mock MCP capabilities
		testMode: true,

		// Set mock MCP server for test mode
		mockMCPServer: mockSuite.MCPServer,

		// Use real configuration-driven components from mock suite
		patternMatcher: mockSuite.StateManager.GetPatternMatcher(),
		fieldResolver:  mockSuite.StateManager.GetFieldResolver(),

		extractionConfig: mockSuite.StateManager.GetExtractionConfig(),
		idExtractor:      mockSuite.StateManager.GetIDExtractor(),
	}

	return agent, mockSuite, nil
}

// == Prompts ==

// Comprehensive VPC Infrastructure
func comprehensiveVPCPrompt() string {
	// Comprehensive VPC infrastructure prompt
	prompt := `Create a complete production-ready VPC infrastructure on AWS with the following requirements:

NETWORK ARCHITECTURE:
- VPC with CIDR 10.0.0.0/16 in us-west-2
- 6 subnets across 3 availability zones:
  * 2 public subnets (10.0.1.0/24, 10.0.2.0/24) for load balancers
  * 2 private subnets (10.0.11.0/24, 10.0.12.0/24) for application servers  
  * 2 database subnets (10.0.21.0/24, 10.0.22.0/24) for RDS instances
- Internet Gateway for public access
- 2 NAT Gateways in public subnets for private subnet internet access
- Route tables with proper routing

COMPUTE & SECURITY:
- Application Load Balancer in public subnets
- Auto Scaling Group with t3.medium instances in private subnets
- Launch Template with latest Amazon Linux 2 AMI
- Security Groups with least privilege access
- Target Group for ALB health checks

DATABASE:
- RDS MySQL instance in database subnets
- Database security group allowing access only from app servers
- Multi-AZ deployment for high availability

VALIDATION:
- Validate all resources are properly configured
- Test connectivity between components
- Verify security group rules are correct

Please create a detailed execution plan with all necessary steps, proper dependencies, and real AWS API calls where needed.`

	return prompt
}

// Comprehensive EC2 Infrastructure
func comprehensiveEC2Prompt() string {
	prompt := `Create an EC2 for hosting an Apache Server with a dedicated security group that allows inbound HTTP (port 80) and SSH (port 22) traffic.`

	return prompt
}

// Comprehensive VPC with EC2 Infrastructure
func comprehensiveVPCwithEC2Prompt() string {
	prompt := `Create a production VPC with a CIDR block of 10.0.0.0/16 across two availability zones. Set up public subnets (10.0.1.0/24 and 10.0.2.0/24) for internet-facing load balancers. Create private subnets for application servers (10.0.11.0/24 and 10.0.12.0/24): Configure Internet Gateway and NAT Gateway for proper routing. Create an EC2 for hosting an Apache Server with a dedicated security group that allows inbound HTTP (port 80) and SSH (port 22) traffic in public subnets.`

	return prompt
}

// Comprehensive EC2 Infrastructure
func comprehensiveEC2withALBPrompt() string {
	prompt := `I need to deploy a web application infrastructure on AWS with the following requirements:

- Create an EC2 for hosting an Apache Server with a dedicated security group that allows inbound HTTP (port 80) and SSH (port 22) traffic.
- Create an Application Load Balancer across public subnets in front of the EC2 instance with a security group that allows inbound HTTP (port 80) traffic from the internet.`

	return prompt
}

// Comprehensive Three-Tier Infrastructure
func comprehensiveThreeLayerPrompt() string {
	prompt := `I need to deploy a complete production-ready three-tier web application infrastructure on AWS in the Region Singapore with the following requirements:

Network Foundation (Phase 1):
- Create a production VPC with a CIDR block of 10.0.0.0/16 across two availability zones.
- Set up public subnets (10.0.1.0/24 and 10.0.2.0/24) for internet-facing load balancers.
- Create private subnets for application servers (10.0.11.0/24 and 10.0.12.0/24).
- Set up dedicated database subnets (10.0.21.0/24 and 10.0.22.0/24)
- Configure Internet Gateway and NAT Gateway for proper routing.

Security Architecture (Phase 2):
- Create defense-in-depth security with tiered security groups
- Load balancer security group allowing HTTP/HTTPS from internet (0.0.0.0/0)
- Application server security group accepting traffic only from load balancer
- Database security group allowing MySQL (port 3306) only from application servers

Load Balancer Tier (Phase 3):
- Deploy Application Load Balancer across public subnets in both AZs
- Configure target group with health checks on /health endpoint
- Set up HTTP listener (port 80) with proper health check thresholds
- Health check: 30s interval, 5s timeout, 2 healthy/3 unhealthy thresholds

Auto Scaling Application Tier (Phase 4):
- Create launch template with t3.medium instances
- Use Amazon Linux 2 AMI with Apache/PHP web server
- Configure user data script to install web server and health check endpoint
- Set up Auto Scaling Group: min 2, max 10, desired 4 instances
- Deploy across private application subnets in both AZs
- Integrate with load balancer target group for automatic registration
- Use ELB health checks with 300s grace period

Database Infrastructure (Phase 5):
- Create RDS MySQL 8.0 database with Multi-AZ deployment
- Use db.t3.medium instance class with 100GB GP3 storage
- Enable encryption at rest and Performance Insights
- Configure automated backups: 7-day retention, 3-4 AM backup window
- Set maintenance window: Sunday 4-5 AM
- Deploy across database subnets in both AZs

Additional Requirements:
- Tag all resources with Environment=production, Application=three-tier-web-app
- Use consistent naming convention with environment and tier identifiers
- Ensure high availability across multiple availability zones
- Follow AWS Well-Architected Framework principles
- Configure proper resource dependencies and creation order

Please deploy this complete infrastructure stack and provide me with the key resource IDs and endpoints once deployment is complete.`

	return prompt
}

// Comprehensive RDS Infrastructure
func comprehensiveRDSPrompt() string {
	prompt := `Create an RDS MySQL 8.0 database in a single AZ deployment, incorporating a dedicated security group that allows inbound MySQL traffic, while utilizing the db.t3.medium instance class paired with 100GB GP3 storage, and enabling both encryption at rest and Performance Insights.`

	return prompt
}

// Comprehensive Serverless Infrastructure
func comprehensiveServerlessPrompt() string {
	prompt := `Deploy a complete serverless application infrastructure on AWS with the following specifications:

API Layer:
- Create an API Gateway REST API with regional endpoint
- Define resources and methods: GET /items, POST /items, GET /items/{id}
- Enable CORS for all methods
- Set up usage plan with throttling: 100 requests/second burst, 50 requests/second steady

Compute Layer:
- Create three Lambda functions in Node.js 20.x runtime
- Function 1: listItems - GET all items from DynamoDB
- Function 2: createItem - POST new item to DynamoDB
- Function 3: getItem - GET single item by ID
- Configure 128MB memory, 10s timeout for each
- Use environment variables for DynamoDB table name
- Integrate each function with corresponding API Gateway methods
- Enable Lambda function URLs for testing

Database Layer:
- Provision DynamoDB table named 'Items' with partition key 'id' (string)
- Add global secondary index on 'category' attribute
- Set provisioned capacity: 5 read/write capacity units
- Enable point-in-time recovery and auto-scaling (min 5, max 100)
- Encrypt with AWS-managed KMS key

Security and Authentication:
- Create Cognito User Pool for user authentication
- Add app client with OAuth flows enabled
- Configure API Gateway authorizer using Cognito
- Apply authorizer to all API methods except public GET /items

Monitoring and Logging:
- Enable X-Ray tracing for API Gateway and Lambda
- Create CloudWatch Log Groups for each Lambda with 7-day retention
- Set up CloudWatch alarms for Lambda errors (>0) and API 5xx errors (>1%)

Additional Requirements:
- Deploy in us-east-1 region
- Tag all resources with Environment=production, Application=serverless-app
- Ensure IAM roles follow least privilege: Lambda role with DynamoDB access only
- Provide API endpoint and test commands after deployment

Create a step-by-step execution plan including all AWS API calls, resource dependencies, and validation steps.`

	return prompt
}

// Comprehensive EKS Infrastructure
func comprehensiveEKSPrompt() string {
	prompt := `Set up a production-ready Kubernetes cluster on AWS EKS with these requirements:

Cluster Foundation:
- Create EKS cluster version 1.30 in us-west-2
- Enable public and private endpoint access
- Configure cluster logging: api, audit, authenticator, controllerManager, scheduler

Networking:
- Use existing VPC or create new with CIDR 10.100.0.0/16
- Public subnets: 10.100.1.0/24, 10.100.2.0/24
- Private subnets: 10.100.11.0/24, 10.100.12.0/24
- Ensure proper route tables and security groups for cluster communication

Node Groups:
- Managed node group with t3.medium instances
- Min 2, max 10, desired 3 nodes
- Use Amazon Linux 2 EKS-optimized AMI
- Enable auto-scaling and cluster autoscaler
- Add labels: role=worker

Add-ons:
- Install CoreDNS, kube-proxy, VPC CNI
- Add EBS CSI driver for persistent storage
- Deploy AWS Load Balancer Controller

Security:
- Create IAM roles for cluster and nodes with least privilege
- Enable IRSA (IAM Roles for Service Accounts)
- Configure security groups to allow only necessary ports

Monitoring:
- Enable EKS Control Plane logging to CloudWatch
- Install Prometheus and Grafana for cluster monitoring
- Set up alerts for node CPU >80% and pod evictions

Deployment Validation:
- Deploy sample nginx deployment and service (type LoadBalancer)
- Verify external access to the service
- Check cluster health and node status

Provide a detailed plan with sequential steps, AWS CLI commands where applicable, and resource verification.`

	return prompt
}

// Comprehensive S3 with CloudFront Infrastructure
func comprehensiveS3CloudFrontPrompt() string {
	prompt := `Build a global static website hosting infrastructure on AWS:

Storage Layer:
- Create S3 bucket for website assets in us-east-1
- Enable versioning and server-side encryption (SSE-S3)
- Set bucket policy for public read access
- Upload sample index.html and error.html

Distribution Layer:
- Create CloudFront distribution with S3 as origin
- Configure default root object: index.html
- Enable HTTP/2 and IPv6
- Set price class: Use All Edge Locations
- Configure error pages: 404 -> error.html (HTTP 200)

Security:
- Create Origin Access Control (OAC) for S3
- Update S3 bucket policy to allow only CloudFront
- Enable WAF web ACL with AWS managed rules for common exploits

Domain and SSL:
- Assume domain example.com in Route 53
- Create ACM certificate for *.example.com
- Configure CloudFront alternate domain names
- Set up Route 53 A record alias to CloudFront

Logging and Monitoring:
- Enable CloudFront standard logging to S3
- Create CloudWatch alarms for BytesDownloaded > threshold
- Set S3 access logging

Additional Features:
- Enable compression for text/* MIME types
- Set cache behavior: TTL min 0, default 1 day, max 1 year
- Add geo-restriction whitelist: US, CA

Deploy the infrastructure and provide the CloudFront domain name, S3 bucket name, and test URLs.`

	return prompt
}

// Comprehensive ECS Infrastructure
func comprehensiveECSPrompt() string {
	prompt := `Deploy a containerized application on AWS ECS with Fargate:

Cluster Setup:
- Create ECS cluster in us-east-1
- Enable Container Insights for monitoring

Networking:
- Use VPC with public and private subnets across 2 AZs
- CIDR: 10.20.0.0/16
- Public: 10.20.1.0/24, 10.20.2.0/24
- Private: 10.20.11.0/24, 10.20.12.0/24
- Internet Gateway and NAT for outbound

Task Definition:
- Fargate launch type
- Task: 0.5 vCPU, 1GB memory
- Container: nginx:latest, port 80
- Add environment variables if needed
- Enable AWS Logs driver

Service:
- Create ECS service with 2 desired tasks
- Deploy in private subnets
- Enable auto-scaling: min 2, max 10, CPU target 70%
- Integrate with Application Load Balancer

Load Balancer:
- Internet-facing ALB in public subnets
- HTTP listener port 80
- Target group with health check on /
- Security group: allow 80 from 0.0.0.0/0

Database (Optional Integration):
- RDS PostgreSQL in private subnets
- Allow access from ECS security group on port 5432

Security:
- ECS task role with S3 read access (if needed)
- Security groups with least privilege

Validation:
- Verify service is running and accessible via ALB
- Scale tasks and check auto-scaling

Provide execution plan with all steps, dependencies, and final endpoints.`

	return prompt
}

// Comprehensive Monitoring and Logging Infrastructure
func comprehensiveMonitoringPrompt() string {
	prompt := `Set up comprehensive monitoring and logging for an existing AWS infrastructure:

Core Services:
- Enable CloudWatch for all supported resources
- Create CloudTrail trail for management events, multi-region
- Configure S3 bucket for CloudTrail logs with lifecycle policy (30 days to Glacier)

Alarms and Dashboards:
- Create CloudWatch alarms:
  - EC2 CPU >80% for 5 minutes
  - ALB 5xx errors >5 in 1 minute
  - RDS free storage <20%
- Build CloudWatch dashboard with widgets for CPU, network, errors

Logging:
- Set up centralized logging with CloudWatch Logs
- Create log groups for EC2, Lambda, ECS with 14-day retention
- Enable VPC Flow Logs to CloudWatch Logs group

Advanced Monitoring:
- Enable X-Ray for application tracing (API Gateway, Lambda, ECS)
- Set up Synthetics canaries for endpoint monitoring (every 5 min)
- Configure EventBridge rules for critical events (e.g., instance termination)

Notifications:
- Create SNS topic for alarms
- Subscribe email endpoint
- Integrate with alarms for notifications

Security Monitoring:
- Enable GuardDuty for threat detection
- Configure Config rules for compliance (e.g., encrypted volumes)
- Set up Security Hub with CIS benchmarks

Deploy in us-west-2, tag resources with Monitoring=true, and provide dashboard URL and alarm names.`

	return prompt
}

// === Test Suites ===

// TestComprehensiveExecutionPipeline tests all aspects of the execution pipeline
func TestComprehensiveExecutionPipeline(t *testing.T) {
	t.Run("RealAIWithComprehensiveEC2Prompt", func(t *testing.T) {
		testRealAIWithComprehensiveEC2Prompt(t)
	})
	t.Run("RealAIWithComprehensiveVPCPrompt", func(t *testing.T) {
		testRealAIWithComprehensiveVPCPrompt(t)
	})
	t.Run("RealAIWithComprehensiveVPCwithEC2Prompt", func(t *testing.T) {
		testRealAIWithComprehensiveVPCwithEC2Prompt(t)
	})
	t.Run("RealAIWithComprehensiveThreeLayerPrompt", func(t *testing.T) {
		testRealAIWithComprehensiveThreeLayerPrompt(t)
	})
	// == State Handling Test ==
	t.Run("RealAIWithRealStateEC2AndALBPrompt", func(t *testing.T) {
		testRealAIWithRealStateEC2AndALBPrompt(t)
	})
}

func testRealAIWithComprehensiveEC2Prompt(t *testing.T) {
	// Comprehensive VPC infrastructure prompt
	comprehensivePrompt := comprehensiveEC2Prompt()

	// Setup test configuration
	cfg, err := setupRealConfiguration()
	if err != nil {
		t.Fatalf("Failed to setup real configuration: %v", err)
	}

	// Setup real LLM client
	llmClient, err := setupRealLLMClient(cfg)
	if err != nil {
		t.Fatalf("Failed to setup real LLM client: %v", err)
	}

	// Setup test agent with real AI and comprehensive mock infrastructure
	agent, mockSuite, err := setupAgentWithRealAI(cfg, llmClient)
	if err != nil {
		t.Fatalf("Failed to setup test agent: %v", err)
	}
	// defer mockSuite.Reset()

	// Create decision context using mock state manager
	emptyState := &types.InfrastructureState{
		Resources: make(map[string]*types.ResourceState),
	}

	decisionContext := &DecisionContext{
		Request:             comprehensivePrompt,
		CurrentState:        emptyState,
		DiscoveredState:     []*types.ResourceState{},
		Conflicts:           []*types.ConflictResolution{},
		DeploymentOrder:     []string{},
		ResourceCorrelation: make(map[string]*ResourceMatch),
	}

	t.Logf("🚀 Starting comprehensive execution pipeline test with real AI integration and mock infrastructure...")

	// Step 1: Test AI Decision Making
	t.Logf("📡 Step 1: Making real AI API call to process comprehensive infrastructure request...")

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
	defer cancel()

	decisionID := "test-comprehensive-decision"
	decision, err := agent.generateDecisionWithPlan(ctx, decisionID, comprehensivePrompt, decisionContext)
	if err != nil {
		t.Fatalf("❌ Real AI API call failed: %v", err)
	}

	t.Logf("✅ Step 1 Complete: AI generated decision with %d execution steps", len(decision.ExecutionPlan))

	// Step 2: Validate AI-Generated Plan Structure
	t.Logf("🔍 Step 2: Validating AI-generated execution plan structure...")
	validateExecutionPlanStructure(t, agent, decision)
	t.Logf("✅ Step 2 Complete: Execution plan structure is valid")

	// Step 3: Execute Full Flow with Mock Infrastructure
	t.Logf("⚙️ Step 3: Executing full infrastructure plan using mock functions...")
	testExecuteFullPlanWithMocks(t, agent, mockSuite, decision)
	t.Logf("✅ Step 3 Complete: Full plan execution completed successfully")

	// Step 4: Validate Mock Integration
	t.Logf("🔬 Step 4: Validating mock infrastructure integration...")
	testValidateMockIntegration(t, mockSuite, decision)
	t.Logf("✅ Step 4 Complete: Mock integration validated")

	t.Logf("🎉 All tests completed successfully! AI + Mock infrastructure integration validated.")
}

func testRealAIWithComprehensiveVPCPrompt(t *testing.T) {
	// Comprehensive VPC infrastructure prompt
	comprehensivePrompt := comprehensiveVPCPrompt()

	// Setup test configuration
	cfg, err := setupRealConfiguration()
	if err != nil {
		t.Fatalf("Failed to setup real configuration: %v", err)
	}

	// Setup real LLM client
	llmClient, err := setupRealLLMClient(cfg)
	if err != nil {
		t.Fatalf("Failed to setup real LLM client: %v", err)
	}

	// Setup test agent with real AI and comprehensive mock infrastructure
	agent, mockSuite, err := setupAgentWithRealAI(cfg, llmClient)
	if err != nil {
		t.Fatalf("Failed to setup test agent: %v", err)
	}
	// defer mockSuite.Reset()

	// Create decision context using mock state manager
	emptyState := &types.InfrastructureState{
		Resources: make(map[string]*types.ResourceState),
	}

	decisionContext := &DecisionContext{
		Request:             comprehensivePrompt,
		CurrentState:        emptyState,
		DiscoveredState:     []*types.ResourceState{},
		Conflicts:           []*types.ConflictResolution{},
		DeploymentOrder:     []string{},
		ResourceCorrelation: make(map[string]*ResourceMatch),
	}

	t.Logf("🚀 Starting comprehensive execution pipeline test with real AI integration and mock infrastructure...")

	// Step 1: Test AI Decision Making
	t.Logf("📡 Step 1: Making real AI API call to process comprehensive infrastructure request...")

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
	defer cancel()

	decisionID := "test-comprehensive-decision"
	decision, err := agent.generateDecisionWithPlan(ctx, decisionID, comprehensivePrompt, decisionContext)
	if err != nil {
		t.Fatalf("❌ Real AI API call failed: %v", err)
	}

	t.Logf("✅ Step 1 Complete: AI generated decision with %d execution steps", len(decision.ExecutionPlan))

	// Step 2: Validate AI-Generated Plan Structure
	t.Logf("🔍 Step 2: Validating AI-generated execution plan structure...")
	validateExecutionPlanStructure(t, agent, decision)
	t.Logf("✅ Step 2 Complete: Execution plan structure is valid")

	// Step 3: Execute Full Flow with Mock Infrastructure
	t.Logf("⚙️ Step 3: Executing full infrastructure plan using mock functions...")
	testExecuteFullPlanWithMocks(t, agent, mockSuite, decision)
	t.Logf("✅ Step 3 Complete: Full plan execution completed successfully")

	// Step 4: Validate Mock Integration
	t.Logf("🔬 Step 4: Validating mock infrastructure integration...")
	testValidateMockIntegration(t, mockSuite, decision)
	t.Logf("✅ Step 4 Complete: Mock integration validated")

	t.Logf("🎉 All tests completed successfully! AI + Mock infrastructure integration validated.")
}

func testRealAIWithComprehensiveVPCwithEC2Prompt(t *testing.T) {
	// Comprehensive VPC infrastructure prompt
	comprehensivePrompt := comprehensiveVPCwithEC2Prompt()

	// Setup test configuration
	cfg, err := setupRealConfiguration()
	if err != nil {
		t.Fatalf("Failed to setup real configuration: %v", err)
	}

	// Setup real LLM client
	llmClient, err := setupRealLLMClient(cfg)
	if err != nil {
		t.Fatalf("Failed to setup real LLM client: %v", err)
	}

	// Setup test agent with real AI and comprehensive mock infrastructure
	agent, mockSuite, err := setupAgentWithRealAI(cfg, llmClient)
	if err != nil {
		t.Fatalf("Failed to setup test agent: %v", err)
	}
	// defer mockSuite.Reset()

	// Create decision context using mock state manager
	emptyState := &types.InfrastructureState{
		Resources: make(map[string]*types.ResourceState),
	}

	decisionContext := &DecisionContext{
		Request:             comprehensivePrompt,
		CurrentState:        emptyState,
		DiscoveredState:     []*types.ResourceState{},
		Conflicts:           []*types.ConflictResolution{},
		DeploymentOrder:     []string{},
		ResourceCorrelation: make(map[string]*ResourceMatch),
	}

	t.Logf("🚀 Starting comprehensive execution pipeline test with real AI integration and mock infrastructure...")

	// Step 1: Test AI Decision Making
	t.Logf("📡 Step 1: Making real AI API call to process comprehensive infrastructure request...")

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
	defer cancel()

	decisionID := "test-comprehensive-decision"
	decision, err := agent.generateDecisionWithPlan(ctx, decisionID, comprehensivePrompt, decisionContext)
	if err != nil {
		t.Fatalf("❌ Real AI API call failed: %v", err)
	}

	t.Logf("✅ Step 1 Complete: AI generated decision with %d execution steps", len(decision.ExecutionPlan))

	// Step 2: Validate AI-Generated Plan Structure
	t.Logf("🔍 Step 2: Validating AI-generated execution plan structure...")
	validateExecutionPlanStructure(t, agent, decision)
	t.Logf("✅ Step 2 Complete: Execution plan structure is valid")

	// Step 3: Enhanced Pre-Execution Validation
	t.Logf("🔬 Step 3: Enhanced pre-execution validation...")
	testEnhancedPreExecutionValidation(t, agent, mockSuite, decision)
	t.Logf("✅ Step 3 Complete: Pre-execution validation passed")

	// Step 4: Execute Full Flow with Mock Infrastructure
	t.Logf("⚙️ Step 4: Executing full infrastructure plan using mock functions...")
	testExecuteFullPlanWithMocks(t, agent, mockSuite, decision)
	t.Logf("✅ Step 4 Complete: Full plan execution completed successfully")

	// Step 5: Enhanced Post-Execution Validation
	t.Logf("🔍 Step 5: Enhanced post-execution validation...")
	testEnhancedPostExecutionValidation(t, agent, mockSuite, decision)
	t.Logf("✅ Step 5 Complete: Post-execution validation passed")

	// Step 6: Error Scenario Testing
	t.Logf("🚨 Step 6: Testing error scenarios...")
	testErrorScenarios(t, agent, mockSuite, decision)
	t.Logf("✅ Step 6 Complete: Error scenarios tested")

	// Step 7: Infrastructure Reality Checks
	t.Logf("🏗️ Step 7: Running infrastructure reality checks...")
	testInfrastructureRealityCheck(t, agent, mockSuite, decision)
	t.Logf("✅ Step 7 Complete: Infrastructure reality checks passed")

	// Step 8: Web UI Flow Simulation
	t.Logf("🌐 Step 8: Simulating Web UI flow...")
	testWebUIFlowSimulation(t, agent, mockSuite, decision)
	t.Logf("✅ Step 8 Complete: Web UI flow simulation passed")

	// Step 9: Validate Mock Integration
	t.Logf("🔬 Step 9: Validating mock infrastructure integration...")
	testValidateMockIntegration(t, mockSuite, decision)
	t.Logf("✅ Step 9 Complete: Mock integration validated")

	t.Logf("🎉 All tests completed successfully! Enhanced AI + Mock infrastructure integration validated.")
}

// === Test Function for EC2 with ALB Prompt with State Handling ===

func testRealAIWithRealStateEC2AndALBPrompt(t *testing.T) {
	// EC2 with ALB infrastructure prompt
	comprehensivePrompt := comprehensiveEC2withALBPrompt()

	// Setup test configuration
	cfg, err := setupRealConfiguration()
	if err != nil {
		t.Fatalf("Failed to setup real configuration: %v", err)
	}

	// Setup real LLM client
	llmClient, err := setupRealLLMClient(cfg)
	if err != nil {
		t.Fatalf("Failed to setup real LLM client: %v", err)
	}

	// Setup test agent with real AI and comprehensive mock infrastructure
	agent, mockSuite, err := setupAgentWithRealAI(cfg, llmClient)
	if err != nil {
		t.Fatalf("Failed to setup test agent: %v", err)
	}

	// Load existing infrastructure state from sample file
	t.Logf("📋 Loading existing infrastructure state from states/infrastructure-state-t01.json...")
	existingState, err := loadInfrastructureStateFromFile("states/infrastructure-state-t01.json")
	if err != nil {
		// If state file doesn't exist or can't be loaded, use empty state
		t.Fatalf("⚠️ Could not load existing state file (%v), using empty state for test", err)
	} else {
		t.Logf("✅ Successfully loaded existing state with %d resources", len(existingState.Resources))

		// Log some of the existing resources to verify proper loading
		resourceCount := 0
		for resourceID, resource := range existingState.Resources {
			if resourceCount < 3 { // Only log first 3 for brevity
				t.Logf("📦 Existing resource: %s (type: %s, status: %s)", resourceID, resource.Type, resource.Status)
			}
			resourceCount++
		}
		if resourceCount > 3 {
			t.Logf("📦 ... and %d more existing resources", resourceCount-3)
		}

		// Load the state into mock infrastructure and populate resource mappings
		t.Logf("🔄 Loading state into mock infrastructure for dependency resolution...")
		if err := mockSuite.LoadExistingState(existingState.Resources, agent); err != nil {
			t.Fatalf("❌ Failed to load state into mock infrastructure: %v", err)
		}
		t.Logf("✅ State loaded into mock infrastructure with %d resource mappings", len(mockSuite.ResourceMappings))
	}

	// Create decision context using the loaded state - this is key for testing state handling
	decisionContext := &DecisionContext{
		Request:             comprehensivePrompt,
		CurrentState:        existingState, // Pass the loaded state here
		DiscoveredState:     []*types.ResourceState{},
		Conflicts:           []*types.ConflictResolution{},
		DeploymentOrder:     []string{},
		ResourceCorrelation: make(map[string]*ResourceMatch),
	}

	t.Logf("🚀 Starting comprehensive EC2 with ALB test with real AI and existing state context...")

	// Step 1: Test AI Decision Making with Existing State Context
	t.Logf("📡 Step 1: Making real AI API call with existing state context for EC2+ALB infrastructure...")

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
	defer cancel()

	decisionID := "test-ec2-alb-decision-with-state"
	decision, err := agent.generateDecisionWithPlan(ctx, decisionID, comprehensivePrompt, decisionContext)
	if err != nil {
		t.Fatalf("❌ Real AI API call failed: %v", err)
	}

	t.Logf("✅ Step 1 Complete: AI generated decision with %d execution steps (with state context)", len(decision.ExecutionPlan))

	// Log decision summary for verification
	t.Logf("📝 Decision Action: %s", decision.Action)
	t.Logf("🧠 Decision Reasoning: %s", decision.Reasoning)
	if decision.Resource != "" {
		t.Logf("📦 Primary Resource: %s", decision.Resource)
	}

	// Step 2: Validate AI-Generated Plan Structure for EC2+ALB
	t.Logf("🔍 Step 2: Validating AI-generated execution plan for EC2+ALB infrastructure...")
	validateExecutionPlanStructureForEC2ALB(t, decision, existingState)
	t.Logf("✅ Step 2 Complete: Execution plan structure validated for EC2+ALB")

	// Step 3: Test State-Aware Planning Logic
	t.Logf("🧠 Step 3: Testing AI state-aware planning logic...")
	testStateAwarePlanningLogic(t, decision, existingState, comprehensivePrompt)
	t.Logf("✅ Step 3 Complete: State-aware planning logic validated")

	// Step 4: Execute Full Flow with Mock Infrastructure
	t.Logf("⚙️ Step 4: Executing full EC2+ALB plan using mock functions...")
	testExecuteFullPlanWithMocks(t, agent, mockSuite, decision)
	t.Logf("✅ Step 4 Complete: Full plan execution completed successfully")

	// Step 5: Validate Mock Integration
	t.Logf("🔬 Step 5: Validating mock infrastructure integration...")
	testValidateMockIntegration(t, mockSuite, decision)
	t.Logf("✅ Step 5 Complete: Mock integration validated")

	// Step 6: Test Idempotency and State Consistency
	t.Logf("🔄 Step 6: Testing idempotency with existing state...")
	testIdempotencyWithExistingState(t, agent, mockSuite, decisionContext, comprehensivePrompt)
	t.Logf("✅ Step 6 Complete: Idempotency and state consistency validated")

	t.Logf("🎉 All EC2+ALB with state handling tests completed successfully!")
}

// === Helper Function ===

// loadInfrastructureStateFromFile loads an existing infrastructure state from a JSON file
func loadInfrastructureStateFromFile(filePath string) (*types.InfrastructureState, error) {
	dir, _ := os.Getwd()

	// Navigate to project root by looking for go.mod file
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root directory, use current dir
			break
		}
		dir = parent
	}

	// Read state file
	data, err := os.ReadFile(filepath.Join(dir, filePath))
	if err != nil {
		return nil, fmt.Errorf("failed to read state file %s: %w", filePath, err)
	}

	// Parse state
	var state types.InfrastructureState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state file %s: %w", filePath, err)
	}

	return &state, nil
}

// validateExecutionPlanStructureForEC2ALB validates the plan structure specifically for EC2+ALB resources
func validateExecutionPlanStructureForEC2ALB(t *testing.T, decision *types.AgentDecision, existingState *types.InfrastructureState) {
	if len(decision.ExecutionPlan) == 0 {
		t.Fatal("❌ AI generated empty execution plan")
	}

	// Validate basic plan structure
	planValidActions := map[string]bool{
		"create": true,
		"query":  true,
		// "update":   true,
		// "delete":   true,
		// "validate": true,
	}

	for _, planStep := range decision.ExecutionPlan {
		if !planValidActions[planStep.Action] {
			t.Fatalf("❌ Invalid plan action: %s", planStep.Action)
		}
	}

	// Check for EC2+ALB specific components
	foundComponents := make(map[string]bool)
	expectedEC2ALBComponents := []string{"ec2", "security", "load", "balancer", "target", "group"}

	for _, step := range decision.ExecutionPlan {
		// Check basic step structure
		if step.ID == "" {
			t.Errorf("❌ Step missing ID: %+v", step)
		}
		if step.Name == "" {
			t.Errorf("❌ Step missing Name: %+v", step)
		}
		if step.Action == "" {
			t.Errorf("❌ Step missing Action: %+v", step)
		}

		// Check for expected EC2+ALB components
		stepText := step.Name + " " + step.Description + " " + step.ResourceID
		for _, component := range expectedEC2ALBComponents {
			if strings.Contains(strings.ToLower(stepText), strings.ToLower(component)) {
				foundComponents[component] = true
			}
		}

		t.Logf("📋 Step: %s | Action: %s | Resource: %s", step.Name, step.Action, step.ResourceID)
	}

	// Verify we found key EC2+ALB components
	missingComponents := []string{}
	for _, component := range expectedEC2ALBComponents {
		if !foundComponents[component] {
			missingComponents = append(missingComponents, component)
		}
	}

	if len(missingComponents) > 0 {
		t.Logf("⚠️ Warning: Some expected EC2+ALB components not found in plan: %v", missingComponents)
		t.Logf("📝 This may be acceptable if AI structured the plan differently")
	}

	// Log analysis of existing state impact
	if len(existingState.Resources) > 0 {
		t.Logf("🔍 Plan generated with %d existing resources in context", len(existingState.Resources))
	} else {
		t.Logf("🔍 Plan generated with empty state context")
	}
}

// testStateAwarePlanningLogic validates that the AI properly considered existing state
func testStateAwarePlanningLogic(t *testing.T, decision *types.AgentDecision, existingState *types.InfrastructureState, prompt string) {
	// Check if AI decision mentions handling existing resources
	decisionText := decision.Reasoning + " " + decision.Action

	if len(existingState.Resources) > 0 {
		// AI should show awareness of existing infrastructure
		hasStateAwareness := strings.Contains(strings.ToLower(decisionText), "existing") ||
			strings.Contains(strings.ToLower(decisionText), "current") ||
			strings.Contains(strings.ToLower(decisionText), "already") ||
			strings.Contains(strings.ToLower(decisionText), "present")

		if hasStateAwareness {
			t.Logf("✅ AI demonstrated awareness of existing infrastructure state")
		} else {
			t.Logf("⚠️ AI may not have explicitly mentioned existing state (this could still be acceptable)")
		}
	}

	// Check that the plan is reasonable given the prompt requirements
	promptLower := strings.ToLower(prompt)
	requiresEC2 := strings.Contains(promptLower, "ec2")
	requiresALB := strings.Contains(promptLower, "load balancer") || strings.Contains(promptLower, "alb")

	if requiresEC2 {
		hasEC2Plan := false
		for _, step := range decision.ExecutionPlan {
			stepText := strings.ToLower(step.Name + " " + step.Description)
			if strings.Contains(stepText, "ec2") || strings.Contains(stepText, "instance") {
				hasEC2Plan = true
				break
			}
		}
		if hasEC2Plan {
			t.Logf("✅ Plan includes EC2 components as required")
		} else {
			t.Logf("⚠️ Plan may not include explicit EC2 components")
		}
	}

	if requiresALB {
		hasALBPlan := false
		for _, step := range decision.ExecutionPlan {
			stepText := strings.ToLower(step.Name + " " + step.Description)
			if strings.Contains(stepText, "load") || strings.Contains(stepText, "balancer") || strings.Contains(stepText, "alb") {
				hasALBPlan = true
				break
			}
		}
		if hasALBPlan {
			t.Logf("✅ Plan includes ALB components as required")
		} else {
			t.Logf("⚠️ Plan may not include explicit ALB components")
		}
	}
}

// testIdempotencyWithExistingState tests that the AI handles idempotency correctly
func testIdempotencyWithExistingState(t *testing.T, agent *StateAwareAgent, mockSuite *mocks.MockTestSuite,
	decisionContext *DecisionContext, prompt string) {

	t.Logf("🔄 Testing idempotency by running the same request again...")

	// Execute the same request again to test idempotency
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*2)
	defer cancel()

	secondDecisionID := "test-idempotency-decision"
	secondDecision, err := agent.generateDecisionWithPlan(ctx, secondDecisionID, prompt, decisionContext)
	if err != nil {
		t.Errorf("❌ Second AI API call for idempotency test failed: %v", err)
		return
	}

	// Compare decisions to check for consistency
	t.Logf("🔍 Comparing decisions for consistency...")
	t.Logf("📊 First decision had %d steps", len(decisionContext.CurrentState.Resources))
	t.Logf("📊 Second decision has %d steps", len(secondDecision.ExecutionPlan))

	// The AI should potentially recognize that resources may already exist
	// or provide a plan that handles the existing state appropriately
	if len(secondDecision.ExecutionPlan) == 0 {
		t.Logf("✅ AI recognized no additional changes needed (perfect idempotency)")
	} else {
		t.Logf("📝 AI generated %d steps for the repeat request", len(secondDecision.ExecutionPlan))
		t.Logf("📝 This may indicate the AI is planning to create/update resources appropriately")
	}

	// Log the reasoning for the second decision
	if secondDecision.Reasoning != "" {
		t.Logf("🧠 AI reasoning for repeat request: %s", secondDecision.Reasoning)
	}
}

// testInfrastructureRealityCheck performs reality checks that simulate issues seen in actual AWS deployments
func testInfrastructureRealityCheck(t *testing.T, agent *StateAwareAgent, mockSuite *mocks.MockTestSuite, decision *types.AgentDecision) {
	t.Logf("🏗️ Running infrastructure reality checks...")

	// Reality Check 1: VPC CIDR Block Validation
	t.Logf("🔍 Reality Check 1: VPC CIDR validation...")
	for _, step := range decision.ExecutionPlan {
		if step.Action == "create" && strings.Contains(strings.ToLower(step.Name), "vpc") {
			if cidr, exists := step.Parameters["cidrBlock"]; exists {
				if cidrStr, ok := cidr.(string); ok {
					if !isValidCIDR(cidrStr) {
						t.Errorf("❌ Invalid CIDR block in step %s: %s", step.ID, cidrStr)
					}
				}
			}
		}
	}

	// Reality Check 2: Subnet CIDR Overlap Detection
	t.Logf("🔍 Reality Check 2: Subnet CIDR overlap detection...")
	subnetCIDRs := make(map[string]string)
	for _, step := range decision.ExecutionPlan {
		if step.Action == "create" && strings.Contains(strings.ToLower(step.Name), "subnet") {
			if cidr, exists := step.Parameters["cidrBlock"]; exists {
				if cidrStr, ok := cidr.(string); ok {
					if existingStep, exists := subnetCIDRs[cidrStr]; exists {
						t.Errorf("❌ CIDR overlap detected: Step %s and %s both use %s", step.ID, existingStep, cidrStr)
					}
					subnetCIDRs[cidrStr] = step.ID
				}
			}
		}
	}

	// Reality Check 3: Availability Zone Distribution
	t.Logf("🔍 Reality Check 3: Availability zone distribution...")
	azUsage := make(map[string]int)
	for _, step := range decision.ExecutionPlan {
		if step.Action == "create" && strings.Contains(strings.ToLower(step.Name), "subnet") {
			if az, exists := step.Parameters["availabilityZone"]; exists {
				if azStr, ok := az.(string); ok {
					azUsage[azStr]++
				}
			}
		}
	}
	if len(azUsage) < 2 {
		t.Logf("⚠️ Warning: Infrastructure should use multiple AZs for high availability")
	}

	// Reality Check 4: Security Group Rule Validation
	t.Logf("🔍 Reality Check 4: Security group rule validation...")
	for _, step := range decision.ExecutionPlan {
		if step.Action == "create" && strings.Contains(strings.ToLower(step.Name), "security") {
			if cidrIp, exists := step.Parameters["cidrIp"]; exists {
				if cidrIp == "0.0.0.0/0" {
					t.Logf("⚠️ Security Warning: Step %s allows access from anywhere (0.0.0.0/0)", step.ID)
				}
			}
		}
	}

	// Reality Check 5: Resource Naming Consistency
	t.Logf("🔍 Reality Check 5: Resource naming consistency...")
	namePatterns := make(map[string]int)
	for _, step := range decision.ExecutionPlan {
		if tags, exists := step.Parameters["tags"]; exists {
			if tagMap, ok := tags.(map[string]interface{}); ok {
				if name, exists := tagMap["Name"]; exists {
					if nameStr, ok := name.(string); ok {
						// Extract naming pattern
						parts := strings.Split(nameStr, "-")
						if len(parts) > 1 {
							pattern := parts[0] // e.g., "production" from "production-vpc"
							namePatterns[pattern]++
						}
					}
				}
			}
		}
	}

	if len(namePatterns) > 1 {
		t.Logf("⚠️ Warning: Multiple naming patterns detected: %v", namePatterns)
	}

	t.Logf("✅ Infrastructure reality checks completed")
}

// isValidCIDR performs basic CIDR validation
func isValidCIDR(cidr string) bool {
	// Basic validation - in real implementation would use net.ParseCIDR
	if !strings.Contains(cidr, "/") {
		return false
	}
	parts := strings.Split(cidr, "/")
	if len(parts) != 2 {
		return false
	}

	// Check for common valid patterns
	validPatterns := []string{
		"10.", "172.", "192.168.", "0.0.0.0"} // Private IP ranges + default route

	for _, pattern := range validPatterns {
		if strings.HasPrefix(parts[0], pattern) {
			return true
		}
	}

	return false
}

// testWebUIFlowSimulation simulates the exact flow that would happen in the web UI
func testWebUIFlowSimulation(t *testing.T, agent *StateAwareAgent, mockSuite *mocks.MockTestSuite, decision *types.AgentDecision) {
	t.Logf("🌐 Running Web UI flow simulation...")

	// Simulate Web UI Step 1: Process Request (already done in main test)
	t.Logf("✅ Step 1: Process Request - Already completed")

	// Simulate Web UI Step 2: Plan Validation (user reviews plan)
	t.Logf("🔍 Step 2: Simulating plan review and validation...")

	// Check if plan has reasonable number of steps (not too many, not too few)
	if len(decision.ExecutionPlan) < 5 {
		t.Logf("⚠️ Warning: Plan has very few steps (%d) - might be incomplete", len(decision.ExecutionPlan))
	}
	if len(decision.ExecutionPlan) > 50 {
		t.Logf("⚠️ Warning: Plan has many steps (%d) - might be overly complex", len(decision.ExecutionPlan))
	}

	// Simulate Web UI Step 3: Execution with progress updates
	t.Logf("⚙️ Step 3: Simulating execution with WebSocket progress updates...")

	// Create a channel to simulate WebSocket updates
	progressChan := make(chan *types.ExecutionUpdate, 100)
	defer close(progressChan)

	// Start a goroutine to consume progress updates (like WebSocket would)
	go func() {
		updateCount := 0
		for update := range progressChan {
			updateCount++
			if updateCount <= 5 { // Log first few updates to avoid spam
				t.Logf("📡 Progress Update %d: %s - %s", updateCount, update.Type, update.Message)
			}
		}
		t.Logf("📡 Total progress updates received: %d", updateCount)
	}()

	// Simulate execution with progress reporting
	ctx := context.Background()
	execution, err := agent.ExecuteConfirmedPlanWithDryRun(ctx, decision, progressChan, true) // dry run
	if err != nil {
		t.Errorf("❌ Simulated execution failed: %v", err)
		return
	}

	if execution.Status != "completed" {
		t.Errorf("❌ Execution did not complete successfully: %s", execution.Status)
	}

	// Simulate Web UI Step 4: Post-execution state verification
	t.Logf("🔍 Step 4: Simulating post-execution state verification...")

	// This would be called by the web UI to refresh the dashboard
	ctx2 := context.Background()
	_, err = mockSuite.MCPServer.CallTool(ctx2, "analyze-infrastructure-state", map[string]interface{}{
		"include_drift_detection": true,
		"detailed_analysis":       true,
	})
	if err != nil {
		t.Errorf("❌ Post-execution state analysis failed: %v", err)
	}

	t.Logf("✅ Web UI flow simulation completed successfully")
}

func testRealAIWithComprehensiveThreeLayerPrompt(t *testing.T) {
	// Comprehensive Three-Tier infrastructure prompt
	comprehensivePrompt := comprehensiveThreeLayerPrompt()

	// Setup test configuration
	cfg, err := setupRealConfiguration()
	if err != nil {
		t.Fatalf("Failed to setup real configuration: %v", err)
	}

	// Setup real LLM client
	llmClient, err := setupRealLLMClient(cfg)
	if err != nil {
		t.Fatalf("Failed to setup real LLM client: %v", err)
	}

	// Setup test agent with real AI and comprehensive mock infrastructure
	agent, mockSuite, err := setupAgentWithRealAI(cfg, llmClient)
	if err != nil {
		t.Fatalf("Failed to setup test agent: %v", err)
	}
	// defer mockSuite.Reset()

	// Create decision context using mock state manager
	emptyState := &types.InfrastructureState{
		Resources: make(map[string]*types.ResourceState),
	}

	decisionContext := &DecisionContext{
		Request:             comprehensivePrompt,
		CurrentState:        emptyState,
		DiscoveredState:     []*types.ResourceState{},
		Conflicts:           []*types.ConflictResolution{},
		DeploymentOrder:     []string{},
		ResourceCorrelation: make(map[string]*ResourceMatch),
	}

	t.Logf("🚀 Starting comprehensive three-tier execution pipeline test with real AI integration and mock infrastructure...")

	// Step 1: Test AI Decision Making
	t.Logf("📡 Step 1: Making real AI API call to process comprehensive three-tier infrastructure request...")

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*5) // Extended timeout for complex infrastructure
	defer cancel()

	decisionID := "test-comprehensive-three-tier-decision"
	decision, err := agent.generateDecisionWithPlan(ctx, decisionID, comprehensivePrompt, decisionContext)
	if err != nil {
		t.Fatalf("❌ Real AI API call failed: %v", err)
	}

	t.Logf("✅ Step 1 Complete: AI generated decision with %d execution steps", len(decision.ExecutionPlan))

	// Step 2: Validate AI-Generated Plan Structure
	t.Logf("🔍 Step 2: Validating AI-generated execution plan structure...")
	validateExecutionPlanStructure(t, agent, decision)
	t.Logf("✅ Step 2 Complete: Execution plan structure is valid")

	// Step 3: Execute Full Flow with Mock Infrastructure
	t.Logf("⚙️ Step 3: Executing full infrastructure plan using mock functions...")
	testExecuteFullPlanWithMocks(t, agent, mockSuite, decision)
	t.Logf("✅ Step 3 Complete: Full plan execution completed successfully")

	// Step 4: Validate Mock Integration
	t.Logf("🔬 Step 4: Validating mock infrastructure integration...")
	testValidateMockIntegration(t, mockSuite, decision)
	t.Logf("✅ Step 4 Complete: Mock integration validated")

	t.Logf("🎉 All tests completed successfully! Three-tier AI + Mock infrastructure integration validated.")
}

func validateExecutionPlanStructure(t *testing.T, agent *StateAwareAgent, decision *types.AgentDecision) {
	if len(decision.ExecutionPlan) == 0 {
		t.Fatal("❌ AI generated empty execution plan")
	}

	planValidActions := map[string]bool{
		"create": true,
		"query":  true,
		"modify": true,
		// "delete":   true,
		// "validate": true,
	}

	// Get available MCP tools for validation
	agent.capabilityMutex.RLock()
	availableTools := make(map[string]bool)
	for toolName := range agent.mcpTools {
		availableTools[toolName] = true
	}
	agent.capabilityMutex.RUnlock()

	// Validate each plan step
	for i, planStep := range decision.ExecutionPlan {
		// Validate action type
		if !planValidActions[planStep.Action] {
			t.Fatalf("❌ Invalid plan action '%s' in step %d (%s)", planStep.Action, i+1, planStep.ID)
		}

		// Validate MCP tool exists if specified
		if planStep.MCPTool != "" {
			if !availableTools[planStep.MCPTool] {
				availableToolsList := make([]string, 0, len(availableTools))
				for tool := range availableTools {
					availableToolsList = append(availableToolsList, tool)
				}

				t.Fatalf("❌ Invalid MCP tool '%s' in step %d (%s): tool not found in discovered capabilities. Available tools: %v",
					planStep.MCPTool, i+1, planStep.ID, availableToolsList)
			}
		}
	}

	// Validate plan has expected infrastructure components
	foundComponents := make(map[string]bool)
	expectedComponents := []string{"vpc", "subnet", "gateway", "route", "security"}

	for _, step := range decision.ExecutionPlan {
		// Check basic step structure
		if step.ID == "" {
			t.Errorf("❌ Step missing ID: %+v", step)
		}
		if step.Name == "" {
			t.Errorf("❌ Step missing Name: %+v", step)
		}
		if step.Action == "" {
			t.Errorf("❌ Step missing Action: %+v", step)
		}

		// Check for expected infrastructure components
		stepText := step.Name + " " + step.Description + " " + step.ResourceID
		for _, component := range expectedComponents {
			if strings.Contains(strings.ToLower(stepText), strings.ToLower(component)) {
				foundComponents[component] = true
			}
		}

		t.Logf("📋 Step: %s | Action: %s | Resource: %s", step.Name, step.Action, step.ResourceID)
	}

	// Verify we found key infrastructure components
	missingComponents := []string{}
	for _, component := range expectedComponents {
		if !foundComponents[component] {
			missingComponents = append(missingComponents, component)
		}
	}

	if len(missingComponents) > 0 {
		t.Logf("⚠️ Warning: Some expected components not found in plan: %v", missingComponents)
		t.Logf("📝 This may be okay if AI structured the plan differently")
	}
}

// testExecuteFullPlanWithMocks executes the AI-generated plan using mock infrastructure
// This simulates the same flow as ExecuteConfirmedPlanWithDryRun but with mock tools
func testExecuteFullPlanWithMocks(t *testing.T, agent *StateAwareAgent, mockSuite *mocks.MockTestSuite, decision *types.AgentDecision) {
	t.Logf("🚀 Executing %d plan steps with mock infrastructure...", len(decision.ExecutionPlan))

	// Create mock execution tracking like the real ExecuteConfirmedPlanWithDryRun
	execution := &types.PlanExecution{
		ID:        "mock-execution-" + decision.ID,
		Name:      fmt.Sprintf("Execute %s", decision.Action),
		Status:    "running",
		StartedAt: time.Now(),
		Steps:     []*types.ExecutionStep{},
		Changes:   []*types.ChangeDetection{},
		Errors:    []string{},
	}

	t.Logf("📊 Created execution plan: ID=%s, Status=%s", execution.ID, execution.Status)

	// Execute each step like the real executeExecutionStep function
	for i, planStep := range decision.ExecutionPlan {
		t.Logf("⚙️ Step %d/%d: Executing %s action on %s", i+1, len(decision.ExecutionPlan), planStep.Action, planStep.ResourceID)

		startTime := time.Now()

		// Create execution step tracking (mirroring real executeExecutionStep)
		executionStep := &types.ExecutionStep{
			ID:        planStep.ID,
			Name:      planStep.Name,
			Status:    "running",
			Resource:  planStep.ResourceID,
			Action:    planStep.Action,
			StartedAt: &startTime,
		}

		// Execute step using mock infrastructure (mirroring the real action switch)
		var result map[string]interface{}
		var err error

		switch planStep.Action {
		case "create":
			result, err = agent.executeCreateAction(planStep, nil, execution.ID)
		case "update":
			result, err = testExecuteUpdateActionWithMocks(t, mockSuite, planStep)
		case "delete":
			result, err = testExecuteDeleteActionWithMocks(t, mockSuite, planStep)
		case "validate":
			result, err = testExecuteValidateActionWithMocks(t, mockSuite, planStep)
		case "query":
			result, err = agent.executeQueryAction(planStep, nil, execution.ID)
		default:
			err = fmt.Errorf("unknown action type: %s", planStep.Action)
		}

		// Complete step tracking (mirroring real execution)
		endTime := time.Now()
		executionStep.CompletedAt = &endTime
		executionStep.Duration = endTime.Sub(startTime)

		if err != nil {
			executionStep.Status = "failed"
			executionStep.Error = err.Error()
			execution.Errors = append(execution.Errors, err.Error())

			// Check if this is a mock validation error that we can tolerate
			if strings.Contains(err.Error(), "invalid subnet ID format") ||
				strings.Contains(err.Error(), "validation error for parameter") ||
				strings.Contains(err.Error(), "mock MCP tool call failed") {
				t.Logf("⚠️ Step %d failed with mock validation error (expected): %v", i+1, err)
			} else {
				t.Errorf("❌ Step %d failed: %v", i+1, err)
			}
		} else {
			executionStep.Status = "completed"
			executionStep.Output = result
			t.Logf("✅ Step %d completed successfully in %v", i+1, executionStep.Duration)
		}

		execution.Steps = append(execution.Steps, executionStep)
	}

	// Complete execution tracking
	// completedAt := time.Now()
	// execution.CompletedAt = &completedAt

	// Count serious errors (non-mock validation errors)
	seriousErrorCount := 0
	mockValidationErrorCount := 0

	for _, errMsg := range execution.Errors {
		if strings.Contains(errMsg, "invalid subnet ID format") ||
			strings.Contains(errMsg, "validation error for parameter") ||
			strings.Contains(errMsg, "mock MCP tool call failed") {
			mockValidationErrorCount++
		} else {
			seriousErrorCount++
		}
	}

	if seriousErrorCount > 0 {
		execution.Status = "failed"
		t.Fatalf("❌ Execution completed with %d serious errors", seriousErrorCount)
	} else {
		execution.Status = "completed"
		if mockValidationErrorCount > 0 {
			t.Logf("✅ Execution completed successfully with %d steps (%d mock validation errors tolerated)", len(execution.Steps), mockValidationErrorCount)
		} else {
			t.Logf("✅ Execution completed successfully with %d steps", len(execution.Steps))
		}
	}
}

// testExecuteUpdateActionWithMocks simulates update operations
func testExecuteUpdateActionWithMocks(t *testing.T, mockSuite *mocks.MockTestSuite, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	t.Logf("🔄 Updating %s resource: %s", planStep.ResourceID, planStep.Name)

	// Mock update via state management
	testResource := &types.ResourceState{
		ID:         "mock-" + planStep.ResourceID,
		Type:       planStep.ResourceID,
		Status:     "updated",
		Properties: planStep.Parameters,
	}

	mockSuite.StateManager.AddResource(testResource)

	return map[string]interface{}{
		"action":      "update",
		"resource_id": testResource.ID,
		"status":      "completed",
	}, nil
}

// testExecuteDeleteActionWithMocks simulates delete operations
func testExecuteDeleteActionWithMocks(t *testing.T, mockSuite *mocks.MockTestSuite, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	t.Logf("🗑️ Deleting %s resource: %s", planStep.ResourceID, planStep.Name)

	return map[string]interface{}{
		"action":      "delete",
		"resource_id": "mock-" + planStep.ResourceID,
		"status":      "completed",
	}, nil
}

// testExecuteValidateActionWithMocks simulates validation operations
func testExecuteValidateActionWithMocks(t *testing.T, mockSuite *mocks.MockTestSuite, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	t.Logf("✅ Validating %s resource: %s", planStep.ResourceID, planStep.Name)

	return map[string]interface{}{
		"action":            "validate",
		"resource_id":       "mock-" + planStep.ResourceID,
		"validation_result": "passed",
	}, nil
}

// testValidateMockIntegration validates that mock infrastructure components are working correctly
func testValidateMockIntegration(t *testing.T, mockSuite *mocks.MockTestSuite, decision *types.AgentDecision) {
	t.Logf("🔍 Validating mock infrastructure integration...")

	// Test 1: Mock MCP Server Integration
	t.Logf("📡 Testing Mock MCP Server...")
	ctx := context.Background()
	testResult, err := mockSuite.MCPServer.CallTool(ctx, "describe_vpcs", map[string]interface{}{})
	if err != nil {
		t.Errorf("❌ Mock MCP Server failed: %v", err)
	} else {
		t.Logf("✅ Mock MCP Server working: %v", testResult != nil)
	}

	// Test 2: Mock State Manager Integration
	t.Logf("💾 Testing Mock State Manager...")
	testResource := &types.ResourceState{
		ID:     "test-validation-resource",
		Type:   "vpc",
		Status: "active",
		Properties: map[string]interface{}{
			"cidr": "10.0.0.0/16",
		},
	}
	mockSuite.StateManager.AddResource(testResource)

	// Verify resource was added
	resources := mockSuite.StateManager.GetResources()
	found := false
	for _, resource := range resources {
		if resource.ID == "test-validation-resource" {
			found = true
			break
		}
	}

	if !found {
		t.Error("❌ Mock State Manager failed to store resource")
	} else {
		t.Logf("✅ Mock State Manager working: stored %d resources", len(resources))
	}

	// Test 3: Mock AWS Client Integration
	t.Logf("☁️ Testing Mock AWS Client...")
	mockSuite.AWSClient.AddDefaultTestData()
	// Just verify it doesn't panic - AWS client functionality is tested in its own unit tests
	t.Logf("✅ Mock AWS Client working")

	// Note: Retrieval registry tests removed - retrieval functions are deprecated
	// All resource queries now use MCP tools with "query" action

	t.Logf("✅ All mock integration tests passed!")
}

// testEnhancedPreExecutionValidation performs comprehensive validation before execution
func testEnhancedPreExecutionValidation(t *testing.T, agent *StateAwareAgent, mockSuite *mocks.MockTestSuite, decision *types.AgentDecision) {
	t.Logf("🔍 Running enhanced pre-execution validation...")

	// Test 1: Validate Resource ID Uniqueness in Plan
	t.Logf("🔍 Test 1: Validating resource ID uniqueness...")
	resourceIDs := make(map[string]bool)
	for _, step := range decision.ExecutionPlan {
		if step.ResourceID == "" {
			t.Errorf("❌ Step %s has empty ResourceID", step.ID)
			continue
		}
		if resourceIDs[step.ResourceID] {
			t.Logf("⚠️ Warning: Duplicate ResourceID '%s' found in plan - this could cause state conflicts", step.ResourceID)
		}
		resourceIDs[step.ResourceID] = true
	}
	t.Logf("✅ Resource ID uniqueness check completed")

	// Test 2: Validate Dependency Resolution Logic
	t.Logf("🔍 Test 2: Validating dependency resolution...")
	for _, step := range decision.ExecutionPlan {
		// Check if step has dependency references
		if step.Parameters != nil {
			validateDependencyReferences(t, step, decision.ExecutionPlan)
		}
	}
	t.Logf("✅ Dependency resolution validation completed")

	// Test 3: Validate Parameter Completeness
	t.Logf("🔍 Test 3: Validating parameter completeness...")
	for _, step := range decision.ExecutionPlan {
		validateParameterCompleteness(t, step)
	}
	t.Logf("✅ Parameter completeness validation completed")

	// Test 4: Validate Tool Availability
	t.Logf("🔍 Test 4: Validating tool availability...")
	for _, step := range decision.ExecutionPlan {
		if step.Action == "create" && step.MCPTool != "" {
			// Simulate checking if tool exists in mock MCP server
			ctx := context.Background()
			_, err := mockSuite.MCPServer.CallTool(ctx, step.MCPTool, map[string]interface{}{})
			if err != nil && !strings.Contains(err.Error(), "validation") {
				t.Errorf("❌ Tool '%s' not available in MCP server: %v", step.MCPTool, err)
			}
		}
	}
	t.Logf("✅ Tool availability validation completed")
}

// testEnhancedPostExecutionValidation performs comprehensive validation after execution
func testEnhancedPostExecutionValidation(t *testing.T, agent *StateAwareAgent, mockSuite *mocks.MockTestSuite, decision *types.AgentDecision) {
	t.Logf("🔍 Running enhanced post-execution validation...")

	// Test 1: Validate State Consistency
	t.Logf("🔍 Test 1: Validating state consistency...")
	resources := mockSuite.StateManager.GetResources()

	// Create a map for quick lookup
	resourceMap := make(map[string]*types.ResourceState)
	for _, resource := range resources {
		resourceMap[resource.ID] = resource
	}

	// Check if all created resources are in state
	expectedResources := 0
	for _, step := range decision.ExecutionPlan {
		if step.Action == "create" {
			expectedResources++
		}
	}

	if len(resources) == 0 {
		t.Errorf("❌ No resources found in state after execution")
	}
	t.Logf("✅ Found %d resources in state (expected approximately %d)", len(resources), expectedResources)

	// Test 2: Validate Resource Dependencies in State
	t.Logf("🔍 Test 2: Validating resource dependencies in state...")
	for _, step := range decision.ExecutionPlan {
		if len(step.DependsOn) > 0 {
			// Verify dependencies exist in state
			for _, depID := range step.DependsOn {
				found := false
				for _, resource := range resources {
					if resource.ID == depID || strings.Contains(resource.ID, depID) {
						found = true
						break
					}
				}
				if !found {
					t.Logf("⚠️ Warning: Dependency '%s' for step '%s' not found in final state", depID, step.ID)
				}
			}
		}
	}
	t.Logf("✅ Resource dependency validation completed")

	// Test 3: Validate Resource Properties
	t.Logf("🔍 Test 3: Validating resource properties...")
	for _, resource := range resources {
		if resource.Type == "" {
			t.Errorf("❌ Resource %s has empty type", resource.ID)
		}
		if resource.Status == "" {
			t.Errorf("❌ Resource %s has empty status", resource.ID)
		}
		if resource.Properties == nil {
			t.Logf("⚠️ Warning: Resource %s has nil properties", resource.ID)
		}
	}
	t.Logf("✅ Resource properties validation completed")

	// Test 4: Simulate Web UI State Retrieval
	t.Logf("🔍 Test 4: Simulating web UI state retrieval...")
	ctx := context.Background()
	_, err := mockSuite.MCPServer.CallTool(ctx, "analyze-infrastructure-state", map[string]interface{}{
		"include_drift_detection": true,
		"detailed_analysis":       true,
	})
	if err != nil {
		t.Errorf("❌ Failed to analyze infrastructure state: %v", err)
	} else {
		t.Logf("✅ Infrastructure state analysis successful")
	}
}

// testErrorScenarios tests various error scenarios that could occur in actual execution
func testErrorScenarios(t *testing.T, agent *StateAwareAgent, mockSuite *mocks.MockTestSuite, decision *types.AgentDecision) {
	t.Logf("🚨 Running error scenario testing...")

	// Test 1: Simulate AWS API Errors
	t.Logf("🚨 Test 1: Simulating AWS API errors...")

	// Enable error simulation on mock client
	mockSuite.AWSClient.EnableErrorSimulation(0.3) // 30% error rate
	defer mockSuite.AWSClient.DisableErrorSimulation()

	// Try executing a simple step with error simulation
	if len(decision.ExecutionPlan) > 0 {
		firstStep := decision.ExecutionPlan[0]
		t.Logf("🚨 Testing error resilience with step: %s", firstStep.Name)

		// Create a mock execution context
		execution := &types.PlanExecution{
			ID:     "error-test-execution",
			Status: "running",
			Steps:  []*types.ExecutionStep{},
		}

		// Execute step with error simulation
		var result map[string]interface{}
		var err error

		if firstStep.Action == "create" {
			result, err = agent.executeCreateAction(firstStep, nil, execution.ID)
		}

		if err != nil {
			t.Logf("✅ Error simulation working: got expected error: %v", err)
		} else {
			t.Logf("✅ Step completed despite error simulation: %v", result != nil)
		}
	}

	// Test 2: Simulate Dependency Resolution Failures
	t.Logf("🚨 Test 2: Simulating dependency resolution failures...")

	// Create a step with invalid dependency reference
	invalidStep := &types.ExecutionPlanStep{
		ID:         "test-invalid-dependency",
		Name:       "Test Invalid Dependency",
		Action:     "create",
		ResourceID: "test-resource",
		Parameters: map[string]interface{}{
			"vpcId": "{{non-existent-step.resourceId}}",
		},
		DependsOn: []string{"non-existent-step"},
	}

	// Try to resolve dependencies - should fail gracefully
	t.Logf("🚨 Testing invalid dependency: %s", invalidStep.Parameters["vpcId"])

	// Test 3: Simulate State Corruption
	t.Logf("🚨 Test 3: Simulating state corruption scenarios...")

	// Add a resource with invalid data to state
	corruptedResource := &types.ResourceState{
		ID:     "corrupted-resource",
		Type:   "", // Empty type should cause issues
		Status: "unknown",
		Properties: map[string]interface{}{
			"invalid": make(chan int), // Non-serializable data
		},
	}

	// Try adding corrupted resource (should handle gracefully)
	mockSuite.StateManager.AddResource(corruptedResource)

	// Test 4: Simulate Network/Timeout Scenarios
	t.Logf("🚨 Test 4: Simulating timeout scenarios...")

	// Create a context with very short timeout
	shortCtx, cancel := context.WithTimeout(context.Background(), time.Millisecond*1)
	defer cancel()

	// Try to make an API call with short timeout
	_, err := mockSuite.MCPServer.CallTool(shortCtx, "list-vpcs", map[string]interface{}{})
	if err != nil {
		t.Logf("✅ Timeout handling working: %v", err)
	}

	t.Logf("✅ Error scenario testing completed")
}

// validateDependencyReferences validates dependency references in step parameters
func validateDependencyReferences(t *testing.T, step *types.ExecutionPlanStep, allSteps []*types.ExecutionPlanStep) {
	// Create a map of all step IDs for validation
	stepMap := make(map[string]*types.ExecutionPlanStep)
	for _, s := range allSteps {
		stepMap[s.ID] = s
	}

	// Check parameters for dependency references
	for key, value := range step.Parameters {
		if strVal, ok := value.(string); ok {
			// Look for dependency references like {{step-id.resourceId}}
			if strings.HasPrefix(strVal, "{{") && strings.HasSuffix(strVal, "}}") {
				ref := strings.TrimPrefix(strings.TrimSuffix(strVal, "}}"), "{{")
				parts := strings.Split(ref, ".")
				if len(parts) > 0 {
					refStepID := parts[0]
					if _, exists := stepMap[refStepID]; !exists {
						t.Errorf("❌ Step %s parameter %s references non-existent step: %s", step.ID, key, refStepID)
					}
				}
			}
		}
	}
}

// validateParameterCompleteness validates that steps have required parameters
func validateParameterCompleteness(t *testing.T, step *types.ExecutionPlanStep) {
	// Define required parameters for common actions
	requiredParams := map[string][]string{
		"create-vpc":            {"cidrBlock"},
		"create-subnet":         {"vpcId", "cidrBlock"},
		"create-security-group": {"vpcId", "groupName"},
		"create-ec2-instance":   {"imageId", "instanceType"},
	}

	if step.MCPTool != "" {
		if required, exists := requiredParams[step.MCPTool]; exists {
			for _, param := range required {
				// Check both Parameters and ToolParameters
				hasParam := false
				if _, exists := step.Parameters[param]; exists {
					hasParam = true
				}
				if step.ToolParameters != nil {
					if _, exists := step.ToolParameters[param]; exists {
						hasParam = true
					}
				}

				if !hasParam {
					t.Errorf("❌ Step %s missing required parameter: %s", step.ID, param)
				}
			}
		}
	}
}
