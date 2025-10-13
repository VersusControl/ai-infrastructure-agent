package resources

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/versus-control/ai-infrastructure-agent/internal/config"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// ValueTypeInferrer handles inference of value_type from step descriptions and names
type ValueTypeInferrer struct {
	patterns map[string][]*ValueTypePattern
	mu       sync.RWMutex
}

// ValueTypePattern represents a pattern for inferring value types
type ValueTypePattern struct {
	ValueType     string
	DescPatterns  []*regexp.Regexp
	NamePatterns  []*regexp.Regexp
	RequiredTerms []string // All must be present
	OptionalTerms []string // At least one must be present
}

// NewValueTypeInferrer creates a new value type inferrer from configuration
func NewValueTypeInferrer(cfg *config.ResourcePatternConfig) (*ValueTypeInferrer, error) {
	inferrer := &ValueTypeInferrer{
		patterns: make(map[string][]*ValueTypePattern),
	}

	// Initialize patterns from configuration
	err := inferrer.initializePatternsFromConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize value type patterns: %w", err)
	}

	return inferrer, nil
}

// initializePatternsFromConfig loads patterns from the YAML configuration
func (vti *ValueTypeInferrer) initializePatternsFromConfig(cfg *config.ResourcePatternConfig) error {
	// Load value type inference patterns from configuration
	for valueType, patternConfig := range cfg.ValueTypeInference {
		pattern := &ValueTypePattern{
			ValueType:     valueType,
			RequiredTerms: patternConfig.RequiredTerms,
			OptionalTerms: patternConfig.OptionalTerms,
		}

		// Compile description patterns
		for _, patternStr := range patternConfig.DescriptionPatterns {
			compiledPattern, err := regexp.Compile("(?i)" + patternStr) // case-insensitive
			if err != nil {
				return fmt.Errorf("failed to compile description pattern '%s' for value type '%s': %w", patternStr, valueType, err)
			}
			pattern.DescPatterns = append(pattern.DescPatterns, compiledPattern)
		}

		// Compile name patterns
		for _, patternStr := range patternConfig.NamePatterns {
			compiledPattern, err := regexp.Compile("(?i)" + patternStr) // case-insensitive
			if err != nil {
				return fmt.Errorf("failed to compile name pattern '%s' for value type '%s': %w", patternStr, valueType, err)
			}
			pattern.NamePatterns = append(pattern.NamePatterns, compiledPattern)
		}

		vti.patterns[valueType] = append(vti.patterns[valueType], pattern)
	}

	return nil
}

// InferValueType attempts to infer the value_type from a plan step's description and name
func (vti *ValueTypeInferrer) InferValueType(planStep *types.ExecutionPlanStep) (string, error) {
	vti.mu.RLock()
	defer vti.mu.RUnlock()

	description := strings.ToLower(planStep.Description)
	name := strings.ToLower(planStep.Name)

	// First, try specific patterns (all except "unknown")
	for valueType, patterns := range vti.patterns {
		if valueType == "unknown" {
			continue // Skip unknown pattern for now
		}
		for _, pattern := range patterns {
			if vti.matchesPattern(description, name, pattern) {
				return valueType, nil
			}
		}
	}

	// If no specific pattern matches, try the unknown pattern as fallback
	if unknownPatterns, exists := vti.patterns["unknown"]; exists {
		for _, pattern := range unknownPatterns {
			if vti.matchesPattern(description, name, pattern) {
				return "unknown", nil
			}
		}
	}

	return "", fmt.Errorf("unable to infer value_type from description: '%s' and name: '%s'", planStep.Description, planStep.Name)
}

// matchesPattern checks if the description and name match a specific pattern
func (vti *ValueTypeInferrer) matchesPattern(description, name string, pattern *ValueTypePattern) bool {
	// Check if description matches any description pattern
	descMatches := len(pattern.DescPatterns) == 0 // Default to true if no patterns
	for _, regex := range pattern.DescPatterns {
		if regex.MatchString(description) {
			descMatches = true
			break
		}
	}

	// Check if name matches any name pattern
	nameMatches := len(pattern.NamePatterns) == 0 // Default to true if no patterns
	for _, regex := range pattern.NamePatterns {
		if regex.MatchString(name) {
			nameMatches = true
			break
		}
	}

	// Check required terms (all must be present in description or name)
	requiredMatch := true
	for _, term := range pattern.RequiredTerms {
		termLower := strings.ToLower(term)
		if !strings.Contains(description, termLower) && !strings.Contains(name, termLower) {
			requiredMatch = false
			break
		}
	}

	// Check optional terms (at least one must be present)
	optionalMatch := len(pattern.OptionalTerms) == 0 // Default to true if no optional terms
	for _, term := range pattern.OptionalTerms {
		termLower := strings.ToLower(term)
		if strings.Contains(description, termLower) || strings.Contains(name, termLower) {
			optionalMatch = true
			break
		}
	}

	// All conditions must be true
	return descMatches && nameMatches && requiredMatch && optionalMatch
}

// GetSupportedValueTypes returns a list of all supported value types
func (vti *ValueTypeInferrer) GetSupportedValueTypes() []string {
	vti.mu.RLock()
	defer vti.mu.RUnlock()

	var types []string
	for valueType := range vti.patterns {
		types = append(types, valueType)
	}
	return types
}
