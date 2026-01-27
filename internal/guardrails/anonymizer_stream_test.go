package guardrails

import (
	"io"
	"strings"
	"testing"
)

func TestDeanonymizingReader_BasicReplacement(t *testing.T) {
	tokenMap := map[string]string{
		"[EMAIL_1]": "test@example.com",
		"[PHONE_1]": "555-123-4567",
	}

	input := `data: {"content":"Contact [EMAIL_1] or call [PHONE_1]"}

data: [DONE]

`
	reader := NewDeanonymizingReader(io.NopCloser(strings.NewReader(input)), tokenMap)
	defer reader.Close()

	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	result := string(output)

	// Check tokens are replaced
	if strings.Contains(result, "[EMAIL_1]") {
		t.Error("output should not contain [EMAIL_1]")
	}
	if strings.Contains(result, "[PHONE_1]") {
		t.Error("output should not contain [PHONE_1]")
	}

	// Check original values are present
	if !strings.Contains(result, "test@example.com") {
		t.Error("output should contain original email")
	}
	if !strings.Contains(result, "555-123-4567") {
		t.Error("output should contain original phone")
	}
}

func TestDeanonymizingReader_SSEFormat(t *testing.T) {
	tokenMap := map[string]string{
		"[EMAIL_1]": "user@test.com",
	}

	input := `data: {"id":"1","choices":[{"delta":{"content":"Email: [EMAIL_1]"}}]}

data: {"id":"2","choices":[{"delta":{"content":" is valid"}}]}

data: [DONE]

`
	reader := NewDeanonymizingReader(io.NopCloser(strings.NewReader(input)), tokenMap)
	defer reader.Close()

	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	result := string(output)

	if !strings.Contains(result, "user@test.com") {
		t.Error("output should contain de-anonymized email")
	}
	if strings.Contains(result, "[EMAIL_1]") {
		t.Error("output should not contain token")
	}
	if !strings.Contains(result, "[DONE]") {
		t.Error("output should preserve [DONE] marker")
	}
}

func TestDeanonymizingReader_NoTokens(t *testing.T) {
	tokenMap := map[string]string{}

	input := `data: {"content":"Hello world"}

data: [DONE]

`
	reader := NewDeanonymizingReader(io.NopCloser(strings.NewReader(input)), tokenMap)
	defer reader.Close()

	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	result := string(output)

	if !strings.Contains(result, "Hello world") {
		t.Error("output should preserve original content")
	}
}

func TestDeanonymizingReader_MultipleChunks(t *testing.T) {
	tokenMap := map[string]string{
		"[EMAIL_1]": "test@example.com",
	}

	input := `data: {"content":"Part 1 [EMAIL_1]"}

data: {"content":"Part 2 [EMAIL_1]"}

data: [DONE]

`
	reader := NewDeanonymizingReader(io.NopCloser(strings.NewReader(input)), tokenMap)
	defer reader.Close()

	// Read in small chunks to test buffering
	var result strings.Builder
	buf := make([]byte, 20)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			result.Write(buf[:n])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Read failed: %v", err)
		}
	}

	output := result.String()

	// Count occurrences of de-anonymized email
	count := strings.Count(output, "test@example.com")
	if count != 2 {
		t.Errorf("expected 2 occurrences of email, got %d. Output: %s", count, output)
	}

	if strings.Contains(output, "[EMAIL_1]") {
		t.Error("output should not contain token")
	}
}

func TestDeanonymizingReader_Close(t *testing.T) {
	closed := false
	mockCloser := &mockReadCloser{
		Reader: strings.NewReader("data: test\n\n"),
		onClose: func() error {
			closed = true
			return nil
		},
	}

	tokenMap := map[string]string{}
	reader := NewDeanonymizingReader(mockCloser, tokenMap)

	_, _ = io.ReadAll(reader)
	_ = reader.Close()

	if !closed {
		t.Error("underlying reader should be closed")
	}
}

// mockReadCloser is a test helper
type mockReadCloser struct {
	io.Reader
	onClose func() error
}

func (m *mockReadCloser) Close() error {
	if m.onClose != nil {
		return m.onClose()
	}
	return nil
}
