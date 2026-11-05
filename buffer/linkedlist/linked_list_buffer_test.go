package linkedlist

import (
	"bytes"
	"io"
	"testing"
)

func TestPushBackPopFIFO(t *testing.T) {
	var b Buffer
	b.PushBack([]byte("abc"))
	b.PushBack([]byte("def"))
	if b.Buffered() != 6 || b.Len() != 2 {
		t.Fatalf("unexpected state: buffered=%d nodes=%d", b.Buffered(), b.Len())
	}
	if got := string(b.Pop()); got != "abc" {
		t.Fatalf("first Pop = %q, want abc", got)
	}
	if got := string(b.Pop()); got != "def" {
		t.Fatalf("second Pop = %q, want def", got)
	}
	if !b.IsEmpty() || b.Pop() != nil {
		t.Fatal("buffer should be empty after draining")
	}
}

func TestPushFrontAndAppend(t *testing.T) {
	var b Buffer
	b.PushBack([]byte("tail"))
	b.PushFront([]byte("head"))
	b.Append([]byte("last"))
	want := []string{"head", "tail", "last"}
	for _, w := range want {
		if got := string(b.Pop()); got != w {
			t.Fatalf("Pop = %q, want %q", got, w)
		}
	}
}

func TestReadPartialAndEOF(t *testing.T) {
	var b Buffer
	b.PushBack([]byte("hello world"))
	p := make([]byte, 5)
	n, err := b.Read(p)
	if err != nil || n != 5 || string(p[:n]) != "hello" {
		t.Fatalf("first Read = %q, %d, %v", p[:n], n, err)
	}
	rest, err := io.ReadAll(&b)
	if err != nil || string(rest) != " world" {
		t.Fatalf("remaining = %q, %v", rest, err)
	}
	if _, err := b.Read(p); err != io.EOF {
		t.Fatalf("expected io.EOF on empty buffer, got %v", err)
	}
}

func TestReadFromAndWriteTo(t *testing.T) {
	var b Buffer
	src := bytes.NewBufferString("streamed payload")
	n, err := b.ReadFrom(src)
	if err != nil || n != int64(len("streamed payload")) {
		t.Fatalf("ReadFrom = %d, %v", n, err)
	}
	var dst bytes.Buffer
	written, err := b.WriteTo(&dst)
	if err != nil {
		t.Fatalf("WriteTo error: %v", err)
	}
	if written != n || dst.String() != "streamed payload" {
		t.Fatalf("WriteTo = %d, %q", written, dst.String())
	}
	if !b.IsEmpty() {
		t.Fatal("WriteTo should drain the buffer")
	}
}

func TestPeekDiscardAndReset(t *testing.T) {
	var b Buffer
	b.PushBack([]byte("abcd"))
	b.PushBack([]byte("efgh"))
	peeked, err := b.Peek(5)
	if err != nil {
		t.Fatalf("Peek: %v", err)
	}
	var joined []byte
	for _, p := range peeked {
		joined = append(joined, p...)
	}
	if string(joined) != "abcde" {
		t.Fatalf("Peek = %q, want abcde", joined)
	}
	if b.Buffered() != 8 {
		t.Fatalf("Peek must not consume data, buffered=%d", b.Buffered())
	}
	if _, err := b.Peek(99); err != io.ErrShortBuffer {
		t.Fatalf("expected io.ErrShortBuffer, got %v", err)
	}
	discarded, err := b.Discard(6)
	if err != nil || discarded != 6 || b.Buffered() != 2 {
		t.Fatalf("Discard = %d, %v, buffered=%d", discarded, err, b.Buffered())
	}
	b.Reset()
	if !b.IsEmpty() || b.Buffered() != 0 || b.Len() != 0 {
		t.Fatal("Reset did not clear the buffer")
	}
}

func TestPeekWithBytes(t *testing.T) {
	var b Buffer
	b.PushBack([]byte("world"))
	// maxBytes is checked against Buffered() only; caller-supplied bytes are
	// prepended and then the buffer is read up to the remaining budget.
	bs, err := b.PeekWithBytes(5, []byte("abc"))
	if err != nil {
		t.Fatalf("PeekWithBytes: %v", err)
	}
	var joined []byte
	for _, p := range bs {
		joined = append(joined, p...)
	}
	if string(joined) != "abcwo" {
		t.Fatalf("PeekWithBytes = %q, want abcwo", joined)
	}
}

func TestAllocNodeFreeNode(t *testing.T) {
	var b Buffer
	p := b.AllocNode(16)
	if len(p) != 16 {
		t.Fatalf("AllocNode returned len %d", len(p))
	}
	b.FreeNode(p)
}
