package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// ========== AI Agent/Execution Handlers ==========
// These handlers provide AI-powered infrastructure management,
// including plan generation, request processing, and execution with recovery.

// getPlanHandler generates a deployment plan for infrastructure resources
// Supports request body parameters:
//   - include_levels (bool): Include deployment level information in response
//
// Returns an ordered deployment plan with optional level grouping
func (ws *WebServer) getPlanHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var requestBody map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	includeLevels := true
	if val, ok := requestBody["include_levels"].(bool); ok {
		includeLevels = val
	}

	ctx := r.Context()
	// Use MCP server to plan deployment
	deploymentOrder, deploymentLevels, err := ws.aiAgent.PlanInfrastructureDeployment(ctx, nil, includeLevels)
	if err != nil {
		ws.aiAgent.Logger.WithError(err).Error("Failed to plan deployment")
		http.Error(w, "Deployment planning failed", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"deployment_order": deploymentOrder,
		"resource_count":   len(deploymentOrder),
		"timestamp":        time.Now(),
	}

	if includeLevels {
		response["deployment_levels"] = deploymentLevels
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		ws.aiAgent.Logger.WithError(err).Error("Failed to encode plan response")
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// processRequestHandler processes a natural language infrastructure request
// Request body parameters:
//   - request (string, required): Natural language request
//   - dry_run (bool): If true, simulate execution without making changes
//
// Returns an AI-generated decision with execution plan for user confirmation
func (ws *WebServer) processRequestHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var requestBody map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	request, ok := requestBody["request"].(string)
	if !ok {
		http.Error(w, "Request field is required", http.StatusBadRequest)
		return
	}

	dryRun := true
	if val, ok := requestBody["dry_run"].(bool); ok {
		dryRun = val
	}

	ctx := r.Context()

	// Check if AI agent is available
	if ws.aiAgent == nil {
		// Return demo response if AI agent is not available
		response := map[string]interface{}{
			"request":    request,
			"dry_run":    dryRun,
			"response":   "AI Agent not available. Please set OPENAI_API_KEY environment variable to enable real AI processing.",
			"confidence": 0.0,
			"mode":       "demo",
			"timestamp":  time.Now(),
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			ws.aiAgent.Logger.WithError(err).Error("Failed to encode demo response")
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
		return
	}

	ws.aiAgent.Logger.WithFields(map[string]interface{}{
		"request": request,
		"dry_run": dryRun,
	}).Info("Processing request with AI agent")

	// Notify WebSocket clients that processing has started
	ws.broadcastUpdate(map[string]interface{}{
		"type":      "processing_started",
		"request":   request,
		"dry_run":   dryRun,
		"timestamp": time.Now(),
	})

	// Process the request
	decision, err := ws.aiAgent.ProcessRequest(ctx, request)
	if err != nil {
		ws.aiAgent.Logger.WithError(err).Error("AI agent request processing failed")
		http.Error(w, fmt.Sprintf("AI processing failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Store the decision for later execution
	ws.storeDecisionWithDryRun(decision, dryRun)

	// Build response with execution plan (without executing yet)
	response := map[string]interface{}{
		"request":              request,
		"dry_run":              dryRun,
		"mode":                 "live",
		"decision":             decision,
		"executionPlan":        decision.ExecutionPlan,
		"confidence":           decision.Confidence,
		"action":               decision.Action,
		"reasoning":            decision.Reasoning,
		"requiresConfirmation": true,
		"timestamp":            time.Now(),
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		ws.aiAgent.Logger.WithError(err).Error("Failed to encode AI response")
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	// Notify WebSocket clients that processing has completed
	ws.broadcastUpdate(map[string]interface{}{
		"type":                 "processing_completed",
		"request":              request,
		"decisionId":           decision.ID,
		"success":              true,
		"requiresConfirmation": true,
		"timestamp":            time.Now(),
	})
}

// executeWithPlanRecoveryHandler executes a confirmed plan with plan-level ReAct recovery
// Request body parameters:
//   - decisionId (string, required): ID of the previously stored decision to execute
//
// Executes the plan asynchronously and streams progress updates via WebSocket
func (ws *WebServer) executeWithPlanRecoveryHandler(w http.ResponseWriter, r *http.Request) {
	var executeRequest struct {
		DecisionID string `json:"decisionId"`
	}

	if err := json.NewDecoder(r.Body).Decode(&executeRequest); err != nil {
		ws.aiAgent.Logger.WithError(err).Error("Failed to decode execute request")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if executeRequest.DecisionID == "" {
		http.Error(w, "Decision ID is required", http.StatusBadRequest)
		return
	}

	ws.aiAgent.Logger.WithField("decision_id", executeRequest.DecisionID).Info("Executing plan with plan-level recovery")

	// Retrieve the stored decision with dry_run flag
	decision, dryRun, exists := ws.getStoredDecisionWithDryRun(executeRequest.DecisionID)
	if !exists {
		ws.aiAgent.Logger.WithField("decision_id", executeRequest.DecisionID).Error("Decision not found")
		http.Error(w, "Decision not found", http.StatusNotFound)
		return
	}

	ws.aiAgent.Logger.WithFields(map[string]interface{}{
		"decision_id": executeRequest.DecisionID,
		"dry_run":     dryRun,
	}).Info("Starting plan execution")

	// Create a buffered progress channel to avoid blocking
	progressChan := make(chan *types.ExecutionUpdate, 100)

	// Start execution in a goroutine
	go func() {
		defer close(progressChan)
		defer ws.removeStoredDecision(executeRequest.DecisionID) // Cleanup after execution

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute*20) // 20 minutes for plan-level recovery
		defer cancel()

		var execution *types.PlanExecution
		var err error

		// Check if dry_run mode - simulate execution instead of real execution
		if dryRun {
			ws.aiAgent.Logger.Info("Dry run mode enabled - simulating execution")
			execution = ws.aiAgent.SimulatePlanExecution(decision, progressChan)
			// Simulation doesn't return errors
			err = nil
		} else {
			ws.aiAgent.Logger.Info("Live mode - executing plan with ReAct recovery")
			// Use new plan-level recovery execution
			execution, err = ws.aiAgent.ExecutePlanWithReActRecovery(ctx, decision, progressChan, ws)
		}

		if err != nil {
			ws.aiAgent.Logger.WithError(err).Error("Plan execution with recovery failed")
			// Send error update
			select {
			case progressChan <- &types.ExecutionUpdate{
				Type:        "execution_failed",
				ExecutionID: "failed",
				Message:     fmt.Sprintf("Execution failed: %v", err),
				Error:       err.Error(),
				Timestamp:   time.Now(),
			}:
			default:
			}
		} else {
			ws.aiAgent.Logger.WithFields(map[string]interface{}{
				"execution_id": execution.ID,
				"status":       execution.Status,
				"total_steps":  len(execution.Steps),
			}).Info("Plan execution with plan-level recovery completed")
		}
	}()

	// Start progress streaming in another goroutine
	go func() {
		for update := range progressChan {
			ws.aiAgent.Logger.WithFields(map[string]interface{}{
				"type":    update.Type,
				"message": update.Message,
			}).Debug("Broadcasting execution update")

			// Broadcast update via WebSocket
			ws.broadcastUpdate(map[string]interface{}{
				"type":        update.Type,
				"executionId": update.ExecutionID,
				"stepId":      update.StepID,
				"message":     update.Message,
				"error":       update.Error,
				"timestamp":   update.Timestamp,
			})
		}
	}()

	// Return immediate response
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"success":      true,
		"message":      "Plan execution started with plan-level recovery",
		"executionId":  "exec-" + executeRequest.DecisionID,
		"recoveryType": "plan-level",
		"timestamp":    time.Now(),
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		ws.aiAgent.Logger.WithError(err).Error("Failed to encode execute response")
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}
