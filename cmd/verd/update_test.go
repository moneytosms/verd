package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestSkippedReleasesBetweenCurAndLatest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/releases" {
			w.Write([]byte(`[{"tag_name":"v0.4.0","body":"d"},{"tag_name":"v0.3.0","body":"c"},{"tag_name":"v0.2.0","body":"b"},{"tag_name":"v0.1.0","body":"a"}]`))
		}
	}))
	defer srv.Close()
	oldAPI := apiBase
	apiBase = srv.URL
	defer func() { apiBase = oldAPI }()
	rels, err := skippedReleases("0.1.0", "0.3.0")
	if err != nil || len(rels) != 2 || rels[0].Tag != "v0.3.0" || rels[1].Tag != "v0.2.0" {
		t.Fatalf("%+v %v", rels, err)
	}
}

type flakyTransport struct{ fails int }

func (f *flakyTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if f.fails > 0 {
		f.fails--
		return nil, errors.New("TLS handshake timeout")
	}
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("ok")), Request: r}, nil
}

func TestFetchRetriesNetworkErrorsOnly(t *testing.T) {
	old := retryDelay
	retryDelay = 0
	defer func() { retryDelay = old }()
	c := &http.Client{Transport: &flakyTransport{fails: 2}}
	if body, err := fetch(c, "http://x/checksums.txt"); err != nil || string(body) != "ok" {
		t.Fatalf("%q %v", body, err)
	}
	c = &http.Client{Transport: &flakyTransport{fails: 3}}
	if _, err := fetch(c, "http://x/checksums.txt"); err == nil {
		t.Fatal("want error after 3 failed attempts")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()
	tr := &countingTransport{}
	if _, err := fetch(&http.Client{Transport: tr}, srv.URL); err == nil || tr.calls != 1 {
		t.Fatalf("404 retried: calls %d, %v", tr.calls, err)
	}
}

type countingTransport struct{ calls int }

func (c *countingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	c.calls++
	return http.DefaultTransport.RoundTrip(r)
}
