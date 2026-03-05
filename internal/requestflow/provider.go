package requestflow

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"gomodel/internal/core"
	"gomodel/internal/guardrails"
)

// Provider applies request flow plans before delegating to the router.
type Provider struct {
	inner   core.RoutableProvider
	manager *Manager
}

// NewProvider wraps the routed provider with request flow planning.
func NewProvider(inner core.RoutableProvider, manager *Manager) *Provider {
	return &Provider{inner: inner, manager: manager}
}

// Supports delegates to the inner provider.
func (p *Provider) Supports(model string) bool { return p.inner.Supports(model) }

// GetProviderType delegates to the inner provider.
func (p *Provider) GetProviderType(model string) string { return p.inner.GetProviderType(model) }

// ListModels delegates directly.
func (p *Provider) ListModels(ctx context.Context) (*core.ModelsResponse, error) {
	return p.inner.ListModels(ctx)
}

// ChatCompletion applies a resolved request flow to chat requests.
func (p *Provider) ChatCompletion(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
	planCtx := p.resolveContext(ctx, "/v1/chat/completions", req.Model, req.Provider)
	plan, err := p.manager.Resolve(planCtx)
	if err != nil {
		return nil, err
	}
	execCtx := p.withExecution(ctx, req.Model, "/v1/chat/completions", plan)
	guardedReq, err := applyPlanToChat(execCtx, req, plan)
	if err != nil {
		entry := Finish(execCtx, p.providerType(req.Model, req.Provider), "guardrail_rejected", err)
		if entry != nil {
			entry.GuardrailRejected = true
			p.manager.LogExecution(entry)
		}
		return nil, err
	}
	resp, err := p.inner.ChatCompletion(execCtx, guardedReq)
	providerType := p.providerType(req.Model, req.Provider)
	if resp != nil && strings.TrimSpace(resp.Provider) != "" {
		providerType = resp.Provider
	}
	status := "success"
	if err != nil {
		status = "error"
	}
	p.manager.LogExecution(Finish(execCtx, providerType, status, err))
	return resp, err
}

// StreamChatCompletion applies a resolved request flow to streaming chat requests.
func (p *Provider) StreamChatCompletion(ctx context.Context, req *core.ChatRequest) (io.ReadCloser, error) {
	planCtx := p.resolveContext(ctx, "/v1/chat/completions", req.Model, req.Provider)
	plan, err := p.manager.Resolve(planCtx)
	if err != nil {
		return nil, err
	}
	execCtx := p.withExecution(ctx, req.Model, "/v1/chat/completions", plan)
	guardedReq, err := applyPlanToChat(execCtx, req, plan)
	if err != nil {
		entry := Finish(execCtx, p.providerType(req.Model, req.Provider), "guardrail_rejected", err)
		if entry != nil {
			entry.GuardrailRejected = true
			p.manager.LogExecution(entry)
		}
		return nil, err
	}
	stream, err := p.inner.StreamChatCompletion(execCtx, guardedReq)
	status := "stream_started"
	if err != nil {
		status = "error"
	}
	p.manager.LogExecution(Finish(execCtx, p.providerType(req.Model, req.Provider), status, err))
	return stream, err
}

// Responses applies a resolved request flow to responses requests.
func (p *Provider) Responses(ctx context.Context, req *core.ResponsesRequest) (*core.ResponsesResponse, error) {
	planCtx := p.resolveContext(ctx, "/v1/responses", req.Model, req.Provider)
	plan, err := p.manager.Resolve(planCtx)
	if err != nil {
		return nil, err
	}
	execCtx := p.withExecution(ctx, req.Model, "/v1/responses", plan)
	guardedReq, err := applyPlanToResponses(execCtx, req, plan)
	if err != nil {
		entry := Finish(execCtx, p.providerType(req.Model, req.Provider), "guardrail_rejected", err)
		if entry != nil {
			entry.GuardrailRejected = true
			p.manager.LogExecution(entry)
		}
		return nil, err
	}
	resp, err := p.inner.Responses(execCtx, guardedReq)
	providerType := p.providerType(req.Model, req.Provider)
	if resp != nil && strings.TrimSpace(resp.Provider) != "" {
		providerType = resp.Provider
	}
	status := "success"
	if err != nil {
		status = "error"
	}
	p.manager.LogExecution(Finish(execCtx, providerType, status, err))
	return resp, err
}

// StreamResponses applies a resolved request flow to streaming responses requests.
func (p *Provider) StreamResponses(ctx context.Context, req *core.ResponsesRequest) (io.ReadCloser, error) {
	planCtx := p.resolveContext(ctx, "/v1/responses", req.Model, req.Provider)
	plan, err := p.manager.Resolve(planCtx)
	if err != nil {
		return nil, err
	}
	execCtx := p.withExecution(ctx, req.Model, "/v1/responses", plan)
	guardedReq, err := applyPlanToResponses(execCtx, req, plan)
	if err != nil {
		entry := Finish(execCtx, p.providerType(req.Model, req.Provider), "guardrail_rejected", err)
		if entry != nil {
			entry.GuardrailRejected = true
			p.manager.LogExecution(entry)
		}
		return nil, err
	}
	stream, err := p.inner.StreamResponses(execCtx, guardedReq)
	status := "stream_started"
	if err != nil {
		status = "error"
	}
	p.manager.LogExecution(Finish(execCtx, p.providerType(req.Model, req.Provider), status, err))
	return stream, err
}

