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

func TestPeekAll(t *testing.T) {
	rb := New(64)
	data := []byte("hello world")
	rb.Write(data)

	head, tail := rb.Peek(0)
	all := append(head, tail...)
	if !bytes.Equal(all, data) {
		t.Fatalf("Peek(0) = %q, want %q", all, data)
	}
}

func TestPeekAllEmpty(t *testing.T) {
	rb := New(64)
	head, tail := rb.Peek(0)
	if head != nil || tail != nil {
		t.Fatal("Peek on empty buffer should return nil slices")
	}
}

func TestPeekEmpty(t *testing.T) {
	rb := New(64)
	head, tail := rb.Peek(5)
	if head != nil || tail != nil {
		t.Fatal("Peek on empty buffer should return nil slices")
	}
}

func TestDiscardAll(t *testing.T) {
	rb := New(64)
	rb.Write([]byte("hello"))
	n, err := rb.Discard(100)
	if err != nil {
		t.Fatalf("Discard error: %v", err)
	}
	if n != 5 {
		t.Fatalf("Discard = %d, want 5", n)
	}
	if !rb.IsEmpty() {
		t.Fatal("buffer should be empty after discarding all")
	}
}

func TestDiscardZero(t *testing.T) {
	rb := New(64)
	rb.Write([]byte("hello"))
	n, err := rb.Discard(0)
	if err != nil || n != 0 {
		t.Fatalf("Discard(0) = %d, %v, want 0, nil", n, err)
	}
	if rb.Buffered() != 5 {
		t.Fatalf("Buffered = %d, want 5", rb.Buffered())
	}
}

func TestDiscardEmpty(t *testing.T) {
	rb := New(64)
	n, err := rb.Discard(5)
	if err != nil || n != 0 {
		t.Fatalf("Discard empty = %d, %v, want 0, nil", n, err)
	}
}

func TestReadByteEmpty(t *testing.T) {
	rb := New(16)
	_, err := rb.ReadByte()
	if err != ErrIsEmpty {
		t.Fatalf("ReadByte empty error = %v, want ErrIsEmpty", err)
	}
}

func TestWriteEmptySlice(t *testing.T) {
	rb := New(16)
	n, err := rb.Write(nil)
	if err != nil || n != 0 {
		t.Fatalf("Write nil = %d, %v, want 0, nil", n, err)
	}
	n, err = rb.Write([]byte{})
	if err != nil || n != 0 {
		t.Fatalf("Write empty = %d, %v, want 0, nil", n, err)
	}
}

func TestWriteStringEmpty(t *testing.T) {
	rb := New(16)
	n, err := rb.WriteString("")
	if err != nil || n != 0 {
		t.Fatalf("WriteString empty = %d, %v, want 0, nil", n, err)
	}
}

func TestReadZeroLen(t *testing.T) {
	rb := New(16)
	rb.Write([]byte("hello"))
	n, err := rb.Read(make([]byte, 0))
	if err != nil || n != 0 {
		t.Fatalf("Read zero len = %d, %v, want 0, nil", n, err)
	}
}

func TestPeekWrapAround(t *testing.T) {
	rb := New(8)
	rb.Write([]byte("12345678"))
	buf := make([]byte, 4)
	rb.Read(buf) // read "1234", now r=4, w=0, isEmpty=false
	rb.Write([]byte("ab")) // write "ab", now w=2

	head, tail := rb.Peek(6)
	all := append(head, tail...)
	expected := []byte("5678ab")
	if !bytes.Equal(all, expected) {
		t.Fatalf("Peek wrap = %q, want %q", all, expected)
	}
}

func TestBytesWrapAround(t *testing.T) {
	rb := New(8)
	rb.Write([]byte("12345678"))
	buf := make([]byte, 4)
	rb.Read(buf)
	rb.Write([]byte("ab"))

	b := rb.Bytes()
	expected := []byte("5678ab")
	if !bytes.Equal(b, expected) {
		t.Fatalf("Bytes wrap = %q, want %q", b, expected)
	}
}

