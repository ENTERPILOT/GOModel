package core

// ChatRequest represents the incoming chat completion request
type ChatRequest struct {
	Temperature *float64  `json:"temperature,omitempty"`
	MaxTokens   *int      `json:"max_tokens,omitempty"`
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Stream      bool      `json:"stream,omitempty"`
}

// WithStreaming returns a shallow copy of the request with Stream set to true.
// This avoids mutating the caller's request object.
func (r *ChatRequest) WithStreaming() *ChatRequest {
	return &ChatRequest{
		Temperature: r.Temperature,
		MaxTokens:   r.MaxTokens,
		Model:       r.Model,
		Messages:    r.Messages,
		Stream:      true,
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
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Model represents a single model in the models list
type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by"`
	Created int64  `json:"created"`
}

// ModelsResponse represents the response from the /v1/models endpoint
type ModelsResponse struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}
