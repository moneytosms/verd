package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetKeysRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte("# mine\nhandle = \"x\"\n\n[lang.cpp]\next = \"cpp\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(SetKeys(p, "problem", "run_tests", []string{"ctrl+t"}))
	must(SetKeys(p, "problem", "submit", []string{"s", "ctrl+s"}))
	must(SetKeys(p, "problem", "run_tests", []string{"F5"})) // replaces in place
	c, err := Load(p)
	must(err)
	kb, err := c.KeyBindings()
	must(err)
	if got := kb["problem.run_tests"]; len(got) != 1 || got[0] != "F5" {
		t.Fatalf("run_tests: %v", got)
	}
	if got := kb["problem.submit"]; len(got) != 2 || got[1] != "ctrl+s" {
		t.Fatalf("submit: %v", got)
	}
	must(SetKeys(p, "problem", "run_tests", nil)) // reset removes it
	must(SetKeys(p, "problem", "nothing", nil))   // no-op
	b, _ := os.ReadFile(p)
	if strings.Contains(string(b), "run_tests") || !strings.Contains(string(b), "# mine") || !strings.Contains(string(b), "[lang.cpp]") {
		t.Fatalf("file:\n%s", b)
	}
	c, err = Load(p)
	must(err)
	if kb, _ = c.KeyBindings(); len(kb) != 1 {
		t.Fatalf("kb: %v", kb)
	}
}

func TestEmbedZoomSettable(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := Set(p, "embed_zoom", "true"); err != nil {
		t.Fatal(err)
	}
	c, err := Load(p)
	if err != nil || !c.EmbedZoom {
		t.Fatalf("%v %v", c.EmbedZoom, err)
	}
	if v, ok := c.Get("embed_zoom"); !ok || v != "true" {
		t.Fatalf("get: %q %v", v, ok)
	}
}
