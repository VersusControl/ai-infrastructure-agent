package agent

import (
	"bufio"
	"context"
	"os/exec"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/versus-control/ai-infrastructure-agent/internal/config"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/agent/resources"
	"github.com/versus-control/ai-infrastructure-agent/pkg/aws"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"

	"github.com/tmc/langchaingo/llms"
)

// ========== Interface defines ==========

// AgentTypesInterface defines all data structures and types used by the AI agent system
//
// Available Types:
//   - MCPProcess                      : Running MCP server process representation
//   - StateAwareAgent                 : Main agent struct with LLM, config, and MCP capabilities
//   - MCPToolInfo                     : Information about available MCP tool capabilities
//   - MCPResourceInfo                 : Information about available MCP resources
//   - DecisionContext                 : Context data for agent decision-making process
//   - ResourceMatch                   : Matched resource from correlation analysis
//   - PlanRecoveryCoordinator         : Interface for coordinating recovery between steps
//   - PlanFailureContext              : Context information about plan execution failures
//   - CompletedStepInfo               : Information about successfully completed steps
//   - RecoveryAttemptHistory          : History of recovery attempts for a failure
//   - PlanRecoveryStrategy            : Strategy for recovering from plan failures
//   - RollbackStepInfo                : Information about rollback steps
//   - AIPlanRecoveryAnalysis          : AI-generated analysis of plan failure and recovery
//   - PlanRecoveryResult              : Result of plan recovery attempt
//   - PlanRecoveryEngine              : Interface for plan-level recovery engine
//   - PlanRecoveryAwareAgent          : Interface for agent with recovery capabilities
//   - PlanRecoveryConfig              : Configuration for recovery behavior
//   - AIResponse                      : Generic AI response structure
//
// Key Features:
//   - MCP Server Integration          : Direct communication with Model Context Protocol servers
//   - Multi-LLM Support              : OpenAI, Google AI, Anthropic, AWS Bedrock, Ollama
//   - Resource Management            : Track mappings between plan steps and AWS resource IDs
//   - ReAct Plan Recovery            : AI-powered recovery from execution failures
//   - Thread-Safe Operations         : Mutex protection for concurrent access
//
// Usage Example:
//   agent := &StateAwareAgent{...}      // Main agent instance
//   context := &DecisionContext{...}    // For decision making
//   engine := NewPlanRecoveryEngine()   // For failure recovery

// ========== Agent Type Definitions ==========

// MCPProcess represents a running MCP server process
type MCPProcess struct {
	cmd    *exec.Cmd
	stdin  *bufio.Writer
	stdout *bufio.Scanner
	mutex  sync.Mutex
	reqID  int64
}

// StateAwareAgent represents an AI agent with state management capabilities
type StateAwareAgent struct {
	llm       llms.Model
	config    *config.AgentConfig
	awsConfig *config.AWSConfig
	awsClient *aws.Client
	Logger    *logging.Logger

	// MCP properties
	mcpProcess       *MCPProcess
	resourceMappings map[string]string
	mappingsMutex    sync.RWMutex
	mcpTools         map[string]MCPToolInfo
	mcpResources     map[string]MCPResourceInfo
	capabilityMutex  sync.RWMutex

	// Configuration-driven components
	fieldResolver  *resources.FieldResolver
	patternMatcher *resources.PatternMatcher

	// Extractor for resource identification
	extractionConfig *config.ResourceExtractionConfig
	idExtractor      *resources.IDExtractor

	// Test mode flag to bypass real MCP server startup
	testMode bool

	// Mock MCP server for testing (only used when testMode is true)
	mockMCPServer interface {
		CallTool(ctx context.Context, toolName string, arguments map[string]interface{}) (*mcp.CallToolResult, error)
	}
}

// MCPToolInfo represents information about an available MCP tool
type MCPToolInfo struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
	Type        string                 `json:"type"`
}

// MCPResourceInfo represents information about an available MCP resource
type MCPResourceInfo struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description"`
	MimeType    string `json:"mimeType"`
}

// DecisionContext contains context for agent decision-making
type DecisionContext struct {
	Request             string                      `json:"request"`
	CurrentState        *types.InfrastructureState  `json:"current_state"`
	DiscoveredState     []*types.ResourceState      `json:"discovered_state"`
	Conflicts           []*types.ConflictResolution `json:"conflicts"`
	DependencyGraph     *types.DependencyGraph      `json:"dependency_graph"`
	DeploymentOrder     []string                    `json:"deployment_order"`
	ResourceCorrelation map[string]*ResourceMatch   `json:"resource_correlation"`
}

