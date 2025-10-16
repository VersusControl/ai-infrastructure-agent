package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// ========== Interface defines ==========

// PlanExecutorInterface defines plan execution functionality
//
// Available Functions:
//   - ExecuteConfirmedPlanWithDryRun() : Execute confirmed plans with dry-run support
//   - SimulatePlanExecution()          : Simulate plan execution for dry-run mode
//   - executeExecutionStep()           : Execute individual plan steps
//   - executeCreateAction()            : Execute create actions via MCP tools
//   - executeQueryAction()             : Execute query actions via MCP tools
//   - executeNativeMCPTool()          : Execute native MCP tool calls
//   - executeUpdateAction()            : Execute update actions on existing resources
//   - executeDeleteAction()            : Execute delete actions on resources
//   - executeValidateAction()          : Execute validation actions
//   - updateStateFromMCPResult()       : Update state from MCP operation results
//   - extractResourceTypeFromStep()    : Extract resource type from execution step
//   - getAvailableToolsContext()       : Get available tools context for AI prompts
//   - persistCurrentState()            : Persist current state to storage
//   - extractResourceIDFromResponse()  : Extract AWS resource IDs from responses
//   - waitForResourceReady()           : Wait for AWS resources to be ready before continuing
//   - checkResourceState()             : Check if a specific AWS resource is ready
//   - checkNATGatewayState()           : Check if NAT gateway is available
//   - checkRDSInstanceState()          : Check if RDS instance is available
//   - storeResourceMapping()           : Store step-to-resource ID mappings
//
// This file manages the execution of infrastructure plans, including dry-run
// capabilities, progress tracking, and state management integration.
//
// Usage Example:
//   1. execution := agent.ExecuteConfirmedPlanWithDryRun(ctx, decision, progressChan, false)
//   2. // Monitor execution through progressChan updates

// ========== Plan-Level Recovery Implementation ==========

// ExecutePlanWithReActRecovery executes a plan with plan-level ReAct recovery
// This is the NEW plan-level recovery method that replaces step-level recovery
func (a *StateAwareAgent) ExecutePlanWithReActRecovery(
	ctx context.Context,
	decision *types.AgentDecision,
	progressChan chan<- *types.ExecutionUpdate,
	coordinator PlanRecoveryCoordinator,
) (*types.PlanExecution, error) {
	a.Logger.WithFields(map[string]interface{}{
		"decision_id": decision.ID,
		"action":      decision.Action,
		"plan_steps":  len(decision.ExecutionPlan),
	}).Info("Executing plan with plan-level ReAct recovery")

	// Create execution plan
	execution := &types.PlanExecution{
		ID:        uuid.New().String(),
		Name:      fmt.Sprintf("Execute %s", decision.Action),
		Status:    "running",
		StartedAt: time.Now(),
		Steps:     []*types.ExecutionStep{},
		Changes:   []*types.ChangeDetection{},
		Errors:    []string{},
	}

	// Send initial progress update
	if progressChan != nil {
		progressChan <- &types.ExecutionUpdate{
			Type:        "execution_started",
			ExecutionID: execution.ID,
			Message:     "Starting plan execution with ReAct recovery",
			Timestamp:   time.Now(),
		}
	}

	// Get recovery configuration
	config := DefaultPlanRecoveryConfig(coordinator)

	// Attempt execution with recovery (up to MaxRecoveryAttempts)
	for attemptNumber := 1; attemptNumber <= config.MaxRecoveryAttempts; attemptNumber++ {

		// Execute the plan
		planToExecute := decision.ExecutionPlan
		if attemptNumber > 1 {
			// For retry attempts, planToExecute will be set by recovery strategy execution
			a.Logger.Info("This is a recovery attempt - plan should have been adjusted by previous recovery")
		}

		// Try to execute all steps
		failedStepIndex := -1
		var failureError error

		for i, planStep := range planToExecute {
			// Skip steps that are already completed (from previous recovery attempts)
			if planStep.Status == "completed" {
				a.Logger.WithFields(map[string]interface{}{
					"step_id":        planStep.ID,
					"step_name":      planStep.Name,
					"step_index":     i,
					"attempt_number": attemptNumber,
				}).Debug("Skipping already completed step from recovery plan")

				// Don't send progress updates or execute, these were done in previous attempt
				continue
			}

			// Send step started update
			if progressChan != nil {
				progressChan <- &types.ExecutionUpdate{
					Type:        "step_started",
					ExecutionID: execution.ID,
					StepID:      planStep.ID,
					Message:     fmt.Sprintf("Starting step %d/%d: %s", i+1, len(planToExecute), planStep.Name),
					Timestamp:   time.Now(),
				}
			}

			// Execute the step
			step, err := a.executeExecutionStep(planStep, execution, progressChan)

			if err != nil {
				// Step failed
				step.Status = "failed"
				step.Error = err.Error()
				execution.Steps = append(execution.Steps, step)

				failedStepIndex = i
				failureError = err

				if progressChan != nil {
					progressChan <- &types.ExecutionUpdate{
						Type:        "step_failed",
						ExecutionID: execution.ID,
						StepID:      planStep.ID,
						Message:     fmt.Sprintf("Step %d failed: %v", i+1, err),
						Error:       err.Error(),
						Timestamp:   time.Now(),
					}
				}

				break // Exit step execution loop
			}

			// Step succeeded
			step.Status = "completed"
			execution.Steps = append(execution.Steps, step)

			if progressChan != nil {
				progressChan <- &types.ExecutionUpdate{
					Type:        "step_completed",
					ExecutionID: execution.ID,
					StepID:      planStep.ID,
					Message:     fmt.Sprintf("Step %d completed: %s", i+1, planStep.Name),
					Timestamp:   time.Now(),
				}
			}

			// Add delay after successful step for smooth frontend display
			if a.config.StepDelayMS > 0 {
				time.Sleep(time.Millisecond * time.Duration(a.config.StepDelayMS))
			}

			// Persist state after successful step
			if err := a.persistCurrentState(); err != nil {
				a.Logger.WithError(err).Warn("Failed to persist state after successful step")
			}

			// Store resource mapping if resource was created
			if step.Output != nil {
				// Extract the mcp_response from the wrapped output
				var mcpResponse map[string]interface{}
				if wrapped, ok := step.Output["mcp_response"].(map[string]interface{}); ok {
					mcpResponse = wrapped
				} else {
					// If not wrapped, use the output directly (for backward compatibility)
					mcpResponse = step.Output
				}

				// Process arrays first to get concatenated values
				processedOutput := a.processArraysInMCPResponse(mcpResponse)
				if resourceID, err := a.extractResourceIDFromResponse(processedOutput, planStep.MCPTool, planStep.ID); err == nil && resourceID != "" {
					a.storeResourceMapping(planStep.ID, resourceID, processedOutput)
				}
			}
		} // Check if execution completed successfully
		if failedStepIndex == -1 {
			// All steps completed successfully
			execution.Status = "completed"
			now := time.Now()
			execution.CompletedAt = &now

			if progressChan != nil {
				progressChan <- &types.ExecutionUpdate{
					Type:        "execution_completed",
					ExecutionID: execution.ID,
					Message:     "Execution completed successfully",
					Timestamp:   time.Now(),
				}
			}

			return execution, nil
		}

		// Execution failed - attempt recovery
		a.Logger.WithFields(map[string]interface{}{
			"execution_id":      execution.ID,
			"failed_step_index": failedStepIndex,
			"attempt_number":    attemptNumber,
		}).Warn("Plan execution failed, attempting recovery")

		// Check if we've exhausted recovery attempts
		if attemptNumber >= config.MaxRecoveryAttempts {
			execution.Status = "failed"
			execution.Errors = append(execution.Errors,
				fmt.Sprintf("Execution failed after %d attempts: %v", attemptNumber, failureError))
			now := time.Now()
			execution.CompletedAt = &now

			if progressChan != nil {
				progressChan <- &types.ExecutionUpdate{
					Type:        "execution_failed",
					ExecutionID: execution.ID,
					Message:     fmt.Sprintf("Execution failed after %d recovery attempts", attemptNumber),
					Error:       failureError.Error(),
					Timestamp:   time.Now(),
				}
			}

			return execution, fmt.Errorf("execution failed after %d attempts: %w", attemptNumber, failureError)
		}

		// Extract user intent safely with fallback
		userIntent := ""
		if decision.Parameters != nil {
			if intent, ok := decision.Parameters["user_intent"].(string); ok {
				userIntent = intent
			}
		}
		// Fallback to reasoning if user_intent is not available
		if userIntent == "" {
			userIntent = decision.Reasoning
		}

		// Build failure context
		failureContext := a.buildPlanFailureContext(
			execution,
			failedStepIndex,
			planToExecute[failedStepIndex],
			failureError,
			decision.ExecutionPlan,
			decision.Action,
			userIntent,
			attemptNumber,
		)

		// Notify UI that we're analyzing recovery options
		if progressChan != nil {
			progressChan <- &types.ExecutionUpdate{
				Type:        "plan_recovery_analyzing",
				ExecutionID: execution.ID,
				StepID:      planToExecute[failedStepIndex].ID,
				Message:     fmt.Sprintf("Plan failed at step %d, analyzing recovery options...", failedStepIndex+1),
				Timestamp:   time.Now(),
			}
		}

		if coordinator != nil {
			coordinator.NotifyRecoveryAnalyzing(
				execution.ID,
				planToExecute[failedStepIndex].ID,
				failedStepIndex,
				len(execution.Steps)-1, // Completed steps (excluding failed one)
				len(planToExecute),
			)
		}

		// Consult AI for recovery strategy
		analysis, err := a.ConsultAIForPlanRecovery(ctx, failureContext)
		if err != nil {
			a.Logger.WithError(err).Error("Failed to get AI recovery analysis")
			// Continue to next attempt without recovery strategy
			continue
		}

		// NOTE: Don't send a separate progress update here - RequestPlanRecoveryDecision will send the full recovery plan
		// Sending a duplicate message would overwrite the complete plan in the frontend

		// Request user approval via coordinator (this sends the full recovery plan to UI)
		approved, err := coordinator.RequestPlanRecoveryDecision(
			execution.ID,
			failureContext,
			analysis.Strategy,
		)

		if err != nil {
			a.Logger.WithError(err).Error("Failed to get recovery decision from user")
			return execution, fmt.Errorf("recovery decision failed: %w", err)
		}

		if !approved {
			// User rejected the recovery plan
			execution.Status = "aborted"
			now := time.Now()
			execution.CompletedAt = &now

			if progressChan != nil {
				progressChan <- &types.ExecutionUpdate{
					Type:        "execution_aborted",
					ExecutionID: execution.ID,
					Message:     "Execution aborted by user",
					Timestamp:   time.Now(),
				}
			}

			return execution, fmt.Errorf("execution aborted by user")
		}

		// Execute the approved recovery strategy
		recoveryStrategy := analysis.Strategy

		if progressChan != nil {
			progressChan <- &types.ExecutionUpdate{
				Type:        "plan_recovery_executing",
				ExecutionID: execution.ID,
				Message:     fmt.Sprintf("Executing recovery plan with %d steps", len(recoveryStrategy.ExecutionPlan)),
				Timestamp:   time.Now(),
			}
		}

		if coordinator != nil {
			coordinator.NotifyRecoveryExecuting(execution.ID, recoveryStrategy)
		}

		// Execute the recovery strategy
		recoveryResult, err := a.ExecuteRecoveryStrategy(ctx, recoveryStrategy, failureContext, progressChan)
		if err != nil {
			a.Logger.WithError(err).Error("Recovery strategy execution failed")

			if coordinator != nil {
				coordinator.NotifyRecoveryFailed(execution.ID, err.Error())
			}

			// Continue to next recovery attempt
			continue
		}

		if progressChan != nil {
			progressChan <- &types.ExecutionUpdate{
				Type:        "plan_recovery_completed",
				ExecutionID: execution.ID,
				Message:     "Recovery completed, continuing execution",
				Timestamp:   time.Now(),
			}
		}

		if coordinator != nil {
			coordinator.NotifyRecoveryCompleted(execution.ID)
		}

		// Update execution with recovery steps
		execution.Steps = append(execution.Steps, recoveryResult.CompletedSteps...)

		// Recovery completed successfully - execution is done
		// ExecuteRecoveryStrategy has already executed the complete plan
		// (completed steps + recovery steps + remaining steps)
		execution.Status = "completed"
		now := time.Now()
		execution.CompletedAt = &now

		a.Logger.WithFields(map[string]interface{}{
			"execution_id":   execution.ID,
			"recovery_steps": len(recoveryResult.CompletedSteps),
			"total_steps":    len(execution.Steps),
			"attempt_number": attemptNumber,
		}).Info("Recovery strategy completed successfully, execution finished")

		// Notify completion via WebSocket
		if progressChan != nil {
			progressChan <- &types.ExecutionUpdate{
				Type:        "execution_completed",
				ExecutionID: execution.ID,
				Message:     fmt.Sprintf("Execution completed after recovery (attempt %d/%d)", attemptNumber, config.MaxRecoveryAttempts),
				Timestamp:   time.Now(),
			}
		}

		return execution, nil
	}

	// Should not reach here
	execution.Status = "failed"
	now := time.Now()
	execution.CompletedAt = &now
	return execution, fmt.Errorf("execution failed - max recovery attempts exhausted")
}

