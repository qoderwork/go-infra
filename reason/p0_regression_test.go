package reason

import (
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
)

// TestWithChainKeepsDetailsOnce: the old With appended e.Details onto a copy
// whose Details already shared the same backing array, producing
// [a a b a a b c] for a three-step chain and mutating the source error.
func TestWithChainKeepsDetailsOnce(t *testing.T) {
	e := &Error{Reason: "p0", Msg: "m"}
	got := e.With("a").With("b").With("c").GetDetails()
	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("details = %v, want %v", got, want)
	}
	if len(e.GetDetails()) != 0 {
		t.Fatalf("With mutated the source error: %v", e.GetDetails())
	}
}

func TestWithfChainKeepsDetailsOnce(t *testing.T) {
	e := &Error{Reason: "p0", Msg: "m"}
	got := e.Withf("n=%d", 1).Withf("n=%d", 2).GetDetails()
	want := []string{"n=1", "n=2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("details = %v, want %v", got, want)
	}
}

var registrationSeq atomic.Uint64

// TestNewErrorConcurrent is meaningful under -race: the global codes map used
// to be registered without synchronization.
func TestNewErrorConcurrent(t *testing.T) {
	prefix := fmt.Sprintf("p0_concurrent_%d_", registrationSeq.Add(1))
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = NewError(fmt.Sprintf("%s%d", prefix, i), "m")
		}(i)
	}
	wg.Wait()
}
