package safety

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

type SimulationMode string

const (
	SimulationDisabled SimulationMode = "disabled"
	SimulationDryRun   SimulationMode = "dry_run"
	SimulationValidate SimulationMode = "validate_only"
)

type SimulationResult struct {
	Success          bool
	Message          string
	WouldCreate      []*types.AWSResource
	WouldModify      []*types.AWSResource
	WouldDelete      []string
	ValidationErrors []ValidationError
	Warnings         []string
	Duration         time.Duration
	EstimatedCost    *CostEstimate
}

type ValidationError struct {
	Field   string
	Message string
	Rule    string
	Level   ValidationLevel
}

type ValidationLevel string

const (
	ValidationErrorLevel   ValidationLevel = "error"
	ValidationWarningLevel ValidationLevel = "warning"
	ValidationInfoLevel    ValidationLevel = "info"
)

type CostEstimate struct {
	Hourly   float64
	Monthly  float64
	Currency string
	Items    []CostItem
}

type CostItem struct {
	Resource    string
	Type        string
	HourlyCost  float64
	MonthlyCost float64
}

type OperationSimulator interface {
	SimulateCreate(ctx context.Context, resourceType string, params interface{}) (*SimulationResult, error)
	SimulateUpdate(ctx context.Context, resourceType string, id string, params interface{}) (*SimulationResult, error)
	SimulateDelete(ctx context.Context, resourceType string, id string) (*SimulationResult, error)
	SimulateSpecial(ctx context.Context, resourceType string, operation string, params interface{}) (*SimulationResult, error)
}

type SafetyValidator interface {
	ValidateParams(operation string, params interface{}) []ValidationError
	ValidateDependencies(operation string, params interface{}, state map[string]interface{}) []ValidationError
	ValidateQuotas(ctx context.Context, resourceType string, params interface{}) []ValidationError
	ValidateNaming(resourceType string, name string) []ValidationError
}

type SimulationConfig struct {
	Mode                SimulationMode
	EnableCostEstimate  bool
	EnableQuotaCheck    bool
	EnableDependencyCheck bool
	MaxWarnings         int
	StrictMode          bool
}

type Simulator struct {
	logger     *logging.Logger
	config     SimulationConfig
	validators map[string]SafetyValidator
	rules      map[string][]ValidationRule
	mu         sync.RWMutex
}

type ValidationRule struct {
	Name        string
	Field       string
	RuleType    string
	Params      map[string]interface{}
	Level       ValidationLevel
	Description string
}

func NewSimulator(logger *logging.Logger, config SimulationConfig) *Simulator {
	return &Simulator{
		logger:     logger,
		config:     config,
		validators: make(map[string]SafetyValidator),
		rules:      make(map[string][]ValidationRule),
	}
}

func (s *Simulator) RegisterValidator(resourceType string, validator SafetyValidator) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.validators[resourceType] = validator
}

func (s *Simulator) AddRule(resourceType string, rule ValidationRule) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rules[resourceType] = append(s.rules[resourceType], rule)
}

func (s *Simulator) SimulateCreate(ctx context.Context, resourceType string, params interface{}) (*SimulationResult, error) {
	start := time.Now()
	result := &SimulationResult{
		Success:          true,
		WouldCreate:      make([]*types.AWSResource, 0),
		WouldModify:      make([]*types.AWSResource, 0),
		WouldDelete:      make([]string, 0),
		ValidationErrors: make([]ValidationError, 0),
		Warnings:         make([]string, 0),
	}

	if s.config.Mode == SimulationDisabled {
		return result, nil
	}

	s.mu.RLock()
	validator, hasValidator := s.validators[resourceType]
	rules := s.rules[resourceType]
	s.mu.RUnlock()

	if hasValidator {
		result.ValidationErrors = append(result.ValidationErrors, validator.ValidateParams("create", params)...)
	}

	for _, rule := range rules {
		if err := s.applyRule(rule, params); err != nil {
			result.ValidationErrors = append(result.ValidationErrors, *err)
		}
	}

	if len(result.ValidationErrors) > 0 {
		for _, e := range result.ValidationErrors {
			if e.Level == ValidationErrorLevel {
				result.Success = false
				break
			}
		}
	}

	simulatedResource := s.createSimulatedResource(resourceType, params)
	result.WouldCreate = append(result.WouldCreate, simulatedResource)

	if s.config.EnableCostEstimate {
		result.EstimatedCost = s.estimateCost(resourceType, params)
	}

	result.Duration = time.Since(start)
	result.Message = s.buildSimulationMessage(result)

	return result, nil
}

