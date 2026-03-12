package core

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// RoutingHints holds minimal routing-relevant request hints derived from the
// transport snapshot.
//
// These hints are intentionally smaller than a full semantic interpretation.
//
// Lifecycle:
//   - DeriveRequestSemantics seeds these values directly from transport/body data.
//   - Canonical JSON decode may refine them from a cached request object.
//   - NormalizeModelSelector canonicalizes model/provider values in place.
//
// Consumers that require canonical selector state should prefer a cached canonical
// request or call NormalizeModelSelector before relying on these fields.
type RoutingHints struct {
	Model    string
	Provider string
	Endpoint string
}

type semanticCacheKey string

const (
	semanticChatRequestKey      semanticCacheKey = "chat_request"
	semanticResponsesRequestKey semanticCacheKey = "responses_request"
	semanticEmbeddingRequestKey semanticCacheKey = "embedding_request"
	semanticBatchRequestKey     semanticCacheKey = "batch_request"
	semanticBatchMetadataKey    semanticCacheKey = "batch_metadata"
	semanticFileRequestKey      semanticCacheKey = "file_request"
)

// RequestSemantics is the gateway's best-effort semantic extraction from the
// transport snapshot.
// It may be partial and should not be treated as authoritative transport state.
//
// The semantics are populated incrementally:
//   - transport seeds RouteKind/OperationKind plus sparse RoutingHints
//   - route-specific metadata may be cached on demand
//   - canonical request decode may cache a parsed request and refine RoutingHints
//   - NormalizeModelSelector may rewrite selector hints into canonical form
type RequestSemantics struct {
	RouteKind    string
	OperationKind string
	RoutingHints RoutingHints
	// BodyParsedAsJSON reports that the captured request body was parsed as JSON
	// (for selector peeking and/or canonical request decode).
	BodyParsedAsJSON bool

	cache map[semanticCacheKey]any
}

// CachedChatRequest returns the cached canonical chat request, if present.
func (env *RequestSemantics) CachedChatRequest() *ChatRequest {
	req, _ := cachedSemanticValue[*ChatRequest](env, semanticChatRequestKey)
	return req
}

// CachedResponsesRequest returns the cached canonical responses request, if present.
func (env *RequestSemantics) CachedResponsesRequest() *ResponsesRequest {
	req, _ := cachedSemanticValue[*ResponsesRequest](env, semanticResponsesRequestKey)
	return req
}

// CachedEmbeddingRequest returns the cached canonical embeddings request, if present.
func (env *RequestSemantics) CachedEmbeddingRequest() *EmbeddingRequest {
	req, _ := cachedSemanticValue[*EmbeddingRequest](env, semanticEmbeddingRequestKey)
	return req
}

// CachedBatchRequest returns the cached canonical batch create request, if present.
func (env *RequestSemantics) CachedBatchRequest() *BatchRequest {
	req, _ := cachedSemanticValue[*BatchRequest](env, semanticBatchRequestKey)
	return req
}

// CachedBatchRouteInfo returns cached sparse batch route info, if present.
func (env *RequestSemantics) CachedBatchRouteInfo() *BatchRouteInfo {
	req, _ := cachedSemanticValue[*BatchRouteInfo](env, semanticBatchMetadataKey)
	return req
}

// CachedFileRouteInfo returns cached sparse file route info, if present.
func (env *RequestSemantics) CachedFileRouteInfo() *FileRouteInfo {
	req, _ := cachedSemanticValue[*FileRouteInfo](env, semanticFileRequestKey)
	return req
}

// CanonicalSelectorFromCachedRequest returns model/provider selector hints from
// any cached canonical JSON request for the current operation kind.
func (env *RequestSemantics) CanonicalSelectorFromCachedRequest() (model, provider string, ok bool) {
	if env == nil {
		return "", "", false
	}
	codec, ok := canonicalOperationCodecFor(env.OperationKind)
	if !ok {
		return "", "", false
	}
	req, ok := cachedSemanticAny(env, codec.key)
	if !ok {
		return "", "", false
	}
	return semanticSelectorFromCanonicalRequest(req)
}

