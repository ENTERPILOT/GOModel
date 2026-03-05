package requestflow

import (
	"time"

	"gomodel/config"
	"gomodel/internal/guardrails"
)

// Definition describes one editable flow plan.
type Definition struct {
	ID          string        `json:"id" bson:"_id" yaml:"id"`
	Name        string        `json:"name" bson:"name" yaml:"name"`
	Description string        `json:"description,omitempty" bson:"description,omitempty" yaml:"description,omitempty"`
	Enabled     bool          `json:"enabled" bson:"enabled" yaml:"enabled"`
	Priority    int           `json:"priority" bson:"priority" yaml:"priority"`
	Match       MatchCriteria `json:"match" bson:"match" yaml:"match"`
	Spec        PlanSpec      `json:"spec" bson:"spec" yaml:"spec"`
	Source      string        `json:"source" bson:"source" yaml:"source"`
	CreatedAt   time.Time     `json:"created_at" bson:"created_at" yaml:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at" bson:"updated_at" yaml:"updated_at"`
}

// MatchCriteria defines the dimensions used to choose a plan.
// Empty criteria means application-wide defaults.
type MatchCriteria struct {
	Model      string `json:"model,omitempty" bson:"model,omitempty" yaml:"model,omitempty"`
	APIKeyHash string `json:"api_key_hash,omitempty" bson:"api_key_hash,omitempty" yaml:"api_key_hash,omitempty"`
	Team       string `json:"team,omitempty" bson:"team,omitempty" yaml:"team,omitempty"`
	User       string `json:"user,omitempty" bson:"user,omitempty" yaml:"user,omitempty"`
}

// PlanSpec holds the configurable behavior for a request flow.
type PlanSpec struct {
	Guardrails GuardrailSpec  `json:"guardrails" bson:"guardrails" yaml:"guardrails"`
	Retry      RetryPolicy    `json:"retry" bson:"retry" yaml:"retry"`
	Failover   FailoverPolicy `json:"failover" bson:"failover" yaml:"failover"`
}

// GuardrailSpec controls how a plan contributes guardrails.
type GuardrailSpec struct {
	Mode  string          `json:"mode,omitempty" bson:"mode,omitempty" yaml:"mode,omitempty"`
	Rules []GuardrailRule `json:"rules,omitempty" bson:"rules,omitempty" yaml:"rules,omitempty"`
}

// GuardrailRule is the editable request-flow representation of a guardrail.
type GuardrailRule struct {
	Name         string               `json:"name" bson:"name" yaml:"name"`
	Type         string               `json:"type" bson:"type" yaml:"type"`
	Order        int                  `json:"order" bson:"order" yaml:"order"`
	SystemPrompt SystemPromptSettings `json:"system_prompt" bson:"system_prompt" yaml:"system_prompt"`
}

// SystemPromptSettings configures a system_prompt guardrail.
type SystemPromptSettings struct {
	Mode    string `json:"mode" bson:"mode" yaml:"mode"`
	Content string `json:"content" bson:"content" yaml:"content"`
}

// RetryPolicy overrides the effective retry config for matching requests.
type RetryPolicy struct {
	MaxRetries     *int      `json:"max_retries,omitempty" bson:"max_retries,omitempty" yaml:"max_retries,omitempty"`
	InitialBackoff *Duration `json:"initial_backoff,omitempty" bson:"initial_backoff,omitempty" yaml:"initial_backoff,omitempty" swaggertype:"string"`
	MaxBackoff     *Duration `json:"max_backoff,omitempty" bson:"max_backoff,omitempty" yaml:"max_backoff,omitempty" swaggertype:"string"`
	BackoffFactor  *float64  `json:"backoff_factor,omitempty" bson:"backoff_factor,omitempty" yaml:"backoff_factor,omitempty"`
	JitterFactor   *float64  `json:"jitter_factor,omitempty" bson:"jitter_factor,omitempty" yaml:"jitter_factor,omitempty"`
}

// FailoverPolicy captures failover intent.
// Execution tracking is implemented now; provider failover remains extensible.
type FailoverPolicy struct {
	Enabled  *bool  `json:"enabled,omitempty" bson:"enabled,omitempty" yaml:"enabled,omitempty"`
	Strategy string `json:"strategy,omitempty" bson:"strategy,omitempty" yaml:"strategy,omitempty"`
}

// ResolveContext is the input to plan resolution.
type ResolveContext struct {
	Endpoint   string
	Model      string
	Provider   string
	APIKeyHash string
	Team       string
	User       string
}

// MatchedPlan captures one definition that contributed to a resolved plan.
type MatchedPlan struct {
	ID       string        `json:"id" bson:"id"`
	Name     string        `json:"name" bson:"name"`
	Priority int           `json:"priority" bson:"priority"`
	Source   string        `json:"source" bson:"source"`
	Match    MatchCriteria `json:"match" bson:"match"`
}

// ResolvedPlan is the merged flow plan used for one request.
type ResolvedPlan struct {
	Retry             config.RetryConfig
	Guardrails        []GuardrailRule
	FailoverEnabled   bool
	FailoverStrategy  string
	Matches           []MatchedPlan
	PipelineSignature string
	pipeline          *guardrails.Pipeline
}

// DefinitionListResult backs the admin plans endpoint.
type DefinitionListResult struct {
	Plans      []*Definition `json:"plans"`
	Writable   bool          `json:"writable"`
	SourceMode string        `json:"source_mode"`
}

// Execution stores the observed flow behavior for one request.
type Execution struct {
	ID                   string        `json:"id" bson:"_id"`
	RequestID            string        `json:"request_id" bson:"request_id"`
	Timestamp            time.Time     `json:"timestamp" bson:"timestamp"`
	Endpoint             string        `json:"endpoint" bson:"endpoint"`
	Model                string        `json:"model" bson:"model"`
	Provider             string        `json:"provider" bson:"provider"`
	PlanName             string        `json:"plan_name" bson:"plan_name"`
	PlanIDs              []string      `json:"plan_ids" bson:"plan_ids"`
	PlanSources          []string      `json:"plan_sources" bson:"plan_sources"`
	GuardrailsConfigured []string      `json:"guardrails_configured" bson:"guardrails_configured"`
	GuardrailsApplied    []string      `json:"guardrails_applied" bson:"guardrails_applied"`
	GuardrailRejected    bool          `json:"guardrail_rejected" bson:"guardrail_rejected"`
	RetryConfigured      int           `json:"retry_configured" bson:"retry_configured"`
	UpstreamAttempts     int           `json:"upstream_attempts" bson:"upstream_attempts"`
	RetriesMade          int           `json:"retries_made" bson:"retries_made"`
	FailoverEnabled      bool          `json:"failover_enabled" bson:"failover_enabled"`
	FailoverUsed         bool          `json:"failover_used" bson:"failover_used"`
	Status               string        `json:"status" bson:"status"`
	Error                string        `json:"error,omitempty" bson:"error,omitempty"`
	Duration             time.Duration `json:"duration_ns" bson:"duration_ns" swaggertype:"integer"`
}

// ExecutionQueryParams are used by the admin execution log endpoint.
type ExecutionQueryParams struct {
	Search    string
	RequestID string
	Model     string
	Limit     int
	Offset    int
}

// ExecutionLogResult backs the admin executions endpoint.
type ExecutionLogResult struct {
	Entries []*Execution `json:"entries"`
	Total   int          `json:"total"`
	Limit   int          `json:"limit"`
	Offset  int          `json:"offset"`
}
