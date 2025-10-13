package resources

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/versus-control/ai-infrastructure-agent/internal/config"
)

// FieldResolver handles dynamic field resolution using configuration
type FieldResolver struct {
	mappings        map[string]map[string][]string
	transformations *config.FieldTransformations
	patternMatcher  *PatternMatcher // Add reference to pattern matcher
	mu              sync.RWMutex
}

// NewFieldResolver creates a new field resolver from configuration
func NewFieldResolver(cfg *config.FieldMappingConfig) *FieldResolver {
	return &FieldResolver{
		mappings:        cfg.ResourceFields,
		transformations: &cfg.FieldTransformations,
		patternMatcher:  nil, // Will be set via SetPatternMatcher
	}
}

// SetPatternMatcher sets the pattern matcher for resource type detection
func (f *FieldResolver) SetPatternMatcher(pm *PatternMatcher) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.patternMatcher = pm
}

// ResolveField attempts to resolve a field value from the provided data
// It tries resource-specific mappings only
func (f *FieldResolver) ResolveField(resourceType, fieldName string, data map[string]interface{}) (interface{}, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Try resource-specific mappings
	if resourceMappings, exists := f.mappings[resourceType]; exists {
		if fieldList, exists := resourceMappings[fieldName]; exists {
			for _, field := range fieldList {
				if value, exists := data[field]; exists {
					return f.transformValue(fieldName, value), true
				}
			}
		}
	}

	return nil, false
}

// ResolveAllFields resolves all configured fields for a resource type
func (f *FieldResolver) ResolveAllFields(resourceType string, data map[string]interface{}) map[string]interface{} {
	f.mu.RLock()
	defer f.mu.RUnlock()

	result := make(map[string]interface{})

	// Get all field names for this resource type
	if resourceMappings, exists := f.mappings[resourceType]; exists {
		for fieldName := range resourceMappings {
			if value, found := f.ResolveField(resourceType, fieldName, data); found {
				result[fieldName] = value
			}
		}
	}

	return result
}

// transformValue applies configured transformations to field values
func (f *FieldResolver) transformValue(fieldName string, value interface{}) interface{} {
	// Handle boolean field transformations
	if f.isBooleanField(fieldName) {
		return f.transformToBoolean(value)
	}

	// Handle array field transformations
	if f.isArrayField(fieldName) {
		return f.transformToArray(value)
	}

	// Handle state transformations
	if fieldName == "state" || fieldName == "status" {
		return f.transformState(value)
	}

	return value
}

// isBooleanField checks if a field should be treated as boolean
func (f *FieldResolver) isBooleanField(fieldName string) bool {
	for _, boolField := range f.transformations.BooleanFields {
		if strings.EqualFold(fieldName, boolField) {
			return true
		}
	}
	return false
}

// isArrayField checks if a field should be treated as an array
func (f *FieldResolver) isArrayField(fieldName string) bool {
	for _, arrayField := range f.transformations.ArrayFields {
		if strings.EqualFold(fieldName, arrayField) {
			return true
		}
	}
	return false
}

// transformToBoolean converts various representations to boolean
func (f *FieldResolver) transformToBoolean(value interface{}) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		switch strings.ToLower(v) {
		case "true", "enabled", "yes", "1", "on":
			return true
		case "false", "disabled", "no", "0", "off":
			return false
		}
	case int, int64:
		return reflect.ValueOf(v).Int() != 0
	case float64:
		return v != 0.0
	}
	return false
}

// transformToArray ensures the value is returned as an array
func (f *FieldResolver) transformToArray(value interface{}) []interface{} {
	if value == nil {
		return []interface{}{}
	}

	// If already an array, return as-is
	if arr, ok := value.([]interface{}); ok {
		return arr
	}

	// If it's a slice of any type, convert to []interface{}
	val := reflect.ValueOf(value)
	if val.Kind() == reflect.Slice {
		result := make([]interface{}, val.Len())
		for i := 0; i < val.Len(); i++ {
			result[i] = val.Index(i).Interface()
		}
		return result
	}

	// Single value - wrap in array
	return []interface{}{value}
}

// transformState normalizes state values to standard representations
func (f *FieldResolver) transformState(value interface{}) string {
	if value == nil {
		return "unknown"
	}

	stateStr := strings.ToLower(fmt.Sprintf("%v", value))

	// Check configured state mappings
	for standardState, aliases := range f.transformations.State {
		for _, alias := range aliases {
			if strings.EqualFold(stateStr, alias) {
				return standardState
			}
		}
	}

	// Return original if no mapping found
	return stateStr
}