// ConsultAIForPlanRecovery asks the AI model for plan-level recovery strategies
func (a *StateAwareAgent) ConsultAIForPlanRecovery(
	ctx context.Context,
	failureContext *PlanFailureContext,
) (*AIPlanRecoveryAnalysis, error) {
	a.Logger.WithField("execution_id", failureContext.ExecutionID).Info("Consulting AI for plan recovery strategies")

	// Create recovery engine
	engine := NewPlanRecoveryEngine(a)

	// Analyze the failure
	analysis, err := engine.AnalyzePlanFailure(ctx, failureContext)
	if err != nil {
		return nil, fmt.Errorf("AI analysis failed: %w", err)
	}

	return analysis, nil
}

// ExecuteRecoveryStrategy executes a selected recovery strategy
func (a *StateAwareAgent) ExecuteRecoveryStrategy(
	ctx context.Context,
	strategy *PlanRecoveryStrategy,
	failureContext *PlanFailureContext,
	progressChan chan<- *types.ExecutionUpdate,
) (*PlanRecoveryResult, error) {

	startTime := time.Now()
	result := &PlanRecoveryResult{
		Success:          false,
		SelectedStrategy: strategy,
		AttemptNumber:    failureContext.AttemptNumber,
		CompletedSteps:   []*types.ExecutionStep{},
	}

	// Execute recovery plan steps (already in standard ExecutionPlanStep format)
	for i, planStep := range strategy.ExecutionPlan {
		// Skip steps that are already completed (preserved from original execution)
		if planStep.Status == "completed" {
			a.Logger.WithFields(map[string]interface{}{
				"step_id":        planStep.ID,
				"step_name":      planStep.Name,
				"step_index":     i,
				"total_steps":    len(strategy.ExecutionPlan),
				"execution_id":   failureContext.ExecutionID,
				"attempt_number": failureContext.AttemptNumber,
			}).Info("✅ Skipping already completed step in recovery plan execution")

			// Don't add to result.CompletedSteps since it's from original execution
			// These are just for UI context, not part of recovery execution
			continue
		}

		// Send step started update (same as main execution loop)
		if progressChan != nil {
			progressChan <- &types.ExecutionUpdate{
				Type:        "step_started",
				ExecutionID: failureContext.ExecutionID,
				StepID:      planStep.ID,
				Message:     fmt.Sprintf("Starting recovery step %d/%d: %s", i+1, len(strategy.ExecutionPlan), planStep.Name),
				Timestamp:   time.Now(),
			}
		}

		// Create temporary execution for this recovery step
		tempExecution := &types.PlanExecution{
			ID:     failureContext.ExecutionID + "-recovery",
			Status: "running",
		}

		// Execute the step
		step, err := a.executeExecutionStep(planStep, tempExecution, progressChan)
		if err != nil {
			result.Success = false
			result.FailureReason = fmt.Sprintf("Recovery step %s failed: %v", planStep.ID, err)
			result.Duration = time.Since(startTime)

			// Send step failed update (same as main execution loop)
			if progressChan != nil {
				progressChan <- &types.ExecutionUpdate{
					Type:        "step_failed",
					ExecutionID: failureContext.ExecutionID,
					StepID:      planStep.ID,
					Message:     fmt.Sprintf("Recovery step failed: %v", err),
					Error:       err.Error(),
					Timestamp:   time.Now(),
				}
			}

			a.Logger.WithError(err).WithField("step_id", planStep.ID).Error("Recovery step failed")
			return result, fmt.Errorf("recovery step failed: %w", err)
		}

		step.Status = "completed"
		result.CompletedSteps = append(result.CompletedSteps, step)

		// Send step completed update (same as main execution loop)
		if progressChan != nil {
			progressChan <- &types.ExecutionUpdate{
				Type:        "step_completed",
				ExecutionID: failureContext.ExecutionID,
				StepID:      planStep.ID,
				Message:     fmt.Sprintf("Completed recovery step %d/%d: %s", i+1, len(strategy.ExecutionPlan), planStep.Name),
				Timestamp:   time.Now(),
			}
		}

		// Persist state after each successful recovery step
		if err := a.persistCurrentState(); err != nil {
			a.Logger.WithError(err).Warn("Failed to persist state after recovery step")
		}

		// Store resource mapping if resource was created
		if step.Output != nil {
			// Extract the mcp_response from the wrapped output
			var mcpResponse map[string]interface{}
			if wrapped, ok := step.Output["mcp_response"].(map[string]interface{}); ok {
				mcpResponse = wrapped
			} else {
				// If not wrapped, use the output directly (for backward compatibility)
				mcpResponse = step.Output
			}

			// Process arrays first to get concatenated values
			processedOutput := a.processArraysInMCPResponse(mcpResponse)
			if resourceID, err := a.extractResourceIDFromResponse(processedOutput, planStep.MCPTool, planStep.ID); err == nil && resourceID != "" {
				a.storeResourceMapping(planStep.ID, resourceID, processedOutput)
				result.NewResourcesCreated = append(result.NewResourcesCreated, resourceID)
			}
		}
	}

	// Recovery completed successfully
	result.Success = true
	result.Duration = time.Since(startTime)

	a.Logger.WithFields(map[string]interface{}{
		"execution_id":     failureContext.ExecutionID,
		"recovery_steps":   len(result.CompletedSteps),
		"new_resources":    len(result.NewResourcesCreated),
		"duration_seconds": result.Duration.Seconds(),
	}).Info("Recovery strategy executed successfully")

	return result, nil
}

