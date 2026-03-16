package anthropic

import (
	"strings"

	"gomodel/internal/core"
	"gomodel/internal/providers"
)

type passthroughSemanticEnricher struct{}

func (passthroughSemanticEnricher) ProviderType() string {
	return "anthropic"
}

func (passthroughSemanticEnricher) Enrich(_ *core.RequestSnapshot, _ *core.WhiteBoxPrompt, info *core.PassthroughRouteInfo) *core.PassthroughRouteInfo {
	if info == nil {
		return nil
	}
	enriched := *info
	switch providers.PassthroughEndpointPath(info) {
	case "/messages":
		enriched.SemanticOperation = "anthropic.messages"
		enriched.AuditPath = "/v1/messages"
	case "/messages/batches":
		enriched.SemanticOperation = "anthropic.messages_batches"
		enriched.AuditPath = "/v1/messages/batches"
	default:
		if strings.TrimSpace(enriched.AuditPath) == "" {
			enriched.AuditPath = "/p/anthropic/" + strings.TrimLeft(strings.TrimSpace(info.RawEndpoint), "/")
		}
	}
	return &enriched
}
