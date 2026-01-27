package guardrails

import (
	"io"
	"strings"
)

// DeanonymizingReader wraps a stream and replaces tokens with original values.
// It buffers data to handle partial tokens split across chunk boundaries.
type DeanonymizingReader struct {
	reader   io.ReadCloser
	tokenMap map[string]string

	// Buffer for handling tokens split across reads
	buffer      []byte
	maxTokenLen int
	outputBuf   []byte // Buffer for output that didn't fit in previous Read

	// Token list for replacement
	tokens []string
}

// NewDeanonymizingReader creates a new DeanonymizingReader.
func NewDeanonymizingReader(reader io.ReadCloser, tokenMap map[string]string) *DeanonymizingReader {
	r := &DeanonymizingReader{
		reader:   reader,
		tokenMap: tokenMap,
		tokens:   make([]string, 0, len(tokenMap)),
	}

	// Build token list and find max token length
	for token := range tokenMap {
		r.tokens = append(r.tokens, token)
		if len(token) > r.maxTokenLen {
			r.maxTokenLen = len(token)
		}
	}

	return r
}

// Read implements io.Reader with token replacement.
func (r *DeanonymizingReader) Read(p []byte) (n int, err error) {
	// First, drain any leftover output from previous Read
	if len(r.outputBuf) > 0 {
		n = copy(p, r.outputBuf)
		r.outputBuf = r.outputBuf[n:]
		return n, nil
	}

	// Read more data from underlying reader
	buf := make([]byte, 4096) // Use a reasonably sized read buffer
	nRead, readErr := r.reader.Read(buf)

	if nRead == 0 && readErr != nil {
		// Flush any remaining buffer on EOF
		if len(r.buffer) > 0 && readErr == io.EOF {
			replaced := r.replaceTokens(string(r.buffer))
			r.buffer = nil
			return r.writeOutput(p, []byte(replaced), io.EOF)
		}
		return 0, readErr
	}

	// Combine buffer with new data
	data := append(r.buffer, buf[:nRead]...)
	r.buffer = nil

	// Determine safe boundary for processing
	// Keep back enough data that could contain a partial token
	if r.maxTokenLen > 0 && len(data) > r.maxTokenLen {
		// Check if the last maxTokenLen bytes could contain a partial token
		holdBack := r.maxTokenLen
		lastPart := string(data[len(data)-holdBack:])

		// If there's an unclosed bracket, buffer it for next read
		openIdx := strings.LastIndex(lastPart, "[")
		if openIdx >= 0 && !strings.Contains(lastPart[openIdx:], "]") {
			safeLen := len(data) - holdBack + openIdx
			r.buffer = data[safeLen:]
			data = data[:safeLen]
		}
	}

	// Replace tokens in the safe portion
	replaced := r.replaceTokens(string(data))

	// If underlying reader returned EOF and we have no more buffer, pass it through
	finalErr := readErr
	if readErr == io.EOF && len(r.buffer) > 0 {
		finalErr = nil // More data to come from buffer
	}

	return r.writeOutput(p, []byte(replaced), finalErr)
}

// writeOutput writes data to p, buffering any overflow
func (r *DeanonymizingReader) writeOutput(p []byte, data []byte, err error) (int, error) {
	n := copy(p, data)
	if n < len(data) {
		r.outputBuf = append(r.outputBuf, data[n:]...)
	}

	// Only return error if there's no more data to output
	if len(r.outputBuf) > 0 || len(r.buffer) > 0 {
		return n, nil
	}
	return n, err
}

// replaceTokens replaces all tokens with original values.
func (r *DeanonymizingReader) replaceTokens(text string) string {
	result := text
	for _, token := range r.tokens {
		if original, ok := r.tokenMap[token]; ok {
			result = strings.ReplaceAll(result, token, original)
		}
	}
	return result
}

// Close closes the underlying reader.
func (r *DeanonymizingReader) Close() error {
	return r.reader.Close()
}