// buildPlanFailureContext creates a PlanFailureContext from execution state
func (a *StateAwareAgent) buildPlanFailureContext(
	execution *types.PlanExecution,
	failedStepIndex int,
	failedStep *types.ExecutionPlanStep,
	failureError error,
	originalPlan []*types.ExecutionPlanStep,
	originalAction string,
	originalUserIntent string,
	attemptNumber int,
) *PlanFailureContext {
	a.Logger.WithField("failed_step_index", failedStepIndex).Debug("Building plan failure context")

	// Extract completed steps (all steps before the failed one)
	completedSteps := make([]*CompletedStepInfo, 0)
	for i := 0; i < failedStepIndex && i < len(execution.Steps); i++ {
		execStep := execution.Steps[i]
		if execStep.Status == "completed" {
			completedStep := &CompletedStepInfo{
				StepID:      execStep.ID,
				StepName:    execStep.Name,
				StepIndex:   i,
				Status:      "completed",
				CompletedAt: *execStep.CompletedAt,
				Duration:    execStep.Duration,
			}

			// Preserve the full original step for recovery (CRITICAL for Action field)
			if i < len(originalPlan) {
				completedStep.OriginalStep = originalPlan[i]
			}

			// Extract resource ID if available
			if execStep.Output != nil {
				// First try to get the already-extracted resource ID from the wrapped output
				if resourceID, ok := execStep.Output["resource_id"].(string); ok && resourceID != "" {
					completedStep.ResourceID = resourceID
					completedStep.ResourceType = a.extractResourceTypeFromStep(originalPlan[i])
				} else {
					// Unwrap mcp_response and extract (for backward compatibility)
					var mcpResponse map[string]interface{}
					if wrapped, ok := execStep.Output["mcp_response"].(map[string]interface{}); ok {
						mcpResponse = wrapped
					} else {
						mcpResponse = execStep.Output
					}

					// Process arrays before extraction
					processedOutput := a.processArraysInMCPResponse(mcpResponse)

					// Try to extract resource ID from processed output
					if resourceID, err := a.extractResourceIDFromResponse(processedOutput, originalPlan[i].MCPTool, originalPlan[i].ID); err == nil && resourceID != "" {
						completedStep.ResourceID = resourceID
						completedStep.ResourceType = a.extractResourceTypeFromStep(originalPlan[i])
					}
				}
			}

			if execStep.Output != nil {
				completedStep.Output = execStep.Output
			}

			completedSteps = append(completedSteps, completedStep)
		}
	}

	// Extract remaining steps (all steps after the failed one)
	remainingSteps := make([]*types.ExecutionPlanStep, 0)
	for i := failedStepIndex + 1; i < len(originalPlan); i++ {
		remainingSteps = append(remainingSteps, originalPlan[i])
	}

	// Get resource mappings
	a.mappingsMutex.RLock()
	resourceMappings := make(map[string]string)
	for k, v := range a.resourceMappings {
		resourceMappings[k] = v
	}
	a.mappingsMutex.RUnlock()

	// Get current infrastructure state
	// Note: We're analyzing state without live scan for speed (scanLive=false)
	// This uses cached state which should be sufficient for recovery context
	currentState, _, _, err := a.AnalyzeInfrastructureState(context.Background(), false)
	if err != nil {
		a.Logger.WithError(err).Warn("Failed to get current infrastructure state, using nil")
		currentState = nil
	}

	// Build the context
	ctx := &PlanFailureContext{
		ExecutionID:        execution.ID,
		ExecutionStarted:   execution.StartedAt,
		FailedStepID:       failedStep.ID,
		FailedStepIndex:    failedStepIndex,
		FailedStep:         failedStep,
		FailureError:       failureError.Error(),
		FailureTime:        time.Now(),
		AttemptNumber:      attemptNumber,
		CompletedSteps:     completedSteps,
		RemainingSteps:     remainingSteps,
		CurrentState:       currentState,
		OriginalPlan:       originalPlan,
		OriginalUserIntent: originalUserIntent,
		OriginalAction:     originalAction,
		AWSRegion:          a.awsConfig.Region,
		ResourceMappings:   resourceMappings,
	}

	return ctx
}

