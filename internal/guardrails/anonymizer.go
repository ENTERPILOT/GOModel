package guardrails

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"

	"gomodel/internal/core"
)

// Anonymizer handles PII detection and anonymization.
type Anonymizer struct {
	config   AnonymizationConfig
	patterns []PIIPattern
	modelSet map[string]struct{} // Set of models requiring anonymization

	// tokenCounter is used to generate unique token IDs
	tokenCounter uint64
	tokenMu      sync.Mutex
}

// NewAnonymizer creates a new Anonymizer with the given configuration.
func NewAnonymizer(cfg AnonymizationConfig) *Anonymizer {
	a := &Anonymizer{
		config:   cfg,
		patterns: getEnabledPatterns(cfg.Detectors),
		modelSet: make(map[string]struct{}),
	}

	// Build model set for fast lookup
	for _, model := range cfg.Models {
		a.modelSet[model] = struct{}{}
	}

	return a
}

// ShouldAnonymize returns true if the model requires anonymization.
func (a *Anonymizer) ShouldAnonymize(model string) bool {
	// If no models specified, anonymize all (when enabled)
	if len(a.modelSet) == 0 {
		return true
	}
	_, ok := a.modelSet[model]
	return ok
}

// AnonymizeChatRequest anonymizes PII in a ChatRequest.
// Returns a new request with anonymized content and a map for de-anonymization.
func (a *Anonymizer) AnonymizeChatRequest(req *core.ChatRequest) (*core.ChatRequest, map[string]string) {
	tokenMap := make(map[string]string)

	// Create a copy of the request
	newReq := *req
	newReq.Messages = make([]core.Message, len(req.Messages))

	for i, msg := range req.Messages {
		newReq.Messages[i] = core.Message{
			Role:    msg.Role,
			Content: a.anonymizeText(msg.Content, tokenMap),
		}
	}

	return &newReq, tokenMap
}

// AnonymizeResponsesRequest anonymizes PII in a ResponsesRequest.
// Returns a new request with anonymized content and a map for de-anonymization.
func (a *Anonymizer) AnonymizeResponsesRequest(req *core.ResponsesRequest) (*core.ResponsesRequest, map[string]string) {
	tokenMap := make(map[string]string)

	// Create a copy of the request
	newReq := *req

	// Anonymize instructions
	if req.Instructions != "" {
		newReq.Instructions = a.anonymizeText(req.Instructions, tokenMap)
	}

	// Anonymize input based on its type
	newReq.Input = a.anonymizeInput(req.Input, tokenMap)

	return &newReq, tokenMap
}

// anonymizeInput handles the polymorphic Input field of ResponsesRequest.
func (a *Anonymizer) anonymizeInput(input interface{}, tokenMap map[string]string) interface{} {
	if input == nil {
		return nil
	}

	// Input can be a string
	if s, ok := input.(string); ok {
		return a.anonymizeText(s, tokenMap)
	}

	// Input can be []interface{} (from JSON unmarshaling)
	if items, ok := input.([]interface{}); ok {
		newItems := make([]interface{}, len(items))
		for i, item := range items {
			newItems[i] = a.anonymizeInputItem(item, tokenMap)
		}
		return newItems
	}

	// Input can be []ResponsesInputItem
	if items, ok := input.([]core.ResponsesInputItem); ok {
		newItems := make([]core.ResponsesInputItem, len(items))
		for i, item := range items {
			newItems[i] = core.ResponsesInputItem{
				Role:    item.Role,
				Content: a.anonymizeInputContent(item.Content, tokenMap),
			}
		}
		return newItems
	}

	return input
}

// anonymizeInputItem handles a single item in the input array.
func (a *Anonymizer) anonymizeInputItem(item interface{}, tokenMap map[string]string) interface{} {
	// Handle map[string]interface{} from JSON
	if m, ok := item.(map[string]interface{}); ok {
		newM := make(map[string]interface{})
		for k, v := range m {
			if k == "content" {
				newM[k] = a.anonymizeInputContent(v, tokenMap)
			} else {
				newM[k] = v
			}
		}
		return newM
	}
	return item
}

// anonymizeInputContent handles the polymorphic content field.
func (a *Anonymizer) anonymizeInputContent(content interface{}, tokenMap map[string]string) interface{} {
	if content == nil {
		return nil
	}

	// Content can be a string
	if s, ok := content.(string); ok {
		return a.anonymizeText(s, tokenMap)
	}

	// Content can be []interface{} (content parts)
	if parts, ok := content.([]interface{}); ok {
		newParts := make([]interface{}, len(parts))
		for i, part := range parts {
			newParts[i] = a.anonymizeContentPart(part, tokenMap)
		}
		return newParts
	}

	return content
}

