package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitAndConfig(t *testing.T) {
	cfgHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgHome)
	path := filepath.Join(cfgHome, "verd", "config.toml")

	var out bytes.Buffer
	if err := run([]string{"init"}, &out); err != nil || !strings.Contains(out.String(), path) {
		t.Fatalf("init: %v %q", err, out.String())
	}
	if err := run([]string{"init"}, &out); err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("second init must refuse and hint --force, got %v", err)
	}
	os.WriteFile(path, []byte(`handle = "tourist"`), 0o644)
	if err := run([]string{"init", "--force"}, &out); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(path, []byte(`handle = "tourist"`+"\n"+`time_multiplier = 3.0`), 0o644)
	out.Reset()
	if err := run([]string{"config"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{path, "handle = 'tourist'", "time_multiplier = 3.0", "default_lang = 'cpp'"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("config output missing %q:\n%s", want, out.String())
		}
	}
	if err := run([]string{"bogus"}, &out); err == nil {
		t.Fatal("unknown command should error")
	}
}
