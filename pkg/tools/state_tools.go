package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/adapters"
	"github.com/versus-control/ai-infrastructure-agent/pkg/aws"
	"github.com/versus-control/ai-infrastructure-agent/pkg/interfaces"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
	"github.com/versus-control/ai-infrastructure-agent/pkg/utilities"
)

// =============================================================================
// BASIC STATE ANALYSIS TOOLS (using adapters)
// =============================================================================

// createAllAvailableAdapters dynamically creates all available AWS resource adapters
func createAllAvailableAdapters(awsClient *aws.Client, logger *logging.Logger) map[string]interfaces.AWSResourceAdapter {
	if awsClient == nil {
		return nil
	}

	return map[string]interfaces.AWSResourceAdapter{
		"vpc":            adapters.NewVPCAdapter(awsClient, logger),
		"ec2":            adapters.NewEC2Adapter(awsClient, logger),
		"rds":            adapters.NewRDSAdapter(awsClient, logger),
		"alb":            adapters.NewALBAdapter(awsClient, logger),
		"asg":            adapters.NewASGAdapter(awsClient, logger),
		"security-group": adapters.NewSecurityGroupAdapter(awsClient, logger),
	}
}

// AnalyzeStateTool implements unified infrastructure state analysis with dynamic resource discovery
type AnalyzeStateTool struct {
	*BaseTool
	deps      *ToolDependencies
	awsClient *aws.Client
	adapters  map[string]interfaces.AWSResourceAdapter
}

// NewAnalyzeStateTool creates a unified state analysis tool with dynamic resource discovery
func NewAnalyzeStateTool(deps *ToolDependencies, awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"scan_live": map[string]interface{}{
				"type":        "boolean",
				"description": "Whether to scan live infrastructure and detect drift",
				"default":     true,
			},
			"resource_filter": map[string]interface{}{
				"type":        "array",
				"description": "Optional filter for specific resource types. Supports all AWS resources dynamically: vpc, ec2, rds, alb, asg, security-group, and any other discovered types",
				"items": map[string]interface{}{
					"type": "string",
				},
			},
		},
	}

	baseTool := NewBaseTool(
		"analyze-infrastructure-state",
		"Analyze current infrastructure state with dynamic resource discovery and drift detection",
		"state",
		actionType,
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"Analyze all infrastructure with live scan",
		map[string]interface{}{
			"scan_live": true,
		},
		"Infrastructure analysis completed with drift detection",
	)

	baseTool.AddExample(
		"Analyze specific resource types",
		map[string]interface{}{
			"scan_live":       true,
			"resource_filter": []string{"vpc", "ec2", "alb"},
		},
		"Infrastructure analysis completed for VPC, EC2, and ALB resources",
	)

	// Create adapters for different AWS services dynamically
	adapters := createAllAvailableAdapters(awsClient, logger)

	return &AnalyzeStateTool{
		BaseTool:  baseTool,
		deps:      deps,
		awsClient: awsClient,
		adapters:  adapters,
	}
}

// Execute analyzes infrastructure state using dynamic resource discovery
func (t *AnalyzeStateTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	// Load current state
	if t.deps != nil && t.deps.StateManager != nil {
		if err := t.deps.StateManager.LoadState(ctx); err != nil {
			t.GetLogger().WithError(err).Warn("Failed to load state from file, continuing with current state")
		}
	}

	scanLive := utilities.GetBoolValue(arguments, "scan_live", true)

	// Get resource filter if provided
	var resourceFilter map[string]bool
	if filter, exists := arguments["resource_filter"]; exists {
		if filterArray, ok := filter.([]interface{}); ok {
			resourceFilter = make(map[string]bool)
			for _, item := range filterArray {
				if resourceType, ok := item.(string); ok {
					resourceFilter[resourceType] = true
				}
			}
		}
	}

	t.GetLogger().WithFields(map[string]interface{}{
		"scan_live":       scanLive,
		"resource_filter": resourceFilter,
	}).Info("Analyzing infrastructure state")

	result := map[string]interface{}{
		"analysis_timestamp": time.Now().UTC().Format(time.RFC3339),
		"scan_live":          scanLive,
	}

	// Get managed resources from state if available
	if t.deps != nil && t.deps.StateManager != nil {
		managedResources := t.deps.StateManager.GetState().Resources
		result["managed_resources"] = managedResources
		result["managed_resource_count"] = len(managedResources)
	}

	if scanLive {
		// Use advanced discovery if available
		if t.deps != nil && t.deps.DiscoveryScanner != nil {
			discoveredResources, err := t.deps.DiscoveryScanner.DiscoverInfrastructure(ctx)
			if err != nil {
				t.GetLogger().WithError(err).Warn("Failed to discover infrastructure using advanced scanner, falling back to adapter-based discovery")
				result["discovery_error"] = err.Error()
			} else {
				result["discovered_resources"] = discoveredResources
				result["discovered_resource_count"] = len(discoveredResources)

				// Detect drift if state manager is available
				if t.deps.StateManager != nil {
					var driftDetections []*types.ChangeDetection
					for _, resource := range discoveredResources {
						if _, exists := t.deps.StateManager.GetResource(resource.ID); exists {
							drift, err := t.deps.StateManager.DetectDrift(ctx, resource.Properties, resource.ID)
							if err != nil {
								t.GetLogger().WithError(err).WithField("resource_id", resource.ID).Warn("Failed to detect drift")
								continue
							}
							if drift != nil {
								driftDetections = append(driftDetections, drift)
							}
						}
					}
					result["drift_detections"] = driftDetections
					result["drift_count"] = len(driftDetections)
				}
			}
		} else {
			// Fall back to adapter-based discovery
			result["discovery_method"] = "adapter-based"
			resourceCounts := make(map[string]interface{})

			for resourceType, adapter := range t.adapters {
				// Apply resource filter if specified
				if resourceFilter != nil && !resourceFilter[resourceType] {
					continue
				}

				resources, err := adapter.List(ctx)
				if err != nil {
					t.GetLogger().WithError(err).WithField("resource_type", resourceType).Warn("Failed to list resources")
					resourceCounts[resourceType] = map[string]interface{}{
						"error": err.Error(),
					}
				} else {
					resourceData := make([]map[string]interface{}, 0, len(resources))
					for _, resource := range resources {
						resourceData = append(resourceData, map[string]interface{}{
							"id":      resource.ID,
							"state":   resource.State,
							"details": resource.Details,
						})
					}
					resourceCounts[resourceType] = map[string]interface{}{
						"count":     len(resources),
						"resources": resourceData,
					}
				}
			}
			result["resource_discovery"] = resourceCounts
		}
	}

	// Marshal the structured data as JSON
	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal analysis result: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Type: "text",
				Text: string(jsonData),
			},
		},
	}, nil
}

