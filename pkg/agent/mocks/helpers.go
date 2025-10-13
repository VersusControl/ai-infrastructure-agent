package mocks

import (
	"github.com/versus-control/ai-infrastructure-agent/internal/config"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/agent/resources"
)

// MockTestSuite represents a complete mock test environment
type MockTestSuite struct {
	Logger           *logging.Logger
	MCPServer        *MockMCPServer
	StateManager     *MockStateManager
	AWSClient        *MockAWSClient
	ResourceMappings map[string]string            // Mock resource mappings for dependency resolution
	ValueInferrer    *resources.ValueTypeInferrer // Mock value type inferrer
	Registry         *MockRetrievalRegistry       // Mock retrieval registry
	agent            interface{}                  // Agent reference for syncing resource mappings
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

	// Initialize value type inferrer
	valueInferrer, err := resources.NewValueTypeInferrer(&config.ResourcePatternConfig{})
	if err != nil {
		logger.WithError(err).Warn("Failed to create value type inferrer, using nil")
		valueInferrer = nil
	}

	suite := &MockTestSuite{
		Logger:           logger,
		MCPServer:        mcpServer,
		StateManager:     stateManager,
		AWSClient:        awsClient,
		ResourceMappings: make(map[string]string),
		ValueInferrer:    valueInferrer,
		Registry:         NewMockRetrievalRegistry(awsClient), // Pass AWS client to registry
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

// // Reset resets all mock components to their default state
// func (suite *MockTestSuite) Reset() {
// 	suite.AWSClient.ResetToDefaults()
// 	suite.StateManager.ClearState()
// 	// Reinitialize state manager with real components and default resources
// 	suite.StateManager = NewMockStateManager()
// 	// suite.RetrievalFuncs.ClearMockResponses()
// }
