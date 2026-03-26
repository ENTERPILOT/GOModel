package server

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v5"

	"gomodel/internal/core"
)

// RequestExecutionPolicyResolver matches persisted execution-plan versions for requests.
type RequestExecutionPolicyResolver interface {
	Match(selector core.ExecutionPlanSelector) (*core.ResolvedExecutionPolicy, error)
}

func applyExecutionPolicy(plan *core.ExecutionPlan, resolver RequestExecutionPolicyResolver, selector core.ExecutionPlanSelector) error {
	if plan == nil || resolver == nil {
		return nil
	}
	policy, err := resolver.Match(selector)
	if err != nil {
		return normalizeExecutionPolicyError(err)
	}
	plan.Policy = policy
	return nil
}

func normalizeExecutionPolicyError(err error) error {
	if err == nil {
		return nil
	}
	if gatewayErr, ok := errors.AsType[*core.GatewayError](err); ok {
		return gatewayErr
	}
	return core.NewProviderError("", http.StatusInternalServerError, "failed to resolve execution policy", err)
}

func cloneCurrentExecutionPlan(c *echo.Context) *core.ExecutionPlan {
	if c == nil {
		return nil
	}
	if existing := core.GetExecutionPlan(c.Request().Context()); existing != nil {
		cloned := *existing
		return &cloned
	}
	return &core.ExecutionPlan{}
}

func executionPlanVersionID(plan *core.ExecutionPlan) string {
	if plan == nil {
		return ""
	}
	return plan.ExecutionPlanVersionID()
}

func boolPtr(value bool) *bool {
	return &value
}
