package resources

import (
	"regexp"
	"strings"
	"sync"

	"github.com/versus-control/ai-infrastructure-agent/internal/config"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// PatternMatcher handles resource type identification using patterns
type PatternMatcher struct {
	idPatterns             map[string][]*regexp.Regexp
	namePatterns           map[string][]*regexp.Regexp
	descPatterns           map[string][]*regexp.Regexp
	toolPatterns           map[string][]*regexp.Regexp
	relationships          *config.ResourceRelationships
	resourceIdentification *config.ResourceIdentification
	toolCategories         map[string][]string // category -> resource types
	resourceToCategory     map[string]string   // resource type -> category
	mu                     sync.RWMutex
}

// NewPatternMatcher creates a new pattern matcher from configuration
func NewPatternMatcher(cfg *config.ResourcePatternConfig) (*PatternMatcher, error) {
	pm := &PatternMatcher{
		idPatterns:             make(map[string][]*regexp.Regexp),
		namePatterns:           make(map[string][]*regexp.Regexp),
		descPatterns:           make(map[string][]*regexp.Regexp),
		toolPatterns:           make(map[string][]*regexp.Regexp),
		relationships:          &cfg.ResourceRelationships,
		resourceIdentification: &cfg.ResourceIdentification,
		toolCategories:         cfg.ToolCategories,
		resourceToCategory:     make(map[string]string),
	}

	// Build reverse mapping: resource type -> category
	for category, resourceTypes := range cfg.ToolCategories {
		for _, resourceType := range resourceTypes {
			pm.resourceToCategory[resourceType] = category
		}
	}

	// Compile ID patterns
	for resourceType, patterns := range cfg.ResourceIdentification.IDPatterns {
		for _, pattern := range patterns {
			compiledPattern, err := regexp.Compile(pattern)
			if err != nil {
				return nil, err
			}
			pm.idPatterns[resourceType] = append(pm.idPatterns[resourceType], compiledPattern)
		}
	}

	// Compile name patterns
	for resourceType, patterns := range cfg.ResourceIdentification.NamePatterns {
		for _, pattern := range patterns {
			compiledPattern, err := regexp.Compile("(?i)" + pattern) // case-insensitive
			if err != nil {
				return nil, err
			}
			pm.namePatterns[resourceType] = append(pm.namePatterns[resourceType], compiledPattern)
		}
	}

	// Compile description patterns
	for resourceType, patterns := range cfg.ResourceIdentification.DescriptionPatterns {
		for _, pattern := range patterns {
			compiledPattern, err := regexp.Compile("(?i)" + pattern) // case-insensitive
			if err != nil {
				return nil, err
			}
			pm.descPatterns[resourceType] = append(pm.descPatterns[resourceType], compiledPattern)
		}
	}

	// Compile tool patterns
	for resourceType, patterns := range cfg.ToolResourcePatterns {
		for _, pattern := range patterns {
			compiledPattern, err := regexp.Compile("(?i)" + pattern) // case-insensitive
			if err != nil {
				return nil, err
			}
			pm.toolPatterns[resourceType] = append(pm.toolPatterns[resourceType], compiledPattern)
		}
	}

	return pm, nil
}

// IdentifyResourceType identifies the resource type from execution plan step
func (p *PatternMatcher) IdentifyResourceType(planStep *types.ExecutionPlanStep) string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Try ID patterns first (most reliable)
	if planStep.ResourceID != "" {
		if resourceType := p.matchIDPatterns(planStep.ResourceID); resourceType != "" {
			return resourceType
		}
	}

	// Try name patterns
	if planStep.Name != "" {
		if resourceType := p.matchNamePatterns(planStep.Name); resourceType != "" {
			return resourceType
		}
	}

	// Try description patterns
	if planStep.Description != "" {
		if resourceType := p.matchDescriptionPatterns(planStep.Description); resourceType != "" {
			return resourceType
		}
	}

	// Try tool name patterns
	if planStep.MCPTool != "" {
		if resourceType := p.matchToolPatterns(planStep.MCPTool); resourceType != "" {
			return resourceType
		}
	}

	return ""
}

// IdentifyResourceTypeFromID identifies resource type from just the resource ID
func (p *PatternMatcher) IdentifyResourceTypeFromID(resourceID string) string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.matchIDPatterns(resourceID)
}

// IdentifyResourceTypeFromToolName identifies resource type from tool name
func (p *PatternMatcher) IdentifyResourceTypeFromToolName(toolName string) string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.matchToolPatterns(toolName)
}

// GetCategoryForTool returns the category for a given tool name using resource type matching
func (p *PatternMatcher) GetCategoryForTool(toolName string) string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// First identify the resource type from the tool name
	resourceType := p.matchToolPatterns(toolName)
	if resourceType == "" {
		return "Other"
	}

	// Then get the category for that resource type
	if category, exists := p.resourceToCategory[resourceType]; exists {
		return category
	}

	return "Other"
}

// GetAvailableCategories returns all available categories
func (p *PatternMatcher) GetAvailableCategories() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	categories := make([]string, 0, len(p.toolCategories))
	for category := range p.toolCategories {
		categories = append(categories, category)
	}
	return categories
}

