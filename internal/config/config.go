package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Config is the main configuration structure
type Config struct {
	AWS     AWSConfig     `mapstructure:"aws"`
	MCP     MCPConfig     `mapstructure:"mcp"`
	Agent   AgentConfig   `mapstructure:"agent"`
	Logging LoggingConfig `mapstructure:"logging"`
	State   StateConfig   `mapstructure:"state"`
	Web     WebConfig     `mapstructure:"web"`
}

// AWSConfig contains AWS-specific configuration
type AWSConfig struct {
	Region string `mapstructure:"region"`
}

// MCPConfig contains Model Context Protocol configuration
type MCPConfig struct {
	ServerName string `mapstructure:"server_name"`
	Version    string `mapstructure:"version"`
}

// AgentConfig contains configuration for the AI agent
type AgentConfig struct {
	Provider             string  `mapstructure:"provider"` // openai, gemini, anthropic
	OpenAIAPIKey         string  `mapstructure:"openai_api_key"`
	GeminiAPIKey         string  `mapstructure:"gemini_api_key"`
	AnthropicAPIKey      string  `mapstructure:"anthropic_api_key"`
	Model                string  `mapstructure:"model"`
	MaxTokens            int     `mapstructure:"max_tokens"`
	Temperature          float64 `mapstructure:"temperature"`
	DryRun               bool    `mapstructure:"dry_run"`
	AutoResolveConflicts bool    `mapstructure:"auto_resolve_conflicts"`
	EnableDebug          bool    `mapstructure:"enable_debug"`
	StepDelayMS          int     `mapstructure:"step_delay_ms"` // Delay after successful step execution (for smooth frontend display)
}

// LoggingConfig contains logging configuration
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
	Output string `mapstructure:"output"`
}

// StateConfig contains state management configuration
type StateConfig struct {
	FilePath      string `mapstructure:"file_path"`
	BackupEnabled bool   `mapstructure:"backup_enabled"`
	BackupDir     string `mapstructure:"backup_dir"`
}

// WebConfig contains web server configuration
type WebConfig struct {
	Port             int    `mapstructure:"port"`
	Host             string `mapstructure:"host"`
	TemplateDir      string `mapstructure:"template_dir"`
	StaticDir        string `mapstructure:"static_dir"`
	EnableWebSockets bool   `mapstructure:"enable_websockets"`
}

// Load loads configuration from file, environment variables, and defaults
func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")
	viper.AddConfigPath("$HOME/.aiops")

	// Environment variable support
	viper.SetEnvPrefix("AIOPS")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Set defaults
	setDefaults()

	// Try to read config file (optional)
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
		// Config file not found is OK, we'll use defaults and env vars
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("unable to decode config: %w", err)
	}

	// Override with environment variables for sensitive data
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		config.Agent.OpenAIAPIKey = apiKey
	}
	if apiKey := os.Getenv("GEMINI_API_KEY"); apiKey != "" {
		config.Agent.GeminiAPIKey = apiKey
	}
	if apiKey := os.Getenv("ANTHROPIC_API_KEY"); apiKey != "" {
		config.Agent.AnthropicAPIKey = apiKey
	}
	if awsRegion := os.Getenv("AWS_REGION"); awsRegion != "" {
		config.AWS.Region = awsRegion
	}

	return &config, nil
}

// setDefaults sets default configuration values
func setDefaults() {
	// AWS defaults
	viper.SetDefault("aws.region", "us-west-2")

	// MCP defaults
	viper.SetDefault("mcp.server_name", "ai-infrastructure-agent")
	viper.SetDefault("mcp.version", "1.0.0")

	// Agent defaults
	viper.SetDefault("agent.provider", "openai")
	viper.SetDefault("agent.model", "gpt-4")
	viper.SetDefault("agent.max_tokens", 4000)
	viper.SetDefault("agent.temperature", 0.3)
	viper.SetDefault("agent.dry_run", true)
	viper.SetDefault("agent.auto_resolve_conflicts", false)
	viper.SetDefault("agent.enable_debug", false)
	viper.SetDefault("agent.step_delay_ms", 500)

	// Logging defaults
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "text")
	viper.SetDefault("logging.output", "stdout")

	// State defaults
	viper.SetDefault("state.file_path", "infrastructure.state")
	viper.SetDefault("state.backup_enabled", true)
	viper.SetDefault("state.backup_dir", "./backups")

	// Web defaults
	viper.SetDefault("web.port", 8080)
	viper.SetDefault("web.host", "localhost")
	viper.SetDefault("web.template_dir", "web/templates")
	viper.SetDefault("web.static_dir", "web/static")
	viper.SetDefault("web.enable_websockets", true)
}

// GetStateFilePath returns the full path to the state file
func (c *Config) GetStateFilePath() string {
	return c.State.FilePath
}

// GetWebPort returns the web server port (fallback to server port if not set)
func (c *Config) GetWebPort() int {
	return c.Web.Port
}

// IsProductionMode returns true if running in production mode
func (c *Config) IsProductionMode() bool {
	return c.Logging.Level != "debug"
}
