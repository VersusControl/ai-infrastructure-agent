package resources

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/versus-control/ai-infrastructure-agent/internal/config"
)

// IDExtractor handles resource ID extraction using configuration
type IDExtractor struct {
	creationPatterns     []config.ExtractionPattern
	modificationPatterns []config.ExtractionPattern
	associationPatterns  []config.ExtractionPattern
	deletionPatterns     []config.ExtractionPattern
	queryPatterns        []config.ExtractionPattern
	toolActionTypes      map[string]string // toolName -> actionType mapping
	mu                   sync.RWMutex
}

// NewIDExtractor creates a new ID extractor from configuration
func NewIDExtractor(cfg *config.ResourceExtractionConfig) (*IDExtractor, error) {
	extractor := &IDExtractor{
		creationPatterns:     cfg.ResourceIDExtraction.CreationTools.Patterns,
		modificationPatterns: cfg.ResourceIDExtraction.ModificationTools.Patterns,
		associationPatterns:  cfg.ResourceIDExtraction.AssociationTools.Patterns,
		deletionPatterns:     cfg.ResourceIDExtraction.DeletionTools.Patterns,
		queryPatterns:        cfg.ResourceIDExtraction.QueryTools.Patterns,
		toolActionTypes:      make(map[string]string),
	}

	return extractor, nil
}

// RegisterToolActionType registers a tool's action type
func (e *IDExtractor) RegisterToolActionType(toolName, actionType string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.toolActionTypes[toolName] = actionType
}

// ClassifyTool determines the action type of a tool using registered metadata
func (e *IDExtractor) ClassifyTool(toolName string) string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Look up the registered action type
	if actionType, exists := e.toolActionTypes[toolName]; exists {
		return actionType
	}

	return "unknown"
}

// ExtractResourceID extracts resource ID from tool parameters and results
func (e *IDExtractor) ExtractResourceID(toolName string, resourceType string, parameters, result map[string]interface{}) (string, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	actionType := e.ClassifyTool(toolName)

	var patterns []config.ExtractionPattern
	var dataSource map[string]interface{}

	switch actionType {
	case "creation":
		patterns = e.creationPatterns
		dataSource = result // Created resources return IDs in result
	case "modification":
		patterns = e.modificationPatterns
		dataSource = result // Modified resources now return IDs in result (like creation tools)
	case "association":
		patterns = e.associationPatterns
		dataSource = result // Association operations return new association IDs in result
	case "deletion":
		patterns = e.deletionPatterns
		dataSource = parameters // Deletion operations specify IDs in parameters
	case "query", "state":
		patterns = e.queryPatterns
		dataSource = mergeData(parameters, result) // Could be in either
	default:
		return "", fmt.Errorf("no patterns matched for unknown action type on tool %s (resource type: %s)", toolName, resourceType)
	}

	// Try patterns for this resource type and action
	for _, pattern := range patterns {
		if e.matchesResourceType(pattern.ResourceTypes, resourceType) {
			for _, fieldPath := range pattern.FieldPaths {
				if value := e.extractFromPath(dataSource, fieldPath); value != "" {
					return value, nil
				}
			}
		}
	}

	// If no pattern matched, return error with helpful context
	return "", fmt.Errorf("no extraction pattern matched for tool %s (resource type: %s, action: %s)", toolName, resourceType, actionType)
}

// matchesResourceType checks if a resource type matches the pattern's resource types
func (e *IDExtractor) matchesResourceType(patternTypes []string, resourceType string) bool {
	for _, patternType := range patternTypes {
		if patternType == "*" || patternType == resourceType {
			return true
		}
	}
	return false
}

