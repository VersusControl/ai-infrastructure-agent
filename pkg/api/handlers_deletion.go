package api

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/graph"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// getDeletionPlanHandler generates a deletion plan with resources in proper deletion order
// This endpoint is safe because it doesn't delete anything - it only returns resource information
// The frontend will use this data to generate bash scripts for the user
//
// Supports:
//   - GET /api/deletion-plan - Get deletion plan for all resources
//   - POST /api/deletion-plan - Get deletion plan for specific resources
//     Request body: { "resourceIds": ["vpc-123", "subnet-456"], "resourceType": "vpc" }
func (ws *WebServer) getDeletionPlanHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Check if config is available
	if ws.config == nil {
		http.Error(w, `{"error": "Configuration not available"}`, http.StatusServiceUnavailable)
		return
	}

	ctx := r.Context()

	// Parse request body for POST requests (optional filters)
	var request DeletionPlanRequest
	if r.Method == http.MethodPost {
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			if ws.aiAgent != nil {
				ws.aiAgent.Logger.WithError(err).Error("Failed to parse deletion plan request")
			}
			http.Error(w, `{"error": "Invalid request body"}`, http.StatusBadRequest)
			return
		}
	}

	if ws.aiAgent != nil {
		ws.aiAgent.Logger.WithFields(map[string]interface{}{
			"resource_ids":  request.ResourceIDs,
			"resource_type": request.ResourceType,
		}).Info("Generating deletion plan")
	}

	// Get the state file path from config
	stateFilePath := ws.config.GetStateFilePath()
	if stateFilePath == "" {
		if ws.aiAgent != nil {
			ws.aiAgent.Logger.Error("State file path not configured")
		}
		http.Error(w, `{"error": "State file path not configured"}`, http.StatusInternalServerError)
		return
	}

	// Read the current state file directly
	data, err := os.ReadFile(stateFilePath)
	if err != nil {
		if ws.aiAgent != nil {
			ws.aiAgent.Logger.WithError(err).Error("Failed to read state file")
		}
		http.Error(w, `{"error": "Failed to read state file"}`, http.StatusInternalServerError)
		return
	}

	// Parse the state JSON
	var state types.InfrastructureState
	if err := json.Unmarshal(data, &state); err != nil {
		if ws.aiAgent != nil {
			ws.aiAgent.Logger.WithError(err).Error("Failed to parse state JSON")
		}
		http.Error(w, `{"error": "Failed to parse state"}`, http.StatusInternalServerError)
		return
	}

	if len(state.Resources) == 0 {
		// Return empty deletion plan
		emptyPlan := DeletionPlan{
			Resources:      []DeletionPlanResource{},
			DeletionOrder:  []string{},
			TotalResources: 0,
			GeneratedAt:    time.Now(),
			Region:         "",
			Warnings:       []string{"No resources found in state"},
		}
		json.NewEncoder(w).Encode(emptyPlan)
		return
	}

	// Build dependency graph
	var logger *logging.Logger
	if ws.aiAgent != nil {
		logger = ws.aiAgent.Logger
	}
	graphManager := graph.NewManager(logger)
	var resourceList []*types.ResourceState
	for _, resource := range state.Resources {
		resourceList = append(resourceList, resource)
	}

	if err := graphManager.BuildGraph(ctx, resourceList); err != nil {
		if ws.aiAgent != nil {
			ws.aiAgent.Logger.WithError(err).Error("Failed to build dependency graph")
		}
		http.Error(w, `{"error": "Failed to build dependency graph"}`, http.StatusInternalServerError)
		return
	}

	// Get deletion order (reverse of deployment order)
	// If there are circular dependencies, we'll break them automatically
	deletionOrder, err := graphManager.GetDeletionOrder()
	warnings := []string{}

	if err != nil {
		// Circular dependency detected - break the cycle and provide safe ordering
		if ws.aiAgent != nil {
			ws.aiAgent.Logger.WithError(err).Warn("Circular dependencies detected, breaking cycles automatically")
		}

		// Get deletion order with circular dependency handling
		deletionOrder, warnings = ws.handleCircularDependencies(&state, graphManager)
	}

	// Filter resources if requested
	filteredOrder := deletionOrder
	if len(request.ResourceIDs) > 0 || request.ResourceType != "" {
		filteredOrder = ws.filterDeletionOrder(deletionOrder, &state, request)
	}

	// Build deletion plan
	deletionPlan := DeletionPlan{
		Resources:      []DeletionPlanResource{},
		DeletionOrder:  filteredOrder,
		TotalResources: len(filteredOrder),
		GeneratedAt:    time.Now(),
		Region:         state.Region,
		Warnings:       warnings,
	}

	// Add resources with dependency information
	for _, resourceID := range filteredOrder {
		resource, exists := state.Resources[resourceID]
		if !exists {
			deletionPlan.Warnings = append(deletionPlan.Warnings,
				"Resource "+resourceID+" in deletion order but not found in state")
			continue
		}

		// Get dependents (resources that depend on this one)
		dependents := graphManager.GetDependents(resourceID)

		// Extract minimal useful properties for deletion
		minimalProps := ws.extractMinimalProperties(resource)

		deletionResource := DeletionPlanResource{
			ID:           resource.ID,
			Name:         resource.Name,
			Type:         resource.Type,
			Status:       resource.Status,
			Properties:   minimalProps,
			Dependencies: resource.Dependencies,
			Dependents:   dependents,
			CreatedAt:    resource.CreatedAt,
		}

		// Extract region from properties if available
		if region, ok := resource.Properties["region"].(string); ok {
			deletionResource.Region = region
		}

		deletionPlan.Resources = append(deletionPlan.Resources, deletionResource)
	}

	// Return the deletion plan
	if err := json.NewEncoder(w).Encode(deletionPlan); err != nil {
		if ws.aiAgent != nil {
			ws.aiAgent.Logger.WithError(err).Error("Failed to encode deletion plan response")
		}
		http.Error(w, `{"error": "Failed to generate response"}`, http.StatusInternalServerError)
		return
	}

	if ws.aiAgent != nil {
		ws.aiAgent.Logger.WithField("total_resources", deletionPlan.TotalResources).
			Info("Deletion plan generated successfully")
	}
}

