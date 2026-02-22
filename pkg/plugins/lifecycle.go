package plugins

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"sync"
	"time"

	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

type PluginState string

const (
	StateUninitialized PluginState = "uninitialized"
	StateInitializing   PluginState = "initializing"
	StateReady          PluginState = "ready"
	StateRunning        PluginState = "running"
	StateError          PluginState = "error"
	StateShuttingDown   PluginState = "shutting_down"
	StateStopped        PluginState = "stopped"
)

type PluginStatus struct {
	Name       string
	State      PluginState
	LastHealth time.Time
	Error      error
	Metadata   PluginMetadata
}

type LifecycleManager struct {
	registry   *PluginRegistry
	logger     *logging.Logger
	statuses   map[string]*PluginStatus
	mu         sync.RWMutex
	stopCh     chan struct{}
	healthTick time.Duration
}

func NewLifecycleManager(registry *PluginRegistry, logger *logging.Logger) *LifecycleManager {
	return &LifecycleManager{
		registry:   registry,
		logger:     logger,
		statuses:   make(map[string]*PluginStatus),
		stopCh:     make(chan struct{}),
		healthTick: 30 * time.Second,
	}
}

func (lm *LifecycleManager) InitializeAll(configs map[string]*PluginConfig) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	plugins := lm.registry.ListPlugins()
	order := lm.resolveInitializationOrder(plugins)

	for _, metadata := range order {
		resourceType := metadata.ResourceType
		lm.statuses[resourceType] = &PluginStatus{
			Name:  metadata.Name,
			State: StateInitializing,
		}

		config := configs[resourceType]
		if config == nil {
			config = &PluginConfig{Config: make(map[string]interface{})}
		}

		if err := lm.registry.Register(nil, config); err != nil {
			lm.statuses[resourceType].State = StateError
			lm.statuses[resourceType].Error = err
			lm.logger.WithError(err).WithField("plugin", metadata.Name).Error("Failed to initialize plugin")
			continue
		}

		lm.statuses[resourceType].State = StateReady
		lm.statuses[resourceType].Metadata = metadata
		lm.logger.WithField("plugin", metadata.Name).Info("Plugin initialized successfully")
	}

	return nil
}

func (lm *LifecycleManager) resolveInitializationOrder(plugins []PluginMetadata) []PluginMetadata {
	graph := make(map[string][]string)
	inDegree := make(map[string]int)

	for _, p := range plugins {
		graph[p.ResourceType] = p.Dependencies
		inDegree[p.ResourceType] = 0
	}

	for _, p := range plugins {
		for _, dep := range p.Dependencies {
			inDegree[p.ResourceType]++
		}
	}

	var queue []PluginMetadata
	for _, p := range plugins {
		if inDegree[p.ResourceType] == 0 {
			queue = append(queue, p)
		}
	}

	var result []PluginMetadata
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)

		for _, p := range plugins {
			for _, dep := range p.Dependencies {
				if dep == current.ResourceType {
					inDegree[p.ResourceType]--
					if inDegree[p.ResourceType] == 0 {
						queue = append(queue, p)
					}
				}
			}
		}
	}

	return result
}

func (lm *LifecycleManager) StartHealthMonitoring(ctx context.Context) {
	ticker := time.NewTicker(lm.healthTick)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-lm.stopCh:
			return
		case <-ticker.C:
			lm.performHealthChecks(ctx)
		}
	}
}

func (lm *LifecycleManager) performHealthChecks(ctx context.Context) {
	results := lm.registry.HealthCheck(ctx)

	lm.mu.Lock()
	defer lm.mu.Unlock()

	for resourceType, err := range results {
		if status, exists := lm.statuses[resourceType]; exists {
			status.LastHealth = time.Now()
			if err != nil {
				status.State = StateError
				status.Error = err
			} else if status.State == StateError {
				status.State = StateReady
				status.Error = nil
			}
		}
	}
}

