package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"

	"gomodel/internal/executionplans"
	"gomodel/internal/guardrails"
)

type guardrailTestStore struct {
	definitions map[string]guardrails.Definition
}

func newGuardrailTestStore(definitions ...guardrails.Definition) *guardrailTestStore {
	store := &guardrailTestStore{definitions: make(map[string]guardrails.Definition, len(definitions))}
	for _, definition := range definitions {
		store.definitions[definition.Name] = definition
	}
	return store
}

func (s *guardrailTestStore) List(context.Context) ([]guardrails.Definition, error) {
	result := make([]guardrails.Definition, 0, len(s.definitions))
	for _, definition := range s.definitions {
		result = append(result, definition)
	}
	return result, nil
}

func (s *guardrailTestStore) Get(_ context.Context, name string) (*guardrails.Definition, error) {
	definition, ok := s.definitions[name]
	if !ok {
		return nil, guardrails.ErrNotFound
	}
	copy := definition
	return &copy, nil
}

func (s *guardrailTestStore) Upsert(_ context.Context, definition guardrails.Definition) error {
	s.definitions[definition.Name] = definition
	return nil
}

func (s *guardrailTestStore) UpsertMany(_ context.Context, definitions []guardrails.Definition) error {
	for _, definition := range definitions {
		s.definitions[definition.Name] = definition
	}
	return nil
}

func (s *guardrailTestStore) Delete(_ context.Context, name string) error {
	if _, ok := s.definitions[name]; !ok {
		return guardrails.ErrNotFound
	}
	delete(s.definitions, name)
	return nil
}

func (s *guardrailTestStore) Close() error { return nil }

func rawGuardrailConfig(t *testing.T, value any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return raw
}

func newGuardrailService(t *testing.T, definitions ...guardrails.Definition) *guardrails.Service {
	t.Helper()

	service, err := guardrails.NewService(newGuardrailTestStore(definitions...))
	if err != nil {
		t.Fatalf("guardrails.NewService() error = %v", err)
	}
	if err := service.Refresh(context.Background()); err != nil {
		t.Fatalf("guardrails.Refresh() error = %v", err)
	}
	return service
}

func newGuardrailHandler(t *testing.T, definitions ...guardrails.Definition) *Handler {
	t.Helper()
	return NewHandler(nil, nil, WithGuardrailService(newGuardrailService(t, definitions...)))
}

