package creds

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestKeyringRoundTripAndDelete(t *testing.T) {
	keyring.MockInit()
	s := Store{File: filepath.Join(t.TempDir(), "creds.json")}
	if _, err := s.Load(); !errors.Is(err, ErrNone) {
		t.Fatalf("want ErrNone, got %v", err)
	}
	where, warn, err := s.Save(Creds{Cookie: "a=b", UserAgent: "UA"})
	if err != nil || warn != "" || where != "OS keyring" {
		t.Fatalf("%q %q %v", where, warn, err)
	}
	if _, err := os.Stat(s.File); err == nil {
		t.Fatal("no file when the keyring works")
	}
	if c, err := s.Load(); err != nil || c.Cookie != "a=b" || c.UserAgent != "UA" {
		t.Fatalf("%+v %v", c, err)
	}
	if err := s.Delete(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load(); !errors.Is(err, ErrNone) {
		t.Fatalf("after delete: %v", err)
	}
}

func TestFileFallbackIs0600WithWarning(t *testing.T) {
	keyring.MockInitWithError(errors.New("no secret service"))
	s := Store{File: filepath.Join(t.TempDir(), "sub", "creds.json")}
	os.MkdirAll(filepath.Dir(s.File), 0o755)
	os.WriteFile(s.File, []byte("old"), 0o644) // an existing loose file must be tightened
	where, warn, err := s.Save(Creds{Cookie: "a=b", UserAgent: "UA"})
	if err != nil || where != s.File || warn == "" {
		t.Fatalf("%q %q %v", where, warn, err)
	}
	if fi, _ := os.Stat(s.File); fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", fi.Mode().Perm())
	}
	if c, err := s.Load(); err != nil || c.Cookie != "a=b" {
		t.Fatalf("%+v %v", c, err)
	}
	keyring.MockInit() // keyring reachable again for the delete
	if err := s.Delete(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.File); err == nil {
		t.Fatal("Delete must remove the file")
	}
}
