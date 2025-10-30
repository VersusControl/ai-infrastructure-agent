package graph

import (
	"context"
	"fmt"
	"sort"

	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// Manager handles dependency graph management for infrastructure resources
type Manager struct {
	logger *logging.Logger
	graph  *types.DependencyGraph
}

// NewManager creates a new dependency graph manager
func NewManager(logger *logging.Logger) *Manager {
	return &Manager{
		logger: logger,
		graph: &types.DependencyGraph{
			Nodes: make(map[string]*types.DependencyNode),
			Edges: make(map[string][]string),
		},
	}
}

// BuildGraph builds a dependency graph from a list of resources
func (m *Manager) BuildGraph(ctx context.Context, resources []*types.ResourceState) error {
	m.logger.Info("Building dependency graph")

	// Clear existing graph
	m.graph.Nodes = make(map[string]*types.DependencyNode)
	m.graph.Edges = make(map[string][]string)

	// Build step reference to resource ID mapping
	stepToResourceID := make(map[string]string)
	for _, resource := range resources {
		if resource.Type == "step_reference" {
			// Extract actual resource ID from unified field
			if unified, ok := resource.Properties["unified"].(map[string]interface{}); ok {
				if resourceID, ok := unified["resourceId"].(string); ok && resourceID != "" {
					stepToResourceID[resource.ID] = resourceID
				}
			}
		}
	}

	// Add all non-step-reference resources as nodes
	for _, resource := range resources {
		// Skip step references - they are not actual infrastructure resources
		if resource.Type == "step_reference" {
			continue
		}

		node := &types.DependencyNode{
			ID:           resource.ID,
			ResourceType: resource.Type,
			Status:       resource.Status,
			Properties:   make(map[string]string),
		}

		// Convert properties to strings for simplified storage
		for key, value := range resource.Properties {
			if strValue, ok := value.(string); ok {
				node.Properties[key] = strValue
			} else {
				node.Properties[key] = fmt.Sprintf("%v", value)
			}
		}

		m.graph.Nodes[resource.ID] = node
	}

	// Build edges based on dependencies from state file
	for _, resource := range resources {
		// Skip step references - they don't represent actual resources
		if resource.Type == "step_reference" {
			continue
		}

		for _, depID := range resource.Dependencies {
			// Resolve step reference to actual resource ID if needed
			actualDepID := depID
			if resolvedID, isStepRef := stepToResourceID[depID]; isStepRef {
				actualDepID = resolvedID
			}

			// Verify dependency exists in the graph
			if _, exists := m.graph.Nodes[actualDepID]; exists {
				m.addEdge(resource.ID, actualDepID)
			} else {
				m.logger.WithFields(map[string]interface{}{
					"resource_id":   resource.ID,
					"dependency_id": actualDepID,
					"original_ref":  depID,
				}).Warn("Dependency not found in graph after resolution, skipping")
			}
		}
	}

	m.logger.WithFields(map[string]interface{}{
		"nodes":         len(m.graph.Nodes),
		"edges":         m.getTotalEdges(),
		"step_mappings": len(stepToResourceID),
	}).Info("Dependency graph built successfully from state file")

	return nil
}

// addEdge adds a directed edge from resource to dependency
func (m *Manager) addEdge(fromID, toID string) {
	if m.graph.Edges[fromID] == nil {
		m.graph.Edges[fromID] = []string{}
	}

	// Check if edge already exists
	for _, edge := range m.graph.Edges[fromID] {
		if edge == toID {
			return
		}
	}

	m.graph.Edges[fromID] = append(m.graph.Edges[fromID], toID)
}

// GetDeploymentOrder returns resources in deployment order (topological sort)
func (m *Manager) GetDeploymentOrder() ([]string, error) {
	m.logger.Debug("Calculating deployment order")

	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	order := []string{}

	var visit func(nodeID string) error
	visit = func(nodeID string) error {
		if recStack[nodeID] {
			return fmt.Errorf("circular dependency detected involving resource: %s", nodeID)
		}
		if visited[nodeID] {
			return nil
		}

		visited[nodeID] = true
		recStack[nodeID] = true

		// Visit all dependencies first
		for _, depID := range m.graph.Edges[nodeID] {
			if err := visit(depID); err != nil {
				return err
			}
		}

		recStack[nodeID] = false
		order = append(order, nodeID)

		return nil
	}

	// Visit all nodes
	for nodeID := range m.graph.Nodes {
		if !visited[nodeID] {
			if err := visit(nodeID); err != nil {
				return nil, err
			}
		}
	}

	// Reverse the order for proper deployment sequence
	for i, j := 0, len(order)-1; i < j; i, j = i+1, j-1 {
		order[i], order[j] = order[j], order[i]
	}

	m.logger.WithField("deployment_order", order).Debug("Deployment order calculated")
	return order, nil
}

// GetDeletionOrder returns resources in deletion order (reverse of deployment order)
func (m *Manager) GetDeletionOrder() ([]string, error) {
	deploymentOrder, err := m.GetDeploymentOrder()
	if err != nil {
		return nil, err
	}

	// Reverse for deletion order
	deletionOrder := make([]string, len(deploymentOrder))
	for i, j := 0, len(deploymentOrder)-1; i <= j; i, j = i+1, j-1 {
		deletionOrder[i] = deploymentOrder[j]
		deletionOrder[j] = deploymentOrder[i]
	}

	m.logger.WithField("deletion_order", deletionOrder).Debug("Deletion order calculated")
	return deletionOrder, nil
}

// GetDependents returns all resources that depend on the given resource
func (m *Manager) GetDependents(resourceID string) []string {
	var dependents []string

	for nodeID, edges := range m.graph.Edges {
		for _, depID := range edges {
			if depID == resourceID {
				dependents = append(dependents, nodeID)
				break
			}
		}
	}

	sort.Strings(dependents)
	return dependents
}

// GetDependencies returns all resources that the given resource depends on
func (m *Manager) GetDependencies(resourceID string) []string {
	dependencies := m.graph.Edges[resourceID]
	if dependencies == nil {
		return []string{}
	}

	result := make([]string, len(dependencies))
	copy(result, dependencies)
	sort.Strings(result)
	return result
}

// ValidateGraph validates the dependency graph for consistency
func (m *Manager) ValidateGraph() error {
	m.logger.Debug("Validating dependency graph")

	// Check for circular dependencies
	_, err := m.GetDeploymentOrder()
	if err != nil {
		return fmt.Errorf("graph validation failed: %w", err)
	}

	// Check for orphaned edges
	for nodeID, edges := range m.graph.Edges {
		if _, exists := m.graph.Nodes[nodeID]; !exists {
			return fmt.Errorf("orphaned edge found: node %s does not exist", nodeID)
		}

		for _, depID := range edges {
			if _, exists := m.graph.Nodes[depID]; !exists {
				return fmt.Errorf("invalid dependency: node %s depends on non-existent node %s", nodeID, depID)
			}
		}
	}

	m.logger.Debug("Dependency graph validation completed successfully")
	return nil
}

// GetGraph returns the current dependency graph
func (m *Manager) GetGraph() *types.DependencyGraph {
	return m.graph
}

// GetResourcesByType returns all resources of a specific type
func (m *Manager) GetResourcesByType(resourceType string) []string {
	var resources []string

	for nodeID, node := range m.graph.Nodes {
		if node.ResourceType == resourceType {
			resources = append(resources, nodeID)
		}
	}

	sort.Strings(resources)
	return resources
}

// GetCriticalPath identifies the critical path for resource deployment
func (m *Manager) GetCriticalPath(targetResource string) ([]string, error) {
	if _, exists := m.graph.Nodes[targetResource]; !exists {
		return nil, fmt.Errorf("target resource %s not found in graph", targetResource)
	}

	visited := make(map[string]bool)
	path := []string{}

	var findPath func(nodeID string) bool
	findPath = func(nodeID string) bool {
		if visited[nodeID] {
			return false
		}

		visited[nodeID] = true
		path = append(path, nodeID)

		if nodeID == targetResource {
			return true
		}

		// Try each dependency
		for _, depID := range m.graph.Edges[nodeID] {
			if findPath(depID) {
				return true
			}
		}

		// Backtrack
		path = path[:len(path)-1]
		return false
	}

	// Find critical path from all leaf nodes (nodes with no dependencies)
	for nodeID := range m.graph.Nodes {
		if len(m.graph.Edges[nodeID]) == 0 {
			if findPath(nodeID) {
				break
			}
		}
	}

	if len(path) == 0 {
		return nil, fmt.Errorf("no path found to target resource %s", targetResource)
	}

	return path, nil
}

// CalculateDeploymentLevels groups resources into deployment levels
func (m *Manager) CalculateDeploymentLevels() ([][]string, error) {
	m.logger.Debug("Calculating deployment levels")

	inDegree := make(map[string]int)

	// Initialize in-degree count
	for nodeID := range m.graph.Nodes {
		inDegree[nodeID] = 0
	}

	// Calculate in-degrees
	for _, edges := range m.graph.Edges {
		for _, depID := range edges {
			inDegree[depID]++
		}
	}

	var levels [][]string
	processed := make(map[string]bool)

	for len(processed) < len(m.graph.Nodes) {
		var currentLevel []string

		// Find all nodes with in-degree 0
		for nodeID, degree := range inDegree {
			if degree == 0 && !processed[nodeID] {
				currentLevel = append(currentLevel, nodeID)
			}
		}

		if len(currentLevel) == 0 {
			return nil, fmt.Errorf("circular dependency detected - no nodes with zero in-degree")
		}

		sort.Strings(currentLevel)
		levels = append(levels, currentLevel)

		// Mark as processed and update in-degrees
		for _, nodeID := range currentLevel {
			processed[nodeID] = true
			for _, depID := range m.graph.Edges[nodeID] {
				inDegree[depID]--
			}
		}
	}

	m.logger.WithField("levels", len(levels)).Debug("Deployment levels calculated")
	return levels, nil
}

// getTotalEdges returns the total number of edges in the graph
func (m *Manager) getTotalEdges() int {
	total := 0
	for _, edges := range m.graph.Edges {
		total += len(edges)
	}
	return total
}
