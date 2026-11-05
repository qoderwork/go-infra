package hook

import (
	"sync"
	"time"

	"github.com/qoderwork/go-infra/conc"
)

// CacheFn 缓存函数
type CacheFn[K comparable, V any] func(K) (V, bool, error)

// UseCache 使用内存缓存一些临时数据
// 由于是临时数据，其 value 应该是一次性数据，用完即丢
// 第二个返回参数用来标识是否命中缓存
//
// 返回的闭包可并发调用：内部用 RWMutex 保护 map。未命中时 fn 在锁外执行，
// 因此并发未命中同一个 key 可能重复调用 fn（不保证 singleflight）。
func UseCache[K comparable, V any](fn func(K) (V, error)) CacheFn[K, V] {
	cache := make(map[K]V)
	var mu sync.RWMutex
	return func(key K) (V, bool, error) {
		mu.RLock()
		v, ok := cache[key]
		mu.RUnlock()
		if ok {
			return v, true, nil
		}
		v, err := fn(key)
		if err == nil {
			mu.Lock()
			cache[key] = v
			mu.Unlock()
		}
		return v, false, err
	}
}

// UseTTLCache 使用内存缓存一些临时数据，指定时间超时自动清理。
//
// 这里使用零值 conc.TTLMap，不启动后台清理 goroutine：过期项在该 key
// 被再次读取时由 Load 惰性删除。这样按请求或按业务对象创建缓存时不会
// 永久泄漏 goroutine。代价是长期不再访问的过期项会一直占内存，直到闭包
// 本身被回收。
//
// 若需要后台主动清理，请显式使用 conc.NewTTLMap 并在结束时调用 Dispose。
func UseTTLCache[K comparable, V any](timeline time.Duration, fn func(K) (V, error)) CacheFn[K, V] {
	cache := &conc.TTLMap[K, V]{} // zero value: no cleanup goroutine
	return func(key K) (V, bool, error) {
		v, ok := cache.Load(key)
		if ok {
			return v, true, nil
		}
		v, err := fn(key)
		if err == nil {
			cache.Store(key, v, timeline)
		}
		return v, false, err
	}
}
