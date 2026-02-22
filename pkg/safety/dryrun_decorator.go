package safety

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

type ExecutionContext struct {
	Operation     string
	ResourceType  string
	Params        interface{}
	DryRun        bool
	SimulatedOnly bool
	StartTime     time.Time
	CorrelationID string
}

type OperationFunc func(ctx context.Context, params interface{}) (*types.AWSResource, error)

type DryRunDecorator struct {
	logger     *logging.Logger
	simulator  *Simulator
	auditLog   *AuditLog
	config     DryRunConfig
	mu         sync.RWMutex
}

type DryRunConfig struct {
	EnabledByDefault   bool
	RequireConfirmation bool
	AllowedOperations  []string
	BlockedOperations  []string
	AuditEnabled       bool
}

func NewDryRunDecorator(logger *logging.Logger, simulator *Simulator, config DryRunConfig) *DryRunDecorator {
	return &DryRunDecorator{
		logger:    logger,
		simulator: simulator,
		auditLog:  NewAuditLog(logger),
		config:    config,
	}
}

func (d *DryRunDecorator) Decorate(operation OperationFunc, execCtx *ExecutionContext) OperationFunc {
	return func(ctx context.Context, params interface{}) (*types.AWSResource, error) {
		if execCtx == nil {
			execCtx = &ExecutionContext{
				DryRun:    d.config.EnabledByDefault,
				StartTime: time.Now(),
			}
		}

		if d.isBlocked(execCtx.Operation) {
			return nil, fmt.Errorf("operation %s is blocked by safety policy", execCtx.Operation)
		}

		if execCtx.DryRun || d.simulator.IsSimulationEnabled() {
			return d.executeDryRun(ctx, operation, params, execCtx)
		}

		if err := d.preExecuteValidation(ctx, execCtx, params); err != nil {
			return nil, fmt.Errorf("pre-execution validation failed: %w", err)
		}

		d.auditLog.Record(AuditEntry{
			CorrelationID: execCtx.CorrelationID,
			Operation:     execCtx.Operation,
			ResourceType:  execCtx.ResourceType,
			Params:        params,
			DryRun:        false,
			Status:        "started",
			Timestamp:     time.Now(),
		})

		result, err := operation(ctx, params)

		status := "completed"
		if err != nil {
			status = "failed"
		}

		d.auditLog.Record(AuditEntry{
			CorrelationID: execCtx.CorrelationID,
			Operation:     execCtx.Operation,
			ResourceType:  execCtx.ResourceType,
			Params:        params,
			Result:        result,
			Error:         err,
			DryRun:        false,
			Status:        status,
			Duration:      time.Since(execCtx.StartTime),
			Timestamp:     time.Now(),
		})

		return result, err
	}
}

func (d *DryRunDecorator) executeDryRun(ctx context.Context, operation OperationFunc, params interface{}, execCtx *ExecutionContext) (*types.AWSResource, error) {
	d.logger.WithFields(map[string]interface{}{
		"operation":    execCtx.Operation,
		"resourceType": execCtx.ResourceType,
		"dryRun":       true,
	}).Info("Executing in dry-run mode")

	var simResult *SimulationResult
	var simErr error

	switch execCtx.Operation {
	case "create":
		simResult, simErr = d.simulator.SimulateCreate(ctx, execCtx.ResourceType, params)
	case "update":
		var id string
		if m, ok := params.(map[string]interface{}); ok {
			id, _ = m["id"].(string)
		}
		simResult, simErr = d.simulator.SimulateUpdate(ctx, execCtx.ResourceType, id, params)
	case "delete":
		var id string
		if m, ok := params.(map[string]interface{}); ok {
			id, _ = m["id"].(string)
		} else {
			id, _ = params.(string)
		}
		simResult, simErr = d.simulator.SimulateDelete(ctx, execCtx.ResourceType, id)
	default:
		simResult, simErr = d.simulator.SimulateSpecial(ctx, execCtx.ResourceType, execCtx.Operation, params)
	}

	if simErr != nil {
		return nil, fmt.Errorf("simulation failed: %w", simErr)
	}

	d.auditLog.Record(AuditEntry{
		CorrelationID: execCtx.CorrelationID,
		Operation:     execCtx.Operation,
		ResourceType:  execCtx.ResourceType,
		Params:        params,
		SimResult:     simResult,
		DryRun:        true,
		Status:        "simulated",
		Duration:      time.Since(execCtx.StartTime),
		Timestamp:     time.Now(),
	})

	if !simResult.Success {
		return nil, fmt.Errorf("dry-run validation failed: %s", simResult.Message)
	}

	return &types.AWSResource{
		ID:     fmt.Sprintf("dry-run-%s", execCtx.CorrelationID),
		Type:   execCtx.ResourceType,
		State:  "simulated",
		Region: "dry-run",
		Details: map[string]interface{}{
			"simulation_result": simResult,
			"dry_run":           true,
		},
	}, nil
}

