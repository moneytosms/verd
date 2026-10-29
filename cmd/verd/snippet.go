package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/moneytosms/verd/internal/config"
	"github.com/moneytosms/verd/internal/editor"
	"github.com/moneytosms/verd/internal/submit"
	"github.com/moneytosms/verd/internal/workspace"
)

// snippetCmd runs `verd snippet`: list the library or print one snippet (--copy: to the clipboard).
func snippetCmd(out io.Writer, cfg config.Config, args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "add", "edit", "rm", "open", "path":
			return snippetManage(out, cfg, args[0], args[1:])
		}
	}
	fs := flag.NewFlagSet("snippet", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	lang := fs.String("lang", cfg.CLIDefaultLang(), "language")
	list := fs.Bool("list", false, "list snippets")
	cp := fs.Bool("copy", false, "copy to the clipboard instead of printing")
	export := fs.Bool("export", false, "print the library as VSCode-style JSON")
	if err := fs.Parse(args); err != nil || !snippetArgsOK(fs, *list || *export) {
		return &exitError{2, "usage: verd snippet [--lang L] --list | --export | [--copy] <name> | add|edit|rm|open|path (see verd help)"}
	}
	l, err := langOf(cfg, *lang)
	if err != nil {
		return &exitError{2, err.Error()}
	}
	dir := cfg.SnippetsPath()
	switch {
	case *export:
		return exportSnippets(out, workspace.ListSnippets(dir, *lang, l))
	case *list:
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

// snippetArgsOK: --list and --export take no name, a snippet takes exactly one.
func snippetArgsOK(fs *flag.FlagSet, listing bool) bool {
	if listing {
		return fs.NArg() == 0
	}
	return fs.NArg() == 1
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

var snippetName = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// snippetManage runs the library-editing subcommands: add, edit, rm, open, path.
func snippetManage(out io.Writer, cfg config.Config, sub string, args []string) error {
	fs := flag.NewFlagSet("snippet "+sub, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	lang := fs.String("lang", cfg.CLIDefaultLang(), "language")
	desc := fs.String("desc", "", "one-line description")
	force := fs.Bool("force", false, "overwrite an existing snippet")
	usage := map[string]string{
		"add":  "usage: verd snippet add [--lang L] [--desc D] [--force] <name> [file|-]",
		"edit": "usage: verd snippet edit [--lang L] <name>",
		"rm":   "usage: verd snippet rm [--lang L] <name>",
		"open": "usage: verd snippet open",
		"path": "usage: verd snippet path",
	}[sub]
	want := map[string][2]int{"add": {1, 2}, "edit": {1, 1}, "rm": {1, 1}, "open": {0, 0}, "path": {0, 0}}[sub]
	if err := fs.Parse(args); err != nil || fs.NArg() < want[0] || fs.NArg() > want[1] {
		return &exitError{2, usage}
	}
	l, err := langOf(cfg, *lang)
	if err != nil {
		return &exitError{2, err.Error()}
	}
	dir := cfg.SnippetsPath()
	switch sub {
	case "path":
		fmt.Fprintln(out, dir)
		return nil
	case "open":
		return openLibrary(cfg, dir)
	}
	name := fs.Arg(0)
	if !snippetName.MatchString(name) {
		return &exitError{2, "snippet name may only contain letters, digits, _ and -"}
	}
	switch sub {
	case "rm":
		return removeSnippet(out, dir, *lang, l, name)
	case "edit":
		return editSnippet(cfg, dir, *lang, l, name)
	}
	return addSnippet(out, cfg, dir, *lang, l, name, *desc, *force, fs.Arg(1))
}

// openLibrary opens the snippet library folder in the editor.
func openLibrary(cfg config.Config, dir string) error {
	if _, err := workspace.InitSnippets(dir, cfg.Lang, false); err != nil {
		return err
	}
	return runEditor(cfg, workspace.LibraryRoot(dir), "")
}

// removeSnippet deletes a user snippet; built-ins cannot be removed.
func removeSnippet(out io.Writer, dir, lang string, l config.Lang, name string) error {
	if err := workspace.RemoveSnippet(dir, lang, l, name); err != nil {
		return &exitError{2, "no user snippet " + name + " for " + lang + " (built-ins cannot be removed, override them with edit)"}
	}
	fmt.Fprintln(out, "removed", workspace.SnippetPath(dir, lang, l, name))
	return nil
}

// editSnippet opens a snippet in the editor, writing an editable copy first if it is a built-in.
func editSnippet(cfg config.Config, dir, lang string, l config.Lang, name string) error {
	s, err := workspace.FindSnippet(dir, lang, l, name)
	if err != nil {
		return &exitError{2, err.Error()}
	}
	path := workspace.SnippetPath(dir, lang, l, s.Name)
	if _, err := os.Stat(path); err != nil { // built-in: write an editable copy first
		if err := writeSnippet(path, s.Code); err != nil {
			return err
		}
	}
	return runEditor(cfg, path, path)
}

// addSnippet adds a snippet from src ("" opens a template in the editor, "-" reads stdin, else a file).
func addSnippet(out io.Writer, cfg config.Config, dir, lang string, l config.Lang, name, desc string, force bool, src string) error {
	path := workspace.SnippetPath(dir, lang, l, name)
	for _, e := range workspace.ListSnippets(dir, lang, l) {
		if e.Name == name && !force {
			return &exitError{2, name + " already exists for " + lang + ": use `verd snippet edit " + name + "` or --force"}
		}
	}
	if src == "" {
		if desc == "" {
			desc = "describe " + name
		}
		if err := writeSnippet(path, workspace.WithDescription("", name, desc, l)); err != nil {
			return err
		}
		return runEditor(cfg, path, path)
	}
	code, err := readSnippetSource(src)
	if err != nil {
		return err
	}
	if strings.TrimSpace(code) == "" {
		return &exitError{2, "nothing to add: the snippet is empty"}
	}
	if !strings.HasSuffix(code, "\n") {
		code += "\n"
	}
	if err := writeSnippet(path, workspace.WithDescription(code, name, desc, l)); err != nil {
		return err
	}
	fmt.Fprintln(out, "added", path)
	return nil
}

// readSnippetSource reads a snippet body from stdin ("-") or a file.
func readSnippetSource(src string) (string, error) {
	if src == "-" {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	b, err := os.ReadFile(src)
	if err != nil {
		return "", &exitError{2, err.Error()}
	}
	return string(b), nil
}

func writeSnippet(path, code string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(code), 0o644)
}

// runEditor runs the configured editor in the foreground on target (a file or a folder).
func runEditor(cfg config.Config, target, file string) error {
	ctrl := &editor.Controller{}
	ctrl.SetEditor(cfg.Editor)
	if err := requireEditor(ctrl); err != nil {
		return &exitError{2, err.Error()}
	}
	var cmd *exec.Cmd
	if file != "" {
		cmd, _ = ctrl.Open(file, 1) // no multiplexer: a foreground command
	} else {
		cmd = ctrl.Command(target, target)
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

// editLibrarySnippet opens a library snippet in the editor pane. A built-in gets an editable copy
// first; add creates a new snippet from a template instead.
func editLibrarySnippet(cfg config.Config, ctrl *editor.Controller, lang, name string, add bool) (*exec.Cmd, error) {
	if err := requireEditor(ctrl); err != nil {
		return nil, err
	}
	l, err := langOf(cfg, lang)
	if err != nil {
		return nil, err
	}
	dir := cfg.SnippetsPath()
	path := workspace.SnippetPath(dir, lang, l, name)
	if add {
		if !snippetName.MatchString(name) {
			return nil, errors.New("snippet name may only contain letters, digits, _ and -")
		}
		for _, s := range workspace.ListSnippets(dir, lang, l) {
			if s.Name == name {
				return nil, errors.New(name + " already exists for " + lang + ": edit it instead")
			}
		}
		if err := writeSnippet(path, workspace.WithDescription("", name, "describe "+name, l)); err != nil {
			return nil, err
		}
		return ctrl.Open(path, 1)
	}
	s, err := workspace.FindSnippet(dir, lang, l, name)
	if err != nil {
		return nil, err
	}
	path = workspace.SnippetPath(dir, lang, l, s.Name)
	if _, err := os.Stat(path); err != nil {
		if err := writeSnippet(path, s.Code); err != nil {
			return nil, err
		}
	}
	return ctrl.Open(path, 1)
}

// openLibraryFolder opens the snippet library folder in the editor pane.
func openLibraryFolder(cfg config.Config, ctrl *editor.Controller) (*exec.Cmd, error) {
	if err := requireEditor(ctrl); err != nil {
		return nil, err
	}
	dir := cfg.SnippetsPath()
	if _, err := workspace.InitSnippets(dir, cfg.Lang, false); err != nil {
		return nil, err
	}
	return ctrl.Open(workspace.LibraryRoot(dir), 1)
}
