package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ConfigLoader handles loading and parsing YAML configuration files
type ConfigLoader struct {
	configDir string
}

// NewConfigLoader creates a new config loader with the specified directory
func NewConfigLoader(configDir string) *ConfigLoader {
	return &ConfigLoader{
		configDir: configDir,
	}
}

// LoadFieldMappings loads the field mapping configuration
func (c *ConfigLoader) LoadFieldMappings() (*FieldMappingConfig, error) {
	var config FieldMappingConfig
	err := c.loadYAMLFile("field-mappings-enhanced.yaml", &config)
	if err != nil {
		return nil, fmt.Errorf("failed to load field mappings: %w", err)
	}
	return &config, nil
}

// LoadResourcePatterns loads the resource pattern configuration
func (c *ConfigLoader) LoadResourcePatterns() (*ResourcePatternConfig, error) {
	var config ResourcePatternConfig
	err := c.loadYAMLFile("resource-patterns-enhanced.yaml", &config)
	if err != nil {
		return nil, fmt.Errorf("failed to load resource patterns: %w", err)
	}
	return &config, nil
}

// LoadResourceExtraction loads the resource extraction configuration
func (c *ConfigLoader) LoadResourceExtraction() (*ResourceExtractionConfig, error) {
	var config ResourceExtractionConfig
	err := c.loadYAMLFile("resource-extraction-enhanced.yaml", &config)
	if err != nil {
		return nil, fmt.Errorf("failed to load resource extraction config: %w", err)
	}
	return &config, nil
}

// loadYAMLFile loads and unmarshals a YAML file into the provided structure
func (c *ConfigLoader) loadYAMLFile(filename string, target interface{}) error {
	filePath := filepath.Join(c.configDir, filename)

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read config file %s: %w", filePath, err)
	}

	err = yaml.Unmarshal(data, target)
	if err != nil {
		return fmt.Errorf("failed to parse YAML in %s: %w", filePath, err)
	}

	return nil
}

// FieldMappingConfig represents the field mapping configuration structure
type FieldMappingConfig struct {
	ResourceFields       map[string]map[string][]string `yaml:"resource_fields"`
	FieldTransformations FieldTransformations           `yaml:"field_transformations"`
}

// FieldTransformations represents field transformation rules
type FieldTransformations struct {
	State         map[string][]string `yaml:"state"`
	BooleanFields []string            `yaml:"boolean_fields"`
	ArrayFields   []string            `yaml:"array_fields"`
}

// ResourcePatternConfig represents the resource pattern configuration structure
type ResourcePatternConfig struct {
	ResourceIdentification ResourceIdentification `yaml:"resource_identification"`
	ToolResourcePatterns   map[string][]string    `yaml:"tool_resource_patterns"`
	ResourceRelationships  ResourceRelationships  `yaml:"resource_relationships"`
	ToolCategories         map[string][]string    `yaml:"tool_categories"`
}

// ResourceIdentification contains patterns for identifying resource types
type ResourceIdentification struct {
	IDPatterns          map[string][]string `yaml:"id_patterns"`
	NamePatterns        map[string][]string `yaml:"name_patterns"`
	DescriptionPatterns map[string][]string `yaml:"description_patterns"`
}

// ResourceRelationships defines parent-child and dependency relationships
type ResourceRelationships struct {
	VPC              []string            `yaml:"vpc"`
	Subnet           []string            `yaml:"subnet"`
	SecurityGroup    []string            `yaml:"security_group"`
	LoadBalancer     []string            `yaml:"load_balancer"`
	AutoScalingGroup []string            `yaml:"auto_scaling_group"`
	LaunchTemplate   []string            `yaml:"launch_template"`
	Dependencies     map[string][]string `yaml:"dependencies"`
}

// ResourceExtractionConfig represents the resource extraction configuration structure
type ResourceExtractionConfig struct {
	CommonFallbackFields   []string                       `yaml:"common_fallback_fields"`
	ResourceIDExtraction   ResourceIDExtraction           `yaml:"resource_id_extraction"`
	SpecialExtractionRules map[string]map[string][]string `yaml:"special_extraction_rules"`
	FallbackStrategies     FallbackStrategies             `yaml:"fallback_strategies"`
}

// ResourceIDExtraction contains extraction patterns by tool type
type ResourceIDExtraction struct {
	CreationTools     ToolPatterns `yaml:"creation_tools"`
	ModificationTools ToolPatterns `yaml:"modification_tools"`
	AssociationTools  ToolPatterns `yaml:"association_tools"`
	DeletionTools     ToolPatterns `yaml:"deletion_tools"`
	QueryTools        ToolPatterns `yaml:"query_tools"`
}

// ToolPatterns contains extraction patterns for a tool category
type ToolPatterns struct {
	Patterns []ExtractionPattern `yaml:"patterns"`
}

// ExtractionPattern defines how to extract resource IDs from tool results
type ExtractionPattern struct {
	FieldPaths    []string `yaml:"field_paths"`
	ResourceTypes []string `yaml:"resource_types"`
	Priority      int      `yaml:"priority"`
}

// FallbackStrategies defines fallback methods when pattern matching fails
type FallbackStrategies struct {
	CommonIDFields   []string `yaml:"common_id_fields"`
	ArrayExtraction  []string `yaml:"array_extraction"`
	NestedExtraction []string `yaml:"nested_extraction"`
}
