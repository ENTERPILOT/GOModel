package providers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// OpenAIResponsesStreamConverter wraps an OpenAI-compatible SSE stream
// and converts it to Responses API format.
// Used by providers that have OpenAI-compatible streaming (Groq, Gemini, etc.)
type OpenAIResponsesStreamConverter struct {
	reader     io.ReadCloser
	model      string
	responseID string
	buffer     []byte
	lineBuffer []byte
	closed     bool
	sentCreate bool
	sentDone   bool
}

// NewOpenAIResponsesStreamConverter creates a new converter that transforms
// OpenAI-format SSE streams to Responses API format.
func NewOpenAIResponsesStreamConverter(reader io.ReadCloser, model string) *OpenAIResponsesStreamConverter {
	return &OpenAIResponsesStreamConverter{
		reader:     reader,
		model:      model,
		responseID: "resp_" + uuid.New().String(),
		buffer:     make([]byte, 0, 4096),
		lineBuffer: make([]byte, 0, 1024),
	}
}

func (sc *OpenAIResponsesStreamConverter) Read(p []byte) (n int, err error) {
	if sc.closed {
		return 0, io.EOF
	}

	// If we have buffered data, return it first
	if len(sc.buffer) > 0 {
		n = copy(p, sc.buffer)
		sc.buffer = sc.buffer[n:]
		return n, nil
	}

	// Send response.created event first
	if !sc.sentCreate {
		sc.sentCreate = true
		createdEvent := map[string]interface{}{
			"type": "response.created",
			"response": map[string]interface{}{
				"id":         sc.responseID,
				"object":     "response",
				"status":     "in_progress",
				"model":      sc.model,
				"created_at": time.Now().Unix(),
			},
		}
		jsonData, err := json.Marshal(createdEvent)
		if err != nil {
			slog.Error("failed to marshal response.created event", "error", err, "response_id", sc.responseID)
			return 0, nil
		}
		created := fmt.Sprintf("event: response.created\ndata: %s\n\n", jsonData)
		sc.buffer = append(sc.buffer, []byte(created)...)
		n = copy(p, sc.buffer)
		sc.buffer = sc.buffer[n:]
		return n, nil
	}

	// Read from the underlying stream
	tempBuf := make([]byte, 1024)
	nr, readErr := sc.reader.Read(tempBuf)
	if nr > 0 {
		sc.lineBuffer = append(sc.lineBuffer, tempBuf[:nr]...)

		// Process complete lines
		for {
			idx := bytes.Index(sc.lineBuffer, []byte("\n"))
			if idx == -1 {
				break
			}

			line := sc.lineBuffer[:idx]
			sc.lineBuffer = sc.lineBuffer[idx+1:]

			line = bytes.TrimSpace(line)
			if len(line) == 0 {
				continue
			}

			if bytes.HasPrefix(line, []byte("data: ")) {
				data := bytes.TrimPrefix(line, []byte("data: "))
				if bytes.Equal(data, []byte("[DONE]")) {
					// Send done event
					if !sc.sentDone {
						sc.sentDone = true
						doneEvent := map[string]interface{}{
							"type": "response.done",
							"response": map[string]interface{}{
								"id":         sc.responseID,
								"object":     "response",
								"status":     "completed",
								"model":      sc.model,
								"created_at": time.Now().Unix(),
							},
						}
						jsonData, err := json.Marshal(doneEvent)
						if err != nil {
							slog.Error("failed to marshal response.done event", "error", err, "response_id", sc.responseID)
							continue
						}
						doneMsg := fmt.Sprintf("event: response.done\ndata: %s\n\ndata: [DONE]\n\n", jsonData)
						sc.buffer = append(sc.buffer, []byte(doneMsg)...)
					}
					continue
				}

				// Parse the chat completion chunk
				var chunk map[string]interface{}
				if err := json.Unmarshal(data, &chunk); err != nil {
					continue
				}

				// Extract content delta
				if choices, ok := chunk["choices"].([]interface{}); ok && len(choices) > 0 {
					if choice, ok := choices[0].(map[string]interface{}); ok {
						if delta, ok := choice["delta"].(map[string]interface{}); ok {
							if content, ok := delta["content"].(string); ok && content != "" {
								deltaEvent := map[string]interface{}{
									"type":  "response.output_text.delta",
									"delta": content,
								}
								jsonData, err := json.Marshal(deltaEvent)
								if err != nil {
									slog.Error("failed to marshal content delta event", "error", err, "response_id", sc.responseID)
									continue
								}
								sc.buffer = append(sc.buffer, []byte(fmt.Sprintf("event: response.output_text.delta\ndata: %s\n\n", jsonData))...)
							}
						}
					}
				}
			}
		}
	}

	if readErr != nil {
		if readErr == io.EOF {
			// Send final done event if we haven't already
			if !sc.sentDone {
				sc.sentDone = true
				doneEvent := map[string]interface{}{
					"type": "response.done",
					"response": map[string]interface{}{
						"id":         sc.responseID,
						"object":     "response",
						"status":     "completed",
						"model":      sc.model,
						"created_at": time.Now().Unix(),
					},
				}
				jsonData, err := json.Marshal(doneEvent)
				if err != nil {
					slog.Error("failed to marshal final response.done event", "error", err, "response_id", sc.responseID)
				} else {
					doneMsg := fmt.Sprintf("event: response.done\ndata: %s\n\ndata: [DONE]\n\n", jsonData)
					sc.buffer = append(sc.buffer, []byte(doneMsg)...)
				}
			}

			if len(sc.buffer) > 0 {
				n = copy(p, sc.buffer)
				sc.buffer = sc.buffer[n:]
				return n, nil
			}

			sc.closed = true
			_ = sc.reader.Close()
			return 0, io.EOF
		}
		return 0, readErr
	}

	if len(sc.buffer) > 0 {
		n = copy(p, sc.buffer)
		sc.buffer = sc.buffer[n:]
		return n, nil
	}

	// No data yet, try again
	return 0, nil
}

func (sc *OpenAIResponsesStreamConverter) Close() error {
	sc.closed = true
	return sc.reader.Close()
}
