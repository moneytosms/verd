package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moneytosms/verd/internal/creds"
	"github.com/zalando/go-keyring"
)

func TestLoginReadsTwoLinesAndLogoutDeletes(t *testing.T) {
	keyring.MockInit()
	store := creds.Store{File: filepath.Join(t.TempDir(), "c.json")}
	in, w, _ := os.Pipe()
	w.WriteString("a=1; b=2\nMozilla/5.0 Chrome\n")
	w.Close()
	var out bytes.Buffer
	if err := loginCmd(&out, in, store); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "a=1") {
		t.Fatalf("cookie must never be echoed: %s", out.String())
	}
	c, err := store.Load()
	if err != nil || c.Cookie != "a=1; b=2" || c.UserAgent != "Mozilla/5.0 Chrome" {
		t.Fatalf("%+v %v", c, err)
	}
	if err := logoutCmd(&out, store); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); err == nil {
		t.Fatal("logout must delete")
	}
}

func TestLoginRejectsMissingUserAgent(t *testing.T) {
	keyring.MockInit()
	in, w, _ := os.Pipe()
	w.WriteString("a=1\n")
	w.Close()
	if err := loginCmd(&bytes.Buffer{}, in, creds.Store{File: filepath.Join(t.TempDir(), "c.json")}); exitCode(err) != 2 {
		t.Fatalf("want exit 2, got %v", err)
	}
}
