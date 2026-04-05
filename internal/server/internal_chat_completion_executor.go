package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"gomodel/internal/auditlog"
	"gomodel/internal/core"
	"gomodel/internal/usage"
)

// InternalChatCompletionExecutorConfig configures the transport-free translated
// chat execution path used by gateway-owned workflows such as guardrails.
type InternalChatCompletionExecutorConfig struct {
	ModelResolver           RequestModelResolver
	ExecutionPolicyResolver RequestExecutionPolicyResolver
	FallbackResolver        RequestFallbackResolver
	AuditLogger             auditlog.LoggerInterface
	UsageLogger             usage.LoggerInterface
	PricingResolver         usage.PricingResolver
}

// InternalChatCompletionExecutor executes internal translated chat requests
// without synthesizing an HTTP request or Echo context.
type InternalChatCompletionExecutor struct {
	provider                core.RoutableProvider
	modelResolver           RequestModelResolver
	executionPolicyResolver RequestExecutionPolicyResolver
	logger                  auditlog.LoggerInterface
	service                 *translatedInferenceService
}

// NewInternalChatCompletionExecutor creates a transport-free translated chat
// executor that reuses planning, fallback, usage, and audit logic.
func NewInternalChatCompletionExecutor(provider core.RoutableProvider, cfg InternalChatCompletionExecutorConfig) *InternalChatCompletionExecutor {
	service := &translatedInferenceService{
		provider:                provider,
		modelResolver:           cfg.ModelResolver,
		executionPolicyResolver: cfg.ExecutionPolicyResolver,
		fallbackResolver:        cfg.FallbackResolver,
		logger:                  cfg.AuditLogger,
		usageLogger:             cfg.UsageLogger,
		pricingResolver:         cfg.PricingResolver,
	}

	return &InternalChatCompletionExecutor{
		provider:                provider,
		modelResolver:           cfg.ModelResolver,
		executionPolicyResolver: cfg.ExecutionPolicyResolver,
		logger:                  cfg.AuditLogger,
		service:                 service,
	}
}

// ChatCompletion executes one internal translated chat request.
func (e *InternalChatCompletionExecutor) ChatCompletion(ctx context.Context, req *core.ChatRequest) (resp *core.ChatResponse, err error) {
	if req == nil {
		return nil, core.NewInvalidRequestError("chat request is required", nil)
	}
	if req.Stream {
		return nil, core.NewInvalidRequestError("internal translated chat executor does not support streaming requests", nil)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = core.WithRequestOrigin(ctx, core.RequestOriginGuardrail)

	requestID := strings.TrimSpace(core.GetRequestID(ctx))
	requested := core.NewRequestedModelSelector(req.Model, req.Provider)
	start := time.Now()
	entry := e.newAuditEntry(ctx, requestID, requested)
	var plan *core.ExecutionPlan
	defer func() {
		e.finishAuditEntry(ctx, entry, start, plan, req, resp, err)
	}()

	resolution, err := resolveRequestModel(e.provider, e.modelResolver, requested)
	if err != nil {
		return nil, err
	}
	plan, err = translatedExecutionPlan(
		ctx,
		requestID,
		core.DescribeEndpoint(http.MethodPost, "/v1/chat/completions"),
		resolution,
		e.executionPolicyResolver,
	)
	if err != nil {
		return nil, err
	}

	execReq := cloneChatRequestForSelector(req, resolution.ResolvedSelector)
	resp, providerType, _, err := e.service.executeChatCompletion(ctx, plan, execReq)
	if err != nil {
		return nil, err
	}

	e.service.logUsage(ctx, plan, resp.Model, providerType, func(pricing *core.ModelPricing) *usage.UsageEntry {
		return usage.ExtractFromChatResponse(resp, requestID, providerType, "/v1/chat/completions", pricing)
	})
	return resp, nil
}

func (e *InternalChatCompletionExecutor) newAuditEntry(
	ctx context.Context,
	requestID string,
	requested core.RequestedModelSelector,
) *auditlog.LogEntry {
	if e.logger == nil || !e.logger.Config().Enabled {
		return nil
	}

	userPath := core.UserPathFromContext(ctx)
	if userPath == "" {
		userPath = "/"
	}

	entry := &auditlog.LogEntry{
		ID:        uuid.NewString(),
		Timestamp: time.Now(),
		RequestID: requestID,
		Method:    http.MethodPost,
		Path:      "/v1/chat/completions",
		UserPath:  userPath,
		Data:      &auditlog.LogData{},
	}
	if requestedModel := requested.RequestedQualifiedModel(); requestedModel != "" {
		entry.Model = requestedModel
	}
	return entry
}

func (e *InternalChatCompletionExecutor) finishAuditEntry(
	ctx context.Context,
	entry *auditlog.LogEntry,
	start time.Time,
	plan *core.ExecutionPlan,
	req *core.ChatRequest,
	resp *core.ChatResponse,
	err error,
) {
	if entry == nil || e.logger == nil || !e.logger.Config().Enabled {
		return
	}

	entry.DurationNs = time.Since(start).Nanoseconds()
	auditlog.EnrichLogEntryWithExecutionPlan(entry, plan)
	auditlog.EnrichLogEntryWithRequestContext(entry, ctx)
	if plan != nil && !plan.AuditEnabled() {
		return
	}

	cfg := e.logger.Config()
	if cfg.LogBodies && entry.Data != nil {
		entry.Data.RequestBody = auditJSONBody(req)
		if resp != nil {
			entry.Data.ResponseBody = auditJSONBody(resp)
		}
	}

	if err != nil {
		var gatewayErr *core.GatewayError
		if errors.As(err, &gatewayErr) && gatewayErr != nil {
			entry.ErrorType = string(gatewayErr.Type)
			entry.StatusCode = gatewayErr.HTTPStatusCode()
			if entry.Data != nil {
				entry.Data.ErrorMessage = gatewayErr.Message
			}
		} else {
			entry.ErrorType = string(core.ErrorTypeProvider)
			entry.StatusCode = http.StatusInternalServerError
			if entry.Data != nil {
				entry.Data.ErrorMessage = err.Error()
			}
		}
	} else {
		entry.StatusCode = http.StatusOK
	}

	e.logger.Write(entry)
}

func auditJSONBody(value any) any {
	if value == nil {
		return nil
	}
	body, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var decoded any
	if err := json.Unmarshal(body, &decoded); err != nil {
		return string(body)
	}
	return decoded
}
