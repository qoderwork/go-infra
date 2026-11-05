package conc

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestUnsafeWaitWithContextDoesNotLeakWaiter 锁住 P1 修复：
// 旧实现为每次带超时的等待额外起一个阻塞在 wg.Wait() 的 goroutine，
// ctx 超时后它无法回收（任务卡死即永久泄漏）。
func TestUnsafeWaitWithContextDoesNotLeakWaiter(t *testing.T) {
	g := New(nil)
	release := make(chan struct{})
	g.GoRun(func() { <-release })

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := g.UnsafeWaitWithContext(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}

	stacks := make([]byte, 1<<18)
	n := runtime.Stack(stacks, true)
	if strings.Contains(string(stacks[:n]), "conc.(*G).Wait") {
		t.Fatal("UnsafeWaitWithContext left a waiter goroutine blocked in (*G).Wait")
	}

	close(release)
	done := make(chan struct{})
	go func() {
		g.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Wait blocked after the task completed")
	}
}

// TestGReusableAfterWait 验证完成通知 channel 在第一批任务结束后能正确重建。
func TestGReusableAfterWait(t *testing.T) {
	g := New(nil)
	var count int
	var mu sync.Mutex
	for batch := 0; batch < 3; batch++ {
		for i := 0; i < 5; i++ {
			g.GoRun(func() {
				mu.Lock()
				count++
				mu.Unlock()
			})
		}
		g.Wait()
		mu.Lock()
		got := count
		mu.Unlock()
		if got != (batch+1)*5 {
			t.Fatalf("batch %d: expected %d completed tasks, got %d", batch, (batch+1)*5, got)
		}
	}
}

// stackTracer 记录 panic 日志里附带的调用栈。
type stackTracer struct {
	mu    sync.Mutex
	stack string
}

func (s *stackTracer) Info(string, ...any) {}

func (s *stackTracer) Error(_ string, args ...any) {
	for i := 0; i+1 < len(args); i += 2 {
		if args[i] == "stack" {
			v, _ := args[i+1].(string)
			s.mu.Lock()
			s.stack = v
			s.mu.Unlock()
		}
	}
}

// TestGoRunPanicLogsStack 验证被 recover 的 panic 至少留下可诊断的调用栈。
func TestGoRunPanicLogsStack(t *testing.T) {
	tracer := &stackTracer{}
	g := New(tracer)
	g.GoRun(func() { panic("expected failure") })
	g.Wait()
	tracer.mu.Lock()
	stack := tracer.stack
	tracer.mu.Unlock()
	if !strings.Contains(stack, ".go:") {
		t.Fatalf("panic log is missing a usable stack trace: %q", stack)
	}
}
