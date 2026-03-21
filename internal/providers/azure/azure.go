package azure

import (
	"net/http"
	"net/url"

	"gomodel/internal/core"
	"gomodel/internal/llmclient"
	"gomodel/internal/providers"
	"gomodel/internal/providers/openai"
)

const defaultAPIVersion = "2024-10-21"

var Registration = providers.Registration{
	Type:                        "azure",
	New:                         New,
	PassthroughSemanticEnricher: openai.Registration.PassthroughSemanticEnricher,
}

type Provider struct {
	*openai.CompatibleProvider
	apiVersion string
}

func New(apiKey string, opts providers.ProviderOptions) core.Provider {
	p := &Provider{apiVersion: defaultAPIVersion}
	p.CompatibleProvider = openai.NewCompatibleProvider(apiKey, opts, openai.CompatibleProviderConfig{
		ProviderName:   "azure",
		DefaultBaseURL: "https://example.invalid",
		SetHeaders:     setHeaders,
	})
	p.SetRequestMutator(p.mutateRequest)
	return p
}

func NewWithHTTPClient(apiKey string, httpClient *http.Client, hooks llmclient.Hooks) *Provider {
	p := &Provider{apiVersion: defaultAPIVersion}
	p.CompatibleProvider = openai.NewCompatibleProviderWithHTTPClient(apiKey, httpClient, hooks, openai.CompatibleProviderConfig{
		ProviderName:   "azure",
		DefaultBaseURL: "https://example.invalid",
		SetHeaders:     setHeaders,
	})
	p.SetRequestMutator(p.mutateRequest)
	return p
}

func (p *Provider) SetAPIVersion(version string) {
	if version == "" {
		return
	}
	p.apiVersion = version
}

func (p *Provider) mutateRequest(req *llmclient.Request) {
	endpoint, err := url.Parse(req.Endpoint)
	if err != nil {
		return
	}
	query := endpoint.Query()
	query.Set("api-version", p.apiVersion)
	endpoint.RawQuery = query.Encode()
	req.Endpoint = endpoint.String()
}

func setHeaders(req *http.Request, apiKey string) {
	req.Header.Set("api-key", apiKey)
	if requestID := core.GetRequestID(req.Context()); requestID != "" && isValidClientRequestID(requestID) {
		req.Header.Set("X-Client-Request-Id", requestID)
	}
}

func isValidClientRequestID(id string) bool {
	if len(id) > 512 {
		return false
	}
	for i := 0; i < len(id); i++ {
		if id[i] > 127 {
			return false
		}
	}
	return true
}
