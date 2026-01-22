//go:build integration

package integration

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"gomodel/config"
	"gomodel/internal/app"
	"gomodel/internal/core"
	"gomodel/internal/llmclient"
	"gomodel/internal/providers"
)

// TestServerConfig configures how the test server is set up.
type TestServerConfig struct {
	// DBType is either "postgresql" or "mongodb"
	DBType string

	// AuditLogEnabled enables audit logging
	AuditLogEnabled bool

	// UsageEnabled enables usage tracking
	UsageEnabled bool

	// LogBodies enables body logging in audit logs
	LogBodies bool

	// LogHeaders enables header logging in audit logs
	LogHeaders bool

	// OnlyModelInteractions limits logging to model endpoints only
	OnlyModelInteractions bool
}

// TestServerFixture holds test server resources.
type TestServerFixture struct {
	// ServerURL is the base URL of the test server
	ServerURL string

	// App is the running application
	App *app.App

	// MockLLM is the mock LLM server
	MockLLM *MockLLMServer

	// PgPool is the PostgreSQL connection pool (for DB assertions)
	PgPool *pgxpool.Pool

	// MongoDb is the MongoDB database (for DB assertions)
	MongoDb *mongo.Database

	// DBType is the configured database type
	DBType string

	cancelFunc context.CancelFunc
}

// SetupTestServer creates a test server with the specified configuration.
func SetupTestServer(t *testing.T, cfg TestServerConfig) *TestServerFixture {
	t.Helper()

	ctx, cancel := context.WithCancel(GetTestContext())

	// Create mock LLM server
	mockLLM := NewMockLLMServer()

	// Find available port
	port, err := findAvailablePort()
	require.NoError(t, err, "failed to find available port")

	// Build app config
	appCfg := buildAppConfig(t, cfg, mockLLM.URL(), port)

	// Create provider factory
	factory := providers.NewProviderFactory()
	testProvider := NewTestProvider(mockLLM.URL(), "sk-test-key")
	factory.Register(providers.Registration{
		Type: "test",
		New:  func(_ string, _ llmclient.Hooks) core.Provider { return testProvider },
	})

	// Create app
	application, err := app.New(ctx, app.Config{
		AppConfig:       appCfg,
		Factory:         factory,
		RefreshInterval: 1 * time.Hour, // Don't refresh during tests
	})
	require.NoError(t, err, "failed to create app")

	// Start server in background
	serverURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	go func() {
		addr := fmt.Sprintf("127.0.0.1:%d", port)
		_ = application.Start(addr)
	}()

	// Wait for server to be healthy
	err = waitForServer(serverURL + "/health")
	require.NoError(t, err, "server failed to become healthy")

	fixture := &TestServerFixture{
		ServerURL:  serverURL,
		App:        application,
		MockLLM:    mockLLM,
		DBType:     cfg.DBType,
		cancelFunc: cancel,
	}

	// Set database references for assertions
	switch cfg.DBType {
	case "postgresql":
		fixture.PgPool = GetPostgreSQLPool()
	case "mongodb":
		fixture.MongoDb = GetMongoDatabase()
	}

	return fixture
}

// FlushAndClose flushes all pending log entries and closes loggers.
// CRITICAL: Call this before making any DB assertions.
func (f *TestServerFixture) FlushAndClose(t *testing.T) {
	t.Helper()

	// Close the app which flushes all loggers
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if f.App != nil {
		err := f.App.Shutdown(ctx)
		require.NoError(t, err, "failed to shutdown app")
	}
}

// Shutdown gracefully shuts down the test server.
func (f *TestServerFixture) Shutdown(t *testing.T) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if f.App != nil {
		_ = f.App.Shutdown(ctx)
	}

	if f.MockLLM != nil {
		f.MockLLM.Close()
	}

	if f.cancelFunc != nil {
		f.cancelFunc()
	}
}

