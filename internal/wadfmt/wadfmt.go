// Package wadfmt reads and writes binary Doom IWAD and PWAD lump containers.
package wadfmt

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
)

// MaxLumps caps the number of directory entries accepted when parsing a WAD header.
const MaxLumps = 65536

// Header describes the 12-byte IWAD or PWAD file header.
type Header struct {
	Ident     string
	NumLumps  int64
	DirOffset int64
}

// DirEntry describes a single 16-byte entry in a WAD lump directory.
type DirEntry struct {
	Offset int64
	Size   int64
	Name   string
}

// Lump holds an in-memory lump name and payload for building a WAD archive.
type Lump struct {
	Name string
	Data []byte
}

// SectionPayload binds a single streamed payload reader to one or more 8-character
// lump names that share the same byte offset and size in a synthesized PWAD.
type SectionPayload struct {
	Reader io.Reader
	Size   uint32
	Names  []string
}

// ReadDirectory validates the 12-byte WAD header and parses all 16-byte directory entries.
func ReadDirectory(r io.ReaderAt, dataLen int64) (Header, []DirEntry, error) {
	if r == nil || dataLen < 12 {
		return Header{}, nil, errors.New("file too small to be a WAD")
	}
	var hdrBuf [12]byte
	if _, err := r.ReadAt(hdrBuf[:], 0); err != nil {
		return Header{}, nil, err
	}
	ident := string(hdrBuf[0:4])
	if ident != "PWAD" && ident != "IWAD" {
		return Header{}, nil, fmt.Errorf("invalid WAD header %q", ident)
	}

	numLumps := int64(binary.LittleEndian.Uint32(hdrBuf[4:8]))
	dirOffset := int64(binary.LittleEndian.Uint32(hdrBuf[8:12]))
	if numLumps <= 0 || numLumps > MaxLumps || dirOffset < 12 || dirOffset+numLumps*16 > dataLen {
		return Header{}, nil, errors.New("corrupt WAD directory bounds")
	}

	dirBuf := make([]byte, numLumps*16)
	if _, err := r.ReadAt(dirBuf, dirOffset); err != nil {
		return Header{}, nil, err
	}

	entries := make([]DirEntry, numLumps)
	for i := int64(0); i < numLumps; i++ {
		raw := dirBuf[i*16 : (i+1)*16]
		nameSlice := raw[8:16]
		if nul := bytes.IndexByte(nameSlice, 0); nul >= 0 {
			nameSlice = nameSlice[:nul]
		}
		entries[i] = DirEntry{
			Offset: int64(binary.LittleEndian.Uint32(raw[0:4])),
			Size:   int64(binary.LittleEndian.Uint32(raw[4:8])),
			Name:   string(nameSlice),
		}
	}

	return Header{
		Ident:     ident,
		NumLumps:  numLumps,
		DirOffset: dirOffset,
	}, entries, nil
}

// ExtractLump extracts the raw payload of the first non-empty lump matching lumpName
// (case-insensitive, up to maxBytes) from an io.ReaderAt without reading the whole WAD.
func ExtractLump(r io.ReaderAt, dataLen int64, lumpName string, maxBytes int64) ([]byte, bool) {
	_, entries, err := ReadDirectory(r, dataLen)
	if err != nil {
		return nil, false
	}
	want := strings.ToUpper(strings.TrimSpace(lumpName))
	if want == "" {
		return nil, false
	}
	for _, entry := range entries {
		if entry.Size <= 0 || (maxBytes > 0 && entry.Size > maxBytes) ||
			entry.Offset < 12 || entry.Offset+entry.Size > dataLen {
			continue
		}
		if strings.EqualFold(entry.Name, want) {
			out := make([]byte, entry.Size)
			if _, err := r.ReadAt(out, entry.Offset); err != nil {
				return nil, false
			}
			return out, true
		}
	}
	return nil, false
}

