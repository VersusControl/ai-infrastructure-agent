package api

import (
	"encoding/json"

	"github.com/sirupsen/logrus"
)

// ========== WebSocket Message Handling ==========
// These methods process incoming WebSocket messages from UI clients,
// primarily for handling user responses to plan-level recovery requests.

// handleIncomingMessage processes messages received from WebSocket clients
func (ws *WebServer) handleIncomingMessage(connID string, msgData []byte) {
	var message RecoveryMessage
	if err := json.Unmarshal(msgData, &message); err != nil {
		ws.aiAgent.Logger.WithError(err).WithField("conn_id", connID).Error("Failed to parse WebSocket message")
		return
	}

	ws.aiAgent.Logger.WithFields(logrus.Fields{
		"conn_id":      connID,
		"type":         message.Type,
		"execution_id": message.ExecutionID,
	}).Debug("Received WebSocket message")

	switch message.Type {
	case "plan_recovery_decision":
		ws.handlePlanRecoveryDecision(message)
	case "plan_recovery_abort":
		ws.handlePlanRecoveryAbort(message)
	default:
		ws.aiAgent.Logger.WithFields(logrus.Fields{
			"conn_id": connID,
			"type":    message.Type,
		}).Warn("Unknown WebSocket message type")
	}
}

// handlePlanRecoveryDecision processes user's plan recovery decision
func (ws *WebServer) handlePlanRecoveryDecision(message RecoveryMessage) {
	ws.aiAgent.Logger.WithFields(logrus.Fields{
		"execution_id": message.ExecutionID,
		"approved":     message.Approved,
	}).Info("Processing plan recovery decision")

	// Find the pending plan recovery request
	ws.planRecoveryMutex.Lock()
	request, exists := ws.planRecoveryRequests[message.ExecutionID]
	ws.planRecoveryMutex.Unlock()

	if !exists {
		ws.aiAgent.Logger.WithField("execution_id", message.ExecutionID).Warn("No pending plan recovery request found")
		return
	}

	// Send response back to waiting goroutine
	response := &PlanRecoveryResponse{
		Approved: message.Approved,
	}

	select {
	case request.ResponseChan <- response:
		ws.aiAgent.Logger.WithFields(logrus.Fields{
			"execution_id": message.ExecutionID,
			"approved":     message.Approved,
		}).Info("Plan recovery decision sent to execution")
	default:
		ws.aiAgent.Logger.WithField("execution_id", message.ExecutionID).Error("Failed to send plan recovery decision - channel full or closed")
	}
}

// handlePlanRecoveryAbort processes user's plan recovery abort decision
func (ws *WebServer) handlePlanRecoveryAbort(message RecoveryMessage) {
	ws.aiAgent.Logger.WithField("execution_id", message.ExecutionID).Info("Processing plan recovery abort")

	// Find the pending plan recovery request
	ws.planRecoveryMutex.Lock()
	request, exists := ws.planRecoveryRequests[message.ExecutionID]
	ws.planRecoveryMutex.Unlock()

	if !exists {
		ws.aiAgent.Logger.WithField("execution_id", message.ExecutionID).Warn("No pending plan recovery request found for abort")
		return
	}

	// Send abort response back to waiting goroutine
	response := &PlanRecoveryResponse{
		Approved: false, // User rejected the recovery plan
	}

	select {
	case request.ResponseChan <- response:
		ws.aiAgent.Logger.WithField("execution_id", message.ExecutionID).Info("Plan recovery abort sent to execution")
	default:
		ws.aiAgent.Logger.WithField("execution_id", message.ExecutionID).Error("Failed to send plan recovery abort - channel full or closed")
	}
}
