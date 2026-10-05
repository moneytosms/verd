package config

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

type keyKind int

const (
	kString keyKind = iota
	kFloat
	kBool
)

// settable lists the top-level keys Set edits, with their type and (optional) allowed values.
var settable = map[string]struct {
	kind keyKind
	opts []string
}{
	"handle":          {kString, nil},
	"workspace":       {kString, nil},
	"default_lang":    {kString, nil},
	"theme":           {kString, nil},
	"background":      {kString, []string{"auto", "dark", "light"}},
	"split":           {kString, []string{"auto", "tmux", "herdr", "embedded", "suspend"}},
	"embed_focus_key": {kString, nil},
	"submit_mode":     {kString, []string{"browser", "direct"}},
	"time_multiplier": {kFloat, nil},
	"float_eps":       {kFloat, nil},
	"embed_ratio":     {kFloat, nil},
	"autotest":        {kBool, nil},
}

// Set writes one top-level key to the config file at path, keeping every comment and other line.
// An existing assignment is replaced in place; a commented-out default (`# key = ...`) is
// uncommented with the new value; otherwise the key is added before the first table.
// value is parsed by the key's type (string, float or bool); enum keys are checked.
func Set(path, key, value string) error {
	spec, ok := settable[key]
	if !ok {
		return fmt.Errorf("%q cannot be set from here", key)
	}
	var lit any = value
	switch spec.kind {
	case kFloat:
		f, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil || f < 0 {
			return fmt.Errorf("%s: %q is not a non-negative number", key, value)
		}
		lit = f
	case kBool:
		b, err := strconv.ParseBool(strings.TrimSpace(value))
		if err != nil {
			return fmt.Errorf("%s: %q is not true or false", key, value)
		}
		lit = b
	}
	if len(spec.opts) > 0 {
		valid := false
		for _, o := range spec.opts {
			valid = valid || o == value
		}
		if !valid {
			return fmt.Errorf("%s: %q is not one of %s", key, value, strings.Join(spec.opts, ", "))
		}
	}
	if key == "embed_ratio" && (lit.(float64) < 0.1 || lit.(float64) > 0.9) {
		return fmt.Errorf("embed_ratio: %v is outside 0.1 to 0.9", value)
	}
	enc, err := toml.Marshal(map[string]any{key: lit})
	if err != nil {
		return err
	}
	line := strings.TrimRight(string(enc), "\n")

	b, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	lines := strings.Split(string(b), "\n")
	live := regexp.MustCompile(`^\s*` + regexp.QuoteMeta(key) + `\s*=`)
	commented := regexp.MustCompile(`^\s*#\s*` + regexp.QuoteMeta(key) + `\s*=`)
	at := -1
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "[") {
			break // top-level keys end at the first table
		}
		if live.MatchString(l) {
			at = i
			break
		}
		if at < 0 && commented.MatchString(l) {
			at = i
		}
	}
	switch {
	case at >= 0:
		lines[at] = line
	default:
		ins := len(lines)
		for i, l := range lines {
			if strings.HasPrefix(strings.TrimSpace(l), "[") {
				ins = i
				break
			}
		}
		lines = append(lines[:ins], append([]string{line, ""}, lines[ins:]...)...)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	mode := fs.FileMode(0o644)
	if st, err := os.Stat(path); err == nil {
		mode = st.Mode()
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), mode)
}

// Options returns the allowed values of an enum key, or nil.
func Options(key string) []string { return settable[key].opts }
