package api

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/versus-control/ai-infrastructure-agent/pkg/agent"
)

// ========== Plan-Level Recovery Coordinator Implementation ==========

// RequestPlanRecoveryDecision implements PlanRecoveryCoordinator interface
// This is the main method called by the agent when plan-level recovery is needed
func (ws *WebServer) RequestPlanRecoveryDecision(
	executionID string,
	context *agent.PlanFailureContext,
	strategy *agent.PlanRecoveryStrategy,
) (approved bool, err error) {
	ws.aiAgent.Logger.WithFields(map[string]interface{}{
		"execution_id": executionID,
		"failed_step":  context.FailedStepID,
		"attempt":      context.AttemptNumber,
		"total_steps":  strategy.TotalSteps,
	}).Info("Plan recovery decision requested from UI")

	// Create response channel for this request
	responseChan := make(chan *PlanRecoveryResponse, 1)

	// Store the recovery request
	ws.planRecoveryMutex.Lock()
	ws.planRecoveryRequests[executionID] = &PlanRecoveryRequest{
		ExecutionID:    executionID,
		FailureContext: context,
		Strategy:       strategy, // Single strategy now
		ResponseChan:   responseChan,
		Timestamp:      time.Now(),
	}
	ws.planRecoveryMutex.Unlock()

	// Convert strategy to JSON-friendly format for UI (same structure as AgentDecision)
	strategyJSON := map[string]interface{}{
		"action":             strategy.Action,
		"reasoning":          strategy.Reasoning,
		"confidence":         strategy.Confidence,
		"successProbability": strategy.SuccessProbability,
		"riskLevel":          strategy.RiskLevel,
		"estimatedDuration":  strategy.EstimatedDuration,
		"executionPlan":      strategy.ExecutionPlan, // Standard ExecutionPlanStep array
		"totalSteps":         strategy.TotalSteps,
		"preservedCount":     strategy.PreservedCount,
		"newStepsCount":      strategy.NewStepsCount,
		"recoveryNotes":      strategy.RecoveryNotes,
	}

	// Convert failure context to JSON-friendly format
	contextJSON := map[string]interface{}{
		"executionId":      context.ExecutionID,
		"failedStepId":     context.FailedStepID,
		"failedStepIndex":  context.FailedStepIndex,
		"failureError":     context.FailureError,
		"attemptNumber":    context.AttemptNumber,
		"completedSteps":   len(context.CompletedSteps),
		"remainingSteps":   len(context.RemainingSteps),
		"failedStepName":   context.FailedStep.Name,
		"failedStepAction": context.FailedStep.Action,
	}

	// Add completed steps details
	completedStepsJSON := make([]map[string]interface{}, len(context.CompletedSteps))
	for i, step := range context.CompletedSteps {
		completedStepsJSON[i] = map[string]interface{}{
			"stepId":       step.StepID,
			"stepName":     step.StepName,
			"stepIndex":    step.StepIndex,
			"resourceId":   step.ResourceID,
			"resourceType": step.ResourceType,
			"status":       step.Status,
		}
	}
	contextJSON["completedStepsDetails"] = completedStepsJSON

	// Send plan recovery request to UI (single strategy)
	ws.broadcastUpdate(map[string]interface{}{
		"type":           "plan_recovery_request",
		"executionId":    executionID,
		"failureContext": contextJSON,
		"strategy":       strategyJSON, // Single strategy now
		"timestamp":      time.Now(),
	})

	// Log the complete execution plan being sent to UI for debugging
	if len(strategy.ExecutionPlan) > 0 {
		// Marshal the complete strategy JSON (including full execution plan with parameters)
		if strategyFullJSON, err := json.MarshalIndent(strategyJSON, "", "  "); err == nil {
			ws.aiAgent.Logger.WithFields(map[string]interface{}{
				"execution_id": executionID,
				"total_steps":  len(strategy.ExecutionPlan),
			}).Infof("📋 Complete strategy sent to front-end:\n%s", string(strategyFullJSON))
		}
	}

	ws.aiAgent.Logger.WithField("execution_id", executionID).Info("Plan recovery request sent to UI, waiting for response")

	// Wait for user response with timeout
	select {
	case response := <-responseChan:
		// Cleanup
		ws.planRecoveryMutex.Lock()
		delete(ws.planRecoveryRequests, executionID)
		ws.planRecoveryMutex.Unlock()

		ws.aiAgent.Logger.WithFields(map[string]interface{}{
			"execution_id": executionID,
			"approved":     response.Approved,
		}).Info("Plan recovery decision received from user")

		return response.Approved, nil

	case <-time.After(15 * time.Minute): // 15 minute timeout for plan-level decisions
		// Cleanup
		ws.planRecoveryMutex.Lock()
		delete(ws.planRecoveryRequests, executionID)
		ws.planRecoveryMutex.Unlock()

		ws.aiAgent.Logger.WithField("execution_id", executionID).Error("Plan recovery decision timeout")
		return false, fmt.Errorf("plan recovery decision timeout after 15 minutes")
	}
}

