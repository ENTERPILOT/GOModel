package guardrails

// Catalog resolves named guardrail references into executable pipelines.
type Catalog interface {
	Len() int
	Names() []string
	BuildPipeline(steps []StepReference) (*Pipeline, string, error)
}
