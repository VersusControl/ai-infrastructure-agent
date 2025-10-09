package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// ========== StateAwareAgent Extensions for ReAct Recovery ==========

// ExecuteStepWithRecovery executes a step with automatic ReAct-style recovery on failure
func (a *StateAwareAgent) ExecuteStepWithRecovery(
	ctx context.Context,
	planStep *types.ExecutionPlanStep,
	execution *types.PlanExecution,
	progressChan chan<- *types.ExecutionUpdate,
	strategy *RecoveryStrategy,
) (*types.ExecutionStep, error) {
	return a.ExecuteStepWithRecoveryAndCoordinator(ctx, planStep, execution, progressChan, strategy, nil)
}

// ExecuteStepWithRecoveryAndCoordinator executes a step with ReAct-style recovery and UI coordination
func (a *StateAwareAgent) ExecuteStepWithRecoveryAndCoordinator(
	ctx context.Context,
	planStep *types.ExecutionPlanStep,
	execution *types.PlanExecution,
	progressChan chan<- *types.ExecutionUpdate,
	strategy *RecoveryStrategy,
	coordinator RecoveryCoordinator,
) (*types.ExecutionStep, error) {

	// Initialize recovery components
	recoveryEngine := NewStepRecoveryEngine(a)
	contextBuilder := NewFailureContextBuilder(a)

	// Set default strategy if none provided
	if strategy == nil {
		strategy = &RecoveryStrategy{
			MaxAttempts:          3,
			EnableAIConsultation: true,
			AllowToolSwapping:    true,
			AllowParameterMod:    true,
			TimeoutPerAttempt:    5 * time.Minute,
		}
	}

	// First attempt - try normal execution
	a.Logger.WithFields(map[string]interface{}{
		"step_id": planStep.ID,
		"attempt": 1,
	}).Info("Attempting step execution")

	// Note: step_started message is already sent by the main execution loop

	startTime := time.Now()
	step, err := a.executeExecutionStep(planStep, execution, progressChan)

	if err == nil {
		// Success on first attempt - send completion message
		if progressChan != nil {
			progressChan <- &types.ExecutionUpdate{
				Type:        "step_completed",
				ExecutionID: execution.ID,
				StepID:      planStep.ID,
				Message:     fmt.Sprintf("Step completed successfully: %s", planStep.Name),
				Timestamp:   time.Now(),
			}
		}
		return step, nil
	}

	// Step failed - if coordinator is available, request user input for recovery
	if coordinator != nil && strategy.EnableAIConsultation {
		a.Logger.WithField("step_id", planStep.ID).Info("Step failed, requesting recovery decision from user")

		// Send step failed message
		if progressChan != nil {
			progressChan <- &types.ExecutionUpdate{
				Type:        "step_failed",
				ExecutionID: execution.ID,
				StepID:      planStep.ID,
				Message:     fmt.Sprintf("Step failed: %v", err),
				Error:       err.Error(),
				Timestamp:   time.Now(),
			}
		}

		// Build failure context for AI consultation
		executionPlan := []*types.ExecutionPlanStep{planStep} // Simplified for now

		failureContext, contextErr := contextBuilder.BuildFailureContext(
			ctx,
			planStep,
			err,
			execution,
			executionPlan,
			1,
			[]*StepRecoveryAttempt{},
		)

		if contextErr != nil {
			a.Logger.WithError(contextErr).Warn("Failed to build failure context")
			return nil, err // Return original error
		}

		// Notify UI that recovery analysis is starting
		if progressChan != nil {
			progressChan <- &types.ExecutionUpdate{
				Type:        "step_recovery_generating",
				ExecutionID: execution.ID,
				StepID:      planStep.ID,
				Message:     "Generating recovery plan using AI analysis...",
				Timestamp:   time.Now(),
			}
		}

		// Get AI recovery analysis
		aiAnalysis, aiErr := recoveryEngine.AnalyzeFailure(ctx, failureContext)
		if aiErr != nil {
			a.Logger.WithError(aiErr).Warn("Failed to get AI recovery analysis")
			return nil, err // Return original error
		}

		// Convert to UI format
		uiFailureContext := map[string]interface{}{
			"stepName":     planStep.Name,
			"toolName":     planStep.MCPTool,
			"action":       planStep.Action,
			"errorMessage": err.Error(),
			"aiAnalysis": map[string]interface{}{
				"rootCause":             aiAnalysis.FailureReason,
				"recommendation":        aiAnalysis.RecommendedAction,
				"alternativeApproaches": []string{aiAnalysis.AlternativeApproach},
			},
		}

		// Convert AI recovery options to UI format
		var uiRecoveryOptions []map[string]interface{}
		for _, option := range aiAnalysis.RecoveryOptions {
			uiOption := map[string]interface{}{
				"action":             option.Action,
				"successProbability": option.SuccessProbability,
				"riskLevel":          option.RiskLevel,
				"reasoning":          option.Reasoning,
				"newTool":            option.ToolName,
				"modifiedParameters": option.Parameters,
			}

			// Include multi-step plan data for UI display
			if option.Action == "multi_step_recovery" && len(option.MultiStepPlan) > 0 {
				var uiSteps []map[string]interface{}
				for _, step := range option.MultiStepPlan {
					uiSteps = append(uiSteps, map[string]interface{}{
						"stepOrder":  step.StepOrder,
						"toolName":   step.ToolName,
						"parameters": step.Parameters,
						"purpose":    step.Purpose,
					})
				}
				uiOption["multiStepPlan"] = uiSteps
				uiOption["totalSteps"] = len(option.MultiStepPlan)
			}

			uiRecoveryOptions = append(uiRecoveryOptions, uiOption)
		}

		// Use AI analysis recovery options directly - no hardcoded fallbacks
		if len(uiRecoveryOptions) == 0 {
			a.Logger.WithField("step_id", planStep.ID).Warn("AI provided no recovery options - this should be handled by the AI model")
			return nil, fmt.Errorf("AI analysis did not provide recovery options for step %s", planStep.ID)
		}

		// Request recovery decision from user
		userDecision, reqErr := coordinator.RequestRecoveryDecision(planStep.ID, uiFailureContext, uiRecoveryOptions)
		if reqErr != nil {
			a.Logger.WithError(reqErr).Error("Failed to get recovery decision from user")
			return nil, fmt.Errorf("recovery coordination failed: %w", reqErr)
		}

		// Check if user aborted
		if abort, exists := userDecision["abort"]; exists && abort.(bool) {
			return nil, fmt.Errorf("execution aborted by user during recovery")
		}

		// Apply user's recovery choice
		selectedIndex, exists := userDecision["selectedOptionIndex"]
		if !exists {
			return nil, fmt.Errorf("no recovery option selected")
		}

		indexStr := selectedIndex.(string)
		if indexStr == "skip" {
			a.Logger.WithField("step_id", planStep.ID).Info("User chose to skip step")

			if progressChan != nil {
				progressChan <- &types.ExecutionUpdate{
					Type:        "step_skipped",
					ExecutionID: execution.ID,
					StepID:      planStep.ID,
					Message:     "Step skipped by user",
					Timestamp:   time.Now(),
				}
			}

			// Return a mock successful step for skipped steps
			now := time.Now()
			return &types.ExecutionStep{
				ID:          planStep.ID,
				Name:        planStep.Name,
				Status:      "skipped",
				Resource:    planStep.ResourceID,
				Action:      planStep.Action,
				StartedAt:   &startTime,
				CompletedAt: &now,
				Output:      map[string]interface{}{"status": "skipped", "reason": "user_request"},
			}, nil
		}

		// Apply the selected recovery option
		if progressChan != nil {
			progressChan <- &types.ExecutionUpdate{
				Type:        "step_recovery_started",
				ExecutionID: execution.ID,
				StepID:      planStep.ID,
				Message:     "Applying recovery strategy...",
				Timestamp:   time.Now(),
			}
		}

		// Apply recovery modifications to the planStep
		modifiedStep, modifyErr := a.applyRecoveryOption(planStep, aiAnalysis.RecoveryOptions, indexStr)
		if modifyErr != nil {
			a.Logger.WithError(modifyErr).Error("Failed to apply recovery option")
			return nil, fmt.Errorf("recovery option application failed: %w", modifyErr)
		}

		// Execute recovery based on AI recommendations
		retryStep, retryErr := a.executeMultiStepRecovery(ctx, modifiedStep, execution, progressChan)
		if retryErr == nil {
			if progressChan != nil {
				progressChan <- &types.ExecutionUpdate{
					Type:        "step_recovery_completed",
					ExecutionID: execution.ID,
					StepID:      planStep.ID,
					Message:     "Recovery successful",
					Timestamp:   time.Now(),
				}
				// Also send step_completed to properly update UI status
				progressChan <- &types.ExecutionUpdate{
					Type:        "step_completed",
					ExecutionID: execution.ID,
					StepID:      planStep.ID,
					Message:     fmt.Sprintf("Step completed via recovery: %s", planStep.Name),
					Timestamp:   time.Now(),
				}
			}
			return retryStep, nil
		} else {
			if progressChan != nil {
				progressChan <- &types.ExecutionUpdate{
					Type:        "step_recovery_failed",
					ExecutionID: execution.ID,
					StepID:      planStep.ID,
					Message:     fmt.Sprintf("Recovery failed: %v", retryErr),
					Error:       retryErr.Error(),
					Timestamp:   time.Now(),
				}
			}
			return nil, retryErr
		}
	}

	// Fallback to original behavior if no coordinator
	return a.executeExecutionStep(planStep, execution, progressChan)
}

