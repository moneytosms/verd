package workspace

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/moneytosms/verd/internal/config"
)

//go:embed templates/*
var defaultTemplates embed.FS

// TemplateVars are the values available to a Template.
type TemplateVars struct {
	Problem struct{ ID, Name, URL string }
	Handle  string
	Date    string
}

func NewVars(contest int, index, name, handle string, now time.Time) TemplateVars {
	var v TemplateVars
	v.Problem.ID = fmt.Sprintf("%d%s", contest, index)
	v.Problem.Name = name
	v.Problem.URL = fmt.Sprintf("https://codeforces.com/problemset/problem/%d/%s", contest, index)
	v.Handle, v.Date = handle, now.Format("2006-01-02")
	return v
}

const cursorMarker = "\x00verd-cursor\x00"

// RenderTemplate executes a Go text/template. `{{cursor}}` marks where the editor cursor starts:
// it is stripped from the output and its 1-based line returned (1 if the Template has none).
func RenderTemplate(src string, v TemplateVars) (text string, cursorLine int, err error) {
	t, err := template.New("solution").Funcs(template.FuncMap{"cursor": func() string { return cursorMarker }}).Parse(src)
	if err != nil {
		return "", 0, err
	}
	var b strings.Builder
	if err := t.Execute(&b, v); err != nil {
		return "", 0, err
	}
	text, cursorLine = b.String(), 1
	if i := strings.Index(text, cursorMarker); i >= 0 {
		cursorLine = strings.Count(text[:i], "\n") + 1
		text = strings.Replace(text, cursorMarker, "", 1)
	}
	return text, cursorLine, nil
}

// templateFile is the file name a language's Template has in the templates directory.
func templateFile(langKey string, l config.Lang) string { return langKey + "." + l.Ext }

// kinds of Template besides the Solution's own: stress-testing helpers.
var kinds = []string{"", "gen.", "brute."}

// LoadKindTemplate is LoadTemplate for "gen." or "brute." Templates.
func LoadKindTemplate(dir, kind, langKey string, l config.Lang) (string, error) {
	name := kind + templateFile(langKey, l)
	if b, err := os.ReadFile(filepath.Join(dir, name)); err == nil {
		return string(b), nil
	}
	b, _ := defaultTemplates.ReadFile("templates/" + name)
	return string(b), nil
}

// EnsureKind creates <kind>main-style helper "gen.<ext>" / "brute.<ext>" in the Problem dir if absent.
func (w Workspace) EnsureKind(contest int, index, kind string, l config.Lang, tmpl string) (string, error) {
	path := filepath.Join(w.Dir(contest, index), kind+l.Ext)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if errors.Is(err, fs.ErrExist) {
		return path, nil
	}
	if err != nil {
		return "", err
	}
	defer f.Close()
	_, err = f.WriteString(tmpl)
	return path, err
}

// LoadTemplate returns the user's Template from dir if present, else the embedded default
// (empty if the language has none).
func LoadTemplate(dir, langKey string, l config.Lang) (string, error) {
	name := templateFile(langKey, l)
	if b, err := os.ReadFile(filepath.Join(dir, name)); err == nil {
		return string(b), nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	if b, err := defaultTemplates.ReadFile("templates/" + name); err == nil {
		return string(b), nil
	}
	return "", nil
}

// InitTemplates writes the embedded default Templates into dir, skipping files that exist unless force.
// It returns the paths written.
func InitTemplates(dir string, langs map[string]config.Lang, force bool) ([]string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	var keys []string
	for k := range langs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var written []string
	for _, k := range keys {
		for _, kind := range kinds {
			name := kind + templateFile(k, langs[k])
			b, err := defaultTemplates.ReadFile("templates/" + name)
			if err != nil {
				continue // no shipped default for this language
			}
			path := filepath.Join(dir, name)
			if _, err := os.Stat(path); err == nil && !force {
				continue
			}
			if err := os.WriteFile(path, b, 0o644); err != nil {
				return written, err
			}
			written = append(written, path)
		}
	}
	return written, nil
}

// Ensure returns the Solution path for a Problem and language, creating it from tmpl if absent.
// An existing Solution is never overwritten: created=false and the cursor starts at line 1.
func (w Workspace) Ensure(contest int, index, langKey string, l config.Lang, tmpl string, v TemplateVars) (path string, cursorLine int, created bool, err error) {
	dir := w.Dir(contest, index)
	path = filepath.Join(dir, "main."+l.Ext)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", 0, false, err
	}
	text, line, err := RenderTemplate(tmpl, v)
	if err != nil {
		return "", 0, false, fmt.Errorf("template for %s: %w", langKey, err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if errors.Is(err, fs.ErrExist) {
		return path, 1, false, nil
	}
	if err != nil {
		return "", 0, false, err
	}
	defer f.Close()
	if _, err := f.WriteString(text); err != nil {
		return "", 0, false, err
	}
	return path, line, true, nil
}

// Warning returns a note when the Workspace is on a slow Windows mount under WSL, else "".
func Warning(root string, wsl bool) string {
	if wsl && strings.HasPrefix(filepath.ToSlash(root), "/mnt/") {
		return "workspace is under /mnt/ (slow I/O under WSL); consider a path in the Linux filesystem"
	}
	return ""
}
