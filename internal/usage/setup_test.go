package usage

import (
	"os"
	"testing"

	"gomodel/internal/core"
	"gomodel/internal/providers/anthropic"
	"gomodel/internal/providers/gemini"
	"gomodel/internal/providers/openai"
	"gomodel/internal/providers/xai"
)

func TestMain(m *testing.M) {
	// Register cost mappings from the authoritative provider Registration vars
	// so tests use the single source of truth rather than duplicated data.
	RegisterCostMappings(map[string][]core.TokenCostMapping{
		openai.Registration.Type:    openai.Registration.CostMappings,
		anthropic.Registration.Type: anthropic.Registration.CostMappings,
		gemini.Registration.Type:    gemini.Registration.CostMappings,
		xai.Registration.Type:       xai.Registration.CostMappings,
	}, openai.Registration.InformationalFields)

	os.Exit(m.Run())
}
