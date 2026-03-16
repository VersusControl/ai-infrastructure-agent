package safety

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
)

type ValidatorRegistry struct {
	logger    *logging.Logger
	rules     map[string][]ValidationRule
	providers map[string]ValidationProvider
	mu        sync.RWMutex
}

type ValidationProvider interface {
	Validate(ctx context.Context, resourceType string, operation string, params interface{}) []ValidationError
	GetRules() []ValidationRule
}

func NewValidatorRegistry(logger *logging.Logger) *ValidatorRegistry {
	vr := &ValidatorRegistry{
		logger:    logger,
		rules:     make(map[string][]ValidationRule),
		providers: make(map[string]ValidationProvider),
	}

	vr.registerDefaultRules()
	return vr
}

func (vr *ValidatorRegistry) registerDefaultRules() {
	ec2Rules := []ValidationRule{
		{Name: "ec2_image_required", Field: "imageId", RuleType: "required", Level: ValidationErrorLevel, Description: "AMI ID is required"},
		{Name: "ec2_instance_type_required", Field: "instanceType", RuleType: "required", Level: ValidationErrorLevel, Description: "Instance type is required"},
		{Name: "ec2_instance_type_valid", Field: "instanceType", RuleType: "pattern", Level: ValidationErrorLevel, Params: map[string]interface{}{"pattern": "^[a-z][0-9][a-z]?\\.[a-z0-9]+$"}, Description: "Invalid instance type format"},
	}

	rdsRules := []ValidationRule{
		{Name: "rds_engine_required", Field: "engine", RuleType: "required", Level: ValidationErrorLevel, Description: "Database engine is required"},
		{Name: "rds_class_required", Field: "dbInstanceClass", RuleType: "required", Level: ValidationErrorLevel, Description: "DB instance class is required"},
		{Name: "rds_identifier_required", Field: "dbInstanceIdentifier", RuleType: "required", Level: ValidationErrorLevel, Description: "DB identifier is required"},
		{Name: "rds_storage_min", Field: "allocatedStorage", RuleType: "min", Level: ValidationWarningLevel, Params: map[string]interface{}{"min": 20.0}, Description: "Storage should be at least 20GB"},
	}

	vpcRules := []ValidationRule{
		{Name: "vpc_cidr_required", Field: "cidrBlock", RuleType: "required", Level: ValidationErrorLevel, Description: "CIDR block is required"},
		{Name: "vpc_cidr_valid", Field: "cidrBlock", RuleType: "cidr", Level: ValidationErrorLevel, Description: "Valid CIDR format required"},
	}

	securityGroupRules := []ValidationRule{
		{Name: "sg_name_required", Field: "groupName", RuleType: "required", Level: ValidationErrorLevel, Description: "Security group name is required"},
		{Name: "sg_vpc_required", Field: "vpcId", RuleType: "required", Level: ValidationErrorLevel, Description: "VPC ID is required"},
	}

	vr.rules["ec2-instance"] = ec2Rules
	vr.rules["rds-instance"] = rdsRules
	vr.rules["vpc"] = vpcRules
	vr.rules["security-group"] = securityGroupRules
}

func (vr *ValidatorRegistry) RegisterProvider(resourceType string, provider ValidationProvider) {
	vr.mu.Lock()
	defer vr.mu.Unlock()
	vr.providers[resourceType] = provider

	for _, rule := range provider.GetRules() {
		vr.rules[resourceType] = append(vr.rules[resourceType], rule)
	}
}

func (vr *ValidatorRegistry) Validate(ctx context.Context, resourceType string, operation string, params interface{}) []ValidationError {
	vr.mu.RLock()
	rules := vr.rules[resourceType]
	provider, hasProvider := vr.providers[resourceType]
	vr.mu.RUnlock()

	errors := make([]ValidationError, 0)

	for _, rule := range rules {
		if err := vr.applyRule(rule, params); err != nil {
			errors = append(errors, *err)
		}
	}

	if hasProvider {
		errors = append(errors, provider.Validate(ctx, resourceType, operation, params)...)
	}

	return errors
}

func (vr *ValidatorRegistry) applyRule(rule ValidationRule, params interface{}) *ValidationError {
	m, ok := params.(map[string]interface{})
	if !ok {
		return nil
	}

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

	case "pattern":
		if !exists || value == nil {
			return nil
		}
		strVal, _ := value.(string)
		pattern, _ := rule.Params["pattern"].(string)
		if pattern != "" {
			matched, err := regexp.MatchString(pattern, strVal)
			if err != nil || !matched {
				return &ValidationError{
					Field:   rule.Field,
					Message: fmt.Sprintf("Field %s has invalid format", rule.Field),
					Rule:    rule.Name,
					Level:   rule.Level,
				}
			}
		}

	case "cidr":
		if !exists || value == nil {
			return nil
		}
		strVal, _ := value.(string)
		if !isValidCIDR(strVal) {
			return &ValidationError{
				Field:   rule.Field,
				Message: fmt.Sprintf("Field %s is not a valid CIDR block", rule.Field),
				Rule:    rule.Name,
				Level:   rule.Level,
			}
		}

	case "min":
		if !exists || value == nil {
			return nil
		}
		minVal, _ := rule.Params["min"].(float64)
		if numVal, ok := value.(float64); ok && numVal < minVal {
			return &ValidationError{
				Field:   rule.Field,
				Message: fmt.Sprintf("Field %s must be at least %v", rule.Field, minVal),
				Rule:    rule.Name,
				Level:   rule.Level,
			}
		}

	case "max":
		if !exists || value == nil {
			return nil
		}
		maxVal, _ := rule.Params["max"].(float64)
		if numVal, ok := value.(float64); ok && numVal > maxVal {
			return &ValidationError{
				Field:   rule.Field,
				Message: fmt.Sprintf("Field %s must be at most %v", rule.Field, maxVal),
				Rule:    rule.Name,
				Level:   rule.Level,
			}
		}

	case "enum":
		if !exists || value == nil {
			return nil
		}
		allowed, _ := rule.Params["values"].([]string)
		strVal, _ := value.(string)
		found := false
		for _, a := range allowed {
			if a == strVal {
				found = true
				break
			}
		}
		if !found {
			return &ValidationError{
				Field:   rule.Field,
				Message: fmt.Sprintf("Field %s must be one of: %v", rule.Field, allowed),
				Rule:    rule.Name,
				Level:   rule.Level,
			}
		}
	}

	return nil
}

