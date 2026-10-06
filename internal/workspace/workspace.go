// Package workspace maps Problems to directories and files: <root>/<contest>/<index>/.
package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/config"
	"github.com/moneytosms/verd/internal/scrape"
)

type Workspace struct{ Root string }

// New expands a leading ~ in root and makes a relative root absolute (so "." means the launch directory).
func New(root string) Workspace {
	if root == "~" || strings.HasPrefix(root, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			root = filepath.Join(home, strings.TrimPrefix(root, "~"))
		}
	}
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	return Workspace{Root: root}
}

// Dir is a Problem's directory: <root>/<contest>/<index> for Codeforces, <root>/<source>/<id> for
// every other source (whose Index is its source tag).
func (w Workspace) Dir(contest int, index string) string {
	if cf.IndexSource(index) != cf.SourceCF {
		return filepath.Join(w.Root, index, strconv.Itoa(contest))
	}
	return filepath.Join(w.Root, strconv.Itoa(contest), index)
}

// Ref identifies a Solution file.
type Ref struct {
	Contest int
	Index   string
	Lang    string // key into config Lang
	Path    string // absolute
}

// Code is the Problem's short id, as cf.Problem.Code.
func (r Ref) Code() string { return cf.Problem{ContestID: r.Contest, Index: r.Index}.Code() }

// Resolve maps a path inside the Workspace back to its Problem and language (by extension).
func (w Workspace) Resolve(path string, langs map[string]config.Lang) (Ref, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return Ref{}, err
	}
	root, err := filepath.Abs(w.Root)
	if err != nil {
		return Ref{}, err
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return Ref{}, fmt.Errorf("%s is not inside the workspace %s", path, root)
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) < 3 {
		return Ref{}, fmt.Errorf("%s: expected <contest>/<index>/<file> under the workspace", path)
	}
	contest, index := 0, parts[1]
	if cf.IndexSource(parts[0]) != cf.SourceCF { // <source>/<id>/<file>
		contest, err = strconv.Atoi(parts[1])
		index = parts[0]
		if err != nil {
			return Ref{}, fmt.Errorf("%s: %q is not a task id", path, parts[1])
		}
	} else if contest, err = strconv.Atoi(parts[0]); err != nil {
		return Ref{}, fmt.Errorf("%s: %q is not a contest id", path, parts[0])
	}
	ext := strings.TrimPrefix(filepath.Ext(abs), ".")
	var keys []string
	for k, l := range langs {
		if l.Ext == ext {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return Ref{}, fmt.Errorf("%s: no configured language uses extension .%s", path, ext)
	}
	sort.Strings(keys) // deterministic if several languages share an extension
	return Ref{Contest: contest, Index: index, Lang: keys[0], Path: abs}, nil
}

// Test is one input/expected-output pair on disk.
type Test struct{ Name, In, Ans string }

// WriteSamples materializes Sample Tests as tests/sample-N.in/.ans (1-based), overwriting.
func (w Workspace) WriteSamples(contest int, index string, samples []scrape.Sample) error {
	dir := filepath.Join(w.Dir(contest, index), "tests")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for i, s := range samples {
		base := filepath.Join(dir, fmt.Sprintf("sample-%d", i+1))
		if err := os.WriteFile(base+".in", []byte(s.Input), 0o644); err != nil {
			return err
		}
		if err := os.WriteFile(base+".ans", []byte(s.Output), 0o644); err != nil {
			return err
		}
	}
	return nil
}

var testName = regexp.MustCompile(`^(sample|custom)-(\d+)\.in$`)

