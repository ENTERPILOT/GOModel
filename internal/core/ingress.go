package core

// IngressFrame is the transport-level capture of an inbound request. It
// preserves the request as received at the HTTP boundary so later stages can
// extract semantics without losing fidelity while keeping mutable state behind
// defensive-copy accessors.
type IngressFrame struct {
	// Method is the inbound HTTP method.
	Method string
	// Path is the request URL path as received at ingress.
	Path string
	// RouteParams contains resolved router parameters such as provider or file id.
	routeParams map[string]string
	// QueryParams contains the raw query string values by key.
	queryParams map[string][]string
	// Headers contains the inbound HTTP headers exactly as captured at ingress.
	headers map[string][]string
	// ContentType is the inbound Content-Type header value.
	ContentType string
	// RawBody contains the captured request body bytes when the body fit within
	// the ingress capture limit.
	rawBody []byte
	// RawBodyTooLarge reports that the request body exceeded the capture limit,
	// so RawBody is omitted and the live body stream remains on the request.
	RawBodyTooLarge bool
	// RequestID is the canonical request id propagated through context, headers,
	// providers, and audit records for this request.
	RequestID string
	// TraceMetadata contains tracing-related key/value pairs such as trace/span
	// ids or baggage/sampling metadata derived from tracing headers.
	traceMetadata map[string]string
}

// NewIngressFrame constructs an IngressFrame and defensively copies its
// mutable map and byte-slice inputs.
func NewIngressFrame(method, path string, routeParams map[string]string, queryParams, headers map[string][]string, contentType string, rawBody []byte, rawBodyTooLarge bool, requestID string, traceMetadata map[string]string) *IngressFrame {
	return &IngressFrame{
		Method:          method,
		Path:            path,
		routeParams:     cloneStringMap(routeParams),
		queryParams:     cloneMultiMap(queryParams),
		headers:         cloneMultiMap(headers),
		ContentType:     contentType,
		rawBody:         cloneBytes(rawBody),
		RawBodyTooLarge: rawBodyTooLarge,
		RequestID:       requestID,
		traceMetadata:   cloneStringMap(traceMetadata),
	}
}

// GetRawBody returns a defensive copy of the captured raw body bytes.
func (f *IngressFrame) GetRawBody() []byte {
	if f == nil {
		return nil
	}
	return cloneBytes(f.rawBody)
}

// GetRouteParams returns a defensive copy of the captured route parameters.
func (f *IngressFrame) GetRouteParams() map[string]string {
	if f == nil {
		return nil
	}
	return cloneStringMap(f.routeParams)
}

// GetQueryParams returns a defensive copy of the captured query parameters.
func (f *IngressFrame) GetQueryParams() map[string][]string {
	if f == nil {
		return nil
	}
	return cloneMultiMap(f.queryParams)
}

// GetHeaders returns a defensive copy of the captured request headers.
func (f *IngressFrame) GetHeaders() map[string][]string {
	if f == nil {
		return nil
	}
	return cloneMultiMap(f.headers)
}

// GetTraceMetadata returns a defensive copy of the captured trace metadata.
func (f *IngressFrame) GetTraceMetadata() map[string]string {
	if f == nil {
		return nil
	}
	return cloneStringMap(f.traceMetadata)
}

func cloneBytes(src []byte) []byte {
	if src == nil {
		return nil
	}
	dst := make([]byte, len(src))
	copy(dst, src)
	return dst
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func cloneMultiMap(src map[string][]string) map[string][]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string][]string, len(src))
	for key, values := range src {
		if len(values) == 0 {
			dst[key] = nil
			continue
		}
		cloned := make([]string, len(values))
		copy(cloned, values)
		dst[key] = cloned
	}
	return dst
}
