package conflict

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"

	"github.com/google/uuid"
)

// Resolver handles conflict resolution for infrastructure resources
type Resolver struct {
	logger     *logging.Logger
	strategies map[string]ResolutionStrategy
}

// ResolutionStrategy defines how to resolve different types of conflicts
type ResolutionStrategy interface {
	Resolve(ctx context.Context, conflict *types.ConflictResolution) (*Resolution, error)
	CanHandle(conflictType string) bool
}

// Resolution represents the result of conflict resolution
type Resolution struct {
	Action       string                 `json:"action"`
	Parameters   map[string]interface{} `json:"parameters"`
	Reason       string                 `json:"reason"`
	Confidence   float64                `json:"confidence"`
	Alternatives []Alternative          `json:"alternatives,omitempty"`
}

// Alternative represents an alternative resolution approach
type Alternative struct {
	Action     string                 `json:"action"`
	Parameters map[string]interface{} `json:"parameters"`
	Reason     string                 `json:"reason"`
	Risk       string                 `json:"risk"` // low, medium, high
}

// NewResolver creates a new conflict resolver
func NewResolver(logger *logging.Logger) *Resolver {
	resolver := &Resolver{
		logger:     logger,
		strategies: make(map[string]ResolutionStrategy),
	}

	// Register default strategies
	resolver.RegisterStrategy(&NamingConflictStrategy{logger: logger})
	resolver.RegisterStrategy(&DependencyConflictStrategy{logger: logger})
	resolver.RegisterStrategy(&StateConflictStrategy{logger: logger})
	resolver.RegisterStrategy(&ResourceConflictStrategy{logger: logger})

	return resolver
}

// RegisterStrategy registers a new resolution strategy
func (r *Resolver) RegisterStrategy(strategy ResolutionStrategy) {
	r.strategies[fmt.Sprintf("%T", strategy)] = strategy
}

// ResolveConflict resolves a specific conflict
func (r *Resolver) ResolveConflict(ctx context.Context, conflict *types.ConflictResolution) (*Resolution, error) {
	r.logger.WithFields(map[string]interface{}{
		"resource_id":   conflict.ResourceID,
		"conflict_type": conflict.ConflictType,
	}).Info("Resolving infrastructure conflict")

	// Find appropriate strategy
	for _, strategy := range r.strategies {
		if strategy.CanHandle(conflict.ConflictType) {
			resolution, err := strategy.Resolve(ctx, conflict)
			if err != nil {
				r.logger.WithError(err).Error("Failed to resolve conflict with strategy")
				continue
			}

			// Update conflict resolution record
			conflict.Resolution = resolution.Action
			conflict.ResolvedAt = time.Now()
			conflict.ResolvedBy = "agent"

			r.logger.WithFields(map[string]interface{}{
				"resource_id": conflict.ResourceID,
				"action":      resolution.Action,
				"confidence":  resolution.Confidence,
			}).Info("Conflict resolved successfully")

			return resolution, nil
		}
	}

	return nil, fmt.Errorf("no suitable strategy found for conflict type: %s", conflict.ConflictType)
}

// DetectConflicts detects potential conflicts in infrastructure resources
func (r *Resolver) DetectConflicts(ctx context.Context, resources []*types.ResourceState) ([]*types.ConflictResolution, error) {
	r.logger.Info("Detecting infrastructure conflicts")

	var conflicts []*types.ConflictResolution

	// Detect naming conflicts
	namingConflicts := r.detectNamingConflicts(resources)
	conflicts = append(conflicts, namingConflicts...)

	// Detect dependency conflicts
	dependencyConflicts := r.detectDependencyConflicts(resources)
	conflicts = append(conflicts, dependencyConflicts...)

	// Detect state conflicts
	stateConflicts := r.detectStateConflicts(resources)
	conflicts = append(conflicts, stateConflicts...)

	// Detect resource limit conflicts
	resourceConflicts := r.detectResourceConflicts(resources)
	conflicts = append(conflicts, resourceConflicts...)

	r.logger.WithField("conflict_count", len(conflicts)).Info("Conflict detection completed")
	return conflicts, nil
}

