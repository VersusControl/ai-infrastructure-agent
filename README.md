# AI Infrastructure Agent

> ⚠️ **Proof of Concept Project**: This repository contains a proof-of-concept implementation of an AI-powered infrastructure management agent. It is currently in active development and **not intended for production use**. We plan to release a production-ready version in the future. Use at your own risk and always test in development environments first.

<h1 align="center" style="border-bottom: none">
  <img alt="AI Infrastructure Agent" src="docs/images/ai-infrastructure-agent.svg" width="150" height="150">
</h1>

<div align="center">

[![Go Version](https://img.shields.io/badge/Go-1.24.2+-00ADD8?style=for-the-badge&logo=go)](https://golang.org/)
[![AWS](https://img.shields.io/badge/AWS-Cloud-FF9900?style=for-the-badge&logo=amazon-aws)](https://aws.amazon.com/)
[![MCP](https://img.shields.io/badge/Protocol-MCP-purple?style=for-the-badge)](https://modelcontextprotocol.io/)

*Intelligent AWS infrastructure management through natural language interactions*

</div>

## What is AI Infrastructure Agent?

AI Infrastructure Agent is an intelligent system that allows you to manage AWS infrastructure using natural language commands. Powered by advanced AI models (OpenAI GPT, Google Gemini, or Anthropic Claude), it translates your infrastructure requests into executable AWS operations while maintaining safety through conflict detection and resolution.

<h1 align="center" style="border-bottom: none">
  <img alt="Web Dashboard" src="docs/images/web-dashboard.svg">
</h1>

### Key Features

- **Natural Language Interface** - Describe what you want, not how to build it
- **Multi-AI Provider Support** - Choose between OpenAI, Google Gemini, Anthropic, or AWS Bedrock Nova
- **Web Dashboard** - Visual interface for infrastructure management, built-in conflict detection and dry-run mode
- **Terraform-like state** - Maintains accurate infrastructure state
- **Current Resource Support** - VPC, EC2, SG, Autoscaling Group, ALB. Check the roadmap here: [Core Platform Development](https://github.com/orgs/VersusControl/projects/19)

## Example Usage

Imagine you want to create AWS infrastructure with a simple request:

> **"Create an EC2 instance for hosting an Apache Server with a dedicated security group that allows inbound HTTP (port 80) and SSH (port 22) traffic."**

> 💡 **Amazon Nova Users**: When using AWS Bedrock Nova models, you may want to specify the region in your request for better context, e.g., *"Create an EC2 instance in us-east-1 for hosting an Apache Server..."*

Here's what happens:

### 1. AI Analysis & Planning

The AI agent analyzes your request and creates a detailed execution plan:

```mermaid
sequenceDiagram
    participant U as User
    participant A as AI Agent
    participant S as State Manager
    participant M as MCP Server
    participant AWS as AWS APIs
    
    U->>A: "Create EC2 instance for Apache Server..."
    A->>S: Get current infrastructure state
    S->>A: Return current state
    A->>M: Query available tools & capabilities
    M->>A: Return tool capabilities
    A->>A: Generate execution plan with LLM
    A->>AWS: Validate plan (dry-run checks)
    AWS->>A: Validation results
    A->>U: Present execution plan for approval
    
    Note over A,U: Plan includes:<br/>• Get Default VPC<br/>• Create Security Group<br/>• Add HTTP & SSH rules<br/>• Get Latest AMI<br/>• Create EC2 Instance
```

The agent presents the plan for your review:
- Shows exactly what will be created
- Waits for your approval

<h1 align="center" style="border-bottom: none">
  <img alt="Execution & Monitoring" src="docs/images/simple-demo.png" width="650">
</h1>

### 2. Execution & Monitoring

Once approved, the agent:
- Creates resources in the correct order
- Monitors progress in real-time
- Handles dependencies automatically
- Reports completion status

Check <a href="https://www.youtube.com/watch?v=41tupPs-BEU]" target="_blank">Live Demo</a>

### 3. More Examples

- Quick Tutorial: **[AI Infrastructure Agent for AWS](https://github.com/VersusControl/devops-ai-guidelines/blob/main/resources/ai-infrastructure-agent-for-aws.md)**
- Series Tutorial: **[Building Your Business on AWS with AI Agent](https://github.com/VersusControl/devops-ai-guidelines/blob/main/04-ai-agent-for-aws/00-toc.md)**

## How To Run

Detailed Guides: [Installation Guide](https://ai-agent.devopsvn.tech/docs.html#/installation)

### Clone the repository

```bash
git clone https://github.com/VersusControl/ai-infrastructure-agent.git
cd ai-infrastructure-agent
```

### 1. Edit Configuration File

```bash
# Edit the main configuration
nano config.yaml
```

### 2. Set Your AI Provider

Choose your preferred AI provider in `config.yaml`:

```yaml
agent:
  provider: "openai"          # Options: openai, gemini, anthropic, bedrock
  model: "gpt-4"             # Model to use
  max_tokens: 4000
  temperature: 0.1
  dry_run: true              # Start with dry-run enabled
  auto_resolve_conflicts: false
```

### 3. Set Environment Variables

**Detailed Setup Guides:**
- **OpenAI**: [OpenAI API Key Setup Guide](https://ai-agent.devopsvn.tech/docs.html#/api-key-setup/openai-api-setup)
- **Google Gemini**: [Gemini API Key Setup Guide](https://ai-agent.devopsvn.tech/docs.html#/api-key-setup/gemini-api-setup)
- **AWS Bedrock Nova**: [AWS Bedrock Nova Configuration Guide](https://ai-agent.devopsvn.tech/docs.html#/api-key-setup/aws-bedrock-nova-setup)

```bash
# For OpenAI
export OPENAI_API_KEY="your-openai-api-key"

# For Google Gemini
export GEMINI_API_KEY="your-gemini-api-key"

# For AWS Bedrock Nova - use AWS credentials (no API key needed)
# Configure AWS credentials using: aws configure, environment variables, or IAM roles
```

### 4. Configure AWS Credentials

```bash
# Configure AWS CLI
aws configure

# Or set environment variables
export AWS_ACCESS_KEY_ID="your-access-key"
export AWS_SECRET_ACCESS_KEY="your-secret-key"
export AWS_DEFAULT_REGION="us-west-2"
```

## Quick Installation

### Method 1: Docker Installation

Basic Docker Run:

```bash
docker run -d \
  --name ai-infrastructure-agent \
  -p 8080:8080 \
  -v $(pwd)/config.yaml:/app/config.yaml:ro \
  -v $(pwd)/states:/app/states \
  -e OPENAI_API_KEY="your-openai-api-key-here" \
  -e AWS_ACCESS_KEY_ID="your-aws-access-key" \
  -e AWS_SECRET_ACCESS_KEY="your-aws-secret-key" \
  -e AWS_DEFAULT_REGION="us-west-2" \
  ghcr.io/versuscontrol/ai-infrastructure-agent
```

Docker Compose (Recommended). Create a `docker-compose.yml` file:

```yaml
version: '3.8'

services:
  ai-infrastructure-agent:
    image: ghcr.io/versuscontrol/ai-infrastructure-agent
    container_name: ai-infrastructure-agent
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      # Mount configuration file (read-only)
      - ./config.yaml:/app/config.yaml:ro
      # Mount data directories (persistent)
      - ./states:/app/states
    environment:
      # AI Provider API Keys (choose one)
      - OPENAI_API_KEY=${OPENAI_API_KEY}
      # - GEMINI_API_KEY=${GEMINI_API_KEY}
      # - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
      
      # AWS Configuration
      - AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID}
      - AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY}
      - AWS_DEFAULT_REGION=${AWS_DEFAULT_REGION:-us-west-2}
```

Start the application:

```bash
# Start with Docker Compose
docker-compose up -d

# View logs
docker-compose logs -f

# Stop the application
docker-compose down
```

### Method 2: Automated Bash Script

```bash
# Clone the repository
git clone https://github.com/VersusControl/ai-infrastructure-agent.git
cd ai-infrastructure-agent

# Run the installation script
./scripts/install.sh
```

Start the Web UI:

```bash
./scripts/run-web-ui.sh
```

### Access the Dashboard

Open your browser and navigate to:
```
http://localhost:8080
```

## Usage Examples

```bash
# Simple EC2 instance
"Create a t3.micro EC2 instance with Ubuntu 22.04"

# Web server setup
"Deploy a load-balanced web application with 2 EC2 instances behind an ALB"

# Database setup
"Create an RDS MySQL database with read replicas in multiple AZs"

# Complete environment
"Set up a development environment with VPC, subnets, EC2, and RDS"
```

## Architecture

<h1 align="center" style="border-bottom: none">
  <img alt="Web Dashboard" src="docs/images/core-components.svg">
</h1>

Read detail: [Technical Architecture Overview](https://ai-agent.devopsvn.tech/docs.html#/architecture/architecture-overview)

### Components

- **Web Interface**: React-based dashboard for visual interaction
- **MCP Server**: Core agent implementing Model Context Protocol
- **Agent Core**: AI-powered decision making and planning
- **AWS Client**: Secure AWS SDK integration
- **State Management**: Infrastructure state tracking and conflict resolution

## Safety Features

### Dry Run Mode
All operations can be run in "dry-run" mode first:
- Shows exactly what would be created/modified/deleted
- Estimates costs before execution
- No actual AWS resources are touched

### State Management
- Maintains accurate infrastructure state
- Detects drift from expected configuration

### Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feature-name`
3. Make your changes
3. Run tests
4. Commit: `git commit -m "Add feature"`
5. Push: `git push origin feature-name`
6. Create a Pull Request

## Documentation

- [AI Infrastructure Agent documentation](https://ai-agent.devopsvn.tech/docs.html#/)

## Troubleshooting

### Common Issues

<details>
<summary><strong>AWS Authentication Issues</strong></summary>

```bash
# Check AWS credentials
aws sts get-caller-identity

# Verify permissions
aws iam get-user

# Test basic AWS access
aws ec2 describe-regions
```

</details>

<details>
<summary><strong>AI Provider API Issues</strong></summary>

```bash
# Check API key is set
echo $OPENAI_API_KEY

# Test API connection
curl -H "Authorization: Bearer $OPENAI_API_KEY" \
     https://api.openai.com/v1/models
```

</details>

<details>
<summary><strong>Port Already in Use</strong></summary>

```bash
# Check what's using the port
lsof -i :8080
lsof -i :3000

# Kill processes if needed
kill -9 <pid>

# Or change ports in config.yaml
```

</details>

<details>
<summary><strong>Go Build Issues</strong></summary>

```bash
# Clean module cache
go clean -modcache

# Re-download dependencies
go mod download
go mod tidy

# Rebuild
go build ./...
```

</details>

<details>
<summary><strong>Decision validation failed: decision confidence too low: 0.000000</strong></summary>

Try increase max_tokens:

```yaml
agent:
  provider: "gemini"              # Use Google AI (Gemini)
  model: "gemini-2.5-flash-lite"
  max_tokens: 10000 # <-- increase
```

</details>

## Security Considerations

- **API Keys**: Never commit API keys to version control
- **AWS Permissions**: Use least-privilege IAM policies
- **Network Security**: Run in private networks when possible
- **Audit Logging**: Enable comprehensive logging for compliance
- **Dry Run**: Always test in dry-run mode first

## Roadmap

### Current Version (v0.0.2 - PoC)
- ✅ Basic natural language processing
- ✅ Core AWS resource management
- ✅ Web dashboard
- ✅ MCP protocol support
- ✅ ReAct-Style Agent

### Upcoming Version (v0.0.3 - PoC)
- 🔄 Better UX/UI

### Upcoming Features (v0.1.*)
- 🔄 Cost optimization recommendations
- 🔄 Enhanced conflict resolution
- 🔄 Infrastructure templates
- 🔄 Multi States
- 🔄 Role-based access control

## 🤝 Community & Support

- **GitHub Issues**: [Report bugs and request features](https://github.com/VersusControl/ai-infrastructure-agent/issues)
- **Discussions**: [Community discussions](https://github.com/VersusControl/ai-infrastructure-agent/discussions)

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## ⚖️ Disclaimer

This is a proof-of-concept project. While we've implemented safety measures like dry-run mode and conflict detection, always:

- Test in development environments first
- Review all generated plans before execution
- Maintain proper AWS IAM permissions
- Monitor costs and resource usage
- Keep backups of critical infrastructure

The authors are not responsible for any costs, data loss, or security issues that may arise from using this software.

---

<div align="center">

**Built with ❤️ by the DevOps VN Team**

*Empowering infrastructure management through AI*

[⭐ Star this repo](https://github.com/VersusControl/ai-infrastructure-agent) | [🐛 Report Bug](https://github.com/VersusControl/ai-infrastructure-agent/issues) | [💡 Request Feature](https://github.com/VersusControl/ai-infrastructure-agent/issues)

</div>
