package ring

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	rb := New(1024)
	if rb.Cap() != 1024 {
		t.Fatalf("Cap = %d, want 1024", rb.Cap())
	}
	if !rb.IsEmpty() {
		t.Fatal("new buffer should be empty")
	}
}

func TestNewZero(t *testing.T) {
	rb := New(0)
	if rb.Cap() != 0 {
		t.Fatalf("Cap = %d, want 0", rb.Cap())
	}
	if !rb.IsEmpty() {
		t.Fatal("zero buffer should be empty")
	}
}

func TestWriteRead(t *testing.T) {
	rb := New(128)
	data := []byte("hello world")
	n, err := rb.Write(data)
	if err != nil || n != len(data) {
		t.Fatalf("Write = %d, %v, want %d, nil", n, err, len(data))
	}
	if rb.Buffered() != len(data) {
		t.Fatalf("Buffered = %d, want %d", rb.Buffered(), len(data))
	}

	buf := make([]byte, len(data))
	n, err = rb.Read(buf)
	if err != nil || n != len(data) {
		t.Fatalf("Read = %d, %v, want %d, nil", n, err, len(data))
	}
	if string(buf) != "hello world" {
		t.Fatalf("Read data = %q, want %q", buf, "hello world")
	}
	if !rb.IsEmpty() {
		t.Fatal("buffer should be empty after read")
	}
}

func TestWriteByteReadByte(t *testing.T) {
	rb := New(16)
	for i := byte(0); i < 10; i++ {
		rb.WriteByte(i)
	}
	for i := byte(0); i < 10; i++ {
		b, err := rb.ReadByte()
		if err != nil {
			t.Fatalf("ReadByte error: %v", err)
		}
		if b != i {
			t.Fatalf("ReadByte = %d, want %d", b, i)
		}
	}
}

func TestPeek(t *testing.T) {
	rb := New(64)
	data := []byte("hello")
	rb.Write(data)

	head, tail := rb.Peek(3)
	if len(tail) != 0 {
		t.Fatalf("Peek tail len = %d, want 0", len(tail))
	}
	if string(head) != "hel" {
		t.Fatalf("Peek = %q, want %q", head, "hel")
	}
	// Peek should not advance read pointer.
	if rb.Buffered() != 5 {
		t.Fatalf("Buffered after Peek = %d, want 5", rb.Buffered())
	}
}

func TestDiscard(t *testing.T) {
	rb := New(64)
	rb.Write([]byte("hello world"))
	n, err := rb.Discard(6)
	if err != nil || n != 6 {
		t.Fatalf("Discard = %d, %v, want 6, nil", n, err)
	}
	buf := make([]byte, 10)
	n, _ = rb.Read(buf)
	if string(buf[:n]) != "world" {
		t.Fatalf("after Discard, Read = %q, want %q", buf[:n], "world")
	}
}

func TestBytes(t *testing.T) {
	rb := New(64)
	data := []byte("hello world")
	rb.Write(data)
	b := rb.Bytes()
	if !bytes.Equal(b, data) {
		t.Fatalf("Bytes = %q, want %q", b, data)
	}
	// Bytes should not advance read pointer.
	if rb.Buffered() != len(data) {
		t.Fatalf("Buffered after Bytes = %d, want %d", rb.Buffered(), len(data))
	}
}

func TestWriteString(t *testing.T) {
	rb := New(64)
	n, err := rb.WriteString("hello")
	if err != nil || n != 5 {
		t.Fatalf("WriteString = %d, %v, want 5, nil", n, err)
	}
	buf := make([]byte, 5)
	rb.Read(buf)
	if string(buf) != "hello" {
		t.Fatalf("Read after WriteString = %q, want hello", buf)
	}
}

func TestGrow(t *testing.T) {
	rb := New(8)
	data := []byte(strings.Repeat("a", 100))
	n, err := rb.Write(data)
	if err != nil || n != 100 {
		t.Fatalf("Write 100 bytes = %d, %v, want 100, nil", n, err)
	}
	if rb.Buffered() != 100 {
		t.Fatalf("Buffered = %d, want 100", rb.Buffered())
	}
	if rb.Cap() < 100 {
		t.Fatalf("Cap = %d, want >= 100", rb.Cap())
	}
}

func TestWrapAround(t *testing.T) {
	rb := New(8) // capacity 8
	// Fill buffer.
	rb.Write([]byte("12345678"))
	if !rb.IsFull() {
		t.Fatal("buffer should be full")
	}
	// Read some to make room at the beginning.
	buf := make([]byte, 4)
	rb.Read(buf)
	if string(buf) != "1234" {
		t.Fatalf("Read = %q, want 1234", buf)
	}
	// Write more, should wrap around.
	rb.Write([]byte("abcd"))
	// Read all.
	buf = make([]byte, 8)
	n, _ := rb.Read(buf)
	if string(buf[:n]) != "5678abcd" {
		t.Fatalf("after wrap, Read = %q, want 5678abcd", buf[:n])
	}
}

func TestReadFrom(t *testing.T) {
	rb := New(32)
	r := strings.NewReader("hello world from reader")
	n, err := rb.ReadFrom(r)
	if err != nil {
		t.Fatalf("ReadFrom error: %v", err)
	}
	if int(n) != len("hello world from reader") {
		t.Fatalf("ReadFrom n = %d, want %d", n, len("hello world from reader"))
	}
	buf := make([]byte, 100)
	m, _ := rb.Read(buf)
	if string(buf[:m]) != "hello world from reader" {
		t.Fatalf("Read = %q, want hello world from reader", buf[:m])
	}
}

func TestWriteTo(t *testing.T) {
	rb := New(32)
	rb.Write([]byte("hello world"))
	var buf bytes.Buffer
	n, err := rb.WriteTo(&buf)
	if err != nil && err != io.EOF {
		t.Fatalf("WriteTo error: %v", err)
	}
	if int(n) != 11 {
		t.Fatalf("WriteTo n = %d, want 11", n)
	}
	if buf.String() != "hello world" {
		t.Fatalf("WriteTo result = %q, want hello world", buf.String())
	}
}

func TestReset(t *testing.T) {
	rb := New(64)
	rb.Write([]byte("hello"))
	rb.Reset()
	if !rb.IsEmpty() {
		t.Fatal("buffer should be empty after Reset")
	}
	if rb.Buffered() != 0 {
		t.Fatalf("Buffered after Reset = %d, want 0", rb.Buffered())
	}
}

func TestReadEmpty(t *testing.T) {
	rb := New(16)
	buf := make([]byte, 10)
	n, err := rb.Read(buf)
	if n != 0 || err != ErrIsEmpty {
		t.Fatalf("Read empty = %d, %v, want 0, ErrIsEmpty", n, err)
	}
}

func TestWriteToEmpty(t *testing.T) {
	rb := New(16)
	var buf bytes.Buffer
	n, err := rb.WriteTo(&buf)
	if n != 0 || err != ErrIsEmpty {
		t.Fatalf("WriteTo empty = %d, %v, want 0, ErrIsEmpty", n, err)
	}
}