package requestflow

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"gomodel/config"
	"gomodel/internal/guardrails"
)

type cacheEntry struct {
	plan      *ResolvedPlan
	expiresAt time.Time
}

// Manager resolves cached request flow plans and serves admin CRUD.
type Manager struct {
	mu         sync.RWMutex
	store      Store
	logger     LoggerInterface
	yamlDefs   []*Definition
	dbDefs     []*Definition
	baseRetry  config.RetryConfig
	baseRules  []GuardrailRule
	cacheTTL   time.Duration
	sourceMode string
	writable   bool
	cache      map[string]cacheEntry
}

// Options configures a Manager.
type Options struct {
	Store      Store
	Logger     LoggerInterface
	YAMLDefs   []*Definition
	DBDefs     []*Definition
	BaseRetry  config.RetryConfig
	BaseRules  []GuardrailRule
	CacheTTL   time.Duration
	SourceMode string
	Writable   bool
}

// NewManager creates a new Manager.
func NewManager(opts Options) *Manager {
	logger := opts.Logger
	if logger == nil {
		logger = &NoopLogger{}
	}
	if opts.CacheTTL <= 0 {
		opts.CacheTTL = 5 * time.Minute
	}
	return &Manager{
		store:      opts.Store,
		logger:     logger,
		yamlDefs:   cloneDefinitions(opts.YAMLDefs),
		dbDefs:     cloneDefinitions(opts.DBDefs),
		baseRetry:  opts.BaseRetry,
		baseRules:  cloneRules(opts.BaseRules),
		cacheTTL:   opts.CacheTTL,
		sourceMode: opts.SourceMode,
		writable:   opts.Writable,
		cache:      make(map[string]cacheEntry),
	}
}

// Resolve returns the effective plan for a request.
func (m *Manager) Resolve(ctx ResolveContext) (*ResolvedPlan, error) {
	key := cacheKey(ctx)
	now := time.Now()

	m.mu.RLock()
	if entry, ok := m.cache[key]; ok {
		if entry.expiresAt.IsZero() || now.Before(entry.expiresAt) {
			plan := entry.plan
			m.mu.RUnlock()
			return plan, nil
		}
	}
	defs := append(cloneDefinitions(m.yamlDefs), cloneDefinitions(m.dbDefs)...)
	baseRetry := m.baseRetry
	baseRules := cloneRules(m.baseRules)
	cacheTTL := m.cacheTTL
	m.mu.RUnlock()

	matches := make([]*Definition, 0)
	for _, def := range defs {
		if def == nil || !def.Enabled {
			continue
		}
		if definitionMatches(def, ctx) {
			matches = append(matches, def)
		}
	}
	sort.SliceStable(matches, func(i, j int) bool {
		si := specificity(matches[i].Match)
		sj := specificity(matches[j].Match)
		if si != sj {
			return si < sj
		}
		if matches[i].Priority != matches[j].Priority {
			return matches[i].Priority < matches[j].Priority
		}
		return matches[i].ID < matches[j].ID
	})

	plan := &ResolvedPlan{
		Retry:            baseRetry,
		Guardrails:       baseRules,
		FailoverEnabled:  false,
		FailoverStrategy: "disabled",
	}
	for _, def := range matches {
		applyDefinition(plan, def)
		plan.Matches = append(plan.Matches, MatchedPlan{
			ID:       def.ID,
			Name:     def.Name,
			Priority: def.Priority,
			Source:   def.Source,
			Match:    def.Match,
		})
	}
	if len(plan.Guardrails) > 0 {
		pipeline, err := buildPipeline(plan.Guardrails)
		if err != nil {
			return nil, err
		}
		plan.PipelineSignature = fmt.Sprintf("guardrails:%d", pipeline.Len())
		plan.pipeline = pipeline
	}

	m.mu.Lock()
	expiresAt := time.Time{}
	if cacheTTL > 0 {
		expiresAt = now.Add(cacheTTL)
	}
	m.cache[key] = cacheEntry{plan: plan, expiresAt: expiresAt}
	m.mu.Unlock()
	return plan, nil
}

// ListDefinitions returns the combined editable and YAML-backed plans.
func (m *Manager) ListDefinitions(_ context.Context) (*DefinitionListResult, error) {
	m.mu.RLock()
	defs := append(cloneDefinitions(m.yamlDefs), cloneDefinitions(m.dbDefs)...)
	writable := m.writable
	sourceMode := m.sourceMode
	m.mu.RUnlock()
	sortDefinitions(defs)
	return &DefinitionListResult{Plans: defs, Writable: writable, SourceMode: sourceMode}, nil
}

