// Package main is the entry point for the LLM gateway server.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"gomodel/config"
	"gomodel/internal/app"
	"gomodel/internal/observability"
	"gomodel/internal/providers"
	"gomodel/internal/providers/anthropic"
	"gomodel/internal/providers/gemini"
	"gomodel/internal/providers/groq"
	"gomodel/internal/providers/ollama"
	"gomodel/internal/providers/openai"
	"gomodel/internal/providers/xai"
	"gomodel/internal/version"

	"github.com/lmittmann/tint"
	"golang.org/x/term"
)

// @title          GOModel API
// @version        1.0
// @description    High-performance AI gateway routing requests to multiple LLM providers (OpenAI, Anthropic, Gemini, Groq, xAI, Ollama). Drop-in OpenAI-compatible API.
// @BasePath       /
// @schemes        http
// @securityDefinitions.apikey BearerAuth
// @in             header
// @name           Authorization
func main() {
	versionFlag := flag.Bool("version", false, "Print version information")
	flag.Parse()

	if *versionFlag {
		fmt.Println(version.Info())
		os.Exit(0)
	}

	isTTYTerminal := term.IsTerminal(int(os.Stderr.Fd()))

	logFormat := strings.ToLower(os.Getenv("LOG_FORMAT"))

	var handler slog.Handler = slog.NewJSONHandler(os.Stderr, nil)

	if (isTTYTerminal && logFormat != "json") || logFormat == "text" {
		handler = tint.NewHandler(os.Stderr, &tint.Options{
			TimeFormat: time.Kitchen,
			NoColor:    !isTTYTerminal,
		})
	}

	slog.SetDefault(slog.New(handler))

	slog.Info("starting gomodel",
		"version", version.Version,
		"commit", version.Commit,
		"build_date", version.Date,
	)

	result, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	factory := providers.NewProviderFactory()

	if result.Config.Metrics.Enabled {
		factory.SetHooks(observability.NewPrometheusHooks())
	}

	factory.Add(openai.Registration)
	factory.Add(anthropic.Registration)
	factory.Add(gemini.Registration)
	factory.Add(groq.Registration)
	factory.Add(ollama.Registration)
	factory.Add(xai.Registration)

	application, err := app.New(context.Background(), app.Config{
		AppConfig: result,
		Factory:   factory,
	})
	if err != nil {
		slog.Error("failed to initialize application", "error", err)
		os.Exit(1)
	}

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := application.Shutdown(ctx); err != nil {
			slog.Error("application shutdown error", "error", err)
		}
	}()

	addr := ":" + result.Config.Server.Port
	if err := application.Start(addr); err != nil {
		slog.Error("application failed", "error", err)
		os.Exit(1)
	}
}
