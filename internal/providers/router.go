// Package providers provides a router for multiple LLM providers.
package providers

import (
	"context"
	"fmt"
	"io"

	"gomodel/internal/core"
)

// ErrRegistryNotInitialized is returned when the router is used before the registry has any models.
var ErrRegistryNotInitialized = fmt.Errorf("model registry has no models: ensure Initialize() or LoadFromCache() is called before using the router")

// Router routes requests to the appropriate provider based on the model lookup.
// It uses a dynamic model-to-provider mapping that is populated at startup
// by fetching available models from each provider's /models endpoint.
type Router struct {
	lookup core.ModelLookup
}

// NewRouter creates a new provider router with a model lookup.
// The lookup must be initialized (via Initialize() or LoadFromCache()) before using the router.
// Returns an error if the lookup is nil.
func NewRouter(lookup core.ModelLookup) (*Router, error) {
	if lookup == nil {
		return nil, fmt.Errorf("lookup cannot be nil")
	}
	return &Router{
		lookup: lookup,
	}, nil
}

// checkReady verifies the lookup has models available.
// Returns ErrRegistryNotInitialized if no models are loaded.
func (r *Router) checkReady() error {
	if r.lookup.ModelCount() == 0 {
		return ErrRegistryNotInitialized
	}
	return nil
}

// Supports returns true if any provider supports the given model.
// Returns false if the lookup has no models loaded.
func (r *Router) Supports(model string) bool {
	if r.lookup.ModelCount() == 0 {
		return false
	}
	return r.lookup.Supports(model)
}

// ChatCompletion routes the request to the appropriate provider.
// Returns ErrRegistryNotInitialized if the lookup has no models loaded.
func (r *Router) ChatCompletion(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
	if err := r.checkReady(); err != nil {
		return nil, err
	}
	provider := r.lookup.GetProvider(req.Model)
	if provider == nil {
		return nil, fmt.Errorf("no provider found for model: %s", req.Model)
	}
	resp, err := provider.ChatCompletion(ctx, req)
	if err == nil && resp != nil {
		resp.Provider = r.GetProviderType(req.Model)
	}
	return resp, err
}

// StreamChatCompletion routes the streaming request to the appropriate provider.
// Returns ErrRegistryNotInitialized if the lookup has no models loaded.
func (r *Router) StreamChatCompletion(ctx context.Context, req *core.ChatRequest) (io.ReadCloser, error) {
	if err := r.checkReady(); err != nil {
		return nil, err
	}
	provider := r.lookup.GetProvider(req.Model)
	if provider == nil {
		return nil, fmt.Errorf("no provider found for model: %s", req.Model)
	}
	return provider.StreamChatCompletion(ctx, req)
}

// ListModels returns all models from the lookup.
// Returns ErrRegistryNotInitialized if the lookup has no models loaded.
func (r *Router) ListModels(_ context.Context) (*core.ModelsResponse, error) {
	if err := r.checkReady(); err != nil {
		return nil, err
	}
	models := r.lookup.ListModels()
	return &core.ModelsResponse{
		Object: "list",
		Data:   models,
	}, nil
}

// Responses routes the Responses API request to the appropriate provider.
// Returns ErrRegistryNotInitialized if the lookup has no models loaded.
func (r *Router) Responses(ctx context.Context, req *core.ResponsesRequest) (*core.ResponsesResponse, error) {
	if err := r.checkReady(); err != nil {
		return nil, err
	}
	provider := r.lookup.GetProvider(req.Model)
	if provider == nil {
		return nil, fmt.Errorf("no provider found for model: %s", req.Model)
	}
	resp, err := provider.Responses(ctx, req)
	if err == nil && resp != nil {
		resp.Provider = r.GetProviderType(req.Model)
	}
	return resp, err
}

// StreamResponses routes the streaming Responses API request to the appropriate provider.
// Returns ErrRegistryNotInitialized if the lookup has no models loaded.
func (r *Router) StreamResponses(ctx context.Context, req *core.ResponsesRequest) (io.ReadCloser, error) {
	if err := r.checkReady(); err != nil {
		return nil, err
	}
	provider := r.lookup.GetProvider(req.Model)
	if provider == nil {
		return nil, fmt.Errorf("no provider found for model: %s", req.Model)
	}
	return provider.StreamResponses(ctx, req)
}

// Embeddings routes the embeddings request to the appropriate provider.
func (r *Router) Embeddings(ctx context.Context, req *core.EmbeddingRequest) (*core.EmbeddingResponse, error) {
	if err := r.checkReady(); err != nil {
		return nil, err
	}
	provider := r.lookup.GetProvider(req.Model)
	if provider == nil {
		return nil, fmt.Errorf("no provider found for model: %s", req.Model)
	}
	resp, err := provider.Embeddings(ctx, req)
	if err == nil && resp != nil {
		resp.Provider = r.GetProviderType(req.Model)
	}
	return resp, err
}

