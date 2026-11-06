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
// conc.TTLMap 默认不启动后台清理协程：过期项在该 key 被再次读取时由 Load
// 惰性删除，因此按请求或按业务对象创建缓存不会泄漏 goroutine。代价是长期
// 不再访问的过期项会一直占内存，直到闭包本身被回收。
//
// 若确实需要后台主动清理，请自行持有 conc.NewTTLMap，显式调用
// SetTickerCleanup 或 SwichFixedTimeClear 开启，并在结束时调用 Dispose。
func UseTTLCache[K comparable, V any](timeline time.Duration, fn func(K) (V, error)) CacheFn[K, V] {
	cache := &conc.TTLMap[K, V]{} // no background cleanup goroutine
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