// detectNamingConflicts detects resources with conflicting names
func (r *Resolver) detectNamingConflicts(resources []*types.ResourceState) []*types.ConflictResolution {
	var conflicts []*types.ConflictResolution
	nameMap := make(map[string][]*types.ResourceState)

	// Group resources by name and type
	for _, resource := range resources {
		if resource.Name != "" {
			key := fmt.Sprintf("%s:%s", resource.Type, resource.Name)
			nameMap[key] = append(nameMap[key], resource)
		}
	}

	// Find conflicts
	for _, resourceList := range nameMap {
		if len(resourceList) > 1 {
			for _, resource := range resourceList {
				conflict := &types.ConflictResolution{
					ResourceID:   resource.ID,
					ConflictType: "naming",
					Details: map[string]interface{}{
						"conflicting_name":      resource.Name,
						"conflicting_resources": r.extractResourceIDs(resourceList),
						"resource_type":         resource.Type,
					},
				}
				conflicts = append(conflicts, conflict)
			}
		}
	}

	return conflicts
}

// detectDependencyConflicts detects circular dependencies and missing dependencies
func (r *Resolver) detectDependencyConflicts(resources []*types.ResourceState) []*types.ConflictResolution {
	var conflicts []*types.ConflictResolution
	resourceMap := make(map[string]*types.ResourceState)

	for _, resource := range resources {
		resourceMap[resource.ID] = resource
	}

	// Check for missing dependencies
	for _, resource := range resources {
		for _, depID := range resource.Dependencies {
			if _, exists := resourceMap[depID]; !exists {
				conflict := &types.ConflictResolution{
					ResourceID:   resource.ID,
					ConflictType: "dependency",
					Details: map[string]interface{}{
						"missing_dependency": depID,
						"resource_type":      resource.Type,
					},
				}
				conflicts = append(conflicts, conflict)
			}
		}
	}

	// Check for circular dependencies (simplified check)
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	var hasCycle func(string) bool
	hasCycle = func(resourceID string) bool {
		if recStack[resourceID] {
			return true
		}
		if visited[resourceID] {
			return false
		}

		visited[resourceID] = true
		recStack[resourceID] = true

		resource := resourceMap[resourceID]
		if resource != nil {
			for _, depID := range resource.Dependencies {
				if hasCycle(depID) {
					return true
				}
			}
		}

		recStack[resourceID] = false
		return false
	}

	for _, resource := range resources {
		if hasCycle(resource.ID) {
			conflict := &types.ConflictResolution{
				ResourceID:   resource.ID,
				ConflictType: "dependency",
				Details: map[string]interface{}{
					"circular_dependency": true,
					"resource_type":       resource.Type,
				},
			}
			conflicts = append(conflicts, conflict)
		}
	}

	return conflicts
}

// detectStateConflicts detects resources with inconsistent states
func (r *Resolver) detectStateConflicts(resources []*types.ResourceState) []*types.ConflictResolution {
	var conflicts []*types.ConflictResolution

	for _, resource := range resources {
		if resource.DesiredState != resource.CurrentState {
			conflict := &types.ConflictResolution{
				ResourceID:   resource.ID,
				ConflictType: "state",
				Details: map[string]interface{}{
					"desired_state": resource.DesiredState,
					"current_state": resource.CurrentState,
					"resource_type": resource.Type,
				},
			}
			conflicts = append(conflicts, conflict)
		}
	}

	return conflicts
}

// detectResourceConflicts detects resource-specific conflicts (e.g., CIDR overlaps)
func (r *Resolver) detectResourceConflicts(resources []*types.ResourceState) []*types.ConflictResolution {
	var conflicts []*types.ConflictResolution

	// Group VPCs to check for CIDR conflicts
	vpcs := r.filterResourcesByType(resources, "vpc")
	cidrConflicts := r.detectCIDRConflicts(vpcs)
	conflicts = append(conflicts, cidrConflicts...)

	return conflicts
}

// detectCIDRConflicts detects overlapping CIDR blocks in VPCs
func (r *Resolver) detectCIDRConflicts(vpcs []*types.ResourceState) []*types.ConflictResolution {
	var conflicts []*types.ConflictResolution

	for i, vpc1 := range vpcs {
		for j, vpc2 := range vpcs {
			if i >= j {
				continue
			}

			cidr1, exists1 := vpc1.Properties["cidr_block"]
			cidr2, exists2 := vpc2.Properties["cidr_block"]

			if exists1 && exists2 {
				if cidr1 == cidr2 {
					conflict := &types.ConflictResolution{
						ResourceID:   vpc1.ID,
						ConflictType: "resource",
						Details: map[string]interface{}{
							"conflicting_cidr": cidr1,
							"conflicting_vpc":  vpc2.ID,
							"conflict_subtype": "cidr_overlap",
						},
					}
					conflicts = append(conflicts, conflict)
				}
			}
		}
	}

	return conflicts
}

