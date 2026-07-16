package elastic

import (
	"bytes"
	"testing"
)

func TestRingBufferWriteRead(t *testing.T) {
	var rb RingBuffer
	defer rb.Done()

	data := []byte("hello world")
	n, err := rb.Write(data)
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != len(data) {
		t.Fatalf("Write returned %d, expected %d", n, len(data))
	}

	if rb.Buffered() != len(data) {
		t.Fatalf("Buffered() = %d, expected %d", rb.Buffered(), len(data))
	}

	buf := make([]byte, len(data))
	n, err = rb.Read(buf)
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}
	if n != len(data) {
		t.Fatalf("Read returned %d, expected %d", n, len(data))
	}
	if !bytes.Equal(buf, data) {
		t.Fatalf("Read data = %q, expected %q", buf, data)
	}

	if !rb.IsEmpty() {
		t.Fatal("expected buffer to be empty after reading all data")
	}
}

func TestRingBufferLazyInit(t *testing.T) {
	var rb RingBuffer
	defer rb.Done()

	if !rb.IsEmpty() {
		t.Fatal("expected empty buffer before write")
	}
	if rb.Buffered() != 0 {
		t.Fatalf("Buffered() = %d, expected 0", rb.Buffered())
	}
	if rb.Cap() != 0 {
		t.Fatalf("Cap() = %d, expected 0", rb.Cap())
	}
}

func TestRingBufferAutoReturn(t *testing.T) {
	var rb RingBuffer
	defer rb.Done()

	data := []byte("test")
	_, _ = rb.Write(data)

	if rb.IsEmpty() {
		t.Fatal("expected non-empty buffer after write")
	}

	buf := make([]byte, len(data))
	_, _ = rb.Read(buf)

	if !rb.IsEmpty() {
		t.Fatal("expected buffer to be empty and returned to pool")
	}
}

func TestRingBufferWriteString(t *testing.T) {
	var rb RingBuffer
	defer rb.Done()

	s := "hello string"
	n, err := rb.WriteString(s)
	if err != nil {
		t.Fatalf("WriteString error: %v", err)
	}
	if n != len(s) {
		t.Fatalf("WriteString returned %d, expected %d", n, len(s))
	}

	buf := make([]byte, len(s))
	_, _ = rb.Read(buf)
	if string(buf) != s {
		t.Fatalf("Read data = %q, expected %q", buf, s)
	}
}

func TestRingBufferPeek(t *testing.T) {
	var rb RingBuffer
	defer rb.Done()

	data := []byte("peek test")
	_, _ = rb.Write(data)

	head, tail := rb.Peek(4)
	peeked := append(head, tail...)
	if !bytes.Equal(peeked, data[:4]) {
		t.Fatalf("Peek(4) = %q, expected %q", peeked, data[:4])
	}

	if rb.Buffered() != len(data) {
		t.Fatal("Peek should not advance read pointer")
	}
}

func TestRingBufferDiscard(t *testing.T) {
	var rb RingBuffer
	defer rb.Done()

	data := []byte("discard test")
	_, _ = rb.Write(data)

	discarded, err := rb.Discard(7)
	if err != nil {
		t.Fatalf("Discard error: %v", err)
	}
	if discarded != 7 {
		t.Fatalf("Discard returned %d, expected 7", discarded)
	}

	remaining := make([]byte, rb.Buffered())
	_, _ = rb.Read(remaining)
	if !bytes.Equal(remaining, data[7:]) {
		t.Fatalf("Remaining data = %q, expected %q", remaining, data[7:])
	}
}

func TestRingBufferReset(t *testing.T) {
	var rb RingBuffer
	defer rb.Done()

	data := []byte("reset test")
	_, _ = rb.Write(data)

	rb.Reset()

	if !rb.IsEmpty() {
		t.Fatal("expected empty buffer after Reset")
	}
	if rb.Buffered() != 0 {
		t.Fatalf("Buffered() = %d after Reset, expected 0", rb.Buffered())
	}
}

func TestRingBufferReadFromWriteTo(t *testing.T) {
	var rb RingBuffer
	defer rb.Done()

	input := bytes.NewBufferString("readfrom test data")
	n, err := rb.ReadFrom(input)
	if err != nil {
		t.Fatalf("ReadFrom error: %v", err)
	}
	if n != int64(len("readfrom test data")) {
		t.Fatalf("ReadFrom returned %d, expected %d", n, len("readfrom test data"))
	}

	var output bytes.Buffer
	n, err = rb.WriteTo(&output)
	if err != nil {
		t.Fatalf("WriteTo error: %v", err)
	}
	if n != int64(len("readfrom test data")) {
		t.Fatalf("WriteTo returned %d, expected %d", n, len("readfrom test data"))
	}
	if output.String() != "readfrom test data" {
		t.Fatalf("WriteTo output = %q, expected %q", output.String(), "readfrom test data")
	}
}

func TestRingBufferReadByte(t *testing.T) {
	var rb RingBuffer
	defer rb.Done()

	data := []byte("ab")
	_, _ = rb.Write(data)

	b1, err := rb.ReadByte()
	if err != nil {
		t.Fatalf("ReadByte error: %v", err)
	}
	if b1 != 'a' {
		t.Fatalf("ReadByte = %q, expected %q", b1, 'a')
	}

	b2, err := rb.ReadByte()
	if err != nil {
		t.Fatalf("ReadByte error: %v", err)
	}
	if b2 != 'b' {
		t.Fatalf("ReadByte = %q, expected %q", b2, 'b')
	}
}

func TestRingBufferWriteByte(t *testing.T) {
	var rb RingBuffer
	defer rb.Done()

	err := rb.WriteByte('x')
	if err != nil {
		t.Fatalf("WriteByte error: %v", err)
	}

	if rb.Buffered() != 1 {
		t.Fatalf("Buffered() = %d, expected 1", rb.Buffered())
	}

	b, err := rb.ReadByte()
	if err != nil {
		t.Fatalf("ReadByte error: %v", err)
	}
	if b != 'x' {
		t.Fatalf("ReadByte = %q, expected %q", b, 'x')
	}
}

func TestRingBufferIsFull(t *testing.T) {
	var rb RingBuffer
	defer rb.Done()

	if rb.IsFull() {
		t.Fatal("empty buffer should not be full")
	}

	rb.Write([]byte("test"))
	if rb.IsFull() {
		t.Fatal("buffer with data should not be full unless capacity is reached")
	}
}

func TestRingBufferBytes(t *testing.T) {
	var rb RingBuffer
	defer rb.Done()

	data := []byte("bytes test")
	_, _ = rb.Write(data)

	b := rb.Bytes()
	if !bytes.Equal(b, data) {
		t.Fatalf("Bytes() = %q, expected %q", b, data)
	}

	if rb.Buffered() != len(data) {
		t.Fatal("Bytes() should not advance read pointer")
	}
}

func TestRingBufferDone(t *testing.T) {
	var rb RingBuffer

	_, _ = rb.Write([]byte("test"))
	rb.Done()

	if !rb.IsEmpty() {
		t.Fatal("expected empty buffer after Done")
	}
}
