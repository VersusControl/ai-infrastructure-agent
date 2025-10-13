package retrieval

import (
	"context"
	"fmt"
	"regexp"
	"sync"

	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// RetrievalFunction defines the signature for all retrieval functions
type RetrievalFunction func(context.Context, *types.ExecutionPlanStep) (map[string]interface{}, error)

// RetrievalRegistryInterface defines the interface that both real and mock registries implement
type RetrievalRegistryInterface interface {
	RegisterRetrieval(valueType string, fn RetrievalFunction)
	RegisterPattern(pattern string, fn RetrievalFunction) error
	Execute(ctx context.Context, valueType string, planStep *types.ExecutionPlanStep) (map[string]interface{}, error)
	GetRegisteredTypes() []string
	GetRegisteredPatterns() []string
}

// RetrievalRegistry manages all retrieval functions and patterns
type RetrievalRegistry struct {
	functions map[string]RetrievalFunction
	patterns  map[string]*PatternEntry
	mu        sync.RWMutex
}

// PatternEntry contains a compiled regex pattern and its associated function
type PatternEntry struct {
	Pattern  *regexp.Regexp
	Function RetrievalFunction
}

var globalRegistry *RetrievalRegistry
var once sync.Once

// GetGlobalRegistry returns the singleton global registry
func GetGlobalRegistry() *RetrievalRegistry {
	once.Do(func() {
		globalRegistry = &RetrievalRegistry{
			functions: make(map[string]RetrievalFunction),
			patterns:  make(map[string]*PatternEntry),
		}

		// Register built-in pattern-based functions
		registerPatternFunctions()
	})
	return globalRegistry
}

// RegisterRetrieval registers a retrieval function for exact value type matching
func (r *RetrievalRegistry) RegisterRetrieval(valueType string, fn RetrievalFunction) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.functions[valueType] = fn
}

// RegisterPattern registers a retrieval function for pattern-based matching
func (r *RetrievalRegistry) RegisterPattern(pattern string, fn RetrievalFunction) error {
	compiledPattern, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid pattern %s: %w", pattern, err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.patterns[pattern] = &PatternEntry{
		Pattern:  compiledPattern,
		Function: fn,
	}

	return nil
}

// Execute executes the appropriate retrieval function for the given value type
func (r *RetrievalRegistry) Execute(ctx context.Context, valueType string, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Try exact match first
	if fn, exists := r.functions[valueType]; exists {
		return fn(ctx, planStep)
	}

	// Try pattern matching
	for _, entry := range r.patterns {
		if entry.Pattern.MatchString(valueType) {
			return entry.Function(ctx, planStep)
		}
	}

	return nil, fmt.Errorf("unsupported value_type: %s", valueType)
}

// GetRegisteredTypes returns all registered exact match types
func (r *RetrievalRegistry) GetRegisteredTypes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]string, 0, len(r.functions))
	for valueType := range r.functions {
		types = append(types, valueType)
	}
	return types
}

// GetRegisteredPatterns returns all registered patterns
func (r *RetrievalRegistry) GetRegisteredPatterns() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	patterns := make([]string, 0, len(r.patterns))
	for pattern := range r.patterns {
		patterns = append(patterns, pattern)
	}
	return patterns
}

// registerPatternFunctions registers pattern-based retrieval functions
func registerPatternFunctions() {
	// Get registry instance without calling GetGlobalRegistry to avoid circular dependency
	if globalRegistry == nil {
		globalRegistry = &RetrievalRegistry{
			functions: make(map[string]RetrievalFunction),
			patterns:  make(map[string]*PatternEntry),
		}
	}

	// Pattern-based registrations for generic cases
	globalRegistry.RegisterPattern(`.*_id$`, retrieveResourceID)
	globalRegistry.RegisterPattern(`.*_name$`, retrieveResourceName)
}

func retrieveResourceID(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	// Generic ID retrieval logic
	return map[string]interface{}{
		"id": planStep.ResourceID,
	}, nil
}

func retrieveResourceName(ctx context.Context, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	// Generic name retrieval logic
	return map[string]interface{}{
		"name": planStep.Name,
	}, nil
}

// Public convenience functions for external registration
func RegisterRetrieval(valueType string, fn RetrievalFunction) {
	GetGlobalRegistry().RegisterRetrieval(valueType, fn)
}

func RegisterPattern(pattern string, fn RetrievalFunction) error {
	return GetGlobalRegistry().RegisterPattern(pattern, fn)
}

func ExecuteRetrieval(ctx context.Context, valueType string, planStep *types.ExecutionPlanStep) (map[string]interface{}, error) {
	return GetGlobalRegistry().Execute(ctx, valueType, planStep)
}