// matchIDPatterns tries to match resource ID patterns
func (p *PatternMatcher) matchIDPatterns(resourceID string) string {
	for resourceType, patterns := range p.idPatterns {
		for _, pattern := range patterns {
			if pattern.MatchString(resourceID) {
				return resourceType
			}
		}
	}
	return ""
}

// matchNamePatterns tries to match resource name patterns
func (p *PatternMatcher) matchNamePatterns(name string) string {
	for resourceType, patterns := range p.namePatterns {
		for _, pattern := range patterns {
			if pattern.MatchString(name) {
				return resourceType
			}
		}
	}
	return ""
}

// matchDescriptionPatterns tries to match resource description patterns
func (p *PatternMatcher) matchDescriptionPatterns(description string) string {
	for resourceType, patterns := range p.descPatterns {
		for _, pattern := range patterns {
			if pattern.MatchString(description) {
				return resourceType
			}
		}
	}
	return ""
}

// matchToolPatterns tries to match tool name patterns
func (p *PatternMatcher) matchToolPatterns(toolName string) string {
	for resourceType, patterns := range p.toolPatterns {
		for _, pattern := range patterns {
			if pattern.MatchString(toolName) {
				return resourceType
			}
		}
	}
	return ""
}

// GetResourceChildren returns child resource types for a given resource type
func (p *PatternMatcher) GetResourceChildren(resourceType string) []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	switch resourceType {
	case "vpc":
		return p.relationships.VPC
	case "subnet":
		return p.relationships.Subnet
	case "security_group":
		return p.relationships.SecurityGroup
	case "load_balancer":
		return p.relationships.LoadBalancer
	case "auto_scaling_group":
		return p.relationships.AutoScalingGroup
	case "launch_template":
		return p.relationships.LaunchTemplate
	default:
		return []string{}
	}
}

// GetResourceDependencies returns required dependencies for a resource type
func (p *PatternMatcher) GetResourceDependencies(resourceType string) []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if deps, exists := p.relationships.Dependencies[resourceType]; exists {
		return deps
	}
	return []string{}
}

// IsValidResourceType checks if a resource type is known/configured
func (p *PatternMatcher) IsValidResourceType(resourceType string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Check if we have patterns for this resource type
	if _, exists := p.idPatterns[resourceType]; exists {
		return true
	}
	if _, exists := p.namePatterns[resourceType]; exists {
		return true
	}
	if _, exists := p.toolPatterns[resourceType]; exists {
		return true
	}

	return false
}

// GetSupportedResourceTypes returns all configured resource types
func (p *PatternMatcher) GetSupportedResourceTypes() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	resourceTypes := make(map[string]bool)

	// Collect from all pattern maps
	for resourceType := range p.idPatterns {
		resourceTypes[resourceType] = true
	}
	for resourceType := range p.namePatterns {
		resourceTypes[resourceType] = true
	}
	for resourceType := range p.descPatterns {
		resourceTypes[resourceType] = true
	}
	for resourceType := range p.toolPatterns {
		resourceTypes[resourceType] = true
	}

	// Convert to slice
	result := make([]string, 0, len(resourceTypes))
	for resourceType := range resourceTypes {
		result = append(result, resourceType)
	}

	return result
}

// InferResourceTypeFromDescription attempts to infer resource type from text description
// This is a smarter version that considers context and common patterns
func (p *PatternMatcher) InferResourceTypeFromDescription(description string) string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	description = strings.ToLower(description)

	// Score-based matching - some patterns are more specific than others
	scores := make(map[string]int)

	for resourceType, patterns := range p.descPatterns {
		for _, pattern := range patterns {
			if pattern.MatchString(description) {
				// Weight matches based on pattern specificity
				patternStr := pattern.String()
				if strings.Contains(patternStr, "create") || strings.Contains(patternStr, "provision") {
					scores[resourceType] += 3 // High priority for creation patterns
				} else if len(patternStr) > 20 {
					scores[resourceType] += 2 // Longer patterns are more specific
				} else {
					scores[resourceType] += 1 // Basic match
				}
			}
		}
	}

	// Return the highest scoring resource type
	maxScore := 0
	bestMatch := ""
	for resourceType, score := range scores {
		if score > maxScore {
			maxScore = score
			bestMatch = resourceType
		}
	}

	return bestMatch
}

// GetPatternsByResourceType returns all patterns for a specific resource type
func (p *PatternMatcher) GetPatternsByResourceType(resourceType string) map[string][]string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make(map[string][]string)

	if patterns, exists := p.idPatterns[resourceType]; exists {
		for _, pattern := range patterns {
			result["id"] = append(result["id"], pattern.String())
		}
	}

	if patterns, exists := p.namePatterns[resourceType]; exists {
		for _, pattern := range patterns {
			result["name"] = append(result["name"], pattern.String())
		}
	}

	if patterns, exists := p.descPatterns[resourceType]; exists {
		for _, pattern := range patterns {
			result["description"] = append(result["description"], pattern.String())
		}
	}

	if patterns, exists := p.toolPatterns[resourceType]; exists {
		for _, pattern := range patterns {
			result["tool"] = append(result["tool"], pattern.String())
		}
	}

	return result
}