func (d *DryRunDecorator) preExecuteValidation(ctx context.Context, execCtx *ExecutionContext, params interface{}) error {
	errors, valid := d.simulator.ValidateBeforeExecute(execCtx.Operation, execCtx.ResourceType, params)
	if !valid {
		var errMsgs []string
		for _, e := range errors {
			if e.Level == ValidationErrorLevel {
				errMsgs = append(errMsgs, fmt.Sprintf("%s: %s", e.Field, e.Message))
			}
		}
		return fmt.Errorf("validation errors: %v", errMsgs)
	}

	for _, e := range errors {
		if e.Level == ValidationWarningLevel {
			d.logger.WithFields(map[string]interface{}{
				"field":   e.Field,
				"message": e.Message,
				"rule":    e.Rule,
			}).Warn("Validation warning")
		}
	}

	return nil
}

func (d *DryRunDecorator) isBlocked(operation string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	for _, blocked := range d.config.BlockedOperations {
		if blocked == operation {
			return true
		}
	}
	return false
}

func (d *DryRunDecorator) SetDryRunMode(enabled bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.config.EnabledByDefault = enabled
	d.logger.WithField("dry_run_enabled", enabled).Info("Dry-run mode updated")
}

func (d *DryRunDecorator) GetAuditLog() []AuditEntry {
	return d.auditLog.GetEntries()
}

type AuditEntry struct {
	CorrelationID string
	Operation     string
	ResourceType  string
	Params        interface{}
	Result        interface{}
	SimResult     *SimulationResult
	Error         error
	DryRun        bool
	Status        string
	Duration      time.Duration
	Timestamp     time.Time
}

type AuditLog struct {
	logger  *logging.Logger
	entries []AuditEntry
	mu      sync.RWMutex
	maxSize int
}

func NewAuditLog(logger *logging.Logger) *AuditLog {
	return &AuditLog{
		logger:  logger,
		entries: make([]AuditEntry, 0),
		maxSize: 10000,
	}
}

func (a *AuditLog) Record(entry AuditEntry) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if len(a.entries) >= a.maxSize {
		a.entries = a.entries[1:]
	}

	a.entries = append(a.entries, entry)

	a.logger.WithFields(map[string]interface{}{
		"correlation_id": entry.CorrelationID,
		"operation":      entry.Operation,
		"resource_type":  entry.ResourceType,
		"dry_run":        entry.DryRun,
		"status":         entry.Status,
		"duration_ms":    entry.Duration.Milliseconds(),
	}).Info("Audit record")
}

func (a *AuditLog) GetEntries() []AuditEntry {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make([]AuditEntry, len(a.entries))
	copy(result, a.entries)
	return result
}

func (a *AuditLog) GetEntriesByOperation(operation string) []AuditEntry {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var result []AuditEntry
	for _, entry := range a.entries {
		if entry.Operation == operation {
			result = append(result, entry)
		}
	}
	return result
}

func (a *AuditLog) GetEntriesByCorrelationID(correlationID string) []AuditEntry {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var result []AuditEntry
	for _, entry := range a.entries {
		if entry.CorrelationID == correlationID {
			result = append(result, entry)
		}
	}
	return result
}

type ValidationLayer struct {
	logger     *logging.Logger
	validators map[string]ResourceValidator
	rules      map[string][]ValidationRule
	mu         sync.RWMutex
}

type ResourceValidator func(ctx context.Context, params interface{}) []ValidationError

func NewValidationLayer(logger *logging.Logger) *ValidationLayer {
	return &ValidationLayer{
		logger:     logger,
		validators: make(map[string]ResourceValidator),
		rules:      make(map[string][]ValidationRule),
	}
}

func (v *ValidationLayer) RegisterValidator(resourceType string, validator ResourceValidator) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.validators[resourceType] = validator
}

func (v *ValidationLayer) AddRule(resourceType string, rule ValidationRule) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.rules[resourceType] = append(v.rules[resourceType], rule)
}

