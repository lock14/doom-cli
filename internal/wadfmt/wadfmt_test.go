package wadfmt

import (
	"bytes"
	"io"
	"testing"
)

func TestBuildAndExtractLump(t *testing.T) {
	t.Parallel()

	raw, err := BuildBytes("PWAD", []Lump{
		{Name: "MAP01", Data: nil},
		{Name: "DEHACKED", Data: []byte("Patch File for DeHackEd v3.0\n")},
	})
	if err != nil {
		t.Fatalf("BuildBytes failed: %v", err)
	}

	hdr, entries, err := ReadDirectory(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatalf("ReadDirectory failed: %v", err)
	}
	if hdr.Ident != "PWAD" || len(entries) != 2 {
		t.Fatalf("unexpected header/entries: %+v (%d entries)", hdr, len(entries))
	}

	got, ok := ExtractLump(bytes.NewReader(raw), int64(len(raw)), "dehacked", 1024)
	if !ok || string(got) != "Patch File for DeHackEd v3.0\n" {
		t.Fatalf("ExtractLump(dehacked) = %q, %v", string(got), ok)
	}
	if _, ok := ExtractLump(bytes.NewReader(raw), int64(len(raw)), "MAP01", 1024); ok {
		t.Fatal("expected zero-byte marker lump MAP01 to return false")
	}
	if _, ok := ExtractLump(bytes.NewReader(raw), int64(len(raw)), "dehacked", 4); ok {
		t.Fatal("expected maxBytes cap to reject oversized lump")
	}
}

func TestReadDirectory_RejectsCorruptHeaders(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		data []byte
	}{
		{name: "too short", data: []byte("PWAD")},
		{name: "invalid magic", data: []byte("ZWAD\x01\x00\x00\x00\x0c\x00\x00\x00................")},
		{name: "zero lumps", data: []byte("PWAD\x00\x00\x00\x00\x0c\x00\x00\x00")},
		{name: "truncated directory", data: []byte("PWAD\x02\x00\x00\x00\x0c\x00\x00\x00................")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, _, err := ReadDirectory(bytes.NewReader(tc.data), int64(len(tc.data))); err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
		})
	}
}

func TestNewStreamingPWADReader_SharedOffsets(t *testing.T) {
	t.Parallel()

	oggPayload := []byte("OggS-fake-track-bytes")
	r, count, err := NewStreamingPWADReader([]SectionPayload{
		{
			Reader: bytes.NewReader(oggPayload),
			Size:   uint32(len(oggPayload)),
			Names:  []string{"D_RUNNIN", "D_RUNNI2"},
		},
	})
	if err != nil {
		t.Fatalf("NewStreamingPWADReader failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 directory entries, got %d", count)
	}

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	// 12-byte header + 1 copy of payload + 2 * 16-byte directory entries.
	wantLen := 12 + len(oggPayload) + 32
	if len(out) != wantLen {
		t.Fatalf("got PWAD size %d, want %d", len(out), wantLen)
	}

	_, entries, err := ReadDirectory(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("ReadDirectory failed: %v", err)
	}
	if entries[0].Offset != entries[1].Offset || entries[0].Name != "D_RUNNIN" || entries[1].Name != "D_RUNNI2" {
		t.Fatalf("expected shared offsets for D_RUNNIN and D_RUNNI2, got %+v", entries)
	}
}
