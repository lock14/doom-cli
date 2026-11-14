package preset

import (
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkLoadCuratedCatalog(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		cat, err := LoadCuratedCatalog()
		if err != nil || len(cat.Presets) == 0 {
			b.Fatalf("LoadCuratedCatalog failed: %v", err)
		}
	}
}

func BenchmarkDecodeText_ASCII(b *testing.B) {
	sample := []byte("Title: Ancient Aliens\nAuthor: Skillsaw\nEngine: Boom\nDescription: 32 levels of alien megawad.")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = DecodeText(sample)
	}
}

func BenchmarkDecodeText_CP437(b *testing.B) {
	// Raw CP437 box-drawing (0xC9, 0xCD, 0xBB, 0xBA, 0xC8, 0xBC) and block-fill bytes
	sample := []byte("\xc9\xcd\xcd\xcd\xcd\xcd\xcd\xcd\xcd\xcd\xcd\xcd\xcd\xcd\xcd\xbb\n" +
		"\xba  DOOM CP437  \xba\n" +
		"\xc8\xcd\xcd\xcd\xcd\xcd\xcd\xcd\xcd\xcd\xcd\xcd\xcd\xcd\xcd\xbc\n" +
		"\xb0\xb1\xb2\xdb\xdf\xdc")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = DecodeText(sample)
	}
}

func BenchmarkNormalizeFilename(b *testing.B) {
	name := "Eviternity_II - Map01.wad"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NormalizeFilename(name)
	}
}

func BenchmarkResolveFile(b *testing.B) {
	tmpDir := b.TempDir()
	files := []string{"doom2.wad", "Eviternity II.wad", "eviternity.wad", "av.wad", "av.deh", "gdturbo.wad"}
	for _, f := range files {
		_ = os.WriteFile(filepath.Join(tmpDir, f), []byte("test"), 0644)
	}
	idx := NewDirIndex(tmpDir)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = idx.ResolveFile("eviternityii.wad")
	}
}
