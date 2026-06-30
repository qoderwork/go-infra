// Package main demonstrates using the lifecycle manager to coordinate
// an HTTP server and a database connection with graceful shutdown.
//
// Run:
//
//	go run ./examples/http_server
//
// Then send SIGINT (Ctrl+C) to trigger graceful shutdown.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	lifecycle "lifecycle"
)

// ---------------------------------------------------------------------------
// httpServer — a Task that wraps net/http.Server
// ---------------------------------------------------------------------------

type httpServer struct {
	srv *http.Server
}

func newHTTPServer(addr string) *httpServer {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello from lifecycle-managed HTTP server!")
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})
	return &httpServer{srv: &http.Server{Addr: addr, Handler: mux}}
}

func (h *httpServer) Name() string     { return "HTTPServer" }
func (h *httpServer) Priority() int    { return 10 } // High priority: start first, stop last

func (h *httpServer) Start(ctx context.Context) error {
	log.Printf("[HTTPServer] listening on %s", h.srv.Addr)
	go func() {
		if err := h.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[HTTPServer] serve error: %v", err)
		}
	}()
	return nil
}

func (h *httpServer) Stop(ctx context.Context) error {
	log.Printf("[HTTPServer] shutting down...")
	return h.srv.Shutdown(ctx)
}

// ---------------------------------------------------------------------------
// dbConnection — a Task that simulates a database connection
// ---------------------------------------------------------------------------

type dbConnection struct{ closed bool }

func (d *dbConnection) Name() string     { return "Database" }
func (d *dbConnection) Priority() int    { return 100 } // Highest priority: start first, stop last

func (d *dbConnection) Start(ctx context.Context) error {
	log.Println("[Database] connecting...")
	// Simulate connection setup
	time.Sleep(50 * time.Millisecond)
	log.Println("[Database] connected")
	return nil
}

func (d *dbConnection) Stop(ctx context.Context) error {
	log.Println("[Database] closing connection...")
	// Simulate connection teardown
	time.Sleep(30 * time.Millisecond)
	d.closed = true
	log.Println("[Database] closed")
	return nil
}

// ---------------------------------------------------------------------------
// main
// ---------------------------------------------------------------------------

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	mgr := lifecycle.NewManager(
		lifecycle.WithLogger(&slogAdapter{}),
		lifecycle.WithShutdownTimeout(10*time.Second),
		lifecycle.WithMetricsHook(func(e lifecycle.MetricEvent) {
			if e.Error != nil {
				log.Printf("[metrics] phase=%s task=%s error=%v", e.Phase, e.TaskName, e.Error)
			}
		}),
	)

	// Register tasks
	mgr.AddTask(&dbConnection{})
	mgr.AddTask(newHTTPServer(":8080"))

	// Register hooks
	mgr.OnStart(func(ctx context.Context) error {
		log.Println("[hook] application starting...")
		return nil
	})
	mgr.OnReady(func(ctx context.Context) error {
		log.Println("[hook] application ready — access http://localhost:8080/")
		return nil
	})
	mgr.OnStopping(func(ctx context.Context) error {
		log.Println("[hook] shutdown initiated, draining connections...")
		return nil
	})
	mgr.OnStopped(func(ctx context.Context) error {
		log.Println("[hook] application stopped cleanly")
		return nil
	})

	// Run blocks until SIGINT/SIGTERM, then gracefully shuts down
	if err := mgr.Run(); err != nil {
		log.Fatalf("lifecycle error: %v", err)
	}

	log.Printf("shutdown reason: %s", mgr.Reason())
	if err := mgr.Err(); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}

// slogAdapter bridges lifecycle.Logger to the standard log package.
type slogAdapter struct{}

func (a *slogAdapter) Printf(format string, v ...any) {
	log.Printf(format, v...)
}