// ExportStateTool implements unified infrastructure state export with multiple formats and discovery options
type ExportStateTool struct {
	*BaseTool
	deps      *ToolDependencies
	awsClient *aws.Client
	adapters  map[string]interfaces.AWSResourceAdapter
}

// NewExportStateTool creates a unified state export tool with multiple format and discovery options
func NewExportStateTool(deps *ToolDependencies, awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"format": map[string]interface{}{
				"type":        "string",
				"description": "Export format (json, yaml, or text)",
				"default":     "json",
				"enum":        []string{"json", "yaml", "text"},
			},
			"include_discovered": map[string]interface{}{
				"type":        "boolean",
				"description": "Include discovered (unmanaged) resources from live infrastructure",
				"default":     false,
			},
			"include_managed": map[string]interface{}{
				"type":        "boolean",
				"description": "Include managed resources from state file",
				"default":     true,
			},
			"resource_filter": map[string]interface{}{
				"type":        "array",
				"description": "Optional filter for specific resource types. Supports all AWS resources dynamically: vpc, ec2, rds, alb, asg, security-group, and any other discovered types",
				"items": map[string]interface{}{
					"type": "string",
				},
			},
		},
	}

	baseTool := NewBaseTool(
		"export-infrastructure-state",
		"Export infrastructure state with multiple formats and dynamic resource discovery",
		"state",
		actionType,
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"Export all infrastructure as JSON with discovered resources",
		map[string]interface{}{
			"format":             "json",
			"include_discovered": true,
			"include_managed":    true,
		},
		"Infrastructure state exported in JSON format with both managed and discovered resources",
	)

	baseTool.AddExample(
		"Export specific resource types as text",
		map[string]interface{}{
			"format":          "text",
			"resource_filter": []string{"vpc", "ec2", "security-group"},
		},
		"Infrastructure state exported in text format for VPC, EC2, and Security Group resources",
	)

	// Create adapters for different AWS services dynamically
	adapterMap := createAllAvailableAdapters(awsClient, logger)

	return &ExportStateTool{
		BaseTool:  baseTool,
		deps:      deps,
		awsClient: awsClient,
		adapters:  adapterMap,
	}
}

