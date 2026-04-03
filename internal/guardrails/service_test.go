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

func TestServiceEnsureSeedDefinitionsOnlySeedsEmptyStore(t *testing.T) {
	store := newTestStore()
	service, err := NewService(store)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	seed := []Definition{
		{
			Name: "policy",
			Type: "system_prompt",
			Config: rawConfig(t, map[string]any{
				"mode":    "override",
				"content": "policy text",
			}),
		},
	}

	if err := service.EnsureSeedDefinitions(context.Background(), seed); err != nil {
		t.Fatalf("EnsureSeedDefinitions() error = %v", err)
	}
	if got := service.Names(); len(got) != 1 || got[0] != "policy" {
		t.Fatalf("Names() after seed = %v, want [policy]", got)
	}

	store.definitions["custom"] = Definition{
		Name: "custom",
		Type: "system_prompt",
		Config: rawConfig(t, map[string]any{
			"mode":    "inject",
			"content": "custom",
		}),
	}
	if err := service.EnsureSeedDefinitions(context.Background(), []Definition{
		{
			Name: "ignored",
			Type: "system_prompt",
			Config: rawConfig(t, map[string]any{
				"mode":    "inject",
				"content": "ignored",
			}),
		},
	}); err != nil {
		t.Fatalf("EnsureSeedDefinitions() second call error = %v", err)
	}
	if _, ok := store.definitions["ignored"]; ok {
		t.Fatal("EnsureSeedDefinitions() seeded into non-empty store, want no-op")
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