// ConsultAIForRecovery asks the AI model for recovery advice
func (a *StateAwareAgent) ConsultAIForRecovery(ctx context.Context, failureContext *StepFailureContext) (*AIRecoveryAnalysis, error) {
	recoveryEngine := NewStepRecoveryEngine(a)
	return recoveryEngine.AnalyzeFailure(ctx, failureContext)
}

// ApplyRecoveryAction executes the chosen recovery action
func (a *StateAwareAgent) ApplyRecoveryAction(ctx context.Context, action *StepRecoveryResult, failureContext *StepFailureContext) (*types.ExecutionStep, error) {
	recoveryEngine := NewStepRecoveryEngine(a)

	// Execute the recovery action and get the result
	result, err := recoveryEngine.(*DefaultStepRecoveryEngine).executeRecoveryAction(ctx, action, failureContext)
	if err != nil {
		return nil, fmt.Errorf("recovery action execution failed: %w", err)
	}

	// Convert the recovery result to an execution step
	if result.Success {
		now := time.Now()
		step := &types.ExecutionStep{
			ID:          failureContext.OriginalStep.ID,
			Name:        failureContext.OriginalStep.Name,
			Status:      "completed",
			Resource:    failureContext.OriginalStep.ResourceID,
			Action:      failureContext.OriginalStep.Action,
			StartedAt:   &now,
			CompletedAt: &now,
			Duration:    0, // Recovery timing would need to be tracked separately
			Output:      result.Output,
		}
		return step, nil
	} else {
		return nil, fmt.Errorf("recovery action failed: %s", result.Reasoning)
	}
}