// Execute exports infrastructure state using dynamic resource discovery
func (t *ExportStateTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	t.GetLogger().Info("=== ExportStateTool.Execute called ===")

	format, _ := arguments["format"].(string)
	if format == "" {
		format = "json"
	}

	includeDiscovered := utilities.GetBoolValue(arguments, "include_discovered", false)
	includeManaged := utilities.GetBoolValue(arguments, "include_managed", true)

	// Get resource filter if provided
	var resourceFilter map[string]bool
	if filter, exists := arguments["resource_filter"]; exists {
		if filterArray, ok := filter.([]interface{}); ok {
			resourceFilter = make(map[string]bool)
			for _, item := range filterArray {
				if resourceType, ok := item.(string); ok {
					resourceFilter[resourceType] = true
				}
			}
		}
	}

	// Create structured data for export
	exportData := map[string]interface{}{
		"export_format":      format,
		"timestamp":          time.Now().UTC().Format(time.RFC3339),
		"include_discovered": includeDiscovered,
		"include_managed":    includeManaged,
	}

	// Include managed resources from state file only if requested
	if includeManaged && t.deps != nil && t.deps.StateManager != nil {
		t.GetLogger().Info("About to force reload state from file")
		// Force reload state from file to get latest state
		if err := t.deps.StateManager.LoadState(ctx); err != nil {
			t.GetLogger().WithError(err).Warn("Failed to load state from file, continuing without managed state")
		} else {
			managedState := t.deps.StateManager.GetState()
			t.GetLogger().WithField("managed_resource_count", len(managedState.Resources)).Info("Loaded managed state from file")
			exportData["managed_state"] = managedState
			exportData["managed_resource_count"] = len(managedState.Resources)
		}
	} else {
		t.GetLogger().Info("Skipping managed state loading (include_managed=false)")
	}

	// Include discovered resources
	if includeDiscovered {
		// Use advanced discovery if available
		if t.deps != nil && t.deps.DiscoveryScanner != nil {
			discoveredResources, err := t.deps.DiscoveryScanner.DiscoverInfrastructure(ctx)
			if err != nil {
				t.GetLogger().WithError(err).Warn("Failed to discover infrastructure using advanced scanner, falling back to adapter-based discovery")
				exportData["discovery_error"] = err.Error()
			} else {
				// Apply resource filter if specified
				if resourceFilter != nil {
					var filteredResources []*types.ResourceState
					for _, resource := range discoveredResources {
						if resourceFilter[resource.Type] {
							filteredResources = append(filteredResources, resource)
						}
					}
					discoveredResources = filteredResources
				}

				exportData["discovered_resources"] = discoveredResources
				exportData["discovered_resource_count"] = len(discoveredResources)
			}
		} else if t.adapters != nil {
			// Fall back to adapter-based discovery
			exportData["discovery_method"] = "adapter-based"
			resourcesData := make(map[string]interface{})

			for resourceType, adapter := range t.adapters {
				// Apply resource filter if specified
				if resourceFilter != nil && !resourceFilter[resourceType] {
					continue
				}

				resources, err := adapter.List(ctx)
				if err != nil {
					t.GetLogger().WithError(err).WithField("resource_type", resourceType).Warn("Failed to list resources")
					resourcesData[resourceType] = map[string]interface{}{
						"error": err.Error(),
					}
				} else {
					resourceList := make([]map[string]interface{}, 0, len(resources))
					for _, resource := range resources {
						resourceList = append(resourceList, map[string]interface{}{
							"id":      resource.ID,
							"state":   resource.State,
							"details": resource.Details,
						})
					}
					resourcesData[resourceType] = resourceList
				}
			}
			exportData["discovered_resources"] = resourcesData
		}
	}

	// Format the response based on requested format
	var responseText string

	switch format {
	case "json":
		jsonBytes, jsonErr := json.MarshalIndent(exportData, "", "  ")
		if jsonErr != nil {
			return nil, fmt.Errorf("failed to marshal JSON: %w", jsonErr)
		}
		responseText = string(jsonBytes)

	case "yaml":
		// For YAML, we'll convert to a simpler text format for now
		responseText = t.formatAsText(exportData)

	default: // "text"
		responseText = t.formatAsText(exportData)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Type: "text",
				Text: responseText,
			},
		},
	}, nil
}

// formatAsText converts the export data to a human-readable text format
func (t *ExportStateTool) formatAsText(exportData map[string]interface{}) string {
	result := fmt.Sprintf("Infrastructure State Export (%s)\n", exportData["timestamp"])
	result += fmt.Sprintf("Format: %s\n", exportData["export_format"])
	result += fmt.Sprintf("Include Discovered: %v\n", exportData["include_discovered"])
	result += fmt.Sprintf("Include Managed: %v\n\n", exportData["include_managed"])

	// Format managed state
	if managedState, exists := exportData["managed_state"]; exists {
		result += "=== MANAGED RESOURCES ===\n"
		if state, ok := managedState.(map[string]interface{}); ok {
			if resources, exists := state["resources"]; exists {
				if resourceMap, ok := resources.(map[string]interface{}); ok {
					result += fmt.Sprintf("Managed resources: %d\n", len(resourceMap))
					for resourceID, resource := range resourceMap {
						if resourceData, ok := resource.(map[string]interface{}); ok {
							resourceType := resourceData["type"]
							status := resourceData["status"]
							result += fmt.Sprintf("  - %s (%s): %s\n", resourceID, resourceType, status)
						}
					}
				}
			}
		}
		result += "\n"
	}

	// Format discovered resources
	if discoveredResources, exists := exportData["discovered_resources"]; exists {
		result += "=== DISCOVERED RESOURCES ===\n"

		// Handle advanced discovery format (array of ResourceState)
		if resourceArray, ok := discoveredResources.([]*types.ResourceState); ok {
			result += fmt.Sprintf("Discovered resources: %d\n", len(resourceArray))
			for _, resource := range resourceArray {
				details := t.extractResourceDetails(resource.Properties)
				result += fmt.Sprintf("  - %s (%s): %s%s\n", resource.ID, resource.Type, resource.Status, details)
			}
		} else if resourceMap, ok := discoveredResources.(map[string]interface{}); ok {
			// Handle adapter-based discovery format (map by resource type)
			for resourceType, resources := range resourceMap {
				result += fmt.Sprintf("\n--- %s ---\n", strings.ToUpper(resourceType))
				if resourceList, ok := resources.([]map[string]interface{}); ok {
					for _, resource := range resourceList {
						details := t.extractResourceDetails(resource["details"])
						result += fmt.Sprintf("  - %s (State: %s%s)\n", resource["id"], resource["state"], details)
					}
				} else if errorInfo, ok := resources.(map[string]interface{}); ok {
					if errorMsg, exists := errorInfo["error"]; exists {
						result += fmt.Sprintf("  Error: %s\n", errorMsg)
					}
				}
			}
		}
	}

	return result
}