// GetProviderType returns the provider type string for the given model.
// Returns empty string if the model is not found.
func (r *Router) GetProviderType(model string) string {
	return r.lookup.GetProviderType(model)
}

func (r *Router) providerByType(providerType string) core.Provider {
	models := r.lookup.ListModels()
	for _, model := range models {
		if r.lookup.GetProviderType(model.ID) != providerType {
			continue
		}
		p := r.lookup.GetProvider(model.ID)
		if p != nil {
			return p
		}
	}
	return nil
}

// CreateBatch routes native batch creation to a provider type.
func (r *Router) CreateBatch(ctx context.Context, providerType string, req *core.BatchRequest) (*core.BatchResponse, error) {
	if err := r.checkReady(); err != nil {
		return nil, err
	}
	if providerType == "" {
		return nil, core.NewInvalidRequestError("provider type is required", nil)
	}
	provider := r.providerByType(providerType)
	if provider == nil {
		return nil, core.NewInvalidRequestError(fmt.Sprintf("no provider found for provider type: %s", providerType), nil)
	}
	bp, ok := provider.(core.NativeBatchProvider)
	if !ok {
		return nil, core.NewInvalidRequestError(fmt.Sprintf("%s does not support native batch processing", providerType), nil)
	}
	resp, err := bp.CreateBatch(ctx, req)
	if err == nil && resp != nil {
		resp.Provider = providerType
	}
	return resp, err
}

// GetBatch routes native batch lookup to a provider type.
func (r *Router) GetBatch(ctx context.Context, providerType, id string) (*core.BatchResponse, error) {
	if err := r.checkReady(); err != nil {
		return nil, err
	}
	if providerType == "" {
		return nil, core.NewInvalidRequestError("provider type is required", nil)
	}
	provider := r.providerByType(providerType)
	if provider == nil {
		return nil, core.NewInvalidRequestError(fmt.Sprintf("no provider found for provider type: %s", providerType), nil)
	}
	bp, ok := provider.(core.NativeBatchProvider)
	if !ok {
		return nil, core.NewInvalidRequestError(fmt.Sprintf("%s does not support native batch processing", providerType), nil)
	}
	resp, err := bp.GetBatch(ctx, id)
	if err == nil && resp != nil {
		resp.Provider = providerType
	}
	return resp, err
}

// ListBatches routes native batch listing to a provider type.
func (r *Router) ListBatches(ctx context.Context, providerType string, limit int, after string) (*core.BatchListResponse, error) {
	if err := r.checkReady(); err != nil {
		return nil, err
	}
	if providerType == "" {
		return nil, core.NewInvalidRequestError("provider type is required", nil)
	}
	provider := r.providerByType(providerType)
	if provider == nil {
		return nil, core.NewInvalidRequestError(fmt.Sprintf("no provider found for provider type: %s", providerType), nil)
	}
	bp, ok := provider.(core.NativeBatchProvider)
	if !ok {
		return nil, core.NewInvalidRequestError(fmt.Sprintf("%s does not support native batch processing", providerType), nil)
	}
	return bp.ListBatches(ctx, limit, after)
}

// CancelBatch routes native batch cancellation to a provider type.
func (r *Router) CancelBatch(ctx context.Context, providerType, id string) (*core.BatchResponse, error) {
	if err := r.checkReady(); err != nil {
		return nil, err
	}
	if providerType == "" {
		return nil, core.NewInvalidRequestError("provider type is required", nil)
	}
	provider := r.providerByType(providerType)
	if provider == nil {
		return nil, core.NewInvalidRequestError(fmt.Sprintf("no provider found for provider type: %s", providerType), nil)
	}
	bp, ok := provider.(core.NativeBatchProvider)
	if !ok {
		return nil, core.NewInvalidRequestError(fmt.Sprintf("%s does not support native batch processing", providerType), nil)
	}
	resp, err := bp.CancelBatch(ctx, id)
	if err == nil && resp != nil {
		resp.Provider = providerType
	}
	return resp, err
}

// GetBatchResults routes native batch results lookup to a provider type.
func (r *Router) GetBatchResults(ctx context.Context, providerType, id string) (*core.BatchResultsResponse, error) {
	if err := r.checkReady(); err != nil {
		return nil, err
	}
	if providerType == "" {
		return nil, core.NewInvalidRequestError("provider type is required", nil)
	}
	provider := r.providerByType(providerType)
	if provider == nil {
		return nil, core.NewInvalidRequestError(fmt.Sprintf("no provider found for provider type: %s", providerType), nil)
	}
	bp, ok := provider.(core.NativeBatchProvider)
	if !ok {
		return nil, core.NewInvalidRequestError(fmt.Sprintf("%s does not support native batch processing", providerType), nil)
	}
	return bp.GetBatchResults(ctx, id)
}