// UpsertDefinition persists a DB-backed plan and refreshes caches.
func (m *Manager) UpsertDefinition(ctx context.Context, def *Definition) (*Definition, error) {
	if def == nil {
		return nil, fmt.Errorf("definition is required")
	}
	if !m.writable || m.store == nil {
		return nil, fmt.Errorf("request flow plans are read-only in %s mode", m.sourceMode)
	}
	cleaned, err := normalizeDefinition(def)
	if err != nil {
		return nil, err
	}
	if err := m.store.SaveDefinition(ctx, cleaned); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	replaced := false
	for i, existing := range m.dbDefs {
		if existing != nil && existing.ID == cleaned.ID {
			m.dbDefs[i] = cleaned
			replaced = true
			break
		}
	}
	if !replaced {
		m.dbDefs = append(m.dbDefs, cleaned)
	}
	m.cache = make(map[string]cacheEntry)
	return cloneDefinition(cleaned), nil
}

// DeleteDefinition removes a DB-backed plan.
func (m *Manager) DeleteDefinition(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("definition id is required")
	}
	if !m.writable || m.store == nil {
		return fmt.Errorf("request flow plans are read-only in %s mode", m.sourceMode)
	}
	if err := m.store.DeleteDefinition(ctx, id); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	filtered := m.dbDefs[:0]
	for _, def := range m.dbDefs {
		if def != nil && def.ID == id {
			continue
		}
		filtered = append(filtered, def)
	}
	m.dbDefs = filtered
	m.cache = make(map[string]cacheEntry)
	return nil
}

// ListExecutions returns persisted execution history.
func (m *Manager) ListExecutions(ctx context.Context, params ExecutionQueryParams) (*ExecutionLogResult, error) {
	if m.store == nil {
		return &ExecutionLogResult{Entries: []*Execution{}, Limit: normalizeLimit(params.Limit), Offset: normalizeOffset(params.Offset)}, nil
	}
	params.Limit = normalizeLimit(params.Limit)
	params.Offset = normalizeOffset(params.Offset)
	return m.store.ListExecutions(ctx, params)
}

// LogExecution queues a completed execution for storage.
func (m *Manager) LogExecution(entry *Execution) {
	if entry == nil {
		return
	}
	m.logger.Write(entry)
}

// Close flushes any pending execution logs.
func (m *Manager) Close() error {
	if m == nil {
		return nil
	}
	return m.logger.Close()
}

func normalizeDefinition(def *Definition) (*Definition, error) {
	copyDef := cloneDefinition(def)
	copyDef.ID = strings.TrimSpace(copyDef.ID)
	copyDef.Name = strings.TrimSpace(copyDef.Name)
	if copyDef.ID == "" {
		copyDef.ID = uuid.NewString()
	}
	if copyDef.Name == "" {
		return nil, fmt.Errorf("plan name is required")
	}
	now := time.Now().UTC()
	if copyDef.CreatedAt.IsZero() {
		copyDef.CreatedAt = now
	}
	copyDef.UpdatedAt = now
	copyDef.Source = "db"
	if _, err := buildPipeline(mergedRules(nil, copyDef.Spec.Guardrails)); err != nil {
		return nil, err
	}
	return copyDef, nil
}

func definitionMatches(def *Definition, ctx ResolveContext) bool {
	match := def.Match
	if match.Model != "" {
		if strings.Contains(match.Model, "/") {
			qualified := qualifiedModel(ctx.Model, ctx.Provider)
			if qualified != match.Model {
				return false
			}
		} else if ctx.Model != match.Model {
			return false
		}
	}
	if match.APIKeyHash != "" && match.APIKeyHash != ctx.APIKeyHash {
		return false
	}
	if match.Team != "" && match.Team != ctx.Team {
		return false
	}
	if match.User != "" && match.User != ctx.User {
		return false
	}
	return true
}

func specificity(match MatchCriteria) int {
	score := 0
	if match.Model != "" {
		score++
	}
	if match.APIKeyHash != "" {
		score++
	}
	if match.Team != "" {
		score++
	}
	if match.User != "" {
		score++
	}
	return score
}

func applyDefinition(plan *ResolvedPlan, def *Definition) {
	plan.Guardrails = mergedRules(plan.Guardrails, def.Spec.Guardrails)
	applyRetryPolicy(&plan.Retry, def.Spec.Retry)
	applyFailoverPolicy(plan, def.Spec.Failover)
}

