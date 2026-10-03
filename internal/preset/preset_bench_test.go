package preset

import (
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkDecodeText_ASCII(b *testing.B) {
	sample := []byte("Title: Ancient Aliens\nAuthor: Skillsaw\nEngine: Boom\nDescription: 32 levels of alien megawad.")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = DecodeText(sample)
	}
}

func BenchmarkDecodeText_CP437(b *testing.B) {
	// Sample with CP437 box-drawing and block-fill characters
	sample := []byte("╔══════════════════════════════════════╗\n" +
		"║  DOOM: THE WAY ID DID - EPISODE 1    ║\n" +
		"╚══════════════════════════════════════╝\n" +
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

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ResolveFile(tmpDir, "eviternityii.wad")
	}
}
