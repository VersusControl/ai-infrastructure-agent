package agent

import (
	"context"
	"time"

	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// RecoveryCoordinator interface for coordinating recovery with the UI
type RecoveryCoordinator interface {
	RequestRecoveryDecision(stepID string, failureContext map[string]interface{}, recoveryOptions []map[string]interface{}) (map[string]interface{}, error)
}

// ========== Core Recovery Types ==========

// StepRecoveryResult represents the outcome of a recovery attempt
type StepRecoveryResult struct {
	Success         bool                   `json:"success"`
	AlternativeTool string                 `json:"alternative_tool,omitempty"`
	ModifiedParams  map[string]interface{} `json:"modified_params,omitempty"`
	Reasoning       string                 `json:"reasoning,omitempty"`
	AttemptNumber   int                    `json:"attempt_number"`
	RecoveryAction  string                 `json:"recovery_action"` // "retry_same", "try_alternative", "modify_params", "skip_step", "fail_plan"
	Output          map[string]interface{} `json:"output,omitempty"`
}

// StepFailureContext contains comprehensive information about a failed step for AI analysis
type StepFailureContext struct {
	// Step Information
	OriginalStep     *types.ExecutionPlanStep `json:"original_step"`
	FailureError     string                   `json:"failure_error"`
	AttemptNumber    int                      `json:"attempt_number"`
	PreviousAttempts []*StepRecoveryAttempt   `json:"previous_attempts"`

	// Execution Context
	ExecutionID    string                     `json:"execution_id"`
	CompletedSteps []*types.ExecutionStep     `json:"completed_steps"`
	RemainingSteps []*types.ExecutionPlanStep `json:"remaining_steps"`
	CurrentState   *types.InfrastructureState `json:"current_state"`

	// Available Tools and Options
	AvailableTools   []MCPToolInfo     `json:"available_tools"`
	SimilarTools     []MCPToolInfo     `json:"similar_tools"`     // Tools that might achieve same goal
	ResourceMappings map[string]string `json:"resource_mappings"` // Current step->resource ID mappings

	// Environmental Context
	AWSRegion        string        `json:"aws_region"`
	Timestamp        time.Time     `json:"timestamp"`
	ExecutionTimeout time.Duration `json:"execution_timeout,omitempty"`
}

// StepRecoveryAttempt records information about a recovery attempt
type StepRecoveryAttempt struct {
	AttemptNumber  int                    `json:"attempt_number"`
	ToolUsed       string                 `json:"tool_used"`
	Parameters     map[string]interface{} `json:"parameters"`
	Error          string                 `json:"error,omitempty"`
	Result         map[string]interface{} `json:"result,omitempty"`
	RecoveryAction string                 `json:"recovery_action"`
	Timestamp      time.Time              `json:"timestamp"`
	Duration       time.Duration          `json:"duration"`
}

// RecoveryStrategy defines how the agent should approach recovery
type RecoveryStrategy struct {
	MaxAttempts          int           `json:"max_attempts"`
	EnableAIConsultation bool          `json:"enable_ai_consultation"`
	AllowToolSwapping    bool          `json:"allow_tool_swapping"`
	AllowParameterMod    bool          `json:"allow_parameter_modification"`
	ConsultationPrompt   string        `json:"consultation_prompt,omitempty"`
	TimeoutPerAttempt    time.Duration `json:"timeout_per_attempt"`
}

// ========== ReAct-Style Recovery Interfaces ==========

// StepRecoveryEngine defines the interface for intelligent step recovery
type StepRecoveryEngine interface {
	// AttemptStepRecovery tries to recover from a failed step using AI consultation
	AttemptStepRecovery(ctx context.Context, failureContext *StepFailureContext) (*StepRecoveryResult, error)

	// AnalyzeFailure analyzes the failure and suggests recovery options
	AnalyzeFailure(ctx context.Context, failureContext *StepFailureContext) (*AIRecoveryAnalysis, error)

	// GetSimilarTools finds alternative tools that might achieve the same goal
	GetSimilarTools(ctx context.Context, originalTool string, objective string) ([]MCPToolInfo, error)

	// ValidateRecoveryAction ensures the proposed recovery action is safe and feasible
	ValidateRecoveryAction(ctx context.Context, action *StepRecoveryResult, context *StepFailureContext) error
}

// AIRecoveryAnalysis represents the AI model's analysis of a failure and suggested recovery
type AIRecoveryAnalysis struct {
	FailureReason       string            `json:"failure_reason"`
	RecoveryOptions     []*RecoveryOption `json:"recovery_options"`
	RecommendedAction   string            `json:"recommended_action"`
	Confidence          float64           `json:"confidence"` // 0.0 to 1.0
	RequiresUserInput   bool              `json:"requires_user_input"`
	RiskAssessment      string            `json:"risk_assessment"`
	AlternativeApproach string            `json:"alternative_approach,omitempty"`
}

// RecoveryOption represents a possible recovery action suggested by the AI
type RecoveryOption struct {
	Action             string                 `json:"action"` // "retry_same", "try_alternative", "modify_params", "skip_step", "multi_step_recovery"
	ToolName           string                 `json:"tool_name,omitempty"`
	Parameters         map[string]interface{} `json:"parameters,omitempty"`
	Reasoning          string                 `json:"reasoning"`
	SuccessProbability float64                `json:"success_probability"`       // 0.0 to 1.0
	RiskLevel          string                 `json:"risk_level"`                // "low", "medium", "high"
	Dependencies       []string               `json:"dependencies,omitempty"`    // Steps that must be completed first
	MultiStepPlan      []*RecoveryStep        `json:"multi_step_plan,omitempty"` // For multi-step recovery scenarios
}

// RecoveryStep represents a single step in a multi-step recovery plan
type RecoveryStep struct {
	StepOrder  int                    `json:"step_order"`
	ToolName   string                 `json:"tool_name"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
	Purpose    string                 `json:"purpose"`
}

// ========== Enhanced Agent Interface ==========

// RecoveryAwareAgent extends the base agent with recovery capabilities
type RecoveryAwareAgent interface {
	// ExecuteStepWithRecovery executes a step with automatic recovery on failure
	ExecuteStepWithRecovery(ctx context.Context, planStep *types.ExecutionPlanStep, execution *types.PlanExecution, progressChan chan<- *types.ExecutionUpdate, strategy *RecoveryStrategy) (*types.ExecutionStep, error)

	// ConsultAIForRecovery asks the AI model for recovery advice
	ConsultAIForRecovery(ctx context.Context, failureContext *StepFailureContext) (*AIRecoveryAnalysis, error)

	// ApplyRecoveryAction executes the chosen recovery action
	ApplyRecoveryAction(ctx context.Context, action *StepRecoveryResult, failureContext *StepFailureContext) (*types.ExecutionStep, error)
}
