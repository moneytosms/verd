package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The whole site must build, and every internal link and anchor must resolve.
func TestSiteBuildsWithoutBrokenLinks(t *testing.T) {
	wd, _ := os.Getwd()
	if err := os.Chdir("../.."); err != nil { // build reads site/ and docs/ relative to the repo root
		t.Fatal(err)
	}
	defer os.Chdir(wd)
	out := t.TempDir()
	if err := build(out); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"index.html", "install.html", "usage.html", "guide/index.html", "guide/start.html", "guide/troubleshooting.html", "docs/index.html", "docs/contests.html", "docs/editorials.html"} {
		if _, err := os.Stat(filepath.Join(out, f)); err != nil {
			t.Errorf("missing page %s", f)
		}
	}

	attr := regexp.MustCompile(`(?:href|src)="([^"]*)"`)
	idRe := regexp.MustCompile(`id="([^"]+)"`)
	ids := map[string]map[string]bool{}
	pages := map[string]string{}
	filepath.WalkDir(out, func(p string, d fs.DirEntry, err error) error {
		if err == nil && strings.HasSuffix(p, ".html") {
			b, _ := os.ReadFile(p)
			pages[p] = string(b)
			ids[p] = map[string]bool{}
			for _, m := range idRe.FindAllStringSubmatch(string(b), -1) {
				ids[p][m[1]] = true
			}
		}
		return nil
	})
	for page, body := range pages {
		for _, m := range attr.FindAllStringSubmatch(body, -1) {
			link := m[1]
			if link == "" || strings.Contains(link, "://") || strings.HasPrefix(link, "mailto:") || strings.HasPrefix(link, "data:") || strings.HasPrefix(link, "javascript:") {
				continue
			}
			path, frag, _ := strings.Cut(link, "#")
			target := page
			if path != "" {
				target = filepath.Join(filepath.Dir(page), path)
			}
			if st, err := os.Stat(target); err != nil || st.IsDir() {
				t.Errorf("%s: broken link %q", strings.TrimPrefix(page, out), link)
				continue
			}
			if frag != "" && strings.HasSuffix(target, ".html") && !ids[target][frag] {
				t.Errorf("%s: link %q has no anchor #%s", strings.TrimPrefix(page, out), link, frag)
			}
		}
	}
}

func TestFixLinkMapsGuideAndDocs(t *testing.T) {
	for _, c := range []struct{ in, section, want string }{
		{"../config.md", "guide", "../docs/config.html"},
		{"../install.md", "guide", "../install.html"},
		{"./stress.md", "guide", "stress.html"},
		{"./guide/start.md", "", "../guide/start.html"},
		{"./keys.md", "", "keys.html"},
		{"https://example.com/x.md", "guide", "https://example.com/x.md"},
	} {
		if got := fixLink(c.in, c.section); got != c.want {
			t.Errorf("fixLink(%q, %q) = %q, want %q", c.in, c.section, got, c.want)
		}
	}
}
