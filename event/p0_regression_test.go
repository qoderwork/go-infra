package event

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

// TestNotifySnapshotsHandlers: a handler that unregisters itself used to
// write the handlers map while Notify was iterating it.
func TestNotifySnapshotsHandlers(t *testing.T) {
	b := NewBus[int]()
	calls := 0
	b.Register("a", func(_ context.Context, _ int) error {
		b.Unregister("a")
		calls++
		return nil
	})
	if err := b.Notify(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if err := b.Notify(context.Background(), 2); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("handler called %d times, want 1 (snapshot semantics)", calls)
	}
}

// TestConcurrentRegisterAndNotify is meaningful under -race: the old Bus had
// an unguarded map and raced between Register and Notify.
func TestConcurrentRegisterAndNotify(t *testing.T) {
	b := NewBus[int]()
	b.Register("seed", func(_ context.Context, _ int) error { return nil })

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			b.Register(fmt.Sprintf("h%d", i), func(_ context.Context, _ int) error { return nil })
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			_ = b.Notify(context.Background(), i)
		}
	}()
	wg.Wait()
}

func TestZeroValueBusRegister(t *testing.T) {
	var b Bus[int]
	b.Register("a", func(_ context.Context, _ int) error { return nil })
	if err := b.Notify(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
}
