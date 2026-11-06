package conc

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"time"
)

// ErrCacheNotFound 缓存未找到错误
var ErrCacheNotFound = errors.New("cache not found")

// Cacher 缓存接口
type Cacher interface {
	Set(context.Context, string, any)
	Del(context.Context, string)
	Get(context.Context, string, any) error
	SetNX(context.Context, string, any)
}

// TTLCache 不使用泛型，是考虑到使用 redis 时，也没办法用泛型
// 为了统一和代码简洁性，故而使用 any，而非泛型
type TTLCache struct {
	*TTLMap[string, any]
	ttl time.Duration
}

func NewTTLCache(ttl time.Duration) *TTLCache {
	return &TTLCache{
		TTLMap: NewTTLMap[string, any](),
		ttl:    ttl,
	}
}

// Close 停止后台清理协程（若有）并释放缓存。NewTTLCache 默认不再启动常驻
// goroutine，过期项在读取时惰性删除；因此普通用法下 Close 只是清空数据、
// 阻止后续读写。若曾调用 SetTickerCleanup/SwichFixedTimeClear 开启主动清理，
// Close 会同时停掉该协程。Close 之后不应再读写该缓存。
func (t *TTLCache) Close() {
	t.TTLMap.Dispose()
}

// Set 设置缓存值
func (t *TTLCache) Set(_ context.Context, key string, value any) {
	t.TTLMap.Store(key, value, t.ttl)
}

// Del 删除缓存值
func (t *TTLCache) Del(_ context.Context, key string) {
	t.TTLMap.Delete(key)
}

// Get 获取缓存值，将结果反序列化到 dest 中
func (t *TTLCache) Get(_ context.Context, key string, dest any) error {
	value, ok := t.TTLMap.Load(key)
	if !ok {
		return ErrCacheNotFound
	}
	// 使用反射进行高效的类型转换
	return assignByReflect(value, dest)
}

// assignByReflect 使用反射进行类型转换，比 JSON 序列化更高效
func assignByReflect(src any, dest any) error {
	srcVal := reflect.ValueOf(src)
	destVal := reflect.ValueOf(dest)

	if destVal.Kind() != reflect.Pointer {
		return errors.New("dest must be a pointer")
	}

	destElem := destVal.Elem()

	// 如果类型相同，直接赋值
	if srcVal.Type() == destElem.Type() {
		destElem.Set(srcVal)
		return nil
	}

	// 如果类型可转换，进行转换
	if srcVal.Type().ConvertibleTo(destElem.Type()) {
		destElem.Set(srcVal.Convert(destElem.Type()))
		return nil
	}

	// 如果反射转换失败，回退到 JSON 序列化（用于复杂类型转换）
	data, err := json.Marshal(src)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, dest)
}

// SetNX 仅当键不存在（且未过期）时设置值。
// 先用 Load 惰性清除可能存在的过期项（过期视同不存在），再用 LoadOrStore 原子
// 落子：并发下只有一个写入者成功，避免旧实现 Load+Store 两步都判“不存在”后互相
// 覆盖。仍在“已过期但未被清理”的边界上保留一次额外判断，属该双表设计的固有取舍。
func (t *TTLCache) SetNX(_ context.Context, key string, value any) {
	if _, exists := t.TTLMap.Load(key); exists {
		return // present and not expired: do nothing
	}
	_, _ = t.TTLMap.LoadOrStore(key, value, t.ttl)
}
