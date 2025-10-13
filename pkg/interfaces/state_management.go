package interfaces

import (
	"context"

	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// StateManager defines the interface for infrastructure state management
type StateManager interface {
	// State persistence
	LoadState(ctx context.Context) error
	SaveState(ctx context.Context) error

	// Resource management
	AddResource(ctx context.Context, resource *types.ResourceState) error
	UpdateResource(ctx context.Context, resourceID string, updates map[string]interface{}) error
	RemoveResource(ctx context.Context, resourceID string) error
	GetResource(resourceID string) (*types.ResourceState, bool)
	ListResources(resourceType string) []*types.ResourceState

	// Dependency management
	AddDependency(ctx context.Context, resourceID, dependsOn string) error
	GetDependencies(resourceID string) []string
	GetDependents(resourceID string) []string

	// State queries
	GetState() *types.InfrastructureState
	DetectDrift(ctx context.Context, actualState map[string]interface{}, resourceID string) (*types.ChangeDetection, error)
}

// ConflictResolver defines the interface for detecting and resolving resource conflicts
type ConflictResolver interface {
	// Conflict detection
	DetectConflicts(ctx context.Context, resources []*types.ResourceState) ([]*types.ConflictResolution, error)

	// Conflict resolution
	ResolveConflict(ctx context.Context, conflict *types.ConflictResolution) error
	AutoResolveConflicts(ctx context.Context, conflicts []*types.ConflictResolution) ([]*types.ConflictResolution, error)

	// Conflict analysis
	AnalyzeImpact(ctx context.Context, conflict *types.ConflictResolution) (*types.ConflictImpact, error)
}

// DependencyGraphManager defines the interface for managing resource dependencies
type DependencyGraphManager interface {
	// Graph construction
	BuildGraph(resources []*types.ResourceState) *types.DependencyGraph
	AddNode(resourceID string, resourceType string) error
	AddEdge(from, to string) error

	// Graph analysis
	GetDeploymentOrder(resources []string) ([]string, error)
	DetectCycles() ([][]string, error)
	GetTopologicalOrder() ([]string, error)

	// Graph queries
	GetDependencies(resourceID string) []string
	GetDependents(resourceID string) []string
	IsReachable(from, to string) bool
}

// DiscoveryScanner defines the interface for discovering existing AWS resources
type DiscoveryScanner interface {
	// Resource discovery
	ScanAll(ctx context.Context) ([]*types.ResourceState, error)
	ScanByType(ctx context.Context, resourceType string) ([]*types.ResourceState, error)
	ScanByTags(ctx context.Context, tags map[string]string) ([]*types.ResourceState, error)

	// Resource correlation
	CorrelateResources(ctx context.Context, managedState []*types.ResourceState, discoveredState []*types.ResourceState) (map[string]*types.ResourceCorrelation, error)

	// Change detection
	DetectChanges(ctx context.Context, baseline []*types.ResourceState) ([]*types.ResourceChange, error)
}