// ExecuteConfirmedPlanWithDryRun executes a confirmed execution plan with a specific dry run setting
func (a *StateAwareAgent) ExecuteConfirmedPlanWithDryRun(ctx context.Context, decision *types.AgentDecision, progressChan chan<- *types.ExecutionUpdate, dryRun bool) (*types.PlanExecution, error) {
	if dryRun {
		a.Logger.Info("Dry run mode - simulating execution")
		a.Logger.Debug("About to call SimulatePlanExecution")
		result := a.SimulatePlanExecution(decision, progressChan)
		a.Logger.WithField("simulation_result", result.Status).Debug("Simulation completed")
		return result, nil
	}

	// Create execution plan
	execution := &types.PlanExecution{
		ID:        uuid.New().String(),
		Name:      fmt.Sprintf("Execute %s", decision.Action),
		Status:    "running",
		StartedAt: time.Now(),
		Steps:     []*types.ExecutionStep{},
		Changes:   []*types.ChangeDetection{},
		Errors:    []string{},
	}

	// Send initial progress update
	if progressChan != nil {
		progressChan <- &types.ExecutionUpdate{
			Type:        "execution_started",
			ExecutionID: execution.ID,
			Message:     "Starting plan execution",
			Timestamp:   time.Now(),
		}
	}

	// Execute each step in the plan (simple execution without step-level recovery)
	for i, planStep := range decision.ExecutionPlan {
		// Send step started update
		if progressChan != nil {
			progressChan <- &types.ExecutionUpdate{
				Type:        "step_started",
				ExecutionID: execution.ID,
				StepID:      planStep.ID,
				Message:     fmt.Sprintf("Starting step %d/%d: %s", i+1, len(decision.ExecutionPlan), planStep.Name),
				Timestamp:   time.Now(),
			}
		}

		// Execute the step without recovery (simple execution)
		step, err := a.executeExecutionStep(planStep, execution, progressChan)
		if err != nil {
			execution.Status = "failed"
			execution.Errors = append(execution.Errors, fmt.Sprintf("Step %s failed: %v", planStep.ID, err))

			if progressChan != nil {
				progressChan <- &types.ExecutionUpdate{
					Type:        "step_failed_final",
					ExecutionID: execution.ID,
					StepID:      planStep.ID,
					Message:     fmt.Sprintf("Step failed: %v", err),
					Error:       err.Error(),
					Timestamp:   time.Now(),
				}
			}
			break
		}

		execution.Steps = append(execution.Steps, step)

		// 🔥 CRITICAL: Save state after each successful step
		// This ensures that if later steps fail, we don't lose track of successfully created resources
		a.Logger.WithField("step_id", planStep.ID).Info("Attempting to persist state after successful step")

		if err := a.persistCurrentState(); err != nil {
			a.Logger.WithError(err).WithField("step_id", planStep.ID).Error("CRITICAL: Failed to persist state after successful step - this may cause state inconsistency")
			// Don't fail the execution for state persistence issues, but make it very visible
		} else {
			a.Logger.WithField("step_id", planStep.ID).Info("Successfully persisted state after step completion")
		}

		// Send step completed update
		if progressChan != nil {
			progressChan <- &types.ExecutionUpdate{
				Type:        "step_completed",
				ExecutionID: execution.ID,
				StepID:      planStep.ID,
				Message:     fmt.Sprintf("Completed step %d/%d: %s", i+1, len(decision.ExecutionPlan), planStep.Name),
				Timestamp:   time.Now(),
			}
		}
	}

	// Complete execution
	now := time.Now()
	execution.CompletedAt = &now
	if execution.Status != "failed" {
		execution.Status = "completed"
	}

	// Update decision record
	decision.ExecutedAt = &now
	if execution.Status == "failed" {
		decision.Result = "failed"
		decision.Error = strings.Join(execution.Errors, "; ")
	} else {
		decision.Result = "success"
	}

	// Send final progress update
	if progressChan != nil {
		progressChan <- &types.ExecutionUpdate{
			Type:        "execution_completed",
			ExecutionID: execution.ID,
			Message:     fmt.Sprintf("Plan execution %s", execution.Status),
			Timestamp:   time.Now(),
		}
	}

	a.Logger.WithFields(map[string]interface{}{
		"execution_id": execution.ID,
		"status":       execution.Status,
		"steps":        len(execution.Steps),
	}).Info("Plan execution completed")

	return execution, nil
}

// SimulatePlanExecution simulates plan execution for dry run mode (exported version)
func (a *StateAwareAgent) SimulatePlanExecution(decision *types.AgentDecision, progressChan chan<- *types.ExecutionUpdate) *types.PlanExecution {
	a.Logger.WithField("plan_steps", len(decision.ExecutionPlan)).Debug("Starting SimulatePlanExecution")

	now := time.Now()
	execution := &types.PlanExecution{
		ID:        uuid.New().String(),
		Name:      fmt.Sprintf("Simulate %s", decision.Action),
		Status:    "running",
		StartedAt: now,
		Steps:     []*types.ExecutionStep{},
		Changes:   []*types.ChangeDetection{},
		Errors:    []string{},
	}

	a.Logger.WithField("execution_id", execution.ID).Debug("Created execution plan")

	// Send initial update
	if progressChan != nil {
		a.Logger.Debug("Sending initial progress update")
		select {
		case progressChan <- &types.ExecutionUpdate{
			Type:        "execution_started",
			ExecutionID: execution.ID,
			Message:     "Starting plan simulation (dry run)",
			Timestamp:   time.Now(),
		}:
			a.Logger.Debug("Initial progress update sent successfully")
		case <-time.After(time.Second * 5):
			a.Logger.Error("Timeout sending initial progress update - channel might be blocked")
		}
	} else {
		a.Logger.Debug("Progress channel is nil - skipping initial update")
	}

	a.Logger.WithField("steps_to_simulate", len(decision.ExecutionPlan)).Debug("Starting step simulation loop")

	// Simulate each step
	for i, planStep := range decision.ExecutionPlan {
		a.Logger.WithFields(map[string]interface{}{
			"step_number": i + 1,
			"step_id":     planStep.ID,
			"step_name":   planStep.Name,
		}).Debug("Simulating step")

		// Send step started update
		if progressChan != nil {
			select {
			case progressChan <- &types.ExecutionUpdate{
				Type:        "step_started",
				ExecutionID: execution.ID,
				StepID:      planStep.ID,
				Message:     fmt.Sprintf("Simulating step %d/%d: %s", i+1, len(decision.ExecutionPlan), planStep.Name),
				Timestamp:   time.Now(),
			}:
				a.Logger.Debug("Step started update sent")
			case <-time.After(time.Second * 2):
				a.Logger.Warn("Timeout sending step started update")
			}
		}

		// Simulate step execution with delay
		a.Logger.Debug("Sleeping for step simulation delay")
		stepDelayDuration := time.Millisecond * time.Duration(a.config.StepDelayMS)
		time.Sleep(stepDelayDuration)

		stepStart := time.Now()
		stepEnd := stepStart.Add(stepDelayDuration)

		step := &types.ExecutionStep{
			ID:          planStep.ID,
			Name:        planStep.Name,
			Status:      "completed",
			Resource:    planStep.ResourceID,
			Action:      planStep.Action,
			StartedAt:   &stepStart,
			CompletedAt: &stepEnd,
			Duration:    stepDelayDuration,
			Output:      map[string]interface{}{"simulated": true, "message": "Dry run - no actual changes made"},
		}

		execution.Steps = append(execution.Steps, step)
		a.Logger.WithField("steps_completed", len(execution.Steps)).Debug("Added step to execution")

		// Send step completed update
		if progressChan != nil {
			select {
			case progressChan <- &types.ExecutionUpdate{
				Type:        "step_completed",
				ExecutionID: execution.ID,
				StepID:      planStep.ID,
				Message:     fmt.Sprintf("Simulated step %d/%d: %s", i+1, len(decision.ExecutionPlan), planStep.Name),
				Timestamp:   time.Now(),
			}:
				a.Logger.Debug("Step completed update sent")
			case <-time.After(time.Second * 2):
				a.Logger.Warn("Timeout sending step completed update")
			}
		}
	}

	a.Logger.Debug("Completed all step simulations, finalizing execution")

	// Complete simulation
	completion := time.Now()
	execution.CompletedAt = &completion
	execution.Status = "completed"

	// Send final update
	if progressChan != nil {
		select {
		case progressChan <- &types.ExecutionUpdate{
			Type:        "execution_completed",
			ExecutionID: execution.ID,
			Message:     "Plan simulation completed (dry run)",
			Timestamp:   time.Now(),
		}:
			a.Logger.Debug("Final progress update sent")
		case <-time.After(time.Second * 2):
			a.Logger.Warn("Timeout sending final progress update")
		}
	}

	a.Logger.WithFields(map[string]interface{}{
		"execution_id": execution.ID,
		"status":       execution.Status,
		"steps":        len(execution.Steps),
	}).Info("Plan simulation completed")

	return execution
}

// executeExecutionStep executes a single step in the execution plan
func (a *StateAwareAgent) executeExecutionStep(planStep *types.ExecutionPlanStep, execution *types.PlanExecution, progressChan chan<- *types.ExecutionUpdate) (*types.ExecutionStep, error) {
	startTime := time.Now()

	step := &types.ExecutionStep{
		ID:        planStep.ID,
		Name:      planStep.Name,
		Status:    "running",
		Resource:  planStep.ResourceID,
		Action:    planStep.Action,
		StartedAt: &startTime,
	}

	// Send progress update for step details
	if progressChan != nil {
		progressChan <- &types.ExecutionUpdate{
			Type:        "step_progress",
			ExecutionID: execution.ID,
			StepID:      planStep.ID,
			Message:     fmt.Sprintf("Executing: %s", planStep.Description),
			Timestamp:   time.Now(),
		}
	}

	// Execute based on action type
	var result map[string]interface{}
	var err error

	switch planStep.Action {
	case "create":
		result, err = a.executeCreateAction(planStep, progressChan, execution.ID)
	case "query":
		// Query action - executes MCP tools for data retrieval (unified with create)
		result, err = a.executeQueryAction(planStep, progressChan, execution.ID)
	// case "update":
	// 	result, err = a.executeUpdateAction(ctx, planStep, progressChan, execution.ID)
	// case "delete":
	// 	result, err = a.executeDeleteAction(planStep, progressChan, execution.ID)
	// case "validate":
	// 	result, err = a.executeValidateAction(planStep, progressChan, execution.ID)
	default:
		err = fmt.Errorf("unknown action type: %s", planStep.Action)
	}

	// Complete the step
	endTime := time.Now()
	step.CompletedAt = &endTime
	step.Duration = endTime.Sub(startTime)

	if err != nil {
		step.Status = "failed"
		step.Error = err.Error()
	} else {
		step.Status = "completed"
		step.Output = result
	}

	return step, err
}

