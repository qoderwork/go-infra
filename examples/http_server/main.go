// Package main demonstrates lifecycle with an HTTP server and database.
//
// Run:
//
//	go run ./examples/http_server
//
// Press Ctrl+C to trigger graceful shutdown.
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
// httpServer task
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

func (h *httpServer) Name() string { return "HTTPServer" }

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
// dbConnection task
// ---------------------------------------------------------------------------

type dbConnection struct{}

func (d *dbConnection) Name() string { return "Database" }

func (d *dbConnection) Start(ctx context.Context) error {
	log.Println("[Database] connecting...")
	time.Sleep(50 * time.Millisecond)
	log.Println("[Database] connected")
	return nil
}

func (d *dbConnection) Stop(ctx context.Context) error {
	log.Println("[Database] closing connection...")
	time.Sleep(30 * time.Millisecond)
	log.Println("[Database] closed")
	return nil
}

// ---------------------------------------------------------------------------
// main
// ---------------------------------------------------------------------------

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	mgr := lifecycle.New(lifecycle.WithTimeout(10 * time.Second))

	// Registration order = startup order = shutdown reverse order.
	// Database starts first, stops last.
	// HTTP server starts last, stops first.
	mgr.Add(&dbConnection{})
	mgr.Add(newHTTPServer(":8080"))

	mgr.OnStart(func(ctx context.Context) error {
		log.Println("[hook] application starting...")
		return nil
	})
	mgr.OnStop(func(ctx context.Context) error {
		log.Println("[hook] application stopped cleanly")
		return nil
	})

	if err := mgr.Run(); err != nil {
		log.Fatalf("lifecycle error: %v", err)
	}
}