// extractFromPath extracts a value from nested data using dot notation
// Arrays are concatenated with "_" delimiter (unified array handling)
func (e *IDExtractor) extractFromPath(data map[string]interface{}, path string) string {
	if data == nil {
		return ""
	}

	parts := strings.Split(path, ".")
	current := data

	for i, part := range parts {
		// Handle array access like "resources[0]"
		if strings.Contains(part, "[") {
			arrayName, index, err := parseArrayAccessForIDExtractor(part)
			if err != nil {
				return ""
			}

			if array, ok := current[arrayName].([]interface{}); ok && len(array) > index {
				if i == len(parts)-1 {
					// Last part of path - return the array element
					return fmt.Sprintf("%v", array[index])
				}
				// Continue navigation
				if nextLevel, ok := array[index].(map[string]interface{}); ok {
					current = nextLevel
					continue
				}
			}
			return ""
		}

		// Regular field access
		if i == len(parts)-1 {
			// Last part - return the value
			if value, ok := current[part]; ok {
				// Check if the value is an array
				if arrayValue, isArray := value.([]interface{}); isArray {
					// With unified array handling, arrays should be pre-processed to concatenated strings
					// If still an array (not yet processed), concatenate with "_" delimiter
					var strValues []string
					for _, item := range arrayValue {
						if item != nil {
							strValues = append(strValues, fmt.Sprintf("%v", item))
						}
					}
					return strings.Join(strValues, "_")
				}
				// Return string value (already concatenated for pre-processed arrays)
				return fmt.Sprintf("%v", value)
			}
		} else {
			// Intermediate part - navigate deeper
			if nextLevel, ok := current[part].(map[string]interface{}); ok {
				current = nextLevel
			} else {
				return ""
			}
		}
	}

	return ""
}

// parseArrayAccessForIDExtractor parses array access syntax like "resources[0]"
func parseArrayAccessForIDExtractor(part string) (string, int, error) {
	leftBracket := strings.Index(part, "[")
	rightBracket := strings.Index(part, "]")

	if leftBracket == -1 || rightBracket == -1 || rightBracket <= leftBracket {
		return "", 0, fmt.Errorf("invalid array access syntax: %s", part)
	}

	arrayName := part[:leftBracket]
	indexStr := part[leftBracket+1 : rightBracket]

	// Handle wildcard or special array access
	if indexStr == "*" {
		return arrayName, 0, nil // Return first element for wildcard
	}

	index, err := strconv.Atoi(indexStr)
	if err != nil {
		return "", 0, fmt.Errorf("invalid array index: %s", indexStr)
	}

	return arrayName, index, nil
}

// mergeData merges parameters and result maps (used by query tools)
func mergeData(parameters, result map[string]interface{}) map[string]interface{} {
	merged := make(map[string]interface{})

	// Copy parameters first
	for k, v := range parameters {
		merged[k] = v
	}

	// Add result (overwrites parameters if same key)
	for k, v := range result {
		merged[k] = v
	}

	return merged
}

// ExtractAllResourceIDs extracts all possible resource IDs from tool data
func (e *IDExtractor) ExtractAllResourceIDs(toolName string, parameters, result map[string]interface{}) map[string]string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	ids := make(map[string]string)

	actionType := e.ClassifyTool(toolName)

	var patterns []config.ExtractionPattern
	var dataSource map[string]interface{}

	switch actionType {
	case "creation":
		patterns = e.creationPatterns
		dataSource = result
	case "modification", "deletion":
		patterns = e.modificationPatterns
		dataSource = parameters
	case "query", "state":
		patterns = e.queryPatterns
		dataSource = mergeData(parameters, result)
	default:
		dataSource = mergeData(parameters, result)
	}

	// Extract IDs for all resource types
	for _, pattern := range patterns {
		for _, resourceType := range pattern.ResourceTypes {
			if resourceType == "*" {
				continue // Skip wildcard for this function
			}

			for _, fieldPath := range pattern.FieldPaths {
				if value := e.extractFromPath(dataSource, fieldPath); value != "" {
					ids[resourceType] = value
					break // Take first match for this resource type
				}
			}
		}
	}

	return ids
}
