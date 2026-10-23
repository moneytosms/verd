package config

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

type keyKind int

const (
	kString keyKind = iota
	kFloat
	kInt
	kBool
	kStrings
)

// settable lists the top-level keys Set edits, with their type and (optional) allowed values.
var settable = map[string]struct {
	kind keyKind
	opts []string
}{
	"handle":             {kString, nil},
	"workspace":          {kString, nil},
	"snippets_dir":       {kString, nil},
	"default_lang":       {kString, nil},
	"theme":              {kString, nil},
	"background":         {kString, []string{"auto", "dark", "light"}},
	"editor":             {kString, nil},
	"split":              {kString, []string{"auto", "tmux", "herdr", "embedded", "suspend"}},
	"embed_focus_key":    {kString, nil},
	"embed_side":         {kString, []string{"left", "right"}},
	"embed_zoom":         {kBool, nil},
	"submit_mode":        {kString, []string{"browser", "direct"}},
	"browser":            {kString, nil},
	"time_multiplier":    {kFloat, nil},
	"float_eps":          {kFloat, nil},
	"embed_ratio":        {kFloat, nil},
	"autotest":           {kBool, nil},
	"resume":             {kBool, nil},
	"source_cf":          {kBool, nil},
	"source_cses":        {kBool, nil},
	"images":             {kString, []string{"auto", "kitty", "blocks", "off"}},
	"reading_width":      {kInt, nil},
	"reading_margin":     {kInt, nil},
	"reading_spacing":    {kString, []string{"compact", "normal", "relaxed"}},
	"reading_headings":   {kString, []string{"plain", "bold", "bar", "underline"}},
	"reading_math":       {kString, []string{"unicode", "latex", "raw"}},
	"reading_emphasis":   {kBool, nil},
	"reading_split":      {kString, []string{"full", "cap"}},
	"reading_palette":    {kString, ReadingPalettes},
	"reading_night_from": {kInt, nil},
	"border":             {kString, []string{"rounded", "square", "heavy", "double", "ascii", "none"}},
	"mouse":              {kBool, nil},
	"wheel_lines":        {kInt, nil},
	"mouse_select":       {kBool, nil},
	"header":             {kBool, nil},
	"footer":             {kBool, nil},
	"tabs":               {kStrings, nil},
}

// ReadingPalettes are the values of reading_palette.
var ReadingPalettes = []string{"app", "paper", "sepia", "cool", "night", "mono", "contrast", "auto"}

// setRanges are the inclusive bounds Set enforces on int keys (reading_width has its own rule).
var setRanges = map[string][2]int64{"reading_margin": {0, 8}, "reading_night_from": {0, 23}, "wheel_lines": {1, 20}}

// Set writes one top-level key to the config file at path, keeping every comment and other line.
// An existing assignment is replaced in place; a commented-out default (`# key = ...`) is
// uncommented with the new value; otherwise the key is added before the first table.
// value is parsed by the key's type (string, float or bool); enum keys are checked.
func Set(path, key, value string) error {
	spec, ok := settable[key]
	if !ok {
		return fmt.Errorf("%q cannot be set from here", key)
	}
	lit, err := parseValue(key, spec.kind, value)
	if err != nil {
		return err
	}
	if len(spec.opts) > 0 && !slices.Contains(spec.opts, value) {
		return fmt.Errorf("%s: %q is not one of %s", key, value, strings.Join(spec.opts, ", "))
	}
	enc, err := toml.Marshal(map[string]any{key: lit})
	if err != nil {
		return err
	}
	b, err := readConfig(path)
	if err != nil {
		return err
	}
	lines := placeLine(strings.Split(string(b), "\n"), key, strings.TrimRight(string(enc), "\n"))
	return saveFile(path, strings.Join(lines, "\n"))
}