// Embeddings applies retry/failover planning without guardrails.
func (p *Provider) Embeddings(ctx context.Context, req *core.EmbeddingRequest) (*core.EmbeddingResponse, error) {
	planCtx := p.resolveContext(ctx, "/v1/embeddings", req.Model, req.Provider)
	plan, err := p.manager.Resolve(planCtx)
	if err != nil {
		return nil, err
	}
	execCtx := p.withExecution(ctx, req.Model, "/v1/embeddings", plan)
	resp, err := p.inner.Embeddings(execCtx, req)
	providerType := p.providerType(req.Model, req.Provider)
	if resp != nil && strings.TrimSpace(resp.Provider) != "" {
		providerType = resp.Provider
	}
	status := "success"
	if err != nil {
		status = "error"
	}
	p.manager.LogExecution(Finish(execCtx, providerType, status, err))
	return resp, err
}

func (p *Provider) resolveContext(ctx context.Context, endpoint, model, provider string) ResolveContext {
	selector := SelectorFromContext(ctx)
	return ResolveContext{
		Endpoint:   endpoint,
		Model:      model,
		Provider:   provider,
		APIKeyHash: selector.APIKeyHash,
		Team:       selector.Team,
		User:       selector.User,
	}
}

func (p *Provider) withExecution(ctx context.Context, model, endpoint string, plan *ResolvedPlan) context.Context {
	planIDs := make([]string, 0, len(plan.Matches))
	planSources := make([]string, 0, len(plan.Matches))
	planNames := make([]string, 0, len(plan.Matches))
	for _, match := range plan.Matches {
		planIDs = append(planIDs, match.ID)
		planSources = append(planSources, match.Source)
		planNames = append(planNames, match.Name)
	}
	guardrailNames := make([]string, 0, len(plan.Guardrails))
	for _, rule := range plan.Guardrails {
		guardrailNames = append(guardrailNames, rule.Name)
	}
	exec := &Execution{
		ID:                   uuid.NewString(),
		RequestID:            core.GetRequestID(ctx),
		Timestamp:            time.Now().UTC(),
		Endpoint:             endpoint,
		Model:                model,
		PlanName:             strings.Join(planNames, " -> "),
		PlanIDs:              planIDs,
		PlanSources:          planSources,
		GuardrailsConfigured: guardrailNames,
		RetryConfigured:      plan.Retry.MaxRetries,
		FailoverEnabled:      plan.FailoverEnabled,
		FailoverUsed:         false,
		Status:               "pending",
	}
	return WithExecutionState(ctx, exec, plan.Retry)
}

func (p *Provider) providerType(model, provider string) string {
	qualified := model
	if provider != "" {
		qualified = provider + "/" + model
	}
	providerType := p.inner.GetProviderType(qualified)
	if providerType == "" {
		providerType = p.inner.GetProviderType(model)
	}
	return providerType
}

func applyPlanToChat(ctx context.Context, req *core.ChatRequest, plan *ResolvedPlan) (*core.ChatRequest, error) {
	if plan == nil || plan.pipeline == nil || plan.pipeline.Len() == 0 {
		return req, nil
	}
	msgs := make([]guardrails.Message, len(req.Messages))
	applied := make([]string, 0, len(plan.Guardrails))
	for i, msg := range req.Messages {
		msgs[i] = guardrails.Message{Role: msg.Role, Content: msg.Content}
	}
	for _, rule := range plan.Guardrails {
		applied = append(applied, rule.Name)
	}
	RecordGuardrailsApplied(ctx, applied)
	modified, err := plan.pipeline.Process(ctx, msgs)
	if err != nil {
		return nil, err
	}
	result := *req
	result.Messages = make([]core.Message, len(modified))
	for i, msg := range modified {
		result.Messages[i] = core.Message{Role: msg.Role, Content: msg.Content}
	}
	return &result, nil
}

func applyPlanToResponses(ctx context.Context, req *core.ResponsesRequest, plan *ResolvedPlan) (*core.ResponsesRequest, error) {
	if plan == nil || plan.pipeline == nil || plan.pipeline.Len() == 0 {
		return req, nil
	}
	msgs := make([]guardrails.Message, 0, 1)
	if req.Instructions != "" {
		msgs = append(msgs, guardrails.Message{Role: "system", Content: req.Instructions})
	}
	applied := make([]string, 0, len(plan.Guardrails))
	for _, rule := range plan.Guardrails {
		applied = append(applied, rule.Name)
	}
	RecordGuardrailsApplied(ctx, applied)
	modified, err := plan.pipeline.Process(ctx, msgs)
	if err != nil {
		return nil, err
	}
	result := *req
	result.Instructions = ""
	for _, msg := range modified {
		if msg.Role != "system" {
			continue
		}
		if result.Instructions != "" {
			result.Instructions += "\n"
		}
		result.Instructions += msg.Content
	}
	return &result, nil
}

