package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/interfaces"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// ResourceDefinition defines the configuration for a single AWS resource type
type ResourceDefinition struct {
	// Basic resource information
	BaseURI     string // e.g., "aws://ec2/instances"
	Name        string // e.g., "EC2 Instances"
	Description string // e.g., "List all EC2 instances in the region"

	// Template configuration for individual resources
	DetailTemplate    string // e.g., "aws://ec2/instances/{instanceId}"
	DetailName        string // e.g., "EC2 Instance Details"
	DetailDescription string // e.g., "Detailed information about a specific EC2 instance"

	// Adapter reference
	Adapter interfaces.AWSResourceAdapter

	// Custom formatting functions (optional)
	ListFormatter   func([]*types.AWSResource) map[string]interface{}
	DetailFormatter func(types.AWSResource) map[string]interface{}
}

// ResourceRegistry manages the registration and handling of all AWS resources
type ResourceRegistry struct {
	logger      *logging.Logger
	definitions []ResourceDefinition
}

// registerResourcesModern uses the new registry-based approach to register resources
func (s *Server) registerResources() {
	s.Logger.Info("Registering resources using modern registry-based approach")

	// Create resource registry
	registry := NewResourceRegistry(s.Logger)

	// Get all resource definitions
	definitions := CreateResourceDefinitions(s.AWSClient, s.Logger)

	// Add all definitions to the registry
	for _, def := range definitions {
		registry.AddResourceDefinition(def)
	}

	// Register all resources with the MCP server
	registry.RegisterAllResources(s.mcpServer)

	s.Logger.WithField("resourceCount", len(definitions)).Info("Successfully registered all resources")
}

// NewResourceRegistry creates a new resource registry
func NewResourceRegistry(logger *logging.Logger) *ResourceRegistry {
	return &ResourceRegistry{
		logger:      logger,
		definitions: make([]ResourceDefinition, 0),
	}
}

// AddResourceDefinition adds a new resource definition to the registry
func (r *ResourceRegistry) AddResourceDefinition(def ResourceDefinition) {
	r.definitions = append(r.definitions, def)
}

// RegisterAllResources registers all defined resources with the MCP server
func (r *ResourceRegistry) RegisterAllResources(mcpServer *server.MCPServer) {
	r.logger.WithField("resourceCount", len(r.definitions)).Info("Registering resources with MCP server")

	for _, def := range r.definitions {
		// Register list resource
		r.registerListResource(mcpServer, def)

		// Register detail resource template
		r.registerDetailResource(mcpServer, def)
	}
}

// registerListResource registers a resource for listing all items of a type
func (r *ResourceRegistry) registerListResource(mcpServer *server.MCPServer, def ResourceDefinition) {
	resource := mcp.NewResource(
		def.BaseURI,
		def.Name,
		mcp.WithResourceDescription(def.Description),
		mcp.WithMIMEType("application/json"),
	)

	handler := func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		r.logger.WithField("uri", def.BaseURI).Info("Received request for resource list")

		result, err := r.handleListResource(ctx, def)
		if err != nil {
			r.logger.WithError(err).WithField("uri", def.BaseURI).Error("Failed to read resource list")
			return nil, err
		}

		return result.Contents, nil
	}

	mcpServer.AddResource(resource, handler)
}

// registerDetailResource registers a resource template for individual resource details
func (r *ResourceRegistry) registerDetailResource(mcpServer *server.MCPServer, def ResourceDefinition) {
	template := mcp.NewResourceTemplate(
		def.DetailTemplate,
		def.DetailName,
		mcp.WithTemplateDescription(def.DetailDescription),
		mcp.WithTemplateMIMEType("application/json"),
	)

	handler := func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		r.logger.WithField("uri", request.Params.URI).Info("Received read resource request for specific resource")

		result, err := r.handleDetailResource(ctx, def, request.Params.URI)
		if err != nil {
			r.logger.WithError(err).WithField("uri", request.Params.URI).Error("Failed to read resource detail")
			return nil, err
		}

		return result.Contents, nil
	}

	mcpServer.AddResourceTemplate(template, handler)
}