// GetAvailableToolsForRecovery gets all available tools that could be used for recovery
func (a *StateAwareAgent) GetAvailableToolsForRecovery() []MCPToolInfo {
	a.capabilityMutex.RLock()
	defer a.capabilityMutex.RUnlock()

	var tools []MCPToolInfo
	for _, tool := range a.mcpTools {
		tools = append(tools, tool)
	}

	return tools
}

// UpdateResourceMapping updates the resource mapping after a successful recovery
func (a *StateAwareAgent) UpdateResourceMapping(stepID string, resourceID string) {
	a.mappingsMutex.Lock()
	defer a.mappingsMutex.Unlock()

	a.resourceMappings[stepID] = resourceID

	a.Logger.WithFields(map[string]interface{}{
		"step_id":     stepID,
		"resource_id": resourceID,
	}).Debug("Updated resource mapping after recovery")
}

// applyRecoveryOption applies the AI-recommended recovery option to the execution step
func (a *StateAwareAgent) applyRecoveryOption(originalStep *types.ExecutionPlanStep, recoveryOptions []*RecoveryOption, selectedIndex string) (*types.ExecutionPlanStep, error) {
	// Parse the selected index
	if selectedIndex == "skip" {
		return originalStep, nil // Should be handled upstream
	}

	// Convert string index to integer
	var optionIndex int
	switch selectedIndex {
	case "0":
		optionIndex = 0
	case "1":
		optionIndex = 1
	case "2":
		optionIndex = 2
	case "3":
		optionIndex = 3
	case "4":
		optionIndex = 4
	default:
		return nil, fmt.Errorf("invalid recovery option index: %s", selectedIndex)
	}

	if optionIndex >= len(recoveryOptions) {
		return nil, fmt.Errorf("recovery option index %d out of range (max %d)", optionIndex, len(recoveryOptions)-1)
	}

	selectedOption := recoveryOptions[optionIndex]

	a.Logger.WithFields(map[string]interface{}{
		"step_id":      originalStep.ID,
		"ai_action":    selectedOption.Action,
		"ai_tool":      selectedOption.ToolName,
		"ai_reasoning": selectedOption.Reasoning,
		"success_prob": selectedOption.SuccessProbability,
		"risk_level":   selectedOption.RiskLevel,
	}).Info("Applying AI recovery recommendation")

	// Create a copy of the original step to modify
	modifiedStep := &types.ExecutionPlanStep{
		ID:                originalStep.ID,
		Name:              originalStep.Name,
		Description:       originalStep.Description,
		Action:            originalStep.Action,
		ResourceID:        originalStep.ResourceID,
		MCPTool:           originalStep.MCPTool,
		ToolParameters:    make(map[string]interface{}),
		Parameters:        make(map[string]interface{}),
		DependsOn:         originalStep.DependsOn,
		EstimatedDuration: originalStep.EstimatedDuration,
		Status:            originalStep.Status,
	}

	// Copy original parameters first
	for k, v := range originalStep.Parameters {
		modifiedStep.Parameters[k] = v
	}
	for k, v := range originalStep.ToolParameters {
		modifiedStep.ToolParameters[k] = v
	}

	// Handle multi-step recovery
	if selectedOption.Action == "multi_step_recovery" && len(selectedOption.MultiStepPlan) > 0 {
		// Store the original step and multi-step plan in the modified step for later execution
		modifiedStep.Parameters["_original_step"] = originalStep
		modifiedStep.Parameters["_multi_step_plan"] = selectedOption.MultiStepPlan
		modifiedStep.Parameters["_current_multi_step"] = 0 // Start with first step

		// Set up the first step in the multi-step plan
		firstStep := selectedOption.MultiStepPlan[0]
		modifiedStep.MCPTool = firstStep.ToolName
		modifiedStep.Name = fmt.Sprintf("%s (Multi-step 1/%d: %s)",
			originalStep.Name, len(selectedOption.MultiStepPlan), firstStep.Purpose)

		// Clear and set parameters for the first step
		modifiedStep.ToolParameters = make(map[string]interface{})
		for k, v := range firstStep.Parameters {
			modifiedStep.ToolParameters[k] = v
		}

		return modifiedStep, nil
	}

	// Apply AI-recommended tool change if specified
	if selectedOption.ToolName != "" && selectedOption.ToolName != originalStep.MCPTool {
		modifiedStep.MCPTool = selectedOption.ToolName
		modifiedStep.Name = fmt.Sprintf("%s (via %s)", originalStep.Name, selectedOption.ToolName)

		// Clear existing parameters when switching tools (AI should provide complete new parameter set)
		modifiedStep.ToolParameters = make(map[string]interface{})
		modifiedStep.Parameters = make(map[string]interface{})
	}

	// Apply AI-recommended parameters directly
	if selectedOption.Parameters != nil {
		for k, v := range selectedOption.Parameters {
			modifiedStep.ToolParameters[k] = v
			modifiedStep.Parameters[k] = v // Maintain compatibility with both parameter systems
		}
	}

	// Update step description with AI reasoning
	if selectedOption.Reasoning != "" {
		modifiedStep.Description = fmt.Sprintf("%s (AI Recovery: %s)",
			originalStep.Description, selectedOption.Reasoning)
	}

	a.Logger.WithFields(map[string]interface{}{
		"step_id":          originalStep.ID,
		"ai_action":        selectedOption.Action,
		"final_tool":       modifiedStep.MCPTool,
		"param_count":      len(modifiedStep.Parameters),
		"tool_param_count": len(modifiedStep.ToolParameters),
	}).Info("Successfully applied AI recovery option")

	return modifiedStep, nil
}

