package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/moneytosms/verd/internal/config"
	"github.com/moneytosms/verd/internal/submit"
	"github.com/moneytosms/verd/internal/workspace"
)

// snippetCmd runs `verd snippet`: list the library or print one snippet (--copy: to the clipboard).
func snippetCmd(out io.Writer, cfg config.Config, args []string) error {
	fs := flag.NewFlagSet("snippet", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	lang := fs.String("lang", cfg.DefaultLang, "language")
	list := fs.Bool("list", false, "list snippets")
	cp := fs.Bool("copy", false, "copy to the clipboard instead of printing")
	export := fs.Bool("export", false, "print the library as VSCode-style JSON")
	if err := fs.Parse(args); err != nil || (!*list && !*export && fs.NArg() != 1) || ((*list || *export) && fs.NArg() != 0) {
		return &exitError{2, "usage: verd snippet [--lang L] --list | --export | [--copy] <name>"}
	}
	l, err := langOf(cfg, *lang)
	if err != nil {
		return &exitError{2, err.Error()}
	}
	dir := cfg.SnippetsPath()
	if *export {
		return exportSnippets(out, workspace.ListSnippets(dir, *lang, l))
	}
	if *list {
		for _, s := range workspace.ListSnippets(dir, *lang, l) {
			fmt.Fprintf(out, "%-18s %s\n", s.Name, s.Desc)
		}
		return nil
	}
	s, err := workspace.FindSnippet(dir, *lang, l, fs.Arg(0))
	if err != nil {
		return &exitError{2, err.Error()}
	}
	if !*cp {
		_, err = io.WriteString(out, s.Code)
		return err
	}
	if err := copyText(s.Code); err != nil {
		return err
	}
	fmt.Fprintf(out, "copied %s (%s)\n", s.Name, *lang)
	return nil
}

// copyText puts text on the clipboard with the platform tool, falling back to OSC 52.
func copyText(text string) error {
	env := submit.Env{GOOS: runtime.GOOS, Getenv: os.Getenv, LookPath: exec.LookPath}
	if argv := env.ClipboardCommand(); argv != nil {
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Stdin = bytes.NewReader([]byte(text))
		if err := cmd.Run(); err == nil {
			return nil
		}
	}
	_, err := fmt.Fprint(os.Stderr, osc52(text))
	return err
}

// exportSnippets writes snippets as VSCode-style JSON (prefix = name), which LuaSnip, friendly-snippets
// and VSCode load. "\" and "$" are escaped because snippet bodies treat them as syntax.
func exportSnippets(out io.Writer, snips []workspace.Snippet) error {
	type entry struct {
		Prefix      string   `json:"prefix"`
		Body        []string `json:"body"`
		Description string   `json:"description"`
	}
	m := map[string]entry{}
	esc := strings.NewReplacer(`\`, `\\`, "$", `\$`)
	for _, s := range snips {
		var body []string
		for _, line := range strings.Split(strings.TrimRight(s.Code, "\n"), "\n") {
			body = append(body, esc.Replace(line))
		}
		m[s.Name] = entry{s.Name, body, s.Desc}
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(m)
}
