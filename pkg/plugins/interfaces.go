package plugins

import (
	"context"

	"github.com/versus-control/ai-infrastructure-agent/pkg/types"
)

type PluginMetadata struct {
	Name           string
	Version        string
	Description    string
	ResourceType   string
	SupportedOps   []string
	Dependencies   []string
	Priority       int
	Author         string
}

type ResourceHandler interface {
	Plugin
	Create(ctx context.Context, params interface{}) (*types.AWSResource, error)
	List(ctx context.Context) ([]*types.AWSResource, error)
	Get(ctx context.Context, id string) (*types.AWSResource, error)
	Update(ctx context.Context, id string, params interface{}) (*types.AWSResource, error)
	Delete(ctx context.Context, id string) error
	ExecuteSpecial(ctx context.Context, operation string, params interface{}) (*types.AWSResource, error)
	ValidateParams(operation string, params interface{}) error
	GetSupportedOperations() []string
}

type Plugin interface {
	Initialize(config *PluginConfig) error
	Shutdown() error
	GetMetadata() PluginMetadata
	HealthCheck(ctx context.Context) error
}

type PluginConfig struct {
	AWSClient interface{}
	Logger    interface{}
	Config    map[string]interface{}
}

type PluginHook interface {
	BeforeExecute(ctx context.Context, operation string, params interface{}) (context.Context, error)
	AfterExecute(ctx context.Context, operation string, params interface{}, result interface{}, err error) error
	OnFailure(ctx context.Context, operation string, params interface{}, err error) error
}

type CompositePlugin struct {
	Metadata    PluginMetadata
	Handler     ResourceHandler
	Hooks       []PluginHook
	Middlewares []Middleware
}

type Middleware func(next ExecuteFunc) ExecuteFunc

type ExecuteFunc func(ctx context.Context, operation string, params interface{}) (*types.AWSResource, error)

type PluginEvent struct {
	Type      string
	Plugin    string
	Timestamp int64
	Data      interface{}
}

type PluginEventListener interface {
	OnPluginEvent(event PluginEvent)
}