func (s *Simulator) SimulateUpdate(ctx context.Context, resourceType string, id string, params interface{}) (*SimulationResult, error) {
	start := time.Now()
	result := &SimulationResult{
		Success:          true,
		WouldCreate:      make([]*types.AWSResource, 0),
		WouldModify:      make([]*types.AWSResource, 0),
		WouldDelete:      make([]string, 0),
		ValidationErrors: make([]ValidationError, 0),
		Warnings:         make([]string, 0),
	}

	if s.config.Mode == SimulationDisabled {
		return result, nil
	}

	s.mu.RLock()
	validator, hasValidator := s.validators[resourceType]
	s.mu.RUnlock()

	if hasValidator {
		result.ValidationErrors = append(result.ValidationErrors, validator.ValidateParams("update", params)...)
	}

	if id == "" {
		result.ValidationErrors = append(result.ValidationErrors, ValidationError{
			Field:   "id",
			Message: "Resource ID is required for update operation",
			Rule:    "required",
			Level:   ValidationErrorLevel,
		})
	}

	if len(result.ValidationErrors) > 0 {
		for _, e := range result.ValidationErrors {
			if e.Level == ValidationErrorLevel {
				result.Success = false
				break
			}
		}
	}

	modifiedResource := &types.AWSResource{
		ID:     id,
		Type:   resourceType,
		State:  "would_be_modified",
		Region: "simulated",
	}
	result.WouldModify = append(result.WouldModify, modifiedResource)

	result.Duration = time.Since(start)
	result.Message = s.buildSimulationMessage(result)

	return result, nil
}

func (s *Simulator) SimulateDelete(ctx context.Context, resourceType string, id string) (*SimulationResult, error) {
	start := time.Now()
	result := &SimulationResult{
		Success:          true,
		WouldCreate:      make([]*types.AWSResource, 0),
		WouldModify:      make([]*types.AWSResource, 0),
		WouldDelete:      make([]string, 0),
		ValidationErrors: make([]ValidationError, 0),
		Warnings:         make([]string, 0),
	}

	if s.config.Mode == SimulationDisabled {
		return result, nil
	}

	if id == "" {
		result.ValidationErrors = append(result.ValidationErrors, ValidationError{
			Field:   "id",
			Message: "Resource ID is required for delete operation",
			Rule:    "required",
			Level:   ValidationErrorLevel,
		})
		result.Success = false
	} else {
		result.WouldDelete = append(result.WouldDelete, id)
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Resource %s of type %s would be permanently deleted", id, resourceType))
	}

	result.Duration = time.Since(start)
	result.Message = s.buildSimulationMessage(result)

	return result, nil
}

