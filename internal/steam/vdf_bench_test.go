package steam

import (
	"strings"
	"testing"
)

func BenchmarkParseVDFPaths(b *testing.B) {
	vdfContent := `
"libraryfolders"
{
	"0"
	{
		"path"		"C:\\Program Files (x86)\\Steam"
		"label"		""
		"contentid"	"12345"
	}
	"1"
	{
		"path"		"D:\\Games\\SteamLibrary"
		"label"		"Secondary"
		"contentid"	"67890"
	}
	"2"
	{
		"path"		"/opt/storage/Steam"
		"label"		"LinuxStorage"
	}
}
`
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := strings.NewReader(vdfContent)
		_ = ParseVDFPaths(r)
	}
}