// ResourceMatch represents correlation between managed and discovered resources
type ResourceMatch struct {
	ManagedResource    *types.ResourceState   `json:"managed_resource"`
	DiscoveredResource *types.ResourceState   `json:"discovered_resource"`
	MatchConfidence    float64                `json:"match_confidence"`
	MatchReason        string                 `json:"match_reason"`
	Capabilities       map[string]interface{} `json:"capabilities"`
}

// ========== Plan-Level Recovery Types ==========
// These types support plan-level recovery where the entire execution plan
// is regenerated when failures occur, rather than recovering individual steps.

// PlanRecoveryCoordinator interface for coordinating plan-level recovery with the UI
type PlanRecoveryCoordinator interface {
	// RequestPlanRecoveryDecision sends the recovery plan to UI and waits for user approval
	RequestPlanRecoveryDecision(
		executionID string,
		failureContext *PlanFailureContext,
		recoveryStrategy *PlanRecoveryStrategy,
	) (approved bool, err error)

	// NotifyRecoveryAnalyzing informs UI that AI is analyzing the failure
	NotifyRecoveryAnalyzing(executionID string, failedStepID string, failedStepIndex int, completedStepsCount int, totalSteps int) error

	// NotifyRecoveryExecuting informs UI that approved recovery plan is being executed
	NotifyRecoveryExecuting(executionID string, strategy *PlanRecoveryStrategy) error

	// NotifyRecoveryCompleted informs UI that recovery succeeded
	NotifyRecoveryCompleted(executionID string) error

	// NotifyRecoveryFailed informs UI that recovery failed
	NotifyRecoveryFailed(executionID string, reason string) error
}

// PlanFailureContext contains comprehensive information about a plan execution failure
// This is sent to the AI for analysis and to the UI for display
type PlanFailureContext struct {
	// Execution Context
	ExecutionID      string    `json:"execution_id"`
	ExecutionStarted time.Time `json:"execution_started"`

	// Failure Information
	FailedStepID    string                   `json:"failed_step_id"`
	FailedStepIndex int                      `json:"failed_step_index"`
	FailedStep      *types.ExecutionPlanStep `json:"failed_step"`
	FailureError    string                   `json:"failure_error"`
	FailureTime     time.Time                `json:"failure_time"`
	AttemptNumber   int                      `json:"attempt_number"` // Which recovery attempt is this (1, 2, 3...)

	// Execution State
	CompletedSteps []*CompletedStepInfo       `json:"completed_steps"` // Steps that succeeded
	RemainingSteps []*types.ExecutionPlanStep `json:"remaining_steps"` // Steps not yet executed
	CurrentState   *types.InfrastructureState `json:"current_state"`   // Current infrastructure state

	// Original Plan Context
	OriginalPlan       []*types.ExecutionPlanStep `json:"original_plan"`
	OriginalUserIntent string                     `json:"original_user_intent"`
	OriginalAction     string                     `json:"original_action"` // create, update, delete

	// Environmental Context
	AWSRegion        string            `json:"aws_region"`
	ResourceMappings map[string]string `json:"resource_mappings"` // step_id -> resource_id

	// Recovery History (for subsequent recovery attempts)
	PreviousRecoveryAttempts []*RecoveryAttemptHistory `json:"previous_recovery_attempts,omitempty"`
}

// CompletedStepInfo represents a successfully completed step that should be preserved
type CompletedStepInfo struct {
	StepID       string                   `json:"step_id"`
	StepName     string                   `json:"step_name"`
	StepIndex    int                      `json:"step_index"`
	ResourceID   string                   `json:"resource_id,omitempty"`
	ResourceType string                   `json:"resource_type,omitempty"`
	Status       string                   `json:"status"` // "completed"
	Output       map[string]interface{}   `json:"output,omitempty"`
	CompletedAt  time.Time                `json:"completed_at"`
	Duration     time.Duration            `json:"duration"`
	OriginalStep *types.ExecutionPlanStep `json:"original_step,omitempty"` // Preserve full step details for recovery
}

