package usage

import (
	"os"
	"testing"

	"gomodel/internal/providers"
	"gomodel/internal/providers/anthropic"
	"gomodel/internal/providers/gemini"
	"gomodel/internal/providers/openai"
	"gomodel/internal/providers/xai"
)

func TestMain(m *testing.M) {
	// Build cost mappings and informational fields the same way production does:
	// register all providers into a factory and use CostRegistry to aggregate.
	factory := providers.NewProviderFactory()
	factory.Add(openai.Registration)
	factory.Add(anthropic.Registration)
	factory.Add(gemini.Registration)
	factory.Add(xai.Registration)

	costMappings, informationalFields := factory.CostRegistry()
	RegisterCostMappings(costMappings, informationalFields)

	os.Exit(m.Run())
}
