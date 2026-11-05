package lifecycle

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type blockingStartTask struct {
	started chan struct{}
	release chan struct{}
	stops   atomic.Int64
}

func (t *blockingStartTask) Name() string { return "blocking-start" }
func (t *blockingStartTask) Start(context.Context) error {
	close(t.started)
	<-t.release
	return nil
}
func (t *blockingStartTask) Stop(context.Context) error {
	t.stops.Add(1)
	return nil
}

// TestStopDuringStartStillStopsTasks reproduces the old failure: Stop() that
// landed while starting saw running == false, consumed stopOnce, and left the
// Manager permanently unstoppable (task.Stop was never called).
func TestStopDuringStartStillStopsTasks(t *testing.T) {
	task := &blockingStartTask{started: make(chan struct{}), release: make(chan struct{})}
	m := New()
	m.Add(task)

	startDone := make(chan error, 1)
	go func() { startDone <- m.Start(context.Background()) }()
	<-task.started

	if err := m.Stop(context.Background()); err != nil {
		t.Fatalf("Stop during Start returned %v", err)
	}
	close(task.release)

	select {
	case err := <-startDone:
		if err != nil {
			t.Fatalf("Start returned %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Start did not finish")
	}

	if got := task.stops.Load(); got != 1 {
		t.Fatalf("task.Stop called %d time(s), want 1", got)
	}
	select {
	case <-m.Done():
	default:
		t.Fatal("Done was not closed after shutdown")
	}
	if err := m.Stop(context.Background()); err != nil {
		t.Fatalf("second Stop returned %v", err)
	}
	if got := task.stops.Load(); got != 1 {
		t.Fatalf("second Stop re-ran teardown (%d calls)", got)
	}
}
