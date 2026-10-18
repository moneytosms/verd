package workspace

import (
	"embed"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/moneytosms/verd/internal/config"
)

//go:embed snippets
var defaultSnippets embed.FS

// Snippet is one reusable piece of code from the library: the file name is its name and the
// comment on its first line is the description.
type Snippet struct {
	Name, Desc, Code string
}

// libraryDir is where a language's snippet files live under the snippets dir.
func libraryDir(dir, langKey string) string { return filepath.Join(dir, "library", langKey) }

func newSnippet(name string, b []byte) Snippet {
	code := string(b)
	first, _, _ := strings.Cut(code, "\n")
	desc := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(first), "//"))
	if strings.HasPrefix(strings.TrimSpace(first), "#") {
		desc = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(first), "#"))
	}
	return Snippet{Name: name, Desc: desc, Code: code}
}

// ListSnippets returns the library for a language sorted by name: the embedded defaults, with
// files in <dir>/library/<lang>/ adding to them or replacing them by name.
func ListSnippets(dir, langKey string, l config.Lang) []Snippet {
	byName := map[string]Snippet{}
	read := func(fsys fs.FS, root string) {
		ents, _ := fs.ReadDir(fsys, root)
		for _, e := range ents {
			ext := "." + l.Ext
			if e.IsDir() || !strings.HasSuffix(e.Name(), ext) {
				continue
			}
			if b, err := fs.ReadFile(fsys, path.Join(root, e.Name())); err == nil {
				name := strings.TrimSuffix(e.Name(), ext)
				byName[name] = newSnippet(name, b)
			}
		}
	}
	read(defaultSnippets, "snippets/"+langKey)
	read(os.DirFS(libraryDir(dir, langKey)), ".")
	out := make([]Snippet, 0, len(byName))
	for _, s := range byName {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// FindSnippet looks a snippet up by exact name, else by unique prefix.
func FindSnippet(dir, langKey string, l config.Lang, name string) (Snippet, error) {
	var prefixed []Snippet
	for _, s := range ListSnippets(dir, langKey, l) {
		if s.Name == name {
			return s, nil
		}
		if strings.HasPrefix(s.Name, name) {
			prefixed = append(prefixed, s)
		}
	}
	switch len(prefixed) {
	case 0:
		return Snippet{}, errors.New("no " + langKey + " snippet named " + name + " (verd snippet --list)")
	case 1:
		return prefixed[0], nil
	}
	names := make([]string, len(prefixed))
	for i, s := range prefixed {
		names[i] = s.Name
	}
	return Snippet{}, errors.New(name + " is ambiguous: " + strings.Join(names, ", "))
}

// InitSnippets writes the embedded library into <dir>/library/<lang>/ so it can be edited, skipping
// files that exist unless force. It returns how many files it wrote.
func InitSnippets(dir string, langs map[string]config.Lang, force bool) (int, error) {
	n := 0
	for key, l := range langs {
		ents, err := fs.ReadDir(defaultSnippets, "snippets/"+key)
		if err != nil {
			continue // no shipped library for this language
		}
		dst := libraryDir(dir, key)
		if err := os.MkdirAll(dst, 0o755); err != nil {
			return n, err
		}
		for _, e := range ents {
			if !strings.HasSuffix(e.Name(), "."+l.Ext) {
				continue
			}
			path := filepath.Join(dst, e.Name())
			if _, err := os.Stat(path); err == nil && !force {
				continue
			}
			b, _ := defaultSnippets.ReadFile("snippets/" + key + "/" + e.Name())
			if err := os.WriteFile(path, b, 0o644); err != nil {
				return n, err
			}
			n++
		}
	}
	return n, nil
}
