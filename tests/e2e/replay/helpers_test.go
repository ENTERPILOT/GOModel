//go:build e2e

package replay

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// sendJSON sends a JSON POST request to the gateway and returns the response.
func sendJSON(t *testing.T, path string, payload any) *http.Response {
	t.Helper()
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	resp, err := http.Post(gatewayURL+path, "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	return resp
}

// sendGET sends a GET request to the gateway and returns the response.
func sendGET(t *testing.T, path string) *http.Response {
	t.Helper()
	resp, err := http.Get(gatewayURL + path)
	require.NoError(t, err)
	return resp
}

// readJSON reads a response body into the target struct.
func readJSON(t *testing.T, resp *http.Response, target any) {
	t.Helper()
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "failed to read response body")
	require.NoError(t, json.Unmarshal(body, target), "failed to unmarshal response: %s", string(body))
}
