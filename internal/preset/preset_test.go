package preset

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadCatalog(t *testing.T) {
	cat, err := LoadCatalog("")
	if err != nil {
		t.Fatalf("LoadCatalog failed: %v", err)
	}
	if len(cat.Presets) != 32 {
		t.Errorf("expected 32 presets, got %d", len(cat.Presets))
	}
}

func TestFind(t *testing.T) {
	cat, err := LoadCatalog("")
	if err != nil {
		t.Fatalf("LoadCatalog failed: %v", err)
	}

	// Exact match
	p := cat.Find("Eviternity II")
	if p == nil || p.Name != "Eviternity II" {
		t.Errorf("expected Eviternity II, got %v", p)
	}

	// Case-insensitive
	p = cat.Find("sunlust")
	if p == nil || p.Name != "Sunlust" {
		t.Errorf("expected Sunlust, got %v", p)
	}

	// Prefix
	p = cat.Find("Alien Vend")
	if p == nil || p.Name != "Alien Vendetta" {
		t.Errorf("expected Alien Vendetta, got %v", p)
	}
}

func TestResolveFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "wads_test_*")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create spaced file
	spacedFile := filepath.Join(tmpDir, "Eviternity II.wad")
	if err := os.WriteFile(spacedFile, []byte("wad"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Test normalized matching
	path, found := ResolveFile(tmpDir, "eviternityii.wad")
	if !found || filepath.Base(path) != "Eviternity II.wad" {
		t.Errorf("expected to find Eviternity II.wad, got %s, found=%v", path, found)
	}

	// Create alias file
	gdFile := filepath.Join(tmpDir, "gd.wad")
	if err := os.WriteFile(gdFile, []byte("wad"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	path, found = ResolveFile(tmpDir, "gdturbo.wad")
	if !found || filepath.Base(path) != "gd.wad" {
		t.Errorf("expected to find gd.wad for gdturbo.wad, got %s, found=%v", path, found)
	}
}

func TestPresetParityAndInvariants(t *testing.T) {
	rootDir := filepath.Join("..", "..")
	presetsPath := filepath.Join(rootDir, "data", "presets.json")
	if _, err := os.Stat(presetsPath); os.IsNotExist(err) {
		t.Skip("data/presets.json not found from relative path, skipping parity test")
	}

	cat, err := LoadCatalog(presetsPath)
	if err != nil {
		t.Fatalf("LoadCatalog failed: %v", err)
	}

	// 1. Check duplicate IWADs in mappacks
	baseIWADs := map[string]bool{
		"DOOM.WAD": true, "DOOM2.WAD": true, "PLUTONIA.WAD": true,
		"TNT.WAD": true, "HERETIC.WAD": true, "HEXEN.WAD": true,
	}
	for _, p := range cat.Presets {
		iwadUpper := strings.ToUpper(p.IWAD)
		for _, m := range p.Mappacks {
			mUpper := strings.ToUpper(m)
			if mUpper == iwadUpper || baseIWADs[mUpper] {
				t.Errorf("Preset '%s' includes base IWAD '%s' in mappacks list", p.Name, m)
			}
		}
	}

	// 2. Parity with README.md table
	readmeFile := filepath.Join(rootDir, "README.md")
	if data, err := os.ReadFile(readmeFile); err == nil {
		expectedTable := GenerateReadmeTable(cat)
		if !strings.Contains(string(data), expectedTable) {
			t.Errorf("%s presets table is out of sync with data/presets.json. Run 'doom presets build'", readmeFile)
		}
	}
}

func TestSyncReadme(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup data/presets.json
	dataDir := filepath.Join(tmpDir, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(data) error = %v", err)
	}

	presetContent := `{
		"presets": [
			{
				"name": "Test Megawad",
				"engine": "dsda-doom",
				"iwad": "DOOM2.WAD",
				"description": "A test megawad"
			}
		]
	}`
	if err := os.WriteFile(filepath.Join(dataDir, "presets.json"), []byte(presetContent), 0o644); err != nil {
		t.Fatalf("WriteFile(presets.json) error = %v", err)
	}

	readmeContent := `# Title

## Preconfigured Presets

Some intro text

| Old Table |
| :--- |
| Old Content |

---
`
	readmePath := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(readmePath, []byte(readmeContent), 0o644); err != nil {
		t.Fatalf("WriteFile(README.md) error = %v", err)
	}

	if err := SyncReadme(tmpDir); err != nil {
		t.Fatalf("SyncReadme() error = %v", err)
	}

	updated, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("ReadFile(README.md) error = %v", err)
	}

	if !strings.Contains(string(updated), "Test Megawad") {
		t.Errorf("expected updated README to contain 'Test Megawad', got:\n%s", string(updated))
	}
}

func TestResolveReadme(t *testing.T) {
	tmpDir := t.TempDir()

	txtPath := filepath.Join(tmpDir, "aaliens_v1_2.txt")
	if err := os.WriteFile(txtPath, []byte("Test Readme"), 0644); err != nil {
		t.Fatalf("failed writing test txt: %v", err)
	}

	p := Preset{
		Name:     "Ancient Aliens",
		Engine:   "uzdoom",
		IWAD:     "doom2.wad",
		Mappacks: []string{"aaliens_v1_2.wad"},
	}

	resolved, ok := ResolveReadme(tmpDir, p)
	if !ok || filepath.Base(resolved) != "aaliens_v1_2.txt" {
		t.Fatalf("expected to resolve aaliens_v1_2.txt, got %s, ok=%v", resolved, ok)
	}
}

func TestPresetMetadata(t *testing.T) {
	cat, err := LoadCatalog("")
	if err != nil {
		t.Fatalf("LoadCatalog failed: %v", err)
	}

	for _, p := range cat.Presets {
		if strings.TrimSpace(p.Author) == "" {
			t.Errorf("Preset %q missing author", p.Name)
		}
		if strings.TrimSpace(p.ReleaseDate) == "" {
			t.Errorf("Preset %q missing release_date", p.Name)
		}
	}
}

func TestDecodeText_And_ReadReadme(t *testing.T) {
	// 1. CRLF normalization
	crlf := []byte("Line 1\r\nLine 2\r\n")
	if got := DecodeText(crlf); got != "Line 1\nLine 2\n" {
		t.Errorf("expected normalized LF, got %q", got)
	}

	// 2. CP437 DOS block/shade art (like Alien Vendetta AV.TXT)
	// In CP437: 0xDB = █, 0xB0 = ░, 0xB1 = ▒, 0xB2 = ▓
	// If mistakenly decoded as UTF-8, 0xDB 0xB0 becomes the Arabic digit '۰'
	cp437Bytes := []byte{0xdb, 0xb0, 0xdb, 0xb1, 0xdb, 0xb2, 0xb0, 0xb1, 0xb2}
	decodedCP437 := DecodeText(cp437Bytes)
	if strings.Contains(decodedCP437, "۰") {
		t.Errorf("CP437 text mistakenly decoded as Arabic numeral: %q", decodedCP437)
	}
	expectedCP437 := "█░█▒█▓░▒▓"
	if decodedCP437 != expectedCP437 {
		t.Errorf("expected CP437 %q, got %q", expectedCP437, decodedCP437)
	}

	// 3. Windows-1252 smart quotes and dashes
	win1252Bytes := []byte("Dragonfly\x92s map \x96 exciting")
	decodedWin := DecodeText(win1252Bytes)
	expectedWin := "Dragonfly’s map – exciting"
	if decodedWin != expectedWin {
		t.Errorf("expected Windows-1252 %q, got %q", expectedWin, decodedWin)
	}

	// 4. ReadReadme from disk
	tmpDir := t.TempDir()
	txtPath := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(txtPath, cp437Bytes, 0644); err != nil {
		t.Fatal(err)
	}
	content, err := ReadReadme(txtPath)
	if err != nil {
		t.Fatalf("ReadReadme failed: %v", err)
	}
	if content != expectedCP437 {
		t.Errorf("expected ReadReadme %q, got %q", expectedCP437, content)
	}

	// 5. Multiline CP437 ASCII art with CRLF
	multilineCP437 := []byte(
		"Header\r\n" +
			"\xb0\xb1\xb2 \xdb\xdb\xdb \xb2\xb1\xb0\r\n" +
			"Footer\r\n",
	)
	decodedMulti := DecodeText(multilineCP437)
	expectedMulti := "Header\n░▒▓ ███ ▓▒░\nFooter\n"
	if decodedMulti != expectedMulti {
		t.Errorf("expected multiline CP437 %q, got %q", expectedMulti, decodedMulti)
	}
}

func TestEffectiveArgsStyle(t *testing.T) {
	tests := []struct {
		name     string
		cfg      EngineConfig
		expected string
	}{
		{
			name:     "explicit boom",
			cfg:      EngineConfig{Name: "custom", ArgsStyle: "boom"},
			expected: "boom",
		},
		{
			name:     "explicit zdoom",
			cfg:      EngineConfig{Name: "custom", ArgsStyle: "zdoom"},
			expected: "zdoom",
		},
		{
			name:     "inferred dsda",
			cfg:      EngineConfig{Name: "dsda-doom"},
			expected: "boom",
		},
		{
			name:     "inferred woof",
			cfg:      EngineConfig{Name: "woof"},
			expected: "boom",
		},
		{
			name:     "inferred crispy",
			cfg:      EngineConfig{Name: "crispy-doom"},
			expected: "boom",
		},
		{
			name:     "inferred chocolate",
			cfg:      EngineConfig{Name: "chocolate-doom"},
			expected: "boom",
		},
		{
			name:     "inferred gzdoom",
			cfg:      EngineConfig{Name: "gzdoom"},
			expected: "zdoom",
		},
		{
			name:     "inferred by family PrBoom",
			cfg:      EngineConfig{Name: "myport", Family: "PrBoom"},
			expected: "boom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.EffectiveArgsStyle(); got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestCatalog_Clone(t *testing.T) {
	cat, err := LoadCatalog("")
	if err != nil {
		t.Fatalf("LoadCatalog failed: %v", err)
	}

	cloned := cat.Clone()
	if cloned == nil {
		t.Fatal("expected non-nil clone")
	}
	if len(cloned.Presets) != len(cat.Presets) {
		t.Fatalf("expected %d presets, got %d", len(cat.Presets), len(cloned.Presets))
	}

	// Modifying clone does not mutate original
	cloned.Presets[0].Name = "Mutated Name"
	if cat.Presets[0].Name == "Mutated Name" {
		t.Error("clone mutation leaked into original catalog")
	}
}

func TestCatalog_MergeAndLayer(t *testing.T) {
	base, err := LoadCatalog("")
	if err != nil {
		t.Fatalf("LoadCatalog failed: %v", err)
	}

	cat := base.Clone()

	// 1. Merge custom engine
	customEngines := map[string]EngineConfig{
		"woof": {
			Name:        "woof",
			Binary:      "/usr/local/bin/woof",
			ArgsStyle:   "boom",
			Description: "Woof MBF21 port",
		},
	}
	cat.MergeEngines(customEngines)
	if _, ok := cat.Engines["woof"]; !ok {
		t.Error("expected woof engine to be merged")
	}

	// 2. Merge custom preset
	customPresets := []Preset{
		{
			Name:        "KDiZD",
			Engine:      "uzdoom",
			IWAD:        "DOOM.WAD",
			Mappacks:    []string{"kdizd_12.pk3"},
			Description: "Knee-Deep in ZDoom",
		},
	}
	cat.MergePresets(customPresets)
	p := cat.Find("KDiZD")
	if p == nil {
		t.Fatal("expected KDiZD preset to be found")
	}
	if !p.Custom {
		t.Error("expected custom flag to be true for added preset")
	}

	// 3. Apply launch options overrides to existing preset
	launchOptions := map[string]WadLaunchOptions{
		"Alien Vendetta": {
			Engine:         "woof",
			AdditionalArgs: "-skill 4",
			ExtraFiles:     []string{"av_custom_music.wad"},
		},
	}
	cat.ApplyLaunchOptions(launchOptions)

	av := cat.Find("Alien Vendetta")
	if av == nil {
		t.Fatal("expected Alien Vendetta to be found")
	}
	if av.Engine != "woof" {
		t.Errorf("expected engine to be overridden to woof, got %s", av.Engine)
	}
	if !strings.Contains(av.AdditionalArgs, "-skill 4") {
		t.Errorf("expected AdditionalArgs to contain '-skill 4', got %s", av.AdditionalArgs)
	}
	foundMusic := false
	for _, m := range av.Mappacks {
		if m == "av_custom_music.wad" {
			foundMusic = true
			break
		}
	}
	if !foundMusic {
		t.Error("expected av_custom_music.wad to be appended to mappacks")
	}
}

func TestLoadLayeredCatalog(t *testing.T) {
	tmpDir := t.TempDir()
	userPresetsFile := filepath.Join(tmpDir, "user_presets.json")

	userJSON := `{
		"engines": {
			"crispy-doom": {
				"name": "crispy-doom",
				"args_style": "boom"
			}
		},
		"presets": [
			{
				"name": "DropInWad",
				"engine": "crispy-doom",
				"iwad": "DOOM2.WAD",
				"mappacks": ["dropin.wad"]
			}
		]
	}`
	if err := os.WriteFile(userPresetsFile, []byte(userJSON), 0644); err != nil {
		t.Fatal(err)
	}

	customEngines := map[string]EngineConfig{
		"woof": {
			Name:      "woof",
			ArgsStyle: "boom",
		},
	}
	customPresets := []Preset{
		{
			Name:     "ConfigWad",
			Engine:   "woof",
			IWAD:     "DOOM2.WAD",
			Mappacks: []string{"cfg.wad"},
		},
	}
	launchOptions := map[string]WadLaunchOptions{
		"Sunlust": {
			AdditionalArgs: "-skill 4",
		},
	}

	cat, err := LoadLayeredCatalog("", userPresetsFile, customEngines, customPresets, launchOptions)
	if err != nil {
		t.Fatalf("LoadLayeredCatalog failed: %v", err)
	}

	// Curated defaults preserved (32 base + 1 dropin + 1 config = 34 total)
	if len(cat.Presets) != 34 {
		t.Errorf("expected 34 presets, got %d", len(cat.Presets))
	}

	// Verify engines present
	if _, ok := cat.Engines["crispy-doom"]; !ok {
		t.Error("expected crispy-doom engine in catalog")
	}
	if _, ok := cat.Engines["woof"]; !ok {
		t.Error("expected woof engine in catalog")
	}

	// Verify launch options applied
	sl := cat.Find("Sunlust")
	if sl == nil || sl.AdditionalArgs != "-skill 4" {
		t.Errorf("expected Sunlust AdditionalArgs to be '-skill 4', got %v", sl)
	}
}
