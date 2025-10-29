package types

import (
	"time"
)

// ServerInfo contains metadata about our MCP server
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ResourceInfo represents an MCP resource that AI systems can access
type ResourceInfo struct {
	URI         string            `json:"uri"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	MimeType    string            `json:"mimeType"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// AWSResource represents AWS infrastructure resources
type AWSResource struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Region   string                 `json:"region"`
	State    string                 `json:"state"`
	Tags     map[string]string      `json:"tags,omitempty"`
	Details  map[string]interface{} `json:"details"`
	LastSeen time.Time              `json:"lastSeen"`
}

// State Management Types

// InfrastructureState represents the complete state of managed infrastructure
type InfrastructureState struct {
	Version      string                    `json:"version"`
	LastUpdated  time.Time                 `json:"lastUpdated"`
	Region       string                    `json:"region"`
	Resources    map[string]*ResourceState `json:"resources"`
	Dependencies map[string][]string       `json:"dependencies"`
	Metadata     map[string]interface{}    `json:"metadata,omitempty"`
}

// ResourceState represents the state of a single infrastructure resource
type ResourceState struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	Type         string                 `json:"type"`
	Status       string                 `json:"status"`
	DesiredState string                 `json:"desiredState"`
	CurrentState string                 `json:"currentState"`
	Tags         map[string]string      `json:"tags,omitempty"`
	Properties   map[string]interface{} `json:"properties"`
	Dependencies []string               `json:"dependencies,omitempty"`
	CreatedAt    time.Time              `json:"createdAt"`
	UpdatedAt    time.Time              `json:"updatedAt"`
	Checksum     string                 `json:"checksum"`
}

// ChangeDetection represents detected changes in infrastructure
type ChangeDetection struct {
	Resource   string                 `json:"resource"`
	ChangeType string                 `json:"changeType"` // create, update, delete, drift
	OldState   map[string]interface{} `json:"oldState,omitempty"`
	NewState   map[string]interface{} `json:"newState,omitempty"`
	Reason     string                 `json:"reason"`
	Timestamp  time.Time              `json:"timestamp"`
}

// DependencyGraph represents resource dependencies
type DependencyGraph struct {
	Nodes map[string]*DependencyNode `json:"nodes"`
	Edges map[string][]string        `json:"edges"`
}

// DependencyNode represents a node in the dependency graph
type DependencyNode struct {
	ID           string            `json:"id"`
	ResourceType string            `json:"resourceType"`
	Status       string            `json:"status"`
	Properties   map[string]string `json:"properties"`
}

// ConflictResolution represents resolution strategies for resource conflicts
type ConflictResolution struct {
	ResourceID   string                 `json:"resourceId"`
	ConflictType string                 `json:"conflictType"` // naming, dependency, state
	Resolution   string                 `json:"resolution"`   // rename, recreate, ignore, manual
	Details      map[string]interface{} `json:"details"`
	ResolvedAt   time.Time              `json:"resolvedAt"`
	ResolvedBy   string                 `json:"resolvedBy"` // agent, user
}

// ConflictImpact represents the impact analysis of a conflict
type ConflictImpact struct {
	Severity          string   `json:"severity"` // low, medium, high, critical
	AffectedResources []string `json:"affectedResources"`
	RiskLevel         string   `json:"riskLevel"`
	Recommendations   []string `json:"recommendations"`
	EstimatedDowntime string   `json:"estimatedDowntime,omitempty"`
}

// ResourceCorrelation represents correlation between managed and discovered resources
type ResourceCorrelation struct {
	ManagedResourceID    string                 `json:"managedResourceId"`
	DiscoveredResourceID string                 `json:"discoveredResourceId"`
	CorrelationType      string                 `json:"correlationType"` // exact, similar, conflict
	Confidence           float64                `json:"confidence"`      // 0.0 to 1.0
	Differences          map[string]interface{} `json:"differences,omitempty"`
}

// ResourceChange represents a detected change in a resource
type ResourceChange struct {
	ResourceID string                 `json:"resourceId"`
	ChangeType string                 `json:"changeType"` // created, modified, deleted, moved
	OldValues  map[string]interface{} `json:"oldValues,omitempty"`
	NewValues  map[string]interface{} `json:"newValues,omitempty"`
	Timestamp  time.Time              `json:"timestamp"`
	DetectedBy string                 `json:"detectedBy"`
	Confidence float64                `json:"confidence"`
}