func isValidCIDR(cidr string) bool {
	parts := strings.Split(cidr, "/")
	if len(parts) != 2 {
		return false
	}

	ipPattern := `^(\d{1,3}\.){3}\d{1,3}$`
	matched, _ := regexp.MatchString(ipPattern, parts[0])
	if !matched {
		return false
	}

	for _, octet := range strings.Split(parts[0], ".") {
		var num int
		fmt.Sscanf(octet, "%d", &num)
		if num < 0 || num > 255 {
			return false
		}
	}

	var prefix int
	fmt.Sscanf(parts[1], "%d", &prefix)
	if prefix < 0 || prefix > 32 {
		return false
	}

	return true
}

func (vr *ValidatorRegistry) AddRule(resourceType string, rule ValidationRule) {
	vr.mu.Lock()
	defer vr.mu.Unlock()
	vr.rules[resourceType] = append(vr.rules[resourceType], rule)
}

func (vr *ValidatorRegistry) GetRules(resourceType string) []ValidationRule {
	vr.mu.RLock()
	defer vr.mu.RUnlock()
	return vr.rules[resourceType]
}

type DependencyValidator struct {
	logger *logging.Logger
}

func NewDependencyValidator(logger *logging.Logger) *DependencyValidator {
	return &DependencyValidator{logger: logger}
}

func (dv *DependencyValidator) ValidateDependencies(plan []*PlanStep) []ValidationError {
	errors := make([]ValidationError, 0)
	resolved := make(map[string]bool)

	for _, step := range plan {
		for _, dep := range step.DependsOn {
			if !resolved[dep] {
				errors = append(errors, ValidationError{
					Field:   step.ID,
					Message: fmt.Sprintf("Step %s depends on unresolved step %s", step.ID, dep),
					Rule:    "dependency_order",
					Level:   ValidationErrorLevel,
				})
			}
		}
		resolved[step.ID] = true
	}

	return errors
}

func (dv *DependencyValidator) DetectCircularDependencies(plan []*PlanStep) []ValidationError {
	errors := make([]ValidationError, 0)
	graph := make(map[string][]string)

	for _, step := range plan {
		graph[step.ID] = step.DependsOn
	}

	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	var dfs func(node string, path []string) bool
	dfs = func(node string, path []string) bool {
		visited[node] = true
		recStack[node] = true

		for _, neighbor := range graph[node] {
			if !visited[neighbor] {
				if dfs(neighbor, append(path, neighbor)) {
					return true
				}
			} else if recStack[neighbor] {
				errors = append(errors, ValidationError{
					Field:   node,
					Message: fmt.Sprintf("Circular dependency detected: %v", append(path, neighbor)),
					Rule:    "circular_dependency",
					Level:   ValidationErrorLevel,
				})
				return true
			}
		}

		recStack[node] = false
		return false
	}

	for _, step := range plan {
		if !visited[step.ID] {
			dfs(step.ID, []string{step.ID})
		}
	}

	return errors
}

type PlanStep struct {
	ID        string
	DependsOn []string
}

type QuotaValidator struct {
	logger *logging.Logger
	quotas map[string]ResourceQuota
}

type ResourceQuota struct {
	ResourceType string
	MaxCount     int
	CurrentCount int
	LastUpdated  time.Time
}

func NewQuotaValidator(logger *logging.Logger) *QuotaValidator {
	return &QuotaValidator{
		logger: logger,
		quotas: make(map[string]ResourceQuota),
	}
}

func (qv *QuotaValidator) SetQuota(resourceType string, maxCount int) {
	qv.quotas[resourceType] = ResourceQuota{
		ResourceType: resourceType,
		MaxCount:     maxCount,
		LastUpdated:  time.Now(),
	}
}

func (qv *QuotaValidator) UpdateCurrentCount(resourceType string, count int) {
	if quota, exists := qv.quotas[resourceType]; exists {
		quota.CurrentCount = count
		quota.LastUpdated = time.Now()
		qv.quotas[resourceType] = quota
	}
}

func (qv *QuotaValidator) ValidateQuota(resourceType string, additionalCount int) []ValidationError {
	errors := make([]ValidationError, 0)

	quota, exists := qv.quotas[resourceType]
	if !exists {
		return errors
	}

	if quota.CurrentCount+additionalCount > quota.MaxCount {
		errors = append(errors, ValidationError{
			Field:   resourceType,
			Message: fmt.Sprintf("Quota exceeded: %s has max %d, current %d, requesting %d", resourceType, quota.MaxCount, quota.CurrentCount, additionalCount),
			Rule:    "quota_exceeded",
			Level:   ValidationErrorLevel,
		})
	}

	return errors
}

func (qv *QuotaValidator) GetQuotaStatus(resourceType string) (*ResourceQuota, error) {
	quota, exists := qv.quotas[resourceType]
	if !exists {
		return nil, fmt.Errorf("no quota configured for resource type: %s", resourceType)
	}
	return &quota, nil
}
