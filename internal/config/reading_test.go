package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadingSplitPaletteNightValidation(t *testing.T) {
	d := Default()
	if d.ReadingSplit != "full" || d.ReadingPalette != "app" || d.ReadingNightFrom != 20 {
		t.Fatalf("defaults: split=%s palette=%s night_from=%d", d.ReadingSplit, d.ReadingPalette, d.ReadingNightFrom)
	}
	p := filepath.Join(t.TempDir(), "c.toml")
	for _, ok := range []string{`reading_split = "cap"`, `reading_palette = "sepia"`, `reading_palette = "auto"`, `reading_night_from = 0`, `reading_night_from = 23`} {
		os.WriteFile(p, []byte(ok), 0o644)
		if _, err := Load(p); err != nil {
			t.Errorf("%s should load: %v", ok, err)
		}
	}
	for _, bad := range []string{`reading_split = "wide"`, `reading_palette = "dark"`, `reading_night_from = 24`, `reading_night_from = -1`} {
		os.WriteFile(p, []byte(bad), 0o644)
		if _, err := Load(p); err == nil {
			t.Errorf("%s should fail", bad)
		}
	}
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := Set(path, "reading_palette", "neon"); err == nil {
		t.Error("reading_palette=neon should fail")
	}
	if err := Set(path, "reading_night_from", "24"); err == nil {
		t.Error("reading_night_from=24 should fail")
	}
	if err := Set(path, "reading_palette", "contrast"); err != nil {
		t.Errorf("reading_palette=contrast: %v", err)
	}
}