// parseValue converts value to the Go type of the key's kind, applying that key's range checks.
func parseValue(key string, kind keyKind, value string) (any, error) {
	switch kind {
	case kFloat:
		return parseFloat(key, value)
	case kInt:
		return parseInt(key, value)
	case kBool:
		b, err := strconv.ParseBool(strings.TrimSpace(value))
		if err != nil {
			return nil, fmt.Errorf("%s: %q is not true or false", key, value)
		}
		return b, nil
	case kStrings:
		return parseList(key, value)
	}
	return value, nil
}

func parseFloat(key, value string) (float64, error) {
	f, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || f < 0 {
		return 0, fmt.Errorf("%s: %q is not a non-negative number", key, value)
	}
	if key == "embed_ratio" && (f < 0.1 || f > 0.9) {
		return 0, fmt.Errorf("embed_ratio: %v is outside 0.1 to 0.9", value)
	}
	return f, nil
}

func parseInt(key, value string) (int, error) {
	i, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: %q is not a number", key, value)
	}
	if key == "reading_width" {
		if i < 0 || i > 200 {
			return 0, fmt.Errorf("reading_width: %d is outside 0 to 200", i)
		}
		if i != 0 && i < 40 {
			return 0, fmt.Errorf("reading_width: %d is outside 40 to 200 (or 0 for unlimited)", i)
		}
	}
	if r, ok := setRanges[key]; ok {
		if err := rangeErr(key, i, r[0], r[1]); err != nil {
			return 0, err
		}
	}
	return int(i), nil
}

// parseList splits a comma list and trims each item. tabs must name real tabs and keep settings.
func parseList(key, value string) ([]string, error) {
	parts := strings.Split(strings.TrimSpace(value), ",")
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	if key != "tabs" {
		return parts, nil
	}
	if len(parts) == 0 {
		return nil, fmt.Errorf("tabs: cannot be empty")
	}
	valid := map[string]bool{"problems": true, "contests": true, "stats": true, "picker": true, "settings": true}
	for _, p := range parts {
		if !valid[p] {
			return nil, fmt.Errorf("tabs: %q is not one of problems, contests, stats, picker, settings", p)
		}
	}
	if !slices.Contains(parts, "settings") {
		return nil, fmt.Errorf("tabs: settings tab cannot be hidden")
	}
	return parts, nil
}

// placeLine replaces key's live or commented-out line with line, else inserts line before the
// first table.
func placeLine(lines []string, key, line string) []string {
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
	if at >= 0 {
		lines[at] = line
		return lines
	}
	ins := len(lines)
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "[") {
			ins = i
			break
		}
	}
	return append(lines[:ins], append([]string{line, ""}, lines[ins:]...)...)
}

// readConfig reads path; a missing file reads as empty.
func readConfig(path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return b, nil
}

// saveFile writes content to path, creating parent dirs and keeping the file's mode if it exists.
func saveFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	mode := fs.FileMode(0o644)
	if st, err := os.Stat(path); err == nil {
		mode = st.Mode()
	}
	return os.WriteFile(path, []byte(content), mode)
}

// Options returns the allowed values of an enum key, or nil.
func Options(key string) []string { return settable[key].opts }

// SettableKeys lists the keys Set accepts, sorted.
func SettableKeys() []string {
	out := make([]string, 0, len(settable))
	for k := range settable {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// KeyInfo describes a key `verd config set` accepts: for the JSON Schema and completions.
type KeyInfo struct {
	Name    string
	Type    string   // JSON Schema type: string, number, integer, boolean or array
	Options []string // allowed values of an enum key
}

// KeyInfos lists every settable key, sorted by name.
func KeyInfos() []KeyInfo {
	types := map[keyKind]string{kString: "string", kFloat: "number", kInt: "integer", kBool: "boolean", kStrings: "array"}
	var out []KeyInfo
	for _, k := range SettableKeys() {
		out = append(out, KeyInfo{k, types[settable[k].kind], settable[k].opts})
	}
	return out
}
