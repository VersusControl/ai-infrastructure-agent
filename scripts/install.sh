#!/bin/bash

# AI Infrastructure Agent Installation Script
# This script sets up the environment and builds the AI Infrastructure Agent

set -e  # Exit on any error

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_header() {
    echo -e "${PURPLE}================================${NC}"
    echo -e "${PURPLE}$1${NC}"
    echo -e "${PURPLE}================================${NC}"
}

# Function to check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Function to get OS type
get_os() {
    if [[ "$OSTYPE" == "linux-gnu"* ]]; then
        echo "linux"
    elif [[ "$OSTYPE" == "darwin"* ]]; then
        echo "macos"
    elif [[ "$OSTYPE" == "cygwin" ]] || [[ "$OSTYPE" == "msys" ]] || [[ "$OSTYPE" == "win32" ]]; then
        echo "windows"
    else
        echo "unknown"
    fi
}

# Function to install Go on different platforms
install_go() {
    local os=$(get_os)
    local go_version="1.24.2"
    
    print_status "Installing Go $go_version..."
    
    case $os in
        "macos")
            if command_exists brew; then
                brew install go
            else
                print_warning "Homebrew not found. Please install Go manually from https://golang.org/dl/"
                print_warning "Required Go version: $go_version or higher"
                return 1
            fi
            ;;
        "linux")
            # Check if we can use package manager
            if command_exists apt-get; then
                sudo apt-get update
                sudo apt-get install -y golang-go
            elif command_exists yum; then
                sudo yum install -y golang
            elif command_exists dnf; then
                sudo dnf install -y golang
            else
                print_warning "Package manager not found. Please install Go manually from https://golang.org/dl/"
                print_warning "Required Go version: $go_version or higher"
                return 1
            fi
            ;;
        *)
            print_warning "Unsupported OS. Please install Go manually from https://golang.org/dl/"
            print_warning "Required Go version: $go_version or higher"
            return 1
            ;;
    esac
}

# Function to check Go version
check_go_version() {
    if ! command_exists go; then
        return 1
    fi
    
    local current_version=$(go version | grep -o 'go[0-9]\+\.[0-9]\+\.[0-9]\+' | sed 's/go//')
    local required_version="1.24.2"
    
    # Simple version comparison (works for most cases)
    if [[ "$(printf '%s\n' "$required_version" "$current_version" | sort -V | head -n1)" == "$required_version" ]]; then
        return 0
    else
        return 1
    fi
}

# Function to setup AWS CLI if not present
setup_aws_cli() {
    print_status "Checking AWS CLI installation..."
    
    if ! command_exists aws; then
        print_warning "AWS CLI not found. Installing..."
        local os=$(get_os)
        
        case $os in
            "macos")
                if command_exists brew; then
                    brew install awscli
                else
                    print_warning "Please install AWS CLI manually: https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html"
                fi
                ;;
            "linux")
                # Install AWS CLI v2
                curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip"
                if command_exists unzip; then
                    unzip awscliv2.zip
                    sudo ./aws/install
                    rm -rf awscliv2.zip aws/
                else
                    print_warning "Please install unzip and run this script again, or install AWS CLI manually"
                    return 1
                fi
                ;;
            *)
                print_warning "Please install AWS CLI manually: https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html"
                ;;
        esac
    else
        print_success "AWS CLI found: $(aws --version)"
    fi
}

# Function to create directories
create_directories() {
    print_status "Creating necessary directories..."
    
    mkdir -p bin
    mkdir -p states
    mkdir -p backups
    
    print_success "Directories created"
}

# Function to setup configuration
setup_config() {
    print_status "Setting up configuration..."
    
    if [[ ! -f "config.yaml" ]]; then
        if [[ -f "config.openai.yaml.example" ]]; then
            print_status "Copying example configuration..."
            cp config.openai.yaml.example config.yaml
            print_warning "Please edit config.yaml to set your AWS region and API keys"
        else
            print_status "Creating default configuration..."
            cat > config.yaml << 'EOF'
# AI Infrastructure Agent Configuration
server:
  port: 3000
  host: "localhost"

aws:
  region: "us-west-2"

mcp:
  server_name: "aws-infrastructure-server"
  version: "1.0.0"

agent:
  provider: "openai"          # openai, gemini, anthropic
  model: "gpt-4"
  max_tokens: 4000
  temperature: 0.1
  dry_run: true
  auto_resolve_conflicts: false
  # Note: Set API keys via environment variables:
  # OPENAI_API_KEY for OpenAI
  # GEMINI_API_KEY for Google AI (Gemini)
  # ANTHROPIC_API_KEY for Anthropic

logging:
  level: "info"
  format: "text"
  output: "stdout"

state:
  file_path: "./states/infrastructure-state.json"
  backup_enabled: true
  backup_dir: "./backups"

web:
  port: 8080
  host: "localhost"
  template_dir: "web/templates"
  static_dir: "web/static"
  enable_websockets: true
EOF
        fi
        print_success "Configuration file created: config.yaml"
    else
        print_success "Configuration file already exists: config.yaml"
    fi
}

