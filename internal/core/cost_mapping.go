package core

// CostSide indicates whether a token cost contributes to input or output.
type CostSide int

const (
	CostSideUnknown CostSide = iota // zero value; must not be used in mappings
	CostSideInput
	CostSideOutput
)

// CostUnit indicates how the pricing field is applied.
type CostUnit int

const (
	CostUnitUnknown CostUnit = iota // zero value; must not be used in mappings
	CostUnitPerMtok                 // divide token count by 1M, multiply by rate
	CostUnitPerItem                 // multiply count directly by rate
)

// TokenCostMapping maps a provider-specific RawData key to a pricing field and cost side.
type TokenCostMapping struct {
	// RawDataKey is the key in the usage RawData map (e.g. "cached_tokens").
	RawDataKey string
	// PricingField returns a pointer to the relevant rate from ModelPricing, or nil
	// if the base rate already covers this token type.
	PricingField func(p *ModelPricing) *float64
	// Side indicates whether this cost contributes to input or output.
	Side CostSide
	// Unit indicates the pricing unit (per million tokens or per item).
	Unit CostUnit
}
