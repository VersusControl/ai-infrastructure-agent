package tools

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/adapters"
	"github.com/versus-control/ai-infrastructure-agent/pkg/aws"
	"github.com/versus-control/ai-infrastructure-agent/pkg/interfaces"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// CreateKeyPairTool implements MCPTool for creating EC2 key pairs
type CreateKeyPairTool struct {
	*BaseTool
	adapter interfaces.AWSResourceAdapter
}

// NewCreateKeyPairTool creates a new key pair creation tool
func NewCreateKeyPairTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"keyName": map[string]interface{}{
				"type":        "string",
				"description": "The name of the key pair (unique within the region)",
			},
			"keyType": map[string]interface{}{
				"type":        "string",
				"description": "The type of key pair: 'rsa' (default) or 'ed25519'",
				"enum":        []string{"rsa", "ed25519"},
				"default":     "rsa",
			},
			"keyFormat": map[string]interface{}{
				"type":        "string",
				"description": "The format of the private key: 'pem' (default) or 'ppk' (for PuTTY)",
				"enum":        []string{"pem", "ppk"},
				"default":     "pem",
			},
			"tags": map[string]interface{}{
				"type":        "object",
				"description": "Tags to apply to the key pair",
			},
		},
		"required": []interface{}{"keyName"},
	}

	baseTool := NewBaseTool(
		"create-key-pair",
		"Create a new EC2 key pair for SSH access. Returns the private key material which should be saved immediately as it cannot be retrieved later.",
		"ec2",
		actionType,
		inputSchema,
		logger,
	)

	// Add examples
	baseTool.AddExample(
		"Create a basic RSA key pair",
		map[string]interface{}{
			"keyName": "my-app-keypair",
		},
		"Successfully created key pair 'my-app-keypair' (RSA). Private key material returned - save it securely.",
	)

	baseTool.AddExample(
		"Create an ED25519 key pair with tags",
		map[string]interface{}{
			"keyName":   "production-keypair",
			"keyType":   "ed25519",
			"keyFormat": "pem",
			"tags": map[string]interface{}{
				"Environment": "production",
				"Purpose":     "SSH access",
			},
		},
		"Successfully created key pair 'production-keypair' (ED25519). Private key material returned.",
	)

	adapter := adapters.NewKeyPairAdapter(awsClient, logger)

	return &CreateKeyPairTool{
		BaseTool: baseTool,
		adapter:  adapter,
	}
}

// Execute creates a key pair
func (t *CreateKeyPairTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	// Extract and validate parameters
	keyName, _ := arguments["keyName"].(string)
	keyType, _ := arguments["keyType"].(string)
	keyFormat, _ := arguments["keyFormat"].(string)

	// Set defaults
	if keyType == "" {
		keyType = "rsa"
	}
	if keyFormat == "" {
		keyFormat = "pem"
	}

	// Extract tags
	tagSpecs := make(map[string]string)
	if tagsArg, exists := arguments["tags"]; exists {
		if tags, ok := tagsArg.(map[string]interface{}); ok {
			for k, v := range tags {
				tagSpecs[k] = fmt.Sprintf("%v", v)
			}
		}
	}

	// Create parameters struct
	params := aws.CreateKeyPairParams{
		KeyName:   keyName,
		KeyType:   keyType,
		KeyFormat: keyFormat,
		TagSpecs:  tagSpecs,
	}

	// Validate parameters
	if err := t.adapter.ValidateParams("create", params); err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Parameter validation failed: %s", err.Error()))
	}

	// Create the key pair using the adapter
	resource, err := t.adapter.Create(ctx, params)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to create key pair: %s", err.Error()))
	}

	// Extract private key material from details
	keyMaterial := ""
	if details, ok := resource.Details["keyMaterial"].(string); ok {
		keyMaterial = details
	}

	// Return success response with warning about saving the private key
	message := fmt.Sprintf("Successfully created key pair '%s' (ID: %s, Type: %s, Format: %s)\n\n"+
		"⚠️  IMPORTANT: Save the private key material below immediately. "+
		"It cannot be retrieved after this response.\n\n"+
		"Private Key Material:\n%s",
		keyName, resource.ID, keyType, keyFormat, keyMaterial)

	data := map[string]interface{}{
		"keyPairId":      resource.ID,
		"keyName":        keyName,
		"keyType":        keyType,
		"keyFormat":      keyFormat,
		"keyFingerprint": resource.Details["keyFingerprint"],
		"keyMaterial":    keyMaterial,
		"tags":           tagSpecs,
		"state":          resource.State,
	}

	return t.CreateSuccessResponse(message, data)
}