// ExtractFromPath extracts a value from nested data using dot notation
// Supports array access like "result.resources[0].resourceId"
func (f *FieldResolver) ExtractFromPath(data map[string]interface{}, path string) interface{} {
	parts := strings.Split(path, ".")
	current := data

	for i, part := range parts {
		// Handle array access like "resources[0]"
		if strings.Contains(part, "[") {
			arrayName, index, err := parseArrayAccess(part)
			if err != nil {
				return nil
			}

			if array, ok := current[arrayName].([]interface{}); ok && len(array) > index {
				if i == len(parts)-1 {
					return array[index]
				}
				if nextLevel, ok := array[index].(map[string]interface{}); ok {
					current = nextLevel
					continue
				}
			}
			return nil
		}

		// Regular field access
		if i == len(parts)-1 {
			// Last part - return the value
			return current[part]
		} else {
			// Intermediate part - navigate deeper
			if nextLevel, ok := current[part].(map[string]interface{}); ok {
				current = nextLevel
			} else {
				return nil
			}
		}
	}

	return nil
}

// parseArrayAccess parses array access syntax like "resources[0]"
func parseArrayAccess(part string) (string, int, error) {
	leftBracket := strings.Index(part, "[")
	rightBracket := strings.Index(part, "]")

	if leftBracket == -1 || rightBracket == -1 || rightBracket <= leftBracket {
		return "", 0, fmt.Errorf("invalid array access syntax: %s", part)
	}

	arrayName := part[:leftBracket]
	indexStr := part[leftBracket+1 : rightBracket]

	index, err := strconv.Atoi(indexStr)
	if err != nil {
		return "", 0, fmt.Errorf("invalid array index: %s", indexStr)
	}

	return arrayName, index, nil
}

// GetSupportedFields returns all supported field names for a resource type
func (f *FieldResolver) GetSupportedFields(resourceType string) []string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	var fields []string

	if resourceMappings, exists := f.mappings[resourceType]; exists {
		for fieldName := range resourceMappings {
			fields = append(fields, fieldName)
		}
	}

	return fields
}

// GetResourceTypes returns all configured resource types
func (f *FieldResolver) GetResourceTypes() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	var types []string
	for resourceType := range f.mappings {
		types = append(types, resourceType)
	}

	return types
}

// DetectResourceType attempts to detect the resource type from MCP response data
func (f *FieldResolver) DetectResourceType(data map[string]interface{}) string {
	if f.patternMatcher == nil {
		return ""
	}

	// Check if there's a nested resource object with type information
	if resource, ok := data["resource"].(map[string]interface{}); ok {
		// Try to detect from resource.id
		if resourceID, ok := resource["id"].(string); ok {
			return f.patternMatcher.IdentifyResourceTypeFromID(resourceID)
		}
	}

	// Check if there's a nested result object with type information
	if resource, ok := data["result"].(map[string]interface{}); ok {
		// Try to detect from resource.id
		if resourceID, ok := resource["id"].(string); ok {
			return f.patternMatcher.IdentifyResourceTypeFromID(resourceID)
		}
	}

	return ""
}

// GetFieldsForRequestWithContext returns prioritized fields based on detected resource type
func (f *FieldResolver) GetFieldsForRequestWithContext(requestedField string, data map[string]interface{}) []string {
	// Try to detect the resource type from the data
	if detectedType := f.DetectResourceType(data); detectedType != "" {
		// Use the specific resource type's field mapping
		if resourceFields := f.GetResourceFieldMapping(detectedType, requestedField); len(resourceFields) > 0 {
			return resourceFields
		}
	}

	return f.GetFieldsForRequest(requestedField)
}

// GetResourceFieldMapping returns the field mapping for a specific resource type and field
func (f *FieldResolver) GetResourceFieldMapping(resourceType, fieldName string) []string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if resourceMappings, exists := f.mappings[resourceType]; exists {
		// First try direct lookup
		if fields, exists := resourceMappings[fieldName]; exists {
			return fields
		}

		// If not found, search through all field mappings to find which canonical field
		// the requested fieldName might be an alias for
		for _, aliases := range resourceMappings {
			for _, alias := range aliases {
				if alias == fieldName {
					// Found it! Return the full alias list for this canonical field
					return aliases
				}
			}
		}
	}

	return nil
}

// GetFieldsForRequest returns the prioritized field list for a specific field request
func (f *FieldResolver) GetFieldsForRequest(requestedField string) []string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Search through all resource types to find a mapping for this field
	for _, resourceMappings := range f.mappings {
		// First try direct lookup by canonical field name
		if fields, exists := resourceMappings[requestedField]; exists {
			return fields
		}

		// If not found, search through aliases
		for _, aliases := range resourceMappings {
			for _, alias := range aliases {
				if alias == requestedField {
					// Found it! Return the full alias list
					return aliases
				}
			}
		}
	}

	return nil
}
