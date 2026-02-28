//go:build contract

package contract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

const goldenOutputDir = "golden"

func goldenPathForFixture(fixture string) string {
	return strings.TrimSuffix(fixture, filepath.Ext(fixture)) + ".golden.json"
}

func shouldRecordGoldenOutputs() bool {
	return os.Getenv("RECORD") == "1" || os.Getenv("UPDATE_GOLDEN") == "1"
}

func compareGoldenJSON(t *testing.T, path string, value any) {
	t.Helper()

	actual := mustMarshalNormalizedJSON(t, value)
	fullPath := filepath.Join(testdataDir, goldenOutputDir, path)

	if shouldRecordGoldenOutputs() {
		require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0755))
		require.NoError(t, os.WriteFile(fullPath, actual, 0644))
	}

	expected, err := os.ReadFile(fullPath)
	if os.IsNotExist(err) {
		t.Fatalf("missing golden file %s; run `make record-api` then `RECORD=1 go test -tags=contract -timeout=5m ./tests/contract/...`", filepath.Join(goldenOutputDir, path))
	}
	require.NoError(t, err)

	require.JSONEq(t, string(expected), string(actual), "golden mismatch for %s", filepath.Join(goldenOutputDir, path))
}

func mustMarshalNormalizedJSON(t *testing.T, value any) []byte {
	t.Helper()

	raw, err := json.Marshal(value)
	require.NoError(t, err)

	var generic any
	require.NoError(t, json.Unmarshal(raw, &generic))

	normalized := normalizeGoldenValue(generic)
	out, err := json.MarshalIndent(normalized, "", "  ")
	require.NoError(t, err)
	return append(out, '\n')
}

func normalizeGoldenValue(v any) any {
	switch val := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(val))
		for key, item := range val {
			lowerKey := strings.ToLower(key)
			if lowerKey == "created" || lowerKey == "created_at" {
				out[key] = int64(0)
				continue
			}

			normalizedItem := normalizeGoldenValue(item)
			if lowerKey == "id" {
				if id, ok := normalizedItem.(string); ok {
					out[key] = normalizeGeneratedID(id)
					continue
				}
			}
			out[key] = normalizedItem
		}
		return out
	case []any:
		out := make([]any, len(val))
		for i, item := range val {
			out[i] = normalizeGoldenValue(item)
		}
		return out
	default:
		return v
	}
}

func normalizeGeneratedID(id string) string {
	if strings.HasPrefix(id, "resp_") {
		if _, err := uuid.Parse(strings.TrimPrefix(id, "resp_")); err == nil {
			return "resp_<generated>"
		}
	}
	if strings.HasPrefix(id, "msg_") {
		if _, err := uuid.Parse(strings.TrimPrefix(id, "msg_")); err == nil {
			return "msg_<generated>"
		}
	}
	return id
}