// RecoveryAttemptHistory records information about previous recovery attempts
type RecoveryAttemptHistory struct {
	AttemptNumber       int           `json:"attempt_number"`
	SelectedStrategy    string        `json:"selected_strategy"` // Strategy type that was attempted
	StrategyIndex       int           `json:"strategy_index"`
	FailureReason       string        `json:"failure_reason"`
	FailedAtStepID      string        `json:"failed_at_step_id,omitempty"`
	Timestamp           time.Time     `json:"timestamp"`
	Duration            time.Duration `json:"duration"`
	AIConfidence        float64       `json:"ai_confidence"`
	RecoveryPlanSteps   int           `json:"recovery_plan_steps"`
	StepsCompletedCount int           `json:"steps_completed_count"`
}

// PlanRecoveryStrategy represents a complete recovery strategy with new execution plan
// Uses the same structure as AgentDecision for consistency - just a new execution plan
type PlanRecoveryStrategy struct {
	// Action and Reasoning (same as AgentDecision)
	Action     string  `json:"action"`     // Same as original: "create_infrastructure", "update_infrastructure", etc.
	Reasoning  string  `json:"reasoning"`  // AI's reasoning for this recovery approach
	Confidence float64 `json:"confidence"` // 0.0 to 1.0

	// Success and Risk Assessment
	SuccessProbability float64 `json:"success_probability"` // 0.0 to 1.0
	RiskLevel          string  `json:"risk_level"`          // "low", "medium", "high"
	EstimatedDuration  string  `json:"estimated_duration"`  // e.g., "5m", "10m"

	// New Execution Plan - Uses standard ExecutionPlanStep (same as normal planning)
	// Frontend will replace the failed plan with this new plan
	ExecutionPlan []*types.ExecutionPlanStep `json:"executionPlan"`

	// Metadata about the recovery plan
	TotalSteps     int    `json:"total_steps"`
	PreservedCount int    `json:"preserved_count"` // How many completed steps were preserved
	NewStepsCount  int    `json:"new_steps_count"` // How many new/modified steps
	RecoveryNotes  string `json:"recovery_notes"`  // Additional context for the user
}

// RollbackStepInfo represents a rollback action to undo a completed step
type RollbackStepInfo struct {
	ResourceID   string `json:"resource_id"`
	ResourceType string `json:"resource_type"`
	Action       string `json:"action"` // "delete", "restore", "modify"
	Reason       string `json:"reason"`
}

// AIPlanRecoveryAnalysis represents the AI's complete analysis and single recovery plan
// This is the output from the AI consultation - simplified to return ONE recovery plan
type AIPlanRecoveryAnalysis struct {
	// Failure Analysis
	FailureReason    string  `json:"failure_reason"`    // Clear explanation of why it failed
	RootCause        string  `json:"root_cause"`        // Deep root cause analysis
	ImpactAssessment string  `json:"impact_assessment"` // How this affects remaining steps
	Confidence       float64 `json:"confidence"`        // AI's confidence in analysis (0.0 to 1.0)

	// Single Recovery Strategy (AI decides the best approach)
	Strategy *PlanRecoveryStrategy `json:"strategy"` // The recommended recovery plan

	// Additional Context
	RiskFactors    []string `json:"risk_factors"`    // Identified risks
	SuccessFactors []string `json:"success_factors"` // Factors for success

	// Metadata
	AnalysisTimestamp time.Time     `json:"analysis_timestamp"`
	AnalysisDuration  time.Duration `json:"analysis_duration"`
	AIModel           string        `json:"ai_model,omitempty"`
	TokensUsed        int           `json:"tokens_used,omitempty"`
}

// PlanRecoveryResult represents the outcome of executing a recovery strategy
type PlanRecoveryResult struct {
	Success             bool                   `json:"success"`
	SelectedStrategy    *PlanRecoveryStrategy  `json:"selected_strategy"`
	ExecutionResult     *types.PlanExecution   `json:"execution_result,omitempty"`
	FailureReason       string                 `json:"failure_reason,omitempty"`
	AttemptNumber       int                    `json:"attempt_number"`
	Duration            time.Duration          `json:"duration"`
	CompletedSteps      []*types.ExecutionStep `json:"completed_steps,omitempty"`
	NewResourcesCreated []string               `json:"new_resources_created,omitempty"`
}

// ========== Plan Recovery Engine Interface ==========