func TestBytesFull(t *testing.T) {
	rb := New(8)
	rb.Write([]byte("12345678"))
	b := rb.Bytes()
	if !bytes.Equal(b, []byte("12345678")) {
		t.Fatalf("Bytes full = %q, want 12345678", b)
	}
}

func TestWriteByteGrow(t *testing.T) {
	rb := New(4)
	for i := 0; i < 10; i++ {
		err := rb.WriteByte(byte('a' + i))
		if err != nil {
			t.Fatalf("WriteByte error: %v", err)
		}
	}
	if rb.Buffered() != 10 {
		t.Fatalf("Buffered = %d, want 10", rb.Buffered())
	}
	if rb.Cap() < 10 {
		t.Fatalf("Cap = %d, want >= 10", rb.Cap())
	}
}

func TestGrowFromZero(t *testing.T) {
	rb := New(0)
	rb.Write([]byte("hello"))
	if rb.Cap() < DefaultBufferSize {
		t.Fatalf("Cap = %d, want >= %d", rb.Cap(), DefaultBufferSize)
	}
}

func TestAvailableAndIsFull(t *testing.T) {
	rb := New(8)
	if rb.Available() != 8 {
		t.Fatalf("Available empty = %d, want 8", rb.Available())
	}
	if rb.IsFull() {
		t.Fatal("empty buffer should not be full")
	}

	rb.Write([]byte("1234"))
	if rb.Available() != 4 {
		t.Fatalf("Available half = %d, want 4", rb.Available())
	}

	rb.Write([]byte("5678"))
	if rb.Available() != 0 {
		t.Fatalf("Available full = %d, want 0", rb.Available())
	}
	if !rb.IsFull() {
		t.Fatal("buffer should be full")
	}
}

func TestReadFromWrapAround(t *testing.T) {
	rb := New(16)
	// Fill first half and read it to move r pointer
	rb.Write([]byte("0123456789abcdef"))
	buf := make([]byte, 10)
	rb.Read(buf) // r = 10
	// Now ReadFrom more data that wraps around
	r := strings.NewReader("XYZUVW")
	n, err := rb.ReadFrom(r)
	if err != nil {
		t.Fatalf("ReadFrom error: %v", err)
	}
	if n != 6 {
		t.Fatalf("ReadFrom n = %d, want 6", n)
	}
	if rb.Buffered() != 12 {
		t.Fatalf("Buffered = %d, want 12", rb.Buffered())
	}
}

func TestWriteToWrapAround(t *testing.T) {
	rb := New(16)
	rb.Write([]byte("0123456789abcdef"))
	buf := make([]byte, 10)
	rb.Read(buf) // r = 10
	rb.Write([]byte("XYZ")) // write some to wrap

	var out bytes.Buffer
	n, err := rb.WriteTo(&out)
	if err != nil && err != io.ErrShortWrite {
		t.Fatalf("WriteTo error: %v", err)
	}
	if n != 9 {
		t.Fatalf("WriteTo n = %d, want 9", n)
	}
	expected := "abcdefXYZ"
	if out.String() != expected {
		t.Fatalf("WriteTo = %q, want %q", out.String(), expected)
	}
}

func TestLen(t *testing.T) {
	rb := New(16)
	if rb.Len() != 16 {
		t.Fatalf("Len = %d, want 16", rb.Len())
	}
}

func TestMultipleReadWriteCycles(t *testing.T) {
	rb := New(8)
	for i := 0; i < 100; i++ {
		rb.Write([]byte("abcd"))
		buf := make([]byte, 4)
		n, _ := rb.Read(buf)
		if n != 4 || string(buf) != "abcd" {
			t.Fatalf("cycle %d: got %q (n=%d)", i, buf[:n], n)
		}
	}
}