func (p *Provider) nativeBatchRouter() (core.NativeBatchRoutableProvider, bool) {
	router, ok := p.inner.(core.NativeBatchRoutableProvider)
	return router, ok
}

func (p *Provider) nativeFileRouter() (core.NativeFileRoutableProvider, bool) {
	router, ok := p.inner.(core.NativeFileRoutableProvider)
	return router, ok
}

// CreateBatch delegates native batch calls to the inner router.
func (p *Provider) CreateBatch(ctx context.Context, providerType string, req *core.BatchRequest) (*core.BatchResponse, error) {
	router, ok := p.nativeBatchRouter()
	if !ok {
		return nil, core.NewInvalidRequestError("batch routing is not supported by the current provider router", nil)
	}
	return router.CreateBatch(ctx, providerType, req)
}

// GetBatch delegates native batch calls to the inner router.
func (p *Provider) GetBatch(ctx context.Context, providerType, id string) (*core.BatchResponse, error) {
	router, ok := p.nativeBatchRouter()
	if !ok {
		return nil, core.NewInvalidRequestError("batch routing is not supported by the current provider router", nil)
	}
	return router.GetBatch(ctx, providerType, id)
}

// ListBatches delegates native batch calls to the inner router.
func (p *Provider) ListBatches(ctx context.Context, providerType string, limit int, after string) (*core.BatchListResponse, error) {
	router, ok := p.nativeBatchRouter()
	if !ok {
		return nil, core.NewInvalidRequestError("batch routing is not supported by the current provider router", nil)
	}
	return router.ListBatches(ctx, providerType, limit, after)
}

// CancelBatch delegates native batch calls to the inner router.
func (p *Provider) CancelBatch(ctx context.Context, providerType, id string) (*core.BatchResponse, error) {
	router, ok := p.nativeBatchRouter()
	if !ok {
		return nil, core.NewInvalidRequestError("batch routing is not supported by the current provider router", nil)
	}
	return router.CancelBatch(ctx, providerType, id)
}

// GetBatchResults delegates native batch calls to the inner router.
func (p *Provider) GetBatchResults(ctx context.Context, providerType, id string) (*core.BatchResultsResponse, error) {
	router, ok := p.nativeBatchRouter()
	if !ok {
		return nil, core.NewInvalidRequestError("batch routing is not supported by the current provider router", nil)
	}
	return router.GetBatchResults(ctx, providerType, id)
}

// CreateFile delegates native file calls to the inner router.
func (p *Provider) CreateFile(ctx context.Context, providerType string, req *core.FileCreateRequest) (*core.FileObject, error) {
	router, ok := p.nativeFileRouter()
	if !ok {
		return nil, core.NewInvalidRequestError("file routing is not supported by the current provider router", nil)
	}
	return router.CreateFile(ctx, providerType, req)
}

// ListFiles delegates native file calls to the inner router.
func (p *Provider) ListFiles(ctx context.Context, providerType, purpose string, limit int, after string) (*core.FileListResponse, error) {
	router, ok := p.nativeFileRouter()
	if !ok {
		return nil, core.NewInvalidRequestError("file routing is not supported by the current provider router", nil)
	}
	return router.ListFiles(ctx, providerType, purpose, limit, after)
}

// GetFile delegates native file calls to the inner router.
func (p *Provider) GetFile(ctx context.Context, providerType, id string) (*core.FileObject, error) {
	router, ok := p.nativeFileRouter()
	if !ok {
		return nil, core.NewInvalidRequestError("file routing is not supported by the current provider router", nil)
	}
	return router.GetFile(ctx, providerType, id)
}

// DeleteFile delegates native file calls to the inner router.
func (p *Provider) DeleteFile(ctx context.Context, providerType, id string) (*core.FileDeleteResponse, error) {
	router, ok := p.nativeFileRouter()
	if !ok {
		return nil, core.NewInvalidRequestError("file routing is not supported by the current provider router", nil)
	}
	return router.DeleteFile(ctx, providerType, id)
}

// GetFileContent delegates native file calls to the inner router.
func (p *Provider) GetFileContent(ctx context.Context, providerType, id string) (*core.FileContentResponse, error) {
	router, ok := p.nativeFileRouter()
	if !ok {
		return nil, core.NewInvalidRequestError("file routing is not supported by the current provider router", nil)
	}
	return router.GetFileContent(ctx, providerType, id)
}

var _ core.RoutableProvider = (*Provider)(nil)
var _ core.NativeBatchRoutableProvider = (*Provider)(nil)
var _ core.NativeFileRoutableProvider = (*Provider)(nil)
var _ = http.MethodGet
