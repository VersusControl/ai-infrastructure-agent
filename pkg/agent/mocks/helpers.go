package mocks

import (
	"strings"

	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// MockTestSuite represents a complete mock test environment
type MockTestSuite struct {
	Logger           *logging.Logger
	MCPServer        *MockMCPServer
	StateManager     *MockStateManager
	AWSClient        *MockAWSClient
	ResourceMappings map[string]string // Mock resource mappings for dependency resolution
	agent            interface{}       // Agent reference for syncing resource mappings
}

// NewMockTestSuite creates a complete mock test environment
func NewMockTestSuite(region string) (*MockTestSuite, error) {
	logger := logging.NewLogger("test", "debug")

	// Create all mock components
	awsClient := NewMockAWSClient(region, logger)
	mcpServer := NewMockMCPServerWithAWSClient(logger, awsClient)
	stateManager := NewMockStateManager()

	// Initialize with default test data
	awsClient.AddDefaultTestData()

	suite := &MockTestSuite{
		Logger:           logger,
		MCPServer:        mcpServer,
		StateManager:     stateManager,
		AWSClient:        awsClient,
		ResourceMappings: make(map[string]string),
	}

	return suite, nil
}

// StoreResourceMapping stores a mapping between step ID and resource ID
func (suite *MockTestSuite) StoreResourceMapping(stepID, resourceID string) {
	suite.ResourceMappings[stepID] = resourceID

	// Also store in agent's resource mappings if agent is available
	if suite.agent != nil {
		// Use type assertion to access the agent's StoreResourceMapping method
		if agent, ok := suite.agent.(interface{ StoreResourceMapping(string, string) }); ok {
			agent.StoreResourceMapping(stepID, resourceID)
		}
	}

	suite.Logger.WithFields(map[string]interface{}{
		"step_id":         stepID,
		"resource_id":     resourceID,
		"synced_to_agent": suite.agent != nil,
	}).Debug("Mock: Stored resource mapping")
}

// SetAgent stores the agent reference for syncing resource mappings
func (suite *MockTestSuite) SetAgent(agent interface{}) {
	suite.agent = agent
}

// LoadExistingState loads existing infrastructure state into the mock state manager
// and populates resource mappings for dependency resolution in test mode
func (suite *MockTestSuite) LoadExistingState(state map[string]*types.ResourceState, agent interface{}) error {
	suite.agent = agent

	suite.Logger.WithField("resource_count", len(state)).Info("Loading existing state into mock infrastructure")

	// Load each resource into the state manager
	for resourceID, resource := range state {
		suite.StateManager.AddResource(resource)

		// Extract resource ID for mapping - handle both step_reference and other types
		// Step IDs that start with "step-" need to be mapped to their actual resource IDs
		if strings.HasPrefix(resourceID, "step-") {
			// This is a step reference - extract the actual resource ID
			var actualResourceID string

			// Check if there's an mcp_response with resource information
			if props, ok := resource.Properties["mcp_response"].(map[string]interface{}); ok {
				fieldResolver := suite.StateManager.GetFieldResolver()
				if fieldResolver != nil {
					// Detect the actual resource type from the MCP response
					detectedType := fieldResolver.DetectResourceType(props)

					suite.Logger.WithFields(map[string]interface{}{
						"step_id":       resourceID,
						"resource_type": resource.Type,
						"detected_type": detectedType,
					}).Debug("Processing step resource for mapping")

					// For step_reference and other step types, try to extract the primary resource ID
					if detectedType != "" {
						// Get the field mapping for "id" field of this resource type
						idFields := fieldResolver.GetResourceFieldMapping(detectedType, "id")

						// Try each field name returned by the resolver
						for _, field := range idFields {
							if value, exists := props[field]; exists {
								if strValue, ok := value.(string); ok && strValue != "" {
									actualResourceID = strValue
									suite.Logger.WithFields(map[string]interface{}{
										"step_id":       resourceID,
										"resource_type": detectedType,
										"field_found":   field,
										"value":         strValue,
									}).Info("Found resource ID using field resolver")
									break
								}
							}
						}
					}

					// If no ID found with detected type, try direct field access for common fields
					if actualResourceID == "" {
						// Try common ID fields directly (for query/discovery results)
						directFields := []string{
							"subnetId", "vpcId", "instanceId", "securityGroupId",
							"value", "resourceId", // Generic fields
						}

						for _, field := range directFields {
							if value, exists := props[field]; exists {
								if strValue, ok := value.(string); ok && strValue != "" {
									actualResourceID = strValue
									suite.Logger.WithFields(map[string]interface{}{
										"step_id":     resourceID,
										"field_found": field,
										"value":       strValue,
									}).Info("Found resource ID using direct field access")
									break
								}
							}
						}
					}
				}
			}

			// Store the mapping if we found a resource ID
			if actualResourceID != "" {
				suite.StoreResourceMapping(resourceID, actualResourceID)
				suite.Logger.WithFields(map[string]interface{}{
					"step_id":     resourceID,
					"resource_id": actualResourceID,
				}).Info("Created resource mapping")
			} else {
				suite.Logger.WithFields(map[string]interface{}{
					"step_id": resourceID,
					"type":    resource.Type,
				}).Debug("No resource ID found for step - may be a query with array results")
			}
		}
	}

	suite.Logger.WithFields(map[string]interface{}{
		"loaded_resources": len(state),
		"mappings_created": len(suite.ResourceMappings),
	}).Info("Successfully loaded existing state into mock infrastructure")

	return nil
}

// // Reset resets all mock components to their default state
// func (suite *MockTestSuite) Reset() {
// 	suite.AWSClient.ResetToDefaults()
// 	suite.StateManager.ClearState()
// 	// Reinitialize state manager with real components and default resources
// 	suite.StateManager = NewMockStateManager()
// 	// suite.RetrievalFuncs.ClearMockResponses()
// }