// extractResourceDetails dynamically extracts the most relevant details from any resource type
func (t *ExportStateTool) extractResourceDetails(details interface{}) string {
	if resourceDetails, ok := details.(map[string]interface{}); ok {
		// Priority list of common important properties across all AWS resource types
		importantProperties := []string{
			// Network properties
			"cidrBlock", "vpcId", "subnetId", "availabilityZone",
			// Compute properties
			"instanceType", "imageId", "state", "launchTime",
			// Database properties
			"engine", "engineVersion", "dbInstanceClass", "allocatedStorage",
			// Load balancer properties
			"scheme", "type", "dnsName", "canonicalHostedZoneId",
			// Security properties
			"groupName", "groupId", "description",
			// Auto scaling properties
			"launchTemplateName", "minSize", "maxSize", "desiredCapacity",
			// Common identifiers and metadata
			"name", "tags", "arn", "status", "createdTime",
		}

		var extractedDetails []string

		// Extract up to 3 most relevant properties
		for _, prop := range importantProperties {
			if value, exists := resourceDetails[prop]; exists {
				if strValue := fmt.Sprintf("%v", value); strValue != "" && strValue != "<nil>" {
					extractedDetails = append(extractedDetails, fmt.Sprintf("%s: %v", prop, value))
					if len(extractedDetails) >= 3 {
						break
					}
				}
			}
		}

		if len(extractedDetails) > 0 {
			return ", " + strings.Join(extractedDetails, ", ")
		}
	}
	return ""
}

// VisualizeDependencyGraphTool generates dependency graph visualization
type VisualizeDependencyGraphTool struct {
	*BaseTool
	deps *ToolDependencies
}

// NewVisualizeDependencyGraphTool creates a new dependency graph visualization tool
func NewVisualizeDependencyGraphTool(deps *ToolDependencies, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"format": map[string]interface{}{
				"type":        "string",
				"description": "Output format: text, mermaid",
				"default":     "text",
				"enum":        []string{"text", "mermaid"},
			},
			"include_bottlenecks": map[string]interface{}{
				"type":        "boolean",
				"description": "Include bottleneck analysis",
				"default":     true,
			},
			"source": map[string]interface{}{
				"type":        "string",
				"description": "Source of resources: 'state' (from state file, default) or 'live' (discover from AWS)",
				"enum":        []string{"state", "live"},
				"default":     "state",
			},
		},
	}

	baseTool := NewBaseTool(
		"visualize-dependency-graph",
		"Generate dependency graph visualization",
		"visualization",
		actionType,
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"Generate text dependency graph from state file",
		map[string]interface{}{
			"format":              "text",
			"include_bottlenecks": true,
			"source":              "state",
		},
		"Dependency graph generated from state file with bottleneck analysis",
	)

	baseTool.AddExample(
		"Generate Mermaid dependency graph from live AWS",
		map[string]interface{}{
			"format":              "mermaid",
			"include_bottlenecks": true,
			"source":              "live",
		},
		"Mermaid diagram generated from live AWS discovery with bottleneck analysis",
	)

	return &VisualizeDependencyGraphTool{
		BaseTool: baseTool,
		deps:     deps,
	}
}

// ValidateArguments validates the tool arguments
func (t *VisualizeDependencyGraphTool) ValidateArguments(args map[string]interface{}) error {
	if format, exists := args["format"]; exists {
		if formatStr, ok := format.(string); ok {
			if formatStr != "text" && formatStr != "mermaid" {
				return fmt.Errorf("format must be 'text' or 'mermaid'")
			}
		} else {
			return fmt.Errorf("format must be a string")
		}
	}

	if includeBottlenecks, exists := args["include_bottlenecks"]; exists {
		if _, ok := includeBottlenecks.(bool); !ok {
			return fmt.Errorf("include_bottlenecks must be a boolean")
		}
	}

	if source, exists := args["source"]; exists {
		if sourceStr, ok := source.(string); ok {
			if sourceStr != "state" && sourceStr != "live" {
				return fmt.Errorf("source must be 'state' or 'live'")
			}
		} else {
			return fmt.Errorf("source must be a string")
		}
	}

	return nil
}

