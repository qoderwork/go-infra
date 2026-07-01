// Package main demonstrates lifecycle with background workers.
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

	lifecycle "github.com/jingc1413/go-infra/lifecycle"
)

// ---------------------------------------------------------------------------
// worker — a generic periodic background task
// ---------------------------------------------------------------------------

type worker struct {
	name     string
	interval time.Duration
	done     atomic.Int64
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

func newWorker(name string, interval time.Duration) *worker {
	return &worker{name: name, interval: interval}
}

func (w *worker) Name() string { return w.name }

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
				log.Printf("[%s] exiting (processed %d items)", w.name, w.done.Load())
				return
			case <-ticker.C:
				w.process()
			}
		}
	}()
	return nil
}

func (w *worker) Stop(ctx context.Context) error {
	log.Printf("[%s] stopping... (processed %d items)", w.name, w.done.Load())
	if w.cancel != nil {
		w.cancel()
	}
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
	time.Sleep(10 * time.Millisecond)
	w.done.Add(1)
}

// ---------------------------------------------------------------------------
// main
// ---------------------------------------------------------------------------

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	mgr := lifecycle.New(lifecycle.WithTimeout(5 * time.Second))

	// Registration order determines start/stop order.
	// First added = starts first, stops last.
	mgr.Add(newWorker("DataProcessor", 200*time.Millisecond))
	mgr.Add(newWorker("CacheWarmer", 500*time.Millisecond))
	mgr.Add(newWorker("MetricsFlusher", 1*time.Second))

	mgr.OnStop(func(ctx context.Context) error {
		log.Println("All workers stopped.")
		return nil
	})

	if err := mgr.Run(); err != nil {
		log.Fatalf("lifecycle error: %v", err)
	}
}
