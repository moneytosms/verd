package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moneytosms/verd/internal/config"
	"github.com/moneytosms/verd/internal/creds"
)

func TestSetupWritesAnswersAndKeepsDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	in, w, _ := os.Pipe()
	// handle, workspace (keep), lang (keep), theme (bad, then good), split (keep), submit mode (keep)
	w.WriteString("tourist\n\n\nnope\nnord\n\n\n")
	w.Close()
	var out bytes.Buffer
	if err := setupCmd(&out, in, path, creds.Store{File: filepath.Join(dir, "c.json")}); err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	cfg, err := config.Load(path)
	if err != nil || cfg.Handle != "tourist" || cfg.Theme != "nord" || cfg.SubmitMode != "browser" {
		t.Fatalf("cfg=%+v err=%v\n%s", cfg, err, out.String())
	}
	b, _ := os.ReadFile(path)
	if !strings.Contains(string(b), "#") {
		t.Fatal("setup must keep the file's comments")
	}
	if !strings.Contains(out.String(), "is not one of") {
		t.Fatalf("a bad theme is rejected and re-asked:\n%s", out.String())
	}
}