// Execute performs the dependency graph visualization
func (t *VisualizeDependencyGraphTool) Execute(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	includeBottlenecks := true
	if val, ok := args["include_bottlenecks"].(bool); ok {
		includeBottlenecks = val
	}

	source := "state" // default to state file
	if val, ok := args["source"].(string); ok {
		source = val
	}

	// Check if advanced dependencies are available
	if t.deps == nil || t.deps.GraphManager == nil || t.deps.GraphAnalyzer == nil {

		// Return a basic response indicating dependency graph visualization requires advanced tools
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: "Dependency graph visualization requires full infrastructure scanner, graph manager, and graph analyzer dependencies. These advanced features are not currently available.",
				},
			},
		}, nil
	}

	// Get resources based on source parameter
	var stateResources []*types.ResourceState

	if source == "live" {
		// Live AWS discovery mode

		if t.deps.DiscoveryScanner == nil {
			return nil, fmt.Errorf("discovery scanner not available for live mode")
		}

		discoveredResources, err := t.deps.DiscoveryScanner.DiscoverInfrastructure(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to discover infrastructure: %w", err)
		}
		stateResources = discoveredResources

	} else {
		// State file mode (default)

		if t.deps.StateManager == nil {
			return nil, fmt.Errorf("state manager not available for state mode")
		}

		// Load fresh state from file
		if err := t.deps.StateManager.LoadState(ctx); err != nil {
			return nil, fmt.Errorf("failed to load state from file: %w", err)
		}

		// Get all resources from state
		state := t.deps.StateManager.GetState()
		if state != nil && state.Resources != nil {
			// Build mapping from step_reference IDs to actual resource IDs
			stepToResourceMap := make(map[string]string)

			for _, res := range state.Resources {
				if res.Type == "step_reference" {
					// Extract actual resource ID from step_reference using helper
					actualResourceID, err := ExtractResourceIDFromStepReference(res)
					if err != nil {
						return nil, fmt.Errorf("failed to extract resource ID from step reference %s: %w", res.ID, err)
					}
					if actualResourceID != "" {
						stepToResourceMap[res.ID] = actualResourceID
					}
				}
			}

			// Collect resources and resolve dependencies
			for _, res := range state.Resources {
				// Filter out step_reference type resources
				if res.Type != "step_reference" {
					// Resolve dependencies from step IDs to actual resource IDs
					resolvedDeps := make([]string, 0, len(res.Dependencies))
					for _, depID := range res.Dependencies {
						if actualID, exists := stepToResourceMap[depID]; exists {
							resolvedDeps = append(resolvedDeps, actualID)
						} else {
							// Keep original dependency if no mapping found
							resolvedDeps = append(resolvedDeps, depID)
						}
					}

					// Create a copy with resolved dependencies
					resolvedResource := *res
					resolvedResource.Dependencies = resolvedDeps
					stateResources = append(stateResources, &resolvedResource)
				}
			}
		}
	}

	// Build dependency graph
	if err := t.deps.GraphManager.BuildGraph(ctx, stateResources); err != nil {
		return nil, fmt.Errorf("failed to build dependency graph: %w", err)
	}

	analyzer := t.deps.GraphAnalyzer
	visualization := analyzer.GenerateMermaidDiagram()

	// Build structured graph data (nodes and edges)
	nodes := make([]map[string]interface{}, 0)
	edges := make([]map[string]interface{}, 0)
	nodeMap := make(map[string]bool)
	resourceTypeCounts := make(map[string]int)
	dependentCounts := make(map[string]int)

	// First pass: create nodes
	for _, resource := range stateResources {
		if resource.ID == "" {
			continue
		}

		nodeMap[resource.ID] = true
		resourceTypeCounts[resource.Type]++

		node := map[string]interface{}{
			"id":     resource.ID,
			"type":   resource.Type,
			"status": resource.Status,
		}

		// Add name if available
		if resource.Name != "" {
			node["name"] = resource.Name
		} else if name, ok := resource.Properties["name"]; ok {
			if nameStr, ok := name.(string); ok && nameStr != "" {
				node["name"] = nameStr
			}
		}

		// Add tags if available
		if len(resource.Tags) > 0 {
			node["tags"] = resource.Tags
		} else if tags, ok := resource.Properties["tags"]; ok {
			node["tags"] = tags
		}

		nodes = append(nodes, node)
	}

	// Second pass: create edges and count dependents
	for _, resource := range stateResources {
		if resource.ID == "" {
			continue
		}

		for _, depID := range resource.Dependencies {
			if depID == "" {
				continue
			}

			// Only create edge if both nodes exist
			if nodeMap[resource.ID] && nodeMap[depID] {
				edges = append(edges, map[string]interface{}{
					"source": resource.ID,
					"target": depID,
				})
				dependentCounts[depID]++
			}
		}
	}

	// Get bottlenecks from analyzer and merge with dependent counts
	var bottlenecksData []map[string]interface{}
	if includeBottlenecks {
		bottlenecks := analyzer.FindBottlenecks()
		bottlenecksData = make([]map[string]interface{}, 0, len(bottlenecks))
		for _, b := range bottlenecks {
			bottlenecksData = append(bottlenecksData, map[string]interface{}{
				"resourceId":     b.ResourceID,
				"resourceType":   b.ResourceType,
				"dependentCount": b.DependentCount,
				"dependents":     b.Dependents,
				"impact":         b.Impact,
			})
		}
	}

	// Build complete response structure
	responseData := map[string]interface{}{
		"visualization": visualization,
		"graph": map[string]interface{}{
			"nodes": nodes,
			"edges": edges,
		},
		"metadata": map[string]interface{}{
			"nodeCount":          len(nodes),
			"edgeCount":          len(edges),
			"resourceTypeCounts": resourceTypeCounts,
			"bottlenecks":        bottlenecksData,
		},
	}

	// Convert to JSON and return as text content
	responseJSON, err := json.Marshal(responseData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal graph response: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(responseJSON),
			},
		},
	}, nil
}

