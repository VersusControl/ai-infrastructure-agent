package graph

import (
	"context"
	"fmt"
	"sort"
	"strings"

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

	// Add all resources as nodes first
	for _, resource := range resources {
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

	// Build edges based on dependencies
	for _, resource := range resources {
		for _, depID := range resource.Dependencies {
			// Verify dependency exists in the graph
			if _, exists := m.graph.Nodes[depID]; exists {
				m.addEdge(resource.ID, depID)
			} else {
				m.logger.WithFields(map[string]interface{}{
					"resource_id":   resource.ID,
					"dependency_id": depID,
				}).Warn("Dependency not found in graph, skipping")
			}
		}
	}

	// Detect and build implicit dependencies
	m.detectImplicitDependencies()

	m.logger.WithFields(map[string]interface{}{
		"nodes": len(m.graph.Nodes),
		"edges": m.getTotalEdges(),
	}).Info("Dependency graph built successfully")

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

// detectImplicitDependencies detects implicit dependencies based on resource properties
func (m *Manager) detectImplicitDependencies() {
	m.logger.Debug("Detecting implicit dependencies")

	for nodeID, node := range m.graph.Nodes {
		switch node.ResourceType {
		case "ec2_instance":
			m.detectEC2Dependencies(nodeID, node)
		case "security_group":
			m.detectSecurityGroupDependencies(nodeID, node)
		case "load_balancer":
			m.detectLoadBalancerDependencies(nodeID, node)
		case "auto_scaling_group":
			m.detectASGDependencies(nodeID, node)
		}
	}
}

// detectEC2Dependencies detects dependencies for EC2 instances
func (m *Manager) detectEC2Dependencies(instanceID string, node *types.DependencyNode) {
	// VPC dependency
	if vpcID, exists := node.Properties["vpc_id"]; exists && vpcID != "" {
		m.addEdge(instanceID, vpcID)
	}

	// Subnet dependency
	if subnetID, exists := node.Properties["subnet_id"]; exists && subnetID != "" {
		m.addEdge(instanceID, subnetID)
	}

	// Security group dependencies
	if sgList, exists := node.Properties["security_groups"]; exists {
		securityGroups := strings.Split(sgList, ",")
		for _, sg := range securityGroups {
			sg = strings.TrimSpace(sg)
			if sg != "" && m.graph.Nodes[sg] != nil {
				m.addEdge(instanceID, sg)
			}
		}
	}
}

// detectSecurityGroupDependencies detects dependencies for security groups
func (m *Manager) detectSecurityGroupDependencies(sgID string, node *types.DependencyNode) {
	// VPC dependency
	if vpcID, exists := node.Properties["vpc_id"]; exists && vpcID != "" {
		m.addEdge(sgID, vpcID)
	}
}

// detectLoadBalancerDependencies detects dependencies for load balancers
func (m *Manager) detectLoadBalancerDependencies(lbID string, node *types.DependencyNode) {
	// VPC dependency
	if vpcID, exists := node.Properties["vpc_id"]; exists && vpcID != "" {
		m.addEdge(lbID, vpcID)
	}

	// Subnet dependencies
	if subnets, exists := node.Properties["subnets"]; exists {
		subnetList := strings.Split(subnets, ",")
		for _, subnet := range subnetList {
			subnet = strings.TrimSpace(subnet)
			if subnet != "" && m.graph.Nodes[subnet] != nil {
				m.addEdge(lbID, subnet)
			}
		}
	}

	// Security group dependencies
	if sgList, exists := node.Properties["security_groups"]; exists {
		securityGroups := strings.Split(sgList, ",")
		for _, sg := range securityGroups {
			sg = strings.TrimSpace(sg)
			if sg != "" && m.graph.Nodes[sg] != nil {
				m.addEdge(lbID, sg)
			}
		}
	}
}

// detectASGDependencies detects dependencies for auto scaling groups
func (m *Manager) detectASGDependencies(asgID string, node *types.DependencyNode) {
	// Launch template dependency
	if ltID, exists := node.Properties["launch_template_id"]; exists && ltID != "" {
		m.addEdge(asgID, ltID)
	}

	// Subnet dependencies (VPC zone identifier)
	if vpcZones, exists := node.Properties["vpc_zone_identifier"]; exists {
		subnetList := strings.Split(vpcZones, ",")
		for _, subnet := range subnetList {
			subnet = strings.TrimSpace(subnet)
			if subnet != "" && m.graph.Nodes[subnet] != nil {
				m.addEdge(asgID, subnet)
			}
		}
	}

	// Target group dependencies
	if tgList, exists := node.Properties["target_group_arns"]; exists {
		targetGroups := strings.Split(tgList, ",")
		for _, tg := range targetGroups {
			tg = strings.TrimSpace(tg)
			if tg != "" && m.graph.Nodes[tg] != nil {
				m.addEdge(asgID, tg)
			}
		}
	}
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