// Tests lists complete .in/.ans pairs: samples first, then custom, each in numeric order.
func (w Workspace) Tests(contest int, index string) ([]Test, error) {
	dir := filepath.Join(w.Dir(contest, index), "tests")
	ents, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	type key struct {
		kind string
		n    int
	}
	var keys []key
	for _, e := range ents {
		if m := testName.FindStringSubmatch(e.Name()); m != nil {
			n, _ := strconv.Atoi(m[2])
			keys = append(keys, key{m[1], n})
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].kind != keys[j].kind {
			return keys[i].kind == "sample"
		}
		return keys[i].n < keys[j].n
	})
	var out []Test
	for _, k := range keys {
		base := filepath.Join(dir, fmt.Sprintf("%s-%d", k.kind, k.n))
		if _, err := os.Stat(base + ".ans"); err != nil {
			continue // incomplete pair
		}
		out = append(out, Test{Name: filepath.Base(base), In: base + ".in", Ans: base + ".ans"})
	}
	return out, nil
}

var customName = regexp.MustCompile(`^custom-(\d+)\.(in|ans)$`)

// NextCustom creates the next empty tests/custom-N.in/.ans pair. N is one past the highest
// existing custom number, so existing files (even a lone .in) are never reused or overwritten.
func (w Workspace) NextCustom(contest int, index string) (n int, in, ans string, err error) {
	dir := filepath.Join(w.Dir(contest, index), "tests")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return 0, "", "", err
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		return 0, "", "", err
	}
	for _, e := range ents {
		if m := customName.FindStringSubmatch(e.Name()); m != nil {
			if k, _ := strconv.Atoi(m[1]); k > n {
				n = k
			}
		}
	}
	n++
	base := filepath.Join(dir, fmt.Sprintf("custom-%d", n))
	in, ans = base+".in", base+".ans"
	for _, p := range []string{in, ans} {
		f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			return 0, "", "", err
		}
		f.Close()
	}
	return n, in, ans, nil
}

// SaveCustom writes input and answer as the next Custom Test and returns its name.
func (w Workspace) SaveCustom(contest int, index, input, ans string) (string, error) {
	n, in, out, err := w.NextCustom(contest, index)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(in, []byte(input), 0o644); err != nil {
		return "", err
	}
	if err := os.WriteFile(out, []byte(ans), 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("custom-%d", n), nil
}

// CustomTest is a Custom Test with its contents.
type CustomTest struct{ Name, Input, Want string }

// Customs lists the Problem's Custom Tests in numeric order, with their contents.
func (w Workspace) Customs(contest int, index string) ([]CustomTest, error) {
	ts, err := w.Tests(contest, index)
	if err != nil {
		return nil, err
	}
	var out []CustomTest
	for _, t := range ts {
		if !strings.HasPrefix(t.Name, "custom-") {
			continue
		}
		in, err := os.ReadFile(t.In)
		if err != nil {
			return nil, err
		}
		ans, err := os.ReadFile(t.Ans)
		if err != nil {
			return nil, err
		}
		out = append(out, CustomTest{t.Name, string(in), string(ans)})
	}
	return out, nil
}

// SaveCase writes the Custom Test called name (a new one when name is empty) and returns its name.
func (w Workspace) SaveCase(contest int, index, name, input, want string) (string, error) {
	if name == "" {
		return w.SaveCustom(contest, index, input, want)
	}
	if !customFile.MatchString(name) {
		return "", fmt.Errorf("%q is not a Custom Test name", name)
	}
	base := filepath.Join(w.Dir(contest, index), "tests", name)
	if err := os.MkdirAll(filepath.Dir(base), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(base+".in", []byte(input), 0o644); err != nil {
		return "", err
	}
	return name, os.WriteFile(base+".ans", []byte(want), 0o644)
}

var customFile = regexp.MustCompile(`^custom-\d+$`)

// DeleteCustom removes a Custom Test's files; samples cannot be deleted.
func (w Workspace) DeleteCustom(contest int, index, name string) error {
	if !customFile.MatchString(name) {
		return fmt.Errorf("%q is not a Custom Test name", name)
	}
	base := filepath.Join(w.Dir(contest, index), "tests", name)
	return errors.Join(removeIfExists(base+".in"), removeIfExists(base+".ans"))
}

func removeIfExists(p string) error {
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