// NotifyRecoveryAnalyzing implements PlanRecoveryCoordinator interface
func (ws *WebServer) NotifyRecoveryAnalyzing(
	executionID string,
	failedStepID string,
	failedIndex int,
	completedCount int,
	totalSteps int,
) error {
	ws.aiAgent.Logger.WithFields(map[string]interface{}{
		"execution_id":    executionID,
		"failed_step":     failedStepID,
		"failed_index":    failedIndex,
		"completed_count": completedCount,
		"total_steps":     totalSteps,
	}).Info("Notifying UI: Analyzing recovery options")

	ws.broadcastUpdate(map[string]interface{}{
		"type":           "plan_recovery_analyzing",
		"executionId":    executionID,
		"failedStepId":   failedStepID,
		"failedIndex":    failedIndex,
		"completedCount": completedCount,
		"totalSteps":     totalSteps,
		"message":        fmt.Sprintf("Plan failed at step %d/%d, analyzing recovery options...", failedIndex+1, totalSteps),
		"timestamp":      time.Now(),
	})

	return nil
}

// NotifyRecoveryExecuting implements PlanRecoveryCoordinator interface
func (ws *WebServer) NotifyRecoveryExecuting(executionID string, strategy *agent.PlanRecoveryStrategy) error {
	ws.aiAgent.Logger.WithFields(map[string]interface{}{
		"execution_id": executionID,
		"action":       strategy.Action,
		"total_steps":  strategy.TotalSteps,
	}).Info("Notifying UI: Executing recovery plan")

	ws.broadcastUpdate(map[string]interface{}{
		"type":        "plan_recovery_executing",
		"executionId": executionID,
		"action":      strategy.Action,
		"reasoning":   strategy.Reasoning,
		"totalSteps":  strategy.TotalSteps,
		"message":     fmt.Sprintf("Executing recovery plan with %d steps", strategy.TotalSteps),
		"timestamp":   time.Now(),
	})

	return nil
}

// NotifyRecoveryCompleted implements PlanRecoveryCoordinator interface
func (ws *WebServer) NotifyRecoveryCompleted(executionID string) error {
	ws.aiAgent.Logger.WithField("execution_id", executionID).Info("Notifying UI: Recovery completed")

	ws.broadcastUpdate(map[string]interface{}{
		"type":        "plan_recovery_completed",
		"executionId": executionID,
		"message":     "Recovery completed successfully, continuing execution",
		"timestamp":   time.Now(),
	})

	return nil
}

// NotifyRecoveryFailed implements PlanRecoveryCoordinator interface
func (ws *WebServer) NotifyRecoveryFailed(executionID string, reason string) error {
	ws.aiAgent.Logger.WithFields(map[string]interface{}{
		"execution_id": executionID,
		"reason":       reason,
	}).Warn("Notifying UI: Recovery failed")

	ws.broadcastUpdate(map[string]interface{}{
		"type":        "plan_recovery_failed",
		"executionId": executionID,
		"reason":      reason,
		"message":     fmt.Sprintf("Recovery attempt failed: %s", reason),
		"timestamp":   time.Now(),
	})

	return nil
}
