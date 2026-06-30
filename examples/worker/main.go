// Package main demonstrates using the lifecycle manager for background
// workers: a periodic data processor and a cache warmer, with different
// priorities and graceful shutdown.
//
// Run:
//
//	go run ./examples/worker
//
// Press Ctrl+C to gracefully stop all workers.
package main

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"

	lifecycle "lifecycle"
)

// ---------------------------------------------------------------------------
// worker — a generic periodic background task
// ---------------------------------------------------------------------------

type worker struct {
	name     string
	priority int
	interval time.Duration
	done     atomic.Int64
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

func newWorker(name string, priority int, interval time.Duration) *worker {
	return &worker{name: name, priority: priority, interval: interval}
}

func (w *worker) Name() string  { return w.name }
func (w *worker) Priority() int { return w.priority }

func (w *worker) Start(ctx context.Context) error {
	ctx, w.cancel = context.WithCancel(ctx)
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()
		log.Printf("[%s] started (interval=%s)", w.name, w.interval)
		for {
			select {
			case <-ctx.Done():
				log.Printf("[%s] context cancelled, exiting (processed %d items)",
					w.name, w.done.Load())
				return
			case <-ticker.C:
				w.process()
			}
		}
	}()
	return nil
}

func (w *worker) Stop(ctx context.Context) error {
	log.Printf("[%s] stopping... (processed %d items total)", w.name, w.done.Load())
	if w.cancel != nil {
		w.cancel()
	}
	// Wait for goroutine with context awareness
	doneCh := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(doneCh)
	}()
	select {
	case <-doneCh:
		log.Printf("[%s] stopped cleanly", w.name)
	case <-ctx.Done():
		log.Printf("[%s] stop timed out", w.name)
		return ctx.Err()
	}
	return nil
}

func (w *worker) process() {
	// Simulate work
	time.Sleep(10 * time.Millisecond)
	w.done.Add(1)
}

// ---------------------------------------------------------------------------
// main
// ---------------------------------------------------------------------------

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	mgr := lifecycle.NewManager(
		lifecycle.WithShutdownTimeout(5 * time.Second),
	)

	// Register workers with different priorities.
	// Higher priority = starts first, stops last.
	mgr.AddTask(newWorker("CacheWarmer", 50, 500*time.Millisecond))
	mgr.AddTask(newWorker("DataProcessor", 100, 200*time.Millisecond))
	mgr.AddTask(newWorker("MetricsFlusher", 10, 1*time.Second))

	// Hook: log startup summary
	mgr.OnReady(func(ctx context.Context) error {
		log.Println("All workers running. Press Ctrl+C to stop.")
		return nil
	})

	// Run blocks until SIGINT/SIGTERM
	if err := mgr.Run(); err != nil {
		log.Fatalf("lifecycle error: %v", err)
	}

	log.Printf("shutdown reason: %s", mgr.Reason())
}
