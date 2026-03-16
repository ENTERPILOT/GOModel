package providers

import (
	"net/http"
	"strings"

	"gomodel/internal/core"
)

// PassthroughEndpoint normalizes a provider-relative passthrough endpoint into
// an absolute path fragment suitable for baseURL + endpoint request building.
func PassthroughEndpoint(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "/"
	}
	if strings.HasPrefix(endpoint, "/") {
		return endpoint
	}
	return "/" + endpoint
}

// CloneHTTPHeaders returns a detached copy of an http.Header map.
func CloneHTTPHeaders(src http.Header) map[string][]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string][]string, len(src))
	for key, values := range src {
		cloned := make([]string, len(values))
		copy(cloned, values)
		dst[key] = cloned
	}
	return dst
}

// PassthroughEndpointPath returns the normalized path portion of a provider
// passthrough endpoint, preferring a semantic normalized endpoint when present.
func PassthroughEndpointPath(info *core.PassthroughRouteInfo) string {
	if info == nil {
		return ""
	}
	endpoint := strings.TrimSpace(info.NormalizedEndpoint)
	if endpoint == "" {
		endpoint = strings.TrimSpace(info.RawEndpoint)
	}
	endpoint, _, _ = strings.Cut(endpoint, "?")
	if endpoint == "" {
		return ""
	}
	if !strings.HasPrefix(endpoint, "/") {
		endpoint = "/" + endpoint
	}
	return endpoint
}
