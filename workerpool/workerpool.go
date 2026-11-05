// Package workerpool provides a long-lived, process-wide worker pool with a
// fixed number of resident goroutines.
//
// It is the complement of structured.Pool: that one owns a bounded batch of
// tasks that ends at Wait(), while a workerpool.Pool is a resident resource
// that tasks are submitted to from anywhere, at any time. Use structured.Pool
// for a parallel batch; use this one for fire-and-forget work across the
// lifetime of a process.
//
// Every failure is reported explicitly. Submit never blocks and never drops a
// task silently: a full queue returns ErrQueueFull. Use SubmitWait when you
// want back pressure with a context. A panicking task is recovered, reported
// through PanicHandler, and does not kill its worker.
package workerpool

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"sync/atomic"
)

var (
	// ErrClosed is returned when submitting to a pool that is closing or closed.
	ErrClosed = errors.New("workerpool: pool is closed")
	// ErrQueueFull is returned by Submit when the queue has no free slot.
	// The task was NOT submitted and may be retried.
	ErrQueueFull = errors.New("workerpool: task queue is full")
	// ErrNilTask is returned for a nil task.
	ErrNilTask = errors.New("workerpool: task must not be nil")
)

// Option configures a Pool.
type Option func(*config)

type config struct {
	ctx          context.Context
	panicHandler func(any)
}

// WithContext closes the pool when ctx is canceled. It starts one watcher
// goroutine, which exits when the pool is closed by any other means.
func WithContext(ctx context.Context) Option {
	return func(c *config) { c.ctx = ctx }
}

// WithPanicHandler sets the handler for panics raised by tasks. The handler
// runs on the worker goroutine; it must not panic. The default logs the value
// and a stack trace via slog.
func WithPanicHandler(fn func(any)) Option {
	return func(c *config) { c.panicHandler = fn }
}

// Pool is a fixed-size set of resident worker goroutines fed by a buffered
// task channel. Create one with New; a zero-value Pool is not usable.
type Pool struct {
	tasks        chan func()
	workers      int
	panicHandler func(any)

	mu     sync.RWMutex // guards closed and the send side of tasks
	closed bool

	quit      chan struct{} // closed first, to unblock SubmitWait
	done      chan struct{} // closed once all workers have exited
	startOnce sync.Once
	wg        sync.WaitGroup
	running   atomic.Int64
}

// New starts workers resident goroutines with a task queue of queueSize.
// queueSize 0 means an unbuffered queue: Submit only succeeds when a worker
// is idle at that instant. WithContext and WithPanicHandler are optional.
func New(workers, queueSize int, opts ...Option) (*Pool, error) {
	if workers < 1 {
		return nil, fmt.Errorf("workerpool: workers must be >= 1, got %d", workers)
	}
	if queueSize < 0 {
		return nil, fmt.Errorf("workerpool: queue size must be >= 0, got %d", queueSize)
	}
	cfg := config{}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	if cfg.panicHandler == nil {
		cfg.panicHandler = defaultPanicHandler
	}

	p := &Pool{
		tasks:        make(chan func(), queueSize),
		workers:      workers,
		panicHandler: cfg.panicHandler,
		quit:         make(chan struct{}),
		done:         make(chan struct{}),
	}
	p.wg.Add(workers)
	for i := 0; i < workers; i++ {
		go p.worker()
	}
	if cfg.ctx != nil {
		go p.watch(cfg.ctx)
	}
	return p, nil
}

// Submit queues task without blocking. It returns ErrQueueFull when the queue
// is full (the task was not submitted), ErrClosed after shutdown starts, and
// ErrNilTask for a nil task.
func (p *Pool) Submit(task func()) error {
	if task == nil {
		return ErrNilTask
	}
	// Holding RLock across the non-blocking send makes close(tasks) safe:
	// shutdown takes the write lock before closing the channel.
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.closed {
		return ErrClosed
	}
	select {
	case p.tasks <- task:
		return nil
	default:
		return ErrQueueFull
	}
}

// SubmitWait queues task, waiting for a free slot, for ctx cancellation, or
// for shutdown. It never drops a task silently. Use a context with a deadline
// if a blocked submitter must not outlive a request.
func (p *Pool) SubmitWait(ctx context.Context, task func()) error {
	if task == nil {
		return ErrNilTask
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.closed {
		return ErrClosed
	}
	select {
	case p.tasks <- task:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-p.quit:
		return ErrClosed
	}
}

// Close stops accepting tasks and waits for every queued task to finish.
// It is safe to call concurrently and repeatedly. Workers drain the buffered
// tasks before exiting, so nothing already accepted is lost.
func (p *Pool) Close() {
	p.startClose()
	<-p.done
}

// Shutdown starts closing the pool and waits up to ctx for it to finish.
// A ctx timeout only stops waiting: shutdown continues in the background and
// tasks are never abandoned. Observe Done() to learn when it really finished.
func (p *Pool) Shutdown(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	p.startClose()
	select {
	case <-p.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Done is closed once all workers have exited after shutdown.
func (p *Pool) Done() <-chan struct{} { return p.done }

// Workers returns the configured number of resident workers.
func (p *Pool) Workers() int { return p.workers }

// QueueLen returns the number of queued (not yet started) tasks.
func (p *Pool) QueueLen() int { return len(p.tasks) }

// Running returns the number of workers currently executing a task.
func (p *Pool) Running() int64 { return p.running.Load() }

func (p *Pool) startClose() {
	// Exactly one shutdown goroutine, however many Close/Shutdown callers there are.
	p.startOnce.Do(func() { go p.shutdown() })
}

func (p *Pool) shutdown() {
	close(p.quit) // release SubmitWait callers holding RLock
	p.mu.Lock()
	p.closed = true
	close(p.tasks) // workers drain the buffered tasks, then exit
	p.mu.Unlock()
	p.wg.Wait()
	close(p.done)
}

func (p *Pool) worker() {
	defer p.wg.Done()
	for task := range p.tasks {
		p.running.Add(1)
		p.run(task)
		p.running.Add(-1)
	}
}

func (p *Pool) run(task func()) {
	defer func() {
		if r := recover(); r != nil {
			p.panicHandler(r)
		}
	}()
	task()
}

func (p *Pool) watch(ctx context.Context) {
	select {
	case <-ctx.Done():
		p.Close()
	case <-p.done:
	}
}

func defaultPanicHandler(v any) {
	slog.Error("workerpool: task panicked", "panic", v, "stack", string(debug.Stack()))
}