// anonymizeContentPart handles a content part (text, image, etc.).
func (a *Anonymizer) anonymizeContentPart(part interface{}, tokenMap map[string]string) interface{} {
	if m, ok := part.(map[string]interface{}); ok {
		newM := make(map[string]interface{})
		for k, v := range m {
			if k == "text" {
				if s, ok := v.(string); ok {
					newM[k] = a.anonymizeText(s, tokenMap)
				} else {
					newM[k] = v
				}
			} else {
				newM[k] = v
			}
		}
		return newM
	}
	return part
}

// anonymizeText detects and replaces PII in text.
// Returns the anonymized text and updates the tokenMap with mappings.
func (a *Anonymizer) anonymizeText(text string, tokenMap map[string]string) string {
	if text == "" {
		return text
	}

	result := text

	for _, pattern := range a.patterns {
		matches := pattern.Pattern.FindAllString(result, -1)
		for _, match := range matches {
			// Check if we've already tokenized this value
			var token string
			found := false
			for t, orig := range tokenMap {
				if orig == match {
					token = t
					found = true
					break
				}
			}

			if !found {
				token = a.generateToken(pattern.Type, match)
				tokenMap[token] = match
			}

			result = strings.ReplaceAll(result, match, token)
		}
	}

	return result
}

// generateToken creates a replacement token for a PII value.
func (a *Anonymizer) generateToken(piiType PIIType, value string) string {
	switch a.config.Strategy {
	case StrategyHash:
		// Use first 8 chars of SHA256 hash
		hash := sha256.Sum256([]byte(value))
		return fmt.Sprintf("[%s_%s]", piiType, hex.EncodeToString(hash[:])[:8])

	case StrategyMask:
		// Mask with asterisks, keeping first/last char visible
		if len(value) <= 2 {
			return fmt.Sprintf("[%s_***]", piiType)
		}
		return fmt.Sprintf("[%s_%c***%c]", piiType, value[0], value[len(value)-1])

	case StrategyToken:
		fallthrough
	default:
		// Generate unique token ID
		a.tokenMu.Lock()
		a.tokenCounter++
		id := a.tokenCounter
		a.tokenMu.Unlock()
		return fmt.Sprintf("[%s_%x]", piiType, id)
	}
}

// DeanonymizeChatResponse restores original PII values in a ChatResponse.
func (a *Anonymizer) DeanonymizeChatResponse(resp *core.ChatResponse, tokenMap map[string]string) *core.ChatResponse {
	if len(tokenMap) == 0 {
		return resp
	}

	// Create a copy
	newResp := *resp
	newResp.Choices = make([]core.Choice, len(resp.Choices))

	for i, choice := range resp.Choices {
		newResp.Choices[i] = core.Choice{
			Index:        choice.Index,
			FinishReason: choice.FinishReason,
			Message: core.Message{
				Role:    choice.Message.Role,
				Content: a.deanonymizeText(choice.Message.Content, tokenMap),
			},
		}
	}

	return &newResp
}

// DeanonymizeResponsesResponse restores original PII values in a ResponsesResponse.
func (a *Anonymizer) DeanonymizeResponsesResponse(resp *core.ResponsesResponse, tokenMap map[string]string) *core.ResponsesResponse {
	if len(tokenMap) == 0 {
		return resp
	}

	// Create a copy
	newResp := *resp
	newResp.Output = make([]core.ResponsesOutputItem, len(resp.Output))

	for i, item := range resp.Output {
		newItem := core.ResponsesOutputItem{
			ID:     item.ID,
			Type:   item.Type,
			Role:   item.Role,
			Status: item.Status,
		}

		if len(item.Content) > 0 {
			newItem.Content = make([]core.ResponsesContentItem, len(item.Content))
			for j, content := range item.Content {
				newItem.Content[j] = core.ResponsesContentItem{
					Type:        content.Type,
					Text:        a.deanonymizeText(content.Text, tokenMap),
					Annotations: content.Annotations,
				}
			}
		}

		newResp.Output[i] = newItem
	}

	return &newResp
}

// deanonymizeText replaces tokens with original values.
func (a *Anonymizer) deanonymizeText(text string, tokenMap map[string]string) string {
	if text == "" || len(tokenMap) == 0 {
		return text
	}

	result := text
	for token, original := range tokenMap {
		result = strings.ReplaceAll(result, token, original)
	}
	return result
}
