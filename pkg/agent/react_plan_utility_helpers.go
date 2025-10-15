package agent

import (
	"encoding/json"
	"fmt"

	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// ========== Private Helper Methods ==========

// parsePlanRecoveryAnalysis parses the AI response into structured analysis
// The AI response contains only recoverySteps and adjustedRemainingSteps
// This function prepends completed steps from execution history to build the complete executionPlan
func (e *DefaultPlanRecoveryEngine) parsePlanRecoveryAnalysis(response string, failureContext *PlanFailureContext) (*AIPlanRecoveryAnalysis, error) {
	// Use existing agent JSON processing methods
	var jsonStr string

	// Try primary extraction method
	jsonStr = e.agent.extractJSON(response)

	// If primary method fails, try alternative methods
	if jsonStr == "" {
		jsonStr = e.agent.extractJSONAlternative(response)
	}

	// If still no JSON found, try truncated JSON parsing
	if jsonStr == "" {
		jsonStr = e.agent.attemptTruncatedJSONParse(response)
	}

	if jsonStr == "" {
		return nil, fmt.Errorf("no valid JSON found in AI response")
	}

	// Clean up common AI JSON issues
	jsonStr = e.agent.cleanJSONComments(jsonStr)

	// Parse AI response with new format (recoverySteps + adjustedRemainingSteps)
	aiResponse := &AIResponse{}

	if err := json.Unmarshal([]byte(jsonStr), &aiResponse); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// Convert completed steps from execution history to ExecutionPlanStep format
	completedSteps := e.convertCompletedStepsToExecutionPlan(failureContext.CompletedSteps)

	// Assemble complete execution plan: [completed + recovery + remaining]
	executionPlan := make([]*types.ExecutionPlanStep, 0, len(completedSteps)+len(aiResponse.RecoverySteps)+len(aiResponse.AdjustedRemainingSteps))
	executionPlan = append(executionPlan, completedSteps...)
	executionPlan = append(executionPlan, aiResponse.RecoverySteps...)
	executionPlan = append(executionPlan, aiResponse.AdjustedRemainingSteps...)

	e.agent.Logger.WithFields(map[string]interface{}{
		"completed_steps": len(completedSteps),
		"recovery_steps":  len(aiResponse.RecoverySteps),
		"remaining_steps": len(aiResponse.AdjustedRemainingSteps),
		"total_steps":     len(executionPlan),
	}).Info("Assembled complete recovery plan from execution history + AI response")

	// Build recovery strategy
	strategy := &PlanRecoveryStrategy{
		Action:             failureContext.OriginalAction, // Use same action as original
		Reasoning:          aiResponse.Reasoning,
		Confidence:         aiResponse.Confidence,
		SuccessProbability: aiResponse.SuccessProbability,
		RiskLevel:          aiResponse.RiskLevel,
		EstimatedDuration:  aiResponse.EstimatedDuration,
		ExecutionPlan:      executionPlan,
		TotalSteps:         len(executionPlan),
		PreservedCount:     len(completedSteps),
		NewStepsCount:      aiResponse.NewStepsCount,
		RecoveryNotes:      fmt.Sprintf("Recovery plan preserves %d completed steps and adds %d new recovery steps", len(completedSteps), aiResponse.NewStepsCount),
	}

	// Build and return complete analysis
	analysis := &AIPlanRecoveryAnalysis{
		FailureReason:    aiResponse.FailureReason,
		RootCause:        aiResponse.RootCause,
		ImpactAssessment: aiResponse.ImpactAssessment,
		Confidence:       aiResponse.Confidence,
		Strategy:         strategy,
		RiskFactors:      aiResponse.RiskFactors,
		SuccessFactors:   aiResponse.SuccessFactors,
	}

	return analysis, nil
}

// convertCompletedStepsToExecutionPlan converts CompletedStepInfo to ExecutionPlanStep format
// This ensures completed steps from execution history are accurately represented
// It uses the OriginalStep if available to preserve all step details including Action
func (e *DefaultPlanRecoveryEngine) convertCompletedStepsToExecutionPlan(completedSteps []*CompletedStepInfo) []*types.ExecutionPlanStep {
	steps := make([]*types.ExecutionPlanStep, len(completedSteps))

	for i, cs := range completedSteps {
		// If we have the original step, use it to preserve all details
		if cs.OriginalStep != nil {
			step := &types.ExecutionPlanStep{
				ID:                cs.OriginalStep.ID,
				Name:              cs.OriginalStep.Name,
				Description:       cs.OriginalStep.Description,
				Action:            cs.OriginalStep.Action,
				ResourceID:        cs.ResourceID,
				MCPTool:           cs.OriginalStep.MCPTool,
				ToolParameters:    cs.OriginalStep.ToolParameters,
				DependsOn:         cs.OriginalStep.DependsOn,
				EstimatedDuration: cs.OriginalStep.EstimatedDuration,
				Status:            "completed",
			}
			steps[i] = step
		}
	}

	return steps
}
