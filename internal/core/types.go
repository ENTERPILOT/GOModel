package core

// StreamOptions controls streaming behavior options.
// This is used to request usage data in streaming responses.
type StreamOptions struct {
	// IncludeUsage requests token usage information in streaming responses.
	// When true, the final streaming chunk will include usage statistics.
	IncludeUsage bool `json:"include_usage,omitempty"`
}

// Reasoning configures reasoning behavior for models that support extended thinking.
// This is used with OpenAI's o-series models and other reasoning-capable models.
type Reasoning struct {
	// Effort controls how much reasoning effort the model should use.
	// Valid values are "low", "medium", and "high".
	Effort string `json:"effort,omitempty"`
}

// ChatRequest represents the incoming chat completion request
type ChatRequest struct {
	Temperature   *float64       `json:"temperature,omitempty"`
	MaxTokens     *int           `json:"max_tokens,omitempty"`
	Model         string         `json:"model"`
	Messages      []Message      `json:"messages"`
	Stream        bool           `json:"stream,omitempty"`
	StreamOptions *StreamOptions `json:"stream_options,omitempty"`
	Reasoning     *Reasoning     `json:"reasoning,omitempty"`
}

// WithStreaming returns a shallow copy of the request with Stream set to true.
// This avoids mutating the caller's request object.
func (r *ChatRequest) WithStreaming() *ChatRequest {
	return &ChatRequest{
		Temperature:   r.Temperature,
		MaxTokens:     r.MaxTokens,
		Model:         r.Model,
		Messages:      r.Messages,
		Stream:        true,
		StreamOptions: r.StreamOptions,
		Reasoning:     r.Reasoning,
	}
}

// Message represents a single message in the chat
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatResponse represents the chat completion response
type ChatResponse struct {
	ID       string   `json:"id"`
	Object   string   `json:"object"`
	Model    string   `json:"model"`
	Provider string   `json:"provider"`
	Choices  []Choice `json:"choices"`
	Usage    Usage    `json:"usage"`
	Created  int64    `json:"created"`
}

// Choice represents a single completion choice
type Choice struct {
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
	Index        int     `json:"index"`
}

// Usage represents token usage information
type Usage struct {
	PromptTokens     int            `json:"prompt_tokens"`
	CompletionTokens int            `json:"completion_tokens"`
	TotalTokens      int            `json:"total_tokens"`
	RawUsage         map[string]any `json:"raw_usage,omitempty"`
}

// Model represents a single model in the models list
type Model struct {
	ID       string         `json:"id"`
	Object   string         `json:"object"`
	OwnedBy  string         `json:"owned_by"`
	Created  int64          `json:"created"`
	Metadata *ModelMetadata `json:"metadata,omitempty"`
}

// ModelMetadata holds enriched metadata from the external model registry.
type ModelMetadata struct {
	DisplayName     string          `json:"display_name,omitempty"`
	Description     string          `json:"description,omitempty"`
	Family          string          `json:"family,omitempty"`
	Mode            string          `json:"mode,omitempty"`
	Category        ModelCategory   `json:"category,omitempty"`
	Tags            []string        `json:"tags,omitempty"`
	ContextWindow   *int            `json:"context_window,omitempty"`
	MaxOutputTokens *int            `json:"max_output_tokens,omitempty"`
	Capabilities    map[string]bool `json:"capabilities,omitempty"`
	Pricing         *ModelPricing   `json:"pricing,omitempty"`
}

// ModelCategory represents a model's functional category for UI grouping.
type ModelCategory string

const (
	CategoryAll            ModelCategory = "all"
	CategoryTextGeneration ModelCategory = "text_generation"
	CategoryEmbedding      ModelCategory = "embedding"
	CategoryImage          ModelCategory = "image"
	CategoryAudio          ModelCategory = "audio"
	CategoryVideo          ModelCategory = "video"
	CategoryUtility        ModelCategory = "utility"
)

// modeToCategory maps mode strings from the external registry to categories.
var modeToCategory = map[string]ModelCategory{
	"chat":                 CategoryTextGeneration,
	"completion":           CategoryTextGeneration,
	"responses":            CategoryTextGeneration,
	"embedding":            CategoryEmbedding,
	"rerank":               CategoryEmbedding,
	"image_generation":     CategoryImage,
	"image_edit":           CategoryImage,
	"audio_transcription":  CategoryAudio,
	"audio_speech":         CategoryAudio,
	"video_generation":     CategoryVideo,
	"moderation":           CategoryUtility,
	"ocr":                  CategoryUtility,
	"search":               CategoryUtility,
}

// CategoryForMode returns the ModelCategory for a given mode string.
// Returns empty string if the mode is not recognized.
func CategoryForMode(mode string) ModelCategory {
	return modeToCategory[mode]
}

// AllCategories returns the ordered list of categories for UI rendering.
func AllCategories() []ModelCategory {
	return []ModelCategory{
		CategoryAll,
		CategoryTextGeneration,
		CategoryEmbedding,
		CategoryImage,
		CategoryAudio,
		CategoryVideo,
		CategoryUtility,
	}
}

// ModelPricing holds pricing information for cost calculation.
type ModelPricing struct {
	Currency               string   `json:"currency"`
	InputPerMtok           *float64 `json:"input_per_mtok,omitempty"`
	OutputPerMtok          *float64 `json:"output_per_mtok,omitempty"`
	CachedInputPerMtok     *float64 `json:"cached_input_per_mtok,omitempty"`
	ReasoningOutputPerMtok *float64 `json:"reasoning_output_per_mtok,omitempty"`
	PerImage               *float64 `json:"per_image,omitempty"`
	PerSecondInput         *float64 `json:"per_second_input,omitempty"`
	PerSecondOutput        *float64 `json:"per_second_output,omitempty"`
	PerCharacterInput      *float64 `json:"per_character_input,omitempty"`
	PerRequest             *float64 `json:"per_request,omitempty"`
	PerPage                *float64 `json:"per_page,omitempty"`
}

// ModelsResponse represents the response from the /v1/models endpoint
type ModelsResponse struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}
