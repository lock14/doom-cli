package tui

import (
	"testing"

	"github.com/lock14/doom-cli/internal/preset"
)

func BenchmarkModel_ComputeLayout_SideBySide(b *testing.B) {
	cat, _ := preset.LoadCatalog("")
	m := initialModel(cat, "", DefaultTheme, false)
	m.width = 120
	m.height = 40

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.computeLayout()
	}
}

func BenchmarkModel_ComputeLayout_Stacked(b *testing.B) {
	cat, _ := preset.LoadCatalog("")
	m := initialModel(cat, "", DefaultTheme, false)
	m.width = 80
	m.height = 24

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.computeLayout()
	}
}

func BenchmarkModel_FuzzyFiltering(b *testing.B) {
	cat, _ := preset.LoadCatalog("")
	m := initialModel(cat, "", DefaultTheme, false)

	queries := []string{"alien", "evi", "boom", "sunder", "rust", "sigil"}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q := queries[i%len(queries)]
		m.updateFiltered(q)
	}
}
