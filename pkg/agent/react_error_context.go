package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// ========== Error Context Enrichment ==========

// FailureContextBuilder builds comprehensive failure context for ReAct recovery
type FailureContextBuilder struct {
	agent *StateAwareAgent
}

// NewFailureContextBuilder creates a new failure context builder
func NewFailureContextBuilder(agent *StateAwareAgent) *FailureContextBuilder {
	return &FailureContextBuilder{agent: agent}
}

// BuildFailureContext creates comprehensive context about a failed step
func (f *FailureContextBuilder) BuildFailureContext(
	ctx context.Context,
	failedStep *types.ExecutionPlanStep,
	stepError error,
	execution *types.PlanExecution,
	executionPlan []*types.ExecutionPlanStep,
	attemptNumber int,
	previousAttempts []*StepRecoveryAttempt,
) (*StepFailureContext, error) {

	f.agent.Logger.WithFields(map[string]interface{}{
		"step_id":        failedStep.ID,
		"attempt_number": attemptNumber,
		"error":          stepError.Error(),
	}).Info("Building comprehensive failure context for ReAct recovery")

	// Get current infrastructure state
	currentState, _, _, err := f.agent.AnalyzeInfrastructureState(ctx, false)
	if err != nil {
		f.agent.Logger.WithError(err).Warn("Failed to get current state for failure context")
		// Continue with nil state rather than failing
	}

	// Extract completed steps from execution
	completedSteps := make([]*types.ExecutionStep, len(execution.Steps))
	copy(completedSteps, execution.Steps)

	// Find remaining steps after the failed step
	remainingSteps := f.findRemainingSteps(failedStep, executionPlan)

	// Get available MCP tools
	availableTools := f.getAvailableTools()

	// Find similar tools that might achieve the same goal
	similarTools := f.findSimilarTools(failedStep)

	// Get current resource mappings
	resourceMappings := f.getResourceMappings()

	// Get AWS region from config
	awsRegion := f.agent.awsConfig.Region
	if awsRegion == "" {
		awsRegion = "us-east-1" // Default fallback
	}

	failureContext := &StepFailureContext{
		// Step Information
		OriginalStep:     failedStep,
		FailureError:     stepError.Error(),
		AttemptNumber:    attemptNumber,
		PreviousAttempts: previousAttempts,

		// Execution Context
		ExecutionID:    execution.ID,
		CompletedSteps: completedSteps,
		RemainingSteps: remainingSteps,
		CurrentState:   currentState,

		// Available Tools and Options
		AvailableTools:   availableTools,
		SimilarTools:     similarTools,
		ResourceMappings: resourceMappings,

		// Environmental Context
		AWSRegion:        awsRegion,
		Timestamp:        time.Now(),
		ExecutionTimeout: 10 * time.Minute, // Default timeout
	}

	f.agent.Logger.WithFields(map[string]interface{}{
		"available_tools":   len(availableTools),
		"similar_tools":     len(similarTools),
		"completed_steps":   len(completedSteps),
		"remaining_steps":   len(remainingSteps),
		"resource_mappings": len(resourceMappings),
	}).Info("Built comprehensive failure context")

	return failureContext, nil
}

// ========== Private Helper Methods ==========

// findRemainingSteps finds all steps that come after the failed step
func (f *FailureContextBuilder) findRemainingSteps(failedStep *types.ExecutionPlanStep, executionPlan []*types.ExecutionPlanStep) []*types.ExecutionPlanStep {
	var remainingSteps []*types.ExecutionPlanStep
	foundFailedStep := false

	for _, step := range executionPlan {
		if foundFailedStep {
			remainingSteps = append(remainingSteps, step)
		} else if step.ID == failedStep.ID {
			foundFailedStep = true
		}
	}

	return remainingSteps
}

// getAvailableTools gets all currently available MCP tools
func (f *FailureContextBuilder) getAvailableTools() []MCPToolInfo {
	f.agent.capabilityMutex.RLock()
	defer f.agent.capabilityMutex.RUnlock()

	var tools []MCPToolInfo
	for _, tool := range f.agent.mcpTools {
		tools = append(tools, tool)
	}

	return tools
}

// findSimilarTools finds tools that might achieve similar goals to the failed step
func (f *FailureContextBuilder) findSimilarTools(failedStep *types.ExecutionPlanStep) []MCPToolInfo {
	if failedStep.MCPTool == "" {
		return []MCPToolInfo{}
	}

	// Use the recovery engine to find similar tools
	recoveryEngine := NewStepRecoveryEngine(f.agent)
	similarTools, err := recoveryEngine.GetSimilarTools(
		context.Background(),
		failedStep.MCPTool,
		failedStep.Description,
	)

	if err != nil {
		f.agent.Logger.WithError(err).Debug("Failed to find similar tools")
		return []MCPToolInfo{}
	}

	return similarTools
}

// getResourceMappings gets current step->resource ID mappings
func (f *FailureContextBuilder) getResourceMappings() map[string]string {
	f.agent.mappingsMutex.RLock()
	defer f.agent.mappingsMutex.RUnlock()

	// Create a copy of the mappings to avoid concurrent access issues
	mappings := make(map[string]string)
	for k, v := range f.agent.resourceMappings {
		mappings[k] = v
	}

	return mappings
}

// EnrichErrorWithContext adds contextual information to an error for better debugging
func (f *FailureContextBuilder) EnrichErrorWithContext(
	originalError error,
	failedStep *types.ExecutionPlanStep,
	executionContext map[string]interface{},
) error {

	enrichedMessage := fmt.Sprintf(
		"Step '%s' (%s) failed: %v. Context: action=%s, mcpTool=%s, resourceId=%s",
		failedStep.Name,
		failedStep.ID,
		originalError,
		failedStep.Action,
		failedStep.MCPTool,
		failedStep.ResourceID,
	)

	// Add parameter information
	if len(failedStep.Parameters) > 0 {
		enrichedMessage += fmt.Sprintf(", parameters=%v", failedStep.Parameters)
	}

	// Add execution context if provided
	if len(executionContext) > 0 {
		enrichedMessage += fmt.Sprintf(", executionContext=%v", executionContext)
	}

	return fmt.Errorf("%s", enrichedMessage)
}

// CreateRecoveryAttempt creates a record of a recovery attempt
func (f *FailureContextBuilder) CreateRecoveryAttempt(
	attemptNumber int,
	toolUsed string,
	parameters map[string]interface{},
	recoveryAction string,
	startTime time.Time,
) *StepRecoveryAttempt {

	return &StepRecoveryAttempt{
		AttemptNumber:  attemptNumber,
		ToolUsed:       toolUsed,
		Parameters:     parameters,
		RecoveryAction: recoveryAction,
		Timestamp:      startTime,
		Duration:       time.Since(startTime),
	}
}

// UpdateRecoveryAttemptWithResult updates a recovery attempt with results
func (f *FailureContextBuilder) UpdateRecoveryAttemptWithResult(
	attempt *StepRecoveryAttempt,
	result map[string]interface{},
	err error,
) {
	attempt.Duration = time.Since(attempt.Timestamp)

	if err != nil {
		attempt.Error = err.Error()
	} else {
		attempt.Result = result
	}
}
