package queue

import (
	"sync"
	"testing"
)

func TestLockFreeBasic(t *testing.T) {
	q := NewLockFree(16, 0)
	if q.Capacity() != 16 {
		t.Fatalf("capacity = %d, want 16", q.Capacity())
	}

	// Put and Get single item.
	if !q.Put("hello") {
		t.Fatal("Put failed")
	}
	v, ok := q.Get()
	if !ok || v.(string) != "hello" {
		t.Fatalf("Get = %v, %v, want hello, true", v, ok)
	}

	// Queue should be empty.
	if q.Len() != 0 {
		t.Fatalf("Len = %d, want 0", q.Len())
	}
}

func TestLockFreeFull(t *testing.T) {
	q := NewLockFree(4, 0)
	for i := 0; i < 4; i++ {
		if !q.Put(i) {
			t.Fatalf("Put %d failed", i)
		}
	}
	// Queue full, Put should fail.
	if q.Put(999) {
		t.Fatal("Put should fail when queue is full")
	}
}

func TestLockFreeEmpty(t *testing.T) {
	q := NewLockFree(4, 0)
	v, ok := q.Get()
	if ok {
		t.Fatalf("Get on empty queue should return false, got %v", v)
	}
}

func TestLockFiferenceOfTwo(t *testing.T) {
	t.Skip("roundToPowerOfTwo is now in mathutil package, tested there")
}

func TestLockFreeConcurrent(t *testing.T) {
	q := NewLockFree(1024, 0)
	var wg sync.WaitGroup
	const producers = 10
	const consumers = 10
	const itemsPerProducer = 100

	// Producers.
	for p := 0; p < producers; p++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < itemsPerProducer; i++ {
				for !q.Put(id*itemsPerProducer + i) {
					// Retry.
				}
			}
		}(p)
	}

	// Consumers.
	received := make([]int, producers*itemsPerProducer)
	var mu sync.Mutex
	for c := 0; c < consumers; c++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < producers*itemsPerProducer/consumers; i++ {
				v, ok := q.Get()
				for !ok {
					v, ok = q.Get()
				}
				mu.Lock()
				received[v.(int)]++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	// Verify all items received exactly once.
	for i, count := range received {
		if count != 1 {
			t.Errorf("item %d received %d times, want 1", i, count)
		}
	}
}

func TestLockFreePutAll(t *testing.T) {
	q := NewLockFree(16, 0)
	values := make([]interface{}, 5)
	for i := range values {
		values[i] = i
	}
	n := q.PutAll(values)
	if n != 5 {
		t.Fatalf("PutAll returned %d, want 5", n)
	}
	if q.Len() != 5 {
		t.Fatalf("Len = %d, want 5", q.Len())
	}
}

func TestLockFreeGetAll(t *testing.T) {
	q := NewLockFree(16, 0)
	for i := 0; i < 5; i++ {
		q.Put(i)
	}
	all := q.GetAll()
	if len(all) != 5 {
		t.Fatalf("GetAll returned %d items, want 5", len(all))
	}
	for i, v := range all {
		if v.(int) != i {
			t.Errorf("all[%d] = %v, want %d", i, v, i)
		}
	}
}