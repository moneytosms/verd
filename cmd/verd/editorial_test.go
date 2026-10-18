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

func TestEditorialCmd(t *testing.T) {
	res := &editorial.Result{
		Status: editorial.OK, Title: "Editorial", URL: "https://codeforces.com/blog/entry/1",
		Blocks: []editorial.Block{
			{HTML: "<p>PLAINIDEA</p>"},
			{Title: "Hint 1", HTML: "<p>HINTONE</p>", Spoiler: true},
			{Title: "Solution", HTML: "<p>SOLVED</p>", Spoiler: true},
		},
		Links: []editorial.Material{{Label: "Video Tutorial", URL: "https://codeforces.com/blog/entry/9"}},
	}
	var asked cf.Problem
	load := func(p cf.Problem) (*editorial.Result, error) { asked = p; return res, nil }
	cfg := config.Default()
	run := func(args ...string) (string, error) {
		var b bytes.Buffer
		err := editorialCmd(&b, cfg, load, args)
		return b.String(), err
	}

	out, err := run("1900/A")
	if err != nil || asked.ContestID != 1900 || asked.Index != "A" {
		t.Fatalf("%v %+v", err, asked)
	}
	if !strings.Contains(out, "PLAINIDEA") || strings.Contains(out, "HINTONE") || strings.Contains(out, "SOLVED") ||
		!strings.Contains(out, "Hint 1 (hidden: --reveal 1)") || !strings.Contains(out, "Solution (hidden: --reveal 2)") || !strings.Contains(out, "Video Tutorial") {
		t.Fatalf("spoilers withheld by default:\n%s", out)
	}
	out, _ = run("--reveal", "1", "1900b")
	if asked.Index != "B" || !strings.Contains(out, "HINTONE") || strings.Contains(out, "SOLVED") {
		t.Fatalf("reveal 1:\n%s", out)
	}
	out, _ = run("--all", "1900A")
	if !strings.Contains(out, "HINTONE") || !strings.Contains(out, "SOLVED") {
		t.Fatalf("--all:\n%s", out)
	}

	res = &editorial.Result{Status: editorial.None, Links: []editorial.Material{{Label: "Discussion stream", URL: "https://codeforces.com/blog/entry/2"}}}
	out, err = run("1900A")
	var ee *exitError
	if !errors.As(err, &ee) || ee.code != 1 || !strings.Contains(out, "no editorial is published for 1900A") || !strings.Contains(out, "Discussion stream") {
		t.Fatalf("none: %v\n%s", err, out)
	}
	res = &editorial.Result{Status: editorial.NotEnglish}
	if out, _ = run("1900A"); !strings.Contains(out, "only a Russian editorial") {
		t.Fatalf("%s", out)
	}
	if _, err := run(); err == nil {
		t.Fatal("usage error")
	}
	if _, err := run("/etc/passwd"); err == nil {
		t.Fatal("a path outside the workspace is refused")
	}
}