func (s *Simulator) SimulateSpecial(ctx context.Context, resourceType string, operation string, params interface{}) (*SimulationResult, error) {
	start := time.Now()
	result := &SimulationResult{
		Success:          true,
		WouldCreate:      make([]*types.AWSResource, 0),
		WouldModify:      make([]*types.AWSResource, 0),
		WouldDelete:      make([]string, 0),
		ValidationErrors: make([]ValidationError, 0),
		Warnings:         make([]string, 0),
	}

	if s.config.Mode == SimulationDisabled {
		return result, nil
	}

	s.mu.RLock()
	validator, hasValidator := s.validators[resourceType]
	s.mu.RUnlock()

	if hasValidator {
		result.ValidationErrors = append(result.ValidationErrors, validator.ValidateParams(operation, params)...)
	}

	switch operation {
	case "start":
		result.Message = "Resource would be started"
	case "stop":
		result.Message = "Resource would be stopped"
		result.Warnings = append(result.Warnings, "Stopping may cause service interruption")
	case "reboot":
		result.Message = "Resource would be rebooted"
		result.Warnings = append(result.Warnings, "Reboot will cause temporary unavailability")
	default:
		result.Message = fmt.Sprintf("Special operation '%s' would be executed", operation)
	}

	if len(result.ValidationErrors) > 0 {
		for _, e := range result.ValidationErrors {
			if e.Level == ValidationErrorLevel {
				result.Success = false
				break
			}
		}
	}

	result.Duration = time.Since(start)
	return result, nil
}

func (s *Simulator) ValidateBeforeExecute(operation string, resourceType string, params interface{}) ([]ValidationError, bool) {
	errors := make([]ValidationError, 0)

	s.mu.RLock()
	validator, hasValidator := s.validators[resourceType]
	rules := s.rules[resourceType]
	s.mu.RUnlock()

	if hasValidator {
		errors = append(errors, validator.ValidateParams(operation, params)...)
	}

	for _, rule := range rules {
		if err := s.applyRule(rule, params); err != nil {
			errors = append(errors, *err)
		}
	}

	hasBlockingErrors := false
	for _, e := range errors {
		if e.Level == ValidationErrorLevel {
			hasBlockingErrors = true
			break
		}
	}

	return errors, !hasBlockingErrors
}

func (s *Simulator) applyRule(rule ValidationRule, params interface{}) *ValidationError {
	v := reflect.ValueOf(params)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() == reflect.Struct {
		field := v.FieldByName(rule.Field)
		if !field.IsValid() {
			if rule.RuleType == "required" {
				return &ValidationError{
					Field:   rule.Field,
					Message: fmt.Sprintf("Field %s is required", rule.Field),
					Rule:    rule.Name,
					Level:   rule.Level,
				}
			}
			return nil
		}

		switch rule.RuleType {
		case "required":
			if field.IsZero() {
				return &ValidationError{
					Field:   rule.Field,
					Message: fmt.Sprintf("Field %s is required", rule.Field),
					Rule:    rule.Name,
					Level:   rule.Level,
				}
			}
		case "min_length":
			if minLen, ok := rule.Params["min"].(int); ok {
				if field.Len() < minLen {
					return &ValidationError{
						Field:   rule.Field,
						Message: fmt.Sprintf("Field %s must be at least %d characters", rule.Field, minLen),
						Rule:    rule.Name,
						Level:   rule.Level,
					}
				}
			}
		case "max_length":
			if maxLen, ok := rule.Params["max"].(int); ok {
				if field.Len() > maxLen {
					return &ValidationError{
						Field:   rule.Field,
						Message: fmt.Sprintf("Field %s must be at most %d characters", rule.Field, maxLen),
						Rule:    rule.Name,
						Level:   rule.Level,
					}
				}
			}
		case "pattern":
			if pattern, ok := rule.Params["pattern"].(string); ok {
				if !strings.Contains(field.String(), pattern) {
					return &ValidationError{
						Field:   rule.Field,
						Message: fmt.Sprintf("Field %s does not match required pattern", rule.Field),
						Rule:    rule.Name,
						Level:   rule.Level,
					}
				}
			}
		}
	}

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
		case "min":
			if minVal, ok := rule.Params["min"].(float64); ok {
				if numVal, ok := value.(float64); ok && numVal < minVal {
					return &ValidationError{
						Field:   rule.Field,
						Message: fmt.Sprintf("Field %s must be at least %v", rule.Field, minVal),
						Rule:    rule.Name,
						Level:   rule.Level,
					}
				}
			}
		case "max":
			if maxVal, ok := rule.Params["max"].(float64); ok {
				if numVal, ok := value.(float64); ok && numVal > maxVal {
					return &ValidationError{
						Field:   rule.Field,
						Message: fmt.Sprintf("Field %s must be at most %v", rule.Field, maxVal),
						Rule:    rule.Name,
						Level:   rule.Level,
					}
				}
			}
		}
	}

	return nil
}

