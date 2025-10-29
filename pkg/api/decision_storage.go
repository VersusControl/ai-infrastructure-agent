package api

import "github.com/versus-control/ai-infrastructure-agent/pkg/types"

// ========== Decision Storage Management ==========
// These methods manage the storage and retrieval of infrastructure decisions
// before they are executed. This allows for a two-step process: plan generation
// followed by user confirmation and execution.

// storeDecisionWithDryRun stores a decision with dry run flag for later execution
func (ws *WebServer) storeDecisionWithDryRun(decision *types.AgentDecision, dryRun bool) {
	ws.decisionsMutex.Lock()
	defer ws.decisionsMutex.Unlock()

	ws.decisions[decision.ID] = &StoredDecision{
		Decision: decision,
		DryRun:   dryRun,
	}
}

// getStoredDecisionWithDryRun retrieves a stored decision with its dry run flag
// Returns: (decision, dryRun, exists)
func (ws *WebServer) getStoredDecisionWithDryRun(decisionID string) (*types.AgentDecision, bool, bool) {
	ws.decisionsMutex.RLock()
	defer ws.decisionsMutex.RUnlock()

	storedDecision, exists := ws.decisions[decisionID]
	if !exists {
		return nil, false, false
	}

	return storedDecision.Decision, storedDecision.DryRun, true
}

// removeStoredDecision removes a stored decision after execution
func (ws *WebServer) removeStoredDecision(decisionID string) {
	ws.decisionsMutex.Lock()
	defer ws.decisionsMutex.Unlock()

	delete(ws.decisions, decisionID)

	ws.aiAgent.Logger.WithField("decision_id", decisionID).Debug("Removed stored decision")
}
