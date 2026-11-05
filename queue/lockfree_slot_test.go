package queue

import (
	"testing"
	"unsafe"

	"github.com/qoderwork/go-infra/assert"
)

// TestCasSlotOccupiesCacheLine guards the false-sharing padding assumption: the
// putNo+getNo+interface{} field set is assumed to be 32 bytes, so casSlotPad
// rounds each slot up to exactly one cache line. If a future field change breaks
// that arithmetic this test fails instead of silently reintroducing false
// sharing between producers and consumers on adjacent positions.
func TestCasSlotOccupiesCacheLine(t *testing.T) {
	if unsafe.Sizeof(casSlot{})%cacheLineSize != 0 {
		t.Fatalf("casSlot size %d is not a multiple of cacheLineSize %d",
			unsafe.Sizeof(casSlot{}), cacheLineSize)
	}
	assert.Equal(t, uintptr(cacheLineSize), unsafe.Sizeof(casSlot{}))
}