// ListKeyPairsTool implements MCPTool for listing EC2 key pairs
type ListKeyPairsTool struct {
	*BaseTool
	adapter interfaces.AWSResourceAdapter
}

// NewListKeyPairsTool creates a new tool for listing key pairs
func NewListKeyPairsTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"tags": map[string]interface{}{
				"type":        "object",
				"description": "Filter key pairs by tags (optional)",
			},
		},
	}

	baseTool := NewBaseTool(
		"list-key-pairs",
		"List EC2 key pairs with optional filtering by tags",
		"ec2",
		actionType,
		inputSchema,
		logger,
	)

	// Add examples
	baseTool.AddExample(
		"List all key pairs",
		map[string]interface{}{},
		"Retrieved 3 key pairs",
	)

	baseTool.AddExample(
		"List key pairs with specific tag",
		map[string]interface{}{
			"tags": map[string]interface{}{"Environment": "production"},
		},
		"Retrieved 1 production key pair",
	)

	adapter := adapters.NewKeyPairAdapter(awsClient, logger)

	return &ListKeyPairsTool{
		BaseTool: baseTool,
		adapter:  adapter,
	}
}

// Execute lists key pairs
func (t *ListKeyPairsTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	var keyPairs []*types.AWSResource
	var err error

	// Check if filtering by tags
	if tagsFilter, exists := arguments["tags"]; exists {
		if tags, ok := tagsFilter.(map[string]interface{}); ok {
			// Convert to map[string]string
			tagMap := make(map[string]string)
			for k, v := range tags {
				tagMap[k] = fmt.Sprintf("%v", v)
			}
			keyPairs, err = t.adapter.ListByTags(ctx, tagMap)
		} else {
			return t.CreateErrorResponse("Invalid tags filter format")
		}
	} else {
		// List all key pairs
		keyPairs, err = t.adapter.List(ctx)
	}

	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to list key pairs: %s", err.Error()))
	}

	// Format results
	var keyPairList []map[string]interface{}
	for _, kp := range keyPairs {
		keyPairList = append(keyPairList, map[string]interface{}{
			"keyPairId":      kp.ID,
			"keyName":        kp.Details["keyName"],
			"keyFingerprint": kp.Details["keyFingerprint"],
			"keyType":        kp.Details["keyType"],
			"createTime":     kp.Details["createTime"],
			"tags":           kp.Tags,
		})
	}

	message := fmt.Sprintf("Retrieved %d key pair(s)", len(keyPairs))
	data := map[string]interface{}{
		"count":    len(keyPairs),
		"keyPairs": keyPairList,
	}

	return t.CreateSuccessResponse(message, data)
}

// GetKeyPairTool implements MCPTool for retrieving a specific key pair
type GetKeyPairTool struct {
	*BaseTool
	adapter interfaces.AWSResourceAdapter
}

// NewGetKeyPairTool creates a new tool for getting key pair details
func NewGetKeyPairTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"keyName": map[string]interface{}{
				"type":        "string",
				"description": "The name of the key pair to retrieve",
			},
		},
		"required": []interface{}{"keyName"},
	}

	baseTool := NewBaseTool(
		"get-key-pair",
		"Get details of a specific EC2 key pair by name. Note: Private key material cannot be retrieved after creation.",
		"ec2",
		actionType,
		inputSchema,
		logger,
	)

	// Add example
	baseTool.AddExample(
		"Get key pair details",
		map[string]interface{}{
			"keyName": "my-app-keypair",
		},
		"Retrieved key pair 'my-app-keypair' details",
	)

	adapter := adapters.NewKeyPairAdapter(awsClient, logger)

	return &GetKeyPairTool{
		BaseTool: baseTool,
		adapter:  adapter,
	}
}