// DetectConflictsTool detects conflicts in infrastructure configuration
type DetectConflictsTool struct {
	*BaseTool
	deps *ToolDependencies
}

// NewDetectConflictsTool creates a new conflict detection tool
func NewDetectConflictsTool(deps *ToolDependencies, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"auto_resolve": map[string]interface{}{
				"type":        "boolean",
				"description": "Automatically resolve conflicts where possible",
				"default":     false,
			},
		},
	}

	baseTool := NewBaseTool(
		"detect-infrastructure-conflicts",
		"Detect conflicts in infrastructure configuration",
		"analysis",
		actionType,
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"Detect conflicts with auto-resolution",
		map[string]interface{}{
			"auto_resolve": true,
		},
		"Conflicts detected and resolved automatically",
	)

	return &DetectConflictsTool{
		BaseTool: baseTool,
		deps:     deps,
	}
}

// ValidateArguments validates the tool arguments
func (t *DetectConflictsTool) ValidateArguments(args map[string]interface{}) error {
	if autoResolve, exists := args["auto_resolve"]; exists {
		if _, ok := autoResolve.(bool); !ok {
			return fmt.Errorf("auto_resolve must be a boolean")
		}
	}
	return nil
}

// Execute performs conflict detection
func (t *DetectConflictsTool) Execute(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	autoResolve := false
	if val, ok := args["auto_resolve"].(bool); ok {
		autoResolve = val
	}

	t.GetLogger().WithField("auto_resolve", autoResolve).Info("Detecting infrastructure conflicts")

	// Check if advanced dependencies are available
	if t.deps == nil || t.deps.DiscoveryScanner == nil || t.deps.ConflictResolver == nil {
		t.GetLogger().Warn("Advanced conflict detection dependencies not available, using basic conflict detection")

		// Return a basic response indicating no conflicts can be detected without advanced tools
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: "Advanced conflict detection requires full infrastructure scanner and conflict resolver dependencies. Currently showing 0 conflicts due to missing dependencies.",
				},
			},
		}, nil
	}

	// Discover infrastructure
	discoveredResources, err := t.deps.DiscoveryScanner.DiscoverInfrastructure(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to discover infrastructure: %w", err)
	}

	// Detect conflicts
	conflicts, err := t.deps.ConflictResolver.DetectConflicts(ctx, discoveredResources)
	if err != nil {
		return nil, fmt.Errorf("failed to detect conflicts: %w", err)
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Found %d conflicts\n\n", len(conflicts)))

	for _, conflict := range conflicts {
		output.WriteString(fmt.Sprintf("- %s: %s (Resource: %s)\n",
			conflict.ConflictType, conflict.Details, conflict.ResourceID))
	}

	if autoResolve && len(conflicts) > 0 {
		output.WriteString("\nResolution Results:\n")
		for _, conflict := range conflicts {
			resolution, err := t.deps.ConflictResolver.ResolveConflict(ctx, conflict)
			if err != nil {
				t.GetLogger().WithError(err).WithField("resource_id", conflict.ResourceID).Warn("Failed to resolve conflict")
				output.WriteString(fmt.Sprintf("- Failed to resolve %s: %v\n", conflict.ResourceID, err))
				continue
			}
			output.WriteString(fmt.Sprintf("- Resolved %s: %v\n", conflict.ResourceID, resolution))
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: output.String(),
			},
		},
	}, nil
}

// SaveStateTool saves current infrastructure state to persistent storage
type SaveStateTool struct {
	*BaseTool
	deps *ToolDependencies
}

// NewSaveStateTool creates a new state save tool
func NewSaveStateTool(deps *ToolDependencies, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"force": map[string]interface{}{
				"type":        "boolean",
				"description": "Force save even if state hasn't changed",
				"default":     false,
			},
		},
	}

	baseTool := NewBaseTool(
		"save-state",
		"Save current infrastructure state to persistent storage",
		"state",
		actionType,
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"Force save state",
		map[string]interface{}{
			"force": true,
		},
		"Infrastructure state saved successfully",
	)

	return &SaveStateTool{
		BaseTool: baseTool,
		deps:     deps,
	}
}