func (v *ValidationLayer) Validate(ctx context.Context, resourceType string, operation string, params interface{}) (*ValidationResult, error) {
	v.mu.RLock()
	validator, hasValidator := v.validators[resourceType]
	rules := v.rules[resourceType]
	v.mu.RUnlock()

	result := &ValidationResult{
		Valid:    true,
		Errors:   make([]ValidationError, 0),
		Warnings: make([]ValidationError, 0),
	}

	if hasValidator {
		errors := validator(ctx, params)
		for _, e := range errors {
			if e.Level == ValidationErrorLevel {
				result.Errors = append(result.Errors, e)
			} else {
				result.Warnings = append(result.Warnings, e)
			}
		}
	}

	for _, rule := range rules {
		if err := v.applyRule(rule, params); err != nil {
			if err.Level == ValidationErrorLevel {
				result.Errors = append(result.Errors, *err)
			} else {
				result.Warnings = append(result.Warnings, *err)
			}
		}
	}

	result.Valid = len(result.Errors) == 0

	if !result.Valid {
		return result, fmt.Errorf("validation failed with %d errors", len(result.Errors))
	}

	return result, nil
}

type ValidationResult struct {
	Valid    bool
	Errors   []ValidationError
	Warnings []ValidationError
}

func (v *ValidationLayer) applyRule(rule ValidationRule, params interface{}) *ValidationError {
	val := reflect.ValueOf(params)

	if val.Kind() == reflect.Map {
		if m, ok := params.(map[string]interface{}); ok {
			value, exists := m[rule.Field]

			switch rule.RuleType {
			case "required":
				if !exists || value == nil || value == "" {
					return &ValidationError{
						Field:   rule.Field,
						Message: fmt.Sprintf("Field %s is required", rule.Field),
						Rule:    rule.Name,
						Level:   rule.Level,
					}
				}
			case "enum":
				if allowed, ok := rule.Params["values"].([]string); ok {
					strVal, _ := value.(string)
					found := false
					for _, a := range allowed {
						if a == strVal {
							found = true
							break
						}
					}
					if !found && exists {
						return &ValidationError{
							Field:   rule.Field,
							Message: fmt.Sprintf("Field %s must be one of: %v", rule.Field, allowed),
							Rule:    rule.Name,
							Level:   rule.Level,
						}
					}
				}
			case "range":
				min, hasMin := rule.Params["min"].(float64)
				max, hasMax := rule.Params["max"].(float64)
				if numVal, ok := value.(float64); ok && exists {
					if hasMin && numVal < min {
						return &ValidationError{
							Field:   rule.Field,
							Message: fmt.Sprintf("Field %s must be >= %v", rule.Field, min),
							Rule:    rule.Name,
							Level:   rule.Level,
						}
					}
					if hasMax && numVal > max {
						return &ValidationError{
							Field:   rule.Field,
							Message: fmt.Sprintf("Field %s must be <= %v", rule.Field, max),
							Rule:    rule.Name,
							Level:   rule.Level,
						}
					}
				}
			}
		}
	}

	return nil
}

func (v *ValidationLayer) ValidatePlan(plan []*types.ExecutionPlanStep) (*PlanValidationResult, error) {
	result := &PlanValidationResult{
		Valid:       true,
		StepResults: make(map[string]*ValidationResult),
		GlobalErrors: make([]ValidationError, 0),
	}

	seenResources := make(map[string]bool)
	for _, step := range plan {
		if step.ResourceID != "" {
			if seenResources[step.ResourceID] {
				result.GlobalErrors = append(result.GlobalErrors, ValidationError{
					Field:   step.ID,
					Message: fmt.Sprintf("Duplicate resource ID: %s", step.ResourceID),
					Rule:    "unique_resource_id",
					Level:   ValidationErrorLevel,
				})
			}
			seenResources[step.ResourceID] = true
		}

		stepResult, _ := v.Validate(context.Background(), step.ResourceID, step.Action, step.ToolParameters)
		result.StepResults[step.ID] = stepResult
		if !stepResult.Valid {
			result.Valid = false
		}
	}

	if len(result.GlobalErrors) > 0 {
		result.Valid = false
	}

	if !result.Valid {
		return result, fmt.Errorf("plan validation failed")
	}

	return result, nil
}

type PlanValidationResult struct {
	Valid        bool
	StepResults  map[string]*ValidationResult
	GlobalErrors []ValidationError
}
