package queue

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestMSQueueEmptyAndFIFO(t *testing.T) {
	q := NewMSQueue[int]()
	if !q.IsEmpty() || q.Len() != 0 {
		t.Fatalf("new queue should be empty: empty=%v len=%d", q.IsEmpty(), q.Len())
	}
	if _, ok := q.Dequeue(); ok {
		t.Fatal("Dequeue on an empty queue must return false")
	}
	for i := 0; i < 100; i++ {
		q.Enqueue(i)
	}
	if q.Len() != 100 {
		t.Fatalf("expected 100 queued items, got %d", q.Len())
	}
	for i := 0; i < 100; i++ {
		v, ok := q.Dequeue()
		if !ok || v != i {
			t.Fatalf("Dequeue #%d = %d, %v", i, v, ok)
		}
	}
	if _, ok := q.Dequeue(); ok || !q.IsEmpty() {
		t.Fatal("queue should be empty after draining")
	}
}

func TestMSQueueInterleavedOperations(t *testing.T) {
	q := NewMSQueue[string]()
	q.Enqueue("a")
	q.Enqueue("b")
	if v, ok := q.Dequeue(); !ok || v != "a" {
		t.Fatalf("expected a, got %q %v", v, ok)
	}
	q.Enqueue("c")
	q.Enqueue("d")
	want := []string{"b", "c", "d"}
	for _, w := range want {
		if v, ok := q.Dequeue(); !ok || v != w {
			t.Fatalf("expected %q, got %q %v", w, v, ok)
		}
	}
}

func TestMSQueueMultiProducer(t *testing.T) {
	const producers, perProducer = 8, 500
	q := NewMSQueue[int]()
	var wg sync.WaitGroup
	for p := 0; p < producers; p++ {
		p := p
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perProducer; i++ {
				q.Enqueue(p*perProducer + i)
			}
		}()
	}
	wg.Wait()
	if q.Len() != producers*perProducer {
		t.Fatalf("expected %d items, got %d", producers*perProducer, q.Len())
	}

	// Drain with several consumers and check that every value arrived once.
	seen := make([]bool, producers*perProducer)
	var mu sync.Mutex
	var cwg sync.WaitGroup
	for c := 0; c < 8; c++ {
		cwg.Add(1)
		go func() {
			defer cwg.Done()
			for {
				v, ok := q.Dequeue()
				if !ok {
					return
				}
				mu.Lock()
				if seen[v] {
					t.Errorf("value %d delivered twice", v)
				}
				seen[v] = true
				mu.Unlock()
			}
		}()
	}
	cwg.Wait()
	for i, ok := range seen {
		if !ok {
			t.Fatalf("value %d was lost", i)
		}
	}
	if !q.IsEmpty() {
		t.Fatalf("queue should be empty, len=%d", q.Len())
	}
}

func TestMSQueueConcurrentProducersAndConsumers(t *testing.T) {
	const producers, perProducer, consumers = 4, 1000, 4
	total := int64(producers * perProducer)
	q := NewMSQueue[int]()
	var consumed atomic.Int64
	var cwg sync.WaitGroup
	for c := 0; c < consumers; c++ {
		cwg.Add(1)
		go func() {
			defer cwg.Done()
			for consumed.Load() < total {
				if _, ok := q.Dequeue(); ok {
					consumed.Add(1)
				}
			}
		}()
	}
	var pwg sync.WaitGroup
	for p := 0; p < producers; p++ {
		pwg.Add(1)
		go func() {
			defer pwg.Done()
			for i := 0; i < perProducer; i++ {
				q.Enqueue(i)
			}
		}()
	}
	pwg.Wait()
	cwg.Wait()
	if consumed.Load() != total {
		t.Fatalf("consumed %d of %d items", consumed.Load(), total)
	}
	if !q.IsEmpty() {
		t.Fatalf("queue should be empty, len=%d", q.Len())
	}
}