// filterDeletionOrder filters the deletion order based on request parameters
func (ws *WebServer) filterDeletionOrder(
	deletionOrder []string,
	state *types.InfrastructureState,
	request DeletionPlanRequest,
) []string {
	// Build a set of requested resource IDs
	requestedIDs := make(map[string]bool)
	if len(request.ResourceIDs) > 0 {
		for _, id := range request.ResourceIDs {
			requestedIDs[id] = true
		}
	}

	// Filter the deletion order
	filtered := []string{}
	for _, resourceID := range deletionOrder {
		resource, exists := state.Resources[resourceID]
		if !exists {
			continue
		}

		// Check if resource matches filters
		include := true

		// Filter by resource IDs if specified
		if len(requestedIDs) > 0 && !requestedIDs[resourceID] {
			include = false
		}

		// Filter by resource type if specified
		if request.ResourceType != "" && resource.Type != request.ResourceType {
			include = false
		}

		if include {
			filtered = append(filtered, resourceID)
		}
	}

	return filtered
}

// handleCircularDependencies resolves circular dependencies and returns AWS-compliant deletion order
// This function implements AWS resource dependency rules based on actual constraint enforcement.
// AWS enforces dependencies through ENI attachments, subnet associations, and VPC containment.
// Filters out non-AWS resources (step_reference, ami, etc.)
func (ws *WebServer) handleCircularDependencies(state *types.InfrastructureState, graphManager *graph.Manager) ([]string, []string) {
	warnings := []string{
		"Circular dependencies detected and automatically resolved.",
	}

	// Skip these resource types - they are not actual AWS resources to delete
	skipTypes := map[string]bool{
		"step_reference": true,
		"ami":            true, // AMIs are discovered, not created
		"key_pair":       true, // Key pairs are often pre-existing
	}

	// Separate resources by type for AWS-compliant ordering
	var listeners []string
	var targetGroups []string
	var loadBalancers []string
	var asgs []string
	var instances []string
	var launchTemplates []string
	var dbInstances []string
	var dbSubnetGroups []string
	var securityGroups []string
	var natGateways []string
	var subnets []string
	var routeTables []string
	var internetGateways []string
	var vpcs []string

	for resourceID, resource := range state.Resources {
		// Skip non-AWS resources
		if skipTypes[resource.Type] {
			continue
		}

		switch resource.Type {
		case "listener":
			listeners = append(listeners, resourceID)
		case "target_group":
			targetGroups = append(targetGroups, resourceID)
		case "load_balancer", "alb":
			loadBalancers = append(loadBalancers, resourceID)
		case "auto_scaling_group", "asg":
			asgs = append(asgs, resourceID)
		case "ec2_instance":
			instances = append(instances, resourceID)
		case "launch_template":
			launchTemplates = append(launchTemplates, resourceID)
		case "rds_instance", "db_instance":
			dbInstances = append(dbInstances, resourceID)
		case "db_subnet_group":
			dbSubnetGroups = append(dbSubnetGroups, resourceID)
		case "security_group":
			securityGroups = append(securityGroups, resourceID)
		case "nat_gateway":
			natGateways = append(natGateways, resourceID)
		case "subnet":
			subnets = append(subnets, resourceID)
		case "route_table":
			routeTables = append(routeTables, resourceID)
		case "internet_gateway", "igw":
			internetGateways = append(internetGateways, resourceID)
		case "vpc":
			vpcs = append(vpcs, resourceID)
		}
	}

	// AWS-Compliant Deletion Order
	// Based on actual AWS dependency constraints and ENI attachment rules
	deletionOrder := []string{}

	// Level 1: Application Load Balancer components (highest dependency level)
	// Listeners depend on: Load Balancer + Target Group
	// Must delete listeners before their parent load balancer
	deletionOrder = append(deletionOrder, listeners...)

	// Level 2: Load Balancing tier
	// Load Balancers have ENIs attached to subnets with security groups
	// Must delete before security groups can be removed
	deletionOrder = append(deletionOrder, loadBalancers...)

	// Target Groups depend on: VPC
	// Can be deleted after load balancers are removed
	deletionOrder = append(deletionOrder, targetGroups...)

	// Level 3: Compute tier
	// Auto Scaling Groups depend on: Launch Template + Subnets + (optional) Target Groups
	// ASGs will terminate all instances when deleted, must go before launch templates
	deletionOrder = append(deletionOrder, asgs...)

	// EC2 Instances have ENIs with security groups attached
	// Must delete before security groups can be removed
	deletionOrder = append(deletionOrder, instances...)

	// Launch Templates depend on: Security Groups (referenced, not blocking)
	// Safe to delete after ASGs and instances are terminated
	deletionOrder = append(deletionOrder, launchTemplates...)

	// Level 4: Database tier
	// RDS Instances depend on: DB Subnet Group + Security Groups
	// RDS instances have ENIs, must delete before security groups
	deletionOrder = append(deletionOrder, dbInstances...)

	// DB Subnet Groups depend on: Subnets (at least 2 in different AZs)
	// Must delete before subnets can be removed
	deletionOrder = append(deletionOrder, dbSubnetGroups...)

	// Level 5: Security layer
	// Security Groups depend on: VPC
	// Cannot be deleted while attached to ENIs (EC2, RDS, ALB, NAT Gateway)
	// May have circular references between security groups
	if len(securityGroups) > 0 {
		warnings = append(warnings,
			"Security groups may have circular references. Consider revoking all ingress/egress rules before deletion if errors occur.")
		deletionOrder = append(deletionOrder, securityGroups...)
	}

	// Level 6: Network routing layer (CRITICAL ORDER - based on AWS association constraints)
	// NAT Gateways depend on: Subnet + Elastic IP
	// NAT Gateways have ENIs in subnets, must delete before subnets
	deletionOrder = append(deletionOrder, natGateways...)

	// Subnets depend on: VPC
	// CRITICAL: Deleting a subnet automatically disassociates any route table associations
	// This is why subnets MUST be deleted BEFORE route tables
	// AWS enforces: cannot delete subnet while it has dependencies (EC2, RDS, NAT, etc.)
	deletionOrder = append(deletionOrder, subnets...)

	// Route Tables depend on: VPC
	// CRITICAL: Cannot delete route table while it has subnet associations
	// Since we delete subnets first, associations are auto-cleared, making this safe
	// Route tables may have routes pointing to IGW/NAT, but AWS allows deletion (routes become "blackhole")
	deletionOrder = append(deletionOrder, routeTables...)

	// Internet Gateways depend on: VPC (must be attached)
	// AWS requires detachment before deletion, but delete command handles this
	// Can have routes pointing to it, but that doesn't block deletion
	deletionOrder = append(deletionOrder, internetGateways...)

	// Level 7: Foundation layer
	// VPCs are the root container - must be empty before deletion
	// AWS will reject deletion if any resources still exist in the VPC
	deletionOrder = append(deletionOrder, vpcs...)

	return deletionOrder, warnings
}

