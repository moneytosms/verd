package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/moneytosms/verd/internal/session"
)

func TestSaveReportNeverOverwrites(t *testing.T) {
	dir := t.TempDir()
	x := session.Session{ContestID: 500, Mode: session.Virtual, Start: time.Date(2026, 10, 22, 15, 0, 0, 0, time.Local).Unix()}
	first, err := saveReport(dir, x, "one")
	if err != nil {
		t.Fatal(err)
	}
	second, err := saveReport(dir, x, "two") // same mode, same minute
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(first) != "virtual-2026-10-22-1500.md" || filepath.Base(second) != "virtual-2026-10-22-1500-2.md" {
		t.Fatalf("names: %s, %s", first, second)
	}
	if b, _ := os.ReadFile(first); string(b) != "one" {
		t.Fatalf("first report overwritten: %q", b)
	}
}
