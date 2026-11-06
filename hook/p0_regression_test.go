package hook

import (
	"bytes"
	"log/slog"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestUseCacheConcurrent is meaningful under -race: the old closure used a
// bare map and produced "concurrent map read and map write".
func TestUseCacheConcurrent(t *testing.T) {
	fn := UseCache[int, int](func(k int) (int, error) { return k * 2, nil })
	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _, _ = fn(i % 8)
		}(i)
	}
	wg.Wait()
}

func countCleanupGoroutines() int {
	buf := make([]byte, 1<<20)
	n := runtime.Stack(buf, true)
	// Count stack frames, not incidental mentions in source paths.
	return strings.Count(string(buf[:n]), ".tickerCleanup(")
}

// TestUseTTLCacheDoesNotStartCleanupGoroutine: the helper must stay lazy.
// TTLMap no longer spawns a cleanup goroutine by default, but the helper must
// not opt into SetTickerCleanup on the caller's behalf either, since the cache
// hidden behind the closure has no reachable Dispose.
func TestUseTTLCacheDoesNotStartCleanupGoroutine(t *testing.T) {
	base := countCleanupGoroutines()
	for i := 0; i < 20; i++ {
		cache := UseTTLCache[int, int](time.Minute, func(k int) (int, error) { return k, nil })
		v, hit, err := cache(1)
		if err != nil || v != 1 || hit {
			t.Fatalf("first lookup = (%d, %v, %v)", v, hit, err)
		}
		_, hit, _ = cache(1)
		if !hit {
			t.Fatal("second lookup should hit the cache")
		}
	}
	if got := countCleanupGoroutines(); got != base {
		t.Fatalf("UseTTLCache started %d cleanup goroutines", got-base)
	}
}

// TestUseTimingLogsWhenOverLimit: UseTiming used to have two identical return
// branches and never logged, contrary to its doc comment.
func TestUseTimingLogsWhenOverLimit(t *testing.T) {
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	defer slog.SetDefault(old)

	cost := UseTiming(time.Nanosecond)
	time.Sleep(time.Millisecond)
	cost()
	if !strings.Contains(buf.String(), "level=ERROR") {
		t.Fatalf("expected an ERROR timing log, got: %s", buf.String())
	}
}

// TestUseTimingWithLogNeverPanics covers the nil FuncForPC case.
func TestUseTimingWithLogNeverPanics(t *testing.T) {
	UseTimingWithLog(time.Hour)()
}
