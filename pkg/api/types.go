package api

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/versus-control/ai-infrastructure-agent/pkg/agent"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// wsConnection wraps a WebSocket connection with metadata
type wsConnection struct {
	conn       *websocket.Conn
	lastPong   time.Time
	writeMutex sync.Mutex // Protects concurrent writes to WebSocket
}

// StoredDecision stores a decision along with its execution parameters
type StoredDecision struct {
	Decision *types.AgentDecision `json:"decision"`
	DryRun   bool                 `json:"dry_run"`
}

// PlanRecoveryRequest represents a pending plan-level recovery decision
type PlanRecoveryRequest struct {
	ExecutionID    string                      `json:"executionId"`
	FailureContext *agent.PlanFailureContext   `json:"failureContext"`
	Strategy       *agent.PlanRecoveryStrategy `json:"strategy"` // Single strategy now
	ResponseChan   chan *PlanRecoveryResponse  `json:"-"`
	Timestamp      time.Time                   `json:"timestamp"`
}

// PlanRecoveryResponse represents the user's plan recovery decision
type PlanRecoveryResponse struct {
	Approved bool `json:"approved"` // User approved (true) or rejected (false) the recovery plan
}

// RecoveryMessage represents incoming recovery-related messages from WebSocket clients
type RecoveryMessage struct {
	Type        string    `json:"type"`
	ExecutionID string    `json:"executionId,omitempty"` // For plan-level recovery
	Approved    bool      `json:"approved,omitempty"`    // For plan-level recovery (approved/rejected)
	Timestamp   time.Time `json:"timestamp"`
}