func (s *Simulator) createSimulatedResource(resourceType string, params interface{}) *types.AWSResource {
	resource := &types.AWSResource{
		Type:   resourceType,
		State:  "would_create",
		Region: "simulated",
		Details: map[string]interface{}{
			"simulated": true,
		},
	}

	if m, ok := params.(map[string]interface{}); ok {
		if name, exists := m["name"].(string); exists {
			resource.ID = name
		} else if id, exists := m["instanceId"].(string); exists {
			resource.ID = id
		} else {
			resource.ID = fmt.Sprintf("simulated-%s-%d", resourceType, time.Now().Unix())
		}

		for k, v := range m {
			resource.Details[k] = v
		}
	}

	return resource
}

func (s *Simulator) estimateCost(resourceType string, params interface{}) *CostEstimate {
	estimate := &CostEstimate{
		Currency: "USD",
		Items:    make([]CostItem, 0),
	}

	costTable := map[string]float64{
		"ec2-instance.t2.micro":       0.0116,
		"ec2-instance.t2.small":       0.023,
		"ec2-instance.t2.medium":      0.0464,
		"ec2-instance.t3.micro":       0.0104,
		"ec2-instance.t3.small":       0.0208,
		"ec2-instance.t3.medium":      0.0416,
		"rds-instance.db.t3.micro":    0.022,
		"rds-instance.db.t3.small":    0.044,
		"rds-instance.db.t3.medium":   0.088,
	}

	var instanceType string
	if m, ok := params.(map[string]interface{}); ok {
		if t, exists := m["instanceType"].(string); exists {
			instanceType = t
		} else if t, exists := m["dbInstanceClass"].(string); exists {
			instanceType = t
		}
	}

	key := fmt.Sprintf("%s.%s", resourceType, instanceType)
	if hourly, exists := costTable[key]; exists {
		item := CostItem{
			Resource:    resourceType,
			Type:        instanceType,
			HourlyCost:  hourly,
			MonthlyCost: hourly * 24 * 30,
		}
		estimate.Items = append(estimate.Items, item)
		estimate.Hourly += hourly
		estimate.Monthly += hourly * 24 * 30
	}

	return estimate
}

func (s *Simulator) buildSimulationMessage(result *SimulationResult) string {
	var parts []string

	if len(result.WouldCreate) > 0 {
		parts = append(parts, fmt.Sprintf("Would create %d resource(s)", len(result.WouldCreate)))
	}
	if len(result.WouldModify) > 0 {
		parts = append(parts, fmt.Sprintf("Would modify %d resource(s)", len(result.WouldModify)))
	}
	if len(result.WouldDelete) > 0 {
		parts = append(parts, fmt.Sprintf("Would delete %d resource(s)", len(result.WouldDelete)))
	}

	if len(result.ValidationErrors) > 0 {
		errorCount := 0
		for _, e := range result.ValidationErrors {
			if e.Level == ValidationErrorLevel {
				errorCount++
			}
		}
		if errorCount > 0 {
			parts = append(parts, fmt.Sprintf("Found %d validation error(s)", errorCount))
		}
	}

	if len(parts) == 0 {
		return "Simulation completed with no changes"
	}

	return strings.Join(parts, "; ")
}

func (s *Simulator) IsSimulationEnabled() bool {
	return s.config.Mode != SimulationDisabled
}

func (s *Simulator) GetMode() SimulationMode {
	return s.config.Mode
}

func (s *Simulator) SetMode(mode SimulationMode) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.Mode = mode
	s.logger.WithField("mode", mode).Info("Simulation mode changed")
}