func (env *RequestSemantics) cacheValue(key semanticCacheKey, value any) {
	if env == nil || value == nil {
		return
	}
	if env.cache == nil {
		env.cache = make(map[semanticCacheKey]any, 4)
	}
	env.cache[key] = value
}

func cachedSemanticValue[T any](env *RequestSemantics, key semanticCacheKey) (T, bool) {
	var zero T
	if env == nil || env.cache == nil {
		return zero, false
	}
	value, ok := env.cache[key]
	if !ok {
		return zero, false
	}
	typed, ok := value.(T)
	if !ok {
		return zero, false
	}
	return typed, true
}

func cachedSemanticAny(env *RequestSemantics, key semanticCacheKey) (any, bool) {
	if env == nil || env.cache == nil {
		return nil, false
	}
	value, ok := env.cache[key]
	return value, ok
}

func cacheBatchRouteMetadata(env *RequestSemantics, req *BatchRouteInfo) {
	if env == nil || req == nil {
		return
	}
	env.cacheValue(semanticBatchMetadataKey, req)
}

// CacheFileRouteInfo stores sparse file route metadata on the request semantics.
func CacheFileRouteInfo(env *RequestSemantics, req *FileRouteInfo) {
	if env == nil || req == nil {
		return
	}
	env.cacheValue(semanticFileRequestKey, req)
	if req.Provider != "" && env.RoutingHints.Provider == "" {
		env.RoutingHints.Provider = req.Provider
	}
}

// DeriveRequestSemantics derives best-effort request semantics from the captured
// transport snapshot.
// Unknown or invalid bodies are tolerated; the returned envelope may be partial.
func DeriveRequestSemantics(snapshot *RequestSnapshot) *RequestSemantics {
	if snapshot == nil {
		return nil
	}

	env := &RequestSemantics{
		RoutingHints: RoutingHints{
			Endpoint: snapshot.Path,
		},
	}

	desc := DescribeEndpointPath(snapshot.Path)
	if desc.Operation == "" {
		return nil
	}
	env.RouteKind = desc.Dialect
	env.OperationKind = desc.Operation

	if env.OperationKind == "files" {
		CacheFileRouteInfo(env, DeriveFileRouteInfoFromTransport(snapshot.Method, snapshot.Path, snapshot.routeParams, snapshot.queryParams))
	}
	if env.OperationKind == "batches" {
		cacheBatchRouteMetadata(env, DeriveBatchRouteInfoFromTransport(snapshot.Method, snapshot.Path, snapshot.routeParams, snapshot.queryParams))
	}

	if env.RouteKind == "provider_passthrough" {
		env.RoutingHints.Endpoint = ""
		if provider := snapshot.routeParams["provider"]; provider != "" {
			env.RoutingHints.Provider = provider
		}
		if endpoint := snapshot.routeParams["endpoint"]; endpoint != "" {
			env.RoutingHints.Endpoint = endpoint
		}
		if env.RoutingHints.Provider == "" || env.RoutingHints.Endpoint == "" {
			if provider, endpoint, ok := ParseProviderPassthroughPath(snapshot.Path); ok {
				if env.RoutingHints.Provider == "" {
					env.RoutingHints.Provider = provider
				}
				if env.RoutingHints.Endpoint == "" {
					env.RoutingHints.Endpoint = endpoint
				}
			}
		}
	}

	if snapshot.capturedBody == nil {
		return env
	}

	trimmed := bytes.TrimSpace(snapshot.capturedBody)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return env
	}

	var selectors struct {
		Model    string `json:"model"`
		Provider string `json:"provider"`
	}
	if err := json.Unmarshal(trimmed, &selectors); err != nil {
		return env
	}
	env.BodyParsedAsJSON = true

	env.RoutingHints.Model = selectors.Model
	if env.RoutingHints.Provider == "" {
		env.RoutingHints.Provider = selectors.Provider
	}

	return env
}

