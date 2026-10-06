package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// releaseBase is where releases live; tests point it at a local server.
var releaseBase = "https://github.com/moneytosms/verd/releases"

// updateCmd replaces the running binary with the latest release (checksum verified).
// With check it only reports.
func updateCmd(out io.Writer, check bool) error {
	cur := strings.TrimPrefix(version, "v")
	if version == "dev" {
		return &exitError{2, "this is a source build (version dev): update with `go install github.com/moneytosms/verd/cmd/verd@latest`"}
	}
	hc := &http.Client{Timeout: 5 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, _ := http.NewRequest("GET", releaseBase+"/latest", nil)
	req.Header.Set("Accept", "*/*") // GitHub's edge caches this redirect per Accept header; a bare request can see a stale tag
	resp, err := hc.Do(req)
	if err != nil {
		return &exitError{2, "update: " + err.Error()}
	}
	resp.Body.Close()
	tag := filepath.Base(resp.Header.Get("Location"))
	if !strings.HasPrefix(tag, "v") {
		return &exitError{2, "update: no release found"}
	}
	latest := strings.TrimPrefix(tag, "v")
	if !newer(latest, cur) {
		fmt.Fprintf(out, "verd %s is up to date\n", cur)
		return nil
	}
	if check {
		fmt.Fprintf(out, "verd %s is available (you have %s): run `verd update`\n", latest, cur)
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return err
	}
	name := fmt.Sprintf("verd_%s_%s_%s.tar.gz", latest, runtime.GOOS, runtime.GOARCH)
	dl := &http.Client{Timeout: 5 * time.Minute}
	sums, err := fetch(dl, releaseBase+"/download/"+tag+"/checksums.txt")
	if err != nil {
		return &exitError{2, "update: " + err.Error()}
	}
	want := ""
	for _, l := range strings.Split(string(sums), "\n") {
		if f := strings.Fields(l); len(f) == 2 && f[1] == name {
			want = f[0]
		}
	}
	archive, err := fetch(dl, releaseBase+"/download/"+tag+"/"+name)
	if err != nil {
		return &exitError{2, "update: " + err.Error()}
	}
	if sum := sha256.Sum256(archive); want == "" || hex.EncodeToString(sum[:]) != want {
		return &exitError{2, "update: checksum mismatch for " + name}
	}
	bin, err := extractVerd(archive)
	if err != nil {
		return &exitError{2, "update: " + err.Error()}
	}
	tmp, err := os.CreateTemp(filepath.Dir(exe), ".verd-update-*")
	if err != nil {
		return &exitError{2, fmt.Sprintf("update: cannot write next to %s (%v): re-run the installer with sudo or from a writable directory", exe, err)}
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(bin); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), exe); err != nil { // atomic; a running verd keeps its old inode
		return err
	}
	fmt.Fprintf(out, "updated verd %s -> %s (restart verd to use it)\n", cur, latest)
	return nil
}

func fetch(c *http.Client, url string) ([]byte, error) {
	resp, err := c.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func extractVerd(archive []byte) ([]byte, error) {
	gz, err := gzip.NewReader(strings.NewReader(string(archive)))
	if err != nil {
		return nil, err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err != nil {
			return nil, fmt.Errorf("no verd binary in the archive")
		}
		if filepath.Base(h.Name) == "verd" && h.Typeflag == tar.TypeReg {
			return io.ReadAll(tr)
		}
	}
}

// newer reports whether version a (x.y.z) is greater than b.
func newer(a, b string) bool {
	pa, pb := semver(a), semver(b)
	for i := range pa {
		if pa[i] != pb[i] {
			return pa[i] > pb[i]
		}
	}
	return false
}

func semver(s string) [3]int {
	var v [3]int
	for i, p := range strings.SplitN(strings.SplitN(s, "-", 2)[0], ".", 3) {
		v[i], _ = strconv.Atoi(p)
	}
	return v
}
