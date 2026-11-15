package steam

import (
	"errors"
	"io"
	"math"
	"os"
	"strings"

	"github.com/lock14/doom-cli/internal/fsutil"
	"github.com/lock14/doom-cli/internal/wadfmt"
)

// idkfaReusedTrackSlots maps an H_ lump name in extras.wad to additional D_ lump
// names that share the same composition in DOOM.WAD and DOOM2.WAD (per TRAKINFO).
// Multiple WAD directory entries share the same lump offset and size without
// duplicating audio payload bytes.
var idkfaReusedTrackSlots = map[string][]string{
	// Doom I reused music slots
	"H_INTRO": {"D_INTROA"},
	"H_E1M6":  {"D_E3M6"},
	"H_E1M7":  {"D_E2M5", "D_E3M5"},
	"H_E1M8":  {"D_E3M4"},
	"H_E1M9":  {"D_E3M9"},
	"H_E2M3":  {"D_INTER"},
	"H_E2M7":  {"D_E3M7"},
	"H_E2M9":  {"D_E3M1"},
	// Doom II reused music slots
	"H_RUNNIN": {"D_RUNNI2"},
	"H_STALKS": {"D_STLKS2", "D_STLKS3"},
	"H_COUNTD": {"D_COUNT2"},
	"H_DOOM":   {"D_DOOM2"},
	"H_THE_DA": {"D_THEDA2", "D_THEDA3"},
	"H_SHAWN":  {"D_SHAWN2", "D_SHAWN3"},
	"H_DDTBLU": {"D_DDTBL2", "D_DDTBL3"},
	"H_DEAD":   {"D_DEAD2"},
	"H_ROMERO": {"D_ROMER2"},
	"H_MESSAG": {"D_MESSG2"},
}

// SynthesizeIDKFAWAD extracts Andrew Hulshult's IDKFA soundtrack lumps (prefixed with H_)
// from a DOOM + DOOM II (2024) extras.wad file, renames each lump prefix from H_ to D_
// (including reused map music slots from TRAKINFO) so classic source ports play them
// automatically, and writes a valid PWAD atomically to dstPath.
func SynthesizeIDKFAWAD(srcExtrasPath, dstPath string) (int, error) {
	f, err := os.Open(srcExtrasPath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return 0, err
	}
	fileSize := fi.Size()

	_, entries, err := wadfmt.ReadDirectory(f, fileSize)
	if err != nil {
		return 0, err
	}

	type audioTrack struct {
		srcPos int64
		size   uint32
		hName  string
		dName  string
	}

	var tracks []audioTrack
	seenDNames := make(map[string]bool)

	for _, entry := range entries {
		if entry.Size < 4 || entry.Offset < 12 || entry.Offset+entry.Size > fileSize {
			continue
		}
		upperName := strings.ToUpper(entry.Name)
		if len(upperName) < 3 || !strings.HasPrefix(upperName, "H_") {
			continue
		}
		dName := "D_" + upperName[2:]
		if seenDNames[dName] {
			continue
		}
		var magic [4]byte
		if _, err := f.ReadAt(magic[:], entry.Offset); err != nil || string(magic[:]) != "OggS" {
			continue
		}
		if entry.Size > math.MaxInt32 {
			return 0, errors.New("synthesized WAD exceeds signed 32-bit PWAD limit")
		}
		seenDNames[dName] = true
		tracks = append(tracks, audioTrack{
			srcPos: entry.Offset,
			size:   uint32(entry.Size),
			hName:  upperName,
			dName:  dName,
		})
	}

	if len(tracks) == 0 {
		return 0, errors.New("no H_ soundtrack lumps found in extras.wad")
	}

	sections := make([]wadfmt.SectionPayload, 0, len(tracks))
	for _, tr := range tracks {
		names := []string{tr.dName}
		for _, alias := range idkfaReusedTrackSlots[tr.hName] {
			if !seenDNames[alias] {
				seenDNames[alias] = true
				names = append(names, alias)
			}
		}
		sections = append(sections, wadfmt.SectionPayload{
			Reader: io.NewSectionReader(f, tr.srcPos, int64(tr.size)),
			Size:   tr.size,
			Names:  names,
		})
	}

	pwadReader, lumpCount, err := wadfmt.NewStreamingPWADReader(sections)
	if err != nil {
		return 0, err
	}
	if _, _, err := fsutil.WriteReaderAtomic(dstPath, pwadReader, 0o644); err != nil {
		return 0, err
	}
	return lumpCount, nil
}