// PlanRecoveryEngine defines the interface for intelligent plan-level recovery
type PlanRecoveryEngine interface {
	// AnalyzePlanFailure analyzes a plan execution failure and generates recovery strategies
	AnalyzePlanFailure(
		ctx context.Context,
		failureContext *PlanFailureContext,
	) (*AIPlanRecoveryAnalysis, error)

	// ValidateRecoveryStrategy ensures the proposed recovery strategy is safe and feasible
	ValidateRecoveryStrategy(
		ctx context.Context,
		strategy *PlanRecoveryStrategy,
		failureContext *PlanFailureContext,
	) error

	// GenerateRecoveryPrompt creates the prompt for AI consultation
	GenerateRecoveryPrompt(
		failureContext *PlanFailureContext,
	) string
}

// ========== Enhanced Agent Interface for Plan Recovery ==========

// PlanRecoveryAwareAgent extends the base agent with plan-level recovery capabilities
type PlanRecoveryAwareAgent interface {
	// ExecutePlanWithReActRecovery executes a plan with plan-level ReAct recovery
	ExecutePlanWithReActRecovery(
		ctx context.Context,
		decision *types.AgentDecision,
		progressChan chan<- *types.ExecutionUpdate,
		coordinator PlanRecoveryCoordinator,
	) (*types.PlanExecution, error)

	// ConsultAIForPlanRecovery asks the AI model for plan-level recovery strategies
	ConsultAIForPlanRecovery(
		ctx context.Context,
		failureContext *PlanFailureContext,
	) (*AIPlanRecoveryAnalysis, error)

	// ExecuteRecoveryStrategy executes a selected recovery strategy
	ExecuteRecoveryStrategy(
		ctx context.Context,
		strategy *PlanRecoveryStrategy,
		failureContext *PlanFailureContext,
		progressChan chan<- *types.ExecutionUpdate,
	) (*PlanRecoveryResult, error)
}

// PlanRecoveryConfig defines configuration for plan-level recovery behavior
type PlanRecoveryConfig struct {
	// Recovery Limits
	MaxRecoveryAttempts int           `json:"max_recovery_attempts"` // Max times to attempt recovery (e.g., 3)
	RecoveryTimeout     time.Duration `json:"recovery_timeout"`      // Max time for entire recovery process
	AnalysisTimeout     time.Duration `json:"analysis_timeout"`      // Max time for AI analysis

	// AI Configuration
	EnableAIConsultation bool    `json:"enable_ai_consultation"` // Whether to use AI for recovery
	MinConfidence        float64 `json:"min_confidence"`         // Minimum AI confidence to proceed (0.0-1.0)

	// Strategy Preferences
	AllowRollback           bool `json:"allow_rollback"`            // Whether rollback strategies are allowed
	RequireUserConfirmation bool `json:"require_user_confirmation"` // Always require user confirmation

	// Coordinator
	Coordinator PlanRecoveryCoordinator `json:"-"` // UI coordinator for recovery decisions
}

// Parse AI response with new format (recoverySteps + adjustedRemainingSteps)
type AIResponse struct {
	FailureReason          string                     `json:"failure_reason"`
	RootCause              string                     `json:"root_cause"`
	ImpactAssessment       string                     `json:"impact_assessment"`
	Confidence             float64                    `json:"confidence"`
	RecoverySteps          []*types.ExecutionPlanStep `json:"recoverySteps"`
	AdjustedRemainingSteps []*types.ExecutionPlanStep `json:"adjustedRemainingSteps"`
	Reasoning              string                     `json:"reasoning"`
	NewStepsCount          int                        `json:"newStepsCount"`
	SuccessProbability     float64                    `json:"successProbability"`
	RiskLevel              string                     `json:"riskLevel"`
	EstimatedDuration      string                     `json:"estimatedDuration"`
	RiskFactors            []string                   `json:"riskFactors"`
	SuccessFactors         []string                   `json:"successFactors"`
}

// DefaultPlanRecoveryConfig returns sensible defaults for plan recovery
func DefaultPlanRecoveryConfig(coordinator PlanRecoveryCoordinator) *PlanRecoveryConfig {
	return &PlanRecoveryConfig{
		MaxRecoveryAttempts:     3,
		RecoveryTimeout:         30 * time.Minute,
		AnalysisTimeout:         30 * time.Second,
		EnableAIConsultation:    true,
		MinConfidence:           0.5,
		AllowRollback:           true,
		RequireUserConfirmation: true,
		Coordinator:             coordinator,
	}
}
