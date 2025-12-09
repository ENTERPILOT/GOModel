package cache

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLocalCache(t *testing.T) {
	t.Run("GetSetRoundTrip", func(t *testing.T) {
		tmpDir := t.TempDir()
		cacheFile := filepath.Join(tmpDir, "models.json")

		cache := NewLocalCache(cacheFile)
		ctx := context.Background()

		// Initially empty
		result, err := cache.Get(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != nil {
			t.Fatalf("expected nil result for empty cache, got %v", result)
		}

		// Set data
		data := &ModelCache{
			Version:   1,
			UpdatedAt: time.Now().UTC(),
			Models: map[string]CachedModel{
				"test-model": {
					ProviderType: "openai",
					Object:       "model",
					OwnedBy:      "openai",
					Created:      1234567890,
				},
			},
		}

		err = cache.Set(ctx, data)
		if err != nil {
			t.Fatalf("unexpected error on set: %v", err)
		}

		// Get data back
		result, err = cache.Get(ctx)
		if err != nil {
			t.Fatalf("unexpected error on get: %v", err)
		}
		if result == nil {
			t.Fatal("expected result, got nil")
		}
		if result.Version != 1 {
			t.Errorf("expected version 1, got %d", result.Version)
		}
		if len(result.Models) != 1 {
			t.Errorf("expected 1 model, got %d", len(result.Models))
		}
		if _, ok := result.Models["test-model"]; !ok {
			t.Error("expected test-model in cache")
		}
	})

	t.Run("CreateDirectoryIfNeeded", func(t *testing.T) {
		tmpDir := t.TempDir()
		cacheFile := filepath.Join(tmpDir, "nested", "dir", "models.json")

		cache := NewLocalCache(cacheFile)
		ctx := context.Background()

		data := &ModelCache{
			Version: 1,
			Models:  map[string]CachedModel{},
		}

		err := cache.Set(ctx, data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify file was created
		if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
			t.Fatal("cache file was not created")
		}
	})

	t.Run("EmptyFilePath", func(t *testing.T) {
		cache := NewLocalCache("")
		ctx := context.Background()

		// Get should return nil
		result, err := cache.Get(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != nil {
			t.Fatal("expected nil result for empty path")
		}

		// Set should be a no-op
		data := &ModelCache{Version: 1}
		err = cache.Set(ctx, data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("CloseIsNoOp", func(t *testing.T) {
		cache := NewLocalCache("/tmp/test.json")
		err := cache.Close()
		if err != nil {
			t.Fatalf("unexpected error on close: %v", err)
		}
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		tmpDir := t.TempDir()
		cacheFile := filepath.Join(tmpDir, "models.json")

		// Write invalid JSON
		if err := os.WriteFile(cacheFile, []byte("not valid json"), 0o644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		cache := NewLocalCache(cacheFile)
		ctx := context.Background()

		_, err := cache.Get(ctx)
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})
}

func TestModelCacheSerialization(t *testing.T) {
	t.Run("JSONRoundTrip", func(t *testing.T) {
		original := &ModelCache{
			Version:   1,
			UpdatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			Models: map[string]CachedModel{
				"gpt-4": {
					ProviderType: "openai",
					Object:       "model",
					OwnedBy:      "openai",
					Created:      1234567890,
				},
				"claude-3": {
					ProviderType: "anthropic",
					Object:       "model",
					OwnedBy:      "anthropic",
					Created:      1234567891,
				},
			},
		}

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}

		var restored ModelCache
		if err := json.Unmarshal(data, &restored); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		if restored.Version != original.Version {
			t.Errorf("version mismatch: got %d, want %d", restored.Version, original.Version)
		}
		if len(restored.Models) != len(original.Models) {
			t.Errorf("model count mismatch: got %d, want %d", len(restored.Models), len(original.Models))
		}
		if restored.Models["gpt-4"].ProviderType != "openai" {
			t.Error("gpt-4 provider type not preserved")
		}
		if restored.Models["claude-3"].ProviderType != "anthropic" {
			t.Error("claude-3 provider type not preserved")
		}
	})
}