// extractMinimalProperties extracts only essential properties needed for AWS CLI deletion
func (ws *WebServer) extractMinimalProperties(resource *types.ResourceState) map[string]interface{} {
	minimal := make(map[string]interface{})

	// Get unified data if available
	if unified, ok := resource.Properties["unified"].(map[string]interface{}); ok {
		// Resource ID/ARN
		if resourceId, ok := unified["resourceId"].(string); ok && resourceId != "" {
			minimal["resourceId"] = resourceId
		}
		if resourceArn, ok := unified["resourceArn"].(string); ok && resourceArn != "" {
			minimal["resourceArn"] = resourceArn
		}
		if resourceName, ok := unified["resourceName"].(string); ok && resourceName != "" {
			minimal["resourceName"] = resourceName
		}
	}

	// Get MCP response data if available
	if mcpResponse, ok := resource.Properties["mcp_response"].(map[string]interface{}); ok {
		// Common useful fields for deletion
		usefulFields := []string{
			"vpcId", "subnetId", "securityGroupId", "instanceId",
			"loadBalancerArn", "targetGroupArn", "listenerArn",
			"natGatewayId", "internetGatewayId", "routeTableId",
			"launchTemplateId", "launchTemplateName",
			"autoScalingGroupName", "dbSubnetGroupName",
			"name", "region",
		}

		for _, field := range usefulFields {
			if value, ok := mcpResponse[field]; ok && value != nil {
				// Skip empty strings
				if str, isStr := value.(string); isStr && str != "" {
					minimal[field] = value
				}
			}
		}
	}

	// Add region if available at top level
	if region, ok := resource.Properties["region"].(string); ok && region != "" {
		minimal["region"] = region
	}

	return minimal
}

