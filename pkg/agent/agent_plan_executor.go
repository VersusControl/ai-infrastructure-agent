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
//   - ExecutePlanWithReActRecovery()        : Execute plan with ReAct-based recovery on failures
//   - ConsultAIForPlanRecovery()            : Consult AI for plan recovery strategy
//   - ExecuteRecoveryStrategy()             : Execute AI-generated recovery strategy
//   - buildPlanFailureContext()             : Build context for plan failure analysis
//   - ExecuteConfirmedPlanWithDryRun()      : Execute confirmed plans with dry-run support
//   - SimulatePlanExecution()               : Simulate plan execution for dry-run mode
//   - executeExecutionStep()                : Execute individual plan steps
//   - executeCreateAction()                 : Execute create actions via MCP tools
//   - executeQueryAction()                  : Execute query actions via MCP tools
//   - executeModifyAction()                 : Execute modify actions on existing resources
//   - executeNativeMCPTool()                : Execute native MCP tool calls
//   - processArraysInMCPResponse()          : Process arrays in MCP tool responses
//   - storeResourceMapping()                : Store resource mapping (private)
//   - StoreResourceMapping()                : Store resource mapping (public)
//
// Usage Example:
//   1. execution, err := agent.ExecutePlanWithReActRecovery(ctx, decision, progressChan)
//   2. execution, err := agent.ExecuteConfirmedPlanWithDryRun(ctx, decision, progressChan, false)

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

	// Track the current plan to execute - starts with original, gets updated by recovery strategies
	var currentPlanToExecute []*types.ExecutionPlanStep = decision.ExecutionPlan

	// Attempt execution with recovery (up to MaxRecoveryAttempts)
	for attemptNumber := 1; attemptNumber <= config.MaxRecoveryAttempts; attemptNumber++ {

		// Execute the plan - use currentPlanToExecute which preserves recovery adjustments
		planToExecute := currentPlanToExecute
		if attemptNumber > 1 {
			a.Logger.WithFields(map[string]interface{}{
				"attempt_number": attemptNumber,
				"plan_steps":     len(planToExecute),
			}).Info("This is a recovery attempt - using plan from previous recovery strategy")
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

			// CRITICAL: Update currentPlanToExecute with the recovery strategy's plan
			// This preserves completed step statuses for the next recovery attempt
			currentPlanToExecute = recoveryStrategy.ExecutionPlan
			a.Logger.WithFields(map[string]interface{}{
				"execution_id":   execution.ID,
				"attempt_number": attemptNumber,
				"plan_steps":     len(currentPlanToExecute),
				"completed_steps": func() int {
					count := 0
					for _, step := range currentPlanToExecute {
						if step.Status == "completed" {
							count++
						}
					}
					return count
				}(),
			}).Info("Recovery failed but preserving recovery plan with completed steps for next attempt")

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
	case "modify":
		// Modify action - updates existing resources (only works on managed resources)
		result, err = a.executeModifyAction(planStep, progressChan, execution.ID)
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

// executeModifyAction handles resource modification operations using native MCP tool calls
// This action ONLY updates existing resources that are managed by the AI Agent.
// For security reasons, it will reject attempts to modify resources not in the managed state.
func (a *StateAwareAgent) executeModifyAction(planStep *types.ExecutionPlanStep, progressChan chan<- *types.ExecutionUpdate, executionID string) (map[string]interface{}, error) {
	// Send progress update
	if progressChan != nil {
		progressChan <- &types.ExecutionUpdate{
			Type:        "step_progress",
			ExecutionID: executionID,
			StepID:      planStep.ID,
			Message:     fmt.Sprintf("Modifying resource: %s", planStep.Name),
			Timestamp:   time.Now(),
		}
	}

	// Log modify operation
	a.Logger.WithFields(map[string]interface{}{
		"step_id":     planStep.ID,
		"resource_id": planStep.ResourceID,
		"mcp_tool":    planStep.MCPTool,
		"action":      "modify",
	}).Info("Executing modify action via MCP tool")

	// Execute the MCP tool (modify tools handle exists check internally)
	result, err := a.executeNativeMCPTool(planStep, progressChan, executionID)

	if err != nil {
		return nil, fmt.Errorf("modify operation failed: %w", err)
	}

	// Update state from result using modify-specific logic (update existing resources only)
	if err := a.updateStateFromMCPResult(planStep, result); err != nil {
		return nil, fmt.Errorf("failed to update state after modify operation: %w", err)
	}

	// Store resource mapping for dependency resolution using configuration-driven extraction
	// The result from executeNativeMCPTool is wrapped, so unwrap the mcp_response
	var mcpResponse map[string]interface{}
	if wrapped, ok := result["mcp_response"].(map[string]interface{}); ok {
		mcpResponse = wrapped
	} else {
		// Use result directly if not wrapped
		mcpResponse = result
	}

	// Process arrays first to get concatenated values
	processedResult := a.processArraysInMCPResponse(mcpResponse)

	// Extract resource ID using the same configuration-driven approach as create action
	resourceID, err := a.extractResourceIDFromResponse(processedResult, planStep.MCPTool, planStep.ID)

	if err != nil {
		return nil, fmt.Errorf("failed to extract resource ID after modify operation: %w", err)
	}

	a.storeResourceMapping(planStep.ID, resourceID, processedResult)

	return result, nil
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
	if err := a.addStateFromMCPResult(planStep, processedResult); err != nil {
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