// handleListResource handles requests for resource lists using the adapter pattern
func (r *ResourceRegistry) handleListResource(ctx context.Context, def ResourceDefinition) (*mcp.ReadResourceResult, error) {
	resources, err := def.Adapter.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list resources: %w", err)
	}

	// Use custom formatter if provided, otherwise use default
	var formatted map[string]interface{}
	if def.ListFormatter != nil {
		formatted = def.ListFormatter(resources)
	} else {
		formatted = r.defaultListFormatter(resources)
	}

	return r.marshalResourceResponse(formatted, def.BaseURI)
}

// handleDetailResource handles requests for individual resource details
func (r *ResourceRegistry) handleDetailResource(ctx context.Context, def ResourceDefinition, uri string) (*mcp.ReadResourceResult, error) {
	// Extract resource ID from URI
	resourceID := r.extractResourceID(uri, def.DetailTemplate)
	if resourceID == "" {
		return nil, fmt.Errorf("failed to extract resource ID from URI: %s", uri)
	}

	resource, err := def.Adapter.Get(ctx, resourceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get resource: %w", err)
	}

	// Use custom formatter if provided, otherwise use default
	var formatted map[string]interface{}
	if def.DetailFormatter != nil {
		formatted = def.DetailFormatter(*resource)
	} else {
		formatted = r.defaultDetailFormatter(*resource)
	}

	return r.marshalResourceResponse(formatted, uri)
}

// extractResourceID extracts the resource ID from a URI using the template
func (r *ResourceRegistry) extractResourceID(uri, template string) string {
	// Simple extraction - find the part after the last /
	parts := strings.Split(uri, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

// marshalResourceResponse marshals data into an MCP resource response
func (r *ResourceRegistry) marshalResourceResponse(data interface{}, uri string) (*mcp.ReadResourceResult, error) {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal resource data: %w", err)
	}

	return &mcp.ReadResourceResult{
		Contents: []mcp.ResourceContents{
			&mcp.TextResourceContents{
				URI:      uri,
				MIMEType: "application/json",
				Text:     string(jsonData),
			},
		},
	}, nil
}

// defaultListFormatter provides a standard format for resource lists
func (r *ResourceRegistry) defaultListFormatter(resources []*types.AWSResource) map[string]interface{} {
	summary := map[string]interface{}{
		"total_resources":  len(resources),
		"resources":        make([]map[string]interface{}, 0, len(resources)),
		"summary_by_state": make(map[string]int),
		"summary_by_type":  make(map[string]int),
	}

	stateCount := make(map[string]int)
	typeCount := make(map[string]int)

	for _, resource := range resources {
		formatted := map[string]interface{}{
			"id":     resource.ID,
			"type":   resource.Type,
			"state":  resource.State,
			"region": resource.Region,
			"tags":   resource.Tags,
		}

		// Add name if available from tags
		if name, exists := resource.Tags["Name"]; exists {
			formatted["name"] = name
		}

		summary["resources"] = append(summary["resources"].([]map[string]interface{}), formatted)

		// Update counters
		stateCount[resource.State]++
		typeCount[resource.Type]++
	}

	summary["summary_by_state"] = stateCount
	summary["summary_by_type"] = typeCount

	return summary
}

// defaultDetailFormatter provides a standard format for individual resources
func (r *ResourceRegistry) defaultDetailFormatter(resource types.AWSResource) map[string]interface{} {
	formatted := map[string]interface{}{
		"id":        resource.ID,
		"type":      resource.Type,
		"state":     resource.State,
		"region":    resource.Region,
		"tags":      resource.Tags,
		"details":   resource.Details,
		"last_seen": resource.LastSeen.Format("2006-01-02T15:04:05Z07:00"),
	}

	// Add computed fields that AI systems find useful
	if name, exists := resource.Tags["Name"]; exists {
		formatted["name"] = name
	} else {
		formatted["name"] = resource.ID
	}

	return formatted
}
