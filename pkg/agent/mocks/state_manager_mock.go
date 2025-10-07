package mocks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/versus-control/ai-infrastructure-agent/internal/config"
	"github.com/versus-control/ai-infrastructure-agent/pkg/agent/resources"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// MockStateManager implements a mock state manager for testing with real resource analysis capabilities
type MockStateManager struct {
	resources map[string]*types.ResourceState

	mutex sync.RWMutex

	patternMatcher *resources.PatternMatcher
	fieldResolver  *resources.FieldResolver

	extractionConfig *config.ResourceExtractionConfig
	idExtractor      *resources.IDExtractor
}

// NewMockStateManager creates a new mock state manager with real resource analysis capabilities
func NewMockStateManager() *MockStateManager {
	// Initialize config loader - navigate to project root
	dir, _ := os.Getwd()

	// Navigate to project root by looking for go.mod file
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root directory, use current dir
			break
		}
		dir = parent
	}

	configLoader := config.NewConfigLoader(filepath.Join(dir, "settings"))

	// Load resource patterns and field mappings from settings folder
	resourcePatternConfig, err := configLoader.LoadResourcePatterns()
	if err != nil {
		// Fall back to empty config if settings can't be loaded
		resourcePatternConfig = &config.ResourcePatternConfig{}
	}

	// Load extraction configuration
	extractionConfig, err := configLoader.LoadResourceExtraction()
	if err != nil {
		// Fall back to empty config if settings can't be loaded
		extractionConfig = &config.ResourceExtractionConfig{}
	}

	fieldMappingConfig, err := configLoader.LoadFieldMappings()
	if err != nil {
		// Fall back to empty config if settings can't be loaded
		fieldMappingConfig = &config.FieldMappingConfig{}
	}

	// Initialize real components
	patternMatcher, err := resources.NewPatternMatcher(resourcePatternConfig)
	if err != nil {
		// Fall back to empty pattern matcher
		patternMatcher, _ = resources.NewPatternMatcher(&config.ResourcePatternConfig{})
	}

	idExtractor, err := resources.NewIDExtractor(extractionConfig)
	if err != nil {
		// Fall back to empty ID extractor
		idExtractor, _ = resources.NewIDExtractor(&config.ResourceExtractionConfig{})
	}

	fieldResolver := resources.NewFieldResolver(fieldMappingConfig)

	manager := &MockStateManager{
		resources:        make(map[string]*types.ResourceState),
		patternMatcher:   patternMatcher,
		fieldResolver:    fieldResolver,
		extractionConfig: extractionConfig,
		idExtractor:      idExtractor,
	}

	// Initialize with some default resources
	manager.initializeDefaultResources()
	return manager
}

func (m *MockStateManager) initializeDefaultResources() {
	// Default VPC resource
	defaultVPC := &types.ResourceState{
		ID:     "vpc-default123",
		Type:   "vpc",
		Status: "active",
		Properties: map[string]interface{}{
			"name":      "default-vpc",
			"cidrBlock": "172.31.0.0/16",
			"isDefault": true,
			"state":     "available",
			"mcp_response": map[string]interface{}{
				"vpcId": "vpc-default123",
				"resource": map[string]interface{}{
					"id": "vpc-default123",
				},
			},
		},
	}
	m.resources["vpc-default123"] = defaultVPC

	// Production VPC resource
	prodVPC := &types.ResourceState{
		ID:     "vpc-prod123",
		Type:   "vpc",
		Status: "active",
		Properties: map[string]interface{}{
			"name":      "production-vpc",
			"cidrBlock": "10.0.0.0/16",
			"isDefault": false,
			"state":     "available",
			"mcp_response": map[string]interface{}{
				"vpcId": "vpc-prod123",
				"resource": map[string]interface{}{
					"id": "vpc-prod123",
				},
			},
		},
	}
	m.resources["vpc-prod123"] = prodVPC

	// Default subnet resource
	defaultSubnet := &types.ResourceState{
		ID:     "subnet-0def123abc456789",
		Type:   "subnet",
		Status: "active",
		Properties: map[string]interface{}{
			"name":             "default-subnet",
			"vpcId":            "vpc-default123",
			"cidrBlock":        "172.31.1.0/24",
			"availabilityZone": "us-west-2a",
			"state":            "available",
			"mcp_response": map[string]interface{}{
				"subnetId": "subnet-0def123abc456789",
				"resource": map[string]interface{}{
					"id": "subnet-0def123abc456789",
				},
			},
		},
	}
	m.resources["subnet-0def123abc456789"] = defaultSubnet
}

