package usage

import "testing"

func TestNormalizedUsageEntryForStorageClearsInvalidCacheTypeWithoutMutatingInput(t *testing.T) {
	entry := &UsageEntry{
		ID:        "usage-1",
		RequestID: "req-1",
		CacheType: "invalid-cache-type",
	}

	got := normalizedUsageEntryForStorage(entry)
	if got == entry {
		t.Fatal("expected invalid cache type to clone entry for normalization")
	}
	if got.CacheType != "" {
		t.Fatalf("normalized CacheType = %q, want empty", got.CacheType)
	}
	if entry.CacheType != "invalid-cache-type" {
		t.Fatalf("input CacheType mutated to %q", entry.CacheType)
	}
}
