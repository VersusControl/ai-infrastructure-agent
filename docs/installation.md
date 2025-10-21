# Installation Guide

This comprehensive installation guide provides two methods to install and run the AI Infrastructure Agent: using the automated Bash script or Docker containerization.

## Prerequisites

Before installing, ensure you have:

- **AWS Account** with appropriate IAM permissions
- **AI Provider API Key** (choose one):
  - OpenAI API Key (recommended for beginners)
  - Google Gemini API Key
  - AWS Bedrock Nova (uses AWS credentials)
- **Docker** (for Docker installation method)
- **Go 1.24.2+** (for Bash script method only)

## Method 1: Automated Bash Script Installation

The automated bash script installation is the **recommended method** for development and local testing.

### Step 1: Clone the Repository

```bash
# Clone the repository
git clone https://github.com/VersusControl/ai-infrastructure-agent.git

cd ai-infrastructure-agent
```

### Step 2: Run the Installation Script

```bash
# Make the script executable and run it
chmod +x scripts/install.sh

./scripts/install.sh
```

The installation script will automatically:
- Check and install Go 1.24.2+ (if needed)
- Setup AWS CLI (if not present)
- Create necessary directories (`bin`, `states`, `backups`)
- Download Go dependencies
- Create configuration files
- Build both the web application and MCP server

### Step 3: Configure Your Environment

#### 3.1 Choose and Configure Your AI Provider

**Option A: OpenAI (Recommended)**
```bash
# Set your API key
export OPENAI_API_KEY="sk-your-openai-api-key-here"

# Update config.yaml
cp config.openai.yaml.example config.yaml
```

**Option B: Google Gemini**
```bash
# Set your API key  
export GEMINI_API_KEY="your-gemini-api-key-here"

# Update config.yaml
cp config.gemini.yaml.example config.yaml
```

**Option C: AWS Bedrock Nova**
```bash
# No API key needed - uses AWS credentials
cp config.bedrock.yaml.example config.yaml
```

#### 3.2 Configure AWS Credentials

```bash
# Configure AWS CLI (interactive)
aws configure

# Or set environment variables
export AWS_ACCESS_KEY_ID="your-access-key-id"
export AWS_SECRET_ACCESS_KEY="your-secret-access-key"
export AWS_DEFAULT_REGION="us-west-2"
```

### Step 4: Start the Application

```bash
# Start the Web UI
./scripts/run-web-ui.sh
```

The web interface will be available at: **http://localhost:8080**

## Method 2: Docker Installation

Docker installation is perfect for **production deployments** and **containerized environments**.

### Step 1: Create Configuration and Data Directory

```bash
# Create subdirectories for data persistence  
mkdir -p states
```

### Step 2: Create Configuration File

Create a `config.yaml` file in the **root directory** (not in a config subdirectory):

```bash
cat > config.yaml << 'EOF'
# AI Infrastructure Agent Configuration
server:
  port: 3000
  host: "0.0.0.0"

aws:
  region: "us-west-2"  # Change to your preferred region

mcp:
  server_name: "ai-infrastructure-agent"
  version: "1.0.0"

agent:
  provider: "openai"          # openai, gemini, anthropic, bedrock, ollama
  model: "gpt-4o-mini"
  max_tokens: 4000
  temperature: 0.1
  dry_run: false              
  auto_resolve_conflicts: false

logging:
  level: "info"
  format: "json"
  output: "stdout"

state:
  file_path: "./states/infrastructure-state.json"
  backup_enabled: true
  backup_dir: "./backups"

web:
  port: 8080
  host: "0.0.0.0"
  template_dir: "web/templates"
  static_dir: "web/static"
  enable_websockets: true
EOF
```

### Step 4: Run with Docker

#### Basic Docker Run

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

#### Docker Compose (Recommended)

Create a `docker-compose.yml` file:

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

### Step 5: Verify Installation

Open your browser and navigate to: **http://localhost:8080**

You should see the AI Infrastructure Agent dashboard.

### Docker Volume Explanation

The Docker setup uses minimal volumes for data persistence:

- **`./config.yaml:/app/config.yaml:ro`** - Configuration file (read-only)
- **`./states:/app/states`** - Infrastructure state files (persistent)

**Note**: The `states/` directory is automatically created by the application if it doesn't exist. Logs are written to stdout/stderr and handled by Docker's logging system. Backups are stored within the `states/` directory structure.

This ensures your data persists across container restarts and updates.

## Troubleshooting

### Common Issues

#### 1. "Go version too old" (Bash Script Method)

```bash
# Update Go to 1.24.2+
# macOS with Homebrew:
brew install go

# Linux:
# Download from https://golang.org/dl/
```

#### 2. "AWS credentials not found"

```bash
# Check credentials configuration
aws configure list

# Test credentials
aws sts get-caller-identity

# For Docker, verify environment variables
docker exec ai-infrastructure-agent env | grep AWS
```

#### 3. "API key not working"

```bash
# Verify API key is set correctly
echo $OPENAI_API_KEY

# For Docker, check container environment
docker exec ai-infrastructure-agent env | grep OPENAI_API_KEY
```

#### 4. "Port already in use"

```bash
# Find what's using port 8080
lsof -i :8080

# Change port in config.yaml or docker-compose.yml
# Example: Change port to 8081
```

## Security Best Practices

### 1. Environment Variables

- Never commit API keys to version control
- Consider using AWS Secrets Manager for production

### 2. AWS IAM Permissions

Create a dedicated IAM user with minimal required permissions:

```json
{
  "Version": "2012-10-17", 
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ec2:*",
        "vpc:*",
        "iam:PassRole",
        "elasticloadbalancing:*",
        "autoscaling:*"
      ],
      "Resource": "*"
    }
  ]
}
```

### 3. Network Security

- Run behind a reverse proxy in production
- Use HTTPS in production environments
- Consider firewall rules for API access

## Next Steps

After successful installation:

1. **Try Examples**: Start with simple infrastructure requests
2. **Architecture**: Understand the system with [Architecture Overview](docs/architecture/architecture-overview.md)
3. **Join Community**: Get help and share experiences on [GitHub Discussions](https://github.com/VersusControl/ai-infrastructure-agent/discussions)