func TestListGuardrails(t *testing.T) {
	h := newGuardrailHandler(t, guardrails.Definition{
		Name: "policy-system",
		Type: "system_prompt",
		Config: rawGuardrailConfig(t, map[string]any{
			"mode":    "inject",
			"content": "be precise",
		}),
	})

	c, rec := newHandlerContext("/admin/api/v1/guardrails")
	if err := h.ListGuardrails(c); err != nil {
		t.Fatalf("ListGuardrails() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var body []guardrails.View
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(body) != 1 || body[0].Name != "policy-system" {
		t.Fatalf("body = %#v, want one policy-system guardrail", body)
	}
	if body[0].Summary == "" {
		t.Fatal("Summary = empty, want populated summary")
	}
}

func TestListGuardrailTypes(t *testing.T) {
	h := newGuardrailHandler(t)
	c, rec := newHandlerContext("/admin/api/v1/guardrails/types")

	if err := h.ListGuardrailTypes(c); err != nil {
		t.Fatalf("ListGuardrailTypes() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var body []guardrails.TypeDefinition
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(body) == 0 || body[0].Type != "system_prompt" {
		t.Fatalf("body = %#v, want system_prompt type definition", body)
	}
}

func TestUpsertGuardrail(t *testing.T) {
	h := newGuardrailHandler(t)
	e := echo.New()

	req := httptest.NewRequest(http.MethodPut, "/admin/api/v1/guardrails/policy-system", bytes.NewBufferString(`{
		"type":"system_prompt",
		"description":"Default policy",
		"user_path":"team/alpha",
		"config":{"mode":"override","content":"Respond carefully."}
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/admin/api/v1/guardrails/:name")
	c.SetPathValues(echo.PathValues{{Name: "name", Value: "policy-system"}})

	if err := h.UpsertGuardrail(c); err != nil {
		t.Fatalf("UpsertGuardrail() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	guardrail, ok := h.guardrailDefs.Get("policy-system")
	if !ok || guardrail == nil {
		t.Fatal("Get(policy-system) = missing, want saved guardrail")
	}
	if guardrail.Type != "system_prompt" {
		t.Fatalf("guardrail.Type = %q, want system_prompt", guardrail.Type)
	}
	if guardrail.UserPath != "/team/alpha" {
		t.Fatalf("guardrail.UserPath = %q, want /team/alpha", guardrail.UserPath)
	}
}

func TestDeleteGuardrailRejectsActiveWorkflowReference(t *testing.T) {
	guardrailService := newGuardrailService(t, guardrails.Definition{
		Name: "policy-system",
		Type: "system_prompt",
		Config: rawGuardrailConfig(t, map[string]any{
			"mode":    "inject",
			"content": "be precise",
		}),
	})
	planStore := &executionPlanTestStore{
		versions: []executionplans.Version{
			{
				ID:       "global-plan",
				Scope:    executionplans.Scope{},
				ScopeKey: "global",
				Version:  1,
				Active:   true,
				Name:     "global",
				Payload: executionplans.Payload{
					SchemaVersion: 1,
					Features:      executionplans.FeatureFlags{Cache: true, Audit: true, Usage: true, Guardrails: true},
					Guardrails:    []executionplans.GuardrailStep{{Ref: "policy-system", Step: 10}},
				},
				PlanHash: "hash-global",
			},
		},
	}
	planService, err := executionplans.NewService(planStore, executionplans.NewCompiler(guardrailService))
	if err != nil {
		t.Fatalf("executionplans.NewService() error = %v", err)
	}
	if err := planService.Refresh(context.Background()); err != nil {
		t.Fatalf("planService.Refresh() error = %v", err)
	}

	h := NewHandler(nil, nil, WithGuardrailService(guardrailService), WithExecutionPlans(planService))
	e := echo.New()
	req := httptest.NewRequest(http.MethodDelete, "/admin/api/v1/guardrails/policy-system", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/admin/api/v1/guardrails/:name")
	c.SetPathValues(echo.PathValues{{Name: "name", Value: "policy-system"}})

	if err := h.DeleteGuardrail(c); err != nil {
		t.Fatalf("DeleteGuardrail() error = %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}

	envelope := decodeExecutionPlanErrorEnvelope(t, rec.Body.Bytes())
	if envelope.Error.Message != "guardrail is used by active workflows: global" {
		t.Fatalf("error message = %q, want active workflow reference", envelope.Error.Message)
	}
}

func TestDeleteGuardrailIgnoresDisabledWorkflowGuardrailRefs(t *testing.T) {
	guardrailService := newGuardrailService(t, guardrails.Definition{
		Name: "policy-system",
		Type: "system_prompt",
		Config: rawGuardrailConfig(t, map[string]any{
			"mode":    "inject",
			"content": "be precise",
		}),
	})
	planStore := &executionPlanTestStore{
		versions: []executionplans.Version{
			{
				ID:       "global-plan",
				Scope:    executionplans.Scope{},
				ScopeKey: "global",
				Version:  1,
				Active:   true,
				Name:     "global",
				Payload: executionplans.Payload{
					SchemaVersion: 1,
					Features:      executionplans.FeatureFlags{Cache: true, Audit: true, Usage: true, Guardrails: false},
					Guardrails:    []executionplans.GuardrailStep{{Ref: "policy-system", Step: 10}},
				},
				PlanHash: "hash-global",
			},
		},
	}
	planService, err := executionplans.NewService(planStore, executionplans.NewCompiler(guardrailService))
	if err != nil {
		t.Fatalf("executionplans.NewService() error = %v", err)
	}
	if err := planService.Refresh(context.Background()); err != nil {
		t.Fatalf("planService.Refresh() error = %v", err)
	}

	h := NewHandler(nil, nil, WithGuardrailService(guardrailService), WithExecutionPlans(planService))
	e := echo.New()
	req := httptest.NewRequest(http.MethodDelete, "/admin/api/v1/guardrails/policy-system", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/admin/api/v1/guardrails/:name")
	c.SetPathValues(echo.PathValues{{Name: "name", Value: "policy-system"}})

	if err := h.DeleteGuardrail(c); err != nil {
		t.Fatalf("DeleteGuardrail() error = %v", err)
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if _, ok := h.guardrailDefs.Get("policy-system"); ok {
		t.Fatal("Get(policy-system) = present, want deleted guardrail")
	}
}