// executeCreateAction handles resource creation using native MCP tool calls
func (a *StateAwareAgent) executeCreateAction(planStep *types.ExecutionPlanStep, progressChan chan<- *types.ExecutionUpdate, executionID string) (map[string]interface{}, error) {
	// Send progress update
	if progressChan != nil {
		progressChan <- &types.ExecutionUpdate{
			Type:        "step_progress",
			ExecutionID: executionID,
			StepID:      planStep.ID,
			Message:     fmt.Sprintf("Creating %s resource: %s", planStep.ResourceID, planStep.Name),
			Timestamp:   time.Now(),
		}
	}

	// Use native MCP tool call approach
	return a.executeNativeMCPTool(planStep, progressChan, executionID)
}

// executeQueryAction handles data retrieval/query operations using native MCP tool calls
func (a *StateAwareAgent) executeQueryAction(planStep *types.ExecutionPlanStep, progressChan chan<- *types.ExecutionUpdate, executionID string) (map[string]interface{}, error) {
	// Send progress update
	if progressChan != nil {
		progressChan <- &types.ExecutionUpdate{
			Type:        "step_progress",
			ExecutionID: executionID,
			StepID:      planStep.ID,
			Message:     fmt.Sprintf("Querying resource: %s", planStep.Name),
			Timestamp:   time.Now(),
		}
	}

	// Execute the MCP tool using the same path as create
	return a.executeNativeMCPTool(planStep, progressChan, executionID)
}

// executeNativeMCPTool executes MCP tools directly with AI-provided parameters
func (a *StateAwareAgent) executeNativeMCPTool(planStep *types.ExecutionPlanStep, _ chan<- *types.ExecutionUpdate, _ string) (map[string]interface{}, error) {
	toolName := planStep.MCPTool

	if a.config.EnableDebug {
		a.Logger.WithFields(map[string]interface{}{
			"tool_name":       toolName,
			"step_id":         planStep.ID,
			"tool_parameters": planStep.ToolParameters,
		}).Info("Executing native MCP tool call")
	}

	// Ensure MCP capabilities are discovered
	if err := a.ensureMCPCapabilities(); err != nil {
		return nil, fmt.Errorf("failed to ensure MCP capabilities: %w", err)
	}

	// Validate tool exists in discovered capabilities
	a.capabilityMutex.RLock()
	toolInfo, exists := a.mcpTools[toolName]
	availableTools := make([]string, 0, len(a.mcpTools))
	for tool := range a.mcpTools {
		availableTools = append(availableTools, tool)
	}
	a.capabilityMutex.RUnlock()

	if !exists {
		a.Logger.WithFields(map[string]interface{}{
			"requested_tool":  toolName,
			"available_tools": availableTools,
			"tools_count":     len(availableTools),
		}).Error("MCP tool not found - debugging tool discovery issue")
		return nil, fmt.Errorf("MCP tool %s not found in discovered capabilities. Available tools: %v", toolName, availableTools)
	}

	// Prepare tool arguments - start with AI-provided parameters
	arguments := make(map[string]interface{})

	for key, value := range planStep.ToolParameters {
		if strValue, ok := value.(string); ok {
			if strings.Contains(strValue, "{{") && strings.Contains(strValue, "}}") {
				if strings.Count(strValue, "{{") > 1 && strings.Contains(strValue, "},{{") {
					// Handles comma-separated: "{{ref1}},{{ref2}},{{ref3}}"
					parts := strings.Split(strValue, ",")
					resolvedParts := make([]string, 0, len(parts))

					for _, part := range parts {
						part = strings.TrimSpace(part)
						if strings.Contains(part, "{{") && strings.Contains(part, "}}") {
							resolvedValue, err := a.resolveDependencyReference(part)
							if err != nil {
								return nil, fmt.Errorf("failed to resolve dependency reference %s in comma-separated list for parameter %s: %w", part, key, err)
							}
							resolvedParts = append(resolvedParts, resolvedValue)
						} else {
							resolvedParts = append(resolvedParts, part)
						}
					}

					// Join resolved values back with comma
					arguments[key] = strings.Join(resolvedParts, ",")

					if a.config.EnableDebug {
						a.Logger.WithFields(map[string]interface{}{
							"key":            key,
							"original_value": strValue,
							"resolved_value": arguments[key],
							"parts_resolved": len(resolvedParts),
						}).Info("Successfully resolved comma-separated dependency references")
					}
				} else {
					// Handles single reference: "{{step-id.field}}"
					resolvedValue, err := a.resolveDependencyReference(strValue)
					if err != nil {
						return nil, fmt.Errorf("failed to resolve dependency reference %s for parameter %s: %w", strValue, key, err)
					}

					// Check if the resolved value is a JSON array string
					if strings.HasPrefix(resolvedValue, "[") && strings.HasSuffix(resolvedValue, "]") {
						var arrayValue []interface{}
						if err := json.Unmarshal([]byte(resolvedValue), &arrayValue); err == nil {
							// Successfully parsed as JSON array - use it as array
							arguments[key] = arrayValue

							if a.config.EnableDebug {
								a.Logger.WithFields(map[string]interface{}{
									"key":            key,
									"original_value": strValue,
									"resolved_type":  "array",
									"array_length":   len(arrayValue),
								}).Info("Successfully resolved dependency reference as JSON array")
							}
						} else {
							// Failed to parse - use as string
							arguments[key] = resolvedValue

							if a.config.EnableDebug {
								a.Logger.WithFields(map[string]interface{}{
									"key":            key,
									"original_value": strValue,
									"resolved_value": resolvedValue,
								}).Info("Successfully resolved dependency reference as string")
							}
						}
					} else {
						// Not a JSON array - use as string
						arguments[key] = resolvedValue

						if a.config.EnableDebug {
							a.Logger.WithFields(map[string]interface{}{
								"key":            key,
								"original_value": strValue,
								"resolved_value": resolvedValue,
							}).Info("Successfully resolved dependency reference")
						}
					}
				}
			} else {
				arguments[key] = value
			}
		} else if arrayValue, ok := value.([]interface{}); ok {
			// Handles array format: ["{{ref1}}", "{{ref2}}"]
			resolvedArray := make([]interface{}, len(arrayValue))
			for i, item := range arrayValue {
				if strItem, ok := item.(string); ok && strings.Contains(strItem, "{{") && strings.Contains(strItem, "}}") {
					resolvedValue, err := a.resolveDependencyReference(strItem)
					if err != nil {
						return nil, fmt.Errorf("failed to resolve dependency reference %s in array parameter %s[%d]: %w", strItem, key, i, err)
					}

					resolvedArray[i] = resolvedValue
				} else {
					resolvedArray[i] = item
				}
			}
			arguments[key] = resolvedArray
		} else {
			arguments[key] = value
		}
	}

	// Fill in missing required parameters with intelligent defaults
	// if err := a.addMissingRequiredParameters(toolName, arguments, toolInfo); err != nil {
	// 	return nil, fmt.Errorf("failed to add required parameters for tool %s: %w", toolName, err)
	// }

	// Validate arguments before MCP call
	if err := a.validateNativeMCPArguments(toolName, arguments, toolInfo); err != nil {
		return nil, fmt.Errorf("invalid arguments for MCP tool %s: %w", toolName, err)
	}

	// Call the actual MCP tool
	result, err := a.callMCPTool(toolName, arguments)
	if err != nil {
		return nil, fmt.Errorf("MCP tool call failed: %w", err)
	}

	// Process arrays BEFORE extraction so extraction sees concatenated values
	processedResult := a.processArraysInMCPResponse(result)

	// Extract actual resource ID from MCP response (using processed result)
	resourceID, err := a.extractResourceIDFromResponse(processedResult, toolName, planStep.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to extract resource ID from MCP response for tool %s: %w", toolName, err)
	}

	// Update the plan step with the actual resource ID so it gets stored correctly
	planStep.ResourceID = resourceID

	// Store the mapping of plan step ID to actual resource ID and all array field mappings
	a.storeResourceMapping(planStep.ID, resourceID, processedResult)

	// Wait for resource to be ready if it has dependencies
	if err := a.waitForResourceReady(toolName, resourceID); err != nil {
		a.Logger.WithError(err).WithFields(map[string]interface{}{
			"step_id":     planStep.ID,
			"tool_name":   toolName,
			"resource_id": resourceID,
		}).Error("Failed to wait for resource to be ready")
		return nil, fmt.Errorf("resource %s not ready: %w", resourceID, err)
	}

	// Update state manager with the new resource (use processed result)
	if err := a.updateStateFromMCPResult(planStep, processedResult); err != nil {
		a.Logger.WithError(err).WithFields(map[string]interface{}{
			"step_id":     planStep.ID,
			"tool_name":   toolName,
			"resource_id": resourceID,
			"result":      processedResult,
		}).Error("CRITICAL: Failed to update state after resource creation - this may cause state inconsistency")

		// Still continue execution but ensure this is visible
		return nil, fmt.Errorf("failed to update state after creating resource %s: %w", resourceID, err)
	}

	// Create result map for return
	resultMap := map[string]interface{}{
		"resource_id":  resourceID,
		"plan_step_id": planStep.ID,
		"mcp_tool":     toolName,
		"mcp_response": processedResult, // Use processed result with concatenated arrays
	}

	return resultMap, nil
}

