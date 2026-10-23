package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/config"
	"github.com/moneytosms/verd/internal/editorial"
)

// editorialRig drives editorialCmd with a fake loader.
type editorialRig struct {
	res   *editorial.Result
	asked cf.Problem
	cfg   config.Config
}

func (r *editorialRig) run(args ...string) (string, error) {
	var b bytes.Buffer
	load := func(p cf.Problem) (*editorial.Result, error) { r.asked = p; return r.res, nil }
	err := editorialCmd(&b, r.cfg, load, args)
	return b.String(), err
}

func TestEditorialCmd(t *testing.T) {
	r := &editorialRig{cfg: config.Default(), res: &editorial.Result{
		Status: editorial.OK, Title: "Editorial", URL: "https://codeforces.com/blog/entry/1",
		Blocks: []editorial.Block{
			{HTML: "<p>PLAINIDEA</p>"},
			{Title: "Hint 1", HTML: "<p>HINTONE</p>", Spoiler: true},
			{Title: "Solution", HTML: "<p>SOLVED</p>", Spoiler: true},
		},
		Links: []editorial.Material{{Label: "Video Tutorial", URL: "https://codeforces.com/blog/entry/9"}},
	}}
	t.Run("spoilers withheld by default", func(t *testing.T) { checkSpoilersWithheld(t, r) })
	t.Run("reveal one", func(t *testing.T) { checkRevealOne(t, r) })
	t.Run("all", func(t *testing.T) { checkRevealAll(t, r) })
	t.Run("no editorial", func(t *testing.T) { checkNoEditorial(t, r) })
	t.Run("russian only", func(t *testing.T) { checkRussianOnly(t, r) })
	t.Run("bad args", func(t *testing.T) { checkBadArgs(t, r) })
}

func checkSpoilersWithheld(t *testing.T, r *editorialRig) {
	out, err := r.run("1900/A")
	if err != nil || r.asked.ContestID != 1900 || r.asked.Index != "A" {
		t.Fatalf("%v %+v", err, r.asked)
	}
	for _, want := range []string{"PLAINIDEA", "Hint 1 (hidden: --reveal 1)", "Solution (hidden: --reveal 2)", "Video Tutorial"} {
		if !strings.Contains(out, want) {
			t.Fatalf("spoilers withheld by default, missing %q:\n%s", want, out)
		}
	}
	for _, hidden := range []string{"HINTONE", "SOLVED"} {
		if strings.Contains(out, hidden) {
			t.Fatalf("spoilers withheld by default, leaked %q:\n%s", hidden, out)
		}
	}
}

func checkRevealOne(t *testing.T, r *editorialRig) {
	out, _ := r.run("--reveal", "1", "1900b")
	if r.asked.Index != "B" || !strings.Contains(out, "HINTONE") || strings.Contains(out, "SOLVED") {
		t.Fatalf("reveal 1:\n%s", out)
	}
}

func checkRevealAll(t *testing.T, r *editorialRig) {
	out, _ := r.run("--all", "1900A")
	if !strings.Contains(out, "HINTONE") || !strings.Contains(out, "SOLVED") {
		t.Fatalf("--all:\n%s", out)
	}
}

func checkNoEditorial(t *testing.T, r *editorialRig) {
	r.res = &editorial.Result{Status: editorial.None, Links: []editorial.Material{{Label: "Discussion stream", URL: "https://codeforces.com/blog/entry/2"}}}
	out, err := r.run("1900A")
	var ee *exitError
	if !errors.As(err, &ee) || ee.code != 1 || !strings.Contains(out, "no editorial is published for 1900A") || !strings.Contains(out, "Discussion stream") {
		t.Fatalf("none: %v\n%s", err, out)
	}
}

func checkRussianOnly(t *testing.T, r *editorialRig) {
	r.res = &editorial.Result{Status: editorial.NotEnglish}
	if out, _ := r.run("1900A"); !strings.Contains(out, "only a Russian editorial") {
		t.Fatalf("%s", out)
	}
}

func checkBadArgs(t *testing.T, r *editorialRig) {
	if _, err := r.run(); err == nil {
		t.Fatal("usage error")
	}
	if _, err := r.run("/etc/passwd"); err == nil {
		t.Fatal("a path outside the workspace is refused")
	}
}