func (lm *LifecycleManager) ShutdownAll() error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	close(lm.stopCh)

	var errors []error
	for resourceType, status := range lm.statuses {
		if status.State == StateReady || status.State == StateRunning {
			status.State = StateShuttingDown
			if err := lm.registry.Unregister(resourceType); err != nil {
				errors = append(errors, err)
				status.State = StateError
				status.Error = err
			} else {
				status.State = StateStopped
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors during shutdown: %v", errors)
	}
	return nil
}

func (lm *LifecycleManager) GetStatus(resourceType string) (*PluginStatus, error) {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	status, exists := lm.statuses[resourceType]
	if !exists {
		return nil, fmt.Errorf("no status for resource type: %s", resourceType)
	}
	return status, nil
}

func (lm *LifecycleManager) GetAllStatuses() map[string]*PluginStatus {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	result := make(map[string]*PluginStatus)
	for k, v := range lm.statuses {
		result[k] = v
	}
	return result
}

func (lm *LifecycleManager) ReloadPlugin(resourceType string, config *PluginConfig) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if status, exists := lm.statuses[resourceType]; exists {
		if status.State == StateRunning {
			return fmt.Errorf("cannot reload running plugin: %s", resourceType)
		}
	}

	if err := lm.registry.Unregister(resourceType); err != nil {
		lm.logger.WithError(err).Debug("Plugin not registered, proceeding with reload")
	}

	lm.statuses[resourceType] = &PluginStatus{
		Name:  resourceType,
		State: StateInitializing,
	}

	if config == nil {
		config = &PluginConfig{Config: make(map[string]interface{})}
	}

	if err := lm.registry.Register(nil, config); err != nil {
		lm.statuses[resourceType].State = StateError
		lm.statuses[resourceType].Error = err
		return err
	}

	lm.statuses[resourceType].State = StateReady
	return nil
}

func CreateChainMiddleware(middlewares ...Middleware) Middleware {
	return func(next ExecuteFunc) ExecuteFunc {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
}

func LoggingMiddleware(logger *logging.Logger) Middleware {
	return func(next ExecuteFunc) ExecuteFunc {
		return func(ctx context.Context, operation string, params interface{}) (*types.AWSResource, error) {
			start := time.Now()
			result, err := next(ctx, operation, params)
			duration := time.Since(start)

			logger.WithFields(map[string]interface{}{
				"operation":    operation,
				"duration_ms":  duration.Milliseconds(),
				"has_error":    err != nil,
				"params_type":  reflect.TypeOf(params).String(),
			}).Info("Operation executed")

			return result, err
		}
	}
}

func RecoveryMiddleware(logger *logging.Logger) Middleware {
	return func(next ExecuteFunc) ExecuteFunc {
		return func(ctx context.Context, operation string, params interface{}) (result *types.AWSResource, err error) {
			defer func() {
				if r := recover(); r != nil {
					buf := make([]byte, 4096)
					n := runtime.Stack(buf, false)
					err = fmt.Errorf("panic recovered: %v\n%s", r, buf[:n])
					logger.WithField("stack", string(buf[:n])).Error("Panic in operation")
				}
			}()
			return next(ctx, operation, params)
		}
	}
}

func MetricsMiddleware() Middleware {
	metrics := make(map[string]int64)
	var mu sync.Mutex

	return func(next ExecuteFunc) ExecuteFunc {
		return func(ctx context.Context, operation string, params interface{}) (*types.AWSResource, error) {
			start := time.Now()
			result, err := next(ctx, operation, params)
			duration := time.Since(start).Microseconds()

			mu.Lock()
			metrics[operation] = duration
			mu.Unlock()

			return result, err
		}
	}
}

func TimeoutMiddleware(timeout time.Duration) Middleware {
	return func(next ExecuteFunc) ExecuteFunc {
		return func(ctx context.Context, operation string, params interface{}) (*types.AWSResource, error) {
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			resultCh := make(chan struct {
				result *types.AWSResource
				err    error
			})

			go func() {
				result, err := next(ctx, operation, params)
				resultCh <- struct {
					result *types.AWSResource
					err    error
				}{result, err}
			}()

			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("operation %s timed out after %v", operation, timeout)
			case res := <-resultCh:
				return res.result, res.err
			}
		}
	}
}