func mergedRules(base []GuardrailRule, spec GuardrailSpec) []GuardrailRule {
	mode := strings.ToLower(strings.TrimSpace(spec.Mode))
	if mode == "replace" {
		return cloneRules(spec.Rules)
	}
	combined := cloneRules(base)
	combined = append(combined, cloneRules(spec.Rules)...)
	return combined
}

func applyRetryPolicy(dst *config.RetryConfig, policy RetryPolicy) {
	if policy.MaxRetries != nil {
		dst.MaxRetries = *policy.MaxRetries
	}
	if policy.InitialBackoff != nil {
		dst.InitialBackoff = time.Duration(*policy.InitialBackoff)
	}
	if policy.MaxBackoff != nil {
		dst.MaxBackoff = time.Duration(*policy.MaxBackoff)
	}
	if policy.BackoffFactor != nil {
		dst.BackoffFactor = *policy.BackoffFactor
	}
	if policy.JitterFactor != nil {
		dst.JitterFactor = *policy.JitterFactor
	}
}

func applyFailoverPolicy(dst *ResolvedPlan, policy FailoverPolicy) {
	if policy.Enabled != nil {
		dst.FailoverEnabled = *policy.Enabled
	}
	if strings.TrimSpace(policy.Strategy) != "" {
		dst.FailoverStrategy = strings.TrimSpace(policy.Strategy)
	}
	if !dst.FailoverEnabled && dst.FailoverStrategy == "" {
		dst.FailoverStrategy = "disabled"
	}
}

func buildPipeline(rules []GuardrailRule) (*guardrails.Pipeline, error) {
	pipeline := guardrails.NewPipeline()
	for i, rule := range rules {
		g, err := buildGuardrail(rule)
		if err != nil {
			return nil, fmt.Errorf("guardrail rule #%d (%q): %w", i, rule.Name, err)
		}
		pipeline.Add(g, rule.Order)
	}
	return pipeline, nil
}

func buildGuardrail(rule GuardrailRule) (guardrails.Guardrail, error) {
	if strings.TrimSpace(rule.Name) == "" {
		return nil, fmt.Errorf("name is required")
	}
	switch rule.Type {
	case "system_prompt":
		mode := guardrails.SystemPromptMode(rule.SystemPrompt.Mode)
		if mode == "" {
			mode = guardrails.SystemPromptInject
		}
		return guardrails.NewSystemPromptGuardrail(rule.Name, mode, rule.SystemPrompt.Content)
	default:
		return nil, fmt.Errorf("unknown guardrail type: %q", rule.Type)
	}
}

func cacheKey(ctx ResolveContext) string {
	return strings.Join([]string{ctx.Endpoint, qualifiedModel(ctx.Model, ctx.Provider), ctx.APIKeyHash, ctx.Team, ctx.User}, "|")
}

func qualifiedModel(model, provider string) string {
	if provider == "" {
		return model
	}
	return provider + "/" + model
}

func cloneRules(rules []GuardrailRule) []GuardrailRule {
	if len(rules) == 0 {
		return nil
	}
	out := make([]GuardrailRule, len(rules))
	copy(out, rules)
	return out
}

func cloneDefinition(def *Definition) *Definition {
	if def == nil {
		return nil
	}
	copyDef := *def
	copyDef.Spec.Guardrails.Rules = cloneRules(def.Spec.Guardrails.Rules)
	return &copyDef
}

func cloneDefinitions(defs []*Definition) []*Definition {
	if len(defs) == 0 {
		return nil
	}
	out := make([]*Definition, 0, len(defs))
	for _, def := range defs {
		if def != nil {
			out = append(out, cloneDefinition(def))
		}
	}
	return out
}

func sortDefinitions(defs []*Definition) {
	sort.SliceStable(defs, func(i, j int) bool {
		si := specificity(defs[i].Match)
		sj := specificity(defs[j].Match)
		if si != sj {
			return si > sj
		}
		if defs[i].Priority != defs[j].Priority {
			return defs[i].Priority > defs[j].Priority
		}
		if defs[i].Source != defs[j].Source {
			return defs[i].Source < defs[j].Source
		}
		return defs[i].Name < defs[j].Name
	})
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return 50
	}
	if limit > 200 {
		return 200
	}
	return limit
}

func normalizeOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}
