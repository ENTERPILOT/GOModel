package core

// IngressFrame is the immutable transport-level capture of an inbound request.
// It preserves the request as received at the HTTP boundary so later stages can
// extract semantics without losing fidelity.
type IngressFrame struct {
	// Method is the inbound HTTP method.
	Method string
	// Path is the request URL path as received at ingress.
	Path string
	// RouteParams contains resolved router parameters such as provider or file id.
	RouteParams map[string]string
	// QueryParams contains the raw query string values by key.
	QueryParams map[string][]string
	// Headers contains the inbound HTTP headers exactly as captured at ingress.
	Headers map[string][]string
	// ContentType is the inbound Content-Type header value.
	ContentType string
	// RawBody contains the captured request body bytes when the body fit within
	// the ingress capture limit.
	RawBody []byte
	// RawBodyTooLarge reports that the request body exceeded the capture limit,
	// so RawBody is omitted and the live body stream remains on the request.
	RawBodyTooLarge bool
	// RequestID is the canonical request id propagated through context, headers,
	// providers, and audit records for this request.
	RequestID string
	// TraceMetadata contains tracing-related key/value pairs such as trace/span
	// ids or baggage/sampling metadata derived from tracing headers.
	TraceMetadata map[string]string
}