// ValidateArguments validates the tool arguments
func (t *SaveStateTool) ValidateArguments(args map[string]interface{}) error {
	if force, exists := args["force"]; exists {
		if _, ok := force.(bool); !ok {
			return fmt.Errorf("force must be a boolean")
		}
	}
	return nil
}

// Execute performs state save
func (t *SaveStateTool) Execute(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Load state
	if err := t.deps.StateManager.LoadState(ctx); err != nil {
		t.GetLogger().WithError(err).Warn("Failed to load state from file, continuing with current state")
	}

	force := false
	if val, ok := args["force"].(bool); ok {
		force = val
	}

	t.GetLogger().WithField("force", force).Info("Saving infrastructure state")

	// Save state using the state manager
	if err := t.deps.StateManager.SaveState(ctx); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf(`{"success": false, "error": "failed to save state: %v"}`, err),
				},
			},
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: `{"success": true, "message": "Infrastructure state saved successfully"}`,
			},
		},
	}, nil
}

// AddResourceToStateTool adds a resource to the managed infrastructure state
type AddResourceToStateTool struct {
	*BaseTool
	deps *ToolDependencies
}

// NewAddResourceToStateTool creates a new add resource tool
func NewAddResourceToStateTool(deps *ToolDependencies, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"resource_id": map[string]interface{}{
				"type":        "string",
				"description": "Resource ID",
			},
			"resource_name": map[string]interface{}{
				"type":        "string",
				"description": "Resource name",
			},
			"description": map[string]interface{}{
				"type":        "string",
				"description": "Resource description",
			},
			"resource_type": map[string]interface{}{
				"type":        "string",
				"description": "Resource type",
			},
			"status": map[string]interface{}{
				"type":        "string",
				"description": "Resource status",
				"default":     "created",
			},
			"properties": map[string]interface{}{
				"type":        "object",
				"description": "Resource properties",
			},
			"dependencies": map[string]interface{}{
				"type":        "array",
				"description": "Resource dependencies",
				"items": map[string]interface{}{
					"type": "string",
				},
			},
		},
		"required": []string{"resource_id", "resource_name", "resource_type"},
	}

	baseTool := NewBaseTool(
		"add-resource-to-state",
		"Add a resource to the managed infrastructure state",
		"state",
		actionType,
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"Add EC2 instance to state",
		map[string]interface{}{
			"resource_id":   "i-1234567890abcdef0",
			"resource_name": "web-server",
			"resource_type": "ec2-instance",
			"status":        "running",
			"properties": map[string]interface{}{
				"instance_type": "t3.micro",
				"ami_id":        "ami-0abcdef1234567890",
			},
		},
		"Resource added to managed state successfully",
	)

	return &AddResourceToStateTool{
		BaseTool: baseTool,
		deps:     deps,
	}
}

// ValidateArguments validates the tool arguments
func (t *AddResourceToStateTool) ValidateArguments(args map[string]interface{}) error {
	// Required fields
	for _, field := range []string{"resource_id", "resource_name", "resource_type"} {
		if val, exists := args[field]; !exists {
			return fmt.Errorf("%s is required", field)
		} else if str, ok := val.(string); !ok || str == "" {
			return fmt.Errorf("%s must be a non-empty string", field)
		}
	}

	// Optional fields
	if status, exists := args["status"]; exists {
		if _, ok := status.(string); !ok {
			return fmt.Errorf("status must be a string")
		}
	}

	if properties, exists := args["properties"]; exists {
		if _, ok := properties.(map[string]interface{}); !ok {
			return fmt.Errorf("properties must be an object")
		}
	}

	if dependencies, exists := args["dependencies"]; exists {
		if deps, ok := dependencies.([]interface{}); ok {
			for _, dep := range deps {
				if _, ok := dep.(string); !ok {
					return fmt.Errorf("dependencies must be an array of strings")
				}
			}
		} else {
			return fmt.Errorf("dependencies must be an array")
		}
	}

	return nil
}

// Execute performs adding resource to state
func (t *AddResourceToStateTool) Execute(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Load state
	if err := t.deps.StateManager.LoadState(ctx); err != nil {
		t.GetLogger().WithError(err).Warn("Failed to load state from file, continuing with current state")
	}

	resourceID := args["resource_id"].(string)
	resourceName := args["resource_name"].(string)
	resourceType := args["resource_type"].(string)

	description := ""
	if val, ok := args["description"].(string); ok {
		description = val
	}

	status := "created"
	if val, ok := args["status"].(string); ok {
		status = val
	}

	var properties map[string]interface{}
	if val, ok := args["properties"].(map[string]interface{}); ok {
		properties = val
	} else {
		properties = make(map[string]interface{})
	}

	var dependencies []string
	if val, ok := args["dependencies"].([]interface{}); ok {
		for _, dep := range val {
			if depStr, ok := dep.(string); ok {
				dependencies = append(dependencies, depStr)
			}
		}
	}

	t.GetLogger().WithFields(map[string]interface{}{
		"resource_id":   resourceID,
		"resource_name": resourceName,
		"resource_type": resourceType,
		"status":        status,
	}).Info("Adding resource to managed state")

	// Create resource state
	resourceState := &types.ResourceState{
		ID:           resourceID,
		Name:         resourceName,
		Description:  description,
		Type:         resourceType,
		Status:       status,
		Properties:   properties,
		Dependencies: dependencies,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Add to state manager
	if err := t.deps.StateManager.AddResource(ctx, resourceState); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf(`{"success": false, "error": "failed to add resource to state: %v"}`, err),
				},
			},
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf(`{"success": true, "message": "Resource %s added to managed state", "resource_id": "%s"}`, resourceID, resourceID),
			},
		},
	}, nil
}