// // executeUpdateAction handles resource updates using real MCP tools
// func (a *StateAwareAgent) executeUpdateAction(_ context.Context, planStep *types.ExecutionPlanStep, progressChan chan<- *types.ExecutionUpdate, executionID string) (map[string]interface{}, error) {
// 	// Send progress update
// 	if progressChan != nil {
// 		progressChan <- &types.ExecutionUpdate{
// 			Type:        "step_progress",
// 			ExecutionID: executionID,
// 			StepID:      planStep.ID,
// 			Message:     fmt.Sprintf("Updating %s resource: %s", planStep.ResourceID, planStep.Name),
// 			Timestamp:   time.Now(),
// 		}
// 	}

// 	// For update actions, we mainly just simulate for now since the focus is on create operations
// 	// The native MCP approach will be extended to update/delete actions in future iterations
// 	a.Logger.WithField("step_id", planStep.ID).Info("Simulating update action as focus is on create operations")
// 	time.Sleep(time.Second * 1)
// 	return map[string]interface{}{
// 		"resource_id": planStep.ResourceID,
// 		"status":      "updated",
// 		"message":     fmt.Sprintf("%s updated successfully (simulated)", planStep.Name),
// 		"changes":     planStep.Parameters,
// 		"simulated":   true,
// 	}, nil
// }

// // executeDeleteAction handles resource deletion
// func (a *StateAwareAgent) executeDeleteAction(planStep *types.ExecutionPlanStep, progressChan chan<- *types.ExecutionUpdate, executionID string) (map[string]interface{}, error) {
// 	// Send progress update
// 	if progressChan != nil {
// 		progressChan <- &types.ExecutionUpdate{
// 			Type:        "step_progress",
// 			ExecutionID: executionID,
// 			StepID:      planStep.ID,
// 			Message:     fmt.Sprintf("Deleting %s resource: %s", planStep.ResourceID, planStep.Name),
// 			Timestamp:   time.Now(),
// 		}
// 	}

// 	// Simulate resource deletion
// 	time.Sleep(time.Second * 1)

// 	return map[string]interface{}{
// 		"resource_id": planStep.ResourceID,
// 		"status":      "deleted",
// 		"message":     fmt.Sprintf("%s deleted successfully", planStep.Name),
// 	}, nil
// }

// // executeValidateAction handles validation steps using real MCP tools where possible
// func (a *StateAwareAgent) executeValidateAction(planStep *types.ExecutionPlanStep, progressChan chan<- *types.ExecutionUpdate, executionID string) (map[string]interface{}, error) {
// 	// Send progress update
// 	if progressChan != nil {
// 		progressChan <- &types.ExecutionUpdate{
// 			Type:        "step_progress",
// 			ExecutionID: executionID,
// 			StepID:      planStep.ID,
// 			Message:     fmt.Sprintf("Validating %s: %s", planStep.ResourceID, planStep.Name),
// 			Timestamp:   time.Now(),
// 		}
// 	}

// 	// For validation actions, we mainly just simulate for now since the focus is on create operations
// 	// The native MCP approach will be extended to validation actions in future iterations
// 	a.Logger.WithField("step_id", planStep.ID).Info("Simulating validation action as focus is on create operations")
// 	time.Sleep(time.Millisecond * 500)
// 	return map[string]interface{}{
// 		"resource_id": planStep.ResourceID,
// 		"status":      "validated",
// 		"message":     fmt.Sprintf("%s validation completed (simulated)", planStep.Name),
// 		"checks":      []string{"basic_validation"},
// 	}, nil
// }

