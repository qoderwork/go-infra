// concurrent
// 主要提供 3 个函数
// GoRun，异步执行函数
// Wait，等待所有任务执行完毕
// UnsafeWaitWithContext，包含超时机制的等待所有任务执行完毕
//
// 语义边界：GoRun / GoSafe 是 fire-and-forget，子 goroutine 的 panic 会被
// recover 并记录日志（含调用栈），但不会向 Wait 的调用方传播。
// 如果需要「panic 必须回到调用方」的作用域并发，请用
// github.com/qoderwork/go-infra/structured.WaitGroup。
package conc

import (
	"context"
	"log/slog"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// G 是一组 fire-and-forget 任务的等待句柄。
//
// 实现上用计数 + 完成通知 channel，而不是 sync.WaitGroup：这样
// UnsafeWaitWithContext 可以直接 select，不需要额外起一个阻塞在
// Wait() 上的 goroutine（旧实现在 ctx 超时后会留下无法回收的等待者，
// 任务卡死时就是永久泄漏）。
type G struct {
	trace Tracer

	mu      sync.Mutex
	pending atomic.Int64
	done    chan struct{} // pending 归零时关闭；再次从 0 加任务时重建
}

type Tracer interface {
	Error(msg string, args ...any)
	Info(msg string, args ...any)
}

func New(l Tracer) *G {
	if l == nil {
		l = DefaultTracer{}
	}
	g := &G{trace: l, done: make(chan struct{})}
	close(g.done) // 零个任务，初始即为已完成
	return g
}

// Wait 阻塞到当前所有任务结束。
//
// 与 sync.WaitGroup 一样，不要在已经 Wait 且计数归零后，再从另一个
// goroutine 并发发起第一批 GoRun；这种用法本身没有明确的完成边界。
func (g *G) Wait() {
	<-g.currentDone()
}

// UnsafeWaitWithContext 带超时地等待所有任务完成，超时返回 ctx.Err()。
// 超时只代表「调用方不等了」，不会取消任务，也不会留下等待 goroutine。
func (g *G) UnsafeWaitWithContext(ctx context.Context) error {
	ch := g.currentDone()
	// 已完成时优先返回 nil，避免与一个刚好也取消的 ctx 抢 select。
	select {
	case <-ch:
		return nil
	default:
	}
	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (g *G) currentDone() <-chan struct{} {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.done
}

// GoRun 异步执行任务，记录数量并可用 Wait 等待结束。
// panic 会被 recover 并记录（含 PanicStack 调用栈），不会向外传播。
func (g *G) GoRun(fn func()) {
	g.mu.Lock()
	if g.pending.Add(1) == 1 {
		g.done = make(chan struct{})
	}
	g.mu.Unlock()

	go func() {
		defer func() {
			pc, _, _, _ := runtime.Caller(1)
			funcName := runtime.FuncForPC(pc).Name()
			g.trace.Info("goroutine exit", "func", funcName)
			if err := recover(); err != nil {
				g.trace.Error("goroutine panic", "err", err, "func", funcName, "stack", string(PanicStack()))
			}

			g.mu.Lock()
			if g.pending.Add(-1) == 0 {
				close(g.done)
			}
			g.mu.Unlock()
		}()
		fn()
	}()
}

// GoSafe 启动一个不归属任何句柄的 goroutine，panic 只记录不传播。
func GoSafe(fn func()) {
	go func() {
		defer func() {
			pc, _, _, _ := runtime.Caller(1)
			funcName := runtime.FuncForPC(pc).Name()
			slog.Info("goroutine exit", "func", funcName)
			if err := recover(); err != nil {
				slog.Error("goroutine panic", "err", err, "func", funcName, "stack", string(PanicStack()))
			}
		}()
		fn()
	}()
}

type DefaultTracer struct{}

func (DefaultTracer) Info(msg string, args ...any) {
	slog.Info(msg, args...)
}

func (DefaultTracer) Error(msg string, args ...any) {
	slog.Error(msg, args...)
}

// DefaultTimer 首次是 3 秒后执行，每隔 every 执行一次
func DefaultTimer(ctx context.Context, every time.Duration, fn func()) {
	Timer(ctx, 3*time.Second, every, fn)
}

// Timer 轮询任务
func Timer(ctx context.Context, first, every time.Duration, fn func()) {
	timer := time.NewTimer(first)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			fn()
			timer.Reset(every)
		}
	}
}
