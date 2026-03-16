package openai

import (
	"strings"

	"gomodel/internal/core"
	"gomodel/internal/providers"
)

type passthroughSemanticEnricher struct{}

func (passthroughSemanticEnricher) ProviderType() string {
	return "openai"
}

func (passthroughSemanticEnricher) Enrich(_ *core.RequestSnapshot, _ *core.WhiteBoxPrompt, info *core.PassthroughRouteInfo) *core.PassthroughRouteInfo {
	if info == nil {
		return nil
	}
	enriched := *info
	switch providers.PassthroughEndpointPath(info) {
	case "/chat/completions":
		enriched.SemanticOperation = "openai.chat_completions"
		enriched.AuditPath = "/v1/chat/completions"
	case "/responses":
		enriched.SemanticOperation = "openai.responses"
		enriched.AuditPath = "/v1/responses"
	case "/embeddings":
		enriched.SemanticOperation = "openai.embeddings"
		enriched.AuditPath = "/v1/embeddings"
	default:
		if strings.TrimSpace(enriched.AuditPath) == "" {
			enriched.AuditPath = "/p/openai/" + strings.TrimLeft(strings.TrimSpace(info.RawEndpoint), "/")
		}
	}
	return &enriched
}