# Function to build the application
build_application() {
    print_status "Building AI Infrastructure Agent..."
    
    # Ensure we have Go modules
    if [[ ! -f "go.mod" ]]; then
        print_error "go.mod not found. Please run this script from the project root directory."
        exit 1
    fi
    
    # Download dependencies
    print_status "Downloading Go dependencies..."
    go mod download
    go mod tidy
    
    # Make scripts executable
    if [[ -f "scripts/run-web-ui.sh" ]]; then
        chmod +x scripts/run-web-ui.sh
        print_success "Made scripts/run-web-ui.sh executable"
    fi
}

# Function to print usage instructions
print_usage() {
    print_header "Installation Complete!"
    
    echo ""
    echo -e "${CYAN}🎉 AI Infrastructure Agent has been successfully installed!${NC}"
    echo ""
    echo -e "${YELLOW}Next Steps:${NC}"
    echo ""
    echo "1. 📝 Configure your settings:"
    echo "   - Edit config.yaml to set your AWS region and preferences"
    echo "   - Set environment variables for your AI provider:"
    echo "     export OPENAI_API_KEY='your-key-here'     # For OpenAI"
    echo "     export GEMINI_API_KEY='your-key-here'     # For Google Gemini"
    echo "     export ANTHROPIC_API_KEY='your-key-here'  # For Anthropic"
    echo ""
    echo "2. 🔐 Configure AWS credentials:"
    echo "   aws configure"
    echo ""
    echo "   # Start Web UI:"
    echo "   ./scripts/run-web-ui.sh"
    echo ""
    echo "4. 🌐 Access the Web UI:"
    echo "   Open http://localhost:8080 in your browser"
    echo ""
    echo -e "${YELLOW}Troubleshooting:${NC}"
    echo "- Ensure your AWS credentials are properly configured"
    echo "- Verify your AI provider API key is set correctly"
    echo "- Check that required ports (3000, 8080) are available"
    echo ""
    echo -e "${CYAN}For more information, check the README.md file${NC}"
}

# Main installation function
main() {
    print_header "AI Infrastructure Agent Installation"
    
    # Check if we're in the right directory
    if [[ ! -f "go.mod" ]] || [[ ! -d "cmd" ]]; then
        print_error "Please run this script from the AI Infrastructure Agent root directory"
        exit 1
    fi
    
    print_status "Starting installation process..."
    
    # Check and install Go
    if ! check_go_version; then
        print_warning "Go 1.24.2+ not found or version too old"
        read -p "Would you like to install/update Go? (y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            install_go
        else
            print_error "Go 1.24.2+ is required. Please install it manually and run this script again."
            exit 1
        fi
    else
        print_success "Go version check passed: $(go version)"
    fi
    
    # Setup AWS CLI
    setup_aws_cli
    
    # Create necessary directories
    create_directories
    
    # Setup configuration
    setup_config
    
    # Build the application
    build_application
    
    # Print usage instructions
    print_usage
}

# Handle command line arguments
case "${1:-}" in
    --help|-h)
        echo "AI Infrastructure Agent Installation Script"
        echo ""
        echo "Usage: $0 [options]"
        echo ""
        echo "Options:"
        echo "  --help, -h     Show this help message"
        echo "  --build-only   Only build the application (skip dependency checks)"
        echo "  --deps-only    Only install dependencies (skip building)"
        echo ""
        echo "This script will:"
        echo "- Check and install Go 1.24.2+"
        echo "- Setup AWS CLI"
        echo "- Create necessary directories"
        echo "- Setup configuration files"
        echo "- Build the MCP server and Web UI"
        echo "- Create launcher scripts"
        exit 0
        ;;
    --build-only)
        print_header "Building AI Infrastructure Agent"
        build_application
        print_success "Build complete!"
        ;;
    --deps-only)
        print_header "Installing Dependencies"
        if ! check_go_version; then
            install_go
        fi
        setup_aws_cli
        create_directories
        setup_config
        print_success "Dependencies setup complete!"
        ;;
    *)
        main
        ;;
esac
