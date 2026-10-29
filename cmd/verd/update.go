package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
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

// apiBase serves release bodies for verd update; the startup check uses the /latest redirect instead (no API rate limit).
var apiBase = "https://api.github.com/repos/moneytosms/verd"

// updateCmd replaces the running binary with the latest release (checksum verified).
// With check it only reports.
func updateCmd(out io.Writer, check bool) error {
	cur := strings.TrimPrefix(version, "v")
	if version == "dev" {
		return &exitError{2, "this is a source build (version dev): update with `go install github.com/moneytosms/verd/cmd/verd@latest`"}
	}
	tag, err := latestTag()
	if err != nil {
		return &exitError{2, "update: " + err.Error()}
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
	bin, err := downloadVerified(tag, latest)
	if err != nil {
		return err
	}
	if err := installBinary(exe, bin); err != nil {
		return err
	}
	fmt.Fprintf(out, "updated verd %s -> %s (restart verd to use it)\n", cur, latest)
	printSkippedNotes(out, cur, latest)
	return nil
}

// newerRelease returns the latest release tag when it is newer than this build, else "".
// Errors are dropped: a failed check is not worth a toast.
func newerRelease() string {
	if version == "dev" {
		return ""
	}
	tag, err := latestTag()
	if err != nil || !newer(strings.TrimPrefix(tag, "v"), strings.TrimPrefix(version, "v")) {
		return ""
	}
	return tag
}

type release struct {
	Tag  string `json:"tag_name"`
	Body string `json:"body"`
}

// printSkippedNotes prints the notes of every release after cur up to latest, newest first.
func printSkippedNotes(out io.Writer, cur, latest string) {
	rels, err := skippedReleases(cur, latest)
	if err != nil {
		fmt.Fprintf(out, "\nrelease notes unavailable (%v): see %s\n", err, releaseBase)
		return
	}
	for _, r := range rels {
		fmt.Fprintf(out, "\n%s\n%s\n", r.Tag, strings.TrimSpace(r.Body))
	}
}

// skippedReleases returns the releases with cur < version <= latest, newest first.
func skippedReleases(cur, latest string) ([]release, error) {
	body, err := fetch(&http.Client{Timeout: 10 * time.Second}, apiBase+"/releases?per_page=100")
	if err != nil {
		return nil, err
	}
	var all []release
	if err := json.Unmarshal(body, &all); err != nil {
		return nil, err
	}
	var out []release
	for _, r := range all {
		v := strings.TrimPrefix(r.Tag, "v")
		if newer(v, cur) && !newer(v, latest) {
			out = append(out, r)
		}
	}
	return out, nil
}

// latestTag reads the newest release tag from the /latest redirect.
func latestTag() (string, error) {
	hc := &http.Client{Timeout: 5 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, _ := http.NewRequest("GET", releaseBase+"/latest", nil)
	req.Header.Set("Accept", "*/*") // GitHub's edge caches this redirect per Accept header; a bare request can see a stale tag
	resp, err := hc.Do(req)
	if err != nil {
		return "", err
	}
	resp.Body.Close()
	tag := filepath.Base(resp.Header.Get("Location"))
	if !strings.HasPrefix(tag, "v") {
		return "", errors.New("no release found")
	}
	return tag, nil
}

// downloadVerified fetches this platform's archive for tag and returns the verd binary in it,
// after checking the archive against checksums.txt.
func downloadVerified(tag, latest string) ([]byte, error) {
	name := fmt.Sprintf("verd_%s_%s_%s.tar.gz", latest, runtime.GOOS, runtime.GOARCH)
	dl := &http.Client{Timeout: 5 * time.Minute}
	sums, err := fetch(dl, releaseBase+"/download/"+tag+"/checksums.txt")
	if err != nil {
		return nil, &exitError{2, "update: " + err.Error()}
	}
	want := ""
	for _, l := range strings.Split(string(sums), "\n") {
		if f := strings.Fields(l); len(f) == 2 && f[1] == name {
			want = f[0]
		}
	}
	archive, err := fetch(dl, releaseBase+"/download/"+tag+"/"+name)
	if err != nil {
		return nil, &exitError{2, "update: " + err.Error()}
	}
	if sum := sha256.Sum256(archive); want == "" || hex.EncodeToString(sum[:]) != want {
		return nil, &exitError{2, "update: checksum mismatch for " + name}
	}
	bin, err := extractVerd(archive)
	if err != nil {
		return nil, &exitError{2, "update: " + err.Error()}
	}
	return bin, nil
}

// installBinary swaps exe for bin: write a temp file beside it, then rename over it.
func installBinary(exe string, bin []byte) error {
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
	return os.Rename(tmp.Name(), exe) // atomic; a running verd keeps its old inode
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
