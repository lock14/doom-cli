package steam

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/lock14/doom-cli/internal/wadfmt"
)

func buildTestPWAD(t *testing.T, lumps []wadfmt.Lump) []byte {
	t.Helper()
	b, err := wadfmt.BuildBytes("PWAD", lumps)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestSynthesizeIDKFAWADAndDiscoverFromExtras(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	mockSteam := filepath.Join(tmpDir, "steam_root")
	baseDir := filepath.Join(mockSteam, "steamapps", "common", "Ultimate Doom", "base")
	rereleaseDir := filepath.Join(mockSteam, "steamapps", "common", "Ultimate Doom", "rerelease")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(rereleaseDir, 0o755); err != nil {
		t.Fatal(err)
	}

	legacyExtras := buildTestPWAD(t, []wadfmt.Lump{
		{Name: "DSSECRET", Data: []byte("secret-sfx")},
	})
	if err := os.WriteFile(filepath.Join(baseDir, "extras.wad"), legacyExtras, 0o644); err != nil {
		t.Fatal(err)
	}

	remasterExtras := buildTestPWAD(t, []wadfmt.Lump{
		{Name: "TRAKINFO", Data: []byte(`{"tracks":[]}`)},
		{Name: "O_E1M1", Data: []byte("OggS-sc55-e1m1")},
		{Name: "H_E1M1", Data: []byte("OggS-idkfa-e1m1")},
		{Name: "H_E2M3", Data: []byte("OggS-idkfa-e2m3")},
		{Name: "H_RUNNIN", Data: []byte("OggS-idkfa-runnin")},
	})
	if err := os.WriteFile(filepath.Join(rereleaseDir, "extras.wad"), remasterExtras, 0o644); err != nil {
		t.Fatal(err)
	}

	destWads := filepath.Join(tmpDir, "target_wads")
	var out bytes.Buffer
	count, err := DiscoverAndExtract(nil, []string{mockSteam}, destWads, false, &out)
	if err != nil {
		t.Fatalf("DiscoverAndExtract failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 files (extras.wad + synthesized idkfa 2024.wad), got %d\nOutput:\n%s", count, out.String())
	}

	gotExtras, err := os.ReadFile(filepath.Join(destWads, "extras.wad"))
	if err != nil {
		t.Fatalf("reading extracted extras.wad: %v", err)
	}
	if !bytes.Equal(gotExtras, remasterExtras) {
		t.Errorf("expected rerelease/extras.wad to be preferred over base/extras.wad")
	}

	idkfaBytes, err := os.ReadFile(filepath.Join(destWads, "idkfa 2024.wad"))
	if err != nil {
		t.Fatalf("reading synthesized idkfa 2024.wad: %v", err)
	}

	hdr, entries, err := wadfmt.ReadDirectory(bytes.NewReader(idkfaBytes), int64(len(idkfaBytes)))
	if err != nil || hdr.Ident != "PWAD" {
		t.Fatalf("synthesized idkfa 2024.wad failed ReadDirectory: hdr=%+v err=%v", hdr, err)
	}

	// H_E1M1 -> D_E1M1; H_E2M3 -> D_E2M3 + D_INTER; H_RUNNIN -> D_RUNNIN + D_RUNNI2 (5 directory entries)
	wantTracks := []wadfmt.Lump{
		{Name: "D_E1M1", Data: []byte("OggS-idkfa-e1m1")},
		{Name: "D_E2M3", Data: []byte("OggS-idkfa-e2m3")},
		{Name: "D_INTER", Data: []byte("OggS-idkfa-e2m3")},
		{Name: "D_RUNNIN", Data: []byte("OggS-idkfa-runnin")},
		{Name: "D_RUNNI2", Data: []byte("OggS-idkfa-runnin")},
	}
	if len(entries) != len(wantTracks) {
		t.Fatalf("expected %d synthesized directory entries, got %d", len(wantTracks), len(entries))
	}

	for i, want := range wantTracks {
		if entries[i].Name != want.Name {
			t.Errorf("track %d name = %q, want %q", i, entries[i].Name, want.Name)
		}
		payload := idkfaBytes[entries[i].Offset : entries[i].Offset+entries[i].Size]
		if !bytes.Equal(payload, want.Data) {
			t.Errorf("track %d payload = %q, want %q", i, string(payload), string(want.Data))
		}
	}
}

func TestSynthesizeIDKFAWAD_DeduplicatesRepeatedTracks(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	dupExtrasPath := filepath.Join(tmpDir, "extras.wad")
	dupExtras := buildTestPWAD(t, []wadfmt.Lump{
		{Name: "H_E1M1", Data: []byte("OggS-first")},
		{Name: "H_E1M1", Data: []byte("OggS-duplicate")},
	})
	if err := os.WriteFile(dupExtrasPath, dupExtras, 0o644); err != nil {
		t.Fatal(err)
	}
	outIDKFA := filepath.Join(tmpDir, "idkfa 2024.wad")
	tracks, err := SynthesizeIDKFAWAD(dupExtrasPath, outIDKFA)
	if err != nil {
		t.Fatalf("SynthesizeIDKFAWAD: %v", err)
	}
	if tracks != 1 {
		t.Errorf("expected 1 deduplicated D_E1M1 track, got %d", tracks)
	}
}
