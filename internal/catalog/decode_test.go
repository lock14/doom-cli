package catalog

import "testing"

func TestDecodeTextCP437(t *testing.T) {
	t.Parallel()
	ascii := []byte("Simple ASCII text\r\nLine 2")
	if got := DecodeText(ascii); got != "Simple ASCII text\nLine 2" {
		t.Fatalf("unexpected ASCII decode: %q", got)
	}
	cp437 := []byte{0xc9, 0xcd, 0xbb} // ╔═╗ in CP437
	if got := DecodeText(cp437); got != "╔═╗" {
		t.Fatalf("unexpected CP437 decode: %q", got)
	}
	// Verify DecodeText maps CP437 0x81 ('ü') cleanly without U+FFFD replacement
	if got := DecodeText([]byte{'M', 0x81, 'l', 'l', 'e', 'r'}); got != "Müller" {
		t.Errorf("DecodeText(CP437 0x81) = %q, want \"Müller\"", got)
	}
}
