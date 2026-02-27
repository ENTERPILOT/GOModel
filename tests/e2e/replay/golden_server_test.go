//go:build e2e

package replay

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
)

// RequestMatcher inspects a request body to decide if a route matches.
// Return true to claim the route.
type RequestMatcher func(body []byte) bool

// GoldenRoute maps an HTTP method + path (+ optional body matcher) to a golden file.
type GoldenRoute struct {
	Method      string
	Path        string
	Match       RequestMatcher // nil = always matches
	GoldenFile  string         // relative to goldenDir
	StatusCode  int            // 0 defaults to 200
	ContentType string         // defaults to "application/json"
}

// GoldenFileServer serves pre-recorded golden files as HTTP responses.
type GoldenFileServer struct {
	server   *httptest.Server
	routes   []GoldenRoute
	golden   string // absolute path to golden file directory
	mu       sync.Mutex
	requests []recordedRequest
}

type recordedRequest struct {
	Method string
	Path   string
	Body   []byte
}

// NewGoldenFileServer creates a server that replays golden files.
// Routes are evaluated in order; first match wins.
func NewGoldenFileServer(goldenDir string, routes []GoldenRoute) *GoldenFileServer {
	g := &GoldenFileServer{
		golden: goldenDir,
		routes: routes,
	}
	g.server = httptest.NewServer(http.HandlerFunc(g.handler))
	return g
}

func (g *GoldenFileServer) handler(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	_ = r.Body.Close()

	g.mu.Lock()
	g.requests = append(g.requests, recordedRequest{
		Method: r.Method,
		Path:   r.URL.Path,
		Body:   body,
	})
	g.mu.Unlock()

	for _, route := range g.routes {
		if !strings.EqualFold(route.Method, r.Method) {
			continue
		}
		if route.Path != r.URL.Path {
			continue
		}
		if route.Match != nil && !route.Match(body) {
			continue
		}

		data, err := os.ReadFile(g.golden + "/" + route.GoldenFile)
		if err != nil {
			http.Error(w, "golden file not found: "+route.GoldenFile, http.StatusInternalServerError)
			return
		}

		ct := route.ContentType
		if ct == "" {
			ct = "application/json"
		}
		status := route.StatusCode
		if status == 0 {
			status = http.StatusOK
		}

		w.Header().Set("Content-Type", ct)
		w.WriteHeader(status)
		_, _ = w.Write(data)
		return
	}

	http.Error(w, "no matching golden route for "+r.Method+" "+r.URL.Path, http.StatusNotFound)
}

// URL returns the base URL of the golden file server.
func (g *GoldenFileServer) URL() string {
	return g.server.URL
}

// Close shuts down the server.
func (g *GoldenFileServer) Close() {
	g.server.Close()
}

// matchEncodingBase64 returns true if the request body contains "encoding_format":"base64".
func matchEncodingBase64(body []byte) bool {
	var req struct {
		EncodingFormat string `json:"encoding_format"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return false
	}
	return req.EncodingFormat == "base64"
}

// matchToolsRequest returns true if the request body contains a "tools" field.
func matchToolsRequest(body []byte) bool {
	var req struct {
		Tools json.RawMessage `json:"tools"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return false
	}
	return len(req.Tools) > 0 && string(req.Tools) != "null"
}