// buildAppConfig creates an application config for testing.
func buildAppConfig(t *testing.T, cfg TestServerConfig, mockLLMURL string, port int) *config.Config {
	t.Helper()

	appCfg := &config.Config{
		Server: config.ServerConfig{
			Port: fmt.Sprintf("%d", port),
			// No master key for tests (unsafe mode)
		},
		Cache: config.CacheConfig{
			Type: "local",
		},
		Logging: config.LogConfig{
			Enabled:               cfg.AuditLogEnabled,
			LogBodies:             cfg.LogBodies,
			LogHeaders:            cfg.LogHeaders,
			BufferSize:            100,           // Smaller buffer for faster flushing in tests
			FlushInterval:         1,             // 1 second flush interval
			RetentionDays:         0,             // No retention cleanup in tests
			OnlyModelInteractions: cfg.OnlyModelInteractions,
		},
		Usage: config.UsageConfig{
			Enabled:                   cfg.UsageEnabled,
			EnforceReturningUsageData: true,
			BufferSize:                100,
			FlushInterval:             1,
			RetentionDays:             0,
		},
		Metrics: config.MetricsConfig{
			Enabled: false,
		},
		Providers: map[string]config.ProviderConfig{
			"test": {
				Type:    "test",
				APIKey:  "sk-test-key",
				BaseURL: mockLLMURL,
			},
		},
	}

	// Configure storage based on DBType
	switch cfg.DBType {
	case "postgresql":
		appCfg.Storage = config.StorageConfig{
			Type: "postgresql",
			PostgreSQL: config.PostgreSQLStorageConfig{
				URL:      GetPostgreSQLURL(),
				MaxConns: 5,
			},
		}
	case "mongodb":
		appCfg.Storage = config.StorageConfig{
			Type: "mongodb",
			MongoDB: config.MongoDBStorageConfig{
				URL:      GetMongoURL(),
				Database: "gomodel_test",
			},
		}
	default:
		t.Fatalf("unsupported DB type: %s", cfg.DBType)
	}

	return appCfg
}

// waitForServer waits for the server to become healthy.
func waitForServer(healthURL string) error {
	client := &http.Client{Timeout: 2 * time.Second}
	for i := 0; i < 50; i++ {
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

// findAvailablePort finds an available TCP port on loopback.
func findAvailablePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer func() { _ = listener.Close() }()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

// MockLLMServer is a mock LLM server for testing.
type MockLLMServer struct {
	server *httptest.Server
}

// NewMockLLMServer creates a new mock LLM server.
func NewMockLLMServer() *MockLLMServer {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/chat/completions":
			handleChatCompletion(w, r)
		case "/v1/responses":
			handleResponses(w, r)
		case "/v1/models":
			handleModels(w)
		default:
			http.NotFound(w, r)
		}
	})

	server := httptest.NewServer(handler)
	return &MockLLMServer{server: server}
}

// URL returns the server URL.
func (m *MockLLMServer) URL() string {
	return m.server.URL
}

// Close shuts down the server.
func (m *MockLLMServer) Close() {
	m.server.Close()
}

func handleChatCompletion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := `{
		"id": "chatcmpl-test123",
		"object": "chat.completion",
		"created": 1700000000,
		"model": "gpt-4",
		"choices": [{
			"index": 0,
			"message": {
				"role": "assistant",
				"content": "Hello! How can I help you today?"
			},
			"finish_reason": "stop"
		}],
		"usage": {
			"prompt_tokens": 10,
			"completion_tokens": 8,
			"total_tokens": 18
		}
	}`
	_, _ = w.Write([]byte(response))
}

func handleResponses(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := `{
		"id": "resp-test123",
		"object": "response",
		"created_at": 1700000000,
		"model": "gpt-4",
		"output": [{
			"type": "message",
			"id": "msg-test123",
			"role": "assistant",
			"content": [{
				"type": "output_text",
				"text": "Hello! How can I help you today?"
			}]
		}],
		"usage": {
			"input_tokens": 10,
			"output_tokens": 8,
			"total_tokens": 18
		}
	}`
	_, _ = w.Write([]byte(response))
}

func handleModels(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := `{
		"object": "list",
		"data": [
			{"id": "gpt-4", "object": "model", "owned_by": "openai"},
			{"id": "gpt-4.1", "object": "model", "owned_by": "openai"},
			{"id": "gpt-3.5-turbo", "object": "model", "owned_by": "openai"}
		]
	}`
	_, _ = w.Write([]byte(response))
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
	return forwardChatRequest(ctx, p.httpClient, p.baseURL, p.apiKey, req)
}

// StreamChatCompletion forwards the streaming request to the mock server.
func (p *TestProvider) StreamChatCompletion(ctx context.Context, req *core.ChatRequest) (io.ReadCloser, error) {
	// For simplicity, return non-streaming response wrapped in ReadCloser
	resp, err := p.ChatCompletion(ctx, req)
	if err != nil {
		return nil, err
	}
	_ = resp // Would need to convert to SSE format for real streaming tests
	return nil, fmt.Errorf("streaming not implemented in test provider")
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
	return forwardResponsesRequest(ctx, p.httpClient, p.baseURL, p.apiKey, req)
}

// StreamResponses forwards the streaming responses API request to the mock server.
func (p *TestProvider) StreamResponses(ctx context.Context, req *core.ResponsesRequest) (io.ReadCloser, error) {
	return nil, fmt.Errorf("streaming not implemented in test provider")
}
