//go:build e2e

// Package replay sends requests through the full gateway stack but replays
// pre-recorded golden files as upstream responses. This catches deserialization
// bugs that E2E tests (fabricated mocks) and contract tests (no gateway code) miss.
package replay

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"gomodel/internal/llmclient"
	"gomodel/internal/providers"
	"gomodel/internal/providers/openai"
	"gomodel/internal/server"
)

var gatewayURL string

func TestMain(m *testing.M) {
	// Resolve golden file directory: tests/contract/testdata
	_, thisFile, _, _ := runtime.Caller(0)
	goldenDir := filepath.Join(filepath.Dir(thisFile), "..", "..", "contract", "testdata")

	// Define route table — specific matchers before generic fallbacks
	routes := []GoldenRoute{
		// Chat completions: tool-call variant first
		{Method: "POST", Path: "/chat/completions", Match: matchToolsRequest, GoldenFile: "openai/chat_with_tools.json"},
		{Method: "POST", Path: "/chat/completions", GoldenFile: "openai/chat_completion.json"},
		// Embeddings: base64 variant first
		{Method: "POST", Path: "/embeddings", Match: matchEncodingBase64, GoldenFile: "openai/embeddings_base64.json"},
		{Method: "POST", Path: "/embeddings", GoldenFile: "openai/embeddings.json"},
		// Models
		{Method: "GET", Path: "/models", GoldenFile: "openai/models.json"},
	}

	// 1. Start golden file server
	goldenServer := NewGoldenFileServer(goldenDir, routes)
	defer goldenServer.Close()

	// 2. Create real OpenAI provider pointed at golden server
	provider := openai.NewWithHTTPClient("test-key", nil, llmclient.Hooks{})
	provider.SetBaseURL(goldenServer.URL())

	// 3. Register in model registry and initialize (discovers models from golden file)
	registry := providers.NewModelRegistry()
	registry.RegisterProviderWithType(provider, "openai")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := registry.Initialize(ctx); err != nil {
		cancel()
		goldenServer.Close()
		fmt.Printf("Failed to initialize model registry: %v\n", err)
		os.Exit(1)
	}
	cancel()

	// 4. Create router and gateway server
	router, err := providers.NewRouter(registry)
	if err != nil {
		goldenServer.Close()
		fmt.Printf("Failed to create router: %v\n", err)
		os.Exit(1)
	}

	srv := server.New(router, &server.Config{})

	// 5. Start gateway on a random port
	port, err := findAvailablePort()
	if err != nil {
		goldenServer.Close()
		fmt.Printf("Failed to find available port: %v\n", err)
		os.Exit(1)
	}
	gatewayURL = fmt.Sprintf("http://127.0.0.1:%d", port)

	go func() {
		if err := srv.Start(fmt.Sprintf("127.0.0.1:%d", port)); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Server error: %v\n", err)
		}
	}()

	// 6. Wait for health
	if err := waitForHealth(gatewayURL + "/health"); err != nil {
		goldenServer.Close()
		fmt.Printf("Server failed to start: %v\n", err)
		os.Exit(1)
	}

	// 7. Run tests
	code := m.Run()

	// 8. Cleanup
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	_ = srv.Shutdown(shutdownCtx)
	shutdownCancel()
	goldenServer.Close()

	os.Exit(code)
}

func findAvailablePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func waitForHealth(url string) error {
	client := &http.Client{Timeout: 2 * time.Second}
	for i := 0; i < 30; i++ {
		resp, err := client.Get(url)
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
