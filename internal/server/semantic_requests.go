package server

import (
	"strings"

	"github.com/labstack/echo/v5"

	"gomodel/internal/core"
)

func ensureSemanticEnvelope(c *echo.Context) *core.SemanticEnvelope {
	ctx := c.Request().Context()
	if env := core.GetSemanticEnvelope(ctx); env != nil {
		return env
	}

	frame := core.GetIngressFrame(ctx)
	if frame == nil {
		return nil
	}

	env := core.BuildSemanticEnvelope(frame)
	if env == nil {
		return nil
	}

	c.SetRequest(c.Request().WithContext(core.WithSemanticEnvelope(ctx, env)))
	return env
}

func semanticJSONBody(c *echo.Context) ([]byte, *core.SemanticEnvelope, error) {
	env := ensureSemanticEnvelope(c)
	bodyBytes, err := requestBodyBytes(c)
	if err != nil {
		return nil, env, err
	}
	return bodyBytes, env, nil
}

func chatRequestFromSemanticEnvelope(c *echo.Context) (*core.ChatRequest, error) {
	bodyBytes, env, err := semanticJSONBody(c)
	if err != nil {
		return nil, err
	}
	return core.DecodeChatRequest(bodyBytes, env)
}

func responsesRequestFromSemanticEnvelope(c *echo.Context) (*core.ResponsesRequest, error) {
	bodyBytes, env, err := semanticJSONBody(c)
	if err != nil {
		return nil, err
	}
	return core.DecodeResponsesRequest(bodyBytes, env)
}

func embeddingRequestFromSemanticEnvelope(c *echo.Context) (*core.EmbeddingRequest, error) {
	bodyBytes, env, err := semanticJSONBody(c)
	if err != nil {
		return nil, err
	}
	return core.DecodeEmbeddingRequest(bodyBytes, env)
}

func batchRequestFromSemanticEnvelope(c *echo.Context) (*core.BatchRequest, error) {
	bodyBytes, env, err := semanticJSONBody(c)
	if err != nil {
		return nil, err
	}
	return core.DecodeBatchRequest(bodyBytes, env)
}

func batchRequestMetadataFromSemanticEnvelope(c *echo.Context) (*core.BatchRequestSemantic, error) {
	return core.BatchRouteMetadata(
		ensureSemanticEnvelope(c),
		c.Request().Method,
		c.Request().URL.Path,
		pathValuesToMap(c.PathValues()),
		c.Request().URL.Query(),
	)
}

func fileRequestFromSemanticEnvelope(c *echo.Context) (*core.FileRequestSemantic, error) {
	env := ensureSemanticEnvelope(c)
	req, err := core.FileRouteMetadata(
		env,
		c.Request().Method,
		c.Request().URL.Path,
		pathValuesToMap(c.PathValues()),
		c.Request().URL.Query(),
	)
	if err != nil {
		return nil, err
	}

	switch req.Action {
	case core.FileActionCreate:
		if req.Provider == "" {
			req.Provider = strings.TrimSpace(c.FormValue("provider"))
		}
		if req.Purpose == "" {
			req.Purpose = strings.TrimSpace(c.FormValue("purpose"))
		}
		if req.Filename == "" {
			fileHeader, err := c.FormFile("file")
			if err == nil && fileHeader != nil {
				req.Filename = strings.TrimSpace(fileHeader.Filename)
			}
		}
	}

	if env != nil {
		env.FileRequest = req
		if req.Provider != "" && env.SelectorHints.Provider == "" {
			env.SelectorHints.Provider = req.Provider
		}
	}
	return req, nil
}

func pathValuesToMap(values echo.PathValues) map[string]string {
	if len(values) == 0 {
		return nil
	}
	params := make(map[string]string, len(values))
	for _, item := range values {
		params[item.Name] = item.Value
	}
	return params
}