// NewStreamingPWADReader constructs a streaming io.Reader producing a valid PWAD
// from ordered payload sections without buffering lump payloads in memory.
// Multiple lump names in a single SectionPayload share the same offset and size.
func NewStreamingPWADReader(payloads []SectionPayload) (io.Reader, int, error) {
	if len(payloads) == 0 {
		return nil, 0, errors.New("no lumps provided for PWAD")
	}

	type rawDirEntry struct {
		offset uint32
		size   uint32
		name   [8]byte
	}

	var dirEntries []rawDirEntry
	readers := make([]io.Reader, 0, len(payloads)+2)
	// Placeholder slot 0 for header once directory offset and entry count are known.
	readers = append(readers, nil)

	curOffset := int64(12)
	for _, p := range payloads {
		if len(p.Names) == 0 {
			continue
		}
		if curOffset+int64(p.Size) > math.MaxInt32 {
			return nil, 0, errors.New("synthesized WAD exceeds signed 32-bit PWAD limit")
		}
		outPos := uint32(curOffset)
		for _, lumpName := range p.Names {
			var nameBytes [8]byte
			copy(nameBytes[:], strings.ToUpper(strings.TrimSpace(lumpName)))
			dirEntries = append(dirEntries, rawDirEntry{
				offset: outPos,
				size:   p.Size,
				name:   nameBytes,
			})
		}
		if p.Size > 0 && p.Reader != nil {
			readers = append(readers, p.Reader)
		}
		curOffset += int64(p.Size)
	}

	if len(dirEntries) == 0 {
		return nil, 0, errors.New("no valid directory entries for PWAD")
	}
	if curOffset+int64(len(dirEntries))*16 > math.MaxInt32 {
		return nil, 0, errors.New("synthesized WAD directory exceeds signed 32-bit PWAD limit")
	}

	var hdr [12]byte
	copy(hdr[0:4], "PWAD")
	binary.LittleEndian.PutUint32(hdr[4:8], uint32(len(dirEntries)))
	binary.LittleEndian.PutUint32(hdr[8:12], uint32(curOffset))
	readers[0] = bytes.NewReader(hdr[:])

	outDir := make([]byte, len(dirEntries)*16)
	for i, de := range dirEntries {
		slot := outDir[i*16 : (i+1)*16]
		binary.LittleEndian.PutUint32(slot[0:4], de.offset)
		binary.LittleEndian.PutUint32(slot[4:8], de.size)
		copy(slot[8:16], de.name[:])
	}
	readers = append(readers, bytes.NewReader(outDir))

	return io.MultiReader(readers...), len(dirEntries), nil
}

// BuildBytes serializes an in-memory IWAD or PWAD archive for the given lumps.
func BuildBytes(ident string, lumps []Lump) ([]byte, error) {
	if ident != "IWAD" && ident != "PWAD" {
		return nil, fmt.Errorf("invalid WAD ident %q", ident)
	}
	var buf bytes.Buffer
	var hdr [12]byte
	buf.Write(hdr[:])

	type entry struct {
		offset uint32
		size   uint32
		name   [8]byte
	}
	entries := make([]entry, len(lumps))
	for i, l := range lumps {
		if int64(buf.Len())+int64(len(l.Data)) > math.MaxInt32 {
			return nil, errors.New("WAD payload exceeds 32-bit limit")
		}
		offset := uint32(buf.Len())
		if len(l.Data) > 0 {
			buf.Write(l.Data)
		}
		var nameBytes [8]byte
		copy(nameBytes[:], strings.ToUpper(strings.TrimSpace(l.Name)))
		entries[i] = entry{
			offset: offset,
			size:   uint32(len(l.Data)),
			name:   nameBytes,
		}
	}

	dirOffset := uint32(buf.Len())
	for _, e := range entries {
		var raw [16]byte
		binary.LittleEndian.PutUint32(raw[0:4], e.offset)
		binary.LittleEndian.PutUint32(raw[4:8], e.size)
		copy(raw[8:16], e.name[:])
		buf.Write(raw[:])
	}

	out := buf.Bytes()
	copy(out[0:4], ident)
	binary.LittleEndian.PutUint32(out[4:8], uint32(len(entries)))
	binary.LittleEndian.PutUint32(out[8:12], dirOffset)
	return out, nil
}