// ExportState exports the current state
func (m *MockStateManager) ExportState(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	includeManaged, _ := arguments["include_managed"].(bool)
	includeDiscovered, _ := arguments["include_discovered"].(bool)

	state := map[string]interface{}{}

	if includeManaged || (!includeManaged && !includeDiscovered) {
		// Convert resources to managed state format
		managedResources := make(map[string]interface{})
		for id, resource := range m.resources {
			managedResources[id] = map[string]interface{}{
				"id":         resource.ID,
				"type":       resource.Type,
				"name":       m.getResourceName(resource),
				"status":     resource.Status,
				"properties": resource.Properties,
			}
		}

		state["managed_state"] = map[string]interface{}{
			"resources": managedResources,
		}
	}

	if includeDiscovered {
		state["discovered_state"] = []interface{}{}
	}

	responseData, _ := json.Marshal(state)

	return &mcp.CallToolResult{
		IsError: false,
		Content: []mcp.Content{
			&mcp.TextContent{
				Type: "text",
				Text: string(responseData),
			},
		},
	}, nil
}

// Helper to get resource name from properties
func (m *MockStateManager) getResourceName(resource *types.ResourceState) string {
	if resource.Properties != nil {
		if name, exists := resource.Properties["name"]; exists {
			if nameStr, ok := name.(string); ok {
				return nameStr
			}
		}
	}
	return resource.ID
}

// AddResource adds a resource to the state
func (m *MockStateManager) AddResource(resource *types.ResourceState) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.resources[resource.ID] = resource
}

// GetResource gets a resource by ID
func (m *MockStateManager) GetResource(id string) *types.ResourceState {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.resources[id]
}

// GetResources gets all resources
func (m *MockStateManager) GetResources() map[string]*types.ResourceState {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	result := make(map[string]*types.ResourceState)
	for k, v := range m.resources {
		result[k] = v
	}
	return result
}

// UpdateResource updates an existing resource
func (m *MockStateManager) UpdateResource(resource *types.ResourceState) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if _, exists := m.resources[resource.ID]; !exists {
		return fmt.Errorf("resource %s not found", resource.ID)
	}

	m.resources[resource.ID] = resource
	return nil
}

// DeleteResource removes a resource from the state
func (m *MockStateManager) DeleteResource(id string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if _, exists := m.resources[id]; !exists {
		return fmt.Errorf("resource %s not found", id)
	}

	delete(m.resources, id)
	return nil
}

// GetResourcesByType gets all resources of a specific type
func (m *MockStateManager) GetResourcesByType(resourceType string) map[string]*types.ResourceState {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	result := make(map[string]*types.ResourceState)
	for id, resource := range m.resources {
		if resource.Type == resourceType {
			result[id] = resource
		}
	}
	return result
}

// SaveState saves the current state to a file (mock implementation)
func (m *MockStateManager) SaveState(filePath string) error {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// In a real implementation, this would save to a file
	// For mocking, we just simulate success
	if len(m.resources) == 0 {
		return fmt.Errorf("no resources to save")
	}

	return nil
}

// LoadState loads state from a file (mock implementation)
func (m *MockStateManager) LoadState(filePath string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// In a real implementation, this would load from a file
	// For mocking, we just simulate success and initialize default resources
	m.initializeDefaultResources()
	return nil
}

