package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "none.toml"))
	if err != nil || c.DefaultLang != "cpp" || c.Handle != "" {
		t.Fatalf("missing file should give defaults, got %+v %v", c, err)
	}
	p := filepath.Join(t.TempDir(), "c.toml")
	os.WriteFile(p, []byte(`handle = "tourist"`+"\n"+`time_multiplier = 2.0`), 0o644)
	c, err = Load(p)
	if err != nil || c.Handle != "tourist" || c.TimeMultiplier != 2 || c.DefaultLang != "cpp" {
		t.Fatalf("got %+v %v", c, err)
	}
}
