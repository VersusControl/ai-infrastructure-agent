package adapters

import (
	"context"
	"fmt"

	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
	"github.com/versus-control/ai-infrastructure-agent/pkg/aws"
	"github.com/versus-control/ai-infrastructure-agent/pkg/interfaces"
	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

// KeyPairAdapter implements the AWSResourceAdapter interface for Key Pair resources
type KeyPairAdapter struct {
	*BaseAWSAdapter
	client *aws.Client
}

// NewKeyPairAdapter creates a new Key Pair adapter
func NewKeyPairAdapter(client *aws.Client, logger *logging.Logger) interfaces.AWSResourceAdapter {
	base := NewBaseAWSAdapter(client, logger, "key_pair")
	return &KeyPairAdapter{
		BaseAWSAdapter: base,
		client:         client,
	}
}

// Create creates a new key pair
func (k *KeyPairAdapter) Create(ctx context.Context, params interface{}) (*types.AWSResource, error) {
	createParams, ok := params.(aws.CreateKeyPairParams)
	if !ok {
		return nil, fmt.Errorf("invalid parameters for key pair creation, expected aws.CreateKeyPairParams")
	}

	result, err := k.client.CreateKeyPair(ctx, createParams)
	if err != nil {
		return nil, fmt.Errorf("failed to create key pair: %w", err)
	}

	k.logger.Infof("Created key pair %s", result.ID)
	return result, nil
}

// List returns all key pairs
func (k *KeyPairAdapter) List(ctx context.Context) ([]*types.AWSResource, error) {
	keyPairs, err := k.client.ListKeyPairs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list key pairs: %w", err)
	}

	return keyPairs, nil
}

// Get retrieves a specific key pair by name
func (k *KeyPairAdapter) Get(ctx context.Context, id string) (*types.AWSResource, error) {
	// For key pairs, the 'id' is actually the key name
	keyPair, err := k.client.GetKeyPair(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get key pair %s: %w", id, err)
	}

	return keyPair, nil
}

// Not supported by AWS, implemented to satisfy interface
func (k *KeyPairAdapter) Update(ctx context.Context, id string, params interface{}) (*types.AWSResource, error) {
	// Key pairs cannot be updated in AWS - they must be deleted and recreated
	return nil, fmt.Errorf("key pair updates are not supported, please delete and recreate the key pair")
}

// Not supported by AWS, implemented to satisfy interface
func (k *KeyPairAdapter) Delete(ctx context.Context, id string) error {
	return fmt.Errorf("key pair deletion is not supported through this adapter")
}

// GetSupportedOperations returns the operations supported by this adapter
func (k *KeyPairAdapter) GetSupportedOperations() []string {
	return []string{
		"create",
		"list",
		"get",
		"import",
	}
}

// ValidateParams validates key pair-specific parameters
func (k *KeyPairAdapter) ValidateParams(operation string, params interface{}) error {
	switch operation {
	case "create":
		createParams, ok := params.(aws.CreateKeyPairParams)
		if !ok {
			return fmt.Errorf("invalid parameters for create operation")
		}
		if createParams.KeyName == "" {
			return fmt.Errorf("keyName is required for key pair creation")
		}
		// Validate key type if specified
		if createParams.KeyType != "" && createParams.KeyType != "rsa" && createParams.KeyType != "ed25519" {
			return fmt.Errorf("keyType must be 'rsa' or 'ed25519'")
		}
		// Validate key format if specified
		if createParams.KeyFormat != "" && createParams.KeyFormat != "pem" && createParams.KeyFormat != "ppk" {
			return fmt.Errorf("keyFormat must be 'pem' or 'ppk'")
		}
		return nil
	case "import":
		importParams, ok := params.(aws.ImportKeyPairParams)
		if !ok {
			return fmt.Errorf("invalid parameters for import operation")
		}
		if importParams.KeyName == "" {
			return fmt.Errorf("keyName is required for key pair import")
		}
		if len(importParams.PublicKeyMaterial) == 0 {
			return fmt.Errorf("publicKeyMaterial is required for key pair import")
		}
		return nil
	case "get":
		if params == nil {
			return fmt.Errorf("key pair name is required for %s operation", operation)
		}
		return nil
	default:
		return fmt.Errorf("unsupported operation: %s", operation)
	}
}

// ExecuteSpecialOperation executes key pair-specific operations
func (k *KeyPairAdapter) ExecuteSpecialOperation(ctx context.Context, operation string, params interface{}) (*types.AWSResource, error) {
	switch operation {
	case "import":
		importParams, ok := params.(aws.ImportKeyPairParams)
		if !ok {
			return nil, fmt.Errorf("import key pair parameters required")
		}

		if err := k.ValidateParams("import", importParams); err != nil {
			return nil, fmt.Errorf("parameter validation failed for import operation: %w", err)
		}

		result, err := k.client.ImportKeyPair(ctx, importParams)
		if err != nil {
			return nil, fmt.Errorf("failed to import key pair: %w", err)
		}

		k.logger.Infof("Imported key pair %s", importParams.KeyName)
		return result, nil

	default:
		return nil, fmt.Errorf("unsupported special operation: %s", operation)
	}
}

// GetSpecialOperations returns the list of supported special operations
func (k *KeyPairAdapter) GetSpecialOperations() []string {
	return []string{
		"import",
	}
}

// ListByTags overrides the base implementation with key pair-specific logic
func (k *KeyPairAdapter) ListByTags(ctx context.Context, tags map[string]string) ([]*types.AWSResource, error) {
	// Get all key pairs and filter by tags
	allKeyPairs, err := k.client.ListKeyPairs(ctx)
	if err != nil {
		return nil, err
	}

	var filtered []*types.AWSResource
	for _, keyPair := range allKeyPairs {
		if k.matchesTags(keyPair.Tags, tags) {
			filtered = append(filtered, keyPair)
		}
	}

	return filtered, nil
}

// ListByFilter overrides the base implementation with key pair-specific logic
func (k *KeyPairAdapter) ListByFilter(ctx context.Context, filters map[string]interface{}) ([]*types.AWSResource, error) {
	// Get all key pairs and filter
	allKeyPairs, err := k.client.ListKeyPairs(ctx)
	if err != nil {
		return nil, err
	}

	var filtered []*types.AWSResource
	for _, keyPair := range allKeyPairs {
		if k.matchesFilters(keyPair, filters) {
			filtered = append(filtered, keyPair)
		}
	}

	return filtered, nil
}