// GetResourceDependencies gets dependencies for a resource using real pattern matching from settings
func (m *MockStateManager) GetResourceDependencies(resourceId string) []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	resource, exists := m.resources[resourceId]
	if !exists {
		return []string{}
	}

	// Use real pattern matcher to determine resource type if not set
	resourceType := resource.Type
	if resourceType == "" {
		resourceType = m.patternMatcher.IdentifyResourceTypeFromID(resourceId)
	}

	dependencies := []string{}

	// Use field resolver to find dependency fields based on settings configuration
	if resource.Properties != nil {
		// Try to resolve VPC dependency for resources that need it
		if vpcId, found := m.fieldResolver.ResolveField(resourceType, "vpc_id", resource.Properties); found {
			if vpcIdStr, ok := vpcId.(string); ok && vpcIdStr != "" {
				dependencies = append(dependencies, vpcIdStr)
			}
		}

		// Try to resolve subnet dependencies
		if subnetIds, found := m.fieldResolver.ResolveField(resourceType, "subnet_ids", resource.Properties); found {
			switch v := subnetIds.(type) {
			case []string:
				dependencies = append(dependencies, v...)
			case []interface{}:
				for _, subnetId := range v {
					if subnetIdStr, ok := subnetId.(string); ok && subnetIdStr != "" {
						dependencies = append(dependencies, subnetIdStr)
					}
				}
			case string:
				if v != "" {
					dependencies = append(dependencies, v)
				}
			}
		}

		// Try to resolve subnet dependency (singular)
		if subnetId, found := m.fieldResolver.ResolveField(resourceType, "subnet_id", resource.Properties); found {
			if subnetIdStr, ok := subnetId.(string); ok && subnetIdStr != "" {
				dependencies = append(dependencies, subnetIdStr)
			}
		}

		// Try to resolve security group dependencies
		if sgIds, found := m.fieldResolver.ResolveField(resourceType, "security_group_ids", resource.Properties); found {
			switch v := sgIds.(type) {
			case []string:
				dependencies = append(dependencies, v...)
			case []interface{}:
				for _, sgId := range v {
					if sgIdStr, ok := sgId.(string); ok && sgIdStr != "" {
						dependencies = append(dependencies, sgIdStr)
					}
				}
			case string:
				if v != "" {
					dependencies = append(dependencies, v)
				}
			}
		}

		// Try to resolve launch template dependency
		if ltId, found := m.fieldResolver.ResolveField(resourceType, "launch_template_id", resource.Properties); found {
			if ltIdStr, ok := ltId.(string); ok && ltIdStr != "" {
				dependencies = append(dependencies, ltIdStr)
			}
		}

		// Try to resolve load balancer ARN dependency
		if lbArn, found := m.fieldResolver.ResolveField(resourceType, "load_balancer_arn", resource.Properties); found {
			if lbArnStr, ok := lbArn.(string); ok && lbArnStr != "" {
				dependencies = append(dependencies, lbArnStr)
			}
		}

		// Try to resolve target group ARN dependency
		if tgArn, found := m.fieldResolver.ResolveField(resourceType, "target_group_arn", resource.Properties); found {
			if tgArnStr, ok := tgArn.(string); ok && tgArnStr != "" {
				dependencies = append(dependencies, tgArnStr)
			}
		}
	}

	// Remove duplicates and self-references
	seen := make(map[string]bool)
	var uniqueDependencies []string
	for _, dep := range dependencies {
		if !seen[dep] && dep != resourceId {
			seen[dep] = true
			uniqueDependencies = append(uniqueDependencies, dep)
		}
	}

	return uniqueDependencies
}

// GetResourceStatus gets the status of a resource
func (m *MockStateManager) GetResourceStatus(resourceId string) string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if resource, exists := m.resources[resourceId]; exists {
		return resource.Status
	}
	return "not_found"
}

// SetResourceStatus sets the status of a resource
func (m *MockStateManager) SetResourceStatus(resourceId, status string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	resource, exists := m.resources[resourceId]
	if !exists {
		return fmt.Errorf("resource %s not found", resourceId)
	}

	resource.Status = status
	return nil
}

// GetResourceCount gets the total number of resources
func (m *MockStateManager) GetResourceCount() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return len(m.resources)
}

// GetResourceCountByType gets the count of resources by type
func (m *MockStateManager) GetResourceCountByType() map[string]int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	counts := make(map[string]int)
	for _, resource := range m.resources {
		counts[resource.Type]++
	}
	return counts
}

// ClearState removes all resources from the state
func (m *MockStateManager) ClearState() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.resources = make(map[string]*types.ResourceState)
}

// SimulateResourceCreation simulates the creation of a new resource
func (m *MockStateManager) SimulateResourceCreation(resourceType, resourceId, name string, properties map[string]interface{}) *types.ResourceState {
	resource := &types.ResourceState{
		ID:         resourceId,
		Type:       resourceType,
		Status:     "creating",
		Properties: properties,
	}

	if properties == nil {
		resource.Properties = make(map[string]interface{})
	}

	if name != "" {
		resource.Properties["name"] = name
	}

	m.AddResource(resource)
	return resource
}

// ==== Real Resource Analysis Methods ====

