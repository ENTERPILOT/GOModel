package core

// CostSide indicates whether a token cost contributes to input or output.
type CostSide int

const (
	CostSideInput  CostSide = iota
	CostSideOutput
)

// CostUnit indicates how the pricing field is applied.
type CostUnit int

const (
	CostUnitPerMtok CostUnit = iota // divide token count by 1M, multiply by rate
	CostUnitPerItem                 // multiply count directly by rate
)

// TokenCostMapping maps a RawData key to a pricing field and cost side.
type TokenCostMapping struct {
	RawDataKey   string
	PricingField func(p *ModelPricing) *float64
	Side         CostSide
	Unit         CostUnit
}