// DeriveFileRouteInfoFromTransport derives sparse file route info from transport metadata.
func DeriveFileRouteInfoFromTransport(method, path string, routeParams map[string]string, queryParams map[string][]string) *FileRouteInfo {
	req := &FileRouteInfo{
		Action:   fileActionFromIngress(method, path),
		Provider: firstTransportValue(queryParams, "provider"),
		Purpose:  firstTransportValue(queryParams, "purpose"),
		After:    firstTransportValue(queryParams, "after"),
		LimitRaw: firstTransportValue(queryParams, "limit"),
		FileID:   fileIDFromTransport(path, routeParams),
	}
	if req.LimitRaw != "" {
		if parsed, err := strconv.Atoi(req.LimitRaw); err == nil {
			req.Limit = parsed
			req.HasLimit = true
		}
	}
	if req.Action == "" && req.Provider == "" && req.Purpose == "" && req.After == "" && req.LimitRaw == "" && req.FileID == "" {
		return nil
	}
	return req
}

// DeriveBatchRouteInfoFromTransport derives sparse batch route info from transport metadata.
func DeriveBatchRouteInfoFromTransport(method, path string, routeParams map[string]string, queryParams map[string][]string) *BatchRouteInfo {
	req := &BatchRouteInfo{
		Action:   batchActionFromIngress(method, path),
		BatchID:  batchIDFromTransport(path, routeParams),
		After:    firstTransportValue(queryParams, "after"),
		LimitRaw: firstTransportValue(queryParams, "limit"),
	}
	if req.LimitRaw != "" {
		if parsed, err := strconv.Atoi(req.LimitRaw); err == nil {
			req.Limit = parsed
			req.HasLimit = true
		}
	}
	if req.Action == "" && req.BatchID == "" && req.After == "" && req.LimitRaw == "" {
		return nil
	}
	return req
}

func fileActionFromIngress(method, path string) string {
	switch {
	case path == "/v1/files" && method == http.MethodPost:
		return FileActionCreate
	case path == "/v1/files" && method == http.MethodGet:
		return FileActionList
	case strings.HasSuffix(path, "/content") && method == http.MethodGet:
		return FileActionContent
	case strings.HasPrefix(path, "/v1/files/") && method == http.MethodGet:
		return FileActionGet
	case strings.HasPrefix(path, "/v1/files/") && method == http.MethodDelete:
		return FileActionDelete
	default:
		return ""
	}
}

func fileIDFromTransport(path string, routeParams map[string]string) string {
	if id := strings.TrimSpace(routeParams["id"]); id != "" {
		return id
	}

	trimmed := strings.Trim(strings.TrimSpace(path), "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 3 || parts[0] != "v1" || parts[1] != "files" {
		return ""
	}
	return strings.TrimSpace(parts[2])
}

func batchActionFromIngress(method, path string) string {
	switch {
	case path == "/v1/batches" && method == http.MethodPost:
		return BatchActionCreate
	case path == "/v1/batches" && method == http.MethodGet:
		return BatchActionList
	case strings.HasSuffix(path, "/results") && strings.HasPrefix(path, "/v1/batches/") && method == http.MethodGet:
		return BatchActionResults
	case strings.HasSuffix(path, "/cancel") && strings.HasPrefix(path, "/v1/batches/") && method == http.MethodPost:
		return BatchActionCancel
	case strings.HasPrefix(path, "/v1/batches/") && method == http.MethodGet:
		return BatchActionGet
	default:
		return ""
	}
}

func batchIDFromTransport(path string, routeParams map[string]string) string {
	if id := strings.TrimSpace(routeParams["id"]); id != "" {
		return id
	}

	trimmed := strings.Trim(strings.TrimSpace(path), "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 3 || parts[0] != "v1" || parts[1] != "batches" {
		return ""
	}
	return strings.TrimSpace(parts[2])
}

func firstTransportValue(values map[string][]string, key string) string {
	if len(values) == 0 {
		return ""
	}
	items, ok := values[key]
	if !ok || len(items) == 0 {
		return ""
	}
	return strings.TrimSpace(items[0])
}