// executeMultiStepRecovery executes single-step or multi-step recovery based on the modified step
func (a *StateAwareAgent) executeMultiStepRecovery(ctx context.Context, modifiedStep *types.ExecutionPlanStep, execution *types.PlanExecution, progressChan chan<- *types.ExecutionUpdate) (*types.ExecutionStep, error) {
	// Check if this is a multi-step recovery
	if multiStepPlan, exists := modifiedStep.Parameters["_multi_step_plan"]; exists {
		return a.executeMultiStepRecoveryPlan(ctx, modifiedStep, execution, progressChan, multiStepPlan.([]*RecoveryStep))
	}

	// Single step recovery - use normal execution
	return a.executeExecutionStep(modifiedStep, execution, progressChan)
}

// executeMultiStepRecoveryPlan executes a multi-step recovery plan designed by AI
func (a *StateAwareAgent) executeMultiStepRecoveryPlan(ctx context.Context, modifiedStep *types.ExecutionPlanStep, execution *types.PlanExecution, progressChan chan<- *types.ExecutionUpdate, multiStepPlan []*RecoveryStep) (*types.ExecutionStep, error) {
	var lastExecutedStep *types.ExecutionStep

	// Execute each step in the AI-designed recovery plan
	for i, recoveryStep := range multiStepPlan {
		stepNum := i + 1
		recoveryStepID := fmt.Sprintf("%s-recovery-%d", modifiedStep.ID, stepNum)

		// Update progress
		if progressChan != nil {
			progressChan <- &types.ExecutionUpdate{
				Type:        "step_recovery_progress",
				ExecutionID: execution.ID,
				StepID:      modifiedStep.ID,
				Message:     fmt.Sprintf("Recovery step %d/%d (%s): %s", stepNum, len(multiStepPlan), recoveryStep.ToolName, recoveryStep.Purpose),
				Timestamp:   time.Now(),
			}
		}

		// Create execution step for this recovery step
		// Important: Use unique recovery step ID for dependency tracking
		recoveryExecStep := &types.ExecutionPlanStep{
			ID:             recoveryStepID,
			Name:           fmt.Sprintf("Recovery Step %d: %s", stepNum, recoveryStep.Purpose),
			Description:    recoveryStep.Purpose,
			Action:         modifiedStep.Action, // Use original step's action for consistency
			MCPTool:        recoveryStep.ToolName,
			ToolParameters: recoveryStep.Parameters, // AI parameters will be resolved by main system
			Parameters:     recoveryStep.Parameters, // Maintain compatibility
			ResourceID:     recoveryStepID,          // Initialize ResourceID to step ID for proper fallback behavior
		}

		// Execute this recovery step using the main execution system
		// This automatically handles dependency resolution using resolveDependencyReference()
		executedStep, err := a.executeExecutionStep(recoveryExecStep, execution, progressChan)
		if err != nil {
			return nil, fmt.Errorf("recovery step %d (%s) failed: %w", stepNum, recoveryStepID, err)
		}

		// Resource mapping storage is handled automatically by executeExecutionStep()
		// No need to call extractAndStoreResourceMapping manually as the main execution system
		// handles all resource mapping through executeNativeMCPTool and other action handlers

		// IMPORTANT: Update the original step's resource mapping so dependent steps can resolve references
		// When recovery succeeds, the original failed step should point to the recovery result
		if executedStep != nil {
			a.mappingsMutex.Lock()

			// Get the recovery step's resource ID from the mapping
			recoveryResourceID, exists := a.resourceMappings[recoveryStepID]
			if exists && recoveryResourceID != "" {
				// Update the original step ID to point to the same resource
				a.resourceMappings[modifiedStep.ID] = recoveryResourceID

				a.Logger.WithFields(map[string]interface{}{
					"original_step_id": modifiedStep.ID,
					"recovery_step_id": recoveryStepID,
					"resource_id":      recoveryResourceID,
				}).Info("Updated original step mapping to point to recovery result for dependency resolution")
			}

			a.mappingsMutex.Unlock()

			lastExecutedStep = executedStep
		}
	}

	// Return the result of the final step in the AI-designed plan
	a.Logger.WithField("step_id", modifiedStep.ID).Info("AI-designed multi-step recovery completed successfully using main dependency system")
	return lastExecutedStep, nil
}
