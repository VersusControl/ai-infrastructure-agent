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

// DeletionPlanResource represents a resource in the deletion plan
type DeletionPlanResource struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Type         string                 `json:"type"`
	Region       string                 `json:"region,omitempty"`
	Status       string                 `json:"status"`
	Properties   map[string]interface{} `json:"properties"`
	Dependencies []string               `json:"dependencies"` // Resources this depends on
	Dependents   []string               `json:"dependents"`   // Resources that depend on this
	CreatedAt    time.Time              `json:"createdAt"`
}

// DeletionPlan represents the deletion plan with ordered resources
type DeletionPlan struct {
	Resources      []DeletionPlanResource `json:"resources"`     // All resources in deletion order
	DeletionOrder  []string               `json:"deletionOrder"` // Resource IDs in deletion order
	TotalResources int                    `json:"totalResources"`
	GeneratedAt    time.Time              `json:"generatedAt"`
	Region         string                 `json:"region"`
	Warnings       []string               `json:"warnings,omitempty"`
}

// DeletionPlanRequest represents a request for deletion plan (optional filters)
type DeletionPlanRequest struct {
	ResourceIDs  []string `json:"resourceIds,omitempty"`  // Specific resources to delete (optional)
	ResourceType string   `json:"resourceType,omitempty"` // Filter by resource type (optional)
}
