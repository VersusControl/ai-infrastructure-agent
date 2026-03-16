package plugins

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
)

type PluginRegistry struct {
	mu          sync.RWMutex
	handlers    map[string]ResourceHandler
	metadata    map[string]PluginMetadata
	hooks       map[string][]PluginHook
	middlewares map[string][]Middleware
	logger      *logging.Logger
	initialized bool
}

func NewPluginRegistry(logger *logging.Logger) *PluginRegistry {
	return &PluginRegistry{
		handlers:    make(map[string]ResourceHandler),
		metadata:    make(map[string]PluginMetadata),
		hooks:       make(map[string][]PluginHook),
		middlewares: make(map[string][]Middleware),
		logger:      logger,
		initialized: false,
	}
}

func (r *PluginRegistry) Register(plugin ResourceHandler, config *PluginConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	metadata := plugin.GetMetadata()
	resourceType := metadata.ResourceType

	if _, exists := r.handlers[resourceType]; exists {
		return fmt.Errorf("plugin already registered for resource type: %s", resourceType)
	}

	if err := plugin.Initialize(config); err != nil {
		return fmt.Errorf("failed to initialize plugin %s: %w", metadata.Name, err)
	}

	r.handlers[resourceType] = plugin
	r.metadata[resourceType] = metadata
	r.hooks[resourceType] = make([]PluginHook, 0)
	r.middlewares[resourceType] = make([]Middleware, 0)

	r.logger.WithFields(map[string]interface{}{
		"plugin_name":    metadata.Name,
		"resource_type":  resourceType,
		"version":        metadata.Version,
		"supported_ops":  metadata.SupportedOps,
	}).Info("Plugin registered successfully")

	return nil
}

func (r *PluginRegistry) Unregister(resourceType string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	plugin, exists := r.handlers[resourceType]
	if !exists {
		return fmt.Errorf("no plugin registered for resource type: %s", resourceType)
	}

	if err := plugin.Shutdown(); err != nil {
		r.logger.WithError(err).Warn("Plugin shutdown encountered error")
	}

	delete(r.handlers, resourceType)
	delete(r.metadata, resourceType)
	delete(r.hooks, resourceType)
	delete(r.middlewares, resourceType)

	r.logger.WithField("resource_type", resourceType).Info("Plugin unregistered")
	return nil
}

func (r *PluginRegistry) GetHandler(resourceType string) (ResourceHandler, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	handler, exists := r.handlers[resourceType]
	if !exists {
		return nil, fmt.Errorf("no handler for resource type: %s", resourceType)
	}
	return handler, nil
}

func (r *PluginRegistry) GetMetadata(resourceType string) (PluginMetadata, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	metadata, exists := r.metadata[resourceType]
	if !exists {
		return PluginMetadata{}, fmt.Errorf("no metadata for resource type: %s", resourceType)
	}
	return metadata, nil
}

func (r *PluginRegistry) ListPlugins() []PluginMetadata {
	r.mu.RLock()
	defer r.mu.RUnlock()

	plugins := make([]PluginMetadata, 0, len(r.metadata))
	for _, m := range r.metadata {
		plugins = append(plugins, m)
	}

	sort.Slice(plugins, func(i, j int) bool {
		return plugins[i].Priority < plugins[j].Priority
	})

	return plugins
}

func (r *PluginRegistry) AddHook(resourceType string, hook PluginHook) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.handlers[resourceType]; !exists {
		return fmt.Errorf("no plugin registered for resource type: %s", resourceType)
	}

	r.hooks[resourceType] = append(r.hooks[resourceType], hook)
	return nil
}

func (r *PluginRegistry) AddMiddleware(resourceType string, middleware Middleware) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.handlers[resourceType]; !exists {
		return fmt.Errorf("no plugin registered for resource type: %s", resourceType)
	}

	r.middlewares[resourceType] = append(r.middlewares[resourceType], middleware)
	return nil
}

func (r *PluginRegistry) ExecuteWithHooks(
	ctx context.Context,
	resourceType string,
	operation string,
	params interface{},
) (interface{}, error) {
	r.mu.RLock()
	handler, handlerExists := r.handlers[resourceType]
	hooks := r.hooks[resourceType]
	middlewares := r.middlewares[resourceType]
	r.mu.RUnlock()

	if !handlerExists {
		return nil, fmt.Errorf("no handler for resource type: %s", resourceType)
	}

	for _, hook := range hooks {
		var err error
		ctx, err = hook.BeforeExecute(ctx, operation, params)
		if err != nil {
			return nil, fmt.Errorf("before hook failed: %w", err)
		}
	}

	executeFunc := r.buildExecuteFunc(handler, operation)

	for i := len(middlewares) - 1; i >= 0; i-- {
		executeFunc = middlewares[i](executeFunc)
	}

	result, err := executeFunc(ctx, operation, params)

	for _, hook := range hooks {
		if hookErr := hook.AfterExecute(ctx, operation, params, result, err); hookErr != nil {
			r.logger.WithError(hookErr).Warn("After hook failed")
		}
	}

	if err != nil {
		for _, hook := range hooks {
			if hookErr := hook.OnFailure(ctx, operation, params, err); hookErr != nil {
				r.logger.WithError(hookErr).Warn("OnFailure hook failed")
			}
		}
	}

	return result, err
}

func (r *PluginRegistry) buildExecuteFunc(handler ResourceHandler, _ string) ExecuteFunc {
	return func(ctx context.Context, operation string, params interface{}) (*types.AWSResource, error) {
		switch operation {
		case "create":
			return handler.Create(ctx, params)
		case "list":
			resources, err := handler.List(ctx)
			if err != nil {
				return nil, err
			}
			if len(resources) > 0 {
				return resources[0], nil
			}
			return nil, nil
		case "get":
			if id, ok := params.(string); ok {
				return handler.Get(ctx, id)
			}
			return nil, fmt.Errorf("invalid params for get operation")
		case "update":
			if p, ok := params.(map[string]interface{}); ok {
				if id, exists := p["id"].(string); exists {
					return handler.Update(ctx, id, p)
				}
			}
			return nil, fmt.Errorf("invalid params for update operation")
		case "delete":
			if id, ok := params.(string); ok {
				return nil, handler.Delete(ctx, id)
			}
			return nil, fmt.Errorf("invalid params for delete operation")
		default:
			return handler.ExecuteSpecial(ctx, operation, params)
		}
	}
}

func (r *PluginRegistry) HealthCheck(ctx context.Context) map[string]error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	results := make(map[string]error)
	for resourceType, handler := range r.handlers {
		results[resourceType] = handler.HealthCheck(ctx)
	}
	return results
}

func (r *PluginRegistry) GetSupportedResourceTypes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]string, 0, len(r.handlers))
	for resourceType := range r.handlers {
		types = append(types, resourceType)
	}
	return types
}
