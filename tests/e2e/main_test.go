//go:build e2e

// Package e2e provides end-to-end tests for the LLM gateway.
package e2e

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"gomodel/internal/core"
	"gomodel/internal/providers"
	"gomodel/internal/server"
)

var (
	gatewayURL  string
	mockLLMURL  string
	testServer  *server.Server
	mockServer  *MockLLMServer
	testContext context.Context
	cancelFunc  context.CancelFunc
)

// TestMain sets up and tears down the test environment.
func TestMain(m *testing.M) {
	testContext, cancelFunc = context.WithCancel(context.Background())
	defer cancelFunc()

	// 1. Start mock LLM provider
	mockServer = NewMockLLMServer()
	mockLLMURL = mockServer.URL()

	// 2. Set up environment for test config
	_ = os.Setenv("TEST_MOCK_LLM_URL", mockLLMURL)
	_ = os.Setenv("MOCK_API_KEY", "sk-test-key-12345")

	// 3. Find available port for gateway
	gatewayPort, err := findAvailablePort()
	if err != nil {
		fmt.Printf("Failed to find available port: %v\n", err)
		os.Exit(1)
	}
	gatewayURL = fmt.Sprintf("http://localhost:%d", gatewayPort)

	// 4. Create a test provider and registry
	testProvider := NewTestProvider(mockLLMURL, "sk-test-key-12345")
	registry := providers.NewModelRegistry()
	registry.RegisterProvider(testProvider)

	// Initialize registry to discover models from test provider
	if err := registry.Initialize(testContext); err != nil {
		fmt.Printf("Failed to initialize model registry: %v\n", err)
		os.Exit(1)
	}

	router, err := providers.NewRouter(registry)
	if err != nil {
		fmt.Printf("Failed to create router: %v\n", err)
		os.Exit(1)
	}

	// 5. Start the gateway server
	// Note: No master key for e2e tests (tests run in unsafe mode)
	testServer = server.New(router, &server.Config{})
	go func() {
		addr := fmt.Sprintf(":%d", gatewayPort)
		if err := testServer.Start(addr); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Server error: %v\n", err)
		}
	}()

	// 6. Wait for server to be healthy
	if err := waitForServer(gatewayURL + "/health"); err != nil {
		fmt.Printf("Server failed to start: %v\n", err)
		cleanup()
		os.Exit(1)
	}

	// 7. Run tests
	code := m.Run()

	// 8. Cleanup
	cleanup()
	os.Exit(code)
}

// cleanup shuts down all test resources.
func cleanup() {
	cancelFunc()
	if testServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = testServer.Shutdown(ctx)
	}
	if mockServer != nil {
		mockServer.Close()
	}
}

// waitForServer waits for the server to become healthy.
func waitForServer(healthURL string) error {
	client := &http.Client{Timeout: 2 * time.Second}
	for i := 0; i < 30; i++ {
		resp, err := client.Get(healthURL)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("server did not become healthy within timeout")
}

// findAvailablePort finds an available TCP port.
func findAvailablePort() (int, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}
	defer func() { _ = listener.Close() }()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

// TestProvider is a test provider that forwards requests to the mock LLM server.
type TestProvider struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewTestProvider creates a new test provider.
func NewTestProvider(baseURL, apiKey string) *TestProvider {
	return &TestProvider{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// ChatCompletion forwards the request to the mock server.
func (p *TestProvider) ChatCompletion(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
	return forwardChatRequest(ctx, p.httpClient, p.baseURL, p.apiKey, req, false)
}

// StreamChatCompletion forwards the streaming request to the mock server.
func (p *TestProvider) StreamChatCompletion(ctx context.Context, req *core.ChatRequest) (io.ReadCloser, error) {
	return forwardStreamRequest(ctx, p.httpClient, p.baseURL, p.apiKey, req)
}

// ListModels returns a mock list of models.
func (p *TestProvider) ListModels(ctx context.Context) (*core.ModelsResponse, error) {
	return &core.ModelsResponse{
		Object: "list",
		Data: []core.Model{
			{ID: "gpt-4.1", Object: "model", OwnedBy: "openai"},
			{ID: "gpt-4", Object: "model", OwnedBy: "openai"},
			{ID: "gpt-3.5-turbo", Object: "model", OwnedBy: "openai"},
		},
	}, nil
}

// Responses forwards the responses API request to the mock server.
func (p *TestProvider) Responses(ctx context.Context, req *core.ResponsesRequest) (*core.ResponsesResponse, error) {
	return forwardResponsesRequest(ctx, p.httpClient, p.baseURL, p.apiKey, req, false)
}

// StreamResponses forwards the streaming responses API request to the mock server.
func (p *TestProvider) StreamResponses(ctx context.Context, req *core.ResponsesRequest) (io.ReadCloser, error) {
	return forwardResponsesStreamRequest(ctx, p.httpClient, p.baseURL, p.apiKey, req)
}
