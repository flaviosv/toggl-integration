package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/flaviosv/toggl-integration/to-jira/internal/routes"
	"github.com/flaviosv/toggl-integration/to-jira/internal/shared/config"
	"github.com/flaviosv/toggl-integration/to-jira/internal/shared/di"
	"github.com/flaviosv/toggl-integration/to-jira/internal/shared/logger"
	"github.com/flaviosv/toggl-integration/to-jira/internal/shared/server"
	"github.com/flaviosv/toggl-integration/to-jira/internal/shared/telemetry"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 20 * time.Second
	writeTimeout      = 25 * time.Second
	idleTimeout       = 60 * time.Second
	maxHeaderBytes    = 1 << 20
	shutdownGrace     = 10 * time.Second
	logEnv            = "production"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	baseLogger := logger.Initialize(logEnv)

	shutdownTelemetry, err := telemetry.Initialize(context.Background(), telemetry.Config{OTLPEndpoint: cfg.OtelExporterOTLPEndpoint})
	if err != nil {
		log.Fatalf("telemetry: %v", err)
	}

	app, err := buildApp(cfg, baseLogger)
	if err != nil {
		log.Fatalf("bootstrap: %v", err)
	}

	httpServer := buildServer(cfg, app)

	baseLogger.Info("to-jira starting", "port", cfg.Port, "dry_run", cfg.DryRun)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	serverErr := server.Run(context.Background(), httpServer, sigCh, shutdownGrace, baseLogger)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()
	if err := shutdownTelemetry(shutdownCtx); err != nil {
		baseLogger.Error("telemetry shutdown failed", "error", err)
	}

	if serverErr != nil {
		log.Fatalf("server: %v", serverErr)
	}
}

func buildApp(cfg *config.Config, baseLogger *slog.Logger) (*gin.Engine, error) {
	deps, err := di.BuildDependencies(cfg, baseLogger)
	if err != nil {
		return nil, fmt.Errorf("build dependencies: %w", err)
	}

	app := gin.New()
	app.Use(gin.Recovery())

	v1 := app.Group("/")
	routes.Routes(v1, deps.Handlers.Webhook)

	return app, nil
}

func buildServer(cfg *config.Config, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
	}
}