// Helper methods

func (r *Resolver) extractResourceIDs(resources []*types.ResourceState) []string {
	var ids []string
	for _, resource := range resources {
		ids = append(ids, resource.ID)
	}
	return ids
}

func (r *Resolver) filterResourcesByType(resources []*types.ResourceState, resourceType string) []*types.ResourceState {
	var filtered []*types.ResourceState
	for _, resource := range resources {
		if resource.Type == resourceType {
			filtered = append(filtered, resource)
		}
	}
	return filtered
}

// Conflict Resolution Strategies

// NamingConflictStrategy resolves naming conflicts
type NamingConflictStrategy struct {
	logger *logging.Logger
}

func (s *NamingConflictStrategy) CanHandle(conflictType string) bool {
	return conflictType == "naming"
}

func (s *NamingConflictStrategy) Resolve(ctx context.Context, conflict *types.ConflictResolution) (*Resolution, error) {
	resourceName := conflict.Details["conflicting_name"].(string)
	resourceType := conflict.Details["resource_type"].(string)

	// Generate unique name with suffix
	newName := fmt.Sprintf("%s-%s", resourceName, strings.ToLower(uuid.New().String()[:8]))

	return &Resolution{
		Action: "rename",
		Parameters: map[string]interface{}{
			"new_name": newName,
		},
		Reason:     fmt.Sprintf("Generated unique name to resolve naming conflict for %s", resourceType),
		Confidence: 0.9,
		Alternatives: []Alternative{
			{
				Action: "prompt_user",
				Parameters: map[string]interface{}{
					"message": fmt.Sprintf("Please provide a unique name for %s resource", resourceType),
				},
				Reason: "Let user decide on the new name",
				Risk:   "low",
			},
		},
	}, nil
}

// DependencyConflictStrategy resolves dependency conflicts
type DependencyConflictStrategy struct {
	logger *logging.Logger
}

func (s *DependencyConflictStrategy) CanHandle(conflictType string) bool {
	return conflictType == "dependency"
}

func (s *DependencyConflictStrategy) Resolve(ctx context.Context, conflict *types.ConflictResolution) (*Resolution, error) {
	if _, exists := conflict.Details["circular_dependency"]; exists {
		return &Resolution{
			Action: "break_cycle",
			Parameters: map[string]interface{}{
				"remove_dependency": conflict.ResourceID,
			},
			Reason:     "Break circular dependency by removing one dependency link",
			Confidence: 0.7,
		}, nil
	}

	if missingDep, exists := conflict.Details["missing_dependency"]; exists {
		return &Resolution{
			Action: "create_dependency",
			Parameters: map[string]interface{}{
				"dependency_id": missingDep,
			},
			Reason:     "Create missing dependency resource",
			Confidence: 0.8,
		}, nil
	}

	return nil, fmt.Errorf("unhandled dependency conflict type")
}

// StateConflictStrategy resolves state conflicts
type StateConflictStrategy struct {
	logger *logging.Logger
}

func (s *StateConflictStrategy) CanHandle(conflictType string) bool {
	return conflictType == "state"
}

func (s *StateConflictStrategy) Resolve(ctx context.Context, conflict *types.ConflictResolution) (*Resolution, error) {
	desiredState := conflict.Details["desired_state"].(string)
	currentState := conflict.Details["current_state"].(string)

	return &Resolution{
		Action: "reconcile_state",
		Parameters: map[string]interface{}{
			"target_state":  desiredState,
			"current_state": currentState,
		},
		Reason:     fmt.Sprintf("Reconcile resource state from %s to %s", currentState, desiredState),
		Confidence: 0.85,
	}, nil
}

// ResourceConflictStrategy resolves resource-specific conflicts
type ResourceConflictStrategy struct {
	logger *logging.Logger
}

func (s *ResourceConflictStrategy) CanHandle(conflictType string) bool {
	return conflictType == "resource"
}

func (s *ResourceConflictStrategy) Resolve(ctx context.Context, conflict *types.ConflictResolution) (*Resolution, error) {
	if subtype, exists := conflict.Details["conflict_subtype"]; exists && subtype == "cidr_overlap" {
		return &Resolution{
			Action: "modify_cidr",
			Parameters: map[string]interface{}{
				"new_cidr": "10.0.0.0/16", // Default non-conflicting CIDR
			},
			Reason:     "Modify CIDR block to resolve overlap",
			Confidence: 0.75,
		}, nil
	}

	return nil, fmt.Errorf("unhandled resource conflict type")
}