// Execute retrieves a key pair
func (t *GetKeyPairTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	keyName, _ := arguments["keyName"].(string)

	if keyName == "" {
		return t.CreateErrorResponse("keyName is required")
	}

	// Get the key pair using the adapter
	resource, err := t.adapter.Get(ctx, keyName)
	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to get key pair: %s", err.Error()))
	}

	message := fmt.Sprintf("Retrieved key pair '%s' (ID: %s)", keyName, resource.ID)
	data := map[string]interface{}{
		"keyPairId":      resource.ID,
		"keyName":        resource.Details["keyName"],
		"keyFingerprint": resource.Details["keyFingerprint"],
		"keyType":        resource.Details["keyType"],
		"createTime":     resource.Details["createTime"],
		"tags":           resource.Tags,
		"region":         resource.Region,
		"state":          resource.State,
	}

	return t.CreateSuccessResponse(message, data)
}

// ImportKeyPairTool implements MCPTool for importing public keys as key pairs
type ImportKeyPairTool struct {
	*BaseTool
	adapter interfaces.AWSResourceAdapter
}

// NewImportKeyPairTool creates a new tool for importing key pairs
func NewImportKeyPairTool(awsClient *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"keyName": map[string]interface{}{
				"type":        "string",
				"description": "The name for the imported key pair (unique within the region)",
			},
			"publicKeyMaterial": map[string]interface{}{
				"type":        "string",
				"description": "The public key material (base64 encoded or plain text SSH public key format)",
			},
			"tags": map[string]interface{}{
				"type":        "object",
				"description": "Tags to apply to the imported key pair",
			},
		},
		"required": []interface{}{"keyName", "publicKeyMaterial"},
	}

	baseTool := NewBaseTool(
		"import-key-pair",
		"Import a public key to create an EC2 key pair. Use this to use your own existing key pair instead of creating a new one.",
		"ec2",
		actionType,
		inputSchema,
		logger,
	)

	// Add example
	baseTool.AddExample(
		"Import an existing public key",
		map[string]interface{}{
			"keyName":           "my-imported-key",
			"publicKeyMaterial": "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQD...",
			"tags": map[string]interface{}{
				"Source": "laptop",
			},
		},
		"Successfully imported key pair 'my-imported-key'",
	)

	adapter := adapters.NewKeyPairAdapter(awsClient, logger)

	return &ImportKeyPairTool{
		BaseTool: baseTool,
		adapter:  adapter,
	}
}

// Execute imports a key pair
func (t *ImportKeyPairTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	keyName, _ := arguments["keyName"].(string)
	publicKeyMaterial, _ := arguments["publicKeyMaterial"].(string)

	if keyName == "" || publicKeyMaterial == "" {
		return t.CreateErrorResponse("keyName and publicKeyMaterial are required")
	}

	// Convert public key material to bytes
	// Try to decode as base64 first, if it fails, assume it's already in the correct format
	var keyBytes []byte
	decoded, err := base64.StdEncoding.DecodeString(publicKeyMaterial)
	if err == nil {
		keyBytes = decoded
	} else {
		// If not base64, use as-is (assuming it's SSH public key format)
		keyBytes = []byte(publicKeyMaterial)
	}

	// Extract tags
	tagSpecs := make(map[string]string)
	if tagsArg, exists := arguments["tags"]; exists {
		if tags, ok := tagsArg.(map[string]interface{}); ok {
			for k, v := range tags {
				tagSpecs[k] = fmt.Sprintf("%v", v)
			}
		}
	}

	// Create parameters struct
	params := aws.ImportKeyPairParams{
		KeyName:           keyName,
		PublicKeyMaterial: keyBytes,
		TagSpecs:          tagSpecs,
	}

	// Import the key pair using the adapter's special operation
	resource, err := t.adapter.(interface {
		ExecuteSpecialOperation(ctx context.Context, operation string, params interface{}) (*types.AWSResource, error)
	}).ExecuteSpecialOperation(ctx, "import", params)

	if err != nil {
		return t.CreateErrorResponse(fmt.Sprintf("Failed to import key pair: %s", err.Error()))
	}

	message := fmt.Sprintf("Successfully imported key pair '%s' (ID: %s)", keyName, resource.ID)
	data := map[string]interface{}{
		"keyPairId":      resource.ID,
		"keyName":        keyName,
		"keyFingerprint": resource.Details["keyFingerprint"],
		"keyType":        resource.Details["keyType"],
		"tags":           tagSpecs,
		"state":          resource.State,
	}

	return t.CreateSuccessResponse(message, data)
}