func TestReadFromEOF(t *testing.T) {
	rb := New(32)
	r := strings.NewReader("hello")
	n, err := rb.ReadFrom(r)
	if err != nil {
		t.Fatalf("ReadFrom error: %v", err)
	}
	if n != 5 {
		t.Fatalf("ReadFrom n = %d, want 5", n)
	}
}

func TestReadFromGrow(t *testing.T) {
	rb := New(8)
	data := strings.Repeat("x", 100)
	r := strings.NewReader(data)
	n, err := rb.ReadFrom(r)
	if err != nil {
		t.Fatalf("ReadFrom error: %v", err)
	}
	if int(n) != 100 {
		t.Fatalf("ReadFrom n = %d, want 100", n)
	}
	if rb.Buffered() != 100 {
		t.Fatalf("Buffered = %d, want 100", rb.Buffered())
	}
}

func TestWriteToShortWrite(t *testing.T) {
	rb := New(32)
	rb.Write([]byte("hello world"))

	w := &shortWriter{limit: 5}
	n, err := rb.WriteTo(w)
	if err != io.ErrShortWrite {
		t.Fatalf("WriteTo error = %v, want io.ErrShortWrite", err)
	}
	if n != 5 {
		t.Fatalf("WriteTo n = %d, want 5", n)
	}
}

type shortWriter struct {
	limit int
	written int
}

func (w *shortWriter) Write(p []byte) (int, error) {
	if w.written >= w.limit {
		return 0, io.ErrShortWrite
	}
	n := len(p)
	if w.written + n > w.limit {
		n = w.limit - w.written
	}
	w.written += n
	return n, nil
}

func TestGrowLargeBuffer(t *testing.T) {
	rb := New(8 * 1024) // start above 4KB threshold
	data := make([]byte, 32 * 1024) // grow beyond double
	n, err := rb.Write(data)
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != len(data) {
		t.Fatalf("Write n = %d, want %d", n, len(data))
	}
	if rb.Cap() < len(data) {
		t.Fatalf("Cap = %d, want >= %d", rb.Cap(), len(data))
	}
}

func TestPeekAllWrap(t *testing.T) {
	rb := New(8)
	rb.Write([]byte("12345678"))
	buf := make([]byte, 3)
	rb.Read(buf) // read "123"
	rb.Write([]byte("ab")) // write "ab", wraps

	head, tail := rb.Peek(0) // peek all
	all := append(head, tail...)
	expected := []byte("45678ab")
	if !bytes.Equal(all, expected) {
		t.Fatalf("PeekAll wrap = %q, want %q", all, expected)
	}
}

func TestReadFromMidWrap(t *testing.T) {
	// Test ReadFrom when w < r (wrapped state) and we read into the middle section
	rb := New(16)
	rb.Write([]byte("0123456789abcdef"))
	buf := make([]byte, 12)
	rb.Read(buf) // r = 12
	rb.Write([]byte("XY")) // w = 2, wrapped

	// Now ReadFrom: w=2, r=12, so w < r, middle space is buf[2:12]
	r := strings.NewReader("abcdefghij")
	n, err := rb.ReadFrom(r)
	if err != nil {
		t.Fatalf("ReadFrom error: %v", err)
	}
	if n != 10 {
		t.Fatalf("ReadFrom n = %d, want 10", n)
	}
}

func TestWriteToWrapShortWrite(t *testing.T) {
	// Test WriteTo with wrap-around and short write in the second segment
	rb := New(16)
	rb.Write([]byte("0123456789abcdef"))
	buf := make([]byte, 10)
	rb.Read(buf) // r = 10
	rb.Write([]byte("XYZ")) // w = 3, wrapped, data is "abcdefXYZ"

	w := &shortWriter{limit: 7} // only write 7 of 9 bytes, first segment is "abcdef" (6 bytes)
	n, err := rb.WriteTo(w)
	if err != io.ErrShortWrite {
		t.Fatalf("WriteTo error = %v, want io.ErrShortWrite", err)
	}
	if n != 7 {
		t.Fatalf("WriteTo n = %d, want 7", n)
	}
}