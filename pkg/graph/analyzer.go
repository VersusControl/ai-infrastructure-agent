package graph

import (
	"fmt"
	"strings"

	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// Analyzer provides analysis capabilities for dependency graphs
type Analyzer struct {
	manager *Manager
}

// GraphComplexity represents complexity metrics of the dependency graph
type GraphComplexity struct {
	TotalNodes      int            `json:"totalNodes"`
	TotalEdges      int            `json:"totalEdges"`
	MaxDependencies int            `json:"maxDependencies"`
	MaxDependents   int            `json:"maxDependents"`
	Density         float64        `json:"density"`
	DependencyCount map[string]int `json:"dependencyCount"`
	DependentCount  map[string]int `json:"dependentCount"`
}

// Bottleneck represents a potential bottleneck in the dependency graph
type Bottleneck struct {
	ResourceID     string   `json:"resourceId"`
	ResourceType   string   `json:"resourceType"`
	DependentCount int      `json:"dependentCount"`
	Dependents     []string `json:"dependents"`
	Impact         string   `json:"impact"` // low, medium, high, critical
}

// NewAnalyzer creates a new graph analyzer
func NewAnalyzer(manager *Manager) *Analyzer {
	return &Analyzer{
		manager: manager,
	}
}

// AnalyzeComplexity analyzes the complexity of the dependency graph
func (a *Analyzer) AnalyzeComplexity() *GraphComplexity {
	graph := a.manager.GetGraph()

	totalNodes := len(graph.Nodes)
	totalEdges := 0
	maxDependencies := 0
	maxDependents := 0

	dependencyCount := make(map[string]int)
	dependentCount := make(map[string]int)

	// Count edges and dependencies
	for nodeID, edges := range graph.Edges {
		edgeCount := len(edges)
		totalEdges += edgeCount
		dependencyCount[nodeID] = edgeCount

		if edgeCount > maxDependencies {
			maxDependencies = edgeCount
		}

		for _, depID := range edges {
			dependentCount[depID]++
		}
	}

	// Find max dependents
	for _, count := range dependentCount {
		if count > maxDependents {
			maxDependents = count
		}
	}

	// Calculate density
	maxPossibleEdges := totalNodes * (totalNodes - 1)
	density := 0.0
	if maxPossibleEdges > 0 {
		density = float64(totalEdges) / float64(maxPossibleEdges)
	}

	return &GraphComplexity{
		TotalNodes:      totalNodes,
		TotalEdges:      totalEdges,
		MaxDependencies: maxDependencies,
		MaxDependents:   maxDependents,
		Density:         density,
		DependencyCount: dependencyCount,
		DependentCount:  dependentCount,
	}
}

// FindBottlenecks identifies potential bottlenecks in the dependency graph
func (a *Analyzer) FindBottlenecks() []Bottleneck {
	var bottlenecks []Bottleneck
	graph := a.manager.GetGraph()

	// Count dependents for each node
	dependentCount := make(map[string]int)
	for _, edges := range graph.Edges {
		for _, depID := range edges {
			dependentCount[depID]++
		}
	}

	// Identify nodes with high dependent count as bottlenecks
	for nodeID, count := range dependentCount {
		if count >= 3 { // Threshold for bottleneck
			node := graph.Nodes[nodeID]
			bottleneck := Bottleneck{
				ResourceID:     nodeID,
				ResourceType:   node.ResourceType,
				DependentCount: count,
				Dependents:     a.manager.GetDependents(nodeID),
				Impact:         a.calculateImpact(count),
			}
			bottlenecks = append(bottlenecks, bottleneck)
		}
	}

	return bottlenecks
}

// calculateImpact calculates the impact level of a bottleneck
func (a *Analyzer) calculateImpact(dependentCount int) string {
	if dependentCount >= 10 {
		return "critical"
	} else if dependentCount >= 5 {
		return "high"
	} else if dependentCount >= 3 {
		return "medium"
	}
	return "low"
}

// FindCycles detects cycles in the dependency graph
func (a *Analyzer) FindCycles() [][]string {
	var cycles [][]string
	graph := a.manager.GetGraph()
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	var findCycle func(nodeID string, path []string) []string
	findCycle = func(nodeID string, path []string) []string {
		if recStack[nodeID] {
			// Found a cycle, return the cycle path
			cycleStart := -1
			for i, node := range path {
				if node == nodeID {
					cycleStart = i
					break
				}
			}
			if cycleStart >= 0 {
				return append(path[cycleStart:], nodeID)
			}
		}

		if visited[nodeID] {
			return nil
		}

		visited[nodeID] = true
		recStack[nodeID] = true
		currentPath := append(path, nodeID)

		for _, depID := range graph.Edges[nodeID] {
			if cycle := findCycle(depID, currentPath); cycle != nil {
				return cycle
			}
		}

		recStack[nodeID] = false
		return nil
	}

	for nodeID := range graph.Nodes {
		if !visited[nodeID] {
			if cycle := findCycle(nodeID, []string{}); cycle != nil {
				cycles = append(cycles, cycle)
			}
		}
	}

	return cycles
}

// GenerateMermaidDiagram generates a Mermaid diagram representation
func (a *Analyzer) GenerateMermaidDiagram() string {
	var builder strings.Builder
	graph := a.manager.GetGraph()

	builder.WriteString("graph TD\n")

	// Add nodes with styling based on type
	for nodeID, node := range graph.Nodes {
		sanitizedID := a.sanitizeID(nodeID)
		nodeLabel := a.getNodeLabel(nodeID, node)
		nodeStyle := a.getNodeStyle(node.ResourceType)

		builder.WriteString(fmt.Sprintf("    %s[\"%s\"]%s\n", sanitizedID, nodeLabel, nodeStyle))
	}

	builder.WriteString("\n")

	// Add edges
	for nodeID, edges := range graph.Edges {
		sanitizedFromID := a.sanitizeID(nodeID)
		for _, depID := range edges {
			sanitizedToID := a.sanitizeID(depID)
			builder.WriteString(fmt.Sprintf("    %s --> %s\n", sanitizedFromID, sanitizedToID))
		}
	}

	// Add styling
	builder.WriteString("\n")
	builder.WriteString("    classDef vpc fill:#e1f5fe\n")
	builder.WriteString("    classDef subnet fill:#b3e5fc\n")
	builder.WriteString("    classDef ec2 fill:#fff3e0\n")
	builder.WriteString("    classDef sg fill:#f3e5f5\n")
	builder.WriteString("    classDef lb fill:#e8f5e8\n")
	builder.WriteString("    classDef asg fill:#fce4ec\n")
	builder.WriteString("    classDef natgw fill:#e0f2f1\n")
	builder.WriteString("    classDef tg fill:#f1f8e9\n")
	builder.WriteString("    classDef rds fill:#fff9c4\n")
	builder.WriteString("    classDef lt fill:#fce4ec\n")

	return builder.String()
}

// sanitizeID sanitizes node IDs for Mermaid compatibility
func (a *Analyzer) sanitizeID(id string) string {
	// Replace problematic characters
	sanitized := strings.ReplaceAll(id, "-", "_")
	sanitized = strings.ReplaceAll(sanitized, ":", "_")
	sanitized = strings.ReplaceAll(sanitized, "/", "_")
	return sanitized
}

// getNodeLabel creates a display label for a node
func (a *Analyzer) getNodeLabel(nodeID string, node *types.DependencyNode) string {
	label := nodeID
	if name, exists := node.Properties["name"]; exists && name != "" {
		label = fmt.Sprintf("%s\\n(%s)", name, nodeID)
	}
	return label
}

// getNodeStyle returns the CSS class for a node type
func (a *Analyzer) getNodeStyle(resourceType string) string {
	switch resourceType {
	case "vpc":
		return ":::vpc"
	case "subnet":
		return ":::subnet"
	case "ec2_instance":
		return ":::ec2"
	case "security_group":
		return ":::sg"
	case "load_balancer":
		return ":::lb"
	case "auto_scaling_group":
		return ":::asg"
	case "nat_gateway":
		return ":::natgw"
	case "target_group":
		return ":::tg"
	case "rds_instance":
		return ":::rds"
	case "launch_template":
		return ":::lt"
	default:
		return ""
	}
}