// AnalyzeResourceType uses real pattern matching to determine resource type
func (m *MockStateManager) AnalyzeResourceType(resourceId string, resourceName string, toolName string) string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Try ID patterns first
	if resourceType := m.patternMatcher.IdentifyResourceTypeFromID(resourceId); resourceType != "" {
		return resourceType
	}

	// Try tool name patterns
	if toolName != "" {
		if resourceType := m.patternMatcher.IdentifyResourceTypeFromToolName(toolName); resourceType != "" {
			return resourceType
		}
	}

	// Try name patterns (if we had a method for this)
	// For now, return empty string if no patterns match
	return ""
}

// InferValueTypeFromDescription is deprecated - value type inference is no longer supported
func (m *MockStateManager) InferValueTypeFromDescription(description, fieldName string) string {
	// Value type inference is no longer supported - return empty string
	return ""
}

// ResolveResourceField uses real field resolution from settings
func (m *MockStateManager) ResolveResourceField(resourceType, fieldName string, data map[string]interface{}) (interface{}, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return m.fieldResolver.ResolveField(resourceType, fieldName, data)
}

// AnalyzeResourceFromMCPResponse analyzes a resource from MCP tool response using real components
func (m *MockStateManager) AnalyzeResourceFromMCPResponse(toolName string, response map[string]interface{}) *ResourceAnalysis {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	analysis := &ResourceAnalysis{
		ToolName: toolName,
		Response: response,
	}

	// Extract resource information using field resolver
	if resourceData, exists := response["resource"].(map[string]interface{}); exists {
		if id, exists := resourceData["id"].(string); exists {
			analysis.ResourceID = id
			analysis.ResourceType = m.patternMatcher.IdentifyResourceTypeFromID(id)
		}

		if name, exists := resourceData["name"].(string); exists {
			analysis.ResourceName = name
		}

		if resourceType, exists := resourceData["type"].(string); exists && analysis.ResourceType == "" {
			analysis.ResourceType = resourceType
		}
	}

	// If we couldn't find resource info in nested structure, try top level
	if analysis.ResourceID == "" {
		for _, possibleIDField := range []string{"id", "resourceId", "arn"} {
			if id, exists := response[possibleIDField].(string); exists {
				analysis.ResourceID = id
				if analysis.ResourceType == "" {
					analysis.ResourceType = m.patternMatcher.IdentifyResourceTypeFromID(id)
				}
				break
			}
		}
	}

	// Use tool name to infer resource type if still unknown
	if analysis.ResourceType == "" {
		analysis.ResourceType = m.patternMatcher.IdentifyResourceTypeFromToolName(toolName)
	}

	// Extract dependencies using field resolver
	if analysis.ResourceType != "" {
		analysis.Dependencies = m.extractDependenciesFromResponse(analysis.ResourceType, response)
	}

	return analysis
}

// ResourceAnalysis represents the result of analyzing a resource
type ResourceAnalysis struct {
	ToolName     string
	Response     map[string]interface{}
	ResourceID   string
	ResourceType string
	ResourceName string
	Dependencies []string
}

func (m *MockStateManager) extractDependenciesFromResponse(resourceType string, response map[string]interface{}) []string {
	var dependencies []string

	// Use field resolver to find dependency fields
	dependencyFields := []string{"vpcId", "subnetId", "subnetIds", "securityGroupIds", "launchTemplateId", "loadBalancerArn", "targetGroupArn"}

	for _, field := range dependencyFields {
		if value, found := m.fieldResolver.ResolveField(resourceType, field, response); found {
			switch v := value.(type) {
			case string:
				if v != "" {
					dependencies = append(dependencies, v)
				}
			case []string:
				dependencies = append(dependencies, v...)
			case []interface{}:
				for _, item := range v {
					if str, ok := item.(string); ok && str != "" {
						dependencies = append(dependencies, str)
					}
				}
			}
		}
	}

	return dependencies
}

// GetPatternMatcher returns the pattern matcher for external use
func (m *MockStateManager) GetPatternMatcher() *resources.PatternMatcher {
	return m.patternMatcher
}

// GetFieldResolver returns the field resolver for external use
func (m *MockStateManager) GetFieldResolver() *resources.FieldResolver {
	return m.fieldResolver
}

// GetIDExtractor returns the ID extractor for external use
func (m *MockStateManager) GetIDExtractor() *resources.IDExtractor {
	return m.idExtractor
}

func (m *MockStateManager) GetExtractionConfig() *config.ResourceExtractionConfig {
	return m.extractionConfig
}
