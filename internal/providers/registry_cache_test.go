package providers

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gomodel/internal/cache"
	"gomodel/internal/core"
)

func TestCacheFile(t *testing.T) {
	t.Run("SetCache", func(t *testing.T) {
		registry := NewModelRegistry()
		localCache := cache.NewLocalCache("/tmp/test-cache.json")
		registry.SetCache(localCache)
		// Verify no panic, cache is set (private field)
	})

	t.Run("SaveToCache", func(t *testing.T) {
		tmpDir := t.TempDir()
		cacheFile := filepath.Join(tmpDir, "models.json")

		registry := NewModelRegistry()
		localCache := cache.NewLocalCache(cacheFile)
		registry.SetCache(localCache)

		mock := &mockProvider{
			name: "openai",
			modelsResponse: &core.ModelsResponse{
				Object: "list",
				Data: []core.Model{
					{ID: "gpt-4o", Object: "model", OwnedBy: "openai", Created: 1234567890},
					{ID: "gpt-3.5-turbo", Object: "model", OwnedBy: "openai", Created: 1234567891},
				},
			},
		}
		registry.RegisterProviderWithType(mock, "openai")
		_ = registry.Initialize(context.Background())

		err := registry.SaveToCache(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify cache file was created
		if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
			t.Fatal("cache file was not created")
		}

		// Verify cache file contents
		data, err := os.ReadFile(cacheFile)
		if err != nil {
			t.Fatalf("failed to read cache file: %v", err)
		}

		var modelCache cache.ModelCache
		if err := json.Unmarshal(data, &modelCache); err != nil {
			t.Fatalf("failed to unmarshal cache: %v", err)
		}

		if modelCache.Version != 1 {
			t.Errorf("expected version 1, got %d", modelCache.Version)
		}
		if len(modelCache.Models) != 2 {
			t.Errorf("expected 2 models, got %d", len(modelCache.Models))
		}
	})

	t.Run("LoadFromCache", func(t *testing.T) {
		tmpDir := t.TempDir()
		cacheFile := filepath.Join(tmpDir, "models.json")

		// Create a cache file (map-based structure)
		modelCache := cache.ModelCache{
			Version:   1,
			UpdatedAt: time.Now().UTC(),
			Models: map[string]cache.CachedModel{
				"gpt-4o": {
					ProviderType: "openai",
					Object:       "model",
					OwnedBy:      "openai",
					Created:      1234567890,
				},
				"claude-3-5-sonnet": {
					ProviderType: "anthropic",
					Object:       "model",
					OwnedBy:      "anthropic",
					Created:      1234567891,
				},
			},
		}
		data, _ := json.Marshal(modelCache)
		if err := os.WriteFile(cacheFile, data, 0o644); err != nil {
			t.Fatalf("failed to write cache file: %v", err)
		}

		// Create registry with providers
		registry := NewModelRegistry()
		localCache := cache.NewLocalCache(cacheFile)
		registry.SetCache(localCache)

		openaiMock := &mockProvider{
			name:           "openai",
			modelsResponse: &core.ModelsResponse{Object: "list"},
		}
		anthropicMock := &mockProvider{
			name:           "anthropic",
			modelsResponse: &core.ModelsResponse{Object: "list"},
		}
		registry.RegisterProviderWithType(openaiMock, "openai")
		registry.RegisterProviderWithType(anthropicMock, "anthropic")

		// Load from cache
		loaded, err := registry.LoadFromCache(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if loaded != 2 {
			t.Errorf("expected 2 models loaded, got %d", loaded)
		}

		// Verify models are accessible
		if !registry.Supports("gpt-4o") {
			t.Error("expected gpt-4o to be supported")
		}
		if !registry.Supports("claude-3-5-sonnet") {
			t.Error("expected claude-3-5-sonnet to be supported")
		}

		// Verify correct provider mapping
		provider := registry.GetProvider("gpt-4o")
		if provider != openaiMock {
			t.Error("expected gpt-4o to be mapped to openai provider")
		}

		provider = registry.GetProvider("claude-3-5-sonnet")
		if provider != anthropicMock {
			t.Error("expected claude-3-5-sonnet to be mapped to anthropic provider")
		}
	})

	t.Run("LoadFromCacheSkipsUnconfiguredProviders", func(t *testing.T) {
		tmpDir := t.TempDir()
		cacheFile := filepath.Join(tmpDir, "models.json")

		// Create cache with models from multiple providers
		modelCache := cache.ModelCache{
			Version:   1,
			UpdatedAt: time.Now().UTC(),
			Models: map[string]cache.CachedModel{
				"gpt-4o": {
					ProviderType: "openai",
					Object:       "model",
					OwnedBy:      "openai",
				},
				"claude-3": {
					ProviderType: "anthropic", // This provider won't be configured
					Object:       "model",
					OwnedBy:      "anthropic",
				},
			},
		}
		data, _ := json.Marshal(modelCache)
		_ = os.WriteFile(cacheFile, data, 0o644)

		// Only register OpenAI provider
		registry := NewModelRegistry()
		localCache := cache.NewLocalCache(cacheFile)
		registry.SetCache(localCache)
		openaiMock := &mockProvider{name: "openai"}
		registry.RegisterProviderWithType(openaiMock, "openai")

		loaded, err := registry.LoadFromCache(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Only the OpenAI model should be loaded
		if loaded != 1 {
			t.Errorf("expected 1 model loaded, got %d", loaded)
		}
		if !registry.Supports("gpt-4o") {
			t.Error("expected gpt-4o to be supported")
		}
		if registry.Supports("claude-3") {
			t.Error("expected claude-3 to NOT be supported (unconfigured provider)")
		}
	})

	t.Run("LoadFromCacheNoFile", func(t *testing.T) {
		tmpDir := t.TempDir()
		cacheFile := filepath.Join(tmpDir, "nonexistent.json")

		registry := NewModelRegistry()
		localCache := cache.NewLocalCache(cacheFile)
		registry.SetCache(localCache)

		loaded, err := registry.LoadFromCache(context.Background())
		if err != nil {
			t.Fatalf("expected no error for missing file, got: %v", err)
		}
		if loaded != 0 {
			t.Errorf("expected 0 models loaded, got %d", loaded)
		}
	})

	t.Run("LoadFromCacheNoCacheSet", func(t *testing.T) {
		registry := NewModelRegistry()

		loaded, err := registry.LoadFromCache(context.Background())
		if err != nil {
			t.Fatalf("expected no error when no cache set, got: %v", err)
		}
		if loaded != 0 {
			t.Errorf("expected 0 models loaded, got %d", loaded)
		}
	})

	t.Run("SaveToCacheNoCacheSet", func(t *testing.T) {
		registry := NewModelRegistry()

		err := registry.SaveToCache(context.Background())
		if err != nil {
			t.Fatalf("expected no error when no cache set, got: %v", err)
		}
	})

	t.Run("SaveToCacheCreatesDirectory", func(t *testing.T) {
		tmpDir := t.TempDir()
		cacheFile := filepath.Join(tmpDir, "subdir", "nested", "models.json")

		registry := NewModelRegistry()
		localCache := cache.NewLocalCache(cacheFile)
		registry.SetCache(localCache)

		mock := &mockProvider{
			name: "test",
			modelsResponse: &core.ModelsResponse{
				Object: "list",
				Data: []core.Model{
					{ID: "test-model", Object: "model", OwnedBy: "test"},
				},
			},
		}
		registry.RegisterProviderWithType(mock, "test")
		_ = registry.Initialize(context.Background())

		err := registry.SaveToCache(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
			t.Fatal("cache file was not created in nested directory")
		}
	})
}

func TestInitializeAsync(t *testing.T) {
	t.Run("LoadsFromCacheImmediately", func(t *testing.T) {
		tmpDir := t.TempDir()
		cacheFile := filepath.Join(tmpDir, "models.json")

		// Create a cache file
		modelCache := cache.ModelCache{
			Version:   1,
			UpdatedAt: time.Now().UTC(),
			Models: map[string]cache.CachedModel{
				"cached-model": {
					ProviderType: "test",
					Object:       "model",
					OwnedBy:      "test",
				},
			},
		}
		data, _ := json.Marshal(modelCache)
		_ = os.WriteFile(cacheFile, data, 0o644)

		// Create registry with slow provider (delay ensures cache check happens before network fetch)
		registry := NewModelRegistry()
		localCache := cache.NewLocalCache(cacheFile)
		registry.SetCache(localCache)

		mock := &mockProvider{
			name:            "test",
			listModelsDelay: 50 * time.Millisecond, // delay long enough for assertion to run
			modelsResponse: &core.ModelsResponse{
				Object: "list",
				Data: []core.Model{
					{ID: "network-model", Object: "model", OwnedBy: "test"},
				},
			},
		}
		registry.RegisterProviderWithType(mock, "test")

		// InitializeAsync should return immediately after loading cache
		registry.InitializeAsync(context.Background())

		// Cached model should be available immediately (before background fetch completes)
		if !registry.Supports("cached-model") {
			t.Error("expected cached-model to be available immediately")
		}

		// Wait for background goroutine to complete (for temp dir cleanup)
		time.Sleep(100 * time.Millisecond)
	})

	t.Run("RefreshesInBackground", func(t *testing.T) {
		tmpDir := t.TempDir()
		cacheFile := filepath.Join(tmpDir, "models.json")

		registry := NewModelRegistry()
		localCache := cache.NewLocalCache(cacheFile)
		registry.SetCache(localCache)

		mock := &mockProvider{
			name: "test",
			modelsResponse: &core.ModelsResponse{
				Object: "list",
				Data: []core.Model{
					{ID: "network-model", Object: "model", OwnedBy: "test"},
				},
			},
		}
		registry.RegisterProviderWithType(mock, "test")

		// InitializeAsync should start background fetch
		registry.InitializeAsync(context.Background())

		// Wait for background initialization
		time.Sleep(100 * time.Millisecond)

		// Network model should be available after background refresh
		if !registry.Supports("network-model") {
			t.Error("expected network-model to be available after background refresh")
		}

		// Should be marked as initialized
		if !registry.IsInitialized() {
			t.Error("expected registry to be marked as initialized")
		}
	})

	t.Run("SavesToCacheAfterRefresh", func(t *testing.T) {
		tmpDir := t.TempDir()
		cacheFile := filepath.Join(tmpDir, "models.json")

		registry := NewModelRegistry()
		localCache := cache.NewLocalCache(cacheFile)
		registry.SetCache(localCache)

		mock := &mockProvider{
			name: "test",
			modelsResponse: &core.ModelsResponse{
				Object: "list",
				Data: []core.Model{
					{ID: "new-model", Object: "model", OwnedBy: "test"},
				},
			},
		}
		registry.RegisterProviderWithType(mock, "test")

		// InitializeAsync should save to cache after network fetch
		registry.InitializeAsync(context.Background())

		// Wait for background initialization and cache save
		time.Sleep(100 * time.Millisecond)

		// Verify cache file was created
		if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
			t.Fatal("cache file was not created after background refresh")
		}

		// Verify cache contains the network model
		data, _ := os.ReadFile(cacheFile)
		var modelCache cache.ModelCache
		_ = json.Unmarshal(data, &modelCache)

		if len(modelCache.Models) != 1 {
			t.Fatalf("expected 1 model in cache, got %d", len(modelCache.Models))
		}
		if _, ok := modelCache.Models["new-model"]; !ok {
			t.Errorf("expected new-model in cache, got %v", modelCache.Models)
		}
	})
}

func TestIsInitialized(t *testing.T) {
	t.Run("FalseBeforeInitialize", func(t *testing.T) {
		registry := NewModelRegistry()

		if registry.IsInitialized() {
			t.Error("expected IsInitialized to be false before initialization")
		}
	})

	t.Run("TrueAfterInitialize", func(t *testing.T) {
		registry := NewModelRegistry()
		mock := &mockProvider{
			name: "test",
			modelsResponse: &core.ModelsResponse{
				Object: "list",
				Data: []core.Model{
					{ID: "test-model", Object: "model", OwnedBy: "test"},
				},
			},
		}
		registry.RegisterProvider(mock)

		_ = registry.Initialize(context.Background())

		if !registry.IsInitialized() {
			t.Error("expected IsInitialized to be true after initialization")
		}
	})

	t.Run("FalseAfterLoadFromCacheOnly", func(t *testing.T) {
		tmpDir := t.TempDir()
		cacheFile := filepath.Join(tmpDir, "models.json")

		// Create a cache file
		modelCache := cache.ModelCache{
			Version:   1,
			UpdatedAt: time.Now().UTC(),
			Models: map[string]cache.CachedModel{
				"cached-model": {
					ProviderType: "test",
					Object:       "model",
					OwnedBy:      "test",
				},
			},
		}
		data, _ := json.Marshal(modelCache)
		_ = os.WriteFile(cacheFile, data, 0o644)

		registry := NewModelRegistry()
		localCache := cache.NewLocalCache(cacheFile)
		registry.SetCache(localCache)
		mock := &mockProvider{name: "test"}
		registry.RegisterProviderWithType(mock, "test")

		_, _ = registry.LoadFromCache(context.Background())

		// Should not be marked as initialized (only loaded from cache)
		if registry.IsInitialized() {
			t.Error("expected IsInitialized to be false after loading from cache only")
		}
	})
}

func TestRegisterProviderWithType(t *testing.T) {
	registry := NewModelRegistry()

	mock := &mockProvider{
		name: "test",
		modelsResponse: &core.ModelsResponse{
			Object: "list",
			Data: []core.Model{
				{ID: "test-model", Object: "model", OwnedBy: "test"},
			},
		},
	}

	registry.RegisterProviderWithType(mock, "openai")

	if registry.ProviderCount() != 1 {
		t.Errorf("expected 1 provider, got %d", registry.ProviderCount())
	}
}
