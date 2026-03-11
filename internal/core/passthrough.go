package core

import (
	"context"
	"io"
	"net/http"
)

// PassthroughRequest is the transport-oriented request for opaque provider-native forwarding.
type PassthroughRequest struct {
	Method   string
	Endpoint string
	Body     io.ReadCloser
	Headers  http.Header
}

// PassthroughResponse is the raw upstream response for opaque forwarding.
// Body is an io.ReadCloser returned by the upstream provider, and callers are
// responsible for closing it when they are finished with the response body.
type PassthroughResponse struct {
	StatusCode int
	Headers    map[string][]string
	Body       io.ReadCloser
}

// PassthroughProvider supports opaque provider-native forwarding.
type PassthroughProvider interface {
	Passthrough(ctx context.Context, req *PassthroughRequest) (*PassthroughResponse, error)
}

// PassthroughRoutableProvider resolves a provider type before issuing an opaque passthrough request.
type PassthroughRoutableProvider interface {
	Passthrough(ctx context.Context, providerType string, req *PassthroughRequest) (*PassthroughResponse, error)
}
