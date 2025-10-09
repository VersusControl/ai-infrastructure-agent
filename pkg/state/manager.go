package state

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// Manager handles infrastructure state management
type Manager struct {
	stateFile string
	logger    *logging.Logger
	state     *types.InfrastructureState
}

// NewManager creates a new state manager
func NewManager(stateFile string, region string, logger *logging.Logger) *Manager {
	return &Manager{
		stateFile: stateFile,
		logger:    logger,
		state: &types.InfrastructureState{
			Version:      "1.0",
			LastUpdated:  time.Now(),
			Region:       region,
			Resources:    make(map[string]*types.ResourceState),
			Dependencies: make(map[string][]string),
			Metadata:     make(map[string]interface{}),
		},
	}
}

// LoadState loads infrastructure state from file
func (m *Manager) LoadState(ctx context.Context) error {
	m.logger.WithField("state_file", m.stateFile).Info("Loading infrastructure state from file")

	// Create state directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(m.stateFile), 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	// Check if state file exists
	if _, err := os.Stat(m.stateFile); os.IsNotExist(err) {
		m.logger.Info("State file does not exist, initializing new state")
		return m.SaveState(ctx)
	}

	// Read state file
	data, err := os.ReadFile(m.stateFile)
	if err != nil {
		return fmt.Errorf("failed to read state file: %w", err)
	}

	m.logger.WithField("file_size", len(data)).Info("Read state file data")

	// Create a new state object to ensure clean loading
	newState := &types.InfrastructureState{}

	// Parse state
	if err := json.Unmarshal(data, newState); err != nil {
		return fmt.Errorf("failed to parse state file: %w", err)
	}

	// Replace the current state with the newly loaded state
	m.state = newState

	return nil
}

// SaveState saves infrastructure state to file
func (m *Manager) SaveState(ctx context.Context) error {
	m.logger.WithField("state_file", m.stateFile).Debug("Saving infrastructure state")

	m.state.LastUpdated = time.Now()

	// Marshal state to JSON
	data, err := json.MarshalIndent(m.state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	// Write to temporary file first
	tempFile := m.stateFile + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temporary state file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempFile, m.stateFile); err != nil {
		return fmt.Errorf("failed to rename temporary state file: %w", err)
	}

	m.logger.Debug("Infrastructure state saved")
	return nil
}

// GetState returns the current infrastructure state
func (m *Manager) GetState() *types.InfrastructureState {
	return m.state
}

// AddResource adds a resource to the state
func (m *Manager) AddResource(ctx context.Context, resource *types.ResourceState) error {
	m.logger.WithFields(map[string]interface{}{
		"resource_id":   resource.ID,
		"resource_type": resource.Type,
		"resource_name": resource.Name,
	}).Info("Adding resource to state")

	// Calculate checksum
	resource.Checksum = m.calculateChecksum(resource)
	resource.CreatedAt = time.Now()
	resource.UpdatedAt = time.Now()

	m.state.Resources[resource.ID] = resource

	return m.SaveState(ctx)
}

// UpdateResource updates a resource in the state
func (m *Manager) UpdateResource(ctx context.Context, resourceID string, updates map[string]interface{}) error {
	resource, exists := m.state.Resources[resourceID]
	if !exists {
		return fmt.Errorf("resource %s not found in state", resourceID)
	}

	m.logger.WithField("resource_id", resourceID).Info("Updating resource in state")

	// Apply updates
	for key, value := range updates {
		resource.Properties[key] = value
	}

	// Update metadata
	resource.UpdatedAt = time.Now()
	resource.Checksum = m.calculateChecksum(resource)

	return m.SaveState(ctx)
}

// RemoveResource removes a resource from the state
func (m *Manager) RemoveResource(ctx context.Context, resourceID string) error {
	if _, exists := m.state.Resources[resourceID]; !exists {
		return fmt.Errorf("resource %s not found in state", resourceID)
	}

	m.logger.WithField("resource_id", resourceID).Info("Removing resource from state")

	delete(m.state.Resources, resourceID)

	// Remove from dependencies
	delete(m.state.Dependencies, resourceID)
	for id, deps := range m.state.Dependencies {
		for i, dep := range deps {
			if dep == resourceID {
				m.state.Dependencies[id] = append(deps[:i], deps[i+1:]...)
				break
			}
		}
	}

	return m.SaveState(ctx)
}

// GetResource returns a resource from the state
func (m *Manager) GetResource(resourceID string) (*types.ResourceState, bool) {
	resource, exists := m.state.Resources[resourceID]
	return resource, exists
}

// ListResources returns all resources of a specific type
func (m *Manager) ListResources(resourceType string) []*types.ResourceState {
	var resources []*types.ResourceState
	for _, resource := range m.state.Resources {
		if resourceType == "" || resource.Type == resourceType {
			resources = append(resources, resource)
		}
	}
	return resources
}

// AddDependency adds a dependency relationship between resources
func (m *Manager) AddDependency(ctx context.Context, resourceID, dependsOn string) error {
	m.logger.WithFields(map[string]interface{}{
		"resource_id": resourceID,
		"depends_on":  dependsOn,
	}).Info("Adding dependency relationship")

	if m.state.Dependencies[resourceID] == nil {
		m.state.Dependencies[resourceID] = []string{}
	}

	// Check if dependency already exists
	for _, dep := range m.state.Dependencies[resourceID] {
		if dep == dependsOn {
			return nil // Dependency already exists
		}
	}

	m.state.Dependencies[resourceID] = append(m.state.Dependencies[resourceID], dependsOn)

	return m.SaveState(ctx)
}

// GetDependencies returns all dependencies for a resource
func (m *Manager) GetDependencies(resourceID string) []string {
	return m.state.Dependencies[resourceID]
}

// GetDependents returns all resources that depend on the given resource
func (m *Manager) GetDependents(resourceID string) []string {
	var dependents []string
	for id, deps := range m.state.Dependencies {
		for _, dep := range deps {
			if dep == resourceID {
				dependents = append(dependents, id)
				break
			}
		}
	}
	return dependents
}

// calculateChecksum calculates a checksum for resource state
func (m *Manager) calculateChecksum(resource *types.ResourceState) string {
	// Create a copy without timestamp fields for checksum calculation
	temp := &types.ResourceState{
		ID:           resource.ID,
		Name:         resource.Name,
		Type:         resource.Type,
		Status:       resource.Status,
		DesiredState: resource.DesiredState,
		CurrentState: resource.CurrentState,
		Tags:         resource.Tags,
		Properties:   resource.Properties,
		Dependencies: resource.Dependencies,
	}

	data, _ := json.Marshal(temp)
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)
}

// DetectDrift compares actual resource state with desired state
func (m *Manager) DetectDrift(ctx context.Context, actualState map[string]interface{}, resourceID string) (*types.ChangeDetection, error) {
	resource, exists := m.state.Resources[resourceID]
	if !exists {
		return nil, fmt.Errorf("resource %s not found in state", resourceID)
	}

	// Calculate checksum of actual state
	tempResource := &types.ResourceState{
		Properties: actualState,
	}
	actualChecksum := m.calculateChecksum(tempResource)

	if actualChecksum != resource.Checksum {
		m.logger.WithField("resource_id", resourceID).Info("Drift detected in resource")

		return &types.ChangeDetection{
			Resource:   resourceID,
			ChangeType: "drift",
			OldState:   resource.Properties,
			NewState:   actualState,
			Reason:     "Resource state has drifted from desired configuration",
			Timestamp:  time.Now(),
		}, nil
	}

	return nil, nil
}
