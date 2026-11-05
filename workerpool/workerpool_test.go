package workerpool

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// blockWorker occupies the single worker until gate is closed.
func blockWorker(t *testing.T, p *Pool) (started <-chan struct{}, gate chan struct{}) {
	t.Helper()
	s := make(chan struct{})
	g := make(chan struct{})
	if err := p.SubmitWait(context.Background(), func() {
		close(s)
		<-g
	}); err != nil {
		t.Fatalf("failed to occupy the worker: %v", err)
	}
	select {
	case <-s:
	case <-time.After(time.Second):
		t.Fatal("worker never started")
	}
	return s, g
}

func TestSubmitAndCloseDrainsQueue(t *testing.T) {
	p, err := New(3, 32)
	if err != nil {
		t.Fatal(err)
	}
	if p.Workers() != 3 {
		t.Fatalf("expected 3 workers, got %d", p.Workers())
	}
	var count atomic.Int64
	for i := 0; i < 50; i++ {
		if err := p.SubmitWait(context.Background(), func() { count.Add(1) }); err != nil {
			t.Fatalf("SubmitWait #%d: %v", i, err)
		}
	}
	p.Close()
	if got := count.Load(); got != 50 {
		t.Fatalf("Close lost queued tasks: executed %d of 50", got)
	}
	select {
	case <-p.Done():
	default:
		t.Fatal("Done should be closed after Close")
	}
}

func TestSubmitNeverBlocksAndReportsFullQueue(t *testing.T) {
	p, err := New(1, 0) // unbuffered: only an idle worker can accept
	if err != nil {
		t.Fatal(err)
	}
	_, gate := blockWorker(t, p)
	var ran atomic.Bool
	if err := p.Submit(func() { ran.Store(true) }); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("expected ErrQueueFull, got %v", err)
	}
	if ran.Load() {
		t.Fatal("rejected task must not be executed")
	}
	close(gate)
	p.Close()
}

func TestSubmitWaitWaitsForFreeSlot(t *testing.T) {
	p, err := New(1, 1)
	if err != nil {
		t.Fatal(err)
	}
	_, gate := blockWorker(t, p)
	if err := p.Submit(func() {}); err != nil { // fill the only queue slot
		t.Fatalf("failed to fill queue: %v", err)
	}

	result := make(chan error, 1)
	go func() { result <- p.SubmitWait(context.Background(), func() {}) }()
	select {
	case err := <-result:
		t.Fatalf("SubmitWait returned before a slot was free: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	close(gate)
	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("SubmitWait failed after a slot freed: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("SubmitWait did not complete after the worker freed up")
	}
	p.Close()
}

func TestSubmitWaitHonorsContext(t *testing.T) {
	p, err := New(1, 1)
	if err != nil {
		t.Fatal(err)
	}
	_, gate := blockWorker(t, p)
	if err := p.Submit(func() {}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if err := p.SubmitWait(ctx, func() {}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}
	close(gate)
	p.Close()
}

func TestCloseIsIdempotentAndRejectsNewTasks(t *testing.T) {
	p, err := New(2, 4)
	if err != nil {
		t.Fatal(err)
	}
	p.Close()
	p.Close()
	if err := p.Submit(func() {}); !errors.Is(err, ErrClosed) {
		t.Fatalf("Submit after Close = %v, want ErrClosed", err)
	}
	if err := p.SubmitWait(context.Background(), func() {}); !errors.Is(err, ErrClosed) {
		t.Fatalf("SubmitWait after Close = %v, want ErrClosed", err)
	}
	if err := p.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown on an already closed pool: %v", err)
	}
}

func TestPanicRecoveredAndWorkerSurvives(t *testing.T) {
	var mu sync.Mutex
	var panics []any
	p, err := New(1, 8, WithPanicHandler(func(v any) {
		mu.Lock()
		panics = append(panics, v)
		mu.Unlock()
	}))
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	if err := p.SubmitWait(context.Background(), func() { panic("boom") }); err != nil {
		t.Fatal(err)
	}
	if err := p.SubmitWait(context.Background(), func() { close(done) }); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker died after a task panicked")
	}
	p.Close()
	mu.Lock()
	defer mu.Unlock()
	if len(panics) != 1 || panics[0] != "boom" {
		t.Fatalf("unexpected panic report: %v", panics)
	}
}

func TestWithContextClosesPool(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	p, err := New(2, 4, WithContext(ctx))
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case <-p.Done():
	case <-time.After(time.Second):
		t.Fatal("pool was not closed when ctx was canceled")
	}
	if err := p.Submit(func() {}); !errors.Is(err, ErrClosed) {
		t.Fatalf("Submit after ctx cancel = %v, want ErrClosed", err)
	}
}

func TestShutdownTimeoutDoesNotAbandonTasks(t *testing.T) {
	p, err := New(1, 8)
	if err != nil {
		t.Fatal(err)
	}
	_, gate := blockWorker(t, p)
	var count atomic.Int64
	for i := 0; i < 5; i++ {
		if err := p.SubmitWait(context.Background(), func() { count.Add(1) }); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	if err := p.Shutdown(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected a shutdown timeout, got %v", err)
	}
	cancel()
	close(gate)
	select {
	case <-p.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown did not finish after the blocked task was released")
	}
	if got := count.Load(); got != 5 {
		t.Fatalf("shutdown abandoned queued tasks: executed %d of 5", got)
	}
}

func TestConcurrentSubmitters(t *testing.T) {
	p, err := New(4, 64)
	if err != nil {
		t.Fatal(err)
	}
	var count atomic.Int64
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				if err := p.SubmitWait(context.Background(), func() { count.Add(1) }); err != nil {
					t.Errorf("submit failed: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
	p.Close()
	if got := count.Load(); got != 800 {
		t.Fatalf("expected 800 executed tasks, got %d", got)
	}
}

func TestNewValidationAndNilTask(t *testing.T) {
	if _, err := New(0, 1); err == nil {
		t.Fatal("expected an error for zero workers")
	}
	if _, err := New(1, -1); err == nil {
		t.Fatal("expected an error for a negative queue size")
	}
	p, err := New(1, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if err := p.Submit(nil); !errors.Is(err, ErrNilTask) {
		t.Fatalf("Submit(nil) = %v, want ErrNilTask", err)
	}
	if err := p.SubmitWait(context.Background(), nil); !errors.Is(err, ErrNilTask) {
		t.Fatalf("SubmitWait(nil) = %v, want ErrNilTask", err)
	}
	if err := p.SubmitWait(context.Background(), func() {}); err != nil {
		t.Fatalf("valid task rejected: %v", err)
	}
}
