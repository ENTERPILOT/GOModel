package gateway

import (
	"encoding/json"
	"strings"

	"gomodel/internal/core"
)

// CloneChatRequestForStreamUsage clones chat stream options before usage mutation.
func CloneChatRequestForStreamUsage(req *core.ChatRequest) *core.ChatRequest {
	if req == nil {
		return nil
	}
	cloned := *req
	if req.StreamOptions != nil {
		streamOptions := *req.StreamOptions
		cloned.StreamOptions = &streamOptions
	}
	return &cloned
}

// CloneChatRequestForSelector clones a chat request for a concrete selector.
func CloneChatRequestForSelector(req *core.ChatRequest, selector core.ModelSelector) *core.ChatRequest {
	if req == nil {
		return nil
	}
	cloned := *req
	cloned.Model = selector.Model
	cloned.Provider = selector.Provider
	if req.StreamOptions != nil {
		streamOptions := *req.StreamOptions
		cloned.StreamOptions = &streamOptions
	}
	return &cloned
}

// CloneResponsesRequestForSelector clones a Responses request for a concrete selector.
func CloneResponsesRequestForSelector(req *core.ResponsesRequest, selector core.ModelSelector) *core.ResponsesRequest {
	if req == nil {
		return nil
	}
	cloned := *req
	cloned.Model = selector.Model
	cloned.Provider = selector.Provider
	if req.StreamOptions != nil {
		streamOptions := *req.StreamOptions
		cloned.StreamOptions = &streamOptions
	}
	return &cloned
}

// ProviderNameFromWorkflow returns the resolved configured provider name.
func ProviderNameFromWorkflow(workflow *core.Workflow) string {
	if workflow == nil || workflow.Resolution == nil {
		return ""
	}
	return strings.TrimSpace(workflow.Resolution.ProviderName)
}

func resolvedModelPrefix(workflow *core.Workflow, providerName string) string {
	if providerName = strings.TrimSpace(providerName); providerName != "" {
		return providerName
	}
	if workflow == nil || workflow.Resolution == nil {
		return ""
	}
	if providerName = strings.TrimSpace(workflow.Resolution.ProviderName); providerName != "" {
		return providerName
	}
	return strings.TrimSpace(workflow.Resolution.ResolvedSelector.Provider)
}

// QualifyModelWithProvider prefixes a model with providerName when needed.
func QualifyModelWithProvider(model, providerName string) string {
	model = strings.TrimSpace(model)
	providerName = strings.TrimSpace(providerName)
	if model == "" {
		return ""
	}
	if providerName == "" || strings.HasPrefix(model, providerName+"/") {
		return model
	}
	return providerName + "/" + model
}

// QualifyExecutedModel returns the public executed model selector.
func QualifyExecutedModel(workflow *core.Workflow, model, providerName string) string {
	return QualifyModelWithProvider(model, resolvedModelPrefix(workflow, providerName))
}

// ResolvedModelFromWorkflow returns the resolved model or fallback.
func ResolvedModelFromWorkflow(workflow *core.Workflow, fallback string) string {
	fallback = strings.TrimSpace(fallback)
	if workflow == nil || workflow.Resolution == nil {
		return fallback
	}
	if resolvedModel := strings.TrimSpace(workflow.Resolution.ResolvedSelector.Model); resolvedModel != "" {
		return resolvedModel
	}
	return fallback
}

// MarshalRequestBody serializes a patched request struct to JSON bytes for cache key computation.
func MarshalRequestBody(req any) ([]byte, error) {
	return json.Marshal(req)
}

// ProviderTypeFromWorkflow returns the workflow provider type.
func ProviderTypeFromWorkflow(workflow *core.Workflow) string {
	if workflow == nil {
		return ""
	}
	return strings.TrimSpace(workflow.ProviderType)
}

func currentSelectorForWorkflow(workflow *core.Workflow, model, provider string) string {
	if workflow != nil && workflow.Resolution != nil {
		if resolved := strings.TrimSpace(workflow.Resolution.ResolvedQualifiedModel()); resolved != "" {
			return resolved
		}
	}
	selector, err := core.ParseModelSelector(model, provider)
	if err != nil {
		return strings.TrimSpace(model)
	}
	return selector.QualifiedModel()
}

// ResponseProviderType returns responseProvider when set, otherwise fallback.
func ResponseProviderType(fallback, responseProvider string) string {
	responseProvider = strings.TrimSpace(responseProvider)
	if responseProvider != "" {
		return responseProvider
	}
	return strings.TrimSpace(fallback)
}
