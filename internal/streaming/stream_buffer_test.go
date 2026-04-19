package streaming

import "testing"

func TestStreamBufferReadConsumeAndAppend(t *testing.T) {
	buffer := NewStreamBuffer(8)
	defer buffer.Release()

	buffer.AppendString("hello")

	out := make([]byte, 2)
	if n := buffer.Read(out); n != 2 {
		t.Fatalf("Read() = %d, want 2", n)
	}
	if string(out) != "he" {
		t.Fatalf("Read() data = %q, want %q", string(out), "he")
	}

	buffer.AppendString(" world")
	if got := string(buffer.Unread()); got != "llo world" {
		t.Fatalf("Unread() = %q, want %q", got, "llo world")
	}

	buffer.Consume(4)
	if got := string(buffer.Unread()); got != "world" {
		t.Fatalf("Unread() after Consume() = %q, want %q", got, "world")
	}
}

func TestStreamBufferReleaseIsIdempotent(t *testing.T) {
	buffer := NewStreamBuffer(8)
	buffer.AppendString("data")

	buffer.Release()
	buffer.Release()

	if got := buffer.Len(); got != 0 {
		t.Fatalf("Len() after Release() = %d, want 0", got)
	}
	if got := buffer.Unread(); got != nil {
		t.Fatalf("Unread() after Release() = %v, want nil", got)
	}
}