// CreateFile routes file upload to a provider type.
func (r *Router) CreateFile(ctx context.Context, providerType string, req *core.FileCreateRequest) (*core.FileObject, error) {
	if err := r.checkReady(); err != nil {
		return nil, err
	}
	if providerType == "" {
		return nil, core.NewInvalidRequestError("provider type is required", nil)
	}
	provider := r.providerByType(providerType)
	if provider == nil {
		return nil, core.NewInvalidRequestError(fmt.Sprintf("no provider found for provider type: %s", providerType), nil)
	}
	fp, ok := provider.(core.NativeFileProvider)
	if !ok {
		return nil, core.NewInvalidRequestError(fmt.Sprintf("%s does not support native file operations", providerType), nil)
	}
	resp, err := fp.CreateFile(ctx, req)
	if err == nil && resp != nil {
		resp.Provider = providerType
	}
	return resp, err
}

// ListFiles routes file listing to a provider type.
func (r *Router) ListFiles(ctx context.Context, providerType, purpose string, limit int, after string) (*core.FileListResponse, error) {
	if err := r.checkReady(); err != nil {
		return nil, err
	}
	if providerType == "" {
		return nil, core.NewInvalidRequestError("provider type is required", nil)
	}
	provider := r.providerByType(providerType)
	if provider == nil {
		return nil, core.NewInvalidRequestError(fmt.Sprintf("no provider found for provider type: %s", providerType), nil)
	}
	fp, ok := provider.(core.NativeFileProvider)
	if !ok {
		return nil, core.NewInvalidRequestError(fmt.Sprintf("%s does not support native file operations", providerType), nil)
	}
	resp, err := fp.ListFiles(ctx, purpose, limit, after)
	if err != nil {
		return nil, err
	}
	if resp != nil {
		for i := range resp.Data {
			resp.Data[i].Provider = providerType
		}
	}
	return resp, nil
}

// GetFile routes file retrieval to a provider type.
func (r *Router) GetFile(ctx context.Context, providerType, id string) (*core.FileObject, error) {
	if err := r.checkReady(); err != nil {
		return nil, err
	}
	if providerType == "" {
		return nil, core.NewInvalidRequestError("provider type is required", nil)
	}
	provider := r.providerByType(providerType)
	if provider == nil {
		return nil, core.NewInvalidRequestError(fmt.Sprintf("no provider found for provider type: %s", providerType), nil)
	}
	fp, ok := provider.(core.NativeFileProvider)
	if !ok {
		return nil, core.NewInvalidRequestError(fmt.Sprintf("%s does not support native file operations", providerType), nil)
	}
	resp, err := fp.GetFile(ctx, id)
	if err == nil && resp != nil {
		resp.Provider = providerType
	}
	return resp, err
}

// DeleteFile routes file deletion to a provider type.
func (r *Router) DeleteFile(ctx context.Context, providerType, id string) (*core.FileDeleteResponse, error) {
	if err := r.checkReady(); err != nil {
		return nil, err
	}
	if providerType == "" {
		return nil, core.NewInvalidRequestError("provider type is required", nil)
	}
	provider := r.providerByType(providerType)
	if provider == nil {
		return nil, core.NewInvalidRequestError(fmt.Sprintf("no provider found for provider type: %s", providerType), nil)
	}
	fp, ok := provider.(core.NativeFileProvider)
	if !ok {
		return nil, core.NewInvalidRequestError(fmt.Sprintf("%s does not support native file operations", providerType), nil)
	}
	return fp.DeleteFile(ctx, id)
}

// GetFileContent routes file content retrieval to a provider type.
func (r *Router) GetFileContent(ctx context.Context, providerType, id string) (*core.FileContentResponse, error) {
	if err := r.checkReady(); err != nil {
		return nil, err
	}
	if providerType == "" {
		return nil, core.NewInvalidRequestError("provider type is required", nil)
	}
	provider := r.providerByType(providerType)
	if provider == nil {
		return nil, core.NewInvalidRequestError(fmt.Sprintf("no provider found for provider type: %s", providerType), nil)
	}
	fp, ok := provider.(core.NativeFileProvider)
	if !ok {
		return nil, core.NewInvalidRequestError(fmt.Sprintf("%s does not support native file operations", providerType), nil)
	}
	return fp.GetFileContent(ctx, id)
}