// cleanResourcePropertiesHandler clears resource properties to {} in the state file
// This allows users to mark resources as deleted/cleaned without removing them from state
//
// POST /api/resources/empty
func (ws *WebServer) cleanResourcePropertiesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Check if config is available
	if ws.config == nil {
		http.Error(w, `{"error": "Configuration not available"}`, http.StatusServiceUnavailable)
		return
	}

	if ws.aiAgent != nil {
		ws.aiAgent.Logger.Info("Cleaning resource properties in state file")
	}

	// Get the state file path from config
	stateFilePath := ws.config.GetStateFilePath()
	if stateFilePath == "" {
		if ws.aiAgent != nil {
			ws.aiAgent.Logger.Error("State file path not configured")
		}
		http.Error(w, `{"error": "State file path not configured"}`, http.StatusInternalServerError)
		return
	}

	// Read the current state file
	data, err := os.ReadFile(stateFilePath)
	if err != nil {
		ws.aiAgent.Logger.WithError(err).Error("Failed to read state file")
		http.Error(w, `{"error": "Failed to read state file"}`, http.StatusInternalServerError)
		return
	}

	// Parse the state
	var state types.InfrastructureState
	if err := json.Unmarshal(data, &state); err != nil {
		ws.aiAgent.Logger.WithError(err).Error("Failed to parse state file")
		http.Error(w, `{"error": "Failed to parse state file"}`, http.StatusInternalServerError)
		return
	}

	// Clear all resources and dependencies
	totalResources := len(state.Resources)
	state.Resources = make(map[string]*types.ResourceState)
	state.Dependencies = make(map[string][]string)

	// Update last modified timestamp
	state.LastUpdated = time.Now()

	// Write back to state file
	cleanedData, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		if ws.aiAgent != nil {
			ws.aiAgent.Logger.WithError(err).Error("Failed to marshal cleaned state")
		}
		http.Error(w, `{"error": "Failed to marshal cleaned state"}`, http.StatusInternalServerError)
		return
	}

	// Write to temporary file first, then atomic rename
	tempFile := stateFilePath + ".tmp"
	if err := os.WriteFile(tempFile, cleanedData, 0644); err != nil {
		if ws.aiAgent != nil {
			ws.aiAgent.Logger.WithError(err).Error("Failed to write temporary state file")
		}
		http.Error(w, `{"error": "Failed to write temporary state file"}`, http.StatusInternalServerError)
		return
	}

	if err := os.Rename(tempFile, stateFilePath); err != nil {
		if ws.aiAgent != nil {
			ws.aiAgent.Logger.WithError(err).Error("Failed to rename temporary state file")
		}
		http.Error(w, `{"error": "Failed to rename temporary state file"}`, http.StatusInternalServerError)
		return
	}

	if ws.aiAgent != nil {
		ws.aiAgent.Logger.WithFields(map[string]interface{}{
			"total_resources": totalResources,
		}).Info("Successfully cleaned all resources")
	}

	// Return response
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Successfully cleared all resources and dependencies",
	})
}
