package usage

import (
	"fmt"
	"sort"
	"strings"
	"sync/atomic"

	"gomodel/internal/core"
)

// CostResult holds the result of a granular cost calculation.
type CostResult struct {
	InputCost  *float64
	OutputCost *float64
	TotalCost  *float64
	Caveat     string
}

// costRegistry holds provider-specific cost mappings and informational fields,
// populated at startup via RegisterCostMappings.
type costRegistry struct {
	providerMappings    map[string][]core.TokenCostMapping
	informationalFields map[string]struct{}
	extendedFieldSet    map[string]struct{}
}

// costRegistryPtr is the package-level registry used by CalculateGranularCost
// and stream_wrapper.go. Published atomically by RegisterCostMappings.
var costRegistryPtr atomic.Pointer[costRegistry]

func init() {
	costRegistryPtr.Store(&costRegistry{
		providerMappings:    make(map[string][]core.TokenCostMapping),
		informationalFields: make(map[string]struct{}),
		extendedFieldSet:    make(map[string]struct{}),
	})
}

// loadCostRegistry returns the current cost registry. Never returns nil.
func loadCostRegistry() *costRegistry {
	return costRegistryPtr.Load()
}

// RegisterCostMappings populates the cost registry with provider-specific mappings
// and informational fields. Called once at startup after providers are registered.
func RegisterCostMappings(mappings map[string][]core.TokenCostMapping, informational []string) {
	reg := &costRegistry{
		providerMappings:    mappings,
		informationalFields: make(map[string]struct{}, len(informational)),
		extendedFieldSet:    make(map[string]struct{}),
	}

	for _, f := range informational {
		reg.informationalFields[f] = struct{}{}
	}

	for _, ms := range mappings {
		for _, m := range ms {
			reg.extendedFieldSet[m.RawDataKey] = struct{}{}
		}
	}

	costRegistryPtr.Store(reg)
}

// CalculateGranularCost computes input, output, and total costs from token counts,
// raw provider-specific data, and pricing information. It accounts for cached tokens,
// reasoning tokens, audio tokens, and other provider-specific token types.
//
// The caveat field in the result describes any unmapped token fields or missing pricing
// data that prevented full cost calculation.
func CalculateGranularCost(inputTokens, outputTokens int, rawData map[string]any, providerType string, pricing *core.ModelPricing) CostResult {
	if pricing == nil {
		return CostResult{}
	}

	reg := loadCostRegistry()

	var inputCost, outputCost float64
	var hasInput, hasOutput bool
	var caveats []string

	// Track which RawData keys are mapped
	mappedKeys := make(map[string]bool)

	// Base input cost
	if pricing.InputPerMtok != nil {
		inputCost += float64(inputTokens) * *pricing.InputPerMtok / 1_000_000
		hasInput = true
	}

	// Base output cost
	if pricing.OutputPerMtok != nil {
		outputCost += float64(outputTokens) * *pricing.OutputPerMtok / 1_000_000
		hasOutput = true
	}

	// Apply provider-specific mappings.
	// Track applied pricing field pointers to avoid double-counting when multiple
	// rawData keys map to the same pricing field (e.g. cached_tokens and prompt_cached_tokens
	// both map to CachedInputPerMtok).
	appliedFields := make(map[*float64]bool)
	if mappings, ok := reg.providerMappings[providerType]; ok {
		for _, m := range mappings {
			count := extractInt(rawData, m.RawDataKey)
			if count == 0 {
				continue
			}
			mappedKeys[m.RawDataKey] = true

			rate := m.PricingField(pricing)
			if rate == nil {
				continue // Base rate covers this token type; no adjustment needed
			}

			if appliedFields[rate] {
				continue // Already applied via a different rawData key for the same pricing field
			}
			appliedFields[rate] = true

			var cost float64
			switch m.Unit {
			case core.CostUnitPerMtok:
				cost = float64(count) * *rate / 1_000_000
			case core.CostUnitPerItem:
				cost = float64(count) * *rate
			default:
				caveats = append(caveats, fmt.Sprintf("unknown cost unit %d for field %s", int(m.Unit), m.RawDataKey))
				continue
			}

			switch m.Side {
			case core.CostSideInput:
				inputCost += cost
				hasInput = true
			case core.CostSideOutput:
				outputCost += cost
				hasOutput = true
			default:
				caveats = append(caveats, fmt.Sprintf("unknown cost side %d for field %s", int(m.Side), m.RawDataKey))
			}
		}
	}

	// Check for unmapped token fields in RawData
	for key := range rawData {
		if mappedKeys[key] {
			continue
		}
		if _, ok := reg.informationalFields[key]; ok {
			continue // Known breakdown of base counts, not separately priced
		}
		if isTokenField(key) {
			count := extractInt(rawData, key)
			if count > 0 {
				caveats = append(caveats, fmt.Sprintf("unmapped token field: %s", key))
			}
		}
	}

	// Add per-request flat fee
	if pricing.PerRequest != nil {
		outputCost += *pricing.PerRequest
		hasOutput = true
	}

	result := CostResult{}

	if hasInput {
		result.InputCost = &inputCost
	}
	if hasOutput {
		result.OutputCost = &outputCost
	}
	if hasInput || hasOutput {
		total := inputCost + outputCost
		result.TotalCost = &total
	}

	// Sort caveats for deterministic output
	sort.Strings(caveats)
	result.Caveat = strings.Join(caveats, "; ")

	return result
}

// extractInt extracts an integer value from a map, handling float64, int, and int64 types.
// Returns 0 if the key is not found or the value is not a numeric type.
func extractInt(data map[string]any, key string) int {
	v, ok := data[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	default:
		return 0
	}
}

// isTokenField returns true if the key looks like a token count field.
func isTokenField(key string) bool {
	return strings.HasSuffix(key, "_tokens") || strings.HasSuffix(key, "_count")
}
