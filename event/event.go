// Package event 提供类型安全的事件通知机制（观察者模式）。
// 每个事件类型对应一个独立的 Bus 实例，无需 map 路由。
// handler 内部可自由选择同步处理或调用 River 入队。
// map 存储天然无序，调用方不应依赖 handler 执行顺序。
package event

import (
	"context"
	"sync"
)

// Notifier 事件通知器接口（发布方持有，便于测试 mock）。
type Notifier[T any] interface {
	Notify(ctx context.Context, data T) error
}

// Bus 内存同步事件广播器。
// 每种事件类型一个 Bus 实例，观察者通过 Register(key, fn) 注册。
// key 仅用于注册/注销标识，不参与通知路由。
//
// Bus 可并发使用：Register/Unregister/Notify 全部受 RWMutex 保护。
// Notify 会先在锁内对 handler 列表做快照，再在锁外逐个执行，因此 handler
// 内部可以安全地注册或注销观察者，不会造成 map 遍历期间的并发写。
type Bus[T any] struct {
	mu       sync.RWMutex
	handlers map[string]func(context.Context, T) error
}

// NewBus 创建事件总线实例。
func NewBus[T any]() *Bus[T] {
	return &Bus[T]{
		handlers: make(map[string]func(context.Context, T) error),
	}
}

// Register 注册观察者函数。key 重复则 panic，防止静默覆盖。
func (b *Bus[T]) Register(key string, fn func(context.Context, T) error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.handlers == nil {
		b.handlers = make(map[string]func(context.Context, T) error)
	}
	if _, exists := b.handlers[key]; exists {
		panic("event.Bus: duplicate key: " + key)
	}
	b.handlers[key] = fn
}

// Unregister 移除指定 key 的观察者。
func (b *Bus[T]) Unregister(key string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.handlers, key)
}

// Notify 通知所有已注册的观察者。任一返回 err 则中止并上抛。
// 执行顺序不确定（map 遍历），调用方不应依赖顺序。
// 通知基于调用时刻的快照：本次通知期间新注册的 handler 下次才会被调用，
// 被注销的 handler 若已在快照中仍会收到本次通知。
func (b *Bus[T]) Notify(ctx context.Context, data T) error {
	b.mu.RLock()
	snapshot := make([]func(context.Context, T) error, 0, len(b.handlers))
	for _, fn := range b.handlers {
		snapshot = append(snapshot, fn)
	}
	b.mu.RUnlock()

	for _, fn := range snapshot {
		if err := fn(ctx, data); err != nil {
			return err
		}
	}
	return nil
}