// ExecutionPlanStep represents a single step in an execution plan
type ExecutionPlanStep struct {
	ID                string                 `json:"id"`
	Name              string                 `json:"name"`
	Description       string                 `json:"description"`
	Action            string                 `json:"action"`
	ResourceID        string                 `json:"resourceId"`
	MCPTool           string                 `json:"mcpTool,omitempty"`        // Direct MCP tool name
	ToolParameters    map[string]interface{} `json:"toolParameters,omitempty"` // Direct MCP tool parameters
	Parameters        map[string]interface{} `json:"parameters"`               // Legacy/fallback parameters
	DependsOn         []string               `json:"dependsOn,omitempty"`
	EstimatedDuration string                 `json:"estimatedDuration,omitempty"`
	Status            string                 `json:"status"` // pending, running, completed, failed, skipped
}

// AgentDecision represents an AI agent's decision about infrastructure changes
type AgentDecision struct {
	ID            string                 `json:"id"`
	Action        string                 `json:"action"`
	Resource      string                 `json:"resource"`
	Reasoning     string                 `json:"reasoning"`
	Confidence    float64                `json:"confidence"`
	Parameters    map[string]interface{} `json:"parameters"`
	ExecutionPlan []*ExecutionPlanStep   `json:"executionPlan,omitempty"`
	Timestamp     time.Time              `json:"timestamp"`
	ExecutedAt    *time.Time             `json:"executedAt,omitempty"`
	Result        string                 `json:"result,omitempty"`
	Error         string                 `json:"error,omitempty"`
}

// PlanExecution represents the execution of an infrastructure plan
type PlanExecution struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Status      string             `json:"status"` // pending, running, completed, failed
	StartedAt   time.Time          `json:"startedAt"`
	CompletedAt *time.Time         `json:"completedAt,omitempty"`
	Steps       []*ExecutionStep   `json:"steps"`
	Changes     []*ChangeDetection `json:"changes"`
	Errors      []string           `json:"errors,omitempty"`
}

// ExecutionStep represents a single step in plan execution
type ExecutionStep struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Status      string                 `json:"status"` // pending, running, completed, failed, skipped
	Resource    string                 `json:"resource"`
	Action      string                 `json:"action"`
	StartedAt   *time.Time             `json:"startedAt,omitempty"`
	CompletedAt *time.Time             `json:"completedAt,omitempty"`
	Duration    time.Duration          `json:"duration,omitempty"`
	Output      map[string]interface{} `json:"output,omitempty"`
	Error       string                 `json:"error,omitempty"`
}

// ExecutionUpdate represents real-time updates during plan execution
type ExecutionUpdate struct {
	Type        string    `json:"type"` // execution_started, step_started, step_progress, step_completed, step_failed, execution_completed
	ExecutionID string    `json:"executionId"`
	StepID      string    `json:"stepId,omitempty"`
	Message     string    `json:"message"`
	Error       string    `json:"error,omitempty"`
	Progress    float64   `json:"progress,omitempty"` // 0.0 to 1.0
	Timestamp   time.Time `json:"timestamp"`
}

// UnifiedResourceInfo provides standardized resource information across all AWS resource types
// This structure unifies the varying response formats from different MCP tools into a consistent format.
// Access via properties["unified"] to get standardized resource information.
// properties["mcp_response"] still contains the full original metadata.
type UnifiedResourceInfo struct {
	// ResourceId is the primary identifier for this resource.
	// For single resources: "i-123456", "vpc-abc123"
	// For array resources: concatenated string "subnet-001_subnet-002_subnet-003"
	// For composite resources: compound identifier "rtb-123-subnet-456"
	ResourceId string `json:"resourceId"`

	// ResourceType indicates the AWS resource type using underscore notation.
	// Examples: "ec2_instance", "vpc", "security_group", "load_balancer", "availability_zone"
	ResourceType string `json:"resourceType"`

	// ResourceName is a human-readable name for the resource (can be empty string).
	// Extracted from Tags["Name"] or tool parameters when available.
	ResourceName string `json:"resourceName"`

	// DependencyReferenceType indicates how this resource should be referenced.
	// Valid values:
	//   "single"    - Single resource ID (most common case)
	//   "array"     - Multiple resource IDs, use ResourceIds array
	DependencyReferenceType string `json:"dependencyReferenceType"`

	// ResourceIds contains individual IDs for array-type resources.
	// Only populated when DependencyReferenceType = "array"
	// Example: ["subnet-001", "subnet-002", "subnet-003"] for availability zones
	ResourceIds []string `json:"resourceIds,omitempty"`

	// ResourceArn contains the full Amazon Resource Name if applicable.
	// Populated for ARN-based resources like Load Balancers, Lambda functions, etc.
	ResourceArn string `json:"resourceArn,omitempty"`
}