// updateStateFromMCPResult updates the state manager with results from MCP operations
func (a *StateAwareAgent) updateStateFromMCPResult(planStep *types.ExecutionPlanStep, result map[string]interface{}) error {
	a.Logger.WithFields(map[string]interface{}{
		"step_id":      planStep.ID,
		"step_name":    planStep.Name,
		"resource_id":  planStep.ResourceID,
		"mcp_response": result,
	}).Info("Starting state update from MCP result")

	// Note: result is already processed (arrays concatenated) from executeNativeMCPTool
	// Create a simple properties map from MCP result
	resultData := map[string]interface{}{
		"mcp_response": result,
		"status":       "created_via_mcp",
	}

	// Extract resource type
	resourceType := a.extractResourceTypeFromStep(planStep)

	// Create a resource state entry
	resourceState := &types.ResourceState{
		ID:           planStep.ResourceID,
		Name:         planStep.Name,
		Description:  planStep.Description,
		Type:         resourceType,
		Status:       "created",
		Properties:   resultData,
		Dependencies: planStep.DependsOn,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	a.Logger.WithFields(map[string]interface{}{
		"step_id":           planStep.ID,
		"resource_state_id": resourceState.ID,
		"resource_type":     resourceState.Type,
		"dependencies":      resourceState.Dependencies,
	}).Info("Calling AddResourceToState for main AWS resource")

	// Add to state manager via MCP server
	if err := a.AddResourceToState(resourceState); err != nil {
		a.Logger.WithError(err).WithFields(map[string]interface{}{
			"step_id":           planStep.ID,
			"resource_state_id": resourceState.ID,
			"resource_type":     resourceState.Type,
		}).Error("Failed to add resource to state via MCP server")
		return fmt.Errorf("failed to add resource %s to state: %w", resourceState.ID, err)
	}

	a.Logger.WithFields(map[string]interface{}{
		"step_id":           planStep.ID,
		"resource_state_id": resourceState.ID,
		"resource_type":     resourceState.Type,
	}).Info("Successfully added main AWS resource to state")

	// Also store the resource with the step ID as the key for dependency resolution
	// This ensures that {{step-create-xxx.resourceId}} references can be resolved
	if planStep.ID != planStep.ResourceID {

		stepResourceState := &types.ResourceState{
			ID:           planStep.ID, // Use step ID as the key
			Name:         planStep.Name,
			Description:  planStep.Description + " (Step Reference)",
			Type:         "step_reference",
			Status:       "created",
			Properties:   resultData,
			Dependencies: planStep.DependsOn,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		if err := a.AddResourceToState(stepResourceState); err != nil {
			if a.config.EnableDebug {
				a.Logger.WithError(err).WithFields(map[string]interface{}{
					"step_id":          planStep.ID,
					"step_resource_id": stepResourceState.ID,
				}).Warn("Failed to add step-based resource to state - dependency resolution may be affected")
			}
			// Don't fail the whole operation for this, just log the warning
		} else {
			a.Logger.WithFields(map[string]interface{}{
				"step_id":          planStep.ID,
				"step_resource_id": stepResourceState.ID,
				"type":             "step_reference",
			}).Info("Successfully added step reference to state for dependency resolution")
		}
	} else {
		a.Logger.WithFields(map[string]interface{}{
			"step_id":     planStep.ID,
			"resource_id": planStep.ResourceID,
			"reason":      "step_id equals resource_id",
		}).Info("Skipping step reference creation - not needed for dependency resolution")
	}

	a.Logger.WithFields(map[string]interface{}{
		"step_id":                planStep.ID,
		"main_resource_id":       resourceState.ID,
		"main_resource_type":     resourceState.Type,
		"step_reference_created": planStep.ID != planStep.ResourceID,
	}).Info("Successfully completed state update from MCP result")

	return nil
}

// Helper function to extract resource type from plan step
func (a *StateAwareAgent) extractResourceTypeFromStep(planStep *types.ExecutionPlanStep) string {
	// First try the resource_type parameter
	if rt, exists := planStep.Parameters["resource_type"]; exists {
		if rtStr, ok := rt.(string); ok {
			return rtStr
		}
	}

	// Try to infer from MCP tool name using pattern matcher
	if planStep.MCPTool != "" {
		resourceType := a.patternMatcher.IdentifyResourceTypeFromToolName(planStep.MCPTool)
		if resourceType != "" && resourceType != "unknown" {
			return resourceType
		}
	}

	// Try to infer from ResourceID field using pattern matcher
	if planStep.ResourceID != "" {
		// Use pattern matcher to identify resource type from ID
		resourceType := a.patternMatcher.IdentifyResourceType(planStep)
		if resourceType != "" && resourceType != "unknown" {
			return resourceType
		}
	}

	// Try to infer from step name or description using pattern matcher
	resourceType := a.patternMatcher.InferResourceTypeFromDescription(planStep.Name + " " + planStep.Description)
	if resourceType != "" && resourceType != "unknown" {
		return resourceType
	}

	return ""
}

// getAvailableToolsContext returns a formatted string of available tools for the AI to understand
func (a *StateAwareAgent) getAvailableToolsContext() (string, error) {
	a.capabilityMutex.RLock()
	toolsCount := len(a.mcpTools)
	a.capabilityMutex.RUnlock()

	if toolsCount == 0 {
		// Try to ensure capabilities are available
		if err := a.ensureMCPCapabilities(); err != nil {
			a.Logger.WithError(err).Error("Failed to ensure MCP capabilities in getAvailableToolsContext")
			return "", fmt.Errorf("failed to ensure MCP capabilities: %w", err)
		}

		// Re-check after ensuring capabilities
		a.capabilityMutex.RLock()
		toolsCount = len(a.mcpTools)
		a.capabilityMutex.RUnlock()
	}

	if toolsCount == 0 {
		return "", fmt.Errorf("no MCP tools discovered - MCP server may not be properly initialized")
	}

	// Generate dynamic MCP tools schema
	mcpToolsSchema := a.generateMCPToolsSchema()

	// Load template with MCP tools placeholder
	placeholders := map[string]string{
		"MCP_TOOLS_SCHEMAS": mcpToolsSchema,
	}

	// Use the new template-based approach
	executionContext, err := a.loadTemplateWithPlaceholders("settings/templates/tools-execution-context-optimized.txt", placeholders)
	if err != nil {
		a.Logger.WithError(err).Error("Failed to load tools execution template with placeholders")
		return "", fmt.Errorf("failed to load tools execution template: %w", err)
	}

	return executionContext, nil
}

// generateMCPToolsSchema generates the dynamic MCP tools schema section
func (a *StateAwareAgent) generateMCPToolsSchema() string {
	a.capabilityMutex.RLock()
	defer a.capabilityMutex.RUnlock()

	var context strings.Builder
	context.WriteString("=== AVAILABLE MCP TOOLS WITH FULL SCHEMAS ===\n\n")
	context.WriteString("You have direct access to these MCP tools. Use the exact tool names and parameter structures shown below.\n\n")

	// Get available categories from pattern matcher configuration
	availableCategories := a.patternMatcher.GetAvailableCategories()

	// Initialize categories with empty slices
	categories := make(map[string][]string)
	for _, category := range availableCategories {
		categories[category] = []string{}
	}

	// Ensure "Other" category exists as fallback
	if _, exists := categories["Other"]; !exists {
		categories["Other"] = []string{}
	}

	toolDetails := make(map[string]string)

	// Categorize tools using pattern matcher
	for toolName, toolInfo := range a.mcpTools {
		// Use pattern matcher to get category based on tool name and resource type patterns
		category := a.patternMatcher.GetCategoryForTool(toolName)

		// Fallback to "Other" if category not found
		if _, exists := categories[category]; !exists {
			category = "Other"
		}

		// Build detailed tool schema
		var toolDetail strings.Builder
		toolDetail.WriteString(fmt.Sprintf("  TOOL: %s\n", toolName))
		toolDetail.WriteString(fmt.Sprintf("  Description: %s\n", toolInfo.Description))

		if toolInfo.InputSchema != nil {
			if properties, ok := toolInfo.InputSchema["properties"].(map[string]interface{}); ok {
				toolDetail.WriteString("  Parameters:\n")

				// Get required fields
				requiredFields := make(map[string]bool)
				if required, ok := toolInfo.InputSchema["required"].([]interface{}); ok {
					for _, field := range required {
						if fieldStr, ok := field.(string); ok {
							requiredFields[fieldStr] = true
						}
					}
				}

				for paramName, paramSchema := range properties {
					if paramSchemaMap, ok := paramSchema.(map[string]interface{}); ok {
						requiredMark := ""
						if requiredFields[paramName] {
							requiredMark = " (REQUIRED)"
						}

						paramType := "string"
						if pType, exists := paramSchemaMap["type"]; exists {
							paramType = fmt.Sprintf("%v", pType)
						}

						description := ""
						if desc, exists := paramSchemaMap["description"]; exists {
							description = fmt.Sprintf(" - %v", desc)
						}

						toolDetail.WriteString(fmt.Sprintf("    - %s: %s%s%s\n", paramName, paramType, requiredMark, description))
					}
				}
			}
		}
		toolDetail.WriteString("\n")

		categories[category] = append(categories[category], toolName)
		toolDetails[toolName] = toolDetail.String()
	}

	// Write categorized tools with full schemas
	for category, tools := range categories {
		if len(tools) > 0 {
			context.WriteString(fmt.Sprintf("=== %s ===\n\n", category))
			for _, toolName := range tools {
				context.WriteString(toolDetails[toolName])
			}
		}
	}

	return context.String()
}

// persistCurrentState saves the current infrastructure state to persistent storage
// This ensures that successfully completed steps are not lost if later steps fail
func (a *StateAwareAgent) persistCurrentState() error {
	a.Logger.Info("Starting state persistence via MCP server")

	// Use MCP server to save the current state
	result, err := a.callMCPTool("save-state", map[string]interface{}{
		"force": true, // Force save even if state hasn't changed much
	})
	if err != nil {
		a.Logger.WithError(err).Error("Failed to call save-state MCP tool")
		return fmt.Errorf("failed to save state via MCP: %w", err)
	}

	a.Logger.WithField("result", result).Info("State persistence completed successfully via MCP server")
	return nil
}

// extractResourceIDFromResponse extracts the actual AWS resource ID from MCP response
// Note: result should be the processed result with concatenated arrays
func (a *StateAwareAgent) extractResourceIDFromResponse(result map[string]interface{}, toolName string, stepID string) (string, error) {
	// Use configuration-driven extraction for primary resource ID
	resourceType := a.patternMatcher.IdentifyResourceTypeFromToolName(toolName)
	if resourceType == "" || resourceType == "unknown" {
		return "", fmt.Errorf("could not identify resource type for tool %s", toolName)
	}

	extractedID, err := a.idExtractor.ExtractResourceID(toolName, resourceType, nil, result)
	if err != nil {
		a.Logger.WithFields(map[string]interface{}{
			"tool_name":     toolName,
			"resource_type": resourceType,
			"error":         err.Error(),
		}).Error("Failed to extract resource ID using configuration-driven approach")
		return "", fmt.Errorf("could not extract resource ID from MCP response for tool %s: %w", toolName, err)
	}

	if extractedID == "" {
		return "", fmt.Errorf("extracted empty resource ID for tool %s", toolName)
	}

	if a.config.EnableDebug {
		a.Logger.WithFields(map[string]interface{}{
			"tool_name":     toolName,
			"resource_type": resourceType,
			"resource_id":   extractedID,
			"step_id":       stepID,
		}).Info("Successfully extracted resource ID")
	}

	return extractedID, nil
}

// waitForResourceReady waits for AWS resources to be in a ready state before continuing
func (a *StateAwareAgent) waitForResourceReady(toolName, resourceID string) error {
	if a.testMode {
		return nil
	}

	// Determine if this resource type needs waiting
	needsWaiting := false
	maxWaitTime := 5 * time.Minute
	checkInterval := 15 * time.Second

	switch toolName {
	case "create-nat-gateway":
		needsWaiting = true
		maxWaitTime = 5 * time.Minute // NAT gateways typically take 2-3 minutes
	case "create-rds-db-instance", "create-database":
		needsWaiting = true
		maxWaitTime = 15 * time.Minute // RDS instances can take longer
	case "create-internet-gateway", "create-vpc", "create-subnet":
		// These are typically available immediately
		needsWaiting = false
	default:
		// For other resources, don't wait
		needsWaiting = false
	}

	if !needsWaiting {
		a.Logger.WithFields(map[string]interface{}{
			"tool_name":   toolName,
			"resource_id": resourceID,
		}).Debug("Resource type does not require waiting")
		return nil
	}

	a.Logger.WithFields(map[string]interface{}{
		"tool_name":      toolName,
		"resource_id":    resourceID,
		"max_wait_time":  maxWaitTime,
		"check_interval": checkInterval,
	}).Info("Waiting for resource to be ready")

	startTime := time.Now()
	timeout := time.After(maxWaitTime)
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			elapsed := time.Since(startTime)
			return fmt.Errorf("timeout waiting for %s %s to be ready after %v", toolName, resourceID, elapsed)

		case <-ticker.C:
			ready, err := a.checkResourceState(toolName, resourceID)
			if err != nil {
				a.Logger.WithError(err).WithFields(map[string]interface{}{
					"tool_name":   toolName,
					"resource_id": resourceID,
				}).Warn("Error checking resource state, will retry")
				continue
			}

			if ready {
				elapsed := time.Since(startTime)
				a.Logger.WithFields(map[string]interface{}{
					"tool_name":   toolName,
					"resource_id": resourceID,
					"elapsed":     elapsed,
				}).Info("Resource is ready")
				return nil
			}

			elapsed := time.Since(startTime)
			a.Logger.WithFields(map[string]interface{}{
				"tool_name":   toolName,
				"resource_id": resourceID,
				"elapsed":     elapsed,
			}).Debug("Resource not ready yet, continuing to wait")
		}
	}
}

