package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"
)

func Run(ctx context.Context, srv *http.Server, sigCh <-chan os.Signal, grace time.Duration, log *slog.Logger) error {
	if log == nil {
		log = slog.Default()
	}

	serveErrCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErrCh <- err
			return
		}
		serveErrCh <- nil
	}()

	select {
	case err := <-serveErrCh:
		return err
	case sig := <-sigCh:
		log.Info("shutdown signal received, draining in-flight requests", "signal", sig.String(), "grace", grace.String())
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, grace)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown grace period exceeded, forcing close", "error", err)
		if closeErr := srv.Close(); closeErr != nil {
			return fmt.Errorf("forced shutdown: close error: %w", closeErr)
		}
		return fmt.Errorf("forced shutdown: grace period of %s exceeded with requests still in flight: %w", grace, err)
	}

	log.Info("shutdown complete")
	return nil
}