// PlanDeploymentTool generates deployment plan with dependency ordering
type PlanDeploymentTool struct {
	*BaseTool
	deps *ToolDependencies
}

// NewPlanDeploymentTool creates a new deployment planning tool
func NewPlanDeploymentTool(deps *ToolDependencies, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"include_levels": map[string]interface{}{
				"type":        "boolean",
				"description": "Include deployment levels for parallel execution",
				"default":     true,
			},
			"target_resources": map[string]interface{}{
				"type":        "array",
				"description": "Target specific resources for deployment planning",
				"items": map[string]interface{}{
					"type": "string",
				},
			},
		},
	}

	baseTool := NewBaseTool(
		"plan-infrastructure-deployment",
		"Generate deployment plan with dependency ordering",
		"planning",
		actionType,
		inputSchema,
		logger,
	)

	baseTool.AddExample(
		"Plan deployment with levels",
		map[string]interface{}{
			"include_levels": true,
		},
		"Deployment plan generated with dependency levels",
	)

	return &PlanDeploymentTool{
		BaseTool: baseTool,
		deps:     deps,
	}
}

// ValidateArguments validates the tool arguments
func (t *PlanDeploymentTool) ValidateArguments(args map[string]interface{}) error {
	if includeLevels, exists := args["include_levels"]; exists {
		if _, ok := includeLevels.(bool); !ok {
			return fmt.Errorf("include_levels must be a boolean")
		}
	}

	if targetResources, exists := args["target_resources"]; exists {
		if resources, ok := targetResources.([]interface{}); ok {
			for _, resource := range resources {
				if _, ok := resource.(string); !ok {
					return fmt.Errorf("target_resources must be an array of strings")
				}
			}
		} else {
			return fmt.Errorf("target_resources must be an array")
		}
	}

	return nil
}

// Execute performs deployment planning
func (t *PlanDeploymentTool) Execute(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	var targetResources []string
	if val, ok := args["target_resources"].([]interface{}); ok {
		for _, v := range val {
			if str, ok := v.(string); ok {
				targetResources = append(targetResources, str)
			}
		}
	}

	includeLevels := true
	if val, ok := args["include_levels"].(bool); ok {
		includeLevels = val
	}

	t.GetLogger().WithFields(map[string]interface{}{
		"target_resources": targetResources,
		"include_levels":   includeLevels,
	}).Info("Planning infrastructure deployment")

	// Check if advanced dependencies are available
	if t.deps == nil || t.deps.DiscoveryScanner == nil || t.deps.GraphManager == nil {
		t.GetLogger().Warn("Advanced deployment planning dependencies not available")

		// Return a basic response indicating deployment planning requires advanced tools
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: "Deployment planning requires full infrastructure scanner and graph manager dependencies. These advanced features are not currently available.",
				},
			},
		}, nil
	}

	// Discover infrastructure to build graph
	discoveredResources, err := t.deps.DiscoveryScanner.DiscoverInfrastructure(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to discover infrastructure: %w", err)
	}

	// Filter target resources if specified
	if len(targetResources) > 0 {
		var filteredResources []*types.ResourceState
		targetMap := make(map[string]bool)
		for _, target := range targetResources {
			targetMap[target] = true
		}

		for _, resource := range discoveredResources {
			if targetMap[resource.ID] {
				filteredResources = append(filteredResources, resource)
			}
		}
		discoveredResources = filteredResources
	}

	// Build dependency graph
	if err := t.deps.GraphManager.BuildGraph(ctx, discoveredResources); err != nil {
		return nil, fmt.Errorf("failed to build dependency graph: %w", err)
	}

	// Get deployment order
	deploymentOrder, err := t.deps.GraphManager.GetDeploymentOrder()
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment order: %w", err)
	}

	var output strings.Builder
	output.WriteString("Deployment Plan:\n\n")
	output.WriteString("Deployment Order:\n")
	for i, resourceID := range deploymentOrder {
		output.WriteString(fmt.Sprintf("%d. %s\n", i+1, resourceID))
	}

	if includeLevels {
		deploymentLevels, err := t.deps.GraphManager.CalculateDeploymentLevels()
		if err != nil {
			return nil, fmt.Errorf("failed to calculate deployment levels: %w", err)
		}

		output.WriteString("\nDeployment Levels (for parallel execution):\n")
		for i, level := range deploymentLevels {
			output.WriteString(fmt.Sprintf("Level %d: %s\n", i+1, strings.Join(level, ", ")))
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: output.String(),
			},
		},
	}, nil
}
