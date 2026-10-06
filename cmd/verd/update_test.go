package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewer(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{{"0.1.1", "0.1.0", true}, {"0.1.0", "0.1.0", false}, {"0.2.0", "0.10.0", false}, {"1.0.0", "0.9.9", true}} {
		if newer(c.a, c.b) != c.want {
			t.Errorf("newer(%s,%s)", c.a, c.b)
		}
	}
}

func TestExtractAndChecksum(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	body := []byte("#!/bin/sh\necho new\n")
	tw.WriteHeader(&tar.Header{Name: "verd", Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg})
	tw.Write(body)
	tw.Close()
	gz.Close()
	got, err := extractVerd(buf.Bytes())
	if err != nil || !bytes.Equal(got, body) {
		t.Fatalf("%q %v", got, err)
	}
}

func TestUpdateCheckReportsNewerRelease(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/latest" {
			http.Redirect(w, r, "/tag/v9.9.9", http.StatusFound)
		}
	}))
	defer srv.Close()
	oldBase, oldVer := releaseBase, version
	releaseBase, version = srv.URL, "0.1.0"
	defer func() { releaseBase, version = oldBase, oldVer }()
	var out bytes.Buffer
	if err := updateCmd(&out, true); err != nil || out.String() != "verd 9.9.9 is available (you have 0.1.0): run `verd update`\n" {
		t.Fatalf("%q %v", out.String(), err)
	}
}