// checkResourceState checks if a specific AWS resource is in a ready state
func (a *StateAwareAgent) checkResourceState(toolName, resourceID string) (bool, error) {
	switch toolName {
	case "create-nat-gateway":
		return a.checkNATGatewayState(resourceID)
	case "create-rds-db-instance", "create-database":
		return a.checkRDSInstanceState(resourceID)
	default:
		// For unknown resource types, assume they're ready
		return true, nil
	}
}

// checkNATGatewayState checks if a NAT gateway is available
func (a *StateAwareAgent) checkNATGatewayState(natGatewayID string) (bool, error) {
	// Try to use MCP tool to describe the NAT gateway if available
	result, err := a.callMCPTool("describe-nat-gateways", map[string]interface{}{
		"natGatewayIds": []string{natGatewayID},
	})
	if err != nil {
		// If describe tool is not available, use a simple time-based approach
		a.Logger.WithFields(map[string]interface{}{
			"nat_gateway_id": natGatewayID,
			"error":          err.Error(),
		}).Warn("describe-nat-gateways tool not available, using time-based wait")

		// NAT gateways typically take 2-3 minutes to become available
		// We'll wait a fixed amount of time and then assume it's ready
		time.Sleep(30 * time.Second)
		return true, nil
	}

	// Parse the response to check the state
	if natGateways, ok := result["natGateways"].([]interface{}); ok && len(natGateways) > 0 {
		if natGateway, ok := natGateways[0].(map[string]interface{}); ok {
			if state, ok := natGateway["state"].(string); ok {
				a.Logger.WithFields(map[string]interface{}{
					"nat_gateway_id": natGatewayID,
					"state":          state,
				}).Debug("NAT gateway state check")

				return state == "available", nil
			}
		}
	}

	return false, fmt.Errorf("could not determine NAT gateway state from response")
}

// checkRDSInstanceState checks if an RDS instance is available
func (a *StateAwareAgent) checkRDSInstanceState(dbInstanceID string) (bool, error) {
	// Try to use MCP tool to describe the RDS instance if available
	result, err := a.callMCPTool("describe-db-instances", map[string]interface{}{
		"dbInstanceIdentifier": dbInstanceID,
	})
	if err != nil {
		// If describe tool is not available, use a simple time-based approach
		a.Logger.WithFields(map[string]interface{}{
			"db_instance_id": dbInstanceID,
			"error":          err.Error(),
		}).Warn("describe-db-instances tool not available, using time-based wait")

		// RDS instances typically take 5-10 minutes to become available
		// We'll wait a fixed amount of time and then assume it's ready
		time.Sleep(60 * time.Second)
		return true, nil
	}

	// Parse the response to check the state
	if dbInstances, ok := result["dbInstances"].([]interface{}); ok && len(dbInstances) > 0 {
		if dbInstance, ok := dbInstances[0].(map[string]interface{}); ok {
			if status, ok := dbInstance["dbInstanceStatus"].(string); ok {
				a.Logger.WithFields(map[string]interface{}{
					"db_instance_id": dbInstanceID,
					"status":         status,
				}).Debug("RDS instance state check")

				return status == "available", nil
			}
		}
	}

	return false, fmt.Errorf("could not determine RDS instance state from response")
}

// processArraysInMCPResponse processes MCP response to concatenate array fields with _ delimiter
// This provides unified array handling where arrays are stored as "item1_item2_item3"
func (a *StateAwareAgent) processArraysInMCPResponse(result map[string]interface{}) map[string]interface{} {
	processed := make(map[string]interface{})

	for key, value := range result {
		// Check if value is an array of strings
		if arrValue, ok := value.([]interface{}); ok && len(arrValue) > 0 {
			// Check if array contains strings
			stringArray := make([]string, 0, len(arrValue))
			allStrings := true

			for _, item := range arrValue {
				if strItem, ok := item.(string); ok {
					stringArray = append(stringArray, strItem)
				} else {
					allStrings = false
					break
				}
			}

			// If it's an array of strings, concatenate with _
			if allStrings && len(stringArray) > 0 {
				concatenated := strings.Join(stringArray, "_")

				// Store both concatenated version and original array
				processed[key] = concatenated
				processed[key+"_array"] = arrValue // Keep original for other processing

				continue
			}
		}

		// For non-string-arrays, keep as-is
		processed[key] = value
	}

	return processed
}

// storeResourceMapping stores the mapping between plan step ID and actual AWS resource ID
// Also stores all array field mappings from the processed result for field-specific references
func (a *StateAwareAgent) storeResourceMapping(stepID, resourceID string, processedResult map[string]interface{}) {
	a.mappingsMutex.Lock()
	defer a.mappingsMutex.Unlock()

	// Store primary resource ID mapping
	a.resourceMappings[stepID] = resourceID

	// Store all array field mappings for field-specific references (e.g., {{step-id.zones}})
	for key, value := range processedResult {
		// Skip the original array fields (ending with _array)
		if strings.HasSuffix(key, "_array") {
			continue
		}

		// Store concatenated string values as step-id.fieldname mappings
		if strValue, ok := value.(string); ok && strValue != "" {
			// Check if this looks like a concatenated array (contains _)
			// or if there's a corresponding _array field
			if strings.Contains(strValue, "_") || processedResult[key+"_array"] != nil {
				mappingKey := fmt.Sprintf("%s.%s", stepID, key)
				a.resourceMappings[mappingKey] = strValue
			}
		}
	}
}

// StoreResourceMapping is a public wrapper for storeResourceMapping for external use
func (a *StateAwareAgent) StoreResourceMapping(stepID, resourceID string, processedResult map[string]interface{}) {
	a.storeResourceMapping(stepID, resourceID, processedResult)
}
