package server

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name   string
		testFn func(t *testing.T)
	}{
		{
			name:   "ListenAndServe error propagation - address already in use",
			testFn: testListenAndServeError,
		},
		{
			name:   "Signal-triggered graceful shutdown",
			testFn: testSignalShutdown,
		},
		{
			name:   "Forced close when grace period exceeded",
			testFn: testForcedShutdown,
		},
		{
			name:   "nil-logger fallback to slog.Default()",
			testFn: testNilLoggerFallback,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Hard timeout wrapper to prevent any test from hanging the suite
			done := make(chan struct{})
			go func() {
				tt.testFn(t)
				close(done)
			}()

			select {
			case <-done:
				// Test completed successfully
			case <-time.After(5 * time.Second):
				t.Fatal("test timeout - Run may have hung indefinitely")
			}
		})
	}
}

// testListenAndServeError verifies that ListenAndServe errors (other than ErrServerClosed)
// are returned immediately without waiting for a signal.
// Strategy: Start one server on a free port and keep it running,
// then try to start another server on the same address.
// The second server's Run should fail quickly with "address already in use" error.
func testListenAndServeError(t *testing.T) {
	// Get a free port by binding a temporary listener
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to bind temporary listener: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close()

	// Start first server on that address and keep it running
	srv1 := &http.Server{
		Addr:    addr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
	}
	sigCh1 := make(chan os.Signal, 1)
	errCh1 := make(chan error, 1)

	go func() {
		errCh1 <- Run(context.Background(), srv1, sigCh1, 30*time.Second, nil)
	}()

	// Wait for first server to bind and start listening
	time.Sleep(300 * time.Millisecond)

	// Now try to start second server on the same address - should fail immediately
	srv2 := &http.Server{
		Addr:    addr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
	}
	sigCh2 := make(chan os.Signal, 1)
	errCh2 := make(chan error, 1)

	go func() {
		errCh2 <- Run(context.Background(), srv2, sigCh2, 30*time.Second, nil)
	}()

	// srv2 should fail quickly (within 1 second) with an address-in-use error
	select {
	case err := <-errCh2:
		if err == nil {
			t.Error("expected error from srv2 due to address already in use, got nil")
		}
		// Verify it's an address-in-use error (this is OS-specific, but we can check the error exists)
	case <-time.After(1 * time.Second):
		t.Fatal("srv2 did not return error within 1 second - Run may not be propagating ListenAndServe errors correctly")
	}

	// Cleanup: stop srv1
	srv1.Close()

	// Wait for srv1's Run to complete
	select {
	case <-errCh1:
		// srv1 completed
	case <-time.After(2 * time.Second):
		// srv1 may not exit cleanly, but the test is already done
	}
}

// testSignalShutdown verifies that sending a signal on sigCh triggers
// graceful shutdown, which returns nil if successful.
func testSignalShutdown(t *testing.T) {
	// Get a free port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to bind temporary listener: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close()

	srv := &http.Server{
		Addr: addr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	}

	sigCh := make(chan os.Signal, 1)
	errCh := make(chan error, 1)

	go func() {
		errCh <- Run(context.Background(), srv, sigCh, 5*time.Second, slog.Default())
	}()

	// Wait for server to start listening
	time.Sleep(300 * time.Millisecond)

	// Send interrupt signal to trigger graceful shutdown
	sigCh <- os.Interrupt

	// Should return nil (graceful shutdown succeeded)
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("expected nil error from graceful shutdown, got: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after signal within 2 seconds")
	}
}

// testForcedShutdown verifies that when Shutdown's grace period is exceeded
// (i.e., there are in-flight requests that don't finish within grace duration),
// the code calls srv.Close() and returns an error containing "forced shutdown".
// Strategy: Create a handler that blocks for longer than the grace period,
// send a request to that handler, then trigger shutdown.
func testForcedShutdown(t *testing.T) {
	// Get a free port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to bind temporary listener: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close()

	// Synchronization: signal when handler starts executing
	handlerStarted := make(chan struct{})

	srv := &http.Server{
		Addr: addr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			close(handlerStarted)
			// Block for 500ms (much longer than grace period of 50ms)
			time.Sleep(500 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}),
	}

	sigCh := make(chan os.Signal, 1)
	errCh := make(chan error, 1)

	// Grace period of 50ms - much shorter than handler's 500ms sleep
	go func() {
		errCh <- Run(context.Background(), srv, sigCh, 50*time.Millisecond, slog.Default())
	}()

	// Wait for server to start listening
	time.Sleep(300 * time.Millisecond)

	// Send an HTTP request in a goroutine (it will block in the handler)
	go func() {
		client := &http.Client{Timeout: 10 * time.Second}
		// Ignore the response/error - we just need the request to reach the handler
		client.Get("http://" + addr + "/")
	}()

	// Wait for handler to start executing
	select {
	case <-handlerStarted:
		// Handler is now blocking for 500ms
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not start executing within 2 seconds")
	}

	// Send signal - handler is already running and will take 500ms to finish,
	// but grace period is only 50ms, so Shutdown will timeout
	sigCh <- os.Interrupt

	// Should return an error due to forced shutdown
	select {
	case err := <-errCh:
		if err == nil {
			t.Error("expected error due to forced shutdown (grace period exceeded), got nil")
		}
		// Verify it contains "forced shutdown" in the message
		if err != nil && !containsSubstring(err.Error(), "forced shutdown") {
			t.Errorf("expected error message to contain 'forced shutdown', got: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not return error within 3 seconds")
	}
}

// testNilLoggerFallback verifies that when log is nil,
// the code falls back to slog.Default() without panicking.
func testNilLoggerFallback(t *testing.T) {
	// Get a free port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to bind temporary listener: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close()

	srv := &http.Server{
		Addr: addr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	}

	sigCh := make(chan os.Signal, 1)
	errCh := make(chan error, 1)

	// Pass nil logger explicitly - should not panic
	go func() {
		errCh <- Run(context.Background(), srv, sigCh, 5*time.Second, nil)
	}()

	// Wait for server to start listening
	time.Sleep(300 * time.Millisecond)

	// Send signal to trigger shutdown (which will use slog.Default())
	sigCh <- os.Interrupt

	// Should return nil without panicking
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("expected nil error from graceful shutdown, got: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return within 2 seconds")
	}
}

// containsSubstring is a simple helper to check if a string contains a substring.
func containsSubstring(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
