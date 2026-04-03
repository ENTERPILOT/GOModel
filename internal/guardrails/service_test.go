package guardrails

import (
	"context"
	"encoding/json"
	"testing"
)

type testStore struct {
	definitions map[string]Definition
}

func newTestStore(definitions ...Definition) *testStore {
	store := &testStore{definitions: make(map[string]Definition, len(definitions))}
	for _, definition := range definitions {
		store.definitions[definition.Name] = definition
	}
	return store
}

func (s *testStore) List(context.Context) ([]Definition, error) {
	result := make([]Definition, 0, len(s.definitions))
	for _, definition := range s.definitions {
		result = append(result, definition)
	}
	return result, nil
}

func (s *testStore) Get(_ context.Context, name string) (*Definition, error) {
	definition, ok := s.definitions[name]
	if !ok {
		return nil, ErrNotFound
	}
	copy := definition
	return &copy, nil
}

func (s *testStore) Upsert(_ context.Context, definition Definition) error {
	s.definitions[definition.Name] = definition
	return nil
}

func (s *testStore) Delete(_ context.Context, name string) error {
	if _, ok := s.definitions[name]; !ok {
		return ErrNotFound
	}
	delete(s.definitions, name)
	return nil
}

func (s *testStore) Close() error { return nil }

func rawConfig(t *testing.T, value any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return raw
}

func TestServiceRefreshBuildsPipelineFromDefinitions(t *testing.T) {
	store := newTestStore(
		Definition{
			Name: "safety",
			Type: "system_prompt",
			Config: rawConfig(t, map[string]any{
				"mode":    "inject",
				"content": "be safe",
			}),
		},
	)

	service, err := NewService(store)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if err := service.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	if got := service.Names(); len(got) != 1 || got[0] != "safety" {
		t.Fatalf("Names() = %v, want [safety]", got)
	}

	pipeline, hash, err := service.BuildPipeline([]StepReference{{Ref: "safety", Step: 10}})
	if err != nil {
		t.Fatalf("BuildPipeline() error = %v", err)
	}
	if pipeline == nil || pipeline.Len() != 1 {
		t.Fatalf("pipeline = %#v, want one entry", pipeline)
	}
	if hash == "" {
		t.Fatal("BuildPipeline() hash = empty, want non-empty")
	}

	msgs, err := pipeline.Process(context.Background(), []Message{{Role: "user", Content: "hi"}})
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if len(msgs) != 2 || msgs[0].Role != "system" || msgs[0].Content != "be safe" {
		t.Fatalf("Process() messages = %#v, want injected system prompt", msgs)
	}
}

func TestServiceUpsertDefinitions_UpdatesConfiguredSubsetAndPreservesCustomEntries(t *testing.T) {
	store := newTestStore(
		Definition{
			Name: "policy",
			Type: "system_prompt",
			Config: rawConfig(t, map[string]any{
				"mode":    "inject",
				"content": "old policy text",
			}),
		},
		Definition{
			Name: "custom",
			Type: "system_prompt",
			Config: rawConfig(t, map[string]any{
				"mode":    "inject",
				"content": "custom",
			}),
		},
	)
	service, err := NewService(store)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	if err := service.UpsertDefinitions(context.Background(), []Definition{
		{
			Name: "policy",
			Type: "system_prompt",
			Config: rawConfig(t, map[string]any{
				"mode":    "override",
				"content": "policy text",
			}),
		},
		{
			Name: "policy-v2",
			Type: "system_prompt",
			Config: rawConfig(t, map[string]any{
				"mode":    "inject",
				"content": "new policy",
			}),
		},
	}); err != nil {
		t.Fatalf("UpsertDefinitions() error = %v", err)
	}

	if got := service.Names(); len(got) != 3 || got[0] != "custom" || got[1] != "policy" || got[2] != "policy-v2" {
		t.Fatalf("Names() after upsert = %v, want [custom policy policy-v2]", got)
	}

	definition, ok := service.Get("policy")
	if !ok || definition == nil {
		t.Fatal("Get(policy) = missing, want updated guardrail")
	}
	var gotConfig map[string]any
	if err := json.Unmarshal(definition.Config, &gotConfig); err != nil {
		t.Fatalf("json.Unmarshal(policy.Config) error = %v", err)
	}
	if gotConfig["mode"] != "override" || gotConfig["content"] != "policy text" {
		t.Fatalf("policy.Config = %#v, want updated config", gotConfig)
	}

	if _, ok := store.definitions["custom"]; !ok {
		t.Fatal("custom guardrail missing after UpsertDefinitions(), want preserved entry")
	}
}

func TestServiceUpsertRejectsInvalidSystemPromptMode(t *testing.T) {
	store := newTestStore()
	service, err := NewService(store)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	err = service.Upsert(context.Background(), Definition{
		Name: "policy",
		Type: "system_prompt",
		Config: rawConfig(t, map[string]any{
			"mode":    "invalid",
			"content": "policy text",
		}),
	})
	if err == nil {
		t.Fatal("Upsert() error = nil, want validation error")
	}
	if !IsValidationError(err) {
		t.Fatalf("Upsert() error = %v, want validation error", err)
	}
	if len(store.definitions) != 0 {
		t.Fatalf("len(store.definitions) = %d, want 0", len(store.definitions))
	}
}

func TestServiceUpsertNormalizesUserPath(t *testing.T) {
	store := newTestStore()
	service, err := NewService(store)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	err = service.Upsert(context.Background(), Definition{
		Name:     "policy",
		Type:     "system_prompt",
		UserPath: "team/alpha",
		Config: rawConfig(t, map[string]any{
			"mode":    "inject",
			"content": "policy text",
		}),
	})
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	definition, ok := service.Get("policy")
	if !ok || definition == nil {
		t.Fatal("Get(policy) = missing, want stored guardrail")
	}
	if definition.UserPath != "/team/alpha" {
		t.Fatalf("definition.UserPath = %q, want /team/alpha", definition.UserPath)
	}
}
