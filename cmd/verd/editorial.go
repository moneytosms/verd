package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/term"

	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/config"
	"github.com/moneytosms/verd/internal/editorial"
	"github.com/moneytosms/verd/internal/scrape"
	"github.com/moneytosms/verd/internal/workspace"
)

// osc8 matches terminal hyperlink escapes, which only clutter piped output.
var osc8 = regexp.MustCompile("\x1b\\]8;[^\x07\x1b]*(?:\x07|\x1b\\\\)")

var problemRef = regexp.MustCompile(`^(\d+)/?([A-Za-z]\d?)$`)

// editorialProblem resolves "1900A", "1900/A" or a Solution path inside the Workspace.
func editorialProblem(cfg config.Config, arg string) (cf.Problem, error) {
	if g := problemRef.FindStringSubmatch(arg); g != nil {
		n, _ := strconv.Atoi(g[1])
		return cf.Problem{ContestID: n, Index: strings.ToUpper(g[2])}, nil
	}
	ref, err := workspace.New(cfg.Workspace).Resolve(arg, cfg.Lang)
	if err != nil {
		return cf.Problem{}, &exitError{2, err.Error()}
	}
	return cf.Problem{ContestID: ref.Contest, Index: ref.Index}, nil
}

// editorialCmd runs `verd editorial`: print a problem's editorial, its hints and solution
// withheld until asked for (--reveal N opens the first N, --all opens them all).
func editorialCmd(out io.Writer, cfg config.Config, load func(cf.Problem) (*editorial.Result, error), args []string) error {
	fs := flag.NewFlagSet("editorial", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	reveal := fs.Int("reveal", 0, "open the first N hints")
	all := fs.Bool("all", false, "open everything")
	if err := fs.Parse(args); err != nil || fs.NArg() != 1 {
		return &exitError{2, "usage: verd editorial [--reveal N | --all] <file|contest/index>"}
	}
	p, err := editorialProblem(cfg, fs.Arg(0))
	if err != nil {
		return err
	}
	res, err := load(p)
	if err != nil {
		return &exitError{2, fmt.Sprintf("editorial %s: %v", p.Code(), err)}
	}
	switch res.Status {
	case editorial.Unsupported:
		return &exitError{1, "only Codeforces problems have editorials"}
	case editorial.NotEnglish, editorial.None:
		return noEditorial(out, p, res)
	}
	return printEditorial(out, p, res, *reveal, *all)
}

// noEditorial says why there is no English editorial and lists any links, then exits 1.
func noEditorial(out io.Writer, p cf.Problem, res *editorial.Result) error {
	msg := "no editorial is published for " + p.Code()
	if res.Status == editorial.NotEnglish {
		msg = "only a Russian editorial exists for " + p.Code()
	}
	fmt.Fprintln(out, msg)
	for _, l := range res.Links {
		fmt.Fprintf(out, "%-22s %s\n", l.Label, l.URL)
	}
	return &exitError{1, ""}
}

// printEditorial prints the title and blocks. Spoiler blocks past reveal stay hidden unless all is set.
func printEditorial(out io.Writer, p cf.Problem, res *editorial.Result, reveal int, all bool) error {
	style, tty := "notty", false
	if f, ok := out.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		style, tty = "dark", true
	}
	fmt.Fprintf(out, "%s  %s\n%s\n", p.Code(), res.Title, res.URL)
	if res.Whole {
		fmt.Fprintln(out, "(no section for this problem: showing the whole post)")
	}
	n := 0
	opts := scrape.DefaultOptions()
	for _, b := range res.Blocks {
		if b.Spoiler {
			n++
			if !all && n > reveal {
				fmt.Fprintf(out, "\n> %s (hidden: --reveal %d)\n", b.Title, n)
				continue
			}
			fmt.Fprintf(out, "\n> %s\n", b.Title)
		}
		text, err := scrape.Render(`<div class="problem-statement">`+b.HTML+`</div>`, 80, style, opts)
		if err != nil {
			return err
		}
		if !tty {
			text = osc8.ReplaceAllString(text, "")
		}
		fmt.Fprintf(out, "\n%s\n", text)
	}
	if len(res.Links) > 0 {
		fmt.Fprintln(out, "\nMore materials")
		for _, l := range res.Links {
			fmt.Fprintf(out, "  %-22s %s\n", l.Label, l.URL)
		}
	}
	return nil
}
